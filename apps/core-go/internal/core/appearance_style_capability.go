package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const appearanceStyleCapabilityName = "appearance.style"

type appearanceStyleCapability struct{ repository *PostgresRepository }

func appearanceStyleDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: appearanceStyleCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Actually set or clear a temporary hairstyle on the shared body. This cannot change hair length, color, injury, or clothing and does not rewrite a style habit.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation": enumStringSchema("set", "clear"),
			"style":     map[string]any{"type": "string", "maxLength": 128},
			"reason":    map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"operation", "reason"}, false),
		OutputSchema: objectSchema(map[string]any{
			"body_revision": map[string]any{"type": "integer", "minimum": 1},
			"status":        enumStringSchema("known", "cleared"),
			"style":         map[string]any{"type": "string"},
		}, []string{"body_revision", "status"}, false),
		SideEffectClass: "native_projection", SuccessBoundary: "temporary_hairstyle_committed", ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (c appearanceStyleCapability) Definition() CapabilityDefinition {
	return appearanceStyleDefinition()
}
func (c appearanceStyleCapability) RequiredContext() []ContextSlot { return nil }
func (c appearanceStyleCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (c appearanceStyleCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.repository == nil {
		return invocation, errors.New("appearance unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, appearanceStyleDefinition())
	if err != nil {
		return invocation, err
	}
	var revision int
	var raw []byte
	if err := c.repository.Pool().QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1`, invocation.Metadata.FluctlightID).Scan(&revision, &raw); err != nil {
		return invocation, err
	}
	if stringValue(args["operation"]) == "set" {
		if style := strings.TrimSpace(stringValue(args["style"])); style == "" {
			return invocation, ErrInvalidArguments
		}
		if stringValue(mapValue(decodeObject(raw)["hair_length"])["status"]) != "known" {
			return invocation, newCapabilityError("current_hair_length_unknown", false, ErrConflict)
		}
	}
	return withCapabilityPreparedData(invocation, "expected_body_revision", revision)
}
func (c appearanceStyleCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	args, err := capabilityExecutionArguments(invocation, appearanceStyleDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	expected, found, err := capabilityPreparedData(invocation, "expected_body_revision")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "appearance_plan_missing", false), errors.New("appearance plan missing")
	}
	fluctlightID := invocation.Metadata.FluctlightID
	var revision int
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &raw); err != nil {
		return failedCapabilityResult(invocation, "appearance_unavailable", true), err
	}
	if revision != intValue(expected) {
		return failedCapabilityResult(invocation, "appearance_revision_conflict", true), ErrConflict
	}
	fields := decodeObject(raw)
	status := "cleared"
	style := ""
	if stringValue(args["operation"]) == "set" {
		status = "known"
		style = strings.TrimSpace(stringValue(args["style"]))
		if stringValue(mapValue(fields["hair_length"])["status"]) != "known" {
			return failedCapabilityResult(invocation, "current_hair_length_unknown", false), ErrConflict
		}
		fields["hair_style"] = map[string]any{"status": status, "value": style}
	} else {
		fields["hair_style"] = map[string]any{"status": status}
	}
	newRevision := revision + 1
	sourceRef := "tool-operation:" + stableDigest(fluctlightID+"\x1f"+capabilityOperationID(invocation))
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=$2,state_json=$3,source_kind='instant_behavior',source_ref=$4,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision, jsonBytes(fields), sourceRef); err != nil {
		return failedCapabilityResult(invocation, "appearance_update_failed", true), err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_appearance_revisions(fluctlight_id,revision,state_json,source_kind,source_ref) VALUES($1,$2,$3,'instant_behavior',$4)`, fluctlightID, newRevision, jsonBytes(fields), sourceRef); err != nil {
		return failedCapabilityResult(invocation, "appearance_revision_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "appearance.updated", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"appearance:"+fluctlightID, "appearance-style:"+sourceRef, map[string]any{"revision": newRevision, "change": "hair_style"}); err != nil {
		return failedCapabilityResult(invocation, "appearance_event_failed", true), err
	}
	output := map[string]any{"body_revision": newRevision, "status": status}
	if style != "" {
		output["style"] = style
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: fmt.Sprintf("appearance:%s:%d", fluctlightID, newRevision)}, nil
}
