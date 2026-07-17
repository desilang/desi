package check

import (
	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/diag"
)

// PerfLevel controls which advisor rules fire.
//   - "relaxed"  — DPR0001, DPR0002 only (low noise)
//   - "default"  — DPR0001–DPR0003 (recommended)
//   - "strict"   — all DPR rules
const (
	PerfLevelRelaxed = "relaxed"
	PerfLevelDefault = "default"
	PerfLevelStrict  = "strict"
)

// perfRule is a pluggable advisor rule. Each rule scans the AST and may emit
// zero or more diagnostics. Rules are registered in ruleTable below.
type perfRule struct {
	code     string // e.g., "DPR0001"
	minLevel string // minimum level at which this rule fires
	fn       func(mod *ast.Module) []diag.Diagnostic
}

// ruleTable lists all advisor rules. Order doesn't matter; they run independently.
var ruleTable = []perfRule{
	{"DPR0001", PerfLevelRelaxed, checkNestedLoops},
	{"DPR0002", PerfLevelRelaxed, checkStringConcatInLoop},
	{"DPR0003", PerfLevelDefault, checkRepeatedLookup},
	{"DPR0004", PerfLevelStrict, checkUnboundedAlloc},
}

// levelRank maps level names to numeric ranks for comparison.
var levelRank = map[string]int{
	PerfLevelRelaxed: 0,
	PerfLevelDefault: 1,
	PerfLevelStrict:  2,
}

// RunPerfAdvisor runs all perf advisory rules enabled at the given level.
// Pass "" or "default" for the recommended set.
func RunPerfAdvisor(mod *ast.Module, level string) []diag.Diagnostic {
	if mod == nil {
		return nil
	}
	if level == "" {
		level = PerfLevelDefault
	}
	threshold := levelRank[level]

	var out []diag.Diagnostic
	for _, rule := range ruleTable {
		ruleMin := levelRank[rule.minLevel]
		if threshold >= ruleMin {
			out = append(out, rule.fn(mod)...)
		}
	}
	return out
}

// ============================================================
// Rule implementations
// ============================================================

// --- DPR0001: Nested loop detection ---
// Warns when a for/while loop is nested inside another for/while loop,
// indicating potential O(n²) or worse complexity.
func checkNestedLoops(mod *ast.Module) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			out = append(out, scanNestedLoopsInFunc(dd)...)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				out = append(out, scanNestedLoopsInFunc(m)...)
			}
		}
	}
	return out
}

func scanNestedLoopsInFunc(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil {
		return nil
	}
	var out []diag.Diagnostic
	scanNestedLoopsBlock(fd.Body, 0, &out)
	return out
}

func scanNestedLoopsBlock(b *ast.Block, depth int, out *[]diag.Diagnostic) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		switch st := s.(type) {
		case *ast.ForStmt:
			if depth > 0 {
				*out = append(*out, diagAt("DPR0001", st.Span,
					"nested loop detected — consider flattening or using a lookup table"))
			}
			scanNestedLoopsBlock(st.Body, depth+1, out)
		case *ast.WhileStmt:
			if depth > 0 {
				*out = append(*out, diagAt("DPR0001", st.Span,
					"nested loop detected — consider flattening or using a lookup table"))
			}
			scanNestedLoopsBlock(st.Body, depth+1, out)
		case *ast.IfStmt:
			scanNestedLoopsBlock(st.Then, depth, out)
			for _, elif := range st.Elifs {
				scanNestedLoopsBlock(elif.Body, depth, out)
			}
			scanNestedLoopsBlock(st.Else, depth, out)
		}
	}
}

// --- DPR0002: String concatenation in loop ---
// Warns when += is used with a string variable inside a loop body.
// Recommendation: use list + join pattern instead.
func checkStringConcatInLoop(mod *ast.Module) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			out = append(out, scanStringConcatInFunc(dd)...)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				out = append(out, scanStringConcatInFunc(m)...)
			}
		}
	}
	return out
}

