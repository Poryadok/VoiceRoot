package grpcsvc

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/role/permissions"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"

	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

// transferRoleStub wires Role Service for TransferOwnership fail-closed tests.
type transferRoleStub struct {
	rolev1.RoleServiceClient

	mu                          sync.Mutex
	ownerRoleID                 string
	memberRoleID                string
	assignErr                   error
	assignCommittedErrByProfile map[string]error
	listErr                     error
	revokeErrByProfile          map[string]error
	// revokeCommittedErrByProfile simulates a Role transport result that is
	// returned after the revoke mutation has committed remotely.
	revokeCommittedErrByProfile map[string]error
	cancelOnRevoke              map[string]context.CancelFunc
	allowCanceled               map[string]bool
	owners                      map[string]bool
	enforceOwnerActor           bool

	ownerAssignCalls    int
	ownerRevokeCalls    int
	ownerRevokeProfiles []string
	ownerRoleCalls      []ownerRoleCall
}

type ownerRoleCall struct {
	operation      string
	spaceID        string
	profileID      string
	roleID         string
	actorProfileID string
	contextActive  bool
	contextBounded bool
}

type ownerRoleRequest struct {
	operation string
	spaceID   string
	profileID string
	roleID    string
}

type contextObservation struct {
	active  bool
	bounded bool
}

func outgoingActorProfileID(ctx context.Context) string {
	md, _ := metadata.FromOutgoingContext(ctx)
	values := md.Get(authctx.HeaderProfileID)
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func newTransferRoleStub() *transferRoleStub {
	return &transferRoleStub{
		ownerRoleID:                 "role-owner",
		memberRoleID:                "role-member",
		assignCommittedErrByProfile: make(map[string]error),
		revokeErrByProfile:          make(map[string]error),
		revokeCommittedErrByProfile: make(map[string]error),
		cancelOnRevoke:              make(map[string]context.CancelFunc),
		allowCanceled:               make(map[string]bool),
		owners:                      make(map[string]bool),
	}
}

type blockingListRolesStub struct {
	*transferRoleStub

	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingListRolesStub() *blockingListRolesStub {
	return &blockingListRolesStub{
		transferRoleStub: newTransferRoleStub(),
		started:          make(chan struct{}),
		release:          make(chan struct{}),
	}
}

func (s *blockingListRolesStub) ListRoles(ctx context.Context, req *rolev1.ListRolesRequest, opts ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	s.startedOnce.Do(func() { close(s.started) })
	select {
	case <-s.release:
		return s.transferRoleStub.ListRoles(ctx, req, opts...)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *blockingListRolesStub) unblock() {
	s.releaseOnce.Do(func() { close(s.release) })
}

func (s *transferRoleStub) BootstrapSpaceRoles(context.Context, *rolev1.BootstrapSpaceRolesRequest, ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (s *transferRoleStub) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func (s *transferRoleStub) ListRoles(context.Context, *rolev1.ListRolesRequest, ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{
		{Id: s.ownerRoleID, Name: permissions.RoleOwner, Position: 4},
		{Id: s.memberRoleID, Name: permissions.RoleMember, Position: 1},
	}}}, nil
}

func (s *transferRoleStub) GetDefaultJoinRole(context.Context, *rolev1.GetDefaultJoinRoleRequest, ...grpc.CallOption) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return &rolev1.GetDefaultJoinRoleResponse{Role: &rolev1.Role{Id: s.memberRoleID, Name: permissions.RoleMember}}, nil
}

func (s *transferRoleStub) AssignRole(ctx context.Context, req *rolev1.AssignRoleRequest, _ ...grpc.CallOption) (*rolev1.AssignRoleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.GetRoleId() == s.ownerRoleID {
		_, bounded := ctx.Deadline()
		s.ownerAssignCalls++
		s.ownerRoleCalls = append(s.ownerRoleCalls, ownerRoleCall{
			operation:      "assign",
			spaceID:        req.GetSpaceId(),
			profileID:      req.GetProfileId(),
			roleID:         req.GetRoleId(),
			actorProfileID: outgoingActorProfileID(ctx),
			contextActive:  ctx.Err() == nil,
			contextBounded: bounded,
		})
		if err := ctx.Err(); err != nil && !s.allowCanceled[req.GetProfileId()] {
			return nil, err
		}
		if s.enforceOwnerActor && !s.owners[outgoingActorProfileID(ctx)] {
			return nil, status.Error(codes.PermissionDenied, "owner role required")
		}
		if s.assignErr != nil {
			return nil, s.assignErr
		}
		s.owners[req.GetProfileId()] = true
		if err := s.assignCommittedErrByProfile[req.GetProfileId()]; err != nil {
			return nil, err
		}
	}
	return &rolev1.AssignRoleResponse{}, nil
}

func (s *transferRoleStub) RevokeRole(ctx context.Context, req *rolev1.RevokeRoleRequest, _ ...grpc.CallOption) (*rolev1.RevokeRoleResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.GetRoleId() == s.ownerRoleID {
		_, bounded := ctx.Deadline()
		s.ownerRevokeCalls++
		s.ownerRevokeProfiles = append(s.ownerRevokeProfiles, req.GetProfileId())
		s.ownerRoleCalls = append(s.ownerRoleCalls, ownerRoleCall{
			operation:      "revoke",
			spaceID:        req.GetSpaceId(),
			profileID:      req.GetProfileId(),
			roleID:         req.GetRoleId(),
			actorProfileID: outgoingActorProfileID(ctx),
			contextActive:  ctx.Err() == nil,
			contextBounded: bounded,
		})
		if err := ctx.Err(); err != nil && !s.allowCanceled[req.GetProfileId()] {
			return nil, err
		}
		if s.enforceOwnerActor && !s.owners[outgoingActorProfileID(ctx)] {
			return nil, status.Error(codes.PermissionDenied, "owner role required")
		}
		if cancel := s.cancelOnRevoke[req.GetProfileId()]; cancel != nil {
			cancel()
		}
		if err := s.revokeErrByProfile[req.GetProfileId()]; err != nil {
			return nil, err
		}
		delete(s.owners, req.GetProfileId())
		if err := s.revokeCommittedErrByProfile[req.GetProfileId()]; err != nil {
			return nil, err
		}
	}
	return &rolev1.RevokeRoleResponse{}, nil
}

func (s *transferRoleStub) setOwner(profileID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.owners[profileID.String()] = true
}

func (s *transferRoleStub) ownerRoleRequests() []ownerRoleRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ownerRoleRequest, 0, len(s.ownerRoleCalls))
	for _, call := range s.ownerRoleCalls {
		out = append(out, ownerRoleRequest{
			operation: call.operation,
			spaceID:   call.spaceID,
			profileID: call.profileID,
			roleID:    call.roleID,
		})
	}
	return out
}

