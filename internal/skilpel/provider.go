package skilpel

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
	"google.golang.org/genai"
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

type ProviderPlugin struct {
	Name             string
	Description      string
	DefaultBaseURL   string
	DefaultAPIKeyEnv string
	BaseURLOverride  bool
	New              func(ResolvedProviderConfig) (Provider, error)
}

type ResolvedProviderConfig struct {
	Name    string
	BaseURL string
	APIKey  string
}

const (
	defaultProviderName = "openai"
	defaultTimeout      = 120 * time.Second
)

var orderedProviderPlugins = []ProviderPlugin{
	{
		Name:             "openai",
		Description:      "OpenAI Responses API",
		DefaultBaseURL:   "https://api.openai.com/v1",
		DefaultAPIKeyEnv: "OPENAI_API_KEY",
		BaseURLOverride:  true,
		New:              newOpenAIProvider,
	},
	{
		Name:             "openai-chat",
		Description:      "OpenAI Chat Completions compatibility",
		DefaultBaseURL:   "https://api.openai.com/v1",
		DefaultAPIKeyEnv: "OPENAI_API_KEY",
		BaseURLOverride:  true,
		New:              newOpenAICompatibleProvider,
	},
	{
		Name:             "xai",
		Description:      "OpenAI-compatible xAI",
		DefaultBaseURL:   "https://api.x.ai/v1",
		DefaultAPIKeyEnv: "XAI_API_KEY",
		BaseURLOverride:  true,
		New:              newOpenAICompatibleProvider,
	},
	{
		Name:             "qwen",
		Description:      "OpenAI-compatible DashScope",
		DefaultBaseURL:   "https://dashscope.aliyuncs.com/compatible-mode/v1",
		DefaultAPIKeyEnv: "DASHSCOPE_API_KEY",
		BaseURLOverride:  true,
		New:              newOpenAICompatibleProvider,
	},
	{
		Name:             "anthropic",
		Description:      "Anthropic SDK",
		DefaultAPIKeyEnv: "ANTHROPIC_API_KEY",
		BaseURLOverride:  true,
		New:              newAnthropicProvider,
	},
	{
		Name:             "claude",
		Description:      "Alias for anthropic",
		DefaultAPIKeyEnv: "ANTHROPIC_API_KEY",
		BaseURLOverride:  true,
		New:              newAnthropicProvider,
	},
	{
		Name:             "gemini",
		Description:      "Google GenAI SDK",
		DefaultAPIKeyEnv: "GEMINI_API_KEY",
		New:              newGeminiProvider,
	},
}

var providerPlugins = providerPluginMap(orderedProviderPlugins)

func providerPluginMap(plugins []ProviderPlugin) map[string]ProviderPlugin {
	mapped := make(map[string]ProviderPlugin, len(plugins))
	for _, plugin := range plugins {
		mapped[plugin.Name] = plugin
	}
	return mapped
}

func providerNames() []string {
	names := make([]string, 0, len(orderedProviderPlugins))
	for _, plugin := range orderedProviderPlugins {
		names = append(names, plugin.Name)
	}
	return names
}

func providerNamesText() string {
	return strings.Join(providerNames(), ", ")
}

func providerHelpText() string {
	width := 0
	for _, plugin := range orderedProviderPlugins {
		width = max(width, len(plugin.Name))
	}
	lines := make([]string, 0, len(orderedProviderPlugins))
	for _, plugin := range orderedProviderPlugins {
		parts := []string{plugin.Description, "default " + plugin.DefaultAPIKeyEnv}
		if plugin.DefaultBaseURL != "" {
			parts = append(parts, plugin.DefaultBaseURL)
		}
		lines = append(lines, fmt.Sprintf("  %-*s  %s", width, plugin.Name, strings.Join(parts, ", ")))
	}
	return strings.Join(lines, "\n")
}

