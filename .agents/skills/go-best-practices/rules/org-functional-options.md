---
title: Use Functional Options for API Configuration
impact: MEDIUM
impactDescription: Config structs with ambiguous zero values lead to unclear defaults and no validation
tags: options, api-design, configuration
---

## Use Functional Options for API Configuration

**Impact: MEDIUM (Config structs with ambiguous zero values lead to unclear defaults and no validation)**

Use the functional options pattern for flexible, API-friendly configuration. A plain config struct cannot distinguish between "not set" and "set to zero value," and it cannot validate individual options at the point of use. Functional options solve both problems: each option is a self-contained function that can validate its input and use pointer fields or flags to distinguish missing from zero.

**Incorrect (what's wrong):**

```go
type Config struct {
    Port int
}

func NewServer(addr string, cfg Config) (*http.Server, error) {
    // Is cfg.Port == 0 intentional or just the zero value?
    port := cfg.Port
    if port == 0 {
        port = 8080 // guess a default
    }
    return &http.Server{Addr: fmt.Sprintf("%s:%d", addr, port)}, nil
}

// Caller: NewServer("localhost", Config{}) — ambiguous
```

**Correct (what's right):**

```go
type options struct {
    port *int
}

type Option func(*options) error

func WithPort(port int) Option {
    return func(o *options) error {
        if port < 0 {
            return errors.New("port should be positive")
        }
        o.port = &port
        return nil
    }
}

func NewServer(addr string, opts ...Option) (*http.Server, error) {
    var o options
    for _, opt := range opts {
        if err := opt(&o); err != nil {
            return nil, fmt.Errorf("invalid option: %w", err)
        }
    }

    port := 8080 // default
    if o.port != nil {
        port = *o.port
    }

    return &http.Server{Addr: fmt.Sprintf("%s:%d", addr, port)}, nil
}

// Caller: NewServer("localhost", WithPort(9090)) — explicit, validated
```

Reference: "100 Go Mistakes and How to Avoid Them" — Mistake #11.
