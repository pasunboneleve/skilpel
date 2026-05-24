package skilpel

import "testing"

func TestEvalSlugAddsIndexWhenEvalLacksID(t *testing.T) {
	first := evalSlug(EvalCase{Name: "duplicate"}, 0)
	second := evalSlug(EvalCase{Name: "duplicate"}, 1)
	if first == second {
		t.Fatalf("expected unique slugs, got %q and %q", first, second)
	}
	if first != "eval-duplicate-1" || second != "eval-duplicate-2" {
		t.Fatalf("unexpected slugs: %q, %q", first, second)
	}
}

func TestEvalSlugPreservesNamedEvalWithID(t *testing.T) {
	got := evalSlug(EvalCase{ID: "case-b", Name: "case b"}, 1)
	if got != "eval-case-b" {
		t.Fatalf("unexpected slug: %q", got)
	}
}
