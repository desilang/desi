package parse

import "testing"

func TestSliceSteps_Parse_OK(t *testing.T) {
	src := []byte(`
def main() -> int:
  let s: str = "abcdef"
  let mut _s: str = ""
  _s := s[1:4]
  _s := s[:5]
  _s := s[::2]
  _s := s[2::]
  _s := s[1:5:2]
  return 0
`)
	_, diags := ParseFile("<mem>", src)
	if len(diags) != 0 {
		t.Fatalf("unexpected parse diags: %+v", diags)
	}
}
