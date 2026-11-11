package core

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// This test requires the isolated PostgreSQL fixture. It does not call a real
// model or image renderer; accepted intents stay pending in the test database.
func TestDomainPluginPostgresSettingsAdmissionAndReplay(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "plugin-admission")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "legacy")
	moment := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "plugin-existing-moment", AuthorizationActorID: owner, FluctlightID: fid, Arguments: json.RawMessage(`{"text":"已发布的文字动态"}`)}
	image := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "plugin-existing-image", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
	firstMoment, err := app.ExecuteTool(ctx, moment)
	if err != nil {
		t.Fatal(err)
	}
	firstImage, err := app.ExecuteTool(ctx, image)
	if err != nil {
		t.Fatal(err)
	}
	momentID := stringValue(mapValue(firstMoment.Result.Output)["target_ref"])
	imageID := stringValue(mapValue(firstImage.Result.Output)["media_intent_id"])
	if momentID == "" || imageID == "" {
		t.Fatalf("missing accepted resources: %#v %#v", firstMoment, firstImage)
	}

	invalid := map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": false}}, "expected_versions": map[string]any{}, "idempotency_key": "invalid-plugin-settings"}
	_, err = app.UpdateSettings(ctx, owner, invalid)
	var surface *SurfaceError
	if !errors.As(err, &surface) || surface.Status != 422 || surface.Code != "domain_plugins_config_invalid" || surface.Details["configuration_admission"] != "rejected" {
		t.Fatalf("invalid configuration was not rejected before admission: %#v %v", surface, err)
	}
	settings := func(image, moment bool, version int, key string) map[string]any {
		return map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": image, "moment": moment}}, "expected_versions": map[string]any{domainPluginsSettingKey: version}, "idempotency_key": key}
	}
	if _, err := app.UpdateSettings(ctx, owner, settings(false, false, 0, "plugins-off")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.UpdateSettings(ctx, owner, settings(false, false, 0, "plugins-off")); err != nil {
		t.Fatalf("configuration receipt was not replayed: %v", err)
	}
	if _, err := app.UpdateSettings(ctx, owner, settings(true, true, 0, "plugins-stale")); err == nil {
		t.Fatal("stale configuration version was accepted")
	}
	if _, err := app.CommentMoment(ctx, owner, momentID, "停用后不能新增评论"); err == nil {
		t.Fatal("legacy comment endpoint bypassed Moment disable")
	}
	if _, err := app.ReactMoment(ctx, owner, momentID, "like"); err == nil {
		t.Fatal("legacy reaction endpoint bypassed Moment disable")
	}

	for _, sample := range []struct {
		request   ToolExecutionRequest
		field, id string
	}{{moment, "target_ref", momentID}, {image, "media_intent_id", imageID}} {
		replay, err := app.ExecuteTool(ctx, sample.request)
		if err != nil || !toolBoolValue(mapValue(replay.Result.Output)["replayed"]) || stringValue(mapValue(replay.Result.Output)[sample.field]) != sample.id {
			t.Fatalf("accepted tool could not replay after disable: %#v %v", replay, err)
		}
		sample.request.OperationID += "-new"
		denied, err := app.ExecuteTool(ctx, sample.request)
		var capability *CapabilityError
		if !errors.As(err, &capability) || capability.Code != "domain_plugin_disabled" || denied.Result.ErrorCode != "domain_plugin_disabled" {
			t.Fatalf("new disabled tool was not rejected: %#v %v", denied, err)
		}
	}
	text := ToolExecutionRequest{CapabilityName: "conversation.reply", OperationID: "plugin-text-only", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"普通文字仍可发送"}`)}
	if receipt, err := app.ExecuteTool(ctx, text); err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("ordinary chat was affected: %#v %v", receipt, err)
	}
	if _, err := app.UpdateSettings(ctx, owner, settings(false, true, 1, "moment-on")); err != nil {
		t.Fatal(err)
	}
	moment.OperationID = "plugin-text-moment-without-image"
	moment.Arguments = json.RawMessage(`{"text":"图片停用时的文字动态"}`)
	if receipt, err := app.ExecuteTool(ctx, moment); err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("text-only Moment depended on Image: %#v %v", receipt, err)
	}
	var momentCount, imageCount int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE id=$1`, momentID).Scan(&momentCount); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE id=$1`, imageID).Scan(&imageCount); err != nil {
		t.Fatal(err)
	}
	if momentCount != 1 || imageCount != 1 {
		t.Fatalf("replay duplicated or deleted existing resources: moment=%d image=%d", momentCount, imageCount)
	}
}

func TestDomainPluginCatalogReportsSavedRendererConfiguration(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "plugin-health")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.runtime_settings WHERE key='media.comfyui'`); err != nil {
		t.Fatal(err)
	}
	read := func() map[string]any {
		t.Helper()
		catalog, err := app.OwnerConfigurationCatalog(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range arrayValue(catalog["domain_plugins"]) {
			if plugin := mapValue(raw); plugin["id"] == "image" {
				return plugin
			}
		}
		t.Fatal("Image plugin missing from saved catalog")
		return nil
	}
	if image := read(); image["health"] != "unconfigured" || image["error_code"] != "media_not_configured" {
		t.Fatalf("missing renderer configuration not reported: %#v", image)
	}
	config := map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{"media.comfyui": config}, "expected_versions": map[string]any{"media.comfyui": 0}, "idempotency_key": "plugin-health-save"}); err != nil {
		t.Fatal(err)
	}
	if image := read(); image["health"] != "not_checked" || image["error_code"] != "" {
		t.Fatalf("saved configuration was not reread or claimed untested health: %#v", image)
	}
}

