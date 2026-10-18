package partysnapshot

import (
	"context"
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/pkg/partyauthority"
)

func authorityFixture() (Subject, Candidate) {
	id := func(prefix string) uuid.UUID { return uuid.MustParse(prefix + "-0000-4000-8000-000000000001") }
	subject := Subject{id("40000000"), id("10000000"), id("20000000"), 11}
	cut := Candidate{Coverage: CoverageCut{id("60000000"), id("70000000"), 3, 4, [32]byte{1}}, Disposition: Present, SourceKind: "ORDINARY", RoomID: id("30000000"), Incarnation: id("50000000"), Purpose: "ORDINARY", RoomState: "ACTIVE", RosterVersion: 2, Fingerprint: [32]byte{1}, DatabaseTime: testNow,
		Members: []Member{{AccountID: id("10000000"), ProfileID: id("20000000"), OperationID: id("80000000"), Generation: "90000000-0000-4000-8000-000000000001", SessionEpoch: 7, State: Joined}, {AccountID: uuid.MustParse("10000000-0000-4000-8000-000000000002"), ProfileID: testPeer, OperationID: uuid.MustParse("80000000-0000-4000-8000-000000000002"), Generation: "90000000-0000-4000-8000-000000000002", SessionEpoch: 9, State: Joined}}}
	return subject, cut
}
func TestPrivateSnapshotAuthorityMatchesFixedBinaryContract(t *testing.T) {
	subject, cut := authorityFixture()
	result, err := buildSnapshot(subject, cut)
	require.NoError(t, err)
	require.Equal(t, partyauthority.Version, result.MemberAuthorityVersion)
	require.Len(t, result.MemberAuthorities, 2)
	require.Equal(t, int64(7), result.MemberAuthorities[0].SessionEpoch, "native admitted epoch must not be replaced by initiating device epoch11")
	require.Equal(t, "c823d784054c783da66790fe15bd516fb3cfbd47b74e5f07c1acc2b4ec5e8f41", hex.EncodeToString(result.SourceAuthorityHash[:]))
	for i, m := range result.MemberAuthorities {
		require.Equal(t, result.MemberProfileIDs[i], m.ProfileID)
		require.Equal(t, cut.Members[i].AccountID, m.AccountID)
	}
}
func TestAuthorityDigestChangesForIdentityABAAndCoverageButNotSampleTime(t *testing.T) {
	subject, cut := authorityFixture()
	base, err := buildSnapshot(subject, cut)
	require.NoError(t, err)
	for _, mutate := range []func(*Candidate){func(c *Candidate) { c.Incarnation = uuid.New() }, func(c *Candidate) { c.Coverage.CoverageID = uuid.New() }, func(c *Candidate) { c.Coverage.AuthorityIncarnation = uuid.New() }, func(c *Candidate) { c.Coverage.CoverageEpoch++ }, func(c *Candidate) { c.Coverage.HeadRevision++ }, func(c *Candidate) { c.Coverage.ReceiptHash[0]++ }, func(c *Candidate) { c.Members[1].OperationID = uuid.New() }, func(c *Candidate) { c.Members[1].Generation = "changed-native-generation" }, func(c *Candidate) { c.Members[1].SessionEpoch++ }, func(c *Candidate) { c.Members[1].AccountID = uuid.New() }} {
		next := cut
		next.Members = append([]Member(nil), cut.Members...)
		mutate(&next)
		result, err := buildSnapshot(subject, next)
		require.NoError(t, err)
		require.Equal(t, base.MemberProfileIDs, result.MemberProfileIDs)
		require.Equal(t, base.RosterVersion, result.RosterVersion)
		require.NotEqual(t, base.SourceAuthorityHash, result.SourceAuthorityHash)
	}
	cut.DatabaseTime = cut.DatabaseTime.Add(time.Second)
	unchanged, err := buildSnapshot(subject, cut)
	require.NoError(t, err)
	require.Equal(t, base.SourceAuthorityHash, unchanged.SourceAuthorityHash)
	cut.SourceKind = "SPACE"
	cut.Members[1].Generation = "opaque/space-generation"
	_, err = buildSnapshot(subject, cut)
	require.NoError(t, err)
}
func TestSnapshotExportsOnlySameValidatedCutAndSoloSubject(t *testing.T) {
	subject, cut := authorityFixture()
	engine := testEngine(&scriptedReader{steps: []Candidate{cut, cut}}, &recordedFloors{value: 6}, nil)
	result, err := engine.Snapshot(context.Background(), subject)
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.MemberAuthorityVersion)
	require.Len(t, result.MemberAuthorities, 2)
	absent := Candidate{Coverage: cut.Coverage, Disposition: Absent, Fingerprint: [32]byte{1}, DatabaseTime: testNow}
	engine = testEngine(&scriptedReader{steps: []Candidate{absent, absent}}, &recordedFloors{value: 6}, nil)
	solo, err := engine.Snapshot(context.Background(), subject)
	require.NoError(t, err)
	require.Equal(t, []partyauthority.Member{{ProfileID: subject.ProfileID, AccountID: subject.AccountID, SessionEpoch: subject.SessionEpoch}}, solo.MemberAuthorities)
	require.NotEqual(t, [32]byte{}, solo.SourceAuthorityHash)
}
func TestPrivateAuthorityRejectsDuplicateAccountsAndInvalidNativeGeneration(t *testing.T) {
	for _, mutate := range []func(*Candidate){func(c *Candidate) { c.Members[1].AccountID = c.Members[0].AccountID }, func(c *Candidate) { c.Members[1].Generation = string([]byte{0xff}) }} {
		subject, cut := authorityFixture()
		mutate(&cut)
		_, err := buildSnapshot(subject, cut)
		require.ErrorIs(t, err, ErrUnavailable)
	}
}
