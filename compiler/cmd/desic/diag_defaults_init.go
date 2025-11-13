package main

import (
	"os"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
	"github.com/desilang/desi/compiler/internal/project"
)

// Seed renderer defaults from desi.mod before main() runs.
// Flags parsed in main() will override these (CLI > manifest).
func init() {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	_, mp, ok := project.FindRoot(cwd)
	if !ok {
		return
	}
	m, diags := project.Load(mp)
	if len(diags) != 0 {
		return // don't set defaults on manifest errors
	}
	df := m.DiagDefaults()
	ef := df.ErrorFormat
	if ef == "" {
		ef = "human"
	}
	col := strings.ToLower(df.Color)
	var cm diag.ColorMode
	switch col {
	case "always":
		cm = diag.Always
	case "never":
		cm = diag.Never
	default:
		cm = diag.Auto
	}
	diag.SetGlobalRender(ef, cm)
}
