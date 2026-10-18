package ordinarycall

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Transactional fake exercises orchestration only; real lock/fence proof is in
// PostgreSQL tests and cannot be inferred from these tests.
type memoryJournal struct {
	mu    sync.Mutex
	rooms map[uuid.UUID][]byte
}

func (s *memoryJournal) Create(_ context.Context, r *Room) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rooms == nil {
		s.rooms = map[uuid.UUID][]byte{}
	}
	if s.rooms[r.Intent.RoomID] != nil {
		return ErrConflict
	}
	b, err := json.Marshal(r)
	s.rooms[r.Intent.RoomID] = b
	return err
}
func (s *memoryJournal) Load(_ context.Context, id uuid.UUID) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rooms[id] == nil {
		return nil, ErrNotFound
	}
	return decodeRoom(s.rooms[id], id)
}
func (s *memoryJournal) Mutate(_ context.Context, id uuid.UUID, fn func(*Room) error) (*Room, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := decodeRoom(s.rooms[id], id)
	if err != nil {
		return nil, err
	}
	if err = fn(r); err != nil {
		return nil, err
	}
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	s.rooms[id] = b
	return r, nil
}
func (*memoryJournal) MarkProjected(context.Context, uuid.UUID, uint64) error { return nil }

type mediaFake struct {
	ensure  func(context.Context, string) error
	close   func(context.Context, string) error
	closes  int
	listed  []string
	removed []string
}

func (s *mediaFake) EnsureRoom(ctx context.Context, n string) error {
	if s.ensure != nil {
		return s.ensure(ctx, n)
	}
	return nil
}
func (s *mediaFake) CloseRoom(ctx context.Context, n string) error {
	s.closes++
	if s.close != nil {
		return s.close(ctx, n)
	}
	return nil
}
func (s *mediaFake) RemoveParticipant(_ context.Context, _ string, identity string) error {
	s.removed = append(s.removed, identity)
	return nil
}
func (s *mediaFake) ListParticipantIdentities(context.Context, string) ([]string, error) {
	return s.listed, nil
}

type projectionFake struct {
	err    error
	stages []Stage
}

func (p *projectionFake) Apply(_ context.Context, r *Room) error {
	p.stages = append(p.stages, r.Stage)
	return p.err
}

func coordinatorFixture() (*Coordinator, *memoryJournal, *mediaFake, *projectionFake) {
	j, m, p := &memoryJournal{}, &mediaFake{}, &projectionFake{}
	return &Coordinator{Journal: j, Media: m, Projection: p, EffectTimeout: time.Second, Now: func() time.Time { return testNow }}, j, m, p
}

func TestConversionCleanupUsesExactMemberAndPreservesOtherGroupParticipant(t *testing.T) {
	c, j, m, _ := coordinatorFixture()
	ctx := context.Background()
	r, owner, _ := pendingGroup(t)
	r, err := c.Start(ctx, r.Intent, nil)
	require.NoError(t, err)
	peer := identity()
	join := uuid.New()
	r, err = c.Admit(ctx, r.Intent.RoomID, join, func(r *Room) error { return r.BeginJoin(join, peer, testNow) }, nil)
	require.NoError(t, err)
	b, err := r.MediaBinding(owner.ProfileID)
	require.NoError(t, err)
	stale := b
	stale.Generation = uuid.New()
	require.ErrorIs(t, c.QuiesceConversionSource(ctx, stale), ErrStaleGeneration)
	require.Empty(t, m.removed)
	require.NoError(t, c.QuiesceConversionSource(ctx, b))
	current, err := j.Load(ctx, r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageActive, current.Stage)
	require.True(t, current.CanIssueToken(peer.ProfileID))
	require.False(t, current.CanIssueToken(owner.ProfileID))
	require.Equal(t, []string{b.MediaIdentity}, m.removed)
	require.Zero(t, m.closes)
	require.NoError(t, c.QuiesceConversionSource(ctx, b), "exact replay is idempotent")
}

func TestStartUnknownEnsureCompensatesExactNameWithoutPublishingReady(t *testing.T) {
	c, j, m, p := coordinatorFixture()
	r, _, _ := pendingGroup(t)
	m.ensure = func(_ context.Context, n string) error {
		require.Equal(t, r.LiveKitRoomName, n)
		return context.DeadlineExceeded
	}
	m.close = func(_ context.Context, n string) error { require.Equal(t, r.LiveKitRoomName, n); return nil }
	_, err := c.Start(context.Background(), r.Intent, nil)
	require.Error(t, err)
	stored, err := j.Load(context.Background(), r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosed, stored.Stage)
	require.Empty(t, stored.Events)
	require.Equal(t, 1, m.closes)
	require.NotContains(t, p.stages, StageActive)
}

