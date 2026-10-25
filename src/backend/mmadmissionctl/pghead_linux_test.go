//go:build linux

package mmadmissionctl

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

func pgSnapshotFixture(scope w.ID, role uint32) Snapshot {
	spec := unitSpec()
	spec.ScopeID = scope
	launch, _ := w.LaunchHash(spec)
	identity := Identity{ScopeID: scope, EnvironmentID: freshID(), InstallationID: freshID(), ControllerID: freshID(), AuthorityIncarnation: freshID(), DatabaseLineageID: freshID()}
	b := w.Binding{ProtocolVersion: 1, EnvironmentID: identity.EnvironmentID, InstallationID: identity.InstallationID, AuthorityIncarnation: identity.AuthorityIncarnation, TransitionID: freshID(), AdmissionGeneration: 1, CredentialCohortID: freshID(), DatabaseBindingHash: w.Hash{1}, SchemaContractHash: w.Hash{2}, SourceFactoryManifestHash: w.Hash{3}, FreshRouteManifestHash: w.Hash{4}, LaunchManifestHash: launch, ExpectedLoginRoleOID: role, ExpectedLoginRoleName: "mm_local_serving"}
	r := w.ArmedReceiptV1{Version: 1, AuthorityScope: w.LocalOwned, ScopeID: scope, ControllerID: identity.ControllerID, TransitionID: b.TransitionID, DatabaseLineageID: identity.DatabaseLineageID, Binding: b, LaunchSpecHash: launch, ClosureEvidenceHash: w.Hash{5}, PreparationEvidenceHash: w.Hash{6}}
	b, _ = w.ReceiptBinding(r)
	root, _ := w.HistoryGenesisHash(b.AdmissionReceiptHash, identity.DatabaseLineageID)
	return Snapshot{Version: 1, Identity: identity, State: w.Armed, Generation: 1, Binding: b, Spec: &spec, ArmedReceipt: &r, ReceiptHistory: []w.ArmedReceiptV1{r}, HistoryRoot: root}
}
func testPGInstaller(t *testing.T, ctx context.Context, root *pgx.Conn, ip string, scope w.ID) {
	t.Helper()
	directory := "/proof/mm-migrations/src/backend/migrations/matchmaking_db"
	files, e := os.ReadDir(directory)
	if e != nil {
		t.Fatal("immutable MM025 migration fixture missing", e)
	}
	var names []string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".up.sql") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)
	if len(names) != 25 {
		t.Fatal("wrong pinned migration set", len(names))
	}
	for _, name := range names {
		raw, e := os.ReadFile(filepath.Join(directory, name))
		if e != nil {
			t.Fatal(e)
		}
		if _, e = root.Exec(ctx, string(raw)); e != nil {
			t.Fatal("migration", name, e)
		}
	}
	if _, e = root.Exec(ctx, `CREATE ROLE mm_local_serving LOGIN PASSWORD 'mm-serving-fixture-only' NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;CREATE ROLE mm_local_controller LOGIN PASSWORD 'mm-controller-fixture-only' NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;GRANT USAGE ON SCHEMA public TO mm_local_serving,mm_local_controller;GRANT SELECT,INSERT,UPDATE ON public.mm_native_admission_head TO mm_local_controller;GRANT SELECT,INSERT ON public.mm_native_admission_receipts,public.mm_native_continuity_records TO mm_local_controller;GRANT SELECT ON public.mm_native_admission_head,public.mm_native_admission_receipts,public.mm_native_continuity_records TO mm_local_serving;`); e != nil {
		t.Fatal(e)
	}
	admin, e := pgx.Connect(ctx, "postgres://mm_local_controller:mm-controller-fixture-only@"+ip+":5432/matchmaking_db?sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	var oid uint32
	if e = root.QueryRow(ctx, `SELECT oid FROM pg_roles WHERE rolname='mm_local_serving'`).Scan(&oid); e != nil {
		t.Fatal(e)
	}
	bad := pgSnapshotFixture(scope, oid+1)
	if e = installMMHead(ctx, admin, bad); e == nil {
		t.Fatal("controller installed receipt whose serving role OID does not match actual login")
	}
	s := pgSnapshotFixture(scope, oid)
	if e = installMMHead(ctx, admin, s); e != nil {
		t.Fatal("canonical exact head install", e)
	}
	if e = installMMHead(ctx, admin, s); e != nil {
		t.Fatal("idempotent exact head retry", e)
	}
	var count int
	if e = root.QueryRow(ctx, `SELECT count(*) FROM public.mm_native_admission_receipts`).Scan(&count); e != nil || count != 1 {
		t.Fatal("receipt replay duplicated history", count, e)
	}
	serving, e := pgx.Connect(ctx, "postgres://mm_local_serving:mm-serving-fixture-only@"+ip+":5432/matchmaking_db?sslmode=disable")
	if e != nil {
		t.Fatal(e)
	}
	defer serving.Close(context.Background())
	if _, e = serving.Exec(ctx, `UPDATE public.mm_native_admission_head SET state='CLOSED'`); e == nil {
		t.Fatal("serving login acquired controller write authority")
	}
	shared, e := serving.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if e != nil {
		t.Fatal(e)
	}
	defer shared.Rollback(context.Background())
	gate := sha256.Sum256([]byte("voice.mm.native-admission.gate.v1\x00"))
	if _, e = shared.Exec(ctx, `SELECT pg_catalog.pg_advisory_xact_lock_shared($1)`, int64(binary.BigEndian.Uint64(gate[:8]))); e != nil {
		t.Fatal(e)
	}
	adminPID := admin.PgConn().PID()
	closed := make(chan error, 1)
	go func() { closed <- closeMMHead(ctx, admin, s.Binding) }()
	waiting := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if e = root.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND NOT granted)`, adminPID).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting {
			break
		}
		select {
		case e := <-closed:
			t.Fatal("close bypassed admitted SQL transaction", e)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !waiting {
		t.Fatal("close never waited on actual stable gate")
	}
	if e = shared.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = <-closed; e != nil {
		t.Fatal(e)
	}
	second := nextPGSnapshot(s)
	if e = installMMHead(ctx, admin, second); e != nil {
		t.Fatal("continuous successor", e)
	}
	if e = closeMMHead(ctx, admin, second.Binding); e != nil {
		t.Fatal(e)
	}
	rollback := nextPGSnapshot(second)
	if e = installMMHead(ctx, admin, rollback); e != nil {
		t.Fatal("compatible continuous rollback", e)
	}
	if e = installMMHead(ctx, admin, s); e == nil {
		t.Fatal("old admission binding restored over newer head")
	}
	var receipts, continuities int
	if e = root.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.mm_native_admission_receipts),(SELECT count(*) FROM public.mm_native_continuity_records)`).Scan(&receipts, &continuities); e != nil || receipts != 3 || continuities != 2 {
		t.Fatal("rollback erased original receipt history", receipts, continuities, e)
	}
	var original []byte
	if e = root.QueryRow(ctx, `SELECT receipt_bytes FROM public.mm_native_admission_receipts WHERE receipt_sha256=$1`, s.Binding.AdmissionReceiptHash[:]).Scan(&original); e != nil {
		t.Fatal("original source receipt missing after rollback", e)
	}
	t.Log("actual close waiter; continuous generations1/2/3 retain genesis+2 links; stale generation denied")
	t.Log("LOCAL_OWNED PG exact receipt/head idempotency and separate-role denial; upstream/launch inputs are explicit fixtures")
}
func nextPGSnapshot(s Snapshot) Snapshot {
	spec := *s.Spec
	spec.ImageDigest = "sha256:" + w.Hash{byte(s.Generation + 20)}.String()
	launch, _ := w.LaunchHash(spec)
	previous := s.Binding.AdmissionReceiptHash
	b := s.Binding
	b.TransitionID = freshID()
	b.AdmissionGeneration++
	b.CredentialCohortID = freshID()
	b.LaunchManifestHash = launch
	b.AdmissionReceiptHash = w.Hash{}
	r := w.ArmedReceiptV1{Version: 1, AuthorityScope: w.LocalOwned, ScopeID: s.Identity.ScopeID, ControllerID: s.Identity.ControllerID, TransitionID: b.TransitionID, PredecessorReceiptHash: &previous, DatabaseLineageID: s.Identity.DatabaseLineageID, Binding: b, LaunchSpecHash: launch, ClosureEvidenceHash: w.Hash{5}, PreparationEvidenceHash: w.Hash{6}, PreviousReceiptHash: &previous}
	b, _ = w.ReceiptBinding(r)
	c := w.ContinuityRecordV1{Version: 1, EnvironmentID: s.Identity.EnvironmentID, InstallationID: s.Identity.InstallationID, AuthorityIncarnation: s.Identity.AuthorityIncarnation, DatabaseLineageID: s.Identity.DatabaseLineageID, PredecessorReceiptHash: previous, SuccessorReceiptHash: b.AdmissionReceiptHash, SupportedSourceVersions: []string{"NATIVE_VOICE_C16_V1_ORIGIN_V1"}, PreviousHistoryRootHash: s.HistoryRoot}
	s.HistoryRoot, _ = w.ContinuityHash(c)
	s.Spec = &spec
	s.State = w.Armed
	s.Binding = b
	s.Generation++
	s.ArmedReceipt = &r
	s.OpenReceipt = nil
	s.ReceiptHistory = append(s.ReceiptHistory, r)
	s.ContinuityHistory = append(s.ContinuityHistory, c)
	return s
}
