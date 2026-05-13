package check

import (
	"strings"
	"testing"
)

func TestRunAudit_NoPermissions(t *testing.T) {
	imports := []string{"math", "json", "fs"}
	report := RunAudit(imports, nil)

	if report.HasBlocked {
		t.Fatal("expected no blocks with nil permissions")
	}
	if len(report.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(report.Entries))
	}
	for _, e := range report.Entries {
		if e.Blocked {
			t.Errorf("entry %q should not be blocked", e.Module)
		}
	}
}

func TestRunAudit_DenyBlocksModule(t *testing.T) {
	imports := []string{"math", "shell", "fs"}
	perms := &Permissions{
		Deny: []string{"shell"},
	}
	report := RunAudit(imports, perms)

	if !report.HasBlocked {
		t.Fatal("expected blocked, deny = [shell]")
	}

	var blocked []string
	for _, e := range report.Entries {
		if e.Blocked {
			blocked = append(blocked, e.Module)
		}
	}
	if len(blocked) != 1 || blocked[0] != "shell" {
		t.Errorf("expected only 'shell' blocked, got %v", blocked)
	}
}

func TestRunAudit_DenyByCategory(t *testing.T) {
	imports := []string{"shell", "crypto", "math"}
	perms := &Permissions{
		Deny: []string{"privileged"},
	}
	report := RunAudit(imports, perms)

	if !report.HasBlocked {
		t.Fatal("expected blocked for privileged category")
	}
	blockedCount := 0
	for _, e := range report.Entries {
		if e.Blocked {
			blockedCount++
		}
	}
	if blockedCount != 2 {
		t.Errorf("expected 2 blocked (shell+crypto), got %d", blockedCount)
	}
}

func TestRunAudit_AllowOnly(t *testing.T) {
	imports := []string{"math", "json", "fs", "http"}
	perms := &Permissions{
		Allow: []string{"math", "json"},
	}
	report := RunAudit(imports, perms)

	if !report.HasBlocked {
		t.Fatal("expected blocked: fs and http not in allow list")
	}

	for _, e := range report.Entries {
		switch e.Module {
		case "math", "json":
			if e.Blocked {
				t.Errorf("%q should be allowed", e.Module)
			}
		case "fs", "http":
			if !e.Blocked {
				t.Errorf("%q should be blocked (not in allow list)", e.Module)
			}
		}
	}
}

func TestRunAudit_DenyTakesPrecedence(t *testing.T) {
	imports := []string{"shell"}
	perms := &Permissions{
		Allow: []string{"shell"}, // explicitly allowed
		Deny:  []string{"shell"}, // but also denied
	}
	report := RunAudit(imports, perms)

	if !report.HasBlocked {
		t.Fatal("deny should take precedence over allow")
	}
}

func TestFormatReport_NoEntries(t *testing.T) {
	report := &AuditReport{}
	out := FormatReport(report)
	if !strings.Contains(out, "no stdlib imports") {
		t.Errorf("expected 'no stdlib imports' message, got: %s", out)
	}
}

func TestFormatReport_WithEntries(t *testing.T) {
	report := &AuditReport{
		Entries: []AuditEntry{
			{Module: "math", Tier: TierSafe, Category: "safe", Blocked: false},
			{Module: "shell", Tier: TierPrivileged, Category: "privileged", Blocked: true},
		},
		HasBlocked: true,
	}
	out := FormatReport(report)
	if !strings.Contains(out, "math") {
		t.Error("expected 'math' in report")
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Error("expected 'BLOCKED' in report")
	}
	if !strings.Contains(out, "shell") {
		t.Error("expected 'shell' in report")
	}
}

func TestFormatJSON(t *testing.T) {
	report := &AuditReport{
		Entries: []AuditEntry{
			{Module: "math", Tier: 0, Category: "safe", Blocked: false},
		},
		HasBlocked: false,
	}
	out := FormatJSON(report)
	if !strings.Contains(out, `"module":"math"`) {
		t.Errorf("expected module:math in JSON, got: %s", out)
	}
	if !strings.Contains(out, `"blocked":false`) {
		t.Errorf("expected blocked:false, got: %s", out)
	}
}

func TestFormatJSON_Nil(t *testing.T) {
	out := FormatJSON(nil)
	if out != `{"entries":[],"blocked":false}` {
		t.Errorf("unexpected nil report JSON: %s", out)
	}
}
