package skilpel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	exitOK      = 0
	exitGate    = 1
	exitRuntime = 2
)

func Main(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 1 && (args[0] == "version" || args[0] == "-v") {
		fmt.Fprintf(stdout, "skilpel %s\n", Version())
		return exitOK, nil
	}

	var outputFormat string
	var emitAnnotations bool
	exitCode := exitOK

	rootCmd := &cobra.Command{
		Use:           "skilpel",
		Short:         "Evaluate Codex-style skills",
		Long:          "skilpel evaluates Codex-style skills by running evals with and without the skill, then judging whether the skill improved the result.",
		Version:       Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	rootCmd.SetVersionTemplate("skilpel {{.Version}}\n")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text", "output format: text, json, or markdown")
	rootCmd.PersistentFlags().BoolVar(&emitAnnotations, "emit-annotations", false, "emit GitHub Actions workflow command annotations (::error) alongside normal output")

	runCmd := &cobra.Command{
		Use:   "run [skill-relpath ...]",
		Short: "Run skill evals",
		Long:  runHelpText(),
		RunE: func(cmd *cobra.Command, skillArgs []string) error {
			cfg, err := configFromRunCommand(cmd, skillArgs)
			if err != nil {
				return err
			}
			if cmd.Root().PersistentFlags().Changed("output") {
				cfg.Output = outputFormat
			}
			if err := validateConfig(cfg); err != nil {
				return err
			}
			logger, closeLogger, err := openProgressLogger(stderr, cfg)
			if err != nil {
				return err
			}
			defer closeLogger()
			cfg.Logger = logger

			summary, gatePassed, err := Run(ctx, cfg)
			if err != nil {
				return err
			}
			cfg.Logger.InfoContext(ctx, "skilpel run completed",
				slog.String("event", "run_completed"),
				slog.Int("passed", summary.Passed),
				slog.Int("failed", summary.Failed),
				slog.Bool("gate_passed", gatePassed),
				slog.Int("gate_failures", len(summary.GateFailures)),
			)
			if emitAnnotations {
				writeAnnotations(stderr, summary)
			}
			if err := writeSummary(stdout, summary, cfg.Output); err != nil {
				return err
			}
			if !gatePassed {
				exitCode = exitGate
			}
			return nil
		},
	}
	addRunFlags(runCmd)
	rootCmd.AddCommand(runCmd)

	rootCmd.SetArgs(args)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	if err := rootCmd.Execute(); err != nil {
		return exitRuntime, err
	}
	return exitCode, nil
}

func addRunFlags(cmd *cobra.Command) {
	addConfigFlags(cmd.Flags())
}

func addConfigFlags(fs *pflag.FlagSet) {
	fs.StringP("config", "c", "", "YAML or JSON config path")
	fs.String("root", "", "skills root directory")
	fs.String("workspace", "", "workspace for JSON artifacts")
	fs.Bool("baseline", false, "run without_skill baseline")
	fs.Bool("no-baseline", false, "disable without_skill baseline")
	fs.String("provider", "", "provider plugin: "+providerNamesText())
	fs.String("target", "", "target model")
	fs.String("judge", "", "judge model")
	fs.String("base-url", "", "provider base URL override")
	fs.String("api-key-env", "", "environment variable containing the API key")
	fs.Float64("min-pass", -1, "minimum with_skill pass rate")
	fs.Float64("min-delta", -1, "minimum with_skill minus without_skill pass-rate delta")
	fs.String("log-format", "", "progress log format: auto, json, or pretty")
	fs.String("log-file", "", "write structured JSON progress logs to this file")
	fs.StringArray("skill", nil, "skill relpath to include; repeatable")
	fs.StringArray("eval-id", nil, "eval id to include; repeatable")
}

func configFromRunCommand(cmd *cobra.Command, skillArgs []string) (Config, error) {
	return configFromFlagSet(cmd.Flags(), skillArgs)
}

func configFromFlagSet(fs *pflag.FlagSet, skillArgs []string) (Config, error) {
	configPath, err := fs.GetString("config")
	if err != nil {
		return Config{}, err
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		return Config{}, err
	}

	if value, _ := fs.GetString("root"); value != "" {
		cfg.Root = value
	}
	if value, _ := fs.GetString("workspace"); value != "" {
		cfg.Workspace = value
	}
	if fs.Changed("baseline") {
		value, _ := fs.GetBool("baseline")
		cfg.Baseline = value
	}
	if value, _ := fs.GetBool("no-baseline"); fs.Changed("no-baseline") && value {
		cfg.Baseline = false
	}
	if value, _ := fs.GetString("provider"); value != "" {
		cfg.Provider = value
	}
	if value, _ := fs.GetString("target"); value != "" {
		cfg.Target = value
	}
	if value, _ := fs.GetString("judge"); value != "" {
		cfg.Judge = value
	}
	if value, _ := fs.GetString("base-url"); value != "" {
		cfg.BaseURL = value
	}
	if value, _ := fs.GetString("api-key-env"); value != "" {
		cfg.APIKeyEnv = value
	}
	if value, _ := fs.GetFloat64("min-pass"); value >= 0 {
		cfg.MinPass = value
	}
	if value, _ := fs.GetFloat64("min-delta"); value >= 0 {
		cfg.MinDelta = value
	}
	if value, _ := fs.GetString("log-format"); value != "" {
		cfg.LogFormat = value
	}
	if value, _ := fs.GetString("log-file"); value != "" {
		cfg.LogFile = value
	}
	if values, _ := fs.GetStringArray("skill"); len(values) > 0 {
		cfg.Skills = values
	}
	if values, _ := fs.GetStringArray("eval-id"); len(values) > 0 {
		cfg.EvalIDs = values
	}
	if len(skillArgs) > 0 {
		cfg.Skills = append(cfg.Skills, skillArgs...)
	}
	if cfg.Judge == "" {
		cfg.Judge = cfg.Target
	}
	return cfg, nil
}

func openProgressLogger(stderr io.Writer, cfg Config) (*slog.Logger, func(), error) {
	visible := progressHandler(stderr, cfg.LogFormat)
	if cfg.LogFile == "" {
		return slog.New(visible), func() {}, nil
	}

	if dir := filepath.Dir(cfg.LogFile); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, func() {}, fmt.Errorf("create log file directory: %w", err)
		}
	}
	file, err := os.Create(cfg.LogFile)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create log file: %w", err)
	}
	closeLogger := func() {
		_ = file.Close()
	}
	return slog.New(multiHandler{visible, structuredHandler(file)}), closeLogger, nil
}

