package core

import (
	"testing"
)

func TestMediaServiceInitialization(t *testing.T) {
	service := NewMediaService(nil, nil, "test-bucket", nil)
	if service == nil {
		t.Fatal("expected non-nil MediaService")
	}
	if service.bucket != "test-bucket" {
		t.Fatalf("expected bucket 'test-bucket', got %s", service.bucket)
	}
}
