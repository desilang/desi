package check

import "strings"

// Reserved language keywords and special words that must not be used as identifiers.
// We include names that the lexer might still emit as IDENT in some contexts (e.g. "none"),
// so that the checker can enforce the restriction consistently.
var reservedKeywords = map[string]struct{}{
	"package": {}, "import": {}, "from": {}, "as": {}, "type": {}, "pub": {},
	"def": {}, "let": {}, "mut": {}, "return": {},
	"if": {}, "elif": {}, "else": {}, "while": {}, "for": {}, "in": {},
	"match": {}, "struct": {}, "enum": {},
	"and": {}, "or": {}, "not": {}, "defer": {},
	"async": {}, "await": {},
	"true": {}, "false": {},
	"none": {},

	// Explicitly forbid "void" everywhere (use 'none').
	"void": {},
}

// Prelude builtins that cannot be shadowed or redefined by user code.
var preludeBuiltins = map[string]struct{}{
	"print": {},
	"len":   {},
	"str":   {},
}

// Case-sensitive check after trimming spaces (source is case-sensitive).
func isReservedIdent(name string) bool {
	n := strings.TrimSpace(name)
	_, ok := reservedKeywords[n]
	return ok
}

func isPreludeBuiltin(name string) bool {
	n := strings.TrimSpace(name)
	_, ok := preludeBuiltins[n]
	return ok
}
