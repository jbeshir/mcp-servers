package scraper

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ukRegion = Regions["uk"]

func TestParseSearchResults(t *testing.T) {
	f, err := os.Open("testdata/amazon_search.html")
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	products, err := ParseSearchResults(f, ukRegion)
	require.NoError(t, err)

	// Should skip the empty-ASIN sponsored result
	require.Len(t, products, 2)

	p := products[0]
	assert.Equal(t, "B09CDRVQZC", p.ASIN)
	assert.Equal(t, "Sony WH-1000XM4 Wireless Noise Cancelling Headphones", p.Name)
	assert.Equal(t, 248.0, p.Price)
	assert.Equal(t, "GBP", p.Currency)
	assert.Equal(t, ukRegion.BaseURL+"/dp/B09CDRVQZC", p.URL)
	assert.Contains(t, p.ImageURL, "51aXvjzcukL")
	assert.Equal(t, "4.6 out of 5 stars", p.Rating)
	assert.True(t, p.IsPrime)

	p2 := products[1]
	assert.Equal(t, "B0BXL6ZZWB", p2.ASIN)
	assert.Equal(t, "JBL Tune 510BT Wireless On-Ear Headphones", p2.Name)
	assert.Equal(t, 29.99, p2.Price)
	assert.False(t, p2.IsPrime)
}

func TestParseProductPage(t *testing.T) {
	f, err := os.Open("testdata/amazon_product.html")
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	p, err := ParseProductPage(f, "B09CDRVQZC", ukRegion)
	require.NoError(t, err)

	assert.Equal(t, "B09CDRVQZC", p.ASIN)
	assert.Equal(t, "Sony WH-1000XM4 Wireless Premium Noise Cancelling Overhead Headphones", p.Name)
	assert.Equal(t, 248.0, p.Price)
	assert.Equal(t, "GBP", p.Currency)
	assert.Equal(t, ukRegion.BaseURL+"/dp/B09CDRVQZC", p.URL)
	assert.Equal(t, "4.6 out of 5 stars", p.Rating)
	assert.Equal(t, "12,345 ratings", p.ReviewCount)
	assert.Equal(t, "Sony", p.Brand)
	assert.Contains(t, p.ImageURL, "51aXvjzcukL")

	// Features should exclude "About this item"
	require.Len(t, p.Features, 4)
	assert.Contains(t, p.Features[0], "noise cancellation")
	assert.Contains(t, p.Features[1], "battery life")

	assert.Contains(t, p.Description, "Premium noise cancelling headphones")
	assert.Equal(t, "In stock.", p.Availability)
}

func TestParseProductPageVariants(t *testing.T) {
	const page = `<html><body>
	<span id="productTitle">Test Product</span>
	<script type="a-state" data-a-state='{"key":"desktop-twister-sort-filter-data"}'>
	{"sortedDimValuesForAllDims":{
		"size_name":[
			{"dimensionValueDisplayText":"64 GB","dimensionValueState":"SELECTED","defaultAsin":"B001"},
			{"dimensionValueDisplayText":"256 GB","dimensionValueState":"AVAILABLE","defaultAsin":"B002"}
		],
		"color_name":[
			{"dimensionValueDisplayText":"Pink","dimensionValueState":"SELECTED","defaultAsin":"B001"},
			{"dimensionValueDisplayText":"Blue","dimensionValueState":"AVAILABLE","defaultAsin":"B003"},
			{"dimensionValueDisplayText":"Purple","dimensionValueState":"UNAVAILABLE","defaultAsin":"B004"}
		],
		"configuration":[
			{"dimensionValueDisplayText":"Without AppleCare+","dimensionValueState":"SELECTED","defaultAsin":"B001"}
		]
	}}
	</script>
	</body></html>`

	p, err := ParseProductPage(strings.NewReader(page), "B001", ukRegion)
	require.NoError(t, err)

	// "configuration" has only 1 option, so it should be excluded.
	require.Len(t, p.Variants, 2)

	// Sorted alphabetically: color, size.
	assert.Equal(t, "Color", p.Variants[0].Dimension)
	assert.Equal(t, "Pink", p.Variants[0].Selected)
	require.Len(t, p.Variants[0].Options, 3)
	assert.Equal(t, "Pink", p.Variants[0].Options[0].Value)
	assert.Equal(t, "SELECTED", p.Variants[0].Options[0].State)
	assert.Equal(t, "Blue", p.Variants[0].Options[1].Value)
	assert.Equal(t, "B003", p.Variants[0].Options[1].ASIN)
	assert.Equal(t, "AVAILABLE", p.Variants[0].Options[1].State)
	assert.Equal(t, "UNAVAILABLE", p.Variants[0].Options[2].State)

	assert.Equal(t, "Size", p.Variants[1].Dimension)
	assert.Equal(t, "64 GB", p.Variants[1].Selected)
	require.Len(t, p.Variants[1].Options, 2)
}

func TestParseProductPageNoVariants(t *testing.T) {
	f, err := os.Open("testdata/amazon_product.html")
	require.NoError(t, err)
	defer f.Close() //nolint:errcheck

	p, err := ParseProductPage(f, "B09CDRVQZC", ukRegion)
	require.NoError(t, err)
	assert.Empty(t, p.Variants)
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		// GBP
		{"£248.00", 248.0},
		{"£29.99", 29.99},
		{"£1,299.99", 1299.99},
		{"  £10.50  ", 10.50},
		// USD/CAD/AUD/SGD
		{"$29.99", 29.99},
		{"$1,299.99", 1299.99},
		// EUR
		{"29,99 €", 29.99},
		{"€29,99", 29.99},
		{"1.299,99 €", 1299.99},
		// BRL
		{"R$ 29,99", 29.99},
		// JPY (halfwidth and fullwidth yen)
		{"¥2,999", 2999},
		{"￥2,999", 2999},
		// INR
		{"₹2,999.00", 2999.0},
		// TRY
		{"₺299,99", 299.99},
		// PLN
		{"29,99 zł", 29.99},
		// SEK
		{"299,00 kr", 299.0},
		// SAR/AED/EGP (text-based)
		{"SAR 29.99", 29.99},
		{"AED 29.99", 29.99},
		{"EGP 299.99", 299.99},
		// TRY (text)
		{"299,99 TL", 299.99},
		// ISO code format (Amazon geo-converts prices)
		{"GBP\u00a0975.41", 975.41},
		{"GBP\u00a01,125.47", 1125.47},
		{"USD\u00a029.99", 29.99},
		// No currency
		{"no price", 0},
		{"", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parsePrice(tt.input))
		})
	}
}

func TestLooksLikeRating(t *testing.T) {
	// Should match
	assert.True(t, looksLikeRating("4.6 out of 5 stars"))     // English
	assert.True(t, looksLikeRating("4,6 von 5 Sternen"))      // German
	assert.True(t, looksLikeRating("4,6 sur 5 étoiles"))      // French
	assert.True(t, looksLikeRating("4,6 de 5 estrellas"))     // Spanish
	assert.True(t, looksLikeRating("4,6 su 5 stelle"))        // Italian
	assert.True(t, looksLikeRating("5つ星のうち4.6"))              // Japanese
	assert.True(t, looksLikeRating("5 üzerinden 4,6 yıldız")) // Turkish

	// Should not match
	assert.False(t, looksLikeRating(""))
	assert.False(t, looksLikeRating("Free delivery"))
	assert.False(t, looksLikeRating("Prime"))
}
