package auth

import (
	"testing"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

func TestLoadLoginFlags(t *testing.T) {
	t.Setenv("TESCO_LOGIN", "true")
	t.Setenv("OCADO_LOGIN", "1")
	t.Setenv("SAINSBURYS_LOGIN", "yes")
	t.Setenv("MORRISONS_LOGIN", "false")

	flags := LoadLoginFlags()

	if !flags[datasource.Tesco] {
		t.Error("expected Tesco login flag to be set")
	}
	if !flags[datasource.Ocado] {
		t.Error("expected Ocado login flag to be set")
	}
	if !flags[datasource.Sainsburys] {
		t.Error("expected Sainsbury's login flag to be set")
	}
	if flags[datasource.Morrisons] {
		t.Error("expected Morrisons login flag to not be set")
	}
}

func TestLoadLoginFlagsEmpty(t *testing.T) {
	flags := LoadLoginFlags()
	if len(flags) != 0 {
		t.Fatalf("expected 0 flags with no env vars, got %d", len(flags))
	}
}
