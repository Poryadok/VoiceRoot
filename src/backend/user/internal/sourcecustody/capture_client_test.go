package sourcecustody

import (
	"context"
	"crypto/sha256"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/identityassociation"
	"voice/backend/user/internal/testsupport/associationfixture"
)

func installStreamPhysical(t *testing.T, f streamFixture, source *pgxpool.Pool) {
	t.Helper()
	var physical PhysicalSource
	require.NoError(t, source.QueryRow(f.ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&physical.SystemIdentifier, &physical.Database, &physical.DatabaseOID))
	_, err := f.pool.Exec(f.ctx, `CREATE TABLE user_custody_physical_source(source_id text NOT NULL,epoch bigint NOT NULL,system_identifier text NOT NULL,database_name text NOT NULL,database_oid oid NOT NULL,PRIMARY KEY(source_id,epoch))`)
	require.NoError(t, err)
	_, err = f.pool.Exec(f.ctx, `INSERT INTO user_custody_physical_source VALUES($1,$2,$3,$4,$5)`, f.control.binding.SourceID, f.control.binding.Epoch, physical.SystemIdentifier, physical.Database, physical.DatabaseOID)
	require.NoError(t, err)
}

func TestCustodiedSpaceAuthorityUsesLiveGrant(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(db.Pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	binding := custodyBinding()
	binding.HistoryGeneration = receipt.HistoryGeneration
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT user_source_generation FROM identity_meta WHERE singleton`).Scan(&binding.UserSourceGeneration))
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	guarded, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	authority, err := grpcsvc.NewCustodiedSpaceAccountCaptureAuthority(guarded.CaptureProfiles)
	require.NoError(t, err)
	request := &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: uuid.NewString(), SpaceId: uuid.NewString(), Purpose: 2, Cohort: &userv1.SpaceAccountCaptureCohort{Entries: []*userv1.SpaceAccountCaptureEntry{{ProfileId: db.Profiles[0].String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{PublicationId: uuid.NewString()}}}}}}
	request.CohortDigest = spaceaccountcapture.CohortDigest(request.Cohort)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	verified := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "user", RPC: userv1.UserService_CaptureSpaceMemberAccountBindings_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, JWTID: uuid.NewString(), ExpiresAt: time.Now().Add(time.Minute)})
	handler := &grpcsvc.SpaceAccountCaptureGRPC{Authority: authority}
	response, err := handler.CaptureSpaceMemberAccountBindings(verified, request)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(request, response))
	require.Equal(t, db.Accounts[db.Profiles[0]].String(), response.Results[0].AccountId)
	spaceClient, spaceIssuer := spaceCaptureTransportFixture(t, authority)
	response, err = signedSpaceCapture(ctx, spaceClient, spaceIssuer, request)
	require.NoError(t, err)
	require.NoError(t, spaceaccountcapture.ValidateResponse(request, response))
	require.Equal(t, db.Accounts[db.Profiles[0]].String(), response.Results[0].AccountId)
	require.NoError(t, f.control.CloseAdmission(ctx))
	require.NoError(t, f.control.WaitPermits(ctx))
	_, err = handler.CaptureSpaceMemberAccountBindings(verified, request)
	require.Equal(t, codes.Unavailable, status.Code(err), "closed custody must deny even an already captured profile")
	_, err = signedSpaceCapture(ctx, spaceClient, spaceIssuer, request)
	require.Equal(t, codes.Unavailable, status.Code(err), "the protected transport must also fail after custody closes")
}

type capturePreReadTrace struct {
	count         atomic.Int32
	started       chan uint32
	commitArmed   atomic.Bool
	commitStarted chan struct{}
	commitRelease chan struct{}
}

func TestCaptureClientRejectsDifferentUserSourceBeforeMutation(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(db.Pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var before uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	binding := custodyBinding()
	binding.HistoryGeneration = receipt.HistoryGeneration
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	// The control service grants this independently pinned U, but the User DB
	// has another U. The source must be rejected before a retained capture commits.
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	capture, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	_, err = capture.CaptureProfiles(ctx, db.Profiles)
	require.Error(t, err)
	var after, associations uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&after, &associations))
	require.Equal(t, before, after)
	require.Zero(t, associations)
}

func TestCaptureClientRejectsDifferentPhysicalSourceBeforeMutation(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(db.Pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var before uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	binding := custodyBinding()
	binding.HistoryGeneration, binding.UserSourceGeneration = receipt.HistoryGeneration, receipt.UserSourceGeneration
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	_, err = f.pool.Exec(ctx, `UPDATE user_custody_physical_source SET database_name='wrong_source' WHERE source_id=$1 AND epoch=$2`, binding.SourceID, binding.Epoch)
	require.NoError(t, err)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	probe, err := client.AcquireQualified(ctx, binding)
	require.NoError(t, err)
	require.Equal(t, "wrong_source", probe.Physical().Database)
	probe.Release()
	capture, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	_, err = capture.CaptureProfiles(ctx, db.Profiles)
	require.Error(t, err)
	require.EqualValues(t, 2, f.tracer.attempts.Load(), "both calls acquired a real control permit")
	var after, associations uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&after, &associations))
	require.Equal(t, before, after)
	require.Zero(t, associations)
}

func TestCaptureClientRejectsDifferentKeysetBeforeMutation(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(db.Pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var before uint64
	var source uuid.UUID
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head,user_source_generation FROM identity_meta WHERE singleton`).Scan(&before, &source))
	binding := custodyBinding()
	binding.HistoryGeneration, binding.UserSourceGeneration = receipt.HistoryGeneration, source
	binding.KeysetDigest = [32]byte{0x44}
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, nil)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	capture, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	_, err = capture.CaptureProfiles(ctx, db.Profiles)
	require.Error(t, err)
	var after, associations uint64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT history_head,(SELECT count(*) FROM identity_associations) FROM identity_meta WHERE singleton`).Scan(&after, &associations))
	require.Equal(t, before, after)
	require.Zero(t, associations)
}

func (t *capturePreReadTrace) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "SELECT id,account_id FROM profiles WHERE id=ANY") {
		t.count.Add(1)
		select {
		case t.started <- conn.PgConn().PID():
		default:
		}
	}
	if strings.EqualFold(strings.TrimSpace(data.SQL), "commit") && t.commitArmed.Swap(false) {
		close(t.commitStarted)
		select {
		case <-t.commitRelease:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (*capturePreReadTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Synthetic OPEN and fixture creation evidence do not establish production
// custody. Only this capture path is exercised, not every possible writer.
func TestCaptureClientHoldsGrantBeforePreReadAndCancelsOnLoss(t *testing.T) {
	ctx := context.Background()
	root := custodyDatabase(t, ctx)
	t.Setenv("USER_ASSOCIATION_TEST_DSN", root.Config().ConnString())
	db := associationfixture.New(t, 1)
	receipt, err := associationfixture.ReadInstallerReceipt(db.InstallerReceiptPath)
	require.NoError(t, err)
	trace := &capturePreReadTrace{started: make(chan uint32, 4), commitStarted: make(chan struct{}), commitRelease: make(chan struct{})}
	config := db.Pool.Config().Copy()
	config.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	fingerprint := sha256.Sum256(receipt.UserKey)
	keys, err := identityassociation.NewKeyset(identityassociation.KeysetSnapshot{Revision: 1, ActiveVersion: receipt.KeyVersion, Versions: map[uint32][32]byte{receipt.KeyVersion: fingerprint}}, map[uint32][]byte{receipt.KeyVersion: receipt.UserKey})
	require.NoError(t, err)
	store, err := identityassociation.NewStore(pool, keys, receipt.HistoryGeneration)
	require.NoError(t, err)
	require.NoError(t, store.Bootstrap(ctx))
	require.NoError(t, store.ActivateGuards(ctx))
	var h, u uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_generation,user_source_generation FROM identity_meta WHERE singleton`).Scan(&h, &u))
	binding := custodyBinding()
	binding.HistoryGeneration = h
	binding.UserSourceGeneration = u
	binding.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	controllerFailed := false
	f := newStreamFixtureBound(t, false, time.Now, 100*time.Millisecond, &binding, &controllerFailed)
	installStreamPhysical(t, f, db.Pool)
	client, err := NewClient(f.client, f.issuer, 500*time.Millisecond)
	require.NoError(t, err)
	_, err = NewCaptureClient(nil, binding, store)
	require.Error(t, err)
	_, err = NewCaptureClient(client, binding, nil)
	require.Error(t, err)
	_, err = NewCaptureClient(client, Binding{}, store)
	require.Error(t, err)
	wrong := binding
	wrong.UserSourceGeneration = uuid.New()
	denied, err := NewCaptureClient(client, wrong, store)
	require.NoError(t, err)
	_, err = denied.CaptureProfiles(ctx, db.Profiles)
	require.Error(t, err)
	require.Zero(t, trace.count.Load(), "denied grant must precede profile pre-read")
	capture, err := NewCaptureClient(client, binding, store)
	require.NoError(t, err)
	cancelled, cancelNow := context.WithCancel(ctx)
	cancelNow()
	_, err = capture.CaptureProfiles(cancelled, db.Profiles)
	require.Error(t, err)
	require.Zero(t, trace.count.Load(), "cancelled parent must not start profile pre-read")
	result, err := capture.CaptureProfiles(ctx, db.Profiles)
	require.NoError(t, err)
	require.Equal(t, identityassociation.CaptureAssociated, result.Observations[db.Profiles[0]].Status)
	select {
	case <-trace.started:
	default:
		t.Fatal("positive capture did not execute actual profile pre-read")
	}
	// A real transaction is paused immediately before pgx sends COMMIT. The
	// independent control server must still show a held shared permit.
	var releaseCommit sync.Once
	release := func() { releaseCommit.Do(func() { close(trace.commitRelease) }) }
	defer release()
	trace.commitArmed.Store(true)
	type captured struct {
		result identityassociation.CaptureBatchResult
		err    error
	}
	commitCtx, stopCommit := context.WithTimeout(ctx, 5*time.Second)
	defer stopCommit()
	committed := make(chan captured, 1)
	commitJoined := false
	go func() { r, e := capture.CaptureProfiles(commitCtx, db.Profiles); committed <- captured{r, e} }()
	defer func() {
		release()
		stopCommit()
		if !commitJoined {
			select {
			case <-committed:
			case <-time.After(5 * time.Second):
				t.Error("commit capture did not join")
			}
		}
	}()
	select {
	case <-trace.commitStarted:
	case got := <-committed:
		commitJoined = true
		t.Fatalf("capture returned before commit gate: %v", got.err)
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("capture did not reach commit")
	}
	var shared int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&shared))
	require.Equal(t, 1, shared, "grant must remain held at User commit")
	release()
	select {
	case got := <-committed:
		commitJoined = true
		require.NoError(t, got.err)
	case <-time.After(2 * time.Second):
		t.Fatal("commit did not finish")
	}
	// Drain the pre-read observation from the committed capture.
	select {
	case <-trace.started:
	default:
		t.Fatal("commit capture did not read profiles")
	}

	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `LOCK TABLE profiles IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	results := make(chan captured, 1)
	joined := false
	captureStarted := time.Now()
	go func() { r, e := capture.CaptureProfiles(callCtx, db.Profiles); results <- captured{r, e} }()
	defer func() {
		cancel()
		_ = blocker.Rollback(context.Background())
		if !joined {
			select {
			case <-results:
			case <-time.After(5 * time.Second):
				t.Error("capture did not join before fixture cleanup")
			}
		}
	}()
	var pid uint32
	select {
	case pid = <-trace.started:
	case got := <-results:
		joined = true
		t.Fatalf("capture returned before blocked profile pre-read: %v", got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("capture pre-read did not start")
	}
	var blocked bool
	require.Eventually(t, func() bool {
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock' AND query LIKE 'SELECT id,account_id FROM profiles WHERE id=ANY%')`, pid).Scan(&blocked) == nil && blocked
	}, time.Second, 25*time.Millisecond)
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&shared))
	require.Equal(t, 1, shared, "actual control grant must be held while User pre-read is blocked")
	var leaderPID int32
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=(6219270163691208961::bigint >> 32)::oid AND objid=(6219270163691208961::bigint & 4294967295)::oid HAVING count(*)=1`).Scan(&leaderPID))
	var terminated bool
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT pg_terminate_backend($1)`, leaderPID).Scan(&terminated))
	require.True(t, terminated)
	controllerFailed = true
	select {
	case got := <-results:
		joined = true
		require.Error(t, got.err, "loss of control stream must not return capture success")
		require.NoError(t, callCtx.Err(), "failure must precede caller deadline")
		require.Less(t, time.Since(captureStarted), 1500*time.Millisecond, "failure must precede the store's two-second internal timeout")
	case <-time.After(1500 * time.Millisecond):
		t.Fatal("stream loss did not promptly cancel blocked User pre-read")
	}
	// The wrapper has returned; this does not prove DB-wide writer drain.
	require.Eventually(t, func() bool {
		return f.pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&shared) == nil && shared == 0
	}, 5*time.Second, 25*time.Millisecond)
}
