package core

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Chat surfaces (the direct main turn, fact-only cognition and the Writer) send
// their dynamic context as readable Markdown instead of JSON. The prompt keeps
// four parts: protocol + persona (system), the current situation, summaries of
// earlier conversation, and the unsummarized messages. Every other Agent keeps
// the deterministic JSON task views.
func chatMarkdownSurface(surface ProviderContextSurface) bool {
	switch surface {
	case ProviderContextSurfaceConversationMain, ProviderContextSurfaceFactOnlyCognition, ProviderContextSurfaceFactOnlyReply:
		return true
	default:
		return false
	}
}

// directChatProtocol replaces the authority rule, the direct main protocol and
// the execution boundary for the direct private chat main turn.
const directChatProtocol = `你就是下面「人格设定」里的这个人，正在用聊天软件和对方私聊。

1. 说话：用你自己的语气自然回复，长短随情境，可以用（）写自己的动作神态，不写第三人称旁白。聊天不代表你们在同一个地方。
2. 事实：「现在」里的时间、地点、在做的事、穿着、衣柜和日程就是你此刻的真实情况。「之前聊过」和历史消息只代表当时说过什么。不知道的事就说不知道，不要编。
3. 记住对方：对方提到自己长期的情况（住在哪里、在国内还是国外、工作学习、作息、喜好、重要的人和事），先用 actor.fact.record 记下来再回复；「关于对方」里已经有的不用重复记。约见面、一起吃饭逛街之前先看「关于对方」，对方在外地或国外就不能当面约。
4. 做事：换衣服、出门、买东西、改日程要真的调用工具去做，拿到结果再说；没做成就如实说没做成和原因。只是嘴上答应不算做了。
5. 衣服：只能穿、只能说自己有「衣柜」里列出的衣服。想要新衣服就去买：用日程或活动工具安排去商场购物，买到的衣服会进衣柜。没有网购和快递。
6. 拍照：对方想看照片而你也愿意时，用图片工具拍；照片拍好后会单独发过去，你可以像平时一样说"拍好了马上发你"。
7. 工具：一开始只列出少数工具，其余能力用 capability_catalog 查（names 留空会列出全部用途），查到后按需加载再调用。
8. 回复：最后用 conversation_reply 发出正文（text）。decision 写一句 response_intent 即可，其余字段有把握再填，没有就省略。`

var directChatReplacedRules = []string{providerContextAuthorityRule, directMainConversationProtocol, factOnlyExecutionBoundary}

// renderDirectChatSystem builds the system message for the direct main turn.
// Extra rules (expression suggestions, retry protocol) are kept after it.
func renderDirectChatSystem(operationRules []string, persona map[string]any) string {
	var builder strings.Builder
	builder.WriteString("# 运行协议\n\n")
	builder.WriteString(directChatProtocol)
	if isMultiPersonalitySystem(mapValue(persona["personality_system"])) {
		builder.WriteString("\n9. 多重人格：只使用已声明的人格与切换规则；持久切换用 persona.switch，本次接管用 persona.takeover，拿到成功结果再继续。")
	}
	seen := map[string]bool{}
	for _, rule := range operationRules {
		rule = strings.TrimSpace(rule)
		if rule == "" || rule == providerLanguageRule || seen[rule] {
			continue
		}
		seen[rule] = true
		replaced := false
		for _, fixed := range directChatReplacedRules {
			if rule == fixed {
				replaced = true
			}
		}
		if !replaced {
			builder.WriteString("\n\n" + rule)
		}
	}
	appendChatPersona(&builder, persona)
	return strings.TrimSpace(builder.String())
}

func directChatRules(operationRules []string) bool {
	for _, rule := range operationRules {
		if rule == directMainConversationProtocol {
			return true
		}
	}
	return false
}

// appendChatPersona renders the Working Persona as labelled text. Every value
// is kept; only the encoding changes from JSON to readable lines.
func appendChatPersona(builder *strings.Builder, persona map[string]any) {
	if len(persona) == 0 {
		return
	}
	builder.WriteString("\n\n# 人格设定\n\n")
	builder.WriteString(strings.TrimRight(renderChatMarkdownValue(persona, 0), "\n"))
}

