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
	keys := []struct {
		domain string
		key    string
	}{
		{"warn", "unused_variable"},
		{"warn", "shadowed_variable"},
		{"warn", "unreachable_code"},
		{"warn", "missing_explicit_return"},
		{"warn", "non_exhaustive_match"},
	}
	up := strings.ToUpper(strings.TrimSpace(code))
	for _, it := range keys {
		if info, ok := diag.LookupFull(it.domain, it.key); ok && strings.ToUpper(strings.TrimSpace(info.Entry.ID)) == up {
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
