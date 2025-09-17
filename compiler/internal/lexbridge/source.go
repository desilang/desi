package lexbridge

import (
	"errors"
	"fmt"
	"strings"

	"github.com/desilang/desi/compiler/internal/lexer"
)

// NewSourceFromFile builds & runs the Desi lexer bridge on <file> and returns a
// lexer.Source that yields Go-lexer-compatible tokens to the parser.
func NewSourceFromFile(file string) (lexer.Source, error) {
	return NewSourceFromFileOpts(file, false /*keepTmp*/, false /*verbose*/)
}

// NewSourceFromFileOpts is the option-bearing variant (keepTmp, verbose).
func NewSourceFromFileOpts(file string, keepTmp, verbose bool) (lexer.Source, error) {
	raw, err := BuildAndRunRaw(file, keepTmp, verbose)
	if err != nil {
		return nil, err
	}
	nd := ConvertRawToNDJSON(raw, true)

	rows, perr := ParseNDJSON(strings.NewReader(nd))
	if perr != nil {
		// Non-fatal: proceed with whatever we parsed; caller will see parser errors if any.
	}

	var mapped []lexer.Token
	var lexerrs []string

	for _, r := range rows {
		// Ignore empty/garbled rows defensively (fixes kind="" text="" cases)
		if r.Kind == "" {
			continue
		}
		// Ignore non-semantic rows if they appear
		if r.Kind == "WS" || r.Kind == "COMMENT" {
			continue
		}

		if r.Kind == "ERR" {
			// Prefer structured key if provided; validate to avoid breaking our regex.
			keyFrag := ""
			if goodKey(r.Key) {
				keyFrag = fmt.Sprintf(" key=%s", r.Key)
			}
			lexerrs = append(lexerrs, fmt.Sprintf(
				"LEXERR line=%d col=%d%s msg=%q",
				r.Line, r.Col, keyFrag, r.Text,
			))
			continue
		}

		gk, ok := mapDesiToTokKind(r.Kind, r.Text)
		if !ok {
			// Surface true mapping gaps loudly with source coordinates.
			return nil, fmt.Errorf("desi-adapter: unmapped token kind=%q text=%q at %d:%d", r.Kind, r.Text, r.Line, r.Col)
		}

		lexeme := r.Text
		// Our Go lexer returns string literals INCLUDING quotes; Desi stream gives unquoted text.
		if gk == lexer.TokStr {
			lexeme = quoteCLike(r.Text)
		}

		mapped = append(mapped, lexer.Token{
			Kind: gk,
			Lex:  lexeme,
			Line: r.Line,
			Col:  r.Col,
		})
	}

	if len(lexerrs) > 0 {
		return nil, errors.New(strings.Join(lexerrs, "\n"))
	}

	mapped = injectNewlinesBeforeDedent(mapped)
	mapped = ensureEOF(mapped)

	return &desiSource{toks: mapped}, nil
}
