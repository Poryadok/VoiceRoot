//go:build integration

package nativeinstaller

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/space/internal/store"
)

// Uses an owned private server only; it never discovers or resets a shared DB.
func TestLegacy24Exact25TransitionPreservesSourceRows(t *testing.T) {
	adminURL, driver := os.Getenv("NATIVE_INSTALLER_TEST_ADMIN_URL"), os.Getenv("NATIVE_INSTALLER_TEST_MIGRATE")
	require.NotEmpty(t, adminURL)
	require.NotEmpty(t, driver)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	_, err = admin.Exec(ctx, `CREATE DATABASE space_db`)
	require.NoError(t, err)
	var oid uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&oid))
	t.Cleanup(func() {
		c, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		var actual uint32
		require.NoError(t, admin.QueryRow(c, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&actual))
		require.Equal(t, oid, actual)
		_, e := admin.Exec(c, `DROP DATABASE space_db`)
		require.NoError(t, e)
	})
	target, err := url.Parse(adminURL)
	require.NoError(t, err)
	target.Path = "/space_db"
	migrate := func(version string) {
		cmd := exec.CommandContext(ctx, driver, "-path", "../../../migrations/space_db", "-database", target.String(), "goto", version)
		output, e := cmd.CombinedOutput()
		require.NoError(t, e, "pinned migrate failed: %s", output)
	}
	migrate("24")
	conn, err := pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	const space = "11111111-1111-4111-8111-111111111111"
	const secondSpace = "55555555-5555-4555-8555-555555555555"
	const owner = "22222222-2222-4222-8222-222222222222"
	const member = "33333333-3333-4333-8333-333333333333"
	const member2 = "44444444-4444-4444-8444-444444444444"
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES($1,'legacy fixture',$2)`, space, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.spaces(id,name,owner_profile_id) VALUES($1,'empty legacy fixture',$2)`, secondSpace, owner)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id,nickname) VALUES($1,$2,'member fixture')`, space, member)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `INSERT INTO public.space_members(space_id,profile_id,nickname) VALUES($1,$2,'second fixture')`, space, member2)
	require.NoError(t, err)
	oldRevision, oldBuild, oldDriver := SourceRevision, BuildSourceSHA256, MigrateSHA256
	t.Cleanup(func() { SourceRevision, BuildSourceSHA256, MigrateSHA256 = oldRevision, oldBuild, oldDriver })
	SourceRevision, BuildSourceSHA256 = strings.Repeat("a", 40), strings.Repeat("b", 64)
	MigrateSHA256, err = fileSHA(driver)
	require.NoError(t, err)
	var inspect bytes.Buffer
	require.NoError(t, InspectLegacy24(ctx, Config{AdminURL: adminURL, MigrationsDir: "../../../migrations/space_db", MigratePath: driver, Output: &inspect}))
	require.Contains(t, inspect.String(), `"event":"inspected-legacy24"`)
	require.NotContains(t, inspect.String(), "legacy fixture")
	require.NotContains(t, inspect.String(), space)
	var inspection struct {
		Event            string `json:"event"`
		Schema           string `json:"schema"`
		SourceRevision   string `json:"source_revision"`
		BuildSHA256      string `json:"build_sha256"`
		ExecutableSHA256 string `json:"executable_sha256"`
		MigratorSHA256   string `json:"migrator_sha256"`
		InventorySHA256  string `json:"inventory_sha256"`
		Target           struct {
			DatabaseName     string `json:"database_name"`
			DatabaseOID      uint32 `json:"database_oid"`
			DatabaseSystemID string `json:"database_system_id"`
			DatabaseOwner    string `json:"database_owner"`
			InstallerRole    string `json:"installer_role"`
			Version          int64  `json:"version"`
		} `json:"target"`
	}
	decoder := json.NewDecoder(bytes.NewReader(inspect.Bytes()))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&inspection))
	require.Equal(t, "voice.space.native-legacy24-inspection.v1", inspection.Schema)
	require.Equal(t, SourceRevision, inspection.SourceRevision)
	require.Equal(t, BuildSourceSHA256, inspection.BuildSHA256)
	require.Equal(t, "space_db", inspection.Target.DatabaseName)
	require.Equal(t, oid, inspection.Target.DatabaseOID)
	require.EqualValues(t, 24, inspection.Target.Version)
	require.NotEmpty(t, inspection.Target.DatabaseSystemID)
	var actualOwner string
	require.NoError(t, conn.QueryRow(ctx, `SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()`).Scan(&actualOwner))
	require.Equal(t, actualOwner, inspection.Target.DatabaseOwner)
	require.NotEmpty(t, inspection.Target.InstallerRole)
	var stillVersion int
	var stillDirty bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&stillVersion, &stillDirty))
	require.Equal(t, 24, stillVersion)
	require.False(t, stillDirty)
	dataHash := func() string {
		var value string
		require.NoError(t, conn.QueryRow(ctx, `SELECT encode(sha256(convert_to(jsonb_build_object('spaces',(SELECT jsonb_agg(to_jsonb(s) ORDER BY s.id) FROM public.spaces s),'members',(SELECT jsonb_agg(to_jsonb(m) ORDER BY m.space_id,m.profile_id) FROM public.space_members m))::text,'UTF8')),'hex')`).Scan(&value))
		return value
	}
	var version int
	var dirty bool
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 24, version)
	require.False(t, dirty)
	catalog, preimage, err := captureLegacy24Snapshot(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, legacy24CatalogPG16SHA256, catalog)
	_, err = conn.Exec(ctx, `CREATE SCHEMA legacy_extra; CREATE TABLE legacy_extra.note(id integer NOT NULL, body text NOT NULL); INSERT INTO legacy_extra.note VALUES(1,'preserve')`)
	require.NoError(t, err)
	origin, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, oid, origin.Target.DatabaseOID)
	require.EqualValues(t, 24, origin.Target.Version)
	require.NotEmpty(t, origin.Target.DatabaseSystemID)
	require.Equal(t, preimage, origin.Rows)
	require.Len(t, origin.PreimageSHA256, 64)
	require.Len(t, origin.AccessSHA256, 64)
	identity := &Legacy24Receipt{Source24: origin.Target, Clean24: true, Catalog24SHA256: origin.CatalogSHA256, PreimageSHA256: origin.PreimageSHA256, AccessSHA256: origin.AccessSHA256, SpaceCount: uint64(len(origin.Spaces))}
	var sidecar bytes.Buffer
	require.NoError(t, writeLegacy24Origin(&sidecar, origin, identity))
	readOrigin, err := readLegacy24Origin(bytes.NewReader(sidecar.Bytes()), identity)
	require.NoError(t, err)
	require.Equal(t, origin.PreimageSHA256, readOrigin.PreimageSHA256)
	badIdentity := *identity
	badIdentity.PreimageSHA256 = strings.Repeat("0", 64)
	_, err = readLegacy24Origin(bytes.NewReader(sidecar.Bytes()), &badIdentity)
	require.Error(t, err, "sidecar must remain bound to the original receipt")
	_, err = readLegacy24Origin(bytes.NewReader(append(sidecar.Bytes(), ' ')), identity)
	require.Error(t, err, "noncanonical or trailing private bytes are not accepted")
	aliased := bytes.Replace(sidecar.Bytes(), []byte(`"target":`), []byte(`"Target":`), 1)
	_, err = readLegacy24Origin(bytes.NewReader(aliased), identity)
	require.Error(t, err, "case aliases are not accepted")
	nullCount := bytes.Replace(sidecar.Bytes(), []byte(`"count":0`), []byte(`"count":null`), 1)
	require.NotEqual(t, sidecar.Bytes(), nullCount)
	_, err = readLegacy24Origin(bytes.NewReader(nullCount), identity)
	require.Error(t, err, "null is not an observed zero count")
	withExtra, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err, "independent ordinary table should be supported")
	_, err = conn.Exec(ctx, `UPDATE legacy_extra.note SET body='changed' WHERE id=1`)
	require.NoError(t, err)
	changedExtra, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.NotEqual(t, withExtra.PreimageSHA256, changedExtra.PreimageSHA256, "independent table contents belong to the preimage")
	_, err = conn.Exec(ctx, `UPDATE legacy_extra.note SET body='preserve' WHERE id=1; CREATE VIEW legacy_extra.unsupported AS SELECT * FROM legacy_extra.note`)
	require.NoError(t, err)
	_, err = observeLegacy24Origin(ctx, conn)
	require.Error(t, err, "unsupported external view must HOLD")
	inspect.Reset()
	require.Error(t, InspectLegacy24(ctx, Config{AdminURL: adminURL, MigrationsDir: "../../../migrations/space_db", MigratePath: driver, Output: &inspect}), "inspection must reject unsupported extras")
	require.Empty(t, inspect.String())
	_, err = conn.Exec(ctx, `DROP VIEW legacy_extra.unsupported`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE SCHEMA native_membership_guard`)
	require.NoError(t, err)
	_, err = observeLegacy24Origin(ctx, conn)
	require.Error(t, err, "a later native schema must not be accepted as original24")
	_, err = conn.Exec(ctx, `DROP SCHEMA native_membership_guard`)
	require.NoError(t, err)
	aclTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = aclTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; GRANT SELECT ON public.spaces TO PUBLIC`)
	require.NoError(t, err)
	changedAccess, err := captureLegacyAccessTx(ctx, aclTx)
	require.NoError(t, err)
	require.NotEqual(t, origin.AccessSHA256, changedAccess, "grant drift changes private access observation")
	require.NoError(t, aclTx.Rollback(ctx))
	columnTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = columnTx.Exec(ctx, `GRANT SELECT(name) ON public.spaces TO PUBLIC`)
	require.NoError(t, err)
	columnAccess, err := captureLegacyAccessTx(ctx, columnTx)
	require.NoError(t, err)
	require.NotEqual(t, origin.AccessSHA256, columnAccess, "column grants change the private access observation")
	stableColumnAccess, err := captureLegacy24StableAccessTx(ctx, columnTx, origin.Rows)
	require.NoError(t, err)
	require.NotEqual(t, origin.StableAccessSHA256, stableColumnAccess, "original column rights drift is detected after migration too")
	require.NoError(t, columnTx.Rollback(ctx))
	functionTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = functionTx.Exec(ctx, `REVOKE ALL ON FUNCTION public.space_voice_access_outbox_immutable() FROM PUBLIC`)
	require.NoError(t, err)
	_, err = captureLegacy24FunctionsTx(ctx, functionTx, origin.Functions)
	require.Error(t, err, "original function privilege drift must not qualify")
	require.NoError(t, functionTx.Rollback(ctx))
	publicTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = publicTx.Exec(ctx, `GRANT CREATE ON SCHEMA public TO PUBLIC`)
	require.NoError(t, err)
	require.Error(t, qualifyLegacy24InstallerTx(ctx, publicTx), "public schema CREATE must not be admitted")
	require.NoError(t, publicTx.Rollback(ctx))
	lifecycleTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = lifecycleTx.Exec(ctx, `INSERT INTO public.space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation) VALUES($1,'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa','PURGED',1)`, space)
	require.NoError(t, err)
	require.Error(t, qualifyLegacy24LifecycleTx(ctx, lifecycleTx), "a present retired Space cannot be initialized")
	require.NoError(t, lifecycleTx.Rollback(ctx))
	sequenceTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = sequenceTx.Exec(ctx, `CREATE SEQUENCE public.legacy_fixture_sequence`)
	require.NoError(t, err)
	sequenceBefore, err := captureLegacyAccessTx(ctx, sequenceTx)
	require.NoError(t, err)
	_, err = sequenceTx.Exec(ctx, `SELECT setval('public.legacy_fixture_sequence',500,false)`)
	require.NoError(t, err)
	sequenceUncalled, err := captureLegacyAccessTx(ctx, sequenceTx)
	require.NoError(t, err)
	require.NotEqual(t, sequenceBefore, sequenceUncalled, "uncalled setval changes the next sequence value")
	_, err = sequenceTx.Exec(ctx, `SELECT nextval('public.legacy_fixture_sequence')`)
	require.NoError(t, err)
	sequenceAfter, err := captureLegacyAccessTx(ctx, sequenceTx)
	require.NoError(t, err)
	require.NotEqual(t, sequenceUncalled, sequenceAfter, "sequence advancement changes the private observation")
	require.NoError(t, sequenceTx.Rollback(ctx))
	restoredAccess, err := observeLegacy24Origin(ctx, conn)
	require.NoError(t, err)
	require.Equal(t, origin.AccessSHA256, restoredAccess.AccessSHA256)
	riTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = riTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`)
	require.NoError(t, err)
	_, err = inspectLegacyCatalogTx(ctx, riTx, 24, legacy24CatalogPG16SHA256)
	require.NoError(t, err, "fixture must qualify before trigger drift")
	var relation, trigger string
	require.NoError(t, riTx.QueryRow(ctx, `SELECT c.relname,t.tgname FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND t.tgisinternal AND t.tgconstraint<>0 ORDER BY c.relname,t.tgname LIMIT 1`).Scan(&relation, &trigger))
	_, err = riTx.Exec(ctx, `ALTER TABLE `+pgx.Identifier{"public", relation}.Sanitize()+` DISABLE TRIGGER `+pgx.Identifier{trigger}.Sanitize())
	require.NoError(t, err)
	_, err = inspectLegacyCatalogTx(ctx, riTx, 24, legacy24CatalogPG16SHA256)
	require.Error(t, err, "disabled internal constraint trigger cannot qualify")
	require.NoError(t, riTx.Rollback(ctx))
	typeTx, err := conn.Begin(ctx)
	require.NoError(t, err)
	_, err = typeTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`)
	require.NoError(t, err)
	_, err = inspectLegacyCatalogTx(ctx, typeTx, 24, legacy24CatalogPG16SHA256)
	require.NoError(t, err)
	_, err = typeTx.Exec(ctx, `CREATE TYPE public.legacy_extra AS (source public.spaces)`)
	require.NoError(t, err)
	_, err = inspectLegacyCatalogTx(ctx, typeTx, 24, legacy24CatalogPG16SHA256)
	require.Error(t, err, "public composite depending on source row type must HOLD")
	require.NoError(t, typeTx.Rollback(ctx))
	_, err = inspectLegacy25Catalog(ctx, conn)
	require.Error(t, err, "24 must never qualify as original25")
	_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=true`)
	require.NoError(t, err)
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "dirty source cannot be admitted")
	_, err = conn.Exec(ctx, `UPDATE public.schema_migrations SET dirty=false`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `ALTER TABLE public.spaces DISABLE TRIGGER space_authority_revision_spaces`)
	require.NoError(t, err)
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "disabled authority trigger cannot be admitted")
	_, err = conn.Exec(ctx, `ALTER TABLE public.spaces ENABLE TRIGGER space_authority_revision_spaces`)
	require.NoError(t, err)
	before := dataHash()
	require.NoError(t, conn.Close(ctx))
	migrate("25")
	conn, err = pgx.Connect(ctx, target.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	intermediate, err := verifyLegacy24Intermediate(ctx, conn, readOrigin)
	require.NoError(t, err)
	require.EqualValues(t, 25, intermediate.Target.Version)
	require.Equal(t, origin.Target.DatabaseOID, intermediate.Target.DatabaseOID)
	wrongOrigin := *origin
	wrongTarget := *origin.Target
	wrongTarget.DatabaseOID++
	wrongOrigin.Target = &wrongTarget
	_, err = verifyLegacy24Intermediate(ctx, conn, &wrongOrigin)
	require.Error(t, err, "a different original database cannot qualify")
	wrongOrigin = *origin
	wrongOrigin.Rows = make(map[string]legacy25TableDigest, len(origin.Rows))
	for name, value := range origin.Rows {
		wrongOrigin.Rows[name] = value
	}
	wrongOrigin.Rows["spaces"] = legacy25TableDigest{}
	_, err = verifyLegacy24Intermediate(ctx, conn, &wrongOrigin)
	require.Error(t, err, "source row drift cannot qualify")
	require.NoError(t, conn.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 25, version)
	require.False(t, dirty)
	require.Equal(t, before, dataHash())
	tx, err := beginLegacySourceRead(ctx, conn)
	require.NoError(t, err)
	_, err = inspectLegacy25CatalogTx(ctx, tx)
	require.NoError(t, err, "actual intermediate25 structure")
	postimage, err := captureLegacy25ProjectionTx(ctx, tx, preimage)
	require.NoError(t, err)
	access, err := captureLegacyAccessTx(ctx, tx)
	require.NoError(t, err)
	require.Equal(t, origin.AccessSHA256, access, "source owners, ACLs and database properties survive 25")
	require.NoError(t, tx.Commit(ctx))
	_, _, err = captureLegacy24Snapshot(ctx, conn)
	require.Error(t, err, "25 must not be relabelled as original24")
	require.Equal(t, preimage, postimage, "all original table rows and columns survive the exact 24-to-25 transition")
	var observed uint32
	require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_db'`).Scan(&observed))
	require.Equal(t, oid, observed)
	t.Run("canonical44-member-derived-effects", func(t *testing.T) {
		var beforeEpoch int64
		require.NoError(t, conn.QueryRow(ctx, `SELECT access_epoch FROM public.space_voice_access_epochs WHERE space_id=$1`, space).Scan(&beforeEpoch))
		var priorID, priorRow string
		require.NoError(t, conn.QueryRow(ctx, `SELECT event_id::text,to_jsonb(o)::text FROM public.space_voice_access_outbox o WHERE space_id=$1 ORDER BY access_epoch LIMIT 1`, space).Scan(&priorID, &priorRow))
		require.NoError(t, conn.Close(ctx))
		migrate("44")
		conn, err = pgx.Connect(ctx, target.String())
		require.NoError(t, err)
		checkTx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		_, err = checkTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`)
		require.NoError(t, err)
		require.NoError(t, verifyLegacy24DerivedTx(ctx, checkTx, preimage, origin.Derived))
		wrong := *origin.Derived
		wrong.Events = make(map[string]legacy24OldEvent, len(origin.Derived.Events))
		for id, event := range origin.Derived.Events {
			wrong.Events[id] = event
		}
		wrong.Events[priorID] = legacy24OldEvent{Hash: strings.Repeat("f", 64)}
		require.Error(t, verifyLegacy24DerivedTx(ctx, checkTx, preimage, &wrong), "altered prior event cannot be preserved")
		require.NoError(t, checkTx.Commit(ctx))
		driftTx, err := conn.Begin(ctx)
		require.NoError(t, err)
		_, err = driftTx.Exec(ctx, `ALTER TABLE public.space_voice_access_outbox DISABLE TRIGGER space_voice_access_outbox_no_change`)
		require.NoError(t, err)
		_, err = driftTx.Exec(ctx, `UPDATE public.space_voice_access_outbox SET created_at=created_at + interval '1 second' WHERE space_id=$1 AND access_epoch=$2`, space, beforeEpoch+1)
		require.NoError(t, err)
		require.Error(t, verifyLegacy24DerivedTx(ctx, driftTx, preimage, origin.Derived), "altered derived event timestamp cannot qualify")
		require.NoError(t, driftTx.Rollback(ctx))
		after, err := captureLegacy25Projection(ctx, conn, preimage)
		require.NoError(t, err)
		for table, old := range preimage {
			if table == "space_voice_access_epochs" || table == "space_voice_access_outbox" {
				continue
			}
			require.Equal(t, old, after[table], "unchanged original table %s", table)
		}
		var epoch int64
		require.NoError(t, conn.QueryRow(ctx, `SELECT access_epoch FROM public.space_voice_access_epochs WHERE space_id=$1`, space).Scan(&epoch))
		require.Equal(t, beforeEpoch+2, epoch, "each legacy member backfill increments its source floor once")
		var priorAfter string
		require.NoError(t, conn.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM public.space_voice_access_outbox o WHERE event_id=$1`, priorID).Scan(&priorAfter))
		require.Equal(t, priorRow, priorAfter, "original outbox event is retained exactly")
		var newCount int
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM public.space_voice_access_outbox WHERE space_id=$1 AND access_epoch>$2 AND access_epoch<=$3 AND event_kind='voice_room_access_invalidated' AND voice_room_id IS NULL AND profile_id IS NULL AND payload=jsonb_build_object('space_id',$1::text,'access_epoch',access_epoch)`, space, beforeEpoch, epoch).Scan(&newCount))
		require.Equal(t, 2, newCount)
		pool, err := pgxpool.New(ctx, target.String())
		require.NoError(t, err)
		defer pool.Close()
		s := &store.SpaceStore{Pool: pool}
		trusted44, err := s.InspectNativeInstallerSource(ctx, 44)
		require.NoError(t, err, "the real populated migration must qualify as native44")
		require.Equal(t, origin.Target.DatabaseOID, trusted44.DatabaseOID)
		preserved44, err := verifyLegacy24Preserved(ctx, conn, readOrigin)
		require.NoError(t, err)
		migrate("45")
		transition45, err := s.TransitionNativeMembershipCatalog45(ctx)
		require.NoError(t, err)
		require.True(t, matchesTransition(trusted44, transition45))
		trusted45, err := s.InspectNativeInstallerSource(ctx, 45)
		require.NoError(t, err)
		require.Equal(t, transition45.NewSealSHA256, trusted45.SealSHA256)
		migrate("50")
		transition50, err := s.TransitionNativeMembershipCatalog50(ctx)
		require.NoError(t, err)
		require.True(t, matchesTransition(trusted45, (*store.NativeMembershipCatalog45Receipt)(transition50)))
		trusted50, err := s.InspectNativeInstallerSource(ctx, 50)
		require.NoError(t, err)
		require.Equal(t, transition50.NewSealSHA256, trusted50.SealSHA256)
		transition51, err := runNativeCatalog51(ctx, s, trusted50, "../../../migrations/space_db", func(context.Context) error { return nil })
		require.NoError(t, err)
		require.True(t, matchesTransition(trusted50, &transition51.NativeMembershipCatalog45Receipt))
		final, err := s.InspectNativeInstallerSource(ctx, 51)
		require.NoError(t, err)
		require.Equal(t, transition51.NewSealSHA256, final.SealSHA256)
		require.Equal(t, []string{space, secondSpace}, origin.Spaces)
		for _, id := range origin.Spaces {
			require.NoError(t, s.InitializeNativeMembership(ctx, uuid.MustParse(id)))
		}
		var initialized int
		require.NoError(t, conn.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.spaces WHERE state='COMPLETE' AND space_id=ANY($1::uuid[])`, origin.Spaces).Scan(&initialized))
		require.Equal(t, len(origin.Spaces), initialized, "every original Space is initialized")
		finalTx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		_, err = finalTx.Exec(ctx, `SET LOCAL search_path=pg_catalog; SET LOCAL TIME ZONE 'UTC'; SET LOCAL DateStyle='ISO, YMD'`)
		require.NoError(t, err)
		require.NoError(t, verifyLegacy24DerivedTx(ctx, finalTx, preimage, origin.Derived))
		finalExtras, err := captureLegacy24ExtrasTx(ctx, finalTx)
		require.NoError(t, err)
		require.Equal(t, origin.Extras, finalExtras, "independent table is preserved through initialization")
		require.NoError(t, finalTx.Commit(ctx))
		preserved, err := verifyLegacy24Preserved(ctx, conn, readOrigin)
		require.NoError(t, err)
		require.Len(t, preserved, 64)
		require.Equal(t, preserved44, preserved)
		pool.Close()
		require.NoError(t, conn.Close(ctx))
		_, err = admin.Exec(ctx, `CREATE DATABASE space_other TEMPLATE space_db`)
		require.NoError(t, err)
		var otherOID uint32
		require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_other'`).Scan(&otherOID))
		otherURL := *target
		otherURL.Path = "/space_other"
		other, err := pgx.Connect(ctx, otherURL.String())
		require.NoError(t, err)
		_, err = verifyLegacy24Preserved(ctx, other, readOrigin)
		require.Error(t, err, "matching cloned contents in another database do not prove the original target")
		require.NoError(t, other.Close(ctx))
		var observedOther uint32
		require.NoError(t, admin.QueryRow(ctx, `SELECT oid FROM pg_database WHERE datname='space_other'`).Scan(&observedOther))
		require.Equal(t, otherOID, observedOther)
		_, err = admin.Exec(ctx, `DROP DATABASE space_other`)
		require.NoError(t, err)
	})
}
