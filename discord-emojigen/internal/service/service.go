package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/imageprep"
)

const maxInputBytes = 16 << 20

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
	Roles     []string
}

type Service struct {
	discord   Discord
	guildID   string
	bot       discord.User
	mutations sync.Mutex
}

func New(ctx context.Context, discordClient Discord, guildID string) (*Service, error) {
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
	return &Service{discord: discordClient, guildID: guildID, bot: bot}, nil
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
	input, declaredMediaType, err := decodeImageData(request.ImageData)
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
