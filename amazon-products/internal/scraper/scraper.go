// Package scraper provides Amazon HTML parsing and browser utilities.
package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

const tagSpan = "span"

// Region defines an Amazon regional site.
type Region struct {
	Name     string // e.g. "United Kingdom"
	BaseURL  string // e.g. "https://www.amazon.co.uk"
	Currency string // e.g. "GBP"
}

// Regions maps region IDs to their configuration.
var Regions = map[string]Region{
	"au": {Name: "Australia", BaseURL: "https://www.amazon.com.au", Currency: "AUD"},
	"be": {Name: "Belgium", BaseURL: "https://www.amazon.com.be", Currency: "EUR"},
	"br": {Name: "Brazil", BaseURL: "https://www.amazon.com.br", Currency: "BRL"},
	"ca": {Name: "Canada", BaseURL: "https://www.amazon.ca", Currency: "CAD"},
	"eg": {Name: "Egypt", BaseURL: "https://www.amazon.eg", Currency: "EGP"},
	"fr": {Name: "France", BaseURL: "https://www.amazon.fr", Currency: "EUR"},
	"de": {Name: "Germany", BaseURL: "https://www.amazon.de", Currency: "EUR"},
	"in": {Name: "India", BaseURL: "https://www.amazon.in", Currency: "INR"},
	"ie": {Name: "Ireland", BaseURL: "https://www.amazon.co.uk", Currency: "GBP"},
	"it": {Name: "Italy", BaseURL: "https://www.amazon.it", Currency: "EUR"},
	"jp": {Name: "Japan", BaseURL: "https://www.amazon.co.jp", Currency: "JPY"},
	"mx": {Name: "Mexico", BaseURL: "https://www.amazon.com.mx", Currency: "MXN"},
	"nl": {Name: "Netherlands", BaseURL: "https://www.amazon.nl", Currency: "EUR"},
	"pl": {Name: "Poland", BaseURL: "https://www.amazon.pl", Currency: "PLN"},
	"sa": {Name: "Saudi Arabia", BaseURL: "https://www.amazon.sa", Currency: "SAR"},
	"sg": {Name: "Singapore", BaseURL: "https://www.amazon.sg", Currency: "SGD"},
	"es": {Name: "Spain", BaseURL: "https://www.amazon.es", Currency: "EUR"},
	"se": {Name: "Sweden", BaseURL: "https://www.amazon.se", Currency: "SEK"},
	"tr": {Name: "Turkey", BaseURL: "https://www.amazon.com.tr", Currency: "TRY"},
	"ae": {Name: "United Arab Emirates", BaseURL: "https://www.amazon.ae", Currency: "AED"},
	"uk": {Name: "United Kingdom", BaseURL: "https://www.amazon.co.uk", Currency: "GBP"},
	"us": {Name: "United States", BaseURL: "https://www.amazon.com", Currency: "USD"},
}

// Product represents an Amazon product.
type Product struct {
	ASIN         string    `json:"asin"`
	Name         string    `json:"name"`
	Price        float64   `json:"price,omitempty"`
	Currency     string    `json:"currency"`
	URL          string    `json:"url"`
	ImageURL     string    `json:"imageURL,omitempty"`
	Rating       string    `json:"rating,omitempty"`
	ReviewCount  string    `json:"reviewCount,omitempty"`
	IsPrime      bool      `json:"isPrime,omitempty"`
	Brand        string    `json:"brand,omitempty"`
	Features     []string  `json:"features,omitempty"`
	Description  string    `json:"description,omitempty"`
	Availability string    `json:"availability,omitempty"`
	Variants     []Variant `json:"variants,omitempty"`
}

// Variant represents a product variation dimension (e.g. size, color).
type Variant struct {
	Dimension string          `json:"dimension"`
	Selected  string          `json:"selected"`
	Options   []VariantOption `json:"options"`
}

// VariantOption represents a single option within a variant dimension.
type VariantOption struct {
	Value string `json:"value"`
	ASIN  string `json:"asin,omitempty"`
	State string `json:"state"` // "SELECTED", "AVAILABLE", "UNAVAILABLE"
}

// Datasource provides access to Amazon product data via a headless browser.
type Datasource struct {
	browser *Browser
	region  Region
}

