package skilpel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func skillSystemPrompt(skill Skill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<skill name=%q>\n", skill.Name)
	if skill.Description != "" {
		fmt.Fprintf(&b, "<description>%s</description>\n", skill.Description)
	}
	b.WriteString("<instructions>\n")
	b.WriteString(skill.Body)
	b.WriteString("\n</instructions>\n</skill>")
	return b.String()
}

func userPrompt(skill Skill, eval EvalCase) (string, error) {
	var b strings.Builder
	b.WriteString(eval.Prompt)
	for _, rel := range eval.Files {
		fullPath, err := safeSkillPath(skill.Dir, rel)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("read eval file %s: %w", rel, err)
		}
		fmt.Fprintf(&b, "\n\n<file path=%q>\n%s\n</file>", rel, string(data))
	}
	return b.String(), nil
}

func safeSkillPath(skillDir, rel string) (string, error) {
	base, err := filepath.Abs(skillDir)
	if err != nil {
		return "", err
	}
	fullPath, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	if fullPath != base && !strings.HasPrefix(fullPath, base+string(filepath.Separator)) {
		return "", fmt.Errorf("eval file path escapes skill directory: %s", rel)
	}
	return fullPath, nil
}
