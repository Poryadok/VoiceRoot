package store

import (
	"context"
	"errors"

	"github.com/google/uuid"
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
// authority revision under one ordinary Space lifecycle/ownership scope. The
// existing membership read preserves ordinary and leased-community semantics.
func (s *SpaceStore) ReadSpaceTreeForViewer(ctx context.Context, spaceID, profileID uuid.UUID) (*SpaceTreeViewerCut, error) {
	if spaceID == uuid.Nil || profileID == uuid.Nil {
		return nil, ErrTreeViewerDenied
	}
	return withOwnershipScopeValue(s, ctx, []uuid.UUID{spaceID}, func(scoped *SpaceStore) (*SpaceTreeViewerCut, error) {
		member, err := scoped.IsSpaceMember(ctx, spaceID, profileID)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, ErrTreeViewerDenied
		}
		revision, err := spaceSourceRevision(ctx, scoped.tx, spaceID.String())
		if err != nil {
			return nil, errors.Join(ErrTreeViewerChanged, err)
		}
		data, err := scoped.ListSpaceTree(ctx, spaceID)
		if err != nil {
			return nil, err
		}
		cut := &SpaceTreeViewerCut{SpaceID: spaceID, ProfileID: profileID, Revision: revision, Data: data}
		// Direct owner writes also advance the durable source revision. Detect a
		// change between the component reads rather than returning a mixed tree.
		if err := scoped.ValidateSpaceTreeViewer(ctx, cut); err != nil {
			return nil, err
		}
		return cut, nil
	})
}

// ValidateSpaceTreeViewer obtains a fresh current cut; no remote dependency runs
// inside its lock. A removed/rejoined viewer cannot reuse an earlier revision.
func (s *SpaceStore) ValidateSpaceTreeViewer(ctx context.Context, cut *SpaceTreeViewerCut) error {
	if cut == nil || cut.SpaceID == uuid.Nil || cut.ProfileID == uuid.Nil || cut.Revision == 0 {
		return ErrTreeViewerDenied
	}
	return s.withOwnershipScope(ctx, []uuid.UUID{cut.SpaceID}, func(scoped *SpaceStore) error {
		member, err := scoped.IsSpaceMember(ctx, cut.SpaceID, cut.ProfileID)
		if err != nil {
			return err
		}
		if !member {
			return ErrTreeViewerDenied
		}
		revision, err := spaceSourceRevision(ctx, scoped.tx, cut.SpaceID.String())
		if err != nil || revision != cut.Revision {
			return errors.Join(ErrTreeViewerChanged, err)
		}
		return nil
	})
}
