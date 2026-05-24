package skilpel

import (
	"context"
	"encoding/json"
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
		BaseURL:   "http://example.test/v1",
		APIKeyEnv: "OPENAI_API_KEY",
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

func TestRunWithProviderFailsGateForSmallDelta(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)

	summary, gatePassed, err := RunWithProvider(context.Background(), Config{
		Root:      root,
		Workspace: filepath.Join(t.TempDir(), "workspace"),
		Baseline:  true,
		Target:    "target",
		Judge:     "judge",
		BaseURL:   "http://example.test/v1",
		APIKeyEnv: "OPENAI_API_KEY",
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
		BaseURL:   "http://example.test/v1",
		APIKeyEnv: "OPENAI_API_KEY",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"case-a"},
		MinPass:   0.4,
		MinDelta:  0.2,
	}, partialProvider{})
	if err != nil {
		t.Fatal(err)
	}
	if !gatePassed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if summary.Passed != 1 || summary.Failed != 1 {
		t.Fatalf("expected direct assertion totals 1/1, got %d/%d", summary.Passed, summary.Failed)
	}
	if summary.Skills[0].Passed != 1 || summary.Skills[0].Failed != 1 {
		t.Fatalf("expected skill assertion totals 1/1, got %d/%d", summary.Skills[0].Passed, summary.Skills[0].Failed)
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
		BaseURL:   "http://example.test/v1",
		APIKeyEnv: "OPENAI_API_KEY",
		Skills:    []string{"demo-skill"},
		EvalIDs:   []string{"missing"},
		MinPass:   0.9,
		MinDelta:  0.2,
	}, &fakeProvider{})
	if err == nil || !strings.Contains(err.Error(), "missing eval ids for demo-skill: missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUserPromptRejectsEscapingEvalFile(t *testing.T) {
	root := t.TempDir()
	writeTestSkill(t, root)
	skill, err := loadSkill(root, "demo-skill", []string{"case-a"})
	if err != nil {
		t.Fatal(err)
	}
	skill.Evals[0].Files = []string{"../../outside.txt"}

	_, err = userPrompt(skill, skill.Evals[0])
	if err == nil || !strings.Contains(err.Error(), "escapes skill directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}
