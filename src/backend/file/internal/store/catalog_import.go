package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/pkg/principal"
)

var ErrCatalogConflict = errors.New("catalog operation selection conflicts")
var ErrCatalogTerminal = errors.New("catalog operation is no longer available")
var ErrCatalogActorClosed = errors.New("catalog actor generation is closed")

func sha256Bytes(value []byte) []byte { sum := sha256.Sum256(value); return sum[:] }

type CatalogActorCapture struct{ AccountGate, ProfileGate, AccountGeneration, ProfileGeneration uuid.UUID }
type CatalogImport struct {
	FileID, SourceID, OperationID, Generation    uuid.UUID
	Actor                                        catalogasset.ActorPair
	Provider, ProviderID, Phase, Status, R2Key   string
	Destination                                  *chatv1.ChatRef
	ChatAccountGeneration, ChatProfileGeneration uuid.UUID
	KeyVersion                                   string
	OperationHMAC, RequestHash, SelectionHash    []byte
	MaxFetchBytes                                int64
}

// CaptureCatalogActor is called only after a fresh owning-authority read.
// It creates first-use gates, but a CLOSED gate is never reopened by a read or
// by Save. Restore requires a separate, fresh generation transition.
func (s *FilesStore) CaptureCatalogActor(ctx context.Context, actor catalogasset.ActorPair, keys *catalogasset.ReplayKeys) (CatalogActorCapture, error) {
	var result CatalogActorCapture
	account, e := keys.Account(actor.AccountID)
	if e != nil {
		return result, e
	}
	profile, e := keys.Profile(actor)
	if e != nil {
		return result, e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return result, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if e = catalogActorLock(ctx, tx, actor, keys); e != nil {
		return result, e
	}
	for i, kind := range []string{"account", "profile"} {
		digests := account
		if i == 1 {
			digests = profile
		}
		var selected, gen uuid.UUID
		for _, d := range digests {
			var id, g uuid.UUID
			var state string
			e = tx.QueryRow(ctx, `SELECT id,generation,state FROM file_catalog_actor_gates WHERE actor_kind=$1 AND key_version=$2 AND actor_hmac=$3 FOR UPDATE`, kind, d.KeyVersion, d.HMAC).Scan(&id, &g, &state)
			if errors.Is(e, pgx.ErrNoRows) {
				continue
			}
			if e != nil {
				return result, e
			}
			if state != "LIVE" {
				return result, ErrCatalogActorClosed
			}
			if selected == uuid.Nil {
				selected, gen = id, g
			}
		}
		if selected == uuid.Nil {
			selected, gen = uuid.New(), uuid.New()
			d := digests[0]
			if _, e = tx.Exec(ctx, `INSERT INTO file_catalog_actor_gates(id,actor_kind,key_version,actor_hmac,generation,state) VALUES($1,$2,$3,$4,$5,'LIVE')`, selected, kind, d.KeyVersion, d.HMAC, gen); e != nil {
				return result, e
			}
		}
		if i == 0 {
			result.AccountGate, result.AccountGeneration = selected, gen
		} else {
			result.ProfileGate, result.ProfileGeneration = selected, gen
		}
	}
	return result, tx.Commit(ctx)
}

func checkCatalogCapture(ctx context.Context, tx pgx.Tx, c CatalogActorCapture) error {
	if c.AccountGate == uuid.Nil || c.ProfileGate == uuid.Nil || c.AccountGeneration == uuid.Nil || c.ProfileGeneration == uuid.Nil {
		return ErrCatalogActorClosed
	}
	var live bool
	err := tx.QueryRow(ctx, `SELECT a.state='LIVE' AND p.state='LIVE' AND a.generation=$3 AND p.generation=$4 AND a.actor_kind='account' AND p.actor_kind='profile'
FROM file_catalog_actor_gates a CROSS JOIN file_catalog_actor_gates p WHERE a.id=$1 AND p.id=$2 FOR SHARE OF a,p`, c.AccountGate, c.ProfileGate, c.AccountGeneration, c.ProfileGeneration).Scan(&live)
	if err != nil {
		return err
	}
	if !live {
		return ErrCatalogActorClosed
	}
	return nil
}

func catalogImportHashes(r *filev1.ImportCatalogAssetRequest) ([]byte, []byte, error) {
	internal, e := principal.RequestHash(r)
	if e != nil {
		return nil, nil, e
	}
	// Deliberately excludes the transient URL, resolver source UUID and captured
	// generations so Chat can recover a committed File before resolving again.
	selection, e := principal.RequestHash(&filev1.ImportCatalogAssetRequest{ProtocolVersion: 1, OperationId: r.OperationId, Provider: r.Provider, ProviderId: r.ProviderId, ActorAccountId: r.ActorAccountId, ActorProfileId: r.ActorProfileId, DestinationChat: r.DestinationChat})
	if e != nil {
		return nil, nil, e
	}
	a, e := hex.DecodeString(strings.TrimPrefix(internal, "sha256:"))
	if e != nil {
		return nil, nil, e
	}
	b, e := hex.DecodeString(strings.TrimPrefix(selection, "sha256:"))
	return a, b, e
}

func (s *FilesStore) BeginCatalogImport(ctx context.Context, r *filev1.ImportCatalogAssetRequest, capture CatalogActorCapture, keys *catalogasset.ReplayKeys, observed chatv1.ChatType, expires *time.Time, budget int64) (CatalogImport, bool, error) {
	var result CatalogImport
	if r == nil || r.ProtocolVersion != 1 || r.DestinationChat == nil || budget <= 0 || budget > 50<<20 || r.SourceUrl == "" || r.Provider == "" || r.ProviderId == "" || r.Provider != strings.TrimSpace(r.Provider) || r.ProviderId != strings.TrimSpace(r.ProviderId) || len(r.ProtoReflect().GetUnknown()) != 0 || len(r.DestinationChat.ProtoReflect().GetUnknown()) != 0 {
		return result, false, catalogasset.ErrAdmission
	}
	ids := []string{r.ActorAccountId, r.ActorProfileId, r.OperationId, r.SourceId, r.DestinationChat.Id, r.ChatAccountGeneration, r.ChatProfileGeneration}
	parsed := make([]uuid.UUID, len(ids))
	for i, value := range ids {
		id, e := uuid.Parse(value)
		if e != nil || id == uuid.Nil || id.String() != value {
			return result, false, catalogasset.ErrAdmission
		}
		parsed[i] = id
	}
	typeName := ""
	switch observed {
	case chatv1.ChatType_CHAT_TYPE_DM:
		typeName = "dm"
	case chatv1.ChatType_CHAT_TYPE_GROUP:
		typeName = "group"
	case chatv1.ChatType_CHAT_TYPE_CHANNEL:
		typeName = "channel"
	default:
		return result, false, catalogasset.ErrAdmission
	}
	if r.DestinationChat.GetType() != 0 && r.DestinationChat.GetType() != observed {
		return result, false, catalogasset.ErrAdmission
	}
	actor := catalogasset.ActorPair{AccountID: parsed[0], ProfileID: parsed[1]}
	digests, e := keys.Operation(actor, parsed[2])
	if e != nil {
		return result, false, e
	}
	requestHash, selectionHash, e := catalogImportHashes(r)
	if e != nil {
		return result, false, e
	}
	tx, e := s.Pool.Begin(ctx)
	if e != nil {
		return result, false, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if e = catalogActorLock(ctx, tx, actor, keys); e != nil {
		return result, false, e
	}
	if e = checkCatalogCapture(ctx, tx, capture); e != nil {
		return result, false, e
	}
	if e = checkCatalogCaptureActor(ctx, tx, capture, actor, keys); e != nil {
		return result, false, e
	}
	for _, d := range digests {
		var terminal bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_terminal_fences WHERE purpose='operation' AND key_version=$1 AND identity_hmac=$2)`, d.KeyVersion, d.HMAC).Scan(&terminal); e != nil {
			return result, false, e
		}
		if terminal {
			return result, false, ErrCatalogTerminal
		}
		var storedHash []byte
		var expired bool
		e = tx.QueryRow(ctx, `SELECT o.file_id,o.source_id,o.generation,o.request_sha256,o.phase,COALESCE(o.retain_until<=clock_timestamp(),false),COALESCE(f.status,o.outcome,'pending_upload')
FROM file_catalog_import_operations o LEFT JOIN files f ON f.id=o.file_id WHERE o.key_version=$1 AND o.operation_hmac=$2`, d.KeyVersion, d.HMAC).Scan(&result.FileID, &result.SourceID, &result.Generation, &storedHash, &result.Phase, &expired, &result.Status)
		if errors.Is(e, pgx.ErrNoRows) {
			continue
		}
		if e != nil {
			return result, false, e
		}
		if expired {
			if e = compactCatalogOperation(ctx, tx, d); e != nil {
				return result, false, e
			}
			if e = tx.Commit(ctx); e != nil {
				return result, false, e
			}
			return CatalogImport{}, false, ErrCatalogTerminal
		}
		if !bytes.Equal(storedHash, requestHash) {
			return result, false, ErrCatalogConflict
		}
		if e = checkCatalogCapture(ctx, tx, capture); e != nil {
			return result, false, e
		}
		// Full immutable data will be loaded by the status reader. An existing
		// reservation is never evidence of READY and never owns a second fetch.
		result.OperationID = parsed[2]
		result.Actor = actor
		result.Provider = r.Provider
		result.ProviderID = r.ProviderId
		result.Destination = r.DestinationChat
		result.ChatAccountGeneration = parsed[5]
		result.ChatProfileGeneration = parsed[6]
		return result, false, tx.Commit(ctx)
	}
	if e = checkCatalogCapture(ctx, tx, capture); e != nil {
		return result, false, e
	}
	sourceDigests, e := keys.Source(actor, parsed[3])
	if e != nil {
		return result, false, e
	}
	for _, d := range sourceDigests {
		var terminal bool
		if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_terminal_fences WHERE purpose='source' AND key_version=$1 AND identity_hmac=$2)`, d.KeyVersion, d.HMAC).Scan(&terminal); e != nil {
			return result, false, e
		}
		if terminal {
			return result, false, ErrCatalogTerminal
		}
	}
	var sourceExists bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_assets WHERE actor_profile_id=$1 AND source_id=$2)`, actor.ProfileID, parsed[3]).Scan(&sourceExists); e != nil {
		return result, false, e
	}
	if sourceExists {
		return result, false, ErrCatalogConflict
	}
	result = CatalogImport{FileID: uuid.New(), SourceID: parsed[3], OperationID: parsed[2], Generation: uuid.New(), Actor: actor, Provider: r.Provider, ProviderID: r.ProviderId, Phase: "PENDING", Status: "pending_upload", Destination: r.DestinationChat, ChatAccountGeneration: parsed[5], ChatProfileGeneration: parsed[6], KeyVersion: digests[0].KeyVersion, OperationHMAC: digests[0].HMAC, RequestHash: requestHash, SelectionHash: selectionHash, MaxFetchBytes: budget}
	result.R2Key = "catalog/" + result.SourceID.String() + "/" + result.Generation.String() + "/source.gif"
	_, e = tx.Exec(ctx, `INSERT INTO files(id,uploader_profile_id,original_name,mime_type,size_bytes,r2_key,status,file_type,scan_result,upload_intent,catalog_asset,catalog_source_size_unknown,expires_at)
