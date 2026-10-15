package core

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"
)

// Only an explicit Owner action invokes this bounded image-input probe. Reading
// settings never warms a model or sends an image.
func (a *App) TestVisionUnderstanding(ctx context.Context, actor string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return nil, err
	}
	vision := a.resolveVisionCapability(ctx)
	result := map[string]any{"state": vision.State, "config_version": vision.Version, "image_input_verified": false, "error_code": vision.Reason}
	if vision.State != "ready" {
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	ctx = withFrozenProviderAssignment(isolatedReplyModelContext(ctx), visionUnderstandingModelRole, vision.Assignment)
	ctx = WithProviderScenario(WithProviderCorrelation(ctx, randomID("vision-probe:")), "vision_explicit_test")
	imageBytes, expected := visionProbeImage()
	imageURL, err := mediaQualityImageDataURL("image/png", imageBytes)
	if err != nil {
		return nil, err
	}
	messages := []map[string]any{{"role": "system", "content": "Inspect the supplied real image. Return only the two primary colors as English lowercase names separated by a comma, in left-to-right order. No explanation."}, {"role": "user", "content": []any{map[string]any{"type": "text", "text": "What are the two colors?"}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}}}}}
	response, err := a.Provider.completeWithToolsSchema(ctx, visionUnderstandingModelRole, messages, false, nil, "", nil, false)
	if err != nil {
		result["state"] = "unavailable"
		result["error_code"] = visionFailureReason(err)
		return result, nil
	}
	observed := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(response.Text)), " ", "")
	if observed != expected {
		result["state"] = "unavailable"
		result["error_code"] = "vision_probe_observation_mismatch"
		return result, nil
	}
	result["image_input_verified"] = true
	result["error_code"] = ""
	return result, nil
}
func visionProbeImage() ([]byte, string) {
	palette := []struct {
		name  string
		value color.RGBA
	}{{"red", color.RGBA{255, 0, 0, 255}}, {"blue", color.RGBA{0, 0, 255, 255}}, {"green", color.RGBA{0, 255, 0, 255}}}
	identity := stableDigest(randomID("vision_image_"))
	left := int(identity[0]) % 3
	right := (left + 1 + int(identity[1])%2) % 3
	picture := image.NewRGBA(image.Rect(0, 0, 128, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 128; x++ {
			index := left
			if x >= 64 {
				index = right
			}
			picture.SetRGBA(x, y, palette[index].value)
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, picture)
	return buffer.Bytes(), palette[left].name + "," + palette[right].name
}
