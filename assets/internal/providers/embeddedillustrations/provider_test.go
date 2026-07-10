package embeddedillustrations

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/jbeshir/mcp-servers/assets/internal/catalog"
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
	results := searchIllustrations("ballet", knownCollection, 0)
	if len(results) == 0 {
		t.Fatalf("searchIllustrations(%q, %q, 0) returned no results", "ballet", knownCollection)
	}
	found := false
	for _, m := range results {
		if m.collection == knownCollection && m.name == knownStem {
			found = true
		}
	}
	if !found {
		t.Fatalf("searchIllustrations(%q, %q, 0) = %+v, want a result for %q", "ballet", knownCollection, results, knownStem)
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

func TestCollections(t *testing.T) {
	want := []string{"humaaans", "open-doodles", "open-peeps"}
	got := loadedCollections()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadedCollections() = %v, want %v", got, want)
	}
}

func TestEveryCollectionHasLicense(t *testing.T) {
	c, err := catalog.Load()
	if err != nil {
		t.Fatalf("catalog.Load: %v", err)
	}

	_ = New(c)

	for _, collection := range loadedCollections() {
		license, _, ok := c.IllustrationLicense(collection)
		if !ok {
			t.Errorf("IllustrationLicense(%q) ok = false, want true", collection)
		}
		if license == "" {
			t.Errorf("IllustrationLicense(%q) license is empty", collection)
		}
	}
}
