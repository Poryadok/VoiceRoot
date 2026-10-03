package store

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
)

func boundMessagingPurge(ctx context.Context, operation uuid.UUID, hash []byte) bool {
	p, ok := principal.FromContext(ctx)
	return ok && p.Kind == "service" && p.Issuer == "messaging" && p.Subject == "service:messaging" &&
		p.Audience == "search" && p.RPC == searchv1.SearchService_PurgeManagedChatMessages_FullMethodName &&
		p.RequestID == operation.String() && p.RequestHash == "sha256:"+hex.EncodeToString(hash) &&
		p.AccountID == "" && p.ProfileID == "" && p.SessionEpoch == 0
}

// The caller holds the shared lifecycle lock for the entire transaction.
// Only the exact Messaging child of the sealed, irreversible Space decision
// may delete through the ordinary frozen guard. Knowing its UUID is insufficient.
func authorizeManagedChatPurge(ctx context.Context, tx pgx.Tx, operation, chat uuid.UUID, hash []byte) error {
	frozen, err := lifecycleFrozenChat(ctx, tx, chat)
	if err != nil || !frozen {
		return err
	}
	if !boundMessagingPurge(ctx, operation, hash) {
		return ErrManagedChatSearchPurgeConflict
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT f.space_id,f.deletion_operation_id
		FROM search_space_chat_manifest_items i
		JOIN search_space_chat_manifests m USING(space_id,deletion_operation_id)
		JOIN search_space_lifecycle_fences f USING(space_id,deletion_operation_id)
		WHERE i.chat_id=$1 AND f.state='PURGE_DECIDED' AND f.generation>=2
		AND m.sealed AND m.generation=f.generation-1 AND i.generation=m.generation`, chat)
	if err != nil {
		return err
	}
	defer rows.Close()
	var parents [][2]uuid.UUID
	for rows.Next() {
		var parent [2]uuid.UUID
		if err := rows.Scan(&parent[0], &parent[1]); err != nil {
			return err
		}
		parents = append(parents, parent)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(parents) != 1 {
		return ErrManagedChatSearchPurgeConflict
	}
	expected := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", parents[0][0], parents[0][1], chat)))
	if operation != expected {
		return ErrManagedChatSearchPurgeConflict
	}
	return nil
}
