package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"voice/backend/pkg/authoritysource"
)

var (
	ErrTreeViewerDenied  = errors.New("space tree viewer denied")
	ErrTreeViewerChanged = errors.New("space tree authority changed")
)

// SpaceTreeViewerCut is internal positioning evidence, never a permission grant.
// Callers must validate it again after external enrichment before disclosure.
type SpaceTreeViewerCut struct {
	SpaceID, ProfileID uuid.UUID
	Revision           uint64
	Data               *SpaceTreeData
}

// ReadSpaceTreeForViewer reads membership, the complete local tree and its
// authority revision under one qualified read-only lifecycle/ownership cut. The
// existing membership read preserves ordinary and leased-community semantics.
func (s *SpaceStore) ReadSpaceTreeForViewer(ctx context.Context, spaceID, profileID uuid.UUID) (*SpaceTreeViewerCut, error) {
	if spaceID == uuid.Nil || profileID == uuid.Nil {
		return nil, ErrTreeViewerDenied
	}
	var result *SpaceTreeViewerCut
	err := s.withQualifiedTreeViewerRead(ctx, spaceID, func(ctx context.Context, scoped *SpaceStore) error {
		member, err := scoped.IsSpaceMember(ctx, spaceID, profileID)
		if err != nil {
			return err
		}
		if !member {
			return ErrTreeViewerDenied
		}
		revision, err := spaceSourceRevision(ctx, scoped.tx, spaceID.String())
		if err != nil {
			return errors.Join(ErrTreeViewerChanged, err)
		}
		data, err := scoped.ListSpaceTree(ctx, spaceID)
		if err != nil {
			return err
		}
		cut := &SpaceTreeViewerCut{SpaceID: spaceID, ProfileID: profileID, Revision: revision, Data: data}
		// Direct owner writes also advance the durable source revision. Detect a
		// change between the component reads rather than returning a mixed tree.
		if err := scoped.validateTreeViewerCut(ctx, cut); err != nil {
			return err
		}
		result = cut
		return nil
	})
	return result, err
}

// ValidateSpaceTreeViewer obtains a fresh current cut; no remote dependency runs
// inside its lock. A removed/rejoined viewer cannot reuse an earlier revision.
func (s *SpaceStore) ValidateSpaceTreeViewer(ctx context.Context, cut *SpaceTreeViewerCut) error {
	if cut == nil || cut.SpaceID == uuid.Nil || cut.ProfileID == uuid.Nil || cut.Revision == 0 {
		return ErrTreeViewerDenied
	}
	return s.withQualifiedTreeViewerRead(ctx, cut.SpaceID, func(ctx context.Context, scoped *SpaceStore) error {
		return scoped.validateTreeViewerCut(ctx, cut)
	})
}

func (s *SpaceStore) validateTreeViewerCut(ctx context.Context, cut *SpaceTreeViewerCut) error {
	member, err := s.IsSpaceMember(ctx, cut.SpaceID, cut.ProfileID)
	if err != nil {
		return err
	}
	if !member {
		return ErrTreeViewerDenied
	}
	revision, err := spaceSourceRevision(ctx, s.tx, cut.SpaceID.String())
	if err != nil || revision != cut.Revision {
		return errors.Join(ErrTreeViewerChanged, err)
	}
	return nil
}

// These owner migration versions preserve the exact revision functions and
// trigger coverage used by this tree read. Unknown future versions fail closed;
// their additional membership semantics must be qualified before adding a pin.
// The separate Federation source service's exact v25 gate is unchanged.
func qualifiedTreeViewerVersion(version int64) bool {
	switch version {
	case 24, 25, 26, 27, 28, 29, 30, 31, 34:
		return true
	default:
		return false
	}
}

// withQualifiedTreeViewerRead does not rely on optional source-service startup.
// BeginRead locks schema relations, repeats exact version/dirty validation and
// checks the canonical tables, triggers and function-body hashes inside the
// SAME read-only repeatable-read snapshot as membership and tree data. No
// ambient tx or earlier successful preflight is reusable assurance.
func (s *SpaceStore) withQualifiedTreeViewerRead(ctx context.Context, spaceID uuid.UUID, action func(context.Context, *SpaceStore) error) error {
	if s == nil || s.Pool == nil || s.tx != nil {
		return ErrTreeViewerChanged
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var version int64
	if err := s.Pool.QueryRow(ctx, `SELECT version FROM public.schema_migrations WHERE NOT dirty`).Scan(&version); err != nil || !qualifiedTreeViewerVersion(version) {
		return ErrTreeViewerChanged
	}
	catalog := spaceAuthorityCatalog
	catalog.Version = version
	tx, err := authoritysource.BeginRead(ctx, s.Pool, catalog)
	if err != nil {
		return errors.Join(ErrTreeViewerChanged, err)
	}
	defer authoritysource.RollbackRead(ctx, tx)
	scoped := *s
	scoped.tx = tx
	if err := checkOwnershipJournalAvailable(ctx, tx, []uuid.UUID{spaceID}); err != nil {
		return err
	}
	if err := checkLifecycleAvailable(ctx, tx, []uuid.UUID{spaceID}); err != nil {
		return err
	}
	if err := action(ctx, &scoped); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
