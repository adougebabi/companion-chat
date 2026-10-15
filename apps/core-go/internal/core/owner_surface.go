package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SurfaceError is a browser-facing failure for owner-surface routes.
// It does not change agent, reflection, wakeup, or autonomy rules.
type SurfaceError struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Replay  bool
}

func (e *SurfaceError) Error() string {
	if e == nil {
		return "owner surface failed"
	}
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

type surfaceRecord struct {
	ID           string
	OwnerID      string
	FluctlightID string
	Kind         string
	ResourceKey  string
	Status       string
	Payload      map[string]any
	Revision     int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type surfaceTx interface {
	Idempotency(ctx context.Context, owner, key string) (digest string, body map[string]any, ok bool, err error)
	SaveIdempotency(ctx context.Context, owner, key, digest string, body map[string]any) error
	Revision(ctx context.Context, owner, fid, scope string) (int, error)
	Bump(ctx context.Context, owner, fid, scope string, expected int) (int, error)
	Save(ctx context.Context, rec surfaceRecord) error
	DeleteKey(ctx context.Context, owner, fid, kind, resourceKey string) error
	Get(ctx context.Context, owner, id string) (surfaceRecord, bool, error)
	GetKey(ctx context.Context, owner, fid, kind, resourceKey string) (surfaceRecord, bool, error)
	List(ctx context.Context, owner, kind, fid, status, cursor string, limit int) ([]surfaceRecord, string, error)
	Audit(ctx context.Context, owner, fid, action, reason, recordID string) (string, error)
}

// SurfaceStore persists owner preferences, drafts, and requests.
type SurfaceStore interface {
	Transact(ctx context.Context, fn func(context.Context, surfaceTx) error) error
	ListKind(ctx context.Context, owner, kind, fid string) ([]surfaceRecord, error)
	Revision(ctx context.Context, owner, fid, scope string) (int, error)
}

// WardrobeOwnerEdit is an owner management correction of current wardrobe
// state. It is not a character wear or use action.
type WardrobeOwnerEdit struct {
	Kind     string
	ItemID   string
	Slot     string
	Active   bool
	Expected int
}

// WardrobeOwnerEditResult is the wardrobe revision after a committed owner edit.
type WardrobeOwnerEditResult struct {
	Revision int
}

// SurfaceHooks are read-only or explicitly owner-attributed adapters.
// Nil hooks mean that capability is not connected.
type SurfaceHooks struct {
	Now               func() time.Time
	Authorize         func(ctx context.Context, owner, fid string) error
	DeleteGroup       func(ctx context.Context, owner, groupID string) error
	PublishMoment     func(ctx context.Context, owner string, moment map[string]any) error
	CapabilityItems   func(ctx context.Context, owner, fid string) ([]map[string]any, error)
	ListMedia         func(ctx context.Context, owner, fid, cursor string, limit int) ([]map[string]any, any, error)
	Sidebar           func(ctx context.Context, owner, fid string) (map[string]any, error)
	MemoryRevision    func(ctx context.Context, owner, memoryID string) (int, bool, error)
	InstanceRevision  func(ctx context.Context, owner, fid string) (int, error)
	ApplyWardrobeEdit func(ctx context.Context, owner, fid string, edit WardrobeOwnerEdit) (WardrobeOwnerEditResult, error)
}

// OwnerSurface implements browser routes that sit outside the agent runtime.
// Its own records are owner drafts, requests, and audits. A confirmed wardrobe
// correction is applied through ApplyWardrobeEdit onto the wardrobe revision,
// and is recorded as an owner edit rather than the character wearing or using
// the item.
type OwnerSurface struct {
	store SurfaceStore
	hooks SurfaceHooks
}

func NewOwnerSurface(store SurfaceStore, hooks SurfaceHooks) *OwnerSurface {
	return &OwnerSurface{store: store, hooks: hooks}
}

func NewMemoryOwnerSurface(hooks SurfaceHooks) *OwnerSurface {
	return NewOwnerSurface(newMemorySurface(), hooks)
}

func (s *OwnerSurface) now() time.Time {
	if s != nil && s.hooks.Now != nil {
		return s.hooks.Now().UTC()
	}
	return time.Now().UTC()
}

type surfaceCall struct {
	name         string
	fid, a, b, c string
	read         bool
	needsKey     bool
	success      int
	cursorDomain string
}

// MatchOwnerSurface reports whether this method and internal path belong to
// the owner-surface contract. Existing routes stay on their original handlers.
func MatchOwnerSurface(method, path string) bool {
	_, ok := matchSurface(method, surfaceParts(path))
	return ok
}

func (s *OwnerSurface) Dispatch(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	if body == nil {
		body = map[string]any{}
	}
	var out map[string]any
	var biz error
	err := s.store.Transact(ctx, func(ctx context.Context, tx surfaceTx) error {
		var storeErr error
		out, biz, storeErr = s.dispatchTx(ctx, tx, method, path, query, owner, body)
		return storeErr
	})
	if err != nil {
		if surface, ok := err.(*SurfaceError); ok {
			return nil, surface
		}
		return nil, &SurfaceError{Status: 503, Code: "surface_unavailable", Message: "Owner surface is unavailable"}
	}
	return out, biz
}

func (s *OwnerSurface) dispatchTx(ctx context.Context, tx surfaceTx, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error, error) {
	op, ok := matchSurface(method, surfaceParts(path))
	if !ok {
		return nil, nil, errors.New("owner surface path was not matched")
	}
	if op.fid != "" {
		if err := s.authorize(ctx, owner, op.fid); err != nil {
			return nil, err, nil
		}
	}
	if op.read {
		result, err := s.perform(ctx, tx, op, query, owner, body)
		return result, err, storeErr(err)
	}
	result, biz, storeErr := s.mutate(ctx, tx, method, path, op, query, owner, body)
	return result, biz, storeErr
}

func storeErr(err error) error {
	if err == nil {
		return nil
	}
	var surface *SurfaceError
	if errors.As(err, &surface) {
		return nil
	}
	return err
}

func (s *OwnerSurface) authorize(ctx context.Context, owner, fid string) error {
	if s.hooks.Authorize == nil || strings.TrimSpace(fid) == "" {
		return nil
	}
	err := s.hooks.Authorize(ctx, owner, fid)
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) {
		return &SurfaceError{Status: 404, Code: "fluctlight_not_found", Message: "Fluctlight is unavailable"}
	}
	return err
}

func (s *OwnerSurface) mutate(ctx context.Context, tx surfaceTx, method, path string, op surfaceCall, query url.Values, owner string, body map[string]any) (map[string]any, error, error) {
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	if op.needsKey && key == "" {
		return nil, &SurfaceError{Status: 400, Code: "invalid_request", Message: "idempotencyKey is required"}, nil
	}
	if len(key) > 256 {
		return nil, &SurfaceError{Status: 400, Code: "invalid_request", Message: "idempotencyKey is invalid"}, nil
	}
	digest := surfaceDigest(method, path, body)
	if key != "" {
		storedDigest, stored, ok, err := tx.Idempotency(ctx, owner, key)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			if storedDigest != digest {
				return nil, &SurfaceError{Status: 422, Code: "idempotency_key_reused", Message: "Idempotency key was reused with a different payload"}, nil
			}
			replayed, replayErr := replaySurface(stored)
			return replayed, replayErr, nil
		}
	}
	result, err := s.perform(ctx, tx, op, query, owner, body)
	if err != nil {
		var surface *SurfaceError
		if !errors.As(err, &surface) {
			return nil, nil, err
		}
		if key != "" && surface.Code != "idempotency_key_reused" {
			if saveErr := tx.SaveIdempotency(ctx, owner, key, digest, errorPayload(surface)); saveErr != nil {
				return nil, nil, saveErr
			}
		}
		return nil, surface, nil
	}
	if result == nil {
		result = map[string]any{}
	}
	if _, exists := result["_status"]; !exists {
		result["_status"] = op.success
		if result["status"] == "not_implemented" || result["available"] == false {
			result["_status"] = 200
		}
	}
	if key != "" {
		if saveErr := tx.SaveIdempotency(ctx, owner, key, digest, cloneMap(result)); saveErr != nil {
			return nil, nil, saveErr
		}
	}
	return result, nil, nil
}

func replaySurface(stored map[string]any) (map[string]any, error) {
	copy := cloneMap(stored)
	if copy["__error"] == true {
		status := intValue(copy["_status"])
		if status == 0 {
			status = 400
		}
		return nil, &SurfaceError{Status: status, Code: stringValue(copy["code"]), Message: stringValue(copy["message"]), Details: mapValue(copy["details"]), Replay: true}
	}
	copy["_replay"] = true
	return copy, nil
}

