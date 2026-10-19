//go:build realtimep3integration

package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"sync"
	"testing"
	"time"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	realtimev1 "voice.app/voice/realtime/v1"
	"voice/backend/realtime/internal/lifecycle"
)

func fanoutPostgres(t *testing.T) (*lifecycle.Store, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("REALTIME_P3_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("isolated Linux PostgreSQL runner is required")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "hub_" + uuid.New().String()[:8]
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	sql, err := os.ReadFile("../migrations/realtime_db/000001_admission.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	return &lifecycle.Store{Pool: pool}, pool
}

type fixedNativeLifecycle struct{ space uuid.UUID }

func (f fixedNativeLifecycle) Resolve(_ context.Context, chat uuid.UUID) (*chatv1.ResolveChatLifecycleResponse, error) {
	return &chatv1.ResolveChatLifecycleResponse{ProtocolVersion: 1, SchemaVersion: 1, ChatId: chat.String(), SpaceId: f.space.String(), State: chatv1.ChatLifecycleState_CHAT_LIFECYCLE_STATE_LIVE}, nil
}

func hubManifest(space, operation, chat uuid.UUID) *realtimev1.ImportSpacePurgeManifestPageRequest {
	h := sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifest\x00"))
	h.Write(space[:])
	h.Write(operation[:])
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], 1)
	h.Write(n[:])
	h.Write(n[:])
	h.Write(chat[:])
	root := h.Sum(nil)
	h = sha256.New()
	h.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	h.Write(root)
	h.Write(make([]byte, 8))
	h.Write(n[:])
	h.Write(chat[:])
	return &realtimev1.ImportSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), ScheduleGeneration: 1, SealsManifest: true, Page: &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: root, ItemCount: 1}, ItemIds: []string{chat.String()}, PageSha256: h.Sum(nil)}}
}

// This composes the REAL hub/channel sink with the REAL PostgreSQL admission
// and receipt store. Channels form deterministic barriers; no sleep proves a race.
func TestActualHubFanoutFreezeWaitsForEveryRecipientAndRejectsCapturedLateScope(t *testing.T) {
	store, _ := fanoutPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	space, operation, chat, inc, profile := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.NewString()
	if err := store.Open(ctx, inc, "real-hub/process/start"); err != nil {
		t.Fatal(err)
	}
	hub := newWSHub()
	hub.lifecycle = &deliveryLifecycle{store: store, executor: lifecycle.NewExecutor(store, inc), resolver: fixedNativeLifecycle{space: space}}
	a := hub.attachConn("i", "a", profile, 8)
	b := hub.attachConn("i", "b", profile, 8)
	hub.addChat(a, chat.String())
	hub.addChat(b, chat.String())
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	guard := func() bool { once.Do(func() { close(entered) }); <-release; return true }
	a.setWriteGuard(guard)
	b.setWriteGuard(guard)
	payload := json.RawMessage(`{"chat_id":"` + chat.String() + `"}`)
	go func() {
		defer close(done)
		hub.broadcastToChat(chat.String(), fanoutEnvelope{Op: "message_create", D: payload}, nil, "")
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	imported := hubManifest(space, operation, chat)
	if _, err := store.Import(ctx, imported); err != nil {
		t.Fatal(err)
	}
	fence := &realtimev1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: imported.Page.Manifest}}
	if response, err := store.Apply(ctx, fence); !errors.Is(err, lifecycle.ErrPending) || response != nil {
		t.Fatalf("receipt completed ahead of actual sinks: %v %v", response, err)
	}
	close(release)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if len(a.fanout) != 1 || len(b.fanout) != 1 {
		t.Fatal("finite admitted fanout did not finish both recipients")
	}
	if _, err := store.Apply(ctx, fence); err != nil {
		t.Fatal(err)
	}
	frame := <-a.fanout
	if got := a.enqueue(frame, false); got != fanoutDropped {
		t.Fatal("captured settled admission allowed a later enqueue")
	}
	hub.broadcastToChat(chat.String(), fanoutEnvelope{Op: "message_create", D: payload}, nil, "")
	hub.broadcastToProfile(profile, fanoutEnvelope{Op: "archive_activity", D: payload}, nil, "")
	hub.broadcastMarkReadSameProfileExcept(profile, "", "", payload)
	if len(a.fanout) != 0 || len(b.fanout) != 1 {
		t.Fatal("stale LIVE hydration bypassed imported fence in a real sink")
	}
}
