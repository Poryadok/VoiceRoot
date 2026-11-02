package sourcecustody

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/principal"
)

// This is a private TLS/JWKS/miniredis fixture with synthetic OPEN, not a
// production controller, physical source qualification, or Redis acceptance.
func streamTransportTLS(t *testing.T) (tls.Certificate, tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "custody fixture CA"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	issue := func(serial int64, usage x509.ExtKeyUsage) tls.Certificate {
		leafKey, e := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, e)
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		leafDER, e := x509.CreateCertificate(rand.Reader, leaf, cert, &leafKey.PublicKey, key)
		require.NoError(t, e)
		return tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey}
	}
	return issue(2, x509.ExtKeyUsageServerAuth), issue(3, x509.ExtKeyUsageClientAuth), roots
}

type streamFixture struct {
	ctx        context.Context
	control    *Controller
	pool       *pgxpool.Pool
	client     userv1.UserSourceCustodyServiceClient
	request    *userv1.AcquireUserSourcePermitRequest
	issuer     *principal.Issuer
	signingKey *rsa.PrivateKey
	address    string
	roots      *x509.CertPool
	clientCert tls.Certificate
	cache      *miniredis.Miniredis
	jwks       *httptest.Server
	tracer     *permitQueryTracer
}

type permitQueryTracer struct{ attempts atomic.Int64 }

func (t *permitQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.Contains(q.SQL, "pg_advisory_lock_shared") {
		t.attempts.Add(1)
	}
	return ctx
}
func (*permitQueryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func newStreamFixture(t *testing.T) streamFixture {
	return newStreamFixtureOptions(t, false, time.Now, 100*time.Millisecond)
}

func newStreamFixtureOptions(t *testing.T, expectControllerFailure bool, clock func() time.Time, heartbeat time.Duration, ticks ...<-chan time.Time) streamFixture {
	return newStreamFixtureBound(t, expectControllerFailure, clock, heartbeat, nil, nil, ticks...)
}

func newStreamFixtureBound(t *testing.T, expectControllerFailure bool, clock func() time.Time, heartbeat time.Duration, bindingOverride *Binding, failureOverride *bool, ticks ...<-chan time.Time) streamFixture {
	t.Helper()
	tracer := &permitQueryTracer{}
	ctx, pool, _, binding, control := custodyFixtureBound(t, expectControllerFailure, tracer, bindingOverride, failureOverride)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	jwk, err := json.Marshal(map[string]any{"keys": []any{map[string]string{"kty": "RSA", "kid": "current", "use": "sig", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())}}})
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(jwk) }))
	t.Cleanup(jwks.Close)
	resolver := principal.NewJWKSResolver(func(ctx context.Context, issuer string) ([]byte, error) {
		if issuer != "user" {
			return nil, errors.New("unexpected issuer")
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwks.URL, nil)
		if err != nil {
			return nil, err
		}
		resp, err := jwks.Client().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, errors.New("JWKS unavailable")
		}
		return io.ReadAll(io.LimitReader(resp.Body, 65537))
	})
	cache := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	replay := func(ctx context.Context, issuer, id string, expires time.Time) error {
		ttl := time.Until(expires)
		if ttl <= 0 {
			return errors.New("expired replay")
		}
		ok, err := redisClient.SetNX(ctx, "fixture:"+issuer+":"+id, "1", ttl).Result()
		if err != nil || !ok {
			return errors.New("replay unavailable or already used")
		}
		return nil
	}
	cfg := StreamConfig{KeyResolver: resolver.Resolve, ReplayGuard: replay, Clock: clock, Heartbeat: heartbeat}
	if len(ticks) != 0 {
		cfg.testTicks = ticks[0]
	}
	handler, err := NewStreamServer(control, cfg)
	require.NoError(t, err)
	cert, clientCert, roots := streamTransportTLS(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	userv1.RegisterUserSourceCustodyServiceServer(server, handler)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{clientCert}})))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return streamFixture{ctx: ctx, pool: pool, control: control, tracer: tracer, client: userv1.NewUserSourceCustodyServiceClient(conn), issuer: issuer, signingKey: key, address: listener.Addr().String(), roots: roots, clientCert: clientCert, cache: cache, jwks: jwks, request: &userv1.AcquireUserSourcePermitRequest{Protocol: 1, Binding: &userv1.UserSourcePermitBinding{Epoch: uint64(binding.Epoch), SourceId: binding.SourceID, HistoryGeneration: binding.HistoryGeneration.String(), UserSourceGeneration: binding.UserSourceGeneration.String(), KeysetDigest: binding.KeysetDigest[:]}}}
}

