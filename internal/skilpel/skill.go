package skilpel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type evalsFile struct {
	SkillName string          `json:"skill_name"`
	Evals     []evalCaseRaw   `json:"evals"`
	Defaults  json.RawMessage `json:"defaults,omitempty"`
}

type evalCaseRaw struct {
	ID             any             `json:"id"`
	Name           string          `json:"name"`
	Prompt         string          `json:"prompt"`
	ExpectedOutput string          `json:"expected_output"`
	Files          []string        `json:"files"`
	Assertions     json.RawMessage `json:"assertions"`
	Params         map[string]any  `json:"params"`
}

func discoverSkills(root string, relpaths []string, evalIDs []string) ([]Skill, error) {
	if len(relpaths) > 0 {
		skills := make([]Skill, 0, len(relpaths))
		requireEvalIDs := len(relpaths) == 1
		for _, rel := range relpaths {
			skill, err := loadSkill(root, rel, evalIDs, requireEvalIDs)
			if err != nil {
				return nil, err
			}
			if len(skill.Evals) == 0 {
				continue
			}
			skills = append(skills, skill)
		}
		if missing := missingEvalIDs(skills, evalIDs); len(missing) > 0 {
			return nil, fmt.Errorf("missing eval ids for selected skills: %s", strings.Join(missing, ", "))
		}
		return skills, nil
	}

	var rels []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err == nil {
			if _, err := os.Stat(filepath.Join(path, "evals", "evals.json")); err == nil {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				rels = append(rels, filepath.ToSlash(rel))
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("discover skills: %w", err)
	}
	sort.Strings(rels)

	skills := make([]Skill, 0, len(rels))
	for _, rel := range rels {
		skill, err := loadSkill(root, rel, evalIDs, false)
		if err != nil {
			return nil, err
		}
		if len(skill.Evals) == 0 {
			continue
		}
		skills = append(skills, skill)
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("no skills with evals found under %s", root)
	}
	return skills, nil
}

func loadSkill(root, rel string, evalIDs []string, requireEvalIDs bool) (Skill, error) {
	dir := filepath.Join(root, filepath.FromSlash(rel))
	skillMD, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return Skill{}, fmt.Errorf("read %s/SKILL.md: %w", rel, err)
	}

	frontmatter, body := parseSkillMarkdown(string(skillMD))
	name := frontmatter.Name
	if name == "" {
		name = filepath.Base(dir)
	}

	evals, err := readEvals(filepath.Join(dir, "evals", "evals.json"))
	if err != nil {
		return Skill{}, fmt.Errorf("read %s/evals/evals.json: %w", rel, err)
	}
	evals, err = filterEvalIDs(evals, evalIDs, rel, requireEvalIDs)
	if err != nil {
		return Skill{}, err
	}

	return Skill{
		Name:        name,
		RelPath:     filepath.ToSlash(rel),
		Dir:         dir,
		SkillMD:     string(skillMD),
		Description: frontmatter.Description,
		Body:        body,
		Evals:       evals,
	}, nil
}

func parseSkillMarkdown(markdown string) (skillFrontmatter, string) {
	if !strings.HasPrefix(markdown, "---\n") && !strings.HasPrefix(markdown, "---\r\n") {
		return skillFrontmatter{}, strings.TrimSpace(markdown)
	}
	re := regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?`)
	match := re.FindStringSubmatch(markdown)
	if len(match) != 2 {
		return skillFrontmatter{}, strings.TrimSpace(markdown)
	}
	var fm skillFrontmatter
	_ = yaml.Unmarshal([]byte(match[1]), &fm)
	return fm, strings.TrimSpace(markdown[len(match[0]):])
}

func readEvals(path string) ([]EvalCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw evalsFile
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if len(raw.Evals) == 0 {
		return nil, fmt.Errorf("evals array is empty")
	}

	evals := make([]EvalCase, 0, len(raw.Evals))
	for i, entry := range raw.Evals {
		if entry.Prompt == "" {
			return nil, fmt.Errorf("evals[%d].prompt is required", i)
		}
		assertions, err := parseAssertions(entry.Assertions)
		if err != nil {
			return nil, fmt.Errorf("evals[%d].assertions: %w", i, err)
		}
		if len(assertions) == 0 && entry.ExpectedOutput != "" {
			assertions = []string{entry.ExpectedOutput}
		}
		if len(assertions) == 0 {
			return nil, fmt.Errorf("evals[%d] needs assertions or expected_output", i)
		}
		evals = append(evals, EvalCase{
			ID:             stringifyID(entry.ID),
			Name:           entry.Name,
			Prompt:         entry.Prompt,
			ExpectedOutput: entry.ExpectedOutput,
			Files:          entry.Files,
			Assertions:     assertions,
			Params:         entry.Params,
		})
	}
	return evals, nil
}

func parseAssertions(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var values []any
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	assertions := make([]string, 0, len(values))
Assertions:
	for i, value := range values {
		switch v := value.(type) {
		case string:
			assertions = append(assertions, v)
		case map[string]any:
			for _, key := range []string{"text", "value", "criterion"} {
				if s, ok := v[key].(string); ok {
					assertions = append(assertions, s)
					continue Assertions
				}
			}
			return nil, fmt.Errorf("[%d] object needs text, value, or criterion", i)
		default:
			return nil, fmt.Errorf("[%d] must be a string or assertion object", i)
		}
	}
	return assertions, nil
}

func stringifyID(id any) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	default:
		return fmt.Sprint(v)
	}
}

func filterEvalIDs(evals []EvalCase, ids []string, rel string, requireAll bool) ([]EvalCase, error) {
	if len(ids) == 0 {
		return evals, nil
	}
	wanted := map[string]bool{}
	for _, id := range ids {
		wanted[id] = true
	}

	var filtered []EvalCase
	found := map[string]bool{}
	for _, eval := range evals {
		if wanted[eval.ID] {
			filtered = append(filtered, eval)
			found[eval.ID] = true
		}
	}

	var missing []string
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		if !requireAll {
			return filtered, nil
		}
		return nil, fmt.Errorf("missing eval ids for %s: %s", rel, strings.Join(missing, ", "))
	}
	return filtered, nil
}

func missingEvalIDs(skills []Skill, ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	found := map[string]bool{}
	for _, skill := range skills {
		for _, eval := range skill.Evals {
			found[eval.ID] = true
		}
	}
	var missing []string
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return missing
}