func (s *transferRoleStub) ownerRoleCall(index int) ownerRoleCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ownerRoleCalls[index]
}

func (s *transferRoleStub) ownerProfiles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.owners))
	for profileID := range s.owners {
		out = append(out, profileID)
	}
	return out
}

type ownershipUpdateContextTracer struct {
	mu                           sync.Mutex
	updates                      []contextObservation
	commits                      []contextObservation
	auditDeletes                 []contextObservation
	auditInsertStarted           chan struct{}
	blockAuditInsert             chan struct{}
	cancelOnAuditInsert          context.CancelFunc
	auditInsertCommitted         chan struct{}
	cancelAfterAuditInsertCommit context.CancelFunc
	cancelAfterCommit            context.CancelFunc
	auditInsertStartedOnce       sync.Once
	auditCancelOnce              sync.Once
	auditCommitCancelOnce        sync.Once
	commitCancelOnce             sync.Once
}

type auditInsertTraceContextKey struct{}
type commitTraceContextKey struct{}

func (t *ownershipUpdateContextTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "UPDATE spaces SET owner_profile_id") {
		_, bounded := ctx.Deadline()
		t.mu.Lock()
		t.updates = append(t.updates, contextObservation{active: ctx.Err() == nil, bounded: bounded})
		t.mu.Unlock()
	}
	if strings.EqualFold(strings.TrimSpace(data.SQL), "commit") {
		_, bounded := ctx.Deadline()
		t.mu.Lock()
		t.commits = append(t.commits, contextObservation{active: ctx.Err() == nil, bounded: bounded})
		t.mu.Unlock()
		if t.cancelAfterCommit != nil {
			return context.WithValue(ctx, commitTraceContextKey{}, true)
		}
	}
	if strings.Contains(data.SQL, "INSERT INTO audit_log") {
		if t.auditInsertStarted != nil {
			t.auditInsertStartedOnce.Do(func() { close(t.auditInsertStarted) })
		}
		if t.blockAuditInsert != nil {
			<-t.blockAuditInsert
		}
		if t.cancelOnAuditInsert != nil {
			t.auditCancelOnce.Do(func() { t.cancelOnAuditInsert() })
		}
	}
	if strings.Contains(data.SQL, "INSERT INTO audit_log") && t.cancelAfterAuditInsertCommit != nil {
		return context.WithValue(ctx, auditInsertTraceContextKey{}, true)
	}
	if strings.Contains(data.SQL, "DELETE FROM audit_log") {
		_, bounded := ctx.Deadline()
		t.mu.Lock()
		t.auditDeletes = append(t.auditDeletes, contextObservation{active: ctx.Err() == nil, bounded: bounded})
		t.mu.Unlock()
	}
	return ctx
}

func (t *ownershipUpdateContextTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if ctx.Value(commitTraceContextKey{}) == true && t.cancelAfterCommit != nil {
		t.commitCancelOnce.Do(t.cancelAfterCommit)
	}
	if ctx.Value(auditInsertTraceContextKey{}) == true && t.cancelAfterAuditInsertCommit != nil {
		t.auditCommitCancelOnce.Do(func() {
			if t.auditInsertCommitted != nil {
				close(t.auditInsertCommitted)
			}
			t.cancelAfterAuditInsertCommit()
		})
	}
}

func (t *ownershipUpdateContextTracer) snapshotCommits() []contextObservation {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]contextObservation(nil), t.commits...)
}

func (t *ownershipUpdateContextTracer) snapshot() []contextObservation {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]contextObservation(nil), t.updates...)
}

func (t *ownershipUpdateContextTracer) snapshotAuditDeletes() []contextObservation {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]contextObservation(nil), t.auditDeletes...)
}

func tracedSpacePool(t *testing.T, pool *pgxpool.Pool, tracer pgx.QueryTracer) *pgxpool.Pool {
	t.Helper()
	config := pool.Config()
	config.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	t.Cleanup(traced.Close)
	require.NoError(t, traced.Ping(context.Background()))
	return traced
}

// commitAmbiguityProxy is a test-only PostgreSQL protocol proxy. Once armed,
// it forwards one COMMIT to PostgreSQL, waits until PostgreSQL returns the
// successful CommandComplete, and then drops that response and the client
// connection. The transaction is therefore durable while pgx receives a
// transport error from Commit.
type commitAmbiguityProxy struct {
	listener net.Listener
	upstream string

	mu        sync.Mutex
	armed     bool
	claimed   bool
	shutdown  chan struct{}
	drop      chan struct{}
	persisted chan struct{}

	shutdownOnce sync.Once
	dropOnce     sync.Once
}

func startCommitAmbiguityProxy(t *testing.T, upstreamHost string, upstreamPort uint16) *commitAmbiguityProxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &commitAmbiguityProxy{
		listener:  listener,
		upstream:  net.JoinHostPort(upstreamHost, fmt.Sprintf("%d", upstreamPort)),
		shutdown:  make(chan struct{}),
		drop:      make(chan struct{}),
		persisted: make(chan struct{}),
	}
	go p.serve()
	t.Cleanup(p.abort)
	return p
}

func (p *commitAmbiguityProxy) port() uint16 {
	return uint16(p.listener.Addr().(*net.TCPAddr).Port)
}

func (p *commitAmbiguityProxy) arm() {
	p.mu.Lock()
	p.armed = true
	p.mu.Unlock()
}

func (p *commitAmbiguityProxy) claimCommit() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.armed || p.claimed {
		return false
	}
	p.claimed = true
	return true
}

func (p *commitAmbiguityProxy) dropCommitResponse() {
	p.dropOnce.Do(func() { close(p.drop) })
}

func (p *commitAmbiguityProxy) abort() {
	p.dropCommitResponse()
	p.shutdownOnce.Do(func() { close(p.shutdown) })
	_ = p.listener.Close()
}

func (p *commitAmbiguityProxy) serve() {
	for {
		client, err := p.listener.Accept()
		if err != nil {
			return
		}
		go p.serveConn(client)
	}
}

