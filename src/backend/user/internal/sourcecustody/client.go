package sourcecustody

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

// Client validates a held stream. Its supplied transport is not source custody
// or proof of a production peer's identity. Recv must obey gRPC context
// cancellation; the first-frame timeout starts after opening the RPC.
type Client struct {
	remote  userv1.UserSourceCustodyServiceClient
	issuer  *principal.Issuer
	timeout time.Duration
	// Test-only synchronization, never an authority or production gate.
	afterFrame func()
}

type Grant struct {
	ctx      context.Context
	cancel   context.CancelFunc
	done     <-chan struct{}
	physical PhysicalSource
}

func NewClient(remote userv1.UserSourceCustodyServiceClient, issuer *principal.Issuer, heartbeatTimeout time.Duration) (*Client, error) {
	if remote == nil || issuer == nil || heartbeatTimeout <= 0 || heartbeatTimeout > 10*time.Second {
		return nil, ErrUnavailable
	}
	return &Client{remote: remote, issuer: issuer, timeout: heartbeatTimeout}, nil
}

type frameResult struct {
	frame *userv1.AcquireUserSourcePermitResponse
	err   error
}

func (c *Client) Acquire(parent context.Context, binding Binding) (*Grant, error) {
	return c.acquire(parent, binding, 1)
}

// AcquireQualified requests the controller-held physical record. The caller
// must compare it against the source transaction and enforce exclusive custody.
func (c *Client) AcquireQualified(parent context.Context, binding Binding) (*Grant, error) {
	return c.acquire(parent, binding, 2)
}

func (c *Client) acquire(parent context.Context, binding Binding, protocol uint32) (*Grant, error) {
	if c == nil || parent == nil || parent.Err() != nil || !binding.valid() {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	req := &userv1.AcquireUserSourcePermitRequest{Protocol: protocol, Binding: &userv1.UserSourcePermitBinding{
		Epoch: uint64(binding.Epoch), SourceId: binding.SourceID, HistoryGeneration: binding.HistoryGeneration.String(),
		UserSourceGeneration: binding.UserSourceGeneration.String(), KeysetDigest: binding.KeysetDigest[:],
	}}
	hash, err := principal.RequestHash(req)
	if err != nil {
		cancel()
		return nil, ErrUnavailable
	}
	id, err := uuid.NewRandom()
	if err != nil {
		cancel()
		return nil, ErrUnavailable
	}
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "user-source-custody", RPC: userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, RequestID: id.String(), RequestHash: hash})
	if err != nil {
		cancel()
		return nil, ErrUnavailable
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id.String()))
	stream, err := c.remote.AcquireUserSourcePermit(ctx, req)
	if err != nil || stream == nil {
		cancel()
		return nil, ErrUnavailable
	}
	frames := make(chan frameResult)
	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		for {
			frame, recvErr := stream.Recv()
			select {
			case frames <- frameResult{frame: frame, err: recvErr}:
			case <-ctx.Done():
				return
			}
			if recvErr != nil {
				return
			}
		}
	}()
	deadline := time.Now().Add(c.timeout)
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	var physical PhysicalSource
	select {
	case result := <-frames:
		if c.afterFrame != nil {
			c.afterFrame()
		}
		if result.err != nil || !validProtocolFrame(result.frame, req.Binding, protocol, nil, 1, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED) || ctx.Err() != nil || !time.Now().Before(deadline) {
			cancel()
			<-recvDone
			return nil, ErrUnavailable
		}
		if protocol == 2 {
			physical = PhysicalSource{SystemIdentifier: result.frame.Physical.SystemIdentifier, Database: result.frame.Physical.Database, DatabaseOID: result.frame.Physical.DatabaseOid}
		}
	case <-timer.C:
		cancel()
		<-recvDone
		return nil, ErrUnavailable
	case <-ctx.Done():
		cancel()
		<-recvDone
		return nil, ErrUnavailable
	}
	done := make(chan struct{})
	grant := &Grant{ctx: ctx, cancel: cancel, done: done, physical: physical}
	heartbeatDeadline := time.Now().Add(c.timeout)
	go func() {
		defer close(done)
		defer func() { cancel(); <-recvDone }()
		deadline := heartbeatDeadline
		t := time.NewTimer(time.Until(deadline))
		defer t.Stop()
		sequence := uint64(1)
		for {
			select {
			case result := <-frames:
				if c.afterFrame != nil {
					c.afterFrame()
				}
				if result.err != nil || sequence == math.MaxUint64 || !validProtocolFrame(result.frame, req.Binding, protocol, &physical, sequence+1, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT) {
					return
				}
				now := time.Now()
				if ctx.Err() != nil || !now.Before(deadline) {
					return
				}
				sequence++
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
				deadline = now.Add(c.timeout)
				t.Reset(time.Until(deadline))
			case <-t.C:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	if ctx.Err() != nil || !time.Now().Before(deadline) {
		grant.Release()
		return nil, ErrUnavailable
	}
	return grant, nil
}

func validProtocolFrame(f *userv1.AcquireUserSourcePermitResponse, binding *userv1.UserSourcePermitBinding, protocol uint32, expected *PhysicalSource, sequence uint64, kind userv1.UserSourcePermitFrameKind) bool {
	if f == nil || f.Protocol != protocol || f.Sequence != sequence || f.Kind != kind || len(f.ProtoReflect().GetUnknown()) != 0 || f.Binding == nil || len(f.Binding.ProtoReflect().GetUnknown()) != 0 || !proto.Equal(f.Binding, binding) {
		return false
	}
	if protocol == 1 {
		return f.Physical == nil
	}
	if protocol != 2 || f.Physical == nil || len(f.Physical.ProtoReflect().GetUnknown()) != 0 || !systemID.MatchString(f.Physical.SystemIdentifier) || !databaseName.MatchString(f.Physical.Database) || f.Physical.DatabaseOid == 0 {
		return false
	}
	actual := PhysicalSource{SystemIdentifier: f.Physical.SystemIdentifier, Database: f.Physical.Database, DatabaseOID: f.Physical.DatabaseOid}
	return expected == nil || actual == *expected
}

func (g *Grant) Physical() PhysicalSource {
	if g == nil {
		return PhysicalSource{}
	}
	return g.physical
}

func (g *Grant) Context() context.Context {
	if g == nil || g.ctx == nil {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return g.ctx
}
func (g *Grant) Release() {
	if g != nil && g.cancel != nil && g.done != nil {
		g.cancel()
		<-g.done
	}
}
