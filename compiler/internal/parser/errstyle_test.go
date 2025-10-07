package parser

import (
  "os"
  "path/filepath"
  "regexp"
  "strings"
  "testing"
)

func TestNoRawErrorsInParser(t *testing.T) {
  rx := regexp.MustCompile(`\b(fmt\.Errorf|errors\.New)\s*\(`)
  err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
    if err != nil {
      return err
    }
    if info.IsDir() {
      return nil
    }
    if !strings.HasSuffix(path, ".go") {
      return nil
    }
    if strings.HasSuffix(path, "_test.go") {
      return nil
    }

    b, readErr := os.ReadFile(path)
    if readErr != nil {
      return readErr
    }
    if rx.Match(b) {
      t.Errorf("raw error construction found in %s", path)
    }
    return nil
  })
  if err != nil {
    t.Fatalf("walk failed: %v", err)
  }
}
