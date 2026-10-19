package core

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

var studioFoundationSections = []string{"identity", "personality", "behavioral_policy", "life_profile", "personality_system"}

const foundationResourceReferencesSQL = `SELECT jsonb_build_object('fluctlight_id',r.fluctlight_id,'runtime_id',r.id,'running_version',r.source_version,'instance_revision',r.revision,'target_kind','foundation','status',r.status,'currently_effective',r.status='accepted' AND f.current_revision=r.revision) FROM public.fluctlight_foundation_revisions r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE r.source_resource_id=$1 AND f.created_by_actor_id=$2 ORDER BY r.created_at,r.id`

type studioFoundationSourceKey struct{}
type studioFoundationSource struct {
	Publication worldPublication
	Command     map[string]any
}

func foundationDefinitionChanges(p worldPublication) (map[string]any, error) {
	var changes map[string]any
	switch p.Kind {
	case "persona_fragment":
		if p.Body["schema"] != "persona-fragment.v1" || !eventKeys(p.Body, "schema", "changes") {
			return nil, invalid("Expected persona-fragment.v1 with explicit Foundation changes")
		}
		changes = mapValue(p.Body["changes"])
	case "character_template":
		if _, err := characterTemplateSource(p.Body); err != nil {
			return nil, err
		}
		if _, prose := p.Body["description"]; prose {
			return nil, &SurfaceError{Status: 409, Code: "foundation_preparation_required", Message: "先在隔离创作任务中解析文本模板，审阅结构化定义后保存发布，再读取应用差异"}
		}
		core := mapValue(mapValue(p.Body["initialization"])["core_persona"])
		changes = map[string]any{}
		for _, key := range studioFoundationSections {
			if value, present := core[key]; present {
				changes[key] = value
			}
		}
		// A template's initial selector is not authority to switch an existing
		// runtime. Profile definitions and their controlled rules still apply.
		if system, ok := changes["personality_system"].(map[string]any); ok {
			changes["personality_system"] = cloneMap(system)
			delete(mapValue(changes["personality_system"]), "active_profile_id")
		}
	default:
		return nil, invalid("Resource has no Foundation consumer")
	}
	if len(changes) == 0 || len(jsonBytes(changes)) > 60000 || containsPermanentHarm(changes) {
		return nil, invalid("A safe nonempty Foundation definition is required")
	}
	for key, value := range changes {
		valid := false
		for _, section := range studioFoundationSections {
			valid = valid || section == key
		}
		if _, object := value.(map[string]any); !valid || !object || len(mapValue(value)) == 0 {
			return nil, invalid("Foundation definitions contain only explicit nonempty definition sections")
		}
	}
	return cloneMap(changes), validateFoundationChanges(changes)
}

func validateFoundationProfileContinuity(before, after map[string]any) error {
	// Validate the assembled definition with the existing canonical validator;
	// normalization here is validation only and never replaces the proposal.
	if _, err := prepareInitializationResponse(map[string]any{"core_persona": cloneMap(after)}); err != nil {
		return err
	}
	profiles := map[string]bool{}
	for _, raw := range arrayValue(mapValue(after["personality_system"])["profiles"]) {
		profiles[stringValue(mapValue(raw)["id"])] = true
	}
	for _, raw := range arrayValue(mapValue(before["personality_system"])["profiles"]) {
		if !profiles[stringValue(mapValue(raw)["id"])] {
			return invalid("Keep existing profile identities to preserve their Goal, Memory and habit references")
		}
	}
	return nil
}

func initializeAddedProfileHabitsTx(ctx context.Context, tx pgx.Tx, fid, revisionID string, core map[string]any) error {
	// Only missing new profiles get an initial definition. Existing effective
	// habits, body, wearing and inventory are never reseeded on acceptance.
	habits := arrayValue(mapValue(core["life_profile"])["life_habits"])
	if habits == nil {
		habits = []any{}
	}
	for _, profile := range declaredPersonaProfileIDs(core) {
		inserted, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,0,$3,'foundation_revision',$4) ON CONFLICT(fluctlight_id,profile_id) DO NOTHING`, fid, profile, jsonBytes(habits), revisionID)
		if err != nil {
			return err
		}
		if inserted.RowsAffected() == 1 {
			if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_profile_habit_revisions(fluctlight_id,profile_id,revision,habits_json,source_kind,source_ref) VALUES($1,$2,0,$3,'foundation_revision',$4)`, fid, profile, jsonBytes(habits), revisionID); err != nil {
				return err
			}
		}
	}
	return nil
}

