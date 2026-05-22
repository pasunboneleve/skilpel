package skilpel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func grade(ctx context.Context, provider Provider, model string, eval EvalCase, output string, params map[string]any) (Grading, string, error) {
	prompt := judgePrompt(eval, output)
	result, err := provider.Complete(ctx, CompletionRequest{
		Model:  model,
		User:   prompt,
		Params: params,
	})
	if err != nil {
		return Grading{}, prompt, err
	}

	var grading Grading
	if err := json.Unmarshal([]byte(cleanJudgeJSON(result.Output)), &grading); err != nil {
		return Grading{}, prompt, fmt.Errorf("judge returned invalid JSON: %w", err)
	}
	normalizeGrading(&grading)
	return grading, prompt, nil
}

func cleanJudgeJSON(output string) string {
	trimmed := strings.TrimSpace(output)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimPrefix(trimmed, "json")
	trimmed = strings.TrimSpace(trimmed)
	trimmed = strings.TrimSuffix(trimmed, "```")
	return strings.TrimSpace(trimmed)
}

func judgePrompt(eval EvalCase, output string) string {
	var b strings.Builder
	b.WriteString("You are a strict JSON-only evaluator.\n")
	b.WriteString("Return only JSON with assertion_results and summary.\n\n")
	b.WriteString("Model output:\n")
	b.WriteString(output)
	b.WriteString("\n\nAssertions:\n")
	for i, assertion := range eval.Assertions {
		fmt.Fprintf(&b, "%d. %s\n", i+1, assertion)
	}
	return b.String()
}

func normalizeGrading(grading *Grading) {
	if grading.Summary.Total == 0 {
		for _, result := range grading.AssertionResults {
			if result.Passed {
				grading.Summary.Passed++
			} else {
				grading.Summary.Failed++
			}
		}
		grading.Summary.Total = grading.Summary.Passed + grading.Summary.Failed
	}
	if grading.Summary.Total > 0 {
		grading.Summary.PassRate = float64(grading.Summary.Passed) / float64(grading.Summary.Total)
	}
}
