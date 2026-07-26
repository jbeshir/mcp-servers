package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
)

type fakeDiscord struct {
	user        discord.User
	emojis      map[string]discord.Emoji
	deleted     []string
	created     discord.Emoji
	uploadPNG   []byte
	uploadRoles []string
	createCalls int
	fetched     []string
	fetchData   []byte
	fetchType   string
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
	_ context.Context, _ string, name string, data []byte, roles []string, _ string,
) (discord.Emoji, error) {
	f.createCalls++
	f.uploadPNG = append([]byte(nil), data...)
	f.uploadRoles = append([]string(nil), roles...)
	f.created.Name = name
	f.emojis[f.created.ID] = f.created
	return f.created, nil
}
func (f *fakeDiscord) DeleteGuildEmoji(_ context.Context, _, id, _ string) error {
	f.deleted = append(f.deleted, id)
	delete(f.emojis, id)
	return nil
}
func (f *fakeDiscord) FetchEmojiImage(
	_ context.Context, emoji discord.Emoji,
) ([]byte, string, error) {
	f.fetched = append(f.fetched, emoji.ID)
	return f.fetchData, f.fetchType, nil
}

func TestListServerEmojisReturnsStableDiscordMetadata(t *testing.T) {
	client := newFakeDiscord()
	client.emojis["201"] = discord.Emoji{
		ID: "201", Name: "wave", Animated: true, Available: true,
		User: &discord.User{ID: "20"},
	}
	svc := newTestService(t, client)
	got, err := svc.ListServerEmojis(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "201" || got[0].Name != "wave" ||
		!got[0].Animated || got[0].Mention != "<a:wave:201>" ||
		got[0].PreviewURL != "https://cdn.discordapp.com/emojis/201.webp?size=128" {
		t.Fatalf("unexpected view: %#v", got)
	}
}

func TestFetchEmojiReference(t *testing.T) {
	client := newFakeDiscord()
	client.emojis["200"] = discord.Emoji{ID: "200", Name: "reference", Available: true}
	client.fetchData = []byte("static image")
	client.fetchType = "image/webp"
	svc := newTestService(t, client)
	got, err := svc.FetchEmojiReference(context.Background(), "200")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "200" || got.ContentType != "image/webp" ||
		got.Base64 != base64.StdEncoding.EncodeToString(client.fetchData) ||
		got.DataURL != "data:image/webp;base64,"+got.Base64 ||
		len(client.fetched) != 1 {
		t.Fatalf("unexpected reference: %#v fetched=%v", got, client.fetched)
	}
}

func TestFetchEmojiReferenceRejectsNonMemberAndAnimated(t *testing.T) {
	client := newFakeDiscord()
	client.emojis["200"] = discord.Emoji{ID: "200", Name: "animated", Animated: true}
	svc := newTestService(t, client)
	for _, id := range []string{"999", "200"} {
		if _, err := svc.FetchEmojiReference(context.Background(), id); err == nil {
			t.Fatalf("expected %s to be rejected", id)
		}
	}
	if len(client.fetched) != 0 {
		t.Fatalf("unexpected CDN fetch: %v", client.fetched)
	}
}

func TestUploadEmojiAcceptsRawBase64AndDataURL(t *testing.T) {
	for _, input := range []string{
		base64.StdEncoding.EncodeToString(testPNG()),
		"data:image/png;base64," + base64.StdEncoding.EncodeToString(testPNG()),
	} {
		t.Run(input[:8], func(t *testing.T) {
			client := newFakeDiscord()
			svc := newTestService(t, client)
			got, err := svc.UploadEmoji(context.Background(), UploadRequest{
				Name: "new_emoji", ImageData: input, Roles: []string{"123"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Mention != "<:new_emoji:300>" || len(client.uploadPNG) == 0 ||
				len(client.uploadRoles) != 1 || client.uploadRoles[0] != "123" {
				t.Fatalf("unexpected upload: %#v roles=%v", got, client.uploadRoles)
			}
			decoded, err := png.Decode(bytes.NewReader(client.uploadPNG))
			if err != nil {
				t.Fatal(err)
			}
			if decoded.Bounds().Size() != (image.Point{X: 128, Y: 128}) {
				t.Fatalf("unexpected prepared dimensions: %v", decoded.Bounds())
			}
		})
	}
}

func TestUploadEmojiAcceptsAbsolutePathAndFileURI(t *testing.T) {
	dir := t.TempDir()
	imagePath := filepath.Join(dir, "generated")
	if err := os.WriteFile(imagePath, testPNG(), 0o600); err != nil {
		t.Fatal(err)
	}
	fileURI := (&url.URL{Scheme: "file", Path: imagePath}).String()
	for _, path := range []string{imagePath, fileURI} {
		t.Run(path[:4], func(t *testing.T) {
			client := newFakeDiscord()
			_, err := newTestService(t, client).UploadEmoji(context.Background(), UploadRequest{
				Name: "path_image", ImagePath: path,
			})
			if err != nil {
				t.Fatal(err)
			}
			if client.createCalls != 1 {
				t.Fatalf("Discord create calls = %d, want 1", client.createCalls)
			}
		})
	}
}

func TestUploadEmojiPathUsesDecodedContentNotExtension(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"extensionless", "actually-not-jpeg.jpg"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, testPNG(), 0o600); err != nil {
			t.Fatal(err)
		}
		client := newFakeDiscord()
		if _, err := newTestService(t, client).UploadEmoji(context.Background(), UploadRequest{
			Name: "decoded_image", ImagePath: path,
		}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if client.createCalls != 1 {
			t.Fatalf("%s: Discord create calls = %d", name, client.createCalls)
		}
	}
}

func TestUploadEmojiRejectsInvalidInputSelectionWithoutDiscordUpload(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(testPNG())
	for _, request := range []UploadRequest{
		{Name: "valid_name"},
		{Name: "valid_name", ImageData: valid, ImagePath: "/unused"},
	} {
		client := newFakeDiscord()
		if _, err := newTestService(t, client).UploadEmoji(context.Background(), request); err == nil {
			t.Fatalf("expected rejection for %#v", request)
		}
		if client.createCalls != 0 {
			t.Fatal("invalid input selection reached Discord upload")
		}
	}
}

func TestUploadEmojiRejectsUnsafePathsWithoutDiscordUpload(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, testPNG(), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(dir, "oversized")
	if err := os.WriteFile(oversized, make([]byte, maxInputBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		path string
	}{
		{"relative", "image.png"},
		{"non-local URI", "file://example.com/image.png"},
		{"malformed URI", "file:///bad%zz"},
		{"directory", dir},
		{"symlink", link},
		{"empty", empty},
		{"oversized", oversized},
		{"missing", filepath.Join(dir, "secret-missing-name")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeDiscord()
			_, err := newTestService(t, client).UploadEmoji(context.Background(), UploadRequest{
				Name: "valid_name", ImagePath: tt.path,
			})
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), tt.path) || strings.Contains(err.Error(), "secret-missing-name") {
				t.Fatalf("error exposes path: %q", err)
			}
			if client.createCalls != 0 {
				t.Fatal("unsafe path reached Discord upload")
			}
		})
	}
}

func TestReadImagePathBoundaries(t *testing.T) {
	dir := t.TempDir()
	for _, size := range []int{maxInputBytes, maxInputBytes + 1} {
		path := filepath.Join(dir, fmt.Sprintf("input-%d", size))
		if err := os.WriteFile(path, make([]byte, size), 0o600); err != nil {
			t.Fatal(err)
		}
		data, err := readImagePath(path)
		if size == maxInputBytes {
			if err != nil || len(data) != size {
				t.Fatalf("exact limit: len=%d err=%v", len(data), err)
			}
		} else if err == nil {
			t.Fatal("over-limit file accepted")
		}
	}
}

func TestDecodeImageDataRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"malformed base64", "%%%"},
		{"non-base64 data URL", "data:image/png,abc"},
		{"unsupported data URL", "data:image/svg+xml;base64,PHN2Zz4="},
		{"empty payload", "data:image/png;base64,"},
		{"oversized", base64.StdEncoding.EncodeToString(make([]byte, maxInputBytes+1))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := decodeImageData(tt.value); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDecodeImageDataBoundaries(t *testing.T) {
	for _, size := range []int{maxInputBytes - 1, maxInputBytes} {
		input := make([]byte, size)
		decoded, _, err := decodeImageData(base64.StdEncoding.EncodeToString(input))
		if err != nil {
			t.Fatalf("size %d rejected: %v", size, err)
		}
		if len(decoded) != size {
			t.Fatalf("size %d decoded to %d bytes", size, len(decoded))
		}
	}
	if _, _, err := decodeImageData(
		base64.StdEncoding.EncodeToString(make([]byte, maxInputBytes+1)),
	); err == nil {
		t.Fatal("expected 16 MiB + 1 byte to be rejected")
	}
}

func TestUploadEmojiRejectsMismatchedDataURLMediaType(t *testing.T) {
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		contentType string
		payload     []byte
	}{
		{"image/jpeg", testPNG()},
		{"image/gif", testPNG()},
		{"image/png", jpegData.Bytes()},
	}
	for _, tt := range tests {
		client := newFakeDiscord()
		svc := newTestService(t, client)
		_, err := svc.UploadEmoji(context.Background(), UploadRequest{
			Name:      "new_emoji",
			ImageData: "data:" + tt.contentType + ";base64," + base64.StdEncoding.EncodeToString(tt.payload),
		})
		if err == nil {
			t.Fatalf("expected payload declared as %s to be rejected", tt.contentType)
		}
		if len(client.uploadPNG) != 0 {
			t.Fatal("mismatched image reached Discord upload")
		}
	}
}

