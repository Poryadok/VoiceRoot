package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
)

type moderationIntent struct {
	actor, account, operation, space uuid.UUID
	kind                             string
	body                             []byte
	execution                        uuid.UUID
	call                             *lifecycleprincipal.VerifiedAdmissionCall
	request                          proto.Message
}

type moderationIntentRoot struct {
	state           string
	selected        *uuid.UUID
	sequence        int64
	execution       *uuid.UUID
	active, expired bool
}

func moderationInProgress() error {
	s := status.New(codes.FailedPrecondition, "moderation intent in progress")
	withDetail, err := s.WithDetails(&errdetails.ErrorInfo{Domain: "voice.space.moderation.v1", Reason: "INTENT_IN_PROGRESS"})
	if err != nil {
		return s.Err()
	}
	return withDetail.Err()
}

func newModerationIntent(call *lifecycleprincipal.VerifiedAdmissionCall, request proto.Message, space uuid.UUID, kind string) (*moderationIntent, error) {
	actor, err := call.ActorFor(request)
	if err != nil {
		return nil, err
	}
	if (kind != "BAN" || actor.RPC != spacev1.SpaceService_BanMember_FullMethodName) && (kind != "UNBAN" || actor.RPC != spacev1.SpaceService_UnbanMember_FullMethodName) {
		return nil, status.Error(codes.Unauthenticated, "invalid moderation method")
	}
	spaceField := request.ProtoReflect().Descriptor().Fields().ByName("space_id")
	if space == uuid.Nil || spaceField == nil || request.ProtoReflect().Get(spaceField).String() != space.String() {
		return nil, status.Error(codes.InvalidArgument, "invalid space_id")
	}
	field := request.ProtoReflect().Descriptor().Fields().ByName("operation_id")
	if field == nil {
		return nil, status.Error(codes.InvalidArgument, "operation_id required")
	}
	operation, err := canonicalLifecycleUUID(request.ProtoReflect().Get(field).String())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operation_id")
	}
	account := uuid.MustParse(actor.AccountID)
	profile := uuid.MustParse(actor.ProfileID)
	// Token, session epoch, request ID and JTI are deliberately excluded.
	h := sha256.New()
	h.Write([]byte("voice.space.moderation.intent.v1\x00"))
	h.Write(profile[:])
	h.Write(account[:])
	h.Write([]byte(actor.RPC))
	h.Write([]byte{0})
	h.Write(deterministicBanBytes(request))
	return &moderationIntent{actor: profile, account: account, operation: operation, space: space, kind: kind, body: h.Sum(nil), execution: uuid.New(), call: call, request: request}, nil
}

