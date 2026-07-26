// desirepl — interactive Desi REPL.
//
// Desi is compiled, so there is no interpreter to evaluate against. This
// REPL keeps a *session* (imports, declarations, and statements that have
// been accepted so far) and, for each new input, assembles a complete
// program, compiles it with `desic run`, and shows the result.
//
// Doing it this way means the REPL can never disagree with the compiler:
// it is the same front end, lowerer, and backend that build a real
// program. The cost is a compile per input (~0.5s) and the fact that
// statements kept in the session are re-executed each time — see :help.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/ast"
	"github.com/desilang/desi/compiler/internal/check"
	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/parse"
	"github.com/desilang/desi/compiler/internal/term"
	"github.com/desilang/desi/compiler/internal/types"
	"github.com/desilang/desi/compiler/internal/version"
)

// session holds everything the user has successfully entered, split by
// where it has to appear in a generated program.
type session struct {
	imports []string // `import x` — must precede declarations
	decls   []string // def/class/enum/struct/trait — top level
	stmts   []string // let/assignment/if/... — inside main()
	// baseline is the stdout of the session as it currently stands, so a
	// re-run's already-seen output can be stripped and only new output shown.
	baseline string
}

func main() {
	term.Println("desirepl", version.String())
	term.Println("Type expressions or statements. :help for commands, :quit to exit.")
	term.Println("")

	desic, err := findDesic()
	if err != nil {
		fmt.Fprintf(os.Stderr, "desirepl: %v\n", err)
		fmt.Fprintf(os.Stderr, "The REPL compiles each input, so it needs the desic binary alongside it.\n")
		os.Exit(1)
	}

	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	s := &session{}
	showAST := false

	for {
		term.Prompt(">>> ")
		line, ok := readLogical(in)
		if !ok {
			break
		}
		trimmed := strings.TrimSpace(line)

		switch trimmed {
		case "":
			continue
		case ":quit", ":q", "quit", "exit":
			term.Flush()
			return
		case ":help", ":h":
			printHelp()
			continue
		case ":reset":
			*s = session{}
			term.Println("(session cleared)")
			continue
		case ":session":
			printSession(s)
			continue
		case ":ast":
			showAST = true
			term.Println("(will show AST for next input)")
			continue
		}

		if showAST {
			showAST = false
			if mod, errs := parse.ParseFile("<repl>", []byte(wrapForAST(line))); len(errs) == 0 {
				ast.Print(os.Stdout, mod)
			} else {
				reportParseErrors(errs)
				continue
			}
		}

		evaluate(s, desic, line)
	}

	term.Flush()
}

// evaluate decides whether the input is a displayable expression or a
// statement, then compiles and runs the resulting program.
func evaluate(s *session, desic, input string) {
	kind := classify(input)

	// Imports and declarations are structural: they never produce a value.
	if kind == kindImport || kind == kindDecl {
		src := s.assemble(input, kind, false)
		if !typeChecks(src, true) {
			return
		}
		out, ok := run(desic, src)
		if !ok {
			return
		}
		s.accept(input, kind, out)
		emitNew(out, s.baseline)
		s.baseline = out
		return
	}

	// Check the input as written, then ask the checker what the trailing
	// expression's type is. Guessing instead (trying `print(<input>)` and
	// seeing if it checks) wrongly wraps a none-typed call: `print(x)`
	// became `print(print(x))`, which passes the checker but emits IR
	// referencing a value that does not exist.
	src := s.assemble(input, kindStmt, false)
	mod, info, ok := parseAndCheck(src, true)
	if !ok {
		return
	}

	display := false
	if t := trailingExprType(mod, info); t != nil && !types.Equal(t, types.None) {
		display = true
	}

	runSrc := src
	if display {
		runSrc = s.assemble(input, kindExpr, true)
	}
	out, ok := run(desic, runSrc)
	if !ok {
		return
	}
	if display {
		// A displayed expression is not retained: it has already been shown,
		// and replaying it would repeat any side effects.
		emitNew(out, s.baseline)
		return
	}
	s.accept(input, kindStmt, out)
	emitNew(out, s.baseline)
	s.baseline = out
}

