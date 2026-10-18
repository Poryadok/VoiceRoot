package automodclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strings"
	"testing"
	"time"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
	"voice/backend/pkg/principal"
)

type authFixture struct {
	resolve func(context.Context, *automodv1.ResolveModerationSubjectRequest) (*automodv1.ResolveModerationSubjectResponse, error)
}

func (f authFixture) ResolveModerationSubject(ctx context.Context, r *automodv1.ResolveModerationSubjectRequest, _ ...grpc.CallOption) (*automodv1.ResolveModerationSubjectResponse, error) {
	return f.resolve(ctx, r)
}
func (f authFixture) ListTerminalModerationSubjects(context.Context, *automodv1.ListTerminalModerationSubjectsRequest, ...grpc.CallOption) (*automodv1.ListTerminalModerationSubjectsResponse, error) {
	return &automodv1.ListTerminalModerationSubjectsResponse{Complete: false}, nil
}
func (f authFixture) ReconcileModerationRestriction(context.Context, *automodv1.ReconcileModerationRestrictionRequest, ...grpc.CallOption) (*automodv1.ReconcileModerationRestrictionResponse, error) {
	return &automodv1.ReconcileModerationRestrictionResponse{}, nil
}
func TestAuthClient_FreshExactCredentialAndUntrustedBindingRefusal(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, subject := uuid.New(), uuid.New()
	requestIDs := map[string]bool{}
	binding := &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), LifecycleRevision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}
	client := &AuthClient{Issuer: issuer, Client: authFixture{resolve: func(ctx context.Context, r *automodv1.ResolveModerationSubjectRequest) (*automodv1.ResolveModerationSubjectResponse, error) {
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		require.Empty(t, md.Get("x-voice-user-id"))
		require.Empty(t, md.Get("x-profile-id"))
		id := md.Get("x-request-id")[0]
		require.False(t, requestIDs[id])
		requestIDs[id] = true
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		p, err := principal.VerifyService(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "moderation", ExpectedAudience: "auth", ExpectedRPC: automodv1.AuthModerationAuthorityService_ResolveModerationSubject_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
		require.NoError(t, err)
		require.Equal(t, "service:moderation", p.Subject)
		return &automodv1.ResolveModerationSubjectResponse{Subject: binding, SessionEpoch: 1}, nil
	}}}
	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "untrusted-user-token", "x-voice-user-id", account.String(), "x-profile-id", uuid.NewString()))
	for attempt := 0; attempt < 2; attempt++ {
		_, err = client.ResolveSubject(ctx, account, subject)
		require.NoError(t, err)
	}
	binding.AccountId = uuid.NewString()
	_, err = client.ResolveSubject(ctx, account, subject)
	require.Error(t, err)
	_, err = client.TerminalSubjects(ctx)
	require.Error(t, err, "incomplete snapshot must not unblock readiness")
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.Error(t, err, "default/unsigned projection response cannot acknowledge a new revision")
}
