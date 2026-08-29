package config

import (
	"fmt"
	"net/url"
	"strings"
)

const defaultListenAddress = "0.0.0.0:3001"

// Config contains only the transport settings required by the first gateway
// slice. Domain dependencies deliberately do not appear here.
type Config struct {
	ListenAddress  string
	CoreBaseURL    *url.URL
	CoreServiceKey string
}

// FromEnv parses configuration through the supplied lookup function so the
// process entrypoint and tests use the same validation path.
func FromEnv(lookup func(string) (string, bool)) (Config, error) {
	listenAddress := defaultListenAddress
	if value, ok := lookup("GATEWAY_LISTEN_ADDR"); ok && strings.TrimSpace(value) != "" {
		listenAddress = strings.TrimSpace(value)
	}

	coreBaseURLValue, ok := lookup("CORE_BASE_URL")
	if !ok || strings.TrimSpace(coreBaseURLValue) == "" {
		return Config{}, fmt.Errorf("CORE_BASE_URL is required")
	}
	coreBaseURL, err := parseCoreBaseURL(coreBaseURLValue)
	if err != nil {
		return Config{}, err
	}

	serviceKey, ok := lookup("FLUCTLIGHT_CORE_SERVICE_KEY")
	if !ok || strings.TrimSpace(serviceKey) == "" {
		return Config{}, fmt.Errorf("FLUCTLIGHT_CORE_SERVICE_KEY is required")
	}

	return Config{
		ListenAddress:  listenAddress,
		CoreBaseURL:    coreBaseURL,
		CoreServiceKey: strings.TrimSpace(serviceKey),
	}, nil
}

func parseCoreBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("CORE_BASE_URL must be an absolute HTTP(S) URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("CORE_BASE_URL must use http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("CORE_BASE_URL must not include credentials, query, or fragment")
	}
	return parsed, nil
}
