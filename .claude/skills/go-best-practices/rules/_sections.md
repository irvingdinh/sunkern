# Sections

This file defines all sections, their ordering, impact levels, and descriptions.
The section ID (in parentheses) is the filename prefix used to group rules.

---

## 1. Code Organization (org)

**Impact:** MEDIUM
**Description:** Proper code and project organization improves readability, maintainability, and idiomatic Go style. Mistakes here rarely cause bugs but make codebases harder to work with over time.

## 2. Data Types (data)

**Impact:** HIGH
**Description:** Misunderstanding slices, maps, and value semantics leads to memory leaks, unexpected mutations, and subtle bugs in production.

## 3. Control Structures (ctrl)

**Impact:** MEDIUM
**Description:** Go's range loops, break semantics, and defer behavior have gotchas that can cause silent bugs if not understood.

## 4. Strings (string)

**Impact:** MEDIUM
**Description:** Go strings are immutable byte slices with UTF-8 encoding. Misunderstanding runes, iteration, and memory sharing leads to bugs and performance issues.

## 5. Functions & Methods (func)

**Impact:** MEDIUM
**Description:** Receiver types, defer evaluation, and interface return semantics have subtle behaviors that affect correctness and API design.

## 6. Error Management (error)

**Impact:** HIGH
**Description:** Go's explicit error handling is a core strength, but misusing wrapping, comparison, and handling patterns leads to silent failures in production.

## 7. Concurrency (conc)

**Impact:** CRITICAL
**Description:** Concurrency is Go's most powerful feature and its biggest footgun. Data races, goroutine leaks, and deadlocks are the most dangerous class of Go bugs.

## 8. Standard Library (stdlib)

**Impact:** MEDIUM-HIGH
**Description:** Misusing net/http, database/sql, encoding/json, and time leads to resource leaks, incorrect behavior, and production outages.

## 9. Testing (test)

**Impact:** LOW-MEDIUM
**Description:** Writing effective tests, benchmarks, and using Go's testing toolchain correctly improves code quality and catches regressions.

## 10. Optimizations (opt)

**Impact:** LOW-MEDIUM
**Description:** Understanding CPU caches, memory allocation, and profiling tools enables targeted performance improvements in hot paths.
