package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	spacev1 "voice.app/voice/space/v1"
)

// The DB, rather than possession of this object, owns capture and completion.
// profiles is the complete DB-derived cohort plus prospective new profiles.
// The native writer must finalize that entire set before completing the token.
type nativeMembershipDBMutation struct {
	tx           pgx.Tx
	space, token uuid.UUID
	profiles     []uuid.UUID
	used         bool
}

func (s *SpaceStore) beginNativeMembershipDBMutation(ctx context.Context, space uuid.UUID, profiles []uuid.UUID) (*nativeMembershipDBMutation, error) {
	if s == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil || len(profiles) == 0 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	capture := &nativeMembershipDBMutation{tx: s.tx, space: space}
	if err := s.tx.QueryRow(ctx, `SELECT public.space_native_membership_begin($1,$2)`, space, profiles).Scan(&capture.token); err != nil || capture.token == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	// The wrapper exposes the DB cohort only to this transaction's frame. No
	// caller-supplied list or page-limited membership read certifies completeness.
	rows, err := s.tx.Query(ctx, `SELECT profile FROM public.space_native_membership_profiles($1,$2) profile`, space, capture.token)
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	defer rows.Close()
	for rows.Next() {
		var profile uuid.UUID
		if rows.Scan(&profile) != nil {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		capture.profiles = append(capture.profiles, profile)
	}
	if rows.Err() != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	return capture, nil
}

func (s *SpaceStore) completeNativeMembershipDBMutation(ctx context.Context, capture *nativeMembershipDBMutation) error {
	if s == nil || s.tx == nil || capture == nil || capture.used || capture.tx != s.tx || ctx == nil || ctx.Err() != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	capture.used = true
	_, err := s.tx.Exec(ctx, `SELECT public.space_native_membership_complete($1,$2)`, capture.space, capture.token)
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

// RequireScopedNativeMembershipReadiness performs no RPC, repair or cutover.
// Native catalog44 qualification and all production placements are a separate
// producer-owned integration requirement; schema version alone never suffices.
func (s *SpaceStore) RequireScopedNativeMembershipReadiness(ctx context.Context, space uuid.UUID) error {
	if s == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if _, err := s.tx.Exec(ctx, `SELECT public.space_native_membership_require($1)`, space); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	return nil
}

// InitializeScopedNativeMembership is an administrative deployment producer,
// never an ordinary app/read fallback. PostgreSQL separately restricts EXECUTE
// to the installer role. The caller supplies an already fenced RC transaction.
func (s *SpaceStore) InitializeScopedNativeMembership(ctx context.Context, space uuid.UUID) error {
	if s == nil || s.tx == nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil {
		return ErrEffectiveEpisodeUnavailable
	}
	if err := checkMemberAuthorityCatalog(ctx, s.tx); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	// scope verifies RC, the actual backend fence and exact installed guard before
	// any native evidence is sampled. Its helper is administrator-only.
	if _, err := s.tx.Exec(ctx, `SELECT native_membership_guard.scope($1)`, space); err != nil {
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
	revision, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	var started time.Time
	if s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&started) != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	rows, err := s.tx.Query(ctx, `SELECT native_membership_guard.cohort($1) ORDER BY 1`, space)
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	var profiles []uuid.UUID
	for rows.Next() {
		var profile uuid.UUID
		if rows.Scan(&profile) != nil {
			rows.Close()
			return ErrEffectiveEpisodeUnavailable
		}
		profiles = append(profiles, profile)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	sources := make([]effectiveSources, len(profiles))
	for index, profile := range profiles {
		sources[index], err = s.readEffectiveMemberSources(ctx, space, profile, started)
		if err != nil {
			return ErrEffectiveEpisodeUnavailable
		}
	}
	if _, err = s.tx.Exec(ctx, `SELECT public.space_native_membership_initialize($1)`, space); err != nil {
		return ErrEffectiveEpisodeUnavailable
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return ErrEffectiveEpisodeUnavailable
	}
	var finished time.Time
	if s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished) != nil || finished.Before(started) || !finished.Before(started.Add(5*time.Second)) {
		return ErrEffectiveEpisodeUnavailable
	}
	for _, source := range sources {
		if !source.equal(source.at(finished)) {
			return ErrEffectiveEpisodeUnavailable
		}
	}
	return nil
}
