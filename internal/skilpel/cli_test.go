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
	if !strings.Contains(stdout.String(), "skilpel run [options]") {
		t.Fatalf("expected usage on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestMainNoArgsPrintsAgentHelpAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected ok exit, got %d", code)
	}
	help := stdout.String()
	for _, want := range []string{
		"Typical run:",
		"Providers:",
		"Eval files:",
		"Gates:",
		"Artifacts:",
		"Logs:",
		"Exit codes:",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("expected help to include %q, got %q", want, help)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestMainHelpSubcommandPrintsUsageAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), []string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected ok exit, got %d", code)
	}
	if !strings.Contains(stdout.String(), "skilpel run [options]") {
		t.Fatalf("expected command usage on stdout, got %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestMainVersionPrintsVersionAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), []string{"version"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected ok exit, got %d", code)
	}
	if got := strings.TrimSpace(stdout.String()); got != "skilpel "+Version() {
		t.Fatalf("version output = %q, want skilpel %s", got, Version())
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
		"--log-format", "pretty",
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
	if cfg.LogFormat != "pretty" {
		t.Fatalf("unexpected log format: %v", cfg.LogFormat)
	}
}

func TestValidateConfigRejectsUnknownLogFormat(t *testing.T) {
	cfg := defaultConfig()
	cfg.Judge = cfg.Target
	cfg.LogFormat = "loud"

	err := validateConfig(cfg)
	if err == nil || !strings.Contains(err.Error(), "log format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrettyProgressLoggerWritesHumanReadableProgress(t *testing.T) {
	var logs bytes.Buffer
	logger := progressLogger(&logs, "pretty")

	logger.InfoContext(context.Background(), "skilpel run started",
		"event", "run_started",
		"skills", 2,
		"evals", 3,
		"provider", "openai",
		"target", "target-model",
		"judge", "judge-model",
		"baseline", true,
	)
	logger.InfoContext(context.Background(), "skilpel eval completed",
		"event", "eval_completed",
		"rel_path", "demo-skill",
		"eval_id", "case-a",
		"passed", 3,
		"failed", 0,
		"total", 3,
		"with_skill_pass_rate", 1.0,
		"without_skill_pass_rate", 0.3333333333333333,
		"delta", 0.6666666666666667,
	)

	got := logs.String()
	for _, want := range []string{
		"skilpel: 2 skills, 3 evals",
		"provider=openai target=target-model judge=judge-model baseline=true",
		"PASS [1/3] demo-skill | case-a | assertions=3/3 | with=100% baseline=33% delta=67%",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected pretty logs to include %q, got %q", want, got)
		}
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
