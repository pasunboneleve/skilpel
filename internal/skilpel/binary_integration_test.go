package skilpel

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompiledBinaryRunsShellScriptYAMLEvalFixture(t *testing.T) {
	repoRoot := testRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "skilpel")

	build := exec.Command("go", "build", "-o", bin, "./cmd/skilpel")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build skilpel: %v\n%s", err, output)
	}

	server := httptest.NewServer(http.HandlerFunc(fakeChatCompletions))
	t.Cleanup(server.Close)
	t.Setenv("SKILPEL_TEST_API_KEY", "test-key")

	workspace := filepath.Join(t.TempDir(), "workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin,
		"run",
		"--root", filepath.Join(repoRoot, "testdata", "skills"),
		"--skill", "shell-script",
		"--eval-id", "new-script-strict-mode",
		"--workspace", workspace,
		"--baseline",
		"--target", "target",
		"--judge", "judge",
		"--base-url", server.URL,
		"--api-key-env", "SKILPEL_TEST_API_KEY",
		"--min-pass", "1",
		"--min-delta", "1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run compiled skilpel: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var summary Summary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("parse stdout summary: %v\n%s", err, stdout.String())
	}
	if !summary.Gates.Passed {
		t.Fatalf("expected gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 {
		t.Fatalf("expected one skill summary, got %#v", summary.Skills)
	}
	skill := summary.Skills[0]
	if skill.RelPath != "shell-script" || skill.Evals != 1 || skill.WithSkillPass != 1 || skill.WithoutSkillPass != 0 || skill.Delta != 1 {
		t.Fatalf("unexpected shell-script summary: %#v", skill)
	}
	if _, err := os.Stat(filepath.Join(workspace, "shell-script", "eval-new-script-strict-mode", "result.json")); err != nil {
		t.Fatalf("expected compiled run to write result artifact: %v", err)
	}
}

func TestCompiledBinaryRunsShellScriptOpenAICanary(t *testing.T) {
	if os.Getenv("RUN_SKILPEL_CANARY") != "1" {
		t.Skip("set RUN_SKILPEL_CANARY=1 to run the OpenAI shell-script canary")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}

	repoRoot := testRepoRoot(t)
	bin := filepath.Join(t.TempDir(), "skilpel")

	build := exec.Command("go", "build", "-o", bin, "./cmd/skilpel")
	build.Dir = repoRoot
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build skilpel: %v\n%s", err, output)
	}

	workspace := filepath.Join(t.TempDir(), "workspace")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin,
		"run",
		"--root", filepath.Join(repoRoot, "testdata", "skills"),
		"--skill", "shell-script",
		"--eval-id", "new-script-strict-mode",
		"--workspace", workspace,
		"--no-baseline",
		"--provider", "openai",
		"--target", "gpt-4o-mini",
		"--judge", "gpt-4o-mini",
		"--min-pass", "1",
		"--min-delta", "0",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "OPENAI_API_KEY="+os.Getenv("OPENAI_API_KEY"))
	if err := cmd.Run(); err != nil {
		t.Fatalf("run OpenAI canary: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var summary Summary
	if err := json.Unmarshal(stdout.Bytes(), &summary); err != nil {
		t.Fatalf("parse stdout summary: %v\n%s", err, stdout.String())
	}
	if !summary.Gates.Passed {
		t.Fatalf("expected canary gates to pass: %#v", summary.GateFailures)
	}
	if len(summary.Skills) != 1 || summary.Skills[0].RelPath != "shell-script" || summary.Skills[0].WithSkillPass != 1 {
		t.Fatalf("unexpected canary summary: %#v", summary.Skills)
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate current test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func fakeChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/chat/completions" {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	content := ""
	switch req.Model {
	case "target":
		if requestIncludesSkill(req.Messages) {
			content = "```bash\n#!/usr/bin/env bash\nset -euo pipefail\n\ngit branch --show-current\ngit status --short\n```"
		} else {
			content = "```bash\n#!/usr/bin/env bash\n\ngit branch --show-current\ngit status --short\n```"
		}
	case "judge":
		prompt := req.Messages[len(req.Messages)-1].Content
		content = fakeJudgeOutput(prompt)
	default:
		http.Error(w, "unexpected model "+req.Model, http.StatusBadRequest)
		return
	}

	w.Header().Set("content-type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]string{"content": content}},
		},
		"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func requestIncludesSkill(messages []struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}) bool {
	for _, message := range messages {
		if message.Role == "system" && strings.Contains(message.Content, "<skill name=\"shell-script\">") {
			return true
		}
	}
	return false
}

func fakeJudgeOutput(prompt string) string {
	assertions := judgeAssertions(prompt)
	modelOutput := judgeModelOutput(prompt)
	passed := strings.Contains(modelOutput, "set -euo pipefail")

	results := make([]map[string]any, 0, len(assertions))
	for _, assertion := range assertions {
		evidence := "missing strict mode"
		if passed {
			evidence = "output contains strict mode immediately after the shebang"
		}
		results = append(results, map[string]any{
			"text":     assertion,
			"passed":   passed,
			"evidence": evidence,
		})
	}
	data, err := json.Marshal(map[string]any{"assertion_results": results})
	if err != nil {
		return `{"assertion_results":[]}`
	}
	return string(data)
}

func judgeAssertions(prompt string) []string {
	const startMarker = "Assertions:\n"
	const endMarker = "\n\nModel output:"
	start := strings.Index(prompt, startMarker)
	end := strings.Index(prompt, endMarker)
	if start < 0 || end < 0 || start >= end {
		return nil
	}
	var assertions []string
	if err := json.Unmarshal([]byte(prompt[start+len(startMarker):end]), &assertions); err != nil {
		return nil
	}
	return assertions
}

func judgeModelOutput(prompt string) string {
	const marker = "\n\nModel output:\n"
	index := strings.Index(prompt, marker)
	if index < 0 {
		return ""
	}
	return prompt[index+len(marker):]
}