func providersSupportingBaseURLText() string {
	var names []string
	for _, plugin := range orderedProviderPlugins {
		if plugin.BaseURLOverride {
			names = append(names, plugin.Name)
		}
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}

func newProvider(cfg Config) (Provider, error) {
	plugin, err := resolveProviderPlugin(cfg.Provider)
	if err != nil {
		return nil, err
	}
	apiKeyEnv := cfg.APIKeyEnv
	if apiKeyEnv == "" {
		apiKeyEnv = plugin.DefaultAPIKeyEnv
	}
	if apiKeyEnv == "" {
		return nil, fmt.Errorf("provider %q does not define a default API key environment variable", plugin.Name)
	}
	apiKey := os.Getenv(apiKeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s is not set", apiKeyEnv)
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = plugin.DefaultBaseURL
	}
	return plugin.New(ResolvedProviderConfig{
		Name:    plugin.Name,
		BaseURL: baseURL,
		APIKey:  apiKey,
	})
}

func resolveProviderPlugin(name string) (ProviderPlugin, error) {
	if name == "" {
		name = defaultProviderName
	}
	plugin, ok := providerPlugins[strings.ToLower(name)]
	if !ok {
		return ProviderPlugin{}, fmt.Errorf("unknown provider %q", name)
	}
	return plugin, nil
}

type openAICompatibleProvider struct {
	client openai.Client
}

type openAIProvider struct {
	client openai.Client
}

func newOpenAIProvider(cfg ResolvedProviderConfig) (Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("provider %q requires a base URL", cfg.Name)
	}
	return &openAIProvider{client: newOpenAIClient(cfg)}, nil
}

func newOpenAICompatibleProvider(cfg ResolvedProviderConfig) (Provider, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("provider %q requires a base URL", cfg.Name)
	}
	return &openAICompatibleProvider{
		client: newOpenAIClient(cfg),
	}, nil
}

func newOpenAIClient(cfg ResolvedProviderConfig) openai.Client {
	return openai.NewClient(
		openaioption.WithAPIKey(cfg.APIKey),
		openaioption.WithBaseURL(cfg.BaseURL),
		openaioption.WithRequestTimeout(defaultTimeout),
	)
}

func (p *openAIProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	params := responses.ResponseNewParams{
		Input: responses.ResponseNewParamsInputUnion{OfString: openai.String(req.User)},
		Model: shared.ResponsesModel(req.Model),
		Store: openai.Bool(false),
	}
	if req.System != "" {
		params.Instructions = openai.String(req.System)
	}

	options, err := openAIResponseParamOptions(req.Params)
	if err != nil {
		return CompletionResult{}, err
	}
	options = append(options, openaioption.WithJSONSet("store", false))
	resp, err := p.client.Responses.New(ctx, params, options...)
	if err != nil {
		return CompletionResult{}, err
	}
	output := resp.OutputText()
	if output == "" {
		return CompletionResult{}, errors.New("provider returned no text content")
	}
	return CompletionResult{
		Output:       output,
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}, nil
}

func (p *openAICompatibleProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	messages := []openai.ChatCompletionMessageParamUnion{}
	if req.System != "" {
		messages = append(messages, openai.SystemMessage(req.System))
	}
	messages = append(messages, openai.UserMessage(req.User))

	options := openAIParamOptions(req.Params)
	resp, err := p.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: messages,
		Model:    shared.ChatModel(req.Model),
	}, options...)
	if err != nil {
		return CompletionResult{}, err
	}
	if len(resp.Choices) == 0 {
		return CompletionResult{}, errors.New("provider returned no choices")
	}
	return CompletionResult{
		Output:       resp.Choices[0].Message.Content,
		InputTokens:  int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
	}, nil
}

func openAIParamOptions(params map[string]any) []openaioption.RequestOption {
	if len(params) == 0 {
		return nil
	}
	options := make([]openaioption.RequestOption, 0, len(params))
	for key, value := range params {
		options = append(options, openaioption.WithJSONSet(key, value))
	}
	return options
}

func openAIResponseParamOptions(params map[string]any) ([]openaioption.RequestOption, error) {
	normalized := maps.Clone(params)
	if _, ok := normalized["store"]; ok {
		return nil, errors.New("provider openai fixes store=false; remove the store parameter")
	}
	if _, ok := normalized["seed"]; ok {
		return nil, errors.New("provider openai uses the Responses API, which does not support seed; use provider openai-chat for Chat Completions parameters")
	}

	if _, ok := normalized["max_output_tokens"]; !ok {
		if value, exists := normalized["max_completion_tokens"]; exists {
			normalized["max_output_tokens"] = value
		} else if value, exists := normalized["max_tokens"]; exists {
			normalized["max_output_tokens"] = value
		}
	}
	delete(normalized, "max_completion_tokens")
	delete(normalized, "max_tokens")

	if effort, ok := normalized["reasoning_effort"]; ok {
		if _, exists := normalized["reasoning"]; exists {
			return nil, errors.New("OpenAI parameters must not set both reasoning and reasoning_effort")
		}
		normalized["reasoning"] = map[string]any{"effort": effort}
		delete(normalized, "reasoning_effort")
	}
	return openAIParamOptions(normalized), nil
}

type anthropicProvider struct {
	client anthropic.Client
}

