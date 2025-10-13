package diag

import (
	"encoding/json"
	"fmt"
	"io"
)

type SuggestionWhere struct {
	Kind string `json:"kind,omitempty"` // e.g. "eol"
}

type Suggestion struct {
	Where         SuggestionWhere `json:"where,omitempty"`
	Label         string          `json:"label,omitempty"`
	Message       string          `json:"message,omitempty"`
	Replacement   string          `json:"replacement,omitempty"`
	Applicability string          `json:"applicability,omitempty"` // "machine-applicable", etc.
}

type Entry struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Help       string `json:"help,omitempty"`
	PrimaryEnd *struct {
		Kind string `json:"kind,omitempty"`
	} `json:"primary_end,omitempty"`
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// Catalog mirrors your hierarchical JSON: lexer/parser/type/module/warn/codegen
type Catalog struct {
	Raw map[string]map[string]Entry
}

// LoadCatalog reads the hierarchical structure as-is.
func LoadCatalog(r io.Reader) (Catalog, error) {
	var raw map[string]map[string]Entry
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return Catalog{}, err
	}
	return Catalog{Raw: raw}, nil
}

// Get returns (Entry, ok) by dotted path, e.g., "lexer.unterminated_string".
func (c Catalog) Get(path string) (Entry, bool) {
	var top, key string
	n, _ := fmt.Sscanf(path, "%[^.].%s", &top, &key)
	if n != 2 {
		return Entry{}, false
	}
	group, ok := c.Raw[top]
	if !ok {
		return Entry{}, false
	}
	e, ok := group[key]
	return e, ok
}
