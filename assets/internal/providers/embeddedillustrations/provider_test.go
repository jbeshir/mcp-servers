package embeddedillustrations

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
)

const (
	knownCollection = "open-doodles"
	knownStem       = "ballet-doodle"
)

func TestGetKnownFile(t *testing.T) {
	data, err := getIllustration(knownCollection, knownStem)
	if err != nil {
		t.Fatalf("getIllustration(%q, %q) returned unexpected error: %v", knownCollection, knownStem, err)
	}
	if !bytes.Contains(data, []byte("<svg")) {
		t.Fatalf("getIllustration(%q, %q) bytes do not contain <svg", knownCollection, knownStem)
	}
}

func TestSearchFindsKnownFile(t *testing.T) {
	results := searchIllustrations("ballet", assetcore.Filter{Only: []string{knownCollection}}, 50)
	if len(results) == 0 {
		t.Fatalf("searchIllustrations(%q, only %q) returned no results", "ballet", knownCollection)
	}
	found := false
	for _, m := range results {
		if m.collection == knownCollection && m.name == knownStem {
			found = true
		}
	}
	if !found {
		t.Fatalf("searchIllustrations(%q, only %q) = %+v, want a result for %q", "ballet", knownCollection, results, knownStem)
	}
}

func TestSearchExcludeCollection(t *testing.T) {
	// "doodle" matches many open-doodles stems; excluding the collection must drop them all.
	for _, m := range searchIllustrations("doodle", assetcore.Filter{Except: []string{knownCollection}}, 200) {
		if m.collection == knownCollection {
			t.Errorf("searchIllustrations with %q excluded still returned a hit from it: %+v", knownCollection, m)
		}
	}
}

func TestGetNotFound(t *testing.T) {
	tests := []struct {
		name       string
		collection string
		illust     string
	}{
		{name: "unknown illustration name", collection: knownCollection, illust: "definitely-not-real"},
		{name: "unknown collection", collection: "no-such-collection", illust: knownStem},
		{name: "path traversal attempt", collection: knownCollection, illust: "../secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getIllustration(tt.collection, tt.illust)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("getIllustration(%q, %q) error = %v, want ErrNotFound", tt.collection, tt.illust, err)
			}
		})
	}
}

func TestFetchPathTraversalGuard(t *testing.T) {
	p := New()

	// A composite-local id whose name half attempts traversal must be rejected as ErrNotFound.
	_, err := p.Fetch(t.Context(), knownCollection+"/..")
	if !errors.Is(err, assetcore.ErrNotFound) {
		t.Errorf("Fetch traversal attempt error = %v, want assetcore.ErrNotFound", err)
	}
}

func TestFetchMapsAssetAndLicense(t *testing.T) {
	p := New()

	blob, err := p.Fetch(t.Context(), knownCollection+"/"+knownStem)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !bytes.Contains(blob.Content, []byte("<svg")) {
		t.Error("Fetch content does not contain <svg")
	}
	if blob.Asset.ID != assetcore.AssetID(providerName, knownCollection+"/"+knownStem) {
		t.Errorf("blob.Asset.ID = %q, want composite id", blob.Asset.ID)
	}
	if blob.Asset.License.SPDX != "CC0-1.0" {
		t.Errorf("blob.Asset.License.SPDX = %q, want CC0-1.0", blob.Asset.License.SPDX)
	}
}

func TestCollections(t *testing.T) {
	want := []string{"humaaans", "open-doodles", "open-peeps"}
	got := loadedCollections()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadedCollections() = %v, want %v", got, want)
	}
}

func TestSourcesReportsCollectionsWithLicenseAndCount(t *testing.T) {
	p := New()

	srcs := p.Sources()
	want := []string{"humaaans", "open-doodles", "open-peeps"}
	if len(srcs) != len(want) {
		t.Fatalf("Sources() length = %d, want %d", len(srcs), len(want))
	}
	for i, name := range want {
		if srcs[i].Name != name {
			t.Errorf("Sources()[%d].Name = %q, want %q", i, srcs[i].Name, name)
		}
		if srcs[i].License.SPDX != "CC0-1.0" {
			t.Errorf("Sources()[%d].License = %q, want CC0-1.0", i, srcs[i].License.SPDX)
		}
		if srcs[i].Count <= 0 {
			t.Errorf("Sources()[%d].Count = %d, want a positive count", i, srcs[i].Count)
		}
	}

	if !strings.EqualFold(srcs[0].Name, "humaaans") {
		t.Errorf("Sources() not sorted: first = %q", srcs[0].Name)
	}
}
