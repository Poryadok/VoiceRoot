package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/creatorauthority"
	"voice/backend/space/internal/lifecycleprincipal"
	"voice/backend/space/internal/store"
)

func TestCreatorRuntimeConfigRequiresCompleteDedicatedTransport(t *testing.T) {
	names := []string{"SPACE_CREATOR_AUTH_GRPC_ADDR", "SPACE_CREATOR_AUTH_TLS_CA_FILE", "SPACE_CREATOR_AUTH_TLS_SERVER_NAME", "SPACE_CREATOR_AUTH_CLIENT_CERT_FILE", "SPACE_CREATOR_AUTH_CLIENT_KEY_FILE"}
	values := map[string]string{"DATABASE_URL": "fixture"}
	for _, name := range names {
		values[name] = "explicit"
	}
	_, enabled, err := loadCreatorRuntimeConfig(func(string) string { return "" }, false)
	require.NoError(t, err)
	require.False(t, enabled)
	for _, missing := range append(append([]string{}, names...), "DATABASE_URL", "protected listener") {
		t.Run(missing, func(t *testing.T) {
			_, _, err := loadCreatorRuntimeConfig(func(name string) string {
				if name == missing {
					return ""
				}
				return values[name]
			}, missing != "protected listener")
			require.Error(t, err)
		})
	}
	for _, field := range names {
		_, _, err := loadCreatorRuntimeConfig(func(name string) string {
			if name == field {
				return " explicit "
			}
			return values[name]
		}, true)
		require.Error(t, err)
	}
	_, enabled, err = loadCreatorRuntimeConfig(func(name string) string { return values[name] }, true)
	require.NoError(t, err)
	require.True(t, enabled)
	t.Run("workload TLS material", func(t *testing.T) {
		pki := captureRuntimeCredentials(t)
		config := creatorRuntimeConfig{address: "auth:9092", caFile: pki.config.caFile, serverName: "auth", certFile: pki.config.certFile, keyFile: pki.config.keyFile}
		transport, err := creatorAuthTLS(config)
		require.NoError(t, err)
		require.Equal(t, uint16(tls.VersionTLS12), transport.MinVersion)
		require.False(t, transport.InsecureSkipVerify)
		require.Equal(t, "auth", transport.ServerName)
		require.NotNil(t, transport.RootCAs)
		require.Len(t, transport.Certificates, 1)
		for _, missing := range []string{"endpoint", "server name", "CA", "certificate", "key"} {
			broken := config
			switch missing {
			case "endpoint":
				broken.address = ""
			case "server name":
				broken.serverName = ""
			case "CA":
				broken.caFile = filepath.Join(t.TempDir(), "missing")
			case "certificate":
				broken.certFile = config.caFile
			case "key":
				broken.keyFile = config.caFile
			}
			_, err := creatorAuthTLS(broken)
			require.Error(t, err)
		}
		_, err = newCreatorRuntime(context.Background(), context.Background(), config, nil, nil)
		require.Error(t, err, "missing durable store/issuer must not dial")
	})
}

type creatorFloorProbe struct {
	authv1.AuthServiceClient
	key     *rsa.PublicKey
	account string
	floor   int64
	err     error
	calls   int
}

