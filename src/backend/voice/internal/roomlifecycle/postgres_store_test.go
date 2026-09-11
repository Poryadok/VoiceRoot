package roomlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/integrationtest"
)

// This file freezes RED-C from R22.2-VOICE-DB-PLAN.md lines 798-827.

var r22StoreTime = time.Date(2100, 1, 1, 12, 0, 0, 0, time.UTC)

type r22StoreFixture struct {
	ctx                    context.Context
	pool                   *pgxpool.Pool
	store                  *PostgresLifecycleStore
	actorAccountID         uuid.UUID
	actorProfileID         uuid.UUID
	subjectProfileID       uuid.UUID
	spaceID                uuid.UUID
	sourceVoiceRoomID      uuid.UUID
	destinationVoiceRoomID uuid.UUID
	sourceRoomID           uuid.UUID
	sourceMediaEpoch       uuid.UUID
	operationID            uuid.UUID
	workerID               uuid.UUID
}

func r22NewStoreFixture(t *testing.T, name string) r22StoreFixture {
	t.Helper()
	ctx := context.Background()
	pool := r22StartVoicePostgres(t, ctx, name)
	return r22StoreFixture{
		ctx:                    ctx,
		pool:                   pool,
		store:                  NewPostgresLifecycleStore(pool),
		actorAccountID:         uuid.New(),
		actorProfileID:         uuid.New(),
		subjectProfileID:       uuid.New(),
		spaceID:                uuid.New(),
		sourceVoiceRoomID:      uuid.New(),
		destinationVoiceRoomID: uuid.New(),
		sourceMediaEpoch:       uuid.New(),
		operationID:            uuid.New(),
		workerID:               uuid.New(),
	}
}

func r22StoreDigest(seed byte) LifecycleDigest {
	var digest LifecycleDigest
	for index := range digest {
		digest[index] = seed + byte(index)
	}
	return digest
}

func r22Pointer[T any](value T) *T { return &value }

func r22AllGrants() LifecycleGrants {
	return LifecycleGrants{
		CanJoin: true, CanPublishAudio: false, CanPublishVideo: true,
		CanPublishScreenShare: false, CanSubscribe: true, CanMuteOthers: false,
		CanDeafenOthers: true, CanMoveOthers: true, CanUsePTT: false,
		PrioritySpeaker: true,
	}
}

func r22RequireGrants(t *testing.T, want, got LifecycleGrants) {
	t.Helper()
	require.Equal(t, want.CanJoin, got.CanJoin)
	require.Equal(t, want.CanPublishAudio, got.CanPublishAudio)
	require.Equal(t, want.CanPublishVideo, got.CanPublishVideo)
	require.Equal(t, want.CanPublishScreenShare, got.CanPublishScreenShare)
	require.Equal(t, want.CanSubscribe, got.CanSubscribe)
	require.Equal(t, want.CanMuteOthers, got.CanMuteOthers)
	require.Equal(t, want.CanDeafenOthers, got.CanDeafenOthers)
	require.Equal(t, want.CanMoveOthers, got.CanMoveOthers)
	require.Equal(t, want.CanUsePTT, got.CanUsePTT)
	require.Equal(t, want.PrioritySpeaker, got.PrioritySpeaker)
}

func r22RequireAuthority(t *testing.T, want, got LifecycleAuthority) {
	t.Helper()
	require.Equal(t, want.SpaceAccessEpoch, got.SpaceAccessEpoch)
	require.Equal(t, want.SubjectRolePolicyEpoch, got.SubjectRolePolicyEpoch)
	require.Equal(t, want.ActorSourceRolePolicyEpoch, got.ActorSourceRolePolicyEpoch)
	require.Equal(t, want.ActorDestinationRolePolicyEpoch, got.ActorDestinationRolePolicyEpoch)
	require.Equal(t, want.AuthorizationDigest, got.AuthorizationDigest)
	r22RequireGrants(t, want.SubjectGrants, got.SubjectGrants)
	require.Equal(t, want.ActorCanMoveSource, got.ActorCanMoveSource)
	require.Equal(t, want.ActorCanMoveDestination, got.ActorCanMoveDestination)
}

func r22RequireOperationDecision(t *testing.T, decision LifecycleDecision, got LifecycleOperation) {
	t.Helper()
	require.Equal(t, decision.ActorAccountID, got.ActorAccountID)
	require.Equal(t, decision.ActorProfileID, got.ActorProfileID)
	require.Equal(t, decision.OperationID, got.OperationID)
	require.Equal(t, decision.SubjectProfileID, got.SubjectProfileID)
	require.Equal(t, decision.SpaceID, got.SpaceID)
	require.Equal(t, decision.Method, got.Method)
	require.NotEqual(t, LifecycleDigest{}, got.Fingerprint)
	require.NotEmpty(t, got.BindingBytes)
	require.Equal(t, decision.SourceVoiceRoomID, got.SourceVoiceRoomID)
	require.Equal(t, decision.DestinationVoiceRoomID, got.DestinationVoiceRoomID)
	r22RequireAuthority(t, decision.Authority, got.Authority)
	require.Equal(t, decision.RedisOwnerToken, got.RedisOwnerToken)
	require.Equal(t, decision.DecidedAt, got.DecidedAt)
	require.Equal(t, decision.DecidedAt, got.CreatedAt)
}

func r22RequireSameImmutableOperation(t *testing.T, want, got LifecycleOperation) {
	t.Helper()
	require.Equal(t, want.ActorAccountID, got.ActorAccountID)
	require.Equal(t, want.ActorProfileID, got.ActorProfileID)
	require.Equal(t, want.OperationID, got.OperationID)
	require.Equal(t, want.SubjectProfileID, got.SubjectProfileID)
	require.Equal(t, want.SpaceID, got.SpaceID)
	require.Equal(t, want.Method, got.Method)
	require.Equal(t, want.Fingerprint, got.Fingerprint)
	require.Equal(t, want.BindingBytes, got.BindingBytes)
	require.Equal(t, want.SourceVoiceRoomID, got.SourceVoiceRoomID)
	require.Equal(t, want.DestinationVoiceRoomID, got.DestinationVoiceRoomID)
	require.Equal(t, want.SourceRoomID, got.SourceRoomID)
	require.Equal(t, want.DestinationRoomID, got.DestinationRoomID)
	require.Equal(t, want.SourceMediaEpoch, got.SourceMediaEpoch)
	require.Equal(t, want.DestinationMediaEpoch, got.DestinationMediaEpoch)
	r22RequireAuthority(t, want.Authority, got.Authority)
	require.Equal(t, want.RedisOwnerToken, got.RedisOwnerToken)
	require.Equal(t, want.DecidedAt, got.DecidedAt)
	require.Equal(t, want.CreatedAt, got.CreatedAt)
}

func (fixture r22StoreFixture) decision(method LifecycleMethod) LifecycleDecision {
	decision := LifecycleDecision{
		ActorAccountID: fixture.actorAccountID, ActorProfileID: fixture.actorProfileID,
		OperationID: fixture.operationID, SubjectProfileID: fixture.subjectProfileID,
		SpaceID: fixture.spaceID, Method: method, RedisOwnerToken: r22StoreDigest(0x40),
		AcceptedClockSkew: 2 * time.Second, DecidedAt: r22StoreTime,
	}
	ensure := LifecycleEffectPlan{EffectID: uuid.New(), Ordinal: 0, Kind: LifecycleEffectEnsureRoom, SchemaVersion: 1}
	eject := LifecycleEffectPlan{EffectID: uuid.New(), Ordinal: 0, Kind: LifecycleEffectEjectParticipant, SchemaVersion: 1}
	switch method {
	case LifecycleMethodJoin:
		decision.DestinationVoiceRoomID = r22Pointer(fixture.destinationVoiceRoomID)
		decision.Authority = r22SubjectAuthority(false)
		decision.Effects = []LifecycleEffectPlan{ensure}
	case LifecycleMethodLeave:
		decision.SourceVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID)
		decision.Effects = []LifecycleEffectPlan{eject}
	case LifecycleMethodSelfMove:
		decision.SourceVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID)
		decision.DestinationVoiceRoomID = r22Pointer(fixture.destinationVoiceRoomID)
		decision.Authority = r22SubjectAuthority(false)
		decision.Effects = []LifecycleEffectPlan{eject, {EffectID: ensure.EffectID, Ordinal: 1, Kind: ensure.Kind, SchemaVersion: 1}}
	case LifecycleMethodModeratorMove:
		decision.SourceVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID)
		decision.DestinationVoiceRoomID = r22Pointer(fixture.destinationVoiceRoomID)
		decision.Authority = r22SubjectAuthority(true)
		decision.Effects = []LifecycleEffectPlan{eject, {EffectID: ensure.EffectID, Ordinal: 1, Kind: ensure.Kind, SchemaVersion: 1}}
	}
	return decision
}

func r22SubjectAuthority(moderator bool) LifecycleAuthority {
	authority := LifecycleAuthority{
		SpaceAccessEpoch: 41, SubjectRolePolicyEpoch: 42,
		AuthorizationDigest: r22StoreDigest(0x20), SubjectGrants: r22AllGrants(),
	}
	if moderator {
		authority.ActorSourceRolePolicyEpoch = r22Pointer[int64](43)
		authority.ActorDestinationRolePolicyEpoch = r22Pointer[int64](44)
		authority.ActorCanMoveSource = r22Pointer(true)
		authority.ActorCanMoveDestination = r22Pointer(true)
	}
	return authority
}

func (fixture *r22StoreFixture) seedMembership(t *testing.T, latestGrantExpiresAt *time.Time) {
	t.Helper()
	fixture.sourceRoomID = r22InsertRoom(t, fixture.ctx, fixture.pool, fixture.spaceID, fixture.sourceVoiceRoomID,
		"r22-store-source-"+uuid.NewString(), "active", 3, nil)
	r22InsertMembership(t, fixture.ctx, fixture.pool, fixture.subjectProfileID, fixture.sourceRoomID, fixture.sourceMediaEpoch, r22ValidGrants)
	if latestGrantExpiresAt != nil {
		_, err := fixture.pool.Exec(fixture.ctx, `
UPDATE voice_room_memberships SET latest_grant_expires_at=$1 WHERE profile_id=$2`, latestGrantExpiresAt, fixture.subjectProfileID)
		require.NoError(t, err)
	}
}

func r22AllTableSnapshots(t *testing.T, fixture r22StoreFixture) map[string][]string {
	t.Helper()
	snapshots := make(map[string][]string, len(r22VoiceTables))
	for _, table := range r22VoiceTables {
		snapshots[table] = r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, table)
	}
	return snapshots
}

func r22InstallFailureAfterInsert(t *testing.T, fixture r22StoreFixture, table string) func() {
	t.Helper()
	require.Contains(t, []string{"voice_lifecycle_operations", "voice_media_epoch_denials", "voice_event_outbox"}, table)
	functionName := "r22_fail_after_" + table
	triggerName := functionName + "_trigger"
	tableSQL := pgx.Identifier{table}.Sanitize()
	functionSQL := pgx.Identifier{functionName}.Sanitize()
	triggerSQL := pgx.Identifier{triggerName}.Sanitize()
	_, err := fixture.pool.Exec(fixture.ctx, `
CREATE FUNCTION `+functionSQL+`() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='r22 forced rollback after insert'; END $$;
CREATE TRIGGER `+triggerSQL+` AFTER INSERT ON `+tableSQL+`
FOR EACH ROW EXECUTE FUNCTION `+functionSQL+`()`)
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, `DROP TRIGGER `+triggerSQL+` ON `+tableSQL+`; DROP FUNCTION `+functionSQL+`()`)
		require.NoError(t, dropErr)
	}
}

