package core

import (
	"testing"
)

func TestMemoryServiceInitialization(t *testing.T) {
	service := NewMemoryService(nil, nil)
	if service == nil {
		t.Fatal("expected non-nil MemoryService")
	}
}
