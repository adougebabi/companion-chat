package core

import (
	"testing"
)

func TestCognitionServiceInitialization(t *testing.T) {
	service := NewCognitionService(nil, nil)
	if service == nil {
		t.Fatal("expected non-nil CognitionService")
	}
}
