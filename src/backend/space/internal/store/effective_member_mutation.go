package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	commonv1 "voice.app/voice/common/v1"
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
	tx                pgx.Tx
	space             uuid.UUID
	started           time.Time
	profiles          []effectiveMemberMutation
	dbMutation        *nativeMembershipDBMutation
	purpose           effectiveMutationPurpose
	scheduleOperation uuid.UUID
	freezeGeneration  uint64
	freezeManifest    *commonv1.ManifestBinding
	freezeBinding     []byte
	freezeReceipt     []byte
	used              bool
}

type effectiveMutationPurpose uint8

const (
	effectiveMutationPositive effectiveMutationPurpose = iota
	effectiveMutationCandidateRetirement
	effectiveMutationLifecycleSchedule
	effectiveMutationLifecycleFreeze
)

// Only the Creator's already verified, preallocated new identity enters here.
// Unlike an existing Space mutation, no LIVE Space row can exist before this
// capture: the protected DB frame must precede the first spaces INSERT.
func (s *SpaceStore) captureBornCreatorEffectiveMutation(ctx context.Context, space, profile uuid.UUID) (*effectiveMemberMutations, error) {
	if profile == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, space); err != nil {
		return nil, err
	}
	version, err := s.memberDecisionSchema(ctx)
	if err != nil || (version != 44 && version != 45 && version != 50 && version != 51) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	parts := make([]string, 0, len(nativeMembershipGuardCoverage))
	for _, table := range nativeMembershipGuardCoverage {
		column := "space_id"
		if table == "spaces" {
			column = "id"
		}
		parts = append(parts, "SELECT 1 FROM "+pgx.Identifier{"public", table}.Sanitize()+" WHERE "+pgx.Identifier{column}.Sanitize()+"=$1")
	}
	var empty bool
	if err = s.tx.QueryRow(ctx, "SELECT NOT EXISTS("+strings.Join(parts, " UNION ALL ")+")", space).Scan(&empty); err != nil || !empty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	token, err := s.beginNativeMembershipDBMutation(ctx, space, []uuid.UUID{profile})
	if err != nil {
		return nil, err
	}
	if len(token.profiles) != 1 || token.profiles[0] != profile {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	capture := &effectiveMemberMutations{tx: s.tx, space: space, dbMutation: token, profiles: []effectiveMemberMutation{{profile: profile, cutover: uuid.New()}}}
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&capture.started); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	return capture, nil
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
	if err := s.effectiveMutationStorageScope(ctx, space); err != nil {
		return err
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

// Storage framing is not domain authorization. Positive writers still call
// effectiveMutationScope; the lifecycle owner uses this only after proving its
// saved DELETE operation and all current participant evidence.
func (s *SpaceStore) effectiveMutationStorageScope(ctx context.Context, space uuid.UUID) error {
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
	if err := s.tx.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || (version != 41 && version != 42 && version != 44 && version != 45 && version != 50 && version != 51) {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

// captureLifecyclePurgeMutation belongs to FinalizeLifecyclePurge's owned TX.
// It is not a positive mutation wrapper, installer fallback or caller-supplied
// maintenance grant. The administrative UNKNOWN cutover must already commit.
func (s *SpaceStore) captureLifecyclePurgeMutation(ctx context.Context, space uuid.UUID) (*nativeMembershipDBMutation, error) {
	var marker, native bool
	if s == nil || s.tx == nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,to_regclass('public.space_ordinary_member_heads') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !marker {
		if native {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return nil, nil
	}
	var version int64
	var dirty bool
	if err := s.tx.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); err != nil || dirty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	switch version {
	case 39, 40, 41, 42, 43:
		return nil, nil // preserved pre-DB-frame terminal protocol
	case 44, 45, 50:
	default:
		if version <= 34 && !native {
			return nil, nil
		}
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := s.effectiveMutationStorageScope(ctx, space); err != nil {
		return nil, err
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, space)
	if err != nil || lifecycle.GetState() != spacev1.MemberAdmissionLifecycleState_MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	ownership, err := readMemberOwnership(ctx, s.tx, space)
	if err != nil || (ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE && ownership.GetState() != spacev1.MemberAdmissionOwnershipState_MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	token, err := s.beginNativeMembershipDBMutation(ctx, space, nil)
	if err != nil {
		return nil, err
	}
	var started time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	revision, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	sources := make([]effectiveSources, 0, len(token.profiles))
	for _, profile := range token.profiles {
		source, e := s.readEffectiveMemberSources(ctx, space, profile, started)
		if e != nil {
			return nil, e
		}
		head, _, e := loadEffectiveHead(ctx, s.tx, space, profile)
		if e != nil || !head.Sources.at(started).equal(source) {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		sources = append(sources, source)
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var finished time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(started) || !finished.Before(started.Add(5*time.Second)) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	for _, source := range sources {
		if !source.equal(source.at(finished)) {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	}
	return token, nil
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
	return s.captureEffectiveMemberMutationFacts(ctx, space, profiles)
}

// Both callers establish their own domain scope before this private fact
// capture. Maintenance cannot select the positive admission scope by a flag.
func (s *SpaceStore) captureEffectiveMemberMutationFacts(ctx context.Context, space uuid.UUID, profiles []uuid.UUID) (*effectiveMemberMutations, error) {
	version, err := s.memberDecisionSchema(ctx)
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if len(profiles) == 0 && (version != 44 && version != 45 && version != 50 && version != 51) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	for _, p := range profiles {
		if p == uuid.Nil {
			return nil, ErrEffectiveEpisodeUnavailable
		}
	}
	profiles = normalizedOwnershipSpaceIDs(profiles)
	capture := &effectiveMemberMutations{tx: s.tx, space: space}
	if version == 44 || version == 45 || (version == 50 || version == 51) {
		capture.dbMutation, err = s.beginNativeMembershipDBMutation(ctx, space, profiles)
		if err != nil {
			return nil, err
		}
		// This includes unchanged owners and fanout members that were not in the
		// caller's requested subset. Every DB witness must be finalized.
		profiles = capture.dbMutation.profiles
	}
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
			return nil, fmt.Errorf("active source without effective head: %w", ErrEffectiveEpisodeUnavailable)
		}
		if !item.head.Sources.at(capture.started).equal(sources) {
			return nil, fmt.Errorf("effective head source mismatch: %w", ErrEffectiveEpisodeUnavailable)
		}
		capture.profiles = append(capture.profiles, item)
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var finished time.Time
	if err := s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(capture.started) || !finished.Before(capture.started.Add(5*time.Second)) {
		return nil, fmt.Errorf("effective capture observation expired after %s: %w", finished.Sub(capture.started), ErrEffectiveEpisodeUnavailable)
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
	var scopeErr error
	switch capture.purpose {
	case effectiveMutationPositive:
		scopeErr = s.effectiveMutationScope(ctx, capture.space)
	case effectiveMutationCandidateRetirement:
		scopeErr = s.memberAdmissionRetirementScope(ctx, capture.space)
	case effectiveMutationLifecycleSchedule:
		scopeErr = s.lifecycleScheduleMutationScope(ctx, capture)
	case effectiveMutationLifecycleFreeze:
		scopeErr = s.lifecycleFreezeMutationScope(ctx, capture)
	default:
		return ErrEffectiveEpisodeUnavailable
	}
	if scopeErr != nil {
		return scopeErr
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
			return fmt.Errorf("effective final source: %w", err)
		}
		next[index], err = advanceEffectiveEpisode(item.head, item.before, after, capture.started, sampled, uuid.New())
		if err != nil {
			return fmt.Errorf("effective episode transition: %w", err)
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
		return fmt.Errorf("effective final observation expired after %s: %w", finished.Sub(capture.started), ErrEffectiveEpisodeUnavailable)
	}
	for _, head := range next {
		if !head.Sources.equal(head.Sources.at(finished)) || (!head.bridgeUntil.IsZero() && !finished.Before(head.bridgeUntil)) {
			return ErrEffectiveEpisodeUnavailable
		}
	}
	if capture.dbMutation != nil {
		return s.completeNativeMembershipDBMutation(ctx, capture.dbMutation)
	}
	return nil
}

// Existing pre-native fixtures/official34 retain their old storage protocol.
// A native relation with a missing/dirty marker never falls back to that path.
func (s *SpaceStore) captureMemberRemovalMutation(ctx context.Context, space, profile uuid.UUID) (*effectiveMemberMutations, error) {
	return s.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{profile})
}

func (s *SpaceStore) captureMemberMutationIfPresent(ctx context.Context, space uuid.UUID, profiles []uuid.UUID) (*effectiveMemberMutations, error) {
	var marker, native bool
	if s == nil || s.tx == nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if err := s.tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL,to_regclass('public.space_ordinary_member_heads') IS NOT NULL`).Scan(&marker, &native); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !marker {
		if native {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return nil, nil
	}
	var version int64
	var dirty bool
	if err := s.tx.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&version, &dirty); err != nil || dirty {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if version <= 34 && !native {
		return nil, nil
	}
	return s.captureNativeEffectiveMemberMutation(ctx, space, profiles)
}
