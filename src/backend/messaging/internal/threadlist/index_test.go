package threadlist

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type memoryNodes struct {
	nodes map[NodeID]Node
	reads int
	puts  int
}

func (s *memoryNodes) Get(_ context.Context, id NodeID) (Node, error) {
	s.reads++
	n, ok := s.nodes[id]
	if !ok {
		return Node{}, fmt.Errorf("missing node %x", id)
	}
	return n, nil
}

func (s *memoryNodes) Put(_ context.Context, n Node) (NodeID, error) {
	s.puts++
	id := n.ID()
	s.nodes[id] = n
	return id, nil
}

func memoryIndex() (*memoryNodes, Index) {
	s := &memoryNodes{nodes: make(map[NodeID]Node)}
	return s, Index{Store: s}
}

func testEntry(i int, at int64) Entry {
	id := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-%012d", i+1))
	return Entry{ParentID: id, LastReplyID: id, LastReplyAt: time.Unix(at, 0).UTC(), ReplyCount: 1, Preview: fmt.Sprint(i)}
}

func TestIndexPathCopyUpdateDeleteAndOriginalRoot(t *testing.T) {
	ctx := context.Background()
	_, idx := memoryIndex()
	a, b := testEntry(1, 100), testEntry(2, 101)
	first, err := idx.Upsert(ctx, a)
	require.NoError(t, err)
	second, err := first.Upsert(ctx, b)
	require.NoError(t, err)
	a.LastReplyAt = time.Unix(102, 0).UTC()
	a.ReplyCount = 100000
	third, err := second.Upsert(ctx, a)
	require.NoError(t, err)
	page, err := first.Page(ctx, 10)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, uint64(1), page[0].ReplyCount)
	page, err = third.Page(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []Entry{a, b}, page)
	fourth, err := third.Delete(ctx, a.ParentID)
	require.NoError(t, err)
	page, err = fourth.Page(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []Entry{b}, page)
	page, err = third.Page(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, []Entry{a, b}, page)
	// Identical retries produce identical root identities and do not accumulate duplicates.
	retry, err := third.Upsert(ctx, a)
	require.NoError(t, err)
	require.Equal(t, third.Roots, retry.Roots)
}

func TestIndexRandomMutationsMatchSortedOracleAndRemainBalanced(t *testing.T) {
	ctx := context.Background()
	s, idx := memoryIndex()
	rng := rand.New(rand.NewSource(23))
	values := map[uuid.UUID]Entry{}
	for step := 0; step < 3000; step++ {
		e := testEntry(rng.Intn(350), int64(rng.Intn(10)))
		var err error
		if step%4 == 0 {
			idx, err = idx.Delete(ctx, e.ParentID)
			delete(values, e.ParentID)
		} else {
			idx, err = idx.Upsert(ctx, e)
			values[e.ParentID] = e
		}
		require.NoError(t, err)
		if step%31 != 0 {
			continue
		}
		want := make([]Entry, 0, len(values))
		for _, v := range values {
			want = append(want, v)
		}
		sort.Slice(want, func(i, j int) bool {
			if !want[i].LastReplyAt.Equal(want[j].LastReplyAt) {
				return want[i].LastReplyAt.After(want[j].LastReplyAt)
			}
			return want[i].ParentID.String() > want[j].ParentID.String()
		})
		got, err := idx.Page(ctx, 1000)
		require.NoError(t, err)
		require.Equal(t, want, got)
		for _, root := range []NodeID{idx.Roots.Activity, idx.Roots.Parent} {
			assertAVL(t, s, root)
		}
	}
}

func assertAVL(t *testing.T, s *memoryNodes, id NodeID) (int, int) {
	if id == (NodeID{}) {
		return 0, 0
	}
	n := s.nodes[id]
	lh, ls := assertAVL(t, s, n.Left)
	rh, rs := assertAVL(t, s, n.Right)
	require.LessOrEqual(t, math.Abs(float64(lh-rh)), float64(1))
	require.Equal(t, 1+max(lh, rh), n.Height)
	require.Equal(t, 1+ls+rs, n.Size)
	return n.Height, n.Size
}

