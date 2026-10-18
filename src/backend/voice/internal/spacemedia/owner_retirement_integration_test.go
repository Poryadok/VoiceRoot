package spacemedia

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/retirementproof"
	"voice/backend/voice/internal/spacelifecycle"
)

func applyOwnerMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migration, err := os.ReadFile("../../../migrations/voice_db/000017_native_owner_retirement.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
}

type ownerMaintenanceFixture struct{ cut retirementproof.CutBinding }

func (f ownerMaintenanceFixture) CheckMaintenance(_ context.Context, cut retirementproof.CutBinding) error {
	if cut != f.cut {
		return retirementproof.ErrConflict
	}
	return nil
}

func ownerCutFixture() retirementproof.CutBinding {
	h := retirementproof.Hash("fixture", []byte("isolated-owner-component-not-resource-acceptance"))
	return retirementproof.CutBinding{EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CutoverOperationID: uuid.New(), CoverageEpoch: 1,
		PlanHash: h, DatabaseIdentity: h, SourceCatalogHash: h, SchemaContractHash: h, WriterManifestHash: h,
		ProviderManifestHash: h, CredentialManifestHash: h, ClosedAdmissionBarrierID: uuid.New(), ClosedAdmissionBarrierHash: h, IssuerImplementationManifestHash: h}
}

