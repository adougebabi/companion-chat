package core

import "strings"

// A private conversation admits one implicit feed-publication action per source.
// Core-owned explicit targets and autonomous actions retain their own operation
// identities. Physical/native call coordinates never grant another publication.
func conversationMomentOperationID(request ADKCapabilityRequest, fallback string) string {
	conversationTarget := request.TargetKind == "" || request.TargetKind == "conversation" && request.TargetRef == request.ConversationID
	if request.Surface != CapabilitySurfaceConversation || request.ConversationID == "" || !conversationTarget {
		return fallback
	}
	source := firstString(request.SourceFactID, firstString(request.OperationID, request.ActionID))
	if source == "" {
		return fallback
	}
	return "conversation_moment_" + stableDigest(strings.Join([]string{request.FluctlightID, request.ConversationID, request.AuthorizationActorID, source, request.ActionID}, "\x1f"))
}

func committedMomentResult(trace *ADKCapabilityTrace) (CapabilityResult, bool) {
	if trace == nil {
		return CapabilityResult{}, false
	}
	_, results := trace.Snapshot()
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		output := mapValue(result.Output)
		if result.CapabilityName == "moment.publish" && result.Status == "completed" && output["target_kind"] == "moment" && strings.TrimSpace(stringValue(output["target_ref"])) != "" {
			return result, true
		}
	}
	return CapabilityResult{}, false
}

func momentAcknowledgementOnly(trace *ADKCapabilityTrace) bool {
	if _, ok := committedMomentResult(trace); !ok || conversationHasAuthoritativeOutput(trace) {
		return false
	}
	_, results := trace.Snapshot()
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName != conversationReplyCapabilityName {
			continue
		}
		// Real pending media actions remain executable before acknowledgement.
		output := mapValue(result.Output)
		return !(result.Status == "accepted" && output["reply_status"] == "needs_action" && strings.TrimSpace(stringValue(output["draft_ref"])) != "")
	}
	return true
}

// Omitted, null and empty optional evidence all mean no selected evidence. This
// changes no nonempty command and never normalizes legacy text/tool arguments.
func canonicalMomentEvidenceArguments(arguments []byte) []byte {
	value := decodeObject(arguments)
	if len(value) == 0 {
		return arguments
	}
	if raw, found := value["evidence_refs"]; found {
		if raw == nil {
			delete(value, "evidence_refs")
		} else if refs, ok := raw.([]any); ok && len(refs) == 0 {
			delete(value, "evidence_refs")
		}
	}
	return jsonBytes(value)
}
