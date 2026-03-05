package shopify_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource/shopify"
)

type shopifyTestCase struct {
	name        string
	constructor func() *shopify.Datasource
	id          datasource.SupermarketID
}

var shopifyStores = []shopifyTestCase{
	{"Hiyou", shopify.NewHiyou, datasource.Hiyou},
	{"TukTukMart", shopify.NewTukTukMart, datasource.TukTukMart},
	{"Morueats", shopify.NewMorueats, datasource.Morueats},
}

func TestShopifySearchIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	for _, tc := range shopifyStores {
		t.Run(tc.name, func(t *testing.T) {
			ds := tc.constructor()
			products, err := ds.SearchProducts(context.Background(), "rice")
			if err != nil {
				t.Fatal(err)
			}
			if len(products) == 0 {
				t.Fatal("expected results")
			}
			for _, p := range products {
				if p.Name == "" {
					t.Error("empty product name")
				}
				if p.Supermarket != tc.id {
					t.Errorf("supermarket = %q, want %q", p.Supermarket, tc.id)
				}
				if p.Currency != "GBP" {
					t.Errorf("currency = %q, want GBP", p.Currency)
				}
			}
			// Sanity-check that at least some results are relevant.
			relevant := 0
			for _, p := range products {
				if strings.Contains(strings.ToLower(p.Name), "rice") {
					relevant++
				}
			}
			if relevant == 0 {
				t.Error("no results contain 'rice' in their name")
			}
		})
	}
}

func TestShopifyProductDetailsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	for _, tc := range shopifyStores {
		t.Run(tc.name, func(t *testing.T) {
			ds := tc.constructor()

			// First search to get a valid handle.
			products, err := ds.SearchProducts(context.Background(), "rice")
			if err != nil {
				t.Fatal(err)
			}
			if len(products) == 0 {
				t.Fatal("no search results to look up")
			}

			p, err := ds.GetProductDetails(context.Background(), products[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if p.Name == "" {
				t.Error("empty product name")
			}
			if p.Price <= 0 {
				t.Errorf("expected positive price, got %f", p.Price)
			}
			if p.URL == "" {
				t.Error("empty product URL")
			}
		})
	}
}

func TestShopifyBrowseCategoriesIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	for _, tc := range shopifyStores {
		t.Run(tc.name, func(t *testing.T) {
			ds := tc.constructor()
			categories, err := ds.BrowseCategories(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(categories) == 0 {
				t.Fatal("expected categories")
			}
			for _, c := range categories {
				if c.Name == "" {
					t.Error("empty category name")
				}
				if c.Supermarket != tc.id {
					t.Errorf("supermarket = %q, want %q",
						c.Supermarket, tc.id)
				}
				if c.URL == "" {
					t.Error("empty category URL")
				}
			}
		})
	}
}
