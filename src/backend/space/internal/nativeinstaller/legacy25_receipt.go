package nativeinstaller

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"voice/backend/space/internal/store"
)

// Legacy25Receipt describes an existing target, never a fresh creation. The
// controller must authenticate actual Job/image/backup custody independently.
type Legacy25Receipt struct {
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
	Source25             *store.NativeInstallerSourceFacts       `json:"source25"`
	CatalogSHA256        string                                  `json:"catalog_sha256"`
	PreimageSHA256       string                                  `json:"preimage_sha256"`
	SpaceCount           uint64                                  `json:"space_count"`
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

type legacy25Event struct {
	Event   string           `json:"event"`
	Receipt *Legacy25Receipt `json:"receipt"`
}

var legacy25Stages = [...]string{"source25", "trusted44", "trusted45", "trusted50", "trusted51", "initialized", "upgrade-complete"}

// ReadLegacy25Receipt requires the complete original append-only event stream.
// It does not authenticate controller custody or recover a partial operation.
func ReadLegacy25Receipt(in io.Reader) (*Legacy25Receipt, error) {
	const limit = 1 << 20
	raw, err := io.ReadAll(io.LimitReader(in, limit+1))
	if err != nil || len(raw) > limit {
		return nil, errors.New("invalid legacy upgrade evidence")
	}
	if err = legacy25UniqueJSON(raw); err != nil {
		return nil, errors.New("invalid legacy upgrade evidence")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	events := make([]legacy25Event, 0, len(legacy25Stages))
	for {
		var e legacy25Event
		var item json.RawMessage
		if err = d.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			return nil, errors.New("invalid legacy upgrade evidence")
		}
		if err = legacy25ExactFields(item, reflect.TypeOf(e), 0); err != nil || json.Unmarshal(item, &e) != nil {
			return nil, errors.New("invalid legacy upgrade evidence")
		}
		if len(events) >= len(legacy25Stages) || e.Event != legacy25Stages[len(events)] || e.Receipt == nil {
			return nil, errors.New("invalid legacy upgrade stages")
		}
		events = append(events, e)
	}
	if len(events) != len(legacy25Stages) {
		return nil, errors.New("complete legacy upgrade evidence required")
	}
	final := events[len(events)-1].Receipt
	if !validLegacy25Completion(final) {
		return nil, errors.New("invalid legacy upgrade completion")
	}
	for i, event := range events {
		projected := *final
		if i < 6 {
			projected.Scope = ""
		}
		if i < 5 {
			projected.PreservationSHA256, projected.InitializationSHA256, projected.InitializationCount = "", "", 0
		}
		if i < 4 {
			projected.Transition51, projected.Final = nil, nil
		}
		if i < 3 {
			projected.Transition50, projected.Trusted50 = nil, nil
		}
		if i < 2 {
			projected.Transition45, projected.Trusted45 = nil, nil
		}
		if i < 1 {
			projected.Trusted44 = nil
		}
		if !reflect.DeepEqual(event.Receipt, &projected) {
			return nil, errors.New("legacy upgrade lineage changed")
		}
	}
	return final, nil
}

// encoding/json matches struct field names case-insensitively. Validate the
// exact required JSON tags recursively first, including flattened receipts.
func legacy25ExactFields(raw json.RawMessage, typ reflect.Type, depth int) error {
	if depth > 64 {
		return errors.New("JSON depth")
	}
	for typ.Kind() == reflect.Pointer {
		if bytes.Equal(raw, []byte("null")) {
			return nil
		}
		typ = typ.Elem()
	}
	if bytes.Equal(raw, []byte("null")) {
		return errors.New("null required field")
	}
	if typ.Kind() != reflect.Struct {
		return nil
	}
	fields := make(map[string]reflect.Type)
	var add func(reflect.Type) error
	add = func(t reflect.Type) error {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" {
				continue
			}
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if tag == "-" {
				continue
			}
			if f.Anonymous && tag == "" && f.Type.Kind() == reflect.Struct {
				if err := add(f.Type); err != nil {
					return err
				}
				continue
			}
			if tag == "" {
				tag = f.Name
			}
			if _, exists := fields[tag]; exists {
				return errors.New("ambiguous JSON schema")
			}
			fields[tag] = f.Type
		}
		return nil
	}
	if err := add(typ); err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != len(fields) {
		return errors.New("JSON fields differ")
	}
	for name, fieldType := range fields {
		child, ok := object[name]
		if !ok {
			return errors.New("JSON field missing")
		}
		if err := legacy25ExactFields(child, fieldType, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func legacy25UniqueJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var value func(int) error
	value = func(depth int) error {
		if depth > 64 {
			return errors.New("JSON depth")
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return nil
		}
		switch delim {
		case '{':
			seen := make(map[string]bool)
			for d.More() {
				key, err := d.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] {
					return errors.New("duplicate JSON field")
				}
				seen[name] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid JSON delimiter")
		}
		end, err := d.Token()
		if err != nil || (delim == '{' && end != json.Delim('}')) || (delim == '[' && end != json.Delim(']')) {
			return errors.New("invalid JSON container")
		}
		return nil
	}
	for {
		err := value(0)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func validLegacy25Completion(r *Legacy25Receipt) bool {
	if r == nil || r.Schema != "voice.space.native-legacy25-upgrade.v1" || r.Origin != "legacy25" ||
		r.Scope != "upgrade-complete" || r.Source25 == nil || r.Trusted44 == nil || r.Trusted45 == nil ||
		r.Transition45 == nil || r.Transition50 == nil || r.Trusted50 == nil || r.Transition51 == nil || r.Final == nil ||
		r.Source25.Version != 25 || r.Source25.DatabaseName != "space_db" || r.Source25.DatabaseOID == 0 ||
		r.Source25.DatabaseSystemID == "" || r.Source25.InstallerRole == "" || r.Source25.Epoch != "" || r.Source25.SealSHA256 != "" ||
		r.InitializationCount != r.SpaceCount || !sameDatabase(r.Source25, r.Trusted44) ||
		!sameTarget(r.Trusted44, r.Trusted45) || !sameTarget(r.Trusted44, r.Trusted50) || !sameTarget(r.Trusted44, r.Final) ||
		r.Trusted44.Version != 44 || r.Trusted45.Version != 45 || r.Trusted50.Version != 50 || r.Final.Version != 51 ||
		r.Trusted44.Epoch != "94db7e08-e958-44fa-94b2-988410ab0044" ||
		!matchesTransition(r.Trusted44, r.Transition45) || r.Trusted45.SealSHA256 != r.Transition45.NewSealSHA256 ||
		!matchesTransition(r.Trusted45, (*store.NativeMembershipCatalog45Receipt)(r.Transition50)) ||
		r.Trusted50.SealSHA256 != r.Transition50.NewSealSHA256 ||
		!matchesTransition(r.Trusted50, &r.Transition51.NativeMembershipCatalog45Receipt) ||
		r.Final.SealSHA256 != r.Transition51.NewSealSHA256 || r.Transition51.Profile != store.NativeCatalogRowJSONProfile ||
		r.Transition51.Engine != "github.com/golang-migrate/migrate/v4@v4.18.1" ||
		r.Transition51.MigrationSHA256 != store.NativeCatalog51MigrationSHA256 {
		return false
	}
	if parsed, err := uuid.Parse(r.RunID); err != nil || parsed == uuid.Nil || parsed.String() != r.RunID {
		return false
	}
	if len(r.OperationID) == 0 || len(r.OperationID) > 256 || len(r.SnapshotID) == 0 || len(r.SnapshotID) > 256 {
		return false
	}
	if r.Transition45.MigrationSHA256 != selectedSQL["000045_join_legacy_selection_context.up.sql"] ||
		r.Transition50.MigrationSHA256 != selectedSQL["000050_account_ban_coverage.up.sql"] {
		return false
	}
	for _, xid := range []string{r.Transition45.TransactionID, r.Transition50.TransactionID, r.Transition51.TransactionID} {
		n, err := strconv.ParseUint(xid, 10, 64)
		if err != nil || n == 0 || strconv.FormatUint(n, 10) != xid {
			return false
		}
	}
	if raw, err := hex.DecodeString(r.SourceRevision); err != nil || len(raw) != 20 || hex.EncodeToString(raw) != r.SourceRevision {
		return false
	}
	for _, digest := range []string{r.SourceSHA256, r.ExecutableSHA256, r.MigratorSHA256, r.InventorySHA256,
		r.CatalogSHA256, r.PreimageSHA256, r.PreservationSHA256, r.InitializationSHA256, r.Trusted44.SealSHA256,
		r.Transition45.NewSealSHA256, r.Transition50.NewSealSHA256, r.Transition51.NewSealSHA256} {
		if raw, err := hex.DecodeString(digest); err != nil || len(raw) != 32 || hex.EncodeToString(raw) != digest {
			return false
		}
	}
	return true
}
