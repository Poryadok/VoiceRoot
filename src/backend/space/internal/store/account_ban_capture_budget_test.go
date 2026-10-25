//go:build spacebanintegration

package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/lifecycleprincipal"
)

type banBudgetQueryKey struct{}

// Delay only the return from the actual snapshot-selection COMMIT. Its SQL,
// final ActorFor and database commit have already succeeded. This models a
// scheduling stall before remote dispatch without changing production seams,
// clocks, native guards, catalog definitions or authenticated proof contents.
type banBudgetSnapshotBoundary struct {
	mu       sync.Mutex
	selected map[*pgx.Conn]bool
	commits  int
	stopAt   int
	reached  chan struct{}
	resume   chan struct{}
}

func (b *banBudgetSnapshotBoundary) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, banBudgetQueryKey{}, strings.TrimSpace(data.SQL))
}

func (b *banBudgetSnapshotBoundary) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		return
	}
	sql, _ := ctx.Value(banBudgetQueryKey{}).(string)
	b.mu.Lock()
	if strings.HasPrefix(sql, "UPDATE space_account_ban_operations SET selected_capture_id=") && data.CommandTag.RowsAffected() == 1 {
		b.selected[conn] = true
	}
	stop := false
	if strings.EqualFold(sql, "commit") && b.selected[conn] {
		delete(b.selected, conn)
		b.commits++
		stop = b.commits == b.stopAt
	}
	b.mu.Unlock()
	if stop {
		close(b.reached)
		<-b.resume
	}
}

// Use the real signed receiver and its existing verifier Clock contract. A
// controlled Auth epoch/replay dependency is component evidence, not B15 or a
// public constructor for VerifiedAdmissionCall.
func banBudgetVerifiedCall(t *testing.T, key *rsa.PrivateKey, request *spacev1.BanMemberRequest, clock func() time.Time, credentialExpiry time.Time) (context.Context, *lifecycleprincipal.VerifiedAdmissionCall) {
	t.Helper()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key, Clock: clock})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	transport := uuid.NewString()
	method := spacev1.SpaceService_BanMember_FullMethodName
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: transport, RequestHash: hash, AccountID: nativeBanAccount(1).String(), ProfileID: nativeBanProfile(1).String(), SessionEpoch: 7, AccountType: "regular", ClientExpiresAt: credentialExpiry})
	require.NoError(t, err)
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", transport))
	verifier := lifecycleprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Clock: clock, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	var verified context.Context
	var call *lifecycleprincipal.VerifiedAdmissionCall
	_, err = verifier.Unary()(incoming, request, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		verified = ctx
		var err error
		call, err = lifecycleprincipal.RequireModerationBan(ctx, request)
		return nil, err
	})
	require.NoError(t, err)
	return verified, call
}

