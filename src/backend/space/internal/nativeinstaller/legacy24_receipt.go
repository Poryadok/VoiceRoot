package nativeinstaller

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"

	"github.com/google/uuid"
	"voice/backend/space/internal/store"
)

// Legacy24Intermediate is an observed intermediate, never the original
// installation or evidence that the database began at 25.
type Legacy24Intermediate struct {
	Target          *store.NativeInstallerSourceFacts `json:"target"`
	Clean           bool                              `json:"clean"`
	MigrationSHA256 string                            `json:"migration_sha256"`
	CatalogSHA256   string                            `json:"catalog_sha256"`
	PreimageSHA256  string                            `json:"preimage_sha256"`
	AccessSHA256    string                            `json:"access_sha256"`
}

type Legacy24Receipt struct {
	Schema               string                                  `json:"schema"`
	Origin               string                                  `json:"origin"`
	RunID                string                                  `json:"run_id"`
	OperationID          string                                  `json:"operation_id"`
	SnapshotID           string                                  `json:"snapshot_id"`
	SourceRevision       string                                  `json:"source_revision"`
	SourceSHA256         string                                  `json:"source_sha256"`
	ExecutableSHA256     string                                  `json:"executable_sha256"`
	MigratorSHA256       string                                  `json:"migrator_sha256"`
	InventorySHA256      string                                  `json:"inventory_sha256"`
	Source24             *store.NativeInstallerSourceFacts       `json:"source24"`
	Clean24              bool                                    `json:"clean24"`
	Catalog24SHA256      string                                  `json:"catalog24_sha256"`
	PreimageSHA256       string                                  `json:"preimage_sha256"`
	AccessSHA256         string                                  `json:"access_sha256"`
	SpaceCount           uint64                                  `json:"space_count"`
	Trusted25            *Legacy24Intermediate                   `json:"trusted25"`
	Trusted44            *store.NativeInstallerSourceFacts       `json:"trusted44"`
	Transition45         *store.NativeMembershipCatalog45Receipt `json:"transition45"`
	Trusted45            *store.NativeInstallerSourceFacts       `json:"trusted45"`
	Transition50         *store.NativeMembershipCatalog50Receipt `json:"transition50"`
	Trusted50            *store.NativeInstallerSourceFacts       `json:"trusted50"`
	Transition51         *store.NativeMembershipCatalog51Receipt `json:"transition51"`
	Final                *store.NativeInstallerSourceFacts       `json:"final"`
	PreservationSHA256   string                                  `json:"preservation_sha256"`
	InitializationSHA256 string                                  `json:"initialization_sha256"`
	InitializationCount  uint64                                  `json:"initialization_count"`
	Scope                string                                  `json:"scope"`
}

type legacy24Event struct {
	Event   string           `json:"event"`
	Receipt *Legacy24Receipt `json:"receipt"`
}

var legacy24Stages = [...]string{"source24", "trusted25", "trusted44", "trusted45", "trusted50", "trusted51", "initialized", "upgrade-complete"}

// ReadLegacy24Receipt parses a complete, closed original-24 evidence stream.
// Parsing is not authentication: the owning controller retains the actual
// source, backup, Job, image and execution custody separately.
func ReadLegacy24Receipt(in io.Reader) (*Legacy24Receipt, error) {
	const limit = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(in, limit+1))
	if err != nil || len(raw) > limit || legacy25UniqueJSON(raw) != nil {
		return nil, errors.New("invalid legacy24 evidence")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	events := make([]legacy24Event, 0, len(legacy24Stages))
	for {
		var item json.RawMessage
		err = d.Decode(&item)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid legacy24 evidence")
		}
		var event legacy24Event
		if legacy25ExactFields(item, reflect.TypeOf(event), 0) != nil || json.Unmarshal(item, &event) != nil || event.Receipt == nil ||
			len(events) >= len(legacy24Stages) || event.Event != legacy24Stages[len(events)] {
			return nil, errors.New("invalid legacy24 stages")
		}
		events = append(events, event)
	}
	if len(events) != len(legacy24Stages) {
		return nil, errors.New("complete legacy24 evidence required")
	}
	final := events[len(events)-1].Receipt
	if !validLegacy24Completion(final) {
		return nil, errors.New("invalid legacy24 completion")
	}
	for i, event := range events {
		projected := *final
		if i < 7 {
			projected.Scope = ""
		}
		if i < 6 {
			projected.PreservationSHA256, projected.InitializationSHA256, projected.InitializationCount = "", "", 0
		}
		if i < 5 {
			projected.Transition51, projected.Final = nil, nil
		}
		if i < 4 {
			projected.Transition50, projected.Trusted50 = nil, nil
		}
		if i < 3 {
			projected.Transition45, projected.Trusted45 = nil, nil
		}
		if i < 2 {
			projected.Trusted44 = nil
		}
		if i < 1 {
			projected.Trusted25 = nil
		}
		if !reflect.DeepEqual(event.Receipt, &projected) {
			return nil, errors.New("legacy24 lineage changed")
		}
	}
	return final, nil
}