func errorPayload(err *SurfaceError) map[string]any {
	payload := map[string]any{"__error": true, "_status": err.Status, "code": err.Code, "message": err.Message}
	if len(err.Details) > 0 {
		payload["details"] = err.Details
	}
	return payload
}

func (s *OwnerSurface) perform(ctx context.Context, tx surfaceTx, op surfaceCall, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	switch op.name {
	case "media_list":
		return s.mediaList(ctx, op, query, owner)
	case "sidebar":
		return s.sidebar(ctx, op, owner)
	case "nearby", "world", "place", "desires", "reflection", "harm", "agents":
		return notImplemented(nil), nil
	case "task":
		return notImplemented(map[string]any{"task_id": op.a}), nil
	case "notifications":
		return notImplemented(nil), nil
	case "unread_count", "notification_read":
		return nil, &SurfaceError{Status: 404, Code: "not_provided", Message: "Notifications are not connected"}
	case "templates":
		return s.templateList(ctx, tx, op, owner)
	case "diary_list":
		return s.diaryList(ctx, tx, op, query, owner)
	case "owner_requests":
		return s.requestList(ctx, tx, op, query, owner)
	case "owner_request":
		return s.requestGet(ctx, tx, owner, op.a)
	case "resources":
		return s.resourceList(ctx, tx, query, owner)
	case "resource":
		return s.resourceGet(ctx, tx, owner, op.a)
	case "previews":
		return s.previewList(ctx, tx, query, owner)
	case "pack_export":
		return s.packExport(ctx, tx, owner)
	case "autonomy_get":
		return s.autonomyGet(ctx, tx, op, owner)
	case "pending":
		return s.pending(ctx, op, owner, false)
	case "pending_count":
		return s.pending(ctx, op, owner, true)
	case "backups":
		return notImplemented(nil), nil
	case "backup_create", "backup_restore":
		if op.name == "backup_restore" && body["confirm"] != true {
			return nil, &SurfaceError{Status: 422, Code: "confirm_required", Message: "Restoring a backup requires confirm"}
		}
		return notImplemented(nil), nil
	case "apply_preview":
		return s.applyPreview(ctx, op, owner, body)
	case "diary_write":
		return s.diaryWrite(ctx, tx, op, owner, body)
	case "wear", "item_use", "schedule_item":
		return s.characterOrOwner(ctx, tx, op, owner, body)
	case "photo":
		if _, exists := body["expected_revision"]; exists {
			return nil, &SurfaceError{Status: 422, Code: "photo_cannot_change_facts", Message: "A photo cannot change item facts"}
		}
		return withStatus(202, map[string]any{"available": false, "status": "not_implemented", "factual": false, "label": "生成图·非事实"}), nil
	case "moment":
		return s.publishMoment(ctx, tx, owner, body)
	case "media_upload":
		return notImplemented(nil), nil
	case "group_delete":
		return s.deleteGroup(ctx, tx, op, owner, body)
	case "visual_identity":
		return s.saveSingleton(ctx, tx, op, owner, body, "visual_identity", "visual:"+op.fid, "visual_identity")
	case "template_put":
		return s.putTemplate(ctx, tx, op, owner, body)
	case "resource_write":
		return s.writeResource(ctx, tx, op, owner, body)
	case "preview_write":
		return s.writePreview(ctx, tx, op, owner, body)
	case "apply":
		return s.applyResource(ctx, tx, op, owner, body)
	case "autonomy_put":
		return s.autonomyPut(ctx, tx, op, owner, body)
	case "pack_import", "pack_validate":
		return s.pack(ctx, tx, op, owner, body)
	case "persona_switch":
		return s.personaSwitch(ctx, tx, op, owner, body)
	case "promise", "moment_suggestion", "memory_suggestion", "reflection_feedback":
		return s.queueRequest(ctx, tx, op, owner, body)
	default:
		return nil, &SurfaceError{Status: 404, Code: "not_provided", Message: "This route is not provided"}
	}
}

func (s *OwnerSurface) mediaList(ctx context.Context, op surfaceCall, query url.Values, owner string) (map[string]any, error) {
	if s.hooks.ListMedia == nil {
		return notImplemented(nil), nil
	}
	cursor := query.Get("cursor")
	if cursor != "" && (len(cursor) > 128 || strings.ContainsAny(cursor, " \t\r\n")) {
		return nil, &SurfaceError{Status: 400, Code: "media_cursor_invalid", Message: "Cursor is invalid"}
	}
	items, next, err := s.hooks.ListMedia(ctx, owner, op.fid, cursor, pageLimit(query))
	if err != nil {
		var surface *SurfaceError
		if errors.As(err, &surface) {
			return nil, surface
		}
		return nil, err
	}
	if items == nil {
		items = []map[string]any{}
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return map[string]any{"items": out, "next_cursor": next}, nil
}

func (s *OwnerSurface) sidebar(ctx context.Context, op surfaceCall, owner string) (map[string]any, error) {
	if s.hooks.Sidebar == nil {
		return notImplemented(nil), nil
	}
	return s.hooks.Sidebar(ctx, owner, op.fid)
}

func (s *OwnerSurface) templateList(ctx context.Context, tx surfaceTx, op surfaceCall, owner string) (map[string]any, error) {
	records, _, err := tx.List(ctx, owner, "world_template", op.fid, "", "", 100)
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(records))
	revision, err := tx.Revision(ctx, owner, op.fid, "templates:"+op.fid)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		item := cloneMap(record.Payload)
		item["template_id"] = record.ResourceKey
		items = append(items, item)
	}
	return map[string]any{"items": items, "revision": revision, "catalog_status": "not_implemented", "runtime_effect": "preference_only"}, nil
}

func (s *OwnerSurface) diaryList(ctx context.Context, tx surfaceTx, op surfaceCall, query url.Values, owner string) (map[string]any, error) {
	cursor := query.Get("cursor")
	if _, _, ok := decodeCursor(cursor); !ok {
		return nil, &SurfaceError{Status: 400, Code: "diary_cursor_invalid", Message: "Cursor is invalid"}
	}
	records, next, err := tx.List(ctx, owner, "diary", op.fid, "", cursor, pageLimit(query))
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(records))
	for _, record := range records {
		items = append(items, s.diaryItem(record))
	}
	var nextCursor any
	if next != "" {
		nextCursor = next
	}
	return map[string]any{"items": items, "next_cursor": nextCursor}, nil
}

func (s *OwnerSurface) diaryItem(record surfaceRecord) map[string]any {
	item := cloneMap(record.Payload)
	item["id"] = record.ID
	item["revision"] = record.Revision
	item["source"] = "owner"
	return item
}

