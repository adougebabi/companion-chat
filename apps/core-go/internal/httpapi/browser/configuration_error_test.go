package browser

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestConfigurationErrorPreservesExplicitAdmissionRejection(t *testing.T) {
	response := httptest.NewRecorder()
	(&Server{}).configurationError(response, &CoreError{Status: 422, Code: "domain_plugins_config_invalid", Details: map[string]any{"configuration_admission": "rejected", "password": "not-public"}})
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	details := objectValue(body["details"])
	if response.Code != 422 || body["code"] != "domain_plugins_config_invalid" || details["configuration_admission"] != "rejected" || details["password"] != nil {
		t.Fatalf("unexpected browser response: status=%d body=%v", response.Code, body)
	}
}
