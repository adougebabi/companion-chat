package core

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const chatPromiseCapabilityName = "promise.propose"

type chatPromiseService struct {
	enqueue func(context.Context, string, string, string, string, map[string]any) (string, error)
	now     func() time.Time
}
type chatPromiseCapability struct{ service *chatPromiseService }

func newChatPromiseService(app *App) *chatPromiseService {
	if app == nil {
		return nil
	}
	return &chatPromiseService{enqueue: app.enqueueOwnerLifeRequest, now: app.now}
}
func (chatPromiseCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: chatPromiseCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Record a proposed appointment using the actual current Owner message verbatim as its title. Supply explicit future start/end (RFC3339 offsets). The existing promise/Goal process decides acceptance; an accepted appointment is then queued for its day schedule. Proposed/queued is not accepted, scheduled or fulfilled. Use this for a new appointment, not schedule.edit on an invented item ID. Refusal needs no proposal.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceConversation}, FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:  objectSchema(map[string]any{"starts_at": stringSchema(), "due_at": stringSchema(), "reason": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}}, []string{"starts_at", "due_at", "reason"}, false),
		OutputSchema: objectSchema(map[string]any{"promise_request_id": stringSchema(), "promise_status": stringSchema(), "accepted_by_character": map[string]any{"type": "boolean"}, "scheduled": map[string]any{"type": "boolean"}, "window_start": stringSchema(), "window_end": stringSchema()}, []string{"promise_request_id", "promise_status", "accepted_by_character", "scheduled", "window_start", "window_end"}, false), SideEffectClass: "native_projection", SuccessBoundary: "proposal_recorded", CompletionBoundary: "promise_accepted", OutcomeReferenceField: "promise_request_id", ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}
func (chatPromiseCapability) RequiredContext() []ContextSlot { return nil }
func (chatPromiseCapability) Execute(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(inv)
}
func (c chatPromiseCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	args, err := capabilityExecutionArguments(inv, c.Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	if c.service == nil || inv.Metadata.ConversationID == "" || inv.SourceFactID == "" {
		return handledCapabilityResult(inv, "promise_source_required", "needs_input", nil), nil
	}
	var source, sourceText string
	err = tx.QueryRow(ctx, `SELECT m.id,m.text FROM public.conversation_messages m WHERE m.conversation_id=$1 AND m.author_actor_id=$2 AND m.kind='user' AND (m.source_fact_id=$3 OR m.id=$3) AND EXISTS(SELECT 1 FROM public.conversation_participants p WHERE p.conversation_id=m.conversation_id AND p.actor_id=$4 AND p.status='active') ORDER BY m.sequence DESC LIMIT 1`, inv.Metadata.ConversationID, inv.Metadata.AuthorizationActorID, inv.SourceFactID, inv.Metadata.FluctlightID).Scan(&source, &sourceText)
	if errors.Is(err, pgx.ErrNoRows) {
		return handledCapabilityResult(inv, "promise_source_required", "needs_input", nil), nil
	}
	if err != nil {
		return failedCapabilityResult(inv, "promise_source_read_failed", true), err
	}
	if len([]rune(sourceText)) > 2000 {
		return handledCapabilityResult(inv, "promise_source_too_long", "needs_input", map[string]any{"recovery": "Please use a concise explicit appointment proposal; the original message is preserved."}), nil
	}
	args["title"] = sourceText
	start, e1 := time.Parse(time.RFC3339Nano, stringValue(args["starts_at"]))
	end, e2 := time.Parse(time.RFC3339Nano, stringValue(args["due_at"]))
	if e1 != nil || e2 != nil {
		return handledCapabilityResult(inv, "invalid_arguments", "needs_input", map[string]any{"detail": "starts_at/due_at require RFC3339 offsets"}), nil
	}
	key := "chat-promise:" + stableDigest(inv.Metadata.FluctlightID+":"+source+":"+strings.TrimSpace(stringValue(args["title"]))+":"+start.UTC().Format(instantLayout)+":"+end.UTC().Format(instantLayout))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
		return failedCapabilityResult(inv, "promise_lock_failed", true), err
	}
	var previous []byte
	err = tx.QueryRow(ctx, `SELECT response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, inv.Metadata.AuthorizationActorID, key).Scan(&previous)
	if err == nil {
		return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "accepted", Output: decodeObject(previous)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return failedCapabilityResult(inv, "promise_replay_read_failed", true), err
	}
	body := cloneMap(args)
	body["idempotency_key"] = key
	surface := NewOwnerSurface(nil, SurfaceHooks{EnqueueLifeRequest: c.service.enqueue, Now: c.service.now})
	ctx = context.WithValue(ctx, surfaceDBTxKey{}, tx)
	request, err := surface.insertRequest(ctx, postgresSurfaceTx{tx}, inv.Metadata.AuthorizationActorID, inv.Metadata.FluctlightID, "promise", body)
	if err != nil {
		return failedCapabilityResult(inv, "promise_proposal_failed", false), err
	}
	id := stringValue(request.Payload["id"])
	record, found, err := (postgresSurfaceTx{tx}).Get(ctx, inv.Metadata.AuthorizationActorID, id)
	if err != nil {
		return failedCapabilityResult(inv, "promise_source_save_failed", true), err
	}
	if !found {
		return failedCapabilityResult(inv, "promise_source_save_failed", true), ErrNotFound
	}
	record.Payload["originating_message_id"] = source
	if err := (postgresSurfaceTx{tx}).Save(ctx, record); err != nil {
		return failedCapabilityResult(inv, "promise_source_save_failed", true), err
	}
	output := map[string]any{"promise_request_id": id, "promise_status": "proposed", "accepted_by_character": false, "scheduled": false, "window_start": args["starts_at"], "window_end": args["due_at"]}
	if _, err := tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,$4)`, inv.Metadata.AuthorizationActorID, key, stableDigest(jsonString(args)), jsonBytes(output)); err != nil {
		return failedCapabilityResult(inv, "promise_receipt_failed", true), err
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "accepted", Output: output}, nil
}

