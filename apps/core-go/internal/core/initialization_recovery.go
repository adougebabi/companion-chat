package core

import (
	"fmt"
	"strings"
	"time"
)

// Recover representation gaps without inventing semantic facts. Keep the source
// and warnings in the analysis so the Owner can review every repaired field.
func recoverInitializationContainers(value map[string]any) map[string]any {
	result := cloneMap(value)
	extensions := mapValue(result["extensions"])
	if extensions == nil {
		extensions = map[string]any{}
	}
	recovered := mapValue(extensions["recovered_fields"])
	if recovered == nil {
		recovered = map[string]any{}
	}
	warnings := []any{}
	if raw, exists := extensions["initialization_warnings"]; exists && raw != nil {
		if entries, ok := raw.([]any); ok {
			for i, entry := range entries {
				warning := mapValue(entry)
				if strings.TrimSpace(stringValue(warning["message"])) != "" {
					warnings = append(warnings, map[string]any{"path": firstInitializationString(warning["path"], "initialization"), "message": warning["message"]})
				} else {
					path := fmt.Sprintf("extensions.initialization_warnings[%d]", i)
					recovered[path] = entry
					warnings = append(warnings, map[string]any{"path": path, "message": "补充说明格式不完整，原值已保留，可继续审阅"})
				}
			}
		} else {
			recovered["extensions.initialization_warnings"] = raw
			warnings = append(warnings, map[string]any{"path": "extensions.initialization_warnings", "message": "补充说明格式不完整，原值已保留，可继续审阅"})
		}
	}
	warn := func(path string, raw any) {
		recovered[path] = raw
		warnings = append(warnings, map[string]any{"path": path, "message": "原解析内容已保留，缺失或不完整结构已补齐，可继续审阅"})
	}
	if _, ok := numberFloat(result["schema_version"]); !ok {
		if raw, exists := result["schema_version"]; exists {
			warn("schema_version", raw)
		}
		result["schema_version"] = 2
	}
	persona, ok := result["core_persona"].(map[string]any)
	if !ok {
		persona = map[string]any{}
		if result["core_persona"] != nil {
			warn("core_persona", result["core_persona"])
		}
		result["core_persona"] = persona
	}
	if _, ok := numberFloat(persona["schema_version"]); !ok {
		if raw, exists := persona["schema_version"]; exists {
			warn("core_persona.schema_version", raw)
		}
		persona["schema_version"] = 1
	}
	for _, key := range []string{"identity", "personality", "behavioral_policy", "life_profile", "personality_system"} {
		if persona[key] == nil && result[key] != nil {
			persona[key] = result[key]
			delete(result, key)
		}
		if raw := persona[key]; raw != nil {
			if _, ok := raw.(map[string]any); !ok {
				warn("core_persona."+key, raw)
				persona[key] = map[string]any{}
				if key == "identity" {
					if text, ok := raw.(string); ok {
						mapValue(persona[key])["notes"] = text
					}
				}
			}
		} else {
			warn("core_persona."+key, nil)
			persona[key] = map[string]any{}
		}
	}
	// These defaults supply key names only. Unknown traits and expression are
	// null in an analyzed foundation; operational defaults remain runtime-owned.
	for group, shape := range map[string]map[string]any{"identity": defaultIdentity("", ""), "personality": defaultPersonality(), "behavioral_policy": defaultPolicy()} {
		fields := mapValue(persona[group])
		for field := range shape {
			if _, exists := fields[field]; !exists {
				fields[field] = nil
			}
		}
	}
	if raw := result["developing_self"]; raw != nil {
		if _, ok := raw.(map[string]any); !ok {
			warn("developing_self", raw)
			result["developing_self"] = map[string]any{"claims": []any{}}
		}
	}
	if identity, ok := persona["identity"].(map[string]any); ok {
		if zone := stringValue(identity["timezone"]); zone != "" {
			if _, err := time.LoadLocation(canonicalTimezone(zone)); err != nil {
				warn("core_persona.identity.timezone", zone)
				identity["timezone"] = nil
			}
		}
	}
	for _, key := range []string{"initial_goals", "initial_intentions", "initial_relationships"} {
		if text, ok := result[key].(string); ok {
			warn(key, text)
			switch key {
			case "initial_goals":
				result[key] = []any{map[string]any{"description": text}}
			case "initial_intentions":
				result[key] = []any{map[string]any{"action": text}}
			default:
				result[key] = []any{}
			}
		}
	}
	if raw, exists := result["actor_user"]; exists {
		if raw == nil {
			delete(result, "actor_user")
		} else if validateActorUserSettings(raw) != nil {
			warn("actor_user", raw)
			delete(result, "actor_user")
		}
	}
	extensions["recovered_fields"] = recovered
	extensions["initialization_warnings"] = warnings
	result["extensions"] = extensions
	return result
}

func recoverInitializationProfiles(result map[string]any) {
	persona := mapValue(result["core_persona"])
	system := mapValue(persona["personality_system"])
	extensions := mapValue(result["extensions"])
	warnings := arrayValue(extensions["initialization_warnings"])
	profiles := []any{}
	for i, raw := range arrayValue(system["profiles"]) {
		if len(mapValue(raw)) == 0 {
			recovered := mapValue(extensions["recovered_fields"])
			recovered[fmt.Sprintf("core_persona.personality_system.profiles[%d]", i)] = raw
			extensions["recovered_fields"] = recovered
			warnings = append(warnings, map[string]any{"path": fmt.Sprintf("core_persona.personality_system.profiles[%d]", i), "message": "空人格项已忽略，原始分析来源保留"})
			continue
		}
		profiles = append(profiles, raw)
	}
	system["profiles"] = profiles
	extensions["initialization_warnings"] = warnings
	normalizePersonalityProfiles(system, extensions)
	warnings = arrayValue(extensions["initialization_warnings"])
	// Missing optional profile fields are unknown rather than a schema rejection.
	for _, raw := range profiles {
		profile := mapValue(raw)
		for _, field := range personalityProfileFieldNames() {
			if _, ok := profile[field]; !ok {
				profile[field] = nil
			}
		}
	}
	if mode := stringValue(system["mode"]); mode == "" {
		system["mode"] = "single"
	}
	if strings.TrimSpace(stringValue(system["active_profile_id"])) == "" {
		system["active_profile_id"] = "default"
	}
	extensions["initialization_warnings"] = warnings
}
