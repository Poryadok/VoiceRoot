package lfpconsentstore

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/storylfpadmission"
)

func fixture(t *testing.T) (*Store, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("LFP_TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("LFP_TEST_DATABASE_DSN isolated PostgreSQL fixture required")
	}
	ctx := context.Background()
	admin, e := pgxpool.New(ctx, dsn)
	require.NoError(t, e)
	schema := "lfp_" + uuid.New().String()[:8]
	_, e = admin.Exec(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, e)
	cfg, e := pgxpool.ParseConfig(dsn)
	require.NoError(t, e)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["application_name"] = schema
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, e)
	t.Cleanup(func() {
		pool.Close()
		_, e := admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		if e != nil {
			t.Error(e)
		}
		admin.Close()
	})
	for _, name := range []string{"000001_init.up.sql", "000002_visibility_audience.up.sql", "000003_hidden_from_feed.up.sql", "000004_archive_purge_outbox.up.sql", "000005_lfp_consent_journal.up.sql", "000006_lfp_actor_provenance.up.sql"} {
		b, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", name))
		if os.IsNotExist(e) && name == "000005_lfp_consent_journal.up.sql" {
			continue
		}
		require.NoError(t, e)
		_, e = pool.Exec(ctx, string(b))
		require.NoError(t, e)
	}
	id, author := uuid.New(), uuid.New()
	_, e = pool.Exec(ctx, `INSERT INTO stories(id,author_profile_id,type,is_looking_for_party,lfp_criteria,visibility,expires_at,archived_until) VALUES($1,$2,'text',true,'{"game_id":"test","mode":"pair"}','everyone',clock_timestamp()+interval '1 hour',clock_timestamp()+interval '31 days')`, id, author)
	require.NoError(t, e)
	// Synthetic admission is ONLY a local journal fixture. It is not installed
	// controller/credential/restore proof and has no production constructor.
	o := origin{environment: uuid.New(), installation: uuid.New(), incarnation: uuid.New(), database: [32]byte{1}}
	_, e = pool.Exec(ctx, `INSERT INTO story_lfp_admission_head(singleton,environment_id,installation_id,authority_incarnation,database_binding_sha256,phase) VALUES(true,$1,$2,$3,$4,'open')`, o.environment, o.installation, o.incarnation, o.database[:])
	require.NoError(t, e)
	s := &Store{Pool: pool, authority: fixtureAdmission{o: o}}
	_, e = s.PublishOffer(ctx, AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{1}})
	require.NoError(t, e)
	return s, id, author
}

type fixtureAdmission struct{ o origin }

func (a fixtureAdmission) enter(context.Context) (origin, func(), error) { return a.o, func() {}, nil }

type fixturePermit struct{ o storylfpadmission.Origin }

func (p fixturePermit) Origin() storylfpadmission.Origin { return p.o }
func (p fixturePermit) Release()                         {}

type fixtureSharedAdmission struct{ o storylfpadmission.Origin }

func (a fixtureSharedAdmission) Enter(context.Context) (storylfpadmission.Permit, error) {
	return fixturePermit{a.o}, nil
}

func TestJournalBarrierDrainsThroughActualCommit(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	barrier := storylfpadmission.NewBarrier(shared, fixtureSharedAdmission{shared})
	s = NewBoundStore(s.Pool, barrier)
	offer, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	// A deferred constraint trigger blocks the owning COMMIT itself, after all
	// journal INSERT work. Synthetic authority proves local composition only.
	_, e = s.Pool.Exec(ctx, `CREATE FUNCTION wait_lfp_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(2910005); RETURN NEW; END $$;
CREATE CONSTRAINT TRIGGER wait_lfp_commit AFTER INSERT ON story_lfp_consents DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION wait_lfp_commit()`)
	require.NoError(t, e)
	lock, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = lock.Rollback(ctx) }()
	_, e = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(2910005)`)
	require.NoError(t, e)
	result := make(chan error, 1)
	go func() { _, _, _, err := s.RecordResponse(ctx, response(offer)); result <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked int
		require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=current_setting('application_name') AND wait_event_type='Lock' AND lower(query)='commit'`).Scan(&blocked))
		if blocked > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("journal never reached actual deferred COMMIT lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, barrier.CloseAndDrain(canceled), context.Canceled)
	_, e = s.ObserveOffer(ctx, id)
	require.ErrorIs(t, e, ErrUnavailable)
	require.NoError(t, lock.Commit(ctx))
	select {
	case e = <-result:
		require.NoError(t, e)
	case <-time.After(3 * time.Second):
		t.Fatal("journal commit did not finish")
	}
	require.NoError(t, barrier.CloseAndDrain(ctx))
}

