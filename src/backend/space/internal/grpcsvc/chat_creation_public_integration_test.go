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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

func TestCompositePublicReplayCurrentSpaceMembershipAndGuestPG(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	ctx := context.Background()
	var pool *pgxpool.Pool
	if dsn := os.Getenv("BE131_PUBLIC_POSTGRES_URL"); dsn != "" {
		var err error
		pool, err = pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		t.Cleanup(pool.Close)
	} else {
		pool = startSpacePostgresForTest(t, ctx)
	}
	applySpaceMigration(t, ctx, pool)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000025_tree_audit_actions.up.sql", "000027_audit_ingestion.up.sql", "000028_chat_creation_outcomes.up.sql", "000031_composite_audit_reservation.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &store.SpaceStore{Pool: pool}
	profile := uuid.New()
	space, err := st.CreateSpace(ctx, profile, "Public replay", "", "private")
	require.NoError(t, err)
	b := &spacev1.SpaceChatCreationBinding{ProtocolVersion: 1, OperationId: uuid.NewString(), CreatorProfileId: profile.String(), SpaceId: space.ID.String(), ChatId: uuid.NewString(), NodeId: uuid.NewString(), AuditEventId: uuid.NewString(), ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, CreateRequestSha256: make([]byte, 32), Initiator: &spacev1.SpaceChatCreationBinding_User{User: &spacev1.SpaceChatCreationUser{AccountId: uuid.NewString(), SessionEpoch: 7, AccountType: "regular"}}}
	committed, err := st.CommitChatCreation(ctx, b, codes.OK)
	require.NoError(t, err)
	svc := NewChatCreationServer(&SpaceGRPC{Store: st}, nil)
	trusted := principal.WithVerified(ctx, principal.Principal{Kind: "service", Issuer: "chat", Subject: "service:chat", Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName})
	req := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: b.SpaceId, OperationId: b.OperationId, PublicActor: &spacev1.SpaceChatCreationPublicActor{ProfileId: profile.String(), Initiator: &spacev1.SpaceChatCreationPublicActor_User{User: proto.Clone(b.GetUser()).(*spacev1.SpaceChatCreationUser)}}}
	read := func(want codes.Code) {
		t.Helper()
		out, err := svc.GetSpaceChatCreationOutcome(trusted, req)
		require.Equal(t, want, status.Code(err), "%v", err)
		if want == codes.OK {
			require.True(t, proto.Equal(committed, out.Outcome))
		} else {
			require.Nil(t, out)
		}
	}
	read(codes.OK)
	// A freshly verified current guest cannot reuse its historical regular fact.
	req.PublicActor.GetUser().AccountType = "guest"
	req.PublicActor.GetUser().SessionEpoch = 8
	read(codes.NotFound)
	_, err = pool.Exec(ctx, "UPDATE spaces SET allow_guests=true WHERE id=$1", space.ID)
	require.NoError(t, err)
	read(codes.OK)
	_, err = pool.Exec(ctx, "UPDATE spaces SET allow_guests=false WHERE id=$1", space.ID)
	require.NoError(t, err)
	read(codes.NotFound)
	req.PublicActor.GetUser().AccountType = "regular"
	read(codes.OK)
	_, err = pool.Exec(ctx, "DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2", space.ID, profile)
	require.NoError(t, err)
	read(codes.NotFound)
	// Neither later ACL change rewrites/removes the durable creation outcome.
	req.PublicActor = nil
	read(codes.OK)
	var nodes int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM space_tree_nodes WHERE id=$1", uuid.MustParse(b.NodeId)).Scan(&nodes))
	require.Equal(t, 1, nodes)
}
