# Eval Files

`skilpel` reads eval definitions from each skill's own `evals/` directory. The
preferred path is `<skill>/evals/evals.yaml`, keeping the eval close to the
skill it tests.

This is a superset of the agentskills.io-style skill layout: the skill remains
the unit of documentation and behavior, and `skilpel` adds an eval file beside
that skill. For a complete repository model, see
[`pasunboneleve/oiticica-style`](https://github.com/pasunboneleve/oiticica-style).

Use `--root` or config `root` to tell `skilpel` where the skills tree starts.
If no root is provided, `skilpel` uses the current directory. It walks that tree
for directories containing `SKILL.md`; for each skill directory, it checks these
relative paths in order:

1. `evals/evals.yaml`
2. `evals/evals.yml`
3. `evals/evals.json`

YAML and JSON use the same field structure. `evals/evals.yaml` is preferred,
`evals/evals.yml` is read next, and `evals/evals.json` is the fallback.

`--skill <relpath>` narrows selection to a skill under the root. It does not
change the eval location: `skilpel` still looks under
`<root>/<skill>/evals/`.

Each eval needs a `prompt` plus either `assertions` or `expected_output`.

```yaml
skill_name: my-skill
evals:
  - id: basic
    prompt: Apply the skill to this input.
    assertions:
      - Produces the expected result.
```

Use `--eval-id` to narrow eval cases by exact ID. `--skill` and `--eval-id` are
both repeatable.
