package partysnapshot

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	testAccount     = uuid.MustParse("10000000-0000-4000-8000-000000000001")
	testPeerAccount = uuid.MustParse("10000000-0000-4000-8000-000000000002")
	testProfile     = uuid.MustParse("20000000-0000-4000-8000-000000000001")
	testPeer        = uuid.MustParse("20000000-0000-4000-8000-000000000002")
	testRoom        = uuid.MustParse("30000000-0000-4000-8000-000000000001")
	testNow         = time.Date(2026, 10, 18, 20, 0, 0, 0, time.UTC)
)

type scriptedReader struct {
	steps []Candidate
	reads int
}

func (r *scriptedReader) Read(context.Context, Subject) (Candidate, error) {
	if r.reads >= len(r.steps) {
		return Candidate{}, ErrUnavailable
	}
	result := r.steps[r.reads]
	r.reads++
	return result, nil
}

type recordedFloors struct {
	value    int64
	err      error
	accounts []string
}

func (f *recordedFloors) CurrentFloor(_ context.Context, account string) (int64, error) {
	f.accounts = append(f.accounts, account)
	return f.value, f.err
}

type recordedNormalizer struct {
	targets []LossTarget
	err     error
}

func (n *recordedNormalizer) ApplyEligibilityLoss(_ context.Context, target LossTarget) error {
	n.targets = append(n.targets, target)
	return n.err
}

func testSubject() Subject {
	return Subject{SearchOperationID: uuid.New(), AccountID: testAccount, ProfileID: testProfile, SessionEpoch: 7}
}

func testMember(profile uuid.UUID) Member {
	account := testAccount
	if profile == testPeer {
		account = testPeerAccount
	}
	return Member{AccountID: account, ProfileID: profile, OperationID: uuid.New(), Generation: uuid.NewString(), SessionEpoch: 7, State: Joined}
}

func testRoster() Candidate {
	return Candidate{Coverage: testCoverage(), Disposition: Present, SourceKind: "ORDINARY", RoomID: testRoom,
		Incarnation: testRoom, Purpose: "ORDINARY", RoomState: "ACTIVE", RosterVersion: 2,
		Members: []Member{testMember(testPeer), testMember(testProfile)}, Fingerprint: [32]byte{1}, DatabaseTime: testNow}
}

func testCoverage() CoverageCut {
	return CoverageCut{CoverageID: testRoom, AuthorityIncarnation: testAccount, CoverageEpoch: 1, HeadRevision: 1, ReceiptHash: [32]byte{1}}
}

func testEngine(reader *scriptedReader, floors *recordedFloors, normalizer *recordedNormalizer) *Engine {
	return &Engine{Reader: reader, Floors: floors, Normalizer: normalizer, Clock: func() time.Time { return testNow }}
}

func TestSnapshotUsesExactSecondPassAndFreshAccountFloor(t *testing.T) {
	candidate := testRoster()
	reader := &scriptedReader{steps: []Candidate{candidate, candidate}}
	floors := &recordedFloors{value: 6}
	result, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.ProtocolVersion)
	require.Equal(t, "VOICE_ROSTER", result.Kind)
	require.Equal(t, testRoom, result.RoomID)
	require.Equal(t, uint64(2), result.RosterVersion)
	require.Equal(t, []uuid.UUID{testProfile, testPeer}, result.MemberProfileIDs)
	require.Equal(t, 2, reader.reads)
	require.Equal(t, []string{testAccount.String(), testPeerAccount.String()}, floors.accounts, "one fresh floor for each distinct native account per attempt")
}

func TestSnapshotGenuineAbsenceRequiresTwoConsistentPassesAndAuth(t *testing.T) {
	absent := Candidate{Coverage: testCoverage(), Disposition: Absent, Fingerprint: [32]byte{1}, DatabaseTime: testNow}
	reader := &scriptedReader{steps: []Candidate{absent, absent}}
	floors := &recordedFloors{value: 7}
	result, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, "SOLO", result.Kind)
	require.Equal(t, uuid.Nil, result.RoomID)
	require.Zero(t, result.RosterVersion)
	require.Equal(t, []uuid.UUID{testProfile}, result.MemberProfileIDs)
	require.Len(t, floors.accounts, 1)
	require.Equal(t, 2, reader.reads)
}