func (s *OwnerSurface) diaryWrite(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if err := rejectPreviewSource(body); err != nil {
		return nil, err
	}
	text := strings.TrimSpace(stringValue(body["body"]))
	if text == "" || len([]rune(text)) > 8000 {
		return nil, invalid("body is required")
	}
	if _, exists := body["occurred_at"]; exists && !validTimestamp(body["occurred_at"]) {
		return nil, invalid("occurredAt is invalid")
	}
	now := s.now()
	if op.a == "" {
		rec := surfaceRecord{ID: randomID("diary_"), OwnerID: owner, FluctlightID: op.fid, Kind: "diary", Status: "saved", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{
			"date": now.Format("2006-01-02"), "body": text, "occurred_at": timestampOr(body["occurred_at"], now), "event_id": nilIfEmpty(stringValue(body["event_id"])), "refs": stringList(body["refs"]), "source": "owner", "replies": []any{},
		}}
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
		auditID, err := tx.Audit(ctx, owner, op.fid, "diary_create", stringValue(body["reason"]), rec.ID)
		if err != nil {
			return nil, err
		}
		return withStatus(201, map[string]any{"id": rec.ID, "revision": 1, "source": "owner", "audit_id": auditID}), nil
	}
	current, ok, err := tx.Get(ctx, owner, op.a)
	if err != nil {
		return nil, err
	}
	if !ok || current.Kind != "diary" || current.FluctlightID != op.fid {
		return nil, &SurfaceError{Status: 404, Code: "diary_not_found", Message: "Diary entry was not found"}
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok {
		return nil, invalid("expectedRevision is required")
	}
	if expected != current.Revision {
		return nil, conflict("diary", expected, current.Revision)
	}
	current.Revision++
	current.UpdatedAt = now
	current.Payload["body"] = text
	if _, exists := body["occurred_at"]; exists {
		current.Payload["occurred_at"] = timestampOr(body["occurred_at"], now)
	}
	if _, exists := body["event_id"]; exists {
		current.Payload["event_id"] = nilIfEmpty(stringValue(body["event_id"]))
	}
	if err := tx.Save(ctx, current); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, "diary_update", stringValue(body["reason"]), current.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": current.ID, "revision": current.Revision, "source": "owner", "audit_id": auditID}, nil
}

func (s *OwnerSurface) characterOrOwner(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	kind := map[string]string{"wear": "wardrobe_wear", "item_use": "item_use", "schedule_item": "schedule_item_suggestion"}[op.name]
	if body["confirm"] != true {
		request, err := s.insertRequest(ctx, tx, owner, op.fid, kind, body)
		if err != nil {
			return nil, err
		}
		response := map[string]any{"request": request.Payload, "audit_id": request.Audit}
		if op.name == "schedule_item" {
			revision, revErr := tx.Revision(ctx, owner, op.fid, "schedule:"+op.a)
			if revErr != nil {
				return nil, revErr
			}
			response["schedule_revision"] = revision
		}
		return withStatus(202, response), nil
	}
	if strings.TrimSpace(stringValue(body["reason"])) == "" {
		return nil, invalid("reason is required")
	}
	if op.name == "wear" || op.name == "item_use" {
		return s.ownerWardrobeEdit(ctx, tx, op, owner, body)
	}
	if op.name != "schedule_item" {
		return nil, &SurfaceError{Status: 404, Code: "not_provided", Message: "This route is not provided"}
	}
	if !validTimestamp(body["start_at"]) || !validTimestamp(body["end_at"]) || strings.TrimSpace(stringValue(body["activity"])) == "" {
		return nil, invalid("schedule item fields are invalid")
	}
	revision, err := s.bump(ctx, tx, owner, op.fid, "schedule:"+op.a, body["expected_revision"])
	if err != nil {
		return nil, err
	}
	now := s.now()
	switch op.name {
	case "schedule_item":
		id := op.b
		if id == "" {
			id = randomID("sched_item_")
		}
		rec := surfaceRecord{ID: id, OwnerID: owner, FluctlightID: op.fid, Kind: "owner_schedule_item", ResourceKey: op.a + ":" + id, Status: "active", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"id": id, "schedule_id": op.a, "activity": stringValue(body["activity"]), "scene": nilIfEmpty(stringValue(body["scene"])), "location": nilIfEmpty(stringValue(body["location"])), "start_at": stringValue(body["start_at"]), "end_at": stringValue(body["end_at"]), "source": "owner_edit", "recorded_by": "owner"}}
		if op.c == "delete" {
			rec.Status = "deleted"
		}
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, "schedule_item", stringValue(body["reason"]), op.fid)
	if err != nil {
		return nil, err
	}
	response := map[string]any{"revision": revision, "audit_id": auditID, "status": "saved", "recorded_by": "owner", "actor": "owner", "identity": "owner_edit"}
	if op.name == "schedule_item" {
		response["schedule_revision"] = revision
	}
	return withStatus(200, response), nil
}

func (s *OwnerSurface) ownerWardrobeEdit(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	expected, ok := parseRevision(body["expected_revision"])
	if !ok {
		return nil, invalid("expectedRevision is required")
	}
	edit := WardrobeOwnerEdit{Kind: op.name, Expected: expected}
	if op.name == "wear" {
		edit.ItemID = strings.TrimSpace(stringValue(body["item_id"]))
		edit.Slot = strings.TrimSpace(stringValue(body["slot"]))
		if edit.ItemID == "" || edit.Slot == "" {
			return nil, invalid("itemId and slot are required")
		}
	} else {
		edit.ItemID = strings.TrimSpace(op.a)
		active, isBool := body["active"].(bool)
		if edit.ItemID == "" || !isBool {
			return nil, invalid("itemId and active are required")
		}
		edit.Active = active
	}
	if s.hooks.ApplyWardrobeEdit == nil {
		return nil, &SurfaceError{Status: 503, Code: "surface_unavailable", Message: "Wardrobe revision is not connected"}
	}
	result, err := s.hooks.ApplyWardrobeEdit(ctx, owner, op.fid, edit)
	if err != nil {
		return nil, err
	}
	ownerRevision, err := s.nextScopeRevision(ctx, tx, owner, op.fid, "wardrobe_owner_count")
	if err != nil {
		return nil, err
	}
	now := s.now()
	if op.name == "wear" {
		rec := surfaceRecord{ID: "worn_" + shortHash(owner+"|"+op.fid+"|"+edit.Slot), OwnerID: owner, FluctlightID: op.fid, Kind: "owner_worn", ResourceKey: edit.Slot, Status: "active", Revision: ownerRevision, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"item_id": edit.ItemID, "slot": edit.Slot, "since": now.Format(time.RFC3339), "recorded_by": "owner"}}
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
	} else if edit.Active {
		rec := surfaceRecord{ID: "used_" + shortHash(owner+"|"+op.fid+"|"+edit.ItemID), OwnerID: owner, FluctlightID: op.fid, Kind: "owner_used", ResourceKey: edit.ItemID, Status: "active", Revision: ownerRevision, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"item_id": edit.ItemID, "since": now.Format(time.RFC3339), "recorded_by": "owner"}}
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
	} else if err := tx.DeleteKey(ctx, owner, op.fid, "owner_used", edit.ItemID); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, op.name, stringValue(body["reason"]), op.fid)
	if err != nil {
		return nil, err
	}
	return withStatus(200, map[string]any{"revision": result.Revision, "owner_revision": ownerRevision, "audit_id": auditID, "status": "saved", "recorded_by": "owner", "actor": "owner", "identity": "owner_edit"}), nil
}

func (s *OwnerSurface) nextScopeRevision(ctx context.Context, tx surfaceTx, owner, fid, scope string) (int, error) {
	current, err := tx.Revision(ctx, owner, fid, scope)
	if err != nil {
		return 0, err
	}
	return tx.Bump(ctx, owner, fid, scope, current)
}

func (s *OwnerSurface) publishMoment(ctx context.Context, tx surfaceTx, owner string, body map[string]any) (map[string]any, error) {
	if momentAuthorForbidden(body) {
		return nil, &SurfaceError{Status: 422, Code: "moment_author_forbidden", Message: "The owner cannot publish as a character"}
	}
	text := strings.TrimSpace(stringValue(body["text"]))
	if text == "" || len([]rune(text)) > 2000 {
		return nil, invalid("text is required and must be at most 2000 characters")
	}
	now := s.now()
	id := randomID("moment_")
	moment := map[string]any{"id": id, "author_actor_id": owner, "author_type": "user", "text": text, "media_ids": stringList(body["media_ids"]), "world_id": nilIfEmpty(stringValue(body["world_id"])), "refs": stringList(body["refs"]), "created_at": now.Format(time.RFC3339)}
	if s.hooks.PublishMoment != nil && stringValue(body["world_id"]) != "" {
		if err := s.hooks.PublishMoment(ctx, owner, moment); err != nil {
			if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) {
				return nil, &SurfaceError{Status: 404, Code: "fluctlight_not_found", Message: "Fluctlight is unavailable"}
			}
			return nil, err
		}
	}
	rec := surfaceRecord{ID: id, OwnerID: owner, FluctlightID: stringValue(body["world_id"]), Kind: "user_moment", ResourceKey: id, Status: "visible", CreatedAt: now, UpdatedAt: now, Revision: 1, Payload: moment}
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, stringValue(body["world_id"]), "moment_publish", "", id)
	if err != nil {
		return nil, err
	}
	return withStatus(201, map[string]any{"id": id, "author_actor_id": owner, "author_type": "user", "created_at": moment["created_at"], "audit_id": auditID}), nil
}

func (s *OwnerSurface) deleteGroup(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if _, err := s.bump(ctx, tx, owner, "", "group:"+op.a, body["expected_revision"]); err != nil {
		return nil, err
	}
	if s.hooks.DeleteGroup == nil {
		return notImplemented(nil), nil
	}
	if err := s.hooks.DeleteGroup(ctx, owner, op.a); err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) {
			return nil, &SurfaceError{Status: 404, Code: "actor_group_not_found", Message: "Actor group was not found"}
		}
		return nil, err
	}
	if _, err := tx.Audit(ctx, owner, "", "actor_group_delete", stringValue(body["reason"]), op.a); err != nil {
		return nil, err
	}
	return map[string]any{"_status": 204}, nil
}