var chatMarkdownLabels = map[string]string{
	"name": "名字", "display_name": "名字", "identity": "身份", "shared_identity": "共同身份", "working_persona": "当前人格",
	"expression": "表达", "interpretation_boundary": "理解边界", "personality": "性格", "background": "背景", "values": "价值观",
	"activity": "在做的事", "location": "地点", "scene": "场景", "effective_at": "开始", "expires_at": "持续到",
	"mood": "心情", "body": "身体", "worn_items": "穿着", "description": "描述", "category": "类别", "slot": "部位",
	"availability": "状态", "summary": "概况", "trend": "趋势", "role": "关系", "label": "称呼", "preferred_addressing": "称呼对方",
	"target_actor": "对象", "desired_outcome": "想要的结果", "motivation": "动机", "state": "状态", "status": "状态",
	"success_criteria": "完成条件", "content": "内容", "occurred_at": "时间", "type": "类型", "confidence": "把握",
	"start": "开始", "end": "结束", "title": "事项", "channel": "渠道", "co_presence": "是否同处",
	"recipient_location": "对方位置", "self_location": "你的位置", "attribute": "项目", "value": "内容", "ref": "引用",
	"execution": "推进情况", "current_stage": "当前阶段", "strategy": "做法", "next_step": "下一步", "wait_condition": "等待条件",
	"appearance": "外表", "inner_state": "内在状态", "affect_profile": "情绪", "active_activities": "进行中的活动",
	"items": "物品", "timezone": "时区", "image_generation_registered": "能拍照发图", "vision_understanding": "能看图",
	"world_environment": "环境", "body_fields": "身体", "wearing_state": "穿着状态", "used_items": "正在用的东西", "secondary": "其他关系", "self_reference": "自称", "emotional_association": "情感联系",
}

// Envelope keys whose children are rendered at the same level.
var chatMarkdownTransparentKeys = map[string]bool{"data": true, "life_context": true}

// Exact metadata keys that describe provenance or interpretation rules for the
// former JSON view. They carry no fact for the model and are not rendered.
var chatMarkdownMetadataKeys = map[string]bool{
	"lineage": true, "provenance_status": true, "source_kind": true, "certainty": true, "projection": true,
	"history_time_semantics": true, "text_semantics": true, "style_semantics": true, "evidence_semantics": true,
	"current_authority": true, "time_semantics": true, "timezone_semantics": true, "summary_semantics": true,
	"snapshot_semantics": true, "control_semantics": true, "confirmation_semantics": true, "channel_source": true,
	"co_presence_source": true, "subject": true, "authority": true, "epistemic_kind": true, "associated_actors": true,
	"history_index": true, "as_of": true, "reference_timezone": true,
}

// chatMarkdownOpaque reports business data that must reach the model exactly
// as stored (schemas, documents, actor-owned values). It stays inline JSON.
func chatMarkdownOpaque(key string, value any) bool {
	if chatMarkdownIsScalar(value) {
		return false
	}
	return key == "world_environment" || promptOpaqueDataField(key, value)
}

func chatMarkdownLabel(key string) string {
	if label, ok := chatMarkdownLabels[key]; ok {
		return label
	}
	return key
}

// chatMarkdownTime turns an RFC3339 instant into a short local reading while
// keeping its own offset (the value is already in the authoritative zone).
func chatMarkdownTime(value string) (string, bool) {
	if len(value) < 16 || value[4] != '-' || value[10] != 'T' {
		return "", false
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", false
	}
	weekdays := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return fmt.Sprintf("%d月%d日 %s %02d:%02d", parsed.Month(), parsed.Day(), weekdays[parsed.Weekday()], parsed.Hour(), parsed.Minute()), true
}

