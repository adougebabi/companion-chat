package core

import "context"

type providerCapacityKey struct{}
type providerCapacity struct {
	class string
	limit int
}

func withProviderCapacity(ctx context.Context, class string, limit int) context.Context {
	return context.WithValue(ctx, providerCapacityKey{}, providerCapacity{class: class, limit: limit})
}

// Task capacity reuses the existing local queue and Redis lease, then acquires
// shared generated capacity. No physical parent permit spans Tool execution.
func (p *ProviderClient) capacityQueue(class string, limit int) *providerQueue {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	if p.capacities == nil {
		p.capacities = map[string]*providerQueue{}
	}
	queue := p.capacities[class]
	if queue == nil {
		queue = newProviderQueue(limit)
		p.capacities[class] = queue
	}
	queue.setLimit(limit)
	return queue
}

func runProviderWithCapacity[T any](p *ProviderClient, ctx context.Context, capacity providerCapacity, role, scenario string, priority int, diagnosticID string, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	release, enabled, err := p.acquireProviderRedisSlotForBinding(ctx, role, "capacity-"+capacity.class, priority, capacity.limit, diagnosticID)
	if err != nil {
		return zero, err
	}
	if enabled {
		defer release()
	}
	ctx = context.WithValue(ctx, providerCapacityKey{}, providerCapacity{})
	ready := make(chan T, 1)
	err = p.capacityQueue(capacity.class, capacity.limit).submit(ctx, priority, func(taskCtx context.Context) error {
		value, err := runProviderQueued(p, taskCtx, role, scenario, priority, diagnosticID, fn)
		ready <- value
		return err
	}, nil)
	if err != nil {
		return zero, err
	}
	return <-ready, nil
}