// Unlike ordinary writes this scope permits the authenticated completed-replay
// branch before fresh Role/member checks. Native catalog, readiness and permanent
// privacy fences still dominate every root read, including saved outcomes.
func (s *SpaceStore) withModerationIntentScope(ctx context.Context, in *moderationIntent, action func(*SpaceStore) error) error {
	if s == nil || s.Pool == nil || s.tx != nil {
		return ErrAccountBanUnavailable
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { cleanup, cancel := BoundedCleanupContext(ctx); defer cancel(); _ = tx.Rollback(cleanup) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(in.space)); err != nil {
		return err
	}
	scoped := *s
	scoped.tx = tx
	if _, err = in.call.ActorFor(in.request); err != nil {
		return err
	}
	var live bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1) AND NOT EXISTS(SELECT 1 FROM space_permanent_purge_fences WHERE space_id=$1)`, in.space).Scan(&live); err != nil || !live {
		return ErrAccountBanUnavailable
	}
	var version int64
	if err = tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 50 {
		return ErrAccountBanUnavailable
	}
	if err = scoped.RequireScopedNativeMembershipReadiness(ctx, in.space); err != nil {
		return err
	}
	if err = action(&scoped); err != nil {
		return err
	}
	if _, err = in.call.ActorFor(in.request); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *SpaceStore) readModerationIntent(ctx context.Context, in *moderationIntent) (*moderationIntentRoot, error) {
	root := &moderationIntentRoot{}
	var body []byte
	var account, space uuid.UUID
	var kind string
	var executionExpires, completed *time.Time
	var created time.Time
	err := s.tx.QueryRow(ctx, `SELECT actor_account_id,space_id,operation_kind,body_sha256,state,selected_capture_id,selected_attempt_seq,execution_id,
 execution_expires_at,completed_at,created_at
 FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 FOR UPDATE`, in.actor, in.operation).Scan(&account, &space, &kind, &body, &root.state, &root.selected, &root.sequence, &root.execution, &executionExpires, &completed, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if account != in.account || space != in.space || kind != in.kind || !bytes.Equal(body, in.body) {
		return nil, status.Error(codes.AlreadyExists, "moderation intent binding conflict")
	}
	var wall time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&wall); err != nil {
		return nil, err
	}
	root.active = root.execution != nil && executionExpires != nil && wall.Before(*executionExpires)
	if completed != nil {
		root.expired = !wall.Before(completed.Add(30 * 24 * time.Hour))
	}
	if root.expired {
		return nil, status.Error(codes.FailedPrecondition, "moderation intent expired")
	}
	return root, nil
}

func (s *SpaceStore) probeModerationIntent(ctx context.Context, in *moderationIntent) (bool, error) {
	replay := false
	err := s.withModerationIntentScope(ctx, in, func(scoped *SpaceStore) error {
		root, err := scoped.readModerationIntent(ctx, in)
		if err != nil {
			return err
		}
		if root == nil {
			return nil
		}
		if root.state == "COMMITTED" {
			replay = true
			return nil
		}
		if root.active {
			return moderationInProgress()
		}
		return nil
	})
	return replay, err
}

func (s *SpaceStore) requireModerationActor(ctx context.Context, in *moderationIntent) error {
	if err := checkOwnershipJournalAvailable(ctx, s.tx, []uuid.UUID{in.space}); err != nil {
		return err
	}
	if err := checkLifecycleAvailable(ctx, s.tx, []uuid.UUID{in.space}); err != nil {
		return err
	}
	var owner uuid.UUID
	if err := s.tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, in.space).Scan(&owner); err != nil {
		return err
	}
	if owner != in.actor {
		member, err := s.IsSpaceMember(ctx, in.space, in.actor)
		if err != nil || !member {
			return status.Error(codes.PermissionDenied, "moderation permission required")
		}
	}
	_, err := in.call.ActorFor(in.request)
	return err
}

func (s *SpaceStore) claimModerationIntent(ctx context.Context, in *moderationIntent) (bool, error) {
	replay := false
	err := s.withModerationIntentScope(ctx, in, func(scoped *SpaceStore) error {
		root, err := scoped.readModerationIntent(ctx, in)
		if err != nil {
			return err
		}
		if root != nil && root.state == "COMMITTED" {
			replay = true
			return nil
		}
		if root != nil && root.active {
			return moderationInProgress()
		}
		if err = scoped.requireModerationActor(ctx, in); err != nil {
			return err
		}
		remaining, err := in.call.Remaining(in.request)
		if err != nil {
			return err
		}
		// clock_timestamp is evaluated after both Space and root locks.
		if root == nil {
			_, err = scoped.tx.Exec(ctx, `INSERT INTO space_account_ban_operations(actor_profile_id,client_operation_id,actor_account_id,space_id,operation_kind,body_sha256,state,execution_id,execution_expires_at)
 VALUES($1,$2,$3,$4,$5,$6,'CAPTURING',$7,clock_timestamp()+$8*interval '1 microsecond')`, in.actor, in.operation, in.account, in.space, in.kind, in.body, in.execution, remaining.Microseconds())
			return err
		}
		tag, err := scoped.tx.Exec(ctx, `UPDATE space_account_ban_operations SET execution_id=$3,execution_expires_at=clock_timestamp()+$4*interval '1 microsecond'
 WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='CAPTURING' AND (execution_id IS NULL OR clock_timestamp()>=execution_expires_at)`, in.actor, in.operation, in.execution, remaining.Microseconds())
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return moderationInProgress()
		}
		return nil
	})
	return replay, err
}

func (s *SpaceStore) requireCurrentModerationExecution(ctx context.Context, in *moderationIntent) (*moderationIntentRoot, error) {
	root, err := s.readModerationIntent(ctx, in)
	if err != nil {
		return nil, err
	}
	if root == nil || root.state != "CAPTURING" || root.execution == nil || *root.execution != in.execution || !root.active {
		return nil, ErrAccountBanUnavailable
	}
	if _, err = in.call.Remaining(in.request); err != nil {
		return nil, err
	}
	return root, nil
}

func (s *SpaceStore) releaseModerationIntent(ctx context.Context, in *moderationIntent) {
	cleanup, cancel := BoundedCleanupContext(ctx)
	defer cancel()
	_ = s.withModerationIntentScope(cleanup, in, func(scoped *SpaceStore) error {
		_, err := scoped.tx.Exec(cleanup, `UPDATE space_account_ban_operations SET execution_id=NULL,execution_expires_at=NULL
 WHERE actor_profile_id=$1 AND client_operation_id=$2 AND body_sha256=$3 AND execution_id=$4 AND state='CAPTURING'`, in.actor, in.operation, in.body, in.execution)
		return err
	})
}

func (s *SpaceStore) commitModerationIntent(ctx context.Context, in *moderationIntent, selected *uuid.UUID, sequence int64, response []byte, profiles []uuid.UUID) error {
	if _, err := in.call.Remaining(in.request); err != nil {
		return err
	}
	if profiles == nil {
		profiles = []uuid.UUID{}
	}
	tag, err := s.tx.Exec(ctx, `UPDATE space_account_ban_operations SET state='COMMITTED',response_bytes=$6,matched_profiles=$7
 WHERE actor_profile_id=$1 AND client_operation_id=$2 AND body_sha256=$3 AND execution_id=$4 AND state='CAPTURING'
 AND selected_capture_id IS NOT DISTINCT FROM $5::uuid AND selected_attempt_seq=$8 AND clock_timestamp()<execution_expires_at`, in.actor, in.operation, in.body, in.execution, selected, response, profiles, sequence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrAccountBanUnavailable
	}
	return nil
}

// CleanupExpiredAccountBanIntents is bounded maintenance, not a replay fallback.
// Deletion ends server reuse detection; clients must never reuse expired IDs.
func (s *SpaceStore) CleanupExpiredAccountBanIntents(ctx context.Context, space uuid.UUID, limit int) (int64, error) {
	if limit < 1 || limit > 256 {
		return 0, ErrAccountBanUnavailable
	}
	var removed int64
	err := s.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		if err := scoped.RequireScopedNativeMembershipReadiness(ctx, space); err != nil {
			return err
		}
		tag, err := scoped.tx.Exec(ctx, `DELETE FROM space_account_ban_operations WHERE (actor_profile_id,client_operation_id) IN
 (SELECT actor_profile_id,client_operation_id FROM space_account_ban_operations WHERE space_id=$1
 AND state='COMMITTED' AND clock_timestamp()>completed_at+interval '30 days'
 AND (execution_id IS NULL OR clock_timestamp()>=execution_expires_at)
 ORDER BY completed_at,actor_profile_id,client_operation_id LIMIT $2 FOR UPDATE)`, space, limit)
		removed = tag.RowsAffected()
		return err
	})
	return removed, err
}

// ListExpiredAccountBanIntentSpaces returns bounded hints, never deletion authority. The per-
// Space worker must repeat the guarded catalog/fence/expiry checks when deleting.
func (s *SpaceStore) ListExpiredAccountBanIntentSpaces(ctx context.Context, limit int) ([]uuid.UUID, error) {
	if s == nil || s.Pool == nil || s.tx != nil || limit < 1 || limit > 256 {
		return nil, ErrAccountBanUnavailable
	}
	var version int64
	if err := s.Pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 50 {
		return nil, ErrAccountBanUnavailable
	}
	rows, err := s.Pool.Query(ctx, `SELECT r.space_id FROM space_account_ban_operations r JOIN spaces s ON s.id=r.space_id
 WHERE r.state='COMMITTED' AND clock_timestamp()>r.completed_at+interval '30 days'
 AND (r.execution_id IS NULL OR clock_timestamp()>=r.execution_expires_at)
 AND NOT EXISTS(SELECT 1 FROM space_permanent_purge_fences f WHERE f.space_id=r.space_id)
 AND NOT EXISTS(SELECT 1 FROM space_lifecycle_aggregates a WHERE a.space_id=r.space_id AND a.phase<>'LIVE')
 AND NOT EXISTS(SELECT 1 FROM ownership_journal j WHERE j.space_id=r.space_id AND j.state NOT IN ('completed','aborted'))
 GROUP BY r.space_id ORDER BY min(r.completed_at),r.space_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	spaces := []uuid.UUID{}
	for rows.Next() {
		var space uuid.UUID
		if err = rows.Scan(&space); err != nil {
			return nil, err
		}
		spaces = append(spaces, space)
	}
	return spaces, rows.Err()
}
