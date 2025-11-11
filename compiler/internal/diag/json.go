package diag

import (
	"encoding/json"
	"io"
	"strings"
)

// JSON encoder for diagnostics.

type jsonPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
	Byte int `json:"byte"`
}

type jsonSpan struct {
	File  string  `json:"file"`
	Start jsonPos `json:"start"`
	End   jsonPos `json:"end"`
	Text  string  `json:"text,omitempty"` // used for primary
}

type jsonLabel struct {
	File  string  `json:"file"`
	Start jsonPos `json:"start"`
	End   jsonPos `json:"end"`
	Note  string  `json:"note,omitempty"`
}

type jsonDiagnostic struct {
	Code     string      `json:"code"`
	Domain   string      `json:"domain"`
	Title    string      `json:"title"`
	Message  string      `json:"message"`
	Severity string      `json:"severity"`
	Primary  jsonSpan    `json:"primary"`
	Labels   []jsonLabel `json:"labels,omitempty"`
	Notes    []string    `json:"notes,omitempty"`
	Help     string      `json:"help,omitempty"`
}

func toJSON(d Diagnostic) jsonDiagnostic {
	// Title/help from diagnostic or catalog fallback.
	title := strings.TrimSpace(d.Title)
	if title == "" {
		if e, ok := Lookup(d.CodeID); ok && e.Title != "" {
			title = e.Title
		}
	}
	help := strings.TrimSpace(d.Help)
	if help == "" {
		if e, ok := Lookup(d.CodeID); ok && e.Help != "" {
			help = e.Help
		}
	}
	msg := strings.TrimSpace(d.Message)
	if msg == "" {
		msg = title
	}
	jd := jsonDiagnostic{
		Code:     d.CodeID,
		Domain:   d.Domain,
		Title:    title,
		Message:  msg,
		Severity: "error",
		Primary: jsonSpan{
			File:  d.Primary.Span.File,
			Start: jsonPos{Line: d.Primary.Span.Start.Line, Col: d.Primary.Span.Start.Col, Byte: d.Primary.Span.Start.Byte},
			End:   jsonPos{Line: d.Primary.Span.End.Line, Col: d.Primary.Span.End.Col, Byte: d.Primary.Span.End.Byte},
			Text:  d.Primary.Text,
		},
		Help: help,
	}
	if len(d.Labels) > 0 {
		jd.Labels = make([]jsonLabel, 0, len(d.Labels))
		for _, l := range d.Labels {
			if l.Primary {
				continue
			}
			jd.Labels = append(jd.Labels, jsonLabel{
				File:  l.Span.File,
				Start: jsonPos{Line: l.Span.Start.Line, Col: l.Span.Start.Col, Byte: l.Span.Start.Byte},
				End:   jsonPos{Line: l.Span.End.Line, Col: l.Span.End.Col, Byte: l.Span.End.Byte},
				Note:  l.Text,
			})
		}
	}
	if len(d.Notes) > 0 {
		jd.Notes = append(jd.Notes, d.Notes...)
	}
	return jd
}

// EncodeJSON writes a JSON array of diagnostics to w (pretty-printed).
func EncodeJSON(w io.Writer, diags []Diagnostic) error {
	arr := make([]jsonDiagnostic, 0, len(diags))
	for _, d := range diags {
		arr = append(arr, toJSON(d))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(arr)
}
