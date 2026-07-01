package illustrations

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

const (
	knownCollection = "open-doodles"
	knownStem       = "ballet-doodle"
)

func TestGetKnownFile(t *testing.T) {
	data, err := Get(knownCollection, knownStem)
	if err != nil {
		t.Fatalf("Get(%q, %q) returned unexpected error: %v", knownCollection, knownStem, err)
	}
	if !bytes.Contains(data, []byte("<svg")) {
		t.Fatalf("Get(%q, %q) bytes do not contain <svg", knownCollection, knownStem)
	}
}

func TestSearchFindsKnownFile(t *testing.T) {
	results := Search("ballet", knownCollection, 0)
	if len(results) == 0 {
		t.Fatalf("Search(%q, %q, 0) returned no results", "ballet", knownCollection)
	}
	found := false
	for _, m := range results {
		if m.Collection == knownCollection && m.Name == knownStem {
			found = true
		}
	}
	if !found {
		t.Fatalf("Search(%q, %q, 0) = %+v, want a result for %q", "ballet", knownCollection, results, knownStem)
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
			_, err := Get(tt.collection, tt.illust)
			if !errors.Is(err, ErrNotFound) {
				t.Fatalf("Get(%q, %q) error = %v, want ErrNotFound", tt.collection, tt.illust, err)
			}
		})
	}
}

func TestCollections(t *testing.T) {
	want := []string{"humaaans", "open-doodles", "open-peeps"}
	got := Collections()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collections() = %v, want %v", got, want)
	}
}
