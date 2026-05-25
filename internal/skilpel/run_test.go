package skilpel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeProvider struct {
	requests []CompletionRequest
}

func (p *fakeProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	p.requests = append(p.requests, req)
	if strings.Contains(req.User, "Return only JSON") {
		passed := !strings.Contains(req.User, "baseline output")
		out := Grading{
			AssertionResults: []AssertionResult{{Text: "assertion", Passed: passed, Evidence: "fake judge"}},
			Summary: GradeSummary{
				Passed:   boolInt(passed),
				Failed:   boolInt(!passed),
				Total:    1,
				PassRate: float64(boolInt(passed)),
			},
		}
		data, _ := json.Marshal(out)
		return CompletionResult{Output: string(data), InputTokens: 1, OutputTokens: 1}, nil
	}
	if req.System == "" {
		return CompletionResult{Output: "baseline output", InputTokens: 2, OutputTokens: 3}, nil
	}
	return CompletionResult{Output: "skill output", InputTokens: 2, OutputTokens: 3}, nil
}

type partialProvider struct{}

func (p partialProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResult, error) {
	if strings.Contains(req.User, "Return only JSON") {
		out := "```json\n" + `{
  "assertion_results": [
    {"text": "one", "passed": true, "evidence": "ok"},
    {"text": "two", "passed": false, "evidence": "missing"}
  ],
  "summary": {"passed": 1, "failed": 1, "total": 2, "pass_rate": 0.5}
}` + "\n```"
		return CompletionResult{Output: out, InputTokens: 1, OutputTokens: 1}, nil
	}
	return CompletionResult{Output: "partial output", InputTokens: 1, OutputTokens: 1}, nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func writeTestSkill(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, "demo-skill")
	if err := os.MkdirAll(filepath.Join(dir, "evals", "files"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: demo-skill\ndescription: Demo skill.\n---\n\nUse the skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evals", "files", "input.txt"), []byte("attached data"), 0o644); err != nil {
		t.Fatal(err)
	}
	evals := `{
  "skill_name": "demo-skill",
  "evals": [
    {
      "id": "case-a",
      "name": "case a",
      "prompt": "Run case A.",
      "files": ["evals/files/input.txt"],
      "assertions": ["passes"]
    },
    {
      "id": 2,
      "name": "case b",
      "prompt": "Run case B.",
      "assertions": ["passes"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "evals", "evals.json"), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestSkillWithEval(t *testing.T, root, name, evalID string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "evals"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := fmt.Sprintf(`---
name: %s
description: Test skill.
---

Use the skill.
`, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	evals := fmt.Sprintf(`{
  "skill_name": "%s",
  "evals": [
    {
      "id": "%s",
      "name": "%s",
      "prompt": "Run %s.",
      "assertions": ["passes"]
    }
  ]
}`, name, evalID, evalID, evalID)
	if err := os.WriteFile(filepath.Join(dir, "evals", "evals.json"), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestSkillWithoutEvals(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	skill := fmt.Sprintf(`---
name: %s
description: Test skill.
---

Use the skill.
`, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeTestSkillFile(t *testing.T, root, name, evalFile, evals string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "evals"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := fmt.Sprintf(`---
name: %s
description: Test skill.
---

Use the skill.
`, name)
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(skill), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evals", evalFile), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunWithProviderFiltersEvalAndWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	workspace := filepath.Join(t.TempDir(), "workspace")
	provider := &fakeProvider{}

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: workspace,
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"2"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, provider)
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 || summary.Skills[0].Evals != 1 {
		t.Fatalf("unexpected summary: %#v", summary.Skills)
	}
	if len(provider.requests) != 4 {
		t.Fatalf("expected target+judge for with_skill and without_skill, got %d requests", len(provider.requests))
	}
	if !strings.Contains(provider.requests[0].User, "Run case B.") {
		t.Fatalf("expected case B prompt, got %q", provider.requests[0].User)
	}
	if strings.Contains(provider.requests[0].User, "Run case A.") {
		t.Fatal("case A was not filtered out")
	}
	if _, err := os.Stat(filepath.Join(workspace, "demo-skill", "eval-case-b", "with_skill", "grading.json")); err != nil {
		t.Fatalf("missing with_skill grading artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "summary.json")); err != nil {
		t.Fatalf("missing summary artifact: %v", err)
	}
}

func TestRunWithProviderStreamsStructuredEvalLogs(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	workspace := filepath.Join(t.TempDir(), "workspace")
	var logs bytes.Buffer

	_, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: workspace,
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"case-a"},
		MinPass:   0.9,
		MinDelta:  0.2,
		Logger:    structuredLogger(&logs),
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatal("expected gates to pass")
	}

	events := decodeJSONLogEvents(t, logs.String())
	if len(events) != 2 {
		t.Fatalf("expected run_started and one eval_completed event, got %#v", events)
	}
	if events[0]["event"] != "run_started" {
		t.Fatalf("expected first event run_started, got %#v", events[0])
	}
	if events[1]["event"] != "eval_completed" {
		t.Fatalf("expected second event eval_completed, got %#v", events[1])
	}
	if events[1]["skill"] != "demo-skill" || events[1]["eval_id"] != "case-a" {
		t.Fatalf("unexpected eval event identity: %#v", events[1])
	}
	if events[1]["severity"] != "INFO" || events[1]["message"] != "skilpel eval completed" {
		t.Fatalf("expected GCP-readable severity and message fields, got %#v", events[1])
	}
	for _, event := range events {
		switch event["event"] {
		case "run_started", "eval_completed":
		default:
			t.Fatalf("unexpected duplicate or unsupported log event: %#v", event)
		}
	}
}

func decodeJSONLogEvents(t *testing.T, logs string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	events := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("log line is not JSON: %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

func TestRunWithProviderSkipsAutoDiscoveredSkillsWithoutEvalID(t *testing.T) {
	root := t.TempDir()
	writeTestSkillWithEval(t, root, "match-skill", "target-case")
	writeTestSkillWithEval(t, root, "other-skill", "other-case")

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		EvalIDs:   []string{"target-case"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 || summary.Skills[0].RelPath != "match-skill" {
		t.Fatalf("expected only matching skill, got %#v", summary.Skills)
	}
}

func TestRunWithProviderWarnsForAutoDiscoveredSkillsWithoutEvals(t *testing.T) {
	root := t.TempDir()
	writeTestSkillWithEval(t, root, "match-skill", "target-case")
	writeTestSkillWithoutEvals(t, root, "empty-skill")
	var logs bytes.Buffer

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		MinPass:   0.9,
		MinDelta:  0.2,
		Logger:    structuredLogger(&logs),
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Warnings) != 1 || summary.Warnings[0].Skill != "empty-skill" {
		t.Fatalf("expected empty skill warning, got %#v", summary.Warnings)
	}

	events := decodeJSONLogEvents(t, logs.String())
	foundWarning := false
	for _, event := range events {
		if event["event"] == "warning" {
			foundWarning = true
			if event["severity"] != "WARN" || event["skill"] != "empty-skill" {
				t.Fatalf("unexpected warning event: %#v", event)
			}
		}
	}
	if !foundWarning {
		t.Fatalf("expected warning event, got %#v", events)
	}
}

func TestRunWithProviderLoadsYAMLEvals(t *testing.T) {
	root := t.TempDir()
	writeTestSkillFile(t, root, "yaml-skill", "evals.yaml", `skill_name: yaml-skill
evals:
  - id: yaml-case
    name: yaml case
    prompt: Run the YAML case.
    assertions:
      - text: reports YAML assertion objects
`)

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"yaml-skill"},
		EvalIDs:   []string{"yaml-case"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 || summary.Skills[0].RelPath != "yaml-skill" {
		t.Fatalf("expected YAML skill to run, got %#v", summary.Skills)
	}
}

func TestLoadSkillUsesYAMLBeforeJSON(t *testing.T) {
	root := t.TempDir()
	writeTestSkillFile(t, root, "precedence-skill", "evals.json", `{
  "skill_name": "precedence-skill",
  "evals": [
    {
      "id": "json-case",
      "name": "json case",
      "prompt": "Run the JSON case.",
      "assertions": ["json assertion"]
    }
  ]
}`)
	yamlEvals := `skill_name: precedence-skill
evals:
  - id: yaml-case
    name: yaml case
    prompt: Run the YAML case.
    assertions:
      - yaml assertion
`
	if err := os.WriteFile(filepath.Join(root, "precedence-skill", "evals", "evals.yaml"), []byte(yamlEvals), 0o644); err != nil {
		t.Fatal(err)
	}

	skill, err := loadSkill(root, "precedence-skill", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skill.Evals) != 1 || skill.Evals[0].ID != "yaml-case" {
		t.Fatalf("expected YAML evals to take precedence, got %#v", skill.Evals)
	}
}

func TestLoadSkillFallsBackToYMLAndJSON(t *testing.T) {
	root := t.TempDir()
	writeTestSkillFile(t, root, "yml-skill", "evals.yml", `skill_name: yml-skill
evals:
  - id: yml-case
    name: yml case
    prompt: Run the YML case.
    assertions:
      - yml assertion
`)
	writeTestSkillWithEval(t, root, "json-skill", "json-case")

	ymlSkill, err := loadSkill(root, "yml-skill", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ymlSkill.Evals) != 1 || ymlSkill.Evals[0].ID != "yml-case" {
		t.Fatalf("expected YML evals, got %#v", ymlSkill.Evals)
	}

	jsonSkill, err := loadSkill(root, "json-skill", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonSkill.Evals) != 1 || jsonSkill.Evals[0].ID != "json-case" {
		t.Fatalf("expected JSON fallback evals, got %#v", jsonSkill.Evals)
	}
}

func TestRunWithProviderAllowsExplicitMultiSkillEvalIDInOneSkill(t *testing.T) {
	root := t.TempDir()
	writeTestSkillWithEval(t, root, "match-skill", "target-case")
	writeTestSkillWithEval(t, root, "other-skill", "other-case")

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"match-skill", "other-skill"},
		EvalIDs:   []string{"target-case"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 || summary.Skills[0].RelPath != "match-skill" {
		t.Fatalf("expected only matching skill, got %#v", summary.Skills)
	}
}

func TestRunWithProviderFailsGateForSmallDelta(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"case-a"},
		MinPass:   0.9,
		MinDelta:  1.1,
	}, &fakeProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if gatePassed {
		t.Fatal("expected gate failure")
	}
	if len(summary.GateFailures) != 1 || !strings.Contains(summary.GateFailures[0], "baseline delta") {
		t.Fatalf("unexpected gate failures: %#v", summary.GateFailures)
	}
}

func TestRunWithProviderAccumulatesAssertionTotals(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	evals := `{
  "skill_name": "demo-skill",
  "evals": [
    {
      "id": "case-a",
      "name": "case a",
      "prompt": "Run case A.",
      "files": ["evals/files/input.txt"],
      "assertions": ["passes", "reports missing detail"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "demo-skill", "evals", "evals.json"), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  false,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"case-a"},
		MinPass:   0.4,
		MinDelta:  0.2,
	}, partialProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if gatePassed {
		t.Fatal("expected gates to fail because one assertion failed")
	}
	if summary.Passed != 1 || summary.Failed != 1 {
		t.Fatalf("expected direct assertion totals 1/1, got %d/%d", summary.Passed, summary.Failed)
	}
	if summary.Skills[0].Passed != 1 || summary.Skills[0].Failed != 1 {
		t.Fatalf("expected skill assertion totals 1/1, got %d/%d", summary.Skills[0].Passed, summary.Skills[0].Failed)
	}
	if len(summary.GateFailures) == 0 || !strings.Contains(summary.GateFailures[0], "failed with_skill assertions") {
		t.Fatalf("unexpected gate failures: %#v", summary.GateFailures)
	}
}

func TestRunWithProviderFailsGateForAnyFailedAssertion(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	evals := `{
  "skill_name": "demo-skill",
  "evals": [
    {
      "id": "case-a",
      "name": "case a",
      "prompt": "Run case A.",
      "assertions": ["passes", "reports missing detail"]
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(root, "demo-skill", "evals", "evals.json"), []byte(evals), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  false,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"case-a"},
		MinPass:   0.4,
		MinDelta:  0.2,
	}, partialProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if gatePassed {
		t.Fatal("expected gate failure for failed assertion")
	}
	if len(summary.GateFailures) == 0 || !strings.Contains(summary.GateFailures[0], "failed with_skill assertions") {
		t.Fatalf("unexpected gate failures: %#v", summary.GateFailures)
	}
}

func TestRunWithProviderReportsMissingEvalID(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)

	_, _, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"missing"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, &fakeProvider{})
	if err == nil || !strings.Contains(err.Error(), "missing eval ids for demo-skill: missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSkillReportsMalformedFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	if err := os.WriteFile(filepath.Join(root, "demo-skill", "SKILL.md"), []byte("---\nname: [\n---\n\nUse the skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadSkill(root, "demo-skill", nil, true)
	if err == nil || !strings.Contains(err.Error(), "frontmatter") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserPromptRejectsEscapingEvalFile(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	skill, err := loadSkill(root, "demo-skill", []string{"case-a"}, true)
	if err != nil {
		t.Fatal(err)
	}
	skill.Evals[0].Files = []string{"../../outside.txt"}

	_, err = userPrompt(skill, skill.Evals[0])
	if err == nil || !strings.Contains(err.Error(), "escapes skill directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
