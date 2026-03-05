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

// matches returns true if the node matches this selector.
func (s ElemSel) matches(n *html.Node) bool {
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
		} else if getAttr(n, s.Att) == "" {
			return false
		}
	}
	return true
}

// Config defines all per-supermarket configuration for an HTML scraper.
type Config struct {
	ID          datasource.SupermarketID
	Name        string
	BaseURL     string
	SearchURL   func(query string) string
	ProductURL  func(id string) string
	CategoryURL string
	Container   ElemSel
	SearchSel   ProductSelectors
	ProductSel  ProductPageSelectors
	CategorySel ElemSel
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

// ProductPageSelectors configures selectors for parsing product detail pages.
type ProductPageSelectors struct {
	Title  ElemSel
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

	return ParseProductPage(body, s.cfg)
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

	return ParseProductPage(body, s.cfg)
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

// Shared parsing logic used by both Scraper and BrowserScraper.

// ParseSearchResults parses search result HTML into a list of products.
func ParseSearchResults(r io.Reader, cfg Config) ([]datasource.Product, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("%s: parse HTML: %w", cfg.ID, err)
	}

	var products []datasource.Product
	walkTree(doc, func(n *html.Node) {
		if !cfg.Container.matches(n) {
			return
		}
		if p, ok := extractProduct(n, cfg.SearchSel, cfg.BaseURL, cfg.ID); ok {
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

	walkTree(doc, func(n *html.Node) {
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
	walkTree(doc, func(n *html.Node) {
		if !cfg.CategorySel.matches(n) {
			return
		}
		catName := textContent(n)
		href := getAttr(n, "href")
		if catName == "" || href == "" {
			return
		}
		categories = append(categories, datasource.Category{
			ID:          lastPathSegment(href),
			Name:        catName,
			URL:         resolveURL(cfg.BaseURL, href),
			Supermarket: cfg.ID,
		})
	})

	return categories, nil
}

// extractProduct builds a Product from a search result container node.
func extractProduct(
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

	walkTree(n, func(node *html.Node) {
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

// fieldMatcher describes how to match and extract a single product field.
type fieldMatcher struct {
	sel   ElemSel
	apply func(*html.Node, *datasource.Product)
}

func searchMatchers(sel ProductSelectors, baseURL string) []fieldMatcher {
	m := []fieldMatcher{
		{sel.Title, func(n *html.Node, p *datasource.Product) {
			p.Name = textContent(n)
			if n.Data == "a" {
				if href := getAttr(n, "href"); href != "" {
					p.URL = resolveURL(baseURL, href)
					p.ID = lastPathSegment(href)
				}
			}
		}},
		{sel.Price, func(n *html.Node, p *datasource.Product) {
			p.Price = parsePrice(textContent(n))
		}},
		{sel.Unit, func(n *html.Node, p *datasource.Product) {
			p.PricePerUnit = textContent(n)
		}},
		{sel.Image, func(n *html.Node, p *datasource.Product) {
			p.ImageURL = getAttr(n, "src")
		}},
	}
	if sel.Link != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Link, func(n *html.Node, p *datasource.Product) {
			if href := getAttr(n, "href"); href != "" {
				p.URL = resolveURL(baseURL, href)
				p.ID = lastPathSegment(href)
			}
		}})
	}
	if sel.Promo != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Promo, func(n *html.Node, p *datasource.Product) {
			p.Promotion = textContent(n)
		}})
	}
	if sel.Weight != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Weight, func(n *html.Node, p *datasource.Product) {
			p.Weight = textContent(n)
		}})
	}
	return m
}

func pageMatchers(sel ProductPageSelectors) []fieldMatcher {
	m := []fieldMatcher{
		{sel.Title, func(n *html.Node, p *datasource.Product) {
			p.Name = textContent(n)
		}},
		{sel.Price, func(n *html.Node, p *datasource.Product) {
			p.Price = parsePrice(textContent(n))
		}},
		{sel.Unit, func(n *html.Node, p *datasource.Product) {
			p.PricePerUnit = textContent(n)
		}},
		{sel.Image, func(n *html.Node, p *datasource.Product) {
			p.ImageURL = getAttr(n, "src")
		}},
	}
	if sel.Promo != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Promo, func(n *html.Node, p *datasource.Product) {
			p.Promotion = textContent(n)
		}})
	}
	if sel.Weight != (ElemSel{}) {
		m = append(m, fieldMatcher{sel.Weight, func(n *html.Node, p *datasource.Product) {
			p.Weight = textContent(n)
		}})
	}
	return m
}

func applyMatchers(node *html.Node, p *datasource.Product, matchers []fieldMatcher) {
	for _, fm := range matchers {
		if fm.sel.matches(node) {
			fm.apply(node, p)
			return
		}
	}
}

// walkTree visits every node in the HTML tree, calling fn for each.
func walkTree(n *html.Node, fn func(*html.Node)) {
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walkTree(c, fn)
	}
}

// parsePrice extracts a float64 price from a string like "£1.50".
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "£")
	s = strings.TrimPrefix(s, "$")
	s = strings.ReplaceAll(s, ",", "")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func lastPathSegment(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

// resolveURL resolves a potentially relative URL against a base URL.
func resolveURL(baseURL, href string) string {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	return baseURL + href
}

// HTML utility functions.

func getAttr(n *html.Node, key string) string {
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

func textContent(n *html.Node) string {
	var sb strings.Builder
	walkTree(n, func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(sb.String())
}

// SetBrowserHeaders sets standard browser-like headers on an HTTP request.
func SetBrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "+
			"AppleWebKit/537.36 (KHTML, like Gecko) "+
			"Chrome/120.0.0.0 Safari/537.36")
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
