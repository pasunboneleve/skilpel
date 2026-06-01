# Eval Files

`skilpel` reads eval definitions beside each skill:

1. `evals/evals.yaml`
2. `evals/evals.yml`
3. `evals/evals.json`

YAML and JSON use the same field structure. `evals/evals.yaml` is preferred,
`evals/evals.yml` is read next, and `evals/evals.json` is the fallback.

Each eval needs a `prompt` plus either `assertions` or `expected_output`.

```yaml
skill_name: my-skill
evals:
  - id: basic
    prompt: Apply the skill to this input.
    assertions:
      - Produces the expected result.
```

Use `--skill` to narrow the selected skills and `--eval-id` to narrow the eval
cases by exact ID. Both flags are repeatable.