func r22InstallMembershipUpdateSkip(t *testing.T, fixture r22StoreFixture) func() {
	t.Helper()
	_, err := fixture.pool.Exec(fixture.ctx, `
CREATE FUNCTION r22_skip_membership_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RETURN NULL; END $$;
CREATE TRIGGER r22_skip_membership_update_trigger BEFORE UPDATE ON voice_room_memberships
FOR EACH ROW EXECUTE FUNCTION r22_skip_membership_update()`)
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, `
DROP TRIGGER r22_skip_membership_update_trigger ON voice_room_memberships;
DROP FUNCTION r22_skip_membership_update()`)
		require.NoError(t, dropErr)
	}
}

func r22InstallMembershipDeleteFailure(t *testing.T, fixture r22StoreFixture) func() {
	t.Helper()
	_, err := fixture.pool.Exec(fixture.ctx, `
CREATE FUNCTION r22_fail_membership_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='move must not delete membership'; END $$;
CREATE TRIGGER r22_fail_membership_delete_trigger BEFORE DELETE ON voice_room_memberships
FOR EACH ROW EXECUTE FUNCTION r22_fail_membership_delete()`)
	require.NoError(t, err)
	return func() {
		_, dropErr := fixture.pool.Exec(fixture.ctx, `
DROP TRIGGER r22_fail_membership_delete_trigger ON voice_room_memberships;
DROP FUNCTION r22_fail_membership_delete()`)
		require.NoError(t, dropErr)
	}
}

func TestPostgresLifecycleStore_C01_FailsClosed(t *testing.T) {
	t.Run("nil receiver and pool", func(t *testing.T) {
		var nilStore *PostgresLifecycleStore
		require.ErrorIs(t, nilStore.CheckSchema(context.Background()), ErrUnavailable)
		require.ErrorIs(t, NewPostgresLifecycleStore(nil).CheckSchema(context.Background()), ErrUnavailable)
	})
	if testing.Short() {
		return
	}
	t.Run("absent schema", func(t *testing.T) {
		ctx := context.Background()
		pool := integrationtest.StartPostgres(t, ctx, "r22storec01absent", "")
		require.ErrorIs(t, NewPostgresLifecycleStore(pool).CheckSchema(ctx), ErrUnavailable)
	})
	t.Run("closed pool", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec01closed")
		fixture.pool.Close()
		require.ErrorIs(t, fixture.store.CheckSchema(fixture.ctx), ErrUnavailable)
	})
	t.Run("migrated schema control", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec01ready")
		require.NoError(t, fixture.store.CheckSchema(fixture.ctx))
	})
}

type r22ReceiptGolden struct {
	name      string
	receipt   LifecycleReceipt
	hex       string
	hash      string
	forbidden func(*LifecycleReceipt)
}

func r22ReceiptGoldens(t *testing.T) []r22ReceiptGolden {
	t.Helper()
	operationID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actorID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	subjectID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	spaceID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	sourceVoiceID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	destinationVoiceID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	roomID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	mediaEpoch := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	sourceRoster, destinationRoster := int64(9), int64(10)
	spaceEpoch, roleEpoch := int64(11), int64(12)
	digest := r22StoreDigest(0xa0)
	base := LifecycleReceipt{OperationID: operationID, ActorProfileID: actorID, SubjectProfileID: subjectID, SpaceID: spaceID}
	join := base
	join.Method, join.Outcome = LifecycleMethodJoin, LifecycleOutcomeJoined
	join.DestinationVoiceRoomID, join.RoomID = &destinationVoiceID, &roomID
	join.DestinationRosterVersion, join.MediaEpoch = &destinationRoster, &mediaEpoch
	join.SpaceAccessEpoch, join.RolePolicyEpoch, join.AuthorizationDigest = &spaceEpoch, &roleEpoch, &digest
	joinNoOp := join
	joinNoOp.Outcome = LifecycleOutcomeNoOp
	leave := base
	leave.Method, leave.Outcome = LifecycleMethodLeave, LifecycleOutcomeLeft
	leave.SourceVoiceRoomID, leave.RoomID, leave.SourceRosterVersion = &sourceVoiceID, &roomID, &sourceRoster
	selfMove := join
	selfMove.Method, selfMove.Outcome = LifecycleMethodSelfMove, LifecycleOutcomeMoved
	selfMove.SourceVoiceRoomID, selfMove.SourceRosterVersion = &sourceVoiceID, &sourceRoster
	moderatorMove := selfMove
	moderatorMove.Method = LifecycleMethodModeratorMove
	leaveNoOp := base
	leaveNoOp.Method, leaveNoOp.Outcome = LifecycleMethodLeave, LifecycleOutcomeNoOp
	return []r22ReceiptGolden{
		{"join joined", join, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d34343434343434343434343428013001422436363636363636362d363636362d363636362d363636362d3636363636363636363636364a2437373737373737372d373737372d373737372d373737372d373737373737373737373737580a622438383838383838382d383838382d383838382d383838382d383838383838383838383838680b700c7a20a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf", "f08d66769eb35f1b9e43979f6d38272b7034326b414ce96979e91017e3956344", func(receipt *LifecycleReceipt) { receipt.SourceVoiceRoomID = &sourceVoiceID }},
		{"join no op", joinNoOp, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d34343434343434343434343428013004422436363636363636362d363636362d363636362d363636362d3636363636363636363636364a2437373737373737372d373737372d373737372d373737372d373737373737373737373737580a622438383838383838382d383838382d383838382d383838382d383838383838383838383838680b700c7a20a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf", "1947346c0a16ebac83ec27fc9cac10da48598af527e6e3bcad429655a091e0c2", func(receipt *LifecycleReceipt) { receipt.SourceRosterVersion = &sourceRoster }},
		{"leave left", leave, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d343434343434343434343434280230023a2435353535353535352d353535352d353535352d353535352d3535353535353535353535354a2437373737373737372d373737372d373737372d373737372d3737373737373737373737375009", "48a4494248c12b36702b75a8e22b2af3921028fb2ece3511a7e8ef026c4f1039", func(receipt *LifecycleReceipt) { receipt.MediaEpoch = &mediaEpoch }},
		{"self move moved", selfMove, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d343434343434343434343434280330033a2435353535353535352d353535352d353535352d353535352d353535353535353535353535422436363636363636362d363636362d363636362d363636362d3636363636363636363636364a2437373737373737372d373737372d373737372d373737372d3737373737373737373737375009580a622438383838383838382d383838382d383838382d383838382d383838383838383838383838680b700c7a20a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf", "f5ed9256216b0cc392449d639abfd425bd62a7ee23fd5d779afde673d8bc4918", func(receipt *LifecycleReceipt) { receipt.DestinationVoiceRoomID = nil }},
		{"moderator move moved", moderatorMove, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d343434343434343434343434280430033a2435353535353535352d353535352d353535352d353535352d353535353535353535353535422436363636363636362d363636362d363636362d363636362d3636363636363636363636364a2437373737373737372d373737372d373737372d373737372d3737373737373737373737375009580a622438383838383838382d383838382d383838382d383838382d383838383838383838383838680b700c7a20a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf", "b304abf873283cfe83cfd10d208e2e1daab9094fd485ae88d98c6716fb43ed69", func(receipt *LifecycleReceipt) { receipt.SourceVoiceRoomID = nil }},
		{"leave no op", leaveNoOp, "0a2431313131313131312d313131312d313131312d313131312d313131313131313131313131122432323232323232322d323232322d323232322d323232322d3232323232323232323232321a2433333333333333332d333333332d333333332d333333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d343434342d34343434343434343434343428023004", "3010127541cf5b99bd54659209c405e3df6157fbbce655391bd92437184b3c94", func(receipt *LifecycleReceipt) { receipt.RoomID = &roomID }},
	}
}

func TestPostgresLifecycleStore_C02_ReceiptEncoderGoldens(t *testing.T) {
	for _, golden := range r22ReceiptGoldens(t) {
		t.Run(golden.name, func(t *testing.T) {
			encoded, digest, err := EncodeLifecycleReceipt(golden.receipt)
			require.NoError(t, err)
			require.Equal(t, golden.hex, hex.EncodeToString(encoded))
			require.Equal(t, golden.hash, hex.EncodeToString(digest[:]))

			var message callsv1.VoiceRoomLifecycleReceipt
			require.NoError(t, proto.Unmarshal(encoded, &message))
			r22RequireReceiptMessageMatches(t, golden.receipt, &message)

			invalid := golden.receipt
			golden.forbidden(&invalid)
			_, _, invalidErr := EncodeLifecycleReceipt(invalid)
			require.Error(t, invalidErr)
		})
	}
}

func r22RequireReceiptMessageMatches(t *testing.T, receipt LifecycleReceipt, message *callsv1.VoiceRoomLifecycleReceipt) {
	t.Helper()
	require.Equal(t, receipt.OperationID.String(), message.OperationId)
	require.Equal(t, receipt.ActorProfileID.String(), message.ActorProfileId)
	require.Equal(t, receipt.SubjectProfileID.String(), message.SubjectProfileId)
	require.NotNil(t, message.Space)
	require.Equal(t, receipt.SpaceID.String(), message.Space.Id)
	require.Equal(t, map[LifecycleMethod]callsv1.VoiceRoomLifecycleMethod{
		LifecycleMethodJoin:          callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_JOIN,
		LifecycleMethodLeave:         callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_LEAVE,
		LifecycleMethodSelfMove:      callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE,
		LifecycleMethodModeratorMove: callsv1.VoiceRoomLifecycleMethod_VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE,
	}[receipt.Method], message.Method)
	require.Equal(t, map[LifecycleOutcome]callsv1.VoiceRoomLifecycleOutcome{
		LifecycleOutcomeJoined: callsv1.VoiceRoomLifecycleOutcome_VOICE_ROOM_LIFECYCLE_OUTCOME_JOINED,
		LifecycleOutcomeLeft:   callsv1.VoiceRoomLifecycleOutcome_VOICE_ROOM_LIFECYCLE_OUTCOME_LEFT,
		LifecycleOutcomeMoved:  callsv1.VoiceRoomLifecycleOutcome_VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED,
		LifecycleOutcomeNoOp:   callsv1.VoiceRoomLifecycleOutcome_VOICE_ROOM_LIFECYCLE_OUTCOME_NO_OP,
	}[receipt.Outcome], message.Outcome)
	require.Equal(t, receipt.SourceVoiceRoomID != nil, message.SourceVoiceRoomId != nil)
	require.Equal(t, receipt.DestinationVoiceRoomID != nil, message.DestinationVoiceRoomId != nil)
	require.Equal(t, receipt.RoomID != nil, message.RoomId != nil)
	require.Equal(t, receipt.SourceRosterVersion != nil, message.SourceRosterVersion != nil)
	require.Equal(t, receipt.DestinationRosterVersion != nil, message.DestinationRosterVersion != nil)
	require.Equal(t, receipt.MediaEpoch != nil, message.MediaEpoch != nil)
	require.Equal(t, receipt.SpaceAccessEpoch != nil, message.SpaceAccessEpoch != nil)
	require.Equal(t, receipt.RolePolicyEpoch != nil, message.RolePolicyEpoch != nil)
	require.Equal(t, receipt.AuthorizationDigest != nil, message.AuthorizationDigest != nil)
	if receipt.SourceVoiceRoomID != nil {
		require.Equal(t, receipt.SourceVoiceRoomID.String(), message.GetSourceVoiceRoomId())
	}
	if receipt.DestinationVoiceRoomID != nil {
		require.Equal(t, receipt.DestinationVoiceRoomID.String(), message.GetDestinationVoiceRoomId())
	}
	if receipt.RoomID != nil {
		require.Equal(t, receipt.RoomID.String(), message.GetRoomId())
	}
	if receipt.SourceRosterVersion != nil {
		require.Equal(t, uint64(*receipt.SourceRosterVersion), message.GetSourceRosterVersion())
	}
	if receipt.DestinationRosterVersion != nil {
		require.Equal(t, uint64(*receipt.DestinationRosterVersion), message.GetDestinationRosterVersion())
	}
	if receipt.MediaEpoch != nil {
		require.Equal(t, receipt.MediaEpoch.String(), message.GetMediaEpoch())
	}
	if receipt.SpaceAccessEpoch != nil {
		require.Equal(t, uint64(*receipt.SpaceAccessEpoch), message.GetSpaceAccessEpoch())
	}
	if receipt.RolePolicyEpoch != nil {
		require.Equal(t, uint64(*receipt.RolePolicyEpoch), message.GetRolePolicyEpoch())
	}
	if receipt.AuthorizationDigest != nil {
		require.Equal(t, receipt.AuthorizationDigest[:], message.GetAuthorizationDigest())
	}
}

func TestPostgresLifecycleStore_C03_DecisionRollbackLeavesNoResidue(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec03")
	before := r22AllTableSnapshots(t, fixture)
	_, err := fixture.pool.Exec(fixture.ctx, `
CREATE FUNCTION r22_fail_effect_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION USING ERRCODE='P0001', MESSAGE='r22 forced decision rollback'; END $$;
CREATE TRIGGER r22_fail_effect_insert BEFORE INSERT ON voice_lifecycle_effects
FOR EACH ROW EXECUTE FUNCTION r22_fail_effect_insert()`)
	require.NoError(t, err)
	_, err = fixture.store.DecideOperation(fixture.ctx, fixture.decision(LifecycleMethodJoin))
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrUnavailable), "decision must reach the injected in-transaction failure")
	require.Equal(t, before, r22AllTableSnapshots(t, fixture))
}

func TestPostgresLifecycleStore_C04_OperationBindingIsPermanent(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	states := []string{"decided", "completed", "quarantined"}
	for _, state := range states {
		t.Run(state, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec04"+state)
			decision := fixture.decision(LifecycleMethodJoin)
			first, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			r22MoveOperationToState(t, fixture, decision, state)
			replayed, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			require.Equal(t, first.ActorProfileID, replayed.ActorProfileID)
			require.Equal(t, first.OperationID, replayed.OperationID)
			require.Equal(t, first.Fingerprint, replayed.Fingerprint)
			require.Equal(t, first.BindingBytes, replayed.BindingBytes)

			changedMethod := decision
			changedMethod.Method = LifecycleMethodLeave
			changedMethod.SourceVoiceRoomID, changedMethod.DestinationVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID), nil
			changedMethod.Authority = LifecycleAuthority{}
			changedMethod.Effects = []LifecycleEffectPlan{{
				EffectID: decision.Effects[0].EffectID, Ordinal: 0,
				Kind: LifecycleEffectEjectParticipant, SchemaVersion: 1,
			}}
			_, err = fixture.store.DecideOperation(fixture.ctx, changedMethod)
			require.ErrorIs(t, err, ErrOperationConflict)
			changedBody := decision
			changedBody.DestinationVoiceRoomID = r22Pointer(uuid.New())
			_, err = fixture.store.DecideOperation(fixture.ctx, changedBody)
			require.ErrorIs(t, err, ErrOperationConflict)
		})
	}
}

func r22MoveOperationToState(t *testing.T, fixture r22StoreFixture, decision LifecycleDecision, state string) {
	t.Helper()
	if state == "decided" {
		return
	}
	claim, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	if state == "quarantined" {
		require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{
			ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
			WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "test_quarantine", At: r22StoreTime.Add(time.Minute),
		}))
		return
	}
	effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	for range effects {
		effectClaim, claimed, claimErr := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, claimErr)
		require.True(t, claimed)
		require.NoError(t, fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{
			EffectID: effectClaim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: effectClaim.Fence,
			Observed: effectClaim.Effect, AppliedAt: r22StoreTime.Add(time.Minute),
		}))
	}
	receipt := r22JoinReceipt(fixture, decision, 1)
	_, err = fixture.store.CompleteOperation(fixture.ctx, LifecycleCompletion{
		ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt,
		CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour),
	})
	require.NoError(t, err)
}

