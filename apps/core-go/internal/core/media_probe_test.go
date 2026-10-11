package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This probe never calls a publication or state-changing Tool. Stub is the
// default; a real Provider and actual Comfy execution are independent opt-ins.
func TestMediaNormalizationProbe(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_MEDIA_PROBE") != "1" {
		t.Skip("set FLUCTLIGHT_MEDIA_PROBE=1 for the independent normalization probe")
	}
	mode := os.Getenv("FLUCTLIGHT_MEDIA_MODEL_MODE")
	if mode == "" {
		mode = "stub"
	}
	if mode != "stub" && mode != "live" {
		t.Fatal("FLUCTLIGHT_MEDIA_MODEL_MODE must be stub or live")
	}
	var app *App
	var spy *formalAgentE2ESpy
	var runCtx = context.Background()
	if mode == "live" {
		fixture := newFormalAgentE2EFixture(t)
		app = fixture.app
		spy = fixture.spy
		runCtx = fixture.ctx
	}
	cases := mediaCases(t)
	selected := os.Getenv("FLUCTLIGHT_MEDIA_CASE")
	if selected == "" {
		selected = "M01"
	}
	var fixture mediaCase
	found := false
	for _, row := range cases {
		if row.ID == selected {
			fixture = row
			found = true
		}
	}
	if !found || fixture.Error != "" {
		t.Fatal("choose a supported non-ambiguous FLUCTLIGHT_MEDIA_CASE")
	}
	if intent := os.Getenv("FLUCTLIGHT_MEDIA_INTENT"); intent != "" {
		fixture.Intent = intent
		if app == nil {
			t.Fatal("custom intent requires live parsing")
		}
		parsed, err := app.parseMediaPlan(runCtx, map[string]any{"intent_text": intent})
		if err != nil {
			t.Fatal(err)
		}
		fixture.Plan = parsed
	}
	state := mediaTestContext("CURRENT")
	if file := os.Getenv("FLUCTLIGHT_MEDIA_STATE_FILE"); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snapshot := decodeObject(raw)
		resolved, err := NewSnapshotContextResolver(snapshot).Resolve(runCtx, ContextRequest{}, []ContextSlot{SlotAppearance, SlotVisualIdentity, SlotCurrentLife})
		if err != nil {
			t.Fatal(err)
		}
		state = resolved
	}
	concept := mediaPreparedConcept(t, fixture.Intent, fixture.Plan, state)
	packet, err := mediaNormalizationInput(concept)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	repairs := 0
	if mode == "live" {
		result, err := app.runBranchedMediaPrompt(WithProviderCorrelation(runCtx, "media-probe:"+fixture.ID), MediaPromptTaskInput{Intent: mediaIntent{ID: "probe_" + fixture.ID, Kind: "image", Prompt: jsonString(concept)}}, concept)
		if err != nil {
			t.Fatal(err)
		}
		output = result.Normalized
		repairs = result.RepairCount
	} else {
		output, repairs, err = normalizeMediaPrompt(runCtx, concept, func(context.Context, map[string]any) (map[string]any, error) { return mediaTestOutput(concept), nil })
		if err != nil {
			t.Fatal(err)
		}
	}
	concept["normalized_output"] = output
	report := map[string]any{"case": fixture.ID, "mode": mode, "intent": fixture.Intent, "capture_plan": fixture.Plan, "model_input": packet, "output": output, "repair_count": repairs, "image_execution": "not_requested"}
	if spy != nil {
		report["actual_model_requests"] = spy.snapshot()
	}
	config := map[string]any{"workflow": map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}}
	if file := os.Getenv("FLUCTLIGHT_MEDIA_COMFY_CONFIG_FILE"); file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		config = decodeObject(raw)
	}
	workflow := selectComfyWorkflow(config, jsonString(concept), mapValue(config["workflow"]))
	if err := validateMediaWorkflowPolicy(config, concept, workflow); err != nil {
		t.Fatal(err)
	}
	if mediaWorkflowNeedsVisualIdentityReference(workflow) {
		t.Fatal("probe workflow must not reference private identity assets; use an isolated workflow or the existing production Worker fixture")
	}
	workflow, err = replaceMediaPlaceholders(workflow, stringValue(output["prompt"]), mediaRendererConstraints(concept))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFinalMediaWorkflow(concept, workflow, stringValue(output["prompt"])); err != nil {
		t.Fatal(err)
	}
	report["final_workflow"] = workflow
	directory := os.Getenv("FLUCTLIGHT_MEDIA_REPORT_DIR")
	if directory == "" {
		directory = "/tmp/fluctlight-media-probe"
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("FLUCTLIGHT_MEDIA_COMFY_EXECUTE") == "1" {
		if mode != "live" {
			t.Fatal("real image execution requires live model mode")
		}
		base, _, err := comfyConfig(config)
		if err != nil {
			t.Fatal(err)
		}
		job, file := executeMediaProbeComfy(t, runCtx, base, workflow, directory, fixture.ID)
		report["image_execution"] = "generated_for_manual_review"
		report["provider_job_id"] = job
		report["image_file"] = file
	}
	raw, _ := json.MarshalIndent(report, "", "  ")
	file := filepath.Join(directory, fixture.ID+"-"+mode+".json")
	if err := os.WriteFile(file, raw, 0600); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("Media normalization report: %s\n", file)
}

func executeMediaProbeComfy(t *testing.T, parent context.Context, base string, workflow map[string]any, directory, id string) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	body := jsonBytes(map[string]any{"prompt": workflow})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/prompt", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	response.Body.Close()
	if err != nil || response.StatusCode != 200 {
		t.Fatal("Comfy submit failed")
	}
	job := stringValue(decodeObject(raw)["prompt_id"])
	if job == "" {
		t.Fatal("missing prompt_id")
	}
	for {
		output, done, err := pollComfy(ctx, base, job)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			filename := stringValue(output["filename"])
			query := url.Values{"filename": {filename}, "subfolder": {stringValue(output["subfolder"])}, "type": {firstString(output["type"], "output")}}
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/view?"+query.Encode(), nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
			response.Body.Close()
			if err != nil || response.StatusCode != 200 {
				t.Fatal("Comfy image fetch failed")
			}
			extension := strings.ToLower(filepath.Ext(filename))
			if extension != ".png" && extension != ".jpg" && extension != ".webp" {
				extension = ".png"
			}
			file := filepath.Join(directory, id+"-"+stableDigest(job)[:12]+extension)
			if err := os.WriteFile(file, data, 0600); err != nil {
				t.Fatal(err)
			}
			return job, file
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
