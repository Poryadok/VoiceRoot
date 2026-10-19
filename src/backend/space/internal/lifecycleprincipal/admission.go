package lifecycleprincipal

import (
	"context"
	"errors"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/authctx"
)

func (v Verifier) verifyAdmissionCall(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if v.Keys == nil || v.AdmissionReplay == nil || v.AdmissionEpoch == nil {
		return nil, status.Error(codes.PermissionDenied, "admission authentication is not configured")
	}
	deny := status.Error(codes.Unauthenticated, "invalid admission principal")
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !canonicalID(transport.RequestID) {
		return nil, deny
	}
	message, ok := request.(proto.Message)
	if !ok || !validAdmissionMessage(message, 0) {
		return nil, status.Error(codes.InvalidArgument, "invalid admission request")
	}
	hash, err := principal.RequestHash(message)
	if err != nil {
		return nil, deny
	}
	clock := v.Clock
	if clock == nil {
		clock = time.Now
	}
	var started time.Time
	epoch := func(ownerCtx context.Context, account string, value int64) error {
		started = clock()
		ownerCtx, cancel := context.WithTimeout(ownerCtx, 2*time.Second)
		defer cancel()
		if err := v.AdmissionEpoch(ownerCtx, account, value); err != nil {
			return err
		}
		if ownerCtx.Err() != nil || clock().Before(started) || !clock().Before(started.Add(2*time.Second)) {
			return ErrDependencyUnavailable
		}
		return nil
	}
	actor, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.AdmissionReplay, SessionEpochChecker: epoch, Clock: clock})
	if errors.Is(err, ErrDependencyUnavailable) || ctx.Err() != nil {
		return nil, status.Error(codes.Unavailable, "admission authentication unavailable")
	}
	if err != nil || !canonicalID(actor.AccountID) || !canonicalID(actor.ProfileID) || (actor.AccountType != "regular" && actor.AccountType != "guest") {
		return nil, deny
	}
	proof := &admissionCallProof{actor: actor, method: info.FullMethod, message: message.ProtoReflect().Descriptor().FullName(), clock: clock, ownerStarted: started}
	ctx = context.WithValue(principal.WithVerified(ctx, actor), admissionCallKey{}, proof)
	if _, err = (&VerifiedAdmissionCall{proof: *proof}).ActorFor(message); err != nil {
		return nil, err
	}
	// Compatibility projection remains server-owned and preserves explicit type.
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderUserID, actor.AccountID, authctx.HeaderProfileID, actor.ProfileID, authctx.HeaderSessionEpoch, strconv.FormatInt(actor.SessionEpoch, 10), "x-voice-account-type", actor.AccountType))
	return handler(ctx, request)
}
