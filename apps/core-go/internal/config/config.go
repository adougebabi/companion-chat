package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
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
	RedisURL      string
	TrustedOrigin string
	SecureCookies bool
}

// DatabaseURLFromEnv is intentionally dependency-light so the migration
// binary can run before any Core/provider/storage secrets are configured.
func DatabaseURLFromEnv(lookup func(string) (string, bool)) string {
	return strings.TrimSpace(first(lookup, "CORE_GO_DATABASE_URL", "DATABASE_URL"))
}

// FromEnv validates the small startup contract for the read-owned Core slice.
// The API owns its database connection, service identity and browser security
// configuration independently.
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
	trustedOrigins, err := ParseTrustedOrigins(first(lookup, "FLUCTLIGHT_TRUSTED_ORIGIN"))
	if err != nil {
		return Config{}, err
	}
	secureCookies := true
	if value := first(lookup, "FLUCTLIGHT_SECURE_COOKIES"); value != "" {
		parsed, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			return Config{}, fmt.Errorf("FLUCTLIGHT_SECURE_COOKIES must be a boolean")
		}
		secureCookies = parsed
	} else {
		for _, origin := range trustedOrigins {
			if strings.HasPrefix(origin, "http://") {
				secureCookies = false
			}
		}
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
		RedisURL:      firstOrDefault(lookup, "REDIS_URL", "redis://redis:6379/0"),
		TrustedOrigin: strings.Join(trustedOrigins, ","),
		SecureCookies: secureCookies,
	}, nil
}

// ParseTrustedOrigins validates every entry, including empty trailing entries.
// The singular environment name remains compatible with existing deployments.
func ParseTrustedOrigins(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("FLUCTLIGHT_TRUSTED_ORIGIN is required")
	}
	origins := []string{}
	for i, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("FLUCTLIGHT_TRUSTED_ORIGIN entry %d is empty", i+1)
		}
		origin, err := parseTrustedOrigin(entry)
		if err != nil {
			return nil, fmt.Errorf("FLUCTLIGHT_TRUSTED_ORIGIN entry %d: must be an absolute HTTP(S) origin without credentials, path, query or fragment", i+1)
		}
		origins = append(origins, origin.String())
	}
	return origins, nil
}

func parseTrustedOrigin(value string) (*url.URL, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.ForceQuery || strings.ContainsAny(value, "?#\\") || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return nil, fmt.Errorf("FLUCTLIGHT_TRUSTED_ORIGIN must be an absolute HTTP(S) origin without path, query, or fragment")
	}
	host := parsed.Hostname()
	if ip := net.ParseIP(host); ip == nil {
		if strings.HasPrefix(parsed.Host, "[") {
			return nil, fmt.Errorf("invalid IPv6 origin")
		}
		if len(host) == 0 || len(host) > 253 || strings.Trim(host, "0123456789.") == "" {
			return nil, fmt.Errorf("invalid origin host")
		}
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return nil, fmt.Errorf("invalid origin host")
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
					return nil, fmt.Errorf("invalid origin host")
				}
			}
		}
	} else if strings.Contains(host, ":") && !strings.HasPrefix(parsed.Host, "[") {
		return nil, fmt.Errorf("IPv6 origins require brackets")
	}
	if port := parsed.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid origin port")
		}
	} else if strings.HasSuffix(parsed.Host, ":") {
		return nil, fmt.Errorf("empty origin port")
	}
	parsed.Path = ""
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed, nil
}

func first(lookup func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(key); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstOrDefault(lookup func(string) (string, bool), key, fallback string) string {
	if value := first(lookup, key); value != "" {
		return value
	}
	return fallback
}
