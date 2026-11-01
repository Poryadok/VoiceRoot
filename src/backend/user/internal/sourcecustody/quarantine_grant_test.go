package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type lostConsumeResponse struct{ authority *quarantineAuthority }

type quarantineTestSigner struct{ key ed25519.PrivateKey }

func (s quarantineTestSigner) publicKey() ed25519.PublicKey {
	return s.key.Public().(ed25519.PublicKey)
}
func (s quarantineTestSigner) sign(_ context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.key, message), nil
}

type invalidQuarantineSigner struct {
	public ed25519.PublicKey
	wrong  ed25519.PrivateKey
}

func (s invalidQuarantineSigner) publicKey() ed25519.PublicKey { return s.public }
func (s invalidQuarantineSigner) sign(_ context.Context, message []byte) ([]byte, error) {
	return ed25519.Sign(s.wrong, message), nil
}

func (l lostConsumeResponse) consume(ctx context.Context, g quarantineGrant, proof []byte) error {
	if err := l.authority.consume(ctx, g, proof); err != nil {
		return err
	}
	return errors.New("response lost after commit")
}

func TestQuarantineTransactionalAuthority(t *testing.T) {
	dsn := os.Getenv("VOICE_TEST_QUARANTINE_DSN")
	if dsn == "" {
		t.Skip("requires isolated disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var database string
	if pool.QueryRow(ctx, "SELECT current_database()").Scan(&database) != nil || database != "voice_quarantine_test" {
		t.Fatal("wrong test database")
	}
	_, err = pool.Exec(ctx, `CREATE TABLE public.quarantine_generation_ledger (
		source bytea PRIMARY KEY, generation bigint NOT NULL, challenge bytea NOT NULL,
		expires bigint NOT NULL, state text NOT NULL, grant_hash bytea)`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), `DROP TABLE public.quarantine_generation_ledger`) }()
	fixture := func() (*quarantineAuthority, quarantineScope, ed25519.PrivateKey, ed25519.PublicKey) {
		hostPublic, hostPrivate, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		var scope quarantineScope
		if _, err = rand.Read(scope.source[:]); err != nil {
			t.Fatal(err)
		}
		scope.generation = 1
		scope.host = sha256.Sum256(hostPublic)
		scope.boot[0] = 1
		scope.executable[0] = 2
		scope.runtime[0] = 3
		scope.data[0] = 4
		a, err := newQuarantineAuthority(pool, scope, hostPublic, issuerPrivate)
		if err != nil {
			t.Fatal(err)
		}
		return a, scope, hostPrivate, issuerPublic
	}
	grantFor := func(a *quarantineAuthority, key ed25519.PrivateKey) quarantineGrant {
		challenge, err := a.begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		proof := ed25519.Sign(key, challengeProofBytes(challenge))
		grant, err := a.issue(ctx, challenge, proof)
		if err != nil {
			t.Fatal(err)
		}
		return grant
	}
	t.Run("invalid issuer key rejected before begin", func(t *testing.T) {
		a, scope, _, _ := fixture()
		bad := append(ed25519.PrivateKey(nil), a.issuer...)
		bad[len(bad)-1] ^= 1
		if _, err := newQuarantineAuthority(pool, scope, a.host, bad); err == nil {
			t.Fatal("malformed issuer accepted")
		}
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.quarantine_generation_ledger WHERE source=$1`, scope.source[:]).Scan(&count); err != nil || count != 0 {
			t.Fatalf("source row changed: %d %v", count, err)
		}
	})
	t.Run("boot client completes signed challenge", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		c := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if c.state() != quarantineClosed {
			t.Fatal("client did not start closed")
		}
		if err := c.start(ctx); err != nil || c.state() != quarantinePreparing {
			t.Fatalf("start: %v", err)
		}
		if c.start(ctx) == nil {
			t.Fatal("client reused its attempt")
		}
		restored := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if restored.start(ctx) == nil || restored.state() != quarantineClosed {
			t.Fatal("restored boot replay accepted")
		}
	})
	t.Run("concurrent fresh boots and stale generation", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		const n = 16
		var wg sync.WaitGroup
		results := make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results <- newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a).start(ctx)
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			if err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("boot winners=%d", wins)
		}
		advanced := scope
		advanced.generation++
		next, err := newQuarantineAuthority(pool, advanced, a.host, a.issuer)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = next.begin(ctx); err == nil {
			t.Fatal("unreviewed generation advance accepted")
		}
	})
	t.Run("concurrent consume and restored client", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		g := grantFor(a, host)
		const n = 16
		var wg sync.WaitGroup
		results := make(chan error, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
				results <- c.prepare(ctx, g)
			}()
		}
		wg.Wait()
		close(results)
		wins := 0
		for err := range results {
			if err == nil {
				wins++
			}
		}
		if wins != 1 {
			t.Fatalf("consume winners=%d", wins)
		}
		restored := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if restored.state() != quarantineClosed || restored.prepare(ctx, g) == nil || restored.state() != quarantineClosed {
			t.Fatal("replayed grant accepted after restart")
		}
	})
	t.Run("lost response permanently denies client", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		g := grantFor(a, host)
		c := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, lostConsumeResponse{a})
		if c.prepare(ctx, g) == nil || c.state() != quarantineClosed {
			t.Fatal("ambiguous consume accepted")
		}
		if c.prepare(ctx, g) == nil {
			t.Fatal("retry accepted")
		}
		fresh := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if fresh.prepare(ctx, g) == nil {
			t.Fatal("committed grant replay accepted")
		}
	})
	t.Run("cancellation after consume cannot enter preparing", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		g := grantFor(a, host)
		client := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		entered, release := make(chan struct{}), make(chan struct{})
		client.afterConsume = func() { close(entered); <-release }
		call, stop := context.WithCancel(ctx)
		result := make(chan error, 1)
		go func() { result <- client.prepare(call, g) }()
		<-entered
		stop()
		close(release)
		if err := <-result; err == nil || client.state() != quarantineClosed {
			t.Fatal("canceled attempt entered preparing")
		}
		if client.prepare(ctx, g) == nil {
			t.Fatal("canceled attempt retried")
		}
	})
	t.Run("wrong boot generation and manifest", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		g := grantFor(a, host)
		for _, changed := range []quarantineScope{func() quarantineScope { v := scope; v.generation++; return v }(), func() quarantineScope { v := scope; v.boot[0]++; return v }(), func() quarantineScope { v := scope; v.data[0]++; return v }(), func() quarantineScope { v := scope; v.executable[0]++; return v }(), func() quarantineScope { v := scope; v.runtime[0]++; return v }(), func() quarantineScope { v := scope; v.source[0]++; return v }()} {
			c := newQuarantineClient(changed, issuer, quarantineTestSigner{host}, a)
			if c.prepare(ctx, g) == nil || c.state() != quarantineClosed {
				t.Fatal("scope mismatch accepted")
			}
		}
		c := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if err := c.prepare(ctx, g); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("wrong host proof leaves challenge unusable by attacker", func(t *testing.T) {
		a, _, host, _ := fixture()
		ch, err := a.begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, wrong, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.issue(ctx, ch, ed25519.Sign(wrong, challengeProofBytes(ch))); err == nil {
			t.Fatal("wrong host accepted")
		}
		if _, err = a.issue(ctx, ch, ed25519.Sign(host, challengeProofBytes(ch))); err != nil {
			t.Fatal(err)
		}
		if _, err = a.issue(ctx, ch, ed25519.Sign(host, challengeProofBytes(ch))); err == nil {
			t.Fatal("challenge replay accepted")
		}
	})
	t.Run("wrong consume proof and expired ledger", func(t *testing.T) {
		a, scope, host, _ := fixture()
		g := grantFor(a, host)
		_, wrong, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		if a.consume(ctx, g, ed25519.Sign(wrong, consumeProofBytes(g))) == nil {
			t.Fatal("wrong consume host accepted")
		}
		// Keep the signed grant, challenge bytes, expiry and row hash consistent;
		// rejection must come from the authority's database-clock predicate.
		g.challenge.expires = time.Now().Unix() - 10
		copy(g.signature[:], ed25519.Sign(a.issuer, grantBytes(g.challenge)))
		hash := grantHash(g)
		_, err = pool.Exec(ctx, `UPDATE public.quarantine_generation_ledger SET expires=$1,challenge=$2,grant_hash=$3 WHERE source=$4`, g.challenge.expires, g.challenge.bytes(), hash[:], scope.source[:])
		if err != nil {
			t.Fatal(err)
		}
		if a.consume(ctx, g, ed25519.Sign(host, consumeProofBytes(g))) == nil {
			t.Fatal("expired ledger accepted")
		}
	})
	t.Run("ambient temp relation and operators cannot redirect ledger", func(t *testing.T) {
		_, err := pool.Exec(ctx, `CREATE SCHEMA quarantine_shadow`)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _, _ = pool.Exec(context.Background(), `DROP SCHEMA quarantine_shadow CASCADE`) }()
		_, err = pool.Exec(ctx, `CREATE FUNCTION quarantine_shadow.always_equal(bytea,bytea) RETURNS boolean LANGUAGE sql IMMUTABLE AS 'SELECT true'`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `CREATE OPERATOR quarantine_shadow.= (LEFTARG=bytea, RIGHTARG=bytea, FUNCTION=quarantine_shadow.always_equal)`)
		if err != nil {
			t.Fatal(err)
		}
		config, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		config.MaxConns = 1
		shadow, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			t.Fatal(err)
		}
		defer shadow.Close()
		conn, err := shadow.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `CREATE TEMP TABLE quarantine_generation_ledger (LIKE public.quarantine_generation_ledger INCLUDING ALL)`); err != nil {
			t.Fatal(err)
		}
		if _, err = conn.Exec(ctx, `SET search_path=quarantine_shadow,pg_catalog`); err != nil {
			t.Fatal(err)
		}
		var hostile bool
		if err = conn.QueryRow(ctx, `SELECT '\x01'::bytea = '\x02'::bytea`).Scan(&hostile); err != nil || !hostile {
			t.Fatalf("operator shadow not active: %v %v", hostile, err)
		}
		conn.Release()
		a, scope, host, issuer := fixture()
		a.pool = shadow
		client := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if err = client.start(ctx); err != nil {
			t.Fatal(err)
		}
		var tempCount int
		if err = shadow.QueryRow(ctx, `SELECT count(*) FROM pg_temp.quarantine_generation_ledger`).Scan(&tempCount); err != nil || tempCount != 0 {
			t.Fatalf("temp ledger used: %d %v", tempCount, err)
		}
	})
	t.Run("host signer must prove pinned key", func(t *testing.T) {
		a, scope, host, issuer := fixture()
		g := grantFor(a, host)
		_, wrong, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		invalid := invalidQuarantineSigner{public: host.Public().(ed25519.PublicKey), wrong: wrong}
		client := newQuarantineClient(scope, issuer, invalid, a)
		if client.prepare(ctx, g) == nil || client.state() != quarantineClosed {
			t.Fatal("forged host proof accepted")
		}
		valid := newQuarantineClient(scope, issuer, quarantineTestSigner{host}, a)
		if err := valid.prepare(ctx, g); err != nil {
			t.Fatalf("invalid signer consumed grant: %v", err)
		}
	})
}