func TestJournalUncertainCommitNeverClaimsDrained(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	barrier := storylfpadmission.NewBarrier(shared, fixtureSharedAdmission{shared})
	s = NewBoundStore(s.Pool, barrier)
	offer, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, `CREATE FUNCTION wait_uncertain_commit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(2910006); RETURN NEW; END $$;
CREATE CONSTRAINT TRIGGER wait_uncertain_commit AFTER INSERT ON story_lfp_consents DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION wait_uncertain_commit()`)
	require.NoError(t, e)
	lock, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = lock.Rollback(ctx) }()
	_, e = lock.Exec(ctx, `SELECT pg_advisory_xact_lock(2910006)`)
	require.NoError(t, e)
	caller, cancelCaller := context.WithCancel(ctx)
	defer cancelCaller()
	result := make(chan error, 1)
	go func() {
		_, b, h, e := s.RecordResponse(caller, response(offer))
		if b != nil || h != [32]byte{} {
			e = errors.New("ambiguous commit returned material")
		}
		result <- e
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked int
		require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=current_setting('application_name') AND wait_event_type='Lock' AND lower(query)='commit'`).Scan(&blocked))
		if blocked > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("caller never reached actual blocked COMMIT")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancelCaller()
	select {
	case e = <-result:
		require.ErrorIs(t, e, ErrUnavailable)
	case <-time.After(3 * time.Second):
		t.Fatal("canceled caller never returned")
	}
	drain, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, barrier.CloseAndDrain(drain), context.DeadlineExceeded, "unconfirmed native COMMIT must retain its permit")
	require.NoError(t, lock.Commit(ctx))
}

func TestJournalUncertainAdmissionRollbackNeverClaimsDrained(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	barrier := storylfpadmission.NewBarrier(shared, fixtureSharedAdmission{shared})
	s = NewBoundStore(s.Pool, barrier)
	lock, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = lock.Rollback(ctx) }()
	_, e = lock.Exec(ctx, `SELECT singleton FROM story_lfp_admission_head FOR UPDATE`)
	require.NoError(t, e)
	caller, cancelCaller := context.WithCancel(ctx)
	defer cancelCaller()
	result := make(chan error, 1)
	go func() { _, e := s.ObserveOffer(caller, id); result <- e }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked int
		require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=current_setting('application_name') AND wait_event_type='Lock' AND query LIKE '%FROM story_lfp_admission_head%FOR SHARE%'`).Scan(&blocked))
		if blocked > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("caller never reached actual admission row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancelCaller()
	select {
	case e = <-result:
		require.ErrorIs(t, e, ErrUnavailable)
	case <-time.After(3 * time.Second):
		t.Fatal("canceled admission caller never returned")
	}
	drain, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, barrier.CloseAndDrain(drain), context.DeadlineExceeded)
	require.NoError(t, lock.Commit(ctx))
}

