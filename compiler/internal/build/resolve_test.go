package build

import (
  "os"
  "path/filepath"
  "strings"
  "testing"

  "github.com/desilang/desi/compiler/internal/diag"
)

func write(t *testing.T, path, body string) {
  t.Helper()
  if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
    t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
  }
  if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
    t.Fatalf("write %s: %v", path, err)
  }
}

func TestResolve_SimpleOK(t *testing.T) {
  td := t.TempDir()
  // project tree:
  //   main.desi (imports util.math)
  //   util/math.desi
  main := filepath.Join(td, "main.desi")
  math := filepath.Join(td, "util", "math.desi")

  write(t, math, `
# util.math
pub def add(a:int,b:int) -> int:
  a + b
`)
  write(t, main, `
# entry
import util.math
def main() -> int:
  0
`)

  plan, diags, err := ResolveEntry(main, ResolveOptions{})
  if err != nil {
    t.Fatalf("ResolveEntry err: %v", err)
  }
  if len(diags) != 0 {
    t.Fatalf("unexpected diags: %+v", diags)
  }
  if got, want := filepath.Clean(plan.Entry.File), filepath.Clean(main); got != want {
    t.Fatalf("entry file = %s, want %s", got, want)
  }
  // Expect two units: entry then util/math.
  if len(plan.Deps) != 2 {
    t.Fatalf("deps len = %d, want 2; deps = %#v", len(plan.Deps), plan.Deps)
  }
  if got, want := filepath.Clean(plan.Deps[0].File), filepath.Clean(main); got != want {
    t.Fatalf("deps[0] = %s, want %s", got, want)
  }
  if got, want := filepath.Clean(plan.Deps[1].File), filepath.Clean(math); got != want {
    t.Fatalf("deps[1] = %s, want %s", got, want)
  }
  if plan.Deps[1].Module != "util.math" {
    t.Fatalf("module name = %q, want util.math", plan.Deps[1].Module)
  }
}

func TestResolve_NotFound(t *testing.T) {
  td := t.TempDir()
  main := filepath.Join(td, "main.desi")
  write(t, main, `
from no.such.mod import x
def main() -> int:
  0
`)
  plan, diags, err := ResolveEntry(main, ResolveOptions{})
  if err != nil {
    t.Fatalf("ResolveEntry err: %v", err)
  }
  if got, want := len(diags), 1; got != want {
    t.Fatalf("diags len = %d, want 1; diags = %+v", got, diags)
  }
  if diags[0].Domain != "module" || diags[0].Key != "not_found" || diags[0].Level != diag.LevelError {
    t.Fatalf("diag = %+v, want module/not_found/error", diags[0])
  }
  // Plan should at least include the entry.
  if got, want := filepath.Clean(plan.Entry.File), filepath.Clean(main); got != want {
    t.Fatalf("entry = %s, want %s", got, want)
  }
  if len(plan.Deps) < 1 {
    t.Fatalf("deps empty; got plan=%+v", plan)
  }
}

func TestResolve_Cycle(t *testing.T) {
  td := t.TempDir()
  main := filepath.Join(td, "main.desi")
  a := filepath.Join(td, "util", "a.desi")
  b := filepath.Join(td, "util", "b.desi")

  write(t, a, `
import util.b
def a() -> int:
  0
`)
  write(t, b, `
import util.a
def b() -> int:
  0
`)
  write(t, main, `
import util.a
def main() -> int:
  0
`)

  plan, diags, err := ResolveEntry(main, ResolveOptions{})
  if err != nil {
    t.Fatalf("ResolveEntry err: %v", err)
  }
  // Expect at least one module-domain error for cycle.
  foundCycle := false
  for _, d := range diags {
    if d.Domain == "module" && d.Key == "import_cycle" && strings.HasPrefix(strings.ToLower(string(d.Level)), "error") {
      foundCycle = true
      break
    }
  }
  if !foundCycle {
    t.Fatalf("expected import_cycle diagnostic, got: %+v", diags)
  }
  // Plan still includes entry as first dep.
  if got, want := filepath.Clean(plan.Entry.File), filepath.Clean(main); got != want {
    t.Fatalf("entry = %s, want %s", got, want)
  }
  if len(plan.Deps) == 0 || filepath.Clean(plan.Deps[0].File) != filepath.Clean(main) {
    t.Fatalf("deps[0] should be entry; deps=%#v", plan.Deps)
  }
}
