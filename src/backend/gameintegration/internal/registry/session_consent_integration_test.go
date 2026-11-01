package registry

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionConsentReceiptIsParticipantBoundDurableAndIdempotent(t *testing.T) {
	store, ctx := startT32SessionStore(t)
	migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "game_integration_db", "000019_t34_session_consent.up.sql"))
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	accountID, profileID, secondAccountID, nonconsentingProfile, outsiderProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	sessionID, operationID := uuid.New(), uuid.New()
	_, err = store.Pool.Exec(ctx, `INSERT INTO gis_sessions
(id,application_id,environment_id,kind,external_key,parent_party_key,session_status,stage,roster_revision,roster_complete,members,lease_expires_at)
VALUES($1,$2,$3,'match','match-1','party-7','active','active',7,true,$4,clock_timestamp()+interval '60 seconds')`,
		sessionID, principal.ApplicationID, principal.EnvironmentID, []uuid.UUID{profileID, nonconsentingProfile})
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `INSERT INTO player_bindings
(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'provider','hmac-sha256-v1:test:1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',$4,$4,$5,$6,'active',1)`,
		uuid.New(), principal.ApplicationID, principal.EnvironmentID, secondAccountID, nonconsentingProfile, uuid.New())
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `INSERT INTO player_bindings
(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,status,authority_revision)
VALUES($1,$2,$3,'provider','hmac-sha256-v1:test:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',$4,$4,$5,$6,'active',1)`,
		uuid.New(), principal.ApplicationID, principal.EnvironmentID, accountID, profileID, uuid.New())
	require.NoError(t, err)

	receipt, err := store.RecordSessionConsent(ctx, accountID, profileID, sessionID, operationID, 7)
	require.NoError(t, err)
	require.Equal(t, accountID, receipt.AccountID)
	require.Equal(t, profileID, receipt.ProfileID)
	require.Equal(t, sessionID, receipt.SessionID)
	require.Equal(t, operationID, receipt.OperationID)
	require.Equal(t, int64(7), receipt.RosterRevision)
	require.Equal(t, "party/party-7", receipt.DestinationKey)
	require.Positive(t, receipt.PolicyRevision)
	require.NotEqual(t, uuid.Nil, receipt.ReceiptID)
	consenting, err := store.CurrentSessionConsentProfiles(ctx, sessionID, 7)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{profileID}, consenting, "the game roster alone does not create a Chat member")

	replayed, err := (&Store{Pool: store.Pool}).RecordSessionConsent(ctx, accountID, profileID, sessionID, operationID, 7)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed, "restart/retry returns the exact persisted receipt")

	_, err = store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 7)
	require.ErrorIs(t, err, ErrSessionConsentConflict, "one participant has one receipt for one roster revision")
	_, err = store.RecordSessionConsent(ctx, accountID, outsiderProfile, sessionID, uuid.New(), 7)
	require.ErrorIs(t, err, ErrSessionConsentDenied, "a profile outside the current roster cannot consent")
	_, err = store.RecordSessionConsent(ctx, uuid.New(), profileID, sessionID, uuid.New(), 7)
	require.ErrorIs(t, err, ErrSessionConsentDenied, "client profile identity cannot be paired with a different account")
	_, err = store.RecordSessionConsent(ctx, accountID, profileID, sessionID, uuid.New(), 6)
	require.ErrorIs(t, err, ErrSessionConsentDenied, "stale roster revision cannot create consent")

	var count int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_consent_receipts WHERE session_id=$1`, sessionID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = store.Pool.Exec(ctx, `UPDATE environments SET revision=revision+1 WHERE id=$1`, principal.EnvironmentID)
	require.NoError(t, err)
	consenting, err = store.CurrentSessionConsentProfiles(ctx, sessionID, 7)
	require.NoError(t, err)
	require.Empty(t, consenting, "changed policy revision invalidates earlier consent for destination writes")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_sessions SET lease_expires_at=$2 WHERE id=$1`, sessionID, time.Now().Add(-time.Second))
	require.NoError(t, err)
	_, err = store.RecordSessionConsent(context.Background(), accountID, profileID, sessionID, uuid.New(), 7)
	require.ErrorIs(t, err, ErrSessionConsentDenied, "expired session consent fails closed")
}
