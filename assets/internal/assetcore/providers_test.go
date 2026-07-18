package assetcore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/jbeshir/mcp-servers/assets/internal/assetcore"
	"github.com/jbeshir/mcp-servers/assets/internal/assetcore/mocks"
)

// newIconProvider returns a mock IconProvider named name, expecting only that its Name is read (every
// registered provider's Name is used as its registry key). Each test adds the Search/Fetch/Kind
// expectations for the interactions it actually drives, so mockery fails the test on any expectation
// that goes unused — and on any call the test did not set up.
func newIconProvider(t *testing.T, name string) *mocks.IconProvider {
	m := mocks.NewIconProvider(t)
	m.EXPECT().Name().Return(name)

	return m
}

func newFontProvider(t *testing.T, name string) *mocks.FontProvider {
	m := mocks.NewFontProvider(t)
	m.EXPECT().Name().Return(name)

	return m
}

func newIllustrationProvider(t *testing.T, name string) *mocks.IllustrationProvider {
	m := mocks.NewIllustrationProvider(t)
	m.EXPECT().Name().Return(name)

	return m
}

func newSpriteProvider(t *testing.T, name string) *mocks.SpriteProvider {
	m := mocks.NewSpriteProvider(t)
	m.EXPECT().Name().Return(name)

	return m
}

// expectIconFetchEcho makes m's Fetch return a Blob whose Asset.ID is the provider-local id it was
// given and whose Content is the provider name, so routing tests can assert both.
func expectIconFetchEcho(m *mocks.IconProvider, name string) {
	m.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, id string, _ assetcore.IconFetchOpts) (assetcore.Blob, error) {
			return assetcore.Blob{Asset: assetcore.Asset{ID: id}, Content: []byte(name)}, nil
		})
}

func expectFontFetchEcho(m *mocks.FontProvider, name string) {
	m.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, id string, _ assetcore.FontFetchOpts) (assetcore.Blob, error) {
			return assetcore.Blob{Asset: assetcore.Asset{ID: id}, Content: []byte(name)}, nil
		})
}

func expectIllustrationFetchEcho(m *mocks.IllustrationProvider, name string) {
	m.EXPECT().Fetch(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, id string) (assetcore.Blob, error) {
			return assetcore.Blob{Asset: assetcore.Asset{ID: id}, Content: []byte(name)}, nil
		})
}

// sourcedIconProvider adds the SourceLister capability to a mock IconProvider.
type sourcedIconProvider struct {
	*mocks.IconProvider
	sources []assetcore.Source
}

func (s sourcedIconProvider) Sources() []assetcore.Source { return s.sources }
