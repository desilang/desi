package term

import (
	"fmt"
	"strings"
)

// Bprintf writes formatted text into a strings.Builder and ignores (n, err)
// so linters don't complain about unhandled errors from fmt.Fprintf.
func Bprintf(b *strings.Builder, format string, a ...any) { _, _ = fmt.Fprintf(b, format, a...) }
