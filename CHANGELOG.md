# Changelog

All notable changes to this project are documented here.

## Unreleased

### Added

- Create the initial `skilpel` Go CLI with `run` support.
- Add skill and eval-case filtering through `--skill` and `--eval-id`.
- Add OpenAI-compatible target and judge model calls.
- Add `with_skill` and `without_skill` baseline comparison.
- Add pass-rate and baseline-delta gates with CI-friendly exit codes.
- Add JSON artifacts for run summaries, eval results, prompts, outputs, timings, and grading.
- Add path containment for attached eval files.
- Add deterministic tests with fake providers.

### Fixed

- Print usage and exit successfully for `skilpel run --help`.
- Skip auto-discovered skills that do not match a workspace-wide `--eval-id` filter.
- Extract fenced judge JSON even when the model includes preamble text.
- Preserve HTTP status context for non-JSON provider error responses.
- Treat malformed judge JSON as failed assertions with diagnostic evidence.

### Documentation

- Add README usage, exit-code behavior, and prior-art links.