func (p *commitAmbiguityProxy) serveConn(client net.Conn) {
	upstream, err := net.Dial("tcp", p.upstream)
	if err != nil {
		_ = client.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()

	var interceptCommit atomic.Bool
	done := make(chan struct{}, 2)
	go func() {
		_ = proxyPostgresFrontend(client, upstream, p, &interceptCommit)
		done <- struct{}{}
	}()
	go func() {
		_ = proxyPostgresBackend(upstream, client, p, &interceptCommit)
		done <- struct{}{}
	}()
	<-done
}

func proxyPostgresFrontend(client, upstream net.Conn, p *commitAmbiguityProxy, interceptCommit *atomic.Bool) error {
	// TLS is disabled on the proxied pgx pool, so the first untyped packet is
	// always the startup message. All later frontend packets are typed.
	startupLength, startupPayload, err := readPostgresStartupPacket(client)
	if err != nil {
		return err
	}
	if err := writePostgresStartupPacket(upstream, startupLength, startupPayload); err != nil {
		return err
	}
	for {
		messageType, payload, err := readPostgresTypedPacket(client)
		if err != nil {
			return err
		}
		if messageType == 'Q' && strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(string(payload), "\x00")), "commit") && p.claimCommit() {
			interceptCommit.Store(true)
		}
		if err := writePostgresTypedPacket(upstream, messageType, payload); err != nil {
			return err
		}
	}
}

func proxyPostgresBackend(upstream, client net.Conn, p *commitAmbiguityProxy, interceptCommit *atomic.Bool) error {
	for {
		messageType, payload, err := readPostgresTypedPacket(upstream)
		if err != nil {
			return err
		}
		if interceptCommit.Load() && messageType == 'C' && strings.EqualFold(strings.TrimSpace(strings.TrimSuffix(string(payload), "\x00")), "commit") {
			close(p.persisted)
			select {
			case <-p.drop:
			case <-p.shutdown:
			}
			return io.ErrUnexpectedEOF
		}
		if err := writePostgresTypedPacket(client, messageType, payload); err != nil {
			return err
		}
	}
}

func readPostgresStartupPacket(conn net.Conn) (uint32, []byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[:])
	if length < 4 || length > 16<<20 {
		return 0, nil, fmt.Errorf("invalid PostgreSQL startup packet length %d", length)
	}
	payload := make([]byte, length-4)
	_, err := io.ReadFull(conn, payload)
	return length, payload, err
}

func writePostgresStartupPacket(conn net.Conn, length uint32, payload []byte) error {
	packet := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(packet[:4], length)
	copy(packet[4:], payload)
	return writeAll(conn, packet)
}

func readPostgresTypedPacket(conn net.Conn) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	length := binary.BigEndian.Uint32(header[1:])
	if length < 4 || length > 64<<20 {
		return 0, nil, fmt.Errorf("invalid PostgreSQL packet length %d", length)
	}
	payload := make([]byte, length-4)
	_, err := io.ReadFull(conn, payload)
	return header[0], payload, err
}

func writePostgresTypedPacket(conn net.Conn, messageType byte, payload []byte) error {
	packet := make([]byte, 5+len(payload))
	packet[0] = messageType
	binary.BigEndian.PutUint32(packet[1:5], uint32(len(payload)+4))
	copy(packet[5:], payload)
	return writeAll(conn, packet)
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func commitAmbiguitySpacePool(t *testing.T, source *pgxpool.Pool, proxy *commitAmbiguityProxy) *pgxpool.Pool {
	t.Helper()
	config := source.Config()
	config.MaxConns = 1
	config.MinConns = 0
	config.ConnConfig.Host = "127.0.0.1"
	config.ConnConfig.Port = proxy.port()
	config.ConnConfig.TLSConfig = nil
	config.ConnConfig.Fallbacks = nil
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	cleanupSpaceLockPool(t, pool)
	// Registered after pool cleanup so LIFO test cleanup unblocks a withheld
	// COMMIT response before waiting for pool.Close.
	t.Cleanup(proxy.abort)
	require.NoError(t, pool.Ping(context.Background()))
	return pool
}

func requireTransferResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(3 * time.Second):
		t.Fatal("ownership transfer did not finish")
		return nil
	}
}

func directServerContext(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)
	return metadata.NewIncomingContext(ctx, md)
}

func (s *transferRoleStub) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{}, nil
}

func setupTransferWithRoles(t *testing.T, stub *transferRoleStub, extraOpts ...spaceServerOption) (
	client spacev1.SpaceServiceClient,
	owner uuid.UUID,
	memberProfile uuid.UUID,
	ownerCtx context.Context,
	spaceID string,
	pool *pgxpool.Pool,
) {
	t.Helper()
	owner, _, ownerCtx = profileFixture(t)
	stub.setOwner(owner)
	memberAccount, memberProfile := uuid.New(), uuid.New()
	memberCtx := withAccountProfileCtx(context.Background(), memberAccount, memberProfile)

	pool = startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	opts := append([]spaceServerOption{withRoleClient(stub)}, extraOpts...)
	client, cleanup := startSpaceGRPCTestServer(t, pool, opts...)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Role transfer"})
	require.NoError(t, err)
	spaceID = created.GetSpace().GetId()

	inv, err := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
	require.NoError(t, err)
	_, err = client.JoinByInvite(memberCtx, &spacev1.JoinByInviteRequest{Code: inv.GetInvite().GetCode()})
	require.NoError(t, err)

	return client, owner, memberProfile, ownerCtx, spaceID, pool
}

func requireOwnershipTransferAuditCount(t *testing.T, pool *pgxpool.Pool, spaceID string, want int) {
	t.Helper()
	var got int
	err := pool.QueryRow(context.Background(), `
SELECT count(*) FROM audit_log WHERE space_id = $1 AND action = 'ownership_transferred'
`, spaceID).Scan(&got)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestTransferOwnership_RoleAssignFails_RollsBackOwnership documents fail-closed when Assign Owner fails.
func TestTransferOwnership_RoleAssignFails_RollsBackOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.assignErr = status.Error(codes.Unavailable, "role assign down")
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 2, stub.ownerAssignCalls)
	require.Equal(t, 1, stub.ownerRevokeCalls)
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "failed transfer must not publish space.updated")
}

// TestTransferOwnership_RoleRevokeFails_CompensatesAndRollsBack documents role and DB compensation.
func TestTransferOwnership_RoleRevokeFails_CompensatesAndRollsBack(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.enforceOwnerActor = true
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	stub.revokeErrByProfile[owner.String()] = status.Error(codes.Internal, "previous owner revoke failed")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 2, stub.ownerAssignCalls)
	require.Equal(t, 2, stub.ownerRevokeCalls)
	require.Equal(t, []string{owner.String(), memberProfile.String()}, stub.ownerRevokeProfiles)
	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	require.ElementsMatch(t, []string{owner.String()}, stub.ownerProfiles(), "failed transition must leave the previous owner as the sole Owner")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "failed transfer must not publish space.updated")
}

