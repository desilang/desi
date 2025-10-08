package diag_test

import (
	"testing"

	"github.com/desilang/desi/compiler/internal/diag"
)

type dk struct{ domain, key string }

func TestCatalog_HasExpectedResolverKeys(t *testing.T) {
	checks := []dk{
		// resolver/module errors & warnings we reference
		{"module", "import_cycle"},
		{"module", "not_found"},
		{"module", "duplicate_import"},
		{"module", "bad_import"},
		{"module", "io_read"},
		{"module", "parse_failed"},
		{"module", "bad_entry"},
		{"module", "entry_not_found"},
		{"module", "internal"},
		{"module", "import_self"},
		{"module", "import_alias_conflict"},
	}

	for _, c := range checks {
		info, ok := diag.LookupFull(c.domain, c.key)
		if !ok {
			t.Fatalf("missing catalog entry for %s.%s", c.domain, c.key)
		}
		if info.Entry.ID == "" {
			t.Fatalf("catalog ID is empty for %s.%s", c.domain, c.key)
		}
		if info.Entry.Title == "" {
			t.Fatalf("catalog Title is empty for %s.%s", c.domain, c.key)
		}
	}
}
