package main

import (
	"context"
	"testing"
	"time"
)

func TestMaintenanceLaneDoesNotBlockDispatchOrHealth(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slowEntered := make(chan struct{}, 1)
	release := make(chan struct{})
	stopSlow := startWorkerLoop(ctx, time.Millisecond, func(ctx context.Context) {
		select {
		case slowEntered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
	})
	defer stopSlow()
	select {
	case <-slowEntered:
	case <-time.After(time.Second):
		t.Fatal("maintenance did not start")
	}
	dispatched := make(chan struct{}, 1)
	stopDispatch := startWorkerLoop(ctx, time.Millisecond, func(context.Context) {
		select {
		case dispatched <- struct{}{}:
		default:
		}
	})
	defer stopDispatch()
	select {
	case <-dispatched:
	case <-time.After(time.Second):
		t.Fatal("dispatch blocked behind maintenance")
	}
	close(release)
}

func TestWorkerLaneStopJoinsItsSerializedOperation(t *testing.T) {
	entered, exited := make(chan struct{}), make(chan struct{})
	stop := startWorkerLoop(context.Background(), time.Millisecond, func(ctx context.Context) { close(entered); <-ctx.Done(); close(exited) })
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("lane did not start")
	}
	stop()
	select {
	case <-exited:
	default:
		t.Fatal("lane stop returned before its operation released the shared client")
	}
}
