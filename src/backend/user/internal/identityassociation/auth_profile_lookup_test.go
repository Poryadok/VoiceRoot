package identityassociation

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func authProfileFixture(t *testing.T) (*Store, *Keyset, *pgxpool.Pool, uuid.UUID, uuid.UUID, AuthProfileSource) {
	t.Helper()
	pool, profile, account := nativeDatabase(t)
	s, keys := nativeStore(t, pool)
	ctx := context.Background()
	require.NoError(t, s.ActivateGuards(ctx))
	p, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	tx, err := s.Begin(ctx)
	require.NoError(t, err)
	_, err = s.CaptureRetainedTx(ctx, tx, p)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	source := AuthProfileSource{HistoryGeneration: s.generation}
	source.KeysetDigest, err = keys.CustodyDigest()
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT user_source_generation FROM identity_meta WHERE singleton`).Scan(&source.UserSourceGeneration))
	require.NoError(t, pool.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&source.Physical.SystemIdentifier, &source.Physical.Database, &source.Physical.DatabaseOID))
	return s, keys, pool, profile, account, source
}

func TestAuthProfileRetiredAliasStillMatchesAndNeverReconstructsAccount(t *testing.T) {
	s, keys, pool, profile, account, source := authProfileFixture(t)
	ctx := context.Background()
	p, err := keys.PrepareSelectors(profile, account)
	require.NoError(t, err)
	lookup, err := keys.PrepareProfileLookup(profile)
	require.NoError(t, err)
	tx, err := s.Begin(ctx)
	require.NoError(t, err)
	fact, err := s.ReadByProfileSelectorsTx(ctx, tx, lookup)
	require.NoError(t, err)
	parent := validParentForTest(account)
	parent.State = ParentTerminal
	parent.Eligibility = ParentIneligible
	parent.Disposition = ParentIrreversible
	parent.CycleID = uuid.New()
	parent.EraseAt = parent.ObservedAt.Add(-time.Second)
	fact, err = s.AttachParentTx(ctx, tx, p, fact.Revision, VerifiedParentProof{verified: true, observation: parent})
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	tx, err = s.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, s.PrepareTerminalDeleteTx(ctx, tx, p, fact.Revision))
	_, err = tx.SQL().Exec(ctx, `DELETE FROM profiles WHERE id=$1`, profile)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM profiles WHERE id=$1`, profile).Scan(&count))
	require.Zero(t, count)
	result, err := s.ReadAuthProfiles(ctx, account, []uuid.UUID{profile}, source)
	require.NoError(t, err)
	require.Equal(t, "MATCH", result.Results[0].Match)
	target, err := s.ReadAuthProfiles(ctx, uuid.Nil, []uuid.UUID{profile}, source)
	require.NoError(t, err)
	require.Equal(t, uuid.Nil, target.Results[0].CurrentAccount)
	require.NotNil(t, target.Results[0].Fact.Terminal)
}

func TestAuthProfileMissingCanonicalAliasIsIncomplete(t *testing.T) {
	for _, kind := range []string{"ACCOUNT", "PROFILE"} {
		t.Run(kind, func(t *testing.T) {
			s, _, pool, profile, account, source := authProfileFixture(t)
			ctx := context.Background()
			// Simulate damaged restored evidence without changing the qualified catalog.
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `SET LOCAL session_replication_role=replica`)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `DELETE FROM association_selector_aliases WHERE kind=$1`, kind)
			require.NoError(t, err)
			require.NoError(t, tx.Commit(ctx))
			for _, owner := range []uuid.UUID{account, uuid.New()} {
				result, err := s.ReadAuthProfiles(ctx, owner, []uuid.UUID{profile}, source)
				require.NoError(t, err)
				require.Equal(t, "INCOMPLETE", result.Results[0].Match)
			}
		})
	}
}

func TestAuthProfileReadsCommittedStateAfterIdentityMutexWait(t *testing.T) {
	s, keys, pool, profile, _, source := authProfileFixture(t)
	ctx := context.Background()
	owner, err := s.Begin(ctx)
	require.NoError(t, err)
	defer owner.Rollback(ctx)
	_, err = owner.SQL().Exec(ctx, `UPDATE profiles SET frozen_at=clock_timestamp() WHERE id=$1`, profile)
	require.NoError(t, err)
	cfg := pool.Config().Copy()
	cfg.ConnConfig.RuntimeParams["application_name"] = "auth-profile-waiter"
	waiterPool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer waiterPool.Close()
	waiter, err := NewStore(waiterPool, keys, s.generation)
	require.NoError(t, err)
	returned := make(chan error, 1)
	var result AuthProfileBatch
	go func() {
		var e error
		result, e = waiter.ReadAuthProfiles(ctx, uuid.Nil, []uuid.UUID{profile}, source)
		returned <- e
	}()
	require.Eventually(t, func() bool {
		var blocked bool
		e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name='auth-profile-waiter' AND cardinality(pg_blocking_pids(pid))>0)`).Scan(&blocked)
		require.NoError(t, e)
		return blocked
	}, time.Second, 5*time.Millisecond)
	select {
	case e := <-returned:
		t.Fatalf("read escaped mutex: %v", e)
	default:
	}
	require.NoError(t, owner.Commit(ctx))
	require.NoError(t, <-returned)
	require.True(t, result.Results[0].Frozen)
}

func TestAuthProfileMatchesCanonicalOwnerWithoutEnumeratingOrWriting(t *testing.T) {
	s, _, pool, profile, account, source := authProfileFixture(t)
	ctx := context.Background()
	var before uint64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_head FROM identity_meta WHERE singleton`).Scan(&before))
	result, err := s.ReadAuthProfiles(ctx, account, []uuid.UUID{profile}, source)
	require.NoError(t, err)
	require.Len(t, result.Results, 1)
	require.Equal(t, "MATCH", result.Results[0].Match)
	require.Equal(t, profile, result.Results[0].ProfileID)
	require.NotEqual(t, uuid.Nil, result.Results[0].Fact.AssociationID)
	require.Equal(t, before, result.Cut.ThroughRevision)
	foreign, err := s.ReadAuthProfiles(ctx, uuid.New(), []uuid.UUID{profile}, source)
	require.NoError(t, err)
	require.Equal(t, "DIFFERENT_ACCOUNT", foreign.Results[0].Match)
	unknown, err := s.ReadAuthProfiles(ctx, account, []uuid.UUID{uuid.New()}, source)
	require.NoError(t, err)
	require.Equal(t, "INCOMPLETE", unknown.Results[0].Match)
	require.Equal(t, before, unknown.Cut.ThroughRevision)
	target, err := s.ReadAuthProfiles(ctx, uuid.Nil, []uuid.UUID{profile}, source)
	require.NoError(t, err)
	require.Equal(t, account, target.Results[0].CurrentAccount)
	require.Positive(t, target.Results[0].CurrentRevision)
}

func TestAuthProfileRejectsUnqualifiedOrChangedSource(t *testing.T) {
	s, _, _, profile, account, source := authProfileFixture(t)
	for _, change := range []func(*AuthProfileSource){func(x *AuthProfileSource) { x.HistoryGeneration = uuid.New() }, func(x *AuthProfileSource) { x.UserSourceGeneration = uuid.New() }, func(x *AuthProfileSource) { x.KeysetDigest = [32]byte{} }, func(x *AuthProfileSource) { x.Physical.DatabaseOID++ }} {
		changed := source
		change(&changed)
		_, err := s.ReadAuthProfiles(context.Background(), account, []uuid.UUID{profile}, changed)
		require.Error(t, err)
	}
}
