package mmadmissionctl

import (
	"context"
	"path/filepath"
	"testing"
	w "voice/backend/pkg/mmadmissionwire"
)

type unitDriver struct {
	closed    int
	failClose bool
}

func (*unitDriver) Scope() w.Scope { return w.LocalOwned }
func (d *unitDriver) ObserveClosed(_ context.Context, s w.LaunchSpecV1, _ w.ID) (Evidence, error) {
	h, _ := w.LaunchHash(s)
	return Evidence{ScopeID: s.ScopeID, LaunchSpecHash: h, DatabaseBindingHash: w.Hash{1}, SchemaContractHash: w.Hash{2}, SourceFactoryManifestHash: w.Hash{3}, FreshRouteManifestHash: w.Hash{4}, LoginRoleOID: 42, LoginRoleName: "mm_app", CredentialCohortID: w.ID{5}, ClosureHash: w.Hash{6}, ResourcesHash: w.Hash{7}}, nil
}
func (*unitDriver) InstallHead(context.Context, Snapshot, w.ID) error { return nil }
func (d *unitDriver) StartPrepared(ctx context.Context, s Snapshot, id w.ID, register func(Evidence) error) (Evidence, error) {
	e, err := d.ObservePrepared(ctx, s, id)
	if err != nil {
		return e, err
	}
	return e, register(e)
}
func (d *unitDriver) ObservePrepared(ctx context.Context, s Snapshot, id w.ID) (Evidence, error) {
	e, _ := d.ObserveClosed(ctx, *s.Spec, id)
	e.LaunchesHash = w.Hash{8}
	e.PreparedHash = w.Hash{9}
	for _, r := range s.Spec.Replicas {
		e.Peers = append(e.Peers, ReplicaPeer{ReplicaID: r.ReplicaID, LaunchID: r.LaunchID, PID: 10, UID: r.RuntimeUID, ExecutableHash: r.ExecutableHash})
	}
	return e, nil
}
func (*unitDriver) OpenIngress(context.Context, Snapshot, w.ID) error { return nil }
func (d *unitDriver) CloseAndDrain(ctx context.Context, s Snapshot, id w.ID) (Evidence, error) {
	d.closed++
	if d.failClose {
		return Evidence{}, ErrPending
	}
	return d.ObserveClosed(ctx, *s.Spec, id)
}
func (d *unitDriver) ReconcileClosed(ctx context.Context, s Snapshot, id w.ID) (Evidence, error) {
	return d.CloseAndDrain(ctx, s, id)
}
func unitSpec() w.LaunchSpecV1 {
	return w.LaunchSpecV1{Version: 1, AuthorityScope: w.LocalOwned, ScopeID: w.ID{1}, ImageDigest: "sha256:" + w.Hash{1}.String(), ConfigContractHash: w.Hash{1}, CredentialRefsManifestHash: w.Hash{2}, UpstreamContractManifestHash: w.Hash{3}, BrokerContractManifestHash: w.Hash{4}, FreshRoutes: []string{"StartSearch"}, Factories: []string{"native-voice"}, Resources: []w.ResourceV1{{ID: "nats", Kind: "NATS", Target: "private-nats", ContractHash: w.Hash{1}}, {ID: "pg", Kind: "POSTGRES", Target: "private-pg", ContractHash: w.Hash{2}}, {ID: "redis", Kind: "REDIS", Target: "private-redis", ContractHash: w.Hash{3}}}, Replicas: []w.ReplicaV1{{ReplicaID: w.ID{2}, LaunchID: w.ID{3}, RuntimeUID: 65532, ExecutableHash: w.Hash{4}, Argv: []string{"/service-native"}, Mounts: []w.MountV1{{ID: "descriptor", Destination: w.DescriptorPath, ReadOnly: true, ContentHash: w.Hash{1}}, {ID: "readback", Destination: w.ReadbackSocket, ReadOnly: true, ContentHash: w.Hash{2}}}}}}
}
func TestControllerCycleAndDeniedPrematureOpen(t *testing.T) {
	ctx := context.Background()
	j, e := CreateLocalJournal(filepath.Join(t.TempDir(), "control.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer j.Close()
	d := &unitDriver{}
	c, e := NewLocalController(j, d, w.ID{1}, w.ID{2}, w.ID{3})
	if e != nil {
		t.Fatal(e)
	}
	spec := unitSpec()
	cmd := Command{Version: 1, Kind: "PREPARE", OperationID: w.ID{11}, LaunchSpec: &spec}
	r, e := c.Execute(ctx, cmd)
	if e != nil || r.Snapshot.State != w.Preparing {
		t.Fatalf("prepare: %+v %v", r, e)
	}
	replay, e := c.Execute(ctx, cmd)
	if e != nil || replay.Revision != r.Revision {
		t.Fatal("exact replay changed result", e)
	}
	bad := Command{Version: 1, Kind: "OPEN", OperationID: w.ID{12}, ExpectedRevision: r.Revision, TransitionID: w.ID{11}}
	r, e = c.Execute(ctx, bad)
	if e == nil || r.Snapshot.State != w.Preparing {
		t.Fatal("premature OPEN accepted", r, e)
	}
	for i, step := range []struct {
		kind  string
		state w.State
	}{{"ARM", w.Armed}, {"OPEN", w.Open}, {"CLOSE", w.Closed}} {
		cmd = Command{Version: 1, Kind: step.kind, OperationID: w.ID{byte(13 + i)}, ExpectedRevision: r.Revision, TransitionID: w.ID{11}}
		r, e = c.Execute(ctx, cmd)
		if e != nil || r.Snapshot.State != step.state {
			t.Fatalf("%s: %+v %v", step.kind, r, e)
		}
	}
	if d.closed != 1 {
		t.Fatal("close did not drain actual driver")
	}
}
