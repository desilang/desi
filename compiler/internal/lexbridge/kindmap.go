package lexbridge

import (
	"github.com/desilang/desi/compiler/internal/term"
	"strings"
)

// KindMap expresses how Desi token "Kind" strings should map to the Go lexer's
// kind naming (as printed by %s on the Go token's Kind). This is derived from
// your `lex-diff` output: Go prints keywords/symbols literally (e.g. "def", "import", ".", "(", ")"),
// while Desi uses generic "KW" with text field for keywords and named kinds for symbols.
type KindMap struct {
	// Map of Desi.Kind -> Go.Kind (string form as printed by the Go lexer)
	// For Desi KW, we take Text instead (see MapKind).
	Plain map[string]string

	// For Desi keyword "Text" -> Go.Kind
	Keywords map[string]string
}

// DefaultKindMap captures the mapping implied by your `lex-diff` sample.
func DefaultKindMap() KindMap {
	return KindMap{
		Plain: map[string]string{
			"IDENT":   "IDENT",
			"INT":     "INT",
			"STR":     "STR",
			"NEWLINE": "NEWLINE",
			"INDENT":  "INDENT",
			"DEDENT":  "DEDENT",
			"DOT":     ".",
			"LPAREN":  "(",
			"RPAREN":  ")",
			"COLON":   ":",
			"EQ":      "=",  // single '=' in Go stream
			"ARROW":   "->", // '->'
			"EQEQ":    "==",
			"NE":      "!=",
			"LE":      "<=",
			"GE":      ">=",
			"LT":      "<",
			"GT":      ">",
			"PLUS":    "+",
			"MINUS":   "-",
			"STAR":    "*",
			"SLASH":   "/",
			"PERCENT": "%",
			"PIPE":    "|>",  // if/when present in Go stream
			"EOF":     "EOF", // printed as TokEOF in code, but %s likely shows "EOF"
		},
		Keywords: map[string]string{
			"def":     "def",
			"import":  "import",
			"let":     "let",
			"mut":     "mut",
			"return":  "return",
			"while":   "while",
			"if":      "if",
			"elif":    "elif",
			"else":    "else",
			"true":    "true",
			"false":   "false",
			"and":     "and",
			"or":      "or",
			"not":     "not",
			"package": "package",
			"defer":   "defer",
		},
	}
}

// MapKind converts a Desi token (kind,text) into the Go lexer's *string-form*
// of the token kind, using the DefaultKindMap semantics.
// - If desiKind == "KW", we consult Keywords[text].
// - Else we consult Plain[desiKind].
// - Returns the mapped kind string and ok=false if unmapped.
func (km KindMap) MapKind(desiKind, text string) (goKind string, ok bool) {
	if desiKind == "KW" {
		g, ok := km.Keywords[text]
		return g, ok
	}
	g, ok := km.Plain[desiKind]
	return g, ok
}

// Coverage tracks how many Desi kinds/keywords we successfully mapped.
type Coverage struct {
	// distinct Desi kinds seen (including "KW")
	KindsSeen map[string]int
	// distinct KW texts seen (only populated when desiKind=="KW")
	KeywordsSeen map[string]int

	// number of tokens processed
	Total int
	// number that successfully mapped
	Mapped int
	// list of unmapped forms (deduped)
	MissingKinds    map[string]struct{} // for non-KW kinds
	MissingKeywords map[string]struct{} // for KW texts
}

// NewCoverage initializes coverage counters.
func NewCoverage() Coverage {
	return Coverage{
		KindsSeen:       map[string]int{},
		KeywordsSeen:    map[string]int{},
		MissingKinds:    map[string]struct{}{},
		MissingKeywords: map[string]struct{}{},
	}
}

// Tally updates coverage with one Desi token.
func (c *Coverage) Tally(desiKind, text string, mappedOK bool) {
	c.Total++
	c.KindsSeen[desiKind]++
	if desiKind == "KW" {
		c.KeywordsSeen[text]++
		if !mappedOK {
			c.MissingKeywords[text] = struct{}{}
		}
	} else if !mappedOK {
		c.MissingKinds[desiKind] = struct{}{}
	}
	if mappedOK {
		c.Mapped++
	}
}

// RenderReport returns a small human-readable summary.
func (c Coverage) RenderReport() string {
	// quick inline builder to avoid fmt.Fprintf linters
	var b strings.Builder
	term.Wprintf(&b, "mapped %d/%d tokens\n", c.Mapped, c.Total)
	term.Wprintf(&b, "distinct kinds seen: %d (KW counts as one)\n", len(c.KindsSeen))
	if len(c.KeywordsSeen) > 0 {
		term.Wprintf(&b, "distinct KW texts seen: %d\n", len(c.KeywordsSeen))
	}
	if len(c.MissingKinds) > 0 {
		term.Wprintf(&b, "missing non-KW kinds: ")
		first := true
		for k := range c.MissingKinds {
			if !first {
				term.Wprintf(&b, ", ")
			}
			first = false
			term.Wprintf(&b, "%s", k)
		}
		term.Wprintf(&b, "\n")
	}
	if len(c.MissingKeywords) > 0 {
		term.Wprintf(&b, "missing KW texts: ")
		first := true
		for k := range c.MissingKeywords {
			if !first {
				term.Wprintf(&b, ", ")
			}
			first = false
			term.Wprintf(&b, "%s", k)
		}
		term.Wprintf(&b, "\n")
	}
	return b.String()
}
