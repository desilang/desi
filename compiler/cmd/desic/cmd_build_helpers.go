package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/desilang/desi/compiler/internal/diag"
)

// guessErrFile tries to extract "load <path>:" prefix from ResolveAndParseWith loader errors.
// Falls back to the provided defaultFile if no path can be found.
func guessErrFile(errText, defaultFile string) string {
	var re = regexp.MustCompile(`(?i)\bload\s+(.+?):`)
	m := re.FindStringSubmatch(errText)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return defaultFile
}

// lookupHelp fetches the catalog help text for a (domain,key).
func lookupHelp(domain, key string) string {
	if info, ok := diag.LookupFull(domain, key); ok {
		return info.Entry.Help
	}
	return ""
}

// warnHelpFromCode resolves a warning code (DW...) back to a known key and returns help.
func warnHelpFromCode(code string) string {
	// Keep this small and explicit; extend as we add warn keys.
	keys := []string{
		"unused_variable",
		"shadowed_variable",
		"unreachable_code",
		"missing_explicit_return",
		"non_exhaustive_match",
	}
	for _, k := range keys {
		if info, ok := diag.LookupFull("warn", k); ok && info.Entry.ID == code {
			return info.Entry.Help
		}
	}
	return ""
}

// makeLineGetter returns a closure that fetches 1-based source lines.
func makeLineGetter(file string) func(int) (string, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return func(int) (string, bool) { return "", false }
	}
	// Split without trimming trailing newline; we only need lines.
	raw := string(data)
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	return func(line int) (string, bool) {
		if line <= 0 || line > len(lines) {
			return "", false
		}
		return lines[line-1], true
	}
}
