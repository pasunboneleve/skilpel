# Changelog

All notable changes to this project are documented here.

## [Unreleased]

## [0.2.0] - 2026-05-24

### Added

- Stream structured progress logs to stderr during eval runs while preserving the final summary JSON on stdout.
- Add `--log-format` to choose automatic, JSON, or pretty terminal progress logs.

### Changed

- Accumulate skill pass-rate totals without retaining per-eval rate slices.

### Fixed

- Fail run gates when any `with_skill` assertion fails, even if the averaged skill pass rate exceeds `--min-pass`.

## [0.1.0] - 2026-05-24

### Added

- Create the initial `skilpel` Go CLI with `run` support.
- Add skill and eval-case filtering through `--skill` and `--eval-id`.
- Add OpenAI-compatible target and judge model calls.
- Add `with_skill` and `without_skill` baseline comparison.
- Add pass-rate and baseline-delta gates with CI-friendly exit codes.
- Add JSON artifacts for run summaries, eval results, prompts, outputs, timings, and grading.
- Add path containment for attached eval files.
- Add deterministic tests with fake providers and a compiled-binary shell-script integration fixture.
- Add YAML eval file support with `evals/evals.yaml` and `evals/evals.yml` precedence before `evals/evals.json`.
- Add GitHub Actions CI for Go formatting and tests.
- Add provider plugins for OpenAI, xAI, Qwen, Anthropic/Claude, and Gemini using standard Go SDKs where available.
- Add expanded CLI help for agents and no-argument runs.
- Add MIT license metadata.
- Add a README CI badge linked to the main-branch workflow runs.
- Add a real OpenAI canary for the compiled shell-script skill workflow.
- Add `skilpel version` and `--version`.
- Add tag-driven release executables for Linux amd64 and macOS arm64 with SHA-256 checksums.

### Fixed

- Print usage and exit successfully for `skilpel run --help`.
- Skip auto-discovered skills that do not match a workspace-wide `--eval-id` filter.
- Extract fenced judge JSON even when the model includes preamble text.
- Match reordered judge results back to assertions by assertion text.
- Allow multi-skill explicit runs when an `--eval-id` matches only some selected skills.
- Report malformed skill frontmatter instead of silently ignoring it.
- Preserve HTTP status context for non-JSON provider error responses.
- Avoid artifact slug collisions for named eval cases that lack IDs.
- Treat malformed judge JSON as failed assertions with diagnostic evidence.

### Documentation

- Add README usage, exit-code behavior, and prior-art links.
