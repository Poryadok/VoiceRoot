package grpcsvc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/store"
)

func TestGetVerificationStatus_PendingOrganizationChallenge(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	events := &profilesVerificationEventsRecorder{}
	dnsCalls := 0
	var dnsTXT string
	cli, _ := startUserGRPCForPhase13WithDNS(t, profiles, events, dnsResolverFunc(func(context.Context, string) ([]string, error) {
		dnsCalls++
		return []string{dnsTXT}, nil
	}))
	account, pid, other := uuid.New(), uuid.New(), uuid.New()
	for _, id := range []uuid.UUID{pid, other} {
		_, err := pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name,is_primary,verification_type,verification_badge)
		 VALUES($1,$2,$3,'0001','Organization',false,'personal','twitch')`, id, account, "org"+strings.ReplaceAll(id.String(), "-", "")[:20])
		require.NoError(t, err)
	}
	authed := withAccountTier(ctx, account, "free")
	read := func(id uuid.UUID) map[string]any {
		t.Helper()
		resp, err := cli.GetVerificationStatus(authed, &userv1.GetVerificationStatusRequest{ProfileId: id.String()})
		require.NoError(t, err)
		b, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(resp)
		require.NoError(t, err)
		var wire map[string]any
		require.NoError(t, json.Unmarshal(b, &wire))
		return wire
	}
	snapshot := func() string {
		t.Helper()
		var value string
		require.NoError(t, pool.QueryRow(ctx, `SELECT json_build_object(
		 'requests',(SELECT json_agg(r ORDER BY id) FROM organization_verification_requests r),
		 'profiles',(SELECT json_agg(p ORDER BY id) FROM profiles p),
		 'sources',(SELECT json_agg(s ORDER BY profile_id,source) FROM profile_verification_sources s))::text`).Scan(&value))
		return value
	}
	require.NotContains(t, read(pid), "pending_organization_challenge")
	txt, err := profiles.StartOrgVerification(ctx, pid, " Example.ORG ")
	require.NoError(t, err)
	var expires time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT created_at + interval '24 hours' FROM organization_verification_requests WHERE profile_id=$1 AND status='pending'`, pid).Scan(&expires))
	before := snapshot()
	for range 2 {
		wire := read(pid)
		require.Contains(t, wire, "pending_organization_challenge", "reload must restore persisted pending challenge")
		pending := wire["pending_organization_challenge"].(map[string]any)
		require.Equal(t, "example.org", pending["domain"])
		require.Equal(t, txt, pending["txt_record"])
		gotExpiry, parseErr := time.Parse(time.RFC3339Nano, pending["expires_at"].(string))
		require.NoError(t, parseErr)
		require.True(t, expires.Equal(gotExpiry))
		require.Equal(t, "personal", wire["verification_status"].(map[string]any)["verification_type"])
	}
	require.Equal(t, before, snapshot(), "GET must leave all durable verification/profile state unchanged")
	require.Zero(t, dnsCalls)
	require.Zero(t, events.verified)
	require.NotContains(t, read(other), "pending_organization_challenge")
	for _, tc := range []struct {
		ctx  context.Context
		id   string
		code codes.Code
	}{
		{ctx, pid.String(), codes.Unauthenticated}, {authed, "bad", codes.InvalidArgument},
		{withAccountTier(ctx, uuid.New(), "free"), pid.String(), codes.NotFound}, {authed, uuid.NewString(), codes.NotFound},
	} {
		_, callErr := cli.GetVerificationStatus(tc.ctx, &userv1.GetVerificationStatusRequest{ProfileId: tc.id})
		require.Equal(t, tc.code, status.Code(callErr))
	}
	dnsTXT, err = profiles.StartOrgVerification(ctx, pid, "new.example")
	require.NoError(t, err)
	pending := read(pid)["pending_organization_challenge"].(map[string]any)
	require.Equal(t, dnsTXT, pending["txt_record"])
	require.Equal(t, "new.example", pending["domain"])
	require.NotEqual(t, txt, dnsTXT)
	_, err = cli.CheckOrganizationVerification(authed, &userv1.CheckOrganizationVerificationRequest{ProfileId: pid.String()})
	require.NoError(t, err)
	require.NotContains(t, read(pid), "pending_organization_challenge")
	require.Equal(t, "organization", read(pid)["verification_status"].(map[string]any)["verification_type"])
	_, err = profiles.StartOrgVerification(ctx, pid, "expired.example")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE organization_verification_requests SET created_at=statement_timestamp()-interval '24 hours' WHERE profile_id=$1 AND status='pending'`, pid)
	require.NoError(t, err)
	before = snapshot()
	require.NotContains(t, read(pid), "pending_organization_challenge")
	require.Equal(t, before, snapshot(), "expired pending row must not be cleaned up on read")
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at=now() WHERE id=$1`, pid)
	require.NoError(t, err)
	_, err = cli.GetVerificationStatus(authed, &userv1.GetVerificationStatusRequest{ProfileId: pid.String()})
	require.Equal(t, codes.NotFound, status.Code(err))
}
