package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/moderation/internal/automod"
	"voice/backend/pkg/automodwire"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func TestAutoModLifecycleReceiverProjectionAndUnknownBinding(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000013_account_lifecycle_receiver.up.sql"))
	s := automod.NewStore(pool)
	check := automod.BootCheck(func(context.Context) error { return nil }) // Explicit test-only current-boot authority.
	generation, subject, account, epoch := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.InstallLifecycleAuthority(ctx, generation, check))
	deadline := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	cycle := uuid.NewString()
	fact := &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: subject.String(), State: automodv1.SubjectState_SUBJECT_STATE_DELETED, LifecycleRevision: 2, DeletionCycleId: &cycle, PurgeAt: timestamppb.New(deadline)}
	response, err := s.ApplyLifecycleFact(ctx, fact, check)
	require.NoError(t, err)
	require.NoError(t, automodwire.ValidateLifecycleReceipt(&automodv1.ApplyLifecycleFactRequest{Fact: fact}, response))
	require.False(t, response.GetTerminal(), "a due DELETED notification is not positive terminal authority")
	var terminal bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&terminal))
	require.False(t, terminal)
	extended := proto.Clone(fact).(*automodv1.AccountLifecycleFact)
	extended.LifecycleRevision = 3
	extended.EventId = uuid.NewString()
	extended.PurgeAt = timestamppb.New(deadline.Add(time.Hour))
	_, err = s.ApplyLifecycleFact(ctx, extended, check)
	require.ErrorIs(t, err, automod.ErrConflict, "same cycle deadline is immutable even without a raw binding")
	stale := automod.Subject{ID: subject, AccountID: account, Epoch: epoch, State: "LIVE", Revision: 1}
	require.ErrorIs(t, s.Provision(ctx, stale), automod.ErrConflict, "unknown-subject opaque fact must fence a delayed raw binding")
	fresh := stale
	fresh.Revision = 3
	require.NoError(t, s.Provision(ctx, fresh), "a separately fresh higher Auth binding may precede its push")
	_, err = pool.Exec(ctx, `UPDATE automod_subjects SET through_sequence=4,post_mute_count=7 WHERE subject_id=$1`, subject)
	require.NoError(t, err)
	_, err = s.ApplyLifecycleFact(ctx, fact, check)
	require.ErrorIs(t, err, automod.ErrConflict, "cannot acknowledge a fabricated higher fact hash when local binding is ahead")
	live := proto.Clone(fact).(*automodv1.AccountLifecycleFact)
	live.EventId = uuid.NewString()
	live.LifecycleRevision = 3
	live.State = automodv1.SubjectState_SUBJECT_STATE_LIVE
	live.DeletionCycleId = nil
	live.PurgeAt = nil
	response, err = s.ApplyLifecycleFact(ctx, live, check)
	require.NoError(t, err)
	require.NoError(t, automodwire.ValidateLifecycleReceipt(&automodv1.ApplyLifecycleFactRequest{Fact: live}, response))
	response, err = s.ApplyLifecycleFact(ctx, fact, check)
	require.NoError(t, err)
	require.EqualValues(t, 3, response.GetLifecycleRevision())
	_, hash, err := automodwire.EncodeLifecycleFact(live)
	require.NoError(t, err)
	require.Equal(t, hash[:], response.GetCurrentFactSha256())
	changed := proto.Clone(live).(*automodv1.AccountLifecycleFact)
	changed.EventId = uuid.NewString()
	_, err = s.ApplyLifecycleFact(ctx, changed, check)
	require.ErrorIs(t, err, automod.ErrConflict)
	var cursor, count int
	var storedEpoch uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT through_sequence,post_mute_count,source_epoch FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&cursor, &count, &storedEpoch))
	require.Equal(t, 4, cursor)
	require.Equal(t, 7, count)
	require.Equal(t, epoch, storedEpoch)
	wrong := proto.Clone(live).(*automodv1.AccountLifecycleFact)
	wrong.AuthorityGeneration = uuid.NewString()
	_, err = s.ApplyLifecycleFact(ctx, wrong, check)
	require.ErrorIs(t, err, automod.ErrConflict)
	_, err = s.ApplyLifecycleFact(ctx, live, nil)
	require.ErrorIs(t, err, automod.ErrBootClosed)
	require.ErrorIs(t, s.InstallLifecycleAuthority(ctx, uuid.New(), check), automod.ErrConflict)
	closedFact := proto.Clone(live).(*automodv1.AccountLifecycleFact)
	closedFact.SubjectId = uuid.NewString()
	closedFact.EventId = uuid.NewString()
	checks := 0
	closing := automod.BootCheck(func(context.Context) error {
		checks++
		if checks == 3 {
			return automod.ErrBootClosed
		}
		return nil
	})
	_, err = s.ApplyLifecycleFact(ctx, closedFact, closing)
	require.ErrorIs(t, err, automod.ErrBootClosed)
	var retained int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_lifecycle_current WHERE subject_id=$1`, uuid.MustParse(closedFact.SubjectId)).Scan(&retained))
	require.Zero(t, retained, "generation closure before commit rolls back the opaque projection")
}

func TestAutoModLifecycleReceiverTerminalAndGenerationRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000013_account_lifecycle_receiver.up.sql"))
	s := automod.NewStore(pool)
	check := automod.BootCheck(func(context.Context) error { return nil })
	generation := uuid.New()
	require.NoError(t, s.InstallLifecycleAuthority(ctx, generation, check))
	for _, mapped := range []bool{false, true} {
		s.BeforeTerminalPurge = nil
		s.BeforeTerminalSubjectPurge = nil
		subject, account := uuid.New(), uuid.New()
		if mapped {
			require.NoError(t, s.BindSubject(ctx, automod.Subject{ID: subject, AccountID: account, Revision: 1, State: "LIVE"}))
		}
		live := &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: subject.String(), State: automodv1.SubjectState_SUBJECT_STATE_LIVE, LifecycleRevision: 1}
		_, err := s.ApplyLifecycleFact(ctx, live, check)
		require.NoError(t, err)
		fact := &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: subject.String(), State: automodv1.SubjectState_SUBJECT_STATE_PURGED}
		failure := errors.New("participant refused")
		s.BeforeTerminalSubjectPurge = func(context.Context, pgx.Tx, uuid.UUID) error { return failure }
		_, err = s.ApplyLifecycleFact(ctx, fact, check)
		require.ErrorIs(t, err, failure)
		var terminal bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM automod_terminal_subjects WHERE subject_id=$1)`, subject).Scan(&terminal))
		require.False(t, terminal)
		calls := 0
		s.BeforeTerminalSubjectPurge = func(_ context.Context, _ pgx.Tx, id uuid.UUID) error {
			require.Equal(t, subject, id)
			calls++
			return nil
		}
		s.BeforeTerminalPurge = func(_ context.Context, _ pgx.Tx, id uuid.UUID) error { require.Equal(t, account, id); return nil }
		response, err := s.ApplyLifecycleFact(ctx, fact, check)
		require.NoError(t, err)
		require.True(t, response.GetTerminal())
		require.Equal(t, 1, calls)
		require.NoError(t, automodwire.ValidateLifecycleReceipt(&automodv1.ApplyLifecycleFactRequest{Fact: fact}, response))
		_, err = s.ApplyLifecycleFact(ctx, fact, check)
		require.NoError(t, err)
		var raw int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&raw))
		require.Zero(t, raw)
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM automod_lifecycle_current WHERE subject_id=$1`, subject).Scan(&raw))
		require.Zero(t, raw, "positive terminal erases prior cycle/fact payload")
	}
}

func TestAutoModLifecycleReceiverRetriesFirstBindingRace(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000013_account_lifecycle_receiver.up.sql"))
	s := automod.NewStore(pool)
	check := automod.BootCheck(func(context.Context) error { return nil })
	generation, subject, account := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, s.InstallLifecycleAuthority(ctx, generation, check))
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-account-sanction:' || $1::text,0))`, account.String())
	require.NoError(t, err)
	_, err = writer.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('moderation-subject-lifecycle:' || $1::text,0))`, subject.String())
	require.NoError(t, err)
	cycle := uuid.NewString()
	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	fact := &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: subject.String(), State: automodv1.SubjectState_SUBJECT_STATE_DELETED, LifecycleRevision: 2, DeletionCycleId: &cycle, PurgeAt: timestamppb.New(deadline)}
	result := make(chan error, 1)
	go func() { _, err := s.ApplyLifecycleFact(ctx, fact, check); result <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%moderation-subject-lifecycle:%')`).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, s.BindSubjectTx(ctx, writer, automod.Subject{ID: subject, AccountID: account, Revision: 1, State: "LIVE"}))
	require.NoError(t, writer.Commit(ctx))
	require.ErrorIs(t, <-result, automod.ErrConflict, "no subject-to-account lock inversion after a previously absent mapping appears")
	_, err = s.ApplyLifecycleFact(ctx, fact, check)
	require.NoError(t, err)
	var revision int
	require.NoError(t, pool.QueryRow(ctx, `SELECT lifecycle_revision FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&revision))
	require.Equal(t, 2, revision)
}

func TestAutoModLifecycleReceiverRepairsLegacyCycleWithoutReset(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	ctx := context.Background()
	pool := automodAdmissionDatabase(t)
	integrationtestApplyAutoMod(t, ctx, pool)
	subject, account, epoch, generation := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `INSERT INTO automod_subjects(subject_id,account_id,source_epoch,lifecycle_revision,lifecycle_state,purge_at,through_sequence,post_mute_count) VALUES($1,$2,$3,2,'DELETED',$4,4,7)`, subject, account, epoch, deadline)
	require.NoError(t, err)
	integrationtestApplySQL(t, ctx, pool, filepath.Join(repoRoot(t), "src/backend/migrations/moderation_db/000013_account_lifecycle_receiver.up.sql"))
	s := automod.NewStore(pool)
	check := automod.BootCheck(func(context.Context) error { return nil })
	require.NoError(t, s.InstallLifecycleAuthority(ctx, generation, check))
	cycle := uuid.NewString()
	fact := &automodv1.AccountLifecycleFact{ProtocolVersion: 1, AuthorityGeneration: generation.String(), EventId: uuid.NewString(), SubjectId: subject.String(), State: automodv1.SubjectState_SUBJECT_STATE_DELETED, LifecycleRevision: 2, DeletionCycleId: &cycle, PurgeAt: timestamppb.New(deadline)}
	_, err = s.ApplyLifecycleFact(ctx, fact, check)
	require.NoError(t, err)
	var storedCycle, storedEpoch uuid.UUID
	var cursor, count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT deletion_cycle_id,source_epoch,through_sequence,post_mute_count FROM automod_subjects WHERE subject_id=$1`, subject).Scan(&storedCycle, &storedEpoch, &cursor, &count))
	require.Equal(t, uuid.MustParse(cycle), storedCycle)
	require.Equal(t, epoch, storedEpoch)
	require.Equal(t, 4, cursor)
	require.Equal(t, 7, count)
}