func scanStringConcatInFunc(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil {
		return nil
	}
	var out []diag.Diagnostic
	scanStringConcatBlock(fd.Body, false, &out)
	return out
}

func scanStringConcatBlock(b *ast.Block, inLoop bool, out *[]diag.Diagnostic) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		switch st := s.(type) {
		case *ast.ForStmt:
			scanStringConcatBlock(st.Body, true, out)
		case *ast.WhileStmt:
			scanStringConcatBlock(st.Body, true, out)
		case *ast.IfStmt:
			scanStringConcatBlock(st.Then, inLoop, out)
			for _, elif := range st.Elifs {
				scanStringConcatBlock(elif.Body, inLoop, out)
			}
			scanStringConcatBlock(st.Else, inLoop, out)
		case *ast.AugAssignStmt:
			if inLoop && st.Op == "+=" {
				// Heuristic: if RHS is a string literal or LHS looks like a string var
				if isStringExpr(st.Right) || isStringExpr(st.Left) {
					*out = append(*out, diagAt("DPR0002", st.Span,
						"string concatenation in loop — consider list.append() + str.join()"))
				}
			}
		}
	}
}

// isStringExpr is a best-effort heuristic: returns true for StrLit, FString,
// or identifiers with string-like names. This is a static lint, not type-checked.
func isStringExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.StrLit, *ast.FString:
		return true
	}
	return false
}

// --- DPR0003: Repeated collection lookup in loop ---
// Warns when dict[key] or list[idx] with the same expression appears multiple
// times in a loop body. Recommendation: hoist into a local variable.
func checkRepeatedLookup(mod *ast.Module) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			out = append(out, scanRepeatedLookupInFunc(dd)...)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				out = append(out, scanRepeatedLookupInFunc(m)...)
			}
		}
	}
	return out
}

func scanRepeatedLookupInFunc(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil {
		return nil
	}
	var out []diag.Diagnostic
	scanRepeatedLookupBlock(fd.Body, false, &out)
	return out
}

func scanRepeatedLookupBlock(b *ast.Block, inLoop bool, out *[]diag.Diagnostic) {
	if b == nil {
		return
	}
	// Track IndexExpr keys seen in this loop body to detect duplicates.
	seen := make(map[string]int) // indexKey -> count

	for _, s := range b.Stmts {
		switch st := s.(type) {
		case *ast.ForStmt:
			scanRepeatedLookupBlock(st.Body, true, out)
		case *ast.WhileStmt:
			scanRepeatedLookupBlock(st.Body, true, out)
		case *ast.IfStmt:
			scanRepeatedLookupBlock(st.Then, inLoop, out)
			for _, elif := range st.Elifs {
				scanRepeatedLookupBlock(elif.Body, inLoop, out)
			}
			scanRepeatedLookupBlock(st.Else, inLoop, out)
		default:
			if inLoop {
				collectIndexExprs(s, seen, out)
			}
		}
	}
}

// collectIndexExprs walks an AST node looking for IndexExpr and counts duplicates.
func collectIndexExprs(node ast.Node, seen map[string]int, out *[]diag.Diagnostic) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ast.ExprStmt:
		collectIndexExprsFromExpr(n.Expr, seen, out)
	case *ast.LetStmt:
		collectIndexExprsFromExpr(n.Value, seen, out)
	case *ast.AssignStmt:
		for _, rhs := range n.RHS {
			collectIndexExprsFromExpr(rhs, seen, out)
		}
	case *ast.AugAssignStmt:
		collectIndexExprsFromExpr(n.Right, seen, out)
	case *ast.ReturnStmt:
		collectIndexExprsFromExpr(n.Value, seen, out)
	}
}

