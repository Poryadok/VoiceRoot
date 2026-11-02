package sourcecustody

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type nilStreamClient struct{}

func (nilStreamClient) AcquireUserSourcePermit(context.Context, *userv1.AcquireUserSourcePermitRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[userv1.AcquireUserSourcePermitResponse], error) {
	return nil, nil
}

func TestUserCustodyClientZeroGrantAndNilStreamFailClosed(t *testing.T) {
	var absent *Grant
	require.Error(t, absent.Context().Err())
	absent.Release()
	var zero Grant
	require.Error(t, zero.Context().Err())
	zero.Release()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client, err := NewClient(nilStreamClient{}, issuer, time.Second)
	require.NoError(t, err)
	grant, err := client.Acquire(context.Background(), custodyBinding())
	require.Error(t, err)
	require.Nil(t, grant)
}

// This uses a real protected handler and disposable control PostgreSQL, but
// synthetic OPEN and miniredis do not establish production custody.
func TestUserCustodyClientSignedGrantHeldUntilRelease(t *testing.T) {
	f := newStreamFixture(t)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	b := f.request.Binding
	var digest [32]byte
	copy(digest[:], b.KeysetDigest)
	binding := Binding{Epoch: int64(b.Epoch), SourceID: b.SourceId, HistoryGeneration: uuid.MustParse(b.HistoryGeneration), UserSourceGeneration: uuid.MustParse(b.UserSourceGeneration), KeysetDigest: digest}
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	grant, err := client.Acquire(ctx, binding)
	require.NoError(t, err)
	require.NotNil(t, grant)
	require.NoError(t, grant.Context().Err())
	require.EqualValues(t, 1, f.tracer.attempts.Load(), "real server verified a signed request and acquired the PG permit")
	require.NoError(t, f.control.CloseAdmission(f.ctx))
	short, stop := context.WithTimeout(f.ctx, 250*time.Millisecond)
	require.Error(t, f.control.WaitPermits(short))
	stop()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); grant.Release() }()
	}
	released := make(chan struct{})
	go func() { wg.Wait(); close(released) }()
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent Release did not join")
	}
	require.Error(t, grant.Context().Err())
	drain, stop := context.WithTimeout(f.ctx, 5*time.Second)
	defer stop()
	require.NoError(t, f.control.WaitPermits(drain))
}

// This deliberately adversarial TLS peer tests client frame handling only. It
// is not a controller or a source-authority fixture.
type maliciousPermitPeer struct {
	userv1.UnimplementedUserSourceCustodyServiceServer
	mode     string
	requests chan metadata.MD
	proceed  chan struct{}
}

func (p *maliciousPermitPeer) AcquireUserSourcePermit(req *userv1.AcquireUserSourcePermitRequest, stream grpc.ServerStreamingServer[userv1.AcquireUserSourcePermitResponse]) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	p.requests <- md.Copy()
	if p.mode == "no-first" {
		<-stream.Context().Done()
		return stream.Context().Err()
	}
	frame := &userv1.AcquireUserSourcePermitResponse{Protocol: req.Protocol, Binding: proto.Clone(req.Binding).(*userv1.UserSourcePermitBinding), Sequence: 1, Kind: userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED}
	if req.Protocol == 2 {
		frame.Physical = &userv1.UserSourcePhysicalBinding{SystemIdentifier: "123456", Database: "source_fixture", DatabaseOid: 123}
	}
	switch p.mode {
	case "wrong-first-kind":
		frame.Kind = userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT
	case "wrong-first-seq":
		frame.Sequence = 2
	case "wrong-first-binding":
		frame.Binding.SourceId = "wrong-source"
	case "wrong-first-protocol":
		frame.Protocol = 2
	case "unknown-first":
		frame.Binding.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	case "unknown-outer-first":
		frame.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	case "missing-first-binding":
		frame.Binding = nil
	case "missing-physical":
		frame.Physical = nil
	case "invalid-physical":
		frame.Physical.DatabaseOid = 0
	case "unknown-physical":
		frame.Physical.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	}
	if err := stream.Send(frame); err != nil {
		return err
	}
	select {
	case <-p.proceed:
	case <-stream.Context().Done():
		return stream.Context().Err()
	}
	if p.mode == "eof" {
		return nil
	}
	if p.mode == "no-heartbeat" {
		<-stream.Context().Done()
		return stream.Context().Err()
	}
	frame.Kind = userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT
	frame.Sequence = 2
	switch p.mode {
	case "duplicate":
		frame.Sequence = 1
	case "gap":
		frame.Sequence = 3
	case "repeat-granted":
		frame.Kind = userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED
	case "unknown-kind":
		frame.Kind = userv1.UserSourcePermitFrameKind(100)
	case "changed-binding":
		frame.Binding.SourceId = "wrong-source"
	case "unknown-heartbeat":
		frame.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	case "unknown-heartbeat-binding":
		frame.Binding.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	case "changed-physical":
		frame.Physical.DatabaseOid++
	case "removed-physical":
		frame.Physical = nil
	}
	if err := stream.Send(frame); err != nil {
		return err
	}
	if p.mode == "valid-heartbeats" {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stream.Context().Done():
				return stream.Context().Err()
			case <-ticker.C:
				frame.Sequence++
				if err := stream.Send(frame); err != nil {
					return err
				}
			}
		}
	}
	<-stream.Context().Done()
	return stream.Context().Err()
}