func TestNativeOwnerColdCutSealsCurrentJournalWithoutInventingP3Head(t *testing.T) {
	ctx := t.Context()
	pool := startSpaceMediaAdmissionPostgres(t, ctx)
	migration, err := os.ReadFile("../../../migrations/voice_db/000017_native_owner_retirement.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(migration))
	require.NoError(t, err)
	admissions := NewPostgresAdmissionStore(pool)
	a := admissionFixture()
	require.NoError(t, prepareAdmission(ctx, admissions, &a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	cut := ownerCutFixture()
	owner := &spacelifecycle.NativeRetirementOwner{Store: spacelifecycle.NewPostgresStore(pool, BeginPartyRoomClose), Authority: ownerMaintenanceFixture{cut}}
	request := spacelifecycle.ExpectedOwnerBinding{WorkID: uuid.New(), SpaceID: a.SpaceID, Purpose: retirementproof.ColdCutDrain,
		Head: retirementproof.OwnerHeadBinding{State: retirementproof.HeadAbsent, Hash: retirementproof.Hash(retirementproof.HeadDomain, []byte(retirementproof.HeadAbsent))}}
	ref, err := owner.PrepareCutoverDrain(ctx, cut, request)
	require.NoError(t, err)
	require.Equal(t, request.WorkID, ref.WorkID)
	var heads, targets int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_lifecycle_fence_heads WHERE space_id=$1`, a.SpaceID).Scan(&heads))
	require.Zero(t, heads, "maintenance must not invent a public deletion generation")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_native_owner_targets WHERE work_id=$1`, ref.WorkID).Scan(&targets))
	require.GreaterOrEqual(t, targets, 2)
	var state string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM voice_space_media_room_heads WHERE voice_room_id=$1 AND room_generation=$2`, a.VoiceRoomID, int64(a.RoomGeneration)).Scan(&state))
	require.Equal(t, "CLOSING", state)
	replayed, err := owner.PrepareCutoverDrain(ctx, cut, request)
	require.NoError(t, err)
	require.Equal(t, ref, replayed)
	changed := cut
	changed.CoverageEpoch++
	_, err = owner.PrepareCutoverDrain(ctx, changed, request)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE voice_native_owner_targets SET target_bytes=convert_to('{}','UTF8') WHERE work_id=$1`, ref.WorkID)
	require.Error(t, err, "sealed target evidence cannot be replaced")
	down, err := os.ReadFile("../../../migrations/voice_db/000017_native_owner_retirement.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.Error(t, err, "rollback must retain existing owner evidence")
	work, err := owner.ReadOwnerWork(ctx, ref)
	require.NoError(t, err)
	assertOwnerReaderRejectsOmittedBinding(t, ctx, pool, owner, work)
}

// Fixture insertion deliberately models corrupt/imported owner evidence. All
// hashes, request bytes, target rows and unknown markers are internally valid;
// the only omission is the binding extracted from the retained native target.
func assertOwnerReaderRejectsOmittedBinding(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner *spacelifecycle.NativeRetirementOwner, work retirementproof.OwnerWork) {
	t.Helper()
	work.Ref.WorkID = uuid.New()
	var request struct {
		Protocol string
		Cut      retirementproof.CutBinding
		Expected spacelifecycle.ExpectedOwnerBinding
	}
	require.NoError(t, json.Unmarshal(work.OwnerRequestBytes, &request))
	request.Expected.WorkID = work.Ref.WorkID
	work.OwnerRequestBytes = ownerFixtureJSON(t, request)
	work.Ref.RequestHash = retirementproof.Hash(work.OwnerRequestDomain, work.OwnerRequestBytes)
	var targets []struct {
		ID, Kind        string
		Data            json.RawMessage
		UnknownIssuance bool
	}
	require.NoError(t, json.Unmarshal(work.SealedTargetManifestBytes, &targets))
	binding := retirementproof.Hash("fixture-binding", []byte("retained-old-provider"))
	changed := false
	work.UnknownIssuanceMarkers = nil
	for i := range targets {
		if targets[i].Kind == "native" {
			var data map[string]any
			require.NoError(t, json.Unmarshal(targets[i].Data, &data))
			data["provider_binding"] = hex.EncodeToString(binding[:])
			targets[i].Data = ownerFixtureJSON(t, data)
			changed = true
		}
		if targets[i].UnknownIssuance {
			work.UnknownIssuanceMarkers = append(work.UnknownIssuanceMarkers, retirementproof.Hash("voice.native-space.retirement.unknown-entry.v1", ownerFixtureJSON(t, targets[i])))
		}
	}
	require.True(t, changed)
	sort.Slice(work.UnknownIssuanceMarkers, func(i, j int) bool {
		return bytes.Compare(work.UnknownIssuanceMarkers[i][:], work.UnknownIssuanceMarkers[j][:]) < 0
	})
	work.SealedTargetManifestBytes = ownerFixtureJSON(t, targets)
	work.SealedTargetManifestHash = retirementproof.Hash(retirementproof.TargetDomain, work.SealedTargetManifestBytes)
	for _, includeBinding := range []bool{false, true} {
		work.Ref.WorkID = uuid.New()
		request.Expected.WorkID = work.Ref.WorkID
		work.OwnerRequestBytes = ownerFixtureJSON(t, request)
		work.Ref.RequestHash = retirementproof.Hash(work.OwnerRequestDomain, work.OwnerRequestBytes)
		if includeBinding {
			work.CapturedProviderBindings = []retirementproof.Digest{binding}
		}
		workBytes := ownerFixtureJSON(t, work)
		workHash := retirementproof.Hash(retirementproof.WorkDomain, workBytes)
		_, err := pool.Exec(ctx, `INSERT INTO voice_native_owner_works(work_id,space_id,owner_protocol,purpose,request_sha256,work_bytes,work_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)`, work.Ref.WorkID, work.SpaceID, work.Ref.OwnerProtocol, work.Purpose, work.Ref.RequestHash[:], workBytes, workHash[:])
		require.NoError(t, err)
		for _, target := range targets {
			raw := ownerFixtureJSON(t, target)
			hash := retirementproof.Hash("voice.native-space.retirement.target-entry.v1", raw)
			_, err = pool.Exec(ctx, `INSERT INTO voice_native_owner_targets(work_id,target_id,source_kind,target_bytes,target_sha256) VALUES($1,$2,$3,$4,$5)`, work.Ref.WorkID, target.ID, target.Kind, raw, hash[:])
			require.NoError(t, err)
		}
		_, err = owner.ReadOwnerWork(ctx, work.Ref)
		if includeBinding {
			require.NoError(t, err, "control: same retained bytes with exhaustive binding list must validate")
		} else {
			require.Error(t, err, "owner reader must extract bindings, not trust an omitted caller list")
		}
	}
}
