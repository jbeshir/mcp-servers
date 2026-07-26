package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/imageprep"
)

const maxInputBytes = 16 << 20

const ImagePathDisabledError = "image_path uploads are disabled; configure DISCORD_EMOJIGEN_ALLOWED_IMAGE_ROOTS"

var (
	emojiNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{2,32}$`)
	snowflakePattern = regexp.MustCompile(`^[0-9]{1,20}$`)
)

type Discord interface {
	CurrentUser(context.Context) (discord.User, error)
	ListGuildEmojis(context.Context, string) ([]discord.Emoji, error)
	GetGuildEmoji(context.Context, string, string) (discord.Emoji, error)
	CreateGuildEmoji(context.Context, string, string, []byte, []string, string) (discord.Emoji, error)
	DeleteGuildEmoji(context.Context, string, string, string) error
	FetchEmojiImage(context.Context, discord.Emoji) ([]byte, string, error)
}

type EmojiView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Animated     bool   `json:"animated"`
	Available    bool   `json:"available"`
	Mention      string `json:"mention"`
	PreviewURL   string `json:"preview_url"`
	CreatedByBot bool   `json:"created_by_bot"`
}

type EmojiReference struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ContentType string `json:"content_type"`
	Base64      string `json:"base64"`
	DataURL     string `json:"data_url"`
}

type UploadRequest struct {
	Name      string
	ImageData string
	ImagePath string
	Roles     []string
}

type Service struct {
	discord           Discord
	guildID           string
	bot               discord.User
	allowedImageRoots []string
	mutations         sync.Mutex
}

func New(
	ctx context.Context,
	discordClient Discord,
	guildID string,
	allowedImageRoots []string,
) (*Service, error) {
	if !snowflakePattern.MatchString(guildID) {
		return nil, errors.New("configured Discord guild ID is not a snowflake")
	}
	bot, err := discordClient.CurrentUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("authenticate Discord bot: %w", err)
	}
	if _, err := discordClient.ListGuildEmojis(ctx, guildID); err != nil {
		return nil, fmt.Errorf("access configured Discord guild: %w", err)
	}
	return &Service{
		discord: discordClient, guildID: guildID, bot: bot,
		allowedImageRoots: append([]string(nil), allowedImageRoots...),
	}, nil
}

func (s *Service) AllowedImageRoots() []string {
	return append([]string(nil), s.allowedImageRoots...)
}

func (s *Service) ListServerEmojis(
	ctx context.Context,
	query string,
	includeUnavailable bool,
) ([]EmojiView, error) {
	emojis, err := s.discord.ListGuildEmojis(ctx, s.guildID)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	views := make([]EmojiView, 0, len(emojis))
	for _, emoji := range emojis {
		if query != "" && !strings.Contains(strings.ToLower(emoji.Name), query) {
			continue
		}
		if !includeUnavailable && !emoji.Available {
			continue
		}
		views = append(views, view(emoji, createdBy(emoji, s.bot.ID)))
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Name == views[j].Name {
			return views[i].ID < views[j].ID
		}
		return views[i].Name < views[j].Name
	})
	return views, nil
}

func (s *Service) FetchEmojiReference(ctx context.Context, emojiID string) (EmojiReference, error) {
	if !snowflakePattern.MatchString(emojiID) {
		return EmojiReference{}, errors.New("emoji_id is not a valid Discord snowflake")
	}
	emoji, err := s.discord.GetGuildEmoji(ctx, s.guildID, emojiID)
	if errors.Is(err, discord.ErrNotFound) {
		return EmojiReference{}, errors.New("emoji does not exist in the configured guild")
	}
	if err != nil {
		return EmojiReference{}, err
	}
	if emoji.Animated {
		return EmojiReference{}, errors.New("animated emoji references are not supported")
	}
	data, contentType, err := s.discord.FetchEmojiImage(ctx, emoji)
	if err != nil {
		return EmojiReference{}, fmt.Errorf("fetch emoji image: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return EmojiReference{
		ID: emoji.ID, Name: emoji.Name, ContentType: contentType, Base64: encoded,
		DataURL: "data:" + contentType + ";base64," + encoded,
	}, nil
}

func (s *Service) UploadEmoji(ctx context.Context, request UploadRequest) (EmojiView, error) {
	if !emojiNamePattern.MatchString(request.Name) {
		return EmojiView{}, errors.New("name must be 2-32 characters using letters, numbers, or underscores")
	}
	if len(request.Roles) > 100 {
		return EmojiView{}, errors.New("at most 100 role IDs are allowed")
	}
	for _, id := range request.Roles {
		if !snowflakePattern.MatchString(id) {
			return EmojiView{}, fmt.Errorf("%q is not a valid Discord role snowflake", id)
		}
	}
	input, declaredMediaType, err := s.resolveImageInput(request.ImageData, request.ImagePath)
	if err != nil {
		return EmojiView{}, err
	}
	png, err := imageprep.Prepare(input, declaredMediaType)
	if err != nil {
		return EmojiView{}, fmt.Errorf("prepare image: %w", err)
	}

	s.mutations.Lock()
	defer s.mutations.Unlock()
	emojis, err := s.discord.ListGuildEmojis(ctx, s.guildID)
	if err != nil {
		return EmojiView{}, err
	}
	for _, emoji := range emojis {
		if strings.EqualFold(emoji.Name, request.Name) {
			return EmojiView{}, fmt.Errorf("an emoji named %q already exists", request.Name)
		}
	}
	emoji, err := s.discord.CreateGuildEmoji(
		ctx, s.guildID, request.Name, png, request.Roles,
		"discord-emojigen-mcp upload "+request.Name,
	)
	if err != nil {
		return EmojiView{}, err
	}
	return view(emoji, createdBy(emoji, s.bot.ID)), nil
}

func (s *Service) resolveImageInput(imageData, imagePath string) ([]byte, string, error) {
	hasData := strings.TrimSpace(imageData) != ""
	hasPath := strings.TrimSpace(imagePath) != ""
	if hasData == hasPath {
		return nil, "", errors.New("exactly one of image_data or image_path is required")
	}
	if hasData {
		return decodeImageData(imageData)
	}
	if len(s.allowedImageRoots) == 0 {
		return nil, "", errors.New(ImagePathDisabledError)
	}
	data, err := readImagePath(imagePath, s.allowedImageRoots)
	return data, "", err
}

// readImagePath performs a best-effort regular-file check without
// platform-specific dependencies. The Lstat-before-Open sequence refuses a
// final-component symlink, but another process can still replace the path
// between those operations.
func readImagePath(value string, allowedRoots []string) ([]byte, error) {
	path, err := localImagePath(strings.TrimSpace(value))
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, errors.New("image_path cannot be accessed")
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("image_path must not be a symlink")
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return nil, errors.New("image_path cannot be resolved")
	}
	if !withinAllowedRoot(canonical, allowedRoots) {
		return nil, errors.New("image_path is outside the configured allowed image roots")
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("image_path must be a regular file")
	}
	// #nosec G304 -- reading an explicit caller-supplied local path is this
	// tool's purpose; the path and file type are validated immediately above.
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("image_path cannot be opened")
	}

	data, err := io.ReadAll(io.LimitReader(file, maxInputBytes+1))
	closeErr := file.Close()
	if err != nil {
		return nil, errors.New("image_path cannot be read")
	}
	if closeErr != nil {
		return nil, errors.New("image_path cannot be closed")
	}
	if len(data) == 0 {
		return nil, errors.New("image_path file is empty")
	}
	if len(data) > maxInputBytes {
		return nil, fmt.Errorf("image_path exceeds %d MiB limit", maxInputBytes>>20)
	}
	return data, nil
}

func withinAllowedRoot(path string, allowedRoots []string) bool {
	for _, root := range allowedRoots {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func localImagePath(value string) (string, error) {
	if value == "" {
		return "", errors.New("image_path is required")
	}
	if filepath.IsAbs(value) {
		return value, nil
	}
	return localFileURIPath(value)
}

func localFileURIPath(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", errors.New("image_path must be an absolute path or local file URI")
	}
	if parsed.Scheme != "file" || parsed.Opaque != "" || parsed.User != nil {
		return "", errors.New("image_path must be an absolute path or local file URI")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("image_path must be an absolute path or local file URI")
	}
	if parsed.Host != "" && !strings.EqualFold(parsed.Host, "localhost") {
		return "", errors.New("image_path must be an absolute path or local file URI")
	}
	if parsed.RawPath != "" {
		if _, err := url.PathUnescape(parsed.RawPath); err != nil {
			return "", errors.New("image_path file URI is malformed")
		}
	}
	if !filepath.IsAbs(parsed.Path) {
		return "", errors.New("image_path file URI must contain an absolute path")
	}
	return filepath.FromSlash(parsed.Path), nil
}

func decodeImageData(value string) ([]byte, string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, "", errors.New("image_data is required")
	}
	declaredMediaType := ""
	if strings.HasPrefix(value, "data:") {
		header, payload, ok := strings.Cut(value, ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return nil, "", errors.New("image_data must be raw base64 or a base64 image data URL")
		}
		declaredMediaType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		switch declaredMediaType {
		case "image/png", "image/jpeg", "image/gif":
		default:
			return nil, "", fmt.Errorf("unsupported image data URL content type %q", declaredMediaType)
		}
		value = payload
	}
	if len(value) > base64.StdEncoding.EncodedLen(maxInputBytes+1) {
		return nil, "", fmt.Errorf("image_data exceeds %d MiB decoded limit", maxInputBytes>>20)
	}
	data, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, "", errors.New("image_data is not valid base64")
	}
	if len(data) > maxInputBytes {
		return nil, "", fmt.Errorf("image_data exceeds %d MiB decoded limit", maxInputBytes>>20)
	}
	if len(data) == 0 {
		return nil, "", errors.New("image_data decodes to an empty payload")
	}
	return data, declaredMediaType, nil
}

func (s *Service) ListCreatedEmojis(ctx context.Context) ([]EmojiView, error) {
	emojis, err := s.discord.ListGuildEmojis(ctx, s.guildID)
	if err != nil {
		return nil, err
	}
	views := make([]EmojiView, 0, len(emojis))
	for _, emoji := range emojis {
		if createdBy(emoji, s.bot.ID) {
			views = append(views, view(emoji, true))
		}
	}
	return views, nil
}

func (s *Service) RemoveEmoji(ctx context.Context, emojiID string) error {
	if !snowflakePattern.MatchString(emojiID) {
		return errors.New("emoji_id is not a valid Discord snowflake")
	}
	s.mutations.Lock()
	defer s.mutations.Unlock()
	emoji, err := s.discord.GetGuildEmoji(ctx, s.guildID, emojiID)
	if errors.Is(err, discord.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if emoji.User == nil || emoji.User.ID != s.bot.ID {
		return errors.New("refusing to remove emoji: Discord creator does not match this bot")
	}
	return s.discord.DeleteGuildEmoji(
		ctx, s.guildID, emojiID, "discord-emojigen-mcp remove "+emoji.Name,
	)
}

func view(emoji discord.Emoji, created bool) EmojiView {
	return EmojiView{
		ID: emoji.ID, Name: emoji.Name, Animated: emoji.Animated, Available: emoji.Available,
		Mention: emoji.Mention(), PreviewURL: emoji.CDNURL(), CreatedByBot: created,
	}
}

func createdBy(emoji discord.Emoji, botID string) bool {
	return emoji.User != nil && emoji.User.ID == botID
}
