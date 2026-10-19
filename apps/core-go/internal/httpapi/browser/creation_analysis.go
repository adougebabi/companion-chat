package browser

// A template selector is resolved by Core from the exact immutable publication;
// accepting both a body and selector would let the browser change its meaning.
func validCreationAnalysisInput(value map[string]any) bool {
	if value["resourceId"] != nil || value["resourceVersion"] != nil {
		return onlyKeys(value, "resourceId", "resourceVersion") && validateString(value["resourceId"], 1, 128) && validateString(value["resourceVersion"], 1, 64)
	}
	if _, hasDescription := value["description"]; hasDescription {
		return onlyKeys(value, "description") && validateInitializationDescriptionShape(value["description"])
	}
	if _, hasJSON := value["json"]; hasJSON {
		return onlyKeys(value, "json") && isObject(value["json"]) && len(object(value["json"])) > 0
	}
	// Preserve the existing direct structured-input transport.
	return isObject(value["core_persona"]) && len(object(value["core_persona"])) > 0
}
