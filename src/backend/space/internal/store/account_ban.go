package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/spaceaccountcapture"
	"voice/backend/space/internal/lifecycleprincipal"
)

// SpaceAccountCaptureClient must be constructed with the protected Space-only
// capture transport, including full request binding, mTLS, replay and deadlines.
// This interface is not an authorization grant to an ordinary User client.
type SpaceAccountCaptureClient interface {
	CaptureSpaceMemberAccountBindings(context.Context, *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error)
}

var ErrAccountBanUnavailable = errors.New("complete account ban authority unavailable")

// AccountBanInput uses a durable intent identity, never a transport request/JTI.
// A retry must preserve every field. Public producer identity is a separate ABI.
type AccountBanInput struct {
	OperationID, SpaceID, AccountID, ActorProfileID uuid.UUID
	Reason                                          *string
	ProfileHint                                     *uuid.UUID
	DecisionStarted                                 time.Time                                 `json:"-"`
	InitiatingCall                                  *lifecycleprincipal.VerifiedAdmissionCall `json:"-"`
	Request                                         *spacev1.BanMemberRequest                 `json:"-"`
	Authorize                                       func(context.Context) error               `json:"-"`
}

type accountBanSource struct {
	entry         *userv1.SpaceAccountCaptureEntry
	nativeAccount uuid.UUID
}

type accountBanSnapshot struct {
	owner    uuid.UUID
	revision uint64
	sources  []accountBanSource
	request  *userv1.CaptureSpaceMemberAccountBindingsRequest
}

func deterministicBanBytes(m proto.Message) []byte {
	b, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	return b
}

// Exact-episode qualification augments identity only. Unknown old episodes keep
// their original provenance, while a positive disagreement fails closed locally.
func readQualifiedSourceAccount(ctx context.Context, tx pgx.Tx, space uuid.UUID, entry *userv1.SpaceAccountCaptureEntry, known *uuid.UUID) (*uuid.UUID, error) {
	var version int64
	if err := tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
		return nil, err
	}
	if version != 50 && version != 51 {
		return known, nil
	}
	var account uuid.UUID
	err := tx.QueryRow(ctx, `SELECT account_id FROM space_account_qualifications WHERE space_id=$1 AND source_bytes=$2`, space, deterministicBanBytes(entry)).Scan(&account)
	if errors.Is(err, pgx.ErrNoRows) {
		return known, nil
	}
	if err != nil {
		return nil, err
	}
	if account == uuid.Nil || (known != nil && *known != account) {
		return nil, ErrAccountBanUnavailable
	}
	return &account, nil
}

// A pre50 partial ban may have left unknown LEGACY0 sources behind. Removing
// its flag would make those sources eligible again. Only exact local episode
// qualifications or immutable native account facts can prove that every still
// active/preparing source belongs to another account. Full Ban reconciliation
// supplies missing qualifications and retires the target sources; Unban never
// guesses identity or retires unrelated membership as a side effect.
func (s *SpaceStore) requireRetiredUnbanSources(ctx context.Context, space, target uuid.UUID) error {
	if s == nil || s.tx == nil {
		return ErrAccountBanUnavailable
	}
	rows, err := s.tx.Query(ctx, `SELECT true,profile_id,generation,publication_id,account_id,0::bigint
 FROM space_ordinary_member_heads WHERE space_id=$1 AND state IN ('ACTIVE','PREPARING')
 UNION ALL SELECT false,profile_id,generation,episode_id,NULL::uuid,owner_generation
 FROM space_community_member_heads WHERE space_id=$1 AND state='ACTIVE'`, space)
	if err != nil {
		return err
	}
	type source struct {
		entry *userv1.SpaceAccountCaptureEntry
		known *uuid.UUID
	}
	var sources []source
	for rows.Next() {
		var ordinary bool
		var profile, identity uuid.UUID
		var generation, owner uint64
		var known *uuid.UUID
		if err = rows.Scan(&ordinary, &profile, &generation, &identity, &known, &owner); err != nil {
			rows.Close()
			return err
		}
		entry := &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String()}
		if ordinary {
			entry.Source = &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{Generation: generation, PublicationId: identity.String()}}
		} else {
			entry.Source = &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: owner, MemberGeneration: generation, EpisodeId: identity.String()}}
		}
		sources = append(sources, source{entry: entry, known: known})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, source := range sources {
		account, err := readQualifiedSourceAccount(ctx, s.tx, space, source.entry, source.known)
		if err != nil {
			return err
		}
		if account == nil || *account == uuid.Nil || *account == target {
			return ErrAccountBanUnavailable
		}
	}
	return nil
}

