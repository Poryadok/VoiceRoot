package grpcsvc

import (
	"context"
	"log"

	"github.com/google/uuid"

	"voice/backend/chat/internal/store"
)

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
	if deleted == nil || profiles == nil || len(rows) == 0 {
		return rows, nil
	}

	peerProfiles := make(map[uuid.UUID]struct{})
	for _, row := range rows {
		if row.Type != "dm" {
			continue
		}
		if pid, ok := peers[row.ID]; ok {
			peerProfiles[pid] = struct{}{}
		}
	}
	if len(peerProfiles) == 0 {
		return rows, nil
	}

	profileToAccount := make(map[uuid.UUID]uuid.UUID, len(peerProfiles))
	accountIDs := make([]uuid.UUID, 0, len(peerProfiles))
	for pid := range peerProfiles {
		acc, err := profiles.AccountIDByProfileID(ctx, pid)
		if err != nil {
			continue
		}
		profileToAccount[pid] = acc
		accountIDs = append(accountIDs, acc)
	}
	if len(accountIDs) == 0 {
		return rows, nil
	}

	deletedSet, err := deleted.DeletedAmong(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	if len(deletedSet) == 0 {
		return rows, nil
	}

	out := make([]*store.ChatRow, 0, len(rows))
	for _, row := range rows {
		if row.Type != "dm" {
			out = append(out, row)
			continue
		}
		pid, ok := peers[row.ID]
		if !ok {
			out = append(out, row)
			continue
		}
		acc, ok := profileToAccount[pid]
		if !ok {
			out = append(out, row)
			continue
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
) []*store.ChatRow {
	if s == nil || s.DeletedAccounts == nil {
		return rows
	}
	filtered, err := filterDeletedPeerDMRows(ctx, rows, peers, s.Profiles, s.DeletedAccounts)
	if err != nil {
		log.Printf("chat: ListChats deleted-account filter skipped: %v", err)
		return rows
	}
	return filtered
}
