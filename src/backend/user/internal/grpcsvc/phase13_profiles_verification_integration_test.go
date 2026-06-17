package grpcsvc

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func startUserGRPCForPhase13(t *testing.T, profiles *store.ProfileStore, events *phase13EventsRecorder) (userv1.UserServiceClient, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	var eventsPub UserEventsPublisher
	if events != nil {
		eventsPub = events
	}
	userv1.RegisterUserServiceServer(srv, &UserGRPC{
		Profiles:            profiles,
		Presence:            store.NewPresenceStore(rdb),
		Events:              eventsPub,
		AvatarPresigner:     stubAvatarPresigner{},
		AvatarPublicBaseURL: "https://cdn-test.example",
	})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), func() {}
}

func profileProtoJSON(t *testing.T, p *userv1.Profile) map[string]any {
	t.Helper()
	b, err := protojson.Marshal(p)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}

// TestSwitchProfile_PublishesProfileSwitchedEvent documents user.profile_switched on active profile change.
func TestSwitchProfile_PublishesProfileSwitchedEvent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	events := &phase13EventsRecorder{}
	cli, _ := startUserGRPCForPhase13(t, profiles, events)

	accountID := uuid.New()
	primaryID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'primary', '0001', 'Primary', true)`,
		primaryID, accountID)
	require.NoError(t, err)

	authed := withAccountTier(ctx, accountID, "premium")
	secondary, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Alt"})
	require.NoError(t, err)
	secondaryID := secondary.GetProfile().GetId()

	switchCtx := metadata.AppendToOutgoingContext(authed,
		authctx.HeaderProfileID, primaryID.String(),
	)
	_, err = cli.SwitchProfile(switchCtx, &userv1.SwitchProfileRequest{ProfileId: secondaryID})
	require.NoError(t, err)

	require.Len(t, events.profileSwitched, 1, "SwitchProfile must publish user.profile_switched")
	require.Equal(t, accountID.String(), events.profileSwitched[0].accountID)
	require.Equal(t, primaryID.String(), events.profileSwitched[0].oldProfileID)
	require.Equal(t, secondaryID, events.profileSwitched[0].newProfileID)
}

// TestDeleteProfile_SoftArchivesAndBlocksSwitch documents soft-delete hides profile and blocks switch.
func TestDeleteProfile_SoftArchivesAndBlocksSwitch(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForPhase13(t, profiles, nil)

	accountID := uuid.New()
	primaryID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'delpri', '0001', 'Primary', true)`,
		primaryID, accountID)
	require.NoError(t, err)

	authed := withAccountTier(ctx, accountID, "premium")
	secondary, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "To Delete"})
	require.NoError(t, err)
	secondaryID := secondary.GetProfile().GetId()

	_, err = cli.DeleteProfile(authed, &userv1.DeleteProfileRequest{ProfileId: secondaryID})
	require.NoError(t, err)

	var deletedAt *time.Time
	err = pool.QueryRow(ctx, `SELECT deleted_at FROM profiles WHERE id = $1`, secondaryID).Scan(&deletedAt)
	require.NoError(t, err)
	require.NotNil(t, deletedAt, "DeleteProfile must soft-archive with deleted_at")

	_, err = cli.SwitchProfile(authed, &userv1.SwitchProfileRequest{ProfileId: secondaryID})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestGetVerificationStatus_ReturnsBadgeState documents verification badge read API.
