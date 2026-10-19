package core

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

func matchMomentFeed(method, path string) bool {
	p := surfaceParts(path)
	if len(p) == 1 && p[0] == "moment-suggestions" {
		return method == "GET"
	}
	if len(p) == 1 && p[0] == "moments" && method == "POST" {
		return true
	}
	if len(p) == 0 || p[0] != "moment-feed" {
		return false
	}
	return method == "GET" && (len(p) == 1 || len(p) == 2 || len(p) == 3 && containsString([]string{"comments", "npc-interactions"}, p[2]) || len(p) == 4 && p[2] == "npc-interactions") || method == "POST" && (len(p) == 3 && containsString([]string{"comments", "reaction", "hide", "restore"}, p[2]) || len(p) == 5 && p[2] == "npc-interactions" && p[4] == "retry")
}

const momentFeedProjection = `jsonb_build_object('id',m.id,'owner_fluctlight_id',m.owner_fluctlight_id,'author_actor_id',m.author_actor_id,'text',m.text,'status',m.status,'visibility',m.visibility,'revision',m.revision,'media_asset_ids',m.media_asset_ids,'created_at',m.created_at,'request_id',m.owner_request_id,'source_event_id',m.source_event_id,'reaction_count',(SELECT count(*) FROM public.moment_reactions r WHERE r.moment_id=m.id),'viewer_reaction',(SELECT kind FROM public.moment_reactions r WHERE r.moment_id=m.id AND r.actor_id=$1),'comment_count',(SELECT count(*) FROM public.moment_comments c WHERE c.moment_id=m.id))`

