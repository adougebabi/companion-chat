package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReplyCognitionSnapshotKeepsAgentScopeAndLegacyBehavior(t *testing.T) {
	assignment := providerAssignment{Role: conversationReplyModelRole, EndpointID: "endpoint", BaseURL: "http://fixture.invalid", ModelID: "original", Secret: "private-fixture", Timeout: time.Second}
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationCognition, replyCognitionSnapshotVariant)
	ctx = withFrozenProviderAssignment(ctx, replyCognitionSnapshotRole, assignment)
	snapshot, ok := replyCognitionSnapshotFromContext(ctx)
	if !ok || snapshot.Assignment.ModelID != "original" || snapshot.Assignment.Secret != "" {
		t.Fatal("conversation cognition binding was not captured without its secret")
	}
	wakeCtx := withAgentModelRoute(context.Background(), FormalAgentWakeUp, "wake_up_response")
	wakeCtx = withFrozenProviderAssignment(wakeCtx, replyCognitionSnapshotRole, assignment)
	if _, ok := replyCognitionSnapshotFromContext(wakeCtx); ok {
		t.Fatal("WakeUp binding was mistaken for conversation cognition")
	}
	app := &App{}
	if _, found, err := app.restoreReplyCognitionSnapshot(context.Background(), map[string]any{}); err != nil || found {
		t.Fatalf("legacy draft no longer follows its prior route: found=%t err=%v", found, err)
	}
	if _, found, err := app.restoreReplyCognitionSnapshot(context.Background(), map[string]any{replyCognitionSnapshotKey: map[string]any{"version": "invalid"}}); !found || !errors.Is(err, errReplyCognitionSnapshotInvalid) {
		t.Fatalf("invalid cognition snapshot silently fell back: found=%t err=%v", found, err)
	}
}
