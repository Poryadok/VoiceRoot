package partysnapshotrpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/partysnapshot"
	"voice/backend/voice/internal/principalgrpc"
)

type snapshotSpy struct {
	calls   int
	subject partysnapshot.Subject
	err     error
}

func (s *snapshotSpy) Snapshot(_ context.Context, subject partysnapshot.Subject) (partysnapshot.Snapshot, error) {
	s.calls++
	s.subject = subject
	return partysnapshot.Snapshot{ProtocolVersion: 1, Kind: "SOLO", MemberProfileIDs: []uuid.UUID{subject.ProfileID}}, s.err
}

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	return f(ctx, token, method, id, hash)
}

func validRequest() *callsv1.GetMatchmakingPartySnapshotRequest {
	return &callsv1.GetMatchmakingPartySnapshotRequest{SearchOperationId: uuid.NewString(), InitiatorAccountId: uuid.NewString(), InitiatorProfileId: uuid.NewString(), InitiatorSessionEpoch: 7}
}
func signedContext(t *testing.T, req *callsv1.GetMatchmakingPartySnapshotRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice", RPC: matchsquadprincipal.SnapshotMethod, RequestID: req.SearchOperationId, RequestHash: hash})
}

func TestProtectedSnapshotCopiesVerifiedSubjectAndNeverUsesClientRoster(t *testing.T) {
	req, spy := validRequest(), &snapshotSpy{}
	response, err := (&Server{Engine: spy}).GetMatchmakingPartySnapshot(signedContext(t, req), req)
	require.NoError(t, err)
	require.Equal(t, 1, spy.calls)
	require.Equal(t, req.SearchOperationId, spy.subject.SearchOperationID.String())
	require.Equal(t, req.InitiatorAccountId, spy.subject.AccountID.String())
	require.Equal(t, req.InitiatorProfileId, spy.subject.ProfileID.String())
	require.Equal(t, int64(7), spy.subject.SessionEpoch)
	require.Equal(t, []string{req.InitiatorProfileId}, response.MemberProfileIds)
	require.Empty(t, response.RoomId)
	require.Zero(t, response.RosterVersion)
}

func TestProtectedSnapshotRejectsSpoofAndMalformedInputBeforeEngine(t *testing.T) {
	for _, kind := range []string{"missing principal", "wrong hash", "delegated user", "foreign service", "wrong method", "unknown field", "account invalid", "profile noncanonical", "zero epoch"} {
		t.Run(kind, func(t *testing.T) {
			req, spy := validRequest(), &snapshotSpy{}
			switch kind {
			case "unknown field":
				req.ProtoReflect().SetUnknown([]byte{0x28, 0x01})
			case "account invalid":
				req.InitiatorAccountId = "not-an-id"
			case "profile noncanonical":
				req.InitiatorProfileId = "20000000000040008000000000000001"
			case "zero epoch":
				req.InitiatorSessionEpoch = 0
			}
			ctx := signedContext(t, req)
			verified, _ := principal.FromContext(ctx)
			switch kind {
			case "missing principal":
				ctx = context.Background()
			case "wrong hash":
				verified.RequestHash = "wrong"
				ctx = principal.WithVerified(ctx, verified)
			case "delegated user":
				verified.Kind = "user"
				verified.AccountID = req.InitiatorAccountId
				ctx = principal.WithVerified(ctx, verified)
			case "foreign service":
				verified.Subject = "service:space"
				ctx = principal.WithVerified(ctx, verified)
			case "wrong method":
				verified.RPC = matchsquadprincipal.CreateMethod
				ctx = principal.WithVerified(ctx, verified)
			}
			_, err := (&Server{Engine: spy}).GetMatchmakingPartySnapshot(ctx, req)
			require.Error(t, err)
			require.Zero(t, spy.calls, "denied input must not reach native/Auth authority")
		})
	}
}

func TestSnapshotUnavailableOnOrdinaryListener(t *testing.T) {
	called := false
	_, err := principalgrpc.OrdinaryUnaryInterceptor()(context.Background(), validRequest(), &grpc.UnaryServerInfo{FullMethod: matchsquadprincipal.SnapshotMethod}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}

func TestSnapshotProtectedInterceptorBindsExactOperationMethodAndHash(t *testing.T) {
	req, spy := validRequest(), &snapshotSpy{}
	for _, mutation := range []string{"none", "metadata operation", "postverify mutation"} {
		t.Run(mutation, func(t *testing.T) {
			request := proto.Clone(req).(*callsv1.GetMatchmakingPartySnapshotRequest)
			requestID := request.SearchOperationId
			if mutation == "metadata operation" {
				requestID = uuid.NewString()
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer service-token", "x-request-id", requestID))
			before := spy.calls
			verify := verifierFunc(func(_ context.Context, token, method, id, hash string) (principal.Principal, error) {
				require.Equal(t, "service-token", token)
				require.Equal(t, matchsquadprincipal.SnapshotMethod, method)
				if mutation == "postverify mutation" {
					request.InitiatorProfileId = uuid.NewString()
				}
				return principal.Principal{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice", RPC: method, RequestID: id, RequestHash: hash}, nil
			})
			_, err := matchsquadprincipal.StrictUnaryInterceptor(verify)(ctx, request, &grpc.UnaryServerInfo{FullMethod: matchsquadprincipal.SnapshotMethod}, func(ctx context.Context, _ any) (any, error) {
				return (&Server{Engine: spy}).GetMatchmakingPartySnapshot(ctx, request)
			})
			if mutation == "none" {
				require.NoError(t, err)
				require.Equal(t, before+1, spy.calls)
			} else {
				require.Error(t, err)
				require.Equal(t, before, spy.calls)
			}
		})
	}
}

func TestSnapshotAuthorityErrorsHaveStableStatus(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{{partysnapshot.ErrUnavailable, codes.Unavailable}, {partysnapshot.ErrStaleSubject, codes.Unauthenticated}, {partysnapshot.ErrIneligible, codes.FailedPrecondition}} {
		req := validRequest()
		_, err := (&Server{Engine: &snapshotSpy{err: tc.err}}).GetMatchmakingPartySnapshot(signedContext(t, req), req)
		require.Equal(t, tc.code, status.Code(err))
	}
}
