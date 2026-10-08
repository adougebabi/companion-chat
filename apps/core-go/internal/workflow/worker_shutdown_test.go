package workflow

import (
	"sync"
	"sync/atomic"
	"testing"

	"go.temporal.io/sdk/worker"
)

type countedStopWorker struct {
	worker.Worker
	stops atomic.Int32
}

func (w *countedStopWorker) Stop() { w.stops.Add(1) }

func TestWorkerExplicitAndContextShutdownStopSDKOnce(t *testing.T) {
	sdk := &countedStopWorker{}
	managed := &managedWorker{Worker: sdk}
	var callers sync.WaitGroup
	for i := 0; i < 20; i++ {
		callers.Add(1)
		go func() { defer callers.Done(); managed.Stop() }()
	}
	callers.Wait()
	managed.Stop()
	if sdk.stops.Load() != 1 {
		t.Fatalf("SDK shutdown repeated: %d", sdk.stops.Load())
	}
}
