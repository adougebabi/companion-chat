package core

import (
	"testing"
)

func TestReflectionServiceInitialization(t *testing.T) {
	service := NewReflectionService(nil, nil)
	if service == nil {
		t.Fatal("expected non-nil ReflectionService")
	}
}
