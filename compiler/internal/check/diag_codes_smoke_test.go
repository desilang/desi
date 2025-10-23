package check

import (
	"os"
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

func TestDiagCatalog_HasCheckerCodes(t *testing.T) {
	f, err := os.Open("compiler/internal/diag/codes.json")
	if err != nil {
		t.Fatalf("open codes.json: %v", err)
	}
	defer f.Close()

	cat, err := diag.LoadCatalog(f)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	expect := map[string]string{
		"type.no_matching_overload":  "DTE0101",
		"type.ambiguous_overload":    "DTE0102",
		"type.bad_pipeline_feed":     "DTE0103",
		"type.invalid_operand_types": "DTE0104",
		"type.not_callable":          "DTE0105",
		// sanity checks for existing ones we use
		"type.call_wrong_arity": "DTE0046",
		"type.undefined_name":   "DTE0001",
	}

	for path, wantID := range expect {
		e, ok := cat.Get(path)
		if !ok {
			t.Fatalf("missing entry in catalog: %s", path)
		}
		if e.ID != wantID {
			t.Fatalf("catalog ID mismatch for %s: got %s, want %s", path, e.ID, wantID)
		}
	}
}
