package container

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// AppendHook adds a lifecycle hook. Hooks are started in registration order
// and stopped in reverse order.
func (c *Container) AppendHook(h Hook) {
	c.mu.Lock()
	c.hooks = append(c.hooks, h)
	c.mu.Unlock()
}

// Hooks returns a copy of all registered lifecycle hooks.
func (c *Container) Hooks() []Hook {
	c.mu.RLock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.RUnlock()
	return hooks
}

// HookReport describes the outcome of executing a single lifecycle hook.
// Returned by StartHooks and StopHooks for structured startup/shutdown
// logging.
type HookReport struct {
	// Name is the hook's display label (its Name field or positional index).
	Name string `json:"name"`
	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64 `json:"duration_ms"`
	// Err is the error string if the hook failed, empty on success.
	Err string `json:"error,omitempty"`
}

// StartHooks calls OnStart on each hook in registration order and returns
// a report for each executed hook. If a hook fails, all previously-started
// hooks are stopped in reverse order before the error is returned. The
// reports cover only the start phase (rollback results are not included).
func (c *Container) StartHooks(ctx context.Context) ([]HookReport, error) {
	c.mu.RLock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.RUnlock()

	var reports []HookReport
	for i, h := range hooks {
		if h.OnStart == nil {
			continue
		}
		start := time.Now()
		err := h.OnStart(ctx)
		report := HookReport{
			Name:       hookLabel(h, i),
			DurationMs: time.Since(start).Milliseconds(),
		}
		if err != nil {
			report.Err = err.Error()
			reports = append(reports, report)
			// Rollback: stop already-started hooks in reverse.
			_, rollbackErr := stopHooksReverse(ctx, hooks[:i])
			return reports, errors.Join(
				fmt.Errorf("container: starting hook %q: %w", hookLabel(h, i), err),
				rollbackErr,
			)
		}
		reports = append(reports, report)
	}
	return reports, nil
}

// StopHooks calls OnStop on each hook in reverse registration order and
// returns a report for each executed hook. Errors are collected but do not
// prevent subsequent hooks from stopping (best-effort shutdown).
func (c *Container) StopHooks(ctx context.Context) ([]HookReport, error) {
	c.mu.RLock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.RUnlock()

	return stopHooksReverse(ctx, hooks)
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// hookLabel returns a display label for a hook: the Name if set, otherwise
// the positional index.
func hookLabel(h Hook, index int) string {
	if h.Name != "" {
		return h.Name
	}
	return fmt.Sprintf("%d", index)
}

func stopHooksReverse(ctx context.Context, hooks []Hook) ([]HookReport, error) {
	var reports []HookReport
	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		if h.OnStop == nil {
			continue
		}
		start := time.Now()
		err := h.OnStop(ctx)
		report := HookReport{
			Name:       hookLabel(h, i),
			DurationMs: time.Since(start).Milliseconds(),
		}
		if err != nil {
			report.Err = err.Error()
			errs = append(errs, fmt.Errorf("container: stopping hook %q: %w", hookLabel(h, i), err))
		}
		reports = append(reports, report)
	}
	return reports, errors.Join(errs...)
}