func newAnthropicProvider(cfg ResolvedProviderConfig) (Provider, error) {
	options := []anthropicoption.RequestOption{
		anthropicoption.WithAPIKey(cfg.APIKey),
		anthropicoption.WithRequestTimeout(defaultTimeout),
	}
	if cfg.BaseURL != "" {
		options = append(options, anthropicoption.WithBaseURL(cfg.BaseURL))
	}
	return &anthropicProvider{client: anthropic.NewClient(options...)}, nil
}

func (p *anthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	params := anthropic.MessageNewParams{
		MaxTokens: maxTokens(req.Params, 4096),
		Messages: []anthropic.MessageParam{{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{{
				OfText: &anthropic.TextBlockParam{Text: req.User},
			}},
		}},
		Model: anthropic.Model(req.Model),
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}

	resp, err := p.client.Messages.New(ctx, params, anthropicParamOptions(req.Params)...)
	if err != nil {
		return CompletionResult{}, err
	}
	parts := make([]string, 0, len(resp.Content))
	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	if len(parts) == 0 {
		return CompletionResult{}, errors.New("provider returned no text content")
	}
	return CompletionResult{
		Output:       strings.Join(parts, ""),
		InputTokens:  int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
	}, nil
}

func anthropicParamOptions(params map[string]any) []anthropicoption.RequestOption {
	if len(params) == 0 {
		return nil
	}
	options := make([]anthropicoption.RequestOption, 0, len(params))
	for key, value := range params {
		if key == "max_tokens" || key == "max_output_tokens" || key == "max_completion_tokens" {
			continue
		}
		options = append(options, anthropicoption.WithJSONSet(key, value))
	}
	return options
}

type geminiProvider struct {
	client *genai.Client
}

func newGeminiProvider(cfg ResolvedProviderConfig) (Provider, error) {
	if cfg.BaseURL != "" {
		return nil, errors.New("provider gemini does not support base URL overrides")
	}
	client, err := genai.NewClient(context.Background(), &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &geminiProvider{client: client}, nil
}

func (p *geminiProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResult, error) {
	resp, err := p.client.Models.GenerateContent(
		ctx,
		req.Model,
		[]*genai.Content{genai.NewContentFromText(req.User, genai.RoleUser)},
		geminiConfig(req),
	)
	if err != nil {
		return CompletionResult{}, err
	}
	output := resp.Text()
	if output == "" {
		return CompletionResult{}, errors.New("provider returned no text content")
	}
	result := CompletionResult{Output: output}
	if resp.UsageMetadata != nil {
		result.InputTokens = int(resp.UsageMetadata.PromptTokenCount)
		result.OutputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
	}
	return result, nil
}

func geminiConfig(req CompletionRequest) *genai.GenerateContentConfig {
	cfg := &genai.GenerateContentConfig{}
	if req.System != "" {
		cfg.SystemInstruction = &genai.Content{Parts: []*genai.Part{{Text: req.System}}}
	}
	if value, ok := float32Param(req.Params, "temperature"); ok {
		cfg.Temperature = &value
	}
	if value, ok := float32Param(req.Params, "top_p"); ok {
		cfg.TopP = &value
	}
	if value, ok := float32Param(req.Params, "topP"); ok {
		cfg.TopP = &value
	}
	if value, ok := float32Param(req.Params, "top_k"); ok {
		cfg.TopK = &value
	}
	if value, ok := float32Param(req.Params, "topK"); ok {
		cfg.TopK = &value
	}
	if value, ok := int32Param(req.Params, "max_output_tokens"); ok {
		cfg.MaxOutputTokens = value
	}
	if value, ok := int32Param(req.Params, "maxOutputTokens"); ok {
		cfg.MaxOutputTokens = value
	}
	if value, ok := int32Param(req.Params, "max_tokens"); ok {
		cfg.MaxOutputTokens = value
	}
	return cfg
}

func maxTokens(params map[string]any, fallback int64) int64 {
	for _, key := range []string{"max_tokens", "max_output_tokens", "max_completion_tokens"} {
		if value, ok := int64Param(params, key); ok {
			return value
		}
	}
	return fallback
}

func int64Param(params map[string]any, key string) (int64, bool) {
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case int32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case float32:
		return int64(typed), true
	default:
		return 0, false
	}
}

func int32Param(params map[string]any, key string) (int32, bool) {
	value, ok := int64Param(params, key)
	return int32(value), ok
}

func float32Param(params map[string]any, key string) (float32, bool) {
	value, ok := params[key]
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return float32(typed), true
	case float32:
		return typed, true
	case int:
		return float32(typed), true
	case int64:
		return float32(typed), true
	case int32:
		return float32(typed), true
	default:
		return 0, false
	}
}
