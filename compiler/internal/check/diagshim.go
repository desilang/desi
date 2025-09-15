package check

import (
	"fmt"

	"github.com/desilang/desi/compiler/internal/diag"
)

// typedError carries a diagnostic code, a rendered title/message,
// and (domain,key) so the CLI can fetch help text from the catalog.
type typedError struct {
	code   string
	title  string
	domain string
	key    string
}

func (e typedError) Error() string {
	if e.code != "" {
		return e.code + ": " + e.title
	}
	return e.title
}
func (e typedError) Code() string   { return e.code }
func (e typedError) Title() string  { return e.title }
func (e typedError) Domain() string { return e.domain }
func (e typedError) Key() string    { return e.key }

// lookupIDTitle consults the diag catalog; falls back if missing.
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

// typedErr constructs a typedError using catalog (or fallbacks) and contextualizes it.
func typedErr(domain, key, fallbackID, fallbackTitle, context string, names, values int) error {
	id, title := lookupIDTitle(domain, key, fallbackID, fallbackTitle)
	msg := fmt.Sprintf("%s in %s: names=%d, values=%d", title, context, names, values)
	return typedError{code: id, title: msg, domain: domain, key: key}
}

// warnCode returns the catalog ID for a warning or a fallback.
func warnCode(domain, key, fallbackID string) string {
	if info, ok := diag.LookupFull(domain, key); ok && info.Entry.ID != "" {
		return info.Entry.ID
	}
	return fallbackID
}