func readMomentWith(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	if lock {
		var locked string
		err := q.QueryRow(ctx, `SELECT id FROM public.moments WHERE id=$2 AND owner_actor_id=$1 FOR UPDATE`, owner, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+momentFeedProjection+` FROM public.moments m WHERE m.owner_actor_id=$1 AND m.id=$2`, owner, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}

func (a *App) decorateFeedMoment(ctx context.Context, owner string, row map[string]any) error {
	author, fid := stringValue(row["author_actor_id"]), stringValue(row["owner_fluctlight_id"])
	kind, name := "unknown", ""
	switch {
	case author == owner:
		kind, name = "user", "我"
	case author == fid:
		kind = "fluctlight"
		if err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(core_persona->'identity'->>'name',id) FROM public.fluctlights WHERE id=$1`, fid).Scan(&name); err != nil {
			return err
		}
	default:
		err := a.DB.Pool().QueryRow(ctx, `SELECT n.name FROM public.world_npcs n JOIN public.life_worlds w ON w.id=n.world_id WHERE n.actor_id=$1 AND w.fluctlight_id=$2`, author, fid).Scan(&name)
		if err == nil {
			kind = "npc"
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	row["author_type"], row["author_name"] = kind, nilIfEmpty(name)
	assets := []any{}
	for _, v := range arrayValue(row["media_asset_ids"]) {
		asset, err := a.ownerMediaAssetWith(ctx, a.DB.Pool(), owner, stringValue(v), false)
		if errors.Is(err, ErrNotFound) {
			assets = append(assets, map[string]any{"id": v, "read_state": "unavailable", "readable": false})
			continue
		}
		if err != nil {
			return err
		}
		assets = append(assets, asset)
	}
	row["media"] = assets
	return nil
}

func (a *App) dispatchMomentFeed(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	p := surfaceParts(path)
	if p[0] == "moment-suggestions" {
		limit, err := reflectionReadPage(q)
		if err != nil {
			return nil, err
		}
		fid := q.Get("fluctlightId")
		if fid != "" {
			if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
				return nil, err
			}
		}
		scope := stableDigest("moment-suggestions:" + owner + ":" + fid)
		cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
		if err != nil {
			return nil, err
		}
		rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND kind='owner_request' AND payload->>'kind'='moment_suggestion' AND ($2='' OR fluctlight_id=$2) AND ($3='' OR (created_at,id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return readGovernanceRows(rows, scope, limit)
	}
	if p[0] == "moments" {
		row, err := a.publishOwnerFeedMoment(ctx, owner, body)
		if err != nil {
			return nil, err
		}
		return row, a.decorateFeedMoment(ctx, owner, row)
	}
	if method == "GET" && len(p) == 1 {
		return a.ownerMomentFeed(ctx, owner, q)
	}
	if method == "GET" && len(p) == 2 {
		row, err := readMomentWith(ctx, a.DB.Pool(), owner, p[1], false)
		if err != nil {
			return nil, err
		}
		return row, a.decorateFeedMoment(ctx, owner, row)
	}
	if len(p) >= 3 && p[2] == "npc-interactions" {
		if method == "GET" {
			if len(p) == 4 {
				row, err := readNPCMomentWith(ctx, a.DB.Pool(), owner, p[3], false)
				if err != nil {
					return nil, err
				}
				if row["moment_id"] != p[1] {
					return nil, ErrNotFound
				}
				return npcMomentPublic(row), nil
			}
			return a.ownerNPCMomentRecords(ctx, owner, p[1], q)
		}
		return a.retryNPCMoment(ctx, owner, p[1], p[3], body)
	}
	if method == "GET" {
		return a.ownerMomentComments(ctx, owner, p[1], q)
	}
	row, err := a.ownerMomentInteraction(ctx, owner, p[1], p[2], body)
	if err != nil {
		return nil, err
	}
	return row, a.decorateFeedMoment(ctx, owner, row)
}

func (a *App) ownerMomentFeed(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	fid, status, search := q.Get("fluctlightId"), q.Get("status"), strings.TrimSpace(q.Get("q"))
	if !containsString([]string{"", "visible", "hidden", "all"}, status) || len([]rune(search)) > 200 {
		return nil, invalid("Invalid feed filter")
	}
	if fid != "" {
		if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
			return nil, err
		}
	}
	if status == "" {
		status = "visible"
	}
	scope := stableDigest(jsonString([]any{"moment-feed", owner, fid, status, search}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.id,m.created_at,`+momentFeedProjection+` FROM public.moments m WHERE m.owner_actor_id=$1 AND ($2='' OR m.owner_fluctlight_id=$2) AND ($3='all' OR m.status=$3) AND ($4='' OR strpos(lower(m.text),lower($4))>0) AND ($5='' OR (m.created_at,m.id)<($6,$5)) ORDER BY m.created_at DESC,m.id DESC LIMIT $7`, owner, fid, status, search, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	page, err := readGovernanceRows(rows, scope, limit)
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, v := range arrayValue(page["items"]) {
		if err := a.decorateFeedMoment(ctx, owner, mapValue(v)); err != nil {
			return nil, err
		}
	}
	return page, nil
}

func (a *App) ownerMomentComments(ctx context.Context, owner, id string, q url.Values) (map[string]any, error) {
	if _, err := readMomentWith(ctx, a.DB.Pool(), owner, id, false); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest("moment-comments:" + owner + ":" + id)
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(c) FROM public.moment_comments c WHERE moment_id=$1 AND ($2='' OR (created_at,id)<($3,$2)) ORDER BY created_at DESC,id DESC LIMIT $4`, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	page, err := readGovernanceRows(rows, scope, limit)
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, v := range arrayValue(page["items"]) {
		row := mapValue(v)
		identity := map[string]any{"author_actor_id": row["author_actor_id"], "owner_fluctlight_id": nil, "media_asset_ids": []any{}}
		var fid *string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT owner_fluctlight_id FROM public.moments WHERE id=$1`, id).Scan(&fid); err != nil {
			return nil, err
		}
		identity["owner_fluctlight_id"] = fid
		if fid != nil {
			identity["owner_fluctlight_id"] = *fid
		}
		if err := a.decorateFeedMoment(ctx, owner, identity); err != nil {
			return nil, err
		}
		row["author_type"], row["author_name"] = identity["author_type"], identity["author_name"]
	}
	return page, nil
}

// Media stays in its original producer scope. Explicit publication creates a
// durable reference, and never certifies that an uploaded picture is a fact.
func (a *App) linkMomentMediaTx(ctx context.Context, tx pgx.Tx, owner, fid, id string, ids []any) error {
	ids = append([]any{}, ids...)
	sort.Slice(ids, func(i, j int) bool { return stringValue(ids[i]) < stringValue(ids[j]) })
	seen := map[string]bool{}
	for _, rawID := range ids {
		assetID, ok := rawID.(string)
		if !ok || strings.TrimSpace(assetID) == "" || len(assetID) > 128 {
			return invalid("Invalid media asset ID")
		}
		if seen[assetID] {
			return invalid("Duplicate media asset")
		}
		seen[assetID] = true
		asset, err := a.ownerMediaAssetWith(ctx, tx, owner, assetID, true)
		if err != nil {
			return err
		}
		if asset["readable"] != true {
			return &SurfaceError{Status: 409, Code: "moment_media_unavailable", Message: "发布素材当前不可读取，请核对原任务和权限"}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES($1,$2,$3,'moment',$4) ON CONFLICT(id) DO NOTHING`, "moment_ref_"+stableDigest(id+":"+assetID), assetID, fid, id); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) publishOwnerFeedMoment(ctx context.Context, owner string, body map[string]any) (map[string]any, error) {
	if momentAuthorForbidden(body) {
		return nil, &SurfaceError{Status: 422, Code: "moment_author_forbidden", Message: "不能以角色或 NPC 身份代发动态"}
	}
	if !eventKeys(body, "text", "media_ids", "world_id", "refs", "idempotency_key") {
		return nil, invalid("Unknown publication field")
	}
	text := strings.TrimSpace(stringValue(body["text"]))
	if text == "" || len([]rune(text)) > 2000 {
		return nil, invalid("Text must contain 1–2000 characters")
	}
	if value, exists := body["media_ids"]; exists {
		if _, ok := value.([]any); !ok {
			return nil, invalid("mediaIds must be an array")
		}
	}
	media := stringList(body["media_ids"])
	if len(media) > 8 {
		return nil, invalid("At most 8 media assets per Moment")
	}
	command := cloneMap(body)
	command["reason"] = "用户以自己的交流身份发布动态"
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "moment_publish")
	return a.worldCommand(ctx, owner, stringValue(body["world_id"]), "owner-moment-publish", command, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		fid, id := stringValue(body["world_id"]), randomID("moment_")
		if err := a.linkMomentMediaTx(ctx, tx, owner, fid, id, media); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,visibility,media_asset_ids) VALUES($1,$2,$3,$2,$4,'owner',$5)`, id, owner, nilIfEmpty(fid), text, jsonBytes(media)); err != nil {
			return nil, err
		}
		if err := a.enqueueNPCMomentInteractionsTx(ctx, tx, id); err != nil {
			return nil, err
		}
		return readMomentWith(ctx, tx, owner, id, false)
	})
}

