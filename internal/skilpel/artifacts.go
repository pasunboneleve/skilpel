package skilpel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func writeModeArtifacts(workspace string, skill Skill, eval EvalCase, evalSlug string, mode Mode, run ModeRun) (string, error) {
	dir := filepath.Join(workspace, slugify(skill.RelPath, slugify(skill.Name, "skill")), evalSlug, string(mode))
	if err := writeJSON(filepath.Join(dir, "output.json"), map[string]any{"output": run.Output}); err != nil {
		return "", fmt.Errorf("write output: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "timing.json"), run.Timing); err != nil {
		return "", fmt.Errorf("write timing: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "grading.json"), run.Grading); err != nil {
		return "", fmt.Errorf("write grading: %w", err)
	}
	if err := writeJSON(filepath.Join(dir, "prompt.json"), run.Prompt); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	return dir, nil
}

func benchmarkPath(workspace string, skill Skill) string {
	return filepath.Join(workspace, slugify(skill.RelPath, slugify(skill.Name, "skill")), "benchmark.json")
}
