# skilpel

`skilpel` cuts skills out of prompts to see what they were really doing.

It is a focused Go evaluator for [agentskills.io](https://agentskills.io)-style repositories. The first version targets fast local skill development and CI gates: run one skill or one eval case, compare `with_skill` against `without_skill`, and fail clearly when a skill does not pass or does not improve enough over baseline.

## Quick Start

```bash
go test ./...
go run ./cmd/skilpel run --root ./skills --skill my-skill --eval-id basic --baseline
```

Model-backed runs need an OpenAI-compatible chat completions endpoint:

```bash
OPENAI_API_KEY=... go run ./cmd/skilpel run \
  --root ./skills \
  --skill my-skill \
  --eval-id basic \
  --workspace ./.skilpel \
  --baseline \
  --target gpt-4o-mini \
  --judge gpt-4o-mini \
  --base-url https://api.openai.com/v1 \
  --min-pass 0.90 \
  --min-delta 0.20
```

## Current Shape

- `run` discovers skills, executes eval cases, writes JSON artifacts, and applies gates.
- `--skill` narrows skill selection by repository-relative path.
- `--eval-id` narrows eval selection by exact ID.
- `--baseline` enables `without_skill` comparison and baseline-delta gates.
- Exit code `0` means the run completed and all configured gates passed.
- Exit code `1` means evals ran but assertions or gates failed.
- Exit code `2` means usage, configuration, filesystem, provider, or runtime failure.

## Prior Art

`skilpel` is inspired by [`agent-skills-eval`](https://github.com/darkrishabh/agent-skills-eval) and agentskills.io-style skill layouts. It deliberately focuses on the subset needed for fast local iteration and CI: skill discovery, eval-case filtering, OpenAI-compatible model calls, baseline comparison, and explicit pass/fail thresholds.
