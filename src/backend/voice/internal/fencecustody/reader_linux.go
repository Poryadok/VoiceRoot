//go:build linux

package fencecustody

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/google/uuid"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

// ReadPostgresFenceReceipt authenticates local installer-file provenance for a
// distinct non-root reader. It is historical readback, not a current resource
// observation, maintenance authorization or activation certificate.
func ReadPostgresFenceReceipt(ctx context.Context, ref ReceiptRef) (PostgresFenceReceipt, error) {
	if os.Geteuid() == 0 || ctx.Err() != nil {
		return PostgresFenceReceipt{}, retirementproof.ErrUnavailable
	}
	return readReceipt(installedPaths, ref)
}

func readReceipt(p paths, ref ReceiptRef) (PostgresFenceReceipt, error) {
	var r PostgresFenceReceipt
	if ref.ReceiptID == uuid.Nil || ref.Hash == ([32]byte{}) {
		return r, retirementproof.ErrConflict
	}
	raw, err := readInstalledFile(p.Proofs, ref.ReceiptID.String()+".json", false)
	if err != nil || retirementproof.Hash(ReceiptDomain, raw) != ref.Hash || json.Unmarshal(raw, &r) != nil {
		return r, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(canonical, raw) || r.Protocol != 1 || r.ReceiptID != ref.ReceiptID || r.ReceiptID != uuid.NewSHA1(r.Command.CommandID, []byte(ReceiptDomain)) {
		return r, retirementproof.ErrConflict
	}
	command, err := readCommand(p, r.Command)
	if err != nil || r.Cut != command.Cut || r.FencerArtifact != command.FencerArtifactSHA256 || r.Login != command.FencerLogin || r.CredentialCustodyID != command.CredentialCustodyID || r.Evidence.Binding != command.Target || !validPostgresEvidence(r.Evidence) {
		return r, retirementproof.ErrConflict
	}
	if _, err = readWindow(p, r.Command, r.Cut); err != nil {
		return r, err
	}
	return r, nil
}

// ReadPostgresFenceObservation resolves a separately published reobservation in
// the current installer execution window. The installer must actually launch
// that bounded observation before cleanup/recovery; this reader cannot refresh
// a resource and must not be used as a cached replacement for that launch.
func ReadPostgresFenceObservation(ctx context.Context, ref ObservationRef) (PostgresFenceObservation, PostgresFenceReceipt, error) {
	if os.Geteuid() == 0 || ctx.Err() != nil {
		return PostgresFenceObservation{}, PostgresFenceReceipt{}, retirementproof.ErrUnavailable
	}
	return readObservation(installedPaths, ref)
}

func readObservation(p paths, ref ObservationRef) (PostgresFenceObservation, PostgresFenceReceipt, error) {
	var o PostgresFenceObservation
	if ref.ObservationID == uuid.Nil || ref.Hash == ([32]byte{}) {
		return o, PostgresFenceReceipt{}, retirementproof.ErrConflict
	}
	raw, err := readInstalledFile(p.Observations, ref.ObservationID.String()+".json", false)
	if err != nil || retirementproof.Hash(ObservationDomain, raw) != ref.Hash || json.Unmarshal(raw, &o) != nil {
		return o, PostgresFenceReceipt{}, retirementproof.ErrConflict
	}
	canonical, err := json.Marshal(o)
	if err != nil || !bytes.Equal(canonical, raw) || o.Protocol != 1 || o.ObservationID != ref.ObservationID || o.ExecutionID == uuid.Nil {
		return o, PostgresFenceReceipt{}, retirementproof.ErrConflict
	}
	if _, err = time.Parse(time.RFC3339Nano, o.ObservedUTC); err != nil {
		return o, PostgresFenceReceipt{}, retirementproof.ErrConflict
	}
	r, err := readReceipt(p, o.Receipt)
	if err != nil || o.Command != r.Command || o.Evidence.Binding != r.Evidence.Binding || !validPostgresEvidence(o.Evidence) {
		return o, r, retirementproof.ErrConflict
	}
	w, err := readWindow(p, o.Command, r.Cut)
	if err != nil || w.ExecutionID != o.ExecutionID {
		return o, r, retirementproof.ErrConflict
	}
	return o, r, nil
}

func validPostgresEvidence(e partycoverage.PostgresFenceEvidence) bool {
	var s struct {
		RoleOID                                                        uint32
		RoleName                                                       string
		Login, Privileged, Prepared, Delegated, Established, Inherited bool
	}
	if e.Protocol != 1 || e.Binding.RoleOID == 0 || e.Binding.RoleName == "" || e.Binding.DatabaseIdentity == ([32]byte{}) || len(e.AuthorityBytes) == 0 || retirementproof.Hash("voice.party.postgres-fence-authority.v1", e.AuthorityBytes) != e.AuthorityHash || json.Unmarshal(e.AuthorityBytes, &s) != nil {
		return false
	}
	raw, err := json.Marshal(s)
	return err == nil && bytes.Equal(raw, e.AuthorityBytes) && s.RoleOID == e.Binding.RoleOID && s.RoleName == e.Binding.RoleName && !s.Login && !s.Privileged && !s.Prepared && !s.Delegated && !s.Established && !s.Inherited
}
