package core

import (
	"context"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

// A chat photo declares who is in frame, so the plan needs no subject-parse
// model call: the only model call left is the one prompt pass.
func TestDeclaredInFrameSkipsTheSubjectParseCall(t *testing.T) {
	cases := []struct {
		capture map[string]any
		want    map[string]any
	}{
		{map[string]any{"in_frame": "me", "mode": "selfie", "framing": "upper_body"}, map[string]any{"self_in_frame": "yes", "human_presence": "present", "visible_scope": "face", "capture_method": "handheld_selfie"}},
		{map[string]any{"in_frame": "me", "framing": "full_body"}, map[string]any{"self_in_frame": "yes", "visible_scope": "full"}},
		{map[string]any{"in_frame": "me_with_others", "mode": "external_capture"}, map[string]any{"self_in_frame": "yes", "primary_subject": "person"}},
		{map[string]any{"in_frame": "others"}, map[string]any{"self_in_frame": "no", "human_presence": "present"}},
		{map[string]any{"in_frame": "nobody", "framing": "scene"}, map[string]any{"self_in_frame": "no", "human_presence": "none", "primary_subject": "landscape"}},
		{map[string]any{"mode": "mirror_selfie"}, map[string]any{"self_in_frame": "yes", "viewpoint": "reflection"}},
	}
	for _, tc := range cases {
		explicit := explicitMediaPlan(map[string]any{"capture": tc.capture})
		parse := func(context.Context, map[string]any) (map[string]any, error) {
			t.Fatalf("capture %v still called the subject parser", tc.capture)
			return nil, nil
		}
		plan, _, err := ResolveMediaCapturePlan(context.Background(), nil, decision.Scope{}, "照片", explicit, parse)
		if err != nil {
			t.Fatalf("capture %v: %v", tc.capture, err)
		}
		for key, value := range tc.want {
			if plan[key] != value {
				t.Fatalf("capture %v: %s=%v, want %v (plan %v)", tc.capture, key, plan[key], value, plan)
			}
		}
	}
}

func TestDeclaredInFrameNeverOverridesAnExplicitPlan(t *testing.T) {
	explicit := explicitMediaPlan(map[string]any{"media_plan": map[string]any{"self_in_frame": "no"}, "capture": map[string]any{"in_frame": "me"}})
	if explicit["self_in_frame"] != "no" {
		t.Fatalf("in_frame overrode media_plan: %v", explicit)
	}
}

func TestChatPhotoToolAsksWhoIsInFrameAndForExpression(t *testing.T) {
	def := chatToolDefinition(imageCapabilityDefinition())
	if !strings.Contains(jsonString(def.InputSchema["required"]), "capture") {
		t.Fatalf("chat photo tool must require capture: %v", def.InputSchema["required"])
	}
	capture := mapValue(mapValue(def.InputSchema["properties"])["capture"])
	if !strings.Contains(jsonString(capture["required"]), "in_frame") {
		t.Fatalf("capture must require in_frame: %v", capture)
	}
	if !strings.Contains(jsonString(def.InputSchema), "expression") {
		t.Fatal("intent should ask for the facial expression")
	}
	full := mapValue(mapValue(mapValue(imageCapabilityInputSchema()["properties"])["capture"])["properties"])
	if _, ok := full["in_frame"]; !ok {
		t.Fatal("full schema must accept capture.in_frame so chat arguments validate")
	}
}

func TestMediaPromptPassKeepsTheExpression(t *testing.T) {
	for _, phrase := range []string{"表情", "不要写成中性", "不能压过表情"} {
		if !strings.Contains(mediaNormalizationInstruction, phrase) {
			t.Fatalf("normalizer instruction lost %q", phrase)
		}
	}
}