func (f streamFixture) signed(t *testing.T, req *userv1.AcquireUserSourcePermitRequest, aud, rpc, requestID string) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	token, err := f.issuer.IssueService(principal.ServiceInput{Audience: aud, RPC: rpc, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return metadata.NewOutgoingContext(f.ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
}

func TestCustodyStreamGrantedHeartbeatReplayAndDisconnect(t *testing.T) {
	f := newStreamFixture(t)
	method := userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName
	signed := f.signed(t, f.request, "user-source-custody", method, "request-1")
	ctx, cancel := context.WithTimeout(signed, 5*time.Second)
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Protocol)
	require.EqualValues(t, 1, first.Sequence)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, first.Kind)
	require.True(t, proto.Equal(f.request.Binding, first.Binding))
	require.EqualValues(t, 1, f.tracer.attempts.Load(), "positive control observes the real shared permit acquisition")
	second, err := stream.Recv()
	require.NoError(t, err)
	require.EqualValues(t, 2, second.Sequence)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT, second.Kind)
	require.True(t, proto.Equal(f.request.Binding, second.Binding))
	duplicate, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	_, err = duplicate.Recv()
	require.Error(t, err, "one signed request grants at most one stream")
	require.EqualValues(t, 1, f.tracer.attempts.Load(), "replay is rejected before a second permit attempt")
	require.NoError(t, f.control.CloseAdmission(f.ctx))
	bounded, stop := context.WithTimeout(f.ctx, 250*time.Millisecond)
	require.Error(t, f.control.WaitPermits(bounded))
	stop()
	cancel()
	drained, stop := context.WithTimeout(f.ctx, 5*time.Second)
	defer stop()
	require.NoError(t, f.control.WaitPermits(drained), "stream cancellation releases held control permit")
}

func TestCustodyStreamRejectsUnboundOrMalformedRequestsWithoutRetainedPermit(t *testing.T) {
	f := newStreamFixture(t)
	method := userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName
	unknownOuter := proto.Clone(f.request).(*userv1.AcquireUserSourcePermitRequest)
	unknownOuter.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	unknownInner := proto.Clone(f.request).(*userv1.AcquireUserSourcePermitRequest)
	unknownInner.Binding.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
	changed := proto.Clone(f.request).(*userv1.AcquireUserSourcePermitRequest)
	changed.Binding.SourceId = "other-source"
	overflow := proto.Clone(f.request).(*userv1.AcquireUserSourcePermitRequest)
	overflow.Binding.Epoch = 1 << 63
	missing := &userv1.AcquireUserSourcePermitRequest{Protocol: 1}
	for _, tc := range []struct {
		name                                string
		request                             *userv1.AcquireUserSourcePermitRequest
		signedRequest                       *userv1.AcquireUserSourcePermitRequest
		audience, rpc, signedID, metadataID string
	}{
		{"audience", f.request, f.request, "user", method, "id-a", "id-a"},
		{"rpc", f.request, f.request, "user-source-custody", "/wrong.Service/Method", "id-b", "id-b"},
		{"request id", f.request, f.request, "user-source-custody", method, "id-c", "different"},
		{"request hash", changed, f.request, "user-source-custody", method, "id-d", "id-d"},
		{"outer unknown", unknownOuter, unknownOuter, "user-source-custody", method, "id-e", "id-e"},
		{"binding unknown", unknownInner, unknownInner, "user-source-custody", method, "id-f", "id-f"},
		{"source mismatch", changed, changed, "user-source-custody", method, "id-g", "id-g"},
		{"epoch overflow", overflow, overflow, "user-source-custody", method, "id-h", "id-h"},
		{"missing binding", missing, missing, "user-source-custody", method, "id-i", "id-i"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := f.tracer.attempts.Load()
			signed := f.signed(t, tc.signedRequest, tc.audience, tc.rpc, tc.signedID)
			if tc.metadataID != tc.signedID {
				md, _ := metadata.FromOutgoingContext(signed)
				md = md.Copy()
				md.Set("x-request-id", tc.metadataID)
				signed = metadata.NewOutgoingContext(f.ctx, md)
			}
			ctx, cancel := context.WithTimeout(signed, 5*time.Second)
			defer cancel()
			stream, err := f.client.AcquireUserSourcePermit(ctx, tc.request)
			require.NoError(t, err)
			_, err = stream.Recv()
			require.Error(t, err)
			require.NotEqual(t, codes.Unimplemented, status.Code(err), "denial must come from the implemented protected handler")
			require.Equal(t, before, f.tracer.attempts.Load(), "denial must not attempt a shared permit")
			// Independent session: a denial cannot leave a held shared permit.
			probe, err := pgx.ConnectConfig(f.ctx, f.pool.Config().ConnConfig.Copy())
			require.NoError(t, err)
			defer func() { _ = probe.Close(context.Background()) }()
			var locked bool
			require.NoError(t, probe.QueryRow(f.ctx, `SELECT pg_try_advisory_lock(6219270163691208962::bigint)`).Scan(&locked))
			require.True(t, locked)
			require.NoError(t, probe.QueryRow(f.ctx, `SELECT pg_advisory_unlock(6219270163691208962::bigint)`).Scan(&locked))
			require.True(t, locked)
		})
	}
}