func (s *OwnerSurface) saveSingleton(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any, kind, scope, action string) (map[string]any, error) {
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	revision, err := s.bump(ctx, tx, owner, op.fid, scope, body["expected_revision"])
	if err != nil {
		return nil, err
	}
	now := s.now()
	payload := cloneMap(body)
	delete(payload, "idempotency_key")
	delete(payload, "expected_revision")
	delete(payload, "reason")
	rec := surfaceRecord{ID: kind + "_" + shortHash(owner+"|"+op.fid), OwnerID: owner, FluctlightID: op.fid, Kind: kind, ResourceKey: "current", Status: "saved", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: payload}
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, action, stringValue(body["reason"]), rec.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"current_revision": revision, "revision": revision, "status": "saved", "audit_id": auditID, "identity": "owner_edit"}, nil
}

func (s *OwnerSurface) putTemplate(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if _, isBool := body["enabled"].(bool); !isBool {
		return nil, invalid("enabled is required")
	}
	if containsPermanentHarm(body) || strings.Contains(strings.ToLower(op.a), "permanent_harm") {
		return nil, harmError()
	}
	revision, err := s.bump(ctx, tx, owner, op.fid, "templates:"+op.fid, body["expected_revision"])
	if err != nil {
		return nil, err
	}
	now := s.now()
	rec := surfaceRecord{ID: "tpl_" + shortHash(owner+"|"+op.fid+"|"+op.a), OwnerID: owner, FluctlightID: op.fid, Kind: "world_template", ResourceKey: op.a, Status: "saved", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"template_id": op.a, "name": op.a, "category": nil, "enabled": body["enabled"], "permanent_harm": false, "locked": false}}
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, "world_template", stringValue(body["reason"]), rec.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"template_id": op.a, "enabled": body["enabled"], "revision": revision, "audit_id": auditID, "runtime_effect": "preference_only"}, nil
}

func (s *OwnerSurface) writeResource(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	kind := stringValue(body["kind"])
	if op.a == "" && !validResourceKind(kind) {
		return nil, invalid("kind is invalid")
	}
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" || len([]rune(name)) > 200 {
		return nil, invalid("name is required")
	}
	if _, isObject := body["body"].(map[string]any); body["body"] != nil && !isObject {
		return nil, invalid("body must be an object")
	}
	now := s.now()
	if op.a == "" {
		version := "1"
		rec := surfaceRecord{ID: randomID("res_"), OwnerID: owner, Kind: "studio_resource", Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"kind": kind, "name": name, "body": body["body"], "published_version": nil, "draft_version": version, "versions": []any{map[string]any{"version": version, "created_at": now.Format(time.RFC3339)}}, "referenced_by": []any{}}}
		rec.ResourceKey = rec.ID
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
		auditID, err := tx.Audit(ctx, owner, "", "studio_resource_create", stringValue(body["reason"]), rec.ID)
		if err != nil {
			return nil, err
		}
		return withStatus(201, s.resourceView(rec, auditID)), nil
	}
	current, ok, err := tx.Get(ctx, owner, op.a)
	if err != nil {
		return nil, err
	}
	if !ok || current.Kind != "studio_resource" {
		return nil, &SurfaceError{Status: 404, Code: "studio_resource_not_found", Message: "Studio resource was not found"}
	}
	expected := strings.TrimSpace(stringValue(body["expected_version"]))
	draft := stringValue(current.Payload["draft_version"])
	published := stringValue(current.Payload["published_version"])
	if expected == "" || (expected != draft && expected != published) {
		return nil, &SurfaceError{Status: 409, Code: "revision_conflict", Message: "Studio resource version conflict", Details: map[string]any{"current_version": draft, "expected_version": expected, "current_revision": current.Revision, "expected_revision": expected, "resource": "studio_resource"}}
	}
	next := strconv.Itoa(current.Revision + 1)
	current.Revision++
	current.UpdatedAt = now
	current.Payload["name"] = name
	if body["body"] != nil {
		current.Payload["body"] = body["body"]
	}
	if kind != "" {
		current.Payload["kind"] = kind
	}
	current.Payload["draft_version"] = next
	versions := arrayValue(current.Payload["versions"])
	current.Payload["versions"] = append(versions, map[string]any{"version": next, "created_at": now.Format(time.RFC3339)})
	if err := tx.Save(ctx, current); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, "", "studio_resource_update", stringValue(body["reason"]), current.ID)
	if err != nil {
		return nil, err
	}
	return s.resourceView(current, auditID), nil
}

func (s *OwnerSurface) resourceView(record surfaceRecord, auditID string) map[string]any {
	view := cloneMap(record.Payload)
	view["id"] = record.ID
	view["audit_id"] = auditID
	view["revision"] = record.Revision
	if _, exists := view["referenced_by"]; !exists {
		view["referenced_by"] = []any{}
	}
	return view
}

func (s *OwnerSurface) resourceList(ctx context.Context, tx surfaceTx, query url.Values, owner string) (map[string]any, error) {
	cursor := query.Get("cursor")
	if _, _, ok := decodeCursor(cursor); !ok {
		return nil, &SurfaceError{Status: 400, Code: "studio_cursor_invalid", Message: "Cursor is invalid"}
	}
	records, next, err := tx.List(ctx, owner, "studio_resource", "", "", cursor, pageLimit(query))
	if err != nil {
		return nil, err
	}
	kind := query.Get("kind")
	items := make([]any, 0, len(records))
	for _, record := range records {
		if kind != "" && stringValue(record.Payload["kind"]) != kind {
			continue
		}
		items = append(items, s.resourceView(record, ""))
	}
	var nextCursor any
	if next != "" {
		nextCursor = next
	}
	return map[string]any{"items": items, "next_cursor": nextCursor}, nil
}

func (s *OwnerSurface) resourceGet(ctx context.Context, tx surfaceTx, owner, id string) (map[string]any, error) {
	record, ok, err := tx.Get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if !ok || record.Kind != "studio_resource" {
		return nil, &SurfaceError{Status: 404, Code: "studio_resource_not_found", Message: "Studio resource was not found"}
	}
	return s.resourceView(record, ""), nil
}

func (s *OwnerSurface) writePreview(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if previewLifeWrite(body) {
		return nil, &SurfaceError{Status: 422, Code: "preview_life_write_forbidden", Message: "A studio preview cannot write life facts"}
	}
	text := strings.TrimSpace(stringValue(body["text"]))
	if text == "" {
		return nil, invalid("text is required")
	}
	now := s.now()
	rec := surfaceRecord{ID: randomID("preview_"), OwnerID: owner, Kind: "studio_draft", Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"preview_id": "", "resource_id": nilIfEmpty(stringValue(body["resource_id"])), "text": text, "label": "创作预览·不进入生活", "life_write": false, "created_at": now.Format(time.RFC3339)}}
	rec.ResourceKey = rec.ID
	rec.Payload["preview_id"] = rec.ID
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, "", "studio_preview", "", rec.ID)
	if err != nil {
		return nil, err
	}
	view := cloneMap(rec.Payload)
	view["audit_id"] = auditID
	return withStatus(201, view), nil
}

func (s *OwnerSurface) previewList(ctx context.Context, tx surfaceTx, query url.Values, owner string) (map[string]any, error) {
	records, _, err := tx.List(ctx, owner, "studio_draft", "", "", "", pageLimit(query))
	if err != nil {
		return nil, err
	}
	resourceID := query.Get("resourceId")
	if resourceID == "" {
		resourceID = query.Get("resource_id")
	}
	items := make([]any, 0, len(records))
	for _, record := range records {
		if resourceID != "" && stringValue(record.Payload["resource_id"]) != resourceID {
			continue
		}
		items = append(items, cloneMap(record.Payload))
	}
	return map[string]any{"items": items, "next_cursor": nil}, nil
}

func (s *OwnerSurface) applyPreview(ctx context.Context, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	var revision any
	if s.hooks.InstanceRevision != nil && stringValue(body["fluctlight_id"]) != "" {
		if err := s.authorize(ctx, owner, stringValue(body["fluctlight_id"])); err != nil {
			return nil, err
		}
		current, err := s.hooks.InstanceRevision(ctx, owner, stringValue(body["fluctlight_id"]))
		if err != nil {
			return nil, err
		}
		revision = current
	}
	return map[string]any{"available": false, "status": "not_implemented", "diff": nil, "conflicts": nil, "instance_revision": revision, "resource_id": op.a}, nil
}

