package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
)

func TestMinimalTerminal48ZeroSpaceSQL(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "46")
	admin := &SpaceStore{Pool: db.Pool}
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	serving := nativeReadinessLogin(t, ctx, db.Pool)
	tx, err := serving.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	var receipt uuid.UUID
	err = tx.QueryRow(ctx, `SELECT receipt_id FROM public.space_bot_global_terminal_register($1,$2)`, uuid.New(), uuid.New()).Scan(&receipt)
	require.NoError(t, err, "a qualified zero-Space Bot root must register its own terminal fence")
	require.NoError(t, tx.Commit(ctx))
	request := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
	st := &SpaceStore{Pool: serving}
	invoke := signedMinimalTerminal(t, st)
	first, err := invoke(request)
	require.NoError(t, err)
	second, err := signedMinimalTerminal(t, &SpaceStore{Pool: serving})(request)
	require.NoError(t, err)
	require.True(t, proto.Equal(first, second))
	changed := proto.Clone(request).(*spacev1.RegisterBotGlobalTerminalRequest)
	changed.DecisionId = uuid.NewString()
	_, err = invoke(changed)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	changed = proto.Clone(request).(*spacev1.RegisterBotGlobalTerminalRequest)
	changed.BotId = uuid.NewString()
	_, err = invoke(changed)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict)
	_, err = signedMinimalTerminal(t, admin)(request)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "owner session is never a serving credential")
	var spaces, legacy, rows int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.spaces),(SELECT count(*) FROM native_membership_guard.bot_terminals),(SELECT count(*) FROM native_membership_guard.bot_terminal_registrations)`).Scan(&spaces, &legacy, &rows))
	require.Zero(t, spaces)
	require.Zero(t, legacy)
	require.Equal(t, 2, rows)
	db.Run(false, "down", "1")
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_terminal_registrations`).Scan(&rows))
	require.Equal(t, 2, rows, "actual driver retained-DOWN refusal preserves permanent results")
}

func signedMinimalTerminal(t *testing.T, st *SpaceStore) func(*spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.BotGlobalTerminalReceipt, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	return func(r *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.BotGlobalTerminalReceipt, error) {
		hash, err := principal.RequestHash(r)
		require.NoError(t, err)
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, RequestID: r.DecisionId, RequestHash: hash})
		require.NoError(t, err)
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", r.DecisionId))
		value, err := verifier.Unary()(ctx, r, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			proof, err := botmembershipprincipal.RequireGlobalTerminal(ctx, r)
			if err != nil {
				return nil, err
			}
			return st.RegisterBotGlobalTerminal(ctx, proof, r)
		})
		if value == nil {
			return nil, err
		}
		return value.(*spacev1.BotGlobalTerminalReceipt), err
	}
}

func TestMinimalTerminal48CanonicalStructures(t *testing.T) {
	for _, major := range []int{16, 18} {
		t.Run(fmt.Sprint(major), func(t *testing.T) {
			ctx := context.Background()
			db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), fmt.Sprintf("postgres:%d-alpine", major))
			db.Run(true, "goto", "47")
			var before []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&before))
			db.Run(true, "up", "1")
			var structure string
			require.NoError(t, db.Pool.QueryRow(ctx, nativeReadinessStructureSQL).Scan(&structure))
			require.NoError(t, os.WriteFile(fmt.Sprintf("/tmp/minimal48-pg%d-structure.json", major), []byte(structure), 0600))
			// Installing schema cannot itself turn an unknown retained seal into serving.
			app := nativeReadinessLogin(t, ctx, db.Pool)
			_, err := app.Exec(ctx, `SELECT public.space_bot_global_terminal_register($1,$2)`, uuid.New(), uuid.New())
			require.Error(t, err)
			_, err = (&SpaceStore{Pool: db.Pool}).TransitionNativeMembershipCatalog48(ctx)
			require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "fixed shapes/UUIDs cannot adopt an unadmitted old46/47 history")
			db.Run(true, "down", "1")
			var after []byte
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT native_membership_guard.catalog()`).Scan(&after))
			require.Equal(t, before, after, "unused DOWN restores exact previous47 function body bytes and catalog")
		})
	}
}
