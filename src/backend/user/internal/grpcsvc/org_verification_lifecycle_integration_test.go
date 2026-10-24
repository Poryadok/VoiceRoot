package grpcsvc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/user/internal/store"
)

func TestOrganizationVerification_RequestLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	create := func(t *testing.T) (uuid.UUID, context.Context) {
		t.Helper()
		pid, account := uuid.New(), uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO profiles (id,account_id,username,discriminator,display_name,is_primary) VALUES($1,$2,$3,'0001','Org',true)`, pid, account, "org"+strings.ReplaceAll(pid.String(), "-", "")[:12])
		require.NoError(t, err)
		return pid, withAccountTier(ctx, account, "free")
	}
	t.Run("migration_reconciles_legacy_rows_and_rollback_never_revives_tokens", func(t *testing.T) {
		pid, _ := create(t)
		expiredPID, _ := create(t)
		root := repoRoot(t)
		readMigration := func(suffix string) string {
			data, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "user_db", "000021_org_verification_request_lifecycle."+suffix+".sql"))
			require.NoError(t, err)
			return string(data)
		}
		_, err := pool.Exec(ctx, readMigration("down"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO organization_verification_requests(profile_id,domain,txt_token,status,created_at)
			VALUES($1,'legacy.example','older','pending',now()-interval '2 hours'),
			($1,'legacy.example','latest','pending',now()-interval '1 hour'),
			($1,'legacy.example','verified','verified',now()),
			($2,'expired.example','expired','pending',now()-interval '25 hours')`, pid, expiredPID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, readMigration("up"))
		require.NoError(t, err)
		_, token, err := profiles.LatestOrgVerification(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, "latest", token)
		_, _, err = profiles.LatestOrgVerification(ctx, expiredPID)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO organization_verification_requests(profile_id,domain,txt_token) VALUES($1,'other.example','duplicate')`, pid)
		require.Error(t, err, "database rejects a second pending request")
		_, err = pool.Exec(ctx, readMigration("down"))
		require.NoError(t, err)
		var retired, verified int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='expired'), count(*) FILTER(WHERE status='verified') FROM organization_verification_requests WHERE profile_id IN($1,$2)`, pid, expiredPID).Scan(&retired, &verified))
		require.Equal(t, 2, retired)
		require.Equal(t, 1, verified)
		_, err = pool.Exec(ctx, readMigration("up"))
		require.NoError(t, err)
	})
	t.Run("concurrent_starts_have_one_current_request", func(t *testing.T) {
		pid, _ := create(t)
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() { defer wg.Done(); _, err := profiles.StartOrgVerification(ctx, pid, "example.org"); errs <- err }()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			require.NoError(t, err)
		}
		var pending, total int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FILTER(WHERE status='pending'),count(*) FROM organization_verification_requests WHERE profile_id=$1`, pid).Scan(&pending, &total))
		require.Equal(t, 1, pending)
		require.Equal(t, 8, total)
	})
	t.Run("expired_request_never_looks_up_dns_or_grants_badge", func(t *testing.T) {
		pid, authed := create(t)
		txt, err := profiles.StartOrgVerification(ctx, pid, "expired.example")
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE organization_verification_requests SET created_at=now()-interval '25 hours' WHERE profile_id=$1`, pid)
		require.NoError(t, err)
		called := false
		cli, _ := startUserGRPCForPhase13WithDNS(t, profiles, nil, dnsResolverFunc(func(context.Context, string) ([]string, error) { called = true; return []string{txt}, nil }))
		_, err = cli.CheckOrganizationVerification(authed, &userv1.CheckOrganizationVerificationRequest{ProfileId: pid.String()})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.False(t, called)
		row, err := profiles.GetByID(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, "none", row.VerificationType)
	})
	t.Run("supersession_during_dns_cannot_verify_new_request", func(t *testing.T) {
		pid, authed := create(t)
		oldTXT, err := profiles.StartOrgVerification(ctx, pid, "old.example")
		require.NoError(t, err)
		events := &profilesVerificationEventsRecorder{}
		cli, _ := startUserGRPCForPhase13WithDNS(t, profiles, events, dnsResolverFunc(func(context.Context, string) ([]string, error) {
			_, err := profiles.StartOrgVerification(ctx, pid, "new.example")
			return []string{oldTXT}, err
		}))
		_, err = cli.CheckOrganizationVerification(authed, &userv1.CheckOrganizationVerificationRequest{ProfileId: pid.String()})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		require.Zero(t, events.verified)
		row, err := profiles.GetByID(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, "none", row.VerificationType)
		domain, _, err := profiles.LatestOrgVerification(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, "new.example", domain)
	})
	t.Run("dns_outage_preserves_request_then_current_token_succeeds", func(t *testing.T) {
		pid, authed := create(t)
		txt, err := profiles.StartOrgVerification(ctx, pid, "retry.example")
		require.NoError(t, err)
		down := true
		cli, _ := startUserGRPCForPhase13WithDNS(t, profiles, nil, dnsResolverFunc(func(context.Context, string) ([]string, error) {
			if down {
				return nil, errors.New("resolver unavailable")
			}
			return []string{txt}, nil
		}))
		req := &userv1.CheckOrganizationVerificationRequest{ProfileId: pid.String()}
		_, err = cli.CheckOrganizationVerification(authed, req)
		require.Equal(t, codes.Unavailable, status.Code(err))
		_, token, err := profiles.LatestOrgVerification(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, strings.TrimPrefix(txt, "voice-verify="), token)
		down = false
		got, err := cli.CheckOrganizationVerification(authed, req)
		require.NoError(t, err)
		require.Equal(t, "organization", got.GetVerificationStatus().GetVerificationType())
		_, _, err = profiles.LatestOrgVerification(ctx, pid)
		require.Error(t, err)
	})
	t.Run("expiry_during_dns_cannot_grant_badge", func(t *testing.T) {
		pid, authed := create(t)
		txt, err := profiles.StartOrgVerification(ctx, pid, "expire-during.example")
		require.NoError(t, err)
		cli, _ := startUserGRPCForPhase13WithDNS(t, profiles, nil, dnsResolverFunc(func(context.Context, string) ([]string, error) {
			_, err := pool.Exec(ctx, `UPDATE organization_verification_requests SET created_at=now()-interval '25 hours' WHERE profile_id=$1`, pid)
			return []string{txt}, err
		}))
		_, err = cli.CheckOrganizationVerification(authed, &userv1.CheckOrganizationVerificationRequest{ProfileId: pid.String()})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
		row, err := profiles.GetByID(ctx, pid)
		require.NoError(t, err)
		require.Equal(t, "none", row.VerificationType)
	})
}
