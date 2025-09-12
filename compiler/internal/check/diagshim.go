package check

import "fmt"

// typedErr formats a single-line error carrying a diagnostic-like code + title.
// For now we use the provided fallbacks; later we can resolve (domain,key)
// against the diag catalog once that API is stable and exported.
func typedErr(domain, key, fallbackID, fallbackTitle, context string, names, values int) error {
  return fmt.Errorf("%s: %s in %s: names=%d, values=%d",
    fallbackID, fallbackTitle, context, names, values)
}
