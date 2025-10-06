package build

import (
	"path/filepath"
	"strings"
)

// moduleToCandidatePaths returns possible absolute paths for a module under roots.
func moduleToCandidatePaths(mod string, roots []string) []string {
	rel := strings.ReplaceAll(mod, ".", string(filepath.Separator)) + ".desi"
	out := make([]string, 0, len(roots))
	for _, r := range roots {
		out = append(out, filepath.Join(r, rel))
	}
	return out
}

// moduleFromRel converts a relative path (without .desi) to dotted form.
func moduleFromRel(rel string) string {
	rel = strings.TrimSuffix(rel, ".desi")
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "" {
		return ""
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return strings.Join(parts, ".")
}