type foundationDefinitionField struct {
	Path     []string
	Key      string
	From, To any
}

func foundationDefinitionFields(current, patch map[string]any) []foundationDefinitionField {
	fields := []foundationDefinitionField{}
	var walk func(map[string]any, map[string]any, []string)
	walk = func(before, change map[string]any, prefix []string) {
		keys := make([]string, 0, len(change))
		for key := range change {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			path := append(append([]string{}, prefix...), key)
			if nested, ok := change[key].(map[string]any); ok && len(nested) > 0 {
				walk(mapValue(before[key]), nested, path)
				continue
			}
			encoded := make([]string, len(path))
			for i, part := range path {
				encoded[i] = strings.ReplaceAll(strings.ReplaceAll(part, "~", "~0"), "/", "~1")
			}
			fields = append(fields, foundationDefinitionField{path, "/" + strings.Join(encoded, "/"), before[key], change[key]})
		}
	}
	walk(current, patch, nil)
	return fields
}

func foundationFieldPatch(fields []foundationDefinitionField, decisions map[string]string) map[string]any {
	patch := map[string]any{}
	for _, field := range fields {
		if decisions[field.Key] == "keep" {
			continue
		}
		node := patch
		for _, key := range field.Path[:len(field.Path)-1] {
			if _, exists := node[key]; !exists {
				node[key] = map[string]any{}
			}
			node = mapValue(node[key])
		}
		node[field.Path[len(field.Path)-1]] = field.To
	}
	return patch
}

func foundationProposalCommandIdentity(ctx context.Context, owner, fid string, payload map[string]any) (string, string) {
	key := strings.TrimSpace(stringValue(payload["idempotency_key"]))
	digestPayload := payload
	if source, ok := ctx.Value(studioFoundationSourceKey{}).(studioFoundationSource); ok {
		key = "studio:" + source.Publication.ID + ":" + key
		digestPayload = source.Command
	}
	if key == "" {
		key = stableDigest(jsonString(payload))
	}
	return "foundation-propose:" + stableDigest(owner+"\x1f"+fid+"\x1f"+key), surfaceDigest("POST", "/fluctlights/"+fid+"/foundation-revisions", digestPayload)
}