func r22JoinReceipt(fixture r22StoreFixture, decision LifecycleDecision, roster int64) LifecycleReceipt {
	operation, _, _ := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	return LifecycleReceipt{
		OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID,
		Method: decision.Method, Outcome: LifecycleOutcomeJoined,
		DestinationVoiceRoomID: decision.DestinationVoiceRoomID, RoomID: operation.DestinationRoomID,
		DestinationRosterVersion: &roster, MediaEpoch: operation.DestinationMediaEpoch,
		SpaceAccessEpoch:    &decision.Authority.SpaceAccessEpoch,
		RolePolicyEpoch:     &decision.Authority.SubjectRolePolicyEpoch,
		AuthorizationDigest: &decision.Authority.AuthorizationDigest,
	}
}

func TestPostgresLifecycleStore_C05_DecisionPersistsPlanAndOwnerToken(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, method := range []LifecycleMethod{LifecycleMethodJoin, LifecycleMethodLeave, LifecycleMethodSelfMove, LifecycleMethodModeratorMove} {
		t.Run(map[LifecycleMethod]string{
			LifecycleMethodJoin: "join", LifecycleMethodLeave: "leave",
			LifecycleMethodSelfMove: "self move", LifecycleMethodModeratorMove: "moderator move",
		}[method], func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec05"+uuid.NewString()[:6])
			if method != LifecycleMethodJoin {
				fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
			}
			decision := fixture.decision(method)
			operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			r22RequireOperationDecision(t, decision, operation)
			reloaded, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			require.NoError(t, err)
			require.True(t, found)
			r22RequireOperationDecision(t, decision, reloaded)
			r22RequireSameImmutableOperation(t, operation, reloaded)
			require.Equal(t, decision.ActorAccountID, operation.ActorAccountID)
			require.Equal(t, decision.ActorProfileID, operation.ActorProfileID)
			require.Equal(t, decision.OperationID, operation.OperationID)
			require.Equal(t, decision.SubjectProfileID, operation.SubjectProfileID)
			require.Equal(t, decision.SpaceID, operation.SpaceID)
			require.Equal(t, decision.Method, operation.Method)
			require.Equal(t, decision.SourceVoiceRoomID, operation.SourceVoiceRoomID)
			require.Equal(t, decision.DestinationVoiceRoomID, operation.DestinationVoiceRoomID)
			require.Equal(t, decision.Authority, operation.Authority)
			require.Equal(t, decision.RedisOwnerToken, operation.RedisOwnerToken)
			require.Equal(t, LifecycleOperationDecided, operation.State)
			require.NotEqual(t, LifecycleDigest{}, operation.Fingerprint)
			require.NotEmpty(t, operation.BindingBytes)
			var sourceRoom, destinationRoom *LifecycleRoomSnapshot
			if decision.SourceVoiceRoomID != nil {
				require.NotNil(t, operation.SourceRoomID)
				require.Equal(t, fixture.sourceMediaEpoch, *operation.SourceMediaEpoch)
				loaded, found, loadErr := fixture.store.LoadRoomSnapshot(fixture.ctx, *operation.SourceRoomID)
				require.NoError(t, loadErr)
				require.True(t, found)
				require.Equal(t, decision.SpaceID, loaded.SpaceID)
				require.Equal(t, *decision.SourceVoiceRoomID, loaded.VoiceRoomID)
				sourceRoom = &loaded
			} else {
				require.Nil(t, operation.SourceRoomID)
				require.Nil(t, operation.SourceMediaEpoch)
			}
			if decision.DestinationVoiceRoomID != nil {
				require.NotNil(t, operation.DestinationRoomID)
				require.NotNil(t, operation.DestinationMediaEpoch)
				loaded, found, loadErr := fixture.store.LoadRoomSnapshot(fixture.ctx, *operation.DestinationRoomID)
				require.NoError(t, loadErr)
				require.True(t, found)
				require.Equal(t, decision.SpaceID, loaded.SpaceID)
				require.Equal(t, *decision.DestinationVoiceRoomID, loaded.VoiceRoomID)
				destinationRoom = &loaded
			} else {
				require.Nil(t, operation.DestinationRoomID)
				require.Nil(t, operation.DestinationMediaEpoch)
			}

			effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
			require.NoError(t, err)
			require.Len(t, effects, len(decision.Effects))
			for index, effect := range effects {
				wantPlan := decision.Effects[index]
				require.Equal(t, wantPlan.EffectID, effect.EffectID)
				require.Equal(t, decision.ActorProfileID, effect.ActorProfileID)
				require.Equal(t, decision.OperationID, effect.OperationID)
				require.Equal(t, int16(index), effect.Ordinal)
				require.Equal(t, wantPlan.Kind, effect.Kind)
				require.Equal(t, int16(1), effect.SchemaVersion)
				require.NotEmpty(t, effect.LiveKitRoomName)
				require.NotEmpty(t, effect.RequestBytes)
				require.Equal(t, LifecycleDigest(sha256.Sum256(effect.RequestBytes)), effect.RequestDigest)
				require.Equal(t, LifecycleEffectStateReady, effect.State)
				require.Zero(t, effect.AttemptCount)
				require.Nil(t, effect.LastErrorClass)
				require.Nil(t, effect.LastErrorAt)
				require.Equal(t, decision.DecidedAt, effect.NextAttemptAt)
				require.Nil(t, effect.LeaseOwner)
				require.Nil(t, effect.LeaseUntil)
				require.Zero(t, effect.LeaseFence)
				require.Nil(t, effect.AppliedAt)
				require.Nil(t, effect.QuarantineClass)
				require.Nil(t, effect.QuarantineDetail)
				require.Nil(t, effect.QuarantinedAt)
				if effect.Kind == LifecycleEffectEnsureRoom {
					require.Equal(t, *decision.DestinationVoiceRoomID, effect.VoiceRoomID)
					require.Equal(t, destinationRoom.LiveKitRoomName, effect.LiveKitRoomName)
					require.Nil(t, effect.TargetProfileID)
					require.Nil(t, effect.ParticipantIdentity)
					require.Nil(t, effect.MediaEpoch)
				} else {
					require.Equal(t, *decision.SourceVoiceRoomID, effect.VoiceRoomID)
					require.Equal(t, sourceRoom.LiveKitRoomName, effect.LiveKitRoomName)
					require.Equal(t, decision.SubjectProfileID, *effect.TargetProfileID)
					require.Equal(t, fixture.sourceMediaEpoch, *effect.MediaEpoch)
					require.Equal(t, "profile:"+decision.SubjectProfileID.String()+":media:"+fixture.sourceMediaEpoch.String(), *effect.ParticipantIdentity)
				}
			}
		})
	}
}

func TestPostgresLifecycleStore_C06_JoinDecisionDoesNotMutateRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec06")
	destinationRoomID := r22InsertRoom(t, fixture.ctx, fixture.pool, fixture.spaceID, fixture.destinationVoiceRoomID,
		deterministicLiveKitRoomName(fixture.destinationVoiceRoomID), "active", 7, nil)
	before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_instances")
	_, err := fixture.store.DecideOperation(fixture.ctx, fixture.decision(LifecycleMethodJoin))
	require.NoError(t, err)
	require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_instances"))
	_, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.False(t, found)
	room, found, err := fixture.store.LoadRoomSnapshot(fixture.ctx, destinationRoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(7), room.RosterVersion)

	t.Run("existing destination requires deterministic LiveKit name", func(t *testing.T) {
		wrong := r22NewStoreFixture(t, "r22storec06wrongname")
		r22InsertRoom(t, wrong.ctx, wrong.pool, wrong.spaceID, wrong.destinationVoiceRoomID,
			"wrong-livekit-name", "active", 0, nil)
		before := r22AllTableSnapshots(t, wrong)
		_, decideErr := wrong.store.DecideOperation(wrong.ctx, wrong.decision(LifecycleMethodJoin))
		require.ErrorIs(t, decideErr, ErrInvariant)
		require.Equal(t, before, r22AllTableSnapshots(t, wrong))
	})
}

