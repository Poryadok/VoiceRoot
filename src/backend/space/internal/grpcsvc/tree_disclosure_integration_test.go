package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
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
	return disclosurePoolAt(t, "000025")
}

func disclosurePoolAt(t *testing.T, version string) *pgxpool.Pool {
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
		if filepath.Base(path)[:6] > version {
			break
		}
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)`)
	require.NoError(t, err)
	number, err := strconv.ParseInt(version, 10, 64)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO schema_migrations VALUES($1,false)`, number)
	require.NoError(t, err)
	return pool
}

func TestListSpaceTree_RequiresQualifiedCatalog(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL")
	}
	for _, name := range []string{"pre24", "missing_trigger", "disabled_trigger", "changed_function", "missing_metadata", "dirty", "unknown_version", "trigger_removed_during_enrichment", "current34"} {
		t.Run(name, func(t *testing.T) {
			version := "000024"
			switch name {
			case "pre24":
				version = "000021"
			case "current34":
				version = "000034"
			}
			pool := disclosurePoolAt(t, version)
			ctx := context.Background()
			viewer := uuid.New()
			st := &store.SpaceStore{Pool: pool}
			space, err := st.CreateSpace(ctx, viewer, "Catalog", "", "private")
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `INSERT INTO space_tree_nodes(id,space_id,kind,chat_id,sort_order) VALUES($1,$2,'text_chat',$3,0)`, uuid.New(), space.ID, uuid.New())
			require.NoError(t, err)
			var epoch uint64
			require.NoError(t, pool.QueryRow(ctx, `SELECT access_epoch FROM space_voice_access_epochs WHERE space_id=$1`, space.ID).Scan(&epoch))
			require.Positive(t, epoch, "a positive old media epoch is deliberately insufficient")
			corrupt := func(query string) {
				_, err := pool.Exec(ctx, query)
				require.NoError(t, err)
			}
			switch name {
			case "missing_trigger":
				corrupt(`DROP TRIGGER space_authority_revision_categories ON categories`)
			case "disabled_trigger":
				corrupt(`ALTER TABLE space_tree_nodes DISABLE TRIGGER space_authority_revision_space_tree_nodes`)
			case "changed_function":
				corrupt(`CREATE OR REPLACE FUNCTION space_authority_revision_changed() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF TG_OP='DELETE' THEN RETURN OLD; END IF; RETURN NEW; END $$`)
			case "missing_metadata":
				corrupt(`DROP TABLE schema_migrations`)
			case "dirty":
				corrupt(`UPDATE schema_migrations SET dirty=true`)
			case "unknown_version":
				corrupt(`UPDATE schema_migrations SET version=999`)
			}
			lookedUp := false
			svc := &SpaceGRPC{Store: st, Chats: disclosureChatLookup{read: func() {
				lookedUp = true
				if name == "trigger_removed_during_enrichment" {
					corrupt(`DROP TRIGGER space_authority_revision_categories ON categories`)
				}
			}}}
			actor := metadata.NewIncomingContext(ctx, metadata.Pairs(authctx.HeaderProfileID, viewer.String()))
			response, err := svc.ListSpaceTree(actor, &spacev1.ListSpaceTreeRequest{SpaceId: space.ID.String()})
			if name == "current34" {
				require.NoError(t, err)
				require.NotNil(t, response)
				require.True(t, lookedUp)
				return
			}
			require.Equal(t, codes.Unavailable, status.Code(err), "unqualified source catalogs must fail closed even with a positive epoch")
			require.Nil(t, response)
			require.Equal(t, name == "trigger_removed_during_enrichment", lookedUp, "initial qualification precedes enrichment, final qualification follows it")
		})
	}
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
