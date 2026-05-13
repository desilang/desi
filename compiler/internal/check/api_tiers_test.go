package check

import "testing"

func TestClassifyModule_KnownModules(t *testing.T) {
	cases := []struct {
		module   string
		wantTier int
		wantCat  string
	}{
		{"math", TierSafe, "safe"},
		{"json", TierSafe, "safe"},
		{"strings", TierSafe, "safe"},
		{"perf", TierSafe, "safe"},
		{"ast", TierSafe, "safe"},

		{"fs", TierSystem, "system"},
		{"os", TierSystem, "system"},
		{"env", TierSystem, "system"},

		{"http", TierNetwork, "network"},
		{"net", TierNetwork, "network"},
		{"websocket", TierNetwork, "network"},

		{"shell", TierPrivileged, "privileged"},
		{"crypto", TierPrivileged, "privileged"},
		{"process", TierPrivileged, "privileged"},
	}
	for _, tc := range cases {
		tier, cat := ClassifyModule(tc.module)
		if tier != tc.wantTier || cat != tc.wantCat {
			t.Errorf("ClassifyModule(%q) = (%d, %q), want (%d, %q)",
				tc.module, tier, cat, tc.wantTier, tc.wantCat)
		}
	}
}

func TestClassifyModule_Unknown(t *testing.T) {
	tier, cat := ClassifyModule("some_unknown_module")
	if tier != TierSafe || cat != "safe" {
		t.Errorf("unknown module: got (%d, %q), want (%d, %q)", tier, cat, TierSafe, "safe")
	}
}

func TestTierName(t *testing.T) {
	cases := []struct {
		tier int
		want string
	}{
		{TierSafe, "safe"},
		{TierSystem, "system"},
		{TierNetwork, "network"},
		{TierPrivileged, "privileged"},
		{99, "unknown"},
	}
	for _, tc := range cases {
		got := TierName(tc.tier)
		if got != tc.want {
			t.Errorf("TierName(%d) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}
