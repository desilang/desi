package parse

import "testing"

func TestDeferRequiresCall_Diag(t *testing.T) {
	src := "def h():\n\tdefer x\n"
	_, diags := ParseFile("<mem>", []byte(src))
	if len(diags) == 0 {
		t.Fatalf("expected a diagnostic for non-call defer")
	}
	found := false
	for _, d := range diags {
		if d.CodeID == "DPE0002" && d.Message != "" &&
			// keep it loose; message is stable enough to key on:
			// "defer requires a call expression like: defer fn(...)"
			// but we only check substring
			contains(d.Message, "defer") && contains(d.Message, "call") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected DPE0002 about defer needing a call, got %+v", diags)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(sub) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	// tiny helper to avoid importing strings; tests stay minimal
outer:
	for i := 0; i+len(sub) <= len(s); i++ {
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}
