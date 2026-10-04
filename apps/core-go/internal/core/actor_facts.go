package core

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

const actorInspectCapabilityName = "actor.inspect"
const actorFactCapabilityName = "actor.fact.record"

var actorAttributePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

type actorFactService struct {
	repository *PostgresRepository
	clock      func() time.Time
}

func (s *actorFactService) now() time.Time {
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

type actorInspectCapability struct{ service *actorFactService }
type actorFactCapability struct{ service *actorFactService }

func actorFactDefinition() CapabilityDefinition {
	return CapabilityDefinition{Name: actorFactCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Record a sourced Actor attribute; correct mistakes or change formerly true values. Use actor.inspect IDs and actual inbound sources.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceNativeCognition}, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		InputSchema: objectSchema(map[string]any{
			"assertion_type": enumStringSchema("explicit_statement", "inference"),
			"operation":      enumStringSchema("assert", "correct", "change"), "actor_id": stringSchema(), "attribute": map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,63}$"},
			"value": map[string]any{"type": []any{"string", "boolean", "null"}}, "source_message_id": stringSchema(), "effective_at": stringSchema(),
			"corrected_fact_ids": map[string]any{"type": "array", "maxItems": 16, "items": stringSchema()}, "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500},
		}, []string{"operation", "attribute", "value", "reason"}, false), OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "actor_fact_committed", ConcurrencyClass: "exclusive", SupportsRetry: true}
}
func (c actorFactCapability) Definition() CapabilityDefinition { return actorFactDefinition() }
func (c actorFactCapability) RequiredContext() []ContextSlot   { return nil }
func (c actorFactCapability) Execute(_ context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(i)
}
func actorInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{Name: actorInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery, Description: "Read sourced Actor attributes. History labels former truths and corrected mistakes.", Surfaces: []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition, CapabilitySurfaceAutonomy}, FailurePolicy: FailurePolicyOptionalInternal, InputSchema: objectSchema(map[string]any{"actor_id": stringSchema(), "attribute": map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,63}$"}, "history": map[string]any{"type": "boolean"}}, nil, false), OutputSchema: openObjectSchema(), SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true}
}
func (c actorInspectCapability) Definition() CapabilityDefinition { return actorInspectDefinition() }
func (c actorInspectCapability) RequiredContext() []ContextSlot   { return nil }
func (c actorInspectCapability) Execute(ctx context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(i, "actor_query_unavailable", true), errors.New("actor service unavailable")
	}
	args, err := capabilityExecutionArguments(i, actorInspectDefinition())
	if err != nil {
		return failedCapabilityResult(i, "invalid_arguments", false), err
	}
	subject := firstString(args["actor_id"], firstString(i.Metadata.SubjectActorID, i.Metadata.AuthorizationActorID))
	if err := requireActorFactScope(ctx, c.service.repository.Pool(), i.Metadata.FluctlightID, i.Metadata.AuthorizationActorID, subject); err != nil {
		return failedCapabilityResult(i, "actor_scope_invalid", false), err
	}
	history, _ := args["history"].(bool)
	rows, err := readActorFactsWith(ctx, c.service.repository.Pool(), i.Metadata.FluctlightID, subject, c.service.now(), history, 32, stringValue(args["attribute"]))
	if err != nil {
		return failedCapabilityResult(i, "actor_query_failed", true), err
	}
	return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: map[string]any{"actor_id": subject, "facts": rows, "history": history, "timezone_semantics": "unknown unless explicitly confirmed"}}, nil
}
func requireActorFactScope(ctx context.Context, q lifeContextQuerier, fluctlightID, ownerID, subject string) error {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlights f WHERE f.id=$1 AND f.created_by_actor_id=$2 AND ($3=$2 OR $3=$1 OR EXISTS(SELECT 1 FROM public.conversation_participants a JOIN public.conversation_participants b ON b.conversation_id=a.conversation_id WHERE a.actor_id=$1 AND b.actor_id=$3 AND a.status='active' AND b.status='active')))`, fluctlightID, ownerID, subject).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrUnauthorized
	}
	return nil
}
func (c actorFactCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(i, "actor_fact_unavailable", true), errors.New("actor service unavailable")
	}
	args, err := capabilityExecutionArguments(i, actorFactDefinition())
	if err != nil {
		return failedCapabilityResult(i, "invalid_arguments", false), err
	}
	subject := firstString(args["actor_id"], firstString(i.Metadata.SubjectActorID, i.Metadata.AuthorizationActorID))
	owner := i.Metadata.FluctlightID
	if err := requireActorFactScope(ctx, tx, owner, i.Metadata.AuthorizationActorID, subject); err != nil {
		return failedCapabilityResult(i, "actor_scope_invalid", false), err
	}
	attribute := stringValue(args["attribute"])
	if !actorAttributePattern.MatchString(attribute) || len([]rune(stringValue(args["value"]))) > 1024 {
		return failedCapabilityResult(i, "actor_attribute_invalid", false), ErrInvalidArguments
	}
	// These fields have existing domain authorities; this Tool cannot bypass them.
	switch attribute {
	case "inventory", "ownership", "wearing", "current_scene", "body", "current_outfit":
		return failedCapabilityResult(i, "actor_domain_attribute_forbidden", false), ErrInvalidArguments
	}
	if subject == owner {
		switch attribute {
		case "timezone", "location", "location_scope", "city", "country", "residence", "hair_length", "hair_color", "hair_style", "chest_cup", "injuries", "body_type", "name", "age", "gender", "occupation":
			return failedCapabilityResult(i, "actor_domain_attribute_forbidden", false), ErrInvalidArguments
		}
	}
	at := c.service.now()
	effective := at
	if raw := stringValue(args["effective_at"]); raw != "" {
		effective, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil || effective.After(at) {
			return failedCapabilityResult(i, "actor_effective_time_invalid", false), ErrInvalidArguments
		}
	}
	sourceID := stringValue(args["source_message_id"])
	sourceRef, fingerprint, kind := "", "", "owner_defined"
	var sourceAuthor, sourceKind, actorType string
	if sourceID != "" {
		err = tx.QueryRow(ctx, `SELECT msg.author_actor_id,msg.kind,a.actor_type,md5(msg.text||msg.attachment_refs::text) FROM public.conversation_messages msg JOIN public.actors a ON a.id=msg.author_actor_id WHERE msg.id=$1 AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=msg.conversation_id AND p.actor_id=$2 AND p.status='active')`, sourceID, owner).Scan(&sourceAuthor, &sourceKind, &actorType, &fingerprint)
	} else if i.Metadata.Source != "direct" {
		err = tx.QueryRow(ctx, `SELECT msg.id,msg.author_actor_id,msg.kind,a.actor_type,md5(msg.text||msg.attachment_refs::text) FROM public.conversation_messages msg JOIN public.actors a ON a.id=msg.author_actor_id WHERE msg.source_fact_id=$1 AND msg.kind='user' AND msg.conversation_id=$2`, i.SourceFactID, i.Metadata.ConversationID).Scan(&sourceID, &sourceAuthor, &sourceKind, &actorType, &fingerprint)
	}
	if err != nil {
		return failedCapabilityResult(i, "actor_source_message_required", false), err
	}
	if i.Metadata.Source != "direct" {
		if stringValue(args["assertion_type"]) == "" {
			return failedCapabilityResult(i, "actor_assertion_type_required", false), ErrInvalidArguments
		}
		var matches bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_messages m JOIN public.cognition_inbox i ON i.id=m.source_fact_id WHERE m.id=$1 AND m.source_fact_id=$2 AND m.kind='user' AND COALESCE(i.payload->>'source_message_invalidated','false')<>'true')`, sourceID, i.SourceFactID).Scan(&matches); err != nil || !matches {
			return failedCapabilityResult(i, "actor_source_message_stale", false), ErrConflict
		}
	}

	status := "active"
	if sourceID != "" {
		sourceRef = "message:" + sourceID
		kind = "inference"
		status = "uncertain"
		if sourceAuthor == subject && sourceKind == "user" && actorType == "human" && stringValue(args["assertion_type"]) != "inference" {
			kind = "user_statement"
			status = "active"
		}
	} else {
		var isOwner bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.actors WHERE id=$1 AND actor_type='human' AND status='active')`, i.Metadata.AuthorizationActorID).Scan(&isOwner); err != nil || !isOwner {
			return failedCapabilityResult(i, "actor_source_invalid", false), ErrUnauthorized
		}
		sourceRef = "owner-command:" + capabilityOperationID(i)
		fingerprint = stableDigest(jsonString(args))
	}
	if attribute == "timezone" && args["value"] != nil {
		name, ok := args["value"].(string)
		if !ok || name == "" || name == "Local" {
			return failedCapabilityResult(i, "actor_timezone_invalid", false), ErrInvalidArguments
		}
		if _, err := time.LoadLocation(name); err != nil {
			return failedCapabilityResult(i, "actor_timezone_invalid", false), ErrInvalidArguments
		}
	}
	operation := stringValue(args["operation"])
	if len(arrayValue(args["corrected_fact_ids"])) > 0 && operation != "correct" {
		return failedCapabilityResult(i, "actor_correction_operation_required", false), ErrInvalidArguments
	}
	if status != "active" && operation != "assert" {
		return failedCapabilityResult(i, "actor_inference_cannot_correct", false), ErrUnauthorized
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "actor-facts:"+owner); err != nil {
		return failedCapabilityResult(i, "actor_lock_failed", true), err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "actor-facts:"+owner+":"+subject); err != nil {
		return failedCapabilityResult(i, "actor_lock_failed", true), err
	}
	id := "actor_fact_" + stableDigest(owner+"\x1f"+capabilityOperationID(i))
	// Attribute corrections are atomic with the replacement. A changed state
	// ends the former interval without rewriting it as a mistaken assertion.
	retiredFacts := map[string]bool{}
	if status == "active" {
		rows, err := tx.Query(ctx, `SELECT id,value_json,valid_from FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND subject_actor_id=$2 AND attribute=$3 AND status='active' AND valid_until IS NULL FOR UPDATE`, owner, subject, attribute)
		if err != nil {
			return failedCapabilityResult(i, "actor_read_failed", true), err
		}
		var previousID string
		var previousValue []byte
		var previousFrom time.Time
		if rows.Next() {
			err = rows.Scan(&previousID, &previousValue, &previousFrom)
		}
		rows.Close()
		if err != nil {
			return failedCapabilityResult(i, "actor_read_failed", true), err
		}
		if previousID != "" {
			if operation == "assert" {
				if jsonString(decodeJSONValue(previousValue)) != jsonString(args["value"]) {
					return failedCapabilityResult(i, "actor_correction_required", false), ErrConflict
				}
				return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: map[string]any{"fact_id": previousID, "reused": true, "status": "active"}}, nil
			}
			if operation == "change" {
				if !effective.After(previousFrom) {
					return failedCapabilityResult(i, "actor_change_time_invalid", false), ErrConflict
				}
				_, err = tx.Exec(ctx, `UPDATE public.actor_facts SET valid_until=$2,revision=revision+1,replaced_by=$3 WHERE id=$1`, previousID, effective, id)
			} else {
				_, err = tx.Exec(ctx, `UPDATE public.actor_facts SET status='corrected',revision=revision+1,replaced_by=$2 WHERE id=$1`, previousID, id)
			}
			if err != nil {
				return failedCapabilityResult(i, "actor_revision_failed", true), err
			}
			retiredFacts[previousID] = true
			if err := retireActorFactArtifactsTx(ctx, tx, owner, previousID); err != nil {
				return failedCapabilityResult(i, "actor_derivation_failed", true), err
			}
		}
	}
	for _, raw := range arrayValue(args["corrected_fact_ids"]) {
		previousID := stringValue(raw)
		if retiredFacts[previousID] {
			continue
		}
		command, err := tx.Exec(ctx, `UPDATE public.actor_facts SET status='corrected',revision=revision+1,replaced_by=$3 WHERE id=$1 AND owner_fluctlight_id=$2 AND subject_actor_id=$4 AND attribute=$5 AND status IN ('active','uncertain')`, previousID, owner, id, subject, attribute)
		if err != nil || command.RowsAffected() != 1 {
			return failedCapabilityResult(i, "actor_correction_target_invalid", false), ErrConflict
		}
		if err := retireActorFactArtifactsTx(ctx, tx, owner, previousID); err != nil {
			return failedCapabilityResult(i, "actor_derivation_failed", true), err
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.actor_facts(id,owner_fluctlight_id,subject_actor_id,attribute,value_json,epistemic_kind,status,valid_from,source_message_id,source_ref,source_fingerprint,operation_id,transition_kind) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, id, owner, subject, attribute, jsonBytes(args["value"]), kind, status, effective, nullableString(sourceID), sourceRef, fingerprint, capabilityOperationID(i), operation)
	if err != nil {
		return failedCapabilityResult(i, "actor_fact_write_failed", true), err
	}
	if status == "active" {
		if err := invalidateRuntimeSummariesForActorTx(ctx, tx, owner, at); err != nil {
			return failedCapabilityResult(i, "actor_summary_invalidation_failed", true), err
		}
	}
	if err := appendOutboxTx(ctx, tx, "actor.fact.recorded", "actor_fact", id, owner, i.SourceFactID, i.Metadata.CorrelationID, "actor-fact:"+id, map[string]any{"fact_id": id, "subject_actor_id": subject, "attribute": attribute, "operation": operation, "status": status}); err != nil {
		return failedCapabilityResult(i, "actor_fact_outbox_failed", true), err
	}
	return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: map[string]any{"fact_id": id, "actor_id": subject, "attribute": attribute, "value": args["value"], "epistemic_kind": kind, "status": status, "effective_at": formatInstant(effective)}}, nil
}

func readActorFactsWith(ctx context.Context, q lifeContextQuerier, owner, subject string, at time.Time, history bool, limit int, attributes ...string) ([]map[string]any, error) {
	query := `SELECT f.id,f.subject_actor_id,f.attribute,f.value_json,f.epistemic_kind,f.status,f.valid_from,f.valid_until,f.source_ref,f.replaced_by,f.revision,f.transition_kind FROM public.actor_facts f WHERE f.owner_fluctlight_id=$1 AND ($2='' OR f.subject_actor_id=$2) AND ($5='' OR f.attribute=$5)`
	if !history {
		query += ` AND f.status='active' AND f.epistemic_kind<>'inference' AND f.valid_from<=$3 AND (f.valid_until IS NULL OR f.valid_until>$3) AND (f.source_message_id IS NULL OR EXISTS(SELECT 1 FROM public.conversation_messages msg WHERE msg.id=f.source_message_id AND md5(msg.text||msg.attachment_refs::text)=f.source_fingerprint))`
	} else {
		query += ` AND $3::timestamptz IS NOT NULL`
	}
	if !history {
		query += ` ORDER BY CASE WHEN f.attribute IN ('location_scope','location','timezone','relationship_distance','meeting_confirmed') THEN 0 ELSE 1 END,f.created_at DESC,f.id DESC LIMIT $4`
	} else {
		query += ` ORDER BY f.created_at DESC,f.id DESC LIMIT $4`
	}
	attribute := ""
	if len(attributes) > 0 {
		attribute = attributes[0]
	}
	rows, err := q.Query(ctx, query, owner, subject, at, limit, attribute)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, actor, attr, kind, status, source, transition string
		var raw []byte
		var from time.Time
		var until *time.Time
		var replacement *string
		var revision int
		if err := rows.Scan(&id, &actor, &attr, &raw, &kind, &status, &from, &until, &source, &replacement, &revision, &transition); err != nil {
			return nil, err
		}
		item := map[string]any{"fact_id": id, "actor_id": actor, "attribute": attr, "value": decodeJSONValue(raw), "epistemic_kind": kind, "status": status, "effective_at": formatInstant(from), "revision": revision, "transition_kind": transition}
		if history {
			item["source_ref"] = source
			item["replaced_by"] = replacement
			if until != nil {
				item["valid_until"] = formatInstant(*until)
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func retireActorFactArtifactsTx(ctx context.Context, tx pgx.Tx, owner, factID string) error {
	for _, statement := range []string{
		`UPDATE public.memories m SET provenance_status='invalid' WHERE m.owner_fluctlight_id=$1 AND EXISTS(SELECT 1 FROM public.actor_fact_artifacts d WHERE d.fact_id=$2 AND d.artifact_kind='memory' AND d.artifact_id=m.id AND d.artifact_revision=m.revision)`,
		`UPDATE public.conversation_summaries s SET status='invalidated' WHERE s.owner_fluctlight_id=$1 AND s.status IN ('active','consolidated') AND EXISTS(SELECT 1 FROM public.actor_fact_artifacts d WHERE d.fact_id=$2 AND d.artifact_kind='summary' AND d.artifact_id=s.id AND d.artifact_revision=s.revision)`,
	} {
		if _, err := tx.Exec(ctx, statement, owner, factID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `UPDATE public.conversation_runtime_summaries SET status='invalidated',revision=revision+1 WHERE owner_fluctlight_id=$1 AND status='active' AND covered_through>0`, owner)
	return err
}

func attachActorFactArtifactTx(ctx context.Context, tx pgx.Tx, owner, factID, kind, artifactID string, revision int, at time.Time) error {
	var factRevision int
	err := tx.QueryRow(ctx, `SELECT f.revision FROM public.actor_facts f WHERE f.id=$1 AND f.owner_fluctlight_id=$2 AND f.status='active' AND f.valid_from<=$3 AND (f.valid_until IS NULL OR f.valid_until>$3) AND (f.source_message_id IS NULL OR EXISTS(SELECT 1 FROM public.conversation_messages msg WHERE msg.id=f.source_message_id AND md5(msg.text||msg.attachment_refs::text)=f.source_fingerprint)) FOR SHARE`, factID, owner, at).Scan(&factRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("actor_fact_dependency_stale: %w", ErrConflict)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.actor_fact_artifacts(fact_id,fact_revision,artifact_kind,artifact_id,artifact_revision) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, factID, factRevision, kind, artifactID, revision)
	return err
}

func compactActorBackground(projection ContextProjection) []map[string]any {
	result := make([]map[string]any, 0)
	for _, fact := range projection.ActorFacts {
		attribute := stringValue(fact["attribute"])
		switch attribute {
		case "location_scope", "location", "timezone", "relationship_distance", "meeting_confirmed", "same_building", "co_located":
		default:
			continue
		}
		actor := actorRefForID(projection.Actors, stringValue(fact["actor_id"]))
		if len(actor) == 0 {
			continue
		}
		item := compactStateMap(fact, []string{"ref", "attribute", "value", "epistemic_kind", "effective_at", "transition_kind"})
		item["subject"] = actor["ref"]
		item["value"] = fact["value"]
		if fact["value"] == nil {
			item["value_state"] = "unknown"
		}
		result = append(result, item)
		if len(result) >= 16 {
			break
		}
	}
	return result
}

// Model-produced artifacts conservatively depend on the exact Actor facts
// that were visible in that call. This closes the correction/late-worker race
// without rejecting harmless new conversation messages globally.
func validateActorFactSnapshotTx(ctx context.Context, tx pgx.Tx, owner string, facts []map[string]any, at time.Time) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "actor-facts:"+owner); err != nil {
		return err
	}
	live, err := readActorFactsWith(ctx, tx, owner, "", at, false, 16)
	if err != nil {
		return err
	}
	identity := func(values []map[string]any) string {
		rows := []map[string]any{}
		for _, value := range values {
			rows = append(rows, compactStateMap(value, []string{"fact_id", "revision"}))
		}
		return jsonString(rows)
	}
	if identity(live) != identity(facts) {
		return fmt.Errorf("actor_fact_snapshot_stale: %w", ErrConflict)
	}
	for _, fact := range facts {
		var revision int
		err := tx.QueryRow(ctx, `SELECT f.revision FROM public.actor_facts f WHERE f.id=$1 AND f.owner_fluctlight_id=$2 AND f.status='active' AND f.valid_from<=$3 AND (f.valid_until IS NULL OR f.valid_until>$3) AND (f.source_message_id IS NULL OR EXISTS(SELECT 1 FROM public.conversation_messages m WHERE m.id=f.source_message_id AND md5(m.text||m.attachment_refs::text)=f.source_fingerprint)) FOR SHARE`, stringValue(fact["fact_id"]), owner, at).Scan(&revision)
		if err != nil || revision != intValue(fact["revision"]) {
			return fmt.Errorf("actor_fact_snapshot_stale: %w", ErrConflict)
		}
	}
	return nil
}
func attachActorFactSnapshotTx(ctx context.Context, tx pgx.Tx, owner string, facts []map[string]any, kind, id string, revision int, at time.Time) error {
	for _, fact := range facts {
		if err := attachActorFactArtifactTx(ctx, tx, owner, stringValue(fact["fact_id"]), kind, id, revision, at); err != nil {
			return err
		}
	}
	return nil
}

func validateArtifactActorDependenciesTx(ctx context.Context, tx pgx.Tx, owner, kind, id string, revision int, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT fact_id,fact_revision FROM public.actor_fact_artifacts WHERE artifact_kind=$1 AND artifact_id=$2 AND artifact_revision=$3 ORDER BY fact_id`, kind, id, revision)
	if err != nil {
		return err
	}
	facts := []map[string]any{}
	for rows.Next() {
		var factID string
		var factRev int
		if err := rows.Scan(&factID, &factRev); err != nil {
			rows.Close()
			return err
		}
		facts = append(facts, map[string]any{"fact_id": factID, "revision": factRev})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		if err := attachActorFactArtifactTx(ctx, tx, owner, stringValue(fact["fact_id"]), kind, id, revision, at); err != nil {
			return err
		}
		var liveRev int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.actor_facts WHERE id=$1 FOR SHARE`, fact["fact_id"]).Scan(&liveRev); err != nil {
			return err
		}
		if liveRev != intValue(fact["revision"]) {
			return fmt.Errorf("actor_fact_dependency_stale: %w", ErrConflict)
		}
	}
	return nil
}
