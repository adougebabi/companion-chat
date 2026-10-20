package core

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func (a *App) OwnerConfigurationCatalog(ctx context.Context, owner string) (map[string]any, error) {
	if _, ok := SurfaceDBTx(ctx); !ok {
		return a.configurationSnapshot(ctx, func(ctx context.Context) (map[string]any, error) { return a.OwnerConfigurationCatalog(ctx, owner) })
	}
	saved, err := a.ReadSettings(ctx, owner)
	if err != nil {
		return nil, err
	}
	effective := cloneMap(mapValue(saved["values"]))
	sources := map[string]any{}
	for key := range effective {
		sources[key] = "stored"
	}
	// These are the same runtime defaults, only for genuinely absent settings.
	if _, ok := mapValue(saved["versions"])["conversation.reply"]; !ok {
		sources["conversation.reply"] = "runtime_default"
	}
	if _, ok := mapValue(saved["versions"])["vision.understanding"]; !ok {
		sources["vision.understanding"] = "runtime_default"
	}
	if _, ok := effective["kev"]; !ok {
		effective["kev"] = decision.DefaultConfig()
		sources["kev"] = "runtime_default"
	} else {
		value, err := decodeKevConfig(jsonBytes(effective["kev"]))
		if err != nil {
			return nil, err
		}
		effective["kev"] = value
	}
	autonomy, err := readGlobalAutonomyConfiguration(ctx, a.configurationReader(ctx))
	if err != nil {
		return nil, err
	}
	effective["product.autonomy"] = autonomy
	if _, ok := mapValue(saved["versions"])["product.autonomy"]; !ok {
		sources["product.autonomy"] = "runtime_default"
	}
	queue := normalizeProviderQueueSettings(effective["llm.queue"])
	if queue == nil {
		return nil, errors.New("llm_queue_invalid")
	}
	effective["llm.queue"] = queue
	if _, ok := mapValue(saved["versions"])["llm.queue"]; !ok {
		sources["llm.queue"] = "runtime_default"
	}
	duration, budget, err := runtimeSummarySettingsWith(ctx, a.configurationReader(ctx))
	if err != nil {
		return nil, err
	}
	effective["product.summary"] = map[string]any{"interval_seconds": int(duration / time.Second), "max_runes": budget}
	if _, ok := mapValue(saved["versions"])["product.summary"]; !ok {
		sources["product.summary"] = "runtime_default"
	}
	maximum, base, capSeconds := effectiveIntentionRetryPolicy(mapValue(effective["product.goal_retry"]))
	effective["product.goal_retry"] = map[string]any{"max_attempts": maximum, "base_seconds": base, "max_seconds": capSeconds}
	if _, ok := mapValue(saved["versions"])["product.goal_retry"]; !ok {
		sources["product.goal_retry"] = "runtime_default"
	}
	if _, ok := mapValue(saved["versions"])["product.wakeup"]; !ok {
		sources["product.wakeup"] = "runtime_default"
	}
	if _, ok := effective["media.comfyui"]; !ok {
		sources["media.comfyui"] = "unconfigured"
	}
	if _, ok := effective["product.notifications"]; !ok {
		effective["product.notifications"] = defaultNotificationPreferences()
		sources["product.notifications"] = "runtime_default"
	}
	if raw, ok := effective["media.comfyui"]; ok {
		media := cloneMap(mapValue(raw))
		if media["baseUrl"] == nil {
			media["baseUrl"] = media["base_url"]
		}
		if media["generation_timeout_seconds"] == nil {
			media["generation_timeout_seconds"] = 1200
		}
		effective["media.comfyui"] = media
	}
	endpoints, err := a.ProviderEndpoints(ctx, owner)
	if err != nil {
		return nil, err
	}
	bindings, err := a.ProviderBindings(ctx, owner)
	if err != nil {
		return nil, err
	}
	saved["endpoints"], saved["bindings"] = endpoints, bindings
	saved["provider_budget_defaults"] = map[string]any{"policy_version": promptBudgetPolicyVersionV1, "safety_margin_tokens": defaultPromptSafetyMarginTokens, "output_tokens": defaultOutputReserveTokens}
	saved["effective_values"], saved["value_sources"] = effective, sources
	return saved, nil
}

func (a *App) OwnerRuntimeCatalog(ctx context.Context, owner string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	agents := []any{}
	for _, agent := range FormalAgentDefinitions() {
		agents = append(agents, map[string]any{"id": agent.ID, "name": agent.Name, "description": agent.Description, "role": agent.Role, "default_surface": agent.DefaultSurface, "mandatory_tools": agent.MandatoryTools})
	}
	return map[string]any{"agents": agents, "capabilities": a.capabilityRegistry().Definitions(), "read_only": true, "dynamic_installation": false}, nil
}