// chatMarkdownKeyed keeps the original UTC offset for sender-side snapshots:
// a sender's own clock is a separate authority from the self's display zone.
func chatMarkdownKeyed(key string, value any) string {
	if text, ok := value.(string); ok && strings.HasPrefix(key, "sender_") {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			formatted, _ := chatMarkdownTime(text)
			return formatted + "（UTC" + parsed.Format("-07:00") + "）"
		}
	}
	return chatMarkdownScalar(value)
}

func chatMarkdownScalar(value any) string {
	switch typed := value.(type) {
	case nil:
		return "未知"
	case string:
		if formatted, ok := chatMarkdownTime(typed); ok {
			return formatted
		}
		return typed
	case bool:
		if typed {
			return "是"
		}
		return "否"
	default:
		return fmt.Sprint(typed)
	}
}

func chatMarkdownIsScalar(value any) bool {
	switch value.(type) {
	case map[string]any, []any, []map[string]any:
		return false
	default:
		return true
	}
}

func chatMarkdownList(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items, true
	case []string:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return items, true
	default:
		return nil, false
	}
}

// renderChatMarkdownValue renders any task value as nested bullet lines.
func renderChatMarkdownValue(value any, depth int) string {
	indent := strings.Repeat("  ", depth)
	var builder strings.Builder
	if items, ok := chatMarkdownList(value); ok {
		scalars := true
		for _, item := range items {
			if !chatMarkdownIsScalar(item) {
				scalars = false
			}
		}
		if scalars {
			parts := make([]string, 0, len(items))
			for _, item := range items {
				parts = append(parts, chatMarkdownScalar(item))
			}
			builder.WriteString(indent + "- " + strings.Join(parts, "、") + "\n")
			return builder.String()
		}
		for _, item := range items {
			if chatMarkdownIsScalar(item) {
				builder.WriteString(indent + "- " + chatMarkdownScalar(item) + "\n")
				continue
			}
			builder.WriteString(chatMarkdownItem(item, depth))
		}
		return builder.String()
	}
	object := mapValue(value)
	if object == nil {
		return indent + "- " + chatMarkdownScalar(value) + "\n"
	}
	object = chatMarkdownFlatten(object)
	keys := make([]string, 0, len(object))
	for key := range object {
		if !chatMarkdownMetadataKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		child := object[key]
		label := chatMarkdownLabel(key)
		if items, ok := chatMarkdownList(child); ok && len(items) == 0 {
			builder.WriteString(indent + "- " + label + "：无\n")
			continue
		}
		if chatMarkdownOpaque(key, child) {
			builder.WriteString(indent + "- " + label + "：" + renderProviderJSON(child) + "\n")
			continue
		}
		if child == nil {
			builder.WriteString(indent + "- " + label + "：未知\n")
			continue
		}
		if chatMarkdownIsScalar(child) {
			builder.WriteString(indent + "- " + label + "：" + chatMarkdownKeyed(key, child) + "\n")
			continue
		}
		if items, ok := chatMarkdownList(child); ok {
			allScalar := true
			for _, item := range items {
				if !chatMarkdownIsScalar(item) {
					allScalar = false
				}
			}
			if allScalar {
				parts := make([]string, 0, len(items))
				for _, item := range items {
					parts = append(parts, chatMarkdownScalar(item))
				}
				builder.WriteString(indent + "- " + label + "：" + strings.Join(parts, "、") + "\n")
				continue
			}
		}
		nested := renderChatMarkdownValue(child, depth+1)
		if strings.TrimSpace(nested) == "" {
			continue
		}
		builder.WriteString(indent + "- " + label + "：\n" + nested)
	}
	return builder.String()
}

// chatMarkdownFlatten lifts the children of envelope keys one level up.
func chatMarkdownFlatten(object map[string]any) map[string]any {
	flattened := false
	for key := range object {
		if chatMarkdownTransparentKeys[key] && mapValue(object[key]) != nil {
			flattened = true
		}
	}
	if !flattened {
		return object
	}
	result := map[string]any{}
	for key, child := range object {
		if chatMarkdownTransparentKeys[key] && mapValue(child) != nil {
			for innerKey, innerValue := range chatMarkdownFlatten(mapValue(child)) {
				if _, exists := result[innerKey]; !exists {
					result[innerKey] = innerValue
				}
			}
			continue
		}
		result[key] = child
	}
	return result
}

