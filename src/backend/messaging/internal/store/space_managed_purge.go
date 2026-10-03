package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/pkg/principal"
)

type spacePurgeParentKey struct{}
type spacePurgeParent struct {
	space, operation uuid.UUID
	generation       uint64
}

func SpacePurgeScope(ctx context.Context) (uuid.UUID, bool) {
	parent, ok := ctx.Value(spacePurgeParentKey{}).(spacePurgeParent)
	return parent.space, ok
}

func SpacePurgeContext(ctx context.Context, request *messagingv1.PurgeSpaceRequest) (context.Context, error) {
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(request)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "messaging" || p.RPC != messagingv1.MessagingService_PurgeSpace_FullMethodName || p.RequestHash != hash || p.RequestID == "" || request.GetPurge() == nil {
		return nil, ErrSpacePurgeReceiptBinding
	}
	space, err := uuid.Parse(request.Purge.SpaceId)
	if err != nil || space == uuid.Nil {
		return nil, ErrSpacePurgeReceiptBinding
	}
	operation, err := uuid.Parse(request.Purge.DeletionOperationId)
	if err != nil || operation == uuid.Nil || request.Purge.Generation < 2 {
		return nil, ErrSpacePurgeReceiptBinding
	}
	return context.WithValue(ctx, spacePurgeParentKey{}, spacePurgeParent{space: space, operation: operation, generation: request.Purge.Generation}), nil
}

// Only the verified Space parent operation can admit deletion through a P3
// fence. The child operation ID alone is public data and is not authorization.
func authorizeSpaceManagedPurge(ctx context.Context, tx pgx.Tx, chat, child uuid.UUID) error {
	parent, ok := ctx.Value(spacePurgeParentKey{}).(spacePurgeParent)
	if !ok {
		return nil
	} // Ordinary purges remain subject to the DB gate.
	rows, err := tx.Query(ctx, `SELECT DISTINCT f.space_id,f.deletion_operation_id FROM messaging_space_lifecycle_fences f JOIN messaging_space_chat_manifest_items i ON i.space_id=f.space_id AND i.deletion_operation_id=f.deletion_operation_id AND i.schedule_generation=f.source_schedule_generation WHERE i.chat_id=$1 AND f.state='PURGE_DECIDED' AND f.generation=$2`, chat, parent.generation)
	if err != nil {
		return err
	}
	var spaces, operations []uuid.UUID
	for rows.Next() {
		var space, operation uuid.UUID
		if err = rows.Scan(&space, &operation); err != nil {
			rows.Close()
			return err
		}
		spaces = append(spaces, space)
		operations = append(operations, operation)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(spaces) != 1 || parent.operation != operations[0] || parent.space != spaces[0] {
		return ErrSpacePurgeReceiptBinding
	}
	expected := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("voice.messaging.v1.SpaceChatPurge\x00%s\x00%s\x00%s", spaces[0], operations[0], chat)))
	if expected != child {
		return ErrSpacePurgeReceiptBinding
	}
	_, err = tx.Exec(ctx, `SELECT set_config('voice.messaging_purge_space_id',$1,true),set_config('voice.messaging_purge_operation_id',$2,true)`, spaces[0].String(), operations[0].String())
	return err
}