func (a *App) OwnerConfigurationCommand(ctx context.Context, owner, key string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if key == "" || len(key) > 256 {
		return nil, invalid("Original configuration command key required")
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2 AND left(response->>'command_id',14)='configuration_'`, owner, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return compactStateMap(decodeObject(raw), []string{"command_id", "resource_keys", "applied_versions", "wake_rearm_required", "status"}), nil
}

func (a *App) RecoverOwnerConfigurationCommand(ctx context.Context, owner, key string) (map[string]any, error) {
	receipt, err := a.OwnerConfigurationCommand(ctx, owner, key)
	if err != nil {
		return nil, err
	}
	if receipt["wake_rearm_required"] == true {
		if _, err := a.EnsureWakeUpIntents(ctx); err != nil {
			return nil, err
		}
		if _, err := a.ReleaseDueWakeUpIntents(ctx, wakeUpDueSweepLimit); err != nil {
			return nil, err
		}
	}
	return receipt, nil
}

func validateComfySetting(raw any) error {
	value, ok := raw.(map[string]any)
	if !ok {
		return invalid("媒体配置必须是对象")
	}
	if _, err := mediaGenerationTimeoutSeconds(value); err != nil {
		return err
	}
	base, workflow, err := comfyConfig(value)
	if err != nil {
		return invalid("请填写 ComfyUI 地址及非空 API 格式工作流")
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return invalid("ComfyUI 地址必须是无嵌入凭据的 HTTP(S) 地址")
	}
	if err := validateComfyGraph(workflow); err != nil {
		return err
	}
	if v, exists := value["visual_identity_workflow"]; exists {
		if err := validateComfyGraph(mapValue(v)); err != nil {
			return err
		}
	}
	for _, key := range []string{"visual_identity_workflows", "media_workflows"} {
		for _, graph := range mapValue(value[key]) {
			if err := validateComfyGraph(mapValue(graph)); err != nil {
				return err
			}
		}
	}
	return nil
}

func mediaGenerationTimeoutSeconds(config map[string]any) (int, error) {
	raw, exists := config["generation_timeout_seconds"]
	if !exists {
		return 1200, nil
	}
	seconds, ok := intValueExact(raw)
	if !ok || seconds < 30 || seconds > 1200 {
		return 0, invalid("媒体执行等待上限应为30–1200秒")
	}
	return seconds, nil
}
func validateComfyGraph(graph map[string]any) error {
	if len(graph) == 0 {
		return invalid("工作流必须是非空的 ComfyUI API 节点对象")
	}
	for _, raw := range graph {
		node, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(stringValue(node["class_type"])) == "" {
			return invalid("工作流缺少 class_type，请导出 ComfyUI API 格式")
		}
		if _, ok := node["inputs"].(map[string]any); !ok {
			return invalid("工作流节点的 inputs 必须是对象")
		}
	}
	return nil
}

func (a *App) TestSavedMediaConnection(ctx context.Context, owner string) (map[string]any, error) {
	saved, err := a.ReadSettings(ctx, owner)
	if err != nil {
		return nil, err
	}
	raw, ok := mapValue(saved["values"])["media.comfyui"]
	if !ok {
		return map[string]any{"state": "unconfigured", "config_version": 0, "generation_tested": false}, nil
	}
	if err := validateComfySetting(raw); err != nil {
		return nil, err
	}
	base, _, _ := comfyConfig(mapValue(raw))
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, base+"/system_stats", nil)
	if err != nil {
		return nil, err
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		return nil, &SurfaceError{Status: 502, Code: "media_connection_failed", Message: "已保存 ComfyUI 地址连接失败"}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &SurfaceError{Status: 502, Code: "media_connection_failed", Message: "ComfyUI 状态接口未成功返回"}
	}
	var result map[string]any
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return nil, &SurfaceError{Status: 502, Code: "media_connection_invalid", Message: "ComfyUI 状态响应不是有效 JSON"}
	}
	if result["system"] == nil {
		return nil, &SurfaceError{Status: 502, Code: "media_connection_invalid", Message: "响应缺少 ComfyUI 系统状态"}
	}
	return map[string]any{"state": "reachable", "config_version": mapValue(saved["versions"])["media.comfyui"], "generation_tested": false, "workflow_structure_valid": true}, nil
}

func defaultNotificationPreferences() map[string]any {
	return map[string]any{"messages": true, "goals": true, "media": true}
}
func validNotificationPreferences(raw any) error {
	value, ok := raw.(map[string]any)
	if !ok || len(value) != 3 {
		return invalid("通知偏好需要明确的消息、目标和媒体开关")
	}
	for _, key := range []string{"messages", "goals", "media"} {
		if _, ok := value[key].(bool); !ok {
			return invalid("通知偏好开关必须是布尔值")
		}
	}
	return nil
}
