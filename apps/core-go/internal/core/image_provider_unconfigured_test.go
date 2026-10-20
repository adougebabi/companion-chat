package core

import (
	"errors"
	"strings"
	"testing"
)

func TestImageTaskWithoutComfyUINamesMissingProvider(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.runtime_settings WHERE key='media.comfyui'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.queueStudioMedia(f.ctx, f.ownerID, "studio-job-"+f.suffix, "attempt-"+f.suffix, map[string]any{}); !errors.Is(err, errImageProviderNotConfigured) || err.Error() != "图像 provider 未配置" {
		t.Fatalf("studio image error=%v", err)
	}
	if err := f.app.preflightImageCapability(f.ctx); !errors.Is(err, errImageProviderNotConfigured) || err.Error() != "图像 provider 未配置" {
		t.Fatalf("image preflight error=%v", err)
	}
	intentID := "media-no-provider-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending')`, intentID, f.fluctlightID, `{"intent":"拍一张照片"}`, "provider-"+intentID, "workflow-"+intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessMediaIntent(f.ctx, intentID); !errors.Is(err, errImageProviderNotConfigured) || err.Error() != "图像 provider 未配置" || strings.Contains(err.Error(), "no rows") {
		t.Fatalf("image task error=%v", err)
	}
	visualID := "media-visual-identity-" + f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending')`, visualID, f.fluctlightID, `{"purpose":"visual_identity"}`, "provider-"+visualID, "workflow-"+visualID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessMediaIntent(f.ctx, visualID); err == nil || !strings.Contains(err.Error(), "no rows") || errors.Is(err, errImageProviderNotConfigured) {
		t.Fatalf("visual identity provider error changed: %v", err)
	}
}
