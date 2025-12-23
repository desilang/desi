package ast

import "github.com/desilang/desi/compiler/internal/diag"

// SelectStmt represents a select statement for multiplexing channel operations.
// Similar to Go's select, it waits on multiple channel operations and executes
// the first one that becomes ready.
//
// Syntax:
//
//	select:
//	    case msg = rx.recv():
//	        handle(msg)
//	    case tx.send(value):
//	        pass
//	    default:
//	        print("nothing ready")
type SelectStmt struct {
	Cases   []SelectCase
	Default []Stmt // Default case body (nil if no default)
	Span    diag.Span
}

// SelectCase represents one case in a select statement.
// Either a receive or send operation on a channel.
type SelectCase struct {
	// For recv: Binding is the variable to bind, Op is the recv expression
	// For send: Binding is empty, Op is the send expression
	Binding *Ident // Variable to bind result (nil for send)
	Op      Expr   // The channel operation (CallExpr for recv/send)
	Body    []Stmt // Statements to execute if this case is selected
	Span    diag.Span
}

func (s *SelectStmt) SpanOf() diag.Span { return s.Span }

// --- satisfy the Stmt interface ---
func (*SelectStmt) isStmt() {}