// trailingExprType reports the type of the statement that assemble() placed
// just before main's closing `return 0`, or nil if it is not an expression.
func trailingExprType(mod *ast.Module, info *check.Info) types.T {
	if mod == nil || info == nil {
		return nil
	}
	for _, d := range mod.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "main" || fd.Body == nil {
			continue
		}
		stmts := fd.Body.Stmts
		if len(stmts) < 2 {
			return nil
		}
		es, ok := stmts[len(stmts)-2].(*ast.ExprStmt)
		if !ok {
			return nil
		}
		return info.Types[es.Expr]
	}
	return nil
}

type inputKind int

const (
	kindExpr inputKind = iota
	kindStmt
	kindDecl
	kindImport
)

var declKeywords = []string{"def ", "class ", "enum ", "struct ", "trait ", "impl ", "macro ", "type "}

func classify(input string) inputKind {
	head := strings.TrimSpace(input)
	if strings.HasPrefix(head, "import ") || strings.HasPrefix(head, "from ") {
		return kindImport
	}
	for _, kw := range declKeywords {
		if strings.HasPrefix(head, kw) {
			return kindDecl
		}
	}
	// Multi-line input is a block (if/while/for/...), never an expression.
	if strings.Contains(strings.TrimRight(input, "\n"), "\n") {
		return kindStmt
	}
	return kindExpr
}

// assemble builds a complete, compilable program from the session plus the
// new input. `wrapPrint` renders the input as print(<input>) so its value
// is displayed.
func (s *session) assemble(input string, kind inputKind, wrapPrint bool) string {
	var b strings.Builder

	for _, im := range s.imports {
		b.WriteString(im)
		b.WriteString("\n")
	}
	if kind == kindImport {
		b.WriteString(input)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	for _, d := range s.decls {
		b.WriteString(d)
		b.WriteString("\n\n")
	}
	if kind == kindDecl {
		b.WriteString(input)
		b.WriteString("\n\n")
	}

	b.WriteString("def main() -> int:\n")
	for _, st := range s.stmts {
		b.WriteString(indent(st))
	}
	switch kind {
	case kindStmt:
		b.WriteString(indent(input))
	case kindExpr:
		if wrapPrint {
			b.WriteString(indent("print(" + strings.TrimSpace(input) + ")"))
		} else {
			b.WriteString(indent(input))
		}
	}
	b.WriteString("\treturn 0\n")
	return b.String()
}

func (s *session) accept(input string, kind inputKind, out string) {
	switch kind {
	case kindImport:
		s.imports = append(s.imports, strings.TrimSpace(input))
	case kindDecl:
		s.decls = append(s.decls, strings.TrimRight(input, "\n"))
	case kindStmt:
		s.stmts = append(s.stmts, strings.TrimRight(input, "\n"))
	}
}

// indent shifts a (possibly multi-line) fragment one level into main().
func indent(fragment string) string {
	var b strings.Builder
	for _, ln := range strings.Split(strings.TrimRight(fragment, "\n"), "\n") {
		b.WriteString("\t")
		b.WriteString(ln)
		b.WriteString("\n")
	}
	return b.String()
}

// emitNew prints only the output the latest run added, so replaying the
// session does not repeat earlier output.
func emitNew(out, baseline string) {
	fresh := out
	if baseline != "" && strings.HasPrefix(out, baseline) {
		fresh = out[len(baseline):]
	}
	if fresh == "" {
		return
	}
	fmt.Print(fresh)
	if !strings.HasSuffix(fresh, "\n") {
		fmt.Println()
	}
}

// parseAndCheck parses and checks src, returning the module and type info.
// Warnings do not make an input invalid; anything else does.
func parseAndCheck(src string, report bool) (*ast.Module, *check.Info, bool) {
	mod, parseErrs := parse.ParseFile("<repl>", []byte(src))
	if len(parseErrs) > 0 {
		if report {
			reportParseErrors(parseErrs)
		}
		return nil, nil, false
	}
	diags, info := check.Check(mod)
	ok := true
	for _, d := range diags {
		if d.Domain == "warn" {
			continue
		}
		ok = false
		if report {
			renderDiag(d)
		}
	}
	return mod, info, ok
}

// typeChecks reports whether src is valid, surfacing any diagnostics.
func typeChecks(src string, report bool) bool {
	_, _, ok := parseAndCheck(src, report)
	return ok
}

// run compiles and executes src via desic, returning its stdout.
func run(desic, src string) (string, bool) {
	dir, err := os.MkdirTemp("", "desirepl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "  repl error: %v\n", err)
		return "", false
	}
	defer os.RemoveAll(dir)

	file := filepath.Join(dir, "repl_session.desi")
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "  repl error: %v\n", err)
		return "", false
	}

	cmd := exec.Command(desic, "run", file)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Surface whatever the compiler or the program reported; the input
		// is rejected so a failure cannot corrupt the session.
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		fmt.Fprintf(os.Stderr, "  %s\n", msg)
		return "", false
	}
	return stdout.String(), true
}

