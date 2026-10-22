package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"sort"
	"time"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/spaceaccountcapture"
)

var errCommunityAccountCaptureRequired = errors.New("community account capture required outside Space fence")

type communityAccountCapture struct {
	inputHash [32]byte
	owner     uuid.UUID
	revision  uint64
	started   time.Time
	request   *userv1.CaptureSpaceMemberAccountBindingsRequest
	response  *userv1.CaptureSpaceMemberAccountBindingsResponse
	banned    map[uuid.UUID]uuid.UUID
}

func communityAccountInputHash(in CommunityRosterInput) [32]byte {
	raw, _ := json.Marshal(in)
	return sha256.Sum256(raw)
}

func (s *SpaceStore) prepareCommunityAccountCapture(ctx context.Context, in CommunityRosterInput) (*communityAccountCapture, error) {
	if s == nil || s.Pool == nil || s.tx != nil || (s.AccountCaptures == nil && len(in.ProfileIDs) != 0) {
		return nil, ErrAccountBanUnavailable
	}
	c := &communityAccountCapture{inputHash: communityAccountInputHash(in), started: time.Now(), banned: map[uuid.UUID]uuid.UUID{}}
	native := map[uuid.UUID]uuid.UUID{}
	err := s.withOwnershipScope(ctx, []uuid.UUID{in.SpaceID}, func(scoped *SpaceStore) error {
		if err := checkMemberAuthorityCatalog(ctx, scoped.tx); err != nil {
			return err
		}
		if err := scoped.tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, in.SpaceID).Scan(&c.owner); err != nil {
			return err
		}
		var err error
		c.revision, err = spaceSourceRevision(ctx, scoped.tx, in.SpaceID.String())
		if err != nil {
			return err
		}
		cohort := &userv1.SpaceAccountCaptureCohort{}
		for _, profile := range in.ProfileIDs {
			if profile == uuid.Nil {
				return ErrCommunityRosterConflict
			}
			entry := &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_ProspectiveCommunity{ProspectiveCommunity: &userv1.SpaceProspectiveCommunityIdentity{OperationId: in.OperationID.String(), OwnerGeneration: uint64(in.OwnerGeneration), SourceRevision: uint64(in.SourceRevision)}}}
			var account *uuid.UUID
			err := scoped.tx.QueryRow(ctx, `SELECT account_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND state IN ('ACTIVE','PREPARING')`, in.SpaceID, profile).Scan(&account)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if account != nil {
				native[profile] = *account
			}
			known, association, revision, err := scoped.currentProfileAccountQualification(ctx, in.SpaceID, profile)
			if err != nil {
				return err
			}
			if known != uuid.Nil {
				if prior := native[profile]; prior != uuid.Nil && prior != known {
					return ErrAccountBanUnavailable
				}
				native[profile] = known
				entry.ExpectedAssociationId = association.String()
				entry.ExpectedAssociationRevision = revision
			}
			cohort.Entries = append(cohort.Entries, entry)
		}
		sort.Slice(cohort.Entries, func(i, j int) bool {
			return bytes.Compare(deterministicBanBytes(cohort.Entries[i]), deterministicBanBytes(cohort.Entries[j])) < 0
		})
		c.request = &userv1.CaptureSpaceMemberAccountBindingsRequest{Protocol: 1, OperationId: in.OperationID.String(), SpaceId: in.SpaceID.String(), Purpose: 3, Cohort: cohort, CohortDigest: spaceaccountcapture.CohortDigest(cohort)}
		if len(in.ProfileIDs) == 0 {
			c.request = nil
			return nil
		}
		return spaceaccountcapture.ValidateRequest(c.request)
	})
	if err != nil {
		return nil, err
	}
	// An empty roster publishes no identity. It needs no fabricated owner reply
	// and can still retire the previous roster during a User dependency outage.
	if c.request == nil {
		return c, nil
	}
	callStarted := time.Now()
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	out, err := s.AccountCaptures.CaptureSpaceMemberAccountBindings(call, proto.Clone(c.request).(*userv1.CaptureSpaceMemberAccountBindingsRequest))
	cancel()
	if err != nil || time.Since(callStarted) >= 2*time.Second || time.Since(c.started) >= 5*time.Second || spaceaccountcapture.ValidateResponse(c.request, out) != nil {
		return nil, ErrAccountBanUnavailable
	}
	c.response = proto.Clone(out).(*userv1.CaptureSpaceMemberAccountBindingsResponse)
	for _, result := range c.response.GetResults() {
		if result.Outcome != 1 {
			return nil, ErrAccountBanUnavailable
		}
		profile := uuid.MustParse(result.Entry.ProfileId)
		account := uuid.MustParse(result.AccountId)
		if known := native[profile]; known != uuid.Nil && known != account {
			return nil, ErrAccountBanUnavailable
		}
	}
	return c, nil
}

