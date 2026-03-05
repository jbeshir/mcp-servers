package client_test

import (
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func TestNewClient(t *testing.T) {
	c := client.NewClient(client.Config{})

	infos := c.ListSupermarkets()
	if len(infos) != 4 {
		t.Fatalf("expected 4 supermarkets, got %d", len(infos))
	}

	expected := map[datasource.SupermarketID]string{
		datasource.Tesco:      "Tesco",
		datasource.Sainsburys: "Sainsbury's",
		datasource.Ocado:      "Ocado",
		datasource.Morrisons:  "Morrisons",
	}

	for _, info := range infos {
		name, ok := expected[info.ID]
		if !ok {
			t.Errorf("unexpected supermarket: %s", info.ID)
			continue
		}
		if info.Name != name {
			t.Errorf("supermarket %s: name = %q, want %q", info.ID, info.Name, name)
		}
	}
}

func TestGetDatasource(t *testing.T) {
	c := client.NewClient(client.Config{})

	ds, ok := c.GetDatasource(datasource.Tesco)
	if !ok {
		t.Fatal("expected to find Tesco datasource")
	}
	if ds.ID() != datasource.Tesco {
		t.Errorf("id = %q, want tesco", ds.ID())
	}

	_, ok = c.GetDatasource("nonexistent")
	if ok {
		t.Error("expected nonexistent datasource to not be found")
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
