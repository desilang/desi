package check

import "strings"

// Hard keywords / special words. We treat them as reserved even if the lexer
// doesn't currently keywordize some of them (e.g., "none").
var reservedWords = map[string]struct{}{
	// Language / control
	"package": {}, "import": {}, "from": {}, "as": {}, "pub": {},
	"def": {}, "struct": {}, "enum": {}, "type": {},
	"let": {}, "mut": {}, "return": {}, "if": {}, "elif": {}, "else": {},
	"while": {}, "for": {}, "in": {}, "match": {}, "defer": {},
	"and": {}, "or": {}, "not": {},
	"async": {}, "await": {},
	// Literals / special
	"true": {}, "false": {},
	// Type-ish words that must not be used as identifiers
	"none": {}, "void": {},
}

// Prelude / builtins that should never be shadowed by locals.
var preludeBuiltins = map[string]struct{}{
	"print": {}, "panic": {},
}

// Standard module roots users commonly import/use; disallow as local names to
// avoid confusion (e.g., let io = 3).
var stdModuleRoots = map[string]struct{}{
	"io": {}, "fs": {}, "os": {}, "mem": {}, "str": {}, "math": {},
}

func isReservedWord(name string) bool {
	_, ok := reservedWords[strings.ToLower(strings.TrimSpace(name))]
	return ok
}
func isPreludeBuiltin(name string) bool {
	_, ok := preludeBuiltins[strings.ToLower(strings.TrimSpace(name))]
	return ok
}
func isStdModuleRoot(name string) bool {
	_, ok := stdModuleRoots[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// enforceAllowedLocalName validates a local binding/parameter name against
// (1) reserved words, (2) prelude builtins, and (3) imported names in this file.
func enforceAllowedLocalName(info *Info, name, context string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return nil
	}
	if isReservedWord(n) || isStdModuleRoot(n) {
		// treat std module root as "reserved identifier" too
		return ErrReservedIdentifier(n, context)
	}
	if isPreludeBuiltin(n) {
		return ErrShadowBuiltin(n, context)
	}
	if info != nil && info.ImportedNames != nil && info.ImportedNames[n] {
		return ErrImportNameConflict(n, context)
	}
	return nil
}
