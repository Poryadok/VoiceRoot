// Package terminalprincipal binds native reads and exact session cancellation
// to an authenticated actor. It cannot authorize enqueue/provider progression.
package terminalprincipal

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type Verifier struct {
	KeyResolver         principal.KeyResolver
	ReplayGuard         principal.ReplayGuard
	SessionEpochChecker principal.SessionEpochChecker
	Clock               func() time.Time
}
type Subject struct {
	AccountID, ProfileID uuid.UUID
	SessionEpoch         int64
}

type subjectKey struct{}
type verifiedSubject struct {
	subject             Subject
	method, requestHash string
}

func (v Verifier) Verify(ctx context.Context, request proto.Message, method string) (context.Context, error) {
	deny := func() (context.Context, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid native terminal principal")
	}
	if ctx == nil || ctx.Err() != nil || !validRequest(request, method) || v.KeyResolver == nil || v.ReplayGuard == nil || v.SessionEpochChecker == nil {
		return deny()
	}
	transport, err := principal.IncomingMetadata(ctx)
	if err != nil || !canonicalID(transport.RequestID) {
		return deny()
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return deny()
	}
	actor, err := principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: v.KeyResolver, ReplayGuard: v.ReplayGuard, SessionEpochChecker: v.SessionEpochChecker, Clock: v.Clock})
	if err != nil || !canonicalID(actor.AccountID) || !canonicalID(actor.ProfileID) || actor.SessionEpoch <= 0 || (actor.AccountType != "regular" && actor.AccountType != "guest") {
		return deny()
	}
	s := Subject{AccountID: uuid.MustParse(actor.AccountID), ProfileID: uuid.MustParse(actor.ProfileID), SessionEpoch: actor.SessionEpoch}
	return context.WithValue(ctx, subjectKey{}, verifiedSubject{subject: s, method: method, requestHash: hash}), nil
}
func SubjectFromContext(ctx context.Context, request proto.Message, method string) (Subject, bool) {
	if ctx == nil || !validRequest(request, method) {
		return Subject{}, false
	}
	v, ok := ctx.Value(subjectKey{}).(verifiedSubject)
	if !ok || v.method != method {
		return Subject{}, false
	}
	hash, err := principal.RequestHash(request)
	if err != nil || hash != v.requestHash {
		return Subject{}, false
	}
	return v.subject, true
}
func canonicalID(text string) bool {
	id, err := uuid.Parse(text)
	return err == nil && id != uuid.Nil && id.String() == text
}
func validRequest(request proto.Message, method string) bool {
	if request == nil || !request.ProtoReflect().IsValid() || len(request.ProtoReflect().GetUnknown()) != 0 {
		return false
	}
	switch method {
	case matchmakingv1.MatchmakingService_GetMatch_FullMethodName:
		r, ok := request.(*matchmakingv1.GetMatchRequest)
		return ok && canonicalID(r.MatchId)
	case matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName:
		r, ok := request.(*matchmakingv1.GetSearchStatusRequest)
		return ok && canonicalID(r.SessionId)
	case matchmakingv1.MatchmakingService_CancelSearch_FullMethodName:
		r, ok := request.(*matchmakingv1.CancelSearchRequest)
		return ok && canonicalID(r.SessionId)
	case matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName:
		r, ok := request.(*matchmakingv1.GetMatchHistoryRequest)
		return ok && (r.ProfileId == "" || canonicalID(r.ProfileId))
	default:
		return false
	}
}
