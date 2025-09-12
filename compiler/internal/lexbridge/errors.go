package lexbridge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/desilang/desi/compiler/internal/diag"
)

/* =============================================================================
   Diagnostic data model
   ========================================================================== */

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelNote    Level = "note"
)

type Applicability string

const (
	MachineApplicable Applicability = "machine-applicable"
	MaybeIncorrect    Applicability = "maybe-incorrect"
	HasPlaceholders   Applicability = "has-placeholders"
)

type Span struct {
	File    string
	Line    int
	Col     int
	EndCol  int // exclusive; 0 => single-col
	Label   string
	Primary bool
}

type Suggestion struct {
	At            Span
	Replacement   string
	Message       string
	Applicability Applicability
}

type Diagnostic struct {
	Level   Level
	Code    string
	Message string

	Primary     Span
	Secondaries []Span

	Notes   []string
	Help    string
	Suggest []Suggestion
}

/* =============================================================================
   Rendering
   ========================================================================== */

func RenderRustStyle(d Diagnostic, srcLoader func(string) ([]byte, error)) string {
	if srcLoader == nil {
		srcLoader = os.ReadFile
	}

	var b strings.Builder
	if d.Code != "" {
		fmt.Fprintf(&b, "%s[%s]: %s\n", d.Level, d.Code, d.Message)
	} else {
		fmt.Fprintf(&b, "%s: %s\n", d.Level, d.Message)
	}
	if d.Primary.File != "" && d.Primary.Line > 0 && d.Primary.Col > 0 {
		fmt.Fprintf(&b, " --> %s:%d:%d\n", d.Primary.File, d.Primary.Line, d.Primary.Col)
	}

	printLineWithUnderlines(&b, d.Primary, d.Secondaries, d.Suggest, srcLoader)

	for _, n := range d.Notes {
		if strings.TrimSpace(n) != "" {
			fmt.Fprintf(&b, "note: %s\n", n)
		}
	}
	if strings.TrimSpace(d.Help) != "" {
		if !strings.HasPrefix(d.Help, "help:") && !strings.HasPrefix(d.Help, "note:") {
			b.WriteString("help: ")
		}
		b.WriteString(d.Help)
		b.WriteByte('\n')
	}
	for _, s := range d.Suggest {
		lead := "help"
		if s.Applicability != "" {
			lead = fmt.Sprintf("help (%s)", s.Applicability)
		}
		msg := s.Message
		if msg == "" && s.Replacement != "" {
			msg = fmt.Sprintf("replace with %q", s.Replacement)
		}
		if msg != "" {
			fmt.Fprintf(&b, "%s: %s\n", lead, msg)
		}
	}
	return b.String()
}

func printLineWithUnderlines(b *strings.Builder, primary Span, secondaries []Span, suggs []Suggestion, loader func(string) ([]byte, error)) {
	if primary.File == "" || primary.Line <= 0 {
		return
	}
	data, err := loader(primary.File)
	if err != nil {
		return
	}
	lineText := getLineText(data, primary.Line)
	lnStr := fmt.Sprintf("%d", primary.Line)
	linePrefix := " " + lnStr + " | "
	underPrefix := " " + strings.Repeat(" ", len(lnStr)) + " | "

	fmt.Fprintf(b, "%s%s\n", linePrefix, lineText)
	b.WriteString(underPrefix)
	writeUnderline(b, lineText, primary.Col, primary.EndCol, primary.Label)
	b.WriteByte('\n')

	for _, s := range secondaries {
		if s.File == primary.File && s.Line == primary.Line {
			b.WriteString(underPrefix)
			writeUnderline(b, lineText, s.Col, s.EndCol, s.Label)
			b.WriteByte('\n')
		}
	}
	for _, sg := range suggs {
		s := sg.At
		if s.File == primary.File && s.Line == primary.Line {
			b.WriteString(underPrefix)
			label := s.Label
			if label == "" {
				if sg.Replacement != "" {
					label = fmt.Sprintf("replace with %q", sg.Replacement)
				} else if sg.Message != "" {
					label = sg.Message
				}
			}
			writeUnderline(b, lineText, s.Col, s.EndCol, label)
			b.WriteByte('\n')
		}
	}
	for _, s := range secondaries {
		if !(s.File == primary.File && s.Line == primary.Line) {
			printMiniBlock(b, s, loader)
		}
	}
	for _, sg := range suggs {
		s := sg.At
		if !(s.File == primary.File && s.Line == primary.Line) {
			printMiniBlock(b, s, loader)
		}
	}
}

func printMiniBlock(b *strings.Builder, sp Span, loader func(string) ([]byte, error)) {
	if sp.File == "" || sp.Line <= 0 {
		return
	}
	data, err := loader(sp.File)
	if err != nil {
		return
	}
	lineText := getLineText(data, sp.Line)
	lnStr := fmt.Sprintf("%d", sp.Line)
	linePrefix := " " + lnStr + " | "
	underPrefix := " " + strings.Repeat(" ", len(lnStr)) + " | "
	fmt.Fprintf(b, "%s%s\n", linePrefix, lineText)
	b.WriteString(underPrefix)
	writeUnderline(b, lineText, sp.Col, sp.EndCol, sp.Label)
	b.WriteByte('\n')
}

func writeUnderline(b *strings.Builder, line string, col, endCol int, label string) {
	vis := visualize(line)
	start := clamp(col-1, 0, len(vis))
	end := start
	if endCol > 0 && endCol > col {
		end = clamp(endCol-1, start+1, len(vis))
	} else if start < len(vis) {
		end = start + 1
	}
	b.WriteString(strings.Repeat(" ", start))
	if end-start <= 1 {
		b.WriteString("^")
	} else {
		b.WriteString("^")
		b.WriteString(strings.Repeat("~", end-start-1))
	}
	if strings.TrimSpace(label) != "" {
		b.WriteString(" ")
		b.WriteString(label)
	}
}

