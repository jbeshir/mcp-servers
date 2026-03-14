// Package scraper provides HTML parsing utilities for supermarket datasources.
package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// ElemSel is a flexible HTML element selector supporting tag, class substring,
// and attribute matching.
type ElemSel struct {
	Tag string // HTML tag (required)
	Cls string // CSS class substring match (optional)
	Att string // attribute name (optional)
	Val string // attribute value (optional, empty = presence check)
}

// Matches returns true if the node matches this selector.
func (s ElemSel) Matches(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Data != s.Tag {
		return false
	}
	if s.Cls != "" && !hasClassContaining(n, s.Cls) {
		return false
	}
	if s.Att != "" {
		if s.Val != "" {
			if !hasAttrValue(n, s.Att, s.Val) {
				return false
			}
		} else if GetAttr(n, s.Att) == "" {
			return false
		}
	}
	return true
}

// Config defines per-supermarket selectors for HTML scraping.
type Config struct {
	ID          datasource.SupermarketID
	BaseURL     string
	Container   ElemSel
	SearchSel   ProductSelectors
	ProductSel  ProductSelectors
	CategorySel ElemSel
}

// ProductSelectors configures selectors for parsing search result product nodes.
type ProductSelectors struct {
	Title       ElemSel
	Link        ElemSel // optional: separate element for link href (if zero, title must be <a>)
	Price       ElemSel
	Unit        ElemSel
	Promo       ElemSel
	Image       ElemSel
	Weight      ElemSel
	Description ElemSel // product page only
	Ingredients ElemSel // product page only
}

// FetchHTML fetches a URL with browser-like headers and optional cookies,
// returning the response body. The caller must close the returned reader.
func FetchHTML(ctx context.Context, targetURL string, cookies []*http.Cookie,
	client *http.Client) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	SetBrowserHeaders(req)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, targetURL)
	}
	return resp.Body, nil
}

// ParseSearchResults parses search result HTML into a list of products.
func ParseSearchResults(r io.Reader, cfg Config) ([]datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%s: parse HTML: %w", cfg.ID, err)
	}

	var products []datasource.Product
	WalkTree(doc, func(n *html.Node) {
		if !cfg.Container.Matches(n) {
			return
		}
		if p, ok := ExtractProduct(n, cfg.SearchSel, cfg.BaseURL, cfg.ID); ok {
			products = append(products, p)
		}
	})

	if len(products) == 0 {
		return nil, fmt.Errorf(
			"%s: no products found in HTML (page may require JavaScript rendering)",
			cfg.ID,
		)
	}

	return products, nil
}

// ParseProductFields extracts product fields from a parsed HTML document.
// The caller retains the doc for post-processing (e.g. nutrition extraction).
func ParseProductFields(doc *html.Node, sel ProductSelectors, id datasource.SupermarketID) *datasource.Product {
	p := &datasource.Product{
		Supermarket: id,
		Currency:    "GBP",
		Available:   true,
	}
	matchers := pageMatchers(sel)

	WalkTree(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		applyMatchers(n, p, matchers)
	})

	return p
}

// ParseCategories parses a categories page into a list of categories.
func ParseCategories(r io.Reader, cfg Config) ([]datasource.Category, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%s: parse categories HTML: %w", cfg.ID, err)
	}

	var categories []datasource.Category
	WalkTree(doc, func(n *html.Node) {
		if !cfg.CategorySel.Matches(n) {
			return
		}
		catName := TextContent(n)
		href := GetAttr(n, "href")
		if catName == "" || href == "" {
			return
		}
		categories = append(categories, datasource.Category{
			ID:          LastPathSegment(href),
			Name:        catName,
			URL:         ResolveURL(cfg.BaseURL, href),
			Supermarket: cfg.ID,
		})
	})

	return categories, nil
}

// ExtractProduct builds a Product from a search result container node.
func ExtractProduct(
	n *html.Node,
	sel ProductSelectors,
	baseURL string,
	sid datasource.SupermarketID,
) (datasource.Product, bool) {
	p := datasource.Product{
		Supermarket: sid,
		Currency:    "GBP",
		Available:   true,
	}
	matchers := searchMatchers(sel, baseURL)

	WalkTree(n, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		applyMatchers(node, &p, matchers)
	})

	if p.Name == "" {
		return datasource.Product{}, false
	}
	return p, true
}

