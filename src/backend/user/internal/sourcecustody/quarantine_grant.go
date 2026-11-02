package sourcecustody

// Private and unwired quarantine-only protocol. Neither a grant nor PREPARING
// is a launcher, fence, receipt, or proof of rollback resistance. Production
// requires an independently hosted durable ledger, an independently protected
// host key, verified host/boot identity, trusted time and a separate boot gate.
// A live-memory snapshot taken after consumption is not invalidated here;
// every irreversible action needs independently verified fresh authority.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"math"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errQuarantineUnavailable = errors.New("quarantine grant unavailable")

type quarantineScope struct {
	source                    [32]byte
	generation                uint64
	host                      [32]byte
	boot                      [16]byte
	executable, runtime, data [32]byte
}

func (s quarantineScope) valid() bool {
	return s.source != [32]byte{} && s.generation > 0 && s.generation <= math.MaxInt64 && s.host != [32]byte{} && s.boot != [16]byte{} && s.executable != [32]byte{} && s.runtime != [32]byte{} && s.data != [32]byte{}
}

func (s quarantineScope) bytes() []byte {
	b := make([]byte, 0, 184)
	b = append(b, s.source[:]...)
	var generation [8]byte
	binary.BigEndian.PutUint64(generation[:], s.generation)
	b = append(b, generation[:]...)
	b = append(b, s.host[:]...)
	b = append(b, s.boot[:]...)
	b = append(b, s.executable[:]...)
	b = append(b, s.runtime[:]...)
	b = append(b, s.data[:]...)
	return b
}

type quarantineChallenge struct {
	scope            quarantineScope
	challenge, nonce [32]byte
	expires          int64
}

func (c quarantineChallenge) bytes() []byte {
	b := c.scope.bytes()
	b = append(b, c.challenge[:]...)
	b = append(b, c.nonce[:]...)
	var expires [8]byte
	binary.BigEndian.PutUint64(expires[:], uint64(c.expires))
	return append(b, expires[:]...)
}

func challengeProofBytes(c quarantineChallenge) []byte {
	return append([]byte("voice.quarantine.host-challenge.v1\x00"), c.bytes()...)
}

type quarantineGrant struct {
	challenge quarantineChallenge
	signature [ed25519.SignatureSize]byte
}

func grantBytes(c quarantineChallenge) []byte {
	return append([]byte("voice.quarantine.QUARANTINE_ONLY.v1\x00"), c.bytes()...)
}

func grantHash(g quarantineGrant) [32]byte {
	b := grantBytes(g.challenge)
	b = append(b, g.signature[:]...)
	return sha256.Sum256(b)
}

func consumeProofBytes(g quarantineGrant) []byte {
	h := grantHash(g)
	return append([]byte("voice.quarantine.host-consume.v1\x00"), h[:]...)
}

type quarantineAuthority struct {
	pool   *pgxpool.Pool
	scope  quarantineScope
	host   ed25519.PublicKey
	issuer ed25519.PrivateKey
}

// The authority and its PostgreSQL ledger must be independently hosted. The
// per-source row and its lifecycle are provisioned/retained by trusted external
// control; this package never resets or advances the source generation.
// Any future remote begin endpoint must authenticate the selected host before
// calling begin; otherwise an unauthenticated request could exhaust its row.
// A remote consume response requires an authenticated pinned authority channel;
// a signed grant alone does not authenticate that response.
func newQuarantineAuthority(pool *pgxpool.Pool, scope quarantineScope, host ed25519.PublicKey, issuer ed25519.PrivateKey) (*quarantineAuthority, error) {
	if pool == nil || !scope.valid() || len(host) != ed25519.PublicKeySize || len(issuer) != ed25519.PrivateKeySize || sha256.Sum256(host) != scope.host {
		return nil, errQuarantineUnavailable
	}
	canonical := ed25519.NewKeyFromSeed(issuer.Seed())
	canonicalOK := subtle.ConstantTimeCompare(issuer, canonical) == 1
	clear(canonical)
	if !canonicalOK {
		return nil, errQuarantineUnavailable
	}
	return &quarantineAuthority{pool: pool, scope: scope, host: append(ed25519.PublicKey(nil), host...), issuer: append(ed25519.PrivateKey(nil), issuer...)}, nil
}

func beginQuarantineTx(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, errQuarantineUnavailable
	}
	if _, err = tx.Exec(ctx, `SET LOCAL search_path = pg_catalog`); err != nil {
		rollbackQuarantineTx(tx)
		return nil, errQuarantineUnavailable
	}
	return tx, nil
}

func rollbackQuarantineTx(tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}

