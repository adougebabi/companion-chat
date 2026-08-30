package config

import (
	"fmt"
	"net/url"
	"strings"
)

type Config struct {
	ListenAddress string
	DatabaseURL   string
	ServiceKey    string
}

// FromEnv validates the small startup contract for the read-owned Core slice.
// It intentionally does not reuse BFF configuration: Core owns its database
// connection and service identity independently.
func FromEnv(lookup func(string) (string, bool)) (Config, error) {
	databaseURL := strings.TrimSpace(first(lookup, "CORE_GO_DATABASE_URL", "DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("CORE_GO_DATABASE_URL is required")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Scheme != "postgresql" || parsed.Host == "" {
		return Config{}, fmt.Errorf("CORE_GO_DATABASE_URL must be a PostgreSQL URL")
	}
	serviceKey := strings.TrimSpace(first(lookup, "FLUCTLIGHT_CORE_SERVICE_KEY"))
	if serviceKey == "" {
		return Config{}, fmt.Errorf("FLUCTLIGHT_CORE_SERVICE_KEY is required")
	}
	listen := strings.TrimSpace(first(lookup, "CORE_GO_LISTEN_ADDRESS"))
	if listen == "" {
		listen = ":8081"
	}
	return Config{ListenAddress: listen, DatabaseURL: databaseURL, ServiceKey: serviceKey}, nil
}

func first(lookup func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