func TestTransferOwnership_RoleRevokeAmbiguousAfterCommit_RestoresExactOwnerRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.enforceOwnerActor = true
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	stub.revokeCommittedErrByProfile[owner.String()] = status.Error(codes.DeadlineExceeded, "previous owner revoke committed before deadline")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))

	got, getErr := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, getErr)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	require.ElementsMatch(t, []string{owner.String()}, stub.ownerProfiles(), "ambiguous revoke must restore the previous owner as the sole Owner")
	for _, index := range []int{2, 3} {
		call := stub.ownerRoleCall(index)
		require.True(t, call.contextActive)
		require.True(t, call.contextBounded)
		require.Equal(t, memberProfile.String(), call.actorProfileID, "new Owner must authorize ambiguous-revoke compensation")
	}
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated())
}

func TestTransferOwnership_RoleAssignAmbiguousAfterCommit_RestoresExactOwnerRoles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	stub.enforceOwnerActor = true
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	stub.assignCommittedErrByProfile[memberProfile.String()] = status.Error(codes.DeadlineExceeded, "new owner assign committed before deadline")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))

	got, getErr := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, getErr)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	for _, index := range []int{1, 2} {
		call := stub.ownerRoleCall(index)
		require.True(t, call.contextActive)
		require.True(t, call.contextBounded)
		require.Equal(t, owner.String(), call.actorProfileID, "previous Owner must authorize ambiguous-assign compensation")
	}
	require.ElementsMatch(t, []string{owner.String()}, stub.ownerProfiles(), "ambiguous assign must restore the previous owner as the sole Owner")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated())
}

// TestTransferOwnership_CompensationRevokeFails_ReportsBothAndRollsBackDB documents dual-error reporting.
func TestTransferOwnership_CompensationRevokeFails_ReportsBothAndRollsBackDB(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	stub.revokeErrByProfile[owner.String()] = status.Error(codes.Internal, "previous owner revoke failed")
	stub.revokeErrByProfile[memberProfile.String()] = status.Error(codes.Unavailable, "compensation revoke failed")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "previous owner revoke failed")
	require.ErrorContains(t, err, "compensation revoke failed")

	got, getErr := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, getErr)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 2, stub.ownerAssignCalls)
	require.Equal(t, []string{owner.String(), memberProfile.String()}, stub.ownerRevokeProfiles)
	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "failed transfer must not publish space.updated")
}

// TestTransferOwnership_WithRoles_ReassignsOwnerRole documents Assign+Revoke Owner on success.
func TestTransferOwnership_WithRoles_ReassignsOwnerRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub)

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.NoError(t, err)

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, memberProfile.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 1, stub.ownerAssignCalls)
	require.Equal(t, 1, stub.ownerRevokeCalls)
	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	require.ElementsMatch(t, []string{memberProfile.String()}, stub.ownerProfiles(), "successful transition must leave the new owner as the sole Owner")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 1)
}

// TestTransferOwnership_RoleListFails_RollsBackOwnership documents fail-closed when ListRoles fails.
func TestTransferOwnership_RoleListFails_RollsBackOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	spy := &spySpaceEvents{}
	client, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	stub.listErr = status.Error(codes.Unavailable, "role list down")

	_, err := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Unavailable, status.Code(err))

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, owner.String(), got.GetSpace().GetOwnerProfileId())
	require.Equal(t, 0, stub.ownerAssignCalls)
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "failed transfer must not publish space.updated")
}

func TestTransferOwnership_RequestCanceledByRoleFailure_UsesBoundedCleanupContext(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	spy := &spySpaceEvents{}
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	defer cancel()
	stub.revokeErrByProfile[owner.String()] = status.Error(codes.Internal, "previous owner revoke failed")
	stub.cancelOnRevoke[owner.String()] = cancel

	dbContexts := &ownershipUpdateContextTracer{}
	tracedPool := tracedSpacePool(t, pool, dbContexts)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}, Roles: stub, SpaceEvents: spy}
	_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "previous owner revoke failed", "cleanup must not replace the request-facing dependency error")
	require.Error(t, requestCtx.Err())

	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: stub.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: stub.ownerRoleID},
	}, stub.ownerRoleRequests())
	for _, index := range []int{2, 3} {
		roleCleanup := stub.ownerRoleCall(index)
		require.True(t, roleCleanup.contextActive, "role compensation must detach from canceled request context")
		require.True(t, roleCleanup.contextBounded, "role compensation must have a cleanup deadline")
	}
	require.ElementsMatch(t, []string{owner.String()}, stub.ownerProfiles(), "role compensation must restore the sole Owner")
	got, getErr := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID, "DB rollback must complete after request cancellation")
	updates := dbContexts.snapshot()
	require.Len(t, updates, 2, "DB rollback must execute after request cancellation")
	require.True(t, updates[1].active, "DB rollback must detach from canceled request context")
	require.True(t, updates[1].bounded, "DB rollback must have a cleanup deadline")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "failed transfer must not publish space.updated")
}

func TestTransferOwnership_RequestCanceled_DBRollbackUsesBoundedCleanupContext(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub)
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	defer cancel()
	stub.revokeErrByProfile[owner.String()] = status.Error(codes.Internal, "previous owner revoke failed")
	stub.cancelOnRevoke[owner.String()] = cancel
	// Isolate DB rollback observability: the Role compensation is permitted to
	// finish even when current production incorrectly reuses the canceled request.
	stub.allowCanceled[memberProfile.String()] = true

	dbContexts := &ownershipUpdateContextTracer{}
	tracedPool := tracedSpacePool(t, pool, dbContexts)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}, Roles: stub}
	_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "previous owner revoke failed")
	require.Error(t, requestCtx.Err())

	updates := dbContexts.snapshot()
	require.Len(t, updates, 2, "DB rollback must execute after request cancellation")
	require.True(t, updates[1].active, "DB rollback must detach from canceled request context")
	require.True(t, updates[1].bounded, "DB rollback must have a cleanup deadline")
	got, getErr := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID)
}

func TestTransferOwnership_RequestCanceledAfterDBCommit_UsesDetachedCommitAndCompensates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	stub := newTransferRoleStub()
	spy := &spySpaceEvents{}
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, stub, withSpaceEventsPublisher(spy))
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	defer cancel()
	tracer := &ownershipUpdateContextTracer{cancelAfterCommit: cancel}
	tracedPool := tracedSpacePool(t, pool, tracer)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}, Roles: stub, SpaceEvents: spy}

	_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.Error(t, requestCtx.Err(), "request must be canceled only after the database commit completed")

	commits := tracer.snapshotCommits()
	require.Len(t, commits, 2, "initial commit and compensation commit must both run")
	for _, observation := range commits {
		require.True(t, observation.active, "commit must run on a context active despite request cancellation")
		require.True(t, observation.bounded, "commit must use the bounded detached cleanup policy")
	}
	require.ElementsMatch(t, []string{owner.String()}, stub.ownerProfiles())
	got, getErr := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID)
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated())
}

