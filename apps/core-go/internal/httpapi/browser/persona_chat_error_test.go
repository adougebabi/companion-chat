package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type personaRejectedBackend struct {
	Backend
	code string
}

func (b personaRejectedBackend) StreamTurn(context.Context, string, string, map[string]any, http.ResponseWriter) error {
	return &CoreError{Status: http.StatusConflict, Code: b.code, Message: "private compiler detail"}
}

func TestPersonaAdmissionFailureRemainsHTTPConflictWithoutStartingStream(t *testing.T) {
	for _, code := range []string{"persona_preparation_failed", "persona_preparation_pending"} {
		server := &Server{backend: personaRejectedBackend{code: code}}
		response := httptest.NewRecorder()
		err := server.streamTurn(context.Background(), "session", "conversation", map[string]any{}, response)
		var business *CoreError
		if !errors.As(err, &business) || business.Code != code || response.Body.Len() != 0 {
			t.Fatalf("admission became a bogus stream: body=%s err=%v", response.Body.String(), err)
		}
		server.conversationTurnError(response, err)
		var body map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusConflict || body["code"] != code || body["type"] != nil {
			t.Fatalf("HTTP rejection body=%s status=%d err=%v", response.Body.String(), response.Code, err)
		}
	}
}

func TestPersonaPreparationErrorsCrossBrowserHTTPAndNDJSONWithRecoveryCopy(t *testing.T) {
	for _, code := range []string{"persona_preparation_failed", "persona_preparation_pending"} {
		response := httptest.NewRecorder()
		server := &Server{}
		server.conversationTurnError(response, &CoreError{Status: http.StatusConflict, Code: code, Message: "private provider password"})
		var value map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		errorBody := value
		if response.Code != http.StatusConflict || errorBody["code"] != code || errorBody["message"] != browserTurnMessage(code) {
			t.Fatalf("HTTP persona error status=%d body=%s", response.Code, response.Body.String())
		}
		output := httptest.NewRecorder()
		if err := TranslateCoreNDJSON(context.Background(), responseBody(coreFrame("error", "persona-turn", 0, map[string]any{"code": code, "message": "private provider password"})), output); err != nil {
			t.Fatal(err)
		}
		frames := decodeFrames(t, output.Body.Bytes())
		if len(frames) != 1 || frames[0].Payload["code"] != code || frames[0].Payload["message"] != browserTurnMessage(code) {
			t.Fatalf("persona terminal=%#v", frames)
		}
		if strings.Contains(response.Body.String()+output.Body.String(), "password") {
			t.Fatal("untrusted Core message leaked")
		}
	}
}
