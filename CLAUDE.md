# CLAUDE.md

Sunkern is an opinionated template repository — a single Go + React stack that AI agents fork to build deployed products from a product spec alone. The framework enforces all architectural decisions so the agent never needs to make stack or pattern choices.

## Bootstrap (mandatory, do this BEFORE responding to ANY user message)

You MUST complete ALL of the following steps at the start of every conversation, before answering the user's first message. No exceptions — even if the user's question seems simple. Do not ask the user whether you should do these; just do them silently.

1. **Read `IDEA.md`** — this is the architecture and design bible. You cannot work in this repo without understanding it.
2. **Read `Makefile`** — use the workflows defined here. Do not invent commands.
3. **Load the `go-best-practices` skill** — always, regardless of the task.
4. **Explore the repository** — walk the actual file tree and read key files to understand the current state of the implementation. Documents and design notes can be outdated or aspirational. The code is the source of truth. Do not assume something exists because IDEA.md says it will — verify by reading the code.

## Testing and verification

These rules apply whenever you implement, change, or test anything. Always include these steps in your plan.

- **Use an isolated data directory.** Multiple Sunkern-based projects may run on this machine simultaneously. NEVER use the default `~/.standalone`. Instead, create a temporary directory (e.g. `DATA_DIR=/tmp/sunkern_data_<random>`) at the start of each session and use it for all runs. This prevents data collisions between concurrent projects.
- **Start the service and test with `curl`.** After making any changes, you MUST start the service and use `curl` to verify the changes work against the real running application. No exceptions.

## Reference materials

All reference materials live under `.idea/github.com/` (gitignored). **Do NOT read these into the main conversation context.** Instead, spawn an Explore subagent to discover and read from them on-demand.

When you need to reference a dependency not yet cloned locally, shallow-clone it into `.idea/github.com/{owner}/{repo}` and update this section.

| Repository | Description |
|---|---|
| `spf13/viper` | Configuration library used by the project |
