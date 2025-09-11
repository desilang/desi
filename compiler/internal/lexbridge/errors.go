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
   Diagnostic data model (future-proofed for richer messages)
   ========================================================================== */

type Level string

const (
	LevelError   Level = "error"
	LevelWarning Level = "warning"
	LevelNote    Level = "note"
)

// Applicability hints for suggestions (loosely following rustc’s semantics).
type Applicability string

const (
	// MachineApplicable means the replacement can be applied automatically.
	MachineApplicable Applicability = "machine-applicable"
	// MaybeIncorrect means the suggestion may be right but needs review.
	MaybeIncorrect Applicability = "maybe-incorrect"
	// HasPlaceholders means the suggestion includes placeholders.
	HasPlaceholders Applicability = "has-placeholders"
)

// Span identifies a region in a source file. EndCol is exclusive; if EndCol==0
// the span is treated as a single-column marker at Col.
type Span struct {
	File    string // path to file for display/loading
	Line    int    // 1-based
	Col     int    // 1-based
	EndCol  int    // 1-based, exclusive (0 => single-col)
	Label   string // inline label shown next to the caret/underline
	Primary bool   // whether this is the primary span
}

// Suggestion describes a potential fix.
type Suggestion struct {
	At            Span
	Replacement   string        // textual replacement (or insertion if zero-width)
	Message       string        // short human-readable hint
	Applicability Applicability // how safe it is to auto-apply
}

// Diagnostic models a rust-like diagnostic with structured data.
type Diagnostic struct {
	Level   Level
	Code    string // e.g., "DLE0001"
	Message string

	Primary     Span
	Secondaries []Span

	Notes   []string // "note: ..." lines
	Help    string   // singular help (still supported)
	Suggest []Suggestion
}

/* =============================================================================
   Rendering
   ========================================================================== */

