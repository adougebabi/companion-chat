package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

func (b *browserBackend) UploadOwnerMedia(ctx context.Context, session string, request *http.Request) (map[string]any, error) {
	owner, err := b.resolveActor(ctx, session)
	if err != nil {
		return nil, err
	}
	if b.server.app == nil {
		return nil, &browser.CoreError{Status: 503, Code: "media_service_unavailable", Message: "媒体服务尚未就绪"}
	}
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, &browser.CoreError{Status: 413, Code: "media_upload_size_invalid", Message: "文件超过64MiB上传上限"}
		}
		return nil, &browser.CoreError{Status: 400, Code: "media_upload_multipart_invalid", Message: "无法读取上传表单"}
	}
	if request.MultipartForm == nil {
		return nil, &browser.CoreError{Status: 400, Code: "media_upload_file_required", Message: "请选择实际文件"}
	}
	defer request.MultipartForm.RemoveAll()
	form := request.MultipartForm
	for key := range form.Value {
		if key != "mime" && key != "idempotencyKey" {
			return nil, &browser.CoreError{Status: 400, Code: "media_upload_fields_invalid", Message: "上传包含未知字段"}
		}
	}
	if len(form.File) != 1 || len(form.File["file"]) != 1 || len(form.Value["idempotencyKey"]) != 1 || len(form.Value["mime"]) != 1 {
		return nil, &browser.CoreError{Status: 400, Code: "media_upload_fields_invalid", Message: "需要单个文件、类型和原提交身份"}
	}
	key := strings.TrimSpace(form.Value["idempotencyKey"][0])
	if header := strings.TrimSpace(request.Header.Get("Idempotency-Key")); header != "" && header != key {
		return nil, &browser.CoreError{Status: 422, Code: "media_upload_key_mismatch", Message: "上传身份不一致"}
	}
	header := form.File["file"][0]
	file, err := header.Open()
	if err != nil {
		return nil, &browser.CoreError{Status: 400, Code: "media_upload_file_unreadable", Message: "无法读取所选文件"}
	}
	defer file.Close()
	result, err := b.server.app.UploadOwnerMedia(ctx, owner, core.OwnerMediaUploadInput{Filename: header.Filename, MIME: form.Value["mime"][0], IdempotencyKey: key, Content: file, Size: header.Size})
	if err != nil {
		return nil, browserBackendError(err, "media_upload_failed")
	}
	return result, nil
}
