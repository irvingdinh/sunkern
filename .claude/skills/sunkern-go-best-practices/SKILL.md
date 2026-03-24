---
name: sunkern-go-best-practices
description: Guide for AI agents working with the Sunkern Go framework. Covers verified, stable framework packages. This skill should be used when writing, reviewing, or modifying any Go code in a Sunkern-based project. Triggers on tasks involving Sunkern framework packages.
license: MIT
metadata:
  author: Irving Dinh <irving.dinh@gmail.com>
  version: "0.1.0"
  status: in-progress
---

# Sunkern Go Framework Guide

Guide for working with the Sunkern framework. Only verified and stable packages are documented here. This skill will grow as more packages are finalized.

## Status

| Package | Status | Rule File |
|---------|--------|-----------|
| `framework/config` | Stable | `rules/framework-config.md` |
| `framework/log` | Stable | `rules/framework-log.md` |
| `framework/container` | Not yet documented | - |
| `framework/app` | Not yet documented | - |
| `framework/http` | Not yet documented | - |

## When to Apply

Reference these guidelines when:
- Reading or setting configuration values anywhere in the codebase
- Adding structured logging to handlers, middleware, or business logic
- Writing new modules that need config defaults or log output

## Quick Reference

### Configuration (HIGH)

- `config-resolution-order` - Priority: env var > config.json > SetDefault (highest to lowest)
- `config-set-default` - Call SetDefault in module Register phase; lowest priority, last call wins
- `config-get-required` - Get[T] panics if key missing or coercion fails; use for required config
- `config-get-optional` - GetOr[T] returns fallback on missing key or coercion failure; never panics
- `config-ensure` - Ensure(keys...) validates required keys exist at boot; panics with env var names
- `config-key-naming` - Dot-notation keys map to UPPER_SNAKE env vars: "db.host" -> DB_HOST
- `config-type-coercion` - Supports all Go scalar types, time.Time, time.Duration, slices, maps
- `config-data-dir` - Always set DATA_DIR to an isolated temp directory; never use default ~/.standalone

### Logging (HIGH)

- `log-use-slog` - Always use stdlib log/slog; never fmt.Println, log.Println, or third-party loggers
- `log-context-always` - Use InfoContext/WarnContext/ErrorContext to propagate request_id and user_id
- `log-structured-attrs` - Use key-value pairs or slog.Attr, never string interpolation in messages
- `log-level-semantics` - DEBUG=trace, INFO=normal ops, WARN=degraded, ERROR=needs attention
- `log-dual-output` - Console: pretty JSON (2-space indent); File: compact JSONL. Both have source location and identical structure
- `log-request-id` - log.WithRequestID(ctx, id) stores ID; contextHandler auto-injects into all logs
- `log-user-id` - log.WithUserID(ctx, id) stores ID; auto-injected same as request_id
- `log-dynamic-level` - Resolve *slog.LevelVar from container to change level at runtime

## How to Use

Read individual rule files for detailed API reference and code examples:

```
rules/framework-config.md
rules/framework-log.md
```
