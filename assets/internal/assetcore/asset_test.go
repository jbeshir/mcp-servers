package assetcore

import "testing"

func TestAssetIDRoundTrip(t *testing.T) {
	id := AssetID("embedded-icons", "lucide/camera")
	if id != "embedded-icons:lucide/camera" {
		t.Fatalf("AssetID = %q, want embedded-icons:lucide/camera", id)
	}

	provider, local, ok := ParseAssetID(id)
	if !ok {
		t.Fatal("ParseAssetID ok = false, want true")
	}
	if provider != "embedded-icons" {
		t.Errorf("provider = %q, want embedded-icons", provider)
	}
	if local != "lucide/camera" {
		t.Errorf("local = %q, want lucide/camera", local)
	}
}

func TestParseAssetIDLocalWithColons(t *testing.T) {
	// The local part is opaque and may contain further colons; only the first colon splits.
	provider, local, ok := ParseAssetID("prov:a:b:c")
	if !ok {
		t.Fatal("ParseAssetID ok = false, want true")
	}
	if provider != "prov" {
		t.Errorf("provider = %q, want prov", provider)
	}
	if local != "a:b:c" {
		t.Errorf("local = %q, want a:b:c", local)
	}
}

func TestParseAssetIDMalformed(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"no colon", "nocolon"},
		{"empty", ""},
		{"empty provider", ":local"},
		{"empty local", "provider:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, ok := ParseAssetID(tt.id); ok {
				t.Errorf("ParseAssetID(%q) ok = true, want false", tt.id)
			}
		})
	}
}
