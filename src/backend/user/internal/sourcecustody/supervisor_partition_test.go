package sourcecustody

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// This owned TCP relay transports PostgreSQL TLS without inspecting its bytes.
// cut joins every relay after closing only sockets created by this fixture.
type supervisorSQLPartition struct {
	listener   net.Listener
	upstream   string
	mu         sync.Mutex
	closed     bool
	sockets    map[net.Conn]struct{}
	acceptDone chan struct{}
	workers    sync.WaitGroup
	once       sync.Once
}

func newSupervisorSQLPartition(t *testing.T, raw string) (string, *supervisorSQLPartition) {
	t.Helper()
	endpoint, err := url.Parse(raw)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxy := &supervisorSQLPartition{listener: listener, upstream: endpoint.Host, sockets: map[net.Conn]struct{}{}, acceptDone: make(chan struct{})}
	go func() {
		defer close(proxy.acceptDone)
		for {
			incoming, err := listener.Accept()
			if err != nil {
				return
			}
			proxy.workers.Add(1)
			go proxy.relay(incoming)
		}
	}()
	t.Cleanup(proxy.cut)
	endpoint.Host = listener.Addr().String()
	return endpoint.String(), proxy
}

func (p *supervisorSQLPartition) track(conn net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.sockets[conn] = struct{}{}
	return true
}
func (p *supervisorSQLPartition) forget(conn net.Conn) {
	p.mu.Lock()
	delete(p.sockets, conn)
	p.mu.Unlock()
	_ = conn.Close()
}
func (p *supervisorSQLPartition) relay(incoming net.Conn) {
	defer p.workers.Done()
	if !p.track(incoming) {
		_ = incoming.Close()
		return
	}
	defer p.forget(incoming)
	outgoing, err := net.DialTimeout("tcp", p.upstream, time.Second)
	if err != nil {
		return
	}
	if !p.track(outgoing) {
		_ = outgoing.Close()
		return
	}
	defer p.forget(outgoing)
	copied := make(chan struct{})
	go func() { defer close(copied); _, _ = io.Copy(outgoing, incoming); _ = outgoing.Close() }()
	_, _ = io.Copy(incoming, outgoing)
	_ = incoming.Close()
	_ = outgoing.Close()
	<-copied
}
func (p *supervisorSQLPartition) cut() {
	p.once.Do(func() {
		p.mu.Lock()
		p.closed = true
		for socket := range p.sockets {
			_ = socket.Close()
		}
		p.mu.Unlock()
		_ = p.listener.Close()
		<-p.acceptDone
		p.workers.Wait()
	})
}

func TestProductionSupervisorSQLPartitionRemainsHoldUntilVerifiedRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires two owned TLS PostgreSQL servers, partition relay and production supervisor")
	}
	binary := buildProductionSupervisor(t)
	ctx := context.Background()
	controlURL, control := recoveryTLSDatabase(t, ctx, "partition-supervisor-control")
	adminURL, admin := recoveryTLSDatabase(t, ctx, "partition-supervisor-source")
	request := CreationRequest{SourceID: uuid.NewString(), Epoch: 1, Database: "voice_source_test"}
	require.NoError(t, control.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.ControlSystemID))
	require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text FROM pg_control_system()`).Scan(&request.SourceSystemID))
	_, _, _, source, keys, controller, claim, public, _, _ := openSourceFixtureDatabases(t, ctx, control, admin, request, true, nil, false)
	partitionedURL, partition := newSupervisorSQLPartition(t, adminURL)
	maintenance, err := openPinnedPostgres(ctx, partitionedURL)
	require.NoError(t, err)
	t.Cleanup(maintenance.Close)
	client, monitor, _ := startProductionSupervisor(t, binary, controlURL, partitionedURL, claim, public)
	h := &activationHandler{controller: controller, admin: maintenance, source: source, keys: keys, signer: public, supervisor: client}
	t.Cleanup(func() {
		if h.supervised != nil {
			_ = h.supervised.stop()
		}
	})
	require.Equal(t, http.StatusNoContent, openThroughOperator(t, h, claim))
	permit, _, err := controller.AcquireAttested(ctx, claim.Binding)
	require.NoError(t, err)
	defer func() { _ = permit.Release(context.Background()) }()
	partition.cut()
	select {
	case <-h.supervised.session.revoked():
	case <-time.After(8 * time.Second):
		t.Fatal("SQL partition did not revoke supervision")
	}
	require.Error(t, permit.Check(ctx))
	require.NoError(t, permit.Release(ctx))
	select {
	case <-h.supervised.done:
	case <-time.After(40 * time.Second):
		t.Fatal("unreachable SQL did not settle to HOLD")
	}
	require.ErrorIs(t, h.supervised.result, ErrSupervisionHold)
	var state, phase string
	var allowed, login bool
	require.NoError(t, control.QueryRow(ctx, `SELECT state FROM user_custody_state WHERE singleton`).Scan(&state))
	require.Equal(t, "CLOSED", state)
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.NotEqual(t, "RECOVERED_CLOSED", phase)
	require.NoError(t, admin.QueryRow(ctx, `SELECT datallowconn FROM pg_database WHERE oid=$1`, claim.Physical.DatabaseOID).Scan(&allowed))
	require.True(t, allowed)
	require.NoError(t, admin.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=$1`, claim.WriterRole).Scan(&login))
	require.True(t, login)
	require.NoError(t, source.Ping(ctx), "the source remains up; unavailable maintenance is not a physical stop")
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim))
	select {
	case err := <-monitor.done:
		monitor.joined = true
		require.Error(t, err)
	case <-time.After(40 * time.Second):
		t.Fatal("independent supervisor did not finish its HOLD recovery")
	}
	// A custodian can later use its separately held reachable maintenance path;
	// this verifies drain without reviving the old controller or decision.
	require.NoError(t, recoverSupervisedSource(control, admin, claim))
	require.NoError(t, control.QueryRow(ctx, `SELECT phase FROM user_custody_creation WHERE singleton`).Scan(&phase))
	require.Equal(t, "RECOVERED_CLOSED", phase)
	require.Equal(t, http.StatusServiceUnavailable, openThroughOperator(t, h, claim))
}
