package service

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/generation"
)

type fakeDiscord struct {
	user    discord.User
	emojis  map[string]discord.Emoji
	deleted []string
	created discord.Emoji
}

func (f *fakeDiscord) CurrentUser(context.Context) (discord.User, error) { return f.user, nil }
func (f *fakeDiscord) ListGuildEmojis(context.Context, string) ([]discord.Emoji, error) {
	output := make([]discord.Emoji, 0, len(f.emojis))
	for _, emoji := range f.emojis {
		output = append(output, emoji)
	}
	return output, nil
}
func (f *fakeDiscord) GetGuildEmoji(_ context.Context, _, id string) (discord.Emoji, error) {
	emoji, ok := f.emojis[id]
	if !ok {
		return discord.Emoji{}, discord.ErrNotFound
	}
	return emoji, nil
}
func (f *fakeDiscord) CreateGuildEmoji(
	_ context.Context, _ string, _ string, _ []byte, _ []string, _ string,
) (discord.Emoji, error) {
	f.emojis[f.created.ID] = f.created
	return f.created, nil
}
func (f *fakeDiscord) DeleteGuildEmoji(_ context.Context, _, id, _ string) error {
	f.deleted = append(f.deleted, id)
	delete(f.emojis, id)
	return nil
}
func (f *fakeDiscord) FetchEmojiImage(context.Context, discord.Emoji) ([]byte, error) {
	return testPNG(), nil
}

type fakeGenerator struct{ request generation.Request }

func (f *fakeGenerator) Generate(_ context.Context, request generation.Request) ([]byte, error) {
	f.request = request
	return testPNG(), nil
}

func TestCreateEmojiUsesGuildReference(t *testing.T) {
	discordClient := newFakeDiscord()
	discordClient.emojis["200"] = discord.Emoji{
		ID: "200", Name: "reference", Available: true,
	}
	generator := &fakeGenerator{}
	emojiService := newTestService(t, discordClient, generator)

	result, err := emojiService.CreateEmoji(context.Background(), CreateRequest{
		Name: "new_emoji", Prompt: "A cheerful blob", ReferenceIDs: []string{"200"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Mention != "<:new_emoji:300>" {
		t.Fatalf("unexpected mention: %s", result.Mention)
	}
	if len(generator.request.References) != 1 {
		t.Fatalf("expected one reference, got %d", len(generator.request.References))
	}
}

func TestRemoveRefusesEmojiCreatedByAnotherUser(t *testing.T) {
	discordClient := newFakeDiscord()
	discordClient.emojis["900"] = discord.Emoji{
		ID: "900", Name: "someone_elses", User: &discord.User{ID: "20"},
	}
	emojiService := newTestService(t, discordClient, &fakeGenerator{})

	err := emojiService.RemoveEmoji(context.Background(), "900")
	if err == nil || len(discordClient.deleted) != 0 {
		t.Fatalf("expected refusal without delete, error=%v deleted=%v", err, discordClient.deleted)
	}
}

func TestRemoveRefusesEmojiWithoutCreator(t *testing.T) {
	discordClient := newFakeDiscord()
	discordClient.emojis["900"] = discord.Emoji{
		ID: "900", Name: "missing_creator",
	}
	emojiService := newTestService(t, discordClient, &fakeGenerator{})

	err := emojiService.RemoveEmoji(context.Background(), "900")
	if err == nil || len(discordClient.deleted) != 0 {
		t.Fatalf("expected refusal without delete, error=%v deleted=%v", err, discordClient.deleted)
	}
}

func TestRemoveDeletesOwnedEmoji(t *testing.T) {
	discordClient := newFakeDiscord()
	discordClient.emojis["900"] = discord.Emoji{
		ID: "900", Name: "owned", User: &discord.User{ID: "10"},
	}
	emojiService := newTestService(t, discordClient, &fakeGenerator{})

	if err := emojiService.RemoveEmoji(context.Background(), "900"); err != nil {
		t.Fatal(err)
	}
	if len(discordClient.deleted) != 1 || discordClient.deleted[0] != "900" {
		t.Fatalf("unexpected deletes: %v", discordClient.deleted)
	}
}

func TestListCreatedEmojisUsesDiscordCreator(t *testing.T) {
	discordClient := newFakeDiscord()
	discordClient.emojis["800"] = discord.Emoji{
		ID: "800", Name: "owned", User: &discord.User{ID: "10"},
	}
	discordClient.emojis["900"] = discord.Emoji{
		ID: "900", Name: "other", User: &discord.User{ID: "20"},
	}
	emojiService := newTestService(t, discordClient, &fakeGenerator{})

	emojis, err := emojiService.ListCreatedEmojis(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(emojis) != 1 || emojis[0].ID != "800" {
		t.Fatalf("unexpected created emoji list: %#v", emojis)
	}
}

func newFakeDiscord() *fakeDiscord {
	return &fakeDiscord{
		user:   discord.User{ID: "10", Username: "emojigen"},
		emojis: make(map[string]discord.Emoji),
		created: discord.Emoji{
			ID: "300", Name: "new_emoji", Available: true, User: &discord.User{ID: "10"},
		},
	}
}

func newTestService(
	t *testing.T,
	discordClient *fakeDiscord,
	generator *fakeGenerator,
) *Service {
	t.Helper()
	emojiService, err := New(context.Background(), discordClient, generator, "100", 4)
	if err != nil {
		t.Fatal(err)
	}
	return emojiService
}

func testPNG() []byte {
	source := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			source.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		panic(err)
	}
	return output.Bytes()
}