func TestSnapshotUnknownCoverageNeverManufacturesSolo(t *testing.T) {
	reader := &scriptedReader{steps: []Candidate{{Disposition: Absent, Fingerprint: [32]byte{1}, DatabaseTime: testNow}}}
	_, err := testEngine(reader, &recordedFloors{value: 7}, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestSnapshotRereadsAuthWhenNativeGenerationChanges(t *testing.T) {
	first := testRoster()
	changed := first
	changed.Members = append([]Member(nil), first.Members...)
	changed.Members[0].Generation = uuid.NewString()
	changed.Fingerprint = [32]byte{2}
	reader := &scriptedReader{steps: []Candidate{first, changed, changed, changed}}
	floors := &recordedFloors{value: 7}
	_, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, 4, reader.reads)
	require.Len(t, floors.accounts, 4, "cannot reuse either account's positive Auth evidence across a retry")
}

func TestSnapshotStaleInitiatorNormalizesExactGenerationAndNeverReturnsSolo(t *testing.T) {
	candidate := testRoster()
	candidate.Members[1].SessionEpoch = 6
	reader := &scriptedReader{steps: []Candidate{candidate}}
	normalizer := &recordedNormalizer{}
	_, err := testEngine(reader, &recordedFloors{value: 7}, normalizer).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrIneligible)
	require.Len(t, normalizer.targets, 1)
	require.Equal(t, candidate.Members[1].Generation, normalizer.targets[0].Member.Generation)
	require.Equal(t, "SESSION_REVOKED", normalizer.targets[0].Reason)
}

func TestSnapshotExpiredReconnectNormalizesAtDeadlineEquality(t *testing.T) {
	candidate := testRoster()
	candidate.Members[1].State = Reconnecting
	candidate.Members[1].ReconnectStartedAt = testNow.Add(-30 * time.Second)
	candidate.Members[1].ReconnectDeadline = testNow
	normalizer := &recordedNormalizer{}
	_, err := testEngine(&scriptedReader{steps: []Candidate{candidate}}, &recordedFloors{value: 7}, normalizer).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrIneligible)
	require.Len(t, normalizer.targets, 1)
	require.Equal(t, "RECONNECT_EXPIRED", normalizer.targets[0].Reason)
}

func TestSnapshotUnknownAuthCannotBecomeEligibilityLoss(t *testing.T) {
	candidate := testRoster()
	normalizer := &recordedNormalizer{}
	_, err := testEngine(&scriptedReader{steps: []Candidate{candidate}}, &recordedFloors{value: 0}, normalizer).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Empty(t, normalizer.targets)
}

func TestSnapshotRejectsIncompleteOrIneligibleNativeEvidence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Candidate)
		want   error
	}{
		{"unknown epoch", func(c *Candidate) { c.Members[0].SessionEpoch = 0 }, ErrUnavailable},
		{"unknown source", func(c *Candidate) { c.SourceKind = "REDIS" }, ErrUnavailable},
		{"pending source", func(c *Candidate) { c.Disposition = Unresolved }, ErrUnavailable},
		{"unknown state", func(c *Candidate) { c.Members[0].State = "MISSING" }, ErrUnavailable},
		{"unknown version", func(c *Candidate) { c.RosterVersion = 0 }, ErrUnavailable},
		{"duplicate profile", func(c *Candidate) { c.Members = append(c.Members, c.Members[0]) }, ErrUnavailable},
		{"duplicate account", func(c *Candidate) { c.Members[0].AccountID = c.Members[1].AccountID }, ErrUnavailable},
		{"invalid generation encoding", func(c *Candidate) { c.Members[0].Generation = string([]byte{0xff}) }, ErrUnavailable},
		{"wrong account", func(c *Candidate) { c.Members[1].AccountID = uuid.New() }, ErrUnavailable},
		{"managed Game", func(c *Candidate) { c.Purpose = "GAME" }, ErrIneligible},
		{"MatchSquad", func(c *Candidate) { c.Purpose = "MATCH_SQUAD" }, ErrIneligible},
		{"initiator leaving", func(c *Candidate) { c.Members[1].State = Leaving }, ErrIneligible},
		{"missing reconnect start", func(c *Candidate) {
			c.Members[0].State = Reconnecting
			c.Members[0].ReconnectDeadline = testNow.Add(time.Second)
		}, ErrUnavailable},
		{"excessive reconnect", func(c *Candidate) {
			c.Members[0].State = Reconnecting
			c.Members[0].ReconnectStartedAt = testNow
			c.Members[0].ReconnectDeadline = testNow.Add(31 * time.Second)
		}, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := testRoster()
			tc.change(&candidate)
			floors, normalizer := &recordedFloors{value: 7}, &recordedNormalizer{}
			_, err := testEngine(&scriptedReader{steps: []Candidate{candidate}}, floors, normalizer).Snapshot(context.Background(), testSubject())
			require.ErrorIs(t, err, tc.want)
			require.Empty(t, floors.accounts, "no Auth IO before complete native evidence")
			require.Empty(t, normalizer.targets, "unknown evidence is never mutated")
		})
	}
}