func (a *App) ownerMomentInteraction(ctx context.Context, owner, id, op string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "text", "kind", "expected_revision", "reason", "idempotency_key") {
		return nil, invalid("Unknown Moment command field")
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || expected < 1 {
		return nil, invalid("Moment revision required")
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "moment_"+op)
	return a.worldCommand(ctx, owner, "", "moment:"+id+":"+op, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		row, err := readMomentWith(ctx, tx, owner, id, true)
		if err != nil {
			return nil, err
		}
		if intValue(row["revision"]) != expected {
			return nil, conflict("moment", expected, intValue(row["revision"]))
		}
		if (op == "comments" || op == "reaction") && row["status"] != "visible" {
			return nil, &SurfaceError{Status: 409, Code: "moment_hidden", Message: "这条动态已隐藏，请先恢复后互动"}
		}
		switch op {
		case "comments":
			text := strings.TrimSpace(stringValue(body["text"]))
			if text == "" || len([]rune(text)) > 2000 {
				return nil, invalid("Comment must contain 1–2000 characters")
			}
			commentID := randomID("comment_")
			if _, err := tx.Exec(ctx, `INSERT INTO public.moment_comments(id,moment_id,author_actor_id,text) VALUES($1,$2,$3,$4)`, commentID, id, owner, text); err != nil {
				return nil, err
			}
			if fid := stringValue(row["owner_fluctlight_id"]); fid != "" {
				if err := appendOutboxTx(ctx, tx, "moment.comment.created", "moment_comment", commentID, fid, id, "moment-comment:"+commentID, "moment-comment:"+commentID, map[string]any{"moment_id": id, "author_actor_id": owner}); err != nil {
					return nil, err
				}
			}
		case "reaction":
			kind := stringValue(body["kind"])
			if !containsString([]string{"like", "care", "celebrate", "none"}, kind) {
				return nil, invalid("Invalid reaction")
			}
			if kind == "none" {
				_, err = tx.Exec(ctx, `DELETE FROM public.moment_reactions WHERE moment_id=$1 AND actor_id=$2`, id, owner)
			} else {
				_, err = tx.Exec(ctx, `INSERT INTO public.moment_reactions(moment_id,actor_id,kind) VALUES($1,$2,$3) ON CONFLICT(moment_id,actor_id) DO UPDATE SET kind=excluded.kind,created_at=now()`, id, owner, kind)
			}
			if err != nil {
				return nil, err
			}
		case "hide", "restore":
			state := "hidden"
			if op == "restore" {
				state = "visible"
			}
			if _, err := tx.Exec(ctx, `UPDATE public.moments SET status=$2 WHERE id=$1`, id, state); err != nil {
				return nil, err
			}
		default:
			return nil, ErrInvalidArguments
		}
		if _, err := tx.Exec(ctx, `UPDATE public.moments SET revision=revision+1 WHERE id=$1`, id); err != nil {
			return nil, err
		}
		return readMomentWith(ctx, tx, owner, id, false)
	})
}

