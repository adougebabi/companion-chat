package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// resolveMemoryCorrectionTarget binds a model-visible opaque ref to one
// authorized current Memory revision. Initial projection refs are checked
// against the frozen Core index; deeper recall refs are recomputed from the
// same host-bound scope used by memory.recall. The model never supplies an ID.
func (a *App) resolveMemoryCorrectionTarget(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext, ref string) (memoryAuthorityRow, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" || !strings.HasPrefix(ref, "memory:ctx_") || len([]rune(ref)) > maxContextReferenceRunes {
		return memoryAuthorityRow{}, errors.New("memory_correction_ref_invalid")
	}
	ownerID := strings.TrimSpace(stringValue(resolved.Memory.Data["owner_actor_id"]))
	fluctlightID := strings.TrimSpace(invocation.Metadata.FluctlightID)
	conversationID := strings.TrimSpace(invocation.Metadata.ConversationID)
	if ownerID == "" || fluctlightID == "" {
		return memoryAuthorityRow{}, errors.New("memory_correction_scope_invalid")
	}
	if index, err := contextReferenceIndexFromValue(invocation.ContextSnapshot["context_reference_index"]); err == nil {
		if index.FluctlightID == fluctlightID && index.OwnerActorID == ownerID && index.ConversationID == conversationID {
			if entry, found := index.ByRef[ref]; found && entry.Kind == ContextReferenceMemory {
				row, err := a.readOwnedMemoryAuthorityRow(ctx, ownerID, entry.EntityID)
				if err != nil {
					return memoryAuthorityRow{}, err
				}
				if row.OwnerFluctlightID != fluctlightID || row.Revision != entry.Revision || row.Status != "active" || (row.ConversationID != "" && row.ConversationID != conversationID) {
					return memoryAuthorityRow{}, errors.New("memory_correction_ref_stale")
				}
				return row, nil
			}
		}
	}
	viewers := decisionServiceRefValues(resolved.Memory.Data["viewer_actor_ids"])
	ownerVisible := false
	for _, viewer := range viewers {
		if viewer == ownerID {
			ownerVisible = true
			break
		}
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,revision FROM public.memories WHERE owner_fluctlight_id=$1 AND status='active'
AND (conversation_id IS NULL OR conversation_id=$2)
AND ((visibility IN ('private','owner') AND $3) OR (visibility='participants' AND actor_refs ?| $4::text[]))`, fluctlightID, nullableString(conversationID), ownerVisible, viewers)
	if err != nil {
		return memoryAuthorityRow{}, err
	}
	var targetID string
	for rows.Next() {
		var id string
		var revision int
		if err := rows.Scan(&id, &revision); err != nil {
			rows.Close()
			return memoryAuthorityRow{}, err
		}
		request := MemoryRecallRequest{FluctlightID: fluctlightID, ConversationID: conversationID}
		if recallOpaqueRef("memory", id+":"+fmt.Sprint(revision), request) == ref {
			if targetID != "" {
				rows.Close()
				return memoryAuthorityRow{}, errors.New("memory_correction_ref_ambiguous")
			}
			targetID = id
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return memoryAuthorityRow{}, err
	}
	if targetID == "" {
		return memoryAuthorityRow{}, errors.New("memory_correction_ref_not_found")
	}
	return a.readOwnedMemoryAuthorityRow(ctx, ownerID, targetID)
}