func (p *creatorFloorProbe) GetSpaceSessionEpochFloor(ctx context.Context, r *authv1.GetSpaceSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetSpaceSessionEpochFloorResponse, error) {
	p.calls++
	md, _ := metadata.FromOutgoingContext(ctx)
	auth, ids := md.Get("authorization"), md.Get("x-request-id")
	if len(auth) != 1 || len(ids) != 1 || r.AccountId != p.account {
		return nil, errors.New("binding mismatch")
	}
	hash, _ := principal.RequestHash(r)
	_, err := principal.VerifyService(ctx, strings.TrimPrefix(auth[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "space", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_GetSpaceSessionEpochFloor_FullMethodName, ExpectedRequestID: ids[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return p.key, nil }})
	if err != nil {
		return nil, err
	}
	if p.err != nil {
		return nil, p.err
	}
	return &authv1.GetSpaceSessionEpochFloorResponse{SessionEpochFloor: p.floor}, nil
}

type creatorReplayProbe struct {
	err         error
	calls       int
	issuer, jti string
	expiry      time.Time
}

func (p *creatorReplayProbe) ClaimCreatorPrincipalReplay(_ context.Context, issuer, jti string, expiry time.Time) error {
	p.calls++
	p.issuer, p.jti, p.expiry = issuer, jti, expiry
	return p.err
}

// This exercises the real signed Checker and the adapter, not a live Auth/PG provider.
func TestCreatorDependenciesUseSignedFreshFloorAndDurableReplayBoundary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account := uuid.NewString()
	floor := &creatorFloorProbe{key: &key.PublicKey, account: account, floor: 7}
	replay := &creatorReplayProbe{}
	deps, err := creatorDependencies(floor, signer, replay)
	require.NoError(t, err)
	require.NoError(t, deps.Epoch(context.Background(), account, 7))
	require.NoError(t, deps.Epoch(context.Background(), account, 8))
	require.Equal(t, 2, floor.calls)
	require.ErrorIs(t, deps.Epoch(context.Background(), account, 6), creatorauthority.ErrSessionRevoked)
	floor.err = errors.New("owner unavailable")
	require.ErrorIs(t, deps.Epoch(context.Background(), account, 7), creatorauthority.ErrUnavailable)
	expiry := time.Now().Add(20 * time.Second)
	require.NoError(t, deps.Replay(context.Background(), "gateway", "bound-jti", expiry))
	require.Equal(t, "gateway", replay.issuer)
	require.Equal(t, "bound-jti", replay.jti)
	require.Equal(t, expiry, replay.expiry)
	replay.err = store.ErrMemberAuthorityUnavailable
	require.ErrorIs(t, deps.Replay(context.Background(), "gateway", "bound-jti", expiry), lifecycleprincipal.ErrDependencyUnavailable)
	replay.err = store.ErrMemberAdmissionConflict
	require.ErrorIs(t, deps.Replay(context.Background(), "gateway", "bound-jti", expiry), store.ErrMemberAdmissionConflict)
	_, err = creatorDependencies(nil, signer, replay)
	require.Error(t, err)
	_, err = creatorDependencies(floor, nil, replay)
	require.Error(t, err)
	_, err = creatorDependencies(floor, signer, nil)
	require.Error(t, err)
}

type creatorFailureCloserProbe struct {
	calls  int
	events *[]string
	err    error
}

func (p *creatorFailureCloserProbe) Close() error {
	p.calls++
	*p.events = append(*p.events, "close")
	return p.err
}

func TestCreatorRuntimeCloseAndFailureBoundary(t *testing.T) {
	// NewClient is idle: no Connect/RPC/listener exists in this lifetime test.
	conn, err := grpc.NewClient("passthrough:///creator-not-dialed", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "auth"})))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var cancellations, joins atomic.Int32
	done := make(chan struct{})
	go func() { <-ctx.Done(); joins.Add(1); close(done) }()
	runtime := &creatorRuntime{conn: conn, cancel: func() { cancellations.Add(1); cancel() }, done: done}
	t.Cleanup(func() { _ = runtime.Close() })
	original := errors.New("original startup failure")
	terminated := 0
	for i := 0; i < 2; i++ {
		failCreatorRuntime(runtime, original, func(cause error) {
			terminated++
			require.Same(t, original, cause)
			require.EqualValues(t, 1, cancellations.Load(), "must cancel exactly once before termination")
			require.EqualValues(t, 1, joins.Load(), "worker must join before termination")
			require.Equal(t, connectivity.Shutdown, conn.GetState(), "owned conn must close before termination")
		})
	}
	require.Equal(t, 2, terminated)
	events := []string{}
	closer := &creatorFailureCloserProbe{events: &events, err: errors.New("secondary cleanup failure")}
	failCreatorRuntime(closer, original, func(cause error) { require.Same(t, original, cause); events = append(events, "terminate") })
	require.Equal(t, []string{"close", "terminate"}, events)
	require.Equal(t, 1, closer.calls)
	failCreatorRuntime(nil, original, func(cause error) { require.Same(t, original, cause) })
}

type creatorLifetimeCounts struct{ cancel, join atomic.Int32 }
type creatorLifetimeSnapshot struct {
	cause        error
	cancel, join int32
	state        connectivity.State
}

