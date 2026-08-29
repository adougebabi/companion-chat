package platform

import (
	"context"
	"errors"
	"testing"
)

func TestHealthConstructorsKeepStableEnvelope(t *testing.T) {
	if got := Live(RoleBFF); got.Status != "ok" || got.Role != RoleBFF {
		t.Fatalf("Live() = %#v", got)
	}
	if got := Ready(RoleCore); got.Status != "ready" || got.Role != RoleCore {
		t.Fatalf("Ready() = %#v", got)
	}
	if got := Unavailable(RoleBFF); got.Status != "unavailable" || got.Role != RoleBFF {
		t.Fatalf("Unavailable() = %#v", got)
	}
}

func TestIsReadyRunsProbeAndHidesFailureDetails(t *testing.T) {
	called := false
	if !IsReady(context.Background(), func(ctx context.Context) error {
		called = true
		return nil
	}) {
		t.Fatal("IsReady(success) = false")
	}
	if !called {
		t.Fatal("probe was not called")
	}

	sentinel := errors.New("database credentials must stay private")
	if IsReady(context.Background(), func(context.Context) error { return sentinel }) {
		t.Fatal("IsReady(failure) = true")
	}
	if IsReady(context.Background(), nil) {
		t.Fatal("IsReady(nil) = true")
	}
}

func TestIsReadyUsesBackgroundContextForNilContext(t *testing.T) {
	if !IsReady(nil, func(ctx context.Context) error {
		if ctx == nil {
			t.Fatal("probe context is nil")
		}
		return nil
	}) {
		t.Fatal("IsReady(nil context) = false")
	}
}
