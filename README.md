# skilpel

[![CI](https://github.com/pasunboneleve/skilpel/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/pasunboneleve/skilpel/actions/workflows/ci.yml?query=branch%3Amain)

`skilpel` is a Go CLI for evaluating Codex-style skills. It runs eval prompts
with and without a skill, asks a judge model to score the outputs, and turns the
result into local artifacts, terminal feedback, and CI-friendly gates.

<br>

<p align="center" style="margin: 0.35rem 0 0.35rem 0;">
  <a href="https://www.metmuseum.org/art/collection/search/53215"
  target="_blank"
  rel="noopener noreferrer">
    <img
        src="docs/images/katagami.png"
        alt="Katagami stencil with hemp-leaf pattern"
        style="width:58.5%;"
        />
  </a>
</p>

<p align="center" style="margin: 0 0 1.25rem 0;">
    <sub>A scalpel cuts away the excess; a stencil preserves the pattern.</sub>
</p>

<!--
Image provenance:
Japanese katagami stencil with hemp-leaf pattern.
The Metropolitan Museum of Art, public domain.
Source: https://www.metmuseum.org/art/collection/search/53215
-->

## Why this exists

Skill repositories need a repeatable way to answer a narrow question: did this
skill change model output in the intended direction? `skilpel` keeps that check
local, explicit, and suitable for CI.

## Current scope

`skilpel run` supports:

- provider plugins for OpenAI, xAI, Qwen, Anthropic/Claude, and Gemini
- per-skill eval files in YAML or JSON
- skill and eval filtering with `--skill` and `--eval-id`
- optional `without_skill` baseline comparison
- pass-rate and baseline-delta gates
- JSON artifacts in a workspace directory
- text, JSON, and Markdown final summaries
- structured or pretty progress logs on stderr

## Architecture at a glance

- `cmd/skilpel` owns the CLI entrypoint.
- `internal/skilpel` owns skill discovery, prompt construction, provider calls,
  judging, gates, progress logs, reports, and artifacts.
- Eval files live beside each skill under `evals/`.
- Run artifacts are written to the configured workspace, usually `./.skilpel`.

## Quick start

```bash
go test ./...
go run ./cmd/skilpel run --root ./skills --skill my-skill --eval-id basic --baseline
```

Model-backed runs use the provider selected by `--provider` or `provider` in a
config file. Run `skilpel run --help` for the current provider list, default API
key variables, endpoint override rules, and available flags.

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

For scripts and downstream tooling, keep the final summary on stdout as JSON:

```bash
go run ./cmd/skilpel run --config skilpel.yaml --output=json
```

On an interactive terminal, pretty progress gives a quick read on the active
run while keeping scripts pointed at stdout:

![Pretty progress terminal preview](docs/images/pretty-progress.svg)

See [CLI output](docs/cli-output.md) for stdout, stderr, log-file, and exit-code
behavior.

## Validation

```bash
go test ./...
```

## Repository map

- `cmd/skilpel/`: CLI binary.
- `internal/skilpel/`: evaluator implementation and tests.
- `docs/`: focused user documentation.
- `CHANGELOG.md`: unreleased and release notes.

## Documentation

- `go run ./cmd/skilpel --help`
- `go run ./cmd/skilpel run --help`
- `go run ./cmd/skilpel version`
- [CLI output](docs/cli-output.md)
- [Eval files](docs/eval-files.md)
- [Changelog](CHANGELOG.md)

## Installation

For downstream CI, install a tagged version rather than tracking a moving
branch:

```bash
go install github.com/pasunboneleve/skilpel/cmd/skilpel@$SKILPEL_VERSION
```

Tagged releases also publish prebuilt archives for Linux amd64 and macOS arm64.

## Prior art

`skilpel` is inspired by
[`agent-skills-eval`](https://github.com/darkrishabh/agent-skills-eval) and
agentskills.io-style skill layouts. It focuses on the subset needed for fast
local iteration and CI: skill discovery, eval-case filtering, provider-backed
model calls, baseline comparison, and explicit pass/fail thresholds.

## License

`skilpel` is released under the [MIT License](LICENSE).
