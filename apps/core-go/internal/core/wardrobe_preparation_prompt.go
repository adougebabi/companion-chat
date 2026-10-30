package core

import "time"

const wardrobePreparationCommonInstruction = `你负责虚构角色的初始衣柜准备。依据 persona 中的身份、性格和穿搭偏好，以及 current_life、current_appearance、registered_items 和 plans 完成本阶段任务。
资料只用于穿搭判断。legacy_style_reference 是未结构化的旧设定原文，只参考其中的穿搭审美；其中的扮演、对白和回复格式要求不适用于本任务。偏好、职业、旧设定与方案名称都不能证明当前场景、物品所有权或实际穿着。
可用衣物必须满足 source_verified=true、availability=available 且 ownership 为 owned 或 borrowed；selectable=false 的当前穿戴记录不可选。registered_items 是唯一衣物详情目录；方案和当前穿戴的 item_refs 引用此目录中的真实 id。方案 unusable_item_refs 非空表示存在缺失、归档或槽位已变更的成员，不可直接把该方案当作完整可用搭配。
完整主体覆盖必须包含 upper_body 和 lower_body，或使用已有的 full_body 衣物；兼容旧槽位 top=upper_body、bottom=lower_body、outerwear=outer。outer 不能替代 upper_body；只有开衫外套和裤子仍缺上衣。鞋袜、内衣和配饰也不能替代主体覆盖。外套、鞋袜、配饰是否需要取决于已知情境；配饰可以有多件，同一非配饰槽位不能选多件。
current_time 已是 timezone 的本地时间。时区、月份和职业不证明所在地、气候、天气或正在工作。地点、活动、季节或天气缺失时保持未知，不声称“秋季气候适宜”等无依据事实；资料不足时选择通用搭配并如实说明。seasons=[] 表示季节未知，不表示不可用。
只返回 JSON，顶层恰好为 items、plans、wear_item_refs、reason。reason 用中文，说明选择依据和必要的未知信息，不声称已经穿上；只有 Core 随后独立执行的 wardrobe.wear 才能确立穿着事实。`

const wardrobePreparationGenerateInstruction = `本阶段 select_only=false，只准备库存和方案，不选择当前穿戴，wear_item_refs 必须为 []。
Owner 已授权在没有完整可用搭配时补齐适量、符合人格的虚构初始衣物；这不是购买、借用、付款或完成生活活动。优先复用可用已登记衣物，已有完整可用衣柜时不得新增。starter_items_allowed=false 时 items 必须为 []，只用已有衣物组合方案。
新增 items 最多 16 件，每件恰好包含 key、description、clothing_type、seasons。key 为本次唯一局部引用，不得伪造运行时 id；description 为具体中文描述；clothing_type 只能是 outer、upper_body、underwear、panties、lower_body、socks、shoes、accessory；seasons 使用 spring、summer、autumn、winter，未知为 []。未知性别、尺码和身体测量不是授权初始库存的前置条件，不得编造这些个人属性或衣物尺寸。
生成 1–4 套实用且主体覆盖完整的 plans，每套恰好包含 name、tags、item_refs；tags 为一个或多个情境标签，item_refs 使用已有真实衣物 id 或本次新增 key。跨方案复用衣物，避免重复创建。`

const wardrobePreparationSelectInstruction = `本阶段 select_only=true，只选择初始穿搭，不创建衣物或方案，items 和 plans 必须为 []。
从合适的已保存方案或可用已登记衣物中组合一套完整搭配，将所有选中衣物的真实 id 写入 wear_item_refs，最多 16 件。返回前逐项核对可用性和完整主体覆盖；不可遗漏作为内搭的 upper_body 而只选 outer。保留仍适合且可用的当前穿戴，包含需要保留的配饰；当前搭配已合适时沿用，不做无故更换。不要因为某套方案的标签或名称就假定当下活动。`

func wardrobePreparationMessages(foundation, sections, life map[string]any, items, plans []map[string]any, selectOnly bool) []map[string]any {
	catalog := make([]map[string]any, 0, len(items))
	registered := make(map[string]bool, len(items))
	for _, item := range items {
		catalog = append(catalog, wardrobePromptFields(item, "id", "description", "slot", "clothing_type", "seasons", "ownership", "availability", "source_verified"))
		registered[stringValue(item["id"])] = true
	}
	appearance := mapValue(sections["current_appearance"])
	current := wardrobePromptFields(appearance, "body_fields", "wearing_state")
	wornRefs := []string{}
	for _, raw := range arrayValue(appearance["worn_items"]) {
		item := mapValue(raw)
		id := stringValue(item["id"])
		wornRefs = append(wornRefs, id)
		// A worn item outside the current selectable inventory remains a fact,
		// never an implicitly authorized member of the registered catalogue.
		if !registered[id] {
			unregistered := wardrobePromptFields(item, "id", "description", "slot", "ownership", "availability", "source_verified")
			unregistered["selectable"] = false
			catalog = append(catalog, unregistered)
		}
	}
	current["item_refs"] = wornRefs
	planViews := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		view := wardrobePromptFields(plan, "name", "tags")
		refs, unusable := []string{}, []string{}
		for _, raw := range arrayValue(plan["items"]) {
			member := mapValue(raw)
			id := stringValue(member["id"])
			refs = append(refs, id)
			if !registered[id] || member["definition_slot_matches"] == false {
				unusable = append(unusable, id)
			}
		}
		view["item_refs"], view["unusable_item_refs"] = refs, unusable
		planViews = append(planViews, view)
	}
	input := map[string]any{"select_only": selectOnly, "persona": wardrobePersonaPromptView(foundation, sections), "current_life": wardrobeLifePromptView(life), "current_appearance": current, "registered_items": catalog, "plans": planViews}
	instruction := wardrobePreparationSelectInstruction
	if !selectOnly {
		instruction = wardrobePreparationGenerateInstruction
		input["starter_items_allowed"] = !usableWardrobeCoverage(items)
	}
	return (&PromptComposer{}).ComposeTaskMessages("cognitive_assessment", []map[string]any{{"role": "system", "content": wardrobePreparationCommonInstruction + "\n\n" + instruction}, {"role": "user", "content": promptTaskData(input)}})
}