func validLegacy24Completion(r *Legacy24Receipt) bool {
	if r == nil || r.Schema != "voice.space.native-legacy24-upgrade.v1" || r.Origin != "legacy24" || r.Scope != "upgrade-complete" ||
		r.Source24 == nil || r.Trusted25 == nil || r.Trusted25.Target == nil || r.Trusted44 == nil || r.Trusted45 == nil ||
		r.Transition45 == nil || r.Transition50 == nil || r.Trusted50 == nil || r.Transition51 == nil || r.Final == nil ||
		!r.Clean24 || !r.Trusted25.Clean || r.Source24.Version != 24 || r.Source24.DatabaseName != "space_db" || r.Source24.DatabaseOID == 0 ||
		r.Source24.DatabaseSystemID == "" || r.Source24.InstallerRole == "" || r.Source24.Epoch != "" || r.Source24.SealSHA256 != "" ||
		r.Trusted25.Target.Version != 25 || r.Trusted25.Target.Epoch != "" || r.Trusted25.Target.SealSHA256 != "" ||
		r.Catalog24SHA256 != legacy24CatalogPG16SHA256 || r.Trusted25.CatalogSHA256 != legacy25CatalogPG16SHA256 ||
		!sameDatabase(r.Source24, r.Trusted25.Target) || !sameDatabase(r.Source24, r.Trusted44) ||
		r.Trusted25.MigrationSHA256 != selectedSQL["000025_tree_audit_actions.up.sql"] || r.Trusted25.PreimageSHA256 != r.PreimageSHA256 || r.Trusted25.AccessSHA256 != r.AccessSHA256 ||
		r.InitializationCount != r.SpaceCount || !sameTarget(r.Trusted44, r.Trusted45) || !sameTarget(r.Trusted44, r.Trusted50) ||
		!sameTarget(r.Trusted44, r.Final) || r.Trusted44.Version != 44 || r.Trusted45.Version != 45 || r.Trusted50.Version != 50 || r.Final.Version != 51 ||
		r.Trusted44.Epoch != "94db7e08-e958-44fa-94b2-988410ab0044" || !matchesTransition(r.Trusted44, r.Transition45) ||
		r.Trusted45.SealSHA256 != r.Transition45.NewSealSHA256 || !matchesTransition(r.Trusted45, (*store.NativeMembershipCatalog45Receipt)(r.Transition50)) ||
		r.Trusted50.SealSHA256 != r.Transition50.NewSealSHA256 || !matchesTransition(r.Trusted50, &r.Transition51.NativeMembershipCatalog45Receipt) ||
		r.Final.SealSHA256 != r.Transition51.NewSealSHA256 || r.Transition51.Profile != store.NativeCatalogRowJSONProfile ||
		r.Transition51.Engine != "github.com/golang-migrate/migrate/v4@v4.18.1" || r.Transition51.MigrationSHA256 != store.NativeCatalog51MigrationSHA256 ||
		r.Transition45.MigrationSHA256 != selectedSQL["000045_join_legacy_selection_context.up.sql"] ||
		r.Transition50.MigrationSHA256 != selectedSQL["000050_account_ban_coverage.up.sql"] {
		return false
	}
	if parsed, err := uuid.Parse(r.RunID); err != nil || parsed == uuid.Nil || parsed.String() != r.RunID {
		return false
	}
	if len(r.OperationID) == 0 || len(r.OperationID) > 256 || len(r.SnapshotID) == 0 || len(r.SnapshotID) > 256 {
		return false
	}
	if raw, err := hex.DecodeString(r.SourceRevision); err != nil || len(raw) != 20 || hex.EncodeToString(raw) != r.SourceRevision {
		return false
	}
	for _, xid := range []string{r.Transition45.TransactionID, r.Transition50.TransactionID, r.Transition51.TransactionID} {
		n, err := strconv.ParseUint(xid, 10, 64)
		if err != nil || n == 0 || strconv.FormatUint(n, 10) != xid {
			return false
		}
	}
	for _, digest := range []string{r.SourceSHA256, r.ExecutableSHA256, r.MigratorSHA256, r.InventorySHA256, r.Catalog24SHA256, r.PreimageSHA256, r.AccessSHA256,
		r.Trusted25.CatalogSHA256, r.PreservationSHA256, r.InitializationSHA256, r.Trusted44.SealSHA256, r.Transition45.NewSealSHA256,
		r.Transition50.NewSealSHA256, r.Transition51.NewSealSHA256} {
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != digest {
			return false
		}
	}
	return true
}
