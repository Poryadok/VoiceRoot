package gisprincipal

import "testing"

func TestComposedRoleAndRealtimeAllowlist(t *testing.T) {
	for _, pair := range []struct {
		issuer, own, foreign string
	}{
		{"role", ScopeAuthorityMethod, ResolveRealtimeLifecycleMethod},
		{"realtime", ResolveRealtimeLifecycleMethod, ScopeAuthorityMethod},
	} {
		t.Run(pair.issuer, func(t *testing.T) {
			methods, err := configuredMethods(pair.issuer, []string{pair.own})
			if err != nil || len(methods) != 1 {
				t.Fatalf("exact own method rejected: methods=%v err=%v", methods, err)
			}
			if _, present := methods[pair.own]; !present {
				t.Fatal("exact own method missing")
			}
			for _, allowed := range [][]string{{pair.foreign}, {pair.own, pair.foreign}} {
				if _, err := configuredMethods(pair.issuer, allowed); err == nil {
					t.Fatalf("foreign-purpose method accepted: %v", allowed)
				}
			}
		})
	}
}
