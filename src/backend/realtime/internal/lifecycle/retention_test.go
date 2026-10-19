//go:build realtimep3integration

package lifecycle

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	realtimev1 "voice.app/voice/realtime/v1"
)

func TestReceiptCompactionNeverRefreshesReplayOrRemovesPendingAndDenies(t *testing.T) {
	s, pool := testStore(t)
	ctx := context.Background()
	space, operation, chat, other, inc := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	if err := s.Open(ctx, inc, "retention/process/start"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hydrate(ctx, other, uuid.Nil, 0, "LIVE"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Begin(ctx, inc, other); err != nil {
		t.Fatal(err)
	}
	imported := manifestFixture(space, operation, []uuid.UUID{chat})
	if _, err := s.Import(ctx, imported); err != nil {
		t.Fatal(err)
	}
	fence := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: imported.Page.Manifest}}
	first, err := s.Apply(ctx, fence)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := s.CompactReceipts(ctx); err != nil || n != 0 {
		t.Fatalf("early compact: %d %v", n, err)
	}
	again, err := s.Apply(ctx, fence)
	if err != nil || !proto.Equal(first, again) {
		t.Fatalf("live replay changed: %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE realtime_lifecycle_operations SET completed_at=clock_timestamp()-interval '31 days'`); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CompactReceipts(ctx); err != nil || n != 2 {
		t.Fatalf("expired compact: %d %v", n, err)
	}
	if _, err = s.Apply(ctx, fence); !errors.Is(err, ErrConflict) {
		t.Fatalf("expired replay renewed: %v", err)
	}
	changed := proto.Clone(fence).(*realtimev1.ApplySpaceLifecycleFenceRequest)
	changed.Fence.Manifest.ItemCount++
	if _, err = s.Apply(ctx, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("compact binding lost: %v", err)
	}
	var pending, compact, denied int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM realtime_admissions WHERE state='PENDING'`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM realtime_lifecycle_operations WHERE request_bytes IS NULL AND response_bytes IS NULL AND octet_length(request_sha256)=32 AND completed_at IS NOT NULL`).Scan(&compact); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM realtime_chat_gates WHERE chat_id=$1 AND state='FROZEN'`, chat).Scan(&denied); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || compact != 2 || denied != 1 {
		t.Fatalf("retention changed authority: pending=%d compact=%d deny=%d", pending, compact, denied)
	}
}
