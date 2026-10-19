package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	grpcsvc "voice/backend/voice/internal/grpcsvc"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/rosteroutbox"
	"voice/backend/voice/internal/spacegatetest"
	"voice/backend/voice/internal/spacelifecycle"
	"voice/backend/voice/internal/spacemedia"
	"voice/backend/voice/internal/store"
	"voice/backend/voice/internal/voiceevents"
)

// Embedding the unrelated publisher methods keeps this spy restricted to the
// production admission publisher call. Any accidental unrelated call panics.
type compositionPublisher struct {
	voiceevents.Publisher
	mu       sync.Mutex
	subjects []string
}

func (p *compositionPublisher) PublishAdmissionEvent(_ context.Context, _ uuid.UUID, subject string, _ []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.subjects = append(p.subjects, subject)
	return nil
}

func (p *compositionPublisher) count(subject string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := 0
	for _, got := range p.subjects {
		if got == subject {
			n++
		}
	}
	return n
}

type compositionAuthority struct{}

func (compositionAuthority) RequireCurrent(context.Context, string, int64) error { return nil }
func (compositionAuthority) CurrentFloor(context.Context, string) (int64, error) { return 3, nil }
func (compositionAuthority) ResolveVoiceRoomAccess(_ context.Context, space, _, _ string) (grpcsvc.CanonicalVoiceRoomAccess, error) {
	return grpcsvc.CanonicalVoiceRoomAccess{SpaceID: space, Member: true, Active: true, AccessEpoch: 4, CommunicationStateResolved: true, DecisionDeadline: time.Now().Add(time.Minute)}, nil
}
func (compositionAuthority) ResolveVoiceRoomGrants(context.Context, string, string, string) (grpcsvc.CanonicalVoiceRoomGrants, error) {
	return grpcsvc.CanonicalVoiceRoomGrants{PolicyEpoch: 5, CanJoin: true, CanSubscribe: true}, nil
}

type compositionMedia struct {
	mu      sync.Mutex
	removed int
}

func (*compositionMedia) RetirementCapability() string            { return "EJECT_ONLY" }
func (*compositionMedia) CloseRoom(context.Context, string) error { return nil }
func (m *compositionMedia) RemoveParticipant(context.Context, string, string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed++
	return nil
}

