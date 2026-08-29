package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const defaultListenAddress = "0.0.0.0:3000"

// Config contains only the transport settings required by the first gateway
// slice. Domain dependencies deliberately do not appear here.
type Config struct {
	ListenAddress  string
	CoreBaseURL    *url.URL
	CoreServiceKey string
	TrustedOrigin  string
	SecureCookies  bool
}

// FromEnv parses configuration through the supplied lookup function so the
// process entrypoint and tests use the same validation path.
func FromEnv(lookup func(string) (string, bool)) (Config, error) {
	listenAddress := listenAddressFromEnv(lookup)

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

	trustedOrigin := ""
	if value, ok := lookup("FLUCTLIGHT_TRUSTED_ORIGIN"); ok {
		trustedOrigin = strings.TrimSpace(value)
	}
	secureCookies := true
	if value, ok := lookup("FLUCTLIGHT_SECURE_COOKIES"); ok && strings.TrimSpace(value) != "" {
		parsed, parseErr := strconv.ParseBool(strings.TrimSpace(value))
		if parseErr != nil {
			return Config{}, fmt.Errorf("FLUCTLIGHT_SECURE_COOKIES must be a boolean")
		}
		secureCookies = parsed
	} else if strings.HasPrefix(trustedOrigin, "http://") {
		secureCookies = false
	}

	return Config{
		ListenAddress:  listenAddress,
		CoreBaseURL:    coreBaseURL,
		CoreServiceKey: strings.TrimSpace(serviceKey),
		TrustedOrigin:  trustedOrigin,
		SecureCookies:  secureCookies,
	}, nil
}

func listenAddressFromEnv(lookup func(string) (string, bool)) string {
	if value, ok := lookup("GATEWAY_LISTEN_ADDR"); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	host, hasHost := lookup("BFF_HOST")
	port, hasPort := lookup("BFF_PORT")
	if (hasHost && strings.TrimSpace(host) != "") || (hasPort && strings.TrimSpace(port) != "") {
		resolvedHost := strings.TrimSpace(host)
		if resolvedHost == "" {
			resolvedHost = "0.0.0.0"
		}
		resolvedPort := strings.TrimSpace(port)
		if resolvedPort == "" {
			resolvedPort = "3000"
		}
		return resolvedHost + ":" + resolvedPort
	}
	return defaultListenAddress
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
