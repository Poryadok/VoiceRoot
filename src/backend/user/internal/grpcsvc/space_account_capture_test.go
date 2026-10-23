package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
)

type captureAuthorityFunc func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)

func (f captureAuthorityFunc) CaptureSpaceMemberAccountBindings(ctx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	return f(ctx, r)
}

func captureTestRequest() *userv1.CaptureSpaceMemberAccountBindingsRequest {
	r := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: uuid.NewString(), SpaceId: uuid.NewString(), Purpose: 2, Cohort: &userv1.SpaceAccountCaptureCohort{}}
	r.CohortDigest = spaceaccountcapture.CohortDigest(r.Cohort)
	return r
}

func captureTestPrincipal(t *testing.T, r *userv1.CaptureSpaceMemberAccountBindingsRequest) principal.Principal {
	t.Helper()
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "user",
		RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, JWTID: uuid.NewString(), ExpiresAt: time.Now().Add(time.Minute)}
}

func captureTestIdentityCut() *userv1.SpaceAccountIdentityCut {
	return &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: "11111111-1111-4111-8111-111111111111", ThroughRevision: 5, KeysetRevision: 1, Digest: make([]byte, 32)}
}

func TestSpaceCaptureRejectsOrdinaryAndWrongBoundPrincipalBeforeAuthority(t *testing.T) {
	r := captureTestRequest()
	called := 0
	s := &SpaceAccountCaptureGRPC{Authority: captureAuthorityFunc(func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		called++
		return nil, errors.New("must not call")
	})}
	_, err := s.CaptureSpaceMemberAccountBindings(metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "space")), r)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for name, mutate := range map[string]func(*principal.Principal){
		"auth caller":    func(p *principal.Principal) { p.Issuer, p.Subject = "auth", "service:auth" },
		"role caller":    func(p *principal.Principal) { p.Issuer, p.Subject = "role", "service:role" },
		"gateway caller": func(p *principal.Principal) { p.Issuer, p.Subject = "gateway", "service:gateway" },
		"user authority": func(p *principal.Principal) { p.AccountID = uuid.NewString() },
		"wrong method": func(p *principal.Principal) {
			p.RPC = userv1.UserService_GetSpaceAdmissionProfileEligibility_FullMethodName
		},
		"wrong audience":     func(p *principal.Principal) { p.Audience = "space" },
		"wrong body":         func(p *principal.Principal) { p.RequestHash = "sha256:" + string(make([]byte, 64)) },
		"missing request id": func(p *principal.Principal) { p.RequestID = "" },
		"expired":            func(p *principal.Principal) { p.ExpiresAt = time.Now().Add(-time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			p := captureTestPrincipal(t, r)
			mutate(&p)
			_, err := s.CaptureSpaceMemberAccountBindings(principal.WithVerified(context.Background(), p), r)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
	require.Zero(t, called)
}

func TestSpaceCaptureBoundsAuthorityDeadlineAndValidatesCompleteResponse(t *testing.T) {
	r := captureTestRequest()
	p := captureTestPrincipal(t, r)
	s := &SpaceAccountCaptureGRPC{Authority: captureAuthorityFunc(func(ctx context.Context, got *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Greater(t, time.Until(deadline), time.Duration(0))
		require.LessOrEqual(t, time.Until(deadline), 2*time.Second)
		return &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: got.Protocol, OperationId: got.OperationId, Purpose: got.Purpose, SpaceId: got.SpaceId, CohortDigest: got.CohortDigest, RequestDigest: spaceaccountcapture.RequestDigest(got), OwnerSourceRevision: 17, UserSourceGeneration: "11111111-1111-4111-8111-111111111111", IdentityCut: captureTestIdentityCut()}, nil
	})}
	out, err := s.CaptureSpaceMemberAccountBindings(principal.WithVerified(context.Background(), p), r)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(r, out))
	s.Authority = captureAuthorityFunc(func(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		return &userv1.CaptureSpaceMemberAccountBindingsResponse{}, nil
	})
	_, err = s.CaptureSpaceMemberAccountBindings(principal.WithVerified(context.Background(), p), r)
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSpaceCaptureListenerRegistersOnlyCapture(t *testing.T) {
	s := grpc.NewServer()
	defer s.Stop()
	RegisterSpaceAccountCaptureServer(s, nil)
	info := s.GetServiceInfo()["voice.user.v1.UserService"]
	require.Len(t, info.Methods, 1)
	require.Equal(t, "CaptureSpaceMemberAccountBindings", info.Methods[0].Name)
	_, err := (&UserGRPC{}).CaptureSpaceMemberAccountBindings(context.Background(), captureTestRequest())
	require.Equal(t, codes.Unimplemented, status.Code(err))
}

func TestSpaceCaptureAuthorityCannotRewriteAuthenticatedRequest(t *testing.T) {
	r := captureTestRequest()
	original := r.SpaceId
	s := &SpaceAccountCaptureGRPC{Authority: captureAuthorityFunc(func(_ context.Context, got *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		got.SpaceId = uuid.NewString()
		return &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: got.Protocol, OperationId: got.OperationId, Purpose: got.Purpose, SpaceId: got.SpaceId, CohortDigest: got.CohortDigest, RequestDigest: spaceaccountcapture.RequestDigest(got), OwnerSourceRevision: 17, UserSourceGeneration: original, IdentityCut: captureTestIdentityCut()}, nil
	})}
	_, err := s.CaptureSpaceMemberAccountBindings(principal.WithVerified(context.Background(), captureTestPrincipal(t, r)), r)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Equal(t, original, r.SpaceId)
}