// Only accepted, genuinely linked chat proposals may enter a date plan.
// Existing request rows (including failed ones) remain the recovery authority.
func (a *App) EnsureAcceptedChatPromiseSchedules(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT p.owner_actor_id,p.fluctlight_id,p.request_id,p.window_start,p.window_end,p.timezone,p.revision FROM public.owner_promise_links p JOIN public.owner_surface_records r ON r.id=p.request_id AND r.owner_actor_id=p.owner_actor_id JOIN public.fluctlights f ON f.id=p.fluctlight_id AND f.status='active' WHERE r.payload->>'originating_message_id' IS NOT NULL AND p.accepted_at IS NOT NULL AND p.cancelled_at IS NULL AND p.commitment_id IS NOT NULL AND p.intention_id IS NOT NULL AND p.window_start>now()+interval '30 seconds' AND NOT EXISTS(SELECT 1 FROM public.owner_schedule_requests s WHERE s.fluctlight_id=p.fluctlight_id AND s.payload->>'promise_id'=p.request_id) ORDER BY p.window_start,p.request_id LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type row struct {
		owner, fid, id, zone string
		start, end           time.Time
		revision             int
	}
	var values []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.owner, &r.fid, &r.id, &r.start, &r.end, &r.zone, &r.revision); err != nil {
			rows.Close()
			return 0, err
		}
		values = append(values, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range values {
		zone, e := time.LoadLocation(r.zone)
		if e != nil {
			return count, e
		}
		date := r.start.In(zone).Format("2006-01-02")
		if r.start.After(a.now().AddDate(0, 0, 30)) {
			continue
		}
		calendar, e := a.OwnerSchedules(ctx, r.owner, r.fid, url.Values{"from": []string{date}, "to": []string{date}})
		if e != nil {
			return count, e
		}
		days := arrayValue(calendar["days"])
		if len(days) == 0 {
			continue
		}
		body := map[string]any{"local_date": date, "expected_revision": intValue(mapValue(mapValue(days[0])["schedule"])["revision"]), "expected_life_context_revision": calendar["context_revision"], "promise_id": r.id, "expected_promise_revision": r.revision, "start_at": r.start.Format(instantLayout), "end_at": r.end.Format(instantLayout), "reason": "将角色已接受的聊天约定安排到对应日期", "idempotency_key": "chat-promise-schedule:" + r.id}
		if _, e := a.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+r.fid+"/schedules/requests", nil, r.owner, body); e != nil {
			var domain *SurfaceError
			if errors.As(e, &domain) && domain.Status >= 400 && domain.Status < 500 || errors.Is(e, ErrConflict) || errors.Is(e, ErrInvalidArguments) || errors.Is(e, ErrNotFound) || errors.Is(e, ErrUnauthorized) {
				slog.Warn("chat_promise_schedule_deferred", "promise_id", r.id, "error", e)
				continue
			}
			return count, e
		}
		count++
	}
	return count, nil
}
