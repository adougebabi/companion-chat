package core

import (
	"container/heap"
	"context"
	"encoding/json"
	"errors"
	"sync"
)

const (
	providerQueueDefaultConcurrency = 2
	providerQueueDefaultEmbedding   = 1
	providerQueueMinConcurrency     = 1
	providerQueueMaxConcurrency     = 8
)

type providerQueueClass string

const (
	providerQueueGenerated providerQueueClass = "generated"
	providerQueueEmbedding providerQueueClass = "embedding"
)

type providerQueueTask struct {
	priority int
	sequence uint64
	ctx      context.Context
	run      func(context.Context) error
	onState  func(string, error)
	done     chan error
	canceled bool
	index    int
}

type providerTaskHeap []*providerQueueTask

func (h providerTaskHeap) Len() int { return len(h) }
func (h providerTaskHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].sequence < h[j].sequence
}
func (h providerTaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *providerTaskHeap) Push(value any) {
	task := value.(*providerQueueTask)
	task.index = len(*h)
	*h = append(*h, task)
}
func (h *providerTaskHeap) Pop() any {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*h = old[:n-1]
	return task
}

// providerQueue is an in-process priority/FIFO executor. Persistence and
// lifecycle diagnostics are supplied by onState; keeping this primitive free
// of SQL makes cancellation and ordering independently testable.
type providerQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	pending providerTaskHeap
	limit   int
	running int
	closed  bool
	seq     uint64
}

func newProviderQueue(limit int) *providerQueue {
	queue := &providerQueue{limit: clampProviderConcurrency(limit)}
	queue.cond = sync.NewCond(&queue.mu)
	heap.Init(&queue.pending)
	for index := 0; index < providerQueueMaxConcurrency; index++ {
		go queue.worker()
	}
	return queue
}

func clampProviderConcurrency(value int) int {
	if value < providerQueueMinConcurrency {
		return providerQueueMinConcurrency
	}
	if value > providerQueueMaxConcurrency {
		return providerQueueMaxConcurrency
	}
	return value
}

func (q *providerQueue) setLimit(value int) {
	q.mu.Lock()
	q.limit = clampProviderConcurrency(value)
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *providerQueue) submit(ctx context.Context, priority int, run func(context.Context) error, onState func(string, error)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		if onState != nil {
			onState(providerRunCancelled, err)
		}
		return err
	}
	task := &providerQueueTask{priority: priority, ctx: ctx, run: run, onState: onState, done: make(chan error, 1)}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		err := errors.New("provider_queue_closed")
		if onState != nil {
			onState(providerRunFailed, err)
		}
		return err
	}
	q.seq++
	task.sequence = q.seq
	heap.Push(&q.pending, task)
	q.cond.Signal()
	q.mu.Unlock()

	select {
	case err := <-task.done:
		return err
	case <-ctx.Done():
		q.cancel(task)
		return ctx.Err()
	}
}

func (q *providerQueue) cancel(task *providerQueueTask) {
	q.mu.Lock()
	if task.index >= 0 && task.index < len(q.pending) && q.pending[task.index] == task {
		task.canceled = true
		heap.Remove(&q.pending, task.index)
		q.mu.Unlock()
		if task.onState != nil {
			task.onState(providerRunCancelled, task.ctx.Err())
		}
		return
	}
	task.canceled = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *providerQueue) worker() {
	for {
		q.mu.Lock()
		for !q.closed && (q.pending.Len() == 0 || q.running >= q.limit) {
			q.cond.Wait()
		}
		if q.closed {
			q.mu.Unlock()
			return
		}
		task := heap.Pop(&q.pending).(*providerQueueTask)
		if task.canceled {
			q.mu.Unlock()
			continue
		}
		q.running++
		// Wake another waiter while capacity remains; a single submit signal
		// must not serialize a queue configured for multiple concurrent calls.
		q.cond.Broadcast()
		q.mu.Unlock()

		var err error
		if task.ctx.Err() != nil {
			err = task.ctx.Err()
			if task.onState != nil {
				task.onState(providerRunCancelled, err)
			}
		} else {
			if task.onState != nil {
				task.onState(providerRunRunning, nil)
			}
			err = task.run(task.ctx)
			if task.onState != nil {
				task.onState(providerRunStatusForError(err), err)
			}
		}
		task.done <- err

		q.mu.Lock()
		q.running--
		q.cond.Broadcast()
		q.mu.Unlock()
	}
}

func (q *providerQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

type providerQueueSettings struct {
	GeneratedConcurrency int `json:"generated_concurrency"`
	EmbeddingConcurrency int `json:"embedding_concurrency"`
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
	p.refreshQueueLimits(ctx)
	queue := p.queueFor(role)
	err := queue.submit(ctx, priority, func(runCtx context.Context) error {
		if guard := providerExecutionGuard(runCtx); guard != nil {
			if guardErr := guard(runCtx); guardErr != nil {
				return guardErr
			}
		}
		var runErr error
		result, runErr = fn(runCtx)
		return runErr
	}, func(status string, runErr error) {
		if p.DB == nil || diagnosticID == "" {
			return
		}
		(&App{DB: p.DB}).updateModelRunState(ctx, diagnosticID, status, runErr)
	})
	return result, err
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
