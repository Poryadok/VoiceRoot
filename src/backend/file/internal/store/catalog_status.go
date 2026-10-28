package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
)

// CatalogStatus recovers File-owned operation authority without a source URL.
// Operation receipt compaction never erases active asset status authority.
func (s *FilesStore) CatalogStatus(ctx context.Context, r *filev1.GetCatalogAssetStatusRequest, keys *catalogasset.ReplayKeys, now time.Time) (*filev1.GetCatalogAssetStatusResponse, error) {
	if r == nil || r.ProtocolVersion != 2 || len(r.ProtoReflect().GetUnknown()) != 0 {
		return nil, catalogasset.ErrAdmission
	}
	account, e := uuid.Parse(r.ActorAccountId)
	if e != nil || account == uuid.Nil || account.String() != r.ActorAccountId {
		return nil, catalogasset.ErrAdmission
	}
	profile, e := uuid.Parse(r.ActorProfileId)
	if e != nil || profile == uuid.Nil || profile.String() != r.ActorProfileId {
		return nil, catalogasset.ErrAdmission
	}
	actor := catalogasset.ActorPair{AccountID: account, ProfileID: profile}
	tx, e := BeginInventoryMutation(ctx, s.Pool, true)
	if e != nil {
		return nil, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if e = catalogActorLock(ctx, tx, actor, keys); e != nil {
		return nil, e
	}
	if op := r.GetOperation(); op != nil {
		if len(op.ProtoReflect().GetUnknown()) != 0 || op.DestinationChat == nil || len(op.DestinationChat.ProtoReflect().GetUnknown()) != 0 {
			return nil, catalogasset.ErrAdmission
		}
		destination, e := uuid.Parse(op.DestinationChat.Id)
		if e != nil || destination == uuid.Nil || destination.String() != op.DestinationChat.Id || op.Provider == "" || op.ProviderId == "" || op.Provider != strings.TrimSpace(op.Provider) || op.ProviderId != strings.TrimSpace(op.ProviderId) || op.DestinationChat.GetType() < 0 || op.DestinationChat.GetType() > chatv1.ChatType_CHAT_TYPE_CHANNEL {
			return nil, catalogasset.ErrAdmission
		}
		id, e := uuid.Parse(op.OperationId)
		if e != nil || id == uuid.Nil || id.String() != op.OperationId {
			return nil, catalogasset.ErrAdmission
		}
		digests, e := keys.Operation(actor, id)
		if e != nil {
			return nil, e
		}
		_, selectionHash, e := catalogImportHashes(&filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, OperationId: op.OperationId, Provider: op.Provider, ProviderId: op.ProviderId, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, DestinationChat: op.DestinationChat})
		if e != nil {
			return nil, e
		}
		for _, d := range digests {
			var terminal bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_terminal_fences WHERE purpose='operation' AND key_version=$1 AND identity_hmac=$2)`, d.KeyVersion, d.HMAC).Scan(&terminal); e != nil {
				return nil, e
			}
			if terminal {
				return nil, ErrCatalogTerminal
			}
			out := &filev1.GetCatalogAssetStatusResponse{OperationId: id.String(), ActorAccountId: account.String(), ActorProfileId: profile.String()}
			var source, file, chat, chatAccount, chatProfile uuid.UUID
			var storedHash []byte
			var requested *int32
			var status, outcome, blobState *string
			var updated, retain *time.Time
			var deleting *bool
			var protocol int
			var head *uuid.UUID
			var revision *int64
			e = tx.QueryRow(ctx, `SELECT o.file_id,o.source_id,o.provider,o.provider_id,o.chat_account_generation,o.chat_profile_generation,o.destination_chat_id,o.requested_chat_type,o.selection_sha256,o.outcome,o.retain_until,f.status,f.updated_at,b.state,b.deleted_at IS NOT NULL,o.authority_protocol,o.selection_head_id,o.selection_revision
FROM file_catalog_import_operations o LEFT JOIN files f ON f.id=o.file_id LEFT JOIN file_blobs b ON b.blob_id=f.blob_id
WHERE o.key_version=$1 AND o.operation_hmac=$2 FOR UPDATE OF o`, d.KeyVersion, d.HMAC).Scan(&file, &source, &out.Provider, &out.ProviderId, &chatAccount, &chatProfile, &chat, &requested, &storedHash, &outcome, &retain, &status, &updated, &blobState, &deleting, &protocol, &head, &revision)
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			if e != nil {
				return nil, e
			}
			// Expired details are erased before comparing a changed body. After
			// compaction only terminal/not-available is promised for every replay.
			if retain != nil && !now.Before(*retain) {
				if e = compactCatalogOperation(ctx, tx, d); e != nil {
					return nil, e
				}
				if e = tx.Commit(ctx); e != nil {
					return nil, e
				}
				return nil, ErrCatalogTerminal
			}
			if !bytes.Equal(storedHash, selectionHash) {
				return nil, ErrCatalogConflict
			}
			if protocol != 2 || head == nil || revision == nil {
				return nil, ErrCatalogTerminal
			}
			current, _, e := readCatalogSelectionTx(ctx, tx, *head, *revision, source, file)
			if e != nil {
				return nil, e
			}
			out.SelectionRevision = uint64(*revision)
			out.CurrentSelectionRevision = uint64(current)
			out.FileId = file.String()
			out.SourceId = source.String()
			out.ChatAccountGeneration = chatAccount.String()
			out.ChatProfileGeneration = chatProfile.String()
			out.DestinationChat = &chatv1.ChatRef{Id: chat.String()}
			if requested != nil {
				v := chatv1.ChatType(*requested)
				out.DestinationChat.Type = &v
			}
			if status != nil {
				out.Status = *status
			} else if outcome != nil {
				out.Status = *outcome
			} else {
				return nil, ErrCatalogTerminal
			}
			if out.Status == "ready" && (blobState == nil || *blobState != "LIVE" || deleting == nil || *deleting) {
				return nil, catalogasset.ErrAdmission
			}
			if outcome == nil && terminalCatalogStatus(out.Status) {
				return nil, catalogasset.ErrAdmission
			}
			if e = tx.Commit(ctx); e != nil {
				return nil, e
			}
			return out, nil
		}
		return nil, pgx.ErrNoRows // the sole fresh-operation path
	}
	asset := r.GetAsset()
	if asset == nil || len(asset.ProtoReflect().GetUnknown()) != 0 {
		return nil, catalogasset.ErrAdmission
	}
	source, e := uuid.Parse(asset.SourceId)
	if e != nil || source == uuid.Nil || source.String() != asset.SourceId {
		return nil, catalogasset.ErrAdmission
	}
	file, e := uuid.Parse(asset.FileId)
	if e != nil || file == uuid.Nil || file.String() != asset.FileId {
		return nil, catalogasset.ErrAdmission
	}
	digests, e := keys.Asset(actor, source, asset.Provider, asset.ProviderId, file)
	if e != nil {
		return nil, e
	}
	out := &filev1.GetCatalogAssetStatusResponse{FileId: file.String(), SourceId: source.String(), ActorAccountId: account.String(), ActorProfileId: profile.String(), Provider: asset.Provider, ProviderId: asset.ProviderId}
	for _, d := range digests {
		var fence uuid.UUID
		var state string
		e = tx.QueryRow(ctx, `SELECT fence_id,outcome FROM file_catalog_terminal_fences WHERE purpose='asset' AND key_version=$1 AND identity_hmac=$2`, d.KeyVersion, d.HMAC).Scan(&fence, &state)
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return nil, e
		}
		if state != "failed" && state != "deleted" && state != "expired" {
			return nil, ErrCatalogTerminal
		}
		out.Status = state
		out.TerminalFenceId = fence.String()
		return out, tx.Commit(ctx)
	}
	var ag, pg uuid.UUID
	var blobState string
	var deleting bool
	var live bool
	var protocol int
	var head *uuid.UUID
	var revision *int64
	e = tx.QueryRow(ctx, `SELECT f.status,a.chat_account_generation,a.chat_profile_generation,b.state,b.deleted_at IS NOT NULL,
ag.state='LIVE' AND pg.state='LIVE' AND ag.generation=a.account_generation AND pg.generation=a.profile_generation,a.authority_protocol,a.selection_head_id,a.selection_revision
FROM file_catalog_assets a JOIN files f ON f.id=a.file_id JOIN file_blobs b ON b.blob_id=f.blob_id
JOIN file_catalog_actor_gates ag ON ag.id=a.account_gate_id JOIN file_catalog_actor_gates pg ON pg.id=a.profile_gate_id
WHERE a.file_id=$1 AND a.source_id=$2 AND a.actor_account_id=$3 AND a.actor_profile_id=$4 AND a.provider=$5 AND a.provider_id=$6
FOR SHARE OF a,f,b,ag,pg`, file, source, account, profile, asset.Provider, asset.ProviderId).Scan(&out.Status, &ag, &pg, &blobState, &deleting, &live, &protocol, &head, &revision)
	if e != nil {
		return nil, e
	}
	if !live {
		return nil, ErrCatalogActorClosed
	}
	if protocol != 2 || head == nil || revision == nil {
		return nil, ErrCatalogTerminal
	}
	current, _, e := readCatalogSelectionTx(ctx, tx, *head, *revision, source, file)
	if e != nil {
		return nil, e
	}
	out.SelectionRevision = uint64(*revision)
	out.CurrentSelectionRevision = uint64(current)
	if out.Status == "ready" && (blobState != "LIVE" || deleting) {
		return nil, catalogasset.ErrAdmission
	}
	out.ChatAccountGeneration = ag.String()
	out.ChatProfileGeneration = pg.String()
	return out, tx.Commit(ctx)
}

func terminalCatalogStatus(state string) bool {
	return state == "ready" || state == "failed" || state == "deleted" || state == "expired"
}
func compactCatalogOperation(ctx context.Context, tx pgx.Tx, d catalogasset.ReplayDigest) error {
	_, e := tx.Exec(ctx, `INSERT INTO file_catalog_terminal_fences(purpose,key_version,identity_hmac,outcome,fence_id) VALUES('operation',$1,$2,'not_available',$3) ON CONFLICT(purpose,key_version,identity_hmac) DO NOTHING`, d.KeyVersion, d.HMAC, uuid.New())
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `DELETE FROM file_catalog_import_operations WHERE key_version=$1 AND operation_hmac=$2`, d.KeyVersion, d.HMAC)
	return e
}