func TestPostgresLifecycleStore_C07_DenialFollowsCommittedBearerExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, method := range []LifecycleMethod{LifecycleMethodLeave, LifecycleMethodSelfMove} {
		for _, withExpiry := range []bool{false, true} {
			name := map[LifecycleMethod]string{LifecycleMethodLeave: "leave", LifecycleMethodSelfMove: "move"}[method] + "/without-expiry"
			if withExpiry {
				name = map[LifecycleMethod]string{LifecycleMethodLeave: "leave", LifecycleMethodSelfMove: "move"}[method] + "/with-expiry"
			}
			t.Run(name, func(t *testing.T) {
				fixture := r22NewStoreFixture(t, "r22storec07"+name+uuid.NewString()[:6])
				var expiry *time.Time
				if withExpiry {
					var databaseNow time.Time
					require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT clock_timestamp()`).Scan(&databaseNow))
					expiry = r22Pointer(databaseNow.UTC().Add(-10 * time.Minute))
				}
				fixture.seedMembership(t, expiry)
				decision := fixture.decision(method)
				_, err := fixture.store.DecideOperation(fixture.ctx, decision)
				require.NoError(t, err)
				effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
				require.NoError(t, err)
				require.NotEmpty(t, effects)
				require.Equal(t, LifecycleEffectEjectParticipant, effects[0].Kind)
				require.Equal(t, fixture.sourceMediaEpoch, *effects[0].MediaEpoch)
				require.Equal(t, "profile:"+fixture.subjectProfileID.String()+":media:"+fixture.sourceMediaEpoch.String(), *effects[0].ParticipantIdentity)
				sourceRoom, sourceFound, err := fixture.store.LoadRoomSnapshot(fixture.ctx, fixture.sourceRoomID)
				require.NoError(t, err)
				require.True(t, sourceFound)
				require.Equal(t, sourceRoom.LiveKitRoomName, effects[0].LiveKitRoomName)
				denial, found, err := fixture.store.LookupMediaEpochDenial(fixture.ctx, fixture.sourceMediaEpoch)
				require.NoError(t, err)
				require.Equal(t, withExpiry, found)
				if withExpiry {
					require.Equal(t, fixture.sourceMediaEpoch, denial.MediaEpoch)
					require.Equal(t, fixture.subjectProfileID, denial.ProfileID)
					require.Equal(t, fixture.sourceRoomID, denial.RoomID)
					require.Equal(t, sourceRoom.LiveKitRoomName, denial.LiveKitRoomName)
					require.Equal(t, effects[0].LiveKitRoomName, denial.LiveKitRoomName)
					require.Equal(t, "profile:"+fixture.subjectProfileID.String()+":media:"+fixture.sourceMediaEpoch.String(), denial.ParticipantIdentity)
					require.Equal(t, *expiry, denial.GrantExpiresAt)
					require.Equal(t, decision.AcceptedClockSkew, denial.AcceptedClockSkew)
					require.Equal(t, expiry.Add(decision.AcceptedClockSkew), denial.DenyUntil)
					require.NotEmpty(t, denial.Reason)
					require.Equal(t, decision.ActorProfileID, *denial.ActorProfileID)
					require.Equal(t, decision.OperationID, *denial.OperationID)
					require.Nil(t, denial.AbsenceObservedAt)
					require.False(t, denial.CreatedAt.IsZero())
					require.Equal(t, denial.CreatedAt, denial.UpdatedAt)
					observedAt := denial.DenyUntil.Add(-time.Second)
					require.NoError(t, fixture.store.MarkDenialAbsenceObserved(fixture.ctx, fixture.sourceMediaEpoch, observedAt))
					deleted, deleteErr := fixture.store.DeleteExpiredObservedDenials(fixture.ctx, denial.DenyUntil.Add(-time.Nanosecond), 10)
					require.NoError(t, deleteErr)
					require.Zero(t, deleted)
					deleted, deleteErr = fixture.store.DeleteExpiredObservedDenials(fixture.ctx, denial.DenyUntil, 10)
					require.NoError(t, deleteErr)
					require.Equal(t, int64(1), deleted)
				}
			})
		}
	}
}

func TestPostgresLifecycleStore_C07_DenialAndDecisionRollbackTogether(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, method := range []LifecycleMethod{LifecycleMethodLeave, LifecycleMethodSelfMove} {
		t.Run(map[LifecycleMethod]string{LifecycleMethodLeave: "leave", LifecycleMethodSelfMove: "move"}[method], func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec07rollback"+uuid.NewString()[:6])
			fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(10*time.Minute)))
			before := r22AllTableSnapshots(t, fixture)
			drop := r22InstallFailureAfterInsert(t, fixture, "voice_media_epoch_denials")
			_, err := fixture.store.DecideOperation(fixture.ctx, fixture.decision(method))
			require.Error(t, err)
			require.False(t, errors.Is(err, ErrUnavailable))
			require.Equal(t, before, r22AllTableSnapshots(t, fixture))
			drop()

			_, err = fixture.store.DecideOperation(fixture.ctx, fixture.decision(method))
			require.NoError(t, err, "control proves the same fully valid decision commits without the injected failure")
			require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials"))
		})
	}
}

func TestPostgresLifecycleStore_C08_NonterminalSubjectFencesDecisionAndGrant(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, state := range []string{"decided", "quarantined"} {
		t.Run(state, func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec08"+state)
			fixture.seedMembership(t, nil)
			decision := fixture.decision(LifecycleMethodLeave)
			_, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			if state == "quarantined" {
				claim, ok, claimErr := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
				require.NoError(t, claimErr)
				require.True(t, ok)
				require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{
					ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
					WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "manual_repair", At: r22StoreTime,
				}))
			}
			other := fixture.decision(LifecycleMethodJoin)
			other.OperationID = uuid.New()
			_, err = fixture.store.DecideOperation(fixture.ctx, other)
			require.ErrorIs(t, err, ErrSubjectTransitionActive)
			_, err = fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, r22StoreTime.Add(time.Hour))
			require.ErrorIs(t, err, ErrSubjectTransitionActive)
			membership, found, loadErr := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
			require.NoError(t, loadErr)
			require.True(t, found)
			require.Nil(t, membership.LatestGrantExpiresAt)
		})
	}
	for _, state := range []string{"decided", "quarantined"} {
		for _, noOpMethod := range []LifecycleMethod{LifecycleMethodJoin, LifecycleMethodLeave} {
			name := map[LifecycleMethod]string{LifecycleMethodJoin: "join no-op", LifecycleMethodLeave: "absent leave no-op"}[noOpMethod]
			t.Run(state+" fences "+name, func(t *testing.T) {
				fixture := r22NewStoreFixture(t, "r22storec08noop"+uuid.NewString()[:6])
				blockingMethod := LifecycleMethodJoin
				if noOpMethod == LifecycleMethodJoin {
					fixture.seedMembership(t, nil)
					blockingMethod = LifecycleMethodLeave
				}
				blocking := fixture.decision(blockingMethod)
				_, err := fixture.store.DecideOperation(fixture.ctx, blocking)
				require.NoError(t, err)
				if state == "quarantined" {
					claim := r22ClaimOperation(t, fixture)
					require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{
						ActorProfileID: blocking.ActorProfileID, OperationID: blocking.OperationID,
						WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "manual_repair", At: r22StoreTime,
					}))
				}
				noOpDecision := fixture.decision(noOpMethod)
				noOpDecision.OperationID = uuid.New()
				if noOpMethod == LifecycleMethodJoin {
					noOpDecision.DestinationVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID)
				} else {
					noOpDecision.SourceVoiceRoomID = r22Pointer(uuid.New())
				}
				before := r22AllTableSnapshots(t, fixture)
				_, err = fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{
					Decision: noOpDecision, CompletedAt: r22StoreTime.Add(time.Minute), ReplayUntil: r22StoreTime.Add(25 * time.Hour),
				})
				require.ErrorIs(t, err, ErrSubjectTransitionActive)
				require.Equal(t, before, r22AllTableSnapshots(t, fixture), "a fenced no-op must leave no durable residue")
			})
		}
	}
}

func TestPostgresLifecycleStore_C09_OperationClaimSkipsLeaseAndReclaimsWithGreaterFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec09")
	decision := fixture.decision(LifecycleMethodJoin)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	first, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	_, ok, err = fixture.store.ClaimNextOperation(fixture.ctx, uuid.New(), r22StoreTime.Add(30*time.Second), time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "an unexpired lease must be skipped")
	secondWorker := uuid.New()
	second, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, secondWorker, r22StoreTime.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, second.Fence, first.Fence)
	require.Equal(t, secondWorker, second.WorkerID)
	require.Equal(t, operation.Fingerprint, second.Operation.Fingerprint)
	require.Equal(t, operation.BindingBytes, second.Operation.BindingBytes)
	require.Equal(t, operation.RedisOwnerToken, second.Operation.RedisOwnerToken)
}

func TestPostgresLifecycleStore_C10_StaleOperationFenceCannotMutate(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec10")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	old, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	newWorker := uuid.New()
	current, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, newWorker, r22StoreTime.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, current.Fence, old.Fence)
	receipt := r22JoinReceipt(fixture, decision, 1)
	before := r22AllTableSnapshots(t, fixture)
	staleMutations := []func() error{
		func() error {
			return fixture.store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, LeaseUntil: r22StoreTime.Add(3 * time.Minute)})
		},
		func() error {
			return fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, Class: "late", At: r22StoreTime.Add(2 * time.Minute)})
		},
		func() error {
			_, callErr := fixture.store.CompleteOperation(fixture.ctx, LifecycleCompletion{
				ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
				WorkerID: fixture.workerID, ExpectedFence: old.Fence, Receipt: receipt,
				CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour),
			})
			return callErr
		},
	}
	for _, mutate := range staleMutations {
		require.ErrorIs(t, mutate(), ErrStaleLease)
		require.Equal(t, before, r22AllTableSnapshots(t, fixture))
	}
}

func TestPostgresLifecycleStore_C11_EffectAppliedRequiresExactObservationAndIsIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec11")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	mutations := map[string]func(*LifecycleEffect){
		"effect id":            func(effect *LifecycleEffect) { effect.EffectID = uuid.New() },
		"actor profile":        func(effect *LifecycleEffect) { effect.ActorProfileID = uuid.New() },
		"operation":            func(effect *LifecycleEffect) { effect.OperationID = uuid.New() },
		"ordinal":              func(effect *LifecycleEffect) { effect.Ordinal++ },
		"kind":                 func(effect *LifecycleEffect) { effect.Kind = LifecycleEffectEjectParticipant },
		"schema":               func(effect *LifecycleEffect) { effect.SchemaVersion++ },
		"voice room":           func(effect *LifecycleEffect) { effect.VoiceRoomID = uuid.New() },
		"livekit room":         func(effect *LifecycleEffect) { effect.LiveKitRoomName += "-changed" },
		"target profile":       func(effect *LifecycleEffect) { effect.TargetProfileID = r22Pointer(uuid.New()) },
		"participant identity": func(effect *LifecycleEffect) { effect.ParticipantIdentity = r22Pointer("changed") },
		"media epoch":          func(effect *LifecycleEffect) { effect.MediaEpoch = r22Pointer(uuid.New()) },
		"request bytes": func(effect *LifecycleEffect) {
			effect.RequestBytes = append(append([]byte(nil), effect.RequestBytes...), 0xff)
		},
		"request digest": func(effect *LifecycleEffect) { effect.RequestDigest[0] ^= 0xff },
		"created at":     func(effect *LifecycleEffect) { effect.CreatedAt = effect.CreatedAt.Add(time.Second) },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			changed := claim.Effect
			mutate(&changed)
			before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects")
			err = fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: changed, AppliedAt: r22StoreTime.Add(time.Minute)})
			require.ErrorIs(t, err, ErrInvariant)
			require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
		})
	}
	apply := LifecycleEffectApplied{EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute)}
	require.NoError(t, fixture.store.MarkEffectApplied(fixture.ctx, apply))
	beforeReplay := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects")
	require.NoError(t, fixture.store.MarkEffectApplied(fixture.ctx, apply), "same terminal observation must be idempotent")
	require.Equal(t, beforeReplay, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
	effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
	require.NoError(t, err)
	require.Len(t, effects, 1)
	require.Equal(t, LifecycleEffectStateApplied, effects[0].State)
	require.Equal(t, apply.AppliedAt, *effects[0].AppliedAt)
}

func TestPostgresLifecycleStore_C12_QuarantinedEffectBlocksCompletionAndSubject(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec12")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	opClaim, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	effectClaim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, fixture.store.QuarantineEffect(fixture.ctx, LifecycleEffectQuarantine{EffectID: effectClaim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: effectClaim.Fence, Class: "target_mismatch", At: r22StoreTime.Add(time.Minute)}))
	_, err = fixture.store.CompleteOperation(fixture.ctx, LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: opClaim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1), CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)})
	require.ErrorIs(t, err, ErrOperationNotCompletable)
	other := fixture.decision(LifecycleMethodJoin)
	other.OperationID = uuid.New()
	_, err = fixture.store.DecideOperation(fixture.ctx, other)
	require.ErrorIs(t, err, ErrSubjectTransitionActive)
}

func r22ApplyAllEffects(t *testing.T, fixture r22StoreFixture, decision LifecycleDecision) {
	t.Helper()
	for {
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		if !ok {
			return
		}
		require.NoError(t, fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{
			EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence,
			Observed: claim.Effect, AppliedAt: r22StoreTime.Add(time.Minute),
		}))
	}
}

func r22ClaimOperation(t *testing.T, fixture r22StoreFixture) LifecycleOperationClaim {
	t.Helper()
	claim, ok, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	return claim
}

func r22TestOutbox(fixture r22StoreFixture, operation LifecycleOperation, ordinal int16, payload string, roster int64) LifecycleOutboxRecord {
	roomID := operation.DestinationRoomID
	voiceRoomID := operation.DestinationVoiceRoomID
	if roomID == nil {
		roomID = operation.SourceRoomID
	}
	if voiceRoomID == nil {
		voiceRoomID = operation.SourceVoiceRoomID
	}
	return LifecycleOutboxRecord{EventID: uuid.New(), Ordinal: ordinal, Subject: "voice.test.r22.lifecycle.v1", SchemaVersion: 1,
		PayloadBytes: []byte(payload), SubjectProfileID: fixture.subjectProfileID, RoomID: *roomID,
		VoiceRoomID: *voiceRoomID, SpaceID: fixture.spaceID, RosterVersion: roster, NextAttemptAt: r22StoreTime}
}

func r22RequireCompletedReceipt(t *testing.T, fixture r22StoreFixture, returned LifecycleOperation, want LifecycleReceipt) {
	t.Helper()
	wantBytes, wantHash, err := EncodeLifecycleReceipt(want)
	require.NoError(t, err)
	require.Equal(t, LifecycleOperationCompleted, returned.State)
	require.NotNil(t, returned.Receipt)
	require.Equal(t, want, *returned.Receipt)
	require.Equal(t, wantBytes, returned.ReceiptBytes)
	require.NotNil(t, returned.ReceiptHash)
	require.Equal(t, wantHash, *returned.ReceiptHash)

	reloaded, found, err := fixture.store.LoadOperation(fixture.ctx, returned.ActorProfileID, returned.OperationID)
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, reloaded.Receipt)
	require.Equal(t, want, *reloaded.Receipt)
	require.Equal(t, wantBytes, reloaded.ReceiptBytes)
	require.NotNil(t, reloaded.ReceiptHash)
	require.Equal(t, wantHash, *reloaded.ReceiptHash)
}

func r22RequireTerminalMembership(t *testing.T, fixture r22StoreFixture, profileID, roomID, mediaEpoch uuid.UUID, authority LifecycleAuthority, completedAt time.Time) {
	t.Helper()
	membership, found, err := fixture.store.LoadMembership(fixture.ctx, profileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, profileID, membership.ProfileID)
	require.Equal(t, roomID, membership.RoomID)
	require.Equal(t, mediaEpoch, membership.MediaEpoch)
	require.Equal(t, authority.SpaceAccessEpoch, membership.SpaceAccessEpoch)
	require.Equal(t, authority.SubjectRolePolicyEpoch, membership.RolePolicyEpoch)
	require.Equal(t, authority.AuthorizationDigest, membership.AuthorizationDigest)
	r22RequireGrants(t, authority.SubjectGrants, membership.Grants)
	require.Nil(t, membership.LatestGrantExpiresAt)
	require.Equal(t, completedAt, membership.JoinedAt)
	require.Equal(t, completedAt, membership.UpdatedAt)
}

func r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t *testing.T, before, after LifecycleMembership) {
	t.Helper()
	want := before
	want.LatestGrantExpiresAt = after.LatestGrantExpiresAt
	want.UpdatedAt = after.UpdatedAt
	require.Equal(t, want, after)
	r22RequireGrants(t, before.Grants, after.Grants)
}

func r22MembershipImmutableRowSnapshot(t *testing.T, fixture r22StoreFixture) string {
	t.Helper()
	var snapshot string
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `
SELECT (to_jsonb(membership_row)-'latest_grant_expires_at'-'updated_at')::text
FROM voice_room_memberships AS membership_row
WHERE profile_id=$1`, fixture.subjectProfileID).Scan(&snapshot))
	return snapshot
}

func r22RequireOutboxRows(t *testing.T, fixture r22StoreFixture, completion LifecycleCompletion) {
	t.Helper()
	rows, err := fixture.pool.Query(fixture.ctx, `
SELECT event_id,actor_profile_id,operation_id,ordinal,subject,schema_version,payload_bytes,
       encode(payload_hash,'hex'),subject_profile_id,room_id,voice_room_id,space_id,roster_version,
       state,attempt_count,last_error_class,last_error_at,next_attempt_at,lease_owner,lease_until,
       lease_fence,delivered_at,quarantine_class,quarantine_detail,quarantined_at,created_at,updated_at
FROM voice_event_outbox
WHERE actor_profile_id=$1 AND operation_id=$2
ORDER BY ordinal`, completion.ActorProfileID, completion.OperationID)
	require.NoError(t, err)
	defer rows.Close()
	index := 0
	for rows.Next() {
		require.Less(t, index, len(completion.Outbox))
		want := completion.Outbox[index]
		var got LifecycleOutbox
		var payloadHashHex, state string
		require.NoError(t, rows.Scan(
			&got.EventID, &got.ActorProfileID, &got.OperationID, &got.Ordinal, &got.Subject, &got.SchemaVersion,
			&got.PayloadBytes, &payloadHashHex, &got.SubjectProfileID, &got.RoomID, &got.VoiceRoomID, &got.SpaceID,
			&got.RosterVersion, &state, &got.AttemptCount, &got.LastErrorClass, &got.LastErrorAt,
			&got.NextAttemptAt, &got.LeaseOwner, &got.LeaseUntil, &got.LeaseFence, &got.DeliveredAt,
			&got.QuarantineClass, &got.QuarantineDetail, &got.QuarantinedAt, &got.CreatedAt, &got.UpdatedAt,
		))
		require.Equal(t, want.EventID, got.EventID)
		require.Equal(t, completion.ActorProfileID, got.ActorProfileID)
		require.Equal(t, completion.OperationID, got.OperationID)
		require.Equal(t, want.Ordinal, got.Ordinal)
		require.Equal(t, want.Subject, got.Subject)
		require.Equal(t, want.SchemaVersion, got.SchemaVersion)
		require.Equal(t, want.PayloadBytes, got.PayloadBytes)
		wantHash := sha256.Sum256(want.PayloadBytes)
		require.Equal(t, hex.EncodeToString(wantHash[:]), payloadHashHex)
		require.Equal(t, want.SubjectProfileID, got.SubjectProfileID)
		require.Equal(t, want.RoomID, got.RoomID)
		require.Equal(t, want.VoiceRoomID, got.VoiceRoomID)
		require.Equal(t, want.SpaceID, got.SpaceID)
		require.Equal(t, want.RosterVersion, got.RosterVersion)
		require.Equal(t, "ready", state)
		require.Zero(t, got.AttemptCount)
		require.Nil(t, got.LastErrorClass)
		require.Nil(t, got.LastErrorAt)
		require.True(t, want.NextAttemptAt.Equal(got.NextAttemptAt), "next_attempt_at must be the exact expected instant")
		require.Nil(t, got.LeaseOwner)
		require.Nil(t, got.LeaseUntil)
		require.Zero(t, got.LeaseFence)
		require.Nil(t, got.DeliveredAt)
		require.Nil(t, got.QuarantineClass)
		require.Nil(t, got.QuarantineDetail)
		require.Nil(t, got.QuarantinedAt)
		require.False(t, got.CreatedAt.IsZero())
		require.Equal(t, got.CreatedAt, got.UpdatedAt)
		index++
	}
	require.NoError(t, rows.Err())
	require.Len(t, completion.Outbox, index)
}

func TestPostgresLifecycleStore_C13_JoinTerminalMutationIsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec13")
	decision := fixture.decision(LifecycleMethodJoin)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	receipt := r22JoinReceipt(fixture, decision, 1)
	outbox := r22TestOutbox(fixture, operation, 0, "joined", 1)
	before := r22AllTableSnapshots(t, fixture)
	drop := r22InstallFailureAfterInsert(t, fixture, "voice_event_outbox")
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt, Outbox: []LifecycleOutboxRecord{outbox},
		CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrUnavailable))
	require.Equal(t, before, r22AllTableSnapshots(t, fixture))
	drop()
	completed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	r22RequireCompletedReceipt(t, fixture, completed, receipt)
	r22RequireOutboxRows(t, fixture, completion)
	r22RequireTerminalMembership(t, fixture, fixture.subjectProfileID, *completed.DestinationRoomID, *completed.DestinationMediaEpoch, decision.Authority, completion.CompletedAt)
	membership, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, *completed.DestinationRoomID, membership.RoomID)
	room, found, err := fixture.store.LoadRoomSnapshot(fixture.ctx, membership.RoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(1), room.RosterVersion)
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
}

func TestPostgresLifecycleStore_C14_LeaveTerminalMutationIsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec14")
	fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
	decision := fixture.decision(LifecycleMethodLeave)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	sourceRoster := int64(4)
	receipt := LifecycleReceipt{OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: decision.Method, Outcome: LifecycleOutcomeLeft,
		SourceVoiceRoomID: decision.SourceVoiceRoomID, RoomID: operation.SourceRoomID, SourceRosterVersion: &sourceRoster}
	outbox := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID,
		OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt,
		Outbox: []LifecycleOutboxRecord{outbox}, CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	before := r22AllTableSnapshots(t, fixture)
	drop := r22InstallFailureAfterInsert(t, fixture, "voice_event_outbox")
	_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrUnavailable))
	require.Equal(t, before, r22AllTableSnapshots(t, fixture))
	drop()
	completed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	r22RequireCompletedReceipt(t, fixture, completed, receipt)
	r22RequireOutboxRows(t, fixture, completion)
	_, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.False(t, found)
	room, found, err := fixture.store.LoadRoomSnapshot(fixture.ctx, fixture.sourceRoomID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, sourceRoster, room.RosterVersion)
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
}

func TestPostgresLifecycleStore_C15_MoveTerminalMutatesBothRoomsAndOrdersOutbox(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec15")
	fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
	decision := fixture.decision(LifecycleMethodSelfMove)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	sourceRoster, destinationRoster := int64(4), int64(1)
	receipt := LifecycleReceipt{OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
		SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID, Method: decision.Method, Outcome: LifecycleOutcomeMoved,
		SourceVoiceRoomID: decision.SourceVoiceRoomID, DestinationVoiceRoomID: decision.DestinationVoiceRoomID,
		RoomID: operation.DestinationRoomID, SourceRosterVersion: &sourceRoster, DestinationRosterVersion: &destinationRoster,
		MediaEpoch: operation.DestinationMediaEpoch, SpaceAccessEpoch: &decision.Authority.SpaceAccessEpoch,
		RolePolicyEpoch: &decision.Authority.SubjectRolePolicyEpoch, AuthorizationDigest: &decision.Authority.AuthorizationDigest}
	leftOutbox := r22TestOutbox(fixture, operation, 0, "left", sourceRoster)
	leftOutbox.RoomID, leftOutbox.VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
	outbox := []LifecycleOutboxRecord{leftOutbox, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster)}
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID,
		OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt, Outbox: outbox,
		CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	before := r22AllTableSnapshots(t, fixture)
	require.Len(t, before["voice_media_epoch_denials"], 1, "decision must durably retain the old bearer denial")
	require.Len(t, before["voice_lifecycle_effects"], 2)
	drop := r22InstallMembershipUpdateSkip(t, fixture)
	_, err = fixture.store.CompleteOperation(fixture.ctx, completion)
	require.ErrorIs(t, err, ErrMembershipConflict)
	require.Equal(t, before, r22AllTableSnapshots(t, fixture), "zero-row generation replacement must roll every terminal write back")
	drop()
	dropDeleteFailure := r22InstallMembershipDeleteFailure(t, fixture)
	completed, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	dropDeleteFailure()
	r22RequireCompletedReceipt(t, fixture, completed, receipt)
	r22RequireOutboxRows(t, fixture, completion)
	r22RequireTerminalMembership(t, fixture, fixture.subjectProfileID, *operation.DestinationRoomID, *operation.DestinationMediaEpoch, decision.Authority, completion.CompletedAt)
	membership, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, *operation.DestinationRoomID, membership.RoomID)
	for roomID, want := range map[uuid.UUID]int64{fixture.sourceRoomID: sourceRoster, *operation.DestinationRoomID: destinationRoster} {
		room, roomFound, loadErr := fixture.store.LoadRoomSnapshot(fixture.ctx, roomID)
		require.NoError(t, loadErr)
		require.True(t, roomFound)
		require.Equal(t, want, room.RosterVersion)
	}
	rows, err := fixture.pool.Query(fixture.ctx, `SELECT ordinal,convert_from(payload_bytes,'UTF8') FROM voice_event_outbox ORDER BY ordinal`)
	require.NoError(t, err)
	defer rows.Close()
	var payloads []string
	for rows.Next() {
		var ordinal int16
		var payload string
		require.NoError(t, rows.Scan(&ordinal, &payload))
		require.Equal(t, int16(len(payloads)), ordinal)
		payloads = append(payloads, payload)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, []string{"left", "joined"}, payloads)
	require.Equal(t, before["voice_media_epoch_denials"], r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials"),
		"move must preserve the exact old-generation denial")
	require.Equal(t, before["voice_lifecycle_effects"], r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"),
		"move must preserve the exact applied eject and ensure observations")

	beforeStale := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships")
	_, err = fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, r22StoreTime.Add(3*time.Hour))
	require.ErrorIs(t, err, ErrMembershipConflict)
	require.Equal(t, beforeStale, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships"),
		"a stale source generation cannot stamp its expiry onto the destination generation")
	firstDestinationExpiry := r22StoreTime.Add(4 * time.Hour)
	advanced, err := fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, *operation.DestinationMediaEpoch, firstDestinationExpiry)
	require.NoError(t, err)
	require.NotNil(t, advanced.LatestGrantExpiresAt)
	require.True(t, firstDestinationExpiry.Equal(*advanced.LatestGrantExpiresAt))
	r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, membership, advanced)
}

func TestPostgresLifecycleStore_C16_TerminalRetryNeverReincrementsRoster(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec16")
	decision := fixture.decision(LifecycleMethodJoin)
	operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	r22ApplyAllEffects(t, fixture, decision)
	claim := r22ClaimOperation(t, fixture)
	completion := LifecycleCompletion{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
		WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: r22JoinReceipt(fixture, decision, 1),
		Outbox:      []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "joined", 1)},
		CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour)}
	first, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	r22RequireCompletedReceipt(t, fixture, first, completion.Receipt)
	r22RequireOutboxRows(t, fixture, completion)
	before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_instances")
	second, err := fixture.store.CompleteOperation(fixture.ctx, completion)
	require.NoError(t, err)
	r22RequireCompletedReceipt(t, fixture, second, completion.Receipt)
	require.Equal(t, first.ReceiptBytes, second.ReceiptBytes)
	require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_instances"))
	require.Equal(t, int64(1), r22TableRowCount(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
}

func TestPostgresLifecycleStore_C16_LeaveAndTwoRoomMoveReplayAreObservationOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, method := range []LifecycleMethod{LifecycleMethodLeave, LifecycleMethodSelfMove} {
		t.Run(map[LifecycleMethod]string{LifecycleMethodLeave: "leave", LifecycleMethodSelfMove: "two-room move"}[method], func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec16replay"+uuid.NewString()[:6])
			fixture.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
			decision := fixture.decision(method)
			operation, err := fixture.store.DecideOperation(fixture.ctx, decision)
			require.NoError(t, err)
			r22ApplyAllEffects(t, fixture, decision)
			claim := r22ClaimOperation(t, fixture)
			sourceRoster, destinationRoster := int64(4), int64(1)
			receipt := LifecycleReceipt{
				OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
				SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID,
				Method: method, Outcome: LifecycleOutcomeLeft, SourceVoiceRoomID: decision.SourceVoiceRoomID,
				RoomID: operation.SourceRoomID, SourceRosterVersion: &sourceRoster,
			}
			outbox := []LifecycleOutboxRecord{r22TestOutbox(fixture, operation, 0, "left", sourceRoster)}
			outbox[0].RoomID, outbox[0].VoiceRoomID = *operation.SourceRoomID, *operation.SourceVoiceRoomID
			if method == LifecycleMethodSelfMove {
				receipt.Outcome = LifecycleOutcomeMoved
				receipt.DestinationVoiceRoomID = decision.DestinationVoiceRoomID
				receipt.RoomID = operation.DestinationRoomID
				receipt.DestinationRosterVersion = &destinationRoster
				receipt.MediaEpoch = operation.DestinationMediaEpoch
				receipt.SpaceAccessEpoch = &decision.Authority.SpaceAccessEpoch
				receipt.RolePolicyEpoch = &decision.Authority.SubjectRolePolicyEpoch
				receipt.AuthorizationDigest = &decision.Authority.AuthorizationDigest
				outbox = append(outbox, r22TestOutbox(fixture, operation, 1, "joined", destinationRoster))
			}
			completion := LifecycleCompletion{
				ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID,
				WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Receipt: receipt, Outbox: outbox,
				CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour),
			}
			first, err := fixture.store.CompleteOperation(fixture.ctx, completion)
			require.NoError(t, err)
			r22RequireCompletedReceipt(t, fixture, first, receipt)
			r22RequireOutboxRows(t, fixture, completion)
			before := r22AllTableSnapshots(t, fixture)
			second, err := fixture.store.CompleteOperation(fixture.ctx, completion)
			require.NoError(t, err)
			r22RequireCompletedReceipt(t, fixture, second, receipt)
			require.Equal(t, first.ReceiptBytes, second.ReceiptBytes)
			require.Equal(t, before, r22AllTableSnapshots(t, fixture))
		})
	}
}

func TestPostgresLifecycleStore_C17_NoOpChangesNoRosterMembershipOrOutbox(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	for _, method := range []LifecycleMethod{LifecycleMethodJoin, LifecycleMethodLeave} {
		t.Run(map[LifecycleMethod]string{LifecycleMethodJoin: "same-room join", LifecycleMethodLeave: "absent leave"}[method], func(t *testing.T) {
			fixture := r22NewStoreFixture(t, "r22storec17"+uuid.NewString()[:6])
			decision := fixture.decision(method)
			expected := LifecycleReceipt{
				OperationID: decision.OperationID, ActorProfileID: decision.ActorProfileID,
				SubjectProfileID: decision.SubjectProfileID, SpaceID: decision.SpaceID,
				Method: method, Outcome: LifecycleOutcomeNoOp,
			}
			if method == LifecycleMethodJoin {
				fixture.seedMembership(t, nil)
				decision.DestinationVoiceRoomID = r22Pointer(fixture.sourceVoiceRoomID)
				roster, spaceEpoch, roleEpoch := int64(3), int64(1), int64(1)
				var digest LifecycleDigest
				copy(digest[:], []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"))
				expected.DestinationVoiceRoomID = decision.DestinationVoiceRoomID
				expected.RoomID = r22Pointer(fixture.sourceRoomID)
				expected.DestinationRosterVersion = &roster
				expected.MediaEpoch = r22Pointer(fixture.sourceMediaEpoch)
				expected.SpaceAccessEpoch = &spaceEpoch
				expected.RolePolicyEpoch = &roleEpoch
				expected.AuthorizationDigest = &digest
			}
			before := r22AllTableSnapshots(t, fixture)
			noOp := LifecycleNoOpDecision{Decision: decision,
				CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)}
			operation, err := fixture.store.CompleteNoOp(fixture.ctx, noOp)
			require.NoError(t, err)
			require.Equal(t, LifecycleOperationCompleted, operation.State)
			require.NotNil(t, operation.Receipt)
			require.Equal(t, LifecycleOutcomeNoOp, operation.Receipt.Outcome)
			if method == LifecycleMethodJoin {
				require.Equal(t, fixture.sourceRoomID, *operation.Receipt.RoomID)
				require.Equal(t, fixture.sourceMediaEpoch, *operation.Receipt.MediaEpoch)
				require.Equal(t, int64(1), *operation.Receipt.SpaceAccessEpoch)
				require.Equal(t, int64(1), *operation.Receipt.RolePolicyEpoch)
			} else {
				require.Equal(t, decision.SourceVoiceRoomID, operation.SourceVoiceRoomID)
				require.Nil(t, operation.DestinationVoiceRoomID)
				require.Nil(t, operation.SourceRoomID)
				require.Nil(t, operation.DestinationRoomID)
				require.Nil(t, operation.SourceMediaEpoch)
				require.Nil(t, operation.DestinationMediaEpoch)
				r22RequireAuthority(t, LifecycleAuthority{}, operation.Authority)
				require.Nil(t, operation.Receipt.SourceVoiceRoomID)
				require.Nil(t, operation.Receipt.DestinationVoiceRoomID)
				require.Nil(t, operation.Receipt.RoomID)
				require.Nil(t, operation.Receipt.MediaEpoch)
				require.Nil(t, operation.Receipt.SourceRosterVersion)
				require.Nil(t, operation.Receipt.DestinationRosterVersion)
				require.Nil(t, operation.Receipt.SpaceAccessEpoch)
				require.Nil(t, operation.Receipt.RolePolicyEpoch)
				require.Nil(t, operation.Receipt.AuthorizationDigest)
			}
			r22RequireCompletedReceipt(t, fixture, operation, expected)
			require.Equal(t, before["voice_room_memberships"], r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships"))
			require.Equal(t, before["voice_room_instances"], r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_instances"))
			require.Empty(t, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
			require.Empty(t, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
			require.Empty(t, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_media_epoch_denials"))
			afterFirst := r22AllTableSnapshots(t, fixture)
			replayed, err := fixture.store.CompleteNoOp(fixture.ctx, noOp)
			require.NoError(t, err)
			require.Equal(t, operation.ReceiptBytes, replayed.ReceiptBytes)
			require.Equal(t, operation.ReceiptHash, replayed.ReceiptHash)
			r22RequireSameImmutableOperation(t, operation, replayed)
			require.Equal(t, afterFirst, r22AllTableSnapshots(t, fixture), "identical no-op replay is observation-only")
			if method == LifecycleMethodLeave {
				require.Empty(t, before["voice_room_instances"])
				require.Empty(t, before["voice_room_memberships"])
				require.Empty(t, before["voice_lifecycle_operations"])
				require.Empty(t, before["voice_lifecycle_effects"])
				require.Empty(t, before["voice_media_epoch_denials"])
				require.Empty(t, before["voice_event_outbox"])
				require.Len(t, afterFirst["voice_lifecycle_operations"], 1)
				changed := decision
				changed.SourceVoiceRoomID = r22Pointer(uuid.New())
				_, err = fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: changed,
					CompletedAt: noOp.CompletedAt, ReplayUntil: noOp.ReplayUntil})
				require.ErrorIs(t, err, ErrOperationConflict)
				require.Equal(t, afterFirst, r22AllTableSnapshots(t, fixture), "changed requested logical binding cannot mutate the completed no-op")
			}
		})
	}
}

func TestPostgresLifecycleStore_C26_AbsentLeaveIsOneAtomicCompletedInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec26")
	decision := fixture.decision(LifecycleMethodLeave)
	before := r22AllTableSnapshots(t, fixture)
	dropFailure := r22InstallFailureAfterInsert(t, fixture, "voice_lifecycle_operations")
	_, err := fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
		CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)})
	r22RequireStableError(t, err, ErrInvariant)
	require.NotContains(t, err.Error(), "r22 forced rollback after insert")
	require.Equal(t, before, r22AllTableSnapshots(t, fixture), "failed direct completion leaves no decided or partial operation")
	dropFailure()
	_, err = fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
		CompletedAt: r22StoreTime, ReplayUntil: r22StoreTime.Add(24 * time.Hour)})
	require.NoError(t, err, "the same completed no-op remains reachable after the injected failure is removed")
}

func TestPostgresLifecycleStore_C18_GrantExpiryUsesExactEpochGreatestAndFence(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec18")
	fixture.seedMembership(t, nil)
	firstExpiry := r22StoreTime.Add(time.Minute)
	before, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	beforeImmutable := r22MembershipImmutableRowSnapshot(t, fixture)
	membership, err := fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, firstExpiry)
	require.NoError(t, err)
	require.Equal(t, firstExpiry, *membership.LatestGrantExpiresAt)
	after, found, err := fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, before, membership)
	r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, before, after)
	require.Equal(t, membership, after)
	require.False(t, after.UpdatedAt.Before(before.UpdatedAt))
	require.Equal(t, beforeImmutable, r22MembershipImmutableRowSnapshot(t, fixture))

	before = after
	beforeImmutable = r22MembershipImmutableRowSnapshot(t, fixture)
	membership, err = fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, firstExpiry.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, firstExpiry, *membership.LatestGrantExpiresAt)
	after, found, err = fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, before, membership)
	r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, before, after)
	require.Equal(t, membership, after)
	require.False(t, after.UpdatedAt.Before(before.UpdatedAt))
	require.Equal(t, beforeImmutable, r22MembershipImmutableRowSnapshot(t, fixture))

	before = after
	beforeRows := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships")
	_, err = fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, uuid.New(), firstExpiry.Add(time.Minute))
	require.ErrorIs(t, err, ErrMembershipConflict)
	after, found, err = fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, after)
	require.Equal(t, beforeRows, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships"))

	decision := fixture.decision(LifecycleMethodLeave)
	_, err = fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	before, found, err = fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	beforeRows = r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships")
	_, err = fixture.store.AdvanceLatestGrantExpiry(fixture.ctx, fixture.subjectProfileID, fixture.sourceMediaEpoch, firstExpiry.Add(time.Minute))
	require.ErrorIs(t, err, ErrSubjectTransitionActive)
	after, found, err = fixture.store.LoadMembership(fixture.ctx, fixture.subjectProfileID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, after)
	require.Equal(t, beforeRows, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_room_memberships"))

	t.Run("moved generation rejects old expiry and accepts its own first expiry", func(t *testing.T) {
		moved := r22NewStoreFixture(t, "r22storec18move")
		moved.seedMembership(t, r22Pointer(r22StoreTime.Add(time.Hour)))
		moveDecision := moved.decision(LifecycleMethodSelfMove)
		moveOperation, decideErr := moved.store.DecideOperation(moved.ctx, moveDecision)
		require.NoError(t, decideErr)
		r22ApplyAllEffects(t, moved, moveDecision)
		moveClaim := r22ClaimOperation(t, moved)
		sourceRoster, destinationRoster := int64(4), int64(1)
		moveReceipt := LifecycleReceipt{
			OperationID: moveDecision.OperationID, ActorProfileID: moveDecision.ActorProfileID,
			SubjectProfileID: moveDecision.SubjectProfileID, SpaceID: moveDecision.SpaceID,
			Method: moveDecision.Method, Outcome: LifecycleOutcomeMoved,
			SourceVoiceRoomID: moveDecision.SourceVoiceRoomID, DestinationVoiceRoomID: moveDecision.DestinationVoiceRoomID,
			RoomID: moveOperation.DestinationRoomID, SourceRosterVersion: &sourceRoster, DestinationRosterVersion: &destinationRoster,
			MediaEpoch: moveOperation.DestinationMediaEpoch, SpaceAccessEpoch: &moveDecision.Authority.SpaceAccessEpoch,
			RolePolicyEpoch: &moveDecision.Authority.SubjectRolePolicyEpoch, AuthorizationDigest: &moveDecision.Authority.AuthorizationDigest,
		}
		left := r22TestOutbox(moved, moveOperation, 0, "left", sourceRoster)
		left.RoomID, left.VoiceRoomID = *moveOperation.SourceRoomID, *moveOperation.SourceVoiceRoomID
		moveCompletion := LifecycleCompletion{
			ActorProfileID: moveDecision.ActorProfileID, OperationID: moveDecision.OperationID,
			WorkerID: moved.workerID, ExpectedFence: moveClaim.Fence, Receipt: moveReceipt,
			Outbox:      []LifecycleOutboxRecord{left, r22TestOutbox(moved, moveOperation, 1, "joined", destinationRoster)},
			CompletedAt: r22StoreTime.Add(2 * time.Minute), ReplayUntil: r22StoreTime.Add(26 * time.Hour),
		}
		_, completeErr := moved.store.CompleteOperation(moved.ctx, moveCompletion)
		require.NoError(t, completeErr)
		newGeneration, found, loadErr := moved.store.LoadMembership(moved.ctx, moved.subjectProfileID)
		require.NoError(t, loadErr)
		require.True(t, found)
		require.Nil(t, newGeneration.LatestGrantExpiresAt)
		beforeOldAttempt := r22TableRowsSnapshot(t, moved.ctx, moved.pool, "voice_room_memberships")
		_, advanceErr := moved.store.AdvanceLatestGrantExpiry(moved.ctx, moved.subjectProfileID, moved.sourceMediaEpoch, r22StoreTime.Add(3*time.Hour))
		require.ErrorIs(t, advanceErr, ErrMembershipConflict)
		require.Equal(t, beforeOldAttempt, r22TableRowsSnapshot(t, moved.ctx, moved.pool, "voice_room_memberships"))
		newExpiry := r22StoreTime.Add(4 * time.Hour)
		advanced, advanceErr := moved.store.AdvanceLatestGrantExpiry(moved.ctx, moved.subjectProfileID, *moveOperation.DestinationMediaEpoch, newExpiry)
		require.NoError(t, advanceErr)
		require.NotNil(t, advanced.LatestGrantExpiresAt)
		require.True(t, newExpiry.Equal(*advanced.LatestGrantExpiresAt))
		r22RequireOnlyMembershipExpiryAndUpdatedAtMayChange(t, newGeneration, advanced)
	})
}

func TestPostgresLifecycleStore_C19_ExpiredCompletionRemainsBindingAndNotRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec19")
	decision := fixture.decision(LifecycleMethodLeave)
	_, err := fixture.store.CompleteNoOp(fixture.ctx, LifecycleNoOpDecision{Decision: decision,
		CompletedAt: r22StoreTime.Add(-48 * time.Hour), ReplayUntil: r22StoreTime.Add(-24 * time.Hour)})
	require.NoError(t, err)
	changed := decision
	changed.SourceVoiceRoomID = r22Pointer(uuid.New())
	_, err = fixture.store.DecideOperation(fixture.ctx, changed)
	require.ErrorIs(t, err, ErrOperationConflict)
	_, found, err := fixture.store.ClaimNextOperation(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.False(t, found)
}

func r22DropCheckContaining(t *testing.T, fixture r22StoreFixture, table, fragment string) {
	t.Helper()
	var constraint string
	require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `
SELECT c.conname FROM pg_constraint c JOIN pg_class r ON r.oid=c.conrelid
WHERE r.relname=$1 AND c.contype='c' AND position($2 in lower(pg_get_constraintdef(c.oid)))>0
ORDER BY c.conname LIMIT 1`, table, fragment).Scan(&constraint))
	_, err := fixture.pool.Exec(fixture.ctx, "ALTER TABLE "+pgx.Identifier{table}.Sanitize()+" DROP CONSTRAINT "+pgx.Identifier{constraint}.Sanitize())
	require.NoError(t, err)
}

func TestPostgresLifecycleStore_C20_CorruptionIsInvariantFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	t.Run("operation", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec20op")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22DropCheckContaining(t, fixture, "voice_lifecycle_operations", "lease_fence")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_operations SET lease_fence=-1 WHERE actor_profile_id=$1 AND operation_id=$2`, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		_, _, err = fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.ErrorIs(t, err, ErrInvariant)
	})
	t.Run("effect", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec20effect")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		r22DropCheckContaining(t, fixture, "voice_lifecycle_effects", "attempt_count")
		_, err = fixture.pool.Exec(fixture.ctx, `UPDATE voice_lifecycle_effects SET attempt_count=-1 WHERE effect_id=$1`, decision.Effects[0].EffectID)
		require.NoError(t, err)
		_, err = fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.ErrorIs(t, err, ErrInvariant)
	})
	t.Run("outbox", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec20outbox")
		seed := r22NewIntegritySeed(t, fixture.ctx, fixture.pool)
		parent := r22OperationArgs(seed, "join", "completed", "joined")
		require.NoError(t, r22InsertOperation(fixture.ctx, fixture.pool, parent))
		args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
		require.NoError(t, r22InsertOutbox(fixture.ctx, fixture.pool, args))
		r22DropCheckContaining(t, fixture, "voice_event_outbox", "attempt_count")
		_, err := fixture.pool.Exec(fixture.ctx, `UPDATE voice_event_outbox SET attempt_count=-1 WHERE event_id=$1`, args["event_id"])
		require.NoError(t, err)
		_, _, err = fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.ErrorIs(t, err, ErrInvariant)
	})
}

