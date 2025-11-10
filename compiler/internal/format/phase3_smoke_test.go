package format

import (
	"bytes"
	"testing"

	"github.com/desilang/desi/compiler/internal/term"
)

// Stresses: EOL comments, long triple-quoted docstrings, and idempotence.
func TestFormatter_Smoke_Phase3Synthetic(t *testing.T) {
	src := "" +
		"def f() -> int:\n" +
		"\t\"\"\"doc line 1\n" +
		"\tdoc line 2\"\"\"\n" +
		"\t(\"a\" in \"abc\")    # membership eol\n" +
		"\tlen(\"abc\")          # len eol\n" +
		"\tprint(\"a#notcomment\")\n" +
		"\t0\n"

	out1, diags1 := FormatBytes([]byte(src))
	if len(diags1) != 0 {
		term.Eprintln("---- SRC (visible) ----")
		term.Eprint(visibleWithLineNos([]byte(src)))
		term.Eprintf("parse diags on src: %+v\n", diags1)
		term.Flush()
		t.Fatalf("parse diags on src: %+v", diags1)
	}
	out2, diags2 := FormatBytes(out1)
	if len(diags2) != 0 {
		term.Eprintln("---- PASS1 (visible) ----")
		term.Eprint(visibleWithLineNos(out1))
		term.Eprintf("parse diags on first formatted: %+v\n", diags2)
		term.Flush()
		t.Fatalf("parse diags on first formatted: %+v", diags2)
	}
	if !bytes.Equal(out1, out2) {
		term.Eprintln("==== PHASE3 NON-IDEMPOTENT ====")
		term.Eprintln("---- PASS1 (visible) ----")
		term.Eprint(visibleWithLineNos(out1))
		term.Eprintln("---- PASS2 (visible) ----")
		term.Eprint(visibleWithLineNos(out2))
		term.Flush()
		t.Fatalf("not idempotent on phase3 synthetic")
	}
	if !bytes.Contains(out1, []byte("# membership eol")) || !bytes.Contains(out1, []byte("# len eol")) {
		term.Eprintln("---- OUT (visible) ----")
		term.Eprint(visibleWithLineNos(out1))
		term.Flush()
		t.Fatalf("EOL comments not preserved")
	}
}
