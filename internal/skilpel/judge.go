package skilpel

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var fencedJSONRE = regexp.MustCompile("(?is)```json\\s*(.*?)\\s*```")

type judgeAssertionResult struct {
	Text     string `json:"text"`
	Passed   bool   `json:"passed"`
	Evidence string `json:"evidence"`
}

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

	grading, err := parseGrading(cleanJudgeJSON(result.Output), eval.Assertions)
	if err != nil {
		grading = failClosedGrading(eval.Assertions, result.Output, err)
	}
	normalizeGrading(&grading)
	return grading, prompt, nil
}

func parseGrading(output string, assertions []string) (Grading, error) {
	var raw struct {
		AssertionResults json.RawMessage `json:"assertion_results"`
	}
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return Grading{}, err
	}
	if len(raw.AssertionResults) == 0 || string(raw.AssertionResults) == "null" {
		return Grading{}, fmt.Errorf("grading response missing assertion_results")
	}
	var rawMessages []json.RawMessage
	if err := json.Unmarshal(raw.AssertionResults, &rawMessages); err != nil {
		return Grading{}, err
	}

	rawResults := make([]judgeAssertionResult, len(rawMessages))
	validResults := make([]bool, len(rawMessages))
	for i, rawMessage := range rawMessages {
		if err := json.Unmarshal(rawMessage, &rawResults[i]); err == nil {
			validResults[i] = true
		}
	}

	results := make([]AssertionResult, 0, len(assertions))
	usedResults := make([]bool, len(rawResults))
	for i, assertion := range assertions {
		resultIndex := matchingJudgeResult(assertion, rawResults, validResults, usedResults)
		if resultIndex < 0 && i < len(rawResults) && validResults[i] && !usedResults[i] {
			resultIndex = i
		}
		if resultIndex < 0 {
			results = append(results, AssertionResult{
				Text:     assertion,
				Passed:   false,
				Evidence: "judge omitted this assertion result",
			})
			continue
		}
		usedResults[resultIndex] = true
		rawResult := rawResults[resultIndex]
		evidence := strings.TrimSpace(rawResult.Evidence)
		if evidence == "" {
			evidence = "judge did not provide concrete evidence"
		}
		results = append(results, AssertionResult{
			Text:     assertion,
			Passed:   rawResult.Passed,
			Evidence: evidence,
		})
	}
	return Grading{AssertionResults: results}, nil
}

func matchingJudgeResult(assertion string, results []judgeAssertionResult, valid, used []bool) int {
	want := strings.TrimSpace(assertion)
	for i, result := range results {
		if valid[i] && !used[i] && strings.TrimSpace(result.Text) == want {
			return i
		}
	}
	return -1
}

func cleanJudgeJSON(output string) string {
	trimmed := strings.TrimSpace(output)
	matches := fencedJSONRE.FindAllStringSubmatch(trimmed, -1)
	if len(matches) > 0 {
		match := matches[len(matches)-1]
		return strings.TrimSpace(match[1])
	}
	return trimmed
}

func failClosedGrading(assertions []string, output string, cause error) Grading {
	evidence := fmt.Sprintf("judge returned unparseable response: %s; error: %v", truncate(output, 500), cause)
	results := make([]AssertionResult, 0, len(assertions))
	for _, assertion := range assertions {
		results = append(results, AssertionResult{Text: assertion, Passed: false, Evidence: evidence})
	}
	return Grading{AssertionResults: results}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func judgePrompt(eval EvalCase, output string) string {
	assertions, err := json.MarshalIndent(eval.Assertions, "", "  ")
	if err != nil {
		assertions = []byte("[]")
	}
	return fmt.Sprintf(`You are grading an agentskills.io evaluation run.

Grading principles:
- Require concrete evidence for every PASS; quote or reference the output.
- Do not give the benefit of the doubt.
- PASS an assertion only if every condition in the assertion text holds.
- A label without substance is a FAIL.

Return only JSON. Use STRICT JSON only. No markdown. Shape:
{"assertion_results":[{"text":"...","passed":true,"evidence":"..."}],"summary":{"passed":0,"failed":0,"total":0,"pass_rate":0}}

Rules:
- Include every assertion exactly once and copy the full assertion text verbatim into text.
- Use short concrete evidence: quote, snippet, or file reference.
- Summary may be included, but it will be recomputed by the caller.

Assertions:
%s

Model output:
%s`, string(assertions), output)
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
