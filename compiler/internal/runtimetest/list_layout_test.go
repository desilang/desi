package runtimetest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/backend/llvm"
)

// The backend emits `xs[i]` as an inline bounds check and load, using DesiList's
// field offsets as literals. That makes the struct layout a contract between
// the runtime header and the code generator.
//
// list.h asserts the offsets at compile time, which catches someone reordering
// the struct. It does not catch someone changing the constants in the backend
// to match a struct they only meant to edit locally, and it does not run at all
// if the header is compiled by a toolchain that skips the assertions. This
// compiles a program against the real header, asks it what the offsets actually
// are, and compares with what the backend believes.
func TestListLayoutMatchesTheBackend(t *testing.T) {
	cc, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang not on PATH; skipping runtime layout check")
	}

	runtimeDir, err := filepath.Abs(filepath.Join("..", "..", "..", "compiler", "runtime"))
	if err != nil {
		t.Fatalf("resolving the runtime directory: %v", err)
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "layout.c")
	prog := `#include <stdio.h>
#include <stddef.h>
#include "list.h"
int main(void) {
    printf("data=%zu\n",     offsetof(DesiList, data));
    printf("length=%zu\n",   offsetof(DesiList, length));
    printf("capacity=%zu\n", offsetof(DesiList, capacity));
    printf("type_tag=%zu\n", offsetof(DesiList, type_tag));
    return 0;
}
`
	if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
		t.Fatalf("writing the probe: %v", err)
	}

	bin := filepath.Join(dir, "layout")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command(cc, "-I", runtimeDir, "-o", bin, src)
	if out, err := build.CombinedOutput(); err != nil {
		// A failure here is very likely list.h's own _Static_asserts firing,
		// which is the header telling us the struct moved.
		t.Fatalf("compiling against list.h failed — if this is a _Static_assert, "+
			"the struct was reordered and the backend needs updating too:\n%v\n%s", err, out)
	}

	out, err := exec.Command(bin).Output()
	if err != nil {
		t.Fatalf("running the layout probe: %v", err)
	}

	got := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("unparseable probe output %q: %v", line, err)
		}
		got[name] = n
	}

	want := map[string]int{
		"data":     llvm.ListDataOffset,
		"length":   llvm.ListLengthOffset,
		"capacity": llvm.ListCapacityOffset,
		"type_tag": llvm.ListTypeTagOffset,
	}
	for field, expect := range want {
		actual, present := got[field]
		if !present {
			t.Fatalf("the probe reported no offset for %q; output was:\n%s", field, out)
		}
		if actual != expect {
			t.Errorf("DesiList.%s is at offset %d in list.h, but the backend emits %d.\n"+
				"Generated code would load the wrong field. Update %s in "+
				"compiler/internal/backend/llvm/list_layout.go, or put the struct back.",
				field, actual, expect, fieldConst(field))
		}
	}
}

func fieldConst(field string) string {
	return fmt.Sprintf("List%sOffset", strings.ToUpper(field[:1])+field[1:])
}
