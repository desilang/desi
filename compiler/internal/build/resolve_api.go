package build

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// ResolveOptions controls search roots.
type ResolveOptions struct {
	AllowStd bool
	Roots    []string
}

// Unit identifies a compilation unit by dotted module name and absolute file path.
type Unit struct {
	Module string
	File   string
}

// Plan is a topologically ordered list of units: Entry first, then its deps.
type Plan struct {
	Entry Unit
	Deps  []Unit // includes Entry as the first element
}

func ResolveEntry(entryPath string, opts ResolveOptions) (Plan, []diag.Diagnostic, error) {
	var plan Plan
	var diags []diag.Diagnostic

	if entryPath == "" {
		return plan, diags, ModuleErrorf("bad_entry", "DME0006", "bad entry path",
			"resolve: entry path is empty")
	}
	entryAbs, err := filepath.Abs(entryPath)
	if err != nil {
		return plan, diags, ErrBadEntryPath(entryPath, err)
	}
	fi, err := os.Stat(entryAbs)
	if err != nil || fi.IsDir() {
		return plan, diags, ErrEntryNotFound(entryAbs)
	}

	roots := uniqStrings(append([]string{}, opts.Roots...))
	auto := autoRoots(filepath.Dir(entryAbs))
	roots = append(roots, auto...)
	roots = filterExistingDirs(roots)
	if len(roots) == 0 {
		roots = []string{filepath.Dir(entryAbs)}
	}

	g := newGraph(roots)
	entryMod := g.moduleNameForFile(entryAbs)
	if entryMod == "" {
		entryMod = strings.TrimSuffix(filepath.Base(entryAbs), filepath.Ext(entryAbs))
	}
	g.addFile(entryAbs, entryMod)
	g.walk(entryAbs)

	diags = append(diags, g.diags...)

	if len(g.cycles) > 0 {
		plan = Plan{Entry: Unit{Module: entryMod, File: entryAbs}, Deps: []Unit{{Module: entryMod, File: entryAbs}}}
		return plan, diags, nil
	}

	order := g.topo()
	var deps []Unit
	seen := map[string]bool{}
	for _, f := range order {
		m := g.fileToModule[f]
		if m == "" {
			m = g.moduleNameForFile(f)
		}
		if !seen[f] {
			deps = append(deps, Unit{Module: m, File: f})
			seen[f] = true
		}
	}
	if len(deps) == 0 || filepath.Clean(deps[0].File) != filepath.Clean(entryAbs) {
		deps = append([]Unit{{Module: entryMod, File: entryAbs}}, deps...)
	}
	plan = Plan{Entry: deps[0], Deps: deps}
	return plan, diags, nil
}

// ---------- helpers: roots & discovery ----------

func uniqStrings(in []string) []string {
	m := map[string]struct{}{}
	var out []string
	for _, s := range in {
		c := filepath.Clean(s)
		if _, ok := m[c]; !ok {
			m[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

func filterExistingDirs(in []string) []string {
	var out []string
	for _, d := range in {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// Exported helper for the checker: does a module path exist when resolving at `atFile`?
// This respects project root, std root, and DESI_PATH the same way the main resolver does.
// It only answers existence (true/false); it does not emit diagnostics.
func ModuleExistsAt(atFile, mod string) bool {
	atDir := filepath.Dir(atFile)
	roots := autoRoots(atDir)
	if len(roots) == 0 {
		roots = []string{atDir}
	}
	cands := moduleToCandidatePaths(mod, roots)
	return firstExistingFile(cands) != ""
}

// findProjectRoot walks up from startDir to find a directory containing "desi.conf".
func findProjectRoot(startDir string) string {
	dir := startDir
	for i := 0; i < 16; i++ { // safety guard
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		if st, err := os.Stat(filepath.Join(dir, "desi.conf")); err == nil && !st.IsDir() {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return ""
}

// findStdRoot walks up to locate "<ancestor>/compiler/lib/std".
func findStdRoot(startDir string) string {
	dir := startDir
	for i := 0; i < 16; i++ {
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		std := filepath.Join(dir, "compiler", "lib", "std")
		if st, err := os.Stat(std); err == nil && st.IsDir() {
			return std
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}
	return ""
}

// autoRoots builds the default search path with priority:
//
//	[projectRoot (if any), entryDir, ancestor dirs..., stdRoot]
func autoRoots(entryDir string) []string {
	var roots []string

	if pr := findProjectRoot(entryDir); pr != "" {
		roots = append(roots, pr)
	}
	roots = append(roots, entryDir)

	dir := entryDir
	for i := 0; i < 12; i++ {
		if dir == "" || dir == "/" || dir == "." {
			break
		}
		roots = append(roots, dir)
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}

	if std := findStdRoot(entryDir); std != "" {
		roots = append(roots, std)
	}

	roots = uniqStrings(roots)
	return roots
}
