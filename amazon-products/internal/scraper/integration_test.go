package scraper_test

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/amazon-products/internal/scraper"
)

func TestSearchByRegionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	browser := scraper.NewBrowser()
	defer browser.Close()

	// Sort region IDs for deterministic test order.
	var ids []string
	for id := range scraper.Regions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		region := scraper.Regions[id]
		t.Run(id, func(t *testing.T) {
			ds := scraper.NewDatasource(browser, region)

			// "Samsung" is a universal brand name that returns results on every Amazon site.
			products, err := ds.SearchProducts(t.Context(), "Samsung")
			require.NoError(t, err)
			require.NotEmpty(t, products, "expected search results for region %s", id)

			for _, p := range products {
				assert.NotEmpty(t, p.ASIN, "missing ASIN")
				assert.NotEmpty(t, p.Name, "missing name")
				assert.NotEmpty(t, p.URL, "missing URL")
				assert.Contains(t, p.URL, region.BaseURL, "URL should use region base URL")
				assert.Equal(t, region.Currency, p.Currency, "currency mismatch")
			}

			var hasPrice bool
			for _, p := range products {
				if p.Price > 0 {
					hasPrice = true
					break
				}
			}
			assert.True(t, hasPrice, "expected at least one product with a price")
		})
	}
}

func TestProductDetailsByRegionIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	browser := scraper.NewBrowser()
	defer browser.Close()

	var ids []string
	for id := range scraper.Regions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		region := scraper.Regions[id]
		t.Run(id, func(t *testing.T) {
			ds := scraper.NewDatasource(browser, region)

			products, err := ds.SearchProducts(t.Context(), "Samsung")
			require.NoError(t, err)
			require.NotEmpty(t, products, "no search results to look up")

			// Prefer products that already showed a price in search results,
			// since some products genuinely have no price on their detail page.
			candidates := prioritisePricedProducts(products)

			var p *scraper.Product
			limit := 3
			if len(candidates) < limit {
				limit = len(candidates)
			}
			for j := range limit {
				p, err = ds.GetProductDetails(t.Context(), candidates[j].ASIN)
				require.NoError(t, err)
				if p.Price > 0 {
					break
				}
			}
			assert.NotEmpty(t, p.Name)
			assert.NotEmpty(t, p.ASIN)
			assert.NotEmpty(t, p.URL)
			assert.Contains(t, p.URL, region.BaseURL)
			assert.Equal(t, region.Currency, p.Currency)
			assert.Positive(t, p.Price)
		})
	}
}

// prioritisePricedProducts reorders products so that those with a price come
// first, preserving relative order within each group.
func prioritisePricedProducts(products []scraper.Product) []scraper.Product {
	out := make([]scraper.Product, 0, len(products))
	var noPriceProducts []scraper.Product
	for _, p := range products {
		if p.Price > 0 {
			out = append(out, p)
		} else {
			noPriceProducts = append(noPriceProducts, p)
		}
	}
	return append(out, noPriceProducts...)
}
