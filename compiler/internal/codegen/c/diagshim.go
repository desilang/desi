package c

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/diag"
)

// lookup helper identical to the ones we used in parser/check
func lookupIDTitle(domain, key, fallbackID, fallbackTitle string) (string, string) {
	if info, ok := diag.LookupFull(domain, key); ok {
		id := info.Entry.ID
		if id == "" {
			id = fallbackID
		}
		title := info.Entry.Title
		if title == "" {
			title = fallbackTitle
		}
		return id, title
	}
	return fallbackID, fallbackTitle
}

func cg(domain, key, id, msg string) diag.Diagnostic {
	return diag.Diagnostic{
		Domain:  domain, // "codegen"
		Key:     key,
		Level:   diag.LevelError,
		Code:    id,
		Message: msg,
	}
}

// CGErrorfT: Formats a message where the FIRST %s is the registry title.
//
//	Example: CGErrorfT("unsupported_return_type","DCE0001","unsupported return type",
//	                   "%s in function %q: %s", fn, detail)
func CGErrorfT(key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, title := lookupIDTitle("codegen", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, append([]any{title}, args...)...)
	return cg("codegen", key, id, msg)
}

// CGErrorf: Plain formatting (no implicit title prefix).
//
//	Example: CGErrorf("c_generic","DCE9000","C backend error", "desic C backend: ...")
func CGErrorf(key, fallbackID, fallbackTitle, format string, args ...any) error {
	id, _ := lookupIDTitle("codegen", key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf(format, args...)
	return cg("codegen", key, id, msg)
}