// Snapshot is short and local. ALL current/potential sources, including retired
// community heads retained by a roster, are included without a page limit.
func (s *SpaceStore) accountBanSnapshot(ctx context.Context, space, operation uuid.UUID) (*accountBanSnapshot, error) {
	if s.tx == nil {
		return nil, ErrAccountBanUnavailable
	}
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 50 && version != 51 {
		return nil, ErrAccountBanUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return nil, err
	}
	b := &accountBanSnapshot{}
	if err := s.tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, space).Scan(&b.owner); err != nil {
		return nil, err
	}
	var err error
	b.revision, err = spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil {
		return nil, err
	}
	rows, err := s.tx.Query(ctx, `SELECT profile_id,generation,publication_id,account_id FROM space_ordinary_member_heads
 WHERE space_id=$1 AND state IN ('ACTIVE','PREPARING') ORDER BY profile_id`, space)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var profile, publication uuid.UUID
		var generation uint64
		var account *uuid.UUID
		if err = rows.Scan(&profile, &generation, &publication, &account); err != nil {
			rows.Close()
			return nil, err
		}
		source := accountBanSource{entry: &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{Generation: generation, PublicationId: publication.String()}}}}
		if account != nil {
			source.nativeAccount = *account
		}
		b.sources = append(b.sources, source)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = s.tx.Query(ctx, `SELECT h.profile_id,h.owner_generation,h.generation,h.episode_id FROM space_community_member_heads h
 WHERE h.space_id=$1 AND (h.state='ACTIVE' OR EXISTS(SELECT 1 FROM community_roster_members m WHERE m.space_id=h.space_id AND m.profile_id=h.profile_id)) ORDER BY h.profile_id`, space)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var profile, episode uuid.UUID
		var owner, generation uint64
		if err = rows.Scan(&profile, &owner, &generation, &episode); err != nil {
			rows.Close()
			return nil, err
		}
		b.sources = append(b.sources, accountBanSource{entry: &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: owner, MemberGeneration: generation, EpisodeId: episode.String()}}}})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	// Keep each exact-source check while avoiding a network round trip per row.
	lookups := &pgx.Batch{}
	for i := range b.sources {
		lookups.Queue(`SELECT association_id::text,association_revision,account_id FROM space_account_qualifications WHERE space_id=$1 AND source_bytes=$2`, space, deterministicBanBytes(b.sources[i].entry))
	}
	results := s.tx.SendBatch(ctx, lookups)
	defer results.Close()
	ownerPresent := false
	for i := range b.sources {
		source := &b.sources[i]
		if source.entry.ProfileId == b.owner.String() {
			ownerPresent = true
		}
		var association string
		var revision uint64
		var qualifiedAccount uuid.UUID
		err = results.QueryRow().Scan(&association, &revision, &qualifiedAccount)
		if err == nil {
			if source.nativeAccount != uuid.Nil && source.nativeAccount != qualifiedAccount {
				return nil, ErrAccountBanUnavailable
			}
			source.nativeAccount = qualifiedAccount
			source.entry.ExpectedAssociationId = association
			source.entry.ExpectedAssociationRevision = revision
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	if err = results.Close(); err != nil {
		return nil, err
	}
	if !ownerPresent || b.revision <= 0 {
		return nil, ErrAccountBanUnavailable
	}
	sort.Slice(b.sources, func(i, j int) bool {
		return bytes.Compare(deterministicBanBytes(b.sources[i].entry), deterministicBanBytes(b.sources[j].entry)) < 0
	})
	cohort := &userv1.SpaceAccountCaptureCohort{}
	for _, source := range b.sources {
		cohort.Entries = append(cohort.Entries, source.entry)
	}
	b.request = &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: operation.String(), SpaceId: space.String(), Purpose: userv1.SpaceAccountCapturePurpose_SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT, Cohort: cohort, CohortDigest: spaceaccountcapture.CohortDigest(cohort)}
	if spaceaccountcapture.ValidateRequest(b.request) != nil {
		return nil, ErrAccountBanUnavailable
	}
	return b, nil
}

