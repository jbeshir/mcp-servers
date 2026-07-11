package assetcore

import "testing"

func TestFilterAllows(t *testing.T) {
	tests := []struct {
		name   string
		filter Filter
		input  string
		want   bool
	}{
		{"empty allows all", Filter{}, "anything", true},
		{"only match", Filter{Only: []string{"a", "b"}}, "b", true},
		{"only miss", Filter{Only: []string{"a", "b"}}, "c", false},
		{"except blocks", Filter{Except: []string{"x"}}, "x", false},
		{"except passes others", Filter{Except: []string{"x"}}, "y", true},
		{"only and except, allowed", Filter{Only: []string{"a", "b"}, Except: []string{"b"}}, "a", true},
		{"except overrides only", Filter{Only: []string{"a", "b"}, Except: []string{"b"}}, "b", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Allows(tt.input); got != tt.want {
				t.Errorf("Allows(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		limit int
		want  int
	}{
		{0, 50},
		{-5, 50},
		{500, 200},
		{100, 100},
		{200, 200},
		{1, 1},
	}

	for _, tt := range tests {
		if got := ClampLimit(tt.limit); got != tt.want {
			t.Errorf("ClampLimit(%d) = %d, want %d", tt.limit, got, tt.want)
		}
	}
}