func TestTransferOwnership_AmbiguousDBCommit_ReconcilesBeforeReleasingMutationLease(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	basePool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, basePool)
	owner := uuid.New()
	newOwner := uuid.New()
	spaceID := createSpaceWithMembers(t, basePool, owner, newOwner)
	roles := newBlockingListRolesStub()
	roles.setOwner(owner)
	t.Cleanup(roles.unblock)

	proxy := startCommitAmbiguityProxy(t, basePool.Config().ConnConfig.Host, basePool.Config().ConnConfig.Port)
	storePool := commitAmbiguitySpacePool(t, basePool, proxy)
	serviceLockPool := independentSpacePool(t, basePool, 1)
	observerLockPool := independentSpacePool(t, basePool, 1)
	svc := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: storePool},
		Roles:          roles,
		MutationLocker: store.NewSpaceMutationLocker(serviceLockPool),
	}
	requestCtx := directServerContext(withAccountProfileCtx(ctx, uuid.New(), owner))

	proxy.arm()
	transferDone := make(chan error, 1)
	go func() {
		_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
			SpaceId:           spaceID.String(),
			NewOwnerProfileId: newOwner.String(),
		})
		transferDone <- err
	}()

	select {
	case <-proxy.persisted:
	case <-time.After(3 * time.Second):
		t.Fatal("ownership COMMIT did not persist through the ambiguity proxy")
	}
	committedRow, err := (&store.SpaceStore{Pool: basePool}).GetSpace(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, newOwner, committedRow.OwnerProfileID, "the server-side COMMIT must be durable before its response is dropped")
	require.ElementsMatch(t, []string{owner.String()}, roles.ownerProfiles(), "Role transition must still be pending while the COMMIT response is held")
	requireOwnershipTransferAuditCount(t, basePool, spaceID.String(), 0)

	type leaseObservation struct {
		owner      uuid.UUID
		roleOwners []string
		auditCount int
		err        error
	}
	observedAfterRelease := make(chan leaseObservation, 1)
	observerCtx, cancelObserver := context.WithTimeout(ctx, 5*time.Second)
	defer cancelObserver()
	go func() {
		release, acquireErr := store.NewSpaceMutationLocker(observerLockPool).Acquire(observerCtx, spaceID)
		if acquireErr != nil {
			observedAfterRelease <- leaseObservation{err: acquireErr}
			return
		}
		defer release()
		row, getErr := (&store.SpaceStore{Pool: basePool}).GetSpace(observerCtx, spaceID)
		if getErr != nil {
			observedAfterRelease <- leaseObservation{err: getErr}
			return
		}
		var auditCount int
		queryErr := basePool.QueryRow(observerCtx, `
SELECT count(*) FROM audit_log WHERE space_id = $1 AND action = 'ownership_transferred'
`, spaceID).Scan(&auditCount)
		observedAfterRelease <- leaseObservation{
			owner:      row.OwnerProfileID,
			roleOwners: roles.ownerProfiles(),
			auditCount: auditCount,
			err:        queryErr,
		}
	}()

	select {
	case observation := <-observedAfterRelease:
		t.Fatalf("observer acquired the advisory lease before the ambiguous COMMIT result was delivered: %+v", observation)
	case <-time.After(150 * time.Millisecond):
	}

	proxy.dropCommitResponse()
	select {
	case <-roles.started:
		// The handler classified the ambiguous COMMIT and entered Role
		// reconciliation while it still owns the mutation lease.
	case transferErr := <-transferDone:
		t.Fatalf("TransferOwnership returned and released its mutation lease before reconciling the durable ambiguous COMMIT: %v", transferErr)
	case <-time.After(3 * time.Second):
		t.Fatal("TransferOwnership neither entered Role reconciliation nor returned")
	}
	select {
	case observation := <-observedAfterRelease:
		t.Fatalf("observer acquired the advisory lease while ambiguous-COMMIT Role reconciliation was blocked: %+v", observation)
	case <-time.After(150 * time.Millisecond):
	}

	roles.unblock()
	transferErr := requireTransferResult(t, transferDone)
	var observation leaseObservation
	select {
	case observation = <-observedAfterRelease:
	case <-time.After(3 * time.Second):
		t.Fatal("observer did not acquire the advisory lease after TransferOwnership returned")
	}

	require.NoError(t, observation.err)
	require.Equal(t, newOwner, observation.owner)
	require.ElementsMatch(t, []string{newOwner.String()}, observation.roleOwners,
		"the mutation lease must not be released with a committed DB owner but the previous Owner role")
	require.Equal(t, 1, observation.auditCount,
		"the mutation lease must not be released before the committed transfer has its audit row")
	require.NoError(t, transferErr, "a confirmed committed owner update must continue Role and audit reconciliation")
}

func TestTransferOwnership_AuditWriteCancellation_CompensatesWithoutFalseAuditOrEvent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roles := newTransferRoleStub()
	spy := &spySpaceEvents{}
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, roles, withSpaceEventsPublisher(spy))
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	defer cancel()
	dbContexts := &ownershipUpdateContextTracer{
		auditInsertStarted:  make(chan struct{}),
		cancelOnAuditInsert: cancel,
	}
	tracedPool := tracedSpacePool(t, pool, dbContexts)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}, Roles: roles, SpaceEvents: spy}
	_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "record ownership transfer audit failed")
	require.Error(t, requestCtx.Err())
	select {
	case <-dbContexts.auditInsertStarted:
	default:
		t.Fatal("audit INSERT did not start before request cancellation")
	}

	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: roles.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: roles.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: roles.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: roles.ownerRoleID},
	}, roles.ownerRoleRequests(), "audit failure must perform the exact inverse Owner-role transition")
	for _, index := range []int{2, 3} {
		call := roles.ownerRoleCall(index)
		require.True(t, call.contextActive, "%s Owner rollback must detach from the canceled request context", call.operation)
		require.True(t, call.contextBounded, "%s Owner rollback must have a cleanup deadline", call.operation)
	}
	require.ElementsMatch(t, []string{owner.String()}, roles.ownerProfiles(), "audit failure must restore the previous owner as the sole Owner")
	got, getErr := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID, "DB rollback must complete after audit-write cancellation")
	updates := dbContexts.snapshot()
	require.Len(t, updates, 2, "DB rollback must execute after audit-write cancellation")
	require.True(t, updates[1].active, "DB rollback must detach from the canceled request context")
	require.True(t, updates[1].bounded, "DB rollback must have a cleanup deadline")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated(), "audit failure must not publish space.updated")
}

