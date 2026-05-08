package check

import (
	"fmt"
	"strings"
)

// Permissions holds the parsed [permissions] section from desi.mod.
type Permissions struct {
	Allow []string // e.g., ["http", "fs"]
	Deny  []string // e.g., ["shell", "process"]
	Audit bool     // if true, print audit report before build
}

// AuditEntry records a single module import and its sensitivity classification.
type AuditEntry struct {
	Module   string // stdlib module name
	Tier     int    // 0=safe, 1=system, 2=network, 3=privileged
	Category string // human-readable tier name
	Blocked  bool   // true if denied by permissions
}

// AuditReport is the result of scanning all imports against permissions.
type AuditReport struct {
	Entries    []AuditEntry
	HasBlocked bool   // true if any denied imports were found
	Summary    string // human-readable summary
}

// RunAudit scans all imported stdlib modules against the permission configuration.
// Returns an AuditReport with entries for each module and whether the build should be blocked.
//
// Parameters:
//   - imports: set of all stdlib module names used in the import graph
//   - perms: the parsed [permissions] section (nil = no restrictions)
func RunAudit(imports []string, perms *Permissions) *AuditReport {
	report := &AuditReport{}

	if perms == nil {
		// No permissions configured — everything is allowed
		for _, mod := range imports {
			tier, cat := ClassifyModule(mod)
			report.Entries = append(report.Entries, AuditEntry{
				Module:   mod,
				Tier:     tier,
				Category: cat,
				Blocked:  false,
			})
		}
		return report
	}

	allowSet := toSet(perms.Allow)
	denySet := toSet(perms.Deny)

	for _, mod := range imports {
		tier, cat := ClassifyModule(mod)
		blocked := false

		// Check deny list first (deny takes precedence)
		if _, denied := denySet[mod]; denied {
			blocked = true
		} else if _, deniedCat := denySet[cat]; deniedCat {
			// Deny by category: deny = ["privileged"] blocks all Tier 3 modules
			blocked = true
		}

		// If an allow list exists, only explicitly allowed modules pass
		if len(perms.Allow) > 0 && !blocked {
			_, allowed := allowSet[mod]
			_, allowedCat := allowSet[cat]
			if !allowed && !allowedCat && tier > TierSafe {
				blocked = true
			}
		}

		if blocked {
			report.HasBlocked = true
		}

		report.Entries = append(report.Entries, AuditEntry{
			Module:   mod,
			Tier:     tier,
			Category: cat,
			Blocked:  blocked,
		})
	}

	report.Summary = buildSummary(report)
	return report
}

// FormatReport returns a human-readable audit report string.
func FormatReport(report *AuditReport) string {
	if report == nil || len(report.Entries) == 0 {
		return "Build Audit: no stdlib imports detected.\n"
	}

	var sb strings.Builder
	sb.WriteString("┌─────────────────────────────────────────┐\n")
	sb.WriteString("│           Build Audit Report            │\n")
	sb.WriteString("├─────────────────────────────────────────┤\n")

	// Group by tier
	tiers := map[int][]AuditEntry{}
	for _, e := range report.Entries {
		tiers[e.Tier] = append(tiers[e.Tier], e)
	}

	for tier := 0; tier <= 3; tier++ {
		entries := tiers[tier]
		if len(entries) == 0 {
			continue
		}
		cat := TierName(tier)
		sb.WriteString(fmt.Sprintf("│ [%s] %-35s│\n", strings.ToUpper(cat[:1])+cat[1:], ""))
		for _, e := range entries {
			status := "  ✓"
			if e.Blocked {
				status = "  ✗ BLOCKED"
			}
			sb.WriteString(fmt.Sprintf("│   %-20s %s%-14s│\n", e.Module, status, ""))
		}
	}

	sb.WriteString("├─────────────────────────────────────────┤\n")
	if report.HasBlocked {
		sb.WriteString("│ ✗ Build BLOCKED — denied imports found  │\n")
	} else {
		sb.WriteString("│ ✓ All imports approved                  │\n")
	}
	sb.WriteString("└─────────────────────────────────────────┘\n")

	return sb.String()
}

// FormatJSON returns a JSON-formatted audit report.
func FormatJSON(report *AuditReport) string {
	if report == nil {
		return `{"entries":[],"blocked":false}`
	}

	var sb strings.Builder
	sb.WriteString(`{"entries":[`)
	for i, e := range report.Entries {
		if i > 0 {
			sb.WriteString(",")
		}
		blocked := "false"
		if e.Blocked {
			blocked = "true"
		}
		sb.WriteString(fmt.Sprintf(`{"module":"%s","tier":%d,"category":"%s","blocked":%s}`,
			e.Module, e.Tier, e.Category, blocked))
	}
	sb.WriteString(fmt.Sprintf(`],"blocked":%v}`, report.HasBlocked))
	return sb.String()
}

func buildSummary(report *AuditReport) string {
	total := len(report.Entries)
	blocked := 0
	for _, e := range report.Entries {
		if e.Blocked {
			blocked++
		}
	}
	if blocked == 0 {
		return fmt.Sprintf("%d imports scanned, all approved", total)
	}
	return fmt.Sprintf("%d imports scanned, %d BLOCKED", total, blocked)
}

func toSet(items []string) map[string]struct{} {
	s := make(map[string]struct{}, len(items))
	for _, item := range items {
		s[strings.TrimSpace(item)] = struct{}{}
	}
	return s
}
