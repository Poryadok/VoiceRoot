package lfpconsenttransport

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/storylfpadmission"
	"voice/backend/story/internal/lfpconsentstore"
)

type actorTestPermit struct{ origin storylfpadmission.Origin }

func (p actorTestPermit) Origin() storylfpadmission.Origin { return p.origin }
func (actorTestPermit) Release()                           {}

type actorTestAdmission struct{ origin storylfpadmission.Origin }

func (a actorTestAdmission) Enter(context.Context) (storylfpadmission.Permit, error) {
	return actorTestPermit{a.origin}, nil
}

// Component-only admission fixture: no installed controller/restore custody.
func actorJournalFixture(t *testing.T, accountType uint64) (*lfpconsentstore.Store, lfpconsent.Envelope) {
	t.Helper()
	dsn := os.Getenv("LFP_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("isolated LFP_TEST_DATABASE_DSN required")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	require.NoError(t, e)
	schema := "lfpt_" + uuid.New().String()[:8]
	_, e = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, e)
	cfg, e := pgxpool.ParseConfig(dsn)
	require.NoError(t, e)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, e)
	t.Cleanup(func() {
		pool.Close()
		_, e := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, e)
		admin.Close()
	})
	for _, name := range []string{"000001_init", "000002_visibility_audience", "000003_hidden_from_feed", "000004_archive_purge_outbox", "000005_lfp_consent_journal", "000006_lfp_actor_provenance"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", name+".up.sql"))
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(b))
		require.NoError(t, e)
	}
	env, install, inc := uuid.New(), uuid.New(), uuid.New()
	db := [32]byte{5}
	origin, e := storylfpadmission.NewOrigin(env, install, inc, db)
	require.NoError(t, e)
	_, e = pool.Exec(ctx, `INSERT INTO story_lfp_admission_head VALUES(true,$1,$2,$3,$4,'open')`, env, install, inc, db[:])
	require.NoError(t, e)
	s := lfpconsentstore.NewBoundStore(pool, storylfpadmission.NewBarrier(origin, actorTestAdmission{origin}))
	id, author := uuid.New(), uuid.New()
	_, e = pool.Exec(ctx, `INSERT INTO stories(id,author_profile_id,type,is_looking_for_party,lfp_criteria,visibility,expires_at,archived_until) VALUES($1,$2,'text',true,'{"game_id":"test","mode":"pair"}','everyone',clock_timestamp()+interval '1 hour',clock_timestamp()+interval '31 days')`, id, author)
	require.NoError(t, e)
	op := uuid.New()
	actor := lfpconsent.Actor{ActionKind: lfpconsent.PublishOffer, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: lfpconsent.Regular, AccountID: uuid.New(), ProfileID: author, SessionEpoch: 7, OperationID: op, Issuer: "gateway", Audience: "story", FullRPC: lfpconsent.PublishOfferRPC, PrincipalRequestSHA256: [32]byte{1}}
	offer, e := s.PublishActorOffer(ctx, lfpconsentstore.AuthorAction{OperationID: op, StoryID: id, ProfileID: author, RequestSHA256: actor.PrincipalRequestSHA256}, actor, origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	op = uuid.New()
	actor = lfpconsent.Actor{ActionKind: lfpconsent.Respond, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: accountType, AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 9, OperationID: op, Issuer: "gateway", Audience: "story", FullRPC: lfpconsent.RespondRPC, PrincipalRequestSHA256: [32]byte{2}}
	envelope, e := s.RecordActorResponse(ctx, lfpconsentstore.Response{OperationID: op, AccountID: actor.AccountID, ProfileID: actor.ProfileID, SessionEpoch: actor.SessionEpoch, RequestSHA256: actor.PrincipalRequestSHA256, Offer: offer, Type: lfpconsent.Join}, actor, origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	return s, envelope
}

// A real mTLS loopback listener exists ONLY inside the owned Linux test runner;
// no host ports or installed deployment trust are involved.
func actorTLSClient(t *testing.T, register func(*grpc.Server)) *grpc.ClientConn {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "isolated LFP fixture"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, e := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	require.NoError(t, e)
	root, e := x509.ParseCertificate(der)
	require.NoError(t, e)
	roots := x509.NewCertPool()
	roots.AddCert(root)
	issue := func(serial int64, server bool) tls.Certificate {
		usage := x509.ExtKeyUsageClientAuth
		if server {
			usage = x509.ExtKeyUsageServerAuth
		}
		leaf := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "mm-fixture"}, DNSNames: []string{"story-fixture"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
		b, e := x509.CreateCertificate(rand.Reader, leaf, root, &key.PublicKey, key)
		require.NoError(t, e)
		return tls.Certificate{Certificate: [][]byte{b, der}, PrivateKey: key}
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{issue(2, true)}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots})))
	register(server)
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, e)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	client, e := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, ServerName: "story-fixture", RootCAs: roots, Certificates: []tls.Certificate{issue(3, false)}})))
	require.NoError(t, e)
	t.Cleanup(func() { require.NoError(t, client.Close()); server.Stop(); <-done })
	return client
}
