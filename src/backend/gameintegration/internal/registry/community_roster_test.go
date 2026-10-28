package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCommunityRosterSnapshotRequiresCompleteCanonicalPages(t *testing.T) {
	member := CommunityRosterMember{
		BindingID: uuid.New(), CharacterKey: "character-opaque-1",
		SourceMembershipRevision: 4, SourceRank: "member", GrantReasons: []string{"membership"},
	}
	in := CommunityRosterSnapshotInput{
		OperationID: uuid.New(), CorporationKey: "corp-1", SourceRevision: 8,
		OwnerGeneration: 3, Pages: []CommunityRosterPage{{Index: 0, Count: 2, Members: []CommunityRosterMember{member}}},
	}
	setTestCommunityRosterHashes(t, &in)
	_, err := validateCommunityRosterSnapshot(in)
	require.Error(t, err, "a partial provider fetch cannot activate or revoke any community grant")

	second := CommunityRosterPage{Index: 1, Count: 2, Members: nil}
	in.Pages = append(in.Pages, second)
	setTestCommunityRosterHashes(t, &in)
	_, err = validateCommunityRosterSnapshot(in)
	require.NoError(t, err, "a complete empty trailing page is distinct from a missing page")

	in.Pages[1].Index = 0
	setTestCommunityRosterHashes(t, &in)
	_, err = validateCommunityRosterSnapshot(in)
	require.Error(t, err, "duplicate page indexes must not be normalized into a complete snapshot")
}

