package lfpconsenttransport

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/story/internal/lfpconsentstore"
)

// This private MM fixture models durable original registration capture only.
// It does NOT claim to implement MM eligibility/acceptance/controller policy.
type actorMMFixture struct {
	storyv1.UnimplementedLfpActorBoundConsentRegistryServiceServer
	key              *rsa.PublicKey
	expected         lfpconsent.Envelope
	mu               sync.Mutex
	seen             map[string]bool
	ack              *storyv1.LfpActorBoundRegistrationAck
	reads, registers atomic.Int32
	mode             atomic.Int32 // 1 read outage,2 corruptACK,3 losefirstcommittedACK
	readHook         func(context.Context) error
}

func (f *actorMMFixture) authenticate(ctx context.Context, r proto.Message, method string) error {
	ep, ok := peer.FromContext(ctx)
	if !ok {
		return errors.New("no TLS")
	}
	ti, ok := ep.AuthInfo.(credentials.TLSInfo)
	if !ok || len(ti.State.VerifiedChains) == 0 {
		return errors.New("no verified TLS")
	}
	md, e := principal.IncomingMetadata(ctx)
	if e != nil {
		return e
	}
	h, e := principal.RequestHash(r)
	if e != nil {
		return e
	}
	_, e = principal.VerifyService(ctx, md.BearerToken, principal.VerifyConfig{ExpectedIssuer: "story", ExpectedAudience: "matchmaking", ExpectedRPC: method, ExpectedRequestID: md.RequestID, ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return f.key, nil }, ReplayGuard: func(_ context.Context, iss, jti string, _ time.Time) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.seen[iss+jti] {
			return errors.New("replay")
		}
		f.seen[iss+jti] = true
		return nil
	}})
	return e
}

func (f *actorMMFixture) ReadActorBoundLfpConsentRegistration(ctx context.Context, r *storyv1.ReadActorBoundLfpConsentRegistrationRequest) (*storyv1.ReadActorBoundLfpConsentRegistrationResponse, error) {
	if e := f.authenticate(ctx, r, storyv1.LfpActorBoundConsentRegistryService_ReadActorBoundLfpConsentRegistration_FullMethodName); e != nil {
		return nil, status.Error(codes.Unauthenticated, "denied")
	}
	f.reads.Add(1)
	if f.readHook != nil {
		if e := f.readHook(ctx); e != nil {
			return nil, status.Error(codes.Unavailable, "hook")
		}
	}
	if f.mode.Load() == 1 {
		return nil, status.Error(codes.Unavailable, "ambiguous")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ch, ah, q := f.expected.ConsentSHA256(), f.expected.ActorSHA256(), f.expected.QualifiedSHA256()
	id := f.expected.Consent().ConsentID
	if r.ProtocolVersion != 1 || !bytes.Equal(r.ConsentId, id[:]) || !bytes.Equal(r.RegistrationOperationId, r.ConsentId) || !bytes.Equal(r.ConsentSha256, ch[:]) || !bytes.Equal(r.ActorSha256, ah[:]) || !bytes.Equal(r.QualifiedConsentSha256, q[:]) || !proto.Equal(r.Origin, wireOrigin(f.expected.Origin())) {
		return nil, status.Error(codes.FailedPrecondition, "wrong binding")
	}
	if f.ack == nil {
		return nil, status.Error(codes.NotFound, "definite original registration absent")
	}
	ack := proto.Clone(f.ack).(*storyv1.LfpActorBoundRegistrationAck)
	if f.mode.Load() == 2 {
		ack.ActorBoundRegistrationSha256[0]++
	}
	return &storyv1.ReadActorBoundLfpConsentRegistrationResponse{Acknowledgement: ack}, nil
}

func (f *actorMMFixture) RegisterActorBoundLfpConsent(ctx context.Context, r *storyv1.RegisterActorBoundLfpConsentRequest) (*storyv1.RegisterActorBoundLfpConsentResponse, error) {
	if e := f.authenticate(ctx, r, storyv1.LfpActorBoundConsentRegistryService_RegisterActorBoundLfpConsent_FullMethodName); e != nil {
		return nil, status.Error(codes.Unauthenticated, "denied")
	}
	f.registers.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.expected.Consent().ConsentID
	if r.ProtocolVersion != 1 || !bytes.Equal(r.RegistrationOperationId, id[:]) || !proto.Equal(r.Envelope, wireEnvelope(f.expected)) {
		return nil, status.Error(codes.FailedPrecondition, "wrong envelope")
	}
	if f.ack == nil {
		reg := lfpconsent.Registration{OperationID: id, ConsentID: id, ConsentSHA256: f.expected.ConsentSHA256(), RegistrationID: uuid.New(), Generation: 17, CapturedSearchIntentSHA256: [32]byte{8}}
		b, h, e := lfpconsent.EncodeRegistration(reg)
		if e != nil {
			return nil, e
		}
		base := &storyv1.LfpConsentRegistration{}
		if e = proto.Unmarshal(b, base); e != nil {
			return nil, e
		}
		base.RegistrationSha256 = h[:]
		q := f.expected.QualifiedSHA256()
		bound, e := lfpconsent.HashActorBoundRegistration(h, q)
		if e != nil {
			return nil, e
		}
		f.ack = &storyv1.LfpActorBoundRegistrationAck{ProtocolVersion: 1, Registration: base, Origin: wireOrigin(f.expected.Origin()), QualifiedConsentSha256: q[:], ActorBoundRegistrationSha256: bound[:]}
	}
	if f.mode.CompareAndSwap(3, 0) {
		return nil, status.Error(codes.Unavailable, "lost committed ACK")
	}
	return &storyv1.RegisterActorBoundLfpConsentResponse{Acknowledgement: proto.Clone(f.ack).(*storyv1.LfpActorBoundRegistrationAck)}, nil
}

func actorRegistryFixture(t *testing.T, journal *lfpconsentstore.Store, envelope lfpconsent.Envelope) (*ActorRegistryProducer, *actorMMFixture) {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, e)
	signer, e := principal.NewIssuer(principal.IssuerConfig{Issuer: "story", KeyID: "test", PrivateKey: key})
	require.NoError(t, e)
	mm := &actorMMFixture{key: &key.PublicKey, expected: envelope, seen: map[string]bool{}}
	conn := actorTLSClient(t, func(s *grpc.Server) { storyv1.RegisterLfpActorBoundConsentRegistryServiceServer(s, mm) })
	return &ActorRegistryProducer{Client: storyv1.NewLfpActorBoundConsentRegistryServiceClient(conn), Signer: signer, Origin: envelope.Origin(), PeerAuthority: responsePeerAuthority{envelope.Origin()}, Journal: journal}, mm
}

