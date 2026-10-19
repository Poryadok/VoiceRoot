package automodclient

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
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
	resolve    func(context.Context, *automodv1.ResolveModerationSubjectRequest) (*automodv1.ResolveModerationSubjectResponse, error)
	reconciled *automodv1.ReconcileModerationRestrictionResponse
	terminal   *automodv1.ListTerminalModerationSubjectsResponse
}

func (f authFixture) ResolveModerationSubject(ctx context.Context, r *automodv1.ResolveModerationSubjectRequest, _ ...grpc.CallOption) (*automodv1.ResolveModerationSubjectResponse, error) {
	return f.resolve(ctx, r)
}
func (f authFixture) ListTerminalModerationSubjects(context.Context, *automodv1.ListTerminalModerationSubjectsRequest, ...grpc.CallOption) (*automodv1.ListTerminalModerationSubjectsResponse, error) {
	if f.terminal != nil {
		return f.terminal, nil
	}
	return &automodv1.ListTerminalModerationSubjectsResponse{Complete: false}, nil
}

func TestAuthClientRejectsUnsealedCompleteTerminalSnapshot(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "moderation", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	client := &AuthClient{Issuer: issuer, Client: authFixture{terminal: &automodv1.ListTerminalModerationSubjectsResponse{Complete: true, SubjectIds: []string{uuid.NewString()}}}}
	_, err = client.TerminalSubjects(context.Background())
	require.Error(t, err, "complete bool alone cannot certify the finite terminal cut")
}
func (f authFixture) ReconcileModerationRestriction(context.Context, *automodv1.ReconcileModerationRestrictionRequest, ...grpc.CallOption) (*automodv1.ReconcileModerationRestrictionResponse, error) {
	if f.reconciled != nil {
		return f.reconciled, nil
	}
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
	client.Client = authFixture{reconciled: &automodv1.ReconcileModerationRestrictionResponse{Revision: 2, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}}
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.Error(t, err, "a newer remote revision never acknowledges the old request")
	client.Client = authFixture{reconciled: &automodv1.ReconcileModerationRestrictionResponse{Revision: 1, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}}
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.Error(t, err, "revision alone does not acknowledge the account, subject and desired state")
	ack := &automodv1.ReconcileModerationRestrictionResponse{ProtocolVersion: 1, AccountId: account.String(), SubjectId: subject.String(), Revision: 1, ModerationBanned: true, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}
	client.Client = authFixture{reconciled: ack}
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.NoError(t, err)
	ack.ModerationBanned = false
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.Error(t, err, "same revision with different accepted payload is not convergence")
	ack.Revision = 2
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	var ahead *RestrictionAheadError
	require.True(t, errors.As(err, &ahead))
	require.Equal(t, uint64(1), ahead.Requested)
	require.Equal(t, uint64(2), ahead.Accepted)
	require.True(t, ahead.DivergesFrom(1), "Auth ahead of current local authority requires recovery")
	require.False(t, ahead.DivergesFrom(2), "a concurrent local mutation explains a stale request reply")
	require.False(t, ahead.DivergesFrom(3), "an even newer local projection must be retried, not overwritten")
	ack.AccountId = uuid.NewString()
	_, err = client.Reconcile(ctx, account, subject, 1, true)
	require.False(t, errors.As(err, &ahead), "unbound remote revision cannot establish authority")
}
