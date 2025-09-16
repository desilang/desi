package ast

// Pos marks a 1-based line/column location in a file.
type Pos struct{ Line, Col int }

// Span marks a half-open range [Start, End).
// End may be {0,0} or equal to Start if unknown.
type Span struct {
	Start Pos
	End   Pos
}
