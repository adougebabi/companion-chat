package browser

import (
	"context"
	"net/http"
)

// Multipart remains transport data. The concrete in-process adapter resolves
// the session before parsing file parts or invoking the Core upload service.
type OwnerMediaUploadBackend interface {
	UploadOwnerMedia(context.Context, string, *http.Request) (map[string]any, error)
}

func (s *Server) uploadOwnerMedia(response http.ResponseWriter, request *http.Request, session string) {
	backend, ok := s.backend.(OwnerMediaUploadBackend)
	if !ok {
		writeError(response, 503, "media_upload_unavailable", "媒体上传服务当前不可用")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, (64<<20)+(2<<20))
	result, err := backend.UploadOwnerMedia(request.Context(), session, request)
	if err != nil {
		s.writeSurfaceError(response, err)
		return
	}
	response.Header().Set("Cache-Control", "private, no-store")
	status := http.StatusCreated
	if result["replayed"] == true {
		status = http.StatusOK
	}
	writeJSON(response, status, result)
}
