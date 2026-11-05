package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Array repair is limited to declared initialization schema paths. Open
// objects and arbitrary user values are never traversed by guessing key names.
type initializationArrayRecovery struct {
	values             map[string]any
	warnings           []any
	profileListChanged bool
	profileIDs         map[string]int
	defaultProfile     bool
}

// The initialization endpoint requests json_object rather than sending the
// native schema, so include a compact, schema-derived array reference in its
// physical prompt. Open user objects remain opaque.
func initializationArraySchemaGuide() []any {
	entries := []any{}
	var visit func(map[string]any, string)
	visit = func(schema map[string]any, path string) {
		if len(arrayValue(schema["anyOf"])) > 0 {
			object, array, other := initializationSchemaVariants(schema)
			if !other && object != nil && array != nil {
				item := mapValue(array["items"])
				entries = append(entries, map[string]any{"path": path, "container": "object or array", "items": initializationArrayItemGuide(path, item)})
				visit(object, path)
			}
			return
		}
		switch schema["type"] {
		case "object":
			properties := mapValue(schema["properties"])
			keys := make([]string, 0, len(properties))
			for key := range properties {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				childPath := key
				if path != "" {
					childPath = path + "." + key
				}
				visit(mapValue(properties[key]), childPath)
			}
		case "array":
			item := mapValue(schema["items"])
			entries = append(entries, map[string]any{"path": path, "container": "array", "items": initializationArrayItemGuide(path, item)})
			visit(item, path+"[]")
		}
	}
	visit(initializationResponseSchema(), "")
	return entries
}

func initializationArrayItemGuide(path string, schema map[string]any) map[string]any {
	itemType := stringValue(schema["type"])
	if itemType == "" {
		itemType = "JSON value"
	}
	guide := map[string]any{"type": itemType}
	if required := arrayValue(schema["required"]); len(required) > 0 {
		guide["required"] = required
	}
	if path == "core_persona.life_profile.relationship_seeds" {
		guide["required"] = []any{"target_actor_id"}
	}
	if properties := mapValue(schema["properties"]); len(properties) > 0 {
		keys := make([]string, 0, len(properties))
		for key := range properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		guide["fields"] = keys
	}
	return guide
}

func (r *initializationArrayRecovery) keep(path string, value any, message string) {
	key := path
	for suffix := 2; ; suffix++ {
		if previous, exists := r.values[key]; !exists {
			break
		} else if jsonString(previous) == jsonString(value) {
			r.addWarning(path, message)
			return
		}
		key = fmt.Sprintf("%s#%d", path, suffix)
	}
	var snapshot any
	if err := json.Unmarshal(jsonBytes(value), &snapshot); err != nil {
		snapshot = value
	}
	r.values[key] = snapshot
	r.addWarning(path, message)
}

func (r *initializationArrayRecovery) addWarning(path, message string) {
	for _, raw := range r.warnings {
		warning := mapValue(raw)
		if warning["path"] == path && warning["message"] == message {
			return
		}
	}
	r.warnings = append(r.warnings, map[string]any{"path": path, "message": message})
}

func recoverAnalyzedInitializationArrays(value map[string]any) map[string]any {
	result := cloneMap(value)
	if result == nil {
		return value
	}
	extensions, validExtensions := result["extensions"].(map[string]any)
	recovered := map[string]any{}
	if validExtensions && extensions != nil {
		if existing, ok := extensions["recovered_fields"].(map[string]any); ok {
			for key, raw := range existing {
				recovered[key] = raw
			}
		}
	}
	r := &initializationArrayRecovery{values: recovered}
	r.promoteArrayOwners(result)
	r.recoverKnownArrayAliases(result)
	result = r.walk(initializationResponseSchema(), result, "").(map[string]any)
	r.recoverGoalsAndIntentions(result)
	r.guardProfileReferences(result)
	// Do not let recovery warnings manufacture an otherwise empty personality.
	if len(r.warnings) == 0 || initializationSemanticUnitCount(result) == 0 {
		return result
	}
	if !validExtensions || extensions == nil {
		extensions = map[string]any{}
		if raw, present := result["extensions"]; present && raw != nil {
			r.keep("extensions", raw, "扩展资料格式不完整，原值已保留供核对。")
		}
	}
	if raw, exists := extensions["recovered_fields"]; exists && raw != nil {
		if _, ok := raw.(map[string]any); !ok {
			r.keep("extensions.recovered_fields", raw, "原补充记录格式不完整，原值已保留供核对。")
		}
	}
	warnings := arrayValue(extensions["initialization_warnings"])
	if raw, exists := extensions["initialization_warnings"]; exists && raw != nil {
		if _, ok := raw.([]any); !ok {
			r.keep("extensions.initialization_warnings", raw, "原补充说明格式不完整，原值已保留供核对。")
		}
	}
	extensions["recovered_fields"] = r.values
	extensions["initialization_warnings"] = append(warnings, r.warnings...)
	result["extensions"] = extensions
	return result
}

