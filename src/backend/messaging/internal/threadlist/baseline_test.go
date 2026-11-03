package threadlist

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/effectivemembership"
)

type baselineSource struct {
	h       effectivemembership.Export
	members []effectivemembership.Member
}

func (s baselineSource) Begin(context.Context, uuid.UUID) (effectivemembership.Export, error) {
	return s.h, nil
}
func (s baselineSource) Page(_ context.Context, h effectivemembership.Export, cursor string, position uint64) (effectivemembership.Page, error) {
	end := min(position+64, uint64(len(s.members)))
	p := effectivemembership.Page{ChatID: h.ChatID, Incarnation: h.Incarnation, SnapshotID: h.SnapshotID, Generation: h.Generation, PinID: h.PinID, Q: h.Q, Cursor: cursor, Position: position, Terminal: end == uint64(len(s.members)), Members: s.members[position:end]}
	if !p.Terminal {
		p.NextCursor = strconv.FormatUint(end, 10)
	}
	return p, nil
}
func (s baselineSource) Validate(_ context.Context, h effectivemembership.Export) (effectivemembership.Export, error) {
	return h, nil
}

func baselineMember(e MembershipRecord) effectivemembership.Member {
	m := effectivemembership.Member{ProfileID: e.ProfileID, Revision: e.Revision, Lineage: e.Lineage, Allowed: e.Kind == MemberJoined}
	for _, source := range e.Sources {
		m.Sources = append(m.Sources, effectivemembership.Source(source))
	}
	return m
}

func baselineFixture(t *testing.T, inbox MembershipInbox, chat, incarnation uuid.UUID, q int64, members ...effectivemembership.Member) (ValidatedMembershipBaseline, baselineSource) {
	t.Helper()
	h := effectivemembership.Export{ProtocolVersion: 1, ChatID: chat, Incarnation: incarnation, SnapshotID: uuid.New(), Generation: uuid.New(), PinID: uuid.New(), Q: q, Count: uint64(len(members)), RetainUntil: time.Now().Add(time.Hour).UTC()}
	seen := map[string]bool{}
	for _, m := range members {
		for _, source := range m.Sources {
			key := source.Owner + source.Scope.String()
			if !seen[key] {
				h.Heads = append(h.Heads, source)
				seen[key] = true
			}
		}
	}
	if len(members) == 0 {
		h.Heads = []effectivemembership.Source{{Owner: "chat", Scope: chat, Incarnation: incarnation, Revision: 1}}
	}
	h.Digest = effectivemembership.ContentDigest(h, members)
	source := baselineSource{h: h, members: members}
	snapshot, err := inbox.ReceiveBaseline(context.Background(), source, chat)
	require.NoError(t, err)
	return snapshot, source
}

func TestPostgresBaselineCoversMissingHistoryAndReplaysBufferedTail(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	_, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 10, baselineMember(e))
	receipt, err := inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	require.True(t, receipt.Current)
	// Origin event was never delivered; the complete baseline covers it.
	require.NoError(t, inbox.Apply(ctx, e))
	pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 10)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, pair.Status)
	e.EventID, e.Offset, e.Revision, e.Kind = uuid.New(), 11, 2, MemberRemoved
	require.NoError(t, inbox.Apply(ctx, e))
	require.NoError(t, inbox.Apply(ctx, e))
	// Exact installation retry cannot undo later local membership application.
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	checkpoint, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.EqualValues(t, 11, checkpoint.AppliedQ)
	require.EqualValues(t, 10, checkpoint.BaselineQ)
	pair, err = inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 11)
	require.NoError(t, err)
	require.Equal(t, MemberRevoked, pair.Status)
	// Q13 arrives while Q12 is still missing; the next sealed baseline fills it.
	tail := membershipFixture()
	tail.ChatID, tail.Incarnation, tail.Offset = e.ChatID, e.Incarnation, 13
	tail.Sources[0] = e.Sources[0]
	require.NoError(t, inbox.Apply(ctx, tail))
	e.EventID, e.Offset, e.Revision, e.Kind, e.Lineage = uuid.New(), 12, 3, MemberJoined, uuid.New()
	next, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 12, baselineMember(e))
	_, err = inbox.InstallBaseline(ctx, next)
	require.NoError(t, err)
	pair, err = inbox.Pair(ctx, e.ChatID, e.Incarnation, tail.ProfileID, 13)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, pair.Status)
	checkpoint, err = inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	require.EqualValues(t, 13, checkpoint.AppliedQ)
	require.EqualValues(t, 12, checkpoint.BaselineQ)
}

func TestPostgresBaselineOmissionCannotResurrectOldLineage(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	require.NoError(t, inbox.Apply(ctx, e))
	_, err := inbox.Checkpoint(ctx, e.ChatID)
	require.NoError(t, err)
	snapshot, _ := baselineFixture(t, inbox, e.ChatID, e.Incarnation, 5)
	_, err = inbox.InstallBaseline(ctx, snapshot)
	require.NoError(t, err)
	pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 5)
	require.NoError(t, err)
	require.Equal(t, MemberRevoked, pair.Status)
	e.EventID, e.Offset, e.Revision = uuid.New(), 6, 2
	require.ErrorIs(t, inbox.Apply(ctx, e), ErrMembershipConflict)
}

func TestPostgresBaselineRejectsZeroCapability(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	_, err := inbox.InstallBaseline(context.Background(), ValidatedMembershipBaseline{})
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}