func TestQualifiedClientRejectsMissingOrMalformedPhysicalBinding(t *testing.T) {
	for _, mode := range []string{"missing-physical", "invalid-physical", "unknown-physical"} {
		t.Run(mode, func(t *testing.T) {
			client, _, binding := adversarialClient(t, mode)
			grant, err := client.AcquireQualified(context.Background(), binding)
			if grant != nil {
				grant.Release()
			}
			require.Error(t, err)
		})
	}
}

func TestQualifiedClientCancelsOnChangedPhysicalHeartbeat(t *testing.T) {
	for _, mode := range []string{"changed-physical", "removed-physical"} {
		t.Run(mode, func(t *testing.T) {
			client, peer, binding := adversarialClient(t, mode)
			grant, err := client.AcquireQualified(context.Background(), binding)
			require.NoError(t, err)
			defer grant.Release()
			close(peer.proceed)
			select {
			case <-grant.Context().Done():
			case <-time.After(2 * time.Second):
				t.Fatal("changed physical binding did not cancel the grant")
			}
		})
	}
}

func adversarialClient(t *testing.T, mode string) (*Client, *maliciousPermitPeer, Binding) {
	t.Helper()
	serverCert, clientCert, roots := streamTransportTLS(t)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	peer := &maliciousPermitPeer{mode: mode, requests: make(chan metadata.MD, 4), proceed: make(chan struct{})}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverCert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	userv1.RegisterUserSourceCustodyServiceServer(server, peer)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client, err := NewClient(userv1.NewUserSourceCustodyServiceClient(conn), issuer, 250*time.Millisecond)
	require.NoError(t, err)
	return client, peer, custodyBinding()
}

func TestUserCustodyClientRejectsInvalidInitialFrames(t *testing.T) {
	for _, mode := range []string{"no-first", "wrong-first-kind", "wrong-first-seq", "wrong-first-binding", "wrong-first-protocol", "unknown-first", "unknown-outer-first", "missing-first-binding"} {
		t.Run(mode, func(t *testing.T) {
			client, _, binding := adversarialClient(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			started := time.Now()
			grant, err := client.Acquire(ctx, binding)
			if grant != nil {
				grant.Release()
			}
			require.Error(t, err)
			require.NoError(t, ctx.Err(), "first-frame timeout or invalid frame must be detected before the parent deadline")
			if mode == "no-first" {
				require.Less(t, time.Since(started), 1500*time.Millisecond, "250ms first-frame timeout must be bounded")
			}
		})
	}
}

func TestUserCustodyClientParentCancelBeforeFirstFrame(t *testing.T) {
	client, peer, binding := adversarialClient(t, "no-first")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		grant, err := client.Acquire(ctx, binding)
		if grant != nil {
			grant.Release()
		}
		result <- err
	}()
	select {
	case <-peer.requests:
	case <-time.After(2 * time.Second):
		t.Fatal("peer did not receive request")
	}
	cancel()
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("parent cancellation did not stop acquisition")
	}
}

func TestUserCustodyClientCancelsOnInvalidOrMissingHeartbeat(t *testing.T) {
	for _, mode := range []string{"no-heartbeat", "eof", "duplicate", "gap", "repeat-granted", "unknown-kind", "changed-binding", "unknown-heartbeat", "unknown-heartbeat-binding"} {
		t.Run(mode, func(t *testing.T) {
			client, peer, binding := adversarialClient(t, mode)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			grant, err := client.Acquire(ctx, binding)
			require.NoError(t, err, "valid first grant must be accepted")
			require.NotNil(t, grant)
			defer grant.Release()
			close(peer.proceed)
			started := time.Now()
			select {
			case <-grant.Context().Done():
			case <-time.After(2 * time.Second):
				t.Fatal("invalid or absent frame did not cancel grant")
			}
			if mode == "no-heartbeat" {
				require.Less(t, time.Since(started), 1500*time.Millisecond, "250ms heartbeat timeout must be bounded")
			}
		})
	}
}

func TestUserCustodyClientUsesFreshCredentialForEachAttempt(t *testing.T) {
	client, peer, binding := adversarialClient(t, "no-first")
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		grant, err := client.Acquire(ctx, binding)
		cancel()
		if grant != nil {
			grant.Release()
		}
		require.Error(t, err)
	}
	first, second := <-peer.requests, <-peer.requests
	require.Len(t, first.Get("authorization"), 1)
	require.Len(t, second.Get("authorization"), 1)
	require.NotEqual(t, first.Get("authorization")[0], second.Get("authorization")[0])
	require.Len(t, first.Get("x-request-id"), 1)
	require.Len(t, second.Get("x-request-id"), 1)
	require.NotEqual(t, first.Get("x-request-id")[0], second.Get("x-request-id")[0])
}

func TestUserCustodyClientHonorsParentDeadlineDespiteHeartbeats(t *testing.T) {
	client, peer, binding := adversarialClient(t, "valid-heartbeats")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	grant, err := client.Acquire(ctx, binding)
	require.NoError(t, err)
	defer grant.Release()
	close(peer.proceed)
	select {
	case <-grant.Context().Done():
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeats extended the parent deadline")
	}
}
