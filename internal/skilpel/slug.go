package skilpel

import (
	"fmt"
	"regexp"
	"strings"
)

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(value, fallback string) string {
	slug := strings.Trim(slugRE.ReplaceAllString(strings.ToLower(value), "-"), "-")
	if slug == "" {
		return fallback
	}
	if len(slug) > 64 {
		slug = strings.Trim(slug[:64], "-")
	}
	if slug == "" {
		return fallback
	}
	return slug
}

func evalSlug(eval EvalCase, index int) string {
	source := eval.Name
	needsIndex := eval.ID == ""
	if source == "" && eval.ID != "" {
		source = "eval-" + eval.ID
		needsIndex = false
	}
	if source == "" {
		source = fmt.Sprintf("eval-%d", index+1)
		needsIndex = false
	}
	slug := slugify(source, "eval")
	if needsIndex {
		slug = fmt.Sprintf("%s-%d", slug, index+1)
	}
	if !strings.HasPrefix(slug, "eval-") {
		slug = "eval-" + slug
	}
	return slug
}