// Match the existing container recovery for known root-level persona groups
// before walking the schema, so model aliases receive the same array repair.
func (r *initializationArrayRecovery) promoteArrayOwners(value map[string]any) {
	persona, ok := value["core_persona"].(map[string]any)
	if !ok || persona == nil {
		if value["core_persona"] != nil {
			r.keep("core_persona", value["core_persona"], "人物资料结构不完整，原值已保留供核对。")
		}
		persona = map[string]any{}
		value["core_persona"] = persona
	}
	for _, key := range []string{"identity", "personality", "behavioral_policy", "life_profile", "personality_system"} {
		if persona[key] == nil && value[key] != nil {
			persona[key] = value[key]
			delete(value, key)
		}
	}
}

func initializationSchemaVariants(schema map[string]any) (map[string]any, map[string]any, bool) {
	var object, array map[string]any
	other := false
	for _, raw := range arrayValue(schema["anyOf"]) {
		variant := mapValue(raw)
		switch variant["type"] {
		case "object":
			object = variant
		case "array":
			array = variant
		default:
			other = true
		}
	}
	return object, array, other
}

func (r *initializationArrayRecovery) walk(schema map[string]any, value any, path string) any {
	if len(arrayValue(schema["anyOf"])) > 0 {
		object, array, other := initializationSchemaVariants(schema)
		// jsonValueSchema and nullable fields accept scalars. Their free-form
		// contents are source data, not paths for recursive schema repair.
		if other || object == nil || array == nil {
			return value
		}
		if _, ok := value.(map[string]any); ok {
			return r.walk(object, value, path)
		}
		if _, ok := value.([]any); ok {
			return r.walk(array, value, path)
		}
		r.keep(path, value, "模型返回的内容不是可用的列表或字段组，原值已保留供核对。")
		return []any{}
	}
	switch schema["type"] {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return value
		}
		properties := mapValue(schema["properties"])
		keys := make([]string, 0, len(properties))
		for key := range properties {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child, present := object[key]
			if !present {
				continue
			}
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			object[key] = r.walk(mapValue(properties[key]), child, childPath)
		}
		if path == "core_persona.personality_system" && r.profileListChanged {
			r.profileIDs = map[string]int{}
			for _, raw := range arrayValue(object["profiles"]) {
				if id := strings.TrimSpace(stringValue(mapValue(raw)["id"])); id != "" {
					r.profileIDs[id]++
				}
			}
			active := stringValue(object["active_profile_id"])
			if active != "" && active != "default" && r.profileIDs[active] != 1 {
				r.keep(path+".active_profile_id", active, "当前人格引用不明确，原值已保留，暂时使用共享人格。")
				object["active_profile_id"] = "default"
				active = "default"
			}
			r.defaultProfile = active == "" || active == "default" || r.profileIDs["default"] == 1
		}
		return object
	case "array":
		if path == "initial_goals" || path == "initial_intentions" {
			return value // Their positions and links are recovered together below.
		}
		itemSchema := mapValue(schema["items"])
		items, ok := value.([]any)
		if !ok {
			if path == "core_persona.personality_system.profiles" {
				r.profileListChanged = true
			}
			r.keep(path, value, "模型返回的列表格式不正确，原值已保留供核对。")
			if value != nil && initializationFreeListItem(itemSchema, value) {
				return []any{value}
			}
			return []any{}
		}
		itemType := stringValue(itemSchema["type"])
		if path == "core_persona.personality_system.profiles" {
			ids := map[string]int{}
			ambiguous := false
			for _, item := range items {
				if !isObjectValue(item) || len(mapValue(item)) == 0 {
					continue
				}
				id := strings.TrimSpace(stringValue(mapValue(item)["id"]))
				if id == "" {
					ambiguous = true
				} else {
					ids[id]++
					ambiguous = ambiguous || ids[id] > 1
				}
			}
			if ambiguous {
				r.profileListChanged = true
				r.keep(path, items, "人格标识缺失或重复，原始档案已保留；不明确的引用暂不采用。")
			}
		}
		if path == "core_persona.life_profile.appearance.wardrobe_items" || path == "core_persona.life_profile.wardrobe_items" {
			for _, item := range items {
				if !initializationGarmentTypesValid(item) {
					r.keep(path, items, "初始衣物包含格式错误的项目，整组原值已保留，避免改变衣物编号和穿着状态。")
					return []any{}
				}
			}
		}
		result := make([]any, 0, len(items))
		for index, item := range items {
			itemPath := fmt.Sprintf("%s[%d]", path, index)
			valid := itemType != "object" || isObjectValue(item)
			if path == "core_persona.personality_system.profiles" && len(mapValue(item)) == 0 {
				valid = false
			}
			if itemType == "string" {
				_, valid = item.(string)
			}
			if valid && (path == "initial_relationships" || path == "core_persona.life_profile.relationship_seeds" || path == "relationships" || path == "relationship_seeds") {
				valid = initializationExplicitOwnerRelationship(mapValue(item), path != "core_persona.life_profile.relationship_seeds")
			}
			if !valid {
				if path == "core_persona.personality_system.profiles" {
					r.profileListChanged = true
				}
				r.keep(itemPath, item, "模型返回的列表项格式或关系对象不明确，原值已保留供核对。")
				continue
			}
			result = append(result, r.walk(itemSchema, item, itemPath))
		}
		return result
	default:
		return value
	}
}

