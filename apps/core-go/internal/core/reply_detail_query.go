package core

import (
	"context"
	"sync"
)

// This bridge never delegates to a parent invoker/catalog. The canonical detail
// Tool keeps owner/profile/revision authorization through ExecuteTool, using
// only the fresh child Runner trace for its native call identity.
type replyWriterInvoker struct {
	image  replyImageRequestInvoker
	app    *App
	input  ConversationReplyTaskInput
	mu     sync.Mutex
	detail ADKCapabilityInvoker
}

func (i *replyWriterInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	if i.input.Context.StyleRepair {
		return "", ErrUnauthorized
	}
	switch name {
	case replyImageRequestName:
		if !replyDefinitionInstalled(i.input.Context, name) {
			return "", ErrUnauthorized
		}
		return i.image.ExecuteWithID(ctx, id, name, args)
	case personaDetailCapabilityName:
		if !replyDefinitionInstalled(i.input.Context, name) || i.app.DB == nil {
			return "", ErrUnauthorized
		}
		fresh, ok := adkCapabilityContext(ctx)
		if !ok || fresh.Trace == nil {
			return "", ErrUnauthorized
		}
		i.mu.Lock()
		if i.detail == nil {
			projection := i.input.Context.Projection
			i.detail = newAppADKCapabilityInvokerForContext(ctx, i.app, ADKCapabilityRequest{AuthorizationActorID: projection.OwnerActorID, SubjectActorID: stringValue(projection.CurrentSpeaker["actor_id"]), FluctlightID: projection.FluctlightID, ConversationID: projection.ConversationID, SourceFactID: projection.SourceFactID, OperationID: firstString(i.input.DraftRef, i.input.CorrelationID), CorrelationID: i.input.CorrelationID, Surface: CapabilitySurfaceConversation, Projection: projection}, fresh.Trace)
		}
		delegate := i.detail
		i.mu.Unlock()
		return delegate.(interface {
			ExecuteWithID(context.Context, string, string, string) (string, error)
		}).ExecuteWithID(ctx, id, name, args)
	default:
		return "", ErrUnauthorized
	}
}
func (i *replyWriterInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "", name, args)
}
func replyDefinitionInstalled(input ReplyContextInput, name string) bool {
	for _, definition := range replyWriterDefinitions(input) {
		if definition.Name == name {
			return true
		}
	}
	return false
}
