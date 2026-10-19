package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const resourcePackSchema = "fluctlight-resource-pack.v1"
const resourcePackMaxBytes = 3 << 20

func packSHA(v any) string { sum := sha256.Sum256(jsonBytes(v)); return hex.EncodeToString(sum[:]) }
func packError(path, message string) error {
	return &SurfaceError{Status: 422, Code: "resource_pack_invalid", Message: message, Details: map[string]any{"path": path}}
}
func packSensitive(value any, path string) error {
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			normalized := strings.ToLower(strings.ReplaceAll(k, "_", ""))
			switch normalized {
			case "password", "apikey", "secret", "secrets", "credential", "credentials", "ciphertext", "nonce", "accesstoken", "refreshtoken", "authorization", "cookie", "settingskey", "servicekey", "assetid", "grantid", "mediascope", "owneractorid", "fluctlightid", "actoruserid", "actorselfid", "conversationid", "memories", "conversations", "innerstate", "lifefacts", "schedules", "moments":
				return packError(path+"/"+k, "公开定义包不能包含凭据、媒体授权或运行事实字段")
			}
			if err := packSensitive(child, path+"/"+k); err != nil {
				return err
			}
		}
	case []any:
		for i, child := range v {
			if err := packSensitive(child, fmt.Sprintf("%s/%d", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}
func validatePackDefinition(kind string, body map[string]any) error {
	if containsPermanentHarm(body) {
		return harmError()
	}
	switch kind {
	case "item":
		return validateItemDefinition(body)
	case "outfit":
		return validateOutfitDefinition(body)
	case "visual_style":
		return validateVisualStyle(body)
	case "character_template":
		_, err := characterTemplateSource(body)
		return err
	case "persona_fragment":
		_, err := foundationDefinitionChanges(worldPublication{Kind: kind, Body: body})
		return err
	case "event_template":
		return validateWorldEventDefinition(body)
	case "world", "place", "npc":
		allowed := []string{"background", "rules", "description", "flavor", "item_pool", "role", "alias", "group", "floor", "scene", "atmosphere", "furniture", "layout", "connections", "connection_place_ids", "room_items"}
		if !eventKeys(body, allowed...) {
			return invalid("世界/地点/NPC定义包含不支持的字段")
		}
		for _, key := range []string{"background", "rules", "description", "role"} {
			limit := 16000
			if key == "role" {
				limit = 512
			}
			if _, err := boundedWorldText(body, key, limit, false); err != nil {
				return err
			}
		}
		return nil
	default:
		return invalid("配置包资源类型不受支持")
	}
}
func packEntryKey(id, source, version string) string { return id + "@" + source + ":" + version }
func packExternalBindings(value any, path string) []any {
	result := []any{}
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := v[k]
			p := path + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
			switch k {
			case "place_id", "npc_id", "home_place_id", "parent_place_id":
				if id := stringValue(child); id != "" {
					result = append(result, map[string]any{"path": p, "kind": k, "source_id": id, "state": "requires_runtime_rebinding"})
				}
			case "place_ids", "connection_place_ids":
				for j, raw := range arrayValue(child) {
					if id := stringValue(raw); id != "" {
						result = append(result, map[string]any{"path": fmt.Sprintf("%s/%d", p, j), "kind": k, "source_id": id, "state": "requires_runtime_rebinding"})
					}
				}
			default:
				result = append(result, packExternalBindings(child, p)...)
			}
		}
	case []any:
		for i, raw := range v {
			result = append(result, packExternalBindings(raw, fmt.Sprintf("%s/%d", path, i))...)
		}
	}
	return result
}

type checkedResourcePack struct {
	Entries  []map[string]any
	Order    []string
	External []any
	Digest   string
}