// NewDatasource creates a new Amazon datasource for the given region.
func NewDatasource(browser *Browser, region Region) *Datasource {
	return &Datasource{browser: browser, region: region}
}

// Region returns the region this datasource is configured for.
func (d *Datasource) Region() Region {
	return d.region
}

// SearchProducts searches Amazon for the given query.
func (d *Datasource) SearchProducts(ctx context.Context, query string) ([]Product, error) {
	searchURL := d.region.BaseURL + "/s?" + url.Values{"k": {query}}.Encode()
	body, err := d.browser.Fetch(ctx, searchURL, "")
	if err != nil {
		return nil, fmt.Errorf("amazon search fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck

	return ParseSearchResults(body, d.region)
}

// GetProductDetails fetches detailed information about a product by ASIN.
func (d *Datasource) GetProductDetails(ctx context.Context, asin string) (*Product, error) {
	productURL := d.region.BaseURL + "/dp/" + url.PathEscape(asin)
	body, err := d.browser.Fetch(ctx, productURL, `#productTitle`)
	if err != nil {
		return nil, fmt.Errorf("amazon product fetch: %w", err)
	}
	defer body.Close() //nolint:errcheck

	return ParseProductPage(body, asin, d.region)
}

// ParseSearchResults parses Amazon search results HTML.
func ParseSearchResults(r io.Reader, region Region) ([]Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("amazon: parse search HTML: %w", err)
	}

	var products []Product
	walkTree(doc, func(n *html.Node) {
		if !isSearchResult(n) {
			return
		}
		asin := getAttr(n, "data-asin")
		if asin == "" {
			return
		}
		p := extractSearchProduct(n, asin, region)
		if p.Name != "" {
			products = append(products, p)
		}
	})

	return products, nil
}

// ParseProductPage parses an Amazon product detail page.
func ParseProductPage(r io.Reader, asin string, region Region) (*Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("amazon: parse product HTML: %w", err)
	}

	p := &Product{
		ASIN:     asin,
		Currency: region.Currency,
		URL:      region.BaseURL + "/dp/" + asin,
	}

	extractProductFields(doc, p)
	return p, nil
}

func extractProductFields(doc *html.Node, p *Product) {
	if el := findByID(doc, "productTitle"); el != nil {
		p.Name = strings.TrimSpace(textContent(el))
	}

	p.Price = extractProductPrice(doc)

	if el := findByID(doc, "acrPopover"); el != nil {
		p.Rating = getAttr(el, "title")
	}
	if el := findByID(doc, "acrCustomerReviewText"); el != nil {
		p.ReviewCount = strings.TrimSpace(textContent(el))
	}

	if el := findByID(doc, "bylineInfo"); el != nil {
		brand := strings.TrimSpace(textContent(el))
		brand = strings.TrimPrefix(brand, "Brand: ")
		brand = strings.TrimPrefix(brand, "Visit the ")
		brand = strings.TrimSuffix(brand, " Store")
		p.Brand = brand
	}

	p.Features = extractFeatures(doc)

	if el := findByID(doc, "productDescription"); el != nil {
		p.Description = strings.TrimSpace(textContent(el))
	}

	if el := findByID(doc, "availability"); el != nil {
		p.Availability = strings.TrimSpace(textContent(el))
	}

	p.ImageURL = extractProductImage(doc)
	p.Variants = extractVariants(doc)
}

func extractProductPrice(doc *html.Node) float64 {
	for _, id := range []string{
		"corePriceDisplay_desktop_feature_div",
		"corePrice_desktop",
		"tp_price_block_total_price_ww",
		"corePrice_feature_div",
		"price_inside_buybox",
	} {
		if el := findByID(doc, id); el != nil {
			if price := extractFirstPrice(el); price != 0 {
				return price
			}
		}
	}
	return 0
}

func extractFeatures(doc *html.Node) []string {
	el := findByID(doc, "feature-bullets")
	if el == nil {
		return nil
	}
	var features []string
	walkTree(el, func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == tagSpan && hasClass(n, "a-list-item") {
			text := strings.TrimSpace(textContent(n))
			if text != "" && text != "About this item" {
				features = append(features, text)
			}
		}
	})
	return features
}

