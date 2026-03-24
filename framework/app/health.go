package app

import (
	"context"
	"sync"
	"time"
)

// HealthChecker performs a health check for a named subsystem. Register
// checkers with App.AddHealthCheck during Boot; call App.CheckHealth to
// run them all.
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckFunc adapts a plain function to HealthChecker:
//
//	a.AddHealthCheck(app.CheckFunc{
//	    CheckerName: "cache",
//	    Fn:          cache.Ping,
//	})
type CheckFunc struct {
	CheckerName string
	Fn          func(ctx context.Context) error
}

func (c CheckFunc) Name() string                    { return c.CheckerName }
func (c CheckFunc) Check(ctx context.Context) error { return c.Fn(ctx) }

// ComponentHealth is the result of a single health check.
type ComponentHealth struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
	Took    string `json:"took"`
}

// HealthReport aggregates the results of all registered health checks.
// Serialize directly to JSON for admin API responses.
type HealthReport struct {
	Status     string            `json:"status"` // "healthy" or "unhealthy"
	Components []ComponentHealth `json:"components"`
	Took       string            `json:"took"`
}

// AddHealthCheck registers a health checker. Safe to call from multiple
// goroutines (e.g., during concurrent module Boot in ModuleGroup).
func (a *App) AddHealthCheck(hc HealthChecker) {
	a.healthMu.Lock()
	a.healthCheckers = append(a.healthCheckers, hc)
	a.healthMu.Unlock()
}

// CheckHealth runs all registered health checks concurrently and returns
// an aggregate report. Each check respects the provided context for
// timeouts and cancellation. The report status is "healthy" when all
// components pass, "unhealthy" when any fail, and "unavailable" when
// no checks are registered.
func (a *App) CheckHealth(ctx context.Context) HealthReport {
	start := time.Now()

	a.healthMu.RLock()
	checkers := make([]HealthChecker, len(a.healthCheckers))
	copy(checkers, a.healthCheckers)
	a.healthMu.RUnlock()

	if len(checkers) == 0 {
		return HealthReport{
			Status: "unavailable",
			Took:   time.Since(start).String(),
		}
	}

	components := make([]ComponentHealth, len(checkers))
	var wg sync.WaitGroup
	for i, hc := range checkers {
		wg.Add(1)
		go func(idx int, checker HealthChecker) {
			defer wg.Done()
			t := time.Now()
			err := checker.Check(ctx)
			components[idx] = ComponentHealth{
				Name:    checker.Name(),
				Healthy: err == nil,
				Took:    time.Since(t).String(),
			}
			if err != nil {
				components[idx].Error = err.Error()
			}
		}(i, hc)
	}
	wg.Wait()

	status := "healthy"
	for _, c := range components {
		if !c.Healthy {
			status = "unhealthy"
			break
		}
	}

	return HealthReport{
		Status:     status,
		Components: components,
		Took:       time.Since(start).String(),
	}
}