func TestActorRegistryActualLostAckRecovery(t *testing.T) {
	for _, kind := range []uint64{lfpconsent.Guest, lfpconsent.Regular} {
		journal, envelope := actorJournalFixture(t, kind)
		producer, mm := actorRegistryFixture(t, journal, envelope)
		mm.mode.Store(3)
		ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-profile-id", "must not forward"))
		id, q := envelope.Consent().ConsentID, envelope.QualifiedSHA256()
		_, e := producer.Deliver(ctx, id, q)
		require.Equal(t, codes.Unavailable, status.Code(e))
		require.Equal(t, int32(1), mm.registers.Load())
		var count int
		require.NoError(t, journal.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_actor_bound_acks").Scan(&count))
		require.Zero(t, count)
		ack, e := producer.Deliver(ctx, id, q)
		require.NoError(t, e)
		require.Equal(t, int32(1), mm.registers.Load())
		require.Equal(t, uint64(17), ack.Registration.Generation)
		saved, e := journal.ReadActorRegistration(ctx, id, q)
		require.NoError(t, e)
		require.Equal(t, ack, saved)
		mm.mode.Store(1)
		_, e = producer.Deliver(ctx, id, q)
		require.Error(t, e)
		require.Equal(t, int32(1), mm.registers.Load())
		mm.mode.Store(2)
		_, e = producer.Deliver(ctx, id, q)
		require.Error(t, e)
		require.Equal(t, int32(1), mm.registers.Load())
		mm.mode.Store(0)
		again, e := producer.Deliver(ctx, id, q)
		require.NoError(t, e)
		require.Equal(t, ack, again)
	}
}

func TestActorRegistryRechecksOriginBeforeRegister(t *testing.T) {
	journal, envelope := actorJournalFixture(t, lfpconsent.Guest)
	producer, mm := actorRegistryFixture(t, journal, envelope)
	mm.readHook = func(ctx context.Context) error {
		_, e := journal.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET phase='closed'")
		return e
	}
	_, e := producer.Deliver(context.Background(), envelope.Consent().ConsentID, envelope.QualifiedSHA256())
	require.Error(t, e)
	require.Equal(t, int32(1), mm.reads.Load())
	require.Zero(t, mm.registers.Load(), "retirement during absent read forbids second outbound Register")
}

func TestActorRegistrySuccessRequiresLocalDurableAck(t *testing.T) {
	journal, envelope := actorJournalFixture(t, lfpconsent.Guest)
	producer, mm := actorRegistryFixture(t, journal, envelope)
	ctx := context.Background()
	_, e := journal.Pool.Exec(ctx, `CREATE FUNCTION fail_bound_ack() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture fail'; END $$; CREATE TRIGGER fail_bound_ack BEFORE INSERT ON story_lfp_actor_bound_acks FOR EACH ROW EXECUTE FUNCTION fail_bound_ack()`)
	require.NoError(t, e)
	id, q := envelope.Consent().ConsentID, envelope.QualifiedSHA256()
	_, e = producer.Deliver(ctx, id, q)
	require.Error(t, e)
	var count int
	require.NoError(t, journal.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_registration_acks").Scan(&count))
	require.Zero(t, count)
	_, e = journal.Pool.Exec(ctx, "DROP TRIGGER fail_bound_ack ON story_lfp_actor_bound_acks")
	require.NoError(t, e)
	_, e = producer.Deliver(ctx, id, q)
	require.NoError(t, e)
	require.Equal(t, int32(1), mm.registers.Load(), "local failure recovers original remote capture")
}
