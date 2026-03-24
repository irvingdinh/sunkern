# CLAUDE.md

Sunkern is an opinionated template repository — a single Go + React stack that AI agents fork to build deployed products from a product spec alone. The framework enforces all architectural decisions so the agent never needs to make stack or pattern choices.

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
| `go-gorm/gorm` | Most popular Go ORM — reflection-based, struct tags, method chaining |
| `uptrace/bun` | SQL-first query builder — explicit query objects, struct tags |
| `ent/ent` | Facebook's entity framework — code generation, DSL schema definition |
| `sqlc-dev/sqlc` | Generates type-safe Go from SQL queries — code generation approach |
| `stephenafamo/bob` | Type-safe SQL query builder using Go generics — mod-based API |
| `bokwoon95/sq` | Type-safe SQL query builder — struct-based tables, minimal deps |
| `drizzle-team/drizzle-orm` | SQL-like TypeScript query builder — schema-as-code, SQL-shaped API |
| `kysely-org/kysely` | Type-safe SQL query builder for TypeScript — AST-based, immutable builders |
| `prisma/prisma` | Schema-first ORM with code generation — custom DSL, generated client |
| `laravel/framework` | Laravel's Eloquent ORM (PHP) — Active Record, fluent query builder, scopes |
| `sqlalchemy/sqlalchemy` | Python's SQL toolkit — dual-layer Core (expression builder) + ORM |
| `diesel-rs/diesel` | Rust's type-safe query builder — compile-time validation, schema macros |