func TestTransferOwnership_AuditWriteCancellationAfterCommit_DeletesAmbiguousAuditAndCompensates(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roles := newTransferRoleStub()
	spy := &spySpaceEvents{}
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, roles, withSpaceEventsPublisher(spy))
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	defer cancel()
	dbContexts := &ownershipUpdateContextTracer{
		auditInsertCommitted:         make(chan struct{}),
		cancelAfterAuditInsertCommit: cancel,
	}
	tracedPool := tracedSpacePool(t, pool, dbContexts)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}, Roles: roles, SpaceEvents: spy}
	_, err := svc.TransferOwnership(requestCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID,
		NewOwnerProfileId: memberProfile.String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
	require.ErrorContains(t, err, "record ownership transfer audit failed")
	require.Error(t, requestCtx.Err())
	select {
	case <-dbContexts.auditInsertCommitted:
	default:
		t.Fatal("audit INSERT did not commit before request cancellation")
	}

	require.Equal(t, []ownerRoleRequest{
		{operation: "assign", spaceID: spaceID, profileID: memberProfile.String(), roleID: roles.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: owner.String(), roleID: roles.ownerRoleID},
		{operation: "assign", spaceID: spaceID, profileID: owner.String(), roleID: roles.ownerRoleID},
		{operation: "revoke", spaceID: spaceID, profileID: memberProfile.String(), roleID: roles.ownerRoleID},
	}, roles.ownerRoleRequests())
	require.ElementsMatch(t, []string{owner.String()}, roles.ownerProfiles())
	got, getErr := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID)
	for _, observation := range dbContexts.snapshotAuditDeletes() {
		require.True(t, observation.active, "ambiguous audit delete must detach from canceled request context")
		require.True(t, observation.bounded, "ambiguous audit delete must have a cleanup deadline")
	}
	require.Len(t, dbContexts.snapshotAuditDeletes(), 1, "ambiguous audit insert must be deleted by its exact generated ID")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
	require.Empty(t, spy.snapshotUpdated())
}

func TestTransferOwnership_AuditFailure_BlocksConcurrentSpaceMutation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	setupRoles := newTransferRoleStub()
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, setupRoles)
	transferCtx, cancelTransfer := context.WithCancel(directServerContext(ownerCtx))
	defer cancelTransfer()
	tracer := &ownershipUpdateContextTracer{
		auditInsertStarted:  make(chan struct{}),
		blockAuditInsert:    make(chan struct{}),
		cancelOnAuditInsert: cancelTransfer,
	}
	tracedPool := tracedSpacePool(t, pool, tracer)
	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: tracedPool}}
	transferResult := make(chan error, 1)
	go func() {
		_, err := svc.TransferOwnership(transferCtx, &spacev1.TransferOwnershipRequest{
			SpaceId:           spaceID,
			NewOwnerProfileId: memberProfile.String(),
		})
		transferResult <- err
	}()
	select {
	case <-tracer.auditInsertStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("ownership transfer did not reach its audit leg")
	}

	memberCtx := directServerContext(withAccountProfileCtx(context.Background(), uuid.New(), memberProfile))
	mutationCtx, cancelMutation := context.WithTimeout(memberCtx, 150*time.Millisecond)
	defer cancelMutation()
	description := "must not persist during ownership transfer"
	_, mutationErr := svc.UpdateSpace(mutationCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:     spaceID,
		Description: &description,
	})
	require.Equal(t, codes.DeadlineExceeded, status.Code(mutationErr), "concurrent write must wait for ownership saga completion")

	close(tracer.blockAuditInsert)
	transferErr := requireTransferResult(t, transferResult)
	require.Equal(t, codes.Internal, status.Code(transferErr))

	row, err := svc.Store.GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, err)
	require.Equal(t, owner, row.OwnerProfileID)
	require.NotEqual(t, description, row.Description, "timed-out concurrent write must not persist")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 0)
}

type timeoutListRolesStub struct {
	rolev1.RoleServiceClient

	mu      sync.Mutex
	listCtx contextObservation
}

func (s *timeoutListRolesStub) ListRoles(ctx context.Context, _ *rolev1.ListRolesRequest, _ ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	_, bounded := ctx.Deadline()
	s.mu.Lock()
	s.listCtx = contextObservation{active: ctx.Err() == nil, bounded: bounded}
	s.mu.Unlock()
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *timeoutListRolesStub) snapshot() contextObservation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listCtx
}

func TestTransferOwnership_AuditFailure_RoleCleanupTimeoutDoesNotSuppressDBRollback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	roles := newTransferRoleStub()
	_, owner, memberProfile, ownerCtx, spaceID, pool := setupTransferWithRoles(t, roles)
	spaceUUID := uuid.MustParse(spaceID)
	st := &store.SpaceStore{Pool: pool}
	require.NoError(t, st.TransferOwnership(context.Background(), spaceUUID, owner, memberProfile))

	timeoutRoles := &timeoutListRolesStub{}
	svc := &SpaceGRPC{Store: st, Roles: timeoutRoles}
	requestCtx, cancel := context.WithCancel(directServerContext(ownerCtx))
	cancel()
	roleErr, auditErr, dbErr := svc.rollbackOwnershipAfterAuditFailure(requestCtx, uuid.New(), spaceUUID, owner, memberProfile)
	require.Error(t, roleErr)
	require.NoError(t, auditErr)
	require.NoError(t, dbErr, "DB rollback must get a fresh cleanup context after Role cleanup times out")
	roleObservation := timeoutRoles.snapshot()
	require.True(t, roleObservation.active)
	require.True(t, roleObservation.bounded)
	got, getErr := st.GetSpace(context.Background(), spaceUUID)
	require.NoError(t, getErr)
	require.Equal(t, owner, got.OwnerProfileID)
}

type serializingRoleStub struct {
	rolev1.RoleServiceClient

	mu                sync.Mutex
	ownerRoleID       string
	owners            map[string]bool
	listCalls         int
	firstListEntered  chan struct{}
	secondListEntered chan struct{}
	releaseFirst      chan struct{}
}

func newSerializingRoleStub(owner uuid.UUID) *serializingRoleStub {
	return &serializingRoleStub{
		ownerRoleID:       "role-owner",
		owners:            map[string]bool{owner.String(): true},
		firstListEntered:  make(chan struct{}),
		secondListEntered: make(chan struct{}),
		releaseFirst:      make(chan struct{}),
	}
}

func (s *serializingRoleStub) BootstrapSpaceRoles(context.Context, *rolev1.BootstrapSpaceRolesRequest, ...grpc.CallOption) (*rolev1.BootstrapSpaceRolesResponse, error) {
	return &rolev1.BootstrapSpaceRolesResponse{}, nil
}

func (s *serializingRoleStub) CheckPermission(context.Context, *rolev1.CheckPermissionRequest, ...grpc.CallOption) (*rolev1.CheckPermissionResponse, error) {
	return &rolev1.CheckPermissionResponse{Allowed: true}, nil
}

