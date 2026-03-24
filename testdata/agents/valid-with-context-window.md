---
name: test-context-window-agent
model: claude-sonnet-4-6
context_window: 200000
context_access:
  - trigger
output_key: result
output_schema:
  summary: string
tools:
  - read_file
---

# Context Window Agent

You are a test agent with a context window hint configured.
