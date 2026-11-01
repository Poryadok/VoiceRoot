package grpcsvc

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"net"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	migrateengine "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

type moderationCaptureFixture map[uuid.UUID]uuid.UUID

// Use the production migration adapter and an exclusive disposable database
// window. Native51 removes repeated full catalog scans from the bounded call.
func migrateModerationFixture51(t *testing.T, ctx context.Context, old *pgxpool.Pool, directory string) *pgxpool.Pool {
	t.Helper()
	prior, err := (&store.SpaceStore{Pool: old}).InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	config := old.Config().Copy()
	old.Close()
	app := "moderation-native51-" + uuid.NewString()
	config.ConnConfig.RuntimeParams["application_name"] = app
	config.MaxConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st := &store.SpaceStore{Pool: pool}
	window := func(ctx context.Context) error {
		var exclusive bool
		err := pool.QueryRow(ctx, `SELECT current_user=session_user AND NOT pg_is_in_recovery()
		 AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database()
		 AND backend_type='client backend' AND application_name<>$1)`, app).Scan(&exclusive)
		if err != nil || !exclusive {
			return store.ErrMemberAuthorityUnavailable
		}
		return nil
	}
	require.NoError(t, window(ctx))
	driver, err := st.NewNativeCatalog51Migration(ctx, prior, window)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, driver.Close()) })
	path, err := filepath.Abs(directory)
	require.NoError(t, err)
	engine, err := migrateengine.NewWithDatabaseInstance((&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String(), "moderation-native51", driver)
	require.NoError(t, err)
	t.Cleanup(func() { sourceErr, dbErr := engine.Close(); require.NoError(t, sourceErr); require.NoError(t, dbErr) })
	migrationErr := engine.Migrate(51)
	receipt, err := driver.CommitQualified(migrationErr)
	require.NoError(t, migrationErr)
	require.NoError(t, err)
	require.True(t, receipt.Changed)
	require.Equal(t, prior.SealSHA256, receipt.OldSealSHA256)
	require.NoError(t, driver.Close())
	current, err := st.InspectNativeInstallerSource(ctx, 51)
	require.NoError(t, err)
	require.Equal(t, receipt.NewSealSHA256, current.SealSHA256)
	require.Equal(t, prior.DatabaseOID, current.DatabaseOID)
	require.Equal(t, prior.Epoch, current.Epoch)
	return pool
}

// User facts are controlled component inputs; this does not claim protected
// User-provider qualification. The Space receiver and database remain real.
func (f moderationCaptureFixture) CaptureSpaceMemberAccountBindings(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
	out := &userv1.CaptureSpaceMemberAccountBindingsResponse{Protocol: 1, OperationId: r.OperationId, SpaceId: r.SpaceId, Purpose: r.Purpose,
		CohortDigest: append([]byte(nil), r.CohortDigest...), RequestDigest: spaceaccountcapture.RequestDigest(r), OwnerSourceRevision: 901,
		UserSourceGeneration: "50000000-0000-0000-0000-000000000001", IdentityCut: &userv1.SpaceAccountIdentityCut{Owner: "user", Stream: "identity-association", HistoryGeneration: "50000000-0000-0000-0000-000000000002", ThroughRevision: 3, KeysetRevision: 1, Digest: bytes.Repeat([]byte{1}, 32)}}
	for _, entry := range r.Cohort.Entries {
		digest := sha256.Sum256([]byte("moderation-receipt:" + entry.ProfileId))
		out.Results = append(out.Results, &userv1.SpaceAccountCaptureResult{Entry: proto.Clone(entry).(*userv1.SpaceAccountCaptureEntry), Outcome: 1,
			AccountId: f[uuid.MustParse(entry.ProfileId)].String(), AssociationId: uuid.NewSHA1(uuid.NameSpaceOID, []byte(entry.ProfileId)).String(), AssociationRevision: 1,
			ProfileRevision: 7, Origin: 1, AssociationOrigin: 2, OwnerReceiptId: uuid.NewSHA1(uuid.NameSpaceOID, digest[:]).String(), OwnerReceiptDigest: digest[:], CaptureCut: timestamppb.Now()})
	}
	return out, nil
}

type nativeModerationFixture struct {
	st                                                *store.SpaceStore
	client                                            spacev1.SpaceServiceClient
	ordinary                                          spacev1.SpaceServiceClient
	issuer                                            *principal.Issuer
	spaceIssuer                                       *principal.Issuer
	roles                                             rolev1.RoleServiceClient
	protectedRoles                                    rolev1.RoleServiceClient
	space, owner, ownerAccount, target, targetAccount uuid.UUID
}

func newNativeModerationFixture(t *testing.T) *nativeModerationFixture {
	return newNativeModerationFixtureWithProfiles(t, uuid.New(), uuid.New(), uuid.New(), uuid.New())
}

func newNativeModerationFixtureWithProfiles(t *testing.T, owner, ownerAccount, target, targetAccount uuid.UUID) *nativeModerationFixture {
	return newNativeModerationFixtureWithMembership(t, owner, ownerAccount, target, targetAccount, false)
}

