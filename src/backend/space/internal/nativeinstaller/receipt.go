// Package nativeinstaller owns fresh database provisioning, outside serving RPCs.
package nativeinstaller

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"

	"voice/backend/space/internal/store"
)

type Receipt struct {
	RunID            string                                  `json:"run_id"`
	SourceRevision   string                                  `json:"source_revision"`
	SourceSHA256     string                                  `json:"source_sha256"`
	ExecutableSHA256 string                                  `json:"executable_sha256"`
	MigratorSHA256   string                                  `json:"migrator_sha256"`
	InventorySHA256  string                                  `json:"inventory_sha256"`
	Created          bool                                    `json:"created"`
	Creation         *store.NativeInstallerSourceFacts       `json:"creation"`
	Trusted44        *store.NativeInstallerSourceFacts       `json:"trusted44"`
	Transition45     *store.NativeMembershipCatalog45Receipt `json:"transition45"`
	Transition50     *store.NativeMembershipCatalog50Receipt `json:"transition50"`
	Trusted50        *store.NativeInstallerSourceFacts       `json:"trusted50"`
	Transition51     *store.NativeMembershipCatalog51Receipt `json:"transition51"`
	Final            *store.NativeInstallerSourceFacts       `json:"final"`
}

type event struct {
	Event   string   `json:"event"`
	Receipt *Receipt `json:"receipt"`
}

// ReadReceipt reads the actual completed JSON-lines output retained by the
// owning deployment controller. A caller's file is not an authentication scheme;
// the controller must retain its original Job/image/source/target custody.
func ReadReceipt(in io.Reader) (*Receipt, error) {
	const maxEvidenceBytes = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(in, maxEvidenceBytes+1))
	if err != nil || len(raw) > maxEvidenceBytes {
		return nil, errors.New("invalid or oversized retained installer evidence")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	var result *Receipt
	for {
		var e event
		if err := d.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.New("invalid retained installer evidence")
		}
		if e.Event == "complete" {
			if result != nil {
				return nil, errors.New("ambiguous retained installer evidence")
			}
			result = e.Receipt
		}
	}
	if !validCompletion(result) {
		return nil, errors.New("completed installation evidence required")
	}
	return result, nil
}

func validCompletion(r *Receipt) bool {
	if r == nil || !r.Created || r.Creation == nil || r.Trusted44 == nil || r.Transition45 == nil || r.Transition50 == nil || r.Trusted50 == nil || r.Transition51 == nil || r.Final == nil {
		return false
	}
	if r.Creation.DatabaseName != "space_db" || r.Creation.DatabaseOID == 0 || r.Creation.InstallerRole == "" || !sameDatabase(r.Creation, r.Trusted44) || !sameTarget(r.Trusted44, r.Trusted50) || !sameTarget(r.Trusted44, r.Final) || r.Trusted44.Version != 44 || r.Trusted50.Version != 50 || r.Final.Version != 51 || r.Transition51.Profile != store.NativeCatalogRowJSONProfile || r.Transition51.Engine != "github.com/golang-migrate/migrate/v4@v4.18.1" {
		return false
	}
	for _, digest := range []string{r.SourceSHA256, r.ExecutableSHA256, r.MigratorSHA256, r.InventorySHA256, r.Trusted44.SealSHA256, r.Transition45.NewSealSHA256, r.Transition50.NewSealSHA256, r.Transition51.NewSealSHA256, r.Transition51.MigrationSHA256} {
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 {
			return false
		}
	}
	if r.Trusted44.Epoch != "94db7e08-e958-44fa-94b2-988410ab0044" || !matchesTransition(r.Trusted44, r.Transition45) {
		return false
	}
	if r.Transition51.MigrationSHA256 != store.NativeCatalog51MigrationSHA256 {
		return false
	}
	f45 := *r.Trusted44
	f45.Version = 45
	f45.SealSHA256 = r.Transition45.NewSealSHA256
	return matchesTransition(&f45, (*store.NativeMembershipCatalog45Receipt)(r.Transition50)) && r.Trusted50.SealSHA256 == r.Transition50.NewSealSHA256 && matchesTransition(r.Trusted50, &r.Transition51.NativeMembershipCatalog45Receipt) && r.Final.SealSHA256 == r.Transition51.NewSealSHA256
}