func (s *SpaceStore) filterQualifiedCommunityProfiles(ctx context.Context, in CommunityRosterInput) ([]uuid.UUID, error) {
	c := s.communityAccountCapture
	if s.tx == nil || c == nil || c.inputHash != communityAccountInputHash(in) || time.Since(c.started) >= 5*time.Second {
		return nil, ErrAccountBanUnavailable
	}
	var owner uuid.UUID
	if err := s.tx.QueryRow(ctx, `SELECT owner_profile_id FROM spaces WHERE id=$1`, in.SpaceID).Scan(&owner); err != nil || owner != c.owner {
		return nil, ErrCommunityRosterConflict
	}
	revision, err := spaceSourceRevision(ctx, s.tx, in.SpaceID.String())
	if err != nil || revision != c.revision {
		return nil, ErrCommunityRosterConflict
	}
	if len(in.ProfileIDs) == 0 {
		return []uuid.UUID{}, nil
	}
	accepted := map[uuid.UUID]bool{}
	for _, result := range c.response.GetResults() {
		profile := uuid.MustParse(result.Entry.ProfileId)
		account := uuid.MustParse(result.AccountId)
		banned, err := s.IsAccountBanned(ctx, in.SpaceID, account)
		if err != nil {
			return nil, err
		}
		if banned {
			c.banned[profile] = account
			continue
		}
		var generation, floor int64
		var bound uuid.UUID
		err = s.tx.QueryRow(ctx, `SELECT owner_generation,source_revision,account_id FROM space_account_ban_suppressions WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, profile).Scan(&generation, &floor, &bound)
		if err == nil {
			if bound != account {
				return nil, ErrAccountBanUnavailable
			}
			if in.OwnerGeneration < generation || (in.OwnerGeneration == generation && in.SourceRevision <= floor) {
				continue
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		accepted[profile] = true
	}
	profiles := make([]uuid.UUID, 0, len(accepted))
	for _, profile := range in.ProfileIDs {
		if accepted[profile] {
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}

// Persist exact owner facts with the actual published episode in the same native
// frame. The claimed input roster and its count remain unchanged in the owner
// operation; this separately records the effective filtered projection.
func (s *SpaceStore) recordCommunityAccountProjection(ctx context.Context, in CommunityRosterInput, effective []uuid.UUID) error {
	c := s.communityAccountCapture
	if c == nil || s.tx == nil || time.Since(c.started) >= 5*time.Second {
		return ErrAccountBanUnavailable
	}
	for profile, account := range c.banned {
		_, err := s.tx.Exec(ctx, `INSERT INTO space_account_ban_suppressions(space_id,profile_id,account_id,owner_generation,source_revision,episode_id,member_generation)
 VALUES($1,$2,$3,$4,$5,(SELECT episode_id FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2),COALESCE((SELECT generation FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2),0))
 ON CONFLICT(space_id,profile_id) DO UPDATE SET owner_generation=EXCLUDED.owner_generation,
 source_revision=CASE WHEN EXCLUDED.owner_generation>space_account_ban_suppressions.owner_generation THEN EXCLUDED.source_revision ELSE GREATEST(space_account_ban_suppressions.source_revision,EXCLUDED.source_revision) END`, in.SpaceID, profile, account, in.OwnerGeneration, in.SourceRevision)
		if err != nil {
			return err
		}
	}
	for _, result := range c.response.GetResults() {
		profile := uuid.MustParse(result.Entry.ProfileId)
		var generation, owner uint64
		var episode uuid.UUID
		err := s.tx.QueryRow(ctx, `SELECT generation,owner_generation,episode_id FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='ACTIVE'`, in.SpaceID, profile).Scan(&generation, &owner, &episode)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		entry := &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: owner, MemberGeneration: generation, EpisodeId: episode.String()}}}
		_, err = s.tx.Exec(ctx, `INSERT INTO space_account_qualifications(space_id,source_bytes,profile_id,account_id,association_id,association_revision,owner_receipt_digest,result_bytes,operation_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(space_id,source_bytes) DO NOTHING`, in.SpaceID, deterministicBanBytes(entry), profile, result.AccountId, result.AssociationId, result.AssociationRevision, result.OwnerReceiptDigest, deterministicBanBytes(result), in.OperationID)
		if err != nil {
			return err
		}
		var same bool
		if err = s.tx.QueryRow(ctx, `SELECT account_id=$3 AND association_id=$4 AND association_revision=$5 FROM space_account_qualifications WHERE space_id=$1 AND source_bytes=$2`, in.SpaceID, deterministicBanBytes(entry), result.AccountId, result.AssociationId, result.AssociationRevision).Scan(&same); err != nil || !same {
			return ErrAccountBanUnavailable
		}
	}
	_, err := s.tx.Exec(ctx, `INSERT INTO space_account_roster_projections(operation_id,space_id,effective_profile_ids) VALUES($1,$2,$3)`, in.OperationID, in.SpaceID, effective)
	return err
}

func (s *SpaceStore) loadCommunityAccountProjection(ctx context.Context, receipt *CommunityRosterReceipt) error {
	var version int64
	if err := s.db().QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
		return ErrAccountBanUnavailable
	}
	if version != 50 {
		receipt.EffectiveProfileIDs = append([]uuid.UUID(nil), receipt.ProfileIDs...)
		return nil
	}
	return s.db().QueryRow(ctx, `SELECT effective_profile_ids FROM space_account_roster_projections WHERE operation_id=$1 AND space_id=$2`, receipt.OperationID, receipt.SpaceID).Scan(&receipt.EffectiveProfileIDs)
}

// Only the exact current local head can select a saved association. Historical
// qualifications never become a global profile/account lookup or rebind a head.
func (s *SpaceStore) currentProfileAccountQualification(ctx context.Context, space, profile uuid.UUID) (uuid.UUID, uuid.UUID, uint64, error) {
	if s.tx == nil {
		return uuid.Nil, uuid.Nil, 0, ErrAccountBanUnavailable
	}
	var entries []*userv1.SpaceAccountCaptureEntry
	var generation uint64
	var publication uuid.UUID
	err := s.tx.QueryRow(ctx, `SELECT generation,publication_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 AND state IN ('ACTIVE','PREPARING')`, space, profile).Scan(&generation, &publication)
	if err == nil {
		entries = append(entries, &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Ordinary{Ordinary: &userv1.SpaceOrdinaryPublicationIdentity{Generation: generation, PublicationId: publication.String()}}})
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, 0, err
	}
	var owner uint64
	err = s.tx.QueryRow(ctx, `SELECT owner_generation,generation,episode_id FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space, profile).Scan(&owner, &generation, &publication)
	if err == nil {
		entries = append(entries, &userv1.SpaceAccountCaptureEntry{ProfileId: profile.String(), Source: &userv1.SpaceAccountCaptureEntry_Community{Community: &userv1.SpaceCommunityPublicationIdentity{OwnerGeneration: owner, MemberGeneration: generation, EpisodeId: publication.String()}}})
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, uuid.Nil, 0, err
	}
	var account, association uuid.UUID
	var revision uint64
	for _, entry := range entries {
		var nextAccount, nextAssociation uuid.UUID
		var nextRevision uint64
		err = s.tx.QueryRow(ctx, `SELECT account_id,association_id,association_revision FROM space_account_qualifications WHERE space_id=$1 AND source_bytes=$2`, space, deterministicBanBytes(entry)).Scan(&nextAccount, &nextAssociation, &nextRevision)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return uuid.Nil, uuid.Nil, 0, err
		}
		if account != uuid.Nil && (account != nextAccount || association != nextAssociation || revision != nextRevision) {
			return uuid.Nil, uuid.Nil, 0, ErrAccountBanUnavailable
		}
		account, association, revision = nextAccount, nextAssociation, nextRevision
	}
	return account, association, revision, nil
}
