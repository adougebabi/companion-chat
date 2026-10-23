package core

import (
	"context"
	"github.com/cloudwego/eino/schema"
)

// Existing callers retain all messages, reasoning and Tool results. Token
// estimates are observations, never authority to compact an outbound prompt.
func (m *queuedToolCallingChatModel) compactPhysicalInput(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
	return input, nil
}
