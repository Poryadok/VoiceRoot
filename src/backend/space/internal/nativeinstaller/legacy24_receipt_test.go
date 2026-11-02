package nativeinstaller

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func legacy24Fixture() *Legacy24Receipt {
	f := completedFixture()
	f.Transition45.MigrationSHA256 = selectedSQL["000045_join_legacy_selection_context.up.sql"]
	f.Transition50.MigrationSHA256 = selectedSQL["000050_account_ban_coverage.up.sql"]
	f.Transition45.TransactionID, f.Transition50.TransactionID, f.Transition51.TransactionID = "101", "102", "103"
	origin := *f.Trusted44
	origin.Version, origin.Epoch, origin.SealSHA256 = 24, "", ""
	intermediate := origin
	intermediate.Version = 25
	t45 := *f.Trusted44
	t45.Version, t45.SealSHA256 = 45, f.Transition45.NewSealSHA256
	return &Legacy24Receipt{Schema: "voice.space.native-legacy24-upgrade.v1", Origin: "legacy24", RunID: f.RunID,
		OperationID: "fixture-operation", SnapshotID: "fixture-snapshot", SourceRevision: f.SourceRevision, SourceSHA256: f.SourceSHA256,
		ExecutableSHA256: f.ExecutableSHA256, MigratorSHA256: f.MigratorSHA256, InventorySHA256: f.InventorySHA256,
		Source24: &origin, Clean24: true, Catalog24SHA256: legacy24CatalogPG16SHA256, PreimageSHA256: strings.Repeat("2", 64), AccessSHA256: strings.Repeat("6", 64), SpaceCount: 1,
		Trusted25: &Legacy24Intermediate{Target: &intermediate, Clean: true, MigrationSHA256: selectedSQL["000025_tree_audit_actions.up.sql"],
			CatalogSHA256: legacy25CatalogPG16SHA256, PreimageSHA256: strings.Repeat("2", 64), AccessSHA256: strings.Repeat("6", 64)},
		Trusted44: f.Trusted44, Transition45: f.Transition45, Trusted45: &t45, Transition50: f.Transition50,
		Trusted50: f.Trusted50, Transition51: f.Transition51, Final: f.Final, PreservationSHA256: strings.Repeat("4", 64),
		InitializationSHA256: strings.Repeat("5", 64), InitializationCount: 1, Scope: "upgrade-complete"}
}

func legacy24FixtureEvents(final *Legacy24Receipt) []legacy24Event {
	names := []string{"source24", "trusted25", "trusted44", "trusted45", "trusted50", "trusted51", "initialized", "upgrade-complete"}
	result := make([]legacy24Event, len(names))
	for i, name := range names {
		raw, _ := json.Marshal(final)
		var r Legacy24Receipt
		_ = json.Unmarshal(raw, &r)
		if i < 7 {
			r.Scope = ""
		}
		if i < 6 {
			r.PreservationSHA256, r.InitializationSHA256, r.InitializationCount = "", "", 0
		}
		if i < 5 {
			r.Transition51, r.Final = nil, nil
		}
		if i < 4 {
			r.Transition50, r.Trusted50 = nil, nil
		}
		if i < 3 {
			r.Transition45, r.Trusted45 = nil, nil
		}
		if i < 2 {
			r.Trusted44 = nil
		}
		if i < 1 {
			r.Trusted25 = nil
		}
		result[i] = legacy24Event{Event: name, Receipt: &r}
	}
	return result
}

func TestLegacy24ReceiptRequiresGenuineIntermediateAndFullLineage(t *testing.T) {
	encode := func(events []legacy24Event) []byte {
		var b bytes.Buffer
		for _, e := range events {
			require.NoError(t, json.NewEncoder(&b).Encode(e))
		}
		return b.Bytes()
	}
	final := legacy24Fixture()
	raw := encode(legacy24FixtureEvents(final))
	actual, err := ReadLegacy24Receipt(bytes.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, final, actual)
	_, err = ReadReceipt(bytes.NewReader(raw))
	require.Error(t, err)
	_, err = ReadLegacy25Receipt(bytes.NewReader(raw))
	require.Error(t, err)
	for _, kind := range []string{"missing25", "wrong25sql", "changed25oid", "wrongpreimage", "wrongaccess", "source25", "dirty24", "dirty25", "mixedrun", "duplicate", "nullcount", "alias"} {
		t.Run(kind, func(t *testing.T) {
			events := legacy24FixtureEvents(final)
			switch kind {
			case "missing25":
				events = append(events[:1], events[2:]...)
			case "duplicate":
				events = append(events, events[len(events)-1])
			case "mixedrun":
				events[2].Receipt.RunID = "foreign"
			default:
				for _, e := range events {
					switch kind {
					case "wrong25sql":
						if e.Receipt.Trusted25 != nil {
							e.Receipt.Trusted25.MigrationSHA256 = strings.Repeat("f", 64)
						}
					case "changed25oid":
						if e.Receipt.Trusted25 != nil {
							e.Receipt.Trusted25.Target.DatabaseOID++
						}
					case "wrongpreimage":
						if e.Receipt.Trusted25 != nil {
							e.Receipt.Trusted25.PreimageSHA256 = strings.Repeat("f", 64)
						}
					case "wrongaccess":
						if e.Receipt.Trusted25 != nil {
							e.Receipt.Trusted25.AccessSHA256 = strings.Repeat("f", 64)
						}
					case "source25":
						e.Receipt.Source24.Version = 25
					case "dirty24":
						e.Receipt.Clean24 = false
					case "dirty25":
						if e.Receipt.Trusted25 != nil {
							e.Receipt.Trusted25.Clean = false
						}
					}
				}
			}
			candidate := encode(events)
			if kind == "nullcount" {
				candidate = bytes.ReplaceAll(candidate, []byte(`"space_count":1`), []byte(`"space_count":null`))
			}
			if kind == "alias" {
				candidate = bytes.Replace(candidate, []byte(`"origin":`), []byte(`"Origin":`), 1)
			}
			_, err := ReadLegacy24Receipt(bytes.NewReader(candidate))
			require.Error(t, err)
		})
	}
}
