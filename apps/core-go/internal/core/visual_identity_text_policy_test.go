package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVisualIdentityImagePromptOmitsIdentityMetadataAcrossSnapshotShapes(t *testing.T) {
	for _, age := range []any{37, "37岁", "37 years old"} {
		snapshot := map[string]any{
			"name": "根级姓名",
			"identity": map[string]any{
				"name": "身份姓名", "age": age, "gender": "女性", "nationality": "中国",
			},
			"life_profile": map[string]any{"appearance": map[string]any{
				"age": age, "hair": "黑色短发", "eyes": "棕色眼睛", "outfit": "蓝色夹克",
			}},
		}
		for name, input := range map[string]map[string]any{
			"direct":            snapshot,
			"identity_snapshot": {"identity_snapshot": snapshot},
			"visual_identity":   {"visual_identity": snapshot},
		} {
			t.Run(name+"/"+firstVisualIdentityString(age), func(t *testing.T) {
				before := jsonString(input)
				prompt := visualIdentityCharacterDescription(input)
				for _, forbidden := range []string{"根级姓名", "身份姓名", "37", "角色："} {
					if strings.Contains(prompt, forbidden) {
						t.Errorf("image description includes identity metadata %q: %s", forbidden, prompt)
					}
				}
				for _, required := range []string{"女性", "中国", "黑色短发", "棕色眼睛", "蓝色夹克"} {
					if !strings.Contains(prompt, required) {
						t.Errorf("image description lost visual trait %q: %s", required, prompt)
					}
				}
				if jsonString(input) != before {
					t.Fatal("image projection mutated the source snapshot")
				}
			})
		}
	}
}

func TestVisualIdentityTemplateDoesNotInventInformationPanelsOrAccessories(t *testing.T) {
	prompt := visualIdentityPromptFromConcept(map[string]any{"visual_identity": map[string]any{
		"identity_snapshot": map[string]any{"life_profile": map[string]any{"appearance": map[string]any{"description": "短发，蓝色夹克"}}},
	}})
	for _, forbidden := range []string{"人物基础资料视觉区", "人物简介视觉区", "性格关键词视觉区", "角色签名视觉区", "空白信息栏", "空白签名线", "领结", "校徽"} {
		if strings.Contains(prompt, forbidden) || strings.Contains(visualIdentityRequiredCardSectionsText, forbidden) {
			t.Errorf("template invents requirement %q: %s", forbidden, prompt)
		}
	}
	// Actual appearance prose remains opaque, including explicitly worn details.
	const prose = "短发，蓝色夹克，佩戴领结和校徽"
	description := visualIdentityCharacterDescription(map[string]any{"life_profile": map[string]any{"appearance": prose}})
	if description != prose {
		t.Fatalf("explicit appearance was rewritten: %s", description)
	}
}

func TestVisualIdentityMediaPhysicalRequestsOmitInformationPanels(t *testing.T) {
	for _, stage := range []string{"seed", "character_sheet"} {
		t.Run(stage, func(t *testing.T) {
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				wire := jsonString(payload["messages"])
				for _, required := range []string{"Do not add information panels, character introductions, personality keywords or signature areas", "front/side/back model views", "six expressions", "accessories matching actual wear", "Do not render any text"} {
					if !strings.Contains(wire, required) {
						t.Errorf("physical %s prompt lost %q", stage, required)
					}
				}
				for _, forbidden := range []string{"visual sections for basic information", "and a signature area", "blank fields instead"} {
					if strings.Contains(wire, forbidden) {
						t.Errorf("physical %s prompt retains %q", stage, forbidden)
					}
				}
				return fakeProviderResult{Text: "text-free character sheet"}
			})
			messages := formatProviderMessagesForRole(addVisualIdentityMediaPromptInstruction("media_prompt", []map[string]any{
				{"role": "system", "content": mediaPromptSystemInstruction(mediaIntent{})},
				{"role": "user", "content": jsonString(map[string]any{"purpose": "visual_identity", "stage": stage})},
			}), "media_prompt")
			_, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(context.Background(), EinoModelCall{
				Assignment: providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "controlled-text", Timeout: time.Second},
				Role:       "media_prompt", Messages: formalPromptMessages(FormalAgentMediaPrompt, messages),
			})
			if err != nil || router.totalRequests() != 1 {
				t.Fatalf("physical %s prompt err=%v requests=%d", stage, err, router.totalRequests())
			}
		})
	}
}

