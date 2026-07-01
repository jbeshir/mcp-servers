// Package catalog loads the embedded metadata describing every vendored asset source:
// icon sets, illustration collections, and font families, along with their licenses.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed catalog.json
var catalogJSON []byte

// FontFile describes a single font file variant within a font family.
type FontFile struct {
	File   string `json:"file"`
	Weight int    `json:"weight"`
	Style  string `json:"style"`
	Format string `json:"format"`
}

// IconSource describes a vendored Iconify icon set.
type IconSource struct {
	Set         string `json:"set"`
	License     string `json:"license"`
	Attribution string `json:"attribution"`
	SourceURL   string `json:"source_url"`
	Count       int    `json:"count"`
	Grid        int    `json:"grid"`
	Note        string `json:"note"`
}

// IllustrationSource describes a vendored illustration collection.
type IllustrationSource struct {
	Collection  string `json:"collection"`
	License     string `json:"license"`
	Attribution string `json:"attribution"`
	SourceURL   string `json:"source_url"`
	Count       int    `json:"count"`
	Note        string `json:"note"`
}

// FontSource describes a vendored font family.
type FontSource struct {
	Family      string     `json:"family"`
	Slug        string     `json:"slug"`
	License     string     `json:"license"`
	Category    string     `json:"category"`
	Attribution string     `json:"attribution"`
	SourceURL   string     `json:"source_url"`
	Files       []FontFile `json:"files"`
	Note        string     `json:"note"`
}

// Catalog is the machine-readable metadata for every vendored asset source.
type Catalog struct {
	Icons         []IconSource         `json:"icons"`
	Illustrations []IllustrationSource `json:"illustrations"`
	Fonts         []FontSource         `json:"fonts"`
}

// Load parses the embedded catalog.json into a Catalog.
func Load() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(catalogJSON, &c); err != nil {
		return nil, fmt.Errorf("unmarshal catalog.json: %w", err)
	}

	return &c, nil
}

// IconLicense returns the license and attribution for the given icon set (case-insensitive).
func (c *Catalog) IconLicense(set string) (license, attribution string, ok bool) {
	for _, s := range c.Icons {
		if strings.EqualFold(s.Set, set) {
			return s.License, s.Attribution, true
		}
	}

	return "", "", false
}

// IllustrationLicense returns the license and attribution for the given illustration collection
// (case-insensitive).
func (c *Catalog) IllustrationLicense(coll string) (license, attribution string, ok bool) {
	for _, s := range c.Illustrations {
		if strings.EqualFold(s.Collection, coll) {
			return s.License, s.Attribution, true
		}
	}

	return "", "", false
}

// FontLicense returns the license and attribution for the given font family, matching on either
// the family display name or its slug (case-insensitive).
func (c *Catalog) FontLicense(family string) (license, attribution string, ok bool) {
	for _, s := range c.Fonts {
		if strings.EqualFold(s.Family, family) || strings.EqualFold(s.Slug, family) {
			return s.License, s.Attribution, true
		}
	}

	return "", "", false
}