func TestCustodyStreamRejectsMissingOrExcessDeadline(t *testing.T) {
	f := newStreamFixture(t)
	method := userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName
	for _, tc := range []struct {
		name   string
		excess bool
	}{{"missing", false}, {"after-expiry", true}} {
		t.Run(tc.name, func(t *testing.T) {
			signed := f.signed(t, f.request, "user-source-custody", method, "deadline-"+tc.name)
			var ctx context.Context
			var cancel context.CancelFunc
			if tc.excess {
				ctx, cancel = context.WithTimeout(signed, time.Minute)
			} else {
				ctx, cancel = context.WithCancel(signed)
			}
			defer cancel()
			result := make(chan error, 1)
			go func() {
				stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
				if err == nil {
					_, err = stream.Recv()
				}
				result <- err
			}()
			select {
			case err := <-result:
				require.Error(t, err)
				require.NotEqual(t, codes.Unimplemented, status.Code(err))
			case <-time.After(3 * time.Second):
				t.Fatal("invalid stream deadline was not rejected")
			}
		})
	}
}

func TestCustodyStreamRejectsUnavailableVerificationDependencies(t *testing.T) {
	for _, dependency := range []string{"jwks", "redis"} {
		t.Run(dependency, func(t *testing.T) {
			f := newStreamFixture(t)
			if dependency == "jwks" {
				f.jwks.Close()
			} else {
				f.cache.Close()
			}
			signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "dependency-"+dependency)
			ctx, cancel := context.WithTimeout(signed, 5*time.Second)
			defer cancel()
			stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
			require.NoError(t, err)
			_, err = stream.Recv()
			require.Error(t, err)
			require.NotEqual(t, codes.Unimplemented, status.Code(err))
			require.Zero(t, f.tracer.attempts.Load(), "dependency failure precedes permit acquisition")
			probe, err := pgx.ConnectConfig(f.ctx, f.pool.Config().ConnConfig.Copy())
			require.NoError(t, err)
			defer func() { _ = probe.Close(context.Background()) }()
			var locked bool
			require.NoError(t, probe.QueryRow(f.ctx, `SELECT pg_try_advisory_lock(6219270163691208962::bigint)`).Scan(&locked))
			require.True(t, locked, "dependency failure must leave no retained permit")
			require.NoError(t, probe.QueryRow(f.ctx, `SELECT pg_advisory_unlock(6219270163691208962::bigint)`).Scan(&locked))
			require.True(t, locked)
		})
	}
}

