package skilpel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Provider interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error)
}

type CompletionRequest struct {
	Model  string
	System string
	User   string
	Params map[string]any
}

type CompletionResult struct {
	Output       string
	InputTokens  int
	OutputTokens int
}

type OpenAIProvider struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func newOpenAIProvider(cfg Config) (*OpenAIProvider, error) {
	apiKey := os.Getenv(cfg.APIKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s is not set", cfg.APIKeyEnv)
	}
	return &OpenAIProvider{
		BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		APIKey:  apiKey,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}, nil
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	if p.Client == nil {
		p.Client = &http.Client{Timeout: 120 * time.Second}
	}
	endpoint, err := url.JoinPath(p.BaseURL, "chat", "completions")
	if err != nil {
		return CompletionResult{}, err
	}

	messages := []map[string]string{}
	if req.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.User})

	body := map[string]any{
		"model":    req.Model,
		"messages": messages,
	}
	for key, value := range req.Params {
		body[key] = value
	}

	data, err := json.Marshal(body)
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return CompletionResult{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+p.APIKey)

	resp, err := p.Client.Do(httpReq)
	if err != nil {
		return CompletionResult{}, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResult{}, fmt.Errorf("read provider response: %w", err)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return CompletionResult{}, fmt.Errorf("provider returned %s with non-JSON response: %s", resp.Status, truncate(string(respBody), 500))
		}
		return CompletionResult{}, fmt.Errorf("decode provider response from %s: %w", resp.Status, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return CompletionResult{}, fmt.Errorf("provider returned %s: %v", resp.Status, parsed.Error)
	}
	if len(parsed.Choices) == 0 {
		return CompletionResult{}, fmt.Errorf("provider returned no choices")
	}
	return CompletionResult{
		Output:       parsed.Choices[0].Message.Content,
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
	}, nil
}