// chatMarkdownItem renders one object of a list as a single bullet whose
// first scalar fields are joined on one line; nested values follow indented.
func chatMarkdownItem(item any, depth int) string {
	object := mapValue(item)
	indent := strings.Repeat("  ", depth)
	marker := ""
	if kind := stringValue(object["epistemic_kind"]); kind == "inference" || kind == "hypothesis" {
		marker = "（推测）"
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		if !chatMarkdownMetadataKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		// Lead with the main text of the item.
		rank := func(key string) int {
			switch key {
			case "content", "description", "summary", "title", "claim", "value":
				return 0
			case "ref":
				return 2
			default:
				return 1
			}
		}
		if rank(keys[i]) != rank(keys[j]) {
			return rank(keys[i]) < rank(keys[j])
		}
		return keys[i] < keys[j]
	})
	parts := []string{}
	nested := map[string]any{}
	for _, key := range keys {
		child := object[key]
		if items, ok := chatMarkdownList(child); ok && len(items) == 0 {
			parts = append(parts, chatMarkdownLabel(key)+"：无")
			continue
		}
		if chatMarkdownOpaque(key, child) {
			parts = append(parts, chatMarkdownLabel(key)+"："+renderProviderJSON(child))
			continue
		}
		if !chatMarkdownIsScalar(child) {
			nested[key] = child
			continue
		}
		switch key {
		case "content", "description", "summary", "title", "claim":
			parts = append(parts, chatMarkdownScalar(child))
		case "ref":
			parts = append(parts, "[引用 "+chatMarkdownScalar(child)+"]")
		default:
			parts = append(parts, chatMarkdownLabel(key)+"："+chatMarkdownKeyed(key, child))
		}
	}
	line := strings.Join(parts, "；")
	if line == "" && len(nested) == 0 {
		return ""
	}
	result := indent + "- " + marker + line + "\n"
	if len(nested) > 0 {
		result += renderChatMarkdownValue(nested, depth+1)
	}
	return result
}

type chatMarkdownSection struct {
	title string
	keys  []string
}

// Section order follows the four-part prompt: what is true now, who the other
// person is, what is wanted, what is remembered, then earlier conversation.
var chatMarkdownSections = []chatMarkdownSection{
	{"现在", []string{"time_view", "self_actor", "current_state", "wardrobe", "schedule", "presence", "communication_state", "action_execution_boundary", "capability_boundaries"}},
	{"关于对方", []string{"current_speaker", "actor_background", "relationships"}},
	{"目标与打算", []string{"goals", "goal_policy", "intentions", "recent_outcomes"}},
	{"你记得的事", []string{"resident_memory", "active_memory", "retrieved_memory"}},
	{"之前聊过", []string{"conversation_summaries"}},
}

var chatMarkdownSubtitles = map[string]string{
	"time_view": "时间", "self_actor": "你", "current_state": "你的状态", "wardrobe": "衣柜", "schedule": "日程",
	"presence": "在场", "communication_state": "交流方式", "action_execution_boundary": "做事边界", "capability_boundaries": "能力",
	"current_speaker": "对方", "actor_background": "对方的情况", "relationships": "你们的关系", "goals": "目标",
	"goal_policy": "目标设置", "intentions": "打算", "recent_outcomes": "最近做过的事", "resident_memory": "一直记得",
	"active_memory": "最近想到", "retrieved_memory": "相关回忆", "conversation_summaries": "",
	"history_metadata": "消息时间", "developing_self": "正在形成的自我认识", "visual_identity": "外貌",
}

