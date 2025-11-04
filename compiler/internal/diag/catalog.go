package diag

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"io"
	"strings"
	"sync"
)

// SuggestionWhere describes where an automatic fix should apply.
// JSON may contain fields like {"kind":"at","role":"primary"} or {"kind":"eol"}.
type SuggestionWhere struct {
	Kind string `json:"kind,omitempty"` // e.g., "at", "eol"
	Role string `json:"role,omitempty"` // e.g., "primary"
}

// Suggestion mirrors the structure stored in codes.json.
type Suggestion struct {
	Where         SuggestionWhere `json:"where,omitempty"`
	Label         string          `json:"label,omitempty"`
	Message       string          `json:"message,omitempty"`
	Replacement   string          `json:"replacement,omitempty"`
	Applicability string          `json:"applicability,omitempty"`
}

// Entry represents an individual diagnostic entry in the catalog.
// Some entries (e.g., lexer unterminated string) also include a top-level
// "primary_end" hint, which we must accept to keep json.DisallowUnknownFields happy.
type Entry struct {
	ID          string           `json:"id"`
	Title       string           `json:"title,omitempty"`
	Help        string           `json:"help,omitempty"`
	Suggestions []Suggestion     `json:"suggestions,omitempty"`
	PrimaryEnd  *SuggestionWhere `json:"primary_end,omitempty"`
}

// Catalog mirrors the hierarchical JSON structure: lexer/parser/type/module/warn/...
type Catalog struct {
	Raw map[string]map[string]Entry
}

// LoadCatalog decodes a catalog from a reader (used by tests that open codes.json on disk).
func LoadCatalog(r io.Reader) (Catalog, error) {
	var raw map[string]map[string]Entry
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return Catalog{}, err
	}
	return Catalog{Raw: raw}, nil
}

// Get returns (Entry, true) for a path like "parser.unexpected_token" or false if absent.
func (c Catalog) Get(path string) (Entry, bool) {
	top, key, ok := strings.Cut(path, ".")
	if !ok {
		return Entry{}, false
	}
	group, ok := c.Raw[strings.TrimSpace(top)]
	if !ok {
		return Entry{}, false
	}
	e, ok := group[strings.TrimSpace(key)]
	return e, ok
}

// ---- Embedded default catalog and helpers ----

//go:embed codes.json
var embeddedCodes []byte

var (
	defOnce sync.Once
	defCat  Catalog
	idIndex map[string]Entry // maps numeric ID, e.g., "DPE0110" -> Entry
	loadErr error
)

// ensureDefault loads the embedded catalog once and builds an ID index.
func ensureDefault() {
	defOnce.Do(func() {
		// Decode from embedded JSON
		c, err := LoadCatalog(bytes.NewReader(embeddedCodes))
		if err != nil {
			loadErr = err
			return
		}
		defCat = c
		// Build reverse index by Entry.ID for fast Lookup("DPE0110")
		idIndex = make(map[string]Entry, 256)
		for _, group := range defCat.Raw {
			for _, e := range group {
				if e.ID != "" {
					idIndex[e.ID] = e
				}
			}
		}
	})
}

// Lookup returns the catalog Entry for the given numeric code ID (e.g., "DPE0110").
func Lookup(id string) (Entry, bool) {
	ensureDefault()
	if loadErr != nil {
		return Entry{}, false
	}
	e, ok := idIndex[id]
	return e, ok
}

// Known reports whether id is defined in the embedded catalog.
func Known(id string) bool {
	_, ok := Lookup(id)
	return ok
}

// FillFromCatalog mutates d to populate Title/Help if they are empty and the ID exists.
//
// Note: we intentionally do NOT copy suggestions here; the renderer will
// read them from the catalog so we don't duplicate data into the Diagnostic.
func FillFromCatalog(d *Diagnostic) {
	if d == nil || d.CodeID == "" {
		return
	}
	entry, ok := Lookup(d.CodeID)
	if !ok {
		return
	}
	if d.Title == "" && entry.Title != "" {
		d.Title = entry.Title
	}
	if d.Help == "" && entry.Help != "" {
		d.Help = entry.Help
	}
}