// This exercises the permitted client deadline at credential expiry; it does
// not prove a server-side timer independent of client cancellation.
func TestCustodyStreamReleasesAtClientDeadline(t *testing.T) {
	// No heartbeat is in flight when the deadline expires; cancellation of a
	// control query can conservatively poison its uncertain connection instead.
	ticks := make(chan time.Time)
	f := newStreamFixtureOptions(t, false, time.Now, 100*time.Millisecond, ticks)
	issued := time.Now().Add(-20 * time.Second)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "user", KeyID: "current", PrivateKey: f.signingKey, Clock: func() time.Time { return issued }})
	require.NoError(t, err)
	f.issuer = issuer
	expires := time.Unix(issued.Add(30*time.Second).Unix(), 0)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "expiry")
	// grpc-timeout rounds its wire duration; leave room below the whole-second
	// JWT expiry rather than accidentally asking the server for a later deadline.
	ctx, cancel := context.WithDeadline(signed, expires.Add(-time.Second))
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, first.Kind)
	for err == nil {
		_, err = stream.Recv()
	}
	require.Error(t, err)
	require.NoError(t, f.control.CloseAdmission(f.ctx))
	drain, stop := context.WithTimeout(f.ctx, 5*time.Second)
	defer stop()
	require.NoError(t, f.control.WaitPermits(drain), "deadline ends stream and releases permit")
}

func TestCustodyStreamEndsWhenLeaderSessionDies(t *testing.T) {
	f := newStreamFixtureOptions(t, true, time.Now, time.Second)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "leader-loss")
	ctx, cancel := context.WithTimeout(signed, 10*time.Second)
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, first.Kind)
	// Match the unique leader lock only on this disposable control server.
	var leaderPID int32
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT min(pid) FROM pg_locks WHERE locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=(6219270163691208961::bigint >> 32)::oid AND objid=(6219270163691208961::bigint & 4294967295)::oid HAVING count(*)=1`).Scan(&leaderPID))
	var terminated bool
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT pg_terminate_backend($1)`, leaderPID).Scan(&terminated))
	require.True(t, terminated)
	// A heartbeat already sent before termination may be buffered in gRPC.
	// Require eventual closure before the client deadline, not its absence.
	for err == nil {
		_, err = stream.Recv()
	}
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "leader loss must close the stream before client deadline")
	probe, err := pgx.ConnectConfig(f.ctx, f.pool.Config().ConnConfig.Copy())
	require.NoError(t, err)
	defer func() { _ = probe.Close(context.Background()) }()
	var locked bool
	require.Eventually(t, func() bool {
		return probe.QueryRow(f.ctx, `SELECT pg_try_advisory_lock(6219270163691208962::bigint)`).Scan(&locked) == nil && locked
	}, 5*time.Second, 25*time.Millisecond)
	require.NoError(t, probe.QueryRow(f.ctx, `SELECT pg_advisory_unlock(6219270163691208962::bigint)`).Scan(&locked))
	require.True(t, locked)
}

func TestCustodyStreamServerExpiresWhileClientDeadlineIsStillLive(t *testing.T) {
	var offset atomic.Int64
	clock := func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }
	f := newStreamFixtureOptions(t, false, clock, 100*time.Millisecond)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "server-expiry")
	ctx, cancel := context.WithTimeout(signed, 10*time.Second)
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, first.Kind)
	// After admission, advance only the server's verified clock past token exp;
	// the real transport context remains live, so client deadline is not the cause.
	offset.Store(int64(31 * time.Second))
	// Previously sent heartbeats may already be buffered; wait for actual close.
	for err == nil {
		_, err = stream.Recv()
	}
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "client context must still be active")
	require.NoError(t, f.control.CloseAdmission(f.ctx))
	drain, stop := context.WithTimeout(f.ctx, 5*time.Second)
	defer stop()
	require.NoError(t, f.control.WaitPermits(drain))
}