func TestPostgresLifecycleStore_C21_StaleEffectFencePreservesExactRow(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec21")
	decision := fixture.decision(LifecycleMethodJoin)
	_, err := fixture.store.DecideOperation(fixture.ctx, decision)
	require.NoError(t, err)
	old, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	newWorker := uuid.New()
	current, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, newWorker, r22StoreTime.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, current.Fence, old.Fence)
	before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects")
	mutations := []func() error{
		func() error {
			return fixture.store.RenewEffectLease(fixture.ctx, LifecycleEffectLease{EffectID: old.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, LeaseUntil: r22StoreTime.Add(3 * time.Minute)})
		},
		func() error {
			return fixture.store.MarkEffectApplied(fixture.ctx, LifecycleEffectApplied{EffectID: old.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, Observed: old.Effect, AppliedAt: r22StoreTime.Add(2 * time.Minute)})
		},
		func() error {
			return fixture.store.MarkEffectRetry(fixture.ctx, LifecycleEffectRetry{EffectID: old.Effect.EffectID, Retry: LifecycleRetry{WorkerID: fixture.workerID, ExpectedFence: old.Fence, ErrorClass: "late", ErrorAt: r22StoreTime.Add(2 * time.Minute), NextAttemptAt: r22StoreTime.Add(3 * time.Minute)}})
		},
		func() error {
			return fixture.store.QuarantineEffect(fixture.ctx, LifecycleEffectQuarantine{EffectID: old.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, Class: "late", At: r22StoreTime.Add(2 * time.Minute)})
		},
	}
	for _, mutate := range mutations {
		require.ErrorIs(t, mutate(), ErrStaleLease)
		require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_lifecycle_effects"))
	}
}

