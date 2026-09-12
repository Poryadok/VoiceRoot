package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	commonv1 "voice.app/voice/common/v1"
	searchv1 "voice.app/voice/search/v1"
	"voice/backend/pkg/principal"
	"voice/backend/search/internal/store"
)

const searchLifecycleLock int64 = 0x5345415243485232

func (s *SearchGRPC) lifecyclePool() *pgxpool.Pool {
	if a, ok := s.Messages.(*MessageStoreAdapter); ok && a != nil && a.MessageSearchStore != nil {
		return a.Pool
	}
	if a, ok := s.Spaces.(*SpaceStoreAdapter); ok && a != nil && a.ProfileSpaceSearchStore != nil {
		return a.Pool
	}
	return nil
}

func trustedSearchLifecycle(ctx context.Context, message proto.Message, rpc string) error {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Space principal required")
	}
	h, err := principal.RequestHash(message)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	if p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "search" || p.RPC != rpc || p.RequestHash != h || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.PermissionDenied, "lifecycle caller is not trusted Space")
	}
	return nil
}

func canonicalUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errors.New("uuid must be canonical and non-zero")
	}
	return id, nil
}
func lifecycleIDs(space, operation string) (uuid.UUID, uuid.UUID, error) {
	s, err := canonicalUUID(space)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	o, err := canonicalUUID(operation)
	return s, o, err
}

func hasUnknown(message proto.Message) bool {
	if message == nil {
		return false
	}
	m := message.ProtoReflect()
	if len(m.GetUnknown()) != 0 {
		return true
	}
	found := false
	m.Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if fd.IsList() && fd.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if hasUnknown(list.Get(i).Message().Interface()) {
					found = true
					return false
				}
			}
		} else if fd.IsMap() && fd.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, v protoreflect.Value) bool {
				if hasUnknown(v.Message().Interface()) {
					found = true
					return false
				}
				return true
			})
		} else if fd.Message() != nil && hasUnknown(value.Message().Interface()) {
			found = true
		}
		return !found
	})
	return found
}

func deterministicBytes(message proto.Message) ([]byte, error) {
	if message == nil || hasUnknown(message) {
		return nil, errors.New("unknown lifecycle fields")
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(message)
}
func domainSeparatedSHA(fqn string, wire []byte) []byte {
	input := make([]byte, 0, len(fqn)+1+len(wire))
	input = append(input, fqn...)
	input = append(input, 0)
	input = append(input, wire...)
	sum := sha256.Sum256(input)
	return sum[:]
}
func lifecycleDigest(message proto.Message, wire []byte) []byte {
	return domainSeparatedSHA(string(message.ProtoReflect().Descriptor().FullName()), wire)
}

func validateManifest(binding *commonv1.ManifestBinding) error {
	if binding == nil || binding.GetManifestId() == "" || len(binding.GetManifestSha256()) != sha256.Size || hasUnknown(binding) {
		return errors.New("invalid manifest binding")
	}
	return nil
}
func validateFence(req *searchv1.ApplySpaceLifecycleFenceRequest) (uuid.UUID, uuid.UUID, error) {
	if req == nil || hasUnknown(req) {
		return uuid.Nil, uuid.Nil, errors.New("invalid lifecycle fence")
	}
	f := req.GetFence()
	if f == nil || f.GetProtocolVersion() != 1 || f.GetGeneration() == 0 || validateManifest(f.GetManifest()) != nil {
		return uuid.Nil, uuid.Nil, errors.New("invalid lifecycle fence")
	}
	if f.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE && f.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN && f.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
		return uuid.Nil, uuid.Nil, errors.New("unsupported lifecycle state")
	}
	return lifecycleIDs(f.GetSpaceId(), f.GetDeletionOperationId())
}

func stateName(state commonv1.LifecycleFenceState) string {
	switch state {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		return "FROZEN"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		return "LIVE"
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		return "PURGE_DECIDED"
	}
	return ""
}

