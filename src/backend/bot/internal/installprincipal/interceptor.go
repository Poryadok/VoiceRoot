// Package installprincipal owns the private initiating-administrator boundary.
// Generic principal context values and identity headers cannot mint this value.
package installprincipal

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

var ErrAdministrator = errors.New("verified initiating administrator required")

type Administrator struct {
	identity                principal.UserIdentity
	method, requestID, hash string
	expires                 time.Time
}

func (a Administrator) AccountID() uuid.UUID { return a.identity.AccountID }
func (a Administrator) ProfileID() uuid.UUID { return a.identity.ProfileID }
func (a Administrator) RequestID() string    { return a.requestID }
func (a Administrator) RequestHash() string  { return a.hash }
func (a Administrator) ExpiresAt() time.Time { return a.expires }
func (a Administrator) SessionEpoch() int64  { return a.identity.SessionEpoch }
func (a Administrator) AccountType() string  { return a.identity.AccountType }

type administratorKey struct{}

type Verifier struct {
	Keys   principal.KeyResolver
	Replay principal.ReplayGuard
	Epoch  principal.SessionEpochChecker
	Clock  func() time.Time
}

func Protected(method string) bool {
	return method == botv1.BotService_InstallBotInSpace_FullMethodName || method == botv1.BotService_UninstallBotFromSpace_FullMethodName
}

func (v *Verifier) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if !Protected(info.FullMethod) {
			return next(ctx, req)
		}
		if v == nil || v.Keys == nil || v.Replay == nil || v.Epoch == nil {
			return nil, status.Error(codes.Unavailable, "administrator verifier unavailable")
		}
		message, ok := req.(proto.Message)
		if !ok || message == nil || !message.ProtoReflect().IsValid() || len(message.ProtoReflect().GetUnknown()) != 0 {
			return nil, status.Error(codes.InvalidArgument, "invalid installation request")
		}
		transport, err := principal.IncomingMetadata(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid administrator credential")
		}
		hash, err := principal.RequestHash(message)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid installation request")
		}
		verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: info.FullMethod, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.Keys, ReplayGuard: v.Replay, SessionEpochChecker: v.Epoch, Clock: v.Clock})
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid administrator credential")
		}
		identity, err := principal.RequireDelegatedUser(principal.WithVerified(ctx, verified), "bot", info.FullMethod, message)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid administrator identity")
		}
		a := Administrator{identity: identity, method: info.FullMethod, requestID: transport.RequestID, hash: hash, expires: verified.ExpiresAt}
		return next(context.WithValue(ctx, administratorKey{}, a), req)
	}
}

func Require(ctx context.Context, method string, message proto.Message) (Administrator, error) {
	a, ok := ctx.Value(administratorKey{}).(Administrator)
	if !ok || !Protected(method) || a.method != method || a.identity.AccountID == uuid.Nil || a.identity.ProfileID == uuid.Nil || message == nil || !message.ProtoReflect().IsValid() {
		return Administrator{}, ErrAdministrator
	}
	hash, err := principal.RequestHash(message)
	if err != nil || hash != a.hash {
		return Administrator{}, ErrAdministrator
	}
	return a, nil
}
