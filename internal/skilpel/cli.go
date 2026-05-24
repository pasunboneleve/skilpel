package skilpel

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
)

const (
	exitOK      = 0
	exitGate    = 1
	exitRuntime = 2
)

type repeated []string

func (r *repeated) String() string { return strings.Join(*r, ",") }
func (r *repeated) Set(v string) error {
	*r = append(*r, v)
	return nil
}

func Main(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if wantsHelp(args) {
		writeUsage(stdout)
		return exitOK, nil
	}
	if args[0] != "run" {
		writeUsage(stderr)
		return exitRuntime, fmt.Errorf("unknown command %q", args[0])
	}

	cfg, err := parseRunArgs(args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			writeUsage(stdout)
			return exitOK, nil
		}
		return exitRuntime, err
	}
	if err := validateConfig(cfg); err != nil {
		return exitRuntime, err
	}

	summary, gatePassed, err := Run(ctx, cfg)
	if err != nil {
		return exitRuntime, err
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		return exitRuntime, fmt.Errorf("write summary: %w", err)
	}
	if !gatePassed {
		return exitGate, nil
	}
	return exitOK, nil
}

func parseRunArgs(args []string) (Config, error) {
	var configPath string
	var skills repeated
	var evalIDs repeated

	fs := flag.NewFlagSet("skilpel run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&configPath, "config", "", "YAML or JSON config path")
	fs.StringVar(&configPath, "c", "", "YAML or JSON config path")
	root := fs.String("root", "", "skills root directory")
	workspace := fs.String("workspace", "", "workspace for JSON artifacts")
	baseline := fs.Bool("baseline", false, "run without_skill baseline")
	noBaseline := fs.Bool("no-baseline", false, "disable without_skill baseline")
	provider := fs.String("provider", "", "provider plugin: "+providerNamesText())
	target := fs.String("target", "", "target model")
	judge := fs.String("judge", "", "judge model")
	baseURL := fs.String("base-url", "", "provider base URL override")
	apiKeyEnv := fs.String("api-key-env", "", "environment variable containing the API key")
	minPass := fs.Float64("min-pass", -1, "minimum with_skill pass rate")
	minDelta := fs.Float64("min-delta", -1, "minimum with_skill minus without_skill pass-rate delta")
	fs.Var(&skills, "skill", "skill relpath to include; repeatable")
	fs.Var(&evalIDs, "eval-id", "eval id to include; repeatable")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	setFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})

	cfg, err := loadConfig(configPath)
	if err != nil {
		return Config{}, err
	}

	if *root != "" {
		cfg.Root = *root
	}
	if *workspace != "" {
		cfg.Workspace = *workspace
	}
	if setFlags["baseline"] {
		cfg.Baseline = *baseline
	}
	if setFlags["no-baseline"] && *noBaseline {
		cfg.Baseline = false
	}
	if *provider != "" {
		cfg.Provider = *provider
	}
	if *target != "" {
		cfg.Target = *target
	}
	if *judge != "" {
		cfg.Judge = *judge
	}
	if *baseURL != "" {
		cfg.BaseURL = *baseURL
	}
	if *apiKeyEnv != "" {
		cfg.APIKeyEnv = *apiKeyEnv
	}
	if *minPass >= 0 {
		cfg.MinPass = *minPass
	}
	if *minDelta >= 0 {
		cfg.MinDelta = *minDelta
	}
	if len(skills) > 0 {
		cfg.Skills = skills
	}
	if len(evalIDs) > 0 {
		cfg.EvalIDs = evalIDs
	}
	if cfg.Judge == "" {
		cfg.Judge = cfg.Target
	}
	if extra := fs.Args(); len(extra) > 0 {
		cfg.Skills = append(cfg.Skills, extra...)
	}
	return cfg, nil
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
		providerNamesText(),
	)
	fmt.Fprintln(w)
}

func wantsHelp(args []string) bool {
	if len(args) == 0 {
		return true
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		return true
	}
	return false
}

const helpTemplate = `skilpel evaluates Codex-style skills by running each eval with and without
the skill, then judging whether the skill improved the result.

Usage:
  skilpel run [options] [skill-relpath ...]
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

Exit codes:
  0  The run completed and all configured gates passed.
  1  Evals ran, but assertions or gates failed.
  2  Usage, configuration, filesystem, provider, or runtime failure.

Options:
  --config <path>       YAML or JSON config path
  --root <path>         skills root directory
  --workspace <path>    workspace for JSON artifacts
  --skill <relpath>     skill relpath to include; repeatable
  --eval-id <id>        eval id to include; repeatable
  --baseline            run without_skill baseline (default true)
  --no-baseline         disable baseline
  --provider <name>     provider: %s
  --target <model>      target model
  --judge <model>       judge model
  --base-url <url>      provider base URL override
  --api-key-env <name>  environment variable containing the API key
  --min-pass <rate>     minimum with_skill pass rate
  --min-delta <rate>    minimum with_skill minus without_skill pass-rate delta`
