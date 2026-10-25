package mmadmissionctl

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/url"
	"os"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

func OpenReadOnlyJournal(path string) (*Journal, error) {
	st, e := os.Stat(path)
	if e != nil || !st.Mode().IsRegular() {
		return nil, ErrPending
	}
	u := url.URL{Scheme: "file", Path: path}
	db, e := sql.Open("sqlite3", u.String()+"?mode=ro&_query_only=1&_busy_timeout=1000")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.Ping(); e != nil {
		_ = db.Close()
		return nil, e
	}
	return &Journal{db: db, path: path}, nil
}
func (j *Journal) Observe(ctx context.Context, r w.ObserveRequestV1, peer w.PeerIdentity) (w.ObserveResponseV1, error) {
	var zero w.ObserveResponseV1
	if r.Validate() != nil {
		return zero, w.ErrInvalid
	}
	h, e := j.Read(ctx)
	if e != nil {
		return zero, ErrPending
	}
	var s Snapshot
	if json.Unmarshal(h.Snapshot, &s) != nil || s.Version != 1 || s.Spec == nil || s.Spec.Validate() != nil || s.Spec.AuthorityScope != w.LocalOwned || s.Spec.ScopeID != s.Identity.ScopeID || !s.HeadInstalled || s.ArmedReceipt == nil {
		return zero, ErrPending
	}
	launchHash, err := w.LaunchHash(*s.Spec)
	if err != nil || launchHash != s.Binding.LaunchManifestHash || validateHistory(s) != nil {
		return zero, ErrPending
	}
	bindingHash, e := w.BindingHash(s.Binding)
	if e != nil || bindingHash != r.ExpectedBindingHash || s.Identity.EnvironmentID != r.EnvironmentID || s.Identity.InstallationID != r.InstallationID {
		return zero, ErrPending
	}
	receiptHash, e := w.ArmedHash(*s.ArmedReceipt)
	if e != nil || receiptHash != s.Binding.AdmissionReceiptHash {
		return zero, ErrPending
	}
	if h.Pending != "" {
		var phase string
		if e = j.db.QueryRowContext(ctx, `SELECT phase FROM effects WHERE operation_id=? ORDER BY sequence DESC LIMIT 1`, h.Pending).Scan(&phase); e != nil {
			return zero, ErrPending
		}
		if r.Purpose != w.Startup || (phase != "STARTUP_PEERS_BOUND" && phase != "OPEN_INGRESS_INTENT") {
			return zero, ErrPending
		}
	}
	if r.Purpose == w.FreshAction && (s.State != w.Open || !s.IngressConfirmed) {
		return zero, ErrPending
	}
	if r.Purpose == w.Startup && s.State != w.Armed && s.State != w.Open {
		return zero, ErrPending
	}
	if s.State != w.Armed && s.State != w.Open {
		return zero, ErrPending
	}
	found := false
	for _, p := range s.Observation.Peers {
		if p.ReplicaID == r.ReplicaID && p.LaunchID == r.ProcessLaunchID && p.PID == peer.PID && p.UID == peer.UID && p.GID == peer.GID && p.ExecutableHash == peer.ExecutableHash {
			for _, replica := range s.Spec.Replicas {
				if replica.ReplicaID == p.ReplicaID && replica.LaunchID == p.LaunchID && replica.RuntimeUID == p.UID && replica.ExecutableHash == p.ExecutableHash {
					found = true
					break
				}
			}
			break
		}
	}
	if !found {
		return zero, ErrPending
	}
	var open *w.Hash
	if s.OpenReceipt != nil {
		hash, err := w.OpenHash(*s.OpenReceipt)
		if err != nil {
			return zero, ErrPending
		}
		open = &hash
	}
	out := w.ObserveResponseV1{ProtocolVersion: 1, Nonce: r.Nonce, ControllerID: s.Identity.ControllerID, AuthorityScope: w.LocalOwned, ScopeID: s.Identity.ScopeID, State: s.State, StateRevision: h.Revision, TransitionID: s.Binding.TransitionID, DatabaseLineageID: s.Identity.DatabaseLineageID, ExactBinding: s.Binding, BindingHash: bindingHash, ArmedReceiptHash: receiptHash, OpenReceiptHash: open, LaunchObservationHash: s.Observation.LaunchesHash, ContinuityReceiptsRootHash: s.HistoryRoot, ActionID: r.ActionID, ActionRequestHash: r.ActionRequestHash}
	if out.Validate() != nil {
		return zero, ErrPending
	}
	return out, nil
}

// ServeReadback has no mutation route. The listener is an owned, protected UDS;
// callers must not mount the control database or platform socket in serving.
type ReadbackRegistration struct {
	Binding w.ReadbackPeerBindingV1
	OwnTLS  tls.Certificate
}

// Registry reads current independently observed driver registrations. It must
// never create a launch registration as a side effect of a client connection.
type ReadbackRegistry interface {
	Lookup(w.KernelPeer) (ReadbackRegistration, error)
	Current(w.Hash) bool
}

func (j *Journal) ServeReadback(ctx context.Context, l *net.UnixListener, registries ...ReadbackRegistry) error {
	if len(registries) != 1 || registries[0] == nil {
		return w.ErrInvalid
	}
	registry := registries[0]
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = l.Close()
		case <-done:
		}
	}()
	slots := make(chan struct{}, 32)
	for {
		conn, e := l.AcceptUnix()
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			return e
		}
		select {
		case slots <- struct{}{}:
			go func() {
				defer func() { <-slots; _ = conn.Close() }()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				peer, e := w.PeerCredentials(conn)
				if e != nil {
					return
				}
				registration, e := registry.Lookup(peer)
				if e != nil || registration.Binding.Validate() != nil || !registry.Current(registration.Binding.PeerBindingHash) {
					return
				}
				secure, e := w.AuthenticateReadbackServer(ctx, conn, registration.Binding, registration.OwnTLS)
				if e != nil {
					return
				}
				var r w.ObserveRequestV1
				if w.ReadFrame(secure, &r) != nil {
					return
				}
				var tail [1]byte
				if n, e := secure.Read(tail[:]); n != 0 || e != io.EOF {
					return
				}
				p := registration.Binding
				if !registry.Current(p.PeerBindingHash) || r.ReplicaID != p.ConsumerReplicaID || r.ProcessLaunchID != p.ConsumerProcessLaunchID || r.ExpectedBindingHash != p.BindingHash || r.EnvironmentID != p.EnvironmentID || r.InstallationID != p.InstallationID {
					return
				}
				// Executable hash below is explicitly driver-observed. The peer
				// tuple was captured from this same authenticated stream.
				out, e := j.Observe(ctx, r, w.PeerIdentity{PID: peer.PID, UID: peer.UID, GID: peer.GID, ExecutableHash: p.ConsumerExecutableHash})
				if e != nil {
					return
				}
				if out.ControllerID != p.ControllerID || out.ScopeID != p.ScopeID || out.AuthorityScope != p.AuthorityScope || out.LaunchObservationHash != p.ConsumerLaunchObservationHash {
					return
				}
				if w.WriteFrame(secure, out) == nil {
					_ = secure.CloseWrite()
				}
			}()
		default:
			_ = conn.Close()
		}
	}
}
