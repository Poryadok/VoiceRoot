package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/lifecycleprincipal"
)

func TestAdmissionReviewReadsRequireExactCurrentReviewerAndNativeReadiness(t *testing.T) {
	ctx := context.Background()
	st, space := admissionReadStore(t)
	account, profile := uuid.New(), uuid.New()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	spaceIssuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, AdmissionEpoch: func(context.Context, string, int64) error { return nil }, AdmissionReplay: func(context.Context, string, string, time.Time) error { return nil }}
	invoke := func(method string, request proto.Message, handler func(context.Context) (any, error)) (any, error) {
		hash, e := principal.RequestHash(request)
		require.NoError(t, e)
		id := uuid.NewString()
		token, e := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash, AccountID: account.String(), ProfileID: profile.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
		require.NoError(t, e)
		incoming := metadata.NewIncomingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		return verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: method}, func(c context.Context, _ any) (any, error) { return handler(c) })
	}
	ownerCalls, permissionCalls := 0, 0
	allowed := false
	subjects := &admissionauthority.Client{Issuer: spaceIssuer, Auth: admissionSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, r.Phase)
		require.Equal(t, authv1.SpaceAdmissionSubjectRole_SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER, r.SubjectRole)
		require.EqualValues(t, 7, r.GetInteractiveSessionEpoch())
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, e)
		sha := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: sha[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	reviewers := &admissionauthority.ReviewerClient{Issuer: spaceIssuer, Role: settingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		permissionCalls++
		require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
		require.Equal(t, space.String(), r.SpaceId)
		require.Equal(t, profile.String(), r.ProfileId)
		return &rolev1.CheckPermissionResponse{Allowed: allowed}, nil
	})}
	api := NewAdmissionServer(&SpaceGRPC{Store: st}, st, AdmissionDependencies{Subjects: subjects, Reviewers: reviewers})
	list := &spacev1.ListJoinRequestsRequest{SpaceId: space.String(), Limit: 2}
	detail := &spacev1.GetJoinRequestRequest{SpaceId: space.String(), RequestId: uuid.NewString()}
	_, err = api.ListJoinRequests(ctx, list)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = api.GetJoinRequest(ctx, detail)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, read := range []struct {
		method  string
		request proto.Message
		handler func(context.Context) (any, error)
	}{
		{spacev1.SpaceService_ListJoinRequests_FullMethodName, list, func(c context.Context) (any, error) { return api.ListJoinRequests(c, list) }},
		{spacev1.SpaceService_GetJoinRequest_FullMethodName, detail, func(c context.Context) (any, error) { return api.GetJoinRequest(c, detail) }},
	} {
		_, err = invoke(read.method, read.request, read.handler)
		require.Equal(t, codes.PermissionDenied, status.Code(err), "fixed42 deny must precede data access")
		allowed = true
		_, err = invoke(read.method, read.request, read.handler)
		require.Equal(t, codes.Unavailable, status.Code(err), "controlled owner allow does not replace actual native readiness")
		allowed = false
	}
	require.Equal(t, 4, ownerCalls)
	require.Equal(t, 4, permissionCalls, "each read obtains new observations")
	invalid := proto.Clone(list).(*spacev1.ListJoinRequestsRequest)
	invalid.Cursor = "foreign-or-malformed"
	_, err = invoke(spacev1.SpaceService_ListJoinRequests_FullMethodName, invalid, func(c context.Context) (any, error) { return api.ListJoinRequests(c, invalid) })
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	require.Equal(t, 4, ownerCalls)
	// A genuine list-method capability cannot be reused as a get-method call.
	_, err = invoke(spacev1.SpaceService_ListJoinRequests_FullMethodName, list, func(c context.Context) (any, error) { return api.GetJoinRequest(c, detail) })
	require.Error(t, err)
	require.Equal(t, 4, ownerCalls)
}