func TestProductionSpaceMediaFactoryKeepsNativeRosterDormant(t *testing.T) {
	if os.Getenv("VOICE_COMPOSITION_RESTART_CHILD") == "1" {
		config, err := pgxpool.ParseConfig(os.Getenv("VOICE_COMPOSITION_TEST_DATABASE_URL"))
		require.NoError(t, err)
		config.ConnConfig.Database = os.Getenv("VOICE_COMPOSITION_RESTART_DATABASE")
		pool, err := pgxpool.NewWithConfig(t.Context(), config)
		require.NoError(t, err)
		defer pool.Close()
		var payload []byte
		require.NoError(t, pool.QueryRow(t.Context(), `SELECT payload FROM voice_roster_outbox WHERE event_id=$1 AND lease_token IS NULL AND lease_until IS NULL AND delivered_at IS NULL`, os.Getenv("VOICE_COMPOSITION_RESTART_EVENT_ID")).Scan(&payload))
		require.Equal(t, os.Getenv("VOICE_COMPOSITION_RESTART_PAYLOAD"), fmt.Sprintf("%x", payload))
		return
	}
	if testing.Short() {
		t.Skip("PostgreSQL/NATS composition integration")
	}
	for _, spaceEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("space_enabled_%t", spaceEnabled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			pool := compositionPostgres(t, ctx)
			admissions := spacemedia.NewPostgresAdmissionStore(pool)
			// Seed a completed native mutation before startup. Recovery racing a
			// live PREPARED/FENCED writer is tracked separately as
			// VOICE-SPACE-RECOVERY-LIVE-ADMISSION-RACE; this test does not hide or
			// claim to resolve that arbitration gap.
			a := compositionAdmission(t, ctx, admissions)
			redisClient := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
			defer redisClient.Close()
			calls := store.NewRedisCallStore(redisClient, uuid.NewString()+":")
			publisher := &compositionPublisher{}
			media := &compositionMedia{}
			lifecycle := &spacelifecycle.Service{Store: spacelifecycle.NewPostgresStore(pool, spacemedia.BeginPartyRoomClose), Effects: spacelifecycle.NewEffects(pool, media)}
			voice := &grpcsvc.VoiceGRPC{Calls: calls, SpaceMediaAdmissions: admissions, Events: publisher,
				SpaceLifecycle: lifecycle, VoiceRoomAccessResolver: compositionAuthority{}, SpaceVoiceRoomGrants: compositionAuthority{}}
			natsURL := ""
			if spaceEnabled {
				natsURL = compositionNATS(t, ctx)
			}
			closeRuntime := startSpaceMediaRuntime(ctx, voice, pool, compositionAuthority{}, true, natsURL, media, slog.New(slog.NewTextHandler(io.Discard, nil)))
			defer func() { cancel(); require.NoError(t, closeRuntime()) }()
			require.NotNil(t, lifecycle.Effects.NativeDrain, "P3 native drain must be wired even without Space/NATS runtime")
			require.Same(t, calls, lifecycle.Effects.ProjectionFence)
			if spaceEnabled {
				require.NotNil(t, voice.SpaceMediaRevoker)
				require.Same(t, voice.SpaceMediaRevoker, lifecycle.Effects.NativeDrain)
				require.True(t, voice.SpaceMediaServing.Load(), "existing initial reconciliation still runs")
			} else {
				require.Nil(t, voice.SpaceMediaRevoker)
			}

			// No factory test hook claims the new A3 delivery queue. The same
			// fully committed native fact must survive either startup configuration.
			var eventID string
			var payload []byte
			require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text,payload FROM voice_roster_outbox WHERE room_id=$1`, a.RoomID).Scan(&eventID, &payload))
			if spaceEnabled {
				require.Eventually(t, func() bool {
					var delivered int
					if err := pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_outbox WHERE operation_id=$1 AND delivered_at IS NOT NULL`, a.OperationID).Scan(&delivered); err != nil {
						return false
					}
					return delivered == 2
				}, 4*time.Second, 20*time.Millisecond,
					"existing compatibility admission relay must still deliver")
				require.Equal(t, 1, publisher.count("voice.member_joined"))
				var delivered int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_space_media_admission_outbox WHERE operation_id=$1 AND delivered_at IS NOT NULL`, a.OperationID).Scan(&delivered))
				require.Equal(t, 2, delivered)
			}
			require.Zero(t, publisher.count(rosteroutbox.Subject), "production factory must never publish unqualified A3 facts")
			assertDormant := func() {
				t.Helper()
				var untouched int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE event_id=$1 AND payload=$2 AND lease_token IS NULL AND lease_until IS NULL AND delivered_at IS NULL`, eventID, payload).Scan(&untouched))
				require.Equal(t, 1, untouched, "native fact must retain exact bytes and never be leased/delivered")
				var touched int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM voice_roster_outbox WHERE lease_token IS NOT NULL OR lease_until IS NOT NULL OR delivered_at IS NOT NULL`).Scan(&touched))
				require.Zero(t, touched, "all native facts, including P3 LEFT, must remain pending")
			}
			assertDormant()
			// A fresh connection and then an actual child process read the same
			// saved bytes and pending state. No fake store carries them forward.
			reopened, err := pgxpool.NewWithConfig(ctx, pool.Config())
			require.NoError(t, err)
			defer reopened.Close()
			var persisted []byte
			require.NoError(t, reopened.QueryRow(ctx, `SELECT payload FROM voice_roster_outbox WHERE event_id=$1`, eventID).Scan(&persisted))
			require.Equal(t, payload, persisted)
			if os.Getenv("VOICE_COMPOSITION_TEST_DATABASE_URL") != "" {
				restart := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProductionSpaceMediaFactoryKeepsNativeRosterDormant$")
				restart.Env = append(os.Environ(), "VOICE_COMPOSITION_RESTART_CHILD=1", "VOICE_COMPOSITION_RESTART_DATABASE="+pool.Config().ConnConfig.Database,
					"VOICE_COMPOSITION_RESTART_EVENT_ID="+eventID, "VOICE_COMPOSITION_RESTART_PAYLOAD="+fmt.Sprintf("%x", payload))
				output, err := restart.CombinedOutput()
				require.NoError(t, err, "%s", output)
			}
			if spaceEnabled {
				fence := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: uuid.NewString(), Generation: 1,
					DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32)}}
				receipt, err := lifecycle.ApplySpaceLifecycleFence(ctx, fence)
				require.ErrorIs(t, err, spacelifecycle.ErrUnavailable)
				require.Nil(t, receipt, "EJECT_ONLY can never claim full FROZEN retirement")
				media.mu.Lock()
				removed := media.removed
				media.mu.Unlock()
				require.Positive(t, removed, "actual factory-bound P3 native drain must eject the saved target")
				require.Equal(t, "EJECT_ONLY", lifecycle.RetirementCapability())
				fence.Generation, fence.DesiredState = 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
				receipt, err = lifecycle.ApplySpaceLifecycleFence(ctx, fence)
				require.Error(t, err, "pending EJECT_ONLY freeze must not permit LIVE")
				require.Nil(t, receipt)
				purged, err := lifecycle.PurgeSpace(ctx, &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: a.SpaceID.String(), DeletionOperationId: fence.DeletionOperationId,
					Generation: 1, PurgeDecidedAt: timestamppb.Now(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_VOICE, Manifest: fence.Manifest})
				require.Error(t, err, "pending EJECT_ONLY freeze must not permit PURGED")
				require.Nil(t, purged)
				unknown := &spacelifecycle.Service{Store: lifecycle.Store, Effects: spacelifecycle.NewEffects(pool, livekit.NewSDKRoomLifecycle("", "", ""))}
				require.Equal(t, "UNKNOWN", unknown.RetirementCapability(), "missing provider configuration never upgrades retirement capability")
				assertDormant()
			}
		})
	}
}

func compositionAdmission(t *testing.T, ctx context.Context, admissions *spacemedia.PostgresAdmissionStore) store.SpaceMediaAdmission {
	t.Helper()
	a := store.SpaceMediaAdmission{OperationID: uuid.New(), Generation: uuid.NewString(), AccountID: uuid.New(), ProfileID: uuid.New(),
		SpaceID: uuid.New(), RoomID: uuid.NewString(), VoiceRoomID: uuid.NewString(), CreatedRoom: true,
		CallStartedAt: time.Now().UTC(), MaxParticipants: store.MaxVoiceRoomParticipants, SessionEpoch: 3, AccessEpoch: 4, PolicyEpoch: 5, CanJoin: true, CanSubscribe: true,
		Events: []store.SpaceMediaAdmissionEvent{{ID: uuid.New(), Subject: "voice.call_started", Payload: []byte("fixed call event")}, {ID: uuid.New(), Subject: "voice.member_joined", Payload: []byte("fixed member event")}}}
	a.Identity, _ = livekit.SpaceParticipantIdentity(a.ProfileID.String(), a.Generation)
	head, err := admissions.ClaimRoomHead(ctx, a.VoiceRoomID, a.SpaceID.String(), a.RoomID, a.OperationID, true)
	require.NoError(t, err)
	a.RoomGeneration, a.LifecycleGeneration = head.RoomGeneration, head.LifecycleGeneration
	require.NoError(t, admissions.Prepare(ctx, a))
	require.NoError(t, admissions.Fence(ctx, a))
	require.NoError(t, admissions.MarkCommitted(ctx, a.OperationID, a.Generation))
	require.NoError(t, admissions.MarkProjectionApplied(ctx, a.OperationID, a.Generation))
	return a
}

func compositionPostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := spacegatetest.StartPostgres(t, ctx, "voice_composition", "")
	for _, migration := range []string{"000001_room_lifecycle", "000002_redis_divergence", "000003_matchmaking_membership", "000004_game_session_rooms", "000005_game_session_close", "000006_game_session_roster_lease", "000007_t17_sdk_conversion_fence", "000008_account_voice_fence", "000009_space_lifecycle", "000010_space_media_admission", "000013_a3_roster_outbox", "000014_space_media_p3"} {
		body, err := os.ReadFile(filepath.Join("..", "migrations", "voice_db", migration+".up.sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
	return pool
}

func compositionNATS(t *testing.T, ctx context.Context) string {
	t.Helper()
	url := os.Getenv("VOICE_COMPOSITION_TEST_NATS_URL")
	if url == "" {
		c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "nats:2.12-alpine", ExposedPorts: []string{"4222/tcp"}, Cmd: []string{"-js"}, WaitingFor: wait.ForListeningPort("4222/tcp")}, Started: true})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, c.Terminate(context.Background())) })
		host, err := c.Host(ctx)
		require.NoError(t, err)
		port, err := c.MappedPort(ctx, "4222/tcp")
		require.NoError(t, err)
		url = "nats://" + host + ":" + port.Port()
	}
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	// These are the existing v5 compatibility consumers. This fixture grants no
	// installed actor rights and deliberately creates no A3 subject or consumer.
	for _, contract := range []struct{ stream, durable, subject, target string }{
		{"chat_events", "voice_space_media_chat", "space.voice_room_access_invalidated", "_INBOX.voice.voice.space_media_chat"},
		{"role_events", "voice_space_media_role", "role.voice_policy_invalidated", "_INBOX.voice.voice.space_media_role"},
	} {
		_, err = js.AddStream(&nats.StreamConfig{Name: contract.stream, Subjects: []string{contract.subject}})
		require.NoError(t, err)
		_, err = js.AddConsumer(contract.stream, &nats.ConsumerConfig{Durable: contract.durable, FilterSubject: contract.subject, DeliverSubject: contract.target, AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy})
		require.NoError(t, err)
	}
	return url
}