func r22SeedStoreOutbox(t *testing.T, fixture r22StoreFixture) (uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	seed := r22NewIntegritySeed(t, fixture.ctx, fixture.pool)
	parent := r22OperationArgs(seed, "join", "completed", "joined")
	require.NoError(t, r22InsertOperation(fixture.ctx, fixture.pool, parent))
	args := r22OutboxArgs(seed, parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID), "ready")
	args["next_attempt_at"] = r22StoreTime
	require.NoError(t, r22InsertOutbox(fixture.ctx, fixture.pool, args))
	return args["event_id"].(uuid.UUID), parent["actor_id"].(uuid.UUID), parent["operation_id"].(uuid.UUID)
}

func TestPostgresLifecycleStore_C22_StaleOutboxFencePreservesExactRow(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec22")
	eventID, _, _ := r22SeedStoreOutbox(t, fixture)
	old, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	newWorker := uuid.New()
	current, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, newWorker, r22StoreTime.Add(time.Minute), time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.Greater(t, current.Fence, old.Fence)
	before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox")
	mutations := []func() error{
		func() error {
			return fixture.store.RenewOutboxLease(fixture.ctx, LifecycleOutboxLease{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, LeaseUntil: r22StoreTime.Add(3 * time.Minute)})
		},
		func() error {
			return fixture.store.MarkOutboxDelivered(fixture.ctx, LifecycleOutboxDelivered{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, DeliveredAt: r22StoreTime.Add(2 * time.Minute)})
		},
		func() error {
			return fixture.store.MarkOutboxRetry(fixture.ctx, LifecycleOutboxRetry{EventID: eventID, Retry: LifecycleRetry{WorkerID: fixture.workerID, ExpectedFence: old.Fence, ErrorClass: "late", ErrorAt: r22StoreTime.Add(2 * time.Minute), NextAttemptAt: r22StoreTime.Add(3 * time.Minute)}})
		},
		func() error {
			return fixture.store.QuarantineOutbox(fixture.ctx, LifecycleOutboxQuarantine{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: old.Fence, Class: "late", At: r22StoreTime.Add(2 * time.Minute)})
		},
	}
	for _, mutate := range mutations {
		require.ErrorIs(t, mutate(), ErrStaleLease)
		require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
	}
}

