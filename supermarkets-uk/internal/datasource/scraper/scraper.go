// Package scraper provides an HTML scraping framework for supermarket datasources.
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

// Config defines all per-supermarket configuration for an HTML scraper.
type Config struct {
	ID          datasource.SupermarketID
	Name        string
	Description string
	BaseURL     string
	SearchURL   func(query string) string
	ProductURL  func(id string) string
	CategoryURL string
	Container   ElemSel
	SearchSel   ProductSelectors
	ProductSel  ProductSelectors
	CategorySel ElemSel

	SessionCheckURL   string  // URL to fetch for session validation
	SessionCheckQuery ElemSel // element selector that indicates authenticated state
}

// ProductSelectors configures selectors for parsing search result product nodes.
type ProductSelectors struct {
	Title  ElemSel
	Link   ElemSel // optional: separate element for link href (if zero, title must be <a>)
	Price  ElemSel
	Unit   ElemSel
	Promo  ElemSel
	Image  ElemSel
	Weight ElemSel
}

// Scraper implements datasource.Datasource by scraping server-rendered HTML via
// direct HTTP requests. Use this for sites whose SSR response contains product data.
type Scraper struct {
	cfg        Config
	cookies    []*http.Cookie
	httpClient *http.Client
}

// NewScraper creates a new HTTP-based HTML scraper datasource.
func NewScraper(cfg Config) *Scraper {
	return &Scraper{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

// SetCookies sets session cookies to inject into every HTTP request.
func (s *Scraper) SetCookies(cookies []*http.Cookie) { s.cookies = cookies }

// ID returns the supermarket identifier.
func (s *Scraper) ID() datasource.SupermarketID { return s.cfg.ID }

// Name returns the human-readable name.
func (s *Scraper) Name() string { return s.cfg.Name }

// Description returns a short description of the supermarket.
func (s *Scraper) Description() string { return s.cfg.Description }

// CheckSession validates whether cached cookies represent a valid session.
func (s *Scraper) CheckSession(ctx context.Context) bool {
	if len(s.cookies) == 0 || s.cfg.SessionCheckURL == "" {
		return true
	}
	body, err := s.fetch(ctx, s.cfg.SessionCheckURL)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return HTMLHasElement(body, s.cfg.SessionCheckQuery)
}

// SearchProducts searches for products matching the query.
func (s *Scraper) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	body, err := s.fetch(ctx, s.cfg.SearchURL(query))
	if err != nil {
		return nil, fmt.Errorf("%s search fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseSearchResults(body, s.cfg)
}

// GetProductDetails fetches details for a specific product.
func (s *Scraper) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := s.fetch(ctx, s.cfg.ProductURL(productID))
	if err != nil {
		return nil, fmt.Errorf("%s product fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := ParseProductPage(body, s.cfg)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = s.cfg.ProductURL(productID)
	return p, nil
}

// BrowseCategories returns the top-level grocery categories.
func (s *Scraper) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := s.fetch(ctx, s.cfg.CategoryURL)
	if err != nil {
		return nil, fmt.Errorf("%s categories fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseCategories(body, s.cfg)
}

func (s *Scraper) fetch(ctx context.Context, targetURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}
	SetBrowserHeaders(req)
	for _, c := range s.cookies {
		req.AddCookie(c)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, targetURL)
	}
	return resp.Body, nil
}

// BrowserScraper implements datasource.Datasource by scraping HTML pages
// rendered in a headless browser. Use this for sites that require JavaScript
// to populate product data.
type BrowserScraper struct {
	cfg          Config
	browser      *Browser
	cookies      []*http.Cookie
	waitSelector string // CSS selector to wait for before capturing HTML
}

// NewBrowserScraper creates a new browser-based HTML scraper datasource.
func NewBrowserScraper(cfg Config, browser *Browser, waitSelector string) *BrowserScraper {
	return &BrowserScraper{
		cfg:          cfg,
		browser:      browser,
		waitSelector: waitSelector,
	}
}

// SetCookies sets session cookies to inject into every browser page load.
func (s *BrowserScraper) SetCookies(cookies []*http.Cookie) { s.cookies = cookies }

// ID returns the supermarket identifier.
func (s *BrowserScraper) ID() datasource.SupermarketID { return s.cfg.ID }

// Name returns the human-readable name.
func (s *BrowserScraper) Name() string { return s.cfg.Name }

// Description returns a short description of the supermarket.
func (s *BrowserScraper) Description() string { return s.cfg.Description }

// CheckSession validates whether cached cookies represent a valid session.
func (s *BrowserScraper) CheckSession(ctx context.Context) bool {
	if len(s.cookies) == 0 || s.cfg.SessionCheckURL == "" {
		return true
	}
	body, err := s.browser.Fetch(ctx, s.cfg.SessionCheckURL, s.cookies)
	if err != nil {
		return false
	}
	defer body.Close() //nolint:errcheck // Best-effort close.
	return HTMLHasElement(body, s.cfg.SessionCheckQuery)
}

// SearchProducts searches for products matching the query.
func (s *BrowserScraper) SearchProducts(ctx context.Context, query string) ([]datasource.Product, error) {
	body, err := s.browser.Fetch(ctx, s.cfg.SearchURL(query), s.cookies, s.waitSelector)
	if err != nil {
		return nil, fmt.Errorf("%s search fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseSearchResults(body, s.cfg)
}

// GetProductDetails fetches details for a specific product.
func (s *BrowserScraper) GetProductDetails(ctx context.Context, productID string) (*datasource.Product, error) {
	body, err := s.browser.Fetch(ctx, s.cfg.ProductURL(productID), s.cookies, s.waitSelector)
	if err != nil {
		return nil, fmt.Errorf("%s product fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	p, err := ParseProductPage(body, s.cfg)
	if err != nil {
		return nil, err
	}
	p.ID = productID
	p.URL = s.cfg.ProductURL(productID)
	return p, nil
}

// BrowseCategories returns the top-level grocery categories.
func (s *BrowserScraper) BrowseCategories(ctx context.Context) ([]datasource.Category, error) {
	body, err := s.browser.Fetch(ctx, s.cfg.CategoryURL, s.cookies, s.waitSelector)
	if err != nil {
		return nil, fmt.Errorf("%s categories fetch: %w", s.cfg.ID, err)
	}
	defer body.Close() //nolint:errcheck // Best-effort close.

	return ParseCategories(body, s.cfg)
}

// FetchPage fetches a URL via the browser with session cookies and returns
// the rendered HTML. Optional wait selectors are passed through to the browser.
func (s *BrowserScraper) FetchPage(
	ctx context.Context, targetURL string, waitSelector ...string,
) (io.ReadCloser, error) {
	args := append([]string(nil), waitSelector...)
	return s.browser.Fetch(ctx, targetURL, s.cookies, args...)
}

// Shared parsing logic used by both Scraper and BrowserScraper.

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

// ParseProductPage parses a product detail page into a Product.
func ParseProductPage(r io.Reader, cfg Config) (*datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%s: parse product HTML: %w", cfg.ID, err)
	}

	p := &datasource.Product{
		Supermarket: cfg.ID,
		Currency:    "GBP",
		Available:   true,
	}
	matchers := pageMatchers(cfg.ProductSel)

	WalkTree(doc, func(n *html.Node) {
		if n.Type != html.ElementNode {
			return
		}
		applyMatchers(n, p, matchers)
	})

	return p, nil
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
		s = strings.TrimPrefix(s, "$")
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
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
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
