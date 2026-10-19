package partysearch

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/pkg/principal"
)

// Synthetic local registration oracle only; this is not root/controller authority.
type forwardProbe struct {
	mu                 sync.Mutex
	ids                []uuid.UUID
	hashes             [][32]byte
	active, released   int
	closed, nilRelease bool
	fail               error
	drained            chan struct{}
	observeRelease     func()
}

func (p *forwardProbe) EnterForward(_ context.Context, id uuid.UUID, hash [32]byte) (func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids = append(p.ids, id)
	p.hashes = append(p.hashes, hash)
	if p.closed {
		return nil, errors.New("synthetic closed")
	}
	if p.fail != nil {
		return nil, p.fail
	}
	if p.nilRelease {
		return nil, nil
	}
	p.active++
	var once sync.Once
	return func() {
		once.Do(func() {
			if p.observeRelease != nil {
				p.observeRelease()
			}
			p.mu.Lock()
			defer p.mu.Unlock()
			p.active--
			p.released++
			if p.closed && p.active == 0 {
				close(p.drained)
			}
		})
	}, nil
}
func (p *forwardProbe) closeFresh() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.drained = make(chan struct{})
	if p.active == 0 {
		close(p.drained)
	}
	return p.drained
}
func (p *forwardProbe) counts() (int, int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ids), p.active, p.released
}

func lifetimeWorker(t *testing.T, st *store.MatchStore, p store.CreateSourcePartyParams, lifetime squad.ForwardLifetime) (*squad.MatchSquadProviderWorker, *gateChatClient, *gateVoiceClient) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "matchmaking", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	chat := &gateChatClient{create: func(r *chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		if err != nil {
			return nil, err
		}
		return &chatv1.CreateMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, MatchId: r.MatchId, ChatId: uuid.NewString(), ParticipantManifestSha256: r.ParticipantManifestSha256, RequestSha256: dispatchDigest(raw), CreatedAt: timestamppb.Now()}}, nil
	}}
	voice := &gateVoiceClient{create: func(r *callsv1.CreateMatchSquadRoomRequest) (*callsv1.CreateMatchSquadRoomResponse, error) {
		raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		if err != nil {
			return nil, err
		}
		cb, err := proto.MarshalOptions{Deterministic: true}.Marshal(r.ChatCreationReceipt)
		if err != nil {
			return nil, err
		}
		return &callsv1.CreateMatchSquadRoomResponse{Receipt: &callsv1.MatchSquadRoomReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: r.OperationId, MatchId: r.MatchId, RoomId: uuid.NewString(), ChatId: r.ChatCreationReceipt.ChatId, ChatCreationReceiptId: r.ChatCreationReceipt.ReceiptId, ChatCreationReceiptSha256: dispatchDigest(cb), ParticipantManifestSha256: r.ParticipantManifestSha256, RequestSha256: dispatchDigest(raw), CreatedAt: timestamppb.Now()}}, nil
	}}
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	return &squad.MatchSquadProviderWorker{Store: st, Issuer: issuer, Chat: chat, Voice: voice, PartyValidator: validator, ForwardLifetime: lifetime}, chat, voice
}

