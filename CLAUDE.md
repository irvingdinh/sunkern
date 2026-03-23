# CLAUDE.md

## Rules of work

Before changing or running the project:

- **Check the Makefile first.** It lists the supported workflows (for example `make run`). Prefer those commands over guessing paths or inventing new ones, unless the task requires something different.
- Load the go-best-practices skill.

## Reference

All reference materials live under `.idea/github.com/` (gitignored). **Do NOT read these into the main conversation context.** Instead, spawn an Explore subagent to discover and read from them on-demand.

When you need to reference a dependency not yet cloned locally, shallow-clone it into `.idea/github.com/{owner}/{repo}` and update this section.

| Repository | Description |
|---|---|
| `spf13/viper` | Configuration library used by the project |
