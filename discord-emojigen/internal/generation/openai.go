package generation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	maxResultBytes = 16 << 20
)

type Reference struct {
	Name string
	PNG  []byte
}

type Request struct {
	Prompt     string
	References []Reference
}

type Generator interface {
	Generate(context.Context, Request) ([]byte, error)
}

type OpenAI struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

func NewOpenAI(apiKey, model string, httpClient *http.Client) *OpenAI {
	return &OpenAI{
		apiKey: apiKey, model: model, baseURL: defaultBaseURL, httpClient: httpClient,
	}
}

func (o *OpenAI) Generate(ctx context.Context, request Request) ([]byte, error) {
	prompt := request.Prompt +
		"\n\nCreate a centered, bold Discord emoji with a simple readable silhouette, " +
		"generous edge clearance, no small text, and a square composition."
	if len(request.References) == 0 {
		return o.generate(ctx, prompt)
	}
	return o.edit(ctx, prompt, request.References)
}

func (o *OpenAI) generate(ctx context.Context, prompt string) ([]byte, error) {
	payload := struct {
		Model        string `json:"model"`
		Prompt       string `json:"prompt"`
		Quality      string `json:"quality"`
		Size         string `json:"size"`
		OutputFormat string `json:"output_format"`
	}{
		Model: o.model, Prompt: prompt, Quality: "low", Size: "1024x1024", OutputFormat: "png",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		o.baseURL+"/images/generations",
		bytes.NewReader(data),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return o.do(req)
}

func (o *OpenAI) edit(ctx context.Context, prompt string, references []Reference) ([]byte, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{
		"model": o.model, "prompt": prompt, "quality": "low",
		"size": "1024x1024", "output_format": "png",
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, err
		}
	}
	for i, reference := range references {
		part, err := writer.CreateFormFile("image[]", fmt.Sprintf("reference-%d.png", i+1))
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(reference.PNG); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		o.baseURL+"/images/edits",
		&body,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return o.do(req)
}

func (o *OpenAI) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("User-Agent", "discord-emojigen-mcp/0.1.0")
	// #nosec G704 -- req URLs use the package-owned OpenAI API base URL.
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResultBytes+1))
	closeErr := resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(data) > maxResultBytes {
		return nil, errors.New("OpenAI response exceeded size limit")
	}
	return decodeResponse(resp.StatusCode, data)
}

func decodeResponse(status int, data []byte) ([]byte, error) {
	if status < 200 || status >= 300 {
		var apiError struct {
			Error struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &apiError) == nil && apiError.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI image generation failed: %s (%s)",
				apiError.Error.Message, apiError.Error.Code)
		}
		return nil, fmt.Errorf("OpenAI image generation returned HTTP %d", status)
	}

	var result struct {
		Data []struct {
			Base64 string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode OpenAI response: %w", err)
	}
	if len(result.Data) != 1 || result.Data[0].Base64 == "" {
		return nil, errors.New("OpenAI response did not contain exactly one image")
	}
	image, err := base64.StdEncoding.DecodeString(result.Data[0].Base64)
	if err != nil {
		return nil, fmt.Errorf("decode generated image: %w", err)
	}
	return image, nil
}
