---
title: Use Go Fuzzing for Edge Case Discovery
impact: MEDIUM
impactDescription: discovers edge cases automatically
tags: testing, fuzzing, security, inputs
---

## Use Go Fuzzing for Edge Case Discovery

**Impact: MEDIUM (discovers edge cases automatically)**

Go 1.18+ includes built-in fuzz testing. The fuzzer generates random inputs and feeds them to your function, looking for panics, crashes, or unexpected behavior. It is especially valuable for parsers, validators, encoders/decoders, and any function that processes untrusted input.

Fuzz tests start with seed inputs (`f.Add`) and then mutate them automatically. When the fuzzer finds a failing input, it saves it to `testdata/fuzz/` so it becomes a permanent regression test.

**Incorrect (what's wrong):**

```go
func TestParse(t *testing.T) {
	// Limited manual inputs — only tests cases you thought of
	tests := []string{"valid", "also-valid"}
	for _, s := range tests {
		result, err := Parse(s)
		if err != nil {
			t.Errorf("Parse(%q) failed: %v", s, err)
		}
		_ = result
	}
	// Never tests: "", "\x00", very long strings, unicode edge cases, etc.
}
```

**Correct (what's right):**

```go
// Fuzz test discovers inputs you never thought of
func FuzzParse(f *testing.F) {
	// Seed corpus — starting points for mutation
	f.Add("valid")
	f.Add("also-valid")
	f.Add("")
	f.Add("{}")

	f.Fuzz(func(t *testing.T, s string) {
		// The fuzzer will generate thousands of random variations
		result, err := Parse(s)
		if err != nil {
			return // Errors are fine — we're looking for panics/crashes
		}
		// Optionally verify invariants on successful parses
		if result == nil {
			t.Error("Parse returned nil result without error")
		}
	})
}

// Run fuzzing:
// go test -fuzz=FuzzParse                    # Run until stopped
// go test -fuzz=FuzzParse -fuzztime=30s      # Run for 30 seconds
// go test -fuzz=FuzzParse -fuzztime=10000x   # Run 10000 iterations

// Failing inputs are saved to testdata/fuzz/FuzzParse/
// and automatically included in future test runs
```
