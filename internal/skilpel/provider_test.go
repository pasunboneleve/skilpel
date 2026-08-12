package skilpel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderPluginMapMatchesOrderedPlugins(t *testing.T) {
	for _, plugin := range orderedProviderPlugins {
		resolved, err := resolveProviderPlugin(plugin.Name)
		if err != nil {
			t.Fatalf("resolve provider %q: %v", plugin.Name, err)
		}
		if resolved.Name != plugin.Name ||
			resolved.Description != plugin.Description ||
			resolved.DefaultBaseURL != plugin.DefaultBaseURL ||
			resolved.DefaultAPIKeyEnv != plugin.DefaultAPIKeyEnv ||
			resolved.BaseURLOverride != plugin.BaseURLOverride {
			t.Fatalf("provider %q map entry drifted from ordered registry", plugin.Name)
		}
		if resolved.New == nil {
			t.Fatalf("provider %q has no constructor", plugin.Name)
		}
	}
	if len(providerPlugins) != len(orderedProviderPlugins) {
		t.Fatalf("provider map has %d entries, ordered registry has %d", len(providerPlugins), len(orderedProviderPlugins))
	}
}

func TestDefaultProviderIsOpenAI(t *testing.T) {
	plugin, err := resolveProviderPlugin("")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if plugin.Name != defaultProviderName {
		t.Fatalf("provider = %q, want %s", plugin.Name, defaultProviderName)
	}
}

func TestOpenAIProviderUsesResponsesAndNormalizesLegacyParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path = %q, want /responses", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if body["instructions"] != "system guidance" || body["input"] != "user prompt" {
			t.Errorf("unexpected prompt fields: %#v", body)
			http.Error(w, "unexpected prompt fields", http.StatusBadRequest)
			return
		}
		if body["store"] != false {
			t.Errorf("store = %#v, want false", body["store"])
			http.Error(w, "storage must be disabled", http.StatusBadRequest)
			return
		}
		if body["max_output_tokens"] != float64(64) {
			t.Errorf("max_output_tokens = %#v, want 64", body["max_output_tokens"])
			http.Error(w, "unexpected token limit", http.StatusBadRequest)
			return
		}
		reasoning, ok := body["reasoning"].(map[string]any)
		if !ok || reasoning["effort"] != "none" {
			t.Errorf("reasoning = %#v, want effort none", body["reasoning"])
			http.Error(w, "unexpected reasoning", http.StatusBadRequest)
			return
		}
		if _, exists := body["max_tokens"]; exists {
			t.Errorf("legacy max_tokens leaked into Responses request: %#v", body)
			http.Error(w, "legacy token parameter leaked", http.StatusBadRequest)
			return
		}
		if _, exists := body["reasoning_effort"]; exists {
			t.Errorf("legacy reasoning_effort leaked into Responses request: %#v", body)
			http.Error(w, "legacy reasoning parameter leaked", http.StatusBadRequest)
			return
		}

		writeFakeResponse(t, w, "native response")
	}))
	t.Cleanup(server.Close)

	provider, err := newOpenAIProvider(ResolvedProviderConfig{
		Name: "openai", BaseURL: server.URL, APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	result, err := provider.Complete(context.Background(), CompletionRequest{
		Model:  "test-model",
		System: "system guidance",
		User:   "user prompt",
		Params: map[string]any{
			"max_tokens":       64,
			"reasoning_effort": "none",
			":store":           true,
		},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.Output != "native response" || result.InputTokens != 3 || result.OutputTokens != 2 {
		t.Fatalf("result = %#v", result)
	}
}

func TestOpenAIProviderRejectsStorageOverride(t *testing.T) {
	provider, err := newOpenAIProvider(ResolvedProviderConfig{
		Name: "openai", BaseURL: "http://unused.invalid", APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, err = provider.Complete(context.Background(), CompletionRequest{
		Model: "test-model",
		User:  "user prompt",
		Params: map[string]any{
			"store": true,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "fixes store=false") {
		t.Fatalf("error = %v, want storage invariant diagnostic", err)
	}
}

func TestOpenAIProviderRejectsChatOnlySeed(t *testing.T) {
	provider, err := newOpenAIProvider(ResolvedProviderConfig{
		Name: "openai", BaseURL: "http://unused.invalid", APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	_, err = provider.Complete(context.Background(), CompletionRequest{
		Model: "test-model",
		User:  "user prompt",
		Params: map[string]any{
			"seed": 12345,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not support seed") {
		t.Fatalf("error = %v, want unsupported seed diagnostic", err)
	}
}

func TestOpenAIChatProviderRetainsChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q, want /chat/completions", r.URL.Path)
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if len(body.Messages) != 2 || body.Messages[0].Role != "system" || body.Messages[1].Role != "user" {
			t.Errorf("messages = %#v", body.Messages)
			http.Error(w, "unexpected messages", http.StatusBadRequest)
			return
		}
		w.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "chat response"}}},
			"usage":   map[string]int{"prompt_tokens": 3, "completion_tokens": 2},
		}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	provider, err := newOpenAICompatibleProvider(ResolvedProviderConfig{
		Name: "openai-chat", BaseURL: server.URL, APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	result, err := provider.Complete(context.Background(), CompletionRequest{
		Model: "test-model", System: "system guidance", User: "user prompt",
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if result.Output != "chat response" {
		t.Fatalf("output = %q, want chat response", result.Output)
	}
}

func writeFakeResponse(t *testing.T, w http.ResponseWriter, output string) {
	t.Helper()
	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"id":     "resp_test",
		"object": "response",
		"status": "completed",
		"output": []map[string]any{{
			"id":     "msg_test",
			"type":   "message",
			"role":   "assistant",
			"status": "completed",
			"content": []map[string]any{{
				"type": "output_text", "text": output, "annotations": []any{},
			}},
		}},
		"usage": map[string]any{
			"input_tokens": 3, "output_tokens": 2, "total_tokens": 5,
		},
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
