package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"os"
	"path/filepath"
	"testing"
	"time"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/bot/internal/installprincipal"
	"voice/backend/pkg/principal"
)

func nativeAdmin(t *testing.T, req proto.Message, method string) installprincipal.Administrator {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.NewString()
	token, err := signer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "bot", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, ClientExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	verifier := &installprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	var admin installprincipal.Administrator
	_, err = verifier.Unary()(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		admin, err = installprincipal.Require(ctx, method, req)
		return nil, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return admin
}
func TestMembershipNativeTransitions(t *testing.T) {
	endpoint := os.Getenv("BOT_MEMBERSHIP_TEST_DATABASE_URL")
	if endpoint == "" {
		if testing.Short() {
			t.Skip("native database test")
		}
		t.Skip("BOT_MEMBERSHIP_TEST_DATABASE_URL not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := "membership_" + uuid.New().String()[:8]
	if _, err = pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	cfg, err := pgxpool.ParseConfig(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	files, err := filepath.Glob("../../../migrations/bot_db/*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		raw, e := os.ReadFile(file)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(ctx, string(raw)); e != nil {
			t.Fatalf("%s: %v", file, e)
		}
	}
	if _, err = db.Exec(ctx, "CREATE TABLE schema_migrations(version bigint,dirty boolean); INSERT INTO schema_migrations VALUES(9,false)"); err != nil {
		t.Fatal(err)
	}
	st := &BotStore{Pool: db}
	var source uuid.UUID
	if err = db.QueryRow(ctx, "SELECT source_generation FROM bot_space_membership_source").Scan(&source); err != nil {
		t.Fatal(err)
	}
	bot, _, err := st.CreateBot(ctx, uuid.New(), "native", "", "[]", uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	req := &botv1.InstallBotInSpaceRequest{BotId: bot.ID.String(), SpaceId: uuid.NewString()}
	admin := nativeAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName)
	first, err := st.PrepareInstall(ctx, source, admin, req)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := st.PrepareInstall(ctx, source, admin, req)
	if err != nil || replay.AddOperationID != first.AddOperationID {
		t.Fatalf("prepared retry changed identity: %v", err)
	}
	var visible int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM bot_space_installations").Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("pending install visible: %d %v", visible, err)
	}
	claim, err := st.ClaimMembershipAttempt(ctx, source, first, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	cancelReq := &botv1.UninstallBotFromSpaceRequest{BotId: req.BotId, SpaceId: req.SpaceId}
	cancelAdmin := nativeAdmin(t, cancelReq, botv1.BotService_UninstallBotFromSpace_FullMethodName)
	canceled, err := st.BeginCancel(ctx, source, cancelAdmin, first.SpaceID, first.BotID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.RecordAddAcknowledged(ctx, source, claim); err == nil {
		t.Fatal("stale ACK activated canceled generation")
	}
	cancelClaim, err := st.ClaimMembershipAttempt(ctx, source, canceled, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := st.RecordCancelAcknowledged(ctx, source, cancelClaim)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.PrepareInstall(ctx, source, nativeAdmin(t, req, botv1.BotService_InstallBotInSpace_FullMethodName), req); err == nil {
		t.Fatal("unsettled cleanup admitted next generation")
	}
	if err = st.CompleteCancellation(ctx, source, ack); err == nil {
		t.Fatal("cleanup without receiver evidence completed")
	}
	if _, err = db.Exec(ctx, "DELETE FROM bot_space_membership_heads"); err == nil {
		t.Fatal("journal deletion accepted")
	}
}