func beginLifecycleTx(ctx context.Context, pool *pgxpool.Pool, spaceID uuid.UUID) (pgx.Tx, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, searchLifecycleLock); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	if err = store.AcquireLifecycleSpaceLock(ctx, tx, spaceID); err != nil {
		_ = tx.Rollback(ctx)
		return nil, err
	}
	var ready bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass('search_space_lifecycle_fences') IS NOT NULL AND to_regclass('search_space_lifecycle_operations') IS NOT NULL AND to_regclass('search_space_lifecycle_receipts') IS NOT NULL AND to_regclass('search_space_purge_receipts') IS NOT NULL AND to_regclass('search_space_chat_manifests') IS NOT NULL AND to_regclass('search_space_chat_manifest_pages') IS NOT NULL AND to_regclass('search_space_chat_manifest_items') IS NOT NULL AND to_regclass('search_space_purged_chat_fences') IS NOT NULL`).Scan(&ready); err != nil || !ready {
		_ = tx.Rollback(ctx)
		if err != nil {
			return nil, err
		}
		return nil, errors.New("search lifecycle schema unavailable")
	}
	return tx, nil
}

func (s *SearchGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *searchv1.ApplySpaceLifecycleFenceRequest) (_ *searchv1.ApplySpaceLifecycleFenceResponse, err error) {
	if err = trustedSearchLifecycle(ctx, req, searchv1.SearchService_ApplySpaceLifecycleFence_FullMethodName); err != nil {
		return nil, err
	}
	spaceID, opID, validationErr := validateFence(req)
	if validationErr != nil {
		return nil, status.Error(codes.InvalidArgument, validationErr.Error())
	}
	pool := s.lifecyclePool()
	if pool == nil {
		return nil, status.Error(codes.Unavailable, "lifecycle store unavailable")
	}
	wire, err := deterministicBytes(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	requestHash := lifecycleDigest(req, wire)
	var imported *importedChatManifest
	if req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		probe, probeErr := beginLifecycleTx(ctx, pool, spaceID)
		if probeErr != nil {
			return nil, status.Error(codes.Unavailable, probeErr.Error())
		}
		replay, done, inspectErr := inspectFenceTx(ctx, probe, spaceID, opID, req, wire)
		if inspectErr != nil {
			_ = probe.Rollback(context.Background())
			return nil, inspectErr
		}
		if done {
			if probeErr = probe.Commit(ctx); probeErr != nil {
				return nil, status.Error(codes.Unavailable, probeErr.Error())
			}
			return replay, nil
		}
		if probeErr = probe.Rollback(ctx); probeErr != nil {
			return nil, status.Error(codes.Unavailable, probeErr.Error())
		}
		imported, err = s.importChatManifest(ctx, spaceID, opID, req.GetFence().GetGeneration(), req.GetFence().GetManifest())
		if err != nil {
			return nil, status.Error(codes.Unavailable, "complete Chat manifest unavailable")
		}
	}
	tx, err := beginLifecycleTx(ctx, pool, spaceID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		} else {
			err = tx.Commit(ctx)
		}
	}()
	if replay, done, inspectErr := inspectFenceTx(ctx, tx, spaceID, opID, req, wire); inspectErr != nil {
		return nil, inspectErr
	} else if done {
		return replay, nil
	}
	if req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
		if e := validatePersistedManifestEvidence(ctx, tx, spaceID, opID, req.GetFence().GetManifest()); e != nil {
			return nil, status.Error(codes.FailedPrecondition, "complete Chat manifest evidence invalid")
		}
	}
	appliedAt := time.Time{}
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&appliedAt); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if imported != nil {
		if err = persistImportedManifest(ctx, tx, spaceID, opID, req.GetFence().GetGeneration(), imported, appliedAt); err != nil {
			return nil, status.Error(codes.FailedPrecondition, err.Error())
		}
	}
	receipt := &commonv1.SpaceLifecycleFenceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: opID.String(), Generation: req.GetFence().GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_SEARCH, AppliedState: req.GetFence().GetDesiredState(), RequestSha256: requestHash, ManifestSha256: bytes.Clone(req.GetFence().GetManifest().GetManifestSha256()), AppliedAt: timestamppb.New(appliedAt)}
	out := &searchv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}
	receiptBytes, e := proto.MarshalOptions{Deterministic: true}.Marshal(out)
	if e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	state := stateName(req.GetFence().GetDesiredState())
	if _, err = tx.Exec(ctx, `INSERT INTO search_space_lifecycle_fences(space_id,generation,state,deletion_operation_id,updated_at) VALUES($1,$2,$3,$4,$5) ON CONFLICT(space_id) DO UPDATE SET generation=EXCLUDED.generation,state=EXCLUDED.state,deletion_operation_id=EXCLUDED.deletion_operation_id,updated_at=EXCLUDED.updated_at`, spaceID, req.GetFence().GetGeneration(), state, opID, appliedAt); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if _, err = tx.Exec(ctx, `INSERT INTO search_space_lifecycle_operations(space_id,deletion_operation_id,generation,request_bytes,request_sha256,state,created_at,retain_until) VALUES($1,$2,$3,$4,$5,$6,$7,$7::timestamptz+interval '30 days')`, spaceID, opID, req.GetFence().GetGeneration(), wire, requestHash, state, appliedAt); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if _, err = tx.Exec(ctx, `INSERT INTO search_space_lifecycle_receipts(space_id,deletion_operation_id,generation,receipt_bytes,request_sha256,applied_at,retain_until) VALUES($1,$2,$3,$4,$5,$6,$6::timestamptz+interval '30 days')`, spaceID, opID, req.GetFence().GetGeneration(), receiptBytes, requestHash, appliedAt); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return out, nil
}

func inspectFenceTx(ctx context.Context, tx pgx.Tx, spaceID, opID uuid.UUID, req *searchv1.ApplySpaceLifecycleFenceRequest, wire []byte) (*searchv1.ApplySpaceLifecycleFenceResponse, bool, error) {
	var generation uint64
	var currentState string
	var currentOp uuid.UUID
	scanErr := tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id FROM search_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&generation, &currentState, &currentOp)
	if scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
		return nil, false, status.Error(codes.Internal, scanErr.Error())
	}
	if scanErr == nil {
		if req.GetFence().GetGeneration() < generation {
			receipt, err := loadCurrentFenceReceipt(ctx, tx, spaceID, generation)
			return receipt, true, err
		}
		if req.GetFence().GetGeneration() == generation {
			receipt, err := replayFenceTx(ctx, tx, spaceID, opID, generation, wire)
			return receipt, true, err
		}
		if currentState == "PURGE_DECIDED" || currentState == "PURGED" {
			return nil, false, status.Error(codes.FailedPrecondition, "space purge is irreversible")
		}
		if req.GetFence().GetGeneration() != generation+1 {
			return nil, false, status.Error(codes.Unavailable, "lifecycle generation requires Space reconciliation")
		}
		if currentState == "FROZEN" && (req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE || req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED) && currentOp != opID {
			return nil, false, status.Error(codes.AlreadyExists, "lifecycle operation conflict")
		}
		if currentState == "LIVE" && req.GetFence().GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
			return nil, false, status.Error(codes.FailedPrecondition, "invalid lifecycle transition")
		}
		if currentState == "FROZEN" && req.GetFence().GetDesiredState() == commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
			return nil, false, status.Error(codes.FailedPrecondition, "invalid lifecycle transition")
		}
	} else if req.GetFence().GetGeneration() != 1 || req.GetFence().GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		return nil, false, status.Error(codes.Unavailable, "initial lifecycle generation requires Space reconciliation")
	}
	return nil, false, nil
}
func replayFenceTx(ctx context.Context, tx pgx.Tx, spaceID, opID uuid.UUID, generation uint64, wire []byte) (*searchv1.ApplySpaceLifecycleFenceResponse, error) {
	var stored, receipt []byte
	e := tx.QueryRow(ctx, `SELECT o.request_bytes,r.receipt_bytes FROM search_space_lifecycle_operations o JOIN search_space_lifecycle_receipts r USING(space_id,deletion_operation_id,generation) WHERE o.space_id=$1 AND o.deletion_operation_id=$2 AND o.generation=$3`, spaceID, opID, generation).Scan(&stored, &receipt)
	if e != nil || !bytes.Equal(stored, wire) {
		return nil, status.Error(codes.AlreadyExists, "lifecycle operation conflict")
	}
	out := &searchv1.ApplySpaceLifecycleFenceResponse{}
	if e = proto.Unmarshal(receipt, out); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	return out, nil
}
func loadCurrentFenceReceipt(ctx context.Context, tx pgx.Tx, spaceID uuid.UUID, generation uint64) (*searchv1.ApplySpaceLifecycleFenceResponse, error) {
	var b []byte
	if err := tx.QueryRow(ctx, `SELECT receipt_bytes FROM search_space_lifecycle_receipts WHERE space_id=$1 AND generation=$2 ORDER BY applied_at DESC LIMIT 1`, spaceID, generation).Scan(&b); err != nil {
		return nil, status.Error(codes.Unavailable, "current lifecycle receipt unavailable")
	}
	out := &searchv1.ApplySpaceLifecycleFenceResponse{}
	if err := proto.Unmarshal(b, out); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return out, nil
}

func persistImportedManifest(ctx context.Context, tx pgx.Tx, spaceID, opID uuid.UUID, generation uint64, m *importedChatManifest, at time.Time) error {
	if m == nil || m.Binding == nil || len(m.ChatIDs) == 0 {
		return errors.New("chat manifest is empty or incomplete")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO search_space_chat_manifests(space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,item_count,page_count,sealed,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,true,$8)`, spaceID, opID, generation, m.Binding.GetManifestId(), m.Binding.GetManifestSha256(), m.Binding.GetItemCount(), len(m.Pages), at); err != nil {
		return err
	}
	for _, page := range m.Pages {
		pageBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(page)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO search_space_chat_manifest_pages(space_id,deletion_operation_id,generation,page_index,page_bytes,page_sha256,item_count,next_page_token,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, spaceID, opID, generation, page.GetPageIndex(), pageBytes, page.GetPageSha256(), len(page.GetItemIds()), page.GetNextPageToken(), at); err != nil {
			return err
		}
		for i, raw := range page.GetItemIds() {
			id, _ := canonicalUUID(raw)
			if _, err = tx.Exec(ctx, `INSERT INTO search_space_chat_manifest_items(space_id,deletion_operation_id,generation,page_index,item_index,chat_id) VALUES($1,$2,$3,$4,$5,$6)`, spaceID, opID, generation, page.GetPageIndex(), i, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePurge(req *searchv1.PurgeSpaceRequest) (uuid.UUID, uuid.UUID, error) {
	if req == nil || hasUnknown(req) {
		return uuid.Nil, uuid.Nil, errors.New("invalid purge")
	}
	p := req.GetPurge()
	if p == nil || p.GetProtocolVersion() != 1 || p.GetGeneration() == 0 || p.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_SEARCH || validateManifest(p.GetManifest()) != nil || p.GetPurgeDecidedAt() == nil || p.GetPurgeDecidedAt().CheckValid() != nil || p.GetPurgeDecidedAt().AsTime().IsZero() {
		return uuid.Nil, uuid.Nil, errors.New("invalid purge")
	}
	return lifecycleIDs(p.GetSpaceId(), p.GetDeletionOperationId())
}

func (s *SearchGRPC) PurgeSpace(ctx context.Context, req *searchv1.PurgeSpaceRequest) (_ *searchv1.PurgeSpaceResponse, err error) {
	if err = trustedSearchLifecycle(ctx, req, searchv1.SearchService_PurgeSpace_FullMethodName); err != nil {
		return nil, err
	}
	spaceID, opID, e := validatePurge(req)
	if e != nil {
		return nil, status.Error(codes.InvalidArgument, e.Error())
	}
	pool := s.lifecyclePool()
	if pool == nil {
		return nil, status.Error(codes.Unavailable, "lifecycle store unavailable")
	}
	wire, e := deterministicBytes(req)
	if e != nil {
		return nil, status.Error(codes.InvalidArgument, e.Error())
	}
	hash := lifecycleDigest(req, wire)
	tx, e := beginLifecycleTx(ctx, pool, spaceID)
	if e != nil {
		return nil, status.Error(codes.Unavailable, e.Error())
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		} else {
			err = tx.Commit(ctx)
		}
	}()
	var generation uint64
	var state string
	var fenceOp uuid.UUID
	if e = tx.QueryRow(ctx, `SELECT generation,state,deletion_operation_id FROM search_space_lifecycle_fences WHERE space_id=$1 FOR UPDATE`, spaceID).Scan(&generation, &state, &fenceOp); e != nil {
		return nil, status.Error(codes.FailedPrecondition, "purge not decided")
	}
	if state == "PURGED" {
		var saved, receipt []byte
		if e = tx.QueryRow(ctx, `SELECT request_bytes,receipt_bytes FROM search_space_purge_receipts WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, spaceID, opID, req.GetPurge().GetGeneration()).Scan(&saved, &receipt); e != nil || !bytes.Equal(saved, wire) {
			return nil, status.Error(codes.AlreadyExists, "purge operation conflict")
		}
		out := &searchv1.PurgeSpaceResponse{}
		if e = proto.Unmarshal(receipt, out); e != nil {
			return nil, status.Error(codes.Internal, e.Error())
		}
		return out, nil
	}
	if generation != req.GetPurge().GetGeneration() || state != "PURGE_DECIDED" || fenceOp != opID {
		return nil, status.Error(codes.FailedPrecondition, "purge not decided")
	}
	var fenceBytes []byte
	if e = tx.QueryRow(ctx, `SELECT request_bytes FROM search_space_lifecycle_operations WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3`, spaceID, opID, generation).Scan(&fenceBytes); e != nil {
		return nil, status.Error(codes.FailedPrecondition, "purge fence evidence missing")
	}
	fenceReq := &searchv1.ApplySpaceLifecycleFenceRequest{}
	if e = proto.Unmarshal(fenceBytes, fenceReq); e != nil || !proto.Equal(fenceReq.GetFence().GetManifest(), req.GetPurge().GetManifest()) {
		return nil, status.Error(codes.FailedPrecondition, "purge manifest does not bind fence")
	}
	if e = validatePersistedManifestEvidence(ctx, tx, spaceID, opID, req.GetPurge().GetManifest()); e != nil {
		return nil, status.Error(codes.FailedPrecondition, "complete Chat manifest evidence invalid")
	}
	completed := time.Time{}
	if e = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&completed); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	receipt := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: spaceID.String(), DeletionOperationId: opID.String(), Generation: generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_SEARCH, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: hash, CompletedAt: timestamppb.New(completed)}
	out := &searchv1.PurgeSpaceResponse{Receipt: receipt}
	receiptBytes, e := proto.MarshalOptions{Deterministic: true}.Marshal(out)
	if e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	for _, statement := range []string{`DELETE FROM message_search_documents WHERE chat_id IN (SELECT chat_id FROM search_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2)`, `DELETE FROM chat_search_documents WHERE chat_id IN (SELECT chat_id FROM search_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2)`, `INSERT INTO search_space_purged_chat_fences(space_id,chat_id) SELECT space_id,chat_id FROM search_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 ON CONFLICT DO NOTHING`} {
		if _, e = tx.Exec(ctx, statement, spaceID, opID); e != nil {
			return nil, status.Error(codes.Internal, e.Error())
		}
	}
	if _, e = tx.Exec(ctx, `DELETE FROM space_search_documents WHERE space_id=$1`, spaceID); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	if _, e = tx.Exec(ctx, `INSERT INTO search_space_purge_receipts(space_id,deletion_operation_id,generation,request_bytes,request_sha256,receipt_bytes,completed_at,retain_until) VALUES($1,$2,$3,$4,$5,$6,$7,$7::timestamptz+interval '30 days')`, spaceID, opID, generation, wire, hash, receiptBytes, completed); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	if _, e = tx.Exec(ctx, `UPDATE search_space_lifecycle_fences SET state='PURGED',updated_at=$2 WHERE space_id=$1`, spaceID, completed); e != nil {
		return nil, status.Error(codes.Internal, e.Error())
	}
	return out, nil
}
