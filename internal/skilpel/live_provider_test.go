package skilpel

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestOpenAIProviderLiveWithAPIKey(t *testing.T) {
	apiKeyEnv := providerPlugins[defaultProviderName].DefaultAPIKeyEnv
	if os.Getenv("RUN_OPENAI_INTEGRATION") != "1" {
		t.Skip("set RUN_OPENAI_INTEGRATION=1 to run the live OpenAI provider check")
	}
	if os.Getenv(apiKeyEnv) == "" {
		t.Skip(apiKeyEnv + " is not set")
	}

	provider, err := newProvider(Config{
		Provider:     defaultProviderName,
		Target:       "gpt-4o-mini",
		APIKeyEnv:    apiKeyEnv,
		TargetParams: map[string]any{"temperature": 0, "max_tokens": 20},
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	result, err := provider.Complete(context.Background(), CompletionRequest{
		Model:  "gpt-4o-mini",
		System: "Reply only with the exact token requested.",
		User:   "Reply with exactly: skilpel-ok",
		Params: map[string]any{"temperature": 0, "max_tokens": 20},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if !strings.Contains(result.Output, "skilpel-ok") {
		t.Fatalf("output = %q, want skilpel-ok", result.Output)
	}
}