func TestApplyCommunityRosterSnapshotPersistsOwnerFencedGrantsAndNegativeRevocations(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	installT37CommunitySchema(t, store, ctx)
	principal := newT31TestSessionPrincipal(t, ctx, store)
	corp := "corp-roster-" + uuid.NewString()
	spaceID := uuid.New()
	_, err := store.Pool.Exec(ctx, `INSERT INTO gis_community_owner_authority
		(application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status,space_id)
		VALUES($1,$2,$3,$4,$5,4,'active',$6)`, principal.ApplicationID, principal.EnvironmentID,
		corp, uuid.New(), uuid.New(), spaceID)
	require.NoError(t, err)
	bindingA, bindingB := uuid.New(), uuid.New()
	insertCommunityRosterBinding(t, ctx, store, principal, bindingA, "character-a")
	insertCommunityRosterBinding(t, ctx, store, principal, bindingB, "character-b")

	first := testCommunityRosterInput(t, corp, 1, 4,
		CommunityRosterMember{BindingID: bindingA, CharacterKey: "character-a", SourceMembershipRevision: 10, SourceRank: "officer", GrantReasons: []string{"membership", "officer_rank"}},
		CommunityRosterMember{BindingID: bindingB, CharacterKey: "character-b", SourceMembershipRevision: 3, SourceRank: "member", GrantReasons: []string{"membership"}},
	)
	accepted, err := store.ApplyCommunityRosterSnapshot(ctx, principal, first)
	require.NoError(t, err)
	require.Equal(t, "accepted", accepted.Outcome)
	require.EqualValues(t, 2, accepted.MemberCount)
	require.NotNil(t, accepted.LeaseExpiresAt)
	require.WithinDuration(t, time.Now().Add(60*time.Second), *accepted.LeaseExpiresAt, 5*time.Second)
	var activeGrants int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_community_grants
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND revoked_at IS NULL`,
		principal.ApplicationID, principal.EnvironmentID, corp).Scan(&activeGrants))
	require.Equal(t, 3, activeGrants, "membership and officer rank are separate durable reasons")
	projection, err := store.BuildCommunityRosterProjection(ctx, principal, first)
	require.NoError(t, err)
	require.Equal(t, spaceID, projection.SpaceID)
	require.Len(t, projection.ProfileIDs, 2, "GIS resolves only active bound profile IDs for the internal Space projection")
	projectionBytes, err := json.Marshal(struct {
		ApplicationID   uuid.UUID   `json:"application_id"`
		EnvironmentID   uuid.UUID   `json:"environment_id"`
		CorporationKey  string      `json:"corporation_key"`
		SpaceID         uuid.UUID   `json:"space_id"`
		OwnerGeneration int64       `json:"owner_generation"`
		SourceRevision  int64       `json:"source_revision"`
		SnapshotSHA256  []byte      `json:"snapshot_sha256"`
		LeaseExpiresAt  time.Time   `json:"lease_expires_at"`
		ProfileIDs      []uuid.UUID `json:"profile_ids"`
	}{projection.ApplicationID, projection.EnvironmentID, projection.CorporationKey, projection.SpaceID,
		projection.OwnerGeneration, projection.SourceRevision, projection.SnapshotSHA256,
		projection.LeaseExpiresAt.UTC(), projection.ProfileIDs})
	require.NoError(t, err)
	spaceRequestHash := sha256.Sum256(projectionBytes)
	spaceReceipt := CommunityRosterSpaceReceipt{OperationID: projection.OperationID, SpaceID: projection.SpaceID,
		ReceiptID: uuid.New(), RequestHash: spaceRequestHash[:], MemberCount: int32(len(projection.ProfileIDs))}
	require.NoError(t, store.RecordCommunityRosterSpaceReceipt(ctx, projection, spaceReceipt))
	var projectionStatus string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT space_projection_status FROM gis_community_roster_snapshots WHERE operation_id=$1`, first.OperationID).Scan(&projectionStatus))
	require.Equal(t, "applied", projectionStatus)

	replayed, err := store.ApplyCommunityRosterSnapshot(ctx, principal, first)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, accepted.LeaseExpiresAt, replayed.LeaseExpiresAt, "exact retry does not renew freshness")

	downgrade := testCommunityRosterInput(t, corp, 2, 4,
		CommunityRosterMember{BindingID: bindingA, CharacterKey: "character-a", SourceMembershipRevision: 11, SourceRank: "member", GrantReasons: []string{"membership"}},
	)
	downgraded, err := store.ApplyCommunityRosterSnapshot(ctx, principal, downgrade)
	require.NoError(t, err)
	require.EqualValues(t, 1, downgraded.MemberCount)
	var membershipActive, officerRevoked, departedRevoked int
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE source_reason='membership' AND revoked_at IS NULL),
		count(*) FILTER (WHERE source_reason='officer_rank' AND revoked_at IS NOT NULL),
		count(*) FILTER (WHERE character_key='character-b' AND revoked_at IS NOT NULL)
		FROM gis_community_grants WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`,
		principal.ApplicationID, principal.EnvironmentID, corp).Scan(&membershipActive, &officerRevoked, &departedRevoked))
	require.Equal(t, 1, membershipActive)
	require.Equal(t, 1, officerRevoked, "rank downgrade records a durable negative revocation")
	require.Equal(t, 1, departedRevoked, "omitted character records a durable negative revocation")
	view, err := store.GetCommunityRosterSnapshot(ctx, principal, corp)
	require.NoError(t, err)
	require.Len(t, view.Members, 1)
	require.Equal(t, bindingA, view.Members[0].BindingID)
	require.Equal(t, []string{"membership"}, view.Members[0].GrantReasons)
	require.NotContains(t, fmt.Sprintf("%+v", view), "profile_id")
	_, err = store.Pool.Exec(ctx, `UPDATE gis_community_grants SET lease_expires_at=clock_timestamp()-interval '1 microsecond'
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND revoked_at IS NULL`,
		principal.ApplicationID, principal.EnvironmentID, corp)
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 microsecond'
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`,
		principal.ApplicationID, principal.EnvironmentID, corp)
	require.NoError(t, err)
	revoked, err := store.RevokeExpiredCommunityRosterGrants(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, revoked, "expired source lease produces a durable negative revocation")
	var expiredReason string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT revocation_reason FROM gis_community_grants
		WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3 AND source_reason='membership' AND revoked_at IS NOT NULL
		ORDER BY source_revision DESC LIMIT 1`, principal.ApplicationID, principal.EnvironmentID, corp).Scan(&expiredReason))
	require.Equal(t, "source_expired", expiredReason)
	_, err = store.GetCommunityRosterSnapshot(ctx, principal, corp)
	require.ErrorIs(t, err, ErrStaleCommunityRoster, "stale roster reads fail closed before the cleanup worker runs")

	stale := testCommunityRosterInput(t, corp, 1, 4,
		CommunityRosterMember{BindingID: bindingA, CharacterKey: "character-a", SourceMembershipRevision: 10, SourceRank: "officer", GrantReasons: []string{"membership", "officer_rank"}},
	)
	_, err = store.ApplyCommunityRosterSnapshot(ctx, principal, stale)
	require.ErrorIs(t, err, ErrStaleCommunityRoster)
	changed := downgrade
	changed.OperationID = uuid.New()
	changed.Pages[0].Members[0].SourceMembershipRevision++
	setTestCommunityRosterHashes(t, &changed)
	_, err = store.ApplyCommunityRosterSnapshot(ctx, principal, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict, "same revision with changed source bytes conflicts")

	_, err = store.Pool.Exec(ctx, `UPDATE gis_community_owner_authority SET owner_generation=5 WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3`,
		principal.ApplicationID, principal.EnvironmentID, corp)
	require.NoError(t, err)
	_, err = store.ApplyCommunityRosterSnapshot(ctx, principal, testCommunityRosterInput(t, corp, 3, 4,
		CommunityRosterMember{BindingID: bindingA, CharacterKey: "character-a", SourceMembershipRevision: 12, SourceRank: "member", GrantReasons: []string{"membership"}},
	))
	require.ErrorIs(t, err, ErrInvalidCommunityRoster, "a stale Owner generation cannot reactivate grants")
}

func insertCommunityRosterBinding(t *testing.T, ctx context.Context, store *Store, p SessionPrincipal, binding uuid.UUID, characterKey string) {
	t.Helper()
	subjectDigest := sha256.Sum256(binding[:])
	_, err := store.Pool.Exec(ctx, `INSERT INTO player_bindings
		(binding_id,application_id,environment_id,provider,provider_subject_digest,account_id,actor_id,profile_id,device_id,
		 status,authority_revision,permitted_character_context)
		VALUES($1,$2,$3,'provider',$4,$5,$5,$6,$7,'active',1,$8::jsonb)`,
		binding, p.ApplicationID, p.EnvironmentID, "hmac-sha256-v1:test:"+hex.EncodeToString(subjectDigest[:]),
		uuid.New(), uuid.New(), uuid.New(), fmt.Sprintf(`["%s"]`, characterKey))
	require.NoError(t, err)
}

func testCommunityRosterInput(t *testing.T, corporation string, revision, generation int64, members ...CommunityRosterMember) CommunityRosterSnapshotInput {
	t.Helper()
	input := CommunityRosterSnapshotInput{OperationID: uuid.New(), CorporationKey: corporation,
		SourceRevision: revision, OwnerGeneration: generation,
		Pages: []CommunityRosterPage{{Index: 0, Count: 1, Members: members}}}
	// Hash setup is duplicated in the test, not delegated to the implementation under test.
	setTestCommunityRosterHashes(t, &input)
	return input
}

func TestCommunityRosterSnapshotRejectsChangedPageOrAggregateChecksum(t *testing.T) {
	in := CommunityRosterSnapshotInput{
		OperationID: uuid.New(), CorporationKey: "corp-1", SourceRevision: 8,
		OwnerGeneration: 3, Pages: []CommunityRosterPage{{Index: 0, Count: 1, Members: []CommunityRosterMember{{
			BindingID: uuid.New(), CharacterKey: "character-opaque-1",
			SourceMembershipRevision: 4, SourceRank: "member", GrantReasons: []string{"membership"},
		}}}},
	}
	setTestCommunityRosterHashes(t, &in)
	in.Pages[0].Members[0].SourceRank = "officer"
	_, err := validateCommunityRosterSnapshot(in)
	require.Error(t, err, "the caller cannot alter rank after hashing the page")

	setTestCommunityRosterHashes(t, &in)
	in.SnapshotSHA256[0] ^= 0xff
	_, err = validateCommunityRosterSnapshot(in)
	require.Error(t, err, "the aggregate digest binds the ordered page hashes and snapshot scope")
}

func setTestCommunityRosterHashes(t *testing.T, in *CommunityRosterSnapshotInput) {
	t.Helper()
	for i := range in.Pages {
		sort.Slice(in.Pages[i].Members, func(a, b int) bool {
			left := in.Pages[i].Members[a].BindingID.String() + "\x00" + in.Pages[i].Members[a].CharacterKey
			right := in.Pages[i].Members[b].BindingID.String() + "\x00" + in.Pages[i].Members[b].CharacterKey
			return left < right
		})
	}
	for i := range in.Pages {
		body, err := canonicalTestRosterJSON(struct {
			Index   int32                   `json:"index"`
			Count   int32                   `json:"count"`
			Members []CommunityRosterMember `json:"members"`
		}{in.Pages[i].Index, in.Pages[i].Count, in.Pages[i].Members})
		require.NoError(t, err)
		digest := sha256.Sum256(body)
		in.Pages[i].SHA256 = digest[:]
	}
	pageDigests := make([]string, len(in.Pages))
	for i, page := range in.Pages {
		pageDigests[i] = hex.EncodeToString(page.SHA256)
	}
	body, err := canonicalTestRosterJSON(struct {
		OperationID     string   `json:"operation_id"`
		CorporationKey  string   `json:"corporation_key"`
		SourceRevision  int64    `json:"source_revision"`
		OwnerGeneration int64    `json:"owner_generation"`
		Pages           []string `json:"pages"`
	}{in.OperationID.String(), in.CorporationKey, in.SourceRevision, in.OwnerGeneration, pageDigests})
	require.NoError(t, err)
	digest := sha256.Sum256(body)
	in.SnapshotSHA256 = digest[:]
}

func canonicalTestRosterJSON(value any) ([]byte, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return jsoncanonicalizer.Transform(body)
}
