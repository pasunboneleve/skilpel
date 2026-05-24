package skilpel

import (
	"strings"
	"testing"
)

func TestParseGradingNormalizesStrictAssertionArray(t *testing.T) {
	grading, err := parseGrading(`{
  "assertion_results": [
    {"text": "ignored by caller", "passed": true, "evidence": "output names the concept"},
    {"text": "also ignored", "passed": false, "evidence": ""}
  ],
  "summary": {"passed": 99, "failed": 0, "total": 99, "pass_rate": 1}
}`, []string{"first assertion", "second assertion"})
	if err != nil {
		t.Fatal(err)
	}

	normalizeGrading(&grading)
	if grading.AssertionResults[0].Text != "first assertion" || !grading.AssertionResults[0].Passed {
		t.Fatalf("unexpected first assertion result: %#v", grading.AssertionResults[0])
	}
	if grading.AssertionResults[1].Text != "second assertion" || grading.AssertionResults[1].Passed {
		t.Fatalf("unexpected second assertion result: %#v", grading.AssertionResults[1])
	}
	if grading.AssertionResults[1].Evidence != "judge did not provide concrete evidence" {
		t.Fatalf("unexpected missing-evidence fallback: %q", grading.AssertionResults[1].Evidence)
	}
	if grading.Summary.Passed != 1 || grading.Summary.Failed != 1 || grading.Summary.Total != 2 || grading.Summary.PassRate != 0.5 {
		t.Fatalf("summary was not recomputed: %#v", grading.Summary)
	}
}

func TestParseGradingMatchesReorderedResultsByText(t *testing.T) {
	grading, err := parseGrading(`{
  "assertion_results": [
    {"text": "second assertion", "passed": false, "evidence": "second evidence"},
    {"text": "first assertion", "passed": true, "evidence": "first evidence"}
  ]
}`, []string{"first assertion", "second assertion"})
	if err != nil {
		t.Fatal(err)
	}

	normalizeGrading(&grading)
	if grading.AssertionResults[0].Text != "first assertion" || !grading.AssertionResults[0].Passed || grading.AssertionResults[0].Evidence != "first evidence" {
		t.Fatalf("unexpected first assertion result: %#v", grading.AssertionResults[0])
	}
	if grading.AssertionResults[1].Text != "second assertion" || grading.AssertionResults[1].Passed || grading.AssertionResults[1].Evidence != "second evidence" {
		t.Fatalf("unexpected second assertion result: %#v", grading.AssertionResults[1])
	}
}

func TestParseGradingRejectsNonArrayAssertionResults(t *testing.T) {
	_, err := parseGrading(`{
  "assertion_results": {"first": true},
  "summary": null
}`, []string{"first assertion"})
	if err == nil {
		t.Fatal("expected keyed assertion results to be rejected")
	}
}

func TestCleanJudgeJSONExtractsFencedJSONAfterPreamble(t *testing.T) {
	got := cleanJudgeJSON("Here is the grading:\n\n```json\n{\"assertion_results\":[]}\n```")
	if got != `{"assertion_results":[]}` {
		t.Fatalf("unexpected cleaned JSON: %q", got)
	}
}

func TestCleanJudgeJSONIgnoresEarlierNonJSONFence(t *testing.T) {
	got := cleanJudgeJSON("notes:\n```text\nnot json\n```\nresult:\n```json\n{\"assertion_results\":[]}\n```")
	if got != `{"assertion_results":[]}` {
		t.Fatalf("unexpected cleaned JSON: %q", got)
	}
}

func TestFailClosedGradingUsesOriginalAssertionsAndEvidence(t *testing.T) {
	grading := failClosedGrading([]string{"first assertion", "second assertion"}, "not json", errExample{})
	normalizeGrading(&grading)

	if grading.Summary.Passed != 0 || grading.Summary.Failed != 2 {
		t.Fatalf("unexpected fail-closed summary: %#v", grading.Summary)
	}
	for _, result := range grading.AssertionResults {
		if result.Passed {
			t.Fatalf("expected failure result: %#v", result)
		}
		if !strings.Contains(result.Evidence, "judge returned unparseable response") {
			t.Fatalf("missing diagnostic evidence: %q", result.Evidence)
		}
	}
}

type errExample struct{}

func (errExample) Error() string { return "example parse failure" }
