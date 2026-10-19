package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/memberadmission"
)

// MemberAdmissionInput identifies one server-owned admission attempt. Remote
// proof verification precedes this API; unsupported interactive policy is denied.
type MemberAdmissionInput struct {
	OperationID, SpaceID, ProfileID, AccountID uuid.UUID
	InviteID                                   *uuid.UUID
	Guest                                      bool
	JoinAttempt                                *JoinAttemptReference `json:",omitempty"`
	DecisionBasisSHA256                        []byte                `json:",omitempty"`
}

type MemberAdmissionIntent struct {
	Input         MemberAdmissionInput
	Generation    uint64
	BindingSHA256 []byte
	PolicyVersion int64
	State         string
}

func (i *MemberAdmissionIntent) EnsureBinding() memberadmission.EnsureBinding {
	return memberadmission.EnsureBinding{ProtocolVersion: 1, SpaceID: i.Input.SpaceID.String(),
		ProfileID: i.Input.ProfileID.String(), AdmissionOperationID: i.Input.OperationID.String(),
		MembershipGeneration: i.Generation, AdmissionBindingSHA256: append([]byte(nil), i.BindingSHA256...)}
}

// MemberAdmissionFinalGuard may access only the provided scoped Space store.
// Callers collect external authority before entering publication; network I/O
// under this callback would violate the protocol's lock ordering.
type MemberAdmissionFinalGuard func(context.Context, *SpaceStore, *MemberAdmissionIntent) error

// MemberAdmissionPublicationHook runs only on first publication, after the
// native outcome and interactive markers exist in the same transaction. It may
// journal local outcomes, never perform network I/O or grant membership itself.
type MemberAdmissionPublicationHook func(context.Context, *SpaceStore, *MemberAdmissionIntent, *MembershipRow) error

var (
	ErrMemberAdmissionConflict = errors.New("member admission binding conflict")
	ErrMemberAdmissionStale    = errors.New("member admission superseded")
	ErrMemberAdmissionReceipt  = errors.New("member admission receipt invalid")
)

func lockMemberAdmissionOperation(ctx context.Context, tx pgx.Tx, operation uuid.UUID) error {
	digest := sha256.Sum256(append([]byte("voice.space.member-admission.operation.v1\x00"), operation[:]...))
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(digest[:4])), int32(binary.BigEndian.Uint32(digest[4:8])))
	return err
}

func scanMemberAdmission(row pgx.Row) (*MemberAdmissionIntent, []byte, error) {
	i := &MemberAdmissionIntent{}
	var request []byte
	var attempt *uuid.UUID
	var evidence []byte
	err := row.Scan(&i.Input.OperationID, &i.Input.SpaceID, &i.Input.ProfileID, &i.Input.AccountID,
		&i.Input.InviteID, &i.Input.Guest, &i.Generation, &request, &i.BindingSHA256, &i.PolicyVersion, &i.State,
		&attempt, &evidence, &i.Input.DecisionBasisSHA256)
	if attempt != nil {
		i.Input.JoinAttempt = &JoinAttemptReference{AttemptID: *attempt, EvidenceSHA256: evidence}
	}
	return i, request, err
}

const memberAdmissionColumns = `operation_id,space_id,profile_id,account_id,invite_id,guest,generation,request_sha256,binding_sha256,policy_version,state,join_attempt_id,join_attempt_evidence_sha256,decision_basis_sha256`

func admissionRequestHash(in MemberAdmissionInput) [32]byte {
	raw, _ := json.Marshal(in)
	return sha256.Sum256(append([]byte("voice.space.member-admission.request.v1\x00"), raw...))
}

