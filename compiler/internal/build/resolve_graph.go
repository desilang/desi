package build

import (
	"os"
	"path/filepath"
	"regexp"
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
	return &graph{
		roots:        roots,
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

		// Scan imports for this file
		imods := scanImports(file)   // valid dotted modules
		bads := scanBadImports(file) // malformed module specs
		g.imports[file] = imods

		// Emit bad-import diagnostics directly; keep walking so users see all issues.
		for _, r := range bads {
			g.diags = append(g.diags, moduleBadImportDiagAt(r.Mod, file, r.Line, r.Col))
		}

		for _, r := range imods {
			// Resolved already?
			if mf := g.moduleToFile[r.Mod]; mf != "" {
				if inStack(mf, stack) {
					// cycle at this site
					g.cycles = append(g.cycles, g.describeCycle(mf, file, stack))
					g.diags = append(g.diags, moduleImportCycleDiagAt(file, r.Line, r.Col, g.describeCycle(mf, file, stack)))
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

// --- Import scanning ---------------------------------------------------------

// Valid dotted module: a.b.c with Python-ish identifier rules.
var dottedModRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

func isValidDottedModule(s string) bool { return dottedModRE.MatchString(strings.TrimSpace(s)) }

// Narrow matchers (only valid modules) — used to build the dependency graph.
var (
	reImportValid = regexp.MustCompile(`^\s*import\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\b`)
	reFromValid   = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s+import\b`)
)

// Broad matchers (capture candidate, even if invalid) — used for diagnostics.
var (
	reImportAny = regexp.MustCompile(`^\s*import\s+([^\s#]+)`)
	reFromAny   = regexp.MustCompile(`^\s*from\s+([^\s#]+)\s+import\b`)
)

func scanImports(file string) []importRef {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var refs []importRef
	lines := strings.Split(string(data), "\n")
	for i, ln := range lines {
		// strip trailing comment
		if j := strings.IndexRune(ln, '#'); j >= 0 {
			ln = ln[:j]
		}
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}
		// Try `import X` (valid only)
		if idx := reImportValid.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1 // 1-based
			refs = append(refs, importRef{Mod: mod, Line: i + 1, Col: col})
			continue
		}
		// Try `from X import ...` (valid only)
		if idx := reFromValid.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			mod := ln[idx[2]:idx[3]]
			col := idx[2] + 1
			refs = append(refs, importRef{Mod: mod, Line: i + 1, Col: col})
			continue
		}
	}
	// Deduplicate (same (mod,line,col) triples)
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
	return out
}

// scanBadImports finds import sites where the module spec is malformed.
func scanBadImports(file string) []importRef {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var refs []importRef
	lines := strings.Split(string(data), "\n")
	for i, ln := range lines {
		// strip trailing comment
		if j := strings.IndexRune(ln, '#'); j >= 0 {
			ln = ln[:j]
		}
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}
		if idx := reImportAny.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			cand := ln[idx[2]:idx[3]]
			if !isValidDottedModule(cand) {
				col := idx[2] + 1
				refs = append(refs, importRef{Mod: cand, Line: i + 1, Col: col})
			}
			continue
		}
		if idx := reFromAny.FindStringSubmatchIndex(ln); len(idx) >= 4 {
			cand := ln[idx[2]:idx[3]]
			if !isValidDottedModule(cand) {
				col := idx[2] + 1
				refs = append(refs, importRef{Mod: cand, Line: i + 1, Col: col})
			}
			continue
		}
	}
	// Deduplicate
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
	return out
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
