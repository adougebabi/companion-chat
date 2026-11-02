package config

import (
	"strings"
	"testing"
)

func TestFromEnvUsesCoreDatabaseURLAndDefaultAddress(t *testing.T) {
	config, err := FromEnv(func(key string) (string, bool) {
		values := map[string]string{
			"CORE_GO_DATABASE_URL":        "postgresql://user:pass@postgres:5432/fluctlight",
			"FLUCTLIGHT_CORE_SERVICE_KEY": "service-key",
			"FLUCTLIGHT_SETTINGS_KEY":     "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			"FLUCTLIGHT_TRUSTED_ORIGIN":   "http://127.0.0.1:13001",
			"S3_SECRET_KEY":               "secret",
		}
		value, ok := values[key]
		return value, ok
	})
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if config.ListenAddress != ":8080" || config.DatabaseURL == "" || config.ServiceKey != "service-key" || config.RedisURL != "redis://redis:6379/0" {
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

func TestTrustedOriginsListValidatesEveryEntryAndTrimsSpaces(t *testing.T) {
	origins, err := ParseTrustedOrigins(" http://127.0.0.1:13001 , http://100.109.12.105:13001 ")
	if err != nil || len(origins) != 2 || origins[0] != "http://127.0.0.1:13001" || origins[1] != "http://100.109.12.105:13001" {
		t.Fatal(origins, err)
	}
	for _, good := range []string{"https://example.com", "http://localhost:13001", "http://[::1]:13001", "https://EXAMPLE.com/"} {
		if _, err := ParseTrustedOrigins(good); err != nil {
			t.Fatal(good, err)
		}
	}
	for _, bad := range []string{"", ",http://localhost", "http://localhost,", "http://localhost,,https://example.com", "http://localhost, null", "http://localhost, ftp://example.com", "http://user:pass@example.com", "http://example.com/path", "http://example.com?", "http://example.com#", "http://example.com:99999", "http://example.com:", "http://::1", "http://bad host", "http://-invalid.test", "http://999.999.999.999", "https://*.test", "http://[not-an-ip]", "http://"} {
		t.Run(bad, func(t *testing.T) {
			if _, err := ParseTrustedOrigins(bad); err == nil || !strings.Contains(err.Error(), "FLUCTLIGHT_TRUSTED_ORIGIN") {
				t.Fatal("invalid list accepted or unclear error", bad, err)
			}
		})
	}
}

func TestFromEnvAcceptsMultipleTrustedOriginsAndHTTPDefaults(t *testing.T) {
	values := map[string]string{"CORE_GO_DATABASE_URL": "postgresql://postgres/test", "FLUCTLIGHT_CORE_SERVICE_KEY": "test-key", "FLUCTLIGHT_SETTINGS_KEY": "test-settings", "S3_SECRET_KEY": "test-secret", "FLUCTLIGHT_TRUSTED_ORIGIN": " http://127.0.0.1:13001 , http://100.109.12.105:13001 "}
	lookup := func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	got, err := FromEnv(lookup)
	if err != nil || got.TrustedOrigin != "http://127.0.0.1:13001,http://100.109.12.105:13001" || got.SecureCookies {
		t.Fatal(got, err)
	}
	values["FLUCTLIGHT_TRUSTED_ORIGIN"] = "https://example.com,http://127.0.0.1:13001"
	got, err = FromEnv(lookup)
	if err != nil || got.SecureCookies {
		t.Fatal("HTTP entry unusable with default secure cookie", got, err)
	}
	values["FLUCTLIGHT_SECURE_COOKIES"] = "true"
	got, err = FromEnv(lookup)
	if err != nil || !got.SecureCookies {
		t.Fatal("explicit cookie choice lost", got, err)
	}
	values["FLUCTLIGHT_TRUSTED_ORIGIN"] = "http://127.0.0.1:13001,"
	if _, err = FromEnv(lookup); err == nil || !strings.Contains(err.Error(), "entry 2 is empty") {
		t.Fatal(err)
	}
}