func (s *OwnerSurface) applyResource(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	fid := stringValue(body["fluctlight_id"])
	if err := s.authorize(ctx, owner, fid); err != nil {
		return nil, err
	}
	record, ok, err := tx.Get(ctx, owner, op.a)
	if err != nil {
		return nil, err
	}
	if !ok || record.Kind != "studio_resource" {
		return nil, &SurfaceError{Status: 404, Code: "studio_resource_not_found", Message: "Studio resource was not found"}
	}
	version := stringValue(body["resource_version"])
	if version == "" || (version != stringValue(record.Payload["draft_version"]) && version != stringValue(record.Payload["published_version"])) {
		return nil, &SurfaceError{Status: 409, Code: "revision_conflict", Message: "Studio resource version conflict", Details: map[string]any{"current_version": record.Payload["draft_version"], "expected_version": version, "resource": "studio_resource"}}
	}
	var recordedRevision any
	if s.hooks.InstanceRevision != nil {
		current, revErr := s.hooks.InstanceRevision(ctx, owner, fid)
		if revErr != nil {
			return nil, revErr
		}
		expected, ok := parseRevision(body["expected_instance_revision"])
		if !ok || expected != current {
			return nil, conflict("fluctlight", expected, current)
		}
		recordedRevision = current
	}
	for _, raw := range arrayValue(body["choices"]) {
		choice := mapValue(raw)
		decision := stringValue(choice["decision"])
		if decision != "keep" && decision != "override" {
			return nil, invalid("choices must decide keep or override")
		}
	}
	now := s.now()
	choices := body["choices"]
	rec := surfaceRecord{ID: randomID("apply_"), OwnerID: owner, FluctlightID: fid, Kind: "studio_apply", Status: "recorded_not_applied", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{
		"resource_id": op.a, "fluctlight_id": fid, "resource_version": version, "choices": choices,
		"instance_revision": recordedRevision, "status": "recorded_not_applied", "runtime_effect": "recorded_only",
		"conflicts_resolved": false,
	}}
	rec.ResourceKey = rec.ID
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	refs := arrayValue(record.Payload["referenced_by"])
	record.Payload["referenced_by"] = append(refs, map[string]any{"fluctlight_id": fid, "selected_version": version, "status": "recorded_not_applied", "instance_revision": recordedRevision})
	if err := tx.Save(ctx, record); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, fid, "studio_apply", stringValue(body["reason"]), rec.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"status": "recorded_not_applied", "runtime_effect": "recorded_only", "applied": false,
		"resource_id": op.a, "resource_version": version, "instance_revision": recordedRevision,
		"instance_revision_unchanged": true, "choices": choices, "conflicts": nil, "audit_id": auditID,
	}, nil
}

func (s *OwnerSurface) autonomyGet(ctx context.Context, tx surfaceTx, op surfaceCall, owner string) (map[string]any, error) {
	record, ok, err := tx.GetKey(ctx, owner, op.fid, "autonomy_policy", "current")
	if err != nil {
		return nil, err
	}
	if !ok {
		return map[string]any{"level": nil, "quiet_hours": nil, "spend_cap": nil, "outreach_limit": nil, "allow_moments": nil, "confirm_action_types": nil, "revision": 0, "effective_at": nil, "runtime_effect": "preference_only"}, nil
	}
	view := cloneMap(record.Payload)
	view["revision"] = record.Revision
	view["runtime_effect"] = "preference_only"
	return view, nil
}

func (s *OwnerSurface) autonomyPut(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	revision, err := s.bump(ctx, tx, owner, op.fid, "autonomy:"+op.fid, body["expected_revision"])
	if err != nil {
		return nil, err
	}
	now := s.now()
	payload := map[string]any{
		"level": body["level"], "quiet_hours": body["quiet_hours"], "spend_cap": body["spend_cap"],
		"outreach_limit": body["outreach_limit"], "allow_moments": body["allow_moments"], "confirm_action_types": stringList(body["confirm_action_types"]),
		"effective_at": now.Format(time.RFC3339), "runtime_effect": "preference_only",
	}
	rec := surfaceRecord{ID: "auto_" + shortHash(owner+"|"+op.fid), OwnerID: owner, FluctlightID: op.fid, Kind: "autonomy_policy", ResourceKey: "current", Status: "saved", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: payload}
	if err := tx.Save(ctx, rec); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, op.fid, "autonomy_policy", stringValue(body["reason"]), rec.ID)
	if err != nil {
		return nil, err
	}
	payload["revision"] = revision
	payload["audit_id"] = auditID
	return payload, nil
}

func (s *OwnerSurface) pack(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	pack := mapValue(body["pack"])
	if len(pack) == 0 {
		return nil, invalid("pack is required")
	}
	if packHasRuntimeFacts(pack) || packHasRuntimeFacts(body) {
		return nil, &SurfaceError{Status: 422, Code: "pack_contains_runtime_facts", Message: "A configuration pack cannot contain runtime facts"}
	}
	if containsPermanentHarm(pack) {
		return nil, harmError()
	}
	if op.name == "pack_validate" {
		return map[string]any{"valid": true, "status": "validated"}, nil
	}
	resources := arrayValue(pack["resources"])
	now := s.now()
	versions := make([]any, 0, len(resources))
	for _, raw := range resources {
		item := mapValue(raw)
		if !validResourceKind(stringValue(item["kind"])) {
			return nil, invalid("pack resource kind is invalid")
		}
		rec := surfaceRecord{ID: randomID("res_"), OwnerID: owner, Kind: "studio_resource", Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"kind": item["kind"], "name": item["name"], "body": item["body"], "published_version": nil, "draft_version": "1", "versions": []any{map[string]any{"version": "1", "created_at": now.Format(time.RFC3339)}}, "referenced_by": []any{}}}
		rec.ResourceKey = rec.ID
		if err := tx.Save(ctx, rec); err != nil {
			return nil, err
		}
		versions = append(versions, map[string]any{"resource_id": rec.ID, "version": "1"})
	}
	packID := randomID("pack_")
	if err := tx.Save(ctx, surfaceRecord{ID: packID, OwnerID: owner, Kind: "studio_pack", ResourceKey: packID, Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"resource_versions": versions}}); err != nil {
		return nil, err
	}
	auditID, err := tx.Audit(ctx, owner, "", "studio_pack_import", "", packID)
	if err != nil {
		return nil, err
	}
	return withStatus(201, map[string]any{"pack_id": packID, "resource_versions": versions, "audit_id": auditID, "status": "draft"}), nil
}

func (s *OwnerSurface) packExport(ctx context.Context, tx surfaceTx, owner string) (map[string]any, error) {
	records, _, err := tx.List(ctx, owner, "studio_resource", "", "", "", 100)
	if err != nil {
		return nil, err
	}
	resources := make([]any, 0, len(records))
	for _, record := range records {
		resources = append(resources, map[string]any{"id": record.ID, "kind": record.Payload["kind"], "name": record.Payload["name"], "body": record.Payload["body"], "draft_version": record.Payload["draft_version"], "published_version": record.Payload["published_version"]})
	}
	return map[string]any{"pack": map[string]any{"resources": resources}, "available": true}, nil
}

func (s *OwnerSurface) personaSwitch(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	if body["confirm"] != true {
		return nil, &SurfaceError{Status: 422, Code: "confirm_required", Message: "Persona switch requires confirm"}
	}
	if strings.TrimSpace(stringValue(body["target_persona_id"])) == "" {
		return nil, invalid("targetPersonaId is required")
	}
	if err := s.checkInstanceRevision(ctx, op.fid, owner, body["expected_revision"]); err != nil {
		return nil, err
	}
	request, err := s.insertRequest(ctx, tx, owner, op.fid, "persona_switch", body)
	if err != nil {
		return nil, err
	}
	return withStatus(202, map[string]any{"revision": request.Payload["revision"], "audit_id": request.Audit, "status": "pending", "request": request.Payload}), nil
}

func (s *OwnerSurface) queueRequest(ctx context.Context, tx surfaceTx, op surfaceCall, owner string, body map[string]any) (map[string]any, error) {
	kind := op.name
	switch op.name {
	case "memory_suggestion":
		if err := s.checkMemory(ctx, owner, op.a, body["expected_revision"]); err != nil {
			return nil, err
		}
		if strings.TrimSpace(stringValue(body["text"])) == "" {
			return nil, invalid("text is required")
		}
	case "promise":
		if strings.TrimSpace(stringValue(body["title"])) == "" {
			return nil, invalid("title is required")
		}
	case "moment_suggestion":
		if momentAuthorForbidden(body) {
			return nil, &SurfaceError{Status: 422, Code: "moment_author_forbidden", Message: "The owner cannot publish as a character"}
		}
		if strings.TrimSpace(stringValue(body["text"])) == "" {
			return nil, invalid("text is required")
		}
	case "reflection_feedback":
		if strings.TrimSpace(stringValue(body["text"])) == "" {
			return nil, invalid("text is required")
		}
	}
	request, err := s.insertRequest(ctx, tx, owner, op.fid, kind, body)
	if err != nil {
		return nil, err
	}
	return withStatus(202, map[string]any{"request": request.Payload, "audit_id": request.Audit}), nil
}

