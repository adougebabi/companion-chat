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
