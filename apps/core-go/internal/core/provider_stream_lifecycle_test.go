package core

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestRunProviderQueuedStreamHoldsLeaseUntilEOF(t *testing.T) {
	provider := &ProviderClient{generated: newProviderQueue(1)}
	firstReader, firstWriter := schema.Pipe[*schema.Message](1)
	firstStarted := make(chan struct{})
	secondStarted := make(chan struct{})
	firstOut, err := runProviderQueuedStream(provider, context.Background(), "generic_llm", "stream-test", 1, "", func(context.Context) (*schema.StreamReader[*schema.Message], error) {
		close(firstStarted)
		return firstReader, nil
	})
	if err != nil {
		t.Fatalf("first stream setup: %v", err)
	}
	if !waitForStreamSignal(firstStarted) {
		t.Fatal("first stream never acquired queue")
	}
	if firstWriter.Send(&schema.Message{Role: schema.Assistant, Content: "first"}, nil) {
		t.Fatal("first stream closed before consumer read")
	}
	if message, recvErr := firstOut.Recv(); recvErr != nil || message == nil || message.Content != "first" {
		t.Fatalf("first stream message = %#v, err=%v", message, recvErr)
	}

	secondReader, err := runProviderQueuedStream(provider, context.Background(), "generic_llm", "stream-test", 1, "", func(context.Context) (*schema.StreamReader[*schema.Message], error) {
		close(secondStarted)
		return schema.StreamReaderFromArray([]*schema.Message{{Role: schema.Assistant, Content: "second"}}), nil
	})
	if err != nil {
		t.Fatalf("second stream setup: %v", err)
	}
	select {
	case <-secondStarted:
		t.Fatal("second stream acquired queue before first stream EOF")
	case <-time.After(50 * time.Millisecond):
	}
	firstWriter.Close()
	if _, recvErr := firstOut.Recv(); !errors.Is(recvErr, io.EOF) {
		t.Fatalf("first stream EOF error = %v", recvErr)
	}
	if !waitForStreamSignal(secondStarted) {
		t.Fatal("second stream did not acquire queue after first EOF")
	}
	message, recvErr := secondReader.Recv()
	if recvErr != nil || message == nil || message.Content != "second" {
		t.Fatalf("second stream message = %#v, err=%v", message, recvErr)
	}
}

func TestRunProviderQueuedStreamCancellationClosesInnerReader(t *testing.T) {
	provider := &ProviderClient{generated: newProviderQueue(1)}
	innerReader, innerWriter := schema.Pipe[*schema.Message](1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, err := runProviderQueuedStream(provider, ctx, "generic_llm", "stream-cancel-test", 1, "", func(context.Context) (*schema.StreamReader[*schema.Message], error) {
		return innerReader, nil
	})
	if err != nil {
		t.Fatalf("stream setup: %v", err)
	}
	cancel()
	_, recvErr := reader.Recv()
	if recvErr == nil {
		t.Fatal("cancelled stream returned a successful message")
	}
	innerWriter.Close()
}

func waitForStreamSignal(signal <-chan struct{}) bool {
	select {
	case <-signal:
		return true
	case <-time.After(time.Second):
		return false
	}
}