func (a *quarantineAuthority) begin(ctx context.Context) (quarantineChallenge, error) {
	var c quarantineChallenge
	if a == nil || ctx == nil || ctx.Err() != nil {
		return c, errQuarantineUnavailable
	}
	tx, err := beginQuarantineTx(ctx, a.pool)
	if err != nil {
		return c, errQuarantineUnavailable
	}
	defer rollbackQuarantineTx(tx)
	var now int64
	if tx.QueryRow(ctx, `SELECT pg_catalog.floor(extract(epoch FROM pg_catalog.clock_timestamp()))::pg_catalog.int8`).Scan(&now) != nil || now <= 0 {
		return c, errQuarantineUnavailable
	}
	c.scope = a.scope
	c.expires = now + 30
	if _, err := rand.Read(c.challenge[:]); err != nil {
		return quarantineChallenge{}, errQuarantineUnavailable
	}
	if _, err := rand.Read(c.nonce[:]); err != nil {
		return quarantineChallenge{}, errQuarantineUnavailable
	}
	tag, err := tx.Exec(ctx, `INSERT INTO public.quarantine_generation_ledger(source,generation,challenge,expires,state) VALUES($1,$2,$3,$4,'CHALLENGED') ON CONFLICT(source) DO NOTHING`, c.scope.source[:], int64(c.scope.generation), c.bytes(), c.expires)
	if err != nil || tag.RowsAffected() != 1 || ctx.Err() != nil {
		return quarantineChallenge{}, errQuarantineUnavailable
	}
	if tx.Commit(ctx) != nil || ctx.Err() != nil {
		return quarantineChallenge{}, errQuarantineUnavailable
	}
	return c, nil
}

func (a *quarantineAuthority) issue(ctx context.Context, c quarantineChallenge, hostProof []byte) (quarantineGrant, error) {
	var g quarantineGrant
	if a == nil || ctx == nil || ctx.Err() != nil || c.scope != a.scope || c.challenge == [32]byte{} || c.nonce == [32]byte{} || c.expires <= 0 || !ed25519.Verify(a.host, challengeProofBytes(c), hostProof) {
		return g, errQuarantineUnavailable
	}
	g.challenge = c
	copy(g.signature[:], ed25519.Sign(a.issuer, grantBytes(c)))
	h := grantHash(g)
	tx, err := beginQuarantineTx(ctx, a.pool)
	if err != nil {
		return quarantineGrant{}, errQuarantineUnavailable
	}
	defer rollbackQuarantineTx(tx)
	tag, err := tx.Exec(ctx, `UPDATE public.quarantine_generation_ledger SET state='ISSUED',grant_hash=$1 WHERE source=$2 AND generation=$3 AND challenge=$4 AND expires=$5 AND expires>pg_catalog.floor(extract(epoch FROM pg_catalog.clock_timestamp()))::pg_catalog.int8 AND state='CHALLENGED'`, h[:], c.scope.source[:], int64(c.scope.generation), c.bytes(), c.expires)
	if err != nil || tag.RowsAffected() != 1 || ctx.Err() != nil {
		return quarantineGrant{}, errQuarantineUnavailable
	}
	if tx.Commit(ctx) != nil || ctx.Err() != nil {
		return quarantineGrant{}, errQuarantineUnavailable
	}
	return g, nil
}

func (a *quarantineAuthority) consume(ctx context.Context, g quarantineGrant, hostProof []byte) error {
	if a == nil || ctx == nil || ctx.Err() != nil || g.challenge.scope != a.scope || !ed25519.Verify(a.issuer.Public().(ed25519.PublicKey), grantBytes(g.challenge), g.signature[:]) || !ed25519.Verify(a.host, consumeProofBytes(g), hostProof) {
		return errQuarantineUnavailable
	}
	tx, err := beginQuarantineTx(ctx, a.pool)
	if err != nil {
		return errQuarantineUnavailable
	}
	defer rollbackQuarantineTx(tx)
	h := grantHash(g)
	c := g.challenge
	tag, err := tx.Exec(ctx, `UPDATE public.quarantine_generation_ledger SET state='CONSUMED' WHERE source=$1 AND generation=$2 AND challenge=$3 AND expires=$4 AND expires>pg_catalog.floor(extract(epoch FROM pg_catalog.clock_timestamp()))::pg_catalog.int8 AND state='ISSUED' AND grant_hash=$5`, c.scope.source[:], int64(c.scope.generation), c.bytes(), c.expires, h[:])
	if err != nil || tag.RowsAffected() != 1 || ctx.Err() != nil {
		return errQuarantineUnavailable
	}
	if tx.Commit(ctx) != nil || ctx.Err() != nil {
		return errQuarantineUnavailable
	}
	return nil
}

