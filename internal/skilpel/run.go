package skilpel

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func Run(ctx context.Context, cfg Config) (Summary, bool, error) {
	provider, err := newProvider(cfg)
	if err != nil {
		return Summary{}, false, err
	}
	return RunWithProvider(ctx, cfg, provider)
}

func RunWithProvider(ctx context.Context, cfg Config, provider Provider) (Summary, bool, error) {
	started := time.Now().UTC()
	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return Summary{}, false, err
	}
	workspace, err := filepath.Abs(cfg.Workspace)
	if err != nil {
		return Summary{}, false, err
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return Summary{}, false, fmt.Errorf("create workspace: %w", err)
	}

	skills, err := discoverSkills(root, cfg.Skills, cfg.EvalIDs)
	if err != nil {
		return Summary{}, false, err
	}
	totalEvals := 0
	for _, skill := range skills {
		totalEvals += len(skill.Evals)
	}
	if cfg.Logger != nil {
		cfg.Logger.InfoContext(ctx, "skilpel run started",
			slog.String("event", "run_started"),
			slog.Int("skills", len(skills)),
			slog.Int("evals", totalEvals),
			slog.String("workspace", workspace),
			slog.Bool("baseline", cfg.Baseline),
			slog.Float64("min_pass", cfg.MinPass),
			slog.Float64("min_delta", cfg.MinDelta),
			slog.String("provider", cfg.Provider),
			slog.String("target", cfg.Target),
			slog.String("judge", cfg.Judge),
		)
	}

	summary := Summary{
		Workspace: workspace,
		StartedAt: started,
		Gates:     GateSummary{MinPass: cfg.MinPass, MinDelta: cfg.MinDelta, Baseline: cfg.Baseline},
	}

	for _, skill := range skills {
		skillSummary, err := runSkill(ctx, cfg, provider, workspace, skill)
		if err != nil {
			return Summary{}, false, err
		}
		summary.Skills = append(summary.Skills, skillSummary)
		summary.Passed += skillSummary.Passed
		summary.Failed += skillSummary.Failed
	}

	summary.GateFailures = gateFailures(summary.Skills, cfg)
	summary.Gates.Passed = len(summary.GateFailures) == 0
	summary.CompletedAt = time.Now().UTC()
	if err := writeJSON(filepath.Join(workspace, "summary.json"), summary); err != nil {
		return Summary{}, false, fmt.Errorf("write summary: %w", err)
	}
	return summary, summary.Gates.Passed, nil
}

func runSkill(ctx context.Context, cfg Config, provider Provider, workspace string, skill Skill) (SkillSummary, error) {
	var withRateTotal float64
	var withoutRateTotal float64
	var withRateCount int
	var withoutRateCount int
	var passed int
	var failed int

	for index, eval := range skill.Evals {
		result, err := runEval(ctx, cfg, provider, workspace, skill, eval, index)
		if err != nil {
			return SkillSummary{}, err
		}
		withSummary := result.ModeResults[WithSkill].Grading.Summary
		withRateTotal += withSummary.PassRate
		withRateCount++
		passed += withSummary.Passed
		failed += withSummary.Failed
		if cfg.Baseline {
			withoutRateTotal += result.ModeResults[WithoutSkill].Grading.Summary.PassRate
			withoutRateCount++
		}
	}

	with := meanFromTotal(withRateTotal, withRateCount)
	without := meanFromTotal(withoutRateTotal, withoutRateCount)
	summary := SkillSummary{
		Skill:            skill.Name,
		RelPath:          skill.RelPath,
		Evals:            len(skill.Evals),
		Passed:           passed,
		Failed:           failed,
		WithSkillPass:    with,
		WithoutSkillPass: without,
		Delta:            with - without,
		BenchmarkPath:    benchmarkPath(workspace, skill),
	}
	if err := writeJSON(summary.BenchmarkPath, summary); err != nil {
		return SkillSummary{}, fmt.Errorf("write benchmark: %w", err)
	}
	return summary, nil
}

