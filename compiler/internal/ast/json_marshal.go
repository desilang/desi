package ast

import (
	"encoding/json"
)

/* ---------- marshal entrypoint ---------- */

func MarshalFileJSON(f *File) ([]byte, error) {
	out := toJFile(f)
	return json.MarshalIndent(out, "", "  ")
}