func TestIndexAdversarialInsertionAndPageBound(t *testing.T) {
	ctx := context.Background()
	s, idx := memoryIndex()
	for i := 0; i < 8192; i++ {
		var err error
		idx, err = idx.Upsert(ctx, testEntry(i, int64(i)))
		require.NoError(t, err)
	}
	height, _ := assertAVL(t, s, idx.Roots.Activity)
	require.LessOrEqual(t, height, int(1.45*math.Log2(8194)))
	for _, limit := range []int{1, 50, 100} {
		s.reads = 0
		page, err := idx.Page(ctx, limit)
		require.NoError(t, err)
		require.Len(t, page, limit)
		require.LessOrEqual(t, s.reads, height+limit, "page reads only the search path and returned nodes")
	}
}

func TestIndexRejectsInvalidEntryAndMissingNodes(t *testing.T) {
	ctx := context.Background()
	_, idx := memoryIndex()
	_, err := idx.Upsert(ctx, Entry{})
	require.ErrorIs(t, err, ErrInvalidEntry)
	idx.Roots.Activity[0] = 1
	_, err = idx.Page(ctx, 1)
	require.Error(t, err)
	_, err = idx.Page(ctx, 0)
	require.ErrorIs(t, err, ErrInvalidPageSize)
}

func TestIndexRejectsLossyContentEncoding(t *testing.T) {
	ctx := context.Background()
	_, idx := memoryIndex()
	for _, e := range []Entry{
		{ParentID: uuid.New(), LastReplyID: uuid.New(), LastReplyAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), ReplyCount: 1},
		{ParentID: uuid.New(), LastReplyID: uuid.New(), LastReplyAt: time.Now(), ReplyCount: 1, Preview: string([]byte{0xff})},
	} {
		_, err := idx.Upsert(ctx, e)
		require.ErrorIs(t, err, ErrInvalidEntry, "a summary must have a lossless canonical identity")
	}
}

func TestIndexRemainingSetHasNoDuplicateParent(t *testing.T) {
	ctx := context.Background()
	_, idx := memoryIndex()
	for i := 0; i < 121; i++ {
		var err error
		idx, err = idx.Upsert(ctx, testEntry(i, 100))
		require.NoError(t, err)
	}
	original := idx
	seen := map[uuid.UUID]bool{}
	for {
		page, err := idx.Page(ctx, 7)
		require.NoError(t, err)
		if len(page) == 0 {
			break
		}
		for _, entry := range page {
			require.False(t, seen[entry.ParentID])
			seen[entry.ParentID] = true
			idx, err = idx.Delete(ctx, entry.ParentID)
			require.NoError(t, err)
		}
	}
	require.Len(t, seen, 121)
	page, err := original.Page(ctx, 1000)
	require.NoError(t, err)
	require.Len(t, page, 121)
	require.Equal(t, testEntry(120, 100).ParentID, page[0].ParentID, "ties use parent ID descending")
	require.Equal(t, testEntry(0, 100).ParentID, page[120].ParentID)
}

func TestIndexCorruptionAndLateReadErrorReturnNoPartialPage(t *testing.T) {
	ctx := context.Background()
	s, idx := memoryIndex()
	for i := 0; i < 3; i++ {
		var err error
		idx, err = idx.Upsert(ctx, testEntry(i, int64(i)))
		require.NoError(t, err)
	}
	root := s.nodes[idx.Roots.Activity]
	require.NotEqual(t, NodeID{}, root.Right)
	saved := s.nodes[root.Right]
	delete(s.nodes, root.Right)
	page, err := idx.Page(ctx, 10)
	require.Error(t, err)
	require.Nil(t, page, "do not expose a partial staged result")
	s.nodes[root.Right] = saved
	root.Entry.Preview = "corrupted payload"
	s.nodes[idx.Roots.Activity] = root
	page, err = idx.Page(ctx, 10)
	require.ErrorIs(t, err, ErrInvalidNode)
	require.Nil(t, page)
}