func TestAccountBanCaptureDispatchBudget50(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "fresh real34 seeded,45/trusted45,50/trusted50 fixture required")
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(admin.Close)
	cases := []struct {
		name       string
		credential time.Duration
		advance    time.Duration
		expired    bool
		near       bool
		replan     bool
	}{
		{name: "phase expired after snapshot", credential: time.Minute, advance: 5 * time.Second, expired: true},
		{name: "credential expired after snapshot", credential: 2 * time.Second, advance: 2 * time.Second, expired: true},
		{name: "remaining phase bounds owner cancellation", credential: time.Minute, advance: 4900 * time.Millisecond, near: true},
		{name: "earlier credential bounds owner cancellation", credential: 2 * time.Second, advance: 1900 * time.Millisecond, near: true},
		{name: "in budget retains two second cap", credential: time.Minute},
		{name: "replan cannot restart original phase", credential: time.Minute, advance: 5 * time.Second, expired: true, replan: true},
	}
	// Enroll genuine pre-existing memberships before granting any serving role.
	for i := range cases {
		space := nativeBanSpace(30 + i)
		require.NoError(t, (&SpaceStore{Pool: admin}).withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			return scoped.InitializeScopedNativeMembership(ctx, space)
		}))
	}
	serving := projectionServingLogin(t, ctx, admin)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			space, operation := nativeBanSpace(30+i), uuid.New()
			boundary := &banBudgetSnapshotBoundary{selected: make(map[*pgx.Conn]bool), stopAt: 1, reached: make(chan struct{}), resume: make(chan struct{})}
			if tc.replan {
				boundary.stopAt = 2
			}
			var release sync.Once
			cfg := serving.Config().Copy()
			cfg.ConnConfig.Tracer = boundary
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			require.NoError(t, err)
			defer pool.Close()
			defer release.Do(func() { close(boundary.resume) })
			base := time.Now().UTC().Truncate(time.Second)
			var advance atomic.Int64
			clock := func() time.Time { return base.Add(time.Duration(advance.Load())) }
			request := &spacev1.BanMemberRequest{SpaceId: space.String(), AccountId: nativeBanAccount(2).String()}
			nativeBanIntentField(t, request, operation)
			verified, call := banBudgetVerifiedCall(t, key, request, clock, base.Add(tc.credential))
			var calls atomic.Int32
			observed := make(chan time.Duration, 2)
			waited := make(chan time.Duration, 1)
			st := &SpaceStore{Pool: pool}
			st.AccountCaptures = nativeBanCaptureFixture(func(callCtx context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
				n := calls.Add(1)
				deadline, ok := callCtx.Deadline()
				if !ok {
					observed <- -1
					return nil, ErrAccountBanUnavailable
				}
				observed <- time.Until(deadline)
				if tc.near {
					started := time.Now()
					<-callCtx.Done()
					waited <- time.Since(started)
					return nil, callCtx.Err()
				}
				if tc.replan && n == 1 {
					_, err := st.PrepareMemberAdmission(callCtx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space, ProfileID: nativeBanProfile(9), AccountID: nativeBanAccount(2)})
					if err != nil {
						return nil, err
					}
				}
				return nativeBanOwnerFixture(r), nil
			})
			result := make(chan error, 1)
			go func() {
				result <- st.BanAccount(verified, AccountBanInput{SpaceID: space, AccountID: nativeBanAccount(2), ActorProfileID: nativeBanProfile(1), InitiatingCall: call, Request: request, Authorize: func(context.Context) error { return nil }})
			}()
			select {
			case <-boundary.reached:
				advance.Store(int64(tc.advance))
				release.Do(func() { close(boundary.resume) })
			case err := <-result:
				t.Fatalf("did not reach actual snapshot-selection commit boundary: %v", err)
			case <-time.After(15 * time.Second):
				t.Fatal("snapshot-selection commit boundary was not reached")
			}
			select {
			case err = <-result:
			case <-time.After(10 * time.Second):
				t.Fatal("owner capture did not leave its bounded phase")
			}
			if tc.expired || tc.near {
				require.Error(t, err)
				nativeBanAssertNoEffect(t, serving, space)
				var qualifications int
				require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_qualifications WHERE space_id=$1`, space).Scan(&qualifications))
				require.Zero(t, qualifications)
			} else {
				require.NoError(t, err, "in-budget owner capture remains usable")
			}
			wantCalls := int32(1)
			if tc.expired && !tc.replan {
				wantCalls = 0
			}
			require.Equal(t, wantCalls, calls.Load(), "expired phase must never dispatch a new owner capture")
			if tc.near {
				budget := <-observed
				require.Positive(t, budget)
				require.LessOrEqual(t, budget, 150*time.Millisecond, "owner receives remaining100ms, not a fresh2s")
				require.Less(t, <-waited, 500*time.Millisecond, "owner cancellation follows remaining authenticated phase")
			} else if !tc.expired {
				budget := <-observed
				require.Greater(t, budget, 1500*time.Millisecond)
				require.LessOrEqual(t, budget, 2*time.Second)
			}
			var children int
			require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_captures WHERE actor_profile_id=$1 AND client_operation_id=$2`, nativeBanProfile(1), operation).Scan(&children))
			require.Equal(t, boundary.stopAt, children, "each real local replan selects its own immutable child without restarting freshness")
		})
	}
}