// RenderRustStyle renders a single diagnostic in a Rust-like multi-line format.
func RenderRustStyle(d Diagnostic, srcLoader func(string) ([]byte, error)) string {
	if srcLoader == nil {
		srcLoader = os.ReadFile
	}

	var b strings.Builder

	// Header: e.g., error[DLE0001]: unterminated string
	if d.Code != "" {
		fmt.Fprintf(&b, "%s[%s]: %s\n", d.Level, d.Code, d.Message)
	} else {
		fmt.Fprintf(&b, "%s: %s\n", d.Level, d.Message)
	}

	// Location header
	if d.Primary.File != "" && d.Primary.Line > 0 && d.Primary.Col > 0 {
		fmt.Fprintf(&b, " --> %s:%d:%d\n", d.Primary.File, d.Primary.Line, d.Primary.Col)
	}

	// Source preview with stacked underlines
	printLineWithUnderlines(&b, d.Primary, d.Secondaries, d.Suggest, srcLoader)

	// Notes
	for _, n := range d.Notes {
		if strings.TrimSpace(n) == "" {
			continue
		}
		fmt.Fprintf(&b, "note: %s\n", n)
	}

	// Singular help (legacy)
	if strings.TrimSpace(d.Help) != "" {
		if !strings.HasPrefix(d.Help, "help:") && !strings.HasPrefix(d.Help, "note:") {
			b.WriteString("help: ")
		}
		b.WriteString(d.Help)
		b.WriteByte('\n')
	}

	// Suggestions after the source block
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

	// Primary line + underline
	fmt.Fprintf(b, "%s%s\n", linePrefix, lineText)
	b.WriteString(underPrefix)
	writeUnderline(b, lineText, primary.Col, primary.EndCol, primary.Label)
	b.WriteByte('\n')

	// Stack secondaries/suggestions for the same file/line
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

	// Other-file/line spans/suggestions as mini-blocks
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
   Lexbridge adapter (pretty-print lex errors via code registry)
   ========================================================================== */

// RenderLexbridgeErrorPretty pretty-prints lexbridge "LEXERR ..." lines in Rust style.
// Uses the diag codes registry (JSON) for code IDs/titles/help.
// Also enriches certain messages with suggestions (no duplicate underlines).
func RenderLexbridgeErrorPretty(err error, defaultFile string, srcLoader func(string) ([]byte, error)) string {
	if err == nil {
		return ""
	}
	lines := splitLines(err.Error())
	if len(lines) == 0 {
		return ""
	}

	// Determine display/load path
	extracted := extractLoadFilePrefix(lines[0])
	effPath := resolveErrorPath(defaultFile, extracted)

	diags := parseLexErrLinesLoose(lines, effPath)
	if len(diags) == 0 {
		return ""
	}

	// Try to load file (for precise suggestions)
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

	// Apply registry-based code/help + heuristics without creating duplicate underlines
	for i := range diags {
		applyRegistryAndHeuristics(&diags[i], data)
	}

	// Render all (usually just one)
	var out strings.Builder
	for i, d := range diags {
		if i > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(RenderRustStyle(d, srcLoader))
	}
	return out.String()
}

// Matches: "... load <whatever>:" and captures <whatever>
var loadPrefixRe = regexp.MustCompile(`(?i)\bload\s+(.+?):`)

func extractLoadFilePrefix(line string) string {
	m := loadPrefixRe.FindStringSubmatch(line)
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// Resolve display/load path given a default path and an extracted path from the error line.
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

// Matches only the "LEXERR line=N col=M msg="..."" portion, ignoring any prefix.
var lexErrCoreRe = regexp.MustCompile(`LEXERR\s+line=(\d+)\s+col=(\d+)\s+msg="(.*)"\s*$`)

func parseLexErrLinesLoose(lines []string, file string) []Diagnostic {
	var out []Diagnostic
	for _, ln := range lines {
		idx := strings.Index(ln, "LEXERR")
		if idx < 0 {
			continue
		}
		core := ln[idx:]
		m := lexErrCoreRe.FindStringSubmatch(core)
		if len(m) != 4 {
			continue
		}
		line := atoiSafe(m[1])
		col := atoiSafe(m[2])
		msg := m[3]
		out = append(out, Diagnostic{
			Level:   LevelError,
			Code:    "", // filled from registry
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
	}
	return out
}

// applyRegistryAndHeuristics assigns code/title/help from the JSON registry and
// adds targeted suggestions without adding duplicate underline lines.
func applyRegistryAndHeuristics(d *Diagnostic, fileData []byte) {
	msgLower := strings.ToLower(d.Message)

	// Map known lexer messages to registry keys
	switch {
	case strings.Contains(msgLower, "unterminated string"):
		// Pull from registry
		ce := diag.MustLookup("lexer", "unterminated_string", "DLE0001", "unterminated string")
		d.Code = ce.ID
		// Prefer registry title for Message; keep original label as-is
		d.Message = ce.Title
		if d.Help == "" && strings.TrimSpace(ce.Help) != "" {
			d.Help = ce.Help
		}

		// Suggest inserting a closing quote at end-of-line (single underline line).
		// We DO NOT add a secondary span here to avoid duplicate underline output.
		if fileData != nil && d.Primary.Line > 0 {
			lineText := getLineText(fileData, d.Primary.Line)
			endVis := len(visualize(lineText))
			if endVis > 0 {
				d.Suggest = append(d.Suggest, Suggestion{
					At: Span{
						File:    d.Primary.File,
						Line:    d.Primary.Line,
						Col:     endVis + 1, // caret after last visible char
						EndCol:  0,
						Label:   `add this: "\""`,
						Primary: false,
					},
					Replacement:   `"`,
					Message:       "insert closing quote",
					Applicability: MachineApplicable,
				})
			}
		}
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

// getLineText returns the raw text (without trailing newline) for a 1-based line number.
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

// visualize returns a "visual column" slice for a line, expanding tabs to 4 spaces
// and treating invalid UTF-8 as width 1. This lets us place carets correctly.
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
