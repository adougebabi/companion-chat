package config

import "testing"

func TestFromEnvUsesCoreDatabaseURLAndDefaultAddress(t *testing.T) {
	config, err := FromEnv(func(key string) (string, bool) {
		values := map[string]string{
			"CORE_GO_DATABASE_URL":        "postgresql://user:pass@postgres:5432/fluctlight",
			"FLUCTLIGHT_CORE_SERVICE_KEY": "service-key",
			"FLUCTLIGHT_SETTINGS_KEY":     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			"S3_SECRET_KEY":               "secret",
		}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if config.ListenAddress != ":8080" || config.DatabaseURL == "" || config.ServiceKey != "service-key" {
		t.Fatalf("unexpected config: %+v", config)
	}
}

func TestFromEnvRejectsMissingServiceKey(t *testing.T) {
	_, err := FromEnv(func(key string) (string, bool) {
		if key == "CORE_GO_DATABASE_URL" {
			return "postgresql://postgres/fluctlight", true
		}
		if key == "FLUCTLIGHT_SETTINGS_KEY" {
			return "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", true
		}
		if key == "S3_SECRET_KEY" {
			return "secret", true
		}
		return "", false
	})
	if err == nil {
		t.Fatal("FromEnv() error = nil")
	}
}
