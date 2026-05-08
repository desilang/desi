package check

// API sensitivity tiers for build audit.
// Each stdlib module is classified into a tier based on its access level:
//   Tier 0 (Safe)       — Pure computation, no side effects (math, strings, collections)
//   Tier 1 (System)     — Local filesystem and process access (fs, os, sys, path)
//   Tier 2 (Network)    — Network I/O (http, net, tls, websocket)
//   Tier 3 (Privileged) — Shell execution, crypto, process spawning (shell, crypto, process)

const (
	TierSafe       = 0
	TierSystem     = 1
	TierNetwork    = 2
	TierPrivileged = 3
)

// ModuleTier maps each stdlib module name to its sensitivity tier.
var ModuleTier = map[string]int{
	// Tier 0: Safe
	"math":        TierSafe,
	"strings":     TierSafe,
	"collections": TierSafe,
	"fmt":         TierSafe,
	"json":        TierSafe,
	"csv":         TierSafe,
	"re":          TierSafe,
	"time":        TierSafe,
	"datetime":    TierSafe,
	"bytes":       TierSafe,
	"encoding":    TierSafe,
	"base64":      TierSafe,
	"uuid":        TierSafe,
	"random":      TierSafe,
	"log":         TierSafe,
	"color":       TierSafe,
	"table":       TierSafe,
	"diff":        TierSafe,
	"validate":    TierSafe,
	"ini":         TierSafe,
	"toml":        TierSafe,
	"yaml":        TierSafe,
	"dotenv":      TierSafe,
	"template":    TierSafe,
	"limits":      TierSafe,
	"sync":        TierSafe,
	"perf":        TierSafe,
	"ast":         TierSafe,
	"testing":     TierSafe,

	// Tier 1: System
	"fs":     TierSystem,
	"os":     TierSystem,
	"sys":    TierSystem,
	"path":   TierSystem,
	"env":    TierSystem,
	"io":     TierSystem,
	"file":   TierSystem,
	"signal": TierSystem,
	"args":   TierSystem,

	// Tier 2: Network
	"http":      TierNetwork,
	"net":       TierNetwork,
	"tls":       TierNetwork,
	"websocket": TierNetwork,
	"url":       TierNetwork,
	"mime":      TierNetwork,
	"jwt":       TierNetwork,
	"sqlite3":   TierNetwork, // DB access → network-adjacent

	// Tier 3: Privileged
	"shell":   TierPrivileged,
	"crypto":  TierPrivileged,
	"process": TierPrivileged,
}

// TierName returns a human-readable name for a tier.
func TierName(tier int) string {
	switch tier {
	case TierSafe:
		return "safe"
	case TierSystem:
		return "system"
	case TierNetwork:
		return "network"
	case TierPrivileged:
		return "privileged"
	default:
		return "unknown"
	}
}

// ClassifyModule returns the tier and category for a given module name.
// Returns (0, "safe") for unknown modules (conservative default).
func ClassifyModule(name string) (tier int, category string) {
	t, ok := ModuleTier[name]
	if !ok {
		return TierSafe, "safe"
	}
	return t, TierName(t)
}
