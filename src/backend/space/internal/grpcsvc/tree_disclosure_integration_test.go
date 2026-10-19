package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

type disclosureChatLookup struct{ read func() }

func (l disclosureChatLookup) GetChatNames(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]ChatInfo, error) {
	l.read()
	return map[uuid.UUID]ChatInfo{ids[0]: {Name: "Private conversation"}}, nil
}

func disclosurePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("BE110_TEST_DATABASE_DSN"); dsn != "" {
		admin, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		name := "be110_" + uuid.New().String()[:8]
		_, err = admin.Exec(ctx, `CREATE DATABASE "`+name+`"`)
		require.NoError(t, err)
		config, err := pgxpool.ParseConfig(dsn)
		require.NoError(t, err)
		config.ConnConfig.Database = name
		pool, err = pgxpool.NewWithConfig(ctx, config)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Close()
			_, err := admin.Exec(context.Background(), `DROP DATABASE "`+name+`" WITH (FORCE)`)
			require.NoError(t, err)
			admin.Close()
		})
	} else {
		pool = startSpacePostgresForTest(t, ctx)
	}
	paths, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", "*.up.sql"))
	require.NoError(t, err)
	for _, path := range paths {
		if filepath.Base(path)[:6] > "000024" {
			break
		}
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	return pool
}

func TestListSpaceTree_DisclosureCut(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL")
	}
	for _, name := range []string{"unchanged", "revoked", "membership_aba", "tree_changed"} {
		t.Run(name, func(t *testing.T) {
			pool := disclosurePool(t)
			ctx := context.Background()
			viewer, account := uuid.New(), uuid.New()
			st := &store.SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, viewer, "Private Space", "", "private")
			require.NoError(t, err)
			category, node, chat := uuid.New(), uuid.New(), uuid.New()
			_, err = pool.Exec(ctx, `INSERT INTO categories(id,space_id,name,sort_order) VALUES($1,$2,'Original',0)`, category, space.ID)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO space_tree_nodes(id,space_id,kind,chat_id,sort_order) VALUES($1,$2,'text_chat',$3,0)`, node, space.ID, chat)
			require.NoError(t, err)
			lookedUp := false
			svc := &SpaceGRPC{Store: st, Chats: disclosureChatLookup{read: func() {
				lookedUp = true
				switch name {
				case "revoked", "membership_aba":
					_, err := pool.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, viewer)
					require.NoError(t, err)
					if name == "membership_aba" {
						_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, viewer)
						require.NoError(t, err)
					}
				case "tree_changed":
					_, err := pool.Exec(ctx, `UPDATE categories SET name='Changed' WHERE id=$1`, category)
					require.NoError(t, err)
				}
			}}}
			actor := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderUserID, account.String(), authctx.HeaderProfileID, viewer.String()))
			response, err := svc.ListSpaceTree(actor, &spacev1.ListSpaceTreeRequest{SpaceId: space.ID.String()})
			require.True(t, lookedUp, "mutation must occur after the real tree read, inside enrichment")
			if name == "unchanged" {
				require.NoError(t, err)
				require.Equal(t, "Private conversation", response.GetNodes()[0].GetDisplayName())
				return
			}
			want := codes.Unavailable
			if name == "revoked" {
				want = codes.PermissionDenied
			}
			require.Equal(t, want, status.Code(err), "an old tree/enrichment cut must not survive a committed authority change")
			require.Nil(t, response, "no partial tree or enriched chat identity may be disclosed")
		})
	}
}