func TestVisualIdentityReviewPhysicalRequestsRejectText(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "visual-text-policy-endpoint")
	enableControlledVision(t, ctx, repository)
	pixels, _ := visionProbeImage()
	imageURL, err := mediaQualityImageDataURL("image/png", pixels)
	if err != nil {
		t.Fatal(err)
	}
	imageContent := map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}}
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire := jsonString(payload["messages"])
		for _, required := range []string{"Reject any visible text", "pseudo-text", "letters", "signatures", "including incidental or unreadable characters", "Missing text, information panels or signature areas is never a defect"} {
			if !strings.Contains(wire, required) {
				t.Errorf("physical review prompt lost %q: %s", required, wire)
			}
		}
		for _, forbidden := range []string{"must not by themselves", "Regenerate only", "角色签名视觉区", "人物基础资料视觉区", "领结", "校徽"} {
			if strings.Contains(wire, forbidden) {
				t.Errorf("physical review prompt retains %q", forbidden)
			}
		}
		for _, key := range []string{"max_tokens", "max_completion_tokens"} {
			if _, exists := payload[key]; exists {
				t.Error("unexpected output token limit", key)
			}
		}
		if strings.Contains(wire, "Return regenerate whenever vision reports") {
			return fakeProviderResult{Structured: map[string]any{
				"decision": "regenerate", "summary": "Visible signature", "prompt_patch": map[string]any{},
				"renderer_constraints": map[string]any{}, "feedback": "Remove the signature", "confidence": 1.0,
			}}
		}
		if !strings.Contains(wire, imageURL) || !strings.Contains(wire, "report a low identity_match and describe the text defect") {
			t.Error("vision request lost actual pixels or text rejection rule")
		}
		return fakeProviderResult{Structured: map[string]any{
			"summary": "Visible signature", "observations": map[string]any{"text": "unreadable signature"}, "identity_match": 0.0, "confidence": 1.0,
		}}
	})
	app := newTestApp(t, repository, router)
	vision, err := app.RunVisualIdentityVisionTask(ctx, VisualIdentityVisionTaskInput{
		CandidateAssetID: "text-policy-candidate", ImageContent: imageContent,
	})
	if err != nil || vision["identity_match"] != float64(0) {
		t.Fatalf("vision result=%#v err=%v", vision, err)
	}
	patch, err := app.RunVisualIdentityPatchTask(ctx, VisualIdentityPatchTaskInput{
		CandidateAssetID: "text-policy-candidate", Vision: vision,
	})
	if err != nil || patch["decision"] != "regenerate" || router.totalRequests() != 2 {
		t.Fatalf("patch result=%#v err=%v requests=%d", patch, err, router.totalRequests())
	}
}

func TestVisualIdentityAgentPhysicalReviewIncludesTextRejection(t *testing.T) {
	pixels, _ := visionProbeImage()
	imageURL, err := mediaQualityImageDataURL("image/png", pixels)
	if err != nil {
		t.Fatal(err)
	}
	imageContent := map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}}
	state := visualIdentityAgentState{Attempt: 1, ActionRequired: visualIdentityCommitReviewCapabilityName, CandidateAssetReady: true}
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire := jsonString(payload["messages"])
		for _, required := range []string{imageURL, "Reject any visible text", "including incidental or unreadable characters", "choose regenerate and describe the text defect", "Missing text, information panels or signature areas is never a defect", "front/side/back full-body views", "six expressions"} {
			if !strings.Contains(wire, required) {
				t.Errorf("physical Agent review lost %q", required)
			}
		}
		if strings.Contains(wire, "intro/personality/signature visual areas") {
			t.Error("Agent still requires nonvisual identity panels")
		}
		return fakeProviderResult{Text: "reviewed"}
	})
	ctx := context.WithValue(context.Background(), visionSettingsReaderKey{}, func(context.Context) (VisionUnderstandingSettings, error) {
		return VisionUnderstandingSettings{Enabled: true, ModelRef: visionUnderstandingModelRole}, nil
	})
	_, err = (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
		Assignment: providerAssignment{Role: visionUnderstandingModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-vision", Timeout: time.Second},
		Role:       "visual_identity", Messages: formalPromptMessages(FormalAgentVisualIdentity, visualIdentityAgentMessages(state, imageContent)),
	})
	if err != nil || router.totalRequests() != 1 {
		t.Fatalf("Agent physical review err=%v requests=%d", err, router.totalRequests())
	}
}