func TestPostgresLifecycleStore_C23_CurrentFenceWritesPairedEvidenceAndClearsLease(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	t.Run("operation quarantine", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec23op")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim := r22ClaimOperation(t, fixture)
		renewedUntil := r22StoreTime.Add(2 * time.Minute)
		require.NoError(t, fixture.store.RenewOperationLease(fixture.ctx, LifecycleOperationLease{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: renewedUntil}))
		quarantinedAt := r22StoreTime.Add(time.Minute)
		require.NoError(t, fixture.store.QuarantineOperation(fixture.ctx, LifecycleOperationQuarantine{ActorProfileID: decision.ActorProfileID, OperationID: decision.OperationID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "manual", Detail: "review", At: quarantinedAt}))
		op, found, err := fixture.store.LoadOperation(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, LifecycleOperationQuarantined, op.State)
		require.Nil(t, op.LeaseOwner)
		require.Nil(t, op.LeaseUntil)
		require.Equal(t, "manual", *op.QuarantineClass)
		require.Equal(t, "review", *op.QuarantineDetail)
		require.Equal(t, quarantinedAt, *op.QuarantinedAt)
	})
	t.Run("effect retry", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec23effect")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, fixture.store.RenewEffectLease(fixture.ctx, LifecycleEffectLease{EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: r22StoreTime.Add(2 * time.Minute)}))
		retry := LifecycleEffectRetry{EffectID: claim.Effect.EffectID, Retry: LifecycleRetry{WorkerID: fixture.workerID, ExpectedFence: claim.Fence, ErrorClass: "temporary", ErrorAt: r22StoreTime.Add(time.Minute), NextAttemptAt: r22StoreTime.Add(2 * time.Minute)}}
		require.NoError(t, fixture.store.MarkEffectRetry(fixture.ctx, retry))
		effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.Len(t, effects, 1)
		require.Equal(t, LifecycleEffectStateReady, effects[0].State)
		require.Equal(t, int64(1), effects[0].AttemptCount)
		require.Equal(t, "temporary", *effects[0].LastErrorClass)
		require.Equal(t, retry.Retry.ErrorAt, *effects[0].LastErrorAt)
		require.Equal(t, retry.Retry.NextAttemptAt, effects[0].NextAttemptAt)
		require.Nil(t, effects[0].LeaseOwner)
		require.Nil(t, effects[0].LeaseUntil)
		require.Nil(t, effects[0].AppliedAt)
		require.Nil(t, effects[0].QuarantineClass)
		require.Nil(t, effects[0].QuarantinedAt)
	})
	t.Run("effect quarantine", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec23effectquarantine")
		decision := fixture.decision(LifecycleMethodJoin)
		_, err := fixture.store.DecideOperation(fixture.ctx, decision)
		require.NoError(t, err)
		claim, ok, err := fixture.store.ClaimNextEffect(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, fixture.store.RenewEffectLease(fixture.ctx, LifecycleEffectLease{EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: r22StoreTime.Add(2 * time.Minute)}))
		quarantine := LifecycleEffectQuarantine{EffectID: claim.Effect.EffectID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "target_mismatch", Detail: "room changed", At: r22StoreTime.Add(time.Minute)}
		require.NoError(t, fixture.store.QuarantineEffect(fixture.ctx, quarantine))
		effects, err := fixture.store.ListOperationEffects(fixture.ctx, decision.ActorProfileID, decision.OperationID)
		require.NoError(t, err)
		require.Len(t, effects, 1)
		require.Equal(t, LifecycleEffectStateQuarantined, effects[0].State)
		require.Equal(t, "target_mismatch", *effects[0].QuarantineClass)
		require.Equal(t, "room changed", *effects[0].QuarantineDetail)
		require.Equal(t, quarantine.At, *effects[0].QuarantinedAt)
		require.Zero(t, effects[0].AttemptCount)
		require.Nil(t, effects[0].LastErrorClass)
		require.Nil(t, effects[0].LastErrorAt)
		require.Equal(t, r22StoreTime, effects[0].NextAttemptAt)
		require.Nil(t, effects[0].LeaseOwner)
		require.Nil(t, effects[0].LeaseUntil)
		require.Nil(t, effects[0].AppliedAt)
	})
	t.Run("outbox retry", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec23outboxretry")
		eventID, _, _ := r22SeedStoreOutbox(t, fixture)
		claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, fixture.store.RenewOutboxLease(fixture.ctx, LifecycleOutboxLease{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: r22StoreTime.Add(2 * time.Minute)}))
		retry := LifecycleOutboxRetry{EventID: eventID, Retry: LifecycleRetry{WorkerID: fixture.workerID, ExpectedFence: claim.Fence, ErrorClass: "temporary", ErrorAt: r22StoreTime.Add(time.Minute), NextAttemptAt: r22StoreTime.Add(2 * time.Minute)}}
		require.NoError(t, fixture.store.MarkOutboxRetry(fixture.ctx, retry))
		var state, class string
		var attempts int64
		var errorAt, nextAttempt time.Time
		var owner *uuid.UUID
		var until *time.Time
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT state,attempt_count,last_error_class,last_error_at,next_attempt_at,lease_owner,lease_until FROM voice_event_outbox WHERE event_id=$1`, eventID).Scan(&state, &attempts, &class, &errorAt, &nextAttempt, &owner, &until))
		require.Equal(t, "ready", state)
		require.Equal(t, int64(1), attempts)
		require.Equal(t, retry.Retry.ErrorClass, class)
		require.True(t, retry.Retry.ErrorAt.Equal(errorAt), "last_error_at must be the exact expected instant")
		require.True(t, retry.Retry.NextAttemptAt.Equal(nextAttempt), "next_attempt_at must be the exact expected instant")
		require.Nil(t, owner)
		require.Nil(t, until)
	})
	t.Run("outbox quarantine", func(t *testing.T) {
		fixture := r22NewStoreFixture(t, "r22storec23outbox")
		eventID, _, _ := r22SeedStoreOutbox(t, fixture)
		claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
		require.NoError(t, err)
		require.True(t, ok)
		require.NoError(t, fixture.store.RenewOutboxLease(fixture.ctx, LifecycleOutboxLease{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, LeaseUntil: r22StoreTime.Add(2 * time.Minute)}))
		quarantinedAt := r22StoreTime.Add(time.Minute)
		require.NoError(t, fixture.store.QuarantineOutbox(fixture.ctx, LifecycleOutboxQuarantine{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "poison", Detail: "fixture", At: quarantinedAt}))
		var state, class, detail string
		var attempts int64
		var lastErrorClass *string
		var lastErrorAt *time.Time
		var nextAttempt time.Time
		var owner *uuid.UUID
		var until, gotQuarantinedAt *time.Time
		require.NoError(t, fixture.pool.QueryRow(fixture.ctx, `SELECT state,attempt_count,last_error_class,last_error_at,next_attempt_at,quarantine_class,quarantine_detail,lease_owner,lease_until,quarantined_at FROM voice_event_outbox WHERE event_id=$1`, eventID).Scan(&state, &attempts, &lastErrorClass, &lastErrorAt, &nextAttempt, &class, &detail, &owner, &until, &gotQuarantinedAt))
		require.Equal(t, "quarantined", state)
		require.Zero(t, attempts)
		require.Nil(t, lastErrorClass)
		require.Nil(t, lastErrorAt)
		require.True(t, r22StoreTime.Equal(nextAttempt), "next_attempt_at must preserve the exact seeded instant")
		require.Equal(t, "poison", class)
		require.Equal(t, "fixture", detail)
		require.Nil(t, owner)
		require.Nil(t, until)
		require.True(t, quarantinedAt.Equal(*gotQuarantinedAt), "quarantined_at must be the exact expected instant")
	})
}

func TestPostgresLifecycleStore_C24_DeliveredOutboxNeverRegresses(t *testing.T) {
	if testing.Short() {
		t.Skip("PostgreSQL store contract requires testcontainers")
	}
	fixture := r22NewStoreFixture(t, "r22storec24")
	eventID, _, _ := r22SeedStoreOutbox(t, fixture)
	claim, ok, err := fixture.store.ClaimNextOutbox(fixture.ctx, fixture.workerID, r22StoreTime, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	delivery := LifecycleOutboxDelivered{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, DeliveredAt: r22StoreTime.Add(time.Minute)}
	require.NoError(t, fixture.store.MarkOutboxDelivered(fixture.ctx, delivery))
	before := r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox")
	require.NoError(t, fixture.store.MarkOutboxDelivered(fixture.ctx, delivery), "exact delivery replay is observation-only")
	require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
	require.ErrorIs(t, fixture.store.MarkOutboxRetry(fixture.ctx, LifecycleOutboxRetry{EventID: eventID, Retry: LifecycleRetry{WorkerID: fixture.workerID, ExpectedFence: claim.Fence, ErrorClass: "late", ErrorAt: r22StoreTime.Add(2 * time.Minute), NextAttemptAt: r22StoreTime.Add(3 * time.Minute)}}), ErrStaleLease)
	require.ErrorIs(t, fixture.store.QuarantineOutbox(fixture.ctx, LifecycleOutboxQuarantine{EventID: eventID, WorkerID: fixture.workerID, ExpectedFence: claim.Fence, Class: "late", At: r22StoreTime.Add(2 * time.Minute)}), ErrStaleLease)
	require.Equal(t, before, r22TableRowsSnapshot(t, fixture.ctx, fixture.pool, "voice_event_outbox"))
}
