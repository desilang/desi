package check

// StdlibModules lists all known stdlib module names.
// This is the single source of truth for stdlib module validation.
var StdlibModules = map[string]bool{
	"log":     true,
	"json":    true,
	"math":    true,
	"http":    true,
	"fs":      true,
	"crypto":  true,
	"sys":     true,
	"os":      true,
	"io":      true,
	"sync":    true,
	"strings": true,
	"csv":      true,
	"encoding": true,
}