func checkResourcePack(pack map[string]any) (checkedResourcePack, error) {
	checked := checkedResourcePack{Entries: []map[string]any{}, Order: []string{}, External: []any{}}
	if pack["schema"] != resourcePackSchema || !eventKeys(pack, "schema", "created_at", "resources", "groups", "preferences", "checksum") {
		return checked, packError("/schema", "需要当前版本的摇光公开定义包；原型演示数据不属于可导入的正式包")
	}
	if len(jsonBytes(pack)) > resourcePackMaxBytes {
		return checked, packError("/", "配置包超过3MiB，请按资源分批导出")
	}
	if err := packSensitive(pack, ""); err != nil {
		return checked, err
	}
	if containsPermanentHarm(pack) {
		return checked, harmError()
	}
	if !validTimestamp(pack["created_at"]) {
		return checked, packError("/created_at", "配置包缺少有效导出时间")
	}
	copy := cloneMap(pack)
	delete(copy, "checksum")
	checked.Digest = packSHA(copy)
	if pack["checksum"] != checked.Digest {
		return checked, packError("/checksum", "配置包摘要不匹配；修改内容后请重新导出，不接收被截断或损坏的包")
	}
	entries, ok := pack["resources"].([]any)
	if !ok || len(entries) > 2000 {
		return checked, packError("/resources", "资源列表格式错误或超过2000项")
	}
	byKey := map[string]map[string]any{}
	published := map[string]string{}
	for index, raw := range entries {
		entry, ok := raw.(map[string]any)
		path := fmt.Sprintf("/resources/%d", index)
		if !ok || !eventKeys(entry, "key", "source_resource_id", "source", "source_version", "source_revision", "kind", "name", "body", "checksum") {
			return checked, packError(path, "资源项结构错误")
		}
		key, id, source, version, kind := stringValue(entry["key"]), stringValue(entry["source_resource_id"]), stringValue(entry["source"]), stringValue(entry["source_version"]), stringValue(entry["kind"])
		if len(id) == 0 || len(id) > 128 || len(version) == 0 || len(version) > 64 || !containsString([]string{"draft", "published"}, source) || key != packEntryKey(id, source, version) || byKey[key] != nil {
			return checked, packError(path, "资源身份或版本不合法、重复")
		}
		if _, ok := positiveInt64(entry["source_revision"]); !ok {
			return checked, packError(path, "资源来源版本无效")
		}
		name := stringValue(entry["name"])
		body, bodyOK := entry["body"].(map[string]any)
		if strings.TrimSpace(name) == "" || len([]rune(name)) > 200 || !bodyOK || len(jsonBytes(body)) > 64000 {
			return checked, packError(path, "资源名称或定义内容无效")
		}
		if err := validatePackDefinition(kind, body); err != nil {
			return checked, packError(path+"/body", err.Error())
		}
		unsigned := cloneMap(entry)
		delete(unsigned, "checksum")
		if entry["checksum"] != packSHA(unsigned) {
			return checked, packError(path+"/checksum", "资源内容摘要不匹配")
		}
		byKey[key] = entry
		checked.Entries = append(checked.Entries, entry)
		if source == "published" {
			published[id+"\x1f"+version] = key
		}
		for _, raw := range packExternalBindings(body, "/body") {
			binding := mapValue(raw)
			binding["entry_key"] = key
			checked.External = append(checked.External, binding)
		}
	}
	seen, visiting := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(key string) error {
		if seen[key] {
			return nil
		}
		if visiting[key] {
			return packError("/resources", "资源引用形成循环")
		}
		visiting[key] = true
		entry := byKey[key]
		if entry["kind"] == "outfit" {
			slots := map[string]bool{}
			for _, raw := range arrayValue(mapValue(entry["body"])["members"]) {
				m := mapValue(raw)
				dependency := published[stringValue(m["resource_id"])+"\x1f"+stringValue(m["resource_version"])]
				target := byKey[dependency]
				if target == nil || target["kind"] != "item" {
					return packError("/resources/"+key+"/body/members", "套装引用的精确已发布物品定义不在包内")
				}
				b := mapValue(target["body"])
				slot := stringValue(b["slot"])
				if b["item_kind"] != "wearable" || slots[slot] {
					return packError("/resources/"+key+"/body/members", "套装成员须为不同槽位的服饰定义")
				}
				slots[slot] = true
				if err := visit(dependency); err != nil {
					return err
				}
			}
		}
		visiting[key] = false
		seen[key] = true
		checked.Order = append(checked.Order, key)
		return nil
	}
	for _, entry := range checked.Entries {
		if err := visit(stringValue(entry["key"])); err != nil {
			return checked, err
		}
	}
	groups, ok := pack["groups"].([]any)
	if !ok || len(groups) > 200 {
		return checked, packError("/groups", "分组建议列表不合法")
	}
	for _, raw := range groups {
		group, ok := raw.(map[string]any)
		if !ok || !eventKeys(group, "name") || strings.TrimSpace(stringValue(group["name"])) == "" || len([]rune(stringValue(group["name"]))) > 128 {
			return checked, packError("/groups", "分组只能包含名称建议；运行角色归属不随公开包迁移")
		}
	}
	preferences, ok := pack["preferences"].(map[string]any)
	if !ok || !eventKeys(preferences, "product.notifications") {
		return checked, packError("/preferences", "公开包只包含受支持的产品偏好草稿")
	}
	if value, present := preferences["product.notifications"]; present {
		if err := validNotificationPreferences(value); err != nil {
			return checked, err
		}
	}
	return checked, nil
}