func TestUploadEmojiRejectsBadImageNameRolesAndDuplicate(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(testPNG())
	tests := []UploadRequest{
		{Name: "x", ImageData: valid},
		{Name: "valid_name", ImageData: "bm90IGFuIGltYWdl"},
		{Name: "valid_name", ImageData: valid, Roles: []string{"not-id"}},
	}
	for _, request := range tests {
		client := newFakeDiscord()
		svc := newTestService(t, client)
		if _, err := svc.UploadEmoji(context.Background(), request); err == nil {
			t.Fatalf("expected rejection for %#v", request)
		}
		if client.createCalls != 0 {
			t.Fatalf("rejected request reached Discord upload: %#v", request)
		}
	}
	client := newFakeDiscord()
	client.emojis["400"] = discord.Emoji{ID: "400", Name: "NEW_EMOJI"}
	if _, err := newTestService(t, client).UploadEmoji(context.Background(), UploadRequest{
		Name: "new_emoji", ImageData: valid,
	}); err == nil || len(client.uploadPNG) != 0 {
		t.Fatal("expected duplicate rejection before upload")
	}
}

func TestRemoveCreatorSafetyAndStatelessListing(t *testing.T) {
	client := newFakeDiscord()
	client.emojis["800"] = discord.Emoji{
		ID: "800", Name: "owned", User: &discord.User{ID: "10"},
	}
	client.emojis["900"] = discord.Emoji{
		ID: "900", Name: "other", User: &discord.User{ID: "20"},
	}
	client.emojis["901"] = discord.Emoji{ID: "901", Name: "missing_creator"}
	svc := newTestService(t, client)
	created, err := svc.ListCreatedEmojis(context.Background())
	if err != nil || len(created) != 1 || created[0].ID != "800" {
		t.Fatalf("unexpected stateless list: %#v error=%v", created, err)
	}
	for _, id := range []string{"900", "901"} {
		if err := svc.RemoveEmoji(context.Background(), id); err == nil {
			t.Fatalf("expected creator rejection for %s", id)
		}
	}
	if len(client.deleted) != 0 {
		t.Fatalf("unsafe deletes: %v", client.deleted)
	}
	if err := svc.RemoveEmoji(context.Background(), "800"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(client.deleted, ",") != "800" {
		t.Fatalf("unexpected deletes: %v", client.deleted)
	}
}

func newFakeDiscord() *fakeDiscord {
	return &fakeDiscord{
		user: discord.User{ID: "10", Username: "emojigen"}, emojis: make(map[string]discord.Emoji),
		created:   discord.Emoji{ID: "300", Available: true, User: &discord.User{ID: "10"}},
		fetchData: testPNG(), fetchType: "image/png",
	}
}

func newTestService(t *testing.T, client *fakeDiscord) *Service {
	t.Helper()
	svc, err := New(context.Background(), client, "100")
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func testPNG() []byte {
	source := image.NewNRGBA(image.Rect(0, 0, 320, 200))
	for y := range 200 {
		for x := range 320 {
			source.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		panic(err)
	}
	return output.Bytes()
}
