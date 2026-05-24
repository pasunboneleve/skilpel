package skilpel

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIProviderReportsStatusForNonJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "<html>bad gateway</html>", http.StatusBadGateway)
	}))
	defer server.Close()

	provider := &OpenAIProvider{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Client:  server.Client(),
	}
	_, err := provider.Complete(context.Background(), CompletionRequest{Model: "model", User: "hello"})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if !strings.Contains(err.Error(), "502 Bad Gateway") {
		t.Fatalf("expected status in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "non-JSON response") {
		t.Fatalf("expected non-JSON context, got %v", err)
	}
}
