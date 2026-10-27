package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/user/internal/store"
)

func TestOrdinaryPrimaryLookupPreservesAuthAndRejectsNotificationHeader(t *testing.T) {
	req := &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: []string{uuid.NewString()}}
	for _, caller := range []string{"notification", "social", "subscription", "auth ", ""} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", caller))
		_, err := (&UserGRPC{}).ResolvePrimaryProfileIDs(ctx, req)
		require.Equal(t, codes.PermissionDenied, status.Code(err), caller)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-internal-caller", "auth"))
	_, err := (&UserGRPC{}).ResolvePrimaryProfileIDs(ctx, req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "documented Auth caller still reaches store dependency")
}

func primaryVerifiedContext(t *testing.T, req *userv1.ResolvePrimaryProfileIDsRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "notification", Subject: "service:notification", Audience: "user", RPC: userv1.UserService_ResolvePrimaryProfileIDs_FullMethodName, RequestHash: hash})
}

func TestNotificationPrimaryDomainBindsIdentityMethodAndOneAccount(t *testing.T) {
	svc := &NotificationPrimaryGRPC{User: &UserGRPC{}}
	req := &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: []string{uuid.NewString()}}
	_, err := svc.ResolvePrimaryProfileIDs(context.Background(), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	ctx := primaryVerifiedContext(t, req)
	_, err = svc.ResolvePrimaryProfileIDs(ctx, req)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "authenticated exact body reaches User store")
	_, err = svc.ResolvePrimaryProfileIDs(ctx, &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: []string{uuid.NewString()}})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	for _, change := range []func(*principal.Principal){
		func(p *principal.Principal) { p.Issuer = "social"; p.Subject = "service:social" },
		func(p *principal.Principal) { p.RPC = userv1.UserService_GetNotificationRoutingPresence_FullMethodName },
		func(p *principal.Principal) { p.Audience = "subscription" },
		func(p *principal.Principal) { p.AccountID = uuid.NewString() },
	} {
		p, _ := principal.FromContext(ctx)
		change(&p)
		_, err = svc.ResolvePrimaryProfileIDs(principal.WithVerified(context.Background(), p), req)
		require.Contains(t, []codes.Code{codes.PermissionDenied, codes.Unauthenticated}, status.Code(err))
	}
	for _, ids := range [][]string{nil, {uuid.NewString(), uuid.NewString()}, {uuid.Nil.String()}, {"bad"}} {
		body := &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: ids}
		_, err = svc.ResolvePrimaryProfileIDs(primaryVerifiedContext(t, body), body)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	var recorder serviceDescriptorRecorder
	RegisterNotificationPrimaryServer(&recorder, &UserGRPC{})
	require.Len(t, recorder.desc.Methods, 1)
	require.Equal(t, "ResolvePrimaryProfileIDs", recorder.desc.Methods[0].MethodName)
	require.Empty(t, recorder.desc.Streams)
}

func TestNotificationPrimaryReadsExistingCanonicalProfileWithoutProvisioning(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	svc := &NotificationPrimaryGRPC{User: &UserGRPC{Profiles: store.NewProfileStore(pool)}}
	account, primary, secondary, deletedAccount, absent := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles (id,account_id,username,discriminator,display_name,is_primary,frozen_at,deleted_at) VALUES
 ($1,$2,'primary','0001','Primary',true,clock_timestamp(),NULL),
 ($3,$2,'secondary','0001','Secondary',false,NULL,NULL),
 ($4,$5,'deletedprimary','0001','Deleted',true,NULL,clock_timestamp())`, primary, account, secondary, uuid.New(), deletedAccount)
	require.NoError(t, err)
	for _, tc := range []struct {
		account uuid.UUID
		want    string
	}{{account, primary.String()}, {deletedAccount, ""}, {absent, ""}} {
		req := &userv1.ResolvePrimaryProfileIDsRequest{AccountIds: []string{tc.account.String()}}
		response, err := svc.ResolvePrimaryProfileIDs(primaryVerifiedContext(t, req), req)
		require.NoError(t, err)
		require.Equal(t, tc.want, response.PrimaryProfileIds[tc.account.String()])
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM profiles").Scan(&count))
	require.Equal(t, 3, count, "lookup creates no profile for absent account")
}
