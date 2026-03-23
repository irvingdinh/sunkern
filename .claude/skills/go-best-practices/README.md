# Go Best Practices

A structured repository for creating and maintaining Go Best Practices optimized for agents and LLMs, based on "100 Go Mistakes and How to Avoid Them" by Teiva Harsanyi.

## Structure

- `rules/` - Individual rule files (one per rule)
  - `_sections.md` - Section metadata (titles, impacts, descriptions)
  - `_template.md` - Template for creating new rules
  - `area-description.md` - Individual rule files
- __`AGENTS.md`__ - Compiled output (generated)
- __`SKILL.md`__ - Skill manifest for Claude Code

## Creating a New Rule

1. Copy `rules/_template.md` to `rules/area-description.md`
2. Choose the appropriate area prefix:
   - `org-` for Code Organization (Section 1)
   - `data-` for Data Types (Section 2)
   - `ctrl-` for Control Structures (Section 3)
   - `string-` for Strings (Section 4)
   - `func-` for Functions & Methods (Section 5)
   - `error-` for Error Management (Section 6)
   - `conc-` for Concurrency (Section 7)
   - `stdlib-` for Standard Library (Section 8)
   - `test-` for Testing (Section 9)
   - `opt-` for Optimizations (Section 10)
3. Fill in the frontmatter and content
4. Ensure you have clear examples with explanations

## Rule File Structure

Each rule file should follow this structure:

```markdown
---
title: Rule Title Here
impact: MEDIUM
impactDescription: Optional description
tags: tag1, tag2, tag3
---

## Rule Title Here

Brief explanation of the rule and why it matters.

**Incorrect (description of what's wrong):**

\```go
// Bad code example
\```

**Correct (description of what's right):**

\```go
// Good code example
\```

Reference: [Link](https://example.com)
```

## File Naming Convention

- Files starting with `_` are special (excluded from build)
- Rule files: `area-description.md` (e.g., `conc-race-problems.md`)
- Section is automatically inferred from filename prefix
- Rules are sorted alphabetically by title within each section
- IDs (e.g., 1.1, 1.2) are auto-generated during build

## Impact Levels

- `CRITICAL` - Highest priority, causes bugs or data races
- `HIGH` - Significant correctness or performance improvements
- `MEDIUM-HIGH` - Moderate-high gains
- `MEDIUM` - Moderate improvements to readability or performance
- `LOW-MEDIUM` - Low-medium gains
- `LOW` - Incremental improvements

## Acknowledgments

Based on "100 Go Mistakes and How to Avoid Them" by Teiva Harsanyi (Manning, 2022). Read online at [100go.co](https://100go.co).