// FindElement returns the first element matching sel in the subtree, or nil.
func FindElement(root *html.Node, sel ElemSel) *html.Node {
	var found *html.Node
	WalkTree(root, func(n *html.Node) {
		if found == nil && sel.Matches(n) {
			found = n
		}
	})
	return found
}

// fieldMatcher describes how to match and extract a single product field.
type fieldMatcher struct {
	sel   ElemSel
	apply func(*html.Node, *datasource.Product)
}

func searchMatchers(sel ProductSelectors, baseURL string) []fieldMatcher {
	m := []fieldMatcher{
		{sel.Title, func(n *html.Node, p *datasource.Product) {
			p.Name = TextContent(n)
			if n.Data == "a" {
				if href := GetAttr(n, "href"); href != "" {
					p.URL = ResolveURL(baseURL, href)
					p.ID = LastPathSegment(href)
				}
			}
		}},
		{sel.Price, func(n *html.Node, p *datasource.Product) {
			p.Price = ParsePrice(TextContent(n))
		}},
		{sel.Unit, func(n *html.Node, p *datasource.Product) {
			p.PricePerUnit = CleanPricePerUnit(TextContent(n))
		}},
		{sel.Image, func(n *html.Node, p *datasource.Product) {
			p.ImageURL = GetAttr(n, "src")
		}},
	}
	if sel.Link != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Link, func(n *html.Node, p *datasource.Product) {
			if href := GetAttr(n, "href"); href != "" {
				p.URL = ResolveURL(baseURL, href)
				p.ID = LastPathSegment(href)
			}
		}})
	}
	if sel.Promo != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Promo, func(n *html.Node, p *datasource.Product) {
			p.Promotion = TextContent(n)
		}})
	}
	if sel.Weight != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Weight, func(n *html.Node, p *datasource.Product) {
			p.Weight = TextContent(n)
		}})
	}
	return m
}

func pageMatchers(sel ProductSelectors) []fieldMatcher {
	m := []fieldMatcher{
		{sel.Title, func(n *html.Node, p *datasource.Product) {
			p.Name = TextContent(n)
		}},
		{sel.Price, func(n *html.Node, p *datasource.Product) {
			p.Price = ParsePrice(TextContent(n))
		}},
		{sel.Unit, func(n *html.Node, p *datasource.Product) {
			p.PricePerUnit = CleanPricePerUnit(TextContent(n))
		}},
		{sel.Image, func(n *html.Node, p *datasource.Product) {
			p.ImageURL = GetAttr(n, "src")
		}},
	}
	if sel.Promo != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Promo, func(n *html.Node, p *datasource.Product) {
			p.Promotion = TextContent(n)
		}})
	}
	if sel.Weight != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Weight, func(n *html.Node, p *datasource.Product) {
			p.Weight = TextContent(n)
		}})
	}
	if sel.Description != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Description, func(n *html.Node, p *datasource.Product) {
			p.Description = TextContent(n)
		}})
	}
	if sel.Ingredients != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Ingredients, func(n *html.Node, p *datasource.Product) {
			p.Ingredients = TextContent(n)
		}})
	}
	return m
}

func applyMatchers(node *html.Node, p *datasource.Product, matchers []fieldMatcher) {
	for _, fm := range matchers {
		if fm.sel.Matches(node) {
			fm.apply(node, p)
			return
		}
	}
}

// WalkTree visits every node in the HTML tree, calling fn for each.
func WalkTree(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		WalkTree(c, fn)
	}
}

// CleanPricePerUnit strips screen-reader prefixes like "Price per unit".
func CleanPricePerUnit(s string) string {
	s = strings.TrimPrefix(s, "Price per unit")
	return strings.TrimSpace(s)
}

// ParsePrice extracts a float64 price from a string like "£1.50".
// It also handles strings with leading text like "Item price£1.50".
func ParsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "£"); i >= 0 {
		s = s[i:]
		s = strings.TrimPrefix(s, "£")
		s = strings.ReplaceAll(s, ",", "")
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// LastPathSegment extracts the last path segment from a URL or path string.
func LastPathSegment(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(u.Path, "/")
	return parts[len(parts)-1]
}

// ResolveURL resolves a potentially relative URL against a base URL.
func ResolveURL(baseURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	return baseURL + href
}

// GetAttr returns the value of the named attribute on an HTML node.
func GetAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasAttrValue(n *html.Node, key, value string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			for _, cls := range strings.Fields(a.Val) {
				if cls == value {
					return true
				}
			}
		}
	}
	return false
}