type queuedRequest struct {
	Payload map[string]any
	Audit   string
}

func (s *OwnerSurface) insertRequest(ctx context.Context, tx surfaceTx, owner, fid, kind string, body map[string]any) (queuedRequest, error) {
	now := s.now()
	id := randomID("req_")
	payload := map[string]any{
		"id": id, "kind": kind, "fluctlight_id": fid, "status": "pending", "payload": requestPayload(body),
		"decided_by": nil, "decision_reason": nil, "result_refs": []any{},
		"created_at": now.Format(time.RFC3339), "updated_at": now.Format(time.RFC3339), "revision": 1,
	}
	rec := surfaceRecord{ID: id, OwnerID: owner, FluctlightID: fid, Kind: "owner_request", ResourceKey: id, Status: "pending", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: payload}
	if err := tx.Save(ctx, rec); err != nil {
		return queuedRequest{}, err
	}
	auditID, err := tx.Audit(ctx, owner, fid, "request:"+kind, stringValue(body["reason"]), id)
	if err != nil {
		return queuedRequest{}, err
	}
	return queuedRequest{Payload: payload, Audit: auditID}, nil
}

func (s *OwnerSurface) requestGet(ctx context.Context, tx surfaceTx, owner, id string) (map[string]any, error) {
	record, ok, err := tx.Get(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if !ok || record.Kind != "owner_request" {
		return nil, &SurfaceError{Status: 404, Code: "owner_request_not_found", Message: "Owner request was not found"}
	}
	return cloneMap(record.Payload), nil
}

func (s *OwnerSurface) requestList(ctx context.Context, tx surfaceTx, op surfaceCall, query url.Values, owner string) (map[string]any, error) {
	cursor := query.Get("cursor")
	if _, _, ok := decodeCursor(cursor); !ok {
		return nil, &SurfaceError{Status: 400, Code: "owner_request_cursor_invalid", Message: "Cursor is invalid"}
	}
	status := query.Get("status")
	records, next, err := tx.List(ctx, owner, "owner_request", op.fid, status, cursor, pageLimit(query))
	if err != nil {
		return nil, err
	}
	items := make([]any, 0, len(records))
	for _, record := range records {
		items = append(items, cloneMap(record.Payload))
	}
	var nextCursor any
	if next != "" {
		nextCursor = next
	}
	return map[string]any{"items": items, "next_cursor": nextCursor}, nil
}

func (s *OwnerSurface) pending(ctx context.Context, op surfaceCall, owner string, countOnly bool) (map[string]any, error) {
	groups := make([]any, 0, 5)
	count := 0
	partial := false
	if s.hooks.CapabilityItems != nil {
		items, err := s.hooks.CapabilityItems(ctx, owner, op.fid)
		if err != nil {
			groups = append(groups, map[string]any{"kind": "capability", "status": "unwired"})
			partial = true
		} else {
			mapped := make([]any, 0, len(items))
			for _, item := range items {
				status := stringValue(item["status"])
				if status != "" && status != "proposed" && status != "reviewing" {
					continue
				}
				if op.fid != "" && stringValue(item["fluctlight_id"]) != "" && stringValue(item["fluctlight_id"]) != op.fid {
					continue
				}
				mapped = append(mapped, map[string]any{"id": item["id"], "title": item["title"], "href": "/pending", "created_at": item["created_at"]})
			}
			groups = append(groups, map[string]any{"kind": "capability", "status": "ok", "items": mapped})
			count += len(mapped)
		}
	} else {
		groups = append(groups, map[string]any{"kind": "capability", "status": "unwired"})
		partial = true
	}
	for _, kind := range []string{"persona_proposal", "studio_apply", "failed_task", "owner_request"} {
		groups = append(groups, map[string]any{"kind": kind, "status": "unwired"})
		partial = true
	}
	if countOnly {
		return map[string]any{"count": count, "partial": partial}, nil
	}
	return map[string]any{"groups": groups}, nil
}

func (s *OwnerSurface) bump(ctx context.Context, tx surfaceTx, owner, fid, scope string, expectedRaw any) (int, error) {
	expected, ok := parseRevision(expectedRaw)
	if !ok {
		return 0, invalid("expectedRevision is required")
	}
	revision, err := tx.Bump(ctx, owner, fid, scope, expected)
	if err != nil {
		var conflictErr revisionConflictError
		if errors.As(err, &conflictErr) {
			return 0, conflict(scope, expected, conflictErr.Current)
		}
		return 0, err
	}
	return revision, nil
}

func (s *OwnerSurface) checkInstanceRevision(ctx context.Context, fid, owner string, expectedRaw any) error {
	expected, ok := parseRevision(expectedRaw)
	if !ok {
		return invalid("expectedRevision is required")
	}
	if s.hooks.InstanceRevision == nil {
		if expected != 0 {
			return conflict("fluctlight", expected, 0)
		}
		return nil
	}
	current, err := s.hooks.InstanceRevision(ctx, owner, fid)
	if err != nil {
		return err
	}
	if current != expected {
		return conflict("fluctlight", expected, current)
	}
	return nil
}

func (s *OwnerSurface) checkMemory(ctx context.Context, owner, memoryID string, expectedRaw any) error {
	expected, ok := parseRevision(expectedRaw)
	if !ok {
		return invalid("expectedRevision is required")
	}
	if s.hooks.MemoryRevision == nil {
		return nil
	}
	current, found, err := s.hooks.MemoryRevision(ctx, owner, memoryID)
	if err != nil {
		return err
	}
	if !found {
		return &SurfaceError{Status: 404, Code: "memory_not_found", Message: "Memory was not found"}
	}
	if current != expected {
		return conflict("memory", expected, current)
	}
	return nil
}

type revisionConflictError struct{ Current int }

func (e revisionConflictError) Error() string { return "revision_conflict" }

func conflict(resource string, expected, current int) *SurfaceError {
	return &SurfaceError{Status: 409, Code: "revision_conflict", Message: "Revision conflict", Details: map[string]any{"current_revision": current, "expected_revision": expected, "resource": resource}}
}

func invalid(message string) *SurfaceError {
	return &SurfaceError{Status: 400, Code: "invalid_request", Message: message}
}

func harmError() *SurfaceError {
	return &SurfaceError{Status: 422, Code: "permanent_harm_forbidden", Message: "Permanent harm cannot be enabled"}
}

func notImplemented(extra map[string]any) map[string]any {
	out := map[string]any{"available": false, "status": "not_implemented"}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func withStatus(status int, body map[string]any) map[string]any {
	body["_status"] = status
	return body
}

func requestPayload(body map[string]any) map[string]any {
	payload := cloneMap(body)
	delete(payload, "idempotency_key")
	delete(payload, "expected_revision")
	delete(payload, "expected_life_context_revision")
	return payload
}

func rejectPreviewSource(body map[string]any) error {
	source := strings.ToLower(stringValue(body["source"]))
	switch source {
	case "preview", "studio", "generated", "trial", "draft":
		return &SurfaceError{Status: 422, Code: "preview_life_write_forbidden", Message: "Preview content cannot be written into the diary"}
	default:
		return nil
	}
}

func momentAuthorForbidden(body map[string]any) bool {
	for key := range body {
		switch strings.ToLower(strings.ReplaceAll(key, "_", "")) {
		case "authorfluctlightid", "asfluctlight", "onbehalfof", "authoractorid":
			return true
		}
	}
	return false
}

func previewLifeWrite(body map[string]any) bool {
	for _, key := range []string{"memory", "memories", "schedule", "diary", "item", "items", "wardrobe", "media_library", "mediaLibrary", "life_facts", "lifeFacts"} {
		if _, exists := body[key]; exists {
			return true
		}
	}
	return body["life_write"] == true || body["lifeWrite"] == true
}

func packHasRuntimeFacts(body map[string]any) bool {
	for _, key := range []string{"memories", "diary", "schedule", "schedules", "relationships", "moments", "conversations", "inner_state", "innerState", "presence", "life_facts", "events"} {
		if _, exists := body[key]; exists {
			return true
		}
	}
	return false
}

func containsPermanentHarm(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if typed["permanent_harm"] == true || typed["permanentHarm"] == true || stringValue(typed["harm"]) == "permanent" {
			return true
		}
		for _, key := range []string{"category", "kind", "type"} {
			if strings.EqualFold(stringValue(typed[key]), "permanent_harm") {
				return true
			}
		}
		for key, child := range typed {
			if key == "reason" || key == "text" || key == "title" || key == "name" || key == "description" || key == "prompt" || key == "message" {
				continue
			}
			if containsPermanentHarm(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsPermanentHarm(child) {
				return true
			}
		}
	}
	return false
}

func validResourceKind(kind string) bool {
	switch kind {
	case "character_template", "persona_fragment", "place", "item", "event_template", "visual_style":
		return true
	default:
		return false
	}
}

func validTimestamp(value any) bool {
	text := strings.TrimSpace(stringValue(value))
	if text == "" {
		return false
	}
	if _, err := time.Parse(time.RFC3339, text); err == nil {
		return true
	}
	_, err := time.Parse(time.RFC3339Nano, text)
	return err == nil
}

func timestampOr(value any, fallback time.Time) string {
	if validTimestamp(value) {
		return stringValue(value)
	}
	return fallback.Format(time.RFC3339)
}

func nilIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func stringList(value any) []any {
	items := arrayValue(value)
	if items == nil {
		return []any{}
	}
	return items
}

func parseRevision(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed < 0 || typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case int:
		if typed < 0 {
			return 0, false
		}
		return typed, true
	case int64:
		if typed < 0 {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil || parsed < 0 {
			return 0, false
		}
		return int(parsed), true
	default:
		return 0, false
	}
}

func pageLimit(query url.Values) int {
	if query == nil || query.Get("limit") == "" {
		return 50
	}
	parsed, err := strconv.Atoi(query.Get("limit"))
	if err != nil || parsed < 1 {
		return 50
	}
	if parsed > 100 {
		return 100
	}
	return parsed
}

func surfaceDigest(method, path string, body map[string]any) string {
	raw, _ := json.Marshal(map[string]any{"method": method, "path": path, "body": body})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:16])
}

