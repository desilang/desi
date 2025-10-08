package diag

import "testing"

func TestCodegenDomainLoads(t *testing.T) {
	cf, ok := LookupFull("codegen", "c_backend_generic")
	if !ok {
		t.Fatalf("missing codegen.c_backend_generic in catalog")
	}
	if cf.Entry.ID == "" || cf.Entry.Title == "" {
		t.Fatalf("empty fields for codegen.c_backend_generic: %+v", cf.Entry)
	}
}