func writeUsage(w io.Writer) {
	defaultProvider := providerPlugins[defaultProviderName]
	fmt.Fprintf(
		w,
		helpTemplate,
		defaultProvider.DefaultAPIKeyEnv,
		defaultProviderName,
		defaultProviderName,
		providerHelpText(),
		providersSupportingBaseURLText(),
	)
	fmt.Fprintln(w)
}

func runHelpText() string {
	var b strings.Builder
	writeUsage(&b)
	return b.String()
}

const helpTemplate = `skilpel evaluates Codex-style skills by running each eval with and without
the skill, then judging whether the skill improved the result.

Usage:
  skilpel run [options] [skill-relpath ...]
  skilpel version
  skilpel help
  skilpel --help

Typical run:
  %s=... skilpel run \
    --root ./skills \
    --skill shell-script \
    --eval-id new-script-strict-mode \
    --workspace ./.skilpel \
    --provider %s \
    --target gpt-4o-mini \
    --judge gpt-4o-mini \
    --baseline \
    --min-pass 0.90 \
    --min-delta 0.20

Config file:
  skilpel run --config skilpel.yaml

  root: ./skills
  workspace: ./.skilpel
  baseline: true
  provider: %s
  target: gpt-4o-mini
  judge: gpt-4o-mini
  minPass: 0.90
  minDelta: 0.20

Providers:
%s

Use --api-key-env to override the environment variable. Use --base-url to
override endpoints for these providers:
  %s
Gemini uses the SDK's Gemini API backend and does not accept --base-url.

Eval files:
  skilpel looks beside each skill for evals/evals.yaml, then evals/evals.yml,
  then evals/evals.json. YAML and JSON use the same structure.

  skill_name: shell-script
  evals:
    - id: new-script-strict-mode
      prompt: Write a Bash script that prints the current Git branch.
      assertions:
        - Starts with a Bash shebang.
        - Enables strict mode with set -euo pipefail.

Run modes:
  with_skill     The skill body is sent as the system instruction.
  without_skill  The same user prompt is sent without the skill when baseline
                 is enabled.

Gates:
  --min-pass sets the minimum pass rate for with_skill results.
  --min-delta sets the minimum pass-rate improvement over without_skill.
  --no-baseline disables without_skill runs and baseline-delta gates.

Artifacts:
  The workspace receives summary.json plus per-skill and per-eval JSON artifacts
  containing prompts, outputs, timing, grading, and gate details.

Logs:
  During runs, skilpel writes progress logs to stderr. Use --log-format=json for
  GCP-readable JSON lines, --log-format=pretty for terminal progress, or
  --log-format=auto to choose pretty only when stderr is an interactive
  terminal. Use --log-file to also write structured JSON progress logs to a file
  without printing those JSON lines in the terminal or CI step log.

Output:
  The default --output=text follows skill-validator's human-readable terminal
  report style. Use --output=json for scripts and CI artifacts, or
  --output=markdown for Markdown release notes and issue comments.

Exit codes:
  0  The run completed and all configured gates passed.
  1  Evals ran, but assertions or gates failed.
  2  Usage, configuration, filesystem, provider, or runtime failure.`