// A fresh Agent trace can replay the same durable operation after disable;
// a different operation must instead return a disabled ToolResult.
func TestDomainPluginPostgresNativeAgentReceiptReplay(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "plugin-native-replay")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "legacy")
	definitions := app.capabilityRegistry().Catalog(CapabilitySurfaceConversation)
	invoke := func(root, callID string) CapabilityResult {
		t.Helper()
		selection := &kevRunSelection{original: definitions, loaded: map[string]bool{}}
		callCtx, err := app.withDomainPluginScope(ctx)
		if err != nil {
			t.Fatal(err)
		}
		callCtx = context.WithValue(callCtx, kevRunSelectionKey{}, selection)
		trace := &ADKCapabilityTrace{}
		trace.RecordModelToolCalls("controlled-native-request", 1, []schema.ToolCall{{ID: callID, Function: schema.FunctionCall{Name: "moment.publish"}}})
		invoker := newAppADKCapabilityInvokerForContext(callCtx, app, ADKCapabilityRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, OperationID: root, Surface: CapabilitySurfaceConversation, Projection: ContextProjection{OwnerActorID: owner, FluctlightID: fid}}, trace).(*appADKCapabilityInvoker)
		if _, err := invoker.ExecuteWithID(callCtx, callID, "moment.publish", `{"text":"同一次动态操作"}`); err != nil {
			t.Fatal(err)
		}
		result, found := trace.FindResult(callID)
		if !found {
			t.Fatal("native result missing")
		}
		return result
	}
	first := invoke("same-run", "first-call")
	if first.Status != "completed" {
		t.Fatalf("initial native call: %#v", first)
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": true, "moment": false}}, "expected_versions": map[string]any{}, "idempotency_key": "disable-moment-native"}); err != nil {
		t.Fatal(err)
	}
	replayed := invoke("same-run", "resumed-call")
	if replayed.Status != "completed" || !toolBoolValue(mapValue(replayed.Output)["replayed"]) {
		t.Fatalf("native receipt did not replay: %#v", replayed)
	}
	denied := invoke("new-run", "new-call")
	if denied.Status != "failed" || denied.ErrorCode != "domain_plugin_disabled" {
		t.Fatalf("native new operation was not rejected: %#v", denied)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1 AND text='同一次动态操作'`, fid).Scan(&count); err != nil || count != 1 {
		t.Fatalf("native replay count=%d err=%v", count, err)
	}
}
