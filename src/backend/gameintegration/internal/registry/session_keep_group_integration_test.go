package registry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
)

type keepGroupOwnerRecorder struct {
	createCalls int
	addCalls    int
	syncCalls   int
	chatID      uuid.UUID
	lastSync    *chatv1.SyncManagedChatMembersRequest
	createFails int
	createOps   []uuid.UUID
}

func (r *keepGroupOwnerRecorder) CreateChat(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
	r.createCalls++
	r.createOps = append(r.createOps, request.OperationID)
	if r.createFails > 0 {
		r.createFails--
		return SessionOwnerReceipt{}, errors.New("Chat temporarily unavailable")
	}
	return SessionOwnerReceipt{ResourceID: r.chatID, ReceiptID: request.OperationID, RequestHash: request.RequestHash}, nil
}

func (r *keepGroupOwnerRecorder) SyncChatRoster(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
	message := request.Proto.(*chatv1.SyncManagedChatMembersRequest)
	r.lastSync = message
	if message.GetAddOnly() {
		r.addCalls++
	} else {
		r.syncCalls++
	}
	return SessionOwnerReceipt{ResourceID: request.ChatID, ReceiptID: request.OperationID, RequestHash: request.RequestHash}, nil
}

func TestKeepSessionGroupCreatesNewChatWithOnlyConsentingProfiles(t *testing.T) {
	store, ctx, principal, _, accountID, profileID, _ := setupT34KeepGroupSession(t)
	sessionID := uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,display_name,session_status,stage,roster_revision,roster_complete,members,lease_expires_at)
VALUES($1,$2,$3,'match','match-no-party','Private group','active','active',3,true,$4,clock_timestamp()+interval '60 seconds')`,
		sessionID, principal.ApplicationID, principal.EnvironmentID, []uuid.UUID{profileID, uuid.New()})
	require.NoError(t, err)
	_, err = store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 3)
	require.NoError(t, err)
	chatID := uuid.New()
	owners := &keepGroupOwnerRecorder{chatID: chatID}
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{CreateChat: owners.CreateChat, SyncChatRoster: owners.SyncChatRoster})
	receipt, err := orchestrator.KeepSessionGroup(ctx, SessionKeepGroupInput{
		OperationID: uuid.New(), SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 3,
	})
	require.NoError(t, err)
	require.Equal(t, chatID, receipt.ChatID)
	require.Equal(t, "session/"+sessionID.String(), receipt.DestinationKey)
	require.Equal(t, []uuid.UUID{profileID}, receipt.AddedProfiles)
	require.Equal(t, 1, owners.createCalls)
	require.Equal(t, 1, owners.addCalls)
	require.Equal(t, 0, owners.syncCalls)
	require.NotNil(t, owners.lastSync)
	require.True(t, owners.lastSync.GetAddOnly(), "concurrent consent snapshots must not replace the permanent group roster")
	require.Equal(t, []string{profileID.String()}, owners.lastSync.GetProfileIds())
}

func TestKeepSessionGroupOlderSnapshotCannotRemoveLaterConsenter(t *testing.T) {
	store, ctx, principal, existingSession, accountA, profileA, _ := setupT34KeepGroupSession(t)
	var accountB, profileB uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT account_id,profile_id FROM player_bindings
WHERE application_id=$1 AND environment_id=$2 AND profile_id<>$3`, principal.ApplicationID, principal.EnvironmentID, profileA).Scan(&accountB, &profileB))
	sessionID := uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,display_name,session_status,stage,roster_revision,roster_complete,members,lease_expires_at)
