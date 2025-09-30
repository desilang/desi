package parsebridge

import (
	"fmt"
	"os"
	"path/filepath"
)

func mirrorDevParserInto(root string, devParserSrc []byte) error {
	dstDir := filepath.Join(root, "compiler", "desi")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}
	dst := filepath.Join(dstDir, "parser.desi")
	if err := os.WriteFile(dst, devParserSrc, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}

func mirrorDevLexerInto(root string, devLexerSrc []byte) error {
	dstDir := filepath.Join(root, "compiler", "desi")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}
	dst := filepath.Join(dstDir, "lexer.desi")
	if err := os.WriteFile(dst, devLexerSrc, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dst, err)
	}
	return nil
}
