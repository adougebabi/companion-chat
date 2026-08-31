package core

import (
	"context"
	"time"
)

// WorkflowRuntime is the narrow management seam used by the Core API. The
// Temporal SDK implementation lives in internal/workflow so domain packages do
// not depend on a concrete client or protobufs.
type WorkflowRuntime interface {
	List(context.Context, string, int) ([]WorkflowExecution, error)
	Status(context.Context, string, string) (WorkflowExecution, error)
	History(context.Context, string, string, int) (WorkflowHistory, error)
	Signal(context.Context, string, string, string, string) error
	Cancel(context.Context, string, string, string) error
	Terminate(context.Context, string, string, string, string) error
	Reset(context.Context, string, string, int64, string, string) (WorkflowExecution, error)
	Restart(context.Context, WorkflowStart) (WorkflowExecution, error)
}

type WorkflowExecution struct {
	WorkflowID    string     `json:"workflow_id"`
	RunID         string     `json:"run_id"`
	WorkflowType  string     `json:"workflow_type"`
	TaskQueue     string     `json:"task_queue"`
	Status        string     `json:"status"`
	StartTime     *time.Time `json:"start_time,omitempty"`
	CloseTime     *time.Time `json:"close_time,omitempty"`
	HistoryLength int64      `json:"history_length,omitempty"`
}

type WorkflowHistory struct {
	WorkflowID string   `json:"workflow_id"`
	RunID      string   `json:"run_id"`
	EventCount int      `json:"event_count"`
	EventTypes []string `json:"event_types"`
}

type WorkflowStart struct {
	WorkflowID string
	TaskQueue  string
	IntentType string
	Payload    []byte
}