func TestSnapshotStaleRequestDoesNotNormalizeNativeMembers(t *testing.T) {
	normalizer := &recordedNormalizer{}
	_, err := testEngine(&scriptedReader{steps: []Candidate{testRoster()}}, &recordedFloors{value: 8}, normalizer).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrStaleSubject)
	require.Empty(t, normalizer.targets)
}

func TestSnapshotDoesNotTrustFingerprintAlone(t *testing.T) {
	first := testRoster()
	changed := first
	changed.Members = append([]Member(nil), first.Members...)
	changed.Members[0].Generation = uuid.NewString()
	reader := &scriptedReader{steps: []Candidate{first, changed, changed, changed}}
	floors := &recordedFloors{value: 7}
	_, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, 4, reader.reads)
	require.Len(t, floors.accounts, 4)
}

func TestSnapshotRetryBudgetIsBounded(t *testing.T) {
	first, second := testRoster(), testRoster()
	reader := &scriptedReader{steps: []Candidate{first, second, first, second, first, second}}
	floors := &recordedFloors{value: 7}
	_, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Equal(t, 6, reader.reads)
	require.Len(t, floors.accounts, 6)
}

func TestSnapshotPeerRevocationRestartsWithFreshAuth(t *testing.T) {
	first := testRoster()
	first.Members[0].SessionEpoch = 6
	remaining := first
	remaining.RosterVersion++
	remaining.Members = []Member{first.Members[1]}
	remaining.Fingerprint = [32]byte{2}
	normalizer, floors := &recordedNormalizer{}, &recordedFloors{value: 7}
	result, err := testEngine(&scriptedReader{steps: []Candidate{first, remaining, remaining}}, floors, normalizer).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{testProfile}, result.MemberProfileIDs)
	require.Equal(t, "VOICE_ROSTER", result.Kind)
	require.Len(t, normalizer.targets, 1)
	require.Equal(t, first.Members[0].Generation, normalizer.targets[0].Member.Generation)
	require.Len(t, floors.accounts, 3)
}

func TestSnapshotReconnectAllowsOnlyUnexpiredImmutableInterval(t *testing.T) {
	candidate := testRoster()
	candidate.Members[0].State = Reconnecting
	candidate.Members[0].ReconnectStartedAt = testNow.Add(-10 * time.Second)
	candidate.Members[0].ReconnectDeadline = testNow.Add(20 * time.Second)
	_, err := testEngine(&scriptedReader{steps: []Candidate{candidate, candidate}}, &recordedFloors{value: 7}, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
}

func TestSnapshotLocalElapsedCannotReturnExpiredReconnect(t *testing.T) {
	candidate := testRoster()
	candidate.Members[0].State = Reconnecting
	candidate.Members[0].ReconnectStartedAt = testNow.Add(-29 * time.Second)
	candidate.Members[0].ReconnectDeadline = testNow.Add(time.Second)
	reader := &scriptedReader{steps: []Candidate{candidate, candidate, candidate, candidate, candidate, candidate}}
	normalizer := &recordedNormalizer{}
	engine := testEngine(reader, &recordedFloors{value: 7}, normalizer)
	ticks := 0
	engine.Clock = func() time.Time { ticks++; return testNow.Add(time.Duration(ticks) * time.Second) }
	_, err := engine.Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Empty(t, normalizer.targets, "local elapsed is not a native expiry receipt")
}

func TestSnapshotCoverageChangeRequiresFreshAuthEvenWithSameFingerprint(t *testing.T) {
	first := testRoster()
	second := first
	second.Coverage.CoverageEpoch++
	reader := &scriptedReader{steps: []Candidate{first, second, second, second}}
	floors := &recordedFloors{value: 7}
	_, err := testEngine(reader, floors, &recordedNormalizer{}).Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, 4, reader.reads)
	require.Len(t, floors.accounts, 4)
}

func TestSnapshotUnknownPeerFloorCannotPartiallyNormalizeKnownLoss(t *testing.T) {
	candidate := testRoster()
	candidate.Members[0].AccountID = uuid.MustParse("10000000-0000-4000-8000-000000000002")
	candidate.Members[1].SessionEpoch = 6
	normalizer := &recordedNormalizer{}
	engine := &Engine{Reader: &scriptedReader{steps: []Candidate{candidate}}, Normalizer: normalizer, Floors: floorFunc(func(_ context.Context, account string) (int64, error) {
		if account == testAccount.String() {
			return 7, nil
		}
		return 0, ErrUnavailable
	})}
	_, err := engine.Snapshot(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Empty(t, normalizer.targets)
}
