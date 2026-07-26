package discord

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://discord.com/api/v10"
	maxBodyBytes   = 2 << 20
)

var ErrNotFound = errors.New("discord resource not found")

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Emoji struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	User      *User  `json:"user,omitempty"`
	Animated  bool   `json:"animated"`
	Available bool   `json:"available"`
	Managed   bool   `json:"managed"`
}

func (e Emoji) Mention() string {
	if e.Animated {
		return fmt.Sprintf("<a:%s:%s>", e.Name, e.ID)
	}
	return fmt.Sprintf("<:%s:%s>", e.Name, e.ID)
}

func (e Emoji) CDNURL() string {
	return fmt.Sprintf("https://cdn.discordapp.com/emojis/%s.webp?size=128", e.ID)
}

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func NewClient(token string, httpClient *http.Client) *Client {
	return &Client{token: token, baseURL: defaultBaseURL, httpClient: httpClient}
}

func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var user User
	err := c.doJSON(ctx, http.MethodGet, "/users/@me", nil, "", &user)
	return user, err
}

func (c *Client) ListGuildEmojis(ctx context.Context, guildID string) ([]Emoji, error) {
	var emojis []Emoji
	err := c.doJSON(ctx, http.MethodGet, "/guilds/"+guildID+"/emojis", nil, "", &emojis)
	return emojis, err
}

func (c *Client) GetGuildEmoji(ctx context.Context, guildID, emojiID string) (Emoji, error) {
	var emoji Emoji
	err := c.doJSON(ctx, http.MethodGet, "/guilds/"+guildID+"/emojis/"+emojiID, nil, "", &emoji)
	return emoji, err
}

func (c *Client) CreateGuildEmoji(
	ctx context.Context,
	guildID, name string,
	png []byte,
	roles []string,
	reason string,
) (Emoji, error) {
	payload := struct {
		Name  string   `json:"name"`
		Image string   `json:"image"`
		Roles []string `json:"roles,omitempty"`
	}{
		Name:  name,
		Image: "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Roles: roles,
	}
	var emoji Emoji
	err := c.doJSON(ctx, http.MethodPost, "/guilds/"+guildID+"/emojis", payload, reason, &emoji)
	return emoji, err
}

func (c *Client) DeleteGuildEmoji(
	ctx context.Context,
	guildID, emojiID, reason string,
) error {
	return c.doJSON(
		ctx,
		http.MethodDelete,
		"/guilds/"+guildID+"/emojis/"+emojiID,
		nil,
		reason,
		nil,
	)
}

func (c *Client) FetchEmojiImage(ctx context.Context, emoji Emoji) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, emoji.CDNURL(), nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, "", fmt.Errorf("discord CDN returned HTTP %d", resp.StatusCode)
	}
	data, readErr := readLimited(resp.Body, maxBodyBytes)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return nil, "", readErr
	}
	if closeErr != nil {
		return nil, "", closeErr
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
	default:
		return nil, "", fmt.Errorf("discord CDN returned unsupported content type %q", contentType)
	}
	return data, contentType, nil
}

func (c *Client) doJSON(
	ctx context.Context,
	method, path string,
	input any,
	reason string,
	output any,
) error {
	var bodyData []byte
	if input != nil {
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		bodyData = data
	}

	for attempt := range 3 {
		status, data, retryAfter, err := c.request(
			ctx, method, path, bodyData, input != nil, reason,
		)
		if err != nil {
			return err
		}
		if status == http.StatusTooManyRequests && attempt < 2 && retryAfter > 0 {
			if err := wait(ctx, retryAfter); err != nil {
				return err
			}
			continue
		}
		return decodeResponse(status, data, output)
	}
	return errors.New("discord API rate limit retry exhausted")
}

func decodeResponse(status int, data []byte, output any) error {
	if status == http.StatusNotFound {
		return ErrNotFound
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("discord API returned HTTP %d: %s", status, safeMessage(data))
	}
	if output == nil || len(data) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode Discord response: %w", err)
	}
	return nil
}

func (c *Client) request(
	ctx context.Context,
	method, path string,
	bodyData []byte,
	hasInput bool,
	reason string,
) (int, []byte, time.Duration, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		method,
		strings.TrimRight(c.baseURL, "/")+path,
		bytes.NewReader(bodyData),
	)
	if err != nil {
		return 0, nil, 0, err
	}
	req.Header.Set("Authorization", "Bot "+c.token)
	req.Header.Set("User-Agent", "discord-emojigen-mcp/0.1.0")
	if hasInput {
		req.Header.Set("Content-Type", "application/json")
	}
	if reason != "" {
		req.Header.Set("X-Audit-Log-Reason", url.QueryEscape(reason))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, 0, err
	}
	data, readErr := readLimited(resp.Body, maxBodyBytes)
	closeErr := resp.Body.Close()
	if readErr != nil {
		return 0, nil, 0, readErr
	}
	if closeErr != nil {
		return 0, nil, 0, closeErr
	}
	return resp.StatusCode, data, parseRetryAfter(data), nil
}

func parseRetryAfter(data []byte) time.Duration {
	var response struct {
		RetryAfter float64 `json:"retry_after"`
	}
	if json.Unmarshal(data, &response) != nil || response.RetryAfter <= 0 {
		return 0
	}
	return time.Duration(response.RetryAfter * float64(time.Second))
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response exceeded size limit")
	}
	return data, nil
}

func safeMessage(data []byte) string {
	var response struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	}
	if json.Unmarshal(data, &response) == nil && response.Message != "" {
		if response.Code != 0 {
			return response.Message + " (code " + strconv.Itoa(response.Code) + ")"
		}
		return response.Message
	}
	return "upstream request failed"
}
