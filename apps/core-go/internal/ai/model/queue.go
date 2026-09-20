package model

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"time"
)

const (
	DefaultConcurrency = 1
	DefaultEmbedding   = 1
	MinConcurrency     = 1
	MaxConcurrency     = 8
	MaximumWait        = 2 * time.Minute

	// Backward-compatible exported aliases matching legacy naming
	ProviderQueueDefaultConcurrency = DefaultConcurrency
	ProviderQueueDefaultEmbedding   = DefaultEmbedding
	ProviderQueueMinConcurrency     = MinConcurrency
	ProviderQueueMaxConcurrency     = MaxConcurrency
	ProviderQueueMaximumWait        = MaximumWait

	RunRunning   = "running"
	RunCompleted = "completed"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
	RunTimeout   = "timeout"
)

type QueueClass string

const (
	QueueGenerated QueueClass = "generated"
	QueueEmbedding QueueClass = "embedding"
)

type QueueTask struct {
	Priority   int
	Sequence   uint64
	EnqueuedAt time.Time
	Ctx        context.Context
	Run        func(context.Context) error
	OnState    func(string, error)
	Done       chan error
	Canceled   bool
	Index      int
}

type TaskHeap []*QueueTask

func (h TaskHeap) Len() int { return len(h) }
func (h TaskHeap) Less(i, j int) bool {
	if h[i].Priority != h[j].Priority {
		return h[i].Priority > h[j].Priority
	}
	return h[i].Sequence < h[j].Sequence
}
func (h TaskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].Index = i
	h[j].Index = j
}
func (h *TaskHeap) Push(value any) {
	task := value.(*QueueTask)
	task.Index = len(*h)
	*h = append(*h, task)
}
func (h *TaskHeap) Pop() any {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.Index = -1
	*h = old[:n-1]
	return task
}

func PopQueueTask(pending *TaskHeap, now time.Time) *QueueTask {
	if pending == nil || pending.Len() == 0 {
		return nil
	}
	agedIndex := -1
	var agedSequence uint64
	for index, task := range *pending {
		if task == nil || task.EnqueuedAt.IsZero() || now.Sub(task.EnqueuedAt) < MaximumWait {
			continue
		}
		if agedIndex < 0 || task.Sequence < agedSequence {
			agedIndex = index
			agedSequence = task.Sequence
		}
	}
	if agedIndex >= 0 {
		return heap.Remove(pending, agedIndex).(*QueueTask)
	}
	return heap.Pop(pending).(*QueueTask)
}

// Queue is an in-process priority/FIFO executor. Persistence and
// lifecycle diagnostics are supplied by onState; keeping this primitive free
// of SQL makes cancellation and ordering independently testable.
type Queue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	pending TaskHeap
	limit   int
	running int
	closed  bool
	seq     uint64
}

// Type aliases for legacy names
type ProviderQueue = Queue
type ProviderQueueTask = QueueTask
type ProviderTaskHeap = TaskHeap

func NewProviderQueue(limit int) *Queue {
	queue := &Queue{limit: ClampProviderConcurrency(limit)}
	queue.cond = sync.NewCond(&queue.mu)
	heap.Init(&queue.pending)
	for index := 0; index < MaxConcurrency; index++ {
		go queue.worker()
	}
	return queue
}

func ClampProviderConcurrency(value int) int {
	if value < MinConcurrency {
		return MinConcurrency
	}
	if value > MaxConcurrency {
		return MaxConcurrency
	}
	return value
}

func (q *Queue) SetLimit(value int) {
	q.mu.Lock()
	q.limit = ClampProviderConcurrency(value)
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *Queue) CurrentLimit() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.limit
}

func (q *Queue) Submit(ctx context.Context, priority int, run func(context.Context) error, onState func(string, error)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		if onState != nil {
			onState(RunCancelled, err)
		}
		return err
	}
	task := &QueueTask{
		Priority:   priority,
		EnqueuedAt: time.Now().UTC(),
		Ctx:        ctx,
		Run:        run,
		OnState:    onState,
		Done:       make(chan error, 1),
	}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		err := errors.New("provider_queue_closed")
		if onState != nil {
			onState(RunFailed, err)
		}
		return err
	}
	q.seq++
	task.Sequence = q.seq
	heap.Push(&q.pending, task)
	q.cond.Signal()
	q.mu.Unlock()

	select {
	case err := <-task.Done:
		return err
	case <-ctx.Done():
		q.cancel(task)
		return ctx.Err()
	}
}

func (q *Queue) cancel(task *QueueTask) {
	q.mu.Lock()
	if task.Index >= 0 && task.Index < len(q.pending) && q.pending[task.Index] == task {
		task.Canceled = true
		heap.Remove(&q.pending, task.Index)
		q.mu.Unlock()
		if task.OnState != nil {
			task.OnState(RunCancelled, task.Ctx.Err())
		}
		return
	}
	task.Canceled = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func (q *Queue) worker() {
	for {
		q.mu.Lock()
		for !q.closed && (q.pending.Len() == 0 || q.running >= q.limit) {
			q.cond.Wait()
		}
		if q.closed {
			q.mu.Unlock()
			return
		}
		task := PopQueueTask(&q.pending, time.Now().UTC())
		if task == nil || task.Canceled {
			q.mu.Unlock()
			continue
		}
		q.running++
		// Wake another waiter while capacity remains; a single submit signal
		// must not serialize a queue configured for multiple concurrent calls.
		q.cond.Broadcast()
		q.mu.Unlock()

		var err error
		if task.Ctx.Err() != nil {
			err = task.Ctx.Err()
			if task.OnState != nil {
				task.OnState(RunCancelled, err)
			}
		} else {
			if task.OnState != nil {
				task.OnState(RunRunning, nil)
			}
			err = task.Run(task.Ctx)
			if task.OnState != nil {
				task.OnState(StatusForError(err), err)
			}
		}
		task.Done <- err

		q.mu.Lock()
		q.running--
		q.cond.Broadcast()
		q.mu.Unlock()
	}
}

func (q *Queue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast()
	q.mu.Unlock()
}

func StatusForError(err error) string {
	if err == nil {
		return RunCompleted
	}
	if errors.Is(err, context.Canceled) {
		return RunCancelled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return RunTimeout
	}
	return RunFailed
}
