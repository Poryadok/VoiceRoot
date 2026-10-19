package gameprovision

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
)

// Fixture authority proves only the journal/storage contract. It is never
// wired into production or evidence of an actual closed-admission installation.
type cutoverFixtureAuthority struct{ err error }

func (a cutoverFixtureAuthority) CheckMaintenance(context.Context, retirementproof.CutBinding) error {
	return a.err
}

func TestOwnedGameCutover_SourceCompleteSealing(t *testing.T) {
	ctx := context.Background()
	pool := cutoverTestDB(t)
	cut := cutoverTestBinding()
	journal := &CutoverWorkStore{Pool: pool, Authority: cutoverFixtureAuthority{}, SourceKind: GameConversionSource}
	provisioned, err := NewPostgresStore(pool).Provision(ctx, validProvisionRequest())
	require.NoError(t, err)
	source, target := uuid.New(), uuid.New()
	conversion := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO voice_sdk_conversion_fence_receipts
 (operation_id,binding_id,source_account_id,source_actor_id,source_profile_id,target_account_id,target_profile_id,
 frozen_authority_epoch,freeze_receipt_id,request_hash,receipt_id,state)
 VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,'pending')`, conversion, uuid.New(), uuid.New(), uuid.New(), source, uuid.New(), target, uuid.New(), make([]byte, 32), uuid.New())
	require.NoError(t, err)
	ref, err := journal.PrepareCutoverDrain(ctx, uuid.New(), cut)
	require.NoError(t, err, "pending conversion without a membership must be sealed, never omitted")
	work, err := journal.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, GameConversionSource, work.SourceKind)
	require.Contains(t, string(work.SealedTargetManifestBytes), conversion.String())
	require.Contains(t, string(work.SealedTargetManifestBytes), provisioned.RoomId)
	require.Contains(t, string(work.SealedTargetManifestBytes), "voice_game_session_closures", "empty sources remain explicit")
	require.NotEmpty(t, work.UnknownIssuanceMarkers, "absence of a grant ledger cannot prove no historical issuance")
	require.Empty(t, work.CapturedProviderBindings, "unknown history must not be mapped to current config")
	replay, err := journal.PrepareCutoverDrain(ctx, ref.WorkID, cut)
	require.NoError(t, err)
	require.Equal(t, ref, replay)
	_, err = pool.Exec(ctx, `UPDATE voice_sdk_conversion_fence_receipts SET target_session_conflict=true WHERE operation_id=$1`, conversion)
	require.NoError(t, err)
	again, err := journal.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	require.Equal(t, work, again, "lost ack/restart reads original sealed raw targets, not mutable live tables")
	changed := cut
	changed.CoverageEpoch++
	_, err = journal.PrepareCutoverDrain(ctx, ref.WorkID, changed)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	changedRef := ref
	changedRef.RequestHash[0] ^= 1
	_, err = journal.ReadOwnerWork(ctx, changedRef)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
	journal.Authority = cutoverFixtureAuthority{retirementproof.ErrPending}
	_, err = journal.ReadOwnerWork(ctx, ref)
	require.ErrorIs(t, err, retirementproof.ErrPending, "restart rereads actual maintenance authority; stored bytes alone cannot authorize")
	journal.Authority = cutoverFixtureAuthority{}
	_, err = pool.Exec(ctx, `UPDATE voice_owned_game_cutover_work SET work_bytes=$2 WHERE work_id=$1`, ref.WorkID, []byte(`{}`))
	require.Error(t, err, "sealed work is append-only even to its DB owner")
	_, err = pool.Exec(ctx, `DELETE FROM voice_owned_game_cutover_work WHERE work_id=$1`, ref.WorkID)
	require.Error(t, err)
}

func TestOwnedGameCutover_NoAuthorityNoSealing(t *testing.T) {
	ctx := context.Background()
	pool := cutoverTestDB(t)
	j := &CutoverWorkStore{Pool: pool, SourceKind: GameConversionSource}
	_, err := j.PrepareCutoverDrain(ctx, uuid.New(), cutoverTestBinding())
	require.ErrorIs(t, err, retirementproof.ErrUnavailable)
	j.Authority = cutoverFixtureAuthority{retirementproof.ErrPending}
	_, err = j.PrepareCutoverDrain(ctx, uuid.New(), cutoverTestBinding())
	require.ErrorIs(t, err, retirementproof.ErrPending)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_owned_game_cutover_work`).Scan(&count))
	require.Zero(t, count)
}