func TestForwardLifetimeCoversRPCAndDurableOutcome(t *testing.T) {
	for _, retry := range []bool{false, true} {
		name := "activation"
		if retry {
			name = "retry classification"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			pool, st, p, intent := admittedProvisionFixture(t)
			candidate, _ := dispatchProof(t, st, intent.MatchID, p)
			probe := &forwardProbe{}
			releaseObserved := make(chan string, 1)
			probe.observeRelease = func() {
				var value string
				query := `SELECT state FROM matchmaking_match_squad_operations WHERE match_id=$1`
				if retry {
					query = `SELECT COALESCE(provision_last_error_class,'missing') FROM matchmaking_match_squad_operations WHERE match_id=$1`
				}
				if err := pool.QueryRow(ctx, query, intent.MatchID).Scan(&value); err != nil {
					value = err.Error()
				}
				releaseObserved <- value
			}
			worker, chat, _ := lifetimeWorker(t, st, p, probe)
			rpcEntered := make(chan struct{})
			resumeRPC := make(chan struct{})
			var resume sync.Once
			defer resume.Do(func() { close(resumeRPC) })
			validChat := chat.create
			chat.create = func(r *chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
				close(rpcEntered)
				<-resumeRPC
				if retry {
					return nil, status.Error(codes.Unavailable, "lost response")
				}
				return validChat(r)
			}
			result := make(chan error, 1)
			go func() {
				bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
				defer cancel()
				_, _, err := worker.Provision(bounded, intent.MatchID, p.MemberProfileIDs)
				result <- err
			}()
			select {
			case <-rpcEntered:
			case <-time.After(5 * time.Second):
				t.Fatal("provider RPC was not reached")
			}
			entered, active, released := probe.counts()
			require.Equal(t, 1, entered, "LIVE attempt must register before first authorization/RPC")
			require.Equal(t, 1, active)
			require.Zero(t, released)
			require.Equal(t, candidate.Hash(), probe.hashes[0], "entry binds actual fresh pre-intent candidate")
			require.NotEqual(t, uuid.Nil, probe.ids[0])
			require.NotEqual(t, intent.OperationID, probe.ids[0])
			lock, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer lock.Rollback(ctx)
			_, err = lock.Exec(ctx, `SELECT match_id FROM matchmaking_match_squad_operations WHERE match_id=$1 FOR UPDATE`, intent.MatchID)
			require.NoError(t, err)
			resume.Do(func() { close(resumeRPC) })
			require.Eventually(t, func() bool {
				var blocked int
				err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%matchmaking_match_squad_operations%'`).Scan(&blocked)
				return err == nil && blocked > 0
			}, 3*time.Second, 10*time.Millisecond)
			drained := probe.closeFresh()
			select {
			case <-drained:
				t.Fatal("fresh close crossed pending receipt/retry SQL after RPC return")
			default:
			}
			require.NoError(t, lock.Commit(ctx))
			select {
			case err := <-result:
				if retry {
					require.Equal(t, codes.Unavailable, status.Code(err))
				} else {
					require.NoError(t, err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("attempt did not finish")
			}
			select {
			case <-drained:
			case <-time.After(time.Second):
				t.Fatal("completed attempt did not release")
			}
			want := "active"
			if retry {
				want = "unavailable"
			}
			require.Equal(t, want, <-releaseObserved, "release follows durable outcome")
			entered, active, released = probe.counts()
			require.Equal(t, 1, entered)
			require.Zero(t, active)
			require.Equal(t, 1, released)
		})
	}
}

func TestForwardLifetimeUnavailableDoesNotAdvance(t *testing.T) {
	for _, stage := range []string{"missing intent", "existing resume", "activation only"} {
		for _, failure := range []string{"missing callback", "failed acquire", "nil release"} {
			t.Run(stage+"/"+failure, func(t *testing.T) {
				ctx := context.Background()
				pool, st, p, intent := admittedProvisionFixture(t)
				if stage == "existing resume" {
					candidate, proof := dispatchProof(t, st, intent.MatchID, p)
					_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
					require.NoError(t, err)
				}
				if stage == "activation only" {
					dispatchBothReceipts(t, st, p, intent)
				}
				probe := &forwardProbe{fail: errors.New("private adapter error")}
				var lifetime squad.ForwardLifetime = probe
				if failure == "missing callback" {
					lifetime = nil
				}
				if failure == "nil release" {
					probe.fail = nil
					probe.nilRelease = true
				}
				worker, chat, voice := lifetimeWorker(t, st, p, lifetime)
				var err error
				if stage == "existing resume" {
					_, err = worker.RunProvisioningOnce(ctx, 10)
				} else {
					_, _, err = worker.Provision(ctx, intent.MatchID, p.MemberProfileIDs)
				}
				require.ErrorIs(t, err, squad.ErrPartyProvisionUnknown)
				require.NotContains(t, err.Error(), "private adapter error")
				require.Zero(t, chat.calls)
				require.Zero(t, voice.calls)
				match, err := st.Get(ctx, intent.MatchID)
				require.NoError(t, err)
				require.Equal(t, store.MatchStatusPendingAccept, match.Status)
				var cancelled int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE match_id=$1 AND status='cancelled'`, intent.MatchID).Scan(&cancelled))
				require.Zero(t, cancelled)
				if stage == "missing intent" {
					var count int
					require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&count))
					require.Zero(t, count)
				}
			})
		}
	}
}

func TestForwardLifetimeRetryUsesDistinctRuntimeIdentity(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	probe := &forwardProbe{}
	worker, chat, _ := lifetimeWorker(t, st, p, probe)
	chat.create = func(*chatv1.CreateMatchSquadChatRequest) (*chatv1.CreateMatchSquadChatResponse, error) {
		return nil, status.Error(codes.Unavailable, "unknown creation")
	}
	for range 2 {
		_, _, err := worker.Provision(ctx, intent.MatchID, p.MemberProfileIDs)
		require.Error(t, err)
		_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_next_attempt_at=clock_timestamp()-interval '1 second' WHERE match_id=$1`, intent.MatchID)
		require.NoError(t, err)
	}
	entered, active, released := probe.counts()
	require.Equal(t, 2, entered)
	require.Zero(t, active)
	require.Equal(t, 2, released)
	require.NotEqual(t, probe.ids[0], probe.ids[1])
}

type lifetimeCleanupChat struct{ *gateChatClient }

func (c *lifetimeCleanupChat) TeardownMatchSquadChat(_ context.Context, r *chatv1.TeardownMatchSquadChatRequest, _ ...grpc.CallOption) (*chatv1.TeardownMatchSquadChatResponse, error) {
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(r)
	if err != nil {
		return nil, err
	}
	return &chatv1.TeardownMatchSquadChatResponse{Receipt: &chatv1.MatchSquadChatTeardownReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), TeardownOperationId: r.TeardownOperationId, MatchId: r.MatchId, ChatId: r.ChatId, CreationReceiptId: r.CreationReceiptId, ParticipantManifestSha256: r.ParticipantManifestSha256, RequestSha256: dispatchDigest(raw), Status: chatv1.MatchSquadTeardownStatus_MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED, CompletedAt: timestamppb.Now()}}, nil
}

func TestForwardLifetimeClosedStillAllowsAbandonedReplayAndCleanup(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	var partyID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM parties WHERE search_operation_id=$1`, p.SearchOperationID).Scan(&partyID))
	_, err = st.ResetVoiceParties(ctx, []uuid.UUID{partyID})
	require.NoError(t, err)
	probe := &forwardProbe{closed: true}
	worker, chat, voice := lifetimeWorker(t, st, p, probe)
	worker.PartyValidator = nil
	worker.Chat = &lifetimeCleanupChat{chat}
	_, _, err = worker.Provision(ctx, intent.MatchID, p.MemberProfileIDs)
	require.Error(t, err)
	require.Equal(t, 1, chat.calls)
	require.Zero(t, voice.calls)
	entered, _, _ := probe.counts()
	require.Zero(t, entered, "receipt-only settlement must bypass fresh registration")
	done, err := worker.RunTeardownOnce(ctx, 10)
	require.NoError(t, err)
	require.Equal(t, 1, done)
	entered, _, _ = probe.counts()
	require.Zero(t, entered, "teardown must remain independent while closed")
	op, err := st.GetMatchSquadProvisioningOperation(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, "closed", op.State)
}