func (r *initializationArrayRecovery) recoverKnownArrayAliases(value map[string]any) {
	schema := initializationResponseSchema()
	root := mapValue(schema["properties"])
	personaSchema := mapValue(mapValue(root["core_persona"])["properties"])
	identitySchema := mapValue(mapValue(personaSchema["identity"])["properties"])
	lifeSchema := mapValue(mapValue(personaSchema["life_profile"])["properties"])
	appearanceSchema := mapValue(mapValue(lifeSchema["appearance"])["properties"])
	persona := mapValue(value["core_persona"])
	identity := mapValue(persona["identity"])
	life := mapValue(persona["life_profile"])
	if raw, present := identity["values"]; present {
		if canonical, exists := identity["core_values"]; exists && canonical != nil {
			if jsonString(canonical) != jsonString(raw) {
				r.keep("core_persona.identity.values", raw, "核心价值观存在两份不同内容，原字段已保留供核对。")
			}
			delete(identity, "values")
		} else {
			identity["values"] = r.walk(mapValue(identitySchema["core_values"]), raw, "core_persona.identity.values")
		}
	}
	if raw, present := life["wardrobe_items"]; present {
		life["wardrobe_items"] = r.walk(mapValue(appearanceSchema["wardrobe_items"]), raw, "core_persona.life_profile.wardrobe_items")
		// Preserve the established alias promotion: an explicit null at the
		// canonical path is missing, not a competing empty wardrobe.
		if appearance, ok := life["appearance"].(map[string]any); ok && appearance != nil {
			if canonical, exists := appearance["wardrobe_items"]; exists && canonical == nil {
				r.keep("core_persona.life_profile.appearance.wardrobe_items", nil, "衣物规范位置尚未填写，已保留原值并核对已有衣物。")
				delete(appearance, "wardrobe_items")
			}
		}
	}
	for _, alias := range []string{"relationships", "relationship_seeds"} {
		if raw, present := value[alias]; present {
			value[alias] = r.walk(mapValue(root["initial_relationships"]), raw, alias)
		}
	}
}

func initializationGarmentTypesValid(value any) bool {
	item, ok := value.(map[string]any)
	if !ok || item == nil {
		return false
	}
	for _, key := range []string{"category", "slot", "description", "ownership", "item_kind"} {
		if raw, present := item[key]; present {
			if _, ok := raw.(string); !ok {
				return false
			}
		}
	}
	for _, key := range []string{"available", "currently_worn"} {
		if raw, present := item[key]; present {
			if _, ok := raw.(bool); !ok {
				return false
			}
		}
	}
	return true
}

func (r *initializationArrayRecovery) validProfileScope(item map[string]any) bool {
	if !r.profileListChanged {
		return true
	}
	for _, field := range []string{"profile_id", "associated_profile", "profile"} {
		raw, present := item[field]
		if !present || raw == nil {
			continue
		}
		id, ok := raw.(string)
		if !ok {
			return false
		}
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		return (id == "default" && r.defaultProfile) || r.profileIDs[id] == 1
	}
	return true
}