func creatorReceive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("owned lifecycle event did not complete")
	}
	var zero T
	return zero
}
func creatorOwnedTestRuntime(t *testing.T, release <-chan struct{}) (*creatorRuntime, *creatorLifetimeCounts, <-chan struct{}) {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///creator-not-dialed", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: "auth"})))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	counts := &creatorLifetimeCounts{}
	done, canceled := make(chan struct{}), make(chan struct{})
	go func() {
		<-ctx.Done()
		close(canceled)
		if release != nil {
			<-release
		}
		counts.join.Add(1)
		close(done)
	}()
	runtime := &creatorRuntime{conn: conn, cancel: func() { counts.cancel.Add(1); cancel() }, done: done}
	t.Cleanup(func() { _ = runtime.Close() })
	return runtime, counts, canceled
}
func creatorSnapshot(runtime *creatorRuntime, counts *creatorLifetimeCounts, cause error) creatorLifetimeSnapshot {
	return creatorLifetimeSnapshot{cause: cause, cancel: counts.cancel.Load(), join: counts.join.Load(), state: runtime.conn.GetState()}
}
func creatorRequireClosed(t *testing.T, snapshot creatorLifetimeSnapshot, cause error) {
	t.Helper()
	require.Same(t, cause, snapshot.cause)
	require.EqualValues(t, 1, snapshot.cancel)
	require.EqualValues(t, 1, snapshot.join, "every fatal must observe completed cleanup")
	require.Equal(t, connectivity.Shutdown, snapshot.state)
}

type creatorObservedCloser struct {
	runtime *creatorRuntime
	entered chan int32
	calls   atomic.Int32
}

func (c *creatorObservedCloser) Close() error { c.entered <- c.calls.Add(1); return c.runtime.Close() }

