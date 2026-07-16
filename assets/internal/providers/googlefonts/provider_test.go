package googlefonts

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/cache"
	"github.com/jbeshir/mcp-servers/assets/internal/httpx"
	"github.com/jbeshir/mcp-servers/assets/internal/ratelimit"
)

const canned = "canned woff2 bytes"

// newTestProvider builds a Provider wired to a fresh cache dir and a permissive rate limiter.
func newTestProvider(t *testing.T) *Provider {
	t.Helper()

	return New(httpx.New(httpx.Config{}), ratelimit.New(1000, 10), cache.New(t.TempDir()))
}

func TestFamiliesIndexParsed(t *testing.T) {
	newTestProvider(t)

	if len(families) == 0 {
		t.Fatal("families index is empty, want it to have parsed the embedded data")
	}
}

// newCSS2Stub serves a css2-style response referencing the given woff2 path on the same server, and a
// woff2 endpoint returning canned bytes. It records the User-Agent and query seen on each css2 request
// and counts woff2 downloads.
func newCSS2Stub(t *testing.T, gotUA *string, gotQuery *url.Values, woff2Requests *int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/css2", func(w http.ResponseWriter, r *http.Request) {
		*gotUA = r.Header.Get("User-Agent")
		*gotQuery = r.URL.Query()

		w.Header().Set("Content-Type", "text/css")
		_, _ = fmt.Fprintf(w, "/* comment */\n@font-face {\n  src: url(%s/font.woff2) format('woff2');\n}\n",
			"http://"+r.Host)
	})
	mux.HandleFunc("/font.woff2", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(woff2Requests, 1)
		_, _ = w.Write([]byte(canned))
	})

	return httptest.NewServer(mux)
}

func withCSS2Base(t *testing.T, base string) {
	t.Helper()

	orig := css2BaseURL
	css2BaseURL = base + "/css2"
	t.Cleanup(func() { css2BaseURL = orig })
}

func TestSearchFiltersBySourcesAndClampsLimit(t *testing.T) {
	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Query: "sans", Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Assets) != 2 {
		t.Fatalf("len(Assets) = %d, want 2 (ClampLimit)", len(res.Assets))
	}

	res, err = p.Search(t.Context(), assetcore.SearchOpts{
		Query:   "",
		Sources: assetcore.Filter{Only: []string{"Roboto"}},
	})
	if err != nil {
		t.Fatalf("Search with Sources filter: %v", err)
	}
	if len(res.Assets) != 1 || res.Assets[0].Title != "Roboto" {
		t.Fatalf("Search with Sources filter = %+v, want exactly [Roboto]", res.Assets)
	}
	if res.Assets[0].Meta[assetcore.MetaCategory] == "" {
		t.Fatal("Meta[category] is empty, want the family's category")
	}
	if res.NextCursor != "" {
		t.Fatalf("NextCursor = %q, want \"\"", res.NextCursor)
	}
}

func TestSearchMatchesCategory(t *testing.T) {
	p := newTestProvider(t)

	res, err := p.Search(t.Context(), assetcore.SearchOpts{Query: "monospace"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res.Assets) == 0 {
		t.Fatal("Search(monospace) returned no assets, want at least one monospace family")
	}
}

func TestFetchDownloadsAndCaches(t *testing.T) {
	var gotUA string
	var gotQuery url.Values
	var woff2Requests int32

	srv := newCSS2Stub(t, &gotUA, &gotQuery, &woff2Requests)
	defer srv.Close()
	withCSS2Base(t, srv.URL)

	p := newTestProvider(t)

	blob, err := p.Fetch(t.Context(), "roboto", assetcore.FontFetchOpts{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(blob.Content) != canned {
		t.Fatalf("Content = %q, want %q", blob.Content, canned)
	}
	if blob.ContentType != "font/woff2" {
		t.Fatalf("ContentType = %q, want font/woff2", blob.ContentType)
	}
	if blob.Asset.Title != "Roboto" {
		t.Fatalf("Asset.Title = %q, want Roboto", blob.Asset.Title)
	}
	if blob.Asset.ID != "googlefonts:roboto" {
		t.Fatalf("Asset.ID = %q, want googlefonts:roboto", blob.Asset.ID)
	}

	if gotUA != browserUserAgent {
		t.Fatalf("css2 request User-Agent = %q, want browser UA %q", gotUA, browserUserAgent)
	}
	if got := gotQuery.Get("family"); got != "Roboto:ital,wght@0,400" {
		t.Fatalf("family query = %q, want Roboto:ital,wght@0,400", got)
	}

	if atomic.LoadInt32(&woff2Requests) != 1 {
		t.Fatalf("woff2Requests = %d, want 1 after first fetch", woff2Requests)
	}

	// A second Fetch for the same slug/weight/style must hit the cache, not refetch.
	if _, err := p.Fetch(t.Context(), "roboto", assetcore.FontFetchOpts{}); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if atomic.LoadInt32(&woff2Requests) != 1 {
		t.Fatalf("woff2Requests after cache hit = %d, want still 1", woff2Requests)
	}
}

func TestFetchItalicSendsItalParam(t *testing.T) {
	var gotUA string
	var gotQuery url.Values
	var woff2Requests int32

	srv := newCSS2Stub(t, &gotUA, &gotQuery, &woff2Requests)
	defer srv.Close()
	withCSS2Base(t, srv.URL)

	p := newTestProvider(t)

	blob, err := p.Fetch(t.Context(), "roboto", assetcore.FontFetchOpts{Weight: 700, Style: "italic"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if got := gotQuery.Get("family"); got != "Roboto:ital,wght@1,700" {
		t.Fatalf("family query = %q, want Roboto:ital,wght@1,700", got)
	}

	weight, style := parseFontFilename(blob.Filename)
	if weight != 700 || style != "italic" {
		t.Fatalf("parseFontFilename(%q) = (%d, %q), want (700, italic)", blob.Filename, weight, style)
	}
}

func TestFetchUnknownFamilyReturnsErrNotFound(t *testing.T) {
	p := newTestProvider(t)

	_, err := p.Fetch(t.Context(), "not-a-real-family", assetcore.FontFetchOpts{})
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Fatalf("Fetch(unknown) error = %v, want assetcore.ErrNotFound", err)
	}
}

func TestRenderFontFaceReferencesLocalFilenameAndVariant(t *testing.T) {
	b := assetcore.Blob{Filename: fontFilename("open-sans", 600, "italic")}

	css := (&Provider{}).RenderFontFace("Open Sans", b)

	wants := []string{
		`font-family: "Open Sans"`,
		`font-style: italic`,
		`font-weight: 600`,
		`url("open-sans-600-italic.woff2") format("woff2")`,
	}
	for _, want := range wants {
		if !strings.Contains(css, want) {
			t.Fatalf("RenderFontFace output missing %q:\n%s", want, css)
		}
	}
}

func TestSourcesListsIndexedFamilies(t *testing.T) {
	p := newTestProvider(t)

	sources := p.Sources()
	if len(sources) != len(families) {
		t.Fatalf("len(Sources()) = %d, want %d", len(sources), len(families))
	}

	for _, s := range sources {
		if s.Count != -1 {
			t.Fatalf("Source(%s).Count = %d, want -1", s.Name, s.Count)
		}
		if s.License.SPDX == "" {
			t.Fatalf("Source(%s).License.SPDX is empty", s.Name)
		}
		if s.Meta[assetcore.MetaCategory] == "" {
			t.Fatalf("Source(%s).Meta[category] is empty", s.Name)
		}
	}
}