/* =============================================================================
   Lexbridge adapter with registry keys + JSON shaping
   ========================================================================== */

func RenderLexbridgeErrorPretty(err error, defaultFile string, srcLoader func(string) ([]byte, error)) string {
	if err == nil {
		return ""
	}
	lines := splitLines(err.Error())
	if len(lines) == 0 {
		return ""
	}

	// Reset key stash per call.
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

// We maintain a parallel slice of keys for the diags parsed above (same order).
var diagKeys []string

func applyRegistry(d *Diagnostic, fileData []byte) {
	var key string
	if len(diagKeys) > 0 {
		key = diagKeys[0]
		diagKeys = diagKeys[1:]
	}

	// Fallback mapping by substring (compat path)
	if key == "" {
		msgLower := strings.ToLower(d.Message)
		if strings.Contains(msgLower, "unterminated string") {
			key = "unterminated_string"
		}
	}
	if key == "" {
		return // unknown; leave plain
	}

	// Lookup full definition
	cf, ok := diag.LookupFull("lexer", key)
	if !ok {
		return
	}

	// Fill code/title/help
	if cf.Entry.ID != "" {
		d.Code = cf.Entry.ID
	}
	if cf.Entry.Title != "" {
		d.Message = cf.Entry.Title
	}
	if d.Help == "" && strings.TrimSpace(cf.Entry.Help) != "" {
		d.Help = cf.Entry.Help
	}

	// Shape primary end from JSON (e.g., eol)
	if endCol, okCol := primaryEndFromWhereSpec(cf.PrimaryEnd, d.Primary, fileData); okCol && endCol > d.Primary.Col {
		d.Primary.EndCol = endCol
	}

	// Materialize JSON suggestions (no duplicate underline lines)
	for _, s := range cf.Suggestions {
		sp, ok := placeFromWhereSpec(s.Where, d.Primary, fileData)
		if !ok {
			continue
		}
		d.Suggest = append(d.Suggest, Suggestion{
			At:            sp,
			Replacement:   s.Replacement,
			Message:       s.Message,
			Applicability: mapApplicability(s.Applicability),
		})
		// carry label onto the underline span
		d.Suggest[len(d.Suggest)-1].At.Label = s.Label
	}
}

func mapApplicability(s string) Applicability {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "machine-applicable":
		return MachineApplicable
	case "has-placeholders":
		return HasPlaceholders
	case "maybe-incorrect":
		return MaybeIncorrect
	default:
		return MaybeIncorrect
	}
}

func primaryEndFromWhereSpec(w diag.WhereSpec, primary Span, fileData []byte) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(w.Kind)) {
	case "eol":
		if fileData == nil || primary.Line <= 0 || primary.File == "" {
			return 0, false
		}
		line := getLineText(fileData, primary.Line)
		endVis := len(visualize(line))
		if endVis == 0 {
			return 0, false
		}
		// EndCol is exclusive; +1 highlights through the last visible char.
		return endVis + 1, true
	case "primary_offset":
		col := primary.Col + w.Delta
		if col < 1 {
			col = 1
		}
		return col, true
	case "pos":
		if w.Line == primary.Line && w.Col > 0 {
			return w.Col, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func placeFromWhereSpec(w diag.WhereSpec, primary Span, fileData []byte) (Span, bool) {
	switch strings.ToLower(strings.TrimSpace(w.Kind)) {
	case "eol":
		if fileData == nil || primary.Line <= 0 || primary.File == "" {
			return Span{}, false
		}
		line := getLineText(fileData, primary.Line)
		endVis := len(visualize(line))
		if endVis == 0 {
			return Span{}, false
		}
		return Span{
			File:   primary.File,
			Line:   primary.Line,
			Col:    endVis + 1,
			EndCol: 0,
		}, true
	case "primary_offset":
		col := primary.Col + w.Delta
		if col < 1 {
			col = 1
		}
		return Span{
			File:   primary.File,
			Line:   primary.Line,
			Col:    col,
			EndCol: 0,
		}, true
	case "pos":
		if primary.File == "" || w.Line <= 0 || w.Col <= 0 {
			return Span{}, false
		}
		return Span{
			File:   primary.File,
			Line:   w.Line,
			Col:    w.Col,
			EndCol: 0,
		}, true
	default:
		return Span{}, false
	}
}

/* =============================================================================
   Helpers
   ========================================================================== */

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

func splitLines(s string) []string {
	sc := bufio.NewScanner(strings.NewReader(s))
	var out []string
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

func getLineText(src []byte, line int) string {
	if line <= 0 {
		return ""
	}
	cur := 1
	start := 0
	for i, b := range src {
		if b == '\n' {
			if cur == line {
				return string(src[start:i])
			}
			cur++
			start = i + 1
		}
	}
	if cur == line && start <= len(src) {
		return string(src[start:])
	}
	return ""
}

func visualize(s string) []rune {
	const tabw = 4
	var vis []rune
	for len(s) > 0 {
		r, sz := utf8.DecodeRuneInString(s)
		if r == '\t' {
			for i := 0; i < tabw; i++ {
				vis = append(vis, ' ')
			}
		} else if r == utf8.RuneError && sz == 1 {
			vis = append(vis, '�')
		} else {
			vis = append(vis, r)
		}
		s = s[sz:]
	}
	return vis
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
