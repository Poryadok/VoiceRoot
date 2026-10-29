package nativeinstaller

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func legacy25Snapshots(final *Legacy25Receipt) []legacy25Event {
	names := []string{"source25", "trusted44", "trusted45", "trusted50", "trusted51", "initialized", "upgrade-complete"}
	result := make([]legacy25Event, len(names))
	for i, name := range names {
		raw, _ := json.Marshal(final)
		var r Legacy25Receipt
		_ = json.Unmarshal(raw, &r)
		if i < 6 {
			r.Scope = ""
		}
		if i < 5 {
			r.InitializationSHA256, r.PreservationSHA256, r.InitializationCount = "", "", 0
		}
		if i < 4 {
			r.Transition51, r.Final = nil, nil
		}
		if i < 3 {
			r.Transition50, r.Trusted50 = nil, nil
		}
		if i < 2 {
			r.Transition45, r.Trusted45 = nil, nil
		}
		if i < 1 {
			r.Trusted44 = nil
		}
		result[i] = legacy25Event{Event: name, Receipt: &r}
	}
	return result
}

func TestLegacyReceiptRequiresCompleteAppendOnlyLineage(t *testing.T) {
	fresh := completedFixture()
	fresh.Transition45.MigrationSHA256 = selectedSQL["000045_join_legacy_selection_context.up.sql"]
	fresh.Transition50.MigrationSHA256 = selectedSQL["000050_account_ban_coverage.up.sql"]
	fresh.Transition45.TransactionID, fresh.Transition50.TransactionID, fresh.Transition51.TransactionID = "101", "102", "103"
	origin := *fresh.Trusted44
	origin.Version, origin.Epoch, origin.SealSHA256 = 25, "", ""
	t45 := *fresh.Trusted44
	t45.Version, t45.SealSHA256 = 45, fresh.Transition45.NewSealSHA256
	r := &Legacy25Receipt{Schema: "voice.space.native-legacy25-upgrade.v1", Origin: "legacy25", RunID: fresh.RunID,
		OperationID: "fixture-operation", SnapshotID: "fixture-snapshot",
		SourceRevision: fresh.SourceRevision, SourceSHA256: fresh.SourceSHA256, ExecutableSHA256: fresh.ExecutableSHA256,
		MigratorSHA256: fresh.MigratorSHA256, InventorySHA256: fresh.InventorySHA256, Source25: &origin,
		CatalogSHA256: strings.Repeat("1", 64), PreimageSHA256: strings.Repeat("2", 64), SpaceCount: 1,
		Trusted44: fresh.Trusted44, Transition45: fresh.Transition45, Trusted45: &t45,
		Transition50: fresh.Transition50, Trusted50: fresh.Trusted50, Transition51: fresh.Transition51, Final: fresh.Final,
		PreservationSHA256: strings.Repeat("3", 64), InitializationSHA256: strings.Repeat("4", 64),
		InitializationCount: 1, Scope: "upgrade-complete"}
	encode := func(events []legacy25Event) []byte {
		var b bytes.Buffer
		for _, e := range events {
			require.NoError(t, json.NewEncoder(&b).Encode(e))
		}
		return b.Bytes()
	}
	events := legacy25Snapshots(r)
	valid := encode(events)
	got, err := ReadLegacy25Receipt(bytes.NewReader(valid))
	require.NoError(t, err)
	require.Equal(t, r, got)
	_, err = ReadReceipt(bytes.NewReader(valid))
	require.Error(t, err, "legacy origin must never qualify as fresh creation")
	for name, mutate := range map[string]func([]legacy25Event) []legacy25Event{
		"partial":   func(e []legacy25Event) []legacy25Event { return e[:len(e)-1] },
		"duplicate": func(e []legacy25Event) []legacy25Event { return append(e, e[len(e)-1]) },
		"skipped":   func(e []legacy25Event) []legacy25Event { return append(e[:1], e[2:]...) },
		"mixed run": func(e []legacy25Event) []legacy25Event { e[2].Receipt.RunID = "foreign"; return e },
		"changed prior seal": func(e []legacy25Event) []legacy25Event {
			e[4].Receipt.Trusted44.SealSHA256 = strings.Repeat("f", 64)
			return e
		},
		"premature final": func(e []legacy25Event) []legacy25Event { e[0].Receipt.Final = r.Final; return e },
	} {
		t.Run(name, func(t *testing.T) {
			copyEvents := legacy25Snapshots(r)
			_, err := ReadLegacy25Receipt(bytes.NewReader(encode(mutate(copyEvents))))
			require.Error(t, err)
		})
	}
	_, err = ReadLegacy25Receipt(bytes.NewReader(append(valid, []byte(`{"unknown":true}`)...)))
	require.Error(t, err)
	duplicated := bytes.Replace(valid, []byte(`"origin":"legacy25"`), []byte(`"origin":"legacy25","origin":"legacy25"`), 1)
	_, err = ReadLegacy25Receipt(bytes.NewReader(duplicated))
	require.Error(t, err, "duplicate object fields cannot be ambiguous")
	for _, field := range []string{"receipt", "origin", "database_oid"} {
		bad := bytes.Replace(valid, []byte(`"`+field+`":`), []byte(`"`+strings.ToUpper(field)+`":`), 1)
		_, err = ReadLegacy25Receipt(bytes.NewReader(bad))
		require.Error(t, err, "noncanonical field %s", field)
	}
	for _, mutation := range []string{"sql45", "sql50", "xid45", "xid50", "xid51"} {
		clone := legacy25Snapshots(r)
		for _, e := range clone {
			switch mutation {
			case "sql45":
				if e.Receipt.Transition45 != nil {
					e.Receipt.Transition45.MigrationSHA256 = strings.Repeat("f", 64)
				}
			case "sql50":
				if e.Receipt.Transition50 != nil {
					e.Receipt.Transition50.MigrationSHA256 = strings.Repeat("f", 64)
				}
			case "xid45":
				if e.Receipt.Transition45 != nil {
					e.Receipt.Transition45.TransactionID = ""
				}
			case "xid50":
				if e.Receipt.Transition50 != nil {
					e.Receipt.Transition50.TransactionID = ""
				}
			case "xid51":
				if e.Receipt.Transition51 != nil {
					e.Receipt.Transition51.TransactionID = ""
				}
			}
		}
		_, err = ReadLegacy25Receipt(bytes.NewReader(encode(clone)))
		require.Error(t, err, mutation)
	}
	_, err = ReadLegacy25Receipt(bytes.NewReader(append(valid, bytes.Repeat([]byte(" "), 1<<20)...)))
	require.Error(t, err)
	empty := *r
	empty.SpaceCount, empty.InitializationCount = 0, 0
	emptyData := encode(legacy25Snapshots(&empty))
	_, err = ReadLegacy25Receipt(bytes.NewReader(emptyData))
	require.NoError(t, err)
	for _, field := range []string{"space_count", "initialization_count"} {
		bad := bytes.ReplaceAll(emptyData, []byte(`"`+field+`":0`), []byte(`"`+field+`":null`))
		_, err = ReadLegacy25Receipt(bytes.NewReader(bad))
		require.Error(t, err, "null is not an observed zero: %s", field)
	}
}