// Channel barriers establish positive ordering; time bounds only fail a stuck
// test. No sleeps, real listener, Auth/PG peer or blocked-call-as-PASS oracle.
func TestCreatorRuntimeOwnerCoordinatesAsyncFailures(t *testing.T) {
	original := errors.New("external callback failure")
	t.Run("closed owner forbids allocation and publication", func(t *testing.T) {
		owner := &creatorRuntimeOwner{}
		terminated := false
		failCreatorRuntime(owner, original, func(cause error) { require.Same(t, original, cause); terminated = true })
		require.True(t, terminated)
		called := false
		_, err := owner.construct(context.Background(), func(context.Context) (*creatorRuntime, error) { called = true; return nil, nil })
		require.Error(t, err)
		require.False(t, called)
		require.Error(t, owner.publish(func() { called = true }))
		require.False(t, called)
		require.NoError(t, owner.Close())
	})
	t.Run("failure during construction registers then closes", func(t *testing.T) {
		owner := &creatorRuntimeOwner{}
		runtime, counts, _ := creatorOwnedTestRuntime(t, nil)
		factoryEntered, allowReturn, failureQueued := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var release sync.Once
		t.Cleanup(func() { release.Do(func() { close(allowReturn) }) })
		constructed := make(chan error, 1)
		terminated := make(chan creatorLifetimeSnapshot, 1)
		go func() {
			_, err := owner.construct(context.Background(), func(startup context.Context) (*creatorRuntime, error) {
				close(factoryEntered)
				<-allowReturn
				if _, ok := startup.Deadline(); !ok {
					return runtime, errors.New("bounded construction required")
				}
				return runtime, nil
			})
			constructed <- err
		}()
		creatorReceive(t, factoryEntered)
		go func() {
			close(failureQueued)
			failCreatorRuntime(owner, original, func(cause error) { terminated <- creatorSnapshot(runtime, counts, cause) })
		}()
		creatorReceive(t, failureQueued)
		release.Do(func() { close(allowReturn) })
		require.NoError(t, creatorReceive(t, constructed))
		creatorRequireClosed(t, creatorReceive(t, terminated), original)
		require.Error(t, owner.publish(func() { t.Error("closed runtime published") }))
	})
	t.Run("registered runtime then asynchronous failure", func(t *testing.T) {
		owner := &creatorRuntimeOwner{}
		runtime, counts, _ := creatorOwnedTestRuntime(t, nil)
		trigger := make(chan struct{})
		terminated := make(chan creatorLifetimeSnapshot, 1)
		go func() {
			<-trigger
			failCreatorRuntime(owner, original, func(cause error) { terminated <- creatorSnapshot(runtime, counts, cause) })
		}()
		var startup context.Context
		_, err := owner.construct(context.Background(), func(ctx context.Context) (*creatorRuntime, error) { startup = ctx; return runtime, nil })
		require.NoError(t, err)
		require.ErrorIs(t, startup.Err(), context.Canceled)
		require.Zero(t, counts.cancel.Load(), "startup deadline is not worker lifetime")
		published := 0
		require.NoError(t, owner.publish(func() { published++ }))
		require.Equal(t, 1, published)
		close(trigger)
		creatorRequireClosed(t, creatorReceive(t, terminated), original)
		require.NoError(t, owner.Close())
		require.EqualValues(t, 1, counts.cancel.Load())
	})
	t.Run("concurrent closers wait", func(t *testing.T) {
		releaseWorker := make(chan struct{})
		runtime, counts, canceled := creatorOwnedTestRuntime(t, releaseWorker)
		var release sync.Once
		unblock := func() { release.Do(func() { close(releaseWorker) }) }
		t.Cleanup(unblock)
		observed := &creatorObservedCloser{runtime: runtime, entered: make(chan int32, 4)}
		owner := &creatorRuntimeOwner{runtime: observed}
		first, second := make(chan creatorLifetimeSnapshot, 1), make(chan creatorLifetimeSnapshot, 1)
		go failCreatorRuntime(owner, original, func(cause error) { first <- creatorSnapshot(runtime, counts, cause) })
		require.EqualValues(t, 1, creatorReceive(t, observed.entered))
		creatorReceive(t, canceled)
		// Close must release the owner mutex BEFORE joining the held worker.
		publication := make(chan error, 1)
		go func() { publication <- owner.publish(func() { t.Error("closed owner published") }) }()
		require.Error(t, creatorReceive(t, publication))
		other := errors.New("second external failure")
		go failCreatorRuntime(owner, other, func(cause error) { second <- creatorSnapshot(runtime, counts, cause) })
		var early *creatorLifetimeSnapshot
		select {
		case entered := <-observed.entered:
			require.EqualValues(t, 2, entered)
		case snapshot := <-second:
			early = &snapshot
		case <-time.After(3 * time.Second):
			unblock()
			t.Fatal("second closer neither entered cleanup nor terminated")
		}
		unblock()
		creatorRequireClosed(t, creatorReceive(t, first), original)
		if early != nil {
			creatorRequireClosed(t, *early, other)
		} else {
			creatorRequireClosed(t, creatorReceive(t, second), other)
		}
		require.EqualValues(t, 1, counts.cancel.Load())
		require.EqualValues(t, 1, counts.join.Load())
	})
	t.Run("publication commits before closure", func(t *testing.T) {
		owner := &creatorRuntimeOwner{}
		runtime, counts, _ := creatorOwnedTestRuntime(t, nil)
		_, err := owner.construct(context.Background(), func(context.Context) (*creatorRuntime, error) { return runtime, nil })
		require.NoError(t, err)
		entered, queued := make(chan struct{}), make(chan struct{})
		committed := make(chan error, 1)
		terminated := make(chan creatorLifetimeSnapshot, 1)
		go func() { committed <- owner.publish(func() { close(entered); <-queued }) }()
		creatorReceive(t, entered)
		go func() {
			close(queued)
			failCreatorRuntime(owner, original, func(cause error) { terminated <- creatorSnapshot(runtime, counts, cause) })
		}()
		require.NoError(t, creatorReceive(t, committed))
		creatorRequireClosed(t, creatorReceive(t, terminated), original)
		require.Error(t, owner.publish(func() { t.Error("closed owner republished") }))
	})
	t.Run("constructor error cannot publish", func(t *testing.T) {
		owner := &creatorRuntimeOwner{}
		runtime, counts, _ := creatorOwnedTestRuntime(t, nil)
		_, err := owner.construct(context.Background(), func(context.Context) (*creatorRuntime, error) { return runtime, original })
		require.Same(t, original, err)
		creatorRequireClosed(t, creatorSnapshot(runtime, counts, original), original)
		require.Error(t, owner.publish(func() { t.Error("failed constructor published") }))
		require.NoError(t, owner.Close())
		empty := &creatorRuntimeOwner{}
		_, err = empty.construct(context.Background(), func(context.Context) (*creatorRuntime, error) { return nil, nil })
		require.Error(t, err)
		require.NoError(t, empty.Close())
	})
}
