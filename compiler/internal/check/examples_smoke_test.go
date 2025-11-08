package check

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

func TestExamples_M10_Smoke(t *testing.T) {
	// Relative to this package: compiler/internal/check → ../../../examples
	exdir := filepath.Join("..", "..", "..", "examples")
	files := []string{
		"23_range_map_filter.desi",
		"24_membership_len.desi",
		"25_comprehensions_lowered.desi",
	}
	for _, name := range files {
		p := filepath.Join(exdir, name)
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		mod, pdiags := parse.ParseFile(p, src)
		if len(pdiags) != 0 {
			t.Fatalf("%s: parse diags: %+v", name, pdiags)
		}
		diags, _ := Check(mod)
		if len(diags) != 0 {
			t.Fatalf("%s: check diags: %+v", name, diags)
		}
	}
}