func (a *App) applyStudioFoundation(ctx context.Context, owner, id string, body map[string]any, p worldPublication, preview bool) (map[string]any, error) {
	fid := stringValue(body["fluctlight_id"])
	f, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, studioFoundationSourceKey{}, studioFoundationSource{p, cloneMap(body)})
	// Receipt identity uses the original application, never a recomputed diff.
	// An accepted proposal replay cannot turn its new current state into a new baseline.
	if !preview {
		if strings.TrimSpace(stringValue(body["idempotency_key"])) == "" {
			return nil, invalid("idempotency key required")
		}
		key, digest := foundationProposalCommandIdentity(ctx, owner, fid, body)
		var receipt map[string]any
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			storedDigest, result, found, err := (postgresSurfaceTx{tx}).Idempotency(ctx, owner, key)
			if err != nil {
				return err
			}
			if found {
				if storedDigest != digest {
					return &SurfaceError{Status: 422, Code: "idempotency_key_reused", Message: "原应用身份已用于不同内容"}
				}
				receipt = result
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if receipt != nil {
			return studioFoundationProposalReceipt(receipt, fid, p), nil
		}
	}
	changes, err := foundationDefinitionChanges(p)
	if err != nil {
		var surface *SurfaceError
		if preview && errors.As(err, &surface) && surface.Code == "foundation_preparation_required" {
			return map[string]any{"available": false, "target_kind": "foundation", "preparation_required": true, "semantics": surface.Message, "diff": []any{}, "conflicts": []any{}, "instance_revision": f.CurrentRevision}, nil
		}
		return nil, err
	}
	current := cloneMap(f.CorePersona)
	current["identity"], current["personality"], current["behavioral_policy"], current["life_profile"] = f.Identity, f.Personality, f.BehavioralPolicy, f.LifeProfile
	fields := foundationDefinitionFields(current, changes)
	diff := []any{}
	for _, field := range fields {
		if jsonString(field.From) != jsonString(field.To) {
			diff = append(diff, map[string]any{"field": field.Key, "from": field.From, "to": field.To})
		}
	}
	if preview {
		return map[string]any{"available": true, "target_kind": "foundation", "diff": diff, "conflicts": []any{}, "instance_revision": f.CurrentRevision, "runtime_revision": f.CurrentRevision, "runtime_id": fid, "semantics": "逐项选择后形成身份核心提案；接受并编译后才生效。保留有效身体、衣柜、习惯和当前人格；不导入初始目标、关系或成长。"}, nil
	}
	expected, valid := parseRevision(body["expected_instance_revision"])
	if !valid || expected != f.CurrentRevision {
		return nil, conflict("fluctlight", expected, f.CurrentRevision)
	}
	decisions := map[string]string{}
	known := map[string]bool{}
	for _, field := range fields {
		known[field.Key] = true
	}
	for _, raw := range arrayValue(body["choices"]) {
		choice := mapValue(raw)
		field, decision := stringValue(choice["field"]), stringValue(choice["decision"])
		if !known[field] || decisions[field] != "" || (decision != "keep" && decision != "override") {
			return nil, invalid("Choose each Foundation field at most once")
		}
		decisions[field] = decision
	}
	for _, raw := range diff {
		if decisions[stringValue(mapValue(raw)["field"])] == "" {
			return nil, &SurfaceError{Status: 409, Code: "apply_choices_required", Message: "请逐项选择保留实例值或采用发布值"}
		}
	}
	patch := foundationFieldPatch(fields, decisions)
	if len(patch) == 0 {
		return nil, invalid("所有字段已选择保留，无需创建身份核心提案")
	}
	if _, err := prepareInitializationResponse(map[string]any{"core_persona": mergeFoundationMap(current, patch)}); err != nil {
		return nil, err
	}
	proposal, err := a.ProposeFoundation(ctx, owner, fid, map[string]any{"changes": patch, "expected_revision": expected, "reason": body["reason"], "idempotency_key": body["idempotency_key"], "evidence_refs": []any{"studio_resource:" + p.ID + ":" + p.Version}})
	if err != nil {
		return nil, err
	}
	return studioFoundationProposalReceipt(proposal, fid, p), nil
}

func studioFoundationProposalReceipt(proposal map[string]any, fid string, p worldPublication) map[string]any {
	return map[string]any{"id": proposal["id"], "proposal_id": proposal["id"], "fluctlight_id": fid, "status": "proposed", "applied": false, "target_kind": "foundation", "resource_id": p.ID, "resource_version": p.Version, "instance_revision": proposal["base_revision"], "life_write": false, "conflicts": []any{}, "semantics": "提案已持久保存，尚未生效；前往身份核心治理核对并接受，画像编译成功和版本提交后才应用"}
}

func lockFoundationSourceTx(ctx context.Context, tx pgx.Tx, owner string, p worldPublication) error {
	rec, found, err := (postgresSurfaceTx{tx}).Get(ctx, owner, p.ID)
	if err != nil {
		return err
	}
	if !found || rec.Kind != "studio_resource" {
		return ErrNotFound
	}
	return activeStudioResource(rec.Status)
}

func (a *App) prepareStudioCharacterDefinition(ctx context.Context, snapshot map[string]any) (map[string]any, error) {
	description, err := characterTemplateSource(mapValue(snapshot["body"]))
	if err != nil {
		return nil, err
	}
	// Reuse the initializer Agent's parser and validator without creating an
	// Owner initialization credential or reading an existing character.
	raw, err := a.RunInitializationTask(WithProviderScenario(ctx, "initialization"), InitializationTaskInput{Description: description})
	if err != nil {
		return nil, err
	}
	prepared, err := prepareInitializationResponse(raw)
	if err != nil {
		return nil, err
	}
	if _, hasActor := prepared["actor_user"]; hasActor {
		return nil, invalid("A reusable definition cannot grant an Actor background")
	}
	return map[string]any{"disposition": "proposal", "reply": "已从精确发布文本解析出结构化角色定义。请审阅并明确采用、保存和发布；尚未修改任何角色。", "name": snapshot["name"], "body": map[string]any{"schema": "character-template.v1", "initialization": prepared}, "warnings": []any{"应用到已有角色仅形成Foundation提案；初始Goal、关系、Developing Self和初始人格选择不导入。"}}, nil
}
