package client_test

import (
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func TestNewClient(t *testing.T) {
	c := client.NewClient(client.Config{})

	infos := c.ListSupermarkets()
	if len(infos) != 9 {
		t.Fatalf("expected 9 supermarkets, got %d", len(infos))
	}

	type expectedInfo struct {
		name        string
		description string
	}
	expected := map[datasource.SupermarketID]expectedInfo{
		datasource.Tesco:      {"Tesco", "The UK's largest supermarket chain"},
		datasource.Sainsburys: {"Sainsbury's", "One of the UK's largest supermarket chains"},
		datasource.Ocado:      {"Ocado", "Online-only UK supermarket and grocery delivery service"},
		datasource.Morrisons:  {"Morrisons", "Major UK supermarket chain"},
		datasource.Asda:       {"Asda", "One of the UK's largest supermarket chains"},
		datasource.Waitrose:   {"Waitrose", "Premium UK supermarket chain"},
		datasource.Hiyou:      {"HiYoU", "Asian supermarket based in Newcastle"},
		datasource.TukTukMart: {"Tuk Tuk Mart", "Manchester-based Asian supermarket (Hang Won Hong's online store)"},
		datasource.Morueats:   {"Morueats", "Asian grocery covering Japanese, Chinese, Korean, and Thai products"},
	}

	for _, info := range infos {
		exp, ok := expected[info.ID]
		if !ok {
			t.Errorf("unexpected supermarket: %s", info.ID)
			continue
		}
		if info.Name != exp.name {
			t.Errorf("supermarket %s: name = %q, want %q", info.ID, info.Name, exp.name)
		}
		if info.Description != exp.description {
			t.Errorf("supermarket %s: description = %q, want %q", info.ID, info.Description, exp.description)
		}
	}
}

func TestParseSupermarketIDs(t *testing.T) {
	tests := []struct {
		input    string
		expected []datasource.SupermarketID
	}{
		{"", nil},
		{"tesco", []datasource.SupermarketID{datasource.Tesco}},
		{
			"tesco,sainsburys",
			[]datasource.SupermarketID{datasource.Tesco, datasource.Sainsburys},
		},
		{
			"tesco, ocado , morrisons",
			[]datasource.SupermarketID{
				datasource.Tesco, datasource.Ocado, datasource.Morrisons,
			},
		},
	}

	for _, tt := range tests {
		ids := client.ParseSupermarketIDs(tt.input)
		if len(ids) != len(tt.expected) {
			t.Errorf(
				"ParseSupermarketIDs(%q): got %d IDs, want %d",
				tt.input, len(ids), len(tt.expected),
			)
			continue
		}
		for i, id := range ids {
			if id != tt.expected[i] {
				t.Errorf(
					"ParseSupermarketIDs(%q)[%d] = %q, want %q",
					tt.input, i, id, tt.expected[i],
				)
			}
		}
	}
}
