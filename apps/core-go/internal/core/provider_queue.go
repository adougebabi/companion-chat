package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/model"
)

const (
	providerQueueDefaultConcurrency = model.DefaultConcurrency
	providerQueueDefaultEmbedding   = model.DefaultEmbedding
	providerQueueMinConcurrency     = model.MinConcurrency
	providerQueueMaxConcurrency     = model.MaxConcurrency
	providerQueueMaximumWait        = model.MaximumWait
)

type providerQueueClass = model.QueueClass

const (
	providerQueueGenerated = model.QueueGenerated
	providerQueueEmbedding = model.QueueEmbedding
)

type providerQueueTask = model.QueueTask
type providerTaskHeap = model.TaskHeap

func popProviderQueueTask(pending *providerTaskHeap, now time.Time) *providerQueueTask {
	return model.PopQueueTask(pending, now)
}

// providerQueue is an in-process priority/FIFO executor wrapping model.Queue.
type providerQueue struct {
	*model.Queue
}

func newProviderQueue(limit int) *providerQueue {
	return &providerQueue{Queue: model.NewProviderQueue(limit)}
}

func clampProviderConcurrency(value int) int {
	return model.ClampProviderConcurrency(value)
}

func (q *providerQueue) setLimit(value int) {
	q.Queue.SetLimit(value)
}

func (q *providerQueue) currentLimit() int {
	return q.Queue.CurrentLimit()
}

func (q *providerQueue) submit(ctx context.Context, priority int, run func(context.Context) error, onState func(string, error)) error {
	return q.Queue.Submit(ctx, priority, run, onState)
}

func (q *providerQueue) close() {
	q.Queue.Close()
}

type providerQueueSettings struct {
	GeneratedConcurrency int `json:"generated_concurrency"`
	EmbeddingConcurrency int `json:"embedding_concurrency"`
}

type providerQueueBypassContextKey struct{}

func withProviderQueueBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, providerQueueBypassContextKey{}, true)
}

func providerQueueBypassed(ctx context.Context) bool {
	value, _ := ctx.Value(providerQueueBypassContextKey{}).(bool)
	return value
}

func withoutProviderQueueBypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, providerQueueBypassContextKey{}, false)
}

func (p *ProviderClient) queueFor(role string) *providerQueue {
	p.queueMu.Lock()
	defer p.queueMu.Unlock()
	if role == "embedding" {
		if p.embedding == nil {
			p.embedding = newProviderQueue(providerQueueDefaultEmbedding)
		}
		return p.embedding
	}
	if p.generated == nil {
		p.generated = newProviderQueue(providerQueueDefaultConcurrency)
	}
	return p.generated
}

func (p *ProviderClient) refreshQueueLimits(ctx context.Context) {
	generated, embedding := providerQueueDefaultConcurrency, providerQueueDefaultEmbedding
	if p != nil && p.DB != nil {
		var raw string
		if err := p.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='llm.queue'`).Scan(&raw); err == nil {
			var settings providerQueueSettings
			if json.Unmarshal([]byte(raw), &settings) == nil {
				if settings.GeneratedConcurrency != 0 {
					generated = settings.GeneratedConcurrency
				}
				if settings.EmbeddingConcurrency != 0 {
					embedding = settings.EmbeddingConcurrency
				}
			}
		}
	}
	p.queueFor("generic_llm").setLimit(generated)
	p.queueFor("embedding").setLimit(embedding)
}

func runProviderQueued[T any](p *ProviderClient, ctx context.Context, role, scenario string, priority int, diagnosticID string, fn func(context.Context) (T, error)) (T, error) {
	var result T
	if p == nil {
		return result, errors.New("provider_unavailable")
	}
	if providerQueueBypassed(ctx) {
		return fn(ctx)
	}
	p.refreshQueueLimits(ctx)
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatch := p.watchProviderCancellation(runCtx, cancel)
	defer stopWatch()
	queue := p.queueFor(role)
	releaseRedis, redisEnabled, redisErr := p.acquireProviderRedisSlot(runCtx, role, priority, queue.currentLimit(), diagnosticID)
	if redisErr != nil {
		if diagnosticID != "" {
			p.runtimeSupport().UpdateModelRunState(ctx, diagnosticID, providerRunFailed, redisErr)
		}
		return result, redisErr
	}
	if redisEnabled {
		defer releaseRedis()
	}
	err := queue.submit(runCtx, priority, func(taskCtx context.Context) error {
		if guard := providerExecutionGuard(taskCtx); guard != nil {
			if guardErr := guard(taskCtx); guardErr != nil {
				return guardErr
			}
		}
		var runErr error
		result, runErr = fn(taskCtx)
		return runErr
	}, func(status string, runErr error) {
		if p.DB == nil || diagnosticID == "" {
			return
		}
		p.runtimeSupport().UpdateModelRunState(ctx, diagnosticID, status, runErr)
	})
	return result, err
}

func (p *ProviderClient) watchProviderCancellation(ctx context.Context, cancel context.CancelFunc) func() {
	marker := providerCancellationMarker(ctx)
	if p == nil || p.redis == nil || marker == "" {
		return func() {}
	}
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		key := providerCognitionCancelPrefix + marker
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if exists, err := p.redis.Exists(context.Background(), key).Result(); err == nil && exists > 0 {
					cancel()
					return
				}
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}

func providerRunStatusForError(err error) string {
	if err == nil {
		return providerRunCompleted
	}
	if errors.Is(err, errProviderPaused) || errors.Is(err, errProviderInactive) {
		return providerRunCancelled
	}
	if errors.Is(err, context.Canceled) {
		return providerRunCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return providerRunTimeout
	}
	return providerRunFailed
}