// accountBanCoverage validates identity independently of profile eligibility:
// frozen/soft-deleted ASSOCIATED rows still match. A genuine terminal fact needs
// its separate exact-source reconciliation before a ban retry; it is not a
// license to retire unrelated sources inside an otherwise invalid account ban.
func accountBanCoverage(in AccountBanInput, snapshot *accountBanSnapshot, response *userv1.CaptureSpaceMemberAccountBindingsResponse) ([]uuid.UUID, error) {
	if snapshot == nil || spaceaccountcapture.ValidateResponse(snapshot.request, response) != nil {
		return nil, ErrAccountBanUnavailable
	}
	accounts := map[uuid.UUID]uuid.UUID{}
	for i, result := range response.Results {
		if result.Outcome != userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED {
			return nil, ErrAccountBanUnavailable
		}
		profile := uuid.MustParse(result.Entry.ProfileId)
		account := uuid.MustParse(result.AccountId)
		if known := snapshot.sources[i].nativeAccount; known != uuid.Nil && known != account {
			return nil, ErrAccountBanUnavailable
		}
		if prior, ok := accounts[profile]; ok && prior != account {
			return nil, ErrAccountBanUnavailable
		}
		accounts[profile] = account
	}
	ownerAccount, ok := accounts[snapshot.owner]
	if !ok {
		return nil, ErrAccountBanUnavailable
	}
	if ownerAccount == in.AccountID {
		return nil, ErrCannotBanOwner
	}
	if in.ProfileHint != nil && accounts[*in.ProfileHint] != in.AccountID {
		return nil, ErrMemberAdmissionConflict
	}
	var matches []uuid.UUID
	for profile, account := range accounts {
		if account == in.AccountID {
			matches = append(matches, profile)
		}
	}
	return normalizedOwnershipSpaceIDs(matches), nil
}

