package core

import (
	"context"
	"errors"
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
	WorkflowID     string     `json:"workflow_id"`
	RunID          string     `json:"run_id"`
	WorkflowType   string     `json:"workflow_type"`
	TaskQueue      string     `json:"task_queue"`
	Status         string     `json:"status"`
	StartTime      *time.Time `json:"start_time,omitempty"`
	CloseTime      *time.Time `json:"close_time,omitempty"`
	ControlActions []string   `json:"control_actions,omitempty"`
	ControlState   string     `json:"control_state,omitempty"`
	OwnerCommandID string     `json:"owner_command_id,omitempty"`
	HistoryLength  int64      `json:"history_length,omitempty"`
}

type WorkflowHistory struct {
	WorkflowID string   `json:"workflow_id"`
	RunID      string   `json:"run_id"`
	EventCount int      `json:"event_count"`
	EventTypes []string `json:"event_types"`
}

type WorkflowStart struct {
	WorkflowID     string
	TaskQueue      string
	IntentType     string
	Payload        []byte
	OwnerCommandID string
}

// Optional paged history seam keeps existing runtime adapters source compatible.
// Cursors always bind to one concrete Run; only workflow task completions are
// valid reset boundaries, not domain state rollback points.
type WorkflowHistoryPager interface {
	HistoryPage(context.Context, string, string, []byte, int) (WorkflowHistoryPage, error)
}
type WorkflowHistoryEvent struct {
	ID         int64          `json:"id"`
	Type       string         `json:"type"`
	At         *time.Time     `json:"at,omitempty"`
	Resettable bool           `json:"resettable"`
	Details    map[string]any `json:"details"`
}
type WorkflowHistoryPage struct {
	WorkflowID string                 `json:"workflow_id"`
	RunID      string                 `json:"run_id"`
	Events     []WorkflowHistoryEvent `json:"events"`
	NextToken  []byte                 `json:"-"`
}

var ErrWorkflowCommandRejected = errors.New("workflow_command_rejected")
