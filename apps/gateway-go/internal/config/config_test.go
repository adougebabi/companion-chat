package config

import "testing"

func TestFromEnvUsesDefaultsAndTrimsValues(t *testing.T) {
	config, err := FromEnv(mapLookup(map[string]string{
		"CORE_BASE_URL":               " http://core:8080 ",
		"FLUCTLIGHT_CORE_SERVICE_KEY": " secret ",
	}))
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if config.ListenAddress != defaultListenAddress {
		t.Fatalf("ListenAddress = %q, want %q", config.ListenAddress, defaultListenAddress)
	}
	if config.CoreBaseURL.String() != "http://core:8080" {
		t.Fatalf("CoreBaseURL = %q, want http://core:8080", config.CoreBaseURL)
	}
	if config.CoreServiceKey != "secret" {
		t.Fatalf("CoreServiceKey = %q, want secret", config.CoreServiceKey)
	}
	if !config.SecureCookies {
		t.Fatalf("SecureCookies = %v, want true by default", config.SecureCookies)
	}
}

func TestFromEnvRejectsMissingRequiredValues(t *testing.T) {
	_, err := FromEnv(mapLookup(map[string]string{}))
	if err == nil {
		t.Fatal("FromEnv() error = nil, want missing CORE_BASE_URL error")
	}

	_, err = FromEnv(mapLookup(map[string]string{"CORE_BASE_URL": "http://core:8080"}))
	if err == nil {
		t.Fatal("FromEnv() error = nil, want missing service key error")
	}
}

func TestFromEnvRejectsUnsafeCoreURL(t *testing.T) {
	values := map[string]string{
		"CORE_BASE_URL":               "ftp://core:8080",
		"FLUCTLIGHT_CORE_SERVICE_KEY": "secret",
	}
	if _, err := FromEnv(mapLookup(values)); err == nil {
		t.Fatal("FromEnv() error = nil, want scheme validation error")
	}

	values["CORE_BASE_URL"] = "http://user:pass@core:8080"
	if _, err := FromEnv(mapLookup(values)); err == nil {
		t.Fatal("FromEnv() error = nil, want credential validation error")
	}
}

func TestFromEnvUsesBFFCompatibilitySettings(t *testing.T) {
	config, err := FromEnv(mapLookup(map[string]string{
		"BFF_HOST":                    "127.0.0.1",
		"BFF_PORT":                    "3100",
		"CORE_BASE_URL":               "http://core:8080",
		"FLUCTLIGHT_CORE_SERVICE_KEY": "secret",
		"FLUCTLIGHT_TRUSTED_ORIGIN":   "http://localhost:5173",
	}))
	if err != nil {
		t.Fatalf("FromEnv() error = %v", err)
	}
	if config.ListenAddress != "127.0.0.1:3100" {
		t.Fatalf("ListenAddress = %q, want 127.0.0.1:3100", config.ListenAddress)
	}
	if config.TrustedOrigin != "http://localhost:5173" {
		t.Fatalf("TrustedOrigin = %q", config.TrustedOrigin)
	}
	if config.SecureCookies {
		t.Fatal("SecureCookies = true, want false for http trusted origin")
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