// PrepareMemberAdmission allocates one positive incarnation while publishing no
// membership, count, invite use or event. It never performs a remote call.
func (s *SpaceStore) PrepareMemberAdmission(ctx context.Context, in MemberAdmissionInput) (*MemberAdmissionIntent, error) {
	if s == nil || s.Pool == nil || s.tx != nil || in.OperationID == uuid.Nil || in.SpaceID == uuid.Nil || in.ProfileID == uuid.Nil || in.AccountID == uuid.Nil || (in.InviteID != nil && *in.InviteID == uuid.Nil) {
		return nil, ErrMemberAdmissionConflict
	}
	in = cloneMemberAdmissionInput(in)
	if err := validateMemberAdmissionReference(in); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockMemberAdmissionOperation(ctx, tx, in.OperationID); err != nil {
		return nil, err
	}
	request := admissionRequestHash(in)
	saved, prior, err := scanMemberAdmission(tx.QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, in.OperationID))
	if err == nil {
		if !bytes.Equal(prior, request[:]) {
			return nil, ErrMemberAdmissionConflict
		}
		return saved, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{in.SpaceID}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	policy, err := scoped.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return nil, err
	}
	intent := &MemberAdmissionIntent{Input: in, PolicyVersion: policy.Version, State: "PREPARING"}
	if err = scoped.validateMemberAdmission(ctx, intent); err != nil {
		return nil, err
	}
	var generation, floor int64
	var state string
	err = tx.QueryRow(ctx, `SELECT generation,retired_through,state FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, in.SpaceID, in.ProfileID).Scan(&generation, &floor, &state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if state == "PREPARING" {
		return nil, ErrMemberAdmissionConflict
	}
	if generation == math.MaxInt64 || floor == math.MaxInt64 {
		return nil, ErrMemberAdmissionStale
	}
	intent.Generation = uint64(max(generation, floor) + 1)
	binding, _ := json.Marshal(struct {
		Request       [32]byte
		PolicyVersion int64
		Policy        any
	}{request, policy.Version, policy.Policy})
	digest := sha256.Sum256(append([]byte("voice.space.member-admission.binding.v1\x00"), binding...))
	intent.BindingSHA256 = append([]byte(nil), digest[:]...)
	effective, err := scoped.captureNativeEffectiveMemberMutation(ctx, in.SpaceID, []uuid.UUID{in.ProfileID})
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,account_id,admission_binding_sha256)
	 VALUES($1,$2,$3,$4,'PREPARING',$5,$6) ON CONFLICT(space_id,profile_id) DO UPDATE SET
	 generation=EXCLUDED.generation,publication_id=EXCLUDED.publication_id,state='PREPARING',legacy=false,
	 account_id=EXCLUDED.account_id,admission_binding_sha256=EXCLUDED.admission_binding_sha256,role_receipt_sha256=NULL,
	 publication_kind=0,native_publication_sha256=NULL`, in.SpaceID, in.ProfileID, intent.Generation, in.OperationID, in.AccountID, digest[:])
	if err != nil {
		return nil, err
	}
	var attempt *uuid.UUID
	var evidence []byte
	if in.JoinAttempt != nil {
		attempt, evidence = &in.JoinAttempt.AttemptID, in.JoinAttempt.EvidenceSHA256
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_member_admission_intents(`+memberAdmissionColumns+`)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'PREPARING',$11,$12,$13)`, in.OperationID, in.SpaceID, in.ProfileID, in.AccountID, in.InviteID, in.Guest, intent.Generation, request[:], digest[:], policy.Version, attempt, evidence, in.DecisionBasisSHA256)
	if err != nil {
		return nil, err
	}
	if effective != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, effective); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return intent, nil
}

func (s *SpaceStore) validateMemberAdmission(ctx context.Context, i *MemberAdmissionIntent) error {
	return s.validateMemberAdmissionWithGuest(ctx, i, i.Input.Guest)
}

func (s *SpaceStore) validateMemberAdmissionWithGuest(ctx context.Context, i *MemberAdmissionIntent, guest bool) error {
	in := i.Input
	var current time.Time
	if err := s.db().QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&current); err != nil {
		return err
	}
	if banned, err := s.IsAccountBanned(ctx, in.SpaceID, in.AccountID); err != nil {
		return err
	} else if banned {
		return ErrAccountBanned
	}
	if member, err := s.IsSpaceMember(ctx, in.SpaceID, in.ProfileID); err != nil {
		return err
	} else if member {
		return ErrAlreadySpaceMember
	}
	policy, err := s.GetEntryPolicy(ctx, in.SpaceID)
	if err != nil {
		return err
	}
	if policy.Version != i.PolicyVersion {
		return ErrEntryPolicyVersion
	}
	var visibility string
	var guests bool
	var count int32
	if err = s.db().QueryRow(ctx, `SELECT visibility,allow_guests,member_count FROM spaces WHERE id=$1`, in.SpaceID).Scan(&visibility, &guests, &count); err != nil {
		return err
	}
	if in.InviteID == nil {
		if guest || visibility != "public" {
			if guest && !in.Guest {
				return ErrGuestsNotAllowed
			}
			return ErrMemberAdmissionConflict
		}
	} else {
		inv, err := s.GetInviteByID(ctx, *in.InviteID)
		if err != nil {
			return err
		}
		if inv == nil || inv.SpaceID != in.SpaceID {
			return ErrInviteNotFound
		}
		if err = s.validateInviteForJoin(inv, current); err != nil {
			return err
		}
		if guest && !guests {
			return ErrGuestsNotAllowed
		}
	}
	cap, err := memberCapTx(ctx, s.tx, in.SpaceID)
	if err != nil {
		return err
	}
	if count >= cap {
		return ErrMemberCapReached
	}
	if guest && policy.Policy.RequirePhone {
		return ErrGuestsNotAllowed
	}
	return s.verifyMemberAdmissionReference(ctx, i)
}

func (s *SpaceStore) invalidateMemberAdmission(ctx context.Context, i *MemberAdmissionIntent) error {
	tag, err := s.db().Exec(ctx, `UPDATE space_ordinary_member_heads SET generation=generation+1,retired_through=generation+1,state='RETIRED',legacy=false
	 WHERE space_id=$1 AND profile_id=$2 AND generation=$3 AND publication_id=$4 AND state='PREPARING'`, i.Input.SpaceID, i.Input.ProfileID, i.Generation, i.Input.OperationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrMemberAdmissionStale
	}
	_, err = s.db().Exec(ctx, `UPDATE space_member_admission_intents SET state='RETIRED' WHERE operation_id=$1 AND state='PREPARING'`, i.Input.OperationID)
	if err != nil {
		return err
	}
	_, err = s.db().Exec(ctx, `INSERT INTO space_member_role_cleanup(operation_id,space_id,profile_id,removal_floor)
	 SELECT gen_random_uuid(),space_id,profile_id,retired_through FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, i.Input.SpaceID, i.Input.ProfileID)
	return err
}

