package core

import (
	"testing"
)

func TestLifeContextServiceInitialization(t *testing.T) {
	service := NewLifeContextService(nil, nil)
	if service == nil {
		t.Fatal("expected non-nil LifeContextService")
	}
}
