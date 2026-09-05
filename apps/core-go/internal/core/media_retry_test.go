package core

import (
	"errors"
	"strings"
	"testing"
)

func TestMediaRetryRestartClassifiesPermanentWorkflowErrors(t *testing.T) {
	for _, message := range []string{
		"workflow restart failed: workflow payload is invalid: invalid character",
		"unsupported workflow intent type \"media.generation\"",
		"workflow restart identity is incomplete",
	} {
		if !mediaRetryRestartIsPermanent(errors.New(message)) {
			t.Fatalf("mediaRetryRestartIsPermanent(%q) = false, want true", message)
		}
	}
	if mediaRetryRestartIsPermanent(errors.New("workflow restart failed: temporal unavailable")) {
		t.Fatal("transient Temporal failure must remain retryable")
	}
}

func TestMediaRetryRestartErrorIsBoundedBeforePersistence(t *testing.T) {
	message := strings.Repeat("x", 1400)
	bounded := visualIdentityBoundedText(strings.Join(strings.Fields(message), " "), 1024)
	if len([]rune(bounded)) != 1024 {
		t.Fatalf("bounded restart error length = %d, want 1024", len([]rune(bounded)))
	}
}
