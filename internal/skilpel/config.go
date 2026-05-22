package skilpel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func defaultConfig() Config {
	return Config{
		Root:      ".",
		Workspace: filepath.Join(".", ".skilpel"),
		Baseline:  true,
		Target:    "gpt-4o-mini",
		APIKeyEnv: "OPENAI_API_KEY",
		BaseURL:   "https://api.openai.com/v1",
		MinPass:   0.90,
		MinDelta:  0.20,
	}
}

func loadConfig(path string) (Config, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse JSON config: %w", err)
		}
	default:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse YAML config: %w", err)
		}
	}

	if cfg.Judge == "" {
		cfg.Judge = cfg.Target
	}
	return cfg, nil
}

func validateConfig(cfg Config) error {
	if cfg.Root == "" {
		return errors.New("root is required")
	}
	if cfg.Workspace == "" {
		return errors.New("workspace is required")
	}
	if cfg.Target == "" {
		return errors.New("target model is required")
	}
	if cfg.Judge == "" {
		return errors.New("judge model is required")
	}
	if cfg.BaseURL == "" {
		return errors.New("base URL is required")
	}
	if cfg.APIKeyEnv == "" {
		return errors.New("api key env is required")
	}
	if cfg.MinPass < 0 || cfg.MinPass > 1 {
		return errors.New("min pass must be between 0 and 1")
	}
	if cfg.MinDelta < 0 || cfg.MinDelta > 1 {
		return errors.New("min delta must be between 0 and 1")
	}
	return nil
}
