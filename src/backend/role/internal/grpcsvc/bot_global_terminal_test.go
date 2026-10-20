package grpcsvc

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func rootTerminalContext(t *testing.T, r *rolev1.RegisterBotGlobalTerminalRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "role", RPC: rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, RequestID: r.DecisionId, RequestHash: hash})
}

func TestBotGlobalTerminalHandler_ZeroSpaceActualStore(t *testing.T) {
	ctx := context.Background()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(root, "src/backend/migrations/role_db"))
	f.Run(true, "goto", "26")
	name, pwd := "terminal_handler_"+strings.ReplaceAll(uuid.NewString(), "-", ""), uuid.NewString()
	id := pgx.Identifier{name}.Sanitize()
	_, err := f.Pool.Exec(ctx, "CREATE ROLE "+id+" LOGIN PASSWORD '"+pwd+"' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION")
	require.NoError(t, err)
	_, err = f.Pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+id+"; GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+id+"; GRANT INSERT ON public.role_bot_global_terminals TO "+id)
	require.NoError(t, err)
	cfg := f.Pool.Config().Copy()
	cfg.ConnConfig.User = name
	cfg.ConnConfig.Password = pwd
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer pool.Close()
	service := &RoleGRPC{Store: &store.RoleStore{Pool: pool}}
	req := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	result, err := service.RegisterBotGlobalTerminal(rootTerminalContext(t, req), req)
	require.NoError(t, err)
	require.NotNil(t, result.GetReceipt())
	require.Equal(t, req.BotId, result.Receipt.BotId)
	require.Equal(t, req.DecisionId, result.Receipt.DecisionId)
	require.Equal(t, "role", result.Receipt.Participant)
	again, err := service.RegisterBotGlobalTerminal(rootTerminalContext(t, req), req)
	require.NoError(t, err)
	require.True(t, proto.Equal(result, again))
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM roles").Scan(&count))
	require.Zero(t, count)
}
