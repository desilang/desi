package check

import "strings"

// ---------- reserved names for aliasing ----------

func reservedAliasReason(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}

	// language keywords (Stage-1 superset, harmless if extra)
	keywords := map[string]struct{}{
		"package": {}, "import": {}, "from": {}, "as": {}, "def": {}, "struct": {}, "enum": {}, "type": {},
		"let": {}, "mut": {}, "if": {}, "elif": {}, "else": {}, "while": {}, "return": {}, "match": {}, "defer": {},
		"true": {}, "false": {},
	}
	if _, ok := keywords[n]; ok {
		return "reserved keyword"
	}

	// builtins / std shims / known module roots
	builtins := map[string]struct{}{
		"print": {}, "io": {}, "fs": {}, "os": {}, "mem": {}, "str": {},
	}
	if _, ok := builtins[n]; ok {
		return "reserved builtin"
	}

	return ""
}
