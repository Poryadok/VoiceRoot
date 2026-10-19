package lfpconsentstore

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/storylfpadmission"
)

func actorPublication(t *testing.T, s *Store, id, author uuid.UUID) (Offer, storylfpadmission.Origin) {
	t.Helper()
	expected := validationFixture(t, s).Origin()
	a := AuthorAction{OperationID: uuid.New(), StoryID: id, ProfileID: author, RequestSHA256: [32]byte{1}}
	actor := lfpconsent.Actor{ActionKind: lfpconsent.PublishOffer, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: lfpconsent.Guest, AccountID: uuid.New(), ProfileID: author, SessionEpoch: 7, OperationID: a.OperationID, Issuer: "gateway", Audience: "story", FullRPC: lfpconsent.PublishOfferRPC, PrincipalRequestSHA256: a.RequestSHA256}
	o, e := s.PublishActorOffer(context.Background(), a, actor, expected, time.Now().Add(time.Minute))
	require.NoError(t, e)
	return o, expected
}

func actorResponse(r Response) lfpconsent.Actor {
	return lfpconsent.Actor{ActionKind: lfpconsent.Respond, ActorKind: lfpconsent.HumanDelegatedUser, AccountType: lfpconsent.Guest, AccountID: r.AccountID, ProfileID: r.ProfileID, SessionEpoch: r.SessionEpoch, OperationID: r.OperationID, Issuer: "gateway", Audience: "story", FullRPC: lfpconsent.RespondRPC, PrincipalRequestSHA256: r.RequestSHA256}
}