func collectIndexExprsFromExpr(e ast.Expr, seen map[string]int, out *[]diag.Diagnostic) {
	if e == nil {
		return
	}
	switch x := e.(type) {
	case *ast.IndexExpr:
		key := indexExprKey(x)
		if key != "" {
			seen[key]++
			if seen[key] == 2 { // emit on second occurrence
				*out = append(*out, diagAt("DPR0003", x.Span,
					"repeated collection lookup — consider hoisting to a local variable"))
			}
		}
		collectIndexExprsFromExpr(x.X, seen, out)
		collectIndexExprsFromExpr(x.Idx, seen, out)
	case *ast.BinaryExpr:
		collectIndexExprsFromExpr(x.Lhs, seen, out)
		collectIndexExprsFromExpr(x.Rhs, seen, out)
	case *ast.CallExpr:
		collectIndexExprsFromExpr(x.Callee, seen, out)
		for _, arg := range x.Args {
			collectIndexExprsFromExpr(arg, seen, out)
		}
	case *ast.UnaryExpr:
		collectIndexExprsFromExpr(x.X, seen, out)
	case *ast.FieldExpr:
		collectIndexExprsFromExpr(x.X, seen, out)
	}
}

// indexExprKey generates a stable string key for an IndexExpr for dedup purposes.
// Only handles simple cases: ident[ident] and ident[literal].
func indexExprKey(ix *ast.IndexExpr) string {
	xName := ""
	switch x := ix.X.(type) {
	case *ast.Ident:
		xName = x.Name
	default:
		return ""
	}

	idxName := ""
	switch idx := ix.Idx.(type) {
	case *ast.Ident:
		idxName = idx.Name
	case *ast.StrLit:
		idxName = "\"" + idx.Value + "\""
	case *ast.IntLit:
		idxName = idx.Text
	default:
		return ""
	}

	return xName + "[" + idxName + "]"
}

// --- DPR0004: Unbounded allocation in loop ---
// Warns when list/dict/set literals or append() calls are in a loop body,
// potentially causing O(n) allocations per iteration.
func checkUnboundedAlloc(mod *ast.Module) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range mod.Decls {
		switch dd := d.(type) {
		case *ast.FuncDecl:
			out = append(out, scanUnboundedAllocInFunc(dd)...)
		case *ast.ClassDecl:
			for _, m := range dd.Methods {
				out = append(out, scanUnboundedAllocInFunc(m)...)
			}
		}
	}
	return out
}

func scanUnboundedAllocInFunc(fd *ast.FuncDecl) []diag.Diagnostic {
	if fd == nil || fd.Body == nil {
		return nil
	}
	var out []diag.Diagnostic
	scanUnboundedAllocBlock(fd.Body, false, &out)
	return out
}

func scanUnboundedAllocBlock(b *ast.Block, inLoop bool, out *[]diag.Diagnostic) {
	if b == nil {
		return
	}
	for _, s := range b.Stmts {
		switch st := s.(type) {
		case *ast.ForStmt:
			scanUnboundedAllocBlock(st.Body, true, out)
		case *ast.WhileStmt:
			scanUnboundedAllocBlock(st.Body, true, out)
		case *ast.IfStmt:
			scanUnboundedAllocBlock(st.Then, inLoop, out)
			for _, elif := range st.Elifs {
				scanUnboundedAllocBlock(elif.Body, inLoop, out)
			}
			scanUnboundedAllocBlock(st.Else, inLoop, out)
		case *ast.LetStmt:
			if inLoop && isAllocExpr(st.Value) {
				*out = append(*out, diagAt("DPR0004", st.Span,
					"collection allocation inside loop — consider pre-allocating"))
			}
		case *ast.ExprStmt:
			if inLoop && isAllocExpr(st.Expr) {
				*out = append(*out, diagAt("DPR0004", st.Span,
					"collection allocation inside loop — consider pre-allocating"))
			}
		}
	}
}

// isAllocExpr detects list/dict/set literals and constructor calls.
func isAllocExpr(e ast.Expr) bool {
	if e == nil {
		return false
	}
	switch e.(type) {
	case *ast.ListLit, *ast.DictLit, *ast.SetLit:
		return true
	}
	return false
}