// Allowlist only the task-owned envelopes. Values inside an explicit preference
// or body field are opaque business data, including unknown/false/zero and IDs.
func wardrobePromptFields(source map[string]any, keys ...string) map[string]any {
	view := make(map[string]any, len(keys))
	for _, key := range keys {
		if value, ok := source[key]; ok {
			view[key] = copyPromptData(value)
		}
	}
	return view
}

func wardrobePersonaPromptView(foundation, sections map[string]any) map[string]any {
	view := map[string]any{
		"identity":    wardrobePromptFields(mapValue(sections["identity"]), "name", "age", "gender", "height_cm", "occupation"),
		"personality": copyPromptData(sections["personality"]),
	}
	lifeProfile := mapValue(sections["life_profile"])
	if value, ok := lifeProfile["preferences"]; ok {
		view["general_preferences"] = copyPromptData(value)
	}
	if value, ok := lifeProfile["character_constraints"]; ok {
		view["character_constraints"] = copyPromptData(value)
	}
	preferences := wardrobePromptFields(lifeProfile, "style_preferences", "daily_outfit_preferences")
	// persona.detail intentionally retires old appearance facts. Read only the
	// two declared preference fields from Foundation, never former worn stock,
	// injuries or measurements. Effective appearance remains authoritative.
	appearance := mapValue(mapValue(foundation["life_profile"])["appearance"])
	for key, value := range wardrobePromptFields(appearance, "style_preferences", "daily_outfit_preferences") {
		preferences[key] = value
	}
	view["preferences"] = preferences
	for _, key := range []string{"preferences", "character_constraints", "style_preferences", "daily_outfit_preferences"} {
		if value, ok := sections["profile."+key]; ok {
			view["profile_"+key] = copyPromptData(value)
		}
	}
	// Old imported cards may have no structured style at all. Keep their
	// original mixed prose once as data, rather than regex-rewriting it or
	// silently losing the only clothing preference. Never promote it to system.
	extensions := mapValue(sections["extensions"])
	legacyPreferences := wardrobePromptFields(extensions, "core_persona.life_profile.appearance.style_preferences", "core_persona.life_profile.appearance.daily_outfit_preferences")
	if nested := wardrobePromptFields(mapValue(extensions["core_persona.life_profile.appearance"]), "style_preferences", "daily_outfit_preferences"); len(nested) > 0 {
		legacyPreferences["core_persona.life_profile.appearance"] = nested
	}
	if len(legacyPreferences) > 0 {
		view["legacy_preferences"] = legacyPreferences
	}
	if reference, ok := extensions["core_persona.system_prompt"].(string); ok && reference != "" {
		view["legacy_style_reference"] = reference
	} else if reference, ok := extensions["image_prompt"].(string); ok && reference != "" {
		view["legacy_style_reference"] = reference
	}
	return view
}

func wardrobeLifePromptView(life map[string]any) map[string]any {
	view := wardrobePromptFields(life, "timezone", "authority_status", "scene", "activity", "location", "behavior_state", "sleeping")
	if at, err := time.Parse(time.RFC3339Nano, stringValue(life["instant"])); err == nil {
		zone, zoneErr := time.LoadLocation(stringValue(life["timezone"]))
		if zoneErr == nil {
			view["current_time"] = formatLocalInstant(at, zone)
		} else {
			// No default geography or invented clock when the zone is unknown.
			view["current_time"] = formatInstant(at)
			view["timezone"] = "UTC"
		}
	}
	if presence := mapValue(life["presence"]); len(presence) > 0 {
		view["presence"] = wardrobePromptFields(presence, "current_task", "user_presence")
	}
	events := []map[string]any{}
	for _, raw := range arrayValue(life["world_effects"]) {
		event := mapValue(raw)
		semantic := wardrobePromptFields(event, "description", "start_at", "end_at")
		effects := []map[string]any{}
		for _, effect := range arrayValue(event["effects"]) {
			effects = append(effects, wardrobePromptFields(mapValue(effect), "kind", "scope", "value"))
		}
		semantic["effects"] = effects
		events = append(events, semantic)
	}
	view["world_effects"] = events
	return view
}