// BanAccount owns the stable public intent, immutable private capture attempts,
// and one guarded retirement. No remote observation runs under a Space fence.
func (s *SpaceStore) BanAccount(ctx context.Context, in AccountBanInput) error {
	if s == nil || s.Pool == nil || s.tx != nil || in.InitiatingCall == nil || in.Request == nil {
		return ErrAccountBanUnavailable
	}
	if in.SpaceID == uuid.Nil || in.AccountID == uuid.Nil || in.ActorProfileID == uuid.Nil || (in.ProfileHint != nil && *in.ProfileHint == uuid.Nil) {
		return status.Error(codes.InvalidArgument, "invalid ban selector")
	}
	in.Request = proto.Clone(in.Request).(*spacev1.BanMemberRequest)
	if in.Reason != nil {
		value := *in.Reason
		in.Reason = &value
	}
	if in.ProfileHint != nil {
		value := *in.ProfileHint
		in.ProfileHint = &value
	}
	intent, err := newModerationIntent(in.InitiatingCall, in.Request, in.SpaceID, "BAN")
	if err != nil {
		return err
	}
	if intent.actor != in.ActorProfileID || (in.OperationID != uuid.Nil && intent.operation != in.OperationID) {
		return ErrMemberAdmissionConflict
	}
	want := &spacev1.BanMemberRequest{SpaceId: in.SpaceID.String(), AccountId: in.AccountID.String(), Reason: in.Reason}
	if in.ProfileHint != nil {
		value := in.ProfileHint.String()
		want.ProfileId = &value
	}
	field := want.ProtoReflect().Descriptor().Fields().ByName("operation_id")
	if field == nil {
		return ErrAccountBanUnavailable
	}
	want.ProtoReflect().Set(field, in.Request.ProtoReflect().Get(field))
	if !proto.Equal(want, in.Request) {
		return status.Error(codes.InvalidArgument, "invalid ban request")
	}
	replay, err := s.probeModerationIntent(ctx, intent)
	if err != nil || replay {
		return err
	}
	if in.Authorize == nil || s.AccountCaptures == nil {
		return ErrAccountBanUnavailable
	}
	if err = in.Authorize(ctx); err != nil {
		return err
	}
	replay, err = s.claimModerationIntent(ctx, intent)
	if err != nil || replay {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			s.releaseModerationIntent(ctx, intent)
		}
	}()
	for {
		if _, err = in.InitiatingCall.Remaining(in.Request); err != nil {
			return err
		}
		var snapshot *accountBanSnapshot
		var sequence int64
		err = s.withModerationIntentScope(ctx, intent, func(scoped *SpaceStore) error {
			root, err := scoped.requireCurrentModerationExecution(ctx, intent)
			if err != nil {
				return err
			}
			if err = scoped.requireModerationActor(ctx, intent); err != nil {
				return err
			}
			captureID := uuid.New()
			if root.selected != nil {
				captureID = *root.selected
			}
			snapshot, err = scoped.accountBanSnapshot(ctx, in.SpaceID, captureID)
			if err != nil {
				return err
			}
			sequence = root.sequence
			if root.selected != nil {
				var manifest []byte
				var revision uint64
				var owner uuid.UUID
				err = scoped.tx.QueryRow(ctx, `SELECT cohort_bytes,source_revision,owner_profile_id FROM space_account_ban_captures
     WHERE capture_id=$1 AND actor_profile_id=$2 AND client_operation_id=$3 AND space_id=$4 AND attempt_seq=$5`, captureID, intent.actor, intent.operation, intent.space, sequence).Scan(&manifest, &revision, &owner)
				if err != nil {
					return err
				}
				if revision == snapshot.revision && owner == snapshot.owner && bytes.Equal(manifest, deterministicBanBytes(snapshot.request)) {
					return nil
				}
				captureID = uuid.New()
				snapshot.request.OperationId = captureID.String()
			}
			sequence++
			_, err = scoped.tx.Exec(ctx, `INSERT INTO space_account_ban_captures(capture_id,actor_profile_id,client_operation_id,space_id,attempt_seq,cohort_bytes,cohort_digest,source_revision,owner_profile_id)
    VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, captureID, intent.actor, intent.operation, intent.space, sequence, deterministicBanBytes(snapshot.request), snapshot.request.CohortDigest, snapshot.revision, snapshot.owner)
			if err != nil {
				return err
			}
			tag, err := scoped.tx.Exec(ctx, `UPDATE space_account_ban_operations SET selected_capture_id=$5,selected_attempt_seq=$6
    WHERE actor_profile_id=$1 AND client_operation_id=$2 AND body_sha256=$3 AND execution_id=$4 AND state='CAPTURING'
    AND selected_capture_id IS NOT DISTINCT FROM $7::uuid AND selected_attempt_seq=$8 AND clock_timestamp()<execution_expires_at`, intent.actor, intent.operation, intent.body, intent.execution, captureID, sequence, root.selected, root.sequence)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return ErrAccountBanUnavailable
			}
			return nil
		})
		if err != nil {
			return err
		}
		remaining, err := in.InitiatingCall.Remaining(in.Request)
		if err != nil {
			return err
		}
		callStarted := time.Now()
		callCtx, cancel := context.WithTimeout(ctx, min(2*time.Second, remaining))
		response, err := s.AccountCaptures.CaptureSpaceMemberAccountBindings(callCtx, proto.Clone(snapshot.request).(*userv1.CaptureSpaceMemberAccountBindingsRequest))
		callErr := callCtx.Err()
		cancel()
		if err != nil || callErr != nil || time.Since(callStarted) >= 2*time.Second {
			return ErrAccountBanUnavailable
		}
		if response != nil {
			response = proto.Clone(response).(*userv1.CaptureSpaceMemberAccountBindingsResponse)
		}
		profiles, err := accountBanCoverage(in, snapshot, response)
		if err != nil {
			if errors.Is(err, ErrAccountBanUnavailable) {
				if diagnostic := s.recordAccountBanReconciliation(ctx, intent, snapshot, response); diagnostic != nil {
					return errors.Join(ErrAccountBanUnavailable, diagnostic)
				}
			}
			return err
		}
		selected := uuid.MustParse(snapshot.request.OperationId)
		err = s.withModerationIntentScope(ctx, intent, func(scoped *SpaceStore) error {
			root, err := scoped.requireCurrentModerationExecution(ctx, intent)
			if err != nil {
				return err
			}
			if root.selected == nil || *root.selected != selected || root.sequence != sequence {
				return ErrAccountBanUnavailable
			}
			if err = scoped.requireModerationActor(ctx, intent); err != nil {
				return err
			}
			current, err := scoped.accountBanSnapshot(ctx, in.SpaceID, selected)
			if err != nil {
				return err
			}
			if current.owner != snapshot.owner || current.revision != snapshot.revision || !proto.Equal(current.request, snapshot.request) {
				return errAccountBanReplan
			}
			capture, err := scoped.captureMemberMutationIfPresent(ctx, in.SpaceID, profiles)
			if err != nil {
				return err
			}
			// Preserve the insert-then-verify order for every receipt, but pipeline
			// the statements under the same transaction and Space fence.
			qualifications := &pgx.Batch{}
			for _, result := range response.Results {
				entry := proto.Clone(result.Entry).(*userv1.SpaceAccountCaptureEntry)
				entry.ExpectedAssociationId = ""
				entry.ExpectedAssociationRevision = 0
				key := deterministicBanBytes(entry)
				qualifications.Queue(`INSERT INTO space_account_qualifications(space_id,source_bytes,profile_id,account_id,association_id,association_revision,owner_receipt_digest,result_bytes,operation_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(space_id,source_bytes) DO NOTHING`, in.SpaceID, deterministicBanBytes(entry), result.Entry.ProfileId, result.AccountId, result.AssociationId, result.AssociationRevision, result.OwnerReceiptDigest, deterministicBanBytes(result), snapshot.request.OperationId)
				qualifications.Queue(`SELECT account_id=$3 AND association_id=$4 AND association_revision=$5 FROM space_account_qualifications WHERE space_id=$1 AND source_bytes=$2`, in.SpaceID, key, result.AccountId, result.AssociationId, result.AssociationRevision)
			}
			qualificationResults := scoped.tx.SendBatch(ctx, qualifications)
			defer qualificationResults.Close()
			for range response.Results {
				if _, err = qualificationResults.Exec(); err != nil {
					return err
				}
				var equal bool
				if err = qualificationResults.QueryRow().Scan(&equal); err != nil || !equal {
					return ErrAccountBanUnavailable
				}
			}
			if err = qualificationResults.Close(); err != nil {
				return err
			}
			for _, profile := range profiles {
				if _, err = scoped.retirePendingMemberAdmission(ctx, in.SpaceID, profile); err != nil {
					return err
				}
			}
			// One statement still invokes every source guard/observer for the
			// entire qualified cohort; no trigger or profile is bypassed.
			if _, err = scoped.tx.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=ANY($2::uuid[])`, in.SpaceID, profiles); err != nil {
				return err
			}
			for _, profile := range profiles {
				_, err = scoped.tx.Exec(ctx, `INSERT INTO space_account_ban_suppressions(space_id,profile_id,account_id,owner_generation,source_revision,episode_id,member_generation)
 SELECT h.space_id,h.profile_id,$3,h.owner_generation,COALESCE(a.roster_source_revision,0),h.episode_id,h.generation FROM space_community_member_heads h
 LEFT JOIN community_owner_authority a ON a.space_id=h.space_id WHERE h.space_id=$1 AND h.profile_id=$2
 ON CONFLICT(space_id,profile_id) DO UPDATE SET owner_generation=EXCLUDED.owner_generation,source_revision=CASE WHEN EXCLUDED.owner_generation>space_account_ban_suppressions.owner_generation THEN EXCLUDED.source_revision ELSE GREATEST(space_account_ban_suppressions.source_revision,EXCLUDED.source_revision) END,episode_id=EXCLUDED.episode_id,member_generation=EXCLUDED.member_generation`, in.SpaceID, profile, in.AccountID)
				if err != nil {
					return err
				}
				if _, err = scoped.tx.Exec(ctx, `UPDATE community_roster_members SET revoked_at=COALESCE(revoked_at,clock_timestamp()) WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, profile); err != nil {
					return err
				}
				if _, err = scoped.tx.Exec(ctx, `DELETE FROM space_member_timeouts WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, profile); err != nil {
					return err
				}
			}
			if _, err = scoped.tx.Exec(ctx, `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id,reason) VALUES($1,$2,$3,$4) ON CONFLICT(space_id,account_id) DO UPDATE SET banned_by_profile_id=EXCLUDED.banned_by_profile_id,reason=EXCLUDED.reason,banned_at=clock_timestamp()`, in.SpaceID, in.AccountID, in.ActorProfileID, in.Reason); err != nil {
				return err
			}
			if err = scoped.refreshEffectiveMemberCount(ctx, in.SpaceID); err != nil {
				return err
			}
			details, _ := json.Marshal(struct {
				Reason *string `json:"reason,omitempty"`
			}{in.Reason})
			if _, err = scoped.tx.Exec(ctx, `INSERT INTO audit_log(space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,'member_banned','account',$3,$4::jsonb)`, in.SpaceID, in.ActorProfileID, in.AccountID, details); err != nil {
				return err
			}

			// Root commit is guarded while the native frame still exists. A deferred
			// database trigger repeats exact lease/selection/body checks at COMMIT.
			if err = scoped.commitModerationIntent(ctx, intent, &selected, sequence, deterministicBanBytes(response), profiles); err != nil {
				return err
			}
			if capture == nil {
				return ErrAccountBanUnavailable
			}
			if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
				return err
			}
			_, err = in.InitiatingCall.Remaining(in.Request)
			return err
		})
		if errors.Is(err, errAccountBanReplan) {
			continue
		}
		if err == nil {
			committed = true
		}
		return err
	}
}

