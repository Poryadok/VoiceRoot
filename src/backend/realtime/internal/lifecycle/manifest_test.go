//go:build realtimep3integration

package lifecycle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sort"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	realtimev1 "voice.app/voice/realtime/v1"
)

func TestMultipageManifestCannotFreezeBeforeExactSealOrAcceptChangedReplay(t *testing.T) {
	s, _ := testStore(t)
	ctx := context.Background()
	ids := make([]uuid.UUID, 1001)
	for i := range ids {
		ids[i] = uuid.New()
	}
	full := manifestFixture(uuid.New(), uuid.New(), ids)
	makePage := func(index uint64, items []string, sealed bool) *realtimev1.ImportSpacePurgeManifestPageRequest {
		req := proto.Clone(full).(*realtimev1.ImportSpacePurgeManifestPageRequest)
		req.SealsManifest = sealed
		req.Page.PageIndex = index
		req.Page.ItemIds = items
		h := sha256.New()
		h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
		h.Write(req.Page.Manifest.ManifestSha256)
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], index)
		h.Write(n[:])
		binary.BigEndian.PutUint64(n[:], uint64(len(items)))
		h.Write(n[:])
		for _, raw := range items {
			id := uuid.MustParse(raw)
			h.Write(id[:])
		}
		req.Page.PageSha256 = h.Sum(nil)
		if !sealed {
			token := make([]byte, 40)
			binary.BigEndian.PutUint64(token[:8], index+1)
			copy(token[8:], req.Page.Manifest.ManifestSha256)
			req.Page.NextPageToken = base64.RawURLEncoding.EncodeToString(token)
		}
		return req
	}
	first := makePage(0, full.Page.ItemIds[:1000], false)
	last := makePage(1, full.Page.ItemIds[1000:], true)
	if _, err := s.Import(ctx, last); err == nil {
		t.Fatal("out-of-order page accepted")
	}
	if _, err := s.Import(ctx, first); err != nil {
		t.Fatal(err)
	}
	fence := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: full.SpaceId, DeletionOperationId: full.DeletionOperationId, Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: full.Page.Manifest}}
	if response, err := s.Apply(ctx, fence); err == nil || response != nil {
		t.Fatal("unsealed coverage produced strong receipt")
	}
	changed := proto.Clone(first).(*realtimev1.ImportSpacePurgeManifestPageRequest)
	changed.Page.NextPageToken = "changed"
	if _, err := s.Import(ctx, changed); err == nil {
		t.Fatal("changed import replay accepted")
	}
	if _, err := s.Import(ctx, first); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(ctx, last); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Apply(ctx, fence); err != nil {
		t.Fatal(err)
	}
}

func manifestFixture(space, operation uuid.UUID, ids []uuid.UUID) *realtimev1.ImportSpacePurgeManifestPageRequest {
	ids = append([]uuid.UUID(nil), ids...)
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	hash := sha256.New()
	hash.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	hash.Write(space[:])
	hash.Write(operation[:])
	var number [8]byte
	binary.BigEndian.PutUint64(number[:], 1)
	hash.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(len(ids)))
	hash.Write(number[:])
	for _, id := range ids {
		hash.Write(id[:])
	}
	root := hash.Sum(nil)
	pageHash := sha256.New()
	pageHash.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	pageHash.Write(root)
	binary.BigEndian.PutUint64(number[:], 0)
	pageHash.Write(number[:])
	binary.BigEndian.PutUint64(number[:], uint64(len(ids)))
	pageHash.Write(number[:])
	for _, id := range ids {
		pageHash.Write(id[:])
	}
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: root, ItemCount: uint64(len(ids))}, PageSha256: pageHash.Sum(nil)}
	for _, id := range ids {
		page.ItemIds = append(page.ItemIds, id.String())
	}
	return &realtimev1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), ScheduleGeneration: 1, Page: page, SealsManifest: true}
}

