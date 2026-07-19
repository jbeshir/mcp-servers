package assetcore

import "io"

// Pack describes one upstream source archive exposed by the MCP pack tools.
type Pack struct {
	ID, Title string
	Tags      []string
	Count     int
	License   License
	Kinds     map[Kind]int
}

// PackStore supplies pack discovery and raw archive retrieval to the MCP server.
type PackStore interface {
	Packs() []Pack
	OpenPack(string) (io.ReadCloser, Pack, error)
}
