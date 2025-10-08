package build

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// importRef captures a single import occurrence in a file.
type importRef struct {
	Mod       string
	Line, Col int // 1-based visual columns (ASCII ok for .desi paths)
}

// graph tracks module → file and file → imports, plus diagnostics and cycles.
type graph struct {
	roots        []string
	fileToModule map[string]string
	moduleToFile map[string]string
	imports      map[string][]importRef // file -> imported modules with positions

	diags  []diag.Diagnostic
	cycles [][]string // list of cycles as sequences of modules
}

func newGraph(roots []string) *graph {
	// Prefer the most specific root (longest path) when deriving module names.
	rs := append([]string{}, roots...)
	sort.SliceStable(rs, func(i, j int) bool { return len(rs[i]) > len(rs[j]) })

	return &graph{
		roots:        rs,
		fileToModule: map[string]string{},
		moduleToFile: map[string]string{},
		imports:      map[string][]importRef{},
	}
}

func (g *graph) moduleNameForFile(abs string) string {
	abs = filepath.Clean(abs)
	for _, r := range g.roots {
		rel, err := filepath.Rel(r, abs)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return moduleFromRel(rel)
		}
	}
	return ""
}

func (g *graph) addFile(absFile, module string) {
	absFile = filepath.Clean(absFile)
	g.fileToModule[absFile] = module
	g.moduleToFile[module] = absFile
}

func (g *graph) walk(absFile string) {
	stack := []string{}
	seen := map[string]bool{}
	var dfs func(file string)
	dfs = func(file string) {
		file = filepath.Clean(file)
		if seen[file] {
			return
		}
		seen[file] = true
		stack = append(stack, file)

		// Scan imports for this file (valid refs + bad-import diags).
		imods, bads := scanImports(file)
		g.imports[file] = imods
		if len(bads) > 0 {
			g.diags = append(g.diags, bads...)
		}

		for _, r := range imods {
			// Resolved already?
			if mf := g.moduleToFile[r.Mod]; mf != "" {
				if inStack(mf, stack) {
					chain := g.describeCycle(mf, file, stack)
					g.cycles = append(g.cycles, chain)
					g.diags = append(g.diags, moduleImportCycleDiagAt(file, r.Line, r.Col, chain))
				}
				continue
			}
			// Resolve module to a file
			cands := moduleToCandidatePaths(r.Mod, g.roots)
			f := firstExistingFile(cands)
			if f == "" {
				// not found at this site
				g.diags = append(g.diags, moduleNotFoundDiagAt(r.Mod, file, r.Line, r.Col, cands))
				continue
			}
			g.addFile(f, r.Mod)
			dfs(f)
		}

		stack = stack[:len(stack)-1]
	}
	dfs(absFile)
}

func inStack(file string, stack []string) bool {
	file = filepath.Clean(file)
	for _, s := range stack {
		if filepath.Clean(s) == file {
			return true
		}
	}
	return false
}

func (g *graph) describeCycle(startFile, _curFile string, stack []string) []string {
	// Convert stack of files to modules to show a readable cycle chain.
	var mods []string
	for _, f := range stack {
		if m := g.fileToModule[f]; m != "" {
			mods = append(mods, m)
		} else {
			mods = append(mods, f)
		}
	}
	return mods
}

// topo returns a deterministic topological order of files (DFS post-order reverse).
func (g *graph) topo() []string {
	// Build file adjacency
	adj := map[string][]string{}
	for f, imods := range g.imports {
		for _, r := range imods {
			if mf := g.moduleToFile[r.Mod]; mf != "" {
				adj[f] = append(adj[f], mf)
			}
		}
	}
	// DFS post-order
	seen := map[string]bool{}
	var order []string
	var visit func(f string)
	visit = func(f string) {
		if seen[f] {
			return
		}
		seen[f] = true
		for _, d := range adj[f] {
			visit(d)
		}
		order = append(order, f)
	}
	// ensure stable iteration
	for f := range g.fileToModule {
		visit(f)
	}
	// reverse
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// --- Simple import scanning (lex-light, line oriented) ---

// Valid matchers (strict dotted identifiers)
var (
	// import foo.bar
	reImport = regexp.MustCompile(`^\s*import\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\b`)
	// from foo.bar import X
	reFrom = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s+import\b`)
)

// Broad matchers to catch invalid module tokens so we can diagnose them
var (
	reImportAny = regexp.MustCompile(`^\s*import\s+([^\s#]+)`)
	reFromAny   = regexp.MustCompile(`^\s*from\s+([^\s#]+)\s+import\b`)
)

func scanImports(file string) ([]importRef, []diag.Diagnostic) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil
	}
	var refs []importRef
	var diagsOut []diag.Diagnostic

	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		// strip trailing comment
		ln := raw
		if j := strings.IndexRune(ln, '#'); j >= 0 {
			ln = ln[:j]
		}
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}

		// First: valid strict matches
		if idx := reImport.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1 // 1-based
			refs = append(refs, importRef{Mod: mod, Line: i + 1, Col: col})
			continue
		}
		if idx := reFrom.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1
			refs = append(refs, importRef{Mod: mod, Line: i + 1, Col: col})
			continue
		}

		// Next: broad matchers to flag invalid module tokens
		if idx := reImportAny.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1
			if ok, why := isValidModulePath(mod); !ok {
				diagsOut = append(diagsOut, moduleBadImportDiagAt(mod, file, i+1, col, why))
			}
			continue
		}
		if idx := reFromAny.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1
			if ok, why := isValidModulePath(mod); !ok {
				diagsOut = append(diagsOut, moduleBadImportDiagAt(mod, file, i+1, col, why))
			}
			continue
		}
	}

	// Deduplicate refs (same (mod,line,col) triples)
	seen := map[string]struct{}{}
	var out []importRef
	for _, r := range refs {
		key := r.Mod + "|" + strconvI(r.Line) + ":" + strconvI(r.Col)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, r)
	}
	return out, diagsOut
}

func firstExistingFile(paths []string) string {
	for _, p := range paths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// tiny int→string without importing strconv for 2 calls
func strconvI(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = digits[n%10]
		n /= 10
	}
	return string(b[i:])
}

// Validate dotted module path; return (ok, whyIfNot)
func isValidModulePath(s string) (bool, string) {
	if strings.TrimSpace(s) == "" {
		return false, "empty module path"
	}
	if strings.HasPrefix(s, ".") || strings.HasSuffix(s, ".") {
		return false, "path cannot start or end with '.'"
	}
	if strings.Contains(s, "..") {
		return false, "empty module segment (consecutive dots)"
	}
	parts := strings.Split(s, ".")
	for _, p := range parts {
		if p == "" {
			return false, "empty module segment"
		}
		// first char
		r0 := p[0]
		if !((r0 >= 'A' && r0 <= 'Z') || (r0 >= 'a' && r0 <= 'z') || r0 == '_') {
			return false, "segment must start with a letter or '_'"
		}
		for i := 1; i < len(p); i++ {
			r := p[i]
			if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				return false, "invalid character in module segment"
			}
		}
	}
	return true, ""
}
