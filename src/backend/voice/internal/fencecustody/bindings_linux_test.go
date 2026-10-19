//go:build linux

package fencecustody

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

func fixtureCut() retirementproof.CutBinding {
	h := retirementproof.Hash("fixture", []byte("shape only, not custody"))
	return retirementproof.CutBinding{EnvironmentID: uuid.New(), AuthorityIncarnation: uuid.New(), CutoverOperationID: uuid.New(), CoverageEpoch: 1, PlanHash: h, DatabaseIdentity: h, SourceCatalogHash: h, SchemaContractHash: h, WriterManifestHash: h, ProviderManifestHash: h, CredentialManifestHash: h, ClosedAdmissionBarrierID: uuid.New(), ClosedAdmissionBarrierHash: h, IssuerImplementationManifestHash: h}
}

func TestProtectedCommandReceiptAndExecutionBindings(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("installer provenance requires the owned Linux root runner")
	}
	root := "/run/rudeus191-fencer-bindings-" + uuid.NewString()
	require.NoError(t, os.Mkdir(root, 0755))
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(root)) })
	p := paths{Plans: root + "/plans", Proofs: root + "/proofs", Observations: root + "/observations", Locks: root + "/locks", Secrets: root + "/secrets", Installation: root + "/installation"}
	for _, dir := range []string{p.Plans, p.Proofs, p.Observations, p.Locks, p.Secrets, p.Installation} {
		require.NoError(t, os.Mkdir(dir, 0755))
	}
	write := func(dir, name string, value any) []byte {
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		// Installer fixture may rotate its external window, unlike immutable proofs.
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), raw, 0444))
		return raw
	}
	cut := fixtureCut()
	c := PreparedPostgresCommand{Protocol: 1, Kind: "RETIRE_POSTGRES_ROLE", CommandID: uuid.New(), Cut: cut, PredecessorExecutionCohortID: uuid.New(), Target: partycoverage.PostgresRoleBinding{DatabaseIdentity: cut.DatabaseIdentity, RoleOID: 20, RoleName: "old_writer"}, Endpoint: UnixPeerEndpoint{SocketDirectory: "/pgsocket", Database: "voice", Port: 5432, PeerUID: 999}, CredentialCustodyID: uuid.New(), FencerLogin: LoginIdentity{RoleOID: 10, RoleName: "installer_fencer"}, FencerArtifactSHA256: cut.PlanHash, TransportBinding: InstallerRootExec}
	raw := write(p.Plans, c.CommandID.String()+".json", c)
	ref := CommandRef{CommandID: c.CommandID, Hash: retirementproof.Hash(CommandDomain, raw)}
	w := InstallationWindow{Protocol: 1, ExecutionID: uuid.New(), Installation: retirementproof.Installation{EnvironmentID: cut.EnvironmentID, AuthorityIncarnation: cut.AuthorityIncarnation, IssuerImplementationManifestHash: cut.IssuerImplementationManifestHash}, CutoverOperationID: cut.CutoverOperationID, CoverageEpoch: cut.CoverageEpoch, PlanHash: cut.PlanHash, ClosedAdmissionBarrierID: cut.ClosedAdmissionBarrierID, ClosedAdmissionBarrierHash: cut.ClosedAdmissionBarrierHash, AdmittedCommands: []CommandRef{ref}}
	write(p.Installation, "current.json", w)
	_, err := readCommand(p, ref)
	require.NoError(t, err)
	_, err = readWindow(p, ref, cut)
	require.NoError(t, err)
	authority := []byte(`{"RoleOID":20,"RoleName":"old_writer","Login":false,"Privileged":false,"Prepared":false,"Delegated":false,"Established":false,"Inherited":false}`)
	e := partycoverage.PostgresFenceEvidence{Protocol: 1, Binding: c.Target, AuthorityBytes: authority, AuthorityHash: retirementproof.Hash("voice.party.postgres-fence-authority.v1", authority)}
	r := PostgresFenceReceipt{Protocol: 1, ReceiptID: uuid.NewSHA1(ref.CommandID, []byte(ReceiptDomain)), Command: ref, Cut: cut, FencerArtifact: c.FencerArtifactSHA256, Login: c.FencerLogin, CredentialCustodyID: c.CredentialCustodyID, Evidence: e}
	raw = write(p.Proofs, r.ReceiptID.String()+".json", r)
	rr := ReceiptRef{ReceiptID: r.ReceiptID, Hash: retirementproof.Hash(ReceiptDomain, raw)}
	_, err = readReceipt(p, rr)
	require.NoError(t, err, "shape fixture is not actual retirement evidence")
	o := PostgresFenceObservation{Protocol: 1, ExecutionID: w.ExecutionID, ObservationID: uuid.New(), Command: ref, Receipt: rr, ObservedUTC: time.Now().UTC().Format(time.RFC3339Nano), Evidence: e}
	raw = write(p.Observations, o.ObservationID.String()+".json", o)
	or := ObservationRef{ObservationID: o.ObservationID, Hash: retirementproof.Hash(ObservationDomain, raw)}
	_, _, err = readObservation(p, or)
	require.NoError(t, err)
	t.Run("wrong command hash", func(t *testing.T) {
		bad := ref
		bad.Hash[0]++
		_, err := readCommand(p, bad)
		require.ErrorIs(t, err, retirementproof.ErrConflict)
	})
	t.Run("copied database binding", func(t *testing.T) {
		bad := c
		bad.CommandID = uuid.New()
		bad.Target.DatabaseIdentity[0]++
		raw := write(p.Plans, bad.CommandID.String()+".json", bad)
		_, err := readCommand(p, CommandRef{CommandID: bad.CommandID, Hash: retirementproof.Hash(CommandDomain, raw)})
		require.ErrorIs(t, err, retirementproof.ErrConflict)
	})
	t.Run("forged receipt authority", func(t *testing.T) {
		bad := r
		bad.Evidence.AuthorityBytes = []byte(`{"RoleOID":21}`)
		bad.Evidence.AuthorityHash = retirementproof.Hash("voice.party.postgres-fence-authority.v1", bad.Evidence.AuthorityBytes)
		raw := write(p.Proofs, r.ReceiptID.String()+".json", bad)
		_, err := readReceipt(p, ReceiptRef{ReceiptID: r.ReceiptID, Hash: retirementproof.Hash(ReceiptDomain, raw)})
		require.ErrorIs(t, err, retirementproof.ErrConflict)
		write(p.Proofs, r.ReceiptID.String()+".json", r)
	})
	t.Run("external incarnation changed", func(t *testing.T) {
		bad := w
		bad.Installation.AuthorityIncarnation = uuid.New()
		write(p.Installation, "current.json", bad)
		_, err := readReceipt(p, rr)
		require.ErrorIs(t, err, retirementproof.ErrConflict)
		write(p.Installation, "current.json", w)
	})
	t.Run("old observation after execution rotation", func(t *testing.T) {
		bad := w
		bad.ExecutionID = uuid.New()
		write(p.Installation, "current.json", bad)
		_, _, err := readObservation(p, or)
		require.ErrorIs(t, err, retirementproof.ErrConflict)
		write(p.Installation, "current.json", w)
	})
	t.Run("non-admitted command", func(t *testing.T) {
		bad := w
		bad.AdmittedCommands = nil
		write(p.Installation, "current.json", bad)
		_, err := readWindow(p, ref, cut)
		require.ErrorIs(t, err, retirementproof.ErrConflict)
		write(p.Installation, "current.json", w)
	})
}
