package mmadmissionwire

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"io"
	"net"
	"reflect"
	"time"
)

type PeerIdentity struct {
	PID            int32
	UID            uint32
	GID            uint32
	ExecutableHash Hash
}

// Client requires an independently verified launch/mount identity. Descriptor bytes
// and a socket name are not a substitute for the driver's custody observation.
type Client struct {
	Descriptor         DescriptorV1
	ControllerPeer     PeerIdentity
	Timeout            time.Duration
	peerBinding        *ReadbackPeerBindingV1
	ownTLS             tls.Certificate
	admittedDescriptor DescriptorV1
}

// NewReadbackClient is the only operational constructor. A descriptor and raw
// numeric peer identity alone do not authenticate independent launch provenance.
func NewReadbackClient(d DescriptorV1, p ValidatedPeerBinding, own tls.Certificate, timeout time.Duration) (Client, error) {
	if d.Validate() != nil || p.record.Validate() != nil || !reflect.DeepEqual(d, p.descriptor) || timeout <= 0 || timeout > 10*time.Second {
		return Client{}, ErrInvalid
	}
	h, e := BindingHash(d.Binding)
	if e != nil || p.record.BindingHash != h || p.record.ConsumerReplicaID != d.ReplicaID || p.record.ConsumerProcessLaunchID != d.ProcessLaunchID {
		return Client{}, ErrInvalid
	}
	if _, e = launchTLSConfig(p.record, own, false); e != nil {
		return Client{}, e
	}
	record := p.record
	return Client{Descriptor: copyDescriptor(d), admittedDescriptor: copyDescriptor(p.descriptor), Timeout: timeout, peerBinding: &record, ownTLS: own}, nil
}

func copyDescriptor(d DescriptorV1) DescriptorV1 {
	d.AdmittedArgv = append([]string(nil), d.AdmittedArgv...)
	return d
}

type StartupObservation struct{ response ObserveResponseV1 }
type FreshAdmissionObservation struct{ response ObserveResponseV1 }

func (o StartupObservation) Binding() Binding        { return o.response.ExactBinding }
func (o FreshAdmissionObservation) Binding() Binding { return o.response.ExactBinding }
func (o FreshAdmissionObservation) Stamp() AdmissionStampV1 {
	return AdmissionStampV1{Version: 1, BindingHash: o.response.BindingHash, DatabaseLineageID: o.response.DatabaseLineageID, ContinuityHistoryRootHash: o.response.ContinuityReceiptsRootHash}
}
func (c Client) ObserveStartup(ctx context.Context) (StartupObservation, error) {
	r, e := c.observe(ctx, Startup, ID{}, Hash{})
	return StartupObservation{response: r}, e
}
func (c Client) ObserveFresh(ctx context.Context, action ID, request Hash) (FreshAdmissionObservation, error) {
	r, e := c.observe(ctx, FreshAction, action, request)
	return FreshAdmissionObservation{response: r}, e
}
func (c Client) observe(ctx context.Context, p Purpose, action ID, request Hash) (ObserveResponseV1, error) {
	var zero ObserveResponseV1
	d := c.Descriptor
	if d.Validate() != nil || !reflect.DeepEqual(d, c.admittedDescriptor) || c.Timeout <= 0 || c.Timeout > 10*time.Second || c.peerBinding == nil || c.peerBinding.Validate() != nil {
		return zero, ErrInvalid
	}
	h, e := BindingHash(d.Binding)
	if e != nil || h != c.peerBinding.BindingHash || d.ReplicaID != c.peerBinding.ConsumerReplicaID || d.ProcessLaunchID != c.peerBinding.ConsumerProcessLaunchID || d.ControllerID != c.peerBinding.ControllerID || d.AuthorityScope != c.peerBinding.AuthorityScope || d.ScopeID != c.peerBinding.ScopeID {
		return zero, ErrInvalid
	}
	r := ObserveRequestV1{ProtocolVersion: 1, Purpose: p, EnvironmentID: d.Binding.EnvironmentID, InstallationID: d.Binding.InstallationID, ReplicaID: d.ReplicaID, ProcessLaunchID: d.ProcessLaunchID, ExpectedBindingHash: h, ActionID: action, ActionRequestHash: request}
	if _, e = rand.Read(r.Nonce[:]); e != nil {
		return zero, e
	}
	if r.Validate() != nil {
		return zero, ErrInvalid
	}
	dial := net.Dialer{Timeout: c.Timeout}
	raw, e := dial.DialContext(ctx, "unix", ReadbackSocket)
	if e != nil {
		return zero, ErrInvalid
	}
	defer func() { _ = raw.Close() }()
	conn, ok := raw.(*net.UnixConn)
	if !ok {
		return zero, ErrInvalid
	}
	deadline := time.Now().Add(c.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	secure, e := authenticateReadbackClient(ctx, conn, *c.peerBinding, c.ownTLS)
	if e != nil {
		return zero, ErrInvalid
	}
	if e = WriteFrame(secure, r); e != nil {
		return zero, e
	}
	if e = secure.CloseWrite(); e != nil {
		return zero, e
	}
	var out ObserveResponseV1
	if ReadFrame(secure, &out) != nil {
		return zero, ErrInvalid
	}
	var extra [1]byte
	if n, e := secure.Read(extra[:]); n != 0 || e != io.EOF {
		return zero, ErrInvalid
	}
	if ValidateObservation(d, r, out) != nil || out.LaunchObservationHash != c.peerBinding.ConsumerLaunchObservationHash {
		return zero, ErrInvalid
	}
	return out, nil
}
func ValidateObservation(d DescriptorV1, r ObserveRequestV1, out ObserveResponseV1) error {
	if d.Validate() != nil || r.Validate() != nil || out.Validate() != nil || out.Nonce != r.Nonce || out.ControllerID != d.ControllerID || out.AuthorityScope != d.AuthorityScope || out.ScopeID != d.ScopeID || out.ExactBinding != d.Binding || out.BindingHash != r.ExpectedBindingHash || out.DatabaseLineageID != d.DatabaseLineageID || out.ActionID != r.ActionID || out.ActionRequestHash != r.ActionRequestHash || r.ReplicaID != d.ReplicaID || r.ProcessLaunchID != d.ProcessLaunchID || r.EnvironmentID != d.Binding.EnvironmentID || r.InstallationID != d.Binding.InstallationID {
		return ErrInvalid
	}
	if r.Purpose == FreshAction && out.State != Open {
		return ErrInvalid
	}
	if r.Purpose == Startup && out.State != Armed && out.State != Open {
		return ErrInvalid
	}
	return nil
}