func extractProductImage(doc *html.Node) string {
	if el := findByID(doc, "landingImage"); el != nil {
		if src := getAttr(el, "src"); src != "" {
			return src
		}
	}
	if el := findByID(doc, "imgBlkFront"); el != nil {
		return getAttr(el, "src")
	}
	return ""
}

// extractVariants parses the "twister" JSON embedded in the product page
// to extract variant dimensions (size, color, style, etc.) and their options.
func extractVariants(doc *html.Node) []Variant {
	twisterJSON := findTwisterJSON(doc)
	if twisterJSON == "" {
		return nil
	}

	var data struct {
		SortedDimValues map[string][]struct {
			DimensionValueDisplayText string `json:"dimensionValueDisplayText"`
			DimensionValueState       string `json:"dimensionValueState"`
			DefaultAsin               string `json:"defaultAsin"`
		} `json:"sortedDimValuesForAllDims"`
	}
	if err := json.Unmarshal([]byte(twisterJSON), &data); err != nil {
		return nil
	}

	// Sort dimension names for deterministic output.
	var dimNames []string
	for name := range data.SortedDimValues {
		dimNames = append(dimNames, name)
	}
	sort.Strings(dimNames)

	var variants []Variant
	for _, dimName := range dimNames {
		values := data.SortedDimValues[dimName]
		if len(values) <= 1 {
			continue // Skip dimensions with only one option (no real choice).
		}

		v := Variant{
			Dimension: formatDimensionName(dimName),
		}
		for _, val := range values {
			opt := VariantOption{
				Value: val.DimensionValueDisplayText,
				ASIN:  val.DefaultAsin,
				State: val.DimensionValueState,
			}
			if opt.State == "SELECTED" {
				v.Selected = opt.Value
			}
			v.Options = append(v.Options, opt)
		}
		variants = append(variants, v)
	}
	return variants
}

// findTwisterJSON locates the <script type="a-state"> tag that contains
// the twister variant data and returns its text content.
func findTwisterJSON(doc *html.Node) string {
	var result string
	walkTree(doc, func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type != html.ElementNode || n.Data != "script" {
			return
		}
		if getAttr(n, "type") != "a-state" {
			return
		}
		if !strings.Contains(getAttr(n, "data-a-state"), "desktop-twister-sort-filter-data") {
			return
		}
		if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
			result = n.FirstChild.Data
		}
	})
	return result
}

// formatDimensionName converts a twister dimension key like "size_name"
// or "color_name" into a display name like "Size" or "Color".
func formatDimensionName(key string) string {
	key = strings.TrimSuffix(key, "_name")
	if key == "" {
		return key
	}
	return strings.ToUpper(key[:1]) + key[1:]
}

func isSearchResult(n *html.Node) bool {
	return n.Type == html.ElementNode &&
		n.Data == "div" &&
		getAttr(n, "data-component-type") == "s-search-result"
}

func extractSearchProduct(n *html.Node, asin string, region Region) Product {
	p := Product{
		ASIN:     asin,
		Currency: region.Currency,
		URL:      region.BaseURL + "/dp/" + asin,
	}

	p.Name = extractSearchTitle(n)
	p.Price = extractFirstPrice(n)
	p.ImageURL = extractSearchImage(n)
	p.Rating = extractSearchRating(n)
	p.IsPrime = hasSearchPrimeBadge(n)

	return p
}

func extractSearchTitle(n *html.Node) string {
	var name string
	walkTree(n, func(el *html.Node) {
		if name != "" || el.Type != html.ElementNode || el.Data != "h2" {
			return
		}
		text := strings.TrimSpace(textContent(el))
		if text != "" {
			name = text
		}
	})
	return name
}

func extractSearchImage(n *html.Node) string {
	var imgURL string
	walkTree(n, func(el *html.Node) {
		if imgURL != "" {
			return
		}
		if el.Type == html.ElementNode && el.Data == "img" && hasClass(el, "s-image") {
			imgURL = getAttr(el, "src")
		}
	})
	return imgURL
}

func extractSearchRating(n *html.Node) string {
	var rating string
	walkTree(n, func(el *html.Node) {
		if rating != "" {
			return
		}
		if el.Type == html.ElementNode && el.Data == tagSpan && hasClass(el, "a-icon-alt") {
			text := textContent(el)
			if looksLikeRating(text) {
				rating = text
			}
		}
	})
	return rating
}

