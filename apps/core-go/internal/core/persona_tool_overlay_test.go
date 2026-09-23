package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPersonaToolComposesAcceptedOverlayAndRejectsCorruption(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, fluctlight, conversation := "overlay-tool-owner", "overlay-tool-fluctlight", "overlay-tool-conversation"
	takeoverChainSeed(t, ctx, repo, owner, fluctlight, conversation)
	profileRef := "personality:ctx_" + stableDigest(fluctlight+"\x1f"+"twilight")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_evolution_states(fluctlight_id,profile_id,profile_ref,revision,domain_revisions) VALUES($1,'twilight',$2,1,'{"personality":1}')`, fluctlight, profileRef); err != nil {
		t.Fatal(err)
	}
	overlayRef := "evolution_overlay:ctx_" + stableDigest(fluctlight + "\x1f" + "openness")[:32]
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_evolution_overlays(id,ref,fluctlight_id,profile_id,kind,field_path,value_kind,semantic_direction,requested_delta,applied_delta,before_value,after_value,confidence,evidence_refs,evidence_windows,policy_version,base_revision,revision,status,supersedes,rollback_of,cooldown_until,created_at) VALUES('overlay-tool-openness',$2,$1,'twilight','personality','traits.openness','numeric','increase',0.25,0.25,'0.3','0.55',0.9,'[]','[]','reflection.policy.v2',0,1,'active',NULL,NULL,now(),now())`, fluctlight, overlayRef); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	seedLegacyTestWorkingPersonas(t, app)
	request := ToolExecutionRequest{CapabilityName: personaTakeoverCapabilityName, OperationID: "overlay-takeover", AuthorizationActorID: owner, FluctlightID: fluctlight, EvidenceID: "explicit-owner-operation", Arguments: json.RawMessage(`{"decision":"takeover_b","source_profile_id":"spark","target_profile_id":"twilight","rule_id":"public-doubt"}`)}
	result, err := app.ExecuteTool(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	working := mapValue(mapValue(result.Result.Output)["working_persona"])
	if got := jsonString(working); !strings.Contains(got, "0.55") || len(mapValue(working["personality"])) != 0 {
		t.Fatalf("committed persona receipt ignored accepted overlay: %#v", working)
	}
	if readActiveProfileForGate(t, ctx, repo, fluctlight) != "spark" {
		t.Fatal("takeover mutated persistent profile")
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlight_evolution_states SET profile_ref='corrupt' WHERE fluctlight_id=$1 AND profile_id='twilight'`, fluctlight); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "corrupt-overlay-takeover"
	if _, err := app.ExecuteTool(ctx, request); err == nil {
		t.Fatal("corrupt overlay silently reverted to baseline")
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND operation_id=$2`, fluctlight, request.OperationID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("failed overlay committed receipt count=%d err=%v", count, err)
	}
}
