# CLI Output

`skilpel run` has two output surfaces:

- progress logs on stderr
- final summaries on stdout

Keep scripts on stdout by choosing `--output=json`. Keep human feedback on
stderr by choosing `--log-format=pretty` or leaving `--log-format=auto` on an
interactive terminal.

## Progress Logs

`--log-format=json` prints GCP-readable JSON progress events to stderr.

`--log-format=pretty` prints human-readable progress:

- completed eval rows as they finish
- yellow warnings for skipped skills, such as skills without eval files
- red failures for failed eval rows or gates
- a live spinner and progress bar only when stderr is a real terminal

The live spinner is transient. It is cleared before durable rows are printed and
redrawn afterward, so it stays near the active prompt area. Captured logs,
including GitHub Actions logs, do not receive animation frames.

Use `--log-file <path>` when you want structured JSON progress logs as an
artifact while keeping pretty progress visible in the terminal or CI log.

## Final Summaries

`--output=text` is the default. It prints a skill-validator-style final report
to stdout. When pretty progress already printed eval rows, the final text
summary prints only the final divider, gates, and result.

`--output=json` prints the final summary as JSON on stdout. Use this for scripts
and downstream tooling.

`--output=markdown` prints a Markdown summary for release notes, issue comments,
or other prose surfaces.

## Exit Codes

- `0`: the run completed and all configured gates passed
- `1`: evals ran, but assertions or gates failed
- `2`: usage, configuration, filesystem, provider, or runtime failure
