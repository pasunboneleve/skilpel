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
		Warnings: []RunWarning{
			{Skill: "empty-skill", Message: "no evals file found; skipping skill"},
		},
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
		"Warnings",
		colorYellow + "⚠ empty-skill: no evals file found; skipping skill" + colorReset,
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
		textResultDivider,
		"✓ minimum pass rate:",
		"Result: ✓ passed",
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
		textResultDivider,
		"Gates",
		"✓ minimum pass rate:",
		"✓ minimum delta:",
		"Result: ✓ passed",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected final summary to include %q, got %q", want, got)
		}
	}
}

func TestWriteFinalSummaryShowsFailureIcon(t *testing.T) {
	summary := Summary{
		Failed: 1,
		Gates: GateSummary{
			MinPass:  0.9,
			MinDelta: 0.2,
			Baseline: true,
			Passed:   false,
		},
		GateFailures: []string{"shell-script failed 50%"},
		Skills: []SkillSummary{
			{
				RelPath:       "shell-script",
				Passed:        1,
				Failed:        1,
				WithSkillPass: 0.5,
				Delta:         0,
			},
		},
	}

	var output bytes.Buffer
	if err := writeFinalSummary(&output, summary, "text", true); err != nil {
		t.Fatal(err)
	}

	got := output.String()
	for _, want := range []string{
		textResultDivider,
		"✗ minimum pass rate:",
		"✗ minimum delta:",
		"✗ shell-script failed 50%",
		"Result: ✗",
		"1 failed assertion",
		"1 gate failure",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected failed final summary to include %q, got %q", want, got)
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
