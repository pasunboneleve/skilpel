package skilpel

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintTextSummaryUsesSkillValidatorStyleSections(t *testing.T) {
	summary := Summary{
		Root:      "/tmp/skills",
		Passed:    2,
		Failed:    0,
		Workspace: "/tmp/workspace",
		Gates: GateSummary{
			MinPass:  0.9,
			MinDelta: 0.2,
			Baseline: true,
			Passed:   true,
		},
		Skills: []SkillSummary{
			{
				RelPath:          "shell-script",
				Passed:           2,
				Failed:           0,
				WithSkillPass:    1,
				WithoutSkillPass: 0.5,
				Delta:            0.5,
			},
		},
	}

	var output bytes.Buffer
	if err := writeSummary(&output, summary, "text"); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, want := range []string{
		"Validating skills: /tmp/skills",
		"Evals",
		"✓ shell-script:",
		"2 passed, 0 failed",
		"with:",
		colorBold + "100%" + colorReset,
		"without:",
		colorBold + "50%" + colorReset,
		"delta:",
		colorBold + "50%" + colorReset,
		"Gates",
		"✓ minimum pass rate:",
		"Result: passed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected text summary to include %q, got %q", want, got)
		}
	}
}

func TestWriteFinalSummaryOmitsEvalRowsWhenPrettyProgressIsVisible(t *testing.T) {
	summary := Summary{
		Root:   "/tmp/skills",
		Passed: 2,
		Failed: 0,
		Gates: GateSummary{
			MinPass:  0.9,
			MinDelta: 0.2,
			Baseline: true,
			Passed:   true,
		},
		Skills: []SkillSummary{
			{
				RelPath:          "shell-script",
				Passed:           2,
				Failed:           0,
				WithSkillPass:    1,
				WithoutSkillPass: 0.5,
				Delta:            0.5,
			},
		},
	}

	var output bytes.Buffer
	if err := writeFinalSummary(&output, summary, "text", true); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, duplicate := range []string{
		"Validating skills:",
		"Evals",
		"shell-script",
		"    with:",
		"    without:",
		"    delta:",
	} {
		if strings.Contains(got, duplicate) {
			t.Fatalf("final summary duplicated %q after pretty progress, got %q", duplicate, got)
		}
	}
	for _, want := range []string{
		"Gates",
		"✓ minimum pass rate:",
		"✓ minimum delta:",
		"Result: passed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected final summary to include %q, got %q", want, got)
		}
	}
}

func TestWriteAnnotationsEscapesGateFailures(t *testing.T) {
	summary := Summary{
		GateFailures: []string{"skill failed 50%\nretry"},
	}

	var output bytes.Buffer
	writeAnnotations(&output, summary)

	if got, want := output.String(), "::error title=skilpel gate failed::skill failed 50%25%0Aretry\n"; got != want {
		t.Fatalf("annotation = %q, want %q", got, want)
	}
}
