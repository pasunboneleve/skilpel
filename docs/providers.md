# Model Providers

`skilpel` sends target and judge requests through the provider selected by
`--provider` or `provider` in the config file.

| Provider | API | Default key variable |
| --- | --- | --- |
| `openai` | OpenAI Responses | `OPENAI_API_KEY` |
| `openai-chat` | OpenAI Chat Completions | `OPENAI_API_KEY` |
| `xai` | OpenAI-compatible Chat Completions | `XAI_API_KEY` |
| `qwen` | OpenAI-compatible Chat Completions | `DASHSCOPE_API_KEY` |
| `anthropic` or `claude` | Anthropic Messages | `ANTHROPIC_API_KEY` |
| `gemini` | Gemini Generate Content | `GEMINI_API_KEY` |

Run `skilpel run --help` for the current provider list and base-URL support.

## OpenAI Responses

The default `openai` provider uses OpenAI's Responses API. It sends the skill as
`instructions`, the eval prompt as `input`, and fixes `store: false`; parameter
maps cannot override response storage. Put Responses request fields under
`targetParams` and `judgeParams`:

```yaml
provider: openai
target: gpt-5.6-luna
judge: gpt-5.6-luna
targetParams:
  temperature: 1
  reasoning:
    effort: none
judgeParams:
  temperature: 1
  reasoning:
    effort: none
```

For compatibility with existing configs, skilpel converts `max_tokens` and
`max_completion_tokens` to `max_output_tokens`. It also converts the flat
`reasoning_effort` field to `reasoning.effort`.

The Responses API does not accept `seed`. The `openai` provider rejects that
field before sending a request. Use `openai-chat` only when a model and workflow
require Chat Completions parameters or a compatible endpoint lacks Responses.

## Compatible endpoints

`openai-chat`, `xai`, and `qwen` retain the Chat Completions request shape. Use
their provider-specific parameter names under `targetParams` and `judgeParams`.
`--base-url` remains available for providers listed by `skilpel run --help`.