func TestJournalConfirmedRejectionRollbackReleasesPermit(t *testing.T) {
	s, _, _ := fixture(t)
	ctx := context.Background()
	o := s.authority.(fixtureAdmission).o
	shared, e := storylfpadmission.NewOrigin(o.environment, o.installation, o.incarnation, o.database)
	require.NoError(t, e)
	barrier := storylfpadmission.NewBarrier(shared, fixtureSharedAdmission{shared})
	s = NewBoundStore(s.Pool, barrier)
	_, e = s.ObserveOffer(ctx, uuid.New())
	require.ErrorIs(t, e, ErrStale)
	drain, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	require.NoError(t, barrier.CloseAndDrain(drain))
}

func publish(t *testing.T, s *Store, id uuid.UUID) Offer {
	t.Helper()
	var author uuid.UUID
	require.NoError(t, s.Pool.QueryRow(context.Background(), "SELECT author_profile_id FROM stories WHERE id=$1", id).Scan(&author))
	o, e := s.PublishOffer(context.Background(), AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{1}})
	require.NoError(t, e)
	return o
}
func response(o Offer) Response {
	return Response{OperationID: uuid.New(), AccountID: uuid.New(), ProfileID: uuid.New(), SessionEpoch: 7, Type: lfpconsent.Join, RequestSHA256: [32]byte{1}, Offer: o}
}

func TestJournalOriginalConsentReplay(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	first, b, h, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	require.Equal(t, r.AccountID, first.ResponderAccountID)
	require.Equal(t, r.ProfileID, first.ResponderProfileID)
	require.Equal(t, int64(7), first.ResponderSessionEpoch)
	// A fresh valid interactive session can read the same operation; it never
	// replaces the original consent epoch or creates a new consent identity.
	r.SessionEpoch = 8
	again, bb, hh, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	require.Equal(t, first, again)
	require.Equal(t, b, bb)
	require.Equal(t, h, hh)
	r.Type = lfpconsent.Invite
	_, _, _, e = s.RecordResponse(ctx, r)
	require.ErrorIs(t, e, ErrConflict)
	r.Type = lfpconsent.Join
	r.AccountID = uuid.New()
	_, _, _, e = s.RecordResponse(ctx, r)
	require.ErrorIs(t, e, ErrConflict)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consents").Scan(&count))
	require.Equal(t, 1, count)
}

func TestJournalOfferRevisionNeverReused(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	a, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, `UPDATE stories SET lfp_criteria='{"game_id":"other","mode":"pair"}' WHERE id=$1`, id)
	require.NoError(t, e)
	_, b, h, e := s.RecordResponse(ctx, response(a))
	require.ErrorIs(t, e, ErrStale)
	require.Nil(t, b)
	require.Equal(t, [32]byte{}, h)
	changed := publish(t, s, id)
	require.Equal(t, a.Revision+1, changed.Revision)
	require.NotEqual(t, a.PayloadSHA256, changed.PayloadSHA256)
	_, e = s.Pool.Exec(ctx, `UPDATE stories SET lfp_criteria='{"game_id":"test","mode":"pair"}' WHERE id=$1`, id)
	require.NoError(t, e)
	back := publish(t, s, id)
	require.Equal(t, a.Revision+2, back.Revision)
	require.Equal(t, a.Incarnation, back.Incarnation)
	require.NotEqual(t, a.PayloadSHA256, back.PayloadSHA256)
}

