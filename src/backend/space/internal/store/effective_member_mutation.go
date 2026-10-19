package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
)

type effectiveMemberMutation struct {
	profile, cutover uuid.UUID
	head             effectiveHead
	before           effectiveSources
}

// This transaction-local capability cannot be supplied by a caller or reused
// with another transaction. No remote owner call belongs between these hooks.
type effectiveMemberMutations struct {
	tx       pgx.Tx
	space    uuid.UUID
	started  time.Time
	profiles []effectiveMemberMutation
	used     bool
}

// Pre-ledger native39/40 have no effective-history storage. Forward41 always
// captures; a missing cutover head for an existing source fails unavailable.
func (s *SpaceStore) captureNativeEffectiveMemberMutation(ctx context.Context, space uuid.UUID, profiles []uuid.UUID) (*effectiveMemberMutations, error) {
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if version == 39 || version == 40 {
		return nil, nil
	}
	return s.captureEffectiveMemberMutations(ctx, space, profiles)
}

func (s *SpaceStore) effectiveMutationScope(ctx context.Context, space uuid.UUID) error {
	if s == nil || s.Pool == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	var held bool
	if err := s.tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation')='read committed'
 AND EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND pid=pg_backend_pid()
 AND granted AND mode='ExclusiveLock' AND objsubid=1
 AND classid::bigint=(($1::bigint>>32)&4294967295) AND objid::bigint=($1::bigint&4294967295))`, spaceMutationAdvisoryKey(space)).Scan(&held); err != nil || !held {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || version != 41 {
		return ErrEffectiveEpisodeUnavailable
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, space)
	if err != nil || lifecycle.GetState() != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE {
		return ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, space)
	if err != nil || ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

func (s *SpaceStore) readEffectiveMemberSources(ctx context.Context, space, profile uuid.UUID, sampled time.Time) (effectiveSources, error) {
	ordinary, err := readOrdinaryMemberAuthority(ctx, s.tx, space, profile)
	if err != nil {
		return effectiveSources{}, ErrEffectiveEpisodeUnavailable
	}
	community, err := readCommunityMemberAuthority(ctx, s.tx, space, profile, sampled)
	if err != nil {
		return effectiveSources{}, ErrEffectiveEpisodeUnavailable
	}
	var sources effectiveSources
	if ordinary.GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		b := ordinary.GetPublishedBinding().GetOrdinary()
		if b == nil {
			return sources, ErrEffectiveEpisodeUnavailable
		}
		sources.Ordinary = fmt.Sprintf("o/%d/%s/%t", b.GetMembershipGeneration(), b.GetPublicationIdentity(), b.GetLegacy())
		sources.Legacy = b.GetLegacy()
	}
	if community.GetState() == spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
		b := community.GetBinding().GetCommunity()
		if b == nil || community.GetValidityRemaining().AsDuration() <= 0 {
			return effectiveSources{}, ErrEffectiveEpisodeUnavailable
		}
		sources.Community = fmt.Sprintf("c/%d/%d/%s", b.GetOwnerGeneration(), b.GetMembershipGeneration(), b.GetMembershipEpisodeId())
		sources.CommunityUntil = sampled.Add(community.GetValidityRemaining().AsDuration())
	}
	return sources, nil
}

func (s *SpaceStore) captureEffectiveMemberMutations(ctx context.Context, space uuid.UUID, profiles []uuid.UUID) (*effectiveMemberMutations, error) {
	if err := s.effectiveMutationScope(ctx, space); err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	for _, p := range profiles {
		if p == uuid.Nil {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	}
	profiles = normalizedOwnershipSpaceIDs(profiles)
	capture := &effectiveMemberMutations{tx: s.tx, space: space}
	if err := s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&capture.started); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	revision, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	for _, profile := range profiles {
		sources, err := s.readEffectiveMemberSources(ctx, space, profile, capture.started)
		if err != nil {
			return nil, err
		}
		var exists bool
		if err := s.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2)`, space, profile).Scan(&exists); err != nil {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		item := effectiveMemberMutation{profile: profile, cutover: uuid.New(), before: sources}
		if exists {
			item.head, item.cutover, err = loadEffectiveHead(ctx, s.tx, space, profile)
			if err != nil {
				return nil, err
			}
		} else if sources.active() {
			// A missing old head is not an implicit UNKNOWN cutover or entry.
			return nil, ErrEffectiveEpisodeUnavailable
		}
		if !item.head.Sources.at(capture.started).equal(sources) {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		capture.profiles = append(capture.profiles, item)
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var finished time.Time
	if err := s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(capture.started) || !finished.Before(capture.started.Add(5*time.Second)) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	for _, item := range capture.profiles {
		if !item.before.equal(item.before.at(finished)) {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	}
	return capture, nil
}

func (s *SpaceStore) finishEffectiveMemberMutations(ctx context.Context, capture *effectiveMemberMutations) error {
	if capture == nil || capture.used || s == nil || s.tx == nil || capture.tx != s.tx {
		return ErrEffectiveEpisodeUnavailable
	}
	capture.used = true
	if err := s.effectiveMutationScope(ctx, capture.space); err != nil {
		return err
	}
	var sampled time.Time
	if err := s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil || sampled.Before(capture.started) {
		return ErrEffectiveEpisodeUnavailable
	}
	revision, err := spaceSourceRevision(ctx, s.tx, capture.space.String())
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	next := make([]effectiveHead, len(capture.profiles))
	for index, item := range capture.profiles {
		after, err := s.readEffectiveMemberSources(ctx, capture.space, item.profile, sampled)
		if err != nil {
			return err
		}
		next[index], err = advanceEffectiveEpisode(item.head, item.before, after, capture.started, sampled, uuid.New())
		if err != nil {
			return err
		}
		if err = saveEffectiveHead(ctx, s.tx, capture.space, item.profile, item.cutover, item.head.Revision, next[index]); err != nil {
			return err
		}
	}
	// A per-profile check is insufficient: the last write can wait after the
	// first profile's finite support or OLD continuity bridge has expired.
	last, err := spaceSourceRevision(ctx, s.tx, capture.space.String())
	if err != nil || last != revision {
		return ErrEffectiveEpisodeUnavailable
	}
	var finished time.Time
	if err := s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(sampled) || !finished.Before(capture.started.Add(5*time.Second)) {
		return ErrEffectiveEpisodeUnavailable
	}
	for _, head := range next {
		if !head.Sources.equal(head.Sources.at(finished)) || (!head.bridgeUntil.IsZero() && !finished.Before(head.bridgeUntil)) {
			return ErrEffectiveEpisodeUnavailable
		}
	}
	return nil
}
