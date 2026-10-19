//go:build linux

package fencecustody

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/partycoverage"
	"voice/backend/voice/internal/retirementproof"
)

type custodyMetadata struct {
	DatabaseIdentity  [32]byte
	FencerOID, OldOID uint32
}

func TestSeparatedPostgresFencerCustody(t *testing.T) {
	// Installer-only probe, executed as the PostgreSQL OS principal in its own
	// container. It is test code, not a production fencer command/SQL escape hatch.
	if os.Getenv("FENCE_CUSTODY_METADATA_PROBE") == "1" {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, "host=/var/run/postgresql user=postgres dbname=postgres")
		require.NoError(t, err)
		defer c.Close(ctx)
		tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		require.NoError(t, err)
		var m custodyMetadata
		m.DatabaseIdentity, err = partycoverage.ObserveDatabaseIdentity(ctx, tx)
		require.NoError(t, err)
		require.NoError(t, tx.QueryRow(ctx, `SELECT (SELECT oid FROM pg_roles WHERE rolname='fencer_admin'),(SELECT oid FROM pg_roles WHERE rolname='old_writer')`).Scan(&m.FencerOID, &m.OldOID))
		require.NoError(t, tx.Commit(ctx))
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		fmt.Printf("CUSTODY_METADATA:%s\n", raw)
		return
	}
	if testing.Short() {
		t.Skip("owned Docker process/Unix-peer custody integration")
	}
	if os.Geteuid() != 0 || os.Getenv("FENCE_CUSTODY_TEST_BINARY") == "" || os.Getenv("PARTY_COVERAGE_TEST_NETWORK") == "" {
		t.Skip("requires explicit owned Linux installer runner, native fencer and private network")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	api, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err)
	defer api.Close()
	suffix := uuid.NewString()[:8]
	labels := map[string]string{"voice.swarm.owner": "rudeus191", "voice.fence-custody.fixture": suffix}
	var containers, volumes []string
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 40*time.Second)
		defer stop()
		for i := len(containers) - 1; i >= 0; i-- {
			require.NoError(t, api.ContainerRemove(cleanup, containers[i], container.RemoveOptions{Force: true, RemoveVolumes: true}))
		}
		for _, name := range volumes {
			require.NoError(t, api.VolumeRemove(cleanup, name, false))
		}
	})
	newVolume := func(kind string) string {
		v, err := api.VolumeCreate(ctx, volume.CreateOptions{Name: "rudeus191-fencer-" + suffix + "-" + kind, Labels: labels})
		require.NoError(t, err)
		volumes = append(volumes, v.Name)
		return v.Name
	}
	artifact, plans, proofs, observations := newVolume("artifact"), newVolume("plans"), newVolume("proofs"), newVolume("observations")
	locks, secrets, installation, socket := newVolume("locks"), newVolume("secrets"), newVolume("installation"), newVolume("socket")
	vm := func(source, target string, readonly bool) mount.Mount {
		return mount.Mount{Type: mount.TypeVolume, Source: source, Target: target, ReadOnly: readonly}
	}
	newContainer := func(name string, config *container.Config, host *container.HostConfig) string {
		config.Labels = labels
		id, err := api.ContainerCreate(ctx, config, host, nil, nil, "rudeus191-fencer-"+suffix+"-"+name)
		require.NoError(t, err)
		containers = append(containers, id.ID)
		return id.ID
	}
	copyFile := func(id, directory, name string, raw []byte, mode int64) {
		var archive bytes.Buffer
		writer := tar.NewWriter(&archive)
		require.NoError(t, writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(raw)), Uid: 0, Gid: 0, Typeflag: tar.TypeReg}))
		_, err := writer.Write(raw)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		require.NoError(t, api.CopyToContainer(ctx, id, directory, &archive, container.CopyToContainerOptions{}))
	}
	installerMounts := []mount.Mount{vm(artifact, "/artifact", false), vm(plans, installedPaths.Plans, false), vm(proofs, installedPaths.Proofs, false), vm(observations, installedPaths.Observations, false), vm(locks, installedPaths.Locks, false), vm(secrets, installedPaths.Secrets, false), vm(installation, installedPaths.Installation, false)}
	installer := newContainer("installer", &container.Config{Image: "alpine:3.22", Cmd: []string{"sleep", "180"}}, &container.HostConfig{NetworkMode: "none", Mounts: installerMounts})
	binary := os.Getenv("FENCE_CUSTODY_TEST_BINARY")
	require.NotEmpty(t, binary, "build Linux fencer first in the owned runner")
	code, err := os.ReadFile(binary)
	require.NoError(t, err)
	artifactHash := sha256.Sum256(code)
	copyFile(installer, "/artifact", "voice-party-fencer", code, 0555)
	probe, err := os.Executable()
	require.NoError(t, err)
	probeCode, err := os.ReadFile(probe)
	require.NoError(t, err)
	copyFile(installer, "/artifact", "installer-probe", probeCode, 0555)
	oldPassword, otherPassword := uuid.NewString(), uuid.NewString()
	network := os.Getenv("PARTY_COVERAGE_TEST_NETWORK")
	require.NotEmpty(t, network, "use only the existing owned private test network")
	pg := newContainer("postgres", &container.Config{Image: "postgres:16-bookworm", Env: []string{"POSTGRES_PASSWORD=" + uuid.NewString()}, Cmd: []string{"postgres", "-c", "hba_file=/tmp/pg_hba.custody", "-c", "ident_file=/tmp/pg_ident.custody", "-c", "max_prepared_transactions=8"}}, &container.HostConfig{NetworkMode: container.NetworkMode(network), Mounts: []mount.Mount{vm(socket, "/var/run/postgresql", false), vm(artifact, "/opt/installer", true)}})
	copyFile(pg, "/tmp", "pg_hba.custody", []byte("local all all peer map=custody\nhost all postgres 0.0.0.0/0 reject\nhost all fencer_admin 0.0.0.0/0 reject\nhost all all 0.0.0.0/0 scram-sha-256\n"), 0444)
	copyFile(pg, "/tmp", "pg_ident.custody", []byte("custody postgres postgres\ncustody root fencer_admin\n"), 0444)
	setup := fmt.Sprintf("CREATE ROLE fencer_admin SUPERUSER LOGIN;CREATE ROLE old_writer LOGIN PASSWORD '%s';CREATE ROLE other_writer LOGIN PASSWORD '%s';CREATE TABLE native_owner_fixture(id int PRIMARY KEY,actor text);GRANT SELECT,INSERT ON native_owner_fixture TO old_writer,other_writer;INSERT INTO native_owner_fixture VALUES(1,'preserved');", oldPassword, otherPassword)
	copyFile(pg, "/docker-entrypoint-initdb.d", "010-custody.sql", []byte(setup), 0444)
	require.NoError(t, api.ContainerStart(ctx, pg, container.StartOptions{}))
	info, err := api.ContainerInspect(ctx, pg)
	require.NoError(t, err)
	ip := info.NetworkSettings.Networks[network].IPAddress
	connect := func(role, password string) (*pgx.Conn, error) {
		cfg, err := pgx.ParseConfig("host=" + ip + " user=" + role + " dbname=postgres sslmode=disable")
		if err != nil {
			return nil, err
		}
		cfg.Password, cfg.ConnectTimeout = password, time.Second
		return pgx.ConnectConfig(ctx, cfg)
	}
	var old *pgx.Conn
	require.Eventually(t, func() bool { old, err = connect("old_writer", oldPassword); return err == nil }, 30*time.Second, 250*time.Millisecond)
	defer func() { old.Close(context.Background()) }()
	other, err := connect("other_writer", otherPassword)
	require.NoError(t, err)
	defer other.Close(context.Background())
	execute := func(id, user string, args, env []string) (string, int) {
		execID, err := api.ContainerExecCreate(ctx, id, container.ExecOptions{User: user, Cmd: args, Env: env, AttachStdout: true, AttachStderr: true})
		require.NoError(t, err)
		stream, err := api.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
		require.NoError(t, err)
		defer stream.Close()
		var stdout, stderr bytes.Buffer
		_, err = stdcopy.StdCopy(&stdout, &stderr, stream.Reader)
		require.NoError(t, err)
		state, err := api.ContainerExecInspect(ctx, execID.ID)
		require.NoError(t, err)
		return stdout.String() + stderr.String(), state.ExitCode
	}
	metadataOutput, status := execute(pg, "postgres", []string{"/opt/installer/installer-probe", "-test.run=^TestSeparatedPostgresFencerCustody$", "-test.v"}, []string{"FENCE_CUSTODY_METADATA_PROBE=1"})
	require.Zero(t, status, "%s", metadataOutput)
	var metadata custodyMetadata
	for _, line := range strings.Split(metadataOutput, "\n") {
		if strings.HasPrefix(line, "CUSTODY_METADATA:") {
			require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(line, "CUSTODY_METADATA:")), &metadata))
		}
	}
	require.NotZero(t, metadata.FencerOID)
	cut := fixtureCut()
	cut.DatabaseIdentity = metadata.DatabaseIdentity
	command := PreparedPostgresCommand{Protocol: 1, Kind: "RETIRE_POSTGRES_ROLE", CommandID: uuid.New(), Cut: cut, PredecessorExecutionCohortID: uuid.New(), Target: partycoverage.PostgresRoleBinding{DatabaseIdentity: metadata.DatabaseIdentity, RoleOID: metadata.OldOID, RoleName: "old_writer"}, Endpoint: UnixPeerEndpoint{SocketDirectory: "/var/run/postgresql", Database: "postgres", Port: 5432, PeerUID: 999}, CredentialCustodyID: uuid.New(), FencerLogin: LoginIdentity{RoleOID: metadata.FencerOID, RoleName: "fencer_admin"}, FencerArtifactSHA256: artifactHash, TransportBinding: InstallerRootExec}
	commandBytes, err := json.Marshal(command)
	require.NoError(t, err)
	ref := CommandRef{CommandID: command.CommandID, Hash: retirementproof.Hash(CommandDomain, commandBytes)}
	window := InstallationWindow{Protocol: 1, ExecutionID: uuid.New(), Installation: retirementproof.Installation{EnvironmentID: cut.EnvironmentID, AuthorityIncarnation: cut.AuthorityIncarnation, IssuerImplementationManifestHash: cut.IssuerImplementationManifestHash}, CutoverOperationID: cut.CutoverOperationID, CoverageEpoch: cut.CoverageEpoch, PlanHash: cut.PlanHash, ClosedAdmissionBarrierID: cut.ClosedAdmissionBarrierID, ClosedAdmissionBarrierHash: cut.ClosedAdmissionBarrierHash, AdmittedCommands: []CommandRef{ref}}
	windowBytes, err := json.Marshal(window)
	require.NoError(t, err)
	copyFile(installer, installedPaths.Plans, command.CommandID.String()+".json", commandBytes, 0444)
	copyFile(installer, installedPaths.Installation, "current.json", windowBytes, 0444)
	copyFile(installer, installedPaths.Secrets, command.CredentialCustodyID.String()+".dsn", []byte("host=/var/run/postgresql port=5432 dbname=postgres user=fencer_admin"), 0400)
	commonMounts := []mount.Mount{vm(artifact, "/usr/local/bin", true), vm(plans, installedPaths.Plans, true), vm(installation, installedPaths.Installation, true)}
	callIndex := 0
	startCall := func(reader bool, operation string, id uuid.UUID, hash [32]byte, extraEnvironment ...string) string {
		callIndex++
		mounts := append([]mount.Mount(nil), commonMounts...)
		user := "0:0"
		if reader {
			user = "1000:1000"
			mounts = append(mounts, vm(proofs, installedPaths.Proofs, true), vm(observations, installedPaths.Observations, true))
		} else {
			mounts = append(mounts, vm(proofs, installedPaths.Proofs, false), vm(observations, installedPaths.Observations, false), vm(locks, installedPaths.Locks, false), vm(secrets, installedPaths.Secrets, true), vm(socket, "/var/run/postgresql", true))
		}
		idString := newContainer(fmt.Sprintf("call-%d", callIndex), &container.Config{Image: "postgres:16-bookworm", User: user, Entrypoint: []string{"/usr/local/bin/voice-party-fencer"}, Cmd: []string{"-operation", operation, "-id", id.String(), "-hash", hex.EncodeToString(hash[:])}, Env: append([]string{"GOMAXPROCS=1"}, extraEnvironment...)}, &container.HostConfig{NetworkMode: "none", ReadonlyRootfs: true, Mounts: mounts, CapDrop: []string{"ALL"}, SecurityOpt: []string{"no-new-privileges:true"}})
		require.NoError(t, api.ContainerStart(ctx, idString, container.StartOptions{}))
		return idString
	}
	waitCall := func(idString string) (string, int) {
		finished, failed := api.ContainerWait(ctx, idString, container.WaitConditionNotRunning)
		var code int64
		select {
		case result := <-finished:
			code = result.StatusCode
		case err := <-failed:
			require.NoError(t, err)
		case <-ctx.Done():
			t.Fatal("finite custody process timed out")
		}
		log, err := api.ContainerLogs(ctx, idString, container.LogsOptions{ShowStdout: true, ShowStderr: true})
		require.NoError(t, err)
		defer log.Close()
		var stdout, stderr bytes.Buffer
		_, err = stdcopy.StdCopy(&stdout, &stderr, log)
		require.NoError(t, err)
		return stdout.String() + stderr.String(), int(code)
	}
	call := func(reader bool, operation string, id uuid.UUID, hash [32]byte, extraEnvironment ...string) (string, int) {
		return waitCall(startCall(reader, operation, id, hash, extraEnvironment...))
	}
	t.Run("unprivileged process cannot launch fencer", func(t *testing.T) {
		_, status := call(true, "fence", ref.CommandID, ref.Hash)
		require.NotZero(t, status)
	})
	t.Run("wrong command hash no mutation", func(t *testing.T) {
		bad := ref.Hash
		bad[0]++
		_, status := call(false, "fence", ref.CommandID, bad)
		require.NotZero(t, status)
		var login bool
		require.NoError(t, old.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user`).Scan(&login))
		require.True(t, login)
	})
	t.Run("ambient connection override", func(t *testing.T) {
		_, status := call(false, "fence", ref.CommandID, ref.Hash, "PGHOST=/unadmitted-socket")
		require.NotZero(t, status)
		var login bool
		require.NoError(t, old.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user`).Scan(&login))
		require.True(t, login)
	})
	for _, negative := range []struct {
		name   string
		change func(*PreparedPostgresCommand)
	}{
		{"wrong target OID", func(c *PreparedPostgresCommand) { c.Target.RoleOID++ }},
		{"wrong target name", func(c *PreparedPostgresCommand) { c.Target.RoleName = "other_writer" }},
		{"wrong authenticated login OID", func(c *PreparedPostgresCommand) { c.FencerLogin.RoleOID++ }},
		{"wrong actual peer UID", func(c *PreparedPostgresCommand) { c.Endpoint.PeerUID++ }},
		{"wrong binary artifact", func(c *PreparedPostgresCommand) { c.FencerArtifactSHA256[0]++ }},
		{"wrong database lineage", func(c *PreparedPostgresCommand) {
			c.Cut.DatabaseIdentity[0]++
			c.Target.DatabaseIdentity = c.Cut.DatabaseIdentity
		}},
		{"wrong external incarnation", func(c *PreparedPostgresCommand) { c.Cut.AuthorityIncarnation = uuid.New() }},
		{"wrong socket endpoint", func(c *PreparedPostgresCommand) { c.Endpoint.SocketDirectory = "/different-socket" }},
	} {
		t.Run(negative.name, func(t *testing.T) {
			bad := command
			bad.CommandID = uuid.New()
			negative.change(&bad)
			raw, err := json.Marshal(bad)
			require.NoError(t, err)
			badRef := CommandRef{CommandID: bad.CommandID, Hash: retirementproof.Hash(CommandDomain, raw)}
			copyFile(installer, installedPaths.Plans, bad.CommandID.String()+".json", raw, 0444)
			admission := window
			admission.AdmittedCommands = []CommandRef{ref, badRef}
			raw, err = json.Marshal(admission)
			require.NoError(t, err)
			copyFile(installer, installedPaths.Installation, "current.json", raw, 0444)
			_, status := call(false, "fence", badRef.CommandID, badRef.Hash)
			require.NotZero(t, status)
			var login bool
			require.NoError(t, old.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user`).Scan(&login))
			require.True(t, login, "wrong command must not mutate the actual predecessor")
			copyFile(installer, installedPaths.Installation, "current.json", windowBytes, 0444)
		})
	}
	installerSQL := func(sql string) {
		output, status := execute(pg, "postgres", []string{"psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres", "-c", sql}, nil)
		require.Zero(t, status, "%s", output)
	}
	t.Run("prepared predecessor transaction blocks fencing without guessing its outcome", func(t *testing.T) {
		_, err := old.Exec(ctx, `BEGIN; INSERT INTO native_owner_fixture VALUES(4,'prepared predecessor'); PREPARE TRANSACTION 'custody_predecessor'`)
		require.NoError(t, err)
		_, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.NotZero(t, status)
		var login bool
		require.NoError(t, old.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user`).Scan(&login))
		require.True(t, login)
		var prepared int
		require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_xacts WHERE gid='custody_predecessor' AND owner=current_user`).Scan(&prepared))
		require.Equal(t, 1, prepared, "fencer must retain unresolved prepared work")
		installerSQL(`ROLLBACK PREPARED 'custody_predecessor'`)
	})
	t.Run("incoming delegation blocks fencing without revoking authority implicitly", func(t *testing.T) {
		installerSQL(`CREATE ROLE old_delegate NOLOGIN; GRANT old_writer TO old_delegate`)
		_, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.NotZero(t, status)
		var login, delegated bool
		require.NoError(t, old.QueryRow(ctx, `SELECT rolcanlogin,EXISTS(SELECT 1 FROM pg_auth_members WHERE roleid=(SELECT oid FROM pg_roles WHERE rolname='old_writer')) FROM pg_roles WHERE rolname=current_user`).Scan(&login, &delegated))
		require.True(t, login)
		require.True(t, delegated)
		installerSQL(`REVOKE old_writer FROM old_delegate; DROP ROLE old_delegate`)
	})
	t.Run("prepared transaction after precheck blocks final proof", func(t *testing.T) {
		// A retained catalog lock lets the actual fencer finish its read-only
		// prechecks and block at ALTER ROLE. Only this owned installer session
		// controls the barrier; no production callback or timing sleep is used.
		application := "custody-authid-barrier-" + suffix
		execID, err := api.ContainerExecCreate(ctx, pg, container.ExecOptions{User: "postgres", Cmd: []string{"psql", "-X", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres", "-c", "BEGIN; LOCK TABLE pg_authid IN SHARE MODE; SELECT pg_sleep(90)"}, Env: []string{"PGAPPNAME=" + application}, AttachStdout: true, AttachStderr: true})
		require.NoError(t, err)
		barrier, err := api.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
		require.NoError(t, err)
		defer barrier.Close()
		adminBool := func(sql string) bool {
			output, status := execute(pg, "postgres", []string{"psql", "-AtX", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres", "-c", sql}, nil)
			return status == 0 && strings.TrimSpace(output) == "t"
		}
		release := func() {
			installerSQL(fmt.Sprintf("SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE application_name='%s' AND usename='postgres'", application))
		}
		released := false
		defer func() {
			if !released {
				release()
			}
		}()
		require.Eventually(t, func() bool {
			return adminBool(fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE a.application_name='%s' AND l.relation='pg_authid'::regclass AND l.mode='ShareLock' AND l.granted)", application))
		}, 5*time.Second, 50*time.Millisecond)
		pending := startCall(false, "fence", ref.CommandID, ref.Hash)
		require.Eventually(t, func() bool {
			return adminBool("SELECT EXISTS(SELECT 1 FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE a.application_name='voice-party-fencer' AND a.usename='fencer_admin' AND a.query LIKE 'ALTER ROLE %' AND l.relation='pg_authid'::regclass AND NOT l.granted)")
		}, 5*time.Second, 50*time.Millisecond)
		_, err = old.Exec(ctx, `BEGIN; INSERT INTO native_owner_fixture VALUES(5,'raced prepared predecessor'); PREPARE TRANSACTION 'custody_raced_predecessor'`)
		require.NoError(t, err)
		release()
		released = true
		_, status := waitCall(pending)
		require.NotZero(t, status, "postcheck must reject work prepared after successful precheck")
		var login bool
		var prepared int
		require.NoError(t, other.QueryRow(ctx, `SELECT rolcanlogin,(SELECT count(*) FROM pg_prepared_xacts WHERE gid='custody_raced_predecessor' AND owner='old_writer') FROM pg_roles WHERE rolname='old_writer'`).Scan(&login, &prepared))
		require.False(t, login)
		require.Equal(t, 1, prepared, "no guessed prepared-transaction disposition")
		installerSQL(`ROLLBACK PREPARED 'custody_raced_predecessor'`)
		output, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.Zero(t, status, "%s", output)
		installerSQL(`ALTER ROLE old_writer LOGIN`)
		old, err = connect("old_writer", oldPassword)
		require.NoError(t, err)
	})
	paused, err := old.Begin(ctx)
	require.NoError(t, err)
	_, err = paused.Exec(ctx, `INSERT INTO native_owner_fixture VALUES(2,'paused predecessor')`)
	require.NoError(t, err)
	_, err = other.Exec(ctx, `INSERT INTO native_owner_fixture VALUES(3,'other principal')`)
	require.NoError(t, err)
	output, status := call(false, "fence", ref.CommandID, ref.Hash)
	require.Zero(t, status, "%s", output)
	var result FenceResult
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	require.Error(t, paused.Commit(ctx), "actual established predecessor transaction cannot commit")
	fresh, err := connect("old_writer", oldPassword)
	if fresh != nil {
		fresh.Close(ctx)
	}
	require.Error(t, err, "new predecessor login stays denied")
	var count int
	require.NoError(t, other.QueryRow(ctx, `SELECT count(*) FROM native_owner_fixture WHERE id IN(1,3)`).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, other.QueryRow(ctx, `SELECT count(*) FROM native_owner_fixture WHERE id=2`).Scan(&count))
	require.Zero(t, count)
	output, status = call(true, "read-receipt", result.Receipt.ReceiptID, result.Receipt.Hash)
	require.Zero(t, status, "%s", output)
	var retained PostgresFenceReceipt
	require.NoError(t, json.Unmarshal([]byte(output), &retained))
	require.Equal(t, command.Target, retained.Evidence.Binding)
	output, status = call(false, "fence", ref.CommandID, ref.Hash)
	require.Zero(t, status, "%s", output)
	var replay FenceResult
	require.NoError(t, json.Unmarshal([]byte(output), &replay))
	require.Equal(t, result.Receipt, replay.Receipt, "lost ack preserves immutable receipt")
	require.NotEqual(t, result.Observation.ObservationID, replay.Observation.ObservationID)
	output, status = call(true, "read-observation", replay.Observation.ObservationID, replay.Observation.Hash)
	require.Zero(t, status, "%s", output)
	t.Run("re-enabled predecessor cannot reuse historical proof as fresh observation", func(t *testing.T) {
		installerSQL(`ALTER ROLE old_writer LOGIN`)
		resumed, err := connect("old_writer", oldPassword)
		require.NoError(t, err, "fixture administrator actually restored predecessor login")
		defer resumed.Close(context.Background())
		_, status := call(false, "observe", ref.CommandID, ref.Hash)
		require.NotZero(t, status, "fresh resource observer must reject the re-enabled role")
		var login bool
		require.NoError(t, resumed.QueryRow(ctx, `SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user`).Scan(&login))
		require.True(t, login, "observation alone must not mutate credentials")
		output, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.Zero(t, status, "%s", output)
		var recovered FenceResult
		require.NoError(t, json.Unmarshal([]byte(output), &recovered))
		require.Equal(t, result.Receipt, recovered.Receipt)
		require.NotEqual(t, replay.Observation, recovered.Observation)
		require.Error(t, resumed.Ping(ctx), "recovery fencing terminates the actual restored session")
	})
	t.Run("termination timeout retains NOLOGIN and requires fresh recovery", func(t *testing.T) {
		installerSQL(`ALTER ROLE old_writer LOGIN`)
		stopped, err := connect("old_writer", oldPassword)
		require.NoError(t, err)
		defer stopped.Close(context.Background())
		var backendPID int
		require.NoError(t, stopped.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID))
		require.Positive(t, backendPID)
		// Only the owned PostgreSQL container's exact authenticated backend is
		// suspended. This models a real unresponsive process, not a mocked SQL
		// return or a host signal. Installer-only authority resumes it below.
		signal := func(name string) {
			output, status := execute(pg, "postgres", []string{"/bin/sh", "-c", `kill "$1" "$2"`, "custody-backend-signal", name, fmt.Sprint(backendPID)}, nil)
			require.Zero(t, status, "%s", output)
		}
		signal("-STOP")
		resumed := false
		defer func() {
			if !resumed {
				signal("-CONT")
			}
		}()
		_, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.NotZero(t, status, "positive backend termination timeout must fail closed")
		var login, established bool
		require.NoError(t, other.QueryRow(ctx, `SELECT rolcanlogin,EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND usesysid=$2::oid) FROM pg_roles WHERE oid=$2::oid`, backendPID, metadata.OldOID).Scan(&login, &established))
		require.False(t, login, "NOLOGIN must remain committed after partial failure")
		require.True(t, established, "the fixture must actually retain the stopped backend")
		_, status = call(false, "observe", ref.CommandID, ref.Hash)
		require.NotZero(t, status, "historical receipt cannot hide a still-established predecessor")
		signal("-CONT")
		resumed = true
		require.Eventually(t, func() bool {
			var exists bool
			return other.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND usesysid=$2::oid)`, backendPID, metadata.OldOID).Scan(&exists) == nil && !exists
		}, 5*time.Second, 50*time.Millisecond)
		output, status := call(false, "fence", ref.CommandID, ref.Hash)
		require.Zero(t, status, "%s", output)
		var recovered FenceResult
		require.NoError(t, json.Unmarshal([]byte(output), &recovered))
		require.Equal(t, result.Receipt, recovered.Receipt)
		output, status = call(true, "read-observation", recovered.Observation.ObservationID, recovered.Observation.Hash)
		require.Zero(t, status, "%s", output)
	})
	t.Run("old observation after external execution rotation", func(t *testing.T) {
		window.ExecutionID = uuid.New()
		raw, err := json.Marshal(window)
		require.NoError(t, err)
		copyFile(installer, installedPaths.Installation, "current.json", raw, 0444)
		_, status := call(true, "read-observation", replay.Observation.ObservationID, replay.Observation.Hash)
		require.NotZero(t, status)
		output, status := call(false, "observe", ref.CommandID, ref.Hash)
		require.Zero(t, status, "%s", output)
		var current FenceResult
		require.NoError(t, json.Unmarshal([]byte(output), &current))
		require.Equal(t, result.Receipt, current.Receipt)
		output, status = call(true, "read-observation", current.Observation.ObservationID, current.Observation.Hash)
		require.Zero(t, status, "%s", output)
	})
	if evidencePath := os.Getenv("FENCE_CUSTODY_TEST_EVIDENCE"); evidencePath != "" {
		sourceHashes := map[string]string{}
		for _, name := range []string{"types.go", "files_linux.go", "fencer_linux.go", "reader_linux.go", "../../cmd/voice-party-fencer/main_linux.go"} {
			body, err := os.ReadFile(filepath.Clean(name))
			require.NoError(t, err)
			digest := sha256.Sum256(body)
			sourceHashes[name] = hex.EncodeToString(digest[:])
		}
		record := struct {
			Protocol        uint32
			Scope           string
			ArtifactSHA256  [32]byte
			SourceSHA256    map[string]string
			Command         PreparedPostgresCommand
			Window          InstallationWindow
			Receipt         PostgresFenceReceipt
			Initial, Replay FenceResult
		}{1, "owned isolated PG/process/file component only; fixture manifest hashes, no native drain/ACTIVE/production qualification", artifactHash, sourceHashes, command, window, retained, result, replay}
		raw, err := json.Marshal(record)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(evidencePath, raw, 0644))
	}
	// These are real PG/process/file component proofs, not a closed all-source
	// native drain or production deployment custody/ACTIVE certificate.
}
