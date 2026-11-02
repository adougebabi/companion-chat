package core

// creationIdentity keeps the Owner's chosen name authoritative over an
// initialization model's interpretation. Other declared identity facts,
// including a distinct nickname, retain their original meaning.
func creationIdentity(identity map[string]any, id, name string) map[string]any {
	result := make(map[string]any, len(identity)+1)
	for key, value := range identity {
		result[key] = value
	}
	result["id"] = id
	if name != "" {
		result["name"] = name
	}
	return result
}

// Profiles feed the portrait compiler independently of the shared identity.
// Rename only their declared name fields; source prose and profile IDs remain
// intact, and the frozen analysis is never changed.
func creationProfileNames(corePersona map[string]any, name string) {
	if name == "" {
		return
	}
	for _, raw := range arrayValue(mapValue(corePersona["personality_system"])["profiles"]) {
		profile := mapValue(raw)
		profile["name"] = name
		identity := mapValue(profile["identity"])
		identity["name"] = name
		profile["identity"] = identity
	}
}