func (a *App) ExportOwnerResourcePack(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	rows, err := tx.Query(ctx, `SELECT id,revision,payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND kind='studio_resource' AND status<>'archived' AND ($2::text='' OR id=$2) ORDER BY created_at,id`, owner, q.Get("resourceId"))
	if err != nil {
		return nil, err
	}
	entries := []any{}
	seen := map[string]bool{}
	add := func(id, source, version, kind, name string, revision int, body map[string]any) {
		key := packEntryKey(id, source, version)
		if seen[key] {
			return
		}
		seen[key] = true
		entry := map[string]any{"key": key, "source_resource_id": id, "source": source, "source_version": version, "source_revision": revision, "kind": kind, "name": name, "body": body}
		entry["checksum"] = packSHA(entry)
		entries = append(entries, entry)
	}
	for rows.Next() {
		var id string
		var revision int
		var raw []byte
		if err := rows.Scan(&id, &revision, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		payload := decodeObject(raw)
		add(id, "draft", stringValue(payload["draft_version"]), stringValue(payload["kind"]), stringValue(payload["name"]), revision, mapValue(payload["body"]))
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	if q.Get("resourceId") != "" && len(entries) == 0 {
		return nil, ErrNotFound
	}
	for index := 0; index < len(entries); index++ {
		entry := mapValue(entries[index])
		if entry["kind"] != "outfit" {
			continue
		}
		for _, raw := range arrayValue(mapValue(entry["body"])["members"]) {
			member := mapValue(raw)
			p, err := readWorldPublication(ctx, tx, owner, stringValue(member["resource_id"]), stringValue(member["resource_version"]))
			if err != nil {
				return nil, err
			}
			var revision int
			if err := tx.QueryRow(ctx, `SELECT revision FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2`, p.ID, owner).Scan(&revision); err != nil {
				return nil, err
			}
			add(p.ID, "published", p.Version, p.Kind, p.Name, revision, p.Body)
		}
	}
	groups, err := readWorldObjects(ctx, tx, `SELECT jsonb_build_object('name',name) FROM public.actor_groups WHERE owner_actor_id=$1 ORDER BY created_at,id`, owner)
	if err != nil {
		return nil, err
	}
	preferences := map[string]any{}
	var notifications string
	err = tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.notifications'`).Scan(&notifications)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		preferences["product.notifications"] = decodeObject([]byte(notifications))
	}
	pack := map[string]any{"schema": resourcePackSchema, "created_at": a.now().UTC().Format(time.RFC3339Nano), "resources": entries, "groups": groups, "preferences": preferences}
	pack["checksum"] = packSHA(pack)
	checked, err := checkResourcePack(pack)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	if err := a.auditWorkflow(ctx, owner, "resource_pack_export", "", true, map[string]any{"pack_digest": checked.Digest, "resource_count": len(entries), "group_drafts": len(groups)}); err != nil {
		return nil, err
	}
	return map[string]any{"pack": pack, "available": true, "complete": true, "validation": resourcePackSummary(checked), "scope": "公开创作定义、分组名称和已保存通知偏好；不含成员归属、Provider配置/凭据、媒体字节/授权或生活历史"}, nil
}
func resourcePackSummary(checked checkedResourcePack) map[string]any {
	entries := []any{}
	for _, e := range checked.Entries {
		entries = append(entries, map[string]any{"key": e["key"], "kind": e["kind"], "name": e["name"], "source_version": e["source_version"], "source": e["source"]})
	}
	return map[string]any{"valid": true, "schema": resourcePackSchema, "digest": checked.Digest, "entries": entries, "publish_order": checked.Order, "external_bindings": checked.External, "semantics": "导入创建独立草稿；依赖须按顺序人工发布，运行地点/NPC引用须核对目标世界；不会自动发布、启用事件或应用角色。分组和偏好保留为建议草稿。"}
}
func (a *App) ValidateOwnerResourcePack(ctx context.Context, owner string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "pack", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	checked, err := checkResourcePack(mapValue(body["pack"]))
	if err != nil {
		return nil, err
	}
	return resourcePackSummary(checked), nil
}
func (a *App) ImportOwnerResourcePack(ctx context.Context, owner string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "pack", "reason", "idempotency_key", "expected_digest") {
		return nil, ErrInvalidArguments
	}
	checked, err := checkResourcePack(mapValue(body["pack"]))
	if err != nil {
		return nil, err
	}
	if body["expected_digest"] != checked.Digest {
		return nil, &SurfaceError{Status: 409, Code: "revision_conflict", Message: "待导入内容与已核对的包不同，请重新校验"}
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_pack_import")
	return a.worldCommand(ctx, owner, "", "resource-pack-import", body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if err := lockStudioReferenceGraphTx(ctx, tx, owner); err != nil {
			return nil, err
		}
		ids := map[string]string{}
		published := map[string]string{}
		for _, e := range checked.Entries {
			key := stringValue(e["key"])
			ids[key] = randomID("res_")
			if e["source"] == "published" {
				published[stringValue(e["source_resource_id"])+"\x1f"+stringValue(e["source_version"])] = key
			}
		}
		now := a.now()
		packID := randomID("pack_")
		mapping := []any{}
		for _, e := range checked.Entries {
			key := stringValue(e["key"])
			definition := cloneMap(mapValue(e["body"]))
			if e["kind"] == "outfit" {
				members := []any{}
				for _, raw := range arrayValue(definition["members"]) {
					m := mapValue(raw)
					target := published[stringValue(m["resource_id"])+"\x1f"+stringValue(m["resource_version"])]
					members = append(members, map[string]any{"resource_id": ids[target], "resource_version": "1"})
				}
				definition["members"] = members
			}
			provenance := map[string]any{"pack_id": packID, "pack_digest": checked.Digest, "entry_key": key, "source_resource_id": e["source_resource_id"], "source_version": e["source_version"], "source": e["source"], "definition_checksum": e["checksum"], "external_bindings": packExternalBindings(definition, "/body")}
			rec := surfaceRecord{ID: ids[key], OwnerID: owner, Kind: "studio_resource", ResourceKey: ids[key], Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"name": e["name"], "kind": e["kind"], "body": definition, "draft_version": "1", "published_version": nil, "referenced_by": []any{}, "versions": []any{map[string]any{"version": "1", "created_at": now.UTC().Format(time.RFC3339Nano)}}, "pack_source": provenance}}
			if err := (postgresSurfaceTx{tx}).Save(ctx, rec); err != nil {
				return nil, err
			}
			mapping = append(mapping, map[string]any{"entry_key": key, "resource_id": ids[key], "version": "1", "kind": e["kind"], "name": e["name"], "status": "draft"})
		}
		payload := map[string]any{"schema": resourcePackSchema, "digest": checked.Digest, "resource_versions": mapping, "publish_order": checked.Order, "groups": mapValue(body["pack"])["groups"], "preferences": mapValue(body["pack"])["preferences"], "external_bindings": checked.External, "life_write": false}
		if err := (postgresSurfaceTx{tx}).Save(ctx, surfaceRecord{ID: packID, OwnerID: owner, Kind: "studio_pack", ResourceKey: packID, Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: payload}); err != nil {
			return nil, err
		}
		return map[string]any{"id": packID, "pack_id": packID, "status": "draft", "resource_versions": mapping, "digest": checked.Digest, "life_write": false, "published": false, "validation": resourcePackSummary(checked)}, nil
	})
}
func (a *App) OwnerResourcePack(ctx context.Context, owner, id string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT payload||jsonb_build_object('id',id,'revision',revision,'created_at',created_at) FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND kind='studio_pack'`, id, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}

func (a *App) AdoptResourcePackGroup(ctx context.Context, owner, id, index string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "name", "expected_revision", "idempotency_key", "reason") {
		return nil, ErrInvalidArguments
	}
	var position int
	if _, err := fmt.Sscanf(index, "%d", &position); err != nil || fmt.Sprint(position) != index || position < 0 {
		return nil, ErrInvalidArguments
	}
	name := strings.TrimSpace(stringValue(body["name"]))
	if name == "" || len([]rune(name)) > 128 {
		return nil, invalid("分组名称须为1到128字符")
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_pack_group_adopt")
	return a.worldCommand(ctx, owner, "", "pack-group:"+id+":"+index, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if _, err := tx.Exec(ctx, `SELECT id FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND kind='studio_pack' FOR UPDATE`, id, owner); err != nil {
			return nil, err
		}
		rec, found, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !found || rec.Kind != "studio_pack" {
			return nil, ErrNotFound
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || rec.Revision != expected {
			return nil, conflict("studio_pack", expected, rec.Revision)
		}
		groups := arrayValue(rec.Payload["groups"])
		if position >= len(groups) {
			return nil, ErrNotFound
		}
		adopted := mapValue(rec.Payload["adopted_groups"])
		if previous := mapValue(adopted[index]); len(previous) > 0 {
			return nil, &SurfaceError{Status: 409, Code: "pack_group_already_adopted", Message: "该分组草稿已处理，请重新读取实际结果"}
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "pack-group-owner:"+owner); err != nil {
			return nil, err
		}
		var group string
		err = tx.QueryRow(ctx, `SELECT id FROM public.actor_groups WHERE owner_actor_id=$1 AND name=$2 ORDER BY created_at,id LIMIT 1`, owner, name).Scan(&group)
		existing := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if !existing {
			group = randomID("group_")
			if _, err := tx.Exec(ctx, `INSERT INTO public.actor_groups(id,owner_actor_id,name) VALUES($1,$2,$3)`, group, owner, name); err != nil {
				return nil, err
			}
		}
		result := map[string]any{"id": group, "name": name, "source_name": mapValue(groups[position])["name"], "reused_existing": existing, "members_moved": false}
		adopted[index] = result
		rec.Payload["adopted_groups"] = adopted
		rec.Revision++
		rec.UpdatedAt = a.now()
		if err := (postgresSurfaceTx{tx}).Save(ctx, rec); err != nil {
			return nil, err
		}
		result["pack_revision"] = rec.Revision
		return result, nil
	})
}