func TestCustodyStreamCanceledDuringCheckDrainsWithoutPoisoningController(t *testing.T) {
	ticks := make(chan time.Time, 1)
	f := newStreamFixtureOptions(t, false, time.Now, 100*time.Millisecond, ticks)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "canceled-check")
	ctx, cancel := context.WithTimeout(signed, 10*time.Second)
	t.Cleanup(cancel)
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)
	blocker, err := f.pool.Begin(f.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(f.ctx, `LOCK TABLE user_custody_state IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	ticks <- time.Now()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.user_custody_state'::regclass AND mode='AccessShareLock' AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	cancel()
	_, err = stream.Recv()
	require.Error(t, err)
	// The server must retain its permit while the control query is unresolved.
	// Client cancellation alone cannot establish that an in-flight check ended.
	var held int
	require.NoError(t, f.pool.QueryRow(f.ctx, `SELECT count(*) FROM pg_locks WHERE locktype='advisory' AND mode='ShareLock' AND granted AND objsubid=1 AND classid=(6219270163691208962::bigint >> 32)::oid AND objid=(6219270163691208962::bigint & 4294967295)::oid`).Scan(&held))
	require.Equal(t, 1, held)
	require.NoError(t, blocker.Rollback(f.ctx))
	bounded, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	require.NoError(t, f.control.CloseAdmission(bounded))
	require.NoError(t, f.control.WaitPermits(bounded))
	require.NoError(t, f.control.Shutdown(bounded))
}

func TestCustodyStreamControlCheckTimeoutPoisonsController(t *testing.T) {
	ticks := make(chan time.Time, 1)
	f := newStreamFixtureOptions(t, true, time.Now, 100*time.Millisecond, ticks)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "timed-out-check")
	ctx, cancel := context.WithTimeout(signed, 10*time.Second)
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	_, err = stream.Recv()
	require.NoError(t, err)
	blocker, err := f.pool.Begin(f.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(f.ctx, `LOCK TABLE user_custody_state IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	ticks <- time.Now()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.user_custody_state'::regclass AND mode='AccessShareLock' AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	_, err = stream.Recv()
	require.Error(t, err)
	require.NoError(t, blocker.Rollback(f.ctx))
	bounded, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	require.Error(t, f.control.CloseAdmission(bounded))
	require.Error(t, f.control.Shutdown(bounded))
}

func TestCustodyStreamRechecksExpiryAfterBlockedHeartbeatCheck(t *testing.T) {
	var offset atomic.Int64
	clock := func() time.Time { return time.Now().Add(time.Duration(offset.Load())) }
	ticks := make(chan time.Time, 1)
	f := newStreamFixtureOptions(t, false, clock, 100*time.Millisecond, ticks)
	signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "blocked-expiry")
	ctx, cancel := context.WithTimeout(signed, 10*time.Second)
	defer cancel()
	stream, err := f.client.AcquireUserSourcePermit(ctx, f.request)
	require.NoError(t, err)
	first, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, userv1.UserSourcePermitFrameKind_USER_SOURCE_PERMIT_FRAME_KIND_GRANTED, first.Kind)
	blocker, err := f.pool.Begin(f.ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = blocker.Rollback(context.Background()) })
	_, err = blocker.Exec(f.ctx, `LOCK TABLE user_custody_state IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	ticks <- time.Now()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.pool.QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.user_custody_state'::regclass AND mode='AccessShareLock' AND NOT granted)`).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 20*time.Millisecond)
	offset.Store(int64(31 * time.Second))
	require.NoError(t, blocker.Commit(f.ctx))
	// The controlled ticker ensures no heartbeat could precede the blocked check.
	_, err = stream.Recv()
	require.Error(t, err, "expired credential must be rechecked after control SQL")
	require.NoError(t, ctx.Err())
	drain, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	require.NoError(t, f.control.CloseAdmission(drain))
	require.NoError(t, f.control.WaitPermits(drain))
}

func TestCustodyStreamRejectsUntrustedTLS(t *testing.T) {
	f := newStreamFixture(t)
	_, wrongClient, wrongRoots := streamTransportTLS(t)
	for _, tc := range []struct {
		name   string
		config *tls.Config
	}{
		{"missing client", &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.roots, ServerName: "localhost"}},
		{"wrong client CA", &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.roots, ServerName: "localhost", Certificates: []tls.Certificate{wrongClient}}},
		{"wrong server CA", &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: wrongRoots, ServerName: "localhost", Certificates: []tls.Certificate{f.clientCert}}},
		{"wrong server name", &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: f.roots, ServerName: "wrong.invalid", Certificates: []tls.Certificate{f.clientCert}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := grpc.NewClient(f.address, grpc.WithTransportCredentials(credentials.NewTLS(tc.config)))
			require.NoError(t, err)
			defer func() { _ = conn.Close() }()
			signed := f.signed(t, f.request, "user-source-custody", userv1.UserSourceCustodyService_AcquireUserSourcePermit_FullMethodName, "tls-"+tc.name)
			ctx, cancel := context.WithTimeout(signed, time.Second)
			defer cancel()
			stream, err := userv1.NewUserSourceCustodyServiceClient(conn).AcquireUserSourcePermit(ctx, f.request)
			if err == nil {
				_, err = stream.Recv()
			}
			require.Equal(t, codes.Unavailable, status.Code(err), "TLS must fail before the RPC handler")
		})
	}
}