func (s *serializingRoleStub) GetDefaultJoinRole(context.Context, *rolev1.GetDefaultJoinRoleRequest, ...grpc.CallOption) (*rolev1.GetDefaultJoinRoleResponse, error) {
	return &rolev1.GetDefaultJoinRoleResponse{Role: &rolev1.Role{Id: "role-member", Name: permissions.RoleMember}}, nil
}

func (s *serializingRoleStub) GetMemberRoles(context.Context, *rolev1.GetMemberRolesRequest, ...grpc.CallOption) (*rolev1.GetMemberRolesResponse, error) {
	return &rolev1.GetMemberRolesResponse{}, nil
}

func (s *serializingRoleStub) ListRoles(ctx context.Context, _ *rolev1.ListRolesRequest, _ ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	s.mu.Lock()
	s.listCalls++
	call := s.listCalls
	s.mu.Unlock()
	switch call {
	case 1:
		close(s.firstListEntered)
		select {
		case <-s.releaseFirst:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	case 2:
		close(s.secondListEntered)
	}
	return &rolev1.ListRolesResponse{RoleList: &rolev1.RoleList{Roles: []*rolev1.Role{
		{Id: s.ownerRoleID, Name: permissions.RoleOwner, Position: 4},
	}}}, nil
}

func (s *serializingRoleStub) AssignRole(_ context.Context, req *rolev1.AssignRoleRequest, _ ...grpc.CallOption) (*rolev1.AssignRoleResponse, error) {
	if req.GetRoleId() == s.ownerRoleID {
		s.mu.Lock()
		s.owners[req.GetProfileId()] = true
		s.mu.Unlock()
	}
	return &rolev1.AssignRoleResponse{}, nil
}

func (s *serializingRoleStub) RevokeRole(_ context.Context, req *rolev1.RevokeRoleRequest, _ ...grpc.CallOption) (*rolev1.RevokeRoleResponse, error) {
	if req.GetRoleId() == s.ownerRoleID {
		s.mu.Lock()
		delete(s.owners, req.GetProfileId())
		s.mu.Unlock()
	}
	return &rolev1.RevokeRoleResponse{}, nil
}

func (s *serializingRoleStub) ownerProfiles() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.owners))
	for profileID := range s.owners {
		out = append(out, profileID)
	}
	return out
}

func TestTransferOwnership_ConcurrentSameSpace_DoesNotInterleaveRoleTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, _, ownerCtx := profileFixture(t)
	roles := newSerializingRoleStub(owner)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool, withRoleClient(roles))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Concurrent transfer"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	memberA := uuid.New()
	memberACtx := withAccountProfileCtx(context.Background(), uuid.New(), memberA)
	memberB := uuid.New()
	memberBCtx := withAccountProfileCtx(context.Background(), uuid.New(), memberB)
	for _, memberCtx := range []context.Context{memberACtx, memberBCtx} {
		invite, inviteErr := client.CreateInvite(ownerCtx, &spacev1.CreateInviteRequest{SpaceId: spaceID})
		require.NoError(t, inviteErr)
		_, joinErr := client.JoinByInvite(memberCtx, &spacev1.JoinByInviteRequest{Code: invite.GetInvite().GetCode()})
		require.NoError(t, joinErr)
	}

	firstDone := make(chan error, 1)
	go func() {
		_, transferErr := client.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{SpaceId: spaceID, NewOwnerProfileId: memberA.String()})
		firstDone <- transferErr
	}()
	select {
	case <-roles.firstListEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("first transfer did not reach Role Service")
	}
	defer func() {
		select {
		case <-roles.releaseFirst:
		default:
			close(roles.releaseFirst)
		}
	}()

	secondReady := make(chan struct{})
	startSecond := make(chan struct{})
	secondStarted := make(chan struct{})
	secondDone := make(chan error, 1)
	go func() {
		close(secondReady)
		<-startSecond
		secondCtx, cancel := context.WithTimeout(memberACtx, time.Second)
		defer cancel()
		close(secondStarted)
		_, transferErr := client.TransferOwnership(secondCtx, &spacev1.TransferOwnershipRequest{SpaceId: spaceID, NewOwnerProfileId: memberB.String()})
		secondDone <- transferErr
	}()
	<-secondReady
	close(startSecond)
	<-secondStarted

	interleaved := false
	var preReleaseErr error
	select {
	case <-roles.secondListEntered:
		interleaved = true
		preReleaseErr = requireTransferResult(t, secondDone)
	case preReleaseErr = <-secondDone:
		select {
		case <-roles.secondListEntered:
			interleaved = true
		default:
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second transfer did not produce a barrier observation or deadline result")
	}
	close(roles.releaseFirst)
	require.NoError(t, requireTransferResult(t, firstDone))
	require.False(t, interleaved, "second transfer reached Role Service before the first transition was released")
	require.Equal(t, codes.DeadlineExceeded, status.Code(preReleaseErr), "serialized second request must remain behind the first transition until its deadline")

	_, err = client.TransferOwnership(memberACtx, &spacev1.TransferOwnershipRequest{SpaceId: spaceID, NewOwnerProfileId: memberB.String()})
	require.NoError(t, err, "retry after the first transition is released must succeed")

	row, err := (&store.SpaceStore{Pool: pool}).GetSpace(context.Background(), uuid.MustParse(spaceID))
	require.NoError(t, err)
	require.Equal(t, memberB, row.OwnerProfileID)
	require.ElementsMatch(t, []string{memberB.String()}, roles.ownerProfiles(), "the DB owner must be the sole Owner role holder")
	requireOwnershipTransferAuditCount(t, pool, spaceID, 2)
}

func independentSpacePool(t *testing.T, source *pgxpool.Pool, maxConns int32) *pgxpool.Pool {
	t.Helper()
	config := source.Config()
	config.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	require.NoError(t, err)
	cleanupSpaceLockPool(t, pool)
	require.NoError(t, pool.Ping(context.Background()))
	return pool
}

func cleanupSpaceLockPool(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	t.Cleanup(func() {
		done := make(chan struct{})
		go func() {
			pool.Close()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Log("timed out closing space lock pool; a failed implementation may still hold a leased connection")
		}
	})
}

type queryCountingTracer struct {
	mu    sync.Mutex
	count int
}

func (t *queryCountingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	t.mu.Lock()
	t.count++
	t.mu.Unlock()
	return ctx
}

func (*queryCountingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (t *queryCountingTracer) reset() {
	t.mu.Lock()
	t.count = 0
	t.mu.Unlock()
}

func (t *queryCountingTracer) snapshot() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.count
}

