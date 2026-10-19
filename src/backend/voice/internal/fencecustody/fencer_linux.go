//go:build linux

package fencecustody

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sys/unix"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

type paths struct{ Plans, Proofs, Observations, Locks, Secrets, Installation string }

var installedPaths = paths{
	Plans: "/var/lib/voice-party-fencer/plans", Proofs: "/var/lib/voice-party-fencer/proofs",
	Observations: "/var/lib/voice-party-fencer/observations", Locks: "/var/lib/voice-party-fencer/locks",
	Secrets: "/run/secrets/voice-party-fencer", Installation: "/run/voice-party-installation",
}

// FencePreparedPostgresRole accepts no DSN, SQL, path, callback or target input.
// Only the installer can launch this root-only artifact with its secret mount.
// The application executor must not have that mount or runtime-control API.
func FencePreparedPostgresRole(ctx context.Context, ref CommandRef) (FenceResult, error) {
	return runPrepared(ctx, installedPaths, ref, true)
}

// ReobservePreparedPostgresFence never re-enables or retires a role. A historical
// receipt is returned only alongside a new observation in the admitted window.
func ReobservePreparedPostgresFence(ctx context.Context, ref CommandRef) (FenceResult, error) {
	return runPrepared(ctx, installedPaths, ref, false)
}