func surfaceParts(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) > 0 && (parts[0] == "internal" || parts[0] == "api") {
		parts = parts[1:]
	}
	if len(parts) == 1 && parts[0] == "" {
		return nil
	}
	return parts
}

func matchSurface(method string, parts []string) (surfaceCall, bool) {
	call := surfaceCall{success: 200, needsKey: method != "GET" && method != "HEAD"}
	switch {
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "media" && method == "GET":
		call.name, call.fid, call.read = "media_list", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "sidebar" && method == "GET":
		call.name, call.fid, call.read = "sidebar", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "nearby" && method == "GET":
		call.name, call.fid, call.read = "nearby", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "diary" && method == "GET":
		call.name, call.fid, call.read = "diary_list", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "diary" && method == "POST":
		call.name, call.fid, call.success = "diary_write", parts[1], 201
	case len(parts) == 4 && parts[0] == "fluctlights" && parts[2] == "diary" && method == "PUT":
		call.name, call.fid, call.a = "diary_write", parts[1], parts[3]
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "world" && method == "GET":
		call.name, call.fid, call.read = "world", parts[1], true
	case len(parts) == 4 && parts[0] == "fluctlights" && parts[2] == "world" && parts[3] == "templates" && method == "GET":
		call.name, call.fid, call.read = "templates", parts[1], true
	case len(parts) == 5 && parts[0] == "fluctlights" && parts[2] == "world" && parts[3] == "templates" && method == "PUT":
		call.name, call.fid, call.a = "template_put", parts[1], parts[4]
	case len(parts) == 5 && parts[0] == "fluctlights" && parts[2] == "world" && parts[3] == "places" && method == "GET":
		call.name, call.fid, call.a, call.read = "place", parts[1], parts[4], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "autonomy-policy" && method == "GET":
		call.name, call.fid, call.read = "autonomy_get", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "autonomy-policy" && method == "PUT":
		call.name, call.fid = "autonomy_put", parts[1]
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "desires" && method == "GET":
		call.name, call.fid, call.read = "desires", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "owner-requests" && method == "GET":
		call.name, call.fid, call.read = "owner_requests", parts[1], true
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "promises" && method == "POST":
		call.name, call.fid, call.success = "promise", parts[1], 202
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "moment-suggestions" && method == "POST":
		call.name, call.fid, call.success = "moment_suggestion", parts[1], 202
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "persona-switch" && method == "POST":
		call.name, call.fid, call.success = "persona_switch", parts[1], 202
	case len(parts) == 3 && parts[0] == "fluctlights" && parts[2] == "visual-identity" && method == "PUT":
		call.name, call.fid = "visual_identity", parts[1]
	case len(parts) == 4 && parts[0] == "fluctlights" && parts[2] == "wardrobe" && parts[3] == "wear" && method == "POST":
		call.name, call.fid, call.success = "wear", parts[1], 202
	case len(parts) == 6 && parts[0] == "fluctlights" && parts[2] == "wardrobe" && parts[3] == "items" && parts[5] == "use" && method == "POST":
		call.name, call.fid, call.a, call.success = "item_use", parts[1], parts[4], 202
	case len(parts) == 6 && parts[0] == "fluctlights" && parts[2] == "wardrobe" && parts[3] == "items" && parts[5] == "photos" && method == "POST":
		call.name, call.fid, call.a, call.success = "photo", parts[1], parts[4], 202
	case len(parts) == 5 && parts[0] == "fluctlights" && parts[2] == "schedules" && parts[4] == "items" && method == "POST":
		call.name, call.fid, call.a, call.success = "schedule_item", parts[1], parts[3], 202
	case len(parts) == 6 && parts[0] == "fluctlights" && parts[2] == "schedules" && parts[4] == "items" && (method == "PUT" || method == "DELETE"):
		call.name, call.fid, call.a, call.b, call.success = "schedule_item", parts[1], parts[3], parts[5], 202
		if method == "DELETE" {
			call.c = "delete"
		}
	case len(parts) == 4 && parts[0] == "fluctlights" && parts[2] == "reflections" && method == "GET":
		call.name, call.fid, call.a, call.read = "reflection", parts[1], parts[3], true
	case len(parts) == 5 && parts[0] == "fluctlights" && parts[2] == "reflections" && parts[4] == "feedback" && method == "POST":
		call.name, call.fid, call.a, call.success = "reflection_feedback", parts[1], parts[3], 202
	case len(parts) == 1 && parts[0] == "notifications" && method == "GET":
		call.name, call.read, call.needsKey = "notifications", true, false
	case len(parts) == 2 && parts[0] == "notifications" && parts[1] == "unread-count" && method == "GET":
		call.name, call.read, call.needsKey = "unread_count", true, false
	case len(parts) == 3 && parts[0] == "notifications" && parts[2] == "read" && method == "POST":
		call.name, call.a = "notification_read", parts[1]
	case len(parts) == 1 && parts[0] == "pending" && method == "GET":
		call.name, call.read, call.needsKey = "pending", true, false
	case len(parts) == 2 && parts[0] == "pending" && parts[1] == "count" && method == "GET":
		call.name, call.read, call.needsKey = "pending_count", true, false
	case len(parts) == 2 && parts[0] == "studio" && parts[1] == "resources" && method == "GET":
		call.name, call.read, call.needsKey = "resources", true, false
	case len(parts) == 2 && parts[0] == "studio" && parts[1] == "resources" && method == "POST":
		call.name, call.success = "resource_write", 201
	case len(parts) == 3 && parts[0] == "studio" && parts[1] == "resources" && method == "GET":
		call.name, call.a, call.read, call.needsKey = "resource", parts[2], true, false
	case len(parts) == 3 && parts[0] == "studio" && parts[1] == "resources" && method == "PUT":
		call.name, call.a = "resource_write", parts[2]
	case len(parts) == 4 && parts[0] == "studio" && parts[1] == "resources" && parts[3] == "apply-preview" && method == "POST":
		call.name, call.a, call.needsKey = "apply_preview", parts[2], false
	case len(parts) == 4 && parts[0] == "studio" && parts[1] == "resources" && parts[3] == "apply" && method == "POST":
		call.name, call.a = "apply", parts[2]
	case len(parts) == 2 && parts[0] == "studio" && parts[1] == "previews" && method == "GET":
		call.name, call.read, call.needsKey = "previews", true, false
	case len(parts) == 2 && parts[0] == "studio" && parts[1] == "previews" && method == "POST":
		call.name, call.success = "preview_write", 201
	case len(parts) == 2 && parts[0] == "studio" && parts[1] == "packs" && method == "POST":
		call.name, call.success = "pack_import", 201
	case len(parts) == 3 && parts[0] == "studio" && parts[1] == "packs" && parts[2] == "validate" && method == "POST":
		call.name = "pack_validate"
	case len(parts) == 3 && parts[0] == "studio" && parts[1] == "packs" && parts[2] == "export" && method == "GET":
		call.name, call.read, call.needsKey = "pack_export", true, false
	case len(parts) == 2 && parts[0] == "ops" && parts[1] == "backups" && method == "GET":
		call.name, call.read, call.needsKey = "backups", true, false
	case len(parts) == 2 && parts[0] == "ops" && parts[1] == "backups" && method == "POST":
		call.name = "backup_create"
	case len(parts) == 4 && parts[0] == "ops" && parts[1] == "backups" && parts[3] == "restore" && method == "POST":
		call.name, call.a = "backup_restore", parts[2]
	case len(parts) == 2 && parts[0] == "diagnostics" && parts[1] == "harm-intercepts" && method == "GET":
		call.name, call.read, call.needsKey = "harm", true, false
	case len(parts) == 3 && parts[0] == "diagnostics" && parts[1] == "tasks" && method == "GET":
		call.name, call.a, call.read, call.needsKey = "task", parts[2], true, false
	case len(parts) == 2 && parts[0] == "settings" && parts[1] == "agents" && method == "GET":
		call.name, call.read, call.needsKey = "agents", true, false
	case len(parts) == 3 && parts[0] == "memories" && parts[2] == "suggestions" && method == "POST":
		call.name, call.a, call.success = "memory_suggestion", parts[1], 202
	case len(parts) == 2 && parts[0] == "owner-requests" && method == "GET":
		call.name, call.a, call.read, call.needsKey = "owner_request", parts[1], true, false
	case len(parts) == 1 && parts[0] == "moments" && method == "POST":
		call.name, call.success = "moment", 201
	case len(parts) == 1 && parts[0] == "media" && method == "POST":
		call.name = "media_upload"
	case len(parts) == 2 && parts[0] == "actor-groups" && method == "DELETE":
		call.name, call.a, call.success = "group_delete", parts[1], 204
	default:
		return surfaceCall{}, false
	}
	return call, true
}

