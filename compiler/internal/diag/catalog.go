package diag

import (
	_ "embed"
	"encoding/json"
	"sync"
)

//go:embed codes.json
var codesJSON []byte

// CodeEntry is a single diagnostic code definition.
type CodeEntry struct {
	ID    string `json:"id"`    // e.g., "DLE0001"
	Title string `json:"title"` // short human title e.g., "unterminated string"
	Help  string `json:"help"`  // optional default help text
}

// Registry is the top-level catalog format.
// You can grow these sections over time (parser/type/checker/warnings/etc.).
type Registry struct {
	Lexer  map[string]CodeEntry `json:"lexer"`
	Parser map[string]CodeEntry `json:"parser"`
	Type   map[string]CodeEntry `json:"type"`
}

var (
	regOnce sync.Once
	reg     Registry
	regErr  error
)

func load() error {
	regOnce.Do(func() {
		if len(codesJSON) == 0 {
			regErr = nil // empty catalog is allowed
			return
		}
		regErr = json.Unmarshal(codesJSON, &reg)
	})
	return regErr
}

// Lookup returns a code entry by (domain, key).
// Domain should be one of: "lexer", "parser", "type" (grow as needed).
func Lookup(domain, key string) (CodeEntry, bool) {
	if err := load(); err != nil {
		return CodeEntry{}, false
	}
	switch domain {
	case "lexer":
		if reg.Lexer == nil {
			return CodeEntry{}, false
		}
		ce, ok := reg.Lexer[key]
		return ce, ok
	case "parser":
		if reg.Parser == nil {
			return CodeEntry{}, false
		}
		ce, ok := reg.Parser[key]
		return ce, ok
	case "type":
		if reg.Type == nil {
			return CodeEntry{}, false
		}
		ce, ok := reg.Type[key]
		return ce, ok
	default:
		return CodeEntry{}, false
	}
}

// MustLookup is a convenience that returns an entry if found; otherwise it
// returns a synthesized placeholder with the provided defaultID and title.
// Use this when you want stable codes even if the JSON is temporarily missing.
func MustLookup(domain, key, defaultID, defaultTitle string) CodeEntry {
	if ce, ok := Lookup(domain, key); ok {
		return ce
	}
	return CodeEntry{ID: defaultID, Title: defaultTitle}
}

// LookupLexer is a convenience for the "lexer" domain.
func LookupLexer(key string) (CodeEntry, bool) { return Lookup("lexer", key) }

// LookupParser is a convenience for the "parser" domain.
func LookupParser(key string) (CodeEntry, bool) { return Lookup("parser", key) }

// LookupType is a convenience for the "type" domain.
func LookupType(key string) (CodeEntry, bool) { return Lookup("type", key) }
