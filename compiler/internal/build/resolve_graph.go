package build

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// graph tracks module → file and file → imports, plus diagnostics and cycles.
type graph struct {
	roots        []string
	fileToModule map[string]string
	moduleToFile map[string]string
	imports      map[string][]string // file -> imported module names (dotted)

	diags  []diag.Diagnostic
	cycles [][]string // list of cycles as sequences of modules
}

func newGraph(roots []string) *graph {
	return &graph{
		roots:        roots,
		fileToModule: map[string]string{},
		moduleToFile: map[string]string{},
		imports:      map[string][]string{},
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

func (g *graph) addFile(absFile, module string) error {
	absFile = filepath.Clean(absFile)
	g.fileToModule[absFile] = module
	g.moduleToFile[module] = absFile
	return nil
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
		imods := scanImports(file)
		g.imports[file] = imods

		for _, m := range imods {
			if mf := g.moduleToFile[m]; mf != "" {
				// already resolved
				if inStack(mf, stack) {
					// record cycle [ ... -> mf -> ... -> file -> ... ]
					g.cycles = append(g.cycles, g.describeCycle(mf, file, stack))
					g.diags = append(g.diags, moduleImportCycleDiag())
				}
				continue
			}
			// Resolve module to a file
			cands := moduleToCandidatePaths(m, g.roots)
			f := firstExisting(cands)
			if f == "" {
				// not found diag
				g.diags = append(g.diags, moduleNotFoundDiag(m, cands))
				continue
			}
			g.addFile(f, m)
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

func (g *graph) describeCycle(startFile, curFile string, stack []string) []string {
	// Convert stack of files to modules and slice the cycle.
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
		for _, m := range imods {
			if mf := g.moduleToFile[m]; mf != "" {
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

var (
	reImport = regexp.MustCompile(`^\s*import\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\b`)
	reFrom   = regexp.MustCompile(`^\s*from\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*)\s+import\b`)
)

func scanImports(file string) []string {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var mods []string
	lines := strings.Split(string(data), "\n")
	for _, ln := range lines {
		// strip trailing comment
		if i := strings.IndexRune(ln, '#'); i >= 0 {
			ln = ln[:i]
		}
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}
		if m := reImport.FindStringSubmatch(ln); len(m) == 2 {
			mods = append(mods, m[1])
			continue
		}
		if m := reFrom.FindStringSubmatch(ln); len(m) == 2 {
			mods = append(mods, m[1])
			continue
		}
	}
	return uniqStrings(mods)
}