func TestJournalConcurrentExactOperation(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	var wg sync.WaitGroup
	results := make(chan []byte, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); _, b, _, e := s.RecordResponse(ctx, r); results <- b; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var first []byte
	for b := range results {
		if first == nil {
			first = b
		}
		require.True(t, bytes.Equal(first, b))
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consents").Scan(&count))
	require.Equal(t, 1, count)
}

func TestJournalExpiryAfterHeldStoryLock(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	_, e := s.Pool.Exec(ctx, "UPDATE stories SET expires_at=clock_timestamp()+interval '2 seconds' WHERE id=$1", id)
	require.NoError(t, e)
	o := publish(t, s, id)
	tx, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback(ctx) }()
	_, e = tx.Exec(ctx, "SELECT id FROM stories WHERE id=$1 FOR UPDATE", id)
	require.NoError(t, e)
	result := make(chan error, 1)
	go func() {
		_, b, h, e := s.RecordResponse(ctx, response(o))
		if b != nil || h != [32]byte{} {
			e = errors.New("partial material escaped expired journal")
		}
		result <- e
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var blocked int
		require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE application_name=current_setting('application_name') AND wait_event_type='Lock' AND query LIKE '%FROM stories%FOR UPDATE%'`).Scan(&blocked))
		if blocked > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writer never reached actual Story row lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Actual PG lock wait, with DB time crossing the offer's immutable deadline.
	_, e = tx.Exec(ctx, "SELECT pg_sleep(2.2)")
	require.NoError(t, e)
	require.NoError(t, tx.Commit(ctx))
	select {
	case e := <-result:
		require.ErrorIs(t, e, ErrStale)
	case <-time.After(3 * time.Second):
		t.Fatal("writer did not finish")
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consents").Scan(&count))
	require.Zero(t, count)
}

func TestJournalRegistrationPreservesOriginalCapture(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	c, _, consentHash, e := s.RecordResponse(ctx, response(o))
	require.NoError(t, e)
	// Producer registration operation is exactly consent_id. Transport request_id
	// and JTI are fresh per attempt, but this domain operation never rotates.
	r := lfpconsent.Registration{OperationID: c.ConsentID, ConsentID: c.ConsentID, ConsentSHA256: consentHash, RegistrationID: uuid.New(), Generation: 3, CapturedSearchIntentSHA256: [32]byte{5}}
	_, h, e := lfpconsent.EncodeRegistration(r)
	require.NoError(t, e)
	require.NoError(t, s.SaveRegistration(ctx, r, h))
	require.NoError(t, s.SaveRegistration(ctx, r, h))
	got, gh, e := s.ReadRegistration(ctx, c.ConsentID)
	require.NoError(t, e)
	require.Equal(t, r, got)
	require.Equal(t, h, gh)
	changed := r
	changed.Generation++
	_, hh, e := lfpconsent.EncodeRegistration(changed)
	require.NoError(t, e)
	require.ErrorIs(t, s.SaveRegistration(ctx, changed, hh), ErrConflict)
	changed = r
	changed.CapturedSearchIntentSHA256[0]++
	_, hh, e = lfpconsent.EncodeRegistration(changed)
	require.NoError(t, e)
	require.ErrorIs(t, s.SaveRegistration(ctx, changed, hh), ErrConflict)
	wrongHash := h
	wrongHash[0]++
	require.ErrorIs(t, s.SaveRegistration(ctx, r, wrongHash), ErrUnavailable)
	foreign := r
	foreign.ConsentID = uuid.New()
	foreign.OperationID = foreign.ConsentID
	_, hh, e = lfpconsent.EncodeRegistration(foreign)
	require.NoError(t, e)
	require.ErrorIs(t, s.SaveRegistration(ctx, foreign, hh), ErrConflict)
}

func TestJournalImmutableEvidenceAndGuardedRollback(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	c, b, h, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_consents SET canonical_consent=canonical_consent WHERE consent_id=$1", c.ConsentID)
	require.Error(t, e)
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_offers SET revision=revision WHERE story_id=$1", id)
	require.Error(t, e)
	down, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", "000005_lfp_consent_journal.down.sql"))
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, string(down))
	require.Error(t, e)
	// Natural expiry/deletion cannot remint this existing operation. This is
	// historical receipt readback only; no new registration was performed.
	_, e = s.Pool.Exec(ctx, "UPDATE stories SET deleted_at=clock_timestamp(),expires_at=clock_timestamp()-interval '1 second' WHERE id=$1", id)
	require.NoError(t, e)
	again, bb, hh, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	require.Equal(t, c, again)
	require.Equal(t, b, bb)
	require.Equal(t, h, hh)
	_, _, _, e = s.RecordResponse(ctx, response(o))
	require.ErrorIs(t, e, ErrStale)
}

func TestJournalMissingAdmissionAndRetiredOrigin(t *testing.T) {
	s, id, author := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	_, _, _, e = NewBoundStore(s.Pool, nil).RecordResponse(ctx, r)
	require.ErrorIs(t, e, ErrUnavailable)
	_, e = NewBoundStore(s.Pool, nil).PublishOffer(ctx, AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{1}})
	require.ErrorIs(t, e, ErrUnavailable)
	c, b, h, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	old := s.authority.(fixtureAdmission).o
	next := old
	next.incarnation = uuid.New()
	// Explicit synthetic local rebind; does NOT prove real close/drain/custody.
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET authority_incarnation=$1", next.incarnation)
	require.NoError(t, e)
	_, _, _, e = s.RecordResponse(ctx, r)
	require.ErrorIs(t, e, ErrStale)
	s.authority = fixtureAdmission{o: next}
	_, e = s.ObserveOffer(ctx, id)
	require.ErrorIs(t, e, ErrStale)
	_, _, _, e = s.RecordResponse(ctx, r)
	require.ErrorIs(t, e, ErrStale)
	fresh := publish(t, s, id)
	require.NotEqual(t, o.Incarnation, fresh.Incarnation)
	require.Equal(t, uint64(1), fresh.Revision)
	require.NotEqual(t, o.PayloadSHA256, fresh.PayloadSHA256)
	_, _, _, e = s.RecordResponse(ctx, response(fresh))
	require.NoError(t, e)
	// Original private bytes remain history, not silently re-signed/rebound.
	var saved, hash []byte
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT canonical_consent,consent_sha256 FROM story_lfp_consents WHERE consent_id=$1", c.ConsentID).Scan(&saved, &hash))
	require.Equal(t, b, saved)
	require.Equal(t, h[:], hash)
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET phase='closed'")
	require.NoError(t, e)
	_, e = s.ObserveOffer(ctx, id)
	require.ErrorIs(t, e, ErrUnavailable)
}

func TestJournalRejectsOldCanonicalBytesRelabelledByMetadata(t *testing.T) {
	s, id, _ := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	c, b, h, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	old := s.authority.(fixtureAdmission).o
	next := old
	next.incarnation = uuid.New()
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET authority_incarnation=$1", next.incarnation)
	require.NoError(t, e)
	s.authority = fixtureAdmission{o: next}
	fresh := publish(t, s, id)
	// Deliberately malformed private import fixture: immutable v1 bytes are old,
	// while relational selectors now claim a new-cohort offer. This is NOT an
	// authorized restore/rebind operation and must never promote the old bytes.
	_, e = s.Pool.Exec(ctx, "DELETE FROM story_lfp_consents WHERE consent_id=$1", c.ConsentID)
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, `INSERT INTO story_lfp_consents(response_operation_id,consent_id,story_id,incarnation,offer_revision,responder_account_id,responder_profile_id,canonical_consent,consent_sha256) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, c.ResponseOperationID, c.ConsentID, c.StoryID, fresh.Incarnation, int64(fresh.Revision), c.ResponderAccountID, c.ResponderProfileID, b, h[:])
	require.NoError(t, e)
	t.Run("response", func(t *testing.T) {
		_, bb, hh, e := s.RecordResponse(ctx, r)
		require.ErrorIs(t, e, ErrUnavailable)
		require.Nil(t, bb)
		require.Equal(t, [32]byte{}, hh)
	})
	t.Run("ack", func(t *testing.T) {
		reg := lfpconsent.Registration{OperationID: c.ConsentID, ConsentID: c.ConsentID, ConsentSHA256: h, RegistrationID: uuid.New(), Generation: 1, CapturedSearchIntentSHA256: [32]byte{4}}
		_, rh, e := lfpconsent.EncodeRegistration(reg)
		require.NoError(t, e)
		require.ErrorIs(t, s.SaveRegistration(ctx, reg, rh), ErrUnavailable)
	})
}
