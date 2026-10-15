package core

import (
	"context"
	"encoding/json"
	"strings"
)

type providerDiagnosticSecretsKey struct{}

func withProviderDiagnosticSecrets(ctx context.Context, secrets ...string) context.Context {
	prior, _ := ctx.Value(providerDiagnosticSecretsKey{}).([]string)
	copied := append([]string{}, prior...)
	for _, secret := range secrets {
		if secret != "" {
			copied = append(copied, secret)
		}
	}
	return context.WithValue(ctx, providerDiagnosticSecretsKey{}, copied)
}
func redactProviderDiagnostic(ctx context.Context, value any) any {
	secrets, _ := ctx.Value(providerDiagnosticSecretsKey{}).([]string)
	return scrubDiagnosticSecrets(redactDiagnostic(value), secrets)
}
func scrubDiagnosticSecrets(value any, secrets []string) any {
	switch typed := value.(type) {
	case string:
		var encoded any
		if json.Unmarshal([]byte(typed), &encoded) == nil {
			switch encoded.(type) {
			case map[string]any, []any:
				sanitized, _ := json.Marshal(scrubDiagnosticSecrets(redactDiagnostic(encoded), secrets))
				return string(sanitized)
			}
		}
		for _, secret := range secrets {
			if secret != "" {
				typed = strings.ReplaceAll(typed, secret, "[REDACTED_SECRET]")
			}
		}
		return typed
	case map[string]any:
		result := map[string]any{}
		for key, child := range typed {
			safeKey := key
			for _, secret := range secrets {
				if secret != "" {
					safeKey = strings.ReplaceAll(safeKey, secret, "[REDACTED_SECRET]")
				}
			}
			result[safeKey] = scrubDiagnosticSecrets(child, secrets)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = scrubDiagnosticSecrets(child, secrets)
		}
		return result
	default:
		return value
	}
}
func diagnosticSecretField(normalized string) bool {
	switch normalized {
	case "apikey", "authorization", "proxyauthorization", "password", "secret", "accesskey", "secretkey", "accesstoken", "refreshtoken", "sessiontoken", "cookie", "setcookie", "assignment", "providerassignment":
		return true
	default:
		return false
	}
}