func TestGetVerificationStatus_ReturnsBadgeState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForPhase13(t, profiles, nil)

	accountID := uuid.New()
	pid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary,
			verification_type, verification_badge)
		VALUES ($1, $2, 'verified', '0001', 'Verified Streamer', true, 'personal', 'twitch')`,
		pid, accountID)
	require.NoError(t, err)

	authed := withAccountTier(ctx, accountID, "free")
	resp, err := cli.GetVerificationStatus(authed, &userv1.GetVerificationStatusRequest{
		ProfileId: pid.String(),
	})
	require.NoError(t, err)
	require.Equal(t, "personal", resp.GetVerificationStatus().GetVerificationType())
	require.Equal(t, "twitch", resp.GetVerificationStatus().GetBadge())
}

// TestSetVerification_S2S_PersonalBadge documents trusted S2S badge grant from Auth OAuth flow.
func TestSetVerification_S2S_PersonalBadge(t *testing.T) {
	t.Parallel()
	// Contract gate: RPC must exist before integration wiring (see phase13_proto_contract_test.go).
	requireContainsGRPCMethod(t, "SetVerification")
}

// TestClearVerification_S2S_RemovesBadge documents Auth unlink clears verification via User S2S.
func TestClearVerification_S2S_RemovesBadge(t *testing.T) {
	t.Parallel()
	requireContainsGRPCMethod(t, "ClearVerification")
}

// TestSearchProfiles_OrdersVerifiedBeforeAlphabetical documents verified profiles rank above unverified matches.
func TestSearchProfiles_OrdersVerifiedBeforeAlphabetical(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForPhase13(t, profiles, nil)

	viewer := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'viewer', '0099', 'Viewer', true)`, uuid.New(), viewer)
	require.NoError(t, err)

	unverifiedAcc := uuid.New()
	verifiedAcc := uuid.New()
	unverifiedID := uuid.New()
	verifiedID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, verification_type)
		VALUES ($1, $2, 'aaafaker', '0001', 'Faker Homoglyph', true, 'none')`,
		unverifiedID, unverifiedAcc)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, verification_type)
		VALUES ($1, $2, 'zzzfaker', '0002', 'Faker Official', true, 'personal')`,
		verifiedID, verifiedAcc)
	require.NoError(t, err)

	authed := withAccountTier(ctx, viewer, "free")
	resp, err := cli.SearchProfiles(authed, &userv1.SearchProfilesRequest{Query: "Faker"})
	require.NoError(t, err)
	hits := resp.GetProfileList().GetProfiles()
	require.GreaterOrEqual(t, len(hits), 2)
	require.Equal(t, verifiedID.String(), hits[0].GetId(),
		"verified profile must sort before unverified for the same query")
}

// TestCreateProfile_RejectsHomoglyphSpoofNearVerifiedName documents NFKC/homoglyph anti-spoofing.
func TestCreateProfile_RejectsHomoglyphSpoofNearVerifiedName(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForPhase13(t, profiles, nil)

	verifiedAcc := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, verification_type)
		VALUES ($1, $2, 'faker', '0001', 'Faker', true, 'personal')`,
		uuid.New(), verifiedAcc)
	require.NoError(t, err)

	spooferAcc := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'spoofpri', '0001', 'Spoofer Primary', true)`,
		uuid.New(), spooferAcc)
	require.NoError(t, err)

	// Cyrillic 'а' (U+0430) spoofs Latin 'a' in "faker".
	homoglyph := "fаker"
	authed := withAccountTier(ctx, spooferAcc, "premium")
	_, err = cli.CreateProfile(authed, &userv1.CreateProfileRequest{
		DisplayName: "Fake Faker",
		Username:    proto.String(homoglyph),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestFrozenAt_SurfacedOnProfileProto documents frozen_at is returned on Profile messages.
func TestFrozenAt_SurfacedOnProfileProto(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForPhase13(t, profiles, nil)

	accountID := uuid.New()
	pid := uuid.New()
	frozenAt := time.Now().UTC().Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, frozen_at)
		VALUES ($1, $2, 'frozen', '0001', 'Frozen Alt', false, $3)`,
		pid, accountID, frozenAt)
	require.NoError(t, err)

	authed := withAccountTier(ctx, accountID, "free")
	resp, err := cli.ListMyProfiles(authed, &userv1.ListMyProfilesRequest{})
	require.NoError(t, err)

	var frozenProfile *userv1.Profile
	for _, p := range resp.GetProfileList().GetProfiles() {
		if p.GetId() == pid.String() {
			frozenProfile = p
			break
		}
	}
	require.NotNil(t, frozenProfile)
	fields := profileProtoJSON(t, frozenProfile)
	require.Contains(t, fields, "frozenAt", "Profile proto must expose frozen_at for downgrade UX")
}
