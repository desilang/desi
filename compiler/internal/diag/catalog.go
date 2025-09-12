package diag

import (
	_ "embed"
	"encoding/json"
	"sync"
)

/* =============================================================================
   Embedded registry (portable to Desi later)
   ========================================================================== */

//go:embed codes.json
var codesJSON []byte

// CodeEntry is the basic definition for a diagnostic code.
type CodeEntry struct {
	ID    string `json:"id"`    // e.g., "DLE0001"
	Title string `json:"title"` // short title, printed in header
	Help  string `json:"help"`  // optional default help text
}

// WhereSpec indicates where a default suggestion or span end should be placed.
type WhereSpec struct {
	// "eol" | "primary_offset" | "pos" (can grow later)
	Kind  string `json:"kind"`
	Delta int    `json:"delta,omitempty"` // for primary_offset
	Line  int    `json:"line,omitempty"`  // for pos
	Col   int    `json:"col,omitempty"`   // for pos
}

// SuggestionSpec describes a default suggestion loaded from JSON.
type SuggestionSpec struct {
	Where         WhereSpec `json:"where"`
	Label         string    `json:"label,omitempty"`
	Message       string    `json:"message,omitempty"`
	Replacement   string    `json:"replacement,omitempty"`
	Applicability string    `json:"applicability,omitempty"` // "machine-applicable", ...
}

// CodeFull bundles a code entry with defaults for primary-end shaping and suggestions.
type CodeFull struct {
	Entry       CodeEntry        `json:"-"`
	PrimaryEnd  WhereSpec        `json:"primary_end,omitempty"`
	Suggestions []SuggestionSpec `json:"suggestions,omitempty"`
}

// Registry is the top-level catalog format.
type Registry struct {
	Lexer map[string]struct {
		CodeEntry
		PrimaryEnd  WhereSpec        `json:"primary_end,omitempty"`
		Suggestions []SuggestionSpec `json:"suggestions,omitempty"`
	} `json:"lexer"`
	Parser map[string]struct {
		CodeEntry
		PrimaryEnd  WhereSpec        `json:"primary_end,omitempty"`
		Suggestions []SuggestionSpec `json:"suggestions,omitempty"`
	} `json:"parser"`
	Type map[string]struct {
		CodeEntry
		PrimaryEnd  WhereSpec        `json:"primary_end,omitempty"`
		Suggestions []SuggestionSpec `json:"suggestions,omitempty"`
	} `json:"type"`
}

var (
	regOnce sync.Once
	reg     Registry
	regErr  error
)

func load() error {
	regOnce.Do(func() {
		if len(codesJSON) == 0 {
			regErr = nil // allow empty catalog
			return
		}
		regErr = json.Unmarshal(codesJSON, &reg)
	})
	return regErr
}

/* =============================================================================
   Lookups
   ========================================================================== */

func Lookup(domain, key string) (CodeEntry, bool) {
	if err := load(); err != nil {
		return CodeEntry{}, false
	}
	switch domain {
	case "lexer":
		if reg.Lexer == nil {
			return CodeEntry{}, false
		}
		if v, ok := reg.Lexer[key]; ok {
			return v.CodeEntry, true
		}
	case "parser":
		if reg.Parser == nil {
			return CodeEntry{}, false
		}
		if v, ok := reg.Parser[key]; ok {
			return v.CodeEntry, true
		}
	case "type":
		if reg.Type == nil {
			return CodeEntry{}, false
		}
		if v, ok := reg.Type[key]; ok {
			return v.CodeEntry, true
		}
	}
	return CodeEntry{}, false
}

func MustLookup(domain, key, defaultID, defaultTitle string) CodeEntry {
	if ce, ok := Lookup(domain, key); ok {
		return ce
	}
	return CodeEntry{ID: defaultID, Title: defaultTitle}
}

// LookupFull returns the code entry plus default primary_end and suggestions.
func LookupFull(domain, key string) (CodeFull, bool) {
	if err := load(); err != nil {
		return CodeFull{}, false
	}
	switch domain {
	case "lexer":
		if v, ok := reg.Lexer[key]; ok {
			return CodeFull{Entry: v.CodeEntry, PrimaryEnd: v.PrimaryEnd, Suggestions: v.Suggestions}, true
		}
	case "parser":
		if v, ok := reg.Parser[key]; ok {
			return CodeFull{Entry: v.CodeEntry, PrimaryEnd: v.PrimaryEnd, Suggestions: v.Suggestions}, true
		}
	case "type":
		if v, ok := reg.Type[key]; ok {
			return CodeFull{Entry: v.CodeEntry, PrimaryEnd: v.PrimaryEnd, Suggestions: v.Suggestions}, true
		}
	}
	return CodeFull{}, false
}

/* Convenience helpers for domains (optional) */

func LookupLexer(key string) (CodeEntry, bool)     { return Lookup("lexer", key) }
func LookupParser(key string) (CodeEntry, bool)    { return Lookup("parser", key) }
func LookupType(key string) (CodeEntry, bool)      { return Lookup("type", key) }
func LookupFullLexer(key string) (CodeFull, bool)  { return LookupFull("lexer", key) }
func LookupFullParser(key string) (CodeFull, bool) { return LookupFull("parser", key) }
func LookupFullType(key string) (CodeFull, bool)   { return LookupFull("type", key) }
