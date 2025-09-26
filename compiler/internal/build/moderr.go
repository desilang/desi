package build

import "strings"

/* ---------- typed module diagnostics (lightweight) ---------- */

type modErr struct {
	code   string // DME0001/2 (errors) or DMW0001 (warning)
	title  string
	key    string // stable key in catalog, e.g. "import_cycle"
	detail string // extra context shown after the title line
}

func (e modErr) Error() string {
	if strings.TrimSpace(e.detail) == "" {
		return e.title
	}
	return e.title + ": " + e.detail
}
func (e modErr) Code() string   { return e.code }
func (e modErr) Title() string  { return e.title }
func (e modErr) Domain() string { return "module" }
func (e modErr) Key() string    { return e.key }
