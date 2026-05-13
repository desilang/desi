package check

import (
	"strings"
	"testing"

	"github.com/desilang/desi/compiler/internal/parse"
)

// helper: parse Desi source and run the performance advisor at a given level.
func runAdvisor(t *testing.T, src, level string) []string {
	t.Helper()
	mod, pdiags := parse.ParseFile("test.desi", []byte(src))
	if len(pdiags) > 0 {
		t.Fatalf("parse errors: %v", pdiags)
	}
	diags := RunPerfAdvisor(mod, level)
	var codes []string
	for _, d := range diags {
		codes = append(codes, d.CodeID)
	}
	return codes
}

func containsCode(codes []string, code string) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}
	return false
}

// --- DPR0001: Nested loop detection ---

func TestPerfAdvisor_NestedLoops(t *testing.T) {
	src := `
def process(items: list[int]) -> none:
	for x in items:
		for y in items:
			print(x)
`
	codes := runAdvisor(t, src, "relaxed")
	if !containsCode(codes, "DPR0001") {
		t.Errorf("expected DPR0001 for nested loops, got %v", codes)
	}
}

func TestPerfAdvisor_NoNestedLoops(t *testing.T) {
	src := `
def process(items: list[int]) -> none:
	for x in items:
		print(x)
	for y in items:
		print(y)
`
	codes := runAdvisor(t, src, "strict")
	if containsCode(codes, "DPR0001") {
		t.Errorf("sequential loops should not trigger DPR0001, got %v", codes)
	}
}

// --- DPR0002: String concatenation in loop ---

func TestPerfAdvisor_StringConcatInLoop(t *testing.T) {
	src := `
def build_string(items: list[str]) -> str:
	let result: str = ""
	for item in items:
		result += "hello"
	return result
`
	codes := runAdvisor(t, src, "relaxed")
	if !containsCode(codes, "DPR0002") {
		t.Errorf("expected DPR0002 for string concat in loop, got %v", codes)
	}
}

func TestPerfAdvisor_StringConcatOutsideLoop(t *testing.T) {
	src := `
def build_string() -> str:
	let result: str = ""
	result += "hello"
	return result
`
	codes := runAdvisor(t, src, "strict")
	if containsCode(codes, "DPR0002") {
		t.Errorf("string concat outside loop should not trigger DPR0002, got %v", codes)
	}
}

// --- DPR0003: Repeated collection lookup ---

func TestPerfAdvisor_RepeatedLookup(t *testing.T) {
	src := `
def process(data: dict[str, int]) -> none:
	for i in range(10):
		let a: int = data["key"]
		let b: int = data["key"]
`
	codes := runAdvisor(t, src, "default")
	if !containsCode(codes, "DPR0003") {
		t.Errorf("expected DPR0003 for repeated lookup, got %v", codes)
	}
}

// --- DPR0004: Unbounded allocation ---

func TestPerfAdvisor_UnboundedAlloc(t *testing.T) {
	src := `
def process(n: int) -> none:
	for i in range(n):
		let tmp: list[int] = [1, 2, 3]
		print(tmp)
`
	codes := runAdvisor(t, src, "strict")
	if !containsCode(codes, "DPR0004") {
		t.Errorf("expected DPR0004 for allocation in loop, got %v", codes)
	}
}

func TestPerfAdvisor_UnboundedAlloc_NotInRelaxed(t *testing.T) {
	src := `
def process(n: int) -> none:
	for i in range(n):
		let tmp: list[int] = [1, 2, 3]
		print(tmp)
`
	codes := runAdvisor(t, src, "relaxed")
	if containsCode(codes, "DPR0004") {
		t.Errorf("DPR0004 should not fire at relaxed level, got %v", codes)
	}
}

// --- Level filtering ---

func TestPerfAdvisor_LevelFiltering(t *testing.T) {
	// Source with nested loops (DPR0001=relaxed) and allocation in loop (DPR0004=strict)
	src := `
def process(items: list[int]) -> none:
	for x in items:
		for y in items:
			let tmp: list[int] = [1, 2, 3]
			print(x)
`
	// Relaxed: should get DPR0001 but NOT DPR0004
	relaxedCodes := runAdvisor(t, src, "relaxed")
	if !containsCode(relaxedCodes, "DPR0001") {
		t.Errorf("relaxed should include DPR0001")
	}
	if containsCode(relaxedCodes, "DPR0004") {
		t.Errorf("relaxed should NOT include DPR0004")
	}

	// Strict: should get both
	strictCodes := runAdvisor(t, src, "strict")
	if !containsCode(strictCodes, "DPR0001") {
		t.Errorf("strict should include DPR0001")
	}
	if !containsCode(strictCodes, "DPR0004") {
		t.Errorf("strict should include DPR0004")
	}
}

// --- Nil module ---

func TestPerfAdvisor_NilModule(t *testing.T) {
	diags := RunPerfAdvisor(nil, "default")
	if len(diags) != 0 {
		t.Errorf("nil module should return no diagnostics, got %d", len(diags))
	}
}

// --- Empty level defaults to "default" ---

func TestPerfAdvisor_EmptyLevel(t *testing.T) {
	src := `
def process(items: list[int]) -> none:
	for x in items:
		for y in items:
			print(x)
`
	codes := runAdvisor(t, src, "")
	if !containsCode(codes, "DPR0001") {
		t.Errorf("empty level should default to 'default', expected DPR0001, got %v", codes)
	}
}

// --- Class methods are scanned ---

func TestPerfAdvisor_ClassMethods(t *testing.T) {
	src := `
class MyClass:
	def process(self, items: list[int]) -> none:
		for x in items:
			for y in items:
				print(x)
`
	codes := runAdvisor(t, src, "relaxed")
	if !containsCode(codes, "DPR0001") {
		t.Errorf("expected DPR0001 in class method, got %v", codes)
	}
}

// --- Verify diagnostic message quality ---

func TestPerfAdvisor_DiagMessages(t *testing.T) {
	src := `
def process(items: list[int]) -> none:
	for x in items:
		for y in items:
			print(x)
`
	mod, _ := parse.ParseFile("test.desi", []byte(src))
	diags := RunPerfAdvisor(mod, "relaxed")
	if len(diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
	msg := diags[0].Message
	if !strings.Contains(msg, "nested loop") {
		t.Errorf("expected message about nested loops, got: %s", msg)
	}
}
