package diag

import "fmt"

// Pos marks a 1-based line/column location in a file.
type Pos struct{ Line, Col int }

// Span marks a half-open range [Start, End) within a file.
type Span struct {
	Start Pos
	End   Pos
}

// Diagnostic is a compiler message with an optional span.
type Diagnostic struct {
	Span Span
	Msg  string
}

func (d Diagnostic) Error() string {
	if d.Span.Start.Line == 0 {
		return d.Msg
	}
	return fmt.Sprintf("%d:%d: %s", d.Span.Start.Line, d.Span.Start.Col, d.Msg)
}
