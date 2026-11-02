package sourcecustody

import (
	"context"
	"io"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

type StreamConfig struct {
	KeyResolver      principal.KeyResolver
	ReplayGuard      principal.ReplayGuard
	Clock            func() time.Time
	Heartbeat        time.Duration
	RequireQualified bool
	RequireAttested  bool
	// Test-only synchronization; never an admission authority.
	testTicks <-chan time.Time
}

type StreamServer struct {
	userv1.UnimplementedUserSourceCustodyServiceServer
	controller *Controller
	cfg        StreamConfig
}

func NewStreamServer(controller *Controller, cfg StreamConfig) (*StreamServer, error) {
	if controller == nil || cfg.KeyResolver == nil || cfg.ReplayGuard == nil || cfg.Heartbeat <= 0 || cfg.Heartbeat > 5*time.Second {
		return nil, ErrUnavailable
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &StreamServer{controller: controller, cfg: cfg}, nil
}

// AcquireUserSourcePermit verifies the initial message and principal before
// taking a control permit. Stream loss is not a User-transaction fence.
func (s *StreamServer) AcquireUserSourcePermit(req *userv1.AcquireUserSourcePermitRequest, stream grpc.ServerStreamingServer[userv1.AcquireUserSourcePermitResponse]) (result error) {
	if s == nil || stream == nil {
		return status.Error(codes.Unavailable, "source custody unavailable")
	}
	ctx := stream.Context()
	endpoint, ok := peer.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "source custody authentication required")
	}
	tlsInfo, ok := endpoint.AuthInfo.(credentials.TLSInfo)
	if !ok || !tlsInfo.State.HandshakeComplete || len(tlsInfo.State.VerifiedChains) == 0 {
		return status.Error(codes.Unauthenticated, "source custody authentication required")
	}
	binding, err := streamBinding(req)
	if err != nil || ((s.cfg.RequireQualified || s.cfg.RequireAttested) && req.Protocol != 2) {
		return status.Error(codes.InvalidArgument, "invalid source custody binding")
	}
	meta, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return status.Error(codes.Unauthenticated, "source custody authentication required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid source custody request")
	}
	verified, err := principal.VerifyService(ctx, meta.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "user", ExpectedAudience: "user-source-custody",
		ExpectedRPC:       userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName,
		ExpectedRequestID: meta.RequestID, ExpectedRequestHash: hash,
		Clock: s.cfg.Clock, KeyResolver: s.cfg.KeyResolver, ReplayGuard: s.cfg.ReplayGuard,
	})
	if err != nil {
		return status.Error(codes.Unauthenticated, "source custody authentication required")
	}
	deadline, ok := ctx.Deadline()
	if !ok || deadline.After(verified.ExpiresAt) || !s.cfg.Clock().Before(verified.ExpiresAt) {
		return status.Error(codes.Unauthenticated, "source custody credential lifetime invalid")
	}
	// The unary request side must close; an additional message cannot select a
	// second binding or keep the handler waiting past the verified deadline.
	var extra userv1.AcquireUserSourcePermitRequest
	if err = stream.RecvMsg(&extra); err != io.EOF {
		return status.Error(codes.InvalidArgument, "source custody request stream invalid")
	}
	var permit *Permit
	var physical PhysicalSource
	if req.Protocol == 2 {
		if s.cfg.RequireAttested {
			permit, physical, err = s.controller.AcquireAttested(ctx, binding)
		} else {
			permit, physical, err = s.controller.AcquireQualified(ctx, binding)
		}
	} else {
		permit, err = s.controller.Acquire(ctx, binding)
	}
	if err != nil {
		return status.Error(codes.Unavailable, "source custody unavailable")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := permit.Release(cleanup); err != nil {
			result = status.Error(codes.Unavailable, "source custody unavailable")
		}
	}()
	if err = checkPermit(ctx, permit); err != nil {
		return status.Error(codes.Unavailable, "source custody unavailable")
	}
	if ctx.Err() != nil || !s.cfg.Clock().Before(verified.ExpiresAt) {
		return status.Error(codes.Unauthenticated, "source custody credential expired")
	}
	frame := &userv1.AcquireUserSourcePermitResponse{Protocol: req.Protocol, Binding: proto.Clone(req.Binding).(*userv1.UserSourcePermitBinding), Sequence: 1, Kind: userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED}
	if req.Protocol == 2 {
		frame.Physical = &userv1.UserSourcePhysicalBinding{SystemIdentifier: physical.SystemIdentifier, Database: physical.Database, DatabaseOid: physical.DatabaseOID}
	}
	if err = stream.Send(frame); err != nil {
		return err
	}
	ticks := s.cfg.testTicks
	if ticks == nil {
		ticker := time.NewTicker(s.cfg.Heartbeat)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticks:
			if !s.cfg.Clock().Before(verified.ExpiresAt) {
				return status.Error(codes.Unauthenticated, "source custody credential expired")
			}
			if err = checkPermit(ctx, permit); err != nil {
				return status.Error(codes.Unavailable, "source custody unavailable")
			}
			if ctx.Err() != nil || !s.cfg.Clock().Before(verified.ExpiresAt) {
				return status.Error(codes.Unauthenticated, "source custody credential expired")
			}
			if frame.Sequence == math.MaxUint64 {
				return status.Error(codes.Unavailable, "source custody unavailable")
			}
			frame.Sequence++
			frame.Kind = userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT
			if err = stream.Send(frame); err != nil {
				return err
			}
		}
	}
}

// A caller disconnect must not cancel a database operation on the shared leader
// connection. Keep the permit until that operation completes or times out; an
// uncertain control operation still poisons the controller in Permit.Check.
func checkPermit(ctx context.Context, permit *Permit) error {
	if ctx.Err() != nil {
		return ErrUnavailable
	}
	check, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	return permit.Check(check)
}

func streamBinding(req *userv1.AcquireUserSourcePermitRequest) (Binding, error) {
	if req == nil || (req.Protocol != 1 && req.Protocol != 2) || req.Binding == nil || len(req.ProtoReflect().GetUnknown()) != 0 || len(req.Binding.ProtoReflect().GetUnknown()) != 0 {
		return Binding{}, ErrUnavailable
	}
	b := req.Binding
	if b.Epoch == 0 || b.Epoch > math.MaxInt64 || len(b.KeysetDigest) != 32 {
		return Binding{}, ErrUnavailable
	}
	h, err := uuid.Parse(b.HistoryGeneration)
	if err != nil || h.String() != b.HistoryGeneration {
		return Binding{}, ErrUnavailable
	}
	u, err := uuid.Parse(b.UserSourceGeneration)
	if err != nil || u.String() != b.UserSourceGeneration {
		return Binding{}, ErrUnavailable
	}
	var digest [32]byte
	copy(digest[:], b.KeysetDigest)
	result := Binding{Epoch: int64(b.Epoch), SourceID: b.SourceId, HistoryGeneration: h, UserSourceGeneration: u, KeysetDigest: digest}
	if !result.valid() {
		return Binding{}, ErrUnavailable
	}
	return result, nil
}
