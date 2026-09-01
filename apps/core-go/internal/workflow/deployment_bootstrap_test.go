package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.temporal.io/sdk/client"
)

type fakeDeploymentVersionSetter struct {
	remainingFailures int
	lastBuildID       string
	calls             int
}

func (fake *fakeDeploymentVersionSetter) SetCurrentVersion(_ context.Context, options client.WorkerDeploymentSetCurrentVersionOptions) (client.WorkerDeploymentSetCurrentVersionResponse, error) {
	fake.calls++
	fake.lastBuildID = options.BuildID
	if fake.remainingFailures > 0 {
		fake.remainingFailures--
		return client.WorkerDeploymentSetCurrentVersionResponse{}, errors.New("pollers are not ready")
	}
	return client.WorkerDeploymentSetCurrentVersionResponse{}, nil
}

func TestEnsureWorkerDeploymentCurrentVersionRetriesUntilPollersReady(t *testing.T) {
	fake := &fakeDeploymentVersionSetter{remainingFailures: 2}
	if err := ensureWorkerDeploymentCurrentVersion(context.Background(), fake, "platform-v1", time.Millisecond, 3); err != nil {
		t.Fatalf("ensure worker deployment current version = %v", err)
	}
	if fake.calls != 3 || fake.lastBuildID != "platform-v1" {
		t.Fatalf("calls=%d build_id=%q, want 3/platform-v1", fake.calls, fake.lastBuildID)
	}
}

func TestEnsureWorkerDeploymentCurrentVersionRejectsEmptyBuildID(t *testing.T) {
	fake := &fakeDeploymentVersionSetter{}
	if err := ensureWorkerDeploymentCurrentVersion(context.Background(), fake, "  ", 0, 1); err == nil {
		t.Fatal("expected empty build ID error")
	}
	if fake.calls != 0 {
		t.Fatalf("SetCurrentVersion calls = %d, want 0", fake.calls)
	}
}

func TestEnsureWorkerDeploymentCurrentVersionHonorsCancellation(t *testing.T) {
	fake := &fakeDeploymentVersionSetter{remainingFailures: 1}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := ensureWorkerDeploymentCurrentVersion(ctx, fake, "platform-v1", time.Hour, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}
