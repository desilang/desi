package build

import (
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// --- tiny registry helpers ---

func lookupIDTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
	if info, ok := diag.LookupFull(domain, key); ok {
		id := info.Entry.ID
		if id == "" {
			id = fallbackID
		}
		title := info.Entry.Title
		if title == "" {
			title = fallbackTitle
		}
		return id, title
	}
	return fallbackID, fallbackTitle
}

func modMk(level diag.Level, key, fallbackID, fallbackTitle, msg string) diag.Diagnostic {
	id, _ := lookupIDTitle("module", key, fallbackID, fallbackTitle)
	return diag.Diagnostic{
		Domain:  "module",
		Key:     key,
		Level:   level,
		Code:    id,
		Message: msg,
	}
}

// ModuleErrorf: generic module-domain error with registry key + fallback.
func ModuleErrorf(key, fallbackID, fallbackTitle, format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return modMk(diag.LevelError, key, fallbackID, fallbackTitle, msg)
}

// --- Specific constructors used by resolver ---

// DME0001: import cycle
func ErrImportCycle(chain []string) error {
	var cyc string
	if len(chain) > 0 {
		cyc = strings.Join(append(append([]string{}, chain...), chain[0]), " -> ")
	}
	return ModuleErrorf("import_cycle", "DME0001", "import cycle",
		"import cycle detected: %s", cyc)
}

// DME0002: module not found
func ErrModuleNotFound(modPath, fromFile string, attempts []string) error {
	// codes.json key = module.missing_module
	msg := fmt.Sprintf("cannot find module %q (imported from %s)", modPath, fromFile)
	d := modMk(diag.LevelError, "missing_module", "DME0002", "cannot find module", msg)
	if len(attempts) > 0 {
		d.Notes = append(d.Notes, "looked for:")
		for _, a := range attempts {
			d.Notes = append(d.Notes, "  - "+a)
		}
	}
	return d
}

// DME0003: bad import path / malformed dotted name
func ErrBadImport(importText, why string) error {
	return ModuleErrorf("bad_import", "DME0003", "invalid import path",
		"invalid import %q: %s", importText, why)
}

// DME0004: IO read failure (during resolution)
func ErrIORead(path string, cause error) error {
	return ModuleErrorf("io_read", "DME0004", "failed to read file",
		"read %s: %v", path, cause)
}

// DME0005: parse failed (bubble parser error up through resolver)
func ErrParseFailed(path string, cause error) error {
	return ModuleErrorf("parse_failed", "DME0005", "parse failed during resolution",
		"parse %s: %v", path, cause)
}

// DME0006: bad entry path (cannot make absolute, etc.)
func ErrBadEntryPath(entry string, cause error) error {
	return ModuleErrorf("bad_entry", "DME0006", "bad entry path",
		"resolve: entry %q: %v", entry, cause)
}

// DME0007: entry not found or is a directory
func ErrEntryNotFound(entryAbs string) error {
	return ModuleErrorf("entry_not_found", "DME0007", "entry not found",
		"resolve: entry not found or is a directory: %s", entryAbs)
}

// DMW0001: duplicate import (warning) — available if you choose to surface it
func WarnDuplicateImport(modPath string) error {
	id, _ := lookupIDTitle("module", "duplicate_import", "DMW0001", "duplicate import")
	return diag.Diagnostic{
		Domain:  "module",
		Key:     "duplicate_import",
		Level:   diag.LevelWarning,
		Code:    id,
		Message: fmt.Sprintf("duplicate import of %q", modPath),
	}
}

// DME0099: internal resolver error (unexpected)
func ErrInternalf(format string, args ...any) error {
	return ModuleErrorf("internal", "DME0099", "internal error", format, args...)
}
