package lexbridge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func RenderLexbridgeErrorPretty(err error, defaultFile string, srcLoader func(string) ([]byte, error)) string {
	if err == nil {
		return ""
	}
	lines := splitLines(err.Error())
	if len(lines) == 0 {
		return ""
	}

	// Reset key stash per call (shared with registry_apply.go).
	diagKeys = nil

	// Determine display/load path
	extracted := extractLoadFilePrefix(lines[0])
	effPath := resolveErrorPath(defaultFile, extracted)

	diags := parseLexErrLinesLoose(lines, effPath) // supports optional key=...
	if len(diags) == 0 {
		return ""
	}

	// Try to load file for shaping & suggestion placement
	loader := srcLoader
	if loader == nil {
		loader = os.ReadFile
	}
	data, loadErr := loader(effPath)
	if loadErr != nil && defaultFile != "" && defaultFile != effPath {
		effPath = defaultFile
		data, _ = loader(effPath)
		for i := range diags {
			diags[i].Primary.File = effPath
		}
	}

	// Apply registry: code/title/help, primary_end shaping, suggestions
	for i := range diags {
		applyRegistry(&diags[i], data)
	}

	var out strings.Builder
	for i, d := range diags {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(RenderRustStyle(d, srcLoader))
	}
	return out.String()
}

// "... load <path>:" → capture path.
var loadPrefixRe = regexp.MustCompile(`(?i)\bload\s+(.+?):`)

func extractLoadFilePrefix(line string) string {
	m := loadPrefixRe.FindStringSubmatch(line)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func resolveErrorPath(defaultFile, extracted string) string {
	if strings.TrimSpace(extracted) == "" {
		return defaultFile
	}
	if strings.Contains(extracted, "/") || strings.Contains(extracted, "\\") {
		return extracted
	}
	if defaultFile == "" {
		return extracted
	}
	return filepath.Join(filepath.Dir(defaultFile), extracted)
}

// Optional key=...  Example:
//
//	LEXERR line=1 col=9 key=unterminated_string msg="unterminated string"
//	LEXERR line=1 col=9 msg="unterminated string"
var lexErrCoreRe = regexp.MustCompile(`LEXERR\s+line=(\d+)\s+col=(\d+)(?:\s+key=([A-Za-z0-9_]+))?\s+msg="(.*)"\s*$`)

func parseLexErrLinesLoose(lines []string, file string) []Diagnostic {
	var out []Diagnostic
	for _, ln := range lines {
		idx := strings.Index(ln, "LEXERR")
		if idx < 0 {
			continue
		}
		core := ln[idx:]
		m := lexErrCoreRe.FindStringSubmatch(core)
		if len(m) != 5 {
			continue
		}
		line := atoiSafe(m[1])
		col := atoiSafe(m[2])
		key := strings.TrimSpace(m[3])
		msg := m[4]
		out = append(out, Diagnostic{
			Level:   LevelError,
			Code:    "",
			Message: msg,
			Primary: Span{
				File:    file,
				Line:    line,
				Col:     col,
				EndCol:  0,
				Label:   msg,
				Primary: true,
			},
		})
		diagKeys = append(diagKeys, key)
	}
	return out
}

func splitLines(s string) []string {
	sc := bufio.NewScanner(strings.NewReader(s))
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
