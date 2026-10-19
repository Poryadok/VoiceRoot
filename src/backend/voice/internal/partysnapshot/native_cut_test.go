package partysnapshot

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/spacemedia"
)

func TestNativeSourceNeverInfersAbsenceWithoutCoverage(t *testing.T) {
	pool, _ := postgresFixture(t)
	bindings := []partycoverage.RuntimeBinding{{}, {
		EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CoverageEpoch: 1,
		DatabaseIdentity: [32]byte{1}, SourceCatalogHash: [32]byte{2}, SchemaContractHash: [32]byte{3}, WriterManifestHash: [32]byte{4},
	}}
	for i, binding := range bindings {
		r := &PostgresReader{Database: pool, Source: &NativeSource{Binding: binding}}
		_, err := r.Read(context.Background(), testSubject())
		require.ErrorIs(t, err, ErrUnavailable, "case %d: missing installation/receipt cannot certify SOLO", i)
		require.Zero(t, pool.Stat().AcquiredConns())
	}
}

// These are DTO composition unit tests, not issued coverage or cold-cut proof.
func TestNativeCompositionPreservesOpaqueIdentityAndAllWitnesses(t *testing.T) {
	coverage := testRoster().Coverage
	account, profile, operation, generation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	room := &ordinarycall.Room{Intent: ordinarycall.CreateIntent{OperationID: uuid.New(), RoomID: uuid.New()}, Stage: ordinarycall.StageActive, RosterVersion: 7,
		Members: map[string]ordinarycall.Member{profile.String(): {Identity: ordinarycall.Identity{AccountID: account, ProfileID: profile, SessionEpoch: 3}, FenceOperationID: operation, Generation: generation, State: ordinarycall.MemberJoined}}}
	ordinary := ordinarycall.NativePartyCut{Room: room, Witness: []byte(`{"ordinary":"one"}`)}
	space := spacemedia.NativePartyCut{Witness: []byte(`{"space_rooms":[]}`)}
	foreign := foreignCut{Witness: []byte(`{"foreign":[]}`)}
	cut, err := composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.Equal(t, coverage, cut.Coverage)
	require.Equal(t, Present, cut.Disposition)
	require.Equal(t, "ORDINARY", cut.SourceKind)
	require.Equal(t, room.Intent.OperationID, cut.Incarnation)
	require.Equal(t, uint64(7), cut.RosterVersion)
	require.Len(t, cut.Members, 1)
	require.Equal(t, operation, cut.Members[0].OperationID)
	require.Equal(t, generation.String(), cut.Members[0].Generation)
	require.Equal(t, int64(3), cut.Members[0].SessionEpoch)
	require.NotEqual(t, [32]byte{}, cut.Fingerprint)
	foreign.Witness = []byte(`{"foreign":["changed"]}`)
	changed, err := composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.NotEqual(t, cut.Fingerprint, changed.Fingerprint)
	foreign.Witness = []byte(`{"foreign":[]}`)
	coverage.HeadRevision++
	changed, err = composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.NotEqual(t, cut.Fingerprint, changed.Fingerprint)
	space = spacemedia.NativePartyCut{Head: &spacemedia.PartyRoomHead{RoomID: uuid.New(), CreatorOperationID: uuid.New(), RosterVersion: 4, State: "OPEN"},
		Members: []spacemedia.PartyMember{{OperationID: operation, AccountID: account, ProfileID: profile, Generation: "space:opaque/generation", SessionEpoch: 9, State: "ACTIVE"}}, Witness: []byte(`{"space":"one"}`)}
	ambiguous, err := composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.Equal(t, Unresolved, ambiguous.Disposition)
	ordinary = ordinarycall.NativePartyCut{Witness: []byte(`{"ordinary_rooms":[]}`)}
	cut, err = composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.Equal(t, "SPACE", cut.SourceKind)
	require.Equal(t, "space:opaque/generation", cut.Members[0].Generation)
	require.Equal(t, space.Head.CreatorOperationID, cut.Incarnation)
	foreign.Unresolved = true
	cut, err = composeNativeCandidate(coverage, ordinary, space, foreign)
	require.NoError(t, err)
	require.Equal(t, Unresolved, cut.Disposition)
}
