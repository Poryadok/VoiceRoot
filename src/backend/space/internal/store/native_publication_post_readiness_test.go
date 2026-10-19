package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
)

type nativePublicationSQLGate struct{ start func(string) }

func (g nativePublicationSQLGate) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	g.start(data.SQL)
	return ctx
}
func (nativePublicationSQLGate) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Local receipt fingerprints and controlled ingress clocks exercise publication
// ordering, not live Role authorization. The native frame/readiness is real.
func TestNative45PostPublicationReadinessAndDeferredIngressDeadline(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	space, err := admin.CreateSpace(ctx, uuid.New(), "post publication frame", "", "public")
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "goto", "45")
	_, err = admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	st := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	for index, name := range []string{"POST can require completed native frame", "deferred SQL precedes initiating FINAL"} {
		t.Run(name, func(t *testing.T) {
			in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
			intent, err := st.PrepareMemberAdmission(ctx, in)
			require.NoError(t, err)
			role := uuid.New()
			receipt, err := memberadmission.EnsureReceiptFingerprint(intent.EnsureBinding(), role.String())
			require.NoError(t, err)
			initiating, clock := signedNativeInitiatingCall(t, in)
			calls := 0
			if index == 0 {
				hook := func(ctx context.Context, local *SpaceStore, _ *MemberAdmissionIntent, _ *MembershipRow) error {
					calls++
					return local.RequireScopedNativeMembershipReadiness(ctx, space.ID)
				}
				_, err = st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], nil, nil, hook, initiating)
				require.NoError(t, err, "FIRST hook must observe completed history/frame for its actual local readiness gate")
				require.Equal(t, 1, calls)
				_, err = st.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], nil, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent, *MembershipRow) error {
					t.Fatal("completed replay invoked POST")
					return nil
				}, nil)
				require.NoError(t, err)
				return
			}
			var armed atomic.Bool
			entered, release := make(chan string, 1), make(chan struct{})
			var releaseOnce sync.Once
			config := st.Pool.Config()
			config.ConnConfig.Tracer = nativePublicationSQLGate{start: func(sql string) {
				normalized := strings.ToUpper(strings.TrimSpace(sql))
				if (normalized == "COMMIT" || strings.HasPrefix(normalized, "SET CONSTRAINTS")) && armed.CompareAndSwap(true, false) {
					entered <- normalized
					<-release
				}
			}}
			pool, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			defer pool.Close()
			defer releaseOnce.Do(func() { close(release) })
			publisher := &SpaceStore{Pool: pool}
			hook := func(ctx context.Context, local *SpaceStore, _ *MemberAdmissionIntent, _ *MembershipRow) error {
				// Hold the real deferred-check boundary query without altering
				// SQL, catalog, frame or authority. Only the ingress clock moves.
				armed.Store(true)
				return nil
			}
			result := make(chan error, 1)
			go func() {
				_, e := publisher.publishMemberAdmissionWithInitiatingCall(ctx, in.OperationID, role, receipt[:], nil, nil, hook, initiating)
				result <- e
			}()
			select {
			case sql := <-entered:
				t.Logf("held actual deferred-check boundary: %s", sql)
			case e := <-result:
				t.Fatalf("publication returned before deferred boundary: %v", e)
			case <-time.After(4 * time.Second):
				t.Fatal("actual deferred-check boundary did not arrive")
			}
			clock.Store(int64(5100 * time.Millisecond))
			releaseOnce.Do(func() { close(release) })
			require.Error(t, <-result, "oldest initiating call expires while deferred SQL waits; publication must roll back")
			saved, err := st.LookupMemberAdmission(ctx, in.OperationID)
			require.NoError(t, err)
			require.Equal(t, "PREPARING", saved.State)
			var count int
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&count))
			require.Zero(t, count)
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_member_admission_outbox WHERE operation_id=$1`, in.OperationID).Scan(&count))
			require.Zero(t, count)
			var history string
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&history))
			require.Equal(t, "NONE", history)
		})
	}
}
