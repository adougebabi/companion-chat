package core

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// directImageDiscoverySchema named the retired pre-turn "does this turn need
// the image tool" check. The chat model now sees media.image.generate from the
// first round and decides itself; the name stays a route variant of
// reply_action_consistency so saved model-route settings remain valid.
const directImageDiscoverySchema = "direct_image_discovery_v1"

const directPhotoReplyStyle = `用角色平时对对方说话的方式接住原消息，正文只写角色想对对方说的话。照片会在拍好后单独发过去，可以像真人一样自然地说"拍好了马上发你""等我一下"之类，不要提系统、任务或回执这些词。`

const directImageAcceptedReplyRule = `本轮的图片请求已受理，尚无图片送达回执。现在用 conversation_reply 发布私聊正文。` + directPhotoReplyStyle + `decision 只需填写 action_type="reply"、一句简短的 response_intent 和 influences=[]。不要填写任何推测的引用或可选字段。`

// directImageReplyPhaseInput: once this turn's photo request is accepted, the
// next request publishes the private reply with the accepted-photo rule first.
func directImageReplyPhaseInput(ctx context.Context, input []*schema.Message) []*schema.Message {
	if !conversationPublicationPolicyEnabled(ctx) {
		return input
	}
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return input
	}
	if correcting, _ := ctx.Value(conversationPublicationCorrectionKey{}).(bool); correcting {
		return input
	}
	_, results := bridge.Trace.Snapshot()
	if !directImageReplyPending(ctx, results) {
		return input
	}
	for i, message := range input {
		if message != nil && message.Role == schema.System {
			updated := append([]*schema.Message(nil), input...)
			system := *message
			system.Content = directImageAcceptedReplyRule + "\n\n" + system.Content
			updated[i] = &system
			return updated
		}
	}
	return input
}

func directImageReplyPending(ctx context.Context, results []CapabilityResult) bool {
	main, _ := ctx.Value(directMainConversationKey{}).(bool)
	run := replyRunFromContext(ctx)
	if !main || run == nil || run.Settings.Mode != "direct" {
		return false
	}
	for _, result := range results {
		output := mapValue(result.Output)
		if result.CapabilityName == "media.image.generate" && result.Status == "accepted" &&
			output["target_kind"] == "conversation" && stringValue(output["media_intent_id"]) != "" {
			return true
		}
	}
	return false
}
