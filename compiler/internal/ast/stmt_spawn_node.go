package ast

import "github.com/desilang/desi/compiler/internal/diag"

// SpawnStmt represents a concurrent task spawn: spawn: block or spawn(name="..."): block
type SpawnStmt struct {
	Name *StrLit // Optional task name (for debugging/tracing)
	Body *Block  // Code to execute concurrently
	Span diag.Span
}

func (*SpawnStmt) isStmt()             {}
func (s *SpawnStmt) SpanOf() diag.Span { return s.Span }