func (r *initializationArrayRecovery) guardProfileReferences(value map[string]any) {
	if !r.profileListChanged {
		return
	}
	persona := mapValue(value["core_persona"])
	life := mapValue(persona["life_profile"])
	system := mapValue(persona["personality_system"])
	filter := func(owner map[string]any, key, path string, valid func(map[string]any) bool) {
		items, ok := owner[key].([]any)
		if !ok {
			return
		}
		kept := make([]any, 0, len(items))
		for index, raw := range items {
			item := mapValue(raw)
			if !valid(item) {
				r.keep(fmt.Sprintf("%s[%d]", path, index), raw, "关联的人格不明确，原值已保留，暂不采用。")
				continue
			}
			kept = append(kept, raw)
		}
		owner[key] = kept
	}
	filter(life, "relationship_seeds", "core_persona.life_profile.relationship_seeds", r.validProfileScope)
	filter(value, "initial_relationships", "initial_relationships", r.validProfileScope)
	filter(value, "relationships", "relationships", r.validProfileScope)
	filter(value, "relationship_seeds", "relationship_seeds", r.validProfileScope)
	for _, collection := range []struct {
		owner  map[string]any
		key    string
		path   string
		fields []string
	}{
		{mapValue(system["switching"]), "rules", "core_persona.personality_system.switching.rules", []string{"target_profile_id"}},
		{mapValue(system["influence"]), "edges", "core_persona.personality_system.influence.edges", []string{"from_profile_id", "to_profile_id"}},
		{system, "takeover_rules", "core_persona.personality_system.takeover_rules", []string{"target_profile_id", "source_profile_id"}},
	} {
		filter(collection.owner, collection.key, collection.path, func(item map[string]any) bool {
			for index, field := range collection.fields {
				raw, present := item[field]
				if !present && index > 0 && field == "source_profile_id" {
					continue
				}
				id, ok := raw.(string)
				if !ok || r.profileIDs[strings.TrimSpace(id)] != 1 {
					return false
				}
			}
			return true
		})
	}
}

func initializationFreeListItem(schema map[string]any, value any) bool {
	if schema["type"] == "string" {
		_, ok := value.(string)
		return ok
	}
	_, _, other := initializationSchemaVariants(schema)
	return other && len(arrayValue(schema["anyOf"])) > 0
}

func initializationExplicitOwnerRelationship(item map[string]any, aliases bool) bool {
	target := stringValue(item["target_actor_id"])
	if target == "" && aliases {
		target = firstInitializationString(item["target"], item["actor"], item["actor_id"])
	}
	if strings.TrimSpace(target) == "" || resolveInitializationActorRef(target, "actor_user", "actor_self") != "actor_user" {
		return false
	}
	if profile, exists := item["profile_id"]; exists && profile != nil {
		if _, ok := profile.(string); !ok {
			return false
		}
	}
	return true
}

func initializationGroupedCollection(value any) bool {
	grouped, ok := value.(map[string]any)
	if !ok || len(grouped) == 0 {
		return false
	}
	for _, group := range grouped {
		if _, ok := group.([]any); !ok {
			return false
		}
	}
	return true
}

func initializationFlattenGroups(value any, field string) []any {
	grouped := mapValue(value)
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := []any{}
	for _, group := range keys {
		for _, raw := range arrayValue(grouped[group]) {
			if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
				items = append(items, map[string]any{field: text, "horizon": group})
			} else {
				items = append(items, raw)
			}
		}
	}
	return items
}

