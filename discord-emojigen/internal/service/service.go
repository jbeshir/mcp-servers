package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/discord"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/generation"
	"github.com/jbeshir/mcp-servers/discord-emojigen/internal/imageprep"
)

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
	FetchEmojiImage(context.Context, discord.Emoji) ([]byte, error)
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

type CreateRequest struct {
	Name         string
	Prompt       string
	ReferenceIDs []string
	Roles        []string
}

type Service struct {
	discord       Discord
	generator     generation.Generator
	guildID       string
	maxReferences int
	bot           discord.User
	mutations     sync.Mutex
}

func New(
	ctx context.Context,
	discordClient Discord,
	generator generation.Generator,
	guildID string,
	maxReferences int,
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
		discord: discordClient, generator: generator,
		guildID: guildID, maxReferences: maxReferences, bot: bot,
	}, nil
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

func (s *Service) CreateEmoji(ctx context.Context, request CreateRequest) (EmojiView, error) {
	request.Prompt = strings.TrimSpace(request.Prompt)
	if err := s.validateCreateRequest(request); err != nil {
		return EmojiView{}, err
	}

	s.mutations.Lock()
	defer s.mutations.Unlock()

	emojis, err := s.discord.ListGuildEmojis(ctx, s.guildID)
	if err != nil {
		return EmojiView{}, err
	}
	byID := make(map[string]discord.Emoji, len(emojis))
	for _, emoji := range emojis {
		byID[emoji.ID] = emoji
		if strings.EqualFold(emoji.Name, request.Name) {
			return EmojiView{}, fmt.Errorf("an emoji named %q already exists", request.Name)
		}
	}

	references, err := s.fetchReferences(ctx, request.ReferenceIDs, byID)
	if err != nil {
		return EmojiView{}, err
	}

	generated, err := s.generator.Generate(ctx, generation.Request{
		Prompt: request.Prompt, References: references,
	})
	if err != nil {
		return EmojiView{}, err
	}
	png, err := imageprep.Prepare(generated)
	if err != nil {
		return EmojiView{}, fmt.Errorf("prepare generated image: %w", err)
	}

	reason := "discord-emojigen-mcp create " + request.Name
	emoji, err := s.discord.CreateGuildEmoji(
		ctx, s.guildID, request.Name, png, request.Roles, reason,
	)
	if err != nil {
		return EmojiView{}, err
	}
	return view(emoji, true), nil
}

func (s *Service) validateCreateRequest(request CreateRequest) error {
	switch {
	case !emojiNamePattern.MatchString(request.Name):
		return errors.New("name must be 2-32 characters using letters, numbers, or underscores")
	case request.Prompt == "" || len(request.Prompt) > 4000:
		return errors.New("prompt must be between 1 and 4000 characters")
	case len(request.ReferenceIDs) > s.maxReferences:
		return fmt.Errorf("at most %d reference emoji are allowed", s.maxReferences)
	case len(request.Roles) > 100:
		return errors.New("at most 100 role IDs are allowed")
	}
	ids := append(append([]string{}, request.ReferenceIDs...), request.Roles...)
	for _, id := range ids {
		if !snowflakePattern.MatchString(id) {
			return fmt.Errorf("%q is not a valid Discord snowflake", id)
		}
	}
	return nil
}

func (s *Service) fetchReferences(
	ctx context.Context,
	ids []string,
	byID map[string]discord.Emoji,
) ([]generation.Reference, error) {
	references := make([]generation.Reference, 0, len(ids))
	for _, id := range ids {
		emoji, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("reference emoji %s does not exist in the configured guild", id)
		}
		if emoji.Animated {
			return nil, fmt.Errorf("reference emoji %s is animated; animated references are not supported", id)
		}
		data, err := s.discord.FetchEmojiImage(ctx, emoji)
		if err != nil {
			return nil, fmt.Errorf("fetch reference emoji %s: %w", id, err)
		}
		png, err := imageprep.Prepare(data)
		if err != nil {
			return nil, fmt.Errorf("prepare reference emoji %s: %w", id, err)
		}
		references = append(references, generation.Reference{Name: emoji.Name, PNG: png})
	}
	return references, nil
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
	if err := s.discord.DeleteGuildEmoji(
		ctx, s.guildID, emojiID, "discord-emojigen-mcp remove "+emoji.Name,
	); err != nil {
		return err
	}
	return nil
}

func view(emoji discord.Emoji, created bool) EmojiView {
	return EmojiView{
		ID: emoji.ID, Name: emoji.Name, Animated: emoji.Animated, Available: emoji.Available,
		Mention: emoji.Mention(), PreviewURL: emoji.CDNURL(),
		CreatedByBot: created,
	}
}

func createdBy(emoji discord.Emoji, botID string) bool {
	return emoji.User != nil && emoji.User.ID == botID
}
