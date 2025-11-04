package diag

import "testing"

func TestCatalog_IncludesDPE0110(t *testing.T) {
	e, ok := Lookup("DPE0110")
	if !ok {
		t.Fatalf("DPE0110 not found in catalog")
	}
	if e.Title == "" {
		t.Fatalf("DPE0110 has empty title in catalog")
	}
	if e.Help == "" {
		t.Fatalf("DPE0110 has empty help in catalog")
	}
}
