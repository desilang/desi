package term

import (
	"fmt"
	"io"
)

// Wprintf writes formatted text to any io.Writer and ignores (n, err)
// so linters don't complain about unhandled fmt.Fprintf results.
func Wprintf(w io.Writer, format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }
