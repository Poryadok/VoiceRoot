package grpcsvc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/chat/internal/store"
)

var errDeletedAccountGateUnavailable = errors.New("deleted-account gate unavailable")

// AccountDeletedChecker reports soft-deleted accounts (Auth source of truth).
type AccountDeletedChecker interface {
	DeletedAmong(ctx context.Context, accountIDs []uuid.UUID) (map[uuid.UUID]struct{}, error)
}

func filterDeletedPeerDMRows(
	ctx context.Context,
	rows []*store.ChatRow,
	peers map[uuid.UUID]uuid.UUID,
	profiles UserProfileLookup,
	deleted AccountDeletedChecker,
) ([]*store.ChatRow, error) {
	if len(rows) == 0 {
		return rows, nil
	}

	peerProfiles := make(map[uuid.UUID]struct{})
	for _, row := range rows {
		if row.Type != "dm" {
			continue
		}
		pid, ok := peers[row.ID]
		if !ok || pid == uuid.Nil {
			return nil, errDeletedAccountGateUnavailable
		}
		peerProfiles[pid] = struct{}{}
	}
	if len(peerProfiles) == 0 {
		return rows, nil
	}
	if deleted == nil || profiles == nil {
		return nil, errDeletedAccountGateUnavailable
	}

	profileToAccount := make(map[uuid.UUID]uuid.UUID, len(peerProfiles))
	accountIDs := make([]uuid.UUID, 0, len(peerProfiles))
	for pid := range peerProfiles {
		acc, err := profiles.AccountIDByProfileID(ctx, pid)
		if err != nil {
			return nil, errDeletedAccountGateUnavailable
		}
		if acc == uuid.Nil {
			return nil, errDeletedAccountGateUnavailable
		}
		profileToAccount[pid] = acc
		accountIDs = append(accountIDs, acc)
	}

	deletedSet, err := deleted.DeletedAmong(ctx, accountIDs)
	if err != nil {
		return nil, errDeletedAccountGateUnavailable
	}
	out := make([]*store.ChatRow, 0, len(rows))
	for _, row := range rows {
		if row.Type != "dm" {
			out = append(out, row)
			continue
		}
		pid, ok := peers[row.ID]
		if !ok {
			return nil, errDeletedAccountGateUnavailable
		}
		acc, ok := profileToAccount[pid]
		if !ok {
			return nil, errDeletedAccountGateUnavailable
		}
		if _, isDeleted := deletedSet[acc]; isDeleted {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *ChatGRPC) filterListChatsDeletedPeerDMs(
	ctx context.Context,
	rows []*store.ChatRow,
	peers map[uuid.UUID]uuid.UUID,
) ([]*store.ChatRow, error) {
	if s == nil {
		return nil, errDeletedAccountGateUnavailable
	}
	return filterDeletedPeerDMRows(ctx, rows, peers, s.Profiles, s.DeletedAccounts)
}

func (s *ChatGRPC) requireActiveDMPeer(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	if s == nil || s.Profiles == nil || s.DeletedAccounts == nil {
		return uuid.Nil, status.Error(codes.Unavailable, "dm availability unavailable")
	}
	accountID, err := s.Profiles.AccountIDByProfileID(ctx, profileID)
	if status.Code(err) == codes.NotFound {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	if err != nil || accountID == uuid.Nil {
		return uuid.Nil, status.Error(codes.Unavailable, "dm availability unavailable")
	}
	deleted, err := s.DeletedAccounts.DeletedAmong(ctx, []uuid.UUID{accountID})
	if err != nil {
		return uuid.Nil, status.Error(codes.Unavailable, "dm availability unavailable")
	}
	if _, ok := deleted[accountID]; ok {
		return uuid.Nil, status.Error(codes.PermissionDenied, "dm not available")
	}
	return accountID, nil
}