func TestFreezeReceiptRequiresDrainAndExactCompletedReplay(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	space, operation, chat, inc := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := store.Open(ctx, inc, "manifest/process/start"); err != nil {
		t.Fatal(err)
	}
	if err := store.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	ticket, err := store.Begin(ctx, inc, chat)
	if err != nil {
		t.Fatal(err)
	}
	imported := manifestFixture(space, operation, []uuid.UUID{chat})
	if _, err = store.Import(ctx, imported); err != nil {
		t.Fatal(err)
	}
	req := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: imported.Page.Manifest}}
	if response, err := store.Apply(ctx, req); !errors.Is(err, ErrPending) || response != nil {
		t.Fatalf("early receipt: %v %v", response, err)
	}
	if err = store.Settle(ctx, inc, chat, ticket); err != nil {
		t.Fatal(err)
	}
	first, err := store.Apply(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.GetReceipt().GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_REALTIME {
		t.Fatal("wrong participant")
	}
	var completed time.Time
	if err = pool.QueryRow(ctx, `SELECT completed_at FROM realtime_lifecycle_operations WHERE space_id=$1 AND kind='FENCE'`, space).Scan(&completed); err != nil {
		t.Fatal(err)
	}
	if !first.Receipt.AppliedAt.AsTime().Equal(completed) {
		t.Fatal("public receipt time and original database replay horizon differ")
	}
	again, err := store.Apply(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := proto.MarshalOptions{Deterministic: true}.Marshal(first)
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("completed replay changed receipt bytes")
	}
	changed := proto.Clone(req).(*realtimev1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ItemCount++
	if _, err = store.Apply(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed body did not conflict: %v", err)
	}
}

func TestExactManifestPurgePersistsAcrossRestartAndEmptyRootCompletes(t *testing.T) {
	store, pool := testStore(t)
	ctx := context.Background()
	space, operation, chat, other, inc := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := store.Open(ctx, inc, "manifest/process/start"); err != nil {
		t.Fatal(err)
	}
	if err := store.Hydrate(ctx, other, uuid.Nil, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	req := manifestFixture(space, operation, []uuid.UUID{chat})
	broken := proto.Clone(req).(*realtimev1.ImportSpacePurgeManifestPageRequest)
	broken.Page.PageSha256[0] ^= 1
	if _, err := store.Import(ctx, broken); err == nil {
		t.Fatal("bad page hash accepted")
	}
	if _, err := store.Import(ctx, req); err != nil {
		t.Fatal(err)
	}
	fence := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: req.Page.Manifest}}
	if _, err := store.Apply(ctx, fence); err != nil {
		t.Fatal(err)
	}
	fence.Fence.Generation = 2
	fence.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	if _, err := store.Apply(ctx, fence); err != nil {
		t.Fatal(err)
	}
	purge := &realtimev1.PurgeSpaceRequest{Purge: &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_REALTIME, Manifest: req.Page.Manifest, PurgeDecidedAt: timestamppb.New(time.Now().UTC())}}
	if _, err := store.Purge(ctx, purge); err != nil {
		t.Fatal(err)
	}
	restarted := &Store{Pool: pool}
	if err := restarted.Hydrate(ctx, chat, space, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.Begin(ctx, inc, chat); !errors.Is(err, ErrDenied) {
		t.Fatalf("terminal chat reopened: %v", err)
	}
	ticket, err := restarted.Begin(ctx, inc, other)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Settle(ctx, inc, other, ticket); err != nil {
		t.Fatal(err)
	}
	empty := manifestFixture(uuid.New(), uuid.New(), nil)
	if _, err = store.Import(ctx, empty); err != nil {
		t.Fatal(err)
	}
	fence.Fence.SpaceId = empty.SpaceId
	fence.Fence.DeletionOperationId = empty.DeletionOperationId
	fence.Fence.Manifest = empty.Page.Manifest
	fence.Fence.Generation = 1
	fence.Fence.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN
	if _, err = store.Apply(ctx, fence); err != nil {
		t.Fatal(err)
	}
}