// PublishMemberAdmission is the sole local publication transaction. A completed
// retry returns its historical outcome before current membership/lifecycle checks;
// it never executes a new final guard or recreates a later removed membership.
func (s *SpaceStore) PublishMemberAdmission(ctx context.Context, operation, role uuid.UUID, receipt []byte, guard MemberAdmissionFinalGuard) (*MembershipRow, error) {
	return s.publishMemberAdmission(ctx, operation, role, receipt, guard, nil, nil)
}

func (s *SpaceStore) publishMemberAdmission(ctx context.Context, operation, role uuid.UUID, receipt []byte, guard MemberAdmissionFinalGuard, continuation *VerifiedAdmissionContinuation, publicationHook MemberAdmissionPublicationHook) (*MembershipRow, error) {
	return s.publishMemberAdmissionWithInitiatingCall(ctx, operation, role, receipt, guard, continuation, publicationHook, nil)
}

func (s *SpaceStore) publishMemberAdmissionWithInitiatingCall(ctx context.Context, operation, role uuid.UUID, receipt []byte, guard MemberAdmissionFinalGuard, continuation *VerifiedAdmissionContinuation, publicationHook MemberAdmissionPublicationHook, initiating *memberAdmissionInitiatingCall) (*MembershipRow, error) {
	if s == nil || s.Pool == nil || s.tx != nil || operation == uuid.Nil || role == uuid.Nil {
		return nil, ErrMemberAdmissionConflict
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockMemberAdmissionOperation(ctx, tx, operation); err != nil {
		return nil, err
	}
	i, _, err := scanMemberAdmission(tx.QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, operation))
	if err != nil {
		return nil, err
	}
	expected, err := memberadmission.EnsureReceiptFingerprint(i.EnsureBinding(), role.String())
	if err != nil || !bytes.Equal(expected[:], receipt) {
		return nil, ErrMemberAdmissionReceipt
	}
	if i.State == "PUBLISHED" {
		var savedRole uuid.UUID
		var savedReceipt []byte
		var joined time.Time
		if err = tx.QueryRow(ctx, `SELECT role_id,role_receipt_sha256,published_at FROM space_member_admission_intents WHERE operation_id=$1`, operation).Scan(&savedRole, &savedReceipt, &joined); err != nil {
			return nil, err
		}
		if savedRole != role || !bytes.Equal(savedReceipt, receipt) {
			return nil, ErrMemberAdmissionConflict
		}
		return &MembershipRow{SpaceID: i.Input.SpaceID, ProfileID: i.Input.ProfileID, JoinedAt: joined}, tx.Commit(ctx)
	}
	if i.State != "PREPARING" {
		return nil, ErrMemberAdmissionStale
	}
	if initiating != nil {
		if err = initiating.check(ctx); err != nil {
			return nil, err
		}
	}
	// The historical journal and a generic callback cannot manufacture fresh
	// applicant/original-reviewer authority. A sealed continuation API must
	// explicitly enable this path; completed replay above stays inert.
	if i.Input.JoinAttempt != nil && continuation == nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if i.Input.JoinAttempt == nil && continuation != nil {
		return nil, ErrJoinAttemptUnavailable
	}
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{i.Input.SpaceID}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	var current bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2
	 AND generation=$3 AND publication_id=$4 AND state='PREPARING' AND admission_binding_sha256=$5)`, i.Input.SpaceID, i.Input.ProfileID, i.Generation, operation, i.BindingSHA256).Scan(&current); err != nil {
		return nil, err
	}
	if !current {
		return nil, ErrMemberAdmissionStale
	}
	scoped := *s
	scoped.tx = tx
	effective, err := scoped.captureNativeEffectiveMemberMutation(ctx, i.Input.SpaceID, []uuid.UUID{i.Input.ProfileID})
	if err != nil {
		return nil, err
	}
	guest := i.Input.Guest
	if continuation != nil {
		if guest, err = continuation.check(i); err != nil {
			return nil, err
		}
		if err = scoped.checkContinuationReviewer(ctx, continuation); err != nil {
			return nil, err
		}
	}
	if err = scoped.validateMemberAdmissionWithGuest(ctx, i, guest); err != nil {
		if continuation != nil {
			if _, proofErr := continuation.check(i); proofErr != nil {
				return nil, proofErr
			}
		}
		if !knownMemberAdmissionDenial(err) {
			return nil, err
		}
		if invalidateErr := scoped.invalidateMemberAdmission(ctx, i); invalidateErr != nil {
			return nil, invalidateErr
		}
		if effective != nil {
			if historyErr := scoped.finishEffectiveMemberMutations(ctx, effective); historyErr != nil {
				return nil, historyErr
			}
		}
		if continuation != nil {
			if _, proofErr := continuation.check(i); proofErr != nil {
				return nil, proofErr
			}
		}
		if initiating != nil {
			if callErr := initiating.check(ctx); callErr != nil {
				return nil, callErr
			}
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, commitErr
		}
		return nil, err
	}
	if guard != nil {
		guardTx, beginErr := tx.Begin(ctx)
		if beginErr != nil {
			return nil, beginErr
		}
		guardStore := scoped
		guardStore.tx = guardTx
		guardIntent := *i
		guardIntent.Input = cloneMemberAdmissionInput(i.Input)
		guardIntent.BindingSHA256 = append([]byte(nil), i.BindingSHA256...)
		if guardErr := guard(ctx, &guardStore, &guardIntent); guardErr != nil {
			if rollbackErr := guardTx.Rollback(ctx); rollbackErr != nil {
				return nil, rollbackErr
			}
			// A missing/expired observation or conflicting immutable binding is
			// not evidence that the candidate lost authority. Preserve it for
			// a fresh attempt; all callback SQL is already rolled back.
			if !knownMemberAdmissionGuardDenial(guardErr) {
				return nil, guardErr
			}
			if continuation != nil {
				if _, proofErr := continuation.check(i); proofErr != nil {
					return nil, proofErr
				}
			}
			if err = scoped.invalidateMemberAdmission(ctx, i); err != nil {
				return nil, err
			}
			if effective != nil {
				if historyErr := scoped.finishEffectiveMemberMutations(ctx, effective); historyErr != nil {
					return nil, historyErr
				}
			}
			if continuation != nil {
				if _, proofErr := continuation.check(i); proofErr != nil {
					return nil, proofErr
				}
			}
			if initiating != nil {
				if err = initiating.check(ctx); err != nil {
					return nil, err
				}
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, guardErr
		}
		if err = guardTx.Commit(ctx); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE space_ordinary_member_heads SET state='ACTIVE',publication_kind=1,role_receipt_sha256=$3 WHERE space_id=$1 AND profile_id=$2`, i.Input.SpaceID, i.Input.ProfileID, receipt)
	if err != nil {
		return nil, err
	}
	member, err := scanMembershipRow(tx.QueryRow(ctx, `INSERT INTO space_members(space_id,profile_id,membership_generation,admission_operation_id)
	 VALUES($1,$2,$3,$4) RETURNING space_id,profile_id,joined_at,nickname`, i.Input.SpaceID, i.Input.ProfileID, i.Generation, operation))
	if err != nil {
		return nil, err
	}
	err = (&SpaceStore{tx: tx}).refreshEffectiveMemberCount(ctx, i.Input.SpaceID)
	if err != nil {
		return nil, err
	}
	if i.Input.InviteID != nil {
		if _, err = tx.Exec(ctx, `UPDATE invites SET use_count=use_count+1 WHERE id=$1`, *i.Input.InviteID); err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE space_member_admission_intents SET state='PUBLISHED',role_id=$2,role_receipt_sha256=$3,published_at=$4 WHERE operation_id=$1`, operation, role, receipt, member.JoinedAt)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_member_admission_outbox(event_id,operation_id,space_id,profile_id,generation,occurred_at)
	 VALUES($1,$2,$3,$4,$5,$6)`, uuid.New(), operation, i.Input.SpaceID, i.Input.ProfileID, i.Generation, member.JoinedAt)
	if err != nil {
		return nil, err
	}
	if continuation != nil {
		if err = scoped.MarkJoinAttemptPublished(ctx, *i.Input.JoinAttempt, operation); err != nil {
			return nil, err
		}
		if err = scoped.markMemberDecisionPublished(ctx, *i.Input.JoinAttempt, i.Input.DecisionBasisSHA256, operation); err != nil {
			return nil, err
		}
	}
	if publicationHook != nil {
		hookIntent := *i
		hookIntent.State = "PUBLISHED"
		hookIntent.Input = cloneMemberAdmissionInput(i.Input)
		hookIntent.BindingSHA256 = append([]byte(nil), i.BindingSHA256...)
		hookMember := *member
		if member.Nickname != nil {
			nickname := *member.Nickname
			hookMember.Nickname = &nickname
		}
		if err = publicationHook(ctx, &scoped, &hookIntent, &hookMember); err != nil {
			return nil, err
		}
		actual, readErr := readOrdinaryMemberAuthority(ctx, tx, i.Input.SpaceID, i.Input.ProfileID)
		if readErr != nil {
			return nil, readErr
		}
		binding := actual.GetPublishedBinding().GetOrdinary()
		if actual.GetState() != spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE || actual.GetPublicationKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY || binding.GetMembershipGeneration() != i.Generation || binding.GetPublicationIdentity() != operation.String() || !bytes.Equal(actual.GetRoleReceiptSha256(), receipt) {
			return nil, ErrMemberAuthorityUnavailable
		}
	}
	if continuation != nil {
		if err = scoped.checkContinuationReviewer(ctx, continuation); err != nil {
			return nil, err
		}
	}
	if effective != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, effective); err != nil {
			return nil, err
		}
	}
	if continuation != nil {
		if _, err = continuation.check(i); err != nil {
			return nil, err // Includes every SQL/hook wait; whole publication rolls back.
		}
	}
	// The initiating ingress budget may be older than every continuation/history
	// budget. This pure restriction follows ALL SQL and cannot move into either
	// PRE authorization or POST journal bookkeeping. No SQL follows before commit.
	if initiating != nil {
		if err = initiating.check(ctx); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return member, nil
}

func knownMemberAdmissionGuardDenial(err error) bool {
	return !errors.Is(err, ErrMemberAdmissionConflict) && knownMemberAdmissionDenial(err)
}

func knownMemberAdmissionDenial(err error) bool {
	return errors.Is(err, ErrAccountBanned) || errors.Is(err, ErrAlreadySpaceMember) || errors.Is(err, ErrAdmissionRequired) || errors.Is(err, ErrEntryPolicyVersion) || errors.Is(err, ErrInviteNotFound) || errors.Is(err, ErrInviteRevoked) || errors.Is(err, ErrInviteExpired) || errors.Is(err, ErrInviteMaxUses) || errors.Is(err, ErrGuestsNotAllowed) || errors.Is(err, ErrMemberCapReached) || errors.Is(err, ErrMemberAdmissionConflict) || errors.Is(err, ErrJoinAttemptDenied)
}
