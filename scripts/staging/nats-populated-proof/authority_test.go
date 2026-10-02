package main

import "testing"

func TestProductionCallerCannotEnrollOwnWitness(t *testing.T) {
	b := newBundle(t)
	if e := run([]string{"--inputs", b.dir, "--acl", b.acl, "--approved-trust", b.trust}); e == nil || e.Error() != "custody_trust_anchor_not_enrolled" {
		t.Fatalf("self-created policy must never authorize production evidence: %v", e)
	}
}
func TestValidFixtureCannotMasqueradeAsDeployed(t *testing.T) {
	b := newBundle(t)
	b.m.Kind = "fixture"
	b.save(t)
	if _, e := loadInput(b.dir, b.acl, b.trust, b.pin, false, b.now); e == nil || e.Error() != "evidence_kind_mismatch" {
		t.Fatalf("fixture cannot become deployed: %v", e)
	}
}