// Preserve the original goal positions while removing malformed candidates so
// an intention cannot silently start referring to a different goal.
func (r *initializationArrayRecovery) recoverGoalsAndIntentions(value map[string]any) {
	rawGoals, goalsPresent := value["initial_goals"]
	goals, goalsAreArray := rawGoals.([]any)
	goalsChanged, goalsGrouped := false, false
	if goalsPresent && !goalsAreArray {
		r.keep("initial_goals", rawGoals, "目标列表格式已整理，原值已保留供核对。")
		goalsChanged = true
		switch {
		case initializationGroupedCollection(rawGoals):
			goals, goalsGrouped = initializationFlattenGroups(rawGoals, "description"), true
		case strings.TrimSpace(stringValue(rawGoals)) != "":
			goals = []any{map[string]any{"description": rawGoals}}
		default:
			goals = []any{}
		}
	} else if !goalsPresent {
		goals = []any{}
	}
	mapping := map[int]int{}
	validGoals := make([]any, 0, len(goals))
	goalIDs := map[string]int{}
	for index, raw := range goals {
		goal, object := raw.(map[string]any)
		if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
			goal, object = map[string]any{"description": text}, true
			r.keep(fmt.Sprintf("initial_goals[%d]", index), raw, "文字目标已整理为目标项目，原值已保留供核对。")
			goalsChanged = true
		}
		if !object || !r.validProfileScope(goal) || firstInitializationString(goal["description"], goal["content"], goal["text"], goal["desired_outcome"], goal["goal"]) == "" {
			r.keep(fmt.Sprintf("initial_goals[%d]", index), raw, "目标项目格式不完整，原值已保留供核对。")
			goalsChanged = true
			continue
		}
		mapping[index] = len(validGoals)
		validGoals = append(validGoals, goal)
		if id := stringValue(goal["id"]); id != "" {
			goalIDs[id]++
		}
	}
	if goalsChanged {
		value["initial_goals"] = validGoals
	}
	rawIntentions, intentionsPresent := value["initial_intentions"]
	if !intentionsPresent {
		return
	}
	intentions, ok := rawIntentions.([]any)
	intentionsChanged := goalsChanged
	if !ok {
		r.keep("initial_intentions", rawIntentions, "意图列表格式已整理，原值已保留供核对。")
		intentionsChanged = true
		switch {
		case initializationGroupedCollection(rawIntentions):
			intentions = initializationFlattenGroups(rawIntentions, "action")
		case strings.TrimSpace(stringValue(rawIntentions)) != "":
			intentions = []any{map[string]any{"action": rawIntentions}}
		default:
			intentions = []any{}
		}
	}
	for _, raw := range intentions {
		item := mapValue(raw)
		if !isObjectValue(raw) || !r.validProfileScope(item) || firstInitializationString(item["action"], item["description"], item["intent"], item["content"], item["text"], item["action_intent"]) == "" {
			intentionsChanged = true
		}
		if _, explicit := item["goal_index"]; !explicit {
			if rawID, linked := item["linked_goal_id"]; linked {
				id, ok := rawID.(string)
				id = strings.TrimSpace(id)
				if !ok || id == "" || goalIDs[id] != 1 {
					intentionsChanged = true
				}
			}
		}
	}
	if !intentionsChanged {
		return
	}
	validIntentions := make([]any, 0, len(intentions))
	for index, raw := range intentions {
		intention, object := raw.(map[string]any)
		if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
			intention, object = map[string]any{"action": text}, true
			r.keep(fmt.Sprintf("initial_intentions[%d]", index), raw, "文字意图已整理为意图项目，原值已保留供核对。")
		}
		if !object || !r.validProfileScope(intention) || firstInitializationString(intention["action"], intention["description"], intention["intent"], intention["content"], intention["text"], intention["action_intent"]) == "" {
			r.keep(fmt.Sprintf("initial_intentions[%d]", index), raw, "意图项目格式不完整，原值已保留供核对。")
			continue
		}
		if rawID, linked := intention["linked_goal_id"]; linked && intention["goal_index"] == nil {
			linkedID, valid := rawID.(string)
			linkedID = strings.TrimSpace(linkedID)
			if !valid || linkedID == "" || goalIDs[linkedID] != 1 {
				r.keep(fmt.Sprintf("initial_intentions[%d]", index), raw, "意图关联的目标不明确，意图原值已保留，暂不采用。")
				continue
			}
		} else {
			if goalsGrouped {
				r.keep(fmt.Sprintf("initial_intentions[%d]", index), raw, "分组目标的位置引用不明确，意图原值已保留，暂不采用。")
				continue
			}
			oldIndex := index
			if rawIndex, explicit := intention["goal_index"]; explicit {
				var valid bool
				oldIndex, valid = initializationArrayIndex(rawIndex)
				if !valid {
					oldIndex = -1
				}
			}
			newIndex, found := mapping[oldIndex]
			if !found {
				r.keep(fmt.Sprintf("initial_intentions[%d]", index), raw, "意图关联的目标不可用，意图原值已保留，暂不采用。")
				continue
			}
			intention["goal_index"] = newIndex
		}
		validIntentions = append(validIntentions, intention)
	}
	value["initial_intentions"] = validIntentions
}
