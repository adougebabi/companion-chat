package browser

import (
	"errors"
	"net/http"
	"strings"
)

func validConfigurationIdentity(body map[string]any) bool {
	if key := stringValue(body["idempotencyKey"]); strings.TrimSpace(key) != key || key == "" {
		return false
	}
	if !validateString(body["idempotencyKey"], 1, 256) || !isObject(body["expectedVersions"]) {
		return false
	}
	for key, v := range objectValue(body["expectedVersions"]) {
		if len(key) == 0 || len(key) > 256 || !validateInteger(v, 0) {
			return false
		}
	}
	return true
}
func (s *Server) configurationError(w http.ResponseWriter, err error) {
	var typed *CoreError
	if errors.As(err, &typed) && typed.Status == 409 {
		s.writeSurfaceError(w, err)
		return
	}
	if errors.As(err, &typed) && typed.Status == 422 {
		writeError(w, 422, typed.Code, "配置未保存，请核对字段与有效范围")
		return
	}
	s.writeSurfaceError(w, err)
}
