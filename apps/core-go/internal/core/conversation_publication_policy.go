package core

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/schema"
)

// conversationPublicationPhysicalRequestPolicy keeps private conversation
// publication inside the native Eino loop. Until a real private reply is
// committed, every physical request must choose a Tool and cannot terminate
// through the final JSON grammar. Media may accompany a reply but cannot
// replace the private conversation message.
type conversationPublicationPhysicalRequestPolicy struct{}

func (conversationPublicationPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	adkContext, ok := adkCapabilityContext(ctx)
	if !ok || adkContext.Trace == nil {
		return physicalModelRequestDecision{}, errors.New("conversation_publication_trace_missing")
	}
	if conversationHasAuthoritativeOutput(adkContext.Trace) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
}

func conversationHasAuthoritativeOutput(trace *ADKCapabilityTrace) bool {
	if trace == nil {
		return false
	}
	_, results := trace.Snapshot()
	_, committed := committedConversationReplyResult(results)
	return committed
}

func conversationPublicationPolicyEnabled(ctx context.Context) bool {
	policy, ok := physicalModelRequestPolicyFromContext(ctx)
	if !ok {
		return false
	}
	_, ok = policy.(conversationPublicationPhysicalRequestPolicy)
	return ok
}
