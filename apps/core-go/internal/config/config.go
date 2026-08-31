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
	SettingsKey   string
	S3Endpoint    string
	S3Region      string
	S3Bucket      string
	S3AccessKey   string
	S3SecretKey   string
	S3UseSSL      bool
	TemporalAddr  string
	TemporalNS    string
}

// DatabaseURLFromEnv is intentionally dependency-light so the migration
// binary can run before any Core/provider/storage secrets are configured.
func DatabaseURLFromEnv(lookup func(string) (string, bool)) string {
	return strings.TrimSpace(first(lookup, "CORE_GO_DATABASE_URL", "DATABASE_URL"))
}

// FromEnv validates the small startup contract for the read-owned Core slice.
// It intentionally does not reuse BFF configuration: Core owns its database
// connection and service identity independently.
func FromEnv(lookup func(string) (string, bool)) (Config, error) {
	databaseURL := DatabaseURLFromEnv(lookup)
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
		listen = ":8080"
	}
	settingsKey := strings.TrimSpace(first(lookup, "FLUCTLIGHT_SETTINGS_KEY"))
	if settingsKey == "" {
		return Config{}, fmt.Errorf("FLUCTLIGHT_SETTINGS_KEY is required")
	}
	s3Endpoint := strings.TrimSpace(first(lookup, "S3_ENDPOINT"))
	if s3Endpoint == "" {
		s3Endpoint = "http://minio:9000"
	}
	s3Region := strings.TrimSpace(first(lookup, "S3_REGION"))
	if s3Region == "" {
		s3Region = "us-east-1"
	}
	s3Bucket := strings.TrimSpace(first(lookup, "S3_BUCKET"))
	if s3Bucket == "" {
		s3Bucket = "fluctlight-media"
	}
	s3Access := strings.TrimSpace(first(lookup, "S3_ACCESS_KEY"))
	if s3Access == "" {
		s3Access = "fluctlight"
	}
	s3Secret := strings.TrimSpace(first(lookup, "S3_SECRET_KEY"))
	if s3Secret == "" {
		return Config{}, fmt.Errorf("S3_SECRET_KEY is required")
	}
	return Config{
		ListenAddress: listen,
		DatabaseURL:   databaseURL,
		ServiceKey:    serviceKey,
		SettingsKey:   settingsKey,
		S3Endpoint:    s3Endpoint,
		S3Region:      s3Region,
		S3Bucket:      s3Bucket,
		S3AccessKey:   s3Access,
		S3SecretKey:   s3Secret,
		S3UseSSL:      strings.EqualFold(first(lookup, "S3_USE_SSL"), "true"),
		TemporalAddr:  first(lookup, "TEMPORAL_ADDRESS"),
		TemporalNS:    first(lookup, "TEMPORAL_NAMESPACE"),
	}, nil
}

func first(lookup func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
