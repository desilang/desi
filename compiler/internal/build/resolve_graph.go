package build

import (
	"fmt"
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

// graph builds a simple import graph and collects diagnostics.
type graph struct {
	roots        []string               // search roots (project first, then ancestors, then std)
	fileToModule map[string]string      // abs file -> dotted module name
	moduleToFile map[string]string      // dotted module name -> abs file
	imports      map[string][]importRef // file -> imported modules with positions

	diags  []diag.Diagnostic
	cycles [][]string // cycles as sequences of module names (A → B → … → A)
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
	// Best-effort fallback: basename-without-ext
	base := strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	return base
}

func (g *graph) addFile(abs, mod string) {
	abs = filepath.Clean(abs)
	g.fileToModule[abs] = mod
	g.moduleToFile[mod] = abs
}

// Resolve a module name to a file; emit diagnostics on conflicts/not-found.
// Returns the absolute file path (or empty string if unresolved).
func (g *graph) resolveModuleAt(mod, atFile string, line, col int) string {
	rel := strings.ReplaceAll(mod, ".", string(filepath.Separator))

	// Detect file-vs-package conflict per root.
	for _, root := range g.roots {
		fileCand := filepath.Join(root, rel+".desi")
		pkgCand := filepath.Join(root, rel, "mod.desi")
		if fileExists(fileCand) && fileExists(pkgCand) {
			g.diags = append(g.diags, moduleFilePackageConflictDiagAt(
				mod, atFile, line, col, fileCand, pkgCand,
			))
			return "" // treat as unresolved (hard error)
		}
	}

	// Try all candidates in priority order.
	cands := moduleToCandidatePaths(mod, g.roots)
	if fpath := firstExistingFile(cands); fpath != "" {
		return filepath.Clean(fpath)
	}

	// Not found: attach "looked for" notes.
	g.diags = append(g.diags, moduleNotFoundDiagAt(mod, atFile, line, col, cands))
	return ""
}

// Build a readable cycle chain of module names from `start` to `cur` and back to `start`.
func (g *graph) cycleChain(start, cur string, parent map[string]string) []string {
	var chainFiles []string
	// Walk back from cur to start via parents.
	v := filepath.Clean(cur)
	for {
		chainFiles = append(chainFiles, v)
		if v == start || v == "" {
			break
		}
		p, ok := parent[v]
		if !ok || p == v {
			break
		}
		v = p
	}
	// close the loop by appending start again
	if len(chainFiles) == 0 || chainFiles[len(chainFiles)-1] != start {
		chainFiles = append(chainFiles, start)
	}

	// Convert to module names (best-effort).
	var mods []string
	for _, f := range chainFiles {
		m := g.fileToModule[f]
		if m == "" {
			m = g.moduleNameForFile(f)
		}
		mods = append(mods, m)
	}
	return mods
}

func (g *graph) walk(entryFile string) {
	entryFile = filepath.Clean(entryFile)

	// DFS state: 0 = unseen, 1 = visiting, 2 = done
	state := map[string]int{}
	parent := map[string]string{} // child file -> parent file (for cycle chains)
	var order []string

	var dfs func(string)
	dfs = func(file string) {
		file = filepath.Clean(file)
		if state[file] == 2 {
			return
		}
		if state[file] == 1 {
			return
		}
		state[file] = 1

		// Scan imports in this file (and record any bad-import diags).
		imods, bads := scanImports(file)
		g.imports[file] = imods
		if len(bads) > 0 {
			g.diags = append(g.diags, bads...)
		}

		for _, r := range g.imports[file] {
			target := g.resolveModuleAt(r.Mod, file, r.Line, r.Col)
			if target == "" {
				// conflict / not-found already diagnosed
				continue
			}
			g.addFile(target, r.Mod)

			switch state[target] {
			case 0: // unseen
				parent[target] = file
				dfs(target)
			case 1: // back edge → cycle
				chain := g.cycleChain(target, file, parent)
				g.cycles = append(g.cycles, chain)
				g.diags = append(g.diags, moduleImportCycleDiagAt(file, r.Line, r.Col, chain))
			case 2: // done
			}
		}

		state[file] = 2
		order = append(order, file)
	}

	dfs(entryFile)
}

// Return files in reverse post-order (topo): dependencies before dependents.
func (g *graph) topo() []string {
	seen := map[string]bool{}
	var order []string
	var visit func(string)
	visit = func(f string) {
		f = filepath.Clean(f)
		if seen[f] {
			return
		}
		seen[f] = true
		for _, r := range g.imports[f] {
			if mf := g.moduleToFile[r.Mod]; mf != "" {
				visit(mf)
			}
		}
		order = append(order, f)
	}
	for f := range g.imports {
		visit(f)
	}
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

		// Then: broad matches for invalid module paths (diagnose)
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
	if len(refs) > 1 {
		sort.SliceStable(refs, func(i, j int) bool {
			if refs[i].Mod != refs[j].Mod {
				return refs[i].Mod < refs[j].Mod
			}
			if refs[i].Line != refs[j].Line {
				return refs[i].Line < refs[j].Line
			}
			return refs[i].Col < refs[j].Col
		})
		k := 1
		for k < len(refs) {
			if refs[k] == refs[k-1] {
				refs = append(refs[:k], refs[k+1:]...)
				continue
			}
			k++
		}
	}

	return refs, diagsOut
}

func firstExistingFile(paths []string) string {
	for _, p := range paths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// Validate dotted module path: a.b.c with segments [A-Za-z_][A-Za-z0-9_]*
func isValidModulePath(mod string) (bool, string) {
	if mod == "" {
		return false, "empty module name"
	}
	if strings.Contains(mod, "/") {
		return false, "slashes not allowed; use dotted identifiers"
	}
	parts := strings.Split(mod, ".")
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

// moduleFilePackageConflictDiagAt reports when a module path corresponds to both a single file
// and a package directory within the same root.
func moduleFilePackageConflictDiagAt(mod, file string, line, col int, filePath, pkgPath string) diag.Diagnostic {
	ce, _ := diag.LookupFull("module", "file_package_conflict")
	code := ce.Entry.ID
	title := ce.Entry.Title
	if code == "" {
		code = "DME0007"
	}
	if title == "" {
		title = "module resolves to both a file and a package"
	}
	short := filepath.Clean(file)
	msg := fmt.Sprintf("%s: %q (at %s:%d:%d)", title, mod, short, line, col)
	d := diag.Diagnostic{
		Domain:  "module",
		Key:     "file_package_conflict",
		Level:   diag.LevelError,
		Code:    code,
		Message: msg,
	}
	d.Notes = append(d.Notes, "file candidate: "+filePath)
	d.Notes = append(d.Notes, "package candidate: "+pkgPath)
	return d
}
