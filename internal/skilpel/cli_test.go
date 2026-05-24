package skilpel

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), []string{"unknown"}, &stdout, &stderr)
	if code != exitRuntime {
		t.Fatalf("expected runtime exit, got %d", code)
	}
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMainRunHelpPrintsUsageAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), []string{"run", "--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected ok exit, got %d", code)
	}
	if !strings.Contains(stdout.String(), "usage: skilpel run") {
		t.Fatalf("expected usage on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestParseRunArgsAppliesRepeatedFilters(t *testing.T) {
	cfg, err := parseRunArgs([]string{
		"--root", "skills",
		"--workspace", "work",
		"--skill", "a",
		"--skill", "b",
		"--eval-id", "one",
		"--eval-id", "2",
		"--no-baseline",
		"--min-pass", "0.8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Baseline {
		t.Fatal("expected baseline disabled")
	}
	if got := strings.Join(cfg.Skills, ","); got != "a,b" {
		t.Fatalf("unexpected skills: %s", got)
	}
	if got := strings.Join(cfg.EvalIDs, ","); got != "one,2" {
		t.Fatalf("unexpected eval ids: %s", got)
	}
	if cfg.MinPass != 0.8 {
		t.Fatalf("unexpected min pass: %v", cfg.MinPass)
	}
}

func TestParseRunArgsDoesNotOverrideConfigBaselineUnlessFlagIsSet(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skilpel.yaml")
	if err := os.WriteFile(configPath, []byte("baseline: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseRunArgs([]string{"--config", configPath})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Baseline {
		t.Fatal("expected config baseline false to be preserved")
	}

	cfg, err = parseRunArgs([]string{"--config", configPath, "--baseline"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Baseline {
		t.Fatal("expected explicit --baseline to override config")
	}
}
