# Custom adapters

Define community runtimes in YAML — no Go required. Place adapter files in `.jerry/adapters/` and Jerry loads them alongside built-in adapters.

## Schema

```yaml
# .jerry/adapters/goose.yaml
name: goose
command: goose run --quiet --output-format json
prompt: stdin
parse:
  text: "result.text"
  cost: "usage.cost"
  input_tokens: "usage.input"
  output_tokens: "usage.output"
capabilities:
  structured_output: false
  cost_reporting: true
  permissions: false
```

## Fields

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Runtime name used in `runtime:` step field |
| `command` | string | yes | Base command to spawn (split on spaces, no shell) |
| `prompt` | string | no | How the prompt reaches the runtime. Default: `arg` |
| `parse.text` | string | yes | Dot-path to extract text from JSON stdout |
| `parse.cost` | string | no | Dot-path to extract cost (USD float) |
| `parse.input_tokens` | string | no | Dot-path to extract input token count |
| `parse.output_tokens` | string | no | Dot-path to extract output token count |
| `capabilities.structured_output` | bool | no | Whether the runtime supports native schema output |
| `capabilities.cost_reporting` | bool | no | Whether cost/token paths are populated |
| `capabilities.permissions` | bool | no | Whether the runtime accepts tool restrictions |

## Prompt delivery modes

| Mode | Behavior |
|---|---|
| `arg` (default) | Append prompt as the last positional argument |
| `stdin` | Write prompt to the child's stdin |
| `file:<flag>` | Write prompt to a temp file, pass path via `<flag>` |

Examples:

```yaml
prompt: arg                    # command "prompt text here"
prompt: stdin                  # echo "prompt" | command
prompt: file:--prompt-file     # command --prompt-file /tmp/jerry-prompt-xyz.md
```

## Output parsing

The runtime must write JSON to stdout. Jerry parses it and extracts fields using dot-path expressions — the same grammar as `${{ trigger.raw.<path> }}`:

```
result.text              → string value
usage.cost               → float
usage.tokens[0]          → array index
response.data.answer     → nested object
```

No JSONPath, no wildcards, no filters. Dot segments + integer indexes only.

## Example: Aider adapter

```yaml
# .jerry/adapters/aider.yaml
name: aider
command: aider --yes --no-git --message
prompt: arg
parse:
  text: "output"
capabilities:
  structured_output: false
  cost_reporting: false
  permissions: false
```

Use in a workflow:

```yaml
steps:
  - name: implement
    prompt: generator.md
    runtime: aider
```

## Example: Codex adapter

```yaml
# .jerry/adapters/codex.yaml
name: codex
command: codex --quiet --json
prompt: stdin
parse:
  text: "response"
  cost: "usage.cost"
  input_tokens: "usage.input_tokens"
  output_tokens: "usage.output_tokens"
capabilities:
  structured_output: false
  cost_reporting: true
  permissions: false
```

## How it works

1. `spec.LoadProject()` reads `.jerry/adapters/*.yaml`
2. Each adapter spec is parsed and validated (name, command, parse.text required)
3. `runtime.NewCustom()` creates an `Adapter` implementation per spec
4. The adapter is registered in the runtime registry alongside built-in adapters
5. Workflow steps using `runtime: <name>` resolve to the custom adapter

## Limitations

- The command is split on spaces — no shell features (pipes, redirects, env vars). Use a wrapper script if needed.
- The `--model` flag is always appended when the step declares a model. If your runtime uses a different flag, wrap it.
- Capabilities are self-declared and trusted. If you claim `cost_reporting: true` but the parse paths are wrong, usage will be zero.