func hasClassContaining(n *html.Node, substr string) bool {
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, substr) {
			return true
		}
	}
	return false
}

// TextContent returns the concatenated text content of an HTML node tree.
func TextContent(n *html.Node) string {
	var sb strings.Builder
	WalkTree(n, func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(sb.String())
}

// SectionContent finds a heading element (e.g. "h2", "h3") whose text matches
// heading and returns the text content of the next sibling element.
func SectionContent(doc *html.Node, tag, heading string) string {
	var result string
	WalkTree(doc, func(n *html.Node) {
		if result != "" {
			return
		}
		if n.Type != html.ElementNode || n.Data != tag {
			return
		}
		if TextContent(n) != heading {
			return
		}
		for sib := n.NextSibling; sib != nil; sib = sib.NextSibling {
			if sib.Type == html.ElementNode {
				result = TextContent(sib)
				return
			}
		}
	})
	return result
}

// HTMLHasElement parses HTML and returns whether any element matches the selector.
func HTMLHasElement(r io.Reader, sel ElemSel) bool {
	doc, err := html.Parse(r)
	if err != nil {
		return false
	}
	var found bool
	WalkTree(doc, func(n *html.Node) {
		if !found && sel.Matches(n) {
			found = true
		}
	})
	return found
}

// SetBrowserHeaders sets standard browser-like headers on an HTTP request.
func SetBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (X11; Linux x86_64) "+
			"AppleWebKit/537.36 (KHTML, like Gecko) "+
			"Chrome/145.0.0.0 Safari/537.36")
	req.Header.Set("Accept",
		"text/html,application/xhtml+xml,"+
			"application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9")
}

// QuerySearchURL returns a search URL builder that appends ?<param>=<query>.
func QuerySearchURL(base, param string) func(string) string {
	return func(query string) string {
		return base + "?" + param + "=" + url.QueryEscape(query)
	}
}

// ProductURLBuilder returns a product URL builder that appends /<id> to the base.
func ProductURLBuilder(base string) func(string) string {
	return func(id string) string {
		return base + url.PathEscape(id)
	}
}

// nutritionHeaderRows are first-cell values that indicate a header row
// rather than a nutrient data row.
var nutritionHeaderRows = map[string]bool{
	"typical values":     true,
	"nutrient":           true,
	"nutritional values": true,
	"per":                true,
}

// ParseNutritionTable extracts nutrition data from an HTML <table> node.
// It expects rows where the first cell is the nutrient name, the second is
// per-100g value, and an optional third is per-portion value.
func ParseNutritionTable(tableNode *html.Node) *datasource.NutritionInfo {
	if tableNode == nil {
		return nil
	}

	info := &datasource.NutritionInfo{
		Per100g: make(map[string]string),
	}

	WalkTree(tableNode, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "tr" {
			return
		}
		cells := extractCells(n)
		if len(cells) < 2 || cells[0] == "" {
			return
		}
		if nutritionHeaderRows[strings.ToLower(cells[0])] {
			return
		}
		info.Per100g[cells[0]] = cells[1]
		if len(cells) >= 3 && cells[2] != "" {
			if info.PerPortion == nil {
				info.PerPortion = make(map[string]string)
			}
			info.PerPortion[cells[0]] = cells[2]
		}
	})

	if len(info.Per100g) == 0 {
		return nil
	}
	return info
}

// extractCells returns trimmed text content of <td> and <th> children of a <tr>.
func extractCells(tr *html.Node) []string {
	var cells []string
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, strings.TrimSpace(TextContent(c)))
		}
	}
	return cells
}

// FindNutritionTable locates the first <table> matching sel in the document,
// or if sel is zero-valued, the first <table> element.
func FindNutritionTable(doc *html.Node, sel ElemSel) *html.Node {
	target := sel
	if target == (ElemSel{}) {
		target = ElemSel{Tag: "table"}
	}
	return FindElement(doc, target)
}