// looksLikeRating checks whether text looks like an Amazon star-rating string
// such as "4.6 out of 5 stars" (EN), "4,6 von 5 Sternen" (DE), etc.
// It matches any text that starts with a digit and contains "5" — which
// covers all known Amazon rating formats across languages.
func looksLikeRating(s string) bool {
	return len(s) > 0 && s[0] >= '0' && s[0] <= '5' && strings.Contains(s, "5")
}

func hasSearchPrimeBadge(n *html.Node) bool {
	var found bool
	walkTree(n, func(el *html.Node) {
		if el.Type == html.ElementNode && el.Data == "i" && hasClass(el, "a-icon-prime") {
			found = true
		}
	})
	return found
}

func extractFirstPrice(n *html.Node) float64 {
	var price float64
	walkTree(n, func(el *html.Node) {
		if price != 0 {
			return
		}
		if el.Type == html.ElementNode && el.Data == tagSpan && hasClass(el, "a-offscreen") {
			if f := parsePrice(textContent(el)); f > 0 {
				price = f
			}
		}
	})
	return price
}

func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if !containsCurrencySymbol(s) {
		return 0
	}

	// Determine decimal format before stripping.
	// Comma is the decimal separator (EU format) if:
	//   - it appears after the last dot, AND
	//   - it is followed by exactly 1-2 digits (e.g. "29,99" or "1.299,5")
	// This avoids misinterpreting thousands separators like "¥2,999".
	lastComma := strings.LastIndex(s, ",")
	lastDot := strings.LastIndex(s, ".")
	commaDecimal := lastComma > lastDot && isDecimalSeparator(s, lastComma)

	// Keep only digits, dots, and commas.
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			b.WriteRune(r)
		}
	}
	num := b.String()

	if commaDecimal {
		num = strings.ReplaceAll(num, ".", "")
		num = strings.ReplaceAll(num, ",", ".")
	} else {
		num = strings.ReplaceAll(num, ",", "")
	}

	if num == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(num, 64)
	return f
}

// isDecimalSeparator checks if the character at position pos in s is a
// decimal separator rather than a thousands separator.  A decimal separator
// is followed by 1 or 2 trailing digits (e.g. ",99" or ",5").
func isDecimalSeparator(s string, pos int) bool {
	trailing := 0
	for i := pos + 1; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			trailing++
		}
	}
	return trailing >= 1 && trailing <= 2
}

func containsCurrencySymbol(s string) bool {
	// Check for common Unicode currency symbols and text indicators.
	for _, sym := range []string{
		"£", "$", "€", "¥", "￥", "₹", "₺",
		"R$", "zł", "kr", "TL",
		"ر.س", "د.إ", "ج.م",
	} {
		if strings.Contains(s, sym) {
			return true
		}
	}
	// Match any ISO 4217-style currency code: 3 uppercase letters not part
	// of a longer word (e.g. "GBP\u00a0975.41", "SAR 29.99").
	return containsISOCurrencyCode(s)
}

func containsISOCurrencyCode(s string) bool {
	runes := []rune(s)
	for i := 0; i <= len(runes)-3; i++ {
		if !isUpperLetter(runes[i]) || !isUpperLetter(runes[i+1]) || !isUpperLetter(runes[i+2]) {
			continue
		}
		// Check character before the code (if any) is not a letter.
		if i > 0 && isLetter(runes[i-1]) {
			continue
		}
		// Check character after the code (if any) is not a letter.
		if i+3 < len(runes) && isLetter(runes[i+3]) {
			continue
		}
		return true
	}
	return false
}

func isUpperLetter(r rune) bool {
	return r >= 'A' && r <= 'Z'
}

func isLetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func findByID(doc *html.Node, id string) *html.Node {
	var found *html.Node
	walkTree(doc, func(n *html.Node) {
		if found == nil && n.Type == html.ElementNode && getAttr(n, "id") == id {
			found = n
		}
	})
	return found
}

func walkTree(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkTree(c, fn)
	}
}

func textContent(n *html.Node) string {
	var sb strings.Builder
	walkTree(n, func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(sb.String())
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, cls string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, cls) {
			return true
		}
	}
	return false
}