func decodeCursor(cursor string) (time.Time, string, bool) {
	if cursor == "" {
		return time.Time{}, "", true
	}
	if len(cursor) > 128 {
		return time.Time{}, "", false
	}
	nano, id, ok := strings.Cut(cursor, "|")
	if !ok || id == "" || strings.Contains(id, "|") {
		return time.Time{}, "", false
	}
	parsed, err := strconv.ParseInt(nano, 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	return time.Unix(0, parsed).UTC(), id, true
}

func encodeCursor(at time.Time, id string) string {
	return strconv.FormatInt(at.UTC().UnixNano(), 10) + "|" + id
}

type memorySurface struct {
	mu       sync.Mutex
	records  map[string]surfaceRecord
	keys     map[string]string
	idem     map[string]memoryIdem
	revision map[string]int
}

type memoryIdem struct {
	digest string
	body   map[string]any
}

func newMemorySurface() *memorySurface {
	return &memorySurface{records: map[string]surfaceRecord{}, keys: map[string]string{}, idem: map[string]memoryIdem{}, revision: map[string]int{}}
}

func (m *memorySurface) Transact(ctx context.Context, fn func(context.Context, surfaceTx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn(ctx, memoryTx{m})
}

func (m *memorySurface) ListKind(ctx context.Context, owner, kind, fid string) ([]surfaceRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	records, _, err := memoryTx{m}.List(ctx, owner, kind, fid, "", "", 100)
	return records, err
}

func (m *memorySurface) Revision(ctx context.Context, owner, fid, scope string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.revision[memoryKey(owner, fid, "scope", scope)], nil
}

type memoryTx struct{ store *memorySurface }

func memoryKey(owner, fid, kind, resourceKey string) string {
	return owner + "\x00" + fid + "\x00" + kind + "\x00" + resourceKey
}

func (t memoryTx) Idempotency(ctx context.Context, owner, key string) (string, map[string]any, bool, error) {
	row, ok := t.store.idem[owner+"\x00"+key]
	if !ok {
		return "", nil, false, nil
	}
	return row.digest, cloneMap(row.body), true, nil
}

func (t memoryTx) SaveIdempotency(ctx context.Context, owner, key, digest string, body map[string]any) error {
	t.store.idem[owner+"\x00"+key] = memoryIdem{digest: digest, body: cloneMap(body)}
	return nil
}

func (t memoryTx) Revision(ctx context.Context, owner, fid, scope string) (int, error) {
	return t.store.revision[memoryKey(owner, fid, "scope", scope)], nil
}

func (t memoryTx) Bump(ctx context.Context, owner, fid, scope string, expected int) (int, error) {
	key := memoryKey(owner, fid, "scope", scope)
	current := t.store.revision[key]
	if current != expected {
		return 0, revisionConflictError{Current: current}
	}
	t.store.revision[key] = current + 1
	return current + 1, nil
}

func (t memoryTx) Save(ctx context.Context, rec surfaceRecord) error {
	if rec.ResourceKey == "" {
		rec.ResourceKey = rec.ID
	}
	if rec.Payload == nil {
		rec.Payload = map[string]any{}
	}
	key := memoryKey(rec.OwnerID, rec.FluctlightID, rec.Kind, rec.ResourceKey)
	if existingID, ok := t.store.keys[key]; ok && existingID != rec.ID {
		delete(t.store.records, existingID)
	}
	t.store.records[rec.ID] = rec
	t.store.keys[key] = rec.ID
	return nil
}

func (t memoryTx) DeleteKey(ctx context.Context, owner, fid, kind, resourceKey string) error {
	key := memoryKey(owner, fid, kind, resourceKey)
	if id, ok := t.store.keys[key]; ok {
		delete(t.store.records, id)
		delete(t.store.keys, key)
	}
	return nil
}

func (t memoryTx) Get(ctx context.Context, owner, id string) (surfaceRecord, bool, error) {
	rec, ok := t.store.records[id]
	if !ok || rec.OwnerID != owner {
		return surfaceRecord{}, false, nil
	}
	return rec, true, nil
}

func (t memoryTx) GetKey(ctx context.Context, owner, fid, kind, resourceKey string) (surfaceRecord, bool, error) {
	id, ok := t.store.keys[memoryKey(owner, fid, kind, resourceKey)]
	if !ok {
		return surfaceRecord{}, false, nil
	}
	return t.Get(ctx, owner, id)
}

func (t memoryTx) List(ctx context.Context, owner, kind, fid, status, cursor string, limit int) ([]surfaceRecord, string, error) {
	cursorAt, cursorID, ok := decodeCursor(cursor)
	if !ok {
		return nil, "", &SurfaceError{Status: 400, Code: "surface_cursor_invalid", Message: "Cursor is invalid"}
	}
	rows := make([]surfaceRecord, 0)
	for _, rec := range t.store.records {
		if rec.OwnerID != owner || rec.Kind != kind {
			continue
		}
		if fid != "" && rec.FluctlightID != fid {
			continue
		}
		if status != "" && rec.Status != status {
			continue
		}
		if cursor != "" && !beforeCursor(rec, cursorAt, cursorID) {
			continue
		}
		rows = append(rows, rec)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].CreatedAt.Equal(rows[j].CreatedAt) {
			return rows[i].CreatedAt.After(rows[j].CreatedAt)
		}
		return rows[i].ID > rows[j].ID
	})
	if limit < 1 {
		limit = 50
	}
	next := ""
	if len(rows) > limit {
		last := rows[limit-1]
		next = encodeCursor(last.CreatedAt, last.ID)
		rows = rows[:limit]
	}
	return rows, next, nil
}

func beforeCursor(rec surfaceRecord, at time.Time, id string) bool {
	if rec.CreatedAt.Before(at) {
		return true
	}
	return rec.CreatedAt.Equal(at) && rec.ID < id
}

func (t memoryTx) Audit(ctx context.Context, owner, fid, action, reason, recordID string) (string, error) {
	id := randomID("audit_")
	now := time.Now().UTC()
	identity, stored := splitAuditAction(action)
	rec := surfaceRecord{ID: id, OwnerID: owner, FluctlightID: fid, Kind: "audit", ResourceKey: id, Status: "recorded", CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"identity": identity, "actor": "owner", "action": stored, "reason": reason, "record_id": recordID}}
	return id, t.Save(ctx, rec)
}

func splitAuditAction(action string) (identity, stored string) {
	if rest, ok := strings.CutPrefix(action, "request:"); ok {
		return "owner_request", rest
	}
	return "owner_edit", action
}
