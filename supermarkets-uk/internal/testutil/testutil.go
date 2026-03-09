// Package testutil provides shared test helpers for datasource tests.
package testutil

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// OpenTestFile opens a file and registers cleanup to close it.
func OpenTestFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path) //nolint:gosec // Test fixture paths are not user-controlled.
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// ParseSearchFile opens a fixture file and parses it as search results.
func ParseSearchFile(
	t *testing.T,
	path string,
	fn func(io.Reader) ([]datasource.Product, error),
) []datasource.Product {
	t.Helper()
	f := OpenTestFile(t, path)
	products, err := fn(f)
	require.NoError(t, err)
	return products
}

// ParseProductFile opens a fixture file and parses it as a product page.
func ParseProductFile(
	t *testing.T,
	path string,
	fn func(io.Reader) (*datasource.Product, error),
) *datasource.Product {
	t.Helper()
	f := OpenTestFile(t, path)
	p, err := fn(f)
	require.NoError(t, err)
	return p
}

// ParseCategoryFile opens a fixture file and parses it as categories.
func ParseCategoryFile(
	t *testing.T,
	path string,
	fn func(io.Reader) ([]datasource.Category, error),
) []datasource.Category {
	t.Helper()
	f := OpenTestFile(t, path)
	categories, err := fn(f)
	require.NoError(t, err)
	return categories
}

// AssertSearchResults validates that search results are non-empty and
// a reasonable fraction contain the query in their name.
func AssertSearchResults(t *testing.T, products []datasource.Product, query string) {
	t.Helper()
	require.NotEmpty(t, products, "expected results")
	relevant := 0
	for _, p := range products {
		assert.NotEmpty(t, p.Name, "empty product name")
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			relevant++
		}
	}
	minRelevant := len(products) / 4
	if minRelevant < 1 {
		minRelevant = 1
	}
	if relevant < minRelevant {
		t.Errorf("only %d/%d results contain %q in their name (want at least %d)",
			relevant, len(products), query, minRelevant)
		for i, p := range products {
			if i >= 5 {
				break
			}
			t.Logf("  [%d] %s", i, p.Name)
		}
	}
}

// HTMLFixtureServer creates an httptest.Server that serves the given HTML fixture file.
func HTMLFixtureServer(t *testing.T, fixturePath string) *httptest.Server {
	t.Helper()
	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // Test fixture path.
	require.NoError(t, err)
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(fixture)
		}),
	)
	t.Cleanup(srv.Close)
	return srv
}

// JSONFixtureServer creates an httptest.Server that serves the given JSON fixture file.
func JSONFixtureServer(t *testing.T, fixturePath string) *httptest.Server {
	t.Helper()
	fixture, err := os.ReadFile(fixturePath) //nolint:gosec // Test fixture path.
	require.NoError(t, err)
	srv := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(fixture)
		}),
	)
	t.Cleanup(srv.Close)
	return srv
}