func TestUnknownCloseRetainsClosingAndDeniedTokens(t *testing.T) {
	c, j, m, _ := coordinatorFixture()
	r, owner, _ := pendingGroup(t)
	_, err := c.Start(context.Background(), r.Intent, nil)
	require.NoError(t, err)
	m.close = func(context.Context, string) error { return context.DeadlineExceeded }
	_, err = c.Change(context.Background(), r.Intent.RoomID, func(r *Room) error { return r.DecideEnd(owner.ProfileID, "hangup", testNow) })
	require.Error(t, err)
	stored, err := j.Load(context.Background(), r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosing, stored.Stage)
	require.True(t, stored.HasPendingClose())
	require.False(t, stored.CanIssueToken(owner.ProfileID))
}

func TestProjectionFailureNeverCompensatesCommittedStart(t *testing.T) {
	c, j, m, p := coordinatorFixture()
	r, owner, _ := pendingGroup(t)
	p.err = errors.New("redis unavailable")
	_, err := c.Start(context.Background(), r.Intent, nil)
	require.Error(t, err)
	stored, err := j.Load(context.Background(), r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageActive, stored.Stage)
	require.True(t, stored.CanIssueToken(owner.ProfileID))
	require.Equal(t, 0, m.closes)
	require.Len(t, stored.Events, 3)
	p.err = nil
	require.NoError(t, c.Reconcile(context.Background(), r.Intent.RoomID))
	replayed, err := j.Load(context.Background(), r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, stored.Events, replayed.Events)
}

func TestCanceledRequestUsesFreshBoundedCleanupContext(t *testing.T) {
	c, j, m, _ := coordinatorFixture()
	r, _, _ := pendingGroup(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.ensure = func(context.Context, string) error { cancel(); return context.Canceled }
	m.close = func(ctx context.Context, _ string) error {
		require.NoError(t, ctx.Err())
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return nil
	}
	_, err := c.Start(ctx, r.Intent, nil)
	require.Error(t, err)
	stored, err := j.Load(context.Background(), r.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosed, stored.Stage)
}

func TestRestartAbortsPreparationButPreservesCommittedActive(t *testing.T) {
	c, j, m, _ := coordinatorFixture()
	pending, _, _ := pendingGroup(t)
	require.NoError(t, j.Create(context.Background(), pending))
	active, owner := activeGroup(t)
	require.NoError(t, j.Create(context.Background(), active))
	m.ensure = func(context.Context, string) error {
		t.Fatal("restart must never ensure an unfinished preparation")
		return nil
	}
	require.NoError(t, c.Reconcile(context.Background(), pending.Intent.RoomID))
	require.NoError(t, c.Reconcile(context.Background(), active.Intent.RoomID))
	closed, err := j.Load(context.Background(), pending.Intent.RoomID)
	require.NoError(t, err)
	require.Equal(t, StageClosed, closed.Stage)
	require.Empty(t, closed.Events)
	kept, err := j.Load(context.Background(), active.Intent.RoomID)
	require.NoError(t, err)
	require.True(t, kept.CanIssueToken(owner.ProfileID))
	require.Equal(t, active.Events, kept.Events)
	require.Equal(t, 1, m.closes)
}

func TestRetiredBearerReconciliationCannotEjectRejoinedGeneration(t *testing.T) {
	c, j, m, _ := coordinatorFixture()
	r, owner := activeGroup(t)
	other := identity()
	join := uuid.New()
	require.NoError(t, r.BeginJoin(join, other, testNow))
	require.NoError(t, r.CommitAdmission(join, testNow))
	old := r.Members[owner.ProfileID.String()]
	require.NoError(t, r.DecideLeave(owner.ProfileID, testNow))
	require.NoError(t, r.CompleteMemberCleanup(owner.ProfileID, old.Generation, testNow))
	rejoin := uuid.New()
	require.NoError(t, r.BeginJoin(rejoin, owner, testNow))
	require.NoError(t, r.CommitAdmission(rejoin, testNow))
	current := r.Members[owner.ProfileID.String()]
	m.listed = []string{old.MediaIdentity, current.MediaIdentity, r.Members[other.ProfileID.String()].MediaIdentity}
	require.NoError(t, j.Create(context.Background(), r))
	require.NoError(t, c.Reconcile(context.Background(), r.Intent.RoomID))
	require.Equal(t, []string{old.MediaIdentity}, m.removed)
}