// findDesic locates the compiler next to this binary (how releases ship),
// falling back to PATH.
func findDesic() (string, error) {
	name := "desic"
	if os.PathSeparator == '\\' {
		name = "desic.exe"
	}
	if self, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(self), name)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("could not find %s next to desirepl or on PATH", name)
}

// readLogical reads one logical input. A line ending in ':' opens a block,
// which continues until a blank line, so multi-line defs work.
func readLogical(in *bufio.Scanner) (string, bool) {
	if !in.Scan() {
		return "", false
	}
	first := in.Text()
	if !strings.HasSuffix(strings.TrimSpace(first), ":") {
		return first, true
	}
	lines := []string{first}
	for {
		term.Prompt("... ")
		if !in.Scan() {
			break
		}
		next := in.Text()
		if strings.TrimSpace(next) == "" {
			break
		}
		lines = append(lines, next)
	}
	return strings.Join(lines, "\n"), true
}

func wrapForAST(line string) string {
	if classify(line) == kindDecl || classify(line) == kindImport {
		return line + "\n"
	}
	return "def __repl():\n" + indent(line)
}

func printHelp() {
	term.Println(":help     Show this message")
	term.Println(":session  Show the accumulated session")
	term.Println(":reset    Clear the session")
	term.Println(":ast      Show the AST for the next input")
	term.Println(":quit     Exit the REPL")
	term.Println("")
	term.Println("Desi is compiled, so each input is compiled and run together")
	term.Println("with everything accepted before it. Declarations and statements")
	term.Println("are kept and therefore re-run each time; expressions are shown")
	term.Println("once and not retained.")
}

func printSession(s *session) {
	if len(s.imports) == 0 && len(s.decls) == 0 && len(s.stmts) == 0 {
		term.Println("(empty session)")
		return
	}
	for _, im := range s.imports {
		term.Println(im)
	}
	for _, d := range s.decls {
		term.Println(d)
	}
	for _, st := range s.stmts {
		term.Println(st)
	}
}

func reportParseErrors(errs []diag.Diagnostic) {
	for _, e := range errs {
		msg := e.Message
		if msg == "" {
			msg = e.Title
		}
		fmt.Fprintf(os.Stderr, "  parse error: %s\n", msg)
	}
}

func renderDiag(d diag.Diagnostic) {
	prefix := d.Domain
	switch d.Domain {
	case "type":
		prefix = "type error"
	case "warn":
		prefix = "warning"
	case "class":
		prefix = "class error"
	case "borrow":
		prefix = "borrow error"
	case "call":
		prefix = "call error"
	case "collections":
		prefix = "collection error"
	case "numeric":
		prefix = "numeric error"
	case "ffi":
		prefix = "ffi error"
	case "sync":
		prefix = "concurrency error"
	case "module":
		prefix = "module error"
	case "project":
		prefix = "project error"
	}
	msg := d.Message
	if msg == "" {
		msg = d.Title
	}
	if d.CodeID != "" {
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s\n", d.CodeID, prefix, msg)
	} else {
		fmt.Fprintf(os.Stderr, "  %s: %s\n", prefix, msg)
	}
}
