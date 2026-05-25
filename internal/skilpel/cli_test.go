package skilpel

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
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
		"Usage:",
		"Available Commands:",
		"run",
		"--output",
		"--emit-annotations",
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
	for _, want := range []string{"Available Commands:", "--output", "--emit-annotations"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected root usage to include %q, got %q", want, stdout.String())
		}
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
	cfg, err := parseRunArgsForTest(t, []string{
		"--root", "skills",
		"--workspace", "work",
		"--skill", "a",
		"--skill", "b",
		"--eval-id", "one",
		"--eval-id", "2",
		"--no-baseline",
		"--min-pass", "0.8",
		"--log-format", "pretty",
		"--log-file", "work/progress.ndjson",
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
	if cfg.LogFile != "work/progress.ndjson" {
		t.Fatalf("unexpected log file: %v", cfg.LogFile)
	}
}

func TestMainRunHelpPrintsAgentRunHelpAndExitsOK(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, err := Main(context.Background(), []string{"run", "--help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != exitOK {
		t.Fatalf("expected ok exit, got %d", code)
	}
	for _, want := range []string{
		"Typical run:",
		"Providers:",
		"Eval files:",
		"Gates:",
		"Artifacts:",
		"Logs:",
		"Output:",
		"Exit codes:",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("expected run help to include %q, got %q", want, stdout.String())
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
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
		"root", "/tmp/skills",
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
		"Validating skills: /tmp/skills",
		"Configuration",
		"ℹ 2 skills, 3 evals",
		"provider=openai target=target-model judge=judge-model without_skill=true",
		"Evals",
		"✓ [1/3] demo-skill / case-a:",
		"3 passed, 0 failed",
		"with:",
		"100%",
		"without:",
		"33%",
		"delta:",
		"67%",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected pretty logs to include %q, got %q", want, got)
		}
	}
}

func TestProgressLoggerHandlesTypedNilFileAsJSON(t *testing.T) {
	var file *os.File

	logger := progressLogger(file, "auto")

	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestPrettyProgressLoggerPreservesWithAttrs(t *testing.T) {
	var logs bytes.Buffer
	logger := progressLogger(&logs, "pretty").With("rel_path", "demo-skill")

	logger.InfoContext(context.Background(), "skilpel eval completed",
		"event", "eval_completed",
		"eval_id", "case-a",
		"passed", 1,
		"failed", 0,
		"total", 1,
		"with_skill_pass_rate", 1.0,
	)

	if got := logs.String(); !strings.Contains(got, "✓ [1/0] demo-skill / case-a:") {
		t.Fatalf("expected With attrs in pretty log, got %q", got)
	}
}

func TestOpenProgressLoggerKeepsPrettyVisibleAndStructuredFile(t *testing.T) {
	var visible bytes.Buffer
	logFile := filepath.Join(t.TempDir(), "logs", "progress.ndjson")

	logger, closeLogger, err := openProgressLogger(&visible, Config{
		LogFormat: "pretty",
		LogFile:   logFile,
	})
	if err != nil {
		t.Fatal(err)
	}
	logger.InfoContext(context.Background(), "skilpel run started",
		"event", "run_started",
		"skills", 1,
		"evals", 1,
		"root", "/tmp/skills",
		"provider", "openai",
		"target", "target-model",
		"judge", "judge-model",
		"baseline", true,
	)
	closeLogger()

	if got := visible.String(); !strings.Contains(got, "Validating skills: /tmp/skills") {
		t.Fatalf("expected visible pretty log, got %q", got)
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`"event":"run_started"`,
		`"severity":"INFO"`,
		`"message":"skilpel run started"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected structured log to include %q, got %q", want, got)
		}
	}
}

func TestOpenProgressLoggerReportsLogFileErrors(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "existing-dir")
	if err := os.Mkdir(logFile, 0o755); err != nil {
		t.Fatal(err)
	}

	_, _, err := openProgressLogger(io.Discard, Config{
		LogFormat: "pretty",
		LogFile:   logFile,
	})
	if err == nil || !strings.Contains(err.Error(), "create log file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultiHandlerWritesRemainingSinksAfterError(t *testing.T) {
	var structured bytes.Buffer
	logger := slog.New(multiHandler{
		structuredHandler(failingWriter{}),
		structuredHandler(&structured),
	})

	err := logger.Handler().Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "message", 0))
	if err == nil {
		t.Fatal("expected joined handler error")
	}
	if got := structured.String(); !strings.Contains(got, `"message":"message"`) {
		t.Fatalf("expected second sink to receive record, got %q", got)
	}
}

func TestParseRunArgsDoesNotOverrideConfigBaselineUnlessFlagIsSet(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "skilpel.yaml")
	if err := os.WriteFile(configPath, []byte("baseline: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseRunArgsForTest(t, []string{"--config", configPath})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Baseline {
		t.Fatal("expected config baseline false to be preserved")
	}

	cfg, err = parseRunArgsForTest(t, []string{"--config", configPath, "--baseline"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Baseline {
		t.Fatal("expected explicit --baseline to override config")
	}
}

func parseRunArgsForTest(t *testing.T, args []string) (Config, error) {
	t.Helper()
	fs := pflag.NewFlagSet("skilpel run", pflag.ContinueOnError)
	addConfigFlags(fs)
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	return configFromFlagSet(fs, fs.Args())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, os.ErrClosed
}
