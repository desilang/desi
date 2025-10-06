package build

import (
	"os"
	"path/filepath"
)

// fileExists reports whether path exists and is a file.
func fileExists(path string) bool {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return true
	}
	return false
}

// firstExisting returns the first path that exists (file) or empty if none.
func firstExisting(paths []string) string {
	for _, p := range paths {
		if fileExists(p) {
			return p
		}
	}
	return ""
}