func TestActorJournalPreservesOriginalGuestOnConvertedReplay(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	first, e := s.RecordActorResponse(context.Background(), r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	require.Equal(t, lfpconsent.Guest, first.Actor().AccountType)
	r.SessionEpoch++
	converted := actorResponse(r)
	converted.AccountType = lfpconsent.Regular
	again, e := s.RecordActorResponse(context.Background(), r, converted, origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	require.Equal(t, first.Actor(), again.Actor())
	require.Equal(t, first.QualifiedSHA256(), again.QualifiedSHA256())
	require.Equal(t, first.ConsentBytes(), again.ConsentBytes())
	var count int
	require.NoError(t, s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM story_lfp_consent_actors").Scan(&count))
	require.Equal(t, 1, count)
	loaded, e := s.ReadActorConsentForDelivery(context.Background(), first.Consent().ConsentID, first.QualifiedSHA256(), origin)
	require.NoError(t, e)
	require.Equal(t, first.Actor(), loaded.Actor())
}

func TestActorJournalNeverEnrichesBareReceiptOrPublication(t *testing.T) {
	s, id, author := fixture(t)
	ctx := context.Background()
	o, e := s.ObserveOffer(ctx, id)
	require.NoError(t, e)
	r := response(o)
	c, _, _, e := s.RecordResponse(ctx, r)
	require.NoError(t, e)
	origin := validationFixture(t, s).Origin()
	_, e = s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.ErrorIs(t, e, ErrUnavailable)
	_, e = s.ReadActorConsentForDelivery(ctx, c.ConsentID, [32]byte{1}, origin)
	require.ErrorIs(t, e, ErrUnavailable)
	r = response(o)
	_, e = s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.ErrorIs(t, e, ErrUnavailable, "legacy author publication cannot qualify")
	actorPublication(t, s, id, author)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consent_actors").Scan(&count))
	require.Zero(t, count)
}

func TestActorJournalAckCommitsExactQualifiedEnvelope(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	envelope, e := s.RecordActorResponse(context.Background(), r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	c := envelope.Consent()
	reg := lfpconsent.Registration{OperationID: c.ConsentID, ConsentID: c.ConsentID, ConsentSHA256: envelope.ConsentSHA256(), RegistrationID: uuid.New(), Generation: 3, CapturedSearchIntentSHA256: [32]byte{5}}
	_, rh, e := lfpconsent.EncodeRegistration(reg)
	require.NoError(t, e)
	bound, e := lfpconsent.HashActorBoundRegistration(rh, envelope.QualifiedSHA256())
	require.NoError(t, e)
	ack := ActorRegistration{Registration: reg, Origin: origin, QualifiedSHA256: envelope.QualifiedSHA256(), BoundSHA256: bound}
	require.NoError(t, s.SaveActorRegistration(context.Background(), ack))
	got, e := s.ReadActorRegistration(context.Background(), c.ConsentID, envelope.QualifiedSHA256())
	require.NoError(t, e)
	require.Equal(t, ack, got)
	bad := ack
	bad.QualifiedSHA256[0]++
	require.Error(t, s.SaveActorRegistration(context.Background(), bad))
	bad = ack
	bad.Registration.Generation++
	_, rh, e = lfpconsent.EncodeRegistration(bad.Registration)
	require.NoError(t, e)
	bad.BoundSHA256, e = lfpconsent.HashActorBoundRegistration(rh, bad.QualifiedSHA256)
	require.NoError(t, e)
	require.ErrorIs(t, s.SaveActorRegistration(context.Background(), bad), ErrConflict)
}

func TestActorOfferContextBindsOriginalAudienceSnapshot(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	ctx := context.Background()
	bound, e := s.ReadActorOfferContext(ctx, id, origin)
	require.NoError(t, e)
	require.Equal(t, o, bound.Offer())
	require.Equal(t, "everyone", bound.Visibility())
	_, e = s.Pool.Exec(ctx, `UPDATE stories SET visibility='friends' WHERE id=$1`, id)
	require.NoError(t, e)
	_, e = s.ReadActorOfferContext(ctx, id, origin)
	require.ErrorIs(t, e, ErrStale)
	// This immutable audience cannot be switched to B while the final owner
	// state returns to A. Fresh remote checks evaluate the captured A snapshot.
	require.Equal(t, "everyone", bound.Visibility())
	_, e = s.Pool.Exec(ctx, `UPDATE stories SET visibility='everyone' WHERE id=$1`, id)
	require.NoError(t, e)
	again, e := s.ReadActorOfferContext(ctx, id, origin)
	require.NoError(t, e)
	require.Equal(t, bound.Visibility(), again.Visibility())
	require.Equal(t, bound.Offer(), again.Offer())
}

func TestActorConsentContextBindsCurrentOfferAndOriginalViewer(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	ctx := context.Background()
	envelope, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	loaded, bound, e := s.ReadActorConsentContext(ctx, envelope.Consent().ConsentID, envelope.QualifiedSHA256(), origin)
	require.NoError(t, e)
	require.Equal(t, envelope.Actor(), loaded.Actor())
	require.Equal(t, lfpconsent.Guest, loaded.Actor().AccountType)
	require.Equal(t, o, bound.Offer())
	require.Equal(t, o.CriteriaSHA256, sha256.Sum256([]byte(bound.CriteriaJSON())))
	wrong := envelope.QualifiedSHA256()
	wrong[0]++
	_, _, e = s.ReadActorConsentContext(ctx, envelope.Consent().ConsentID, wrong, origin)
	require.ErrorIs(t, e, ErrConflict)
	_, e = s.Pool.Exec(ctx, "UPDATE stories SET visibility='friends' WHERE id=$1", id)
	require.NoError(t, e)
	_, _, e = s.ReadActorConsentContext(ctx, envelope.Consent().ConsentID, envelope.QualifiedSHA256(), origin)
	require.ErrorIs(t, e, ErrStale)
}

func TestActorReplayLookupNeverRenewsOrNeedsCurrentOffer(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	ctx := context.Background()
	_, found, e := s.FindActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	require.False(t, found)
	original, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, "UPDATE stories SET visibility='friends',hidden_from_feed_at=clock_timestamp() WHERE id=$1", id)
	require.NoError(t, e)
	r.SessionEpoch++
	converted := actorResponse(r)
	converted.AccountType = lfpconsent.Regular
	again, found, e := s.FindActorResponse(ctx, r, converted, origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	require.True(t, found)
	require.Equal(t, original.Actor(), again.Actor())
	require.Equal(t, original.QualifiedSHA256(), again.QualifiedSHA256())
	r.RequestSHA256[0]++
	converted.PrincipalRequestSHA256 = r.RequestSHA256
	_, _, e = s.FindActorResponse(ctx, r, converted, origin, time.Now().Add(time.Minute))
	require.ErrorIs(t, e, ErrConflict)
}

func TestActorJournalFailureRollsBackAllNewEvidence(t *testing.T) {
	for _, stage := range []string{"consent", "ack"} {
		t.Run(stage, func(t *testing.T) {
			s, id, author := fixture(t)
			o, origin := actorPublication(t, s, id, author)
			r := response(o)
			ctx := context.Background()
			table := "story_lfp_consent_actors"
			if stage == "ack" {
				table = "story_lfp_actor_bound_acks"
			}
			if stage == "consent" {
				_, e := s.Pool.Exec(ctx, `CREATE FUNCTION reject_actor() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture unavailable'; END $$; CREATE TRIGGER reject_actor BEFORE INSERT ON story_lfp_consent_actors FOR EACH ROW EXECUTE FUNCTION reject_actor()`)
				require.NoError(t, e)
				envelope, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
				require.ErrorIs(t, e, ErrUnavailable)
				require.False(t, envelope.Valid())
				var count int
				require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consents").Scan(&count))
				require.Zero(t, count)
			} else {
				envelope, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
				require.NoError(t, e)
				reg := lfpconsent.Registration{OperationID: envelope.Consent().ConsentID, ConsentID: envelope.Consent().ConsentID, ConsentSHA256: envelope.ConsentSHA256(), RegistrationID: uuid.New(), Generation: 3, CapturedSearchIntentSHA256: [32]byte{5}}
				_, rh, e := lfpconsent.EncodeRegistration(reg)
				require.NoError(t, e)
				bound, e := lfpconsent.HashActorBoundRegistration(rh, envelope.QualifiedSHA256())
				require.NoError(t, e)
				ack := ActorRegistration{Registration: reg, Origin: origin, QualifiedSHA256: envelope.QualifiedSHA256(), BoundSHA256: bound}
				_, e = s.Pool.Exec(ctx, `CREATE FUNCTION reject_actor() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture unavailable'; END $$; CREATE TRIGGER reject_actor BEFORE INSERT ON story_lfp_actor_bound_acks FOR EACH ROW EXECUTE FUNCTION reject_actor()`)
				require.NoError(t, e)
				require.ErrorIs(t, s.SaveActorRegistration(ctx, ack), ErrUnavailable)
				var count int
				require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_registration_acks").Scan(&count))
				require.Zero(t, count)
			}
			var count int
			require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestActorJournalConcurrentResponseKeepsOneSidecar(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	var wg sync.WaitGroup
	values := make(chan lfpconsent.Envelope, 8)
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := s.RecordActorResponse(context.Background(), r, actorResponse(r), origin, time.Now().Add(time.Minute))
			values <- v
			errs <- e
		}()
	}
	wg.Wait()
	close(values)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var first [32]byte
	for v := range values {
		if first == [32]byte{} {
			first = v.QualifiedSHA256()
		}
		require.Equal(t, first, v.QualifiedSHA256())
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM story_lfp_consent_actors").Scan(&count))
	require.Equal(t, 1, count)
}

func TestActorJournalOriginalCohortCannotBeRelabelled(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	ctx := context.Background()
	envelope, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, time.Now().Add(time.Minute))
	require.NoError(t, e)
	next := s.authority.(fixtureAdmission).o
	next.incarnation = uuid.New()
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_admission_head SET authority_incarnation=$1", next.incarnation)
	require.NoError(t, e)
	s.authority = fixtureAdmission{next}
	newOrigin := validationFixture(t, s).Origin()
	_, e = s.ReadActorConsentForDelivery(ctx, envelope.Consent().ConsentID, envelope.QualifiedSHA256(), newOrigin)
	require.ErrorIs(t, e, ErrStale)
	_, e = s.RecordActorResponse(ctx, r, actorResponse(r), newOrigin, time.Now().Add(time.Minute))
	require.ErrorIs(t, e, ErrStale)
	var saved []byte
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT canonical_actor FROM story_lfp_consent_actors WHERE consent_id=$1", envelope.Consent().ConsentID).Scan(&saved))
	require.Equal(t, envelope.ActorBytes(), saved)
	_, e = s.Pool.Exec(ctx, "UPDATE story_lfp_consent_actors SET canonical_actor=canonical_actor")
	require.Error(t, e)
	down, e := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "story_db", "000006_lfp_actor_provenance.down.sql"))
	require.NoError(t, e)
	_, e = s.Pool.Exec(ctx, string(down))
	require.Error(t, e)
}

func TestActorJournalCredentialExpiryRecheckedAfterRowLock(t *testing.T) {
	s, id, author := fixture(t)
	o, origin := actorPublication(t, s, id, author)
	r := response(o)
	ctx := context.Background()
	lock, e := s.Pool.Begin(ctx)
	require.NoError(t, e)
	defer func() { _ = lock.Rollback(ctx) }()
	_, e = lock.Exec(ctx, "SELECT id FROM stories WHERE id=$1 FOR UPDATE", id)
	require.NoError(t, e)
	var now time.Time
	require.NoError(t, lock.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now))
	result := make(chan error, 1)
	go func() {
		v, e := s.RecordActorResponse(ctx, r, actorResponse(r), origin, now.Add(250*time.Millisecond))
		if v.Valid() {
			e = nil
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
			t.Fatal("actor writer never waited on actual Story lock")
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, e = lock.Exec(ctx, "SELECT pg_sleep(0.35)")
	require.NoError(t, e)
	require.NoError(t, lock.Commit(ctx))
	select {
	case e = <-result:
		require.ErrorIs(t, e, ErrUnavailable)
	case <-time.After(3 * time.Second):
		t.Fatal("actor writer did not finish")
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM story_lfp_consents").Scan(&count))
	require.Zero(t, count)
}
