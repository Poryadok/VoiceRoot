package mmadmissionwire

import (
	"context"
	"errors"
	"testing"
	"time"
)

type rejectDialContext struct {
	context.Context
	deadlineCalls int
}

func (c *rejectDialContext) Deadline() (time.Time, bool) {
	c.deadlineCalls++
	return time.Now().Add(-time.Second), true
}

func TestClientRejectsDescriptorIdentityDrift(t *testing.T) {
	server, own := launchTestCertificate(t, true, false), launchTestCertificate(t, false, false)
	p := launchTestBinding(t, server, own)
	d, _, _ := testObservation()
	expected := LaunchExpectations{p.ControllerProcessLaunchID, p.ControllerLaunchObservationHash, p.ConsumerLaunchObservationHash, p.SocketMountObservationHash, p.LaunchKeyCustodyContractHash, p.ControllerExpectedKernelPeer.ObserverNamespaceID, p.ConsumerExpectedKernelPeer.ObserverNamespaceID}
	validated, e := validatePeerBinding(d, p, expected)
	if e != nil {
		t.Fatal(e)
	}
	changes := map[string]func(*DescriptorV1){
		"controller":  func(x *DescriptorV1) { x.ControllerID[0]++ },
		"scope-id":    func(x *DescriptorV1) { x.ScopeID[0]++ },
		"scope":       func(x *DescriptorV1) { x.AuthorityScope = Installed },
		"binding":     func(x *DescriptorV1) { x.Binding.AdmissionGeneration++ },
		"replica":     func(x *DescriptorV1) { x.ReplicaID[0]++ },
		"process":     func(x *DescriptorV1) { x.ProcessLaunchID[0]++ },
		"lineage":     func(x *DescriptorV1) { x.DatabaseLineageID[0]++ },
		"image":       func(x *DescriptorV1) { x.AdmittedImageDigest = "sha256:" + Hash{90}.String() },
		"executable":  func(x *DescriptorV1) { x.AdmittedExecutableHash[0]++ },
		"argv":        func(x *DescriptorV1) { x.AdmittedArgv = []string{"/different-native"} },
		"config":      func(x *DescriptorV1) { x.ConfigContractHash[0]++ },
		"credentials": func(x *DescriptorV1) { x.CredentialRefsManifestHash[0]++ },
		"upstream":    func(x *DescriptorV1) { x.UpstreamContractManifestHash[0]++ },
		"broker":      func(x *DescriptorV1) { x.BrokerContractManifestHash[0]++ },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			bad := d
			bad.AdmittedArgv = append([]string(nil), d.AdmittedArgv...)
			change(&bad)
			if bad.Validate() != nil {
				t.Fatal("test must preserve a structurally valid descriptor")
			}
			if _, e = NewReadbackClient(bad, validated, own, time.Second); !errors.Is(e, ErrInvalid) {
				t.Errorf("constructor accepted foreign identity: %v", e)
			}
			c, e := NewReadbackClient(d, validated, own, time.Second)
			if e != nil {
				t.Fatal(e)
			}
			c.Descriptor = bad
			// Any accidental dial consults this deadline; identity denial must happen
			// before invoking the transport, including for an otherwise valid binding.
			ctx := &rejectDialContext{Context: context.Background()}
			if _, e = c.ObserveStartup(ctx); !errors.Is(e, ErrInvalid) {
				t.Errorf("startup mismatch returned %v", e)
			}
			if _, e = c.ObserveFresh(ctx, ID{70}, Hash{71}); !errors.Is(e, ErrInvalid) {
				t.Errorf("fresh mismatch returned %v", e)
			}
			if ctx.deadlineCalls != 0 {
				t.Error("identity mismatch reached the dialer")
			}
		})
	}
	// Protected bootstrap and client snapshots must not alias caller-owned argv.
	c, e := NewReadbackClient(d, validated, own, time.Second)
	if e != nil {
		t.Fatal(e)
	}
	d.AdmittedArgv[0] = "/mutated-after-bootstrap"
	if c.Descriptor.AdmittedArgv[0] == d.AdmittedArgv[0] {
		t.Fatal("client retained caller-owned descriptor argv")
	}
	c.Descriptor.AdmittedArgv[0] = "/mutated-client-argv"
	if _, e = c.ObserveStartup(context.Background()); !errors.Is(e, ErrInvalid) {
		t.Fatal("mutable argv changed protected identity", e)
	}
}