func newNativeModerationFixtureWithMembership(t *testing.T, owner, ownerAccount, target, targetAccount uuid.UUID, community bool) *nativeModerationFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	f := &nativeModerationFixture{owner: owner, ownerAccount: ownerAccount, target: target, targetAccount: targetAccount}
	pool, row := nativeLifecycleProofFixtureWithSeed(t, ctx, f.owner, "Native moderation", func(st *store.SpaceStore, row *store.SpaceRow) {
		var err error
		if community {
			_, err = st.Pool.Exec(ctx, `INSERT INTO community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,roster_source_revision,roster_sha256,roster_lease_expires_at) VALUES($1,$2,$3,'ban-community',$4,$5,1,'active',1,$6,clock_timestamp()+interval '10 minutes')`, row.ID, uuid.New(), uuid.New(), f.ownerAccount, f.owner, bytes.Repeat([]byte{1}, 32))
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO community_roster_members(space_id,profile_id,source_rank,source_reasons,source_revision,owner_generation,lease_expires_at) VALUES($1,$2,'member',ARRAY['membership'],1,1,clock_timestamp()+interval '10 minutes')`, row.ID, f.target)
		} else {
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, row.ID, f.target)
		}
		require.NoError(t, err)
	}, true)
	f.space = row.ID
	f.st = &store.SpaceStore{Pool: pool, AccountCaptures: moderationCaptureFixture{f.owner: f.ownerAccount, f.target: f.targetAccount}}
	roles, closeRoles := startSharedRoleClient(t)
	f.roles = roles
	t.Cleanup(closeRoles)
	_, err := roles.BootstrapSpaceRoles(ctx, &rolev1.BootstrapSpaceRolesRequest{SpaceId: row.ID.String(), OwnerProfileId: f.owner.String()})
	require.NoError(t, err)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f.issuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "moderation-test", PrivateKey: key})
	require.NoError(t, err)
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	roleFixture := newInviteIngressFixture(t)
	f.spaceIssuer = roleFixture.space
	service := &SpaceGRPC{Store: f.st, Roles: roles, PrincipalIssuer: f.spaceIssuer}
	f.protectedRoles = legacySettingsRoleFixture{RoleServiceClient: roles, svc: service, fixture: roleFixture}
	service.OwnershipRoles = f.protectedRoles
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer(grpc.UnaryInterceptor(verifier.Unary()))
	spacev1.RegisterSpaceServiceServer(server, &PublicLifecycleSpace{SpaceGRPC: service})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///moderation", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	f.client = spacev1.NewSpaceServiceClient(conn)
	var cleanup func()
	f.ordinary, cleanup = startSpaceGRPCTestServer(t, pool, withRoleClient(roles), withProfileAccounts(mapProfileAccounts{f.owner: f.ownerAccount, f.target: f.targetAccount}), func(s *SpaceGRPC) {
		s.PrincipalIssuer = f.spaceIssuer
		s.OwnershipRoles = f.protectedRoles
	})
	t.Cleanup(cleanup)
	return f
}

func (f *nativeModerationFixture) signed(t *testing.T, method string, r proto.Message) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := f.issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: id, RequestHash: hash, AccountID: f.ownerAccount.String(), ProfileID: f.owner.String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
}

func (f *nativeModerationFixture) ban(t *testing.T, account uuid.UUID, reason *string) error {
	r := &spacev1.BanMemberRequest{SpaceId: f.space.String(), AccountId: account.String(), Reason: reason, OperationId: uuid.NewString()}
	_, err := f.client.BanMember(f.signed(t, spacev1.SpaceService_BanMember_FullMethodName, r), r)
	return err
}

func (f *nativeModerationFixture) unban(t *testing.T) error {
	r := &spacev1.UnbanMemberRequest{SpaceId: f.space.String(), AccountId: f.targetAccount.String(), OperationId: uuid.NewString()}
	_, err := f.client.UnbanMember(f.signed(t, spacev1.SpaceService_UnbanMember_FullMethodName, r), r)
	return err
}

func (f *nativeModerationFixture) invite(t *testing.T) string {
	t.Helper()
	r, err := f.ordinary.CreateInvite(withAccountProfileCtx(context.Background(), f.ownerAccount, f.owner), &spacev1.CreateInviteRequest{SpaceId: f.space.String()})
	require.NoError(t, err)
	return r.GetInvite().GetCode()
}

func (f *nativeModerationFixture) admitAgain(t *testing.T, code string) {
	t.Helper()
	ctx := context.Background()
	invite, err := f.st.GetInviteByCode(ctx, code)
	require.NoError(t, err)
	require.NotNil(t, invite)
	intent, err := f.st.PrepareMemberAdmission(ctx, store.MemberAdmissionInput{OperationID: uuid.New(), SpaceID: f.space, ProfileID: f.target, AccountID: f.targetAccount, InviteID: &invite.ID})
	require.NoError(t, err)
	// Controlled Role receipt, as in the native store component tests. This
	// proves new publication after Unban, never revival of the retired source.
	role := uuid.New()
	receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
	require.NoError(t, err)
	_, err = f.st.PublishMemberAdmission(ctx, intent.Input.OperationID, role, receipt[:], nil)
	require.NoError(t, err)
}
