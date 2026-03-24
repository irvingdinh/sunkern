package container

import (
	"context"
	"errors"
	"fmt"
)

// AppendHook adds a lifecycle hook. Hooks are started in registration order
// and stopped in reverse order.
func (c *Container) AppendHook(h Hook) {
	c.mu.Lock()
	c.hooks = append(c.hooks, h)
	c.mu.Unlock()
}

// StartHooks calls OnStart on each hook in registration order. If a hook
// fails, all previously-started hooks are stopped in reverse order before
// the start error is returned.
func (c *Container) StartHooks(ctx context.Context) error {
	c.mu.RLock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.RUnlock()

	for i, h := range hooks {
		if h.OnStart == nil {
			continue
		}
		if err := h.OnStart(ctx); err != nil {
			// Rollback: stop already-started hooks in reverse.
			rollbackErr := stopHooksReverse(ctx, hooks[:i])
			return errors.Join(
				fmt.Errorf("container: starting hook %s: %w", hookLabel(h, i), err),
				rollbackErr,
			)
		}
	}
	return nil
}

// StopHooks calls OnStop on each hook in reverse registration order.
// Errors are collected but do not prevent subsequent hooks from stopping
// (best-effort shutdown).
func (c *Container) StopHooks(ctx context.Context) error {
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
		return fmt.Sprintf("%q", h.Name)
	}
	return fmt.Sprintf("%d", index)
}

func stopHooksReverse(ctx context.Context, hooks []Hook) error {
	var errs []error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		if h.OnStop == nil {
			continue
		}
		if err := h.OnStop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("container: stopping hook %s: %w", hookLabel(h, i), err))
		}
	}
	return errors.Join(errs...)
}
