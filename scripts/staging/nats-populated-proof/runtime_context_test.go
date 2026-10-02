package main

import (
	"strings"
	"testing"
	"time"
)

func contextFixture() (runtimeContext, runtimeIdentity) {
	owner := "voice-populated-" + strings.Repeat("a", 24)
	c := runtimeContext{Schema: schema, Kind: "fixture", Owner: owner, Role: "diagnostic", Container: owner + "-diagnostic", ContainerID: strings.Repeat("b", 64), ManifestSHA: strings.Repeat("c", 64), IP: "172.24.0.3", HubIP: "172.24.0.2", Subnet: "172.24.0.0/16", CreatedAt: time.Now()}
	id := runtimeIdentity{hostname: c.Container, addresses: []string{c.IP}, routes: "Iface Destination Gateway Flags\neth0 000018AC 00000000 0001", status: "NoNewPrivs:\t1\nCapEff:\t0000000000000000", mounts: []runtimeMount{{path: "/", root: "/", fs: "overlay", readonly: true}, {path: "/etc/hostname", root: "/var/lib/docker/containers/" + c.ContainerID + "/hostname", fs: "ext4"}}}
	for _, p := range contextFiles(c.Role) {
		id.mounts = append(id.mounts, runtimeMount{path: p, root: "/secure/" + owner + "-owned-random/input", fs: "ext4", readonly: true})
	}
	return c, id
}
func TestInspectedOwnedRuntimeContext(t *testing.T) {
	c, id := contextFixture()
	if e := verifyRuntimeContext(c, id, "setup", time.Now()); e != nil {
		t.Fatal(e)
	}
}
func TestPinnedAlpineAliasAndReadonlyDockerCgroupV1(t *testing.T) {
	c, id := contextFixture()
	for i := range id.mounts {
		id.mounts[i].path = runtimeMountPath(id.mounts[i].path)
	}
	id.mounts = append(id.mounts, runtimeMount{path: "/sys/fs/cgroup", root: "/", fs: "tmpfs"}, runtimeMount{path: "/sys/fs/cgroup/memory", root: "/", fs: "cgroup", readonly: true})
	if e := verifyRuntimeContext(c, id, "setup", time.Now()); e != nil {
		t.Fatal(e)
	}
	id.mounts[len(id.mounts)-1].readonly = false
	if e := verifyRuntimeContext(c, id, "setup", time.Now()); e == nil {
		t.Fatal("writable host cgroup controller accepted")
	}
	id.mounts[len(id.mounts)-1] = runtimeMount{path: "/sys/fs/cgroup/source", root: "/retained-source", fs: "ext4", readonly: true}
	if e := verifyRuntimeContext(c, id, "setup", time.Now()); e == nil {
		t.Fatal("source filesystem hidden under cgroup accepted")
	}
}
func TestRuntimeContextRejectsAmbientAndWrongPhase(t *testing.T) {
	cases := []struct {
		name   string
		change func(*runtimeContext, *runtimeIdentity)
		phase  string
	}{
		{"unowned hostname", func(c *runtimeContext, id *runtimeIdentity) { id.hostname = "staging-hub" }, "setup"},
		{"wrong Docker container ID", func(c *runtimeContext, id *runtimeIdentity) { c.ContainerID = strings.Repeat("d", 64) }, "setup"},
		{"source mount", func(c *runtimeContext, id *runtimeIdentity) { id.mounts[3].root = "/retained-source-pvc/private" }, "setup"},
		{"missing readonly context", func(c *runtimeContext, id *runtimeIdentity) { id.mounts[3].readonly = false }, "setup"},
		{"extra credential holder", func(c *runtimeContext, id *runtimeIdentity) {
			id.mounts = append(id.mounts, runtimeMount{path: "/all-credentials", root: "/source", fs: "ext4", readonly: true})
		}, "setup"},
		{"default external route", func(c *runtimeContext, id *runtimeIdentity) { id.routes += "\neth0 00000000 010018AC 0003" }, "setup"},
		{"ambient IP", func(c *runtimeContext, id *runtimeIdentity) { id.addresses = []string{"192.168.1.9"} }, "setup"},
		{"wrong mounted phase", func(c *runtimeContext, id *runtimeIdentity) {}, "leaf-publish"},
		{"unenrolled deployed context", func(c *runtimeContext, id *runtimeIdentity) { c.Kind = "deployed" }, "setup"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, id := contextFixture()
			tc.change(&c, &id)
			if e := verifyRuntimeContext(c, id, tc.phase, time.Now()); e == nil {
				t.Fatal("ambient/forged/wrong-phase context accepted")
			}
		})
	}
}
func TestForgedMarkerWithoutOwnedContextIsDefaultDenied(t *testing.T) {
	t.Setenv("NATS_POPULATED_PROOF_RUNTIME", "isolated-internal-v1")
	for _, phase := range []string{"setup", "exercise", "leaf-publish"} {
		if e := run([]string{"runtime", phase}); e == nil || e.Error() != "runtime_container_context_missing" {
			t.Fatalf("must refuse ambient helper before any broker access: %v", e)
		}
	}
}
