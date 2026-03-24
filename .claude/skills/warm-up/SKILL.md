---
name: warm-up
description: Load project context into the conversation before substantial work — planning, coding, or reviewing.
---

# /warm-up

Front-load project understanding so subsequent tasks are faster and better informed. Run this before substantial work — skip it for quick questions.

## Steps

Execute in order. Maximize parallelism where noted.

### Step 1 — Load foundation (sequential, main context)

These go into YOUR context directly — you need them for all downstream work:

1. **Read `IDEA.md`** — architecture and design bible for this project.
2. **Read `Makefile`** — defined workflows. Use these; do not invent commands.
3. **Load the `go-best-practices`, `sunkern-go-best-practices` skill** — always, regardless of the task.

### Step 2 — Explore the codebase (parallel subagents)

Spawn parallel **Explore subagents** to map the codebase fast. Each returns a **concise summary** — not raw file contents. This keeps your main context lean.

Launch all of these simultaneously:

| Subagent | Explore what | Report back |
|---|---|---|
| **Backend** | Go packages, entry points (`main.go`, `cmd/`), route/handler definitions, middleware | Package tree, key types, API surface |
| **Frontend** | React components, pages, routing, state management | Component tree, page routes, data flow patterns |
| **Data layer** | Database schemas, migrations, models, storage interfaces | Schema shape, migration status, repository patterns |
| **Config & infra** | Docker, environment config, build and deployment setup | How to build, configure, and run |
| **Recent activity** | `git log --oneline -20`, uncommitted changes, current branch | What's been worked on, any WIP to be aware of |

**Adapt to reality:** if a layer doesn't exist yet (e.g., no frontend, no database), skip that subagent. Don't explore what isn't there.

### Step 3 — Synthesize and report

After all subagents return, produce a brief status report:

- **What this project does** (from IDEA.md, one sentence)
- **Current state** — what's implemented vs. what's still planned
- **Key files** — the handful of files most likely to be touched in upcoming work
- **Gaps or WIP** — anything incomplete or in-progress

This confirms warm-up is done and gives the user a chance to correct misunderstandings before real work begins.
