// Package searchprincipal verifies the protected A3 StartSearch subject.
// It is not wired into the legacy runtime: deployment cutover also requires
// the authoritative Voice snapshot, TLS, rotation, replay and epoch dependencies.
package searchprincipal

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

// Verifier dependencies must be provided by the protected transport owner.
// A missing dependency denies every request rather than enabling legacy input.
type Verifier struct {
	KeyResolver         principal.KeyResolver
	ReplayGuard         principal.ReplayGuard
	SessionEpochChecker principal.SessionEpochChecker
	Clock               func() time.Time
}

// Subject is authenticated subject data for the Voice party snapshot request.
type Subject struct {
	AccountID    uuid.UUID
	ProfileID    uuid.UUID
	SessionEpoch int64
}

type verifiedSearch struct {
	subject     Subject
	requestHash string
}

type searchContextKey struct{}

// Verify binds the caller to the exact public StartSearch request. No forwarded
// identity metadata or preexisting context principal can authorize this call.
func (v Verifier) Verify(ctx context.Context, request *matchmakingv1.StartSearchRequest) (context.Context, error) {
	deny := func() (context.Context, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid search principal")
	}
	if request == nil || v.KeyResolver == nil || v.ReplayGuard == nil || v.SessionEpochChecker == nil {
		return deny()
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil {
		return deny()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return deny()
	}
	verified, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{
		ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking",
		ExpectedRPC:       matchmakingv1.MatchmakingService_StartSearch_FullMethodName,
		ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
		KeyResolver: v.KeyResolver, ReplayGuard: v.ReplayGuard,
		SessionEpochChecker: v.SessionEpochChecker, Clock: v.Clock,
	})
	if err != nil {
		return deny()
	}
	accountID, err := uuid.Parse(verified.AccountID)
	if err != nil || accountID == uuid.Nil {
		return deny()
	}
	profileID, err := uuid.Parse(verified.ProfileID)
	if err != nil || profileID == uuid.Nil {
		return deny()
	}
	return context.WithValue(ctx, searchContextKey{}, verifiedSearch{
		subject:     Subject{AccountID: accountID, ProfileID: profileID, SessionEpoch: verified.SessionEpoch},
		requestHash: hash,
	}), nil
}

// SubjectFromContext returns only a subject produced by Verify for these exact
// request bytes. It intentionally never consults legacy authctx or metadata.
func SubjectFromContext(ctx context.Context, request *matchmakingv1.StartSearchRequest) (Subject, bool) {
	if request == nil {
		return Subject{}, false
	}
	verified, ok := ctx.Value(searchContextKey{}).(verifiedSearch)
	if !ok {
		return Subject{}, false
	}
	hash, err := principal.RequestHash(request)
	if err != nil || hash != verified.requestHash {
		return Subject{}, false
	}
	return verified.subject, true
}