func createSpaceWithMembers(t *testing.T, pool *pgxpool.Pool, owner uuid.UUID, members ...uuid.UUID) uuid.UUID {
	t.Helper()
	st := &store.SpaceStore{Pool: pool}
	created, err := st.CreateSpace(context.Background(), owner, "Cross-instance mutation", "before", "private")
	require.NoError(t, err)
	for _, member := range members {
		_, err = pool.Exec(context.Background(), `
INSERT INTO space_members (space_id, profile_id) VALUES ($1, $2)
`, created.ID, member)
		require.NoError(t, err)
	}
	return created.ID
}

func TestTransferOwnership_CrossInstance_DoesNotInterleaveRoleTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	owner := uuid.New()
	memberA := uuid.New()
	memberB := uuid.New()
	spaceID := createSpaceWithMembers(t, pool, owner, memberA, memberB)
	roles := newSerializingRoleStub(owner)
	svcBQueries := &queryCountingTracer{}
	svcBPool := tracedSpacePool(t, pool, svcBQueries)
	svcBQueries.reset() // Ignore the test pool's readiness Ping.
	lockPoolA := independentSpacePool(t, pool, 1)
	lockPoolB := independentSpacePool(t, pool, 1)
	svcA := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: pool},
		Roles:          roles,
		MutationLocker: store.NewSpaceMutationLocker(lockPoolA),
	}
	svcB := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: svcBPool},
		Roles:          roles,
		MutationLocker: store.NewSpaceMutationLocker(lockPoolB),
	}
	ownerCtx := directServerContext(withAccountProfileCtx(ctx, uuid.New(), owner))
	memberACtx := directServerContext(withAccountProfileCtx(ctx, uuid.New(), memberA))

	firstDone := make(chan error, 1)
	go func() {
		_, err := svcA.TransferOwnership(ownerCtx, &spacev1.TransferOwnershipRequest{
			SpaceId:           spaceID.String(),
			NewOwnerProfileId: memberA.String(),
		})
		firstDone <- err
	}()
	select {
	case <-roles.firstListEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("first instance did not reach the Role transition")
	}
	defer func() {
		select {
		case <-roles.releaseFirst:
		default:
			close(roles.releaseFirst)
		}
	}()

	secondCtx, cancelSecond := context.WithTimeout(memberACtx, 200*time.Millisecond)
	defer cancelSecond()
	_, secondErr := svcB.TransferOwnership(secondCtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID.String(),
		NewOwnerProfileId: memberB.String(),
	})
	secondReachedRoles := false
	select {
	case <-roles.secondListEntered:
		secondReachedRoles = true
	default:
	}
	svcBQueriesBeforeRelease := svcBQueries.snapshot()

	close(roles.releaseFirst)
	require.NoError(t, requireTransferResult(t, firstDone))
	require.Equal(t, codes.DeadlineExceeded, status.Code(secondErr), "another instance must wait before entering the DB/Role ownership saga")
	require.False(t, secondReachedRoles, "second instance reached Role Service while the first transition still held the space lease")
	require.Zero(t, svcBQueriesBeforeRelease, "the distributed lease must precede owner and transfer database reads on the second instance")
	_, err := svcB.TransferOwnership(memberACtx, &spacev1.TransferOwnershipRequest{
		SpaceId:           spaceID.String(),
		NewOwnerProfileId: memberB.String(),
	})
	require.NoError(t, err, "retry after the first instance releases the lease must succeed")

	row, err := (&store.SpaceStore{Pool: pool}).GetSpace(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, memberB, row.OwnerProfileID)
	require.ElementsMatch(t, []string{memberB.String()}, roles.ownerProfiles(), "the database owner must remain the sole Owner role holder")
	requireOwnershipTransferAuditCount(t, pool, spaceID.String(), 2)
}

func TestUpdateSpace_CrossInstance_WaitsThroughTransferAuditRollback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	owner := uuid.New()
	member := uuid.New()
	spaceID := createSpaceWithMembers(t, pool, owner, member)
	transferCtx, cancelTransfer := context.WithCancel(directServerContext(withAccountProfileCtx(ctx, uuid.New(), owner)))
	defer cancelTransfer()
	tracer := &ownershipUpdateContextTracer{
		auditInsertStarted:  make(chan struct{}),
		blockAuditInsert:    make(chan struct{}),
		cancelOnAuditInsert: cancelTransfer,
	}
	tracedPool := tracedSpacePool(t, pool, tracer)
	svcBQueries := &queryCountingTracer{}
	svcBPool := tracedSpacePool(t, pool, svcBQueries)
	svcBQueries.reset() // Ignore the test pool's readiness Ping.
	lockPoolA := independentSpacePool(t, pool, 1)
	lockPoolB := independentSpacePool(t, pool, 1)
	svcA := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: tracedPool},
		MutationLocker: store.NewSpaceMutationLocker(lockPoolA),
	}
	svcB := &SpaceGRPC{
		Store:          &store.SpaceStore{Pool: svcBPool},
		MutationLocker: store.NewSpaceMutationLocker(lockPoolB),
	}

	transferDone := make(chan error, 1)
	go func() {
		_, err := svcA.TransferOwnership(transferCtx, &spacev1.TransferOwnershipRequest{
			SpaceId:           spaceID.String(),
			NewOwnerProfileId: member.String(),
		})
		transferDone <- err
	}()
	select {
	case <-tracer.auditInsertStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("first instance did not reach the blocked audit write")
	}
	defer func() {
		select {
		case <-tracer.blockAuditInsert:
		default:
			close(tracer.blockAuditInsert)
		}
	}()

	memberCtx := directServerContext(withAccountProfileCtx(ctx, uuid.New(), member))
	updateCtx, cancelUpdate := context.WithTimeout(memberCtx, 200*time.Millisecond)
	defer cancelUpdate()
	description := "must not observe the temporary owner"
	_, updateErr := svcB.UpdateSpace(updateCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:     spaceID.String(),
		Description: &description,
	})
	svcBQueriesBeforeRelease := svcBQueries.snapshot()

	close(tracer.blockAuditInsert)
	transferErr := requireTransferResult(t, transferDone)
	require.Equal(t, codes.DeadlineExceeded, status.Code(updateErr), "another instance must wait through audit failure and ownership rollback")
	require.Zero(t, svcBQueriesBeforeRelease, "the distributed lease must precede permission and update database reads on the second instance")
	require.Equal(t, codes.Internal, status.Code(transferErr))
	row, err := (&store.SpaceStore{Pool: pool}).GetSpace(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, owner, row.OwnerProfileID)
	require.Equal(t, "before", row.Description, "the timed-out cross-instance update must not persist")
	requireOwnershipTransferAuditCount(t, pool, spaceID.String(), 0)
}
