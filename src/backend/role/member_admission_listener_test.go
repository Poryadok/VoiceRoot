package main

import (
	"context"
	"crypto/tls"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/role/internal/grpcsvc"
	"voice/backend/role/internal/store"
)

func admissionListenerStore(t *testing.T) (context.Context, *store.RoleStore) {
	t.Helper()
	integrationtest.ConfigureDockerTesting()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	pg, err := postgres.Run(ctx, integrationtest.PostgresImage, postgres.BasicWaitStrategies(), postgres.WithDatabase("admission"), postgres.WithUsername("fixture"), postgres.WithPassword("fixture_only"))
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		require.NoError(t, pg.Terminate(cleanup))
	})
	dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, strings.Replace(dsn, "localhost", "127.0.0.1", 1))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	directory := filepath.Join("..", "migrations", "role_db")
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	var copied []testcontainers.ContainerFile
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") {
			copied = append(copied, testcontainers.ContainerFile{HostFilePath: filepath.Join(directory, file.Name()), ContainerFilePath: "/migrations/" + file.Name(), FileMode: 0644})
		}
	}
	database := &url.URL{Scheme: "postgres", User: url.UserPassword("fixture", "fixture_only"), Host: "127.0.0.1:5432", Path: "/admission", RawQuery: "sslmode=disable"}
	migration, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "migrate/migrate:v4.18.1", NetworkMode: container.NetworkMode("container:" + pg.GetContainerID()), Files: copied, Cmd: []string{"-path", "/migrations", "-database", database.String(), "up"}, WaitingFor: wait.ForExit()}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		require.NoError(t, migration.Terminate(cleanup))
	})
	result, err := migration.State(ctx)
	require.NoError(t, err)
	require.Zero(t, result.ExitCode)
	return ctx, &store.RoleStore{Pool: pool}
}

func TestMemberAdmissionMissingNativeAuthorityFailsClosed(t *testing.T) {
	ctx, reader := admissionListenerStore(t)
	space, owner, profile := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, reader.BootstrapSpaceRoles(ctx, space, owner))
	runtime, roots, key, certificate := newMainPrincipalRuntime(t)
	legacy, protected := newRoleGRPCServers(nil, &grpcsvc.RoleGRPC{Store: reader}, runtime)
	legacy.Stop()
	_, address := sourceListenerClient(t, protected, credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}}))
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{certificate}})))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	operation := uuid.NewString()
	req := &rolev1.EnsureDefaultMemberRoleRequest{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: profile.String(), MembershipGeneration: 1, AdmissionOperationId: operation, AdmissionBindingSha256: make([]byte, 32)}
	_, err = rolev1.NewRoleServiceClient(conn).EnsureDefaultMemberRole(sourceCallContext(t, key, "space", rolev1.RoleService_EnsureDefaultMemberRole_FullMethodName, uuid.NewString(), req), req)
	require.Equal(t, codes.Unavailable, status.Code(err))
}
