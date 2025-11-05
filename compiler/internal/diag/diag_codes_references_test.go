package diag

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// findRepoRoot walks up from this file to locate the repo root by finding go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	for i := 0; i < 10; i++ {
		mod := filepath.Join(dir, "go.mod")
		if fi, err := os.Stat(mod); err == nil && !fi.IsDir() {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not locate repo root (go.mod)")
	return ""
}

func Test_AllGoEmittedCodesExistInCatalog(t *testing.T) {
	root := findRepoRoot(t)
	target := filepath.Join(root, "compiler", "internal")

	// Match CodeID: "D??dddd" (e.g., DPE0110, DMW0004)
	codeRE := regexp.MustCompile(`CodeID:\s*"(?P<id>D[A-Z]{2,3}\d{4})"`)

	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Read file contents
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		m := codeRE.FindAllSubmatch(b, -1)
		for _, sm := range m {
			id := string(sm[1])
			if !Known(id) {
				t.Fatalf("unknown CodeID %q referenced in %s (not present in codes.json)", id, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk failed: %v", err)
	}
}
