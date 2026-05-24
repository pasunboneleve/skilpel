package skilpel

import "testing"

func TestProviderPluginDefaults(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		apiKeyEnv string
	}{
		{name: "openai", baseURL: "https://api.openai.com/v1", apiKeyEnv: "OPENAI_API_KEY"},
		{name: "xai", baseURL: "https://api.x.ai/v1", apiKeyEnv: "XAI_API_KEY"},
		{name: "qwen", baseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", apiKeyEnv: "DASHSCOPE_API_KEY"},
		{name: "anthropic", apiKeyEnv: "ANTHROPIC_API_KEY"},
		{name: "claude", apiKeyEnv: "ANTHROPIC_API_KEY"},
		{name: "gemini", apiKeyEnv: "GEMINI_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugin, err := resolveProviderPlugin(tt.name)
			if err != nil {
				t.Fatalf("resolve provider: %v", err)
			}
			if plugin.DefaultBaseURL != tt.baseURL {
				t.Fatalf("base URL = %q, want %q", plugin.DefaultBaseURL, tt.baseURL)
			}
			if plugin.DefaultAPIKeyEnv != tt.apiKeyEnv {
				t.Fatalf("API key env = %q, want %q", plugin.DefaultAPIKeyEnv, tt.apiKeyEnv)
			}
		})
	}
}

func TestDefaultProviderIsOpenAI(t *testing.T) {
	plugin, err := resolveProviderPlugin("")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if plugin.Name != "openai" {
		t.Fatalf("provider = %q, want openai", plugin.Name)
	}
}