SELECT $1,application_id,environment_id,'match','keep-concurrent','Group','active','active',4,true,$2,clock_timestamp()+interval '60 seconds'
FROM gis_sessions WHERE id=$3`, sessionID, []uuid.UUID{profileA, profileB}, existingSession)
	require.NoError(t, err)
	_, err = store.RecordSessionConsent(ctx, accountA, profileA, sessionID, uuid.New(), 4)
	require.NoError(t, err)
	older := SessionKeepGroupInput{OperationID: uuid.New(), SessionID: sessionID, AccountID: accountA, ProfileID: profileA, RosterRevision: 4}
	_, saved, err := store.prepareSessionKeepGroup(ctx, older)
	require.NoError(t, err)
	require.Nil(t, saved)
	_, err = store.RecordSessionConsent(ctx, accountB, profileB, sessionID, uuid.New(), 4)
	require.NoError(t, err)

	chatID := uuid.New()
	members := map[string]bool{}
	owners := SessionOwnerAdapters{
		CreateChat: func(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
			return SessionOwnerReceipt{ResourceID: chatID, ReceiptID: request.OperationID, RequestHash: request.RequestHash}, nil
		},
		SyncChatRoster: func(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
			message := request.Proto.(*chatv1.SyncManagedChatMembersRequest)
			if !message.GetAddOnly() {
				clear(members)
			}
			for _, profile := range message.GetProfileIds() {
				members[profile] = true
			}
			return SessionOwnerReceipt{ResourceID: request.ChatID, ReceiptID: request.OperationID, RequestHash: request.RequestHash}, nil
		},
	}
	orchestrator := NewSessionOrchestrator(store, owners)
	younger := SessionKeepGroupInput{OperationID: uuid.New(), SessionID: sessionID, AccountID: accountB, ProfileID: profileB, RosterRevision: 4}
	_, err = orchestrator.KeepSessionGroup(ctx, younger)
	require.NoError(t, err)
	_, err = orchestrator.KeepSessionGroup(ctx, older)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{profileA.String(): true, profileB.String(): true}, members,
		"a delayed older consent snapshot must not remove a member added by a later operation")
	replay, err := orchestrator.KeepSessionGroup(ctx, younger)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, map[string]bool{profileA.String(): true, profileB.String(): true}, members)
}

func TestSuspendedApplicationCannotGrantFreshConsentOrKeepGroup(t *testing.T) {
	store, ctx, p, sessionID, accountID, profileID, _ := setupT34KeepGroupSession(t)
	_, err := store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 7)
	require.NoError(t, err)
	_, err = store.SetApplicationSuspension(ctx, SetApplicationSuspensionInput{ApplicationID: p.ApplicationID, OperatorAccountID: uuid.New(), Suspended: true, IdempotencyKey: uuid.NewString()})
	require.NoError(t, err)
	profiles, err := store.CurrentSessionConsentProfiles(ctx, sessionID, 7)
	require.NoError(t, err)
	require.Empty(t, profiles)
	_, err = store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 7)
	require.ErrorIs(t, err, ErrSessionConsentDenied)
	owners := &keepGroupOwnerRecorder{chatID: uuid.New()}
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{CreateChat: owners.CreateChat, SyncChatRoster: owners.SyncChatRoster})
	_, err = orchestrator.KeepSessionGroup(ctx, SessionKeepGroupInput{OperationID: uuid.New(), SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 7})
	require.ErrorIs(t, err, ErrSessionKeepGroupDenied)
	require.Zero(t, owners.createCalls)
	require.Zero(t, owners.addCalls)
	require.Zero(t, owners.syncCalls)
}

func TestKeepSessionGroupAddsOnlyCurrentConsentsToParentAndReplaysOwnerReceipt(t *testing.T) {
	store, ctx, principal, sessionID, accountID, profileID, parentChatID := setupT34KeepGroupSession(t)
	operationID := uuid.New()
	_, err := store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 7)
	require.NoError(t, err)
	owners := &keepGroupOwnerRecorder{chatID: uuid.New()}
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{CreateChat: owners.CreateChat, SyncChatRoster: owners.SyncChatRoster})
	input := SessionKeepGroupInput{OperationID: operationID, SessionID: sessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 7}

	receipt, err := orchestrator.KeepSessionGroup(ctx, input)
	require.NoError(t, err)
	require.Equal(t, parentChatID, receipt.ChatID)
	require.Equal(t, "party/party-t34", receipt.DestinationKey)
	require.Equal(t, []uuid.UUID{profileID}, receipt.AddedProfiles)
	require.Equal(t, 0, owners.createCalls, "continuing a party must not create a replacement group")
	require.Equal(t, 1, owners.addCalls)
	require.Zero(t, owners.syncCalls, "existing party roster must not be replaced")

	replayed, err := orchestrator.KeepSessionGroup(ctx, input)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	replayed.Replayed = false
	require.Equal(t, receipt, replayed)
	require.Equal(t, 1, owners.addCalls, "completed retry must return the durable owner receipt")

	changed := input
	changed.RosterRevision++
	_, err = orchestrator.KeepSessionGroup(ctx, changed)
	require.ErrorIs(t, err, ErrSessionKeepGroupConflict)
	_ = principal
}

func TestKeepSessionGroupRetriesAfterChatCreateOutageWithStableOwnerOperation(t *testing.T) {
	store, ctx, _, sessionID, accountID, profileID, _ := setupT34KeepGroupSession(t)
	noPartySessionID := uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,display_name,session_status,stage,roster_revision,roster_complete,members,lease_expires_at)
SELECT $1,application_id,environment_id,'match','match-no-party-retry','Retry group','active','active',4,true,$2,clock_timestamp()+interval '60 seconds'
FROM gis_sessions WHERE id=$3`, noPartySessionID, []uuid.UUID{profileID}, sessionID)
	require.NoError(t, err)
	_, err = store.RecordSessionConsent(ctx, accountID, profileID, noPartySessionID, uuid.New(), 4)
	require.NoError(t, err)
	owners := &keepGroupOwnerRecorder{chatID: uuid.New(), createFails: 1}
	orchestrator := NewSessionOrchestrator(store, SessionOwnerAdapters{CreateChat: owners.CreateChat, SyncChatRoster: owners.SyncChatRoster})
	input := SessionKeepGroupInput{OperationID: uuid.New(), SessionID: noPartySessionID, AccountID: accountID, ProfileID: profileID, RosterRevision: 4}

	_, err = orchestrator.KeepSessionGroup(ctx, input)
	require.ErrorContains(t, err, "temporarily unavailable")
	var savedChatID *uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT chat_id FROM gis_session_keep_group_operations WHERE operation_id=$1`, input.OperationID).Scan(&savedChatID))
	require.Nil(t, savedChatID, "a failed create leaves durable pending work without a Chat ID")

	receipt, err := orchestrator.KeepSessionGroup(ctx, input)
	require.NoError(t, err, "pending new-group work must replay after the nullable chat_id read")
	require.Equal(t, owners.chatID, receipt.ChatID)
	require.Equal(t, []uuid.UUID{profileID}, receipt.AddedProfiles)
	require.Len(t, owners.createOps, 2)
	require.Equal(t, owners.createOps[0], owners.createOps[1], "Chat create uses one stable idempotency key across outage retry")
	require.Equal(t, 1, owners.addCalls)
}

func setupT34KeepGroupSession(t *testing.T) (*Store, context.Context, SessionPrincipal, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID) {
	t.Helper()
	store, ctx := startT32SessionStore(t)
	for _, migrationName := range []string{"000019_t34_session_consent.up.sql", "000020_t34_keep_group_operations.up.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", migrationName))
		require.NoError(t, err)
		_, err = store.Pool.Exec(ctx, string(migration))
		require.NoError(t, err)
	}
	principal := newT31TestSessionPrincipal(t, ctx, store)
	accountID, profileID, otherProfileID := uuid.New(), uuid.New(), uuid.New()
	parentSessionID, sessionID, parentChatID := uuid.New(), uuid.New(), uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,session_status,stage,roster_revision,roster_complete,members,lease_expires_at,chat_id,chat_owner_session_id,chat_create_receipt_id,chat_roster_receipt_id)
VALUES($1,$2,$3,'party','party-t34','active','active',1,true,$4,clock_timestamp()+interval '60 seconds',$5,$1,$6,$7)`,
		parentSessionID, principal.ApplicationID, principal.EnvironmentID, []uuid.UUID{profileID, otherProfileID}, parentChatID, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,parent_party_key,session_status,stage,roster_revision,roster_complete,members,lease_expires_at,chat_id,chat_owner_session_id,chat_create_receipt_id,chat_roster_receipt_id)
VALUES($1,$2,$3,'match','match-t34','party-t34','active','active',7,true,$4,clock_timestamp()+interval '60 seconds',$5,$6,$7,$8)`,
		sessionID, principal.ApplicationID, principal.EnvironmentID, []uuid.UUID{profileID, otherProfileID}, parentChatID, parentSessionID, uuid.New(), uuid.New())
	require.NoError(t, err)
	for i, profile := range []uuid.UUID{profileID, otherProfileID} {
		account := accountID
		if i == 1 {
			account = uuid.New()
		}
		digest := "hmac-sha256-v1:test:1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		if i == 1 {
			digest = "hmac-sha256-v1:test:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
		}
		_, err = store.Pool.Exec(ctx, `INSERT INTO player_bindings
(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'provider',$4,$5,$5,$6,$7,'active',1)`, uuid.New(), principal.ApplicationID, principal.EnvironmentID,
			digest, account, profile, uuid.New())
		require.NoError(t, err)
	}
	return store, ctx, principal, sessionID, accountID, profileID, parentChatID
}