func runPrepared(ctx context.Context, p paths, ref CommandRef, retire bool) (FenceResult, error) {
	if os.Getuid() != 0 || os.Geteuid() != 0 {
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	if !fixedConnectionEnvironment() {
		return FenceResult{}, retirementproof.ErrUnsupported
	}
	command, err := readCommand(p, ref)
	if err != nil {
		return FenceResult{}, err
	}
	artifact, err := actualArtifactHash()
	if err != nil || artifact != command.FencerArtifactSHA256 {
		return FenceResult{}, retirementproof.ErrConflict
	}
	unlock, err := commandLock(p.Locks, ref.CommandID)
	if err != nil {
		return FenceResult{}, err
	}
	defer unlock()
	window, err := readWindow(p, ref, command.Cut)
	if err != nil {
		return FenceResult{}, err
	}
	secret, err := readInstalledFile(p.Secrets, command.CredentialCustodyID.String()+".dsn", true)
	if err != nil {
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	config, err := pgx.ParseConfig(strings.TrimSpace(string(secret)))
	if err != nil || config.Host != command.Endpoint.SocketDirectory || config.Port != command.Endpoint.Port || config.Database != command.Endpoint.Database || config.User != command.FencerLogin.RoleName || config.Password != "" || config.TLSConfig != nil || len(config.Fallbacks) != 0 {
		return FenceResult{}, retirementproof.ErrConflict
	}
	for key := range config.RuntimeParams {
		if key != "application_name" {
			return FenceResult{}, retirementproof.ErrUnsupported
		}
	}
	// Resource identity/authority queries must resolve the real system catalog,
	// independently of database/role search_path defaults left by old writers.
	config.RuntimeParams = map[string]string{"application_name": "voice-party-fencer", "search_path": "pg_catalog"}
	config.ConnectTimeout = 5 * time.Second
	connection, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		// Never return DSN-bearing driver errors across the component boundary.
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		connection.Close(cleanup)
	}()
	if err = checkSession(ctx, connection, command); err != nil {
		return FenceResult{}, err
	}
	if retire {
		if err = partycoverage.RetirePostgresRole(ctx, connection, command.Target.RoleName); err != nil {
			return FenceResult{}, retirementproof.ErrUnavailable
		}
	}
	evidence, err := partycoverage.ObservePostgresFence(ctx, connection, command.Target)
	if err != nil {
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	if err = checkSession(ctx, connection, command); err != nil {
		return FenceResult{}, err
	}
	current, err := readWindow(p, ref, command.Cut)
	if err != nil || current.ExecutionID != window.ExecutionID {
		return FenceResult{}, retirementproof.ErrConflict
	}
	id := uuid.NewSHA1(ref.CommandID, []byte(ReceiptDomain))
	receipt := PostgresFenceReceipt{Protocol: 1, ReceiptID: id, Command: ref, Cut: command.Cut, FencerArtifact: artifact, Login: command.FencerLogin, CredentialCustodyID: command.CredentialCustodyID, Evidence: evidence}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	result := FenceResult{Receipt: ReceiptRef{ReceiptID: id, Hash: retirementproof.Hash(ReceiptDomain, raw)}}
	if err = writeInstalledFile(p.Proofs, id.String()+".json", raw); err != nil {
		return FenceResult{}, err
	}
	observation := PostgresFenceObservation{Protocol: 1, ExecutionID: window.ExecutionID, ObservationID: uuid.New(), Command: ref, Receipt: result.Receipt, ObservedUTC: time.Now().UTC().Format(time.RFC3339Nano), Evidence: evidence}
	raw, err = json.Marshal(observation)
	if err != nil {
		return FenceResult{}, retirementproof.ErrUnavailable
	}
	if err = writeInstalledFile(p.Observations, observation.ObservationID.String()+".json", raw); err != nil {
		return FenceResult{}, err
	}
	result.Observation = ObservationRef{ObservationID: observation.ObservationID, Hash: retirementproof.Hash(ObservationDomain, raw)}
	return result, nil
}

func fixedConnectionEnvironment() bool {
	// ParseConfig otherwise consults ambient libpq-compatible variables. Only
	// the private admitted credential document may select connection settings.
	for _, name := range []string{"PGHOST", "PGHOSTADDR", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGPASSFILE", "PGSERVICE", "PGSERVICEFILE", "PGOPTIONS", "PGTARGETSESSIONATTRS", "PGSSLMODE", "PGSSLROOTCERT", "PGSSLCERT", "PGSSLKEY", "PGSSLCRL", "PGSSLCRLDIR", "PGCONNECT_TIMEOUT", "PGAPPNAME", "PGCLIENTENCODING"} {
		if value, present := os.LookupEnv(name); present && value != "" {
			return false
		}
	}
	return true
}

func readCommand(p paths, ref CommandRef) (PreparedPostgresCommand, error) {
	var c PreparedPostgresCommand
	if ref.CommandID == uuid.Nil || ref.Hash == ([32]byte{}) {
		return c, retirementproof.ErrConflict
	}
	raw, err := readInstalledFile(p.Plans, ref.CommandID.String()+".json", false)
	if err != nil || retirementproof.Hash(CommandDomain, raw) != ref.Hash || json.Unmarshal(raw, &c) != nil {
		return c, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(c)
	if err != nil || !bytes.Equal(canonical, raw) || c.Protocol != 1 || c.Kind != "RETIRE_POSTGRES_ROLE" || c.CommandID != ref.CommandID || !validCut(c.Cut) || c.PredecessorExecutionCohortID == uuid.Nil || c.Target.DatabaseIdentity != c.Cut.DatabaseIdentity || c.Target.RoleOID == 0 || c.Target.RoleName == "" || c.CredentialCustodyID == uuid.Nil || c.FencerLogin.RoleOID == 0 || c.FencerLogin.RoleName == "" || c.FencerLogin.RoleOID == c.Target.RoleOID || c.FencerLogin.RoleName == c.Target.RoleName || c.FencerArtifactSHA256 == ([32]byte{}) || c.TransportBinding != InstallerRootExec || !filepath.IsAbs(c.Endpoint.SocketDirectory) || filepath.Clean(c.Endpoint.SocketDirectory) != c.Endpoint.SocketDirectory || c.Endpoint.Port == 0 || c.Endpoint.Database == "" {
		return c, retirementproof.ErrConflict
	}
	return c, nil
}

func validCut(c retirementproof.CutBinding) bool {
	if c.EnvironmentID == uuid.Nil || c.AuthorityIncarnation == uuid.Nil || c.CutoverOperationID == uuid.Nil || c.ClosedAdmissionBarrierID == uuid.Nil || c.CoverageEpoch == 0 || c.CoverageEpoch > math.MaxInt64 {
		return false
	}
	for _, hash := range [][32]byte{c.PlanHash, c.DatabaseIdentity, c.SourceCatalogHash, c.SchemaContractHash, c.WriterManifestHash, c.ProviderManifestHash, c.CredentialManifestHash, c.ClosedAdmissionBarrierHash, c.IssuerImplementationManifestHash} {
		if hash == ([32]byte{}) {
			return false
		}
	}
	return true
}

func readWindow(p paths, ref CommandRef, cut retirementproof.CutBinding) (InstallationWindow, error) {
	var w InstallationWindow
	raw, err := readInstalledFile(p.Installation, "current.json", false)
	if err != nil || json.Unmarshal(raw, &w) != nil {
		return w, retirementproof.ErrUnavailable
	}
	canonical, err := json.Marshal(w)
	if err != nil || !bytes.Equal(canonical, raw) || w.Protocol != 1 || w.ExecutionID == uuid.Nil || w.Installation.EnvironmentID != cut.EnvironmentID || w.Installation.AuthorityIncarnation != cut.AuthorityIncarnation || w.Installation.IssuerImplementationManifestHash != cut.IssuerImplementationManifestHash || w.CutoverOperationID != cut.CutoverOperationID || w.CoverageEpoch != cut.CoverageEpoch || w.PlanHash != cut.PlanHash || w.ClosedAdmissionBarrierID != cut.ClosedAdmissionBarrierID || w.ClosedAdmissionBarrierHash != cut.ClosedAdmissionBarrierHash {
		return w, retirementproof.ErrConflict
	}
	found := false
	seen := map[uuid.UUID]bool{}
	for _, admitted := range w.AdmittedCommands {
		if admitted.CommandID == uuid.Nil || admitted.Hash == ([32]byte{}) || seen[admitted.CommandID] {
			return w, retirementproof.ErrConflict
		}
		seen[admitted.CommandID] = true
		found = found || admitted == ref
	}
	if !found {
		return w, retirementproof.ErrConflict
	}
	return w, nil
}

func checkSession(ctx context.Context, c *pgx.Conn, command PreparedPostgresCommand) error {
	if c == nil || c.Config().User != command.FencerLogin.RoleName {
		return retirementproof.ErrUnavailable
	}
	transport, ok := c.PgConn().Conn().(syscall.Conn)
	if !ok || c.PgConn().Conn().RemoteAddr().Network() != "unix" {
		return retirementproof.ErrUnsupported
	}
	raw, err := transport.SyscallConn()
	if err != nil {
		return retirementproof.ErrUnavailable
	}
	peerOK := false
	if err = raw.Control(func(fd uintptr) {
		peer, e := unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
		peerOK = e == nil && peer.Uid == command.Endpoint.PeerUID
	}); err != nil || !peerOK {
		return retirementproof.ErrConflict
	}
	var matched bool
	err = c.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles a JOIN pg_stat_activity b ON b.pid=pg_backend_pid()
 WHERE a.oid=$1::oid AND a.rolname=$2 AND a.rolsuper AND a.rolcanlogin
 AND current_user=a.rolname AND session_user=a.rolname AND b.usesysid=a.oid)
 AND EXISTS(SELECT 1 FROM pg_roles WHERE oid=$3::oid AND rolname=$4)`, command.FencerLogin.RoleOID, command.FencerLogin.RoleName, command.Target.RoleOID, command.Target.RoleName).Scan(&matched)
	if err != nil || !matched {
		return retirementproof.ErrConflict
	}
	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return retirementproof.ErrUnavailable
	}
	defer tx.Rollback(ctx)
	identity, err := partycoverage.ObserveDatabaseIdentity(ctx, tx)
	if err != nil || identity != command.Target.DatabaseIdentity {
		return retirementproof.ErrConflict
	}
	return tx.Commit(ctx)
}

func actualArtifactHash() ([32]byte, error) {
	path, err := os.Executable()
	if err != nil {
		return [32]byte{}, retirementproof.ErrUnavailable
	}
	dir, err := protectedDirectory(filepath.Dir(path))
	if err != nil {
		return [32]byte{}, err
	}
	defer unix.Close(dir)
	fd, err := unix.Openat(dir, filepath.Base(path), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return [32]byte{}, retirementproof.ErrUnavailable
	}
	f := os.NewFile(uintptr(fd), path)
	defer f.Close()
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != 0 || st.Mode&06222 != 0 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 || st.Size <= 0 || st.Size > 256<<20 {
		return [32]byte{}, retirementproof.ErrUnavailable
	}
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return [32]byte{}, retirementproof.ErrUnavailable
	}
	var digest [32]byte
	copy(digest[:], h.Sum(nil))
	return digest, nil
}

func commandLock(root string, id uuid.UUID) (func(), error) {
	dir, err := protectedDirectory(root)
	if err != nil {
		return nil, err
	}
	defer unix.Close(dir)
	fd, err := unix.Openat(dir, id.String()+".lock", unix.O_RDWR|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, retirementproof.ErrUnavailable
	}
	var st unix.Stat_t
	if unix.Fstat(fd, &st) != nil || st.Uid != 0 || st.Mode&0077 != 0 || st.Mode&unix.S_IFMT != unix.S_IFREG || st.Nlink != 1 {
		unix.Close(fd)
		return nil, retirementproof.ErrUnavailable
	}
	if err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		unix.Close(fd)
		return nil, retirementproof.ErrPending
	}
	return func() { unix.Close(fd) }, nil
}
