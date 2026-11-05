package core

import (
	"regexp"
	"strings"
)

// Only character naming clauses and standalone name fields qualify. In
// particular, a friend's name, the Owner's name and nicknames are not names
// of the character being initialized.
var initializationNameDeclaration = regexp.MustCompile(`(?i)(?:^|[\r\n，,。；;！？!?])[\t ]*(?:(?:角色|人物|她|他)(?:的)?(?:姓名|名字|名称)(?:叫做|叫|是|为|[:：=])|(?:角色|人物|她|他)(?:叫做|叫|名为)|(?:姓名|名字|名称|name)[\t ]*(?:叫做|叫|是|为|[:：=])|(?:请)?(?:帮我)?(?:创建|设计|生成)(?:一个|一位|一名)?(?:角色|人物)?(?:，|,)?(?:名为|名字叫)|名为)[\t ]*`)

var initializationBareName = regexp.MustCompile(`^[\p{L}\p{N}\p{M}\p{Pd}_·・'’]+$`)

func prepareAnalyzedInitializationResponse(description string, value map[string]any) (map[string]any, error) {
	result, err := prepareInitializationResponse(recoverAnalyzedInitializationArrays(value))
	if err != nil {
		return nil, err
	}
	identity := mapValue(mapValue(result["core_persona"])["identity"])
	if name := explicitInitializationName(description, identity); name != "" {
		identity["name"] = name
	}
	return result, nil
}

func explicitInitializationName(description string, identity map[string]any) string {
	var declared string
	for _, match := range initializationNameDeclaration.FindAllStringIndex(description, -1) {
		source := strings.TrimSpace(description[match[1]:])
		name := initializationNameLiteral(source)
		if name == "" {
			continue
		}
		quoted := strings.HasPrefix(source, "\"") || strings.HasPrefix(source, "'") || strings.HasPrefix(source, "「") || strings.HasPrefix(source, "『") || strings.HasPrefix(source, "“")
		if !quoted {
			// "名为FT-林夏的角色" includes grammar after the literal name.
			// Remove it only when the remaining name agrees with the model's
			// recognized identity, never from an explicitly quoted name.
			for _, suffix := range []string{"的角色", "的人物", "的女孩", "的男孩"} {
				if candidate := strings.TrimSuffix(name, suffix); candidate != name && initializationNameMatchesIdentity(candidate, identity) {
					name = candidate
					break
				}
			}
			// "她叫我不要迟到" is not a naming declaration. Unquoted
			// pronoun + 叫 needs support from the analyzed name/nickname.
			marker := strings.TrimSpace(description[match[0]:match[1]])
			if (strings.HasSuffix(marker, "她叫") || strings.HasSuffix(marker, "他叫") || strings.HasSuffix(marker, "她叫做") || strings.HasSuffix(marker, "他叫做")) && !initializationNameMatchesIdentity(name, identity) {
				continue
			}
		}
		// Conflicting declarations need semantic analysis; never pick an
		// arbitrary first or last character/profile name.
		if declared != "" && declared != name {
			return ""
		}
		declared = name
	}
	if declared != "" {
		return declared
	}

	// Character cards often start with "FT-林夏，24岁，…" without a label.
	// Require the first literal to agree with the analyzed identity rather
	// than treating arbitrary introductory prose as a name.
	name := initializationNameLiteral(strings.TrimSpace(description))
	if name == "" {
		return ""
	}
	if initializationNameMatchesIdentity(name, identity) {
		return name
	}
	return ""
}

func initializationNameMatchesIdentity(name string, identity map[string]any) bool {
	for _, key := range []string{"name", "nickname"} {
		analyzed := stringValue(identity[key])
		if name == analyzed {
			return true
		}
		// A prefix separated by a dash is part of the supplied name. This
		// also handles a model that dropped the prefix without a nickname.
		for index, character := range name {
			if strings.ContainsRune("-‐‑‒–—﹘﹣－", character) && analyzed != "" && (name[index+len(string(character)):] == analyzed || name[:index] == analyzed) {
				return true
			}
		}
	}
	return false
}

func initializationNameLiteral(text string) string {
	text = strings.TrimSpace(text)
	for _, quotes := range [][2]string{{"「", "」"}, {"『", "』"}, {"“", "”"}, {"\"", "\""}, {"'", "'"}} {
		if strings.HasPrefix(text, quotes[0]) {
			rest := strings.TrimPrefix(text, quotes[0])
			if end := strings.Index(rest, quotes[1]); end >= 0 && !strings.ContainsAny(rest[:end], "\r\n") {
				return strings.TrimSpace(rest[:end])
			}
			return ""
		}
	}
	end := strings.IndexAny(text, "，,。.;；！!？?\r\n")
	if end >= 0 {
		text = text[:end]
	}
	name := strings.TrimSpace(text)
	if initializationBareName.MatchString(name) {
		return name
	}
	return ""
}