// Both legacy text and fact-only independent Writer publication use the same
// trusted request fence. Neither model arguments nor browser author IDs grant
// authority. One request may publish at most one real Moment.
func (a *App) publishInvocationMomentTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget, text string) (publishedResource, error) {
	if target.Kind != "owner_moment_request" {
		return NewToolPublicationService(a).PublishMomentTx(ctx, tx, MomentPublication{FluctlightID: target.FluctlightID, OperationID: capabilityOperationID(inv), CorrelationID: inv.Metadata.CorrelationID, Text: text})
	}
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' FOR UPDATE`, target.Ref, target.AuthorizationActorID, target.FluctlightID).Scan(&raw)
	if err != nil {
		return publishedResource{}, err
	}
	request := decodeObject(raw)
	if request["kind"] != "moment_suggestion" || request["source_fact_id"] != inv.SourceFactID {
		return publishedResource{}, ErrUnauthorized
	}
	var existing, committedText string
	err = tx.QueryRow(ctx, `SELECT id,text FROM public.moments WHERE owner_request_id=$1 AND owner_actor_id=$2 AND owner_fluctlight_id=$3`, target.Ref, target.AuthorizationActorID, target.FluctlightID).Scan(&existing, &committedText)
	if err == nil {
		return publishedResource{ID: existing, Replayed: true, Text: committedText}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return publishedResource{}, err
	}
	if request["status"] != "pending" {
		return publishedResource{}, ErrConflict
	}
	if err := requireOwnerMutableTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID); err != nil {
		return publishedResource{}, err
	}
	resource, err := NewToolPublicationService(a).PublishMomentTx(ctx, tx, MomentPublication{FluctlightID: target.FluctlightID, OperationID: "owner-moment:" + target.Ref, CorrelationID: inv.Metadata.CorrelationID, Text: text})
	if err != nil {
		return resource, err
	}
	media := stringList(mapValue(request["payload"])["media_ids"])
	if err := a.linkMomentMediaTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, resource.ID, media); err != nil {
		return resource, err
	}
	_, err = tx.Exec(ctx, `UPDATE public.moments SET owner_request_id=$2,media_asset_ids=$3 WHERE id=$1`, resource.ID, target.Ref, jsonBytes(media))
	return resource, err
}

func (a *App) ownerMomentWriterInput(ctx context.Context, scope ToolExecutionRequest) (string, error) {
	if scope.TargetKind != "owner_moment_request" {
		return "", nil
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' AND status='pending' AND payload->>'kind'='moment_suggestion' AND payload->>'source_fact_id'=$4`, scope.TargetRef, scope.AuthorizationActorID, scope.FluctlightID, scope.EvidenceID).Scan(&raw)
	if err != nil {
		return "", err
	}
	return "用户建议以下动态议题。角色已选择发布，正文由你独立决定；建议不是生活事实、对话原文或必须照抄的草稿。附件只是用户选定的素材，不证明画面事件发生；未经视觉观察不可声称看见内容。\n" + jsonString(mapValue(decodeObject(raw)["payload"])), nil
}