VALUES($1,$2,'catalog.gif','image/gif',0,$3,'pending_upload','image','skipped','gif',true,true,$4)`, result.FileID, actor.ProfileID, result.R2Key, expires)
	if e != nil {
		return result, false, e
	}
	urlHash := sha256Bytes([]byte(r.SourceUrl))
	_, e = tx.Exec(ctx, `INSERT INTO file_catalog_assets(file_id,source_id,actor_account_id,actor_profile_id,provider,provider_id,account_gate_id,profile_gate_id,account_generation,profile_generation,chat_account_generation,chat_profile_generation,source_url_sha256)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, result.FileID, result.SourceID, actor.AccountID, actor.ProfileID, r.Provider, r.ProviderId, capture.AccountGate, capture.ProfileGate, capture.AccountGeneration, capture.ProfileGeneration, result.ChatAccountGeneration, result.ChatProfileGeneration, urlHash)
	if e != nil {
		return result, false, e
	}
	var requested *int32
	if r.DestinationChat.Type != nil {
		v := int32(r.DestinationChat.GetType())
		requested = &v
	}
	_, e = tx.Exec(ctx, `INSERT INTO file_catalog_import_operations(key_version,operation_hmac,generation,phase,operation_id,file_id,source_id,provider,provider_id,chat_account_generation,chat_profile_generation,actor_account_id,actor_profile_id,destination_chat_id,destination_chat_type,requested_chat_type,request_sha256,selection_sha256,max_fetch_bytes)
VALUES($1,$2,$3,'PENDING',$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`, result.KeyVersion, result.OperationHMAC, result.Generation, result.OperationID, result.FileID, result.SourceID, r.Provider, r.ProviderId, result.ChatAccountGeneration, result.ChatProfileGeneration, actor.AccountID, actor.ProfileID, parsed[4], typeName, requested, requestHash, selectionHash, budget)
	if e != nil {
		return result, false, e
	}
	return result, true, tx.Commit(ctx)
}

func checkCatalogCaptureActor(ctx context.Context, tx pgx.Tx, c CatalogActorCapture, actor catalogasset.ActorPair, keys *catalogasset.ReplayKeys) error {
	a, e := keys.Account(actor.AccountID)
	if e != nil {
		return e
	}
	p, e := keys.Profile(actor)
	if e != nil {
		return e
	}
	for _, b := range []struct {
		id      uuid.UUID
		kind    string
		digests []catalogasset.ReplayDigest
	}{{c.AccountGate, "account", a}, {c.ProfileGate, "profile", p}} {
		matched := false
		for _, d := range b.digests {
			var found bool
			if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM file_catalog_actor_gates WHERE id=$1 AND actor_kind=$2 AND key_version=$3 AND actor_hmac=$4)`, b.id, b.kind, d.KeyVersion, d.HMAC).Scan(&found); e != nil {
				return e
			}
			matched = matched || found
		}
		if !matched {
			return ErrCatalogActorClosed
		}
	}
	return nil
}
