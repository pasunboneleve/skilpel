package skilpel

import (
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
	if source == "" && eval.ID != "" {
		source = "eval-" + eval.ID
	}
	if source == "" {
		source = "eval"
	}
	slug := slugify(source, "eval")
	if !strings.HasPrefix(slug, "eval-") {
		slug = "eval-" + slug
	}
	return slug
}
