package ast

import "github.com/desilang/desi/compiler/internal/diag"

// TypeAliasDecl represents `type Name<T> = TargetType`.
type TypeAliasDecl struct {
	Pub        bool             // public visibility
	Name       Ident            // alias name
	TypeParams []*TypeParamNode // optional <T, U, ...> type parameters
	Target     *TypeName        // the type being aliased
	Span       diag.Span
}

func (*TypeAliasDecl) isDecl()             {}
func (d *TypeAliasDecl) SpanOf() diag.Span { return d.Span }
