package skilpel

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

func writeSummary(w io.Writer, summary Summary, format string) error {
	switch format {
	case "", "text":
		printTextSummary(w, summary)
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			return fmt.Errorf("write summary: %w", err)
		}
	case "markdown":
		printMarkdownSummary(w, summary)
	default:
		return fmt.Errorf("output format must be text, json, or markdown")
	}
	return nil
}

func printTextSummary(w io.Writer, summary Summary) {
	_, _ = fmt.Fprintf(w, "\n%sValidating skills: %s%s\n", colorBold, reportPath(summary), colorReset)

	_, _ = fmt.Fprintf(w, "\n%sEvals%s\n", colorBold, colorReset)
	for _, skill := range summary.Skills {
		icon, color := levelIcon(skill.Failed == 0)
		_, _ = fmt.Fprintf(
			w,
			"  %s%s %s:%s %d passed, %d failed\n",
			color,
			icon,
			skill.RelPath,
			colorReset,
			skill.Passed,
			skill.Failed,
		)
		printRateRow(w, "with", skill.WithSkillPass)
		if summary.Gates.Baseline {
			printRateRow(w, "without", skill.WithoutSkillPass)
			printRateRow(w, "delta", skill.Delta)
		}
	}

	_, _ = fmt.Fprintf(w, "\n%sGates%s\n", colorBold, colorReset)
	printGateRow(w, "minimum pass rate", summary.Gates.MinPass, summary.Skills, func(skill SkillSummary) float64 {
		return skill.WithSkillPass
	})
	if summary.Gates.Baseline {
		printGateRow(w, "minimum delta", summary.Gates.MinDelta, summary.Skills, func(skill SkillSummary) float64 {
			return skill.Delta
		})
	} else {
		_, _ = fmt.Fprintf(w, "  %sℹ without_skill disabled; delta gate skipped%s\n", colorCyan, colorReset)
	}
	for _, failure := range summary.GateFailures {
		_, _ = fmt.Fprintf(w, "  %s✗ %s%s\n", colorRed, failure, colorReset)
	}

	_, _ = fmt.Fprintln(w)
	if summary.Gates.Passed {
		_, _ = fmt.Fprintf(w, "%s%sResult: passed%s\n", colorBold, colorGreen, colorReset)
	} else {
		parts := []string{}
		if summary.Failed > 0 {
			parts = append(parts, fmt.Sprintf("%s%d failed assertion%s%s", colorRed, summary.Failed, pluralS(summary.Failed), colorReset))
		}
		if len(summary.GateFailures) > 0 {
			parts = append(parts, fmt.Sprintf("%s%d gate failure%s%s", colorRed, len(summary.GateFailures), pluralS(len(summary.GateFailures)), colorReset))
		}
		_, _ = fmt.Fprintf(w, "%sResult: %s%s\n", colorBold, strings.Join(parts, ", "), colorReset)
	}
	_, _ = fmt.Fprintln(w)
}

func printGateRow(w io.Writer, label string, threshold float64, skills []SkillSummary, value func(SkillSummary) float64) {
	passed := true
	for _, skill := range skills {
		if value(skill) < threshold {
			passed = false
			break
		}
	}
	icon, color := levelIcon(passed)
	_, _ = fmt.Fprintf(w, "  %s%s %s:%s threshold %s\n", color, icon, label, colorReset, formatRate(threshold))
}

func printRateRow(w io.Writer, label string, rate float64) {
	_, _ = fmt.Fprintf(w, "    %s%s:%s %s%s%s\n", colorCyan, label, colorReset, colorBold, formatRate(rate), colorReset)
}

func printMarkdownSummary(w io.Writer, summary Summary) {
	_, _ = fmt.Fprintf(w, "## Validating skills: %s\n\n", reportPath(summary))
	_, _ = fmt.Fprintln(w, "### Evals")
	for _, skill := range summary.Skills {
		status := "passed"
		if skill.Failed > 0 {
			status = "failed"
		}
		_, _ = fmt.Fprintf(w, "- **%s**: %s, %d passed, %d failed, with=%s\n", skill.RelPath, status, skill.Passed, skill.Failed, formatRate(skill.WithSkillPass))
	}
	_, _ = fmt.Fprintln(w, "\n### Gates")
	for _, failure := range summary.GateFailures {
		_, _ = fmt.Fprintf(w, "- %s\n", failure)
	}
	if len(summary.GateFailures) == 0 {
		_, _ = fmt.Fprintln(w, "- All gates passed.")
	}
	_, _ = fmt.Fprintln(w)
	if summary.Gates.Passed {
		_, _ = fmt.Fprintln(w, "**Result: passed**")
	} else {
		_, _ = fmt.Fprintf(w, "**Result: %d failed assertion%s, %d gate failure%s**\n", summary.Failed, pluralS(summary.Failed), len(summary.GateFailures), pluralS(len(summary.GateFailures)))
	}
}

func writeAnnotations(w io.Writer, summary Summary) {
	for _, failure := range summary.GateFailures {
		_, _ = fmt.Fprintf(w, "::error title=skilpel gate failed::%s\n", escapeAnnotation(failure))
	}
}

func levelIcon(passed bool) (string, string) {
	if passed {
		return "✓", colorGreen
	}
	return "✗", colorRed
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func escapeAnnotation(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	value = strings.ReplaceAll(value, "\n", "%0A")
	return value
}

func reportPath(summary Summary) string {
	if summary.Root != "" {
		return summary.Root
	}
	return summary.Workspace
}