func runEval(ctx context.Context, cfg Config, provider Provider, workspace string, skill Skill, eval EvalCase, index int) (RunResult, error) {
	eSlug := evalSlug(eval, index)
	modes := []Mode{WithSkill}
	if cfg.Baseline {
		modes = append(modes, WithoutSkill)
	}

	result := RunResult{
		Skill:       skill.Name,
		SkillPath:   skill.RelPath,
		EvalID:      eval.ID,
		EvalName:    eval.Name,
		EvalSlug:    eSlug,
		ModeResults: map[Mode]ModeRun{},
	}

	for _, mode := range modes {
		modeRun, err := runMode(ctx, cfg, provider, skill, eval, mode)
		if err != nil {
			return RunResult{}, err
		}
		artifact, err := writeModeArtifacts(workspace, skill, eval, eSlug, mode, modeRun)
		if err != nil {
			return RunResult{}, err
		}
		modeRun.Artifact = artifact
		result.ModeResults[mode] = modeRun
	}

	if err := writeJSON(filepath.Join(workspace, slugify(skill.RelPath, slugify(skill.Name, "skill")), eSlug, "result.json"), result); err != nil {
		return RunResult{}, fmt.Errorf("write eval result: %w", err)
	}
	emitEvalCompletedLog(ctx, cfg.Logger, workspace, skill, eval, eSlug, result)
	return result, nil
}

func runMode(ctx context.Context, cfg Config, provider Provider, skill Skill, eval EvalCase, mode Mode) (ModeRun, error) {
	user, err := userPrompt(skill, eval)
	if err != nil {
		return ModeRun{}, err
	}
	system := ""
	if mode == WithSkill {
		system = skillSystemPrompt(skill)
	}

	start := time.Now()
	completion, err := provider.Complete(ctx, CompletionRequest{
		Model:  cfg.Target,
		System: system,
		User:   user,
		Params: mergeParams(cfg.TargetParams, eval.Params),
	})
	if err != nil {
		return ModeRun{}, err
	}
	duration := time.Since(start)

	grading, judgePrompt, err := grade(ctx, provider, cfg.Judge, eval, completion.Output, cfg.JudgeParams)
	if err != nil {
		return ModeRun{}, err
	}

	return ModeRun{
		Output: completion.Output,
		Timing: Timing{
			DurationMS:  duration.Milliseconds(),
			TotalTokens: completion.InputTokens + completion.OutputTokens,
		},
		Grading: grading,
		Prompt:  PromptRecord{System: system, User: user, Judge: judgePrompt},
	}, nil
}

func mergeParams(base, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func meanFromTotal(total float64, count int) float64 {
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

func gateFailures(skills []SkillSummary, cfg Config) []string {
	var failures []string
	for _, skill := range skills {
		if skill.Failed > 0 {
			failures = append(failures, fmt.Sprintf("%s has %d failed with_skill assertions", skill.RelPath, skill.Failed))
		}
		if skill.WithSkillPass < cfg.MinPass {
			failures = append(failures, fmt.Sprintf("%s with_skill pass rate %.3f < %.3f", skill.RelPath, skill.WithSkillPass, cfg.MinPass))
		}
		if cfg.Baseline && skill.Delta < cfg.MinDelta {
			failures = append(failures, fmt.Sprintf("%s baseline delta %.3f < %.3f", skill.RelPath, skill.Delta, cfg.MinDelta))
		}
	}
	return failures
}

func emitEvalCompletedLog(ctx context.Context, logger *slog.Logger, workspace string, skill Skill, eval EvalCase, evalSlug string, result RunResult) {
	if logger == nil {
		return
	}
	withSummary := result.ModeResults[WithSkill].Grading.Summary
	attrs := []slog.Attr{
		slog.String("event", "eval_completed"),
		slog.String("skill", skill.Name),
		slog.String("rel_path", skill.RelPath),
		slog.String("eval_id", eval.ID),
		slog.String("eval_name", eval.Name),
		slog.String("eval_slug", evalSlug),
		slog.Int("passed", withSummary.Passed),
		slog.Int("failed", withSummary.Failed),
		slog.Int("total", withSummary.Total),
		slog.Float64("with_skill_pass_rate", withSummary.PassRate),
		slog.String("result_path", filepath.Join(workspace, slugify(skill.RelPath, slugify(skill.Name, "skill")), evalSlug, "result.json")),
	}
	if withoutRun, ok := result.ModeResults[WithoutSkill]; ok {
		withoutRate := withoutRun.Grading.Summary.PassRate
		attrs = append(attrs,
			slog.Float64("without_skill_pass_rate", withoutRate),
			slog.Float64("delta", withSummary.PassRate-withoutRate),
		)
	}
	logger.LogAttrs(ctx, slog.LevelInfo, "skilpel eval completed", attrs...)
}
