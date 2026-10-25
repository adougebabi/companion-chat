package core

import (
	"context"
	"errors"
	"time"
)

const momentInspectCapabilityName = "moment.inspect"

// This query reads committed self publications, never drafts or other actors'
// feed entries. It cannot publish, react, or change feed read positions.
type momentInspectCapability struct{ repository *PostgresRepository }

func (momentInspectCapability) Definition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: momentInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:     "Read your own actually published Moments, including exact text and publication time. Use this to recall a previous post; moment.publish creates a NEW post. Optional local_date uses your configured timezone. Reuse next_cursor to read older pages.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyOptionalInternal,
		InputSchema:     objectSchema(map[string]any{"local_date": stringSchema(), "cursor": stringSchema(), "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 30}}, nil, false),
		OutputSchema:    objectSchema(map[string]any{"items": arraySchema(objectSchema(map[string]any{"text": stringSchema(), "published_at": stringSchema()}, []string{"text", "published_at"}, false)), "next_cursor": stringSchema(), "has_more": map[string]any{"type": "boolean"}}, []string{"items", "next_cursor", "has_more"}, false),
		SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}
func (momentInspectCapability) RequiredContext() []ContextSlot { return nil }
func (c momentInspectCapability) Execute(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.repository == nil {
		return failedCapabilityResult(inv, "moment_query_unavailable", true), errors.New("moment query unavailable")
	}
	args, err := capabilityExecutionArguments(inv, c.Definition())
	if err != nil {
		return failedCapabilityResult(inv, "invalid_arguments", false), err
	}
	self, err := c.repository.GetFluctlight(ctx, inv.Metadata.FluctlightID, inv.Metadata.AuthorizationActorID)
	if err != nil {
		return failedCapabilityResult(inv, "moment_query_unauthorized", false), err
	}
	limit := intValue(args["limit"])
	if limit == 0 {
		limit = 10
	}
	var before, from, to *time.Time
	scope := stableDigest(inv.Metadata.AuthorizationActorID + ":" + inv.Metadata.FluctlightID + ":" + stringValue(args["local_date"]))
	cursor, err := decodeOwnerReadCursor(stringValue(args["cursor"]), scope)
	if err != nil || (stringValue(args["cursor"]) != "" && (cursor.Time.IsZero() || cursor.ID == "")) {
		return handledCapabilityResult(inv, "invalid_arguments", "needs_input", map[string]any{"detail": "cursor must be returned by this query with the same local_date"}), nil
	}
	if !cursor.Time.IsZero() {
		before = &cursor.Time
	}
	if day := stringValue(args["local_date"]); day != "" {
		zone, e := time.LoadLocation(firstString(self.Identity["timezone"], "Asia/Shanghai"))
		if e != nil {
			return failedCapabilityResult(inv, "moment_timezone_invalid", false), e
		}
		start, e := time.ParseInLocation("2006-01-02", day, zone)
		if e != nil {
			return handledCapabilityResult(inv, "invalid_arguments", "needs_input", map[string]any{"detail": "local_date must use YYYY-MM-DD"}), nil
		}
		end := start.AddDate(0, 0, 1)
		from, to = &start, &end
	}
	rows, err := readWorldObjects(ctx, c.repository.Pool(), `SELECT jsonb_build_object('id',id,'text',text,'published_at',created_at) FROM public.moments WHERE owner_actor_id=$1 AND owner_fluctlight_id=$2 AND author_actor_id=$2 AND status='visible' AND ($3::timestamptz IS NULL OR (created_at,id)<($3,$7)) AND ($4::timestamptz IS NULL OR created_at>=$4) AND ($5::timestamptz IS NULL OR created_at<$5) ORDER BY created_at DESC,id DESC LIMIT $6`, inv.Metadata.AuthorizationActorID, inv.Metadata.FluctlightID, before, from, to, limit+1, cursor.ID)
	if err != nil {
		return failedCapabilityResult(inv, "moment_query_failed", true), err
	}
	more := len(rows) > limit
	next := ""
	if more {
		rows = rows[:limit]
		last := mapValue(rows[len(rows)-1])
		at, e := time.Parse(time.RFC3339Nano, stringValue(last["published_at"]))
		if e != nil {
			return failedCapabilityResult(inv, "moment_query_failed", true), e
		}
		next = encodeOwnerReadCursor(ownerReadCursor{Scope: scope, Time: at, ID: stringValue(last["id"])})
	}
	for _, raw := range rows {
		delete(mapValue(raw), "id")
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"items": rows, "next_cursor": next, "has_more": more}}, nil
}
