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

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