var errAccountBanReplan = errors.New("account ban cohort changed; replan same intent")

// Evidence failures produce only bounded private diagnostics for the exact
// selected manifest. They never turn terminal/unknown into an account guess or
// retire unrelated sources as a side effect of a failed public Ban.
func (s *SpaceStore) recordAccountBanReconciliation(ctx context.Context, intent *moderationIntent, snapshot *accountBanSnapshot, response *userv1.CaptureSpaceMemberAccountBindingsResponse) error {
	return s.withModerationIntentScope(ctx, intent, func(scoped *SpaceStore) error {
		root, err := scoped.requireCurrentModerationExecution(ctx, intent)
		if err != nil {
			return err
		}
		capture := uuid.MustParse(snapshot.request.OperationId)
		if root.selected == nil || *root.selected != capture {
			return ErrAccountBanUnavailable
		}
		valid := spaceaccountcapture.ValidateResponse(snapshot.request, response) == nil
		accounts := map[string]string{}
		for i, source := range snapshot.sources {
			reason := "INVALID_RESPONSE"
			var resultBytes []byte
			if valid {
				result := response.Results[i]
				switch result.Outcome {
				case userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE:
					reason = "INCOMPLETE"
				case userv1.SpaceAccountCaptureOutcome_SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE:
					reason = "TERMINAL_PROFILE"
				default:
					previous, seen := accounts[result.Entry.ProfileId]
					accounts[result.Entry.ProfileId] = result.AccountId
					if (source.nativeAccount == uuid.Nil || source.nativeAccount.String() == result.AccountId) && (!seen || previous == result.AccountId) {
						continue
					}
					reason = "CONTRADICTION"
				}
				resultBytes = deterministicBanBytes(result)
			}
			_, err = scoped.tx.Exec(ctx, `INSERT INTO space_account_ban_reconciliation(capture_id,space_id,execution_id,source_bytes,profile_id,reason,result_bytes)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(capture_id,source_bytes) DO NOTHING`, capture, intent.space, intent.execution, deterministicBanBytes(source.entry), source.entry.ProfileId, reason, resultBytes)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *SpaceStore) UnbanAccount(ctx context.Context, call *lifecycleprincipal.VerifiedAdmissionCall, request *spacev1.UnbanMemberRequest, authorize func(context.Context) error) error {
	if s == nil || s.Pool == nil || s.tx != nil || call == nil || request == nil {
		return ErrAccountBanUnavailable
	}
	request = proto.Clone(request).(*spacev1.UnbanMemberRequest)
	space, err := canonicalLifecycleUUID(request.SpaceId)
	if err != nil {
		return err
	}
	account, err := canonicalLifecycleUUID(request.AccountId)
	if err != nil {
		return err
	}
	intent, err := newModerationIntent(call, request, space, "UNBAN")
	if err != nil {
		return err
	}
	replay, err := s.probeModerationIntent(ctx, intent)
	if err != nil || replay {
		return err
	}
	if authorize == nil {
		return ErrAccountBanUnavailable
	}
	if err = authorize(ctx); err != nil {
		return err
	}
	replay, err = s.claimModerationIntent(ctx, intent)
	if err != nil || replay {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			s.releaseModerationIntent(ctx, intent)
		}
	}()
	err = s.withModerationIntentScope(ctx, intent, func(scoped *SpaceStore) error {
		if _, err := scoped.requireCurrentModerationExecution(ctx, intent); err != nil {
			return err
		}
		if err := scoped.requireModerationActor(ctx, intent); err != nil {
			return err
		}
		return scoped.unbanAccountWithOutcome(ctx, space, account, intent.actor, func() error {
			return scoped.commitModerationIntent(ctx, intent, nil, 0, []byte{}, nil)
		})
	})
	if err == nil {
		committed = true
	}
	return err
}