type quarantineTransport interface {
	consume(context.Context, quarantineGrant, []byte) error
}
type quarantineChallengeTransport interface {
	begin(context.Context) (quarantineChallenge, error)
	issue(context.Context, quarantineChallenge, []byte) (quarantineGrant, error)
}

// Production must implement this through an independently protected host key;
// this private interface by itself does not establish host or boot identity.
type quarantineHostSigner interface {
	publicKey() ed25519.PublicKey
	sign(context.Context, []byte) ([]byte, error)
}
type quarantineClientState uint8

const (
	quarantineClosed quarantineClientState = iota
	quarantinePreparing
)

type quarantineClient struct {
	mu        sync.Mutex
	expected  quarantineScope
	issuer    ed25519.PublicKey
	host      quarantineHostSigner
	transport quarantineTransport
	attempted bool
	phase     quarantineClientState
	// Private scheduling seam for a cancellation regression test.
	afterConsume func()
}

func newQuarantineClient(expected quarantineScope, issuer ed25519.PublicKey, host quarantineHostSigner, transport quarantineTransport) *quarantineClient {
	return &quarantineClient{expected: expected, issuer: append(ed25519.PublicKey(nil), issuer...), host: host, transport: transport, phase: quarantineClosed}
}

func (c *quarantineClient) hostProof(ctx context.Context, message []byte) ([]byte, error) {
	if c.host == nil || ctx == nil || ctx.Err() != nil {
		return nil, errQuarantineUnavailable
	}
	public := c.host.publicKey()
	if len(public) != ed25519.PublicKeySize || sha256.Sum256(public) != c.expected.host {
		return nil, errQuarantineUnavailable
	}
	proof, err := c.host.sign(ctx, message)
	if err != nil || ctx.Err() != nil || !ed25519.Verify(public, message, proof) {
		return nil, errQuarantineUnavailable
	}
	return proof, nil
}

// prepare makes one attempt only. It returns no grant, executable or fence
// receipt; callers must not interpret PREPARING as authority to launch.
func (c *quarantineClient) prepare(ctx context.Context, g quarantineGrant) error {
	if !c.reserve() {
		return errQuarantineUnavailable
	}
	return c.prepareReserved(ctx, g)
}

func (c *quarantineClient) reserve() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempted {
		return false
	}
	c.attempted = true
	return true
}

// start performs the independently authenticated challenge, then consumes the
// signed grant. Any failure, including an ambiguous reply, burns this client's
// sole attempt; a fresh client must obtain fresh external authority.
func (c *quarantineClient) start(ctx context.Context) error {
	if !c.reserve() {
		return errQuarantineUnavailable
	}
	transport, ok := c.transport.(quarantineChallengeTransport)
	if !ok || ctx == nil || ctx.Err() != nil || !c.expected.valid() {
		return errQuarantineUnavailable
	}
	challenge, err := transport.begin(ctx)
	if err != nil || ctx.Err() != nil || challenge.scope != c.expected || challenge.expires <= time.Now().Unix() || challenge.challenge == [32]byte{} || challenge.nonce == [32]byte{} {
		return errQuarantineUnavailable
	}
	proof, err := c.hostProof(ctx, challengeProofBytes(challenge))
	if err != nil {
		return errQuarantineUnavailable
	}
	grant, err := transport.issue(ctx, challenge, proof)
	if err != nil || ctx.Err() != nil || grant.challenge != challenge {
		return errQuarantineUnavailable
	}
	return c.prepareReserved(ctx, grant)
}

func (c *quarantineClient) prepareReserved(ctx context.Context, g quarantineGrant) error {
	if ctx == nil || ctx.Err() != nil || c.transport == nil || !c.expected.valid() || len(c.issuer) != ed25519.PublicKeySize || g.challenge.scope != c.expected || g.challenge.expires <= time.Now().Unix() || !ed25519.Verify(c.issuer, grantBytes(g.challenge), g.signature[:]) {
		return errQuarantineUnavailable
	}
	proof, err := c.hostProof(ctx, consumeProofBytes(g))
	if err != nil {
		return errQuarantineUnavailable
	}
	if c.transport.consume(ctx, g, proof) != nil || ctx.Err() != nil || g.challenge.expires <= time.Now().Unix() {
		return errQuarantineUnavailable
	}
	if c.afterConsume != nil {
		c.afterConsume()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx.Err() != nil || g.challenge.expires <= time.Now().Unix() {
		return errQuarantineUnavailable
	}
	c.phase = quarantinePreparing
	return nil
}

func (c *quarantineClient) state() quarantineClientState {
	if c == nil {
		return quarantineClosed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.phase
}
