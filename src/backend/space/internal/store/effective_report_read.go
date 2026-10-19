package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	rolev1 "voice.app/voice/role/v1"
)

var ErrSpaceReportReadDenied = errors.New("space report read context denied")

// ReadProtectedSpaceReportReadContext is the local protected producer; request
// authorization and Role/content decisions remain outside the SQL snapshot.
func (s *SpaceStore) ReadProtectedSpaceReportReadContext(ctx context.Context, space, profile, chat uuid.UUID) (*SpaceReportReadContext, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil || profile == uuid.Nil || chat == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	cut, err := withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(scoped *SpaceStore) (*SpaceReportReadContext, error) {
		if err := scoped.RequireScopedNativeMembershipReadiness(ctx, space); err != nil {
			return nil, ErrEffectiveEpisodeUnavailable
		}
		return scoped.readSpaceReportReadContext(ctx, space, profile, chat)
	})
	if errors.Is(err, ErrSpaceReportReadDenied) {
		return nil, ErrSpaceReportReadDenied
	}
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	return cut, nil
}

// SpaceReportReadContext contains current owner-local positioning facts, not a
// permission or completed all-writer cutover capability. Protected serving must
// additionally require the genuine native readiness producer before disclosure.
type SpaceReportReadContext struct {
	Eligible                           bool
	SpaceID, ProfileID, ChatID         uuid.UUID
	TreeNodeID, EpisodeID              uuid.UUID
	SpaceRevision, LifecycleGeneration uint64
	EpisodeGeneration, LedgerRevision  int64
	SourceBinding                      *rolev1.MembershipBinding
	HistoryKnown                       bool
	HistoryVisibleSince, SampledAt     time.Time
	ValidityRemaining                  time.Duration
}

// ReadSpaceReportReadContext never adopts an old source or repairs a missing
// ledger. All database reads finish under one native Space fence; no caller may
// pass an ambient transaction or run a remote owner request inside this scope.
func (s *SpaceStore) ReadSpaceReportReadContext(ctx context.Context, space, profile, chat uuid.UUID) (*SpaceReportReadContext, error) {
	if s == nil || s.Pool == nil || s.tx != nil || ctx == nil || ctx.Err() != nil || space == uuid.Nil || profile == uuid.Nil || chat == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	cut, err := withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(scoped *SpaceStore) (*SpaceReportReadContext, error) {
		return scoped.readSpaceReportReadContext(ctx, space, profile, chat)
	})
	if errors.Is(err, ErrSpaceReportReadDenied) {
		return nil, ErrSpaceReportReadDenied
	}
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	return cut, nil
}

func (s *SpaceStore) readSpaceReportReadContext(ctx context.Context, space, profile, chat uuid.UUID) (*SpaceReportReadContext, error) {
	if err := s.effectiveMutationScope(ctx, space); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	revision, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || revision == 0 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var sampled time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&sampled); err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	sources, err := s.readEffectiveMemberSources(ctx, space, profile, sampled)
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	if !sources.active() {
		return nil, ErrSpaceReportReadDenied
	}
	bindings, err := s.ReadScopedMemberSourceBindings(ctx, space, profile)
	if err != nil || len(bindings) == 0 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	var node uuid.UUID
	err = s.tx.QueryRow(ctx, `SELECT id FROM space_tree_nodes WHERE space_id=$1 AND chat_id=$2 AND kind='text_chat' FOR SHARE`, space, chat).Scan(&node)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrSpaceReportReadDenied
	}
	if err != nil || node == uuid.Nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	head, _, err := loadEffectiveHead(ctx, s.tx, space, profile)
	if err != nil || !head.Active || sampled.Before(head.ObservedAt) || !head.Sources.at(sampled).equal(sources) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	lifecycle, err := readMemberLifecycle(ctx, s.tx, space)
	if err != nil {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	last, err := spaceSourceRevision(ctx, s.tx, space.String())
	if err != nil || last != revision {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	// The head load can wait on a row lock after earlier source/lease reads.
	// Sampling before that wait would incorrectly extend finite support.
	var finished time.Time
	if err = s.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&finished); err != nil || finished.Before(sampled) || !finished.Before(sampled.Add(5*time.Second)) || !sources.equal(sources.at(finished)) {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	remaining := 5*time.Second - finished.Sub(sampled)
	if sources.Community != "" && sources.CommunityUntil.Sub(finished) < remaining {
		remaining = sources.CommunityUntil.Sub(finished)
	}
	if remaining <= 0 {
		return nil, ErrEffectiveEpisodeUnavailable
	}
	return &SpaceReportReadContext{Eligible: true, SpaceID: space, ProfileID: profile, ChatID: chat, TreeNodeID: node, EpisodeID: head.EpisodeID, SpaceRevision: revision, LifecycleGeneration: lifecycle.GetGeneration(), EpisodeGeneration: head.Generation, LedgerRevision: head.Revision, SourceBinding: bindings[0], HistoryKnown: head.Known, HistoryVisibleSince: head.Since, SampledAt: finished, ValidityRemaining: remaining}, nil
}
