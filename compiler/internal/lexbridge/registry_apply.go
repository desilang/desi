package lexbridge

import (
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// We maintain a parallel slice of keys for the diags parsed (same order).
var diagKeys []string

func applyRegistry(d *Diagnostic, fileData []byte) {
	var key string
	if len(diagKeys) > 0 {
		key = diagKeys[0]
		diagKeys = diagKeys[1:]
	}

	// Fallback mapping by substring (compat path)
	if key == "" {
		msgLower := strings.ToLower(d.Message)
		if strings.Contains(msgLower, "unterminated string") {
			key = "unterminated_string"
		}
	}
	if key == "" {
		return // unknown; leave plain
	}

	// Lookup full definition
	cf, ok := diag.LookupFull("lexer", key)
	if !ok {
		return
	}

	// Fill code/title/help
	if cf.Entry.ID != "" {
		d.Code = cf.Entry.ID
	}
	if cf.Entry.Title != "" {
		d.Message = cf.Entry.Title
	}
	if d.Help == "" && strings.TrimSpace(cf.Entry.Help) != "" {
		d.Help = cf.Entry.Help
	}

	// Shape primary end from JSON (e.g., eol)
	if endCol, okCol := primaryEndFromWhereSpec(cf.PrimaryEnd, d.Primary, fileData); okCol && endCol > d.Primary.Col {
		d.Primary.EndCol = endCol
	}

	// Materialize JSON suggestions
	for _, s := range cf.Suggestions {
		sp, ok := placeFromWhereSpec(s.Where, d.Primary, fileData)
		if !ok {
			continue
		}
		d.Suggest = append(d.Suggest, Suggestion{
			At:            sp,
			Replacement:   s.Replacement,
			Message:       s.Message,
			Applicability: mapApplicability(s.Applicability),
		})
		// carry label onto the underline span
		d.Suggest[len(d.Suggest)-1].At.Label = s.Label
	}
}

func mapApplicability(s string) Applicability {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "machine-applicable":
		return MachineApplicable
	case "has-placeholders":
		return HasPlaceholders
	case "maybe-incorrect":
		return MaybeIncorrect
	default:
		return MaybeIncorrect
	}
}

func primaryEndFromWhereSpec(w diag.WhereSpec, primary Span, fileData []byte) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(w.Kind)) {
	case "eol":
		if fileData == nil || primary.Line <= 0 || primary.File == "" {
			return 0, false
		}
		line := getLineText(fileData, primary.Line)
		endVis := len(visualize(line))
		if endVis == 0 {
			return 0, false
		}
		// EndCol is exclusive; +1 highlights through the last visible char.
		return endVis + 1, true
	case "primary_offset":
		col := primary.Col + w.Delta
		if col < 1 {
			col = 1
		}
		return col, true
	case "pos":
		if w.Line == primary.Line && w.Col > 0 {
			return w.Col, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func placeFromWhereSpec(w diag.WhereSpec, primary Span, fileData []byte) (Span, bool) {
	switch strings.ToLower(strings.TrimSpace(w.Kind)) {
	case "eol":
		if fileData == nil || primary.Line <= 0 || primary.File == "" {
			return Span{}, false
		}
		line := getLineText(fileData, primary.Line)
		endVis := len(visualize(line))
		if endVis == 0 {
			return Span{}, false
		}
		return Span{
			File:   primary.File,
			Line:   primary.Line,
			Col:    endVis + 1,
			EndCol: 0,
		}, true
	case "primary_offset":
		col := primary.Col + w.Delta
		if col < 1 {
			col = 1
		}
		return Span{
			File:   primary.File,
			Line:   primary.Line,
			Col:    col,
			EndCol: 0,
		}, true
	case "pos":
		if primary.File == "" || w.Line <= 0 || w.Col <= 0 {
			return Span{}, false
		}
		return Span{
			File:   primary.File,
			Line:   w.Line,
			Col:    w.Col,
			EndCol: 0,
		}, true
	default:
		return Span{}, false
	}
}