func TestOwnedGameCutover_MatchPendingHistoricalAndNewFence(t *testing.T) {
	ctx := context.Background()
	pool := cutoverTestDB(t)
	room, match, profile, account, epoch := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	create, member, grant := uuid.New(), uuid.New(), uuid.New()
	hash := make([]byte, 32)
	hash[0] = 1
	_, err := pool.Exec(ctx, `INSERT INTO voice_room_instances
 (room_id,livekit_room_name,state,roster_version,room_type,purpose,chat_id,owner_id,creation_operation_id,creation_manifest_hash,creation_receipt_id,chat_creation_receipt_id,created_at,updated_at)
 VALUES($1,$2,'active',0,'group_voice','MATCH_SQUAD',$3,$4,$5,$6,$7,$8,clock_timestamp(),clock_timestamp())`, room, "match-squad-"+room.String(), uuid.New(), match, create, hash, uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_match_squad_operations
 (operation_id,match_id,room_id,chat_id,owner_id,creation_receipt_id,chat_creation_receipt_id,participant_manifest_sha256,chat_creation_receipt_sha256,create_request_sha256,create_request_bytes,create_receipt_bytes,state)
 VALUES($1,$2,$3,$4,$2,$5,$6,$7,$7,$7,$8,$8,'pending')`, create, match, room, uuid.New(), uuid.New(), uuid.New(), hash, []byte("retained-creation-fixture"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_match_squad_member_operations
 (operation_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,method,request_sha256,request_bytes,state,created_at,updated_at)
 VALUES($1,$2,$3,$4,$5,1,$6,'join',$7,$8,'pending',clock_timestamp(),clock_timestamp())`, member, match, room, profile, account, epoch, hash, []byte("pending-join-fixture"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_match_squad_member_effects
 (operation_id,effect_kind,match_id,room_id,profile_id,media_epoch,state,created_at,updated_at)
 VALUES($1,'join_projection',$2,$3,$4,$5,'pending',clock_timestamp(),clock_timestamp())`, member, match, room, profile, epoch)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_match_squad_member_grants
 (request_id,match_id,room_id,profile_id,account_id,session_epoch,media_epoch,issued_at,expires_at)
 VALUES($1,$2,$3,$4,$5,1,$6,statement_timestamp()-interval '2 hours',statement_timestamp()-interval '119 minutes')`, grant, match, room, profile, account, epoch)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO voice_profile_account_mappings(profile_id,account_id) VALUES($1,$2)`, profile, account)
	require.NoError(t, err)
	newerRoom := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO voice_account_voice_fences(account_id,profile_id,room_id,state) VALUES($1,$2,$3,'active')`, account, profile, newerRoom)
	require.NoError(t, err)
	j := &CutoverWorkStore{Pool: pool, Authority: cutoverFixtureAuthority{}, SourceKind: OwnedRoomSource}
	ref, err := j.PrepareCutoverDrain(ctx, uuid.New(), cutoverTestBinding())
	require.NoError(t, err)
	work, err := j.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	raw := string(work.SealedTargetManifestBytes)
	for _, id := range []string{create.String(), member.String(), grant.String(), newerRoom} {
		require.Contains(t, raw, id)
	}
	require.Contains(t, raw, "voice_match_squad_member_effects")
	var memberships int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_room_memberships`).Scan(&memberships))
	require.Zero(t, memberships)
	var state, roomRef string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state,room_id FROM voice_account_voice_fences WHERE account_id=$1`, account).Scan(&state, &roomRef))
	require.Equal(t, "active", state)
	require.Equal(t, newerRoom, roomRef, "capture cannot release a newer owner fence")
	// Caller/deserialized lists cannot suppress the derived unknown history.
	work.UnknownIssuanceMarkers = nil
	forged, err := json.Marshal(work)
	require.NoError(t, err)
	_, err = decodeCutoverWork(forged, ref, OwnedRoomSource)
	require.ErrorIs(t, err, retirementproof.ErrConflict)
}

func cutoverTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("VOICE_OWNED_CUTOVER_TEST_DSN")
	if dsn == "" {
		t.Skip("set VOICE_OWNED_CUTOVER_TEST_DSN for isolated PostgreSQL16 source/journal proof")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	schema := "owned_cut_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, `CREATE SCHEMA `+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, `DROP SCHEMA `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
		admin.Close()
	})
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "voice_db")
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	require.NoError(t, err)
	for _, path := range files {
		name := filepath.Base(path)
		if name >= "000015" && name < "000026" {
			continue
		}
		body, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, name)
	}
	return pool
}
func cutoverTestBinding() retirementproof.CutBinding {
	var cut retirementproof.CutBinding
	// All nonzero identity/hash fields are explicit storage fixtures.
	requireFixture := []byte(`{"fixture":"storage-only"}`)
	d := retirementproof.Hash("fixture", requireFixture)
	cut.EnvironmentID = uuid.New()
	cut.AuthorityIncarnation = uuid.New()
	cut.CutoverOperationID = uuid.New()
	cut.CoverageEpoch = 1
	cut.ClosedAdmissionBarrierID = uuid.New()
	cut.PlanHash = d
	cut.DatabaseIdentity = d
	cut.SourceCatalogHash = d
	cut.SchemaContractHash = d
	cut.WriterManifestHash = d
	cut.ProviderManifestHash = d
	cut.CredentialManifestHash = d
	cut.ClosedAdmissionBarrierHash = d
	cut.IssuerImplementationManifestHash = d
	return cut
}

func TestOwnedGameCutover_RederivesOpaqueBindings(t *testing.T) {
	binding := retirementproof.Hash("fixture", []byte("provider"))
	raw, _ := json.Marshal([]cutoverTarget{{Source: "voice_fixture", Row: json.RawMessage(`{"provider_binding":"\\x` + hex.EncodeToString(binding[:]) + `","state":"expired"}`)}})
	providers, markers, err := deriveCutoverIssuance(raw)
	require.NoError(t, err)
	require.Equal(t, []retirementproof.Digest{binding}, providers)
	require.NotEmpty(t, markers)
	_, _, err = deriveCutoverIssuance([]byte(`[{"Source":"fixture","Row":{"provider_binding":"garbage"}}]`))
	require.ErrorIs(t, err, retirementproof.ErrConflict)
}
