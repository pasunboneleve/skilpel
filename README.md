# skilpel

[![CI](https://github.com/pasunboneleve/skilpel/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pasunboneleve/skilpel/actions/workflows/ci.yml?query=branch%3Amain)

`skilpel` evaluates Codex-style skills by running the same prompt with and without the skill, then judging whether the skill improved the result.

It is a focused Go evaluator for [agentskills.io](https://agentskills.io)-style repositories. The current version targets local skill development and CI gates: run one skill or one eval case, compare `with_skill` against `without_skill`, and fail clearly when a skill does not pass or improve enough over baseline.

## Why This Exists

Skill repositories need a repeatable way to answer a narrow question: did this skill change the model output in the intended direction? `skilpel` keeps that check local, explicit, and suitable for CI.

## Architecture At A Glance

- `cmd/skilpel` owns the CLI entrypoint.
- `internal/skilpel` owns skill discovery, prompt construction, provider calls, judging, gates, and artifacts.
- Eval files live beside each skill under `evals/`.
- Run artifacts are written to the configured workspace, usually `./.skilpel`.

## Quick Start

```bash
go test ./...
go run ./cmd/skilpel run --root ./skills --skill my-skill --eval-id basic --baseline
```

Model-backed runs use the provider plugin selected by `--provider` or `provider` in config. Run `skilpel help` for the current provider list, default API key variables, and endpoint override rules.

```bash
OPENAI_API_KEY=... go run ./cmd/skilpel run \
  --root ./skills \
  --skill my-skill \
  --eval-id basic \
  --workspace ./.skilpel \
  --baseline \
  --provider openai \
  --target gpt-4o-mini \
  --judge gpt-4o-mini \
  --min-pass 0.90 \
  --min-delta 0.20
```

Use `--api-key-env` when a provider key lives in a non-default environment variable.

## Eval Files

- Skill evals may be stored as `evals/evals.yaml`, `evals/evals.yml`, or `evals/evals.json`.
- `evals/evals.yaml` is preferred; `evals/evals.yml` is read next; `evals/evals.json` is the fallback.
- YAML and JSON evals use the same field structure.
- Each eval needs a `prompt` plus `assertions` or `expected_output`.

```yaml
skill_name: my-skill
evals:
  - id: basic
    prompt: Apply the skill to this input.
    assertions:
      - Produces the expected result.
```

## Current Shape

- `run` discovers skills, executes eval cases, writes JSON artifacts, and applies gates.
- `--skill` narrows skill selection by repository-relative path.
- `--eval-id` narrows eval selection by exact ID.
- `--baseline` enables `without_skill` comparison and baseline-delta gates.
- Exit code `0` means the run completed and all configured gates passed.
- Exit code `1` means evals ran but assertions or gates failed.
- Exit code `2` means usage, configuration, filesystem, provider, or runtime failure.

## Validation

```bash
go test ./...
```

## Repository Map

- `cmd/skilpel/`: CLI binary.
- `internal/skilpel/`: evaluator implementation and tests.
- `CHANGELOG.md`: unreleased and release notes.

## Documentation

- `go run ./cmd/skilpel --help`
- `go run ./cmd/skilpel run --help`
- `go run ./cmd/skilpel version`
- [Changelog](CHANGELOG.md)

## License

`skilpel` is released under the [MIT License](LICENSE).

## Status

`skilpel` is an MVP. It supports provider plugins, per-skill eval files, baseline comparison, JSON artifacts, and pass-rate or baseline-delta gates.

For downstream CI, install a tagged version rather than tracking a moving branch:

```bash
go install github.com/pasunboneleve/skilpel/cmd/skilpel@$SKILPEL_VERSION
```

Tagged releases also publish prebuilt archives for Linux amd64 and macOS arm64.

## Prior Art

`skilpel` is inspired by [`agent-skills-eval`](https://github.com/darkrishabh/agent-skills-eval) and agentskills.io-style skill layouts. It focuses on the subset needed for fast local iteration and CI: skill discovery, eval-case filtering, provider-backed model calls, baseline comparison, and explicit pass/fail thresholds.
