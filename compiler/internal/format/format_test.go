package format

import (
	"bytes"
	"testing"
)

func TestFormatBytes_NormalizesBasicWhitespace(t *testing.T) {
	src := []byte("def main() -> int:\n\tprint(\"hi\")  \n\t0")
	got, diags := FormatBytes(src)
	if len(diags) > 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
	// Trailing spaces must be gone
	if bytes.Contains(got, []byte("  \n")) {
		t.Fatalf("expected trailing spaces to be trimmed, got:\n%s", string(got))
	}
	// Must end with a single newline
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Fatalf("missing trailing newline")
	}
	if bytes.HasSuffix(bytes.TrimRight(got, "\n"), []byte("\n")) {
		t.Fatalf("more than one trailing newline")
	}
}
