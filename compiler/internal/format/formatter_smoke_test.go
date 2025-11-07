package format

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFormatter_Smoke_OnExamples(t *testing.T) {
	exdir := filepath.Join("..", "..", "..", "examples")
	files := []string{
		"14_m7_main.desi",
		"15_m8_async_basic.desi",
		"16_m8_async_lambda.desi",
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
		out, diags := FormatBytes(src)
		if len(diags) != 0 {
			t.Fatalf("%s: parse diags: %+v", name, diags)
		}
		out2, diags2 := FormatBytes(out)
		if len(diags2) != 0 {
			t.Fatalf("%s: parse diags on formatted: %+v", name, diags2)
		}
		if !bytes.Equal(out, out2) {
			t.Fatalf("%s: not idempotent", name)
		}
	}
}
