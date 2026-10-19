package browser

import (
	"context"
	"net/http"
	"net/url"
)

type diagnosticExportBackend interface {
	ExportDiagnostics(context.Context, string, url.Values, http.ResponseWriter) error
}
type diagnosticExportResponse struct {
	http.ResponseWriter
	started bool
}

func (w *diagnosticExportResponse) Write(p []byte) (int, error) {
	w.started = true
	return w.ResponseWriter.Write(p)
}
func (w *diagnosticExportResponse) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
func (s *Server) diagnosticStreamExport(w http.ResponseWriter, r *http.Request) {
	session, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	backend, ok := s.backend.(diagnosticExportBackend)
	if !ok {
		writeError(w, 503, "diagnostic_export_unavailable", "诊断流式导出服务不可用")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="fluctlight-diagnostics.jsonl"`)
	w.Header().Set("Cache-Control", "no-store")
	tracked := &diagnosticExportResponse{ResponseWriter: w}
	if err := backend.ExportDiagnostics(r.Context(), session, r.URL.Query(), tracked); err != nil && !tracked.started {
		s.writeSurfaceError(w, err)
	}
}