// renderChatRuntimeMarkdown renders the scoped runtime context of a chat
// surface. Every non-metadata value is emitted exactly once.
func renderChatRuntimeMarkdown(value map[string]any) string {
	used := map[string]bool{}
	var builder strings.Builder
	writeKey := func(key string) {
		child, ok := value[key]
		used[key] = true
		if !ok || child == nil {
			return
		}
		var body string
		switch key {
		case "time_view":
			body = renderChatTimeView(mapValue(child))
		case "wardrobe":
			body = renderChatWardrobe(child)
		case "self_actor", "current_speaker":
			body = renderChatActor(mapValue(child))
		default:
			body = renderChatMarkdownValue(child, 0)
		}
		if strings.TrimSpace(body) == "" {
			return
		}
		if subtitle := chatMarkdownSubtitles[key]; subtitle != "" {
			builder.WriteString("## " + subtitle + "\n")
		}
		builder.WriteString(body)
	}
	for _, section := range chatMarkdownSections {
		start := builder.Len()
		header := "# " + section.title + "\n"
		builder.WriteString(header)
		for _, key := range section.keys {
			writeKey(key)
		}
		if builder.Len() == start+len(header) {
			// Drop an empty section header.
			current := builder.String()[:start]
			builder.Reset()
			builder.WriteString(current)
			continue
		}
		builder.WriteString("\n")
	}
	rest := make([]string, 0)
	for key := range value {
		if !used[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	if len(rest) > 0 {
		builder.WriteString("# 其他\n")
		for _, key := range rest {
			body := renderChatMarkdownValue(value[key], 0)
			if strings.TrimSpace(body) == "" {
				continue
			}
			builder.WriteString("## " + firstString(chatMarkdownSubtitles[key], key) + "\n" + body)
		}
	}
	return strings.TrimSpace(builder.String())
}

func renderChatTimeView(view map[string]any) string {
	var builder strings.Builder
	if local := stringValue(view["actor_self_local_time"]); local != "" {
		zone := stringValue(view["reference_timezone"])
		line := "- 你这边：" + chatMarkdownScalar(local)
		if zone != "" {
			line += "（" + zone + "）"
		}
		builder.WriteString(line + "\n")
	}
	if remote := stringValue(view["actor_user_local_time"]); remote != "" {
		line := "- 对方那边：" + chatMarkdownScalar(remote)
		if zone := stringValue(view["actor_user_timezone"]); zone != "" {
			line += "（" + zone + "）"
		}
		builder.WriteString(line + "\n")
	}
	rest := cloneMap(view)
	for _, key := range []string{"actor_self_local_time", "actor_user_local_time", "actor_user_timezone"} {
		delete(rest, key)
	}
	builder.WriteString(renderChatMarkdownValue(rest, 0))
	return builder.String()
}

func renderChatWardrobe(value any) string {
	items, _ := chatMarkdownList(value)
	if len(items) == 0 {
		return "- 衣柜里还没有记录任何衣物\n"
	}
	byCategory := map[string][]string{}
	categories := []string{}
	for _, raw := range items {
		item := mapValue(raw)
		description := strings.TrimSpace(stringValue(item["description"]))
		if description == "" {
			continue
		}
		switch stringValue(item["availability"]) {
		case "unavailable":
			description += "（暂时不能穿）"
		case "lost":
			description += "（丢了）"
		}
		category := firstString(stringValue(item["category"]), "其他")
		if _, ok := byCategory[category]; !ok {
			categories = append(categories, category)
		}
		byCategory[category] = append(byCategory[category], description)
	}
	sort.Strings(categories)
	var builder strings.Builder
	for _, category := range categories {
		builder.WriteString("- " + category + "：" + strings.Join(byCategory[category], "、") + "\n")
	}
	return builder.String()
}

func renderChatActor(actor map[string]any) string {
	rest := cloneMap(actor)
	name := firstString(stringValue(rest["display_name"]), stringValue(rest["name"]))
	delete(rest, "display_name")
	delete(rest, "name")
	kind := stringValue(rest["type"])
	if kind == "human" || kind == "fluctlight" {
		delete(rest, "type")
	}
	line := "- " + firstString(name, "未知")
	if kind == "human" {
		line += "（真人）"
	}
	return line + "\n" + renderChatMarkdownValue(rest, 1)
}
