// Package instant owns the public instant format. Storage and cursor identities
// retain their precision; dates and original source/audit payloads stay intact.
package instant

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
)

const Layout = "2006-01-02T15:04:05.000-07:00"

func Format(at time.Time) string { return at.UTC().Format(Layout) }
func FormatLocal(at time.Time, zone *time.Location) string {
	if zone == nil {
		zone = time.UTC
	}
	return at.In(zone).Format(Layout)
}

// Marshal applies the contract only to declared timestamp fields at transport
// boundaries. It preserves JSON numbers and never rewrites user text, stored
// prompt/response evidence, or raw signed source snapshots.
func Marshal(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	return json.Marshal(normalize(decoded, ""))
}
func normalize(value any, key string) any {
	switch key {
	case "core_persona", "corePersona", "identity", "personality", "behavioral_policy", "behavioralPolicy", "life_profile", "lifeProfile", "extensions", "payload", "prompt", "response", "snapshot", "before_value", "after_value", "source_records":
		return value
	}
	switch v := value.(type) {
	case map[string]any:
		for k, child := range v {
			v[k] = normalize(child, k)
		}
		return v
	case []any:
		for i, child := range v {
			v[i] = normalize(child, "")
		}
		return v
	case string:
		declared := strings.HasSuffix(key, "_at") || strings.HasSuffix(key, "At")
		switch key {
		case "instant", "as_of", "asOf", "deadline", "expiration", "valid_from", "valid_until", "validFrom", "validUntil", "not_before", "notBefore", "completed_before", "completedBefore", "preferred_time", "preferredTime":
			declared = true
		}
		if declared {
			if at, err := time.Parse(time.RFC3339Nano, v); err == nil {
				return Format(at)
			}
		}
	}
	return value
}

// LogAttribute keeps operational audit clocks real while formatting their
// declared time values with the same public instant contract.
func LogAttribute(_ []string, attr slog.Attr) slog.Attr {
	if attr.Value.Kind() == slog.KindTime {
		attr.Value = slog.StringValue(Format(attr.Value.Time()))
	}
	return attr
}
