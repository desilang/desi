package build

import (
	"path/filepath"
	"strings"
)

// moduleToCandidatePaths returns possible absolute paths for a module under roots.
//
// New behavior (to support Python-like packages):
//   - Try "<root>/<a>/<b>/<c>.desi"
//   - Also try "<root>/<a>/<b>/<c>/mod.desi"
//
// This allows both flat modules (math.desi) and package dirs (math/mod.desi).
func moduleToCandidatePaths(mod string, roots []string) []string {
	relDir := strings.ReplaceAll(mod, ".", string(filepath.Separator))
	out := make([]string, 0, len(roots)*2)
	for _, r := range roots {
		out = append(out, filepath.Join(r, relDir+".desi"))
		out = append(out, filepath.Join(r, relDir, "mod.desi"))
	}
	return out
}

// moduleFromRel converts a relative path (possibly ending with ".desi" or "/mod.desi")
// to dotted form ("a.b.c").
//
// Examples:
//
//	"foo/bar.desi"      -> "foo.bar"
//	"foo/bar/mod.desi"  -> "foo.bar"
func moduleFromRel(rel string) string {
	rel = filepath.Clean(rel)
	// strip top-level .desi or .../mod.desi
	if strings.HasSuffix(rel, string(filepath.Separator)+"mod.desi") {
		rel = strings.TrimSuffix(rel, string(filepath.Separator)+"mod.desi")
	} else {
		rel = strings.TrimSuffix(rel, ".desi")
	}
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return strings.Join(parts, ".")
}
