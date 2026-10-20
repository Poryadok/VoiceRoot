package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

type purgeNativeObservation struct {
	OperationID   uuid.UUID `json:"operation_id"`
	RequestBytes  []byte    `json:"request_bytes"`
	RequestSHA256 []byte    `json:"request_sha256"`
	ReceiptBytes  []byte    `json:"receipt_bytes,omitempty"`
	ReceiptSHA256 []byte    `json:"receipt_sha256,omitempty"`
}

// The sealed request and receipt are owned byte copies; a transport response
// does not itself move the local source or any accepted journal to completion.
type BotPurgeNativeEffect struct {
	OperationID                uuid.UUID
	RequestBytes, ReceiptBytes []byte
}

func (c *BotPurgeClaim) NativeDrainRequests(bot uuid.UUID) ([]*spacev1.ReconcileBotPurgeInstallationRequest, error) {
	if c == nil {
		return nil, ErrDeletionClaimLost
	}
	var source *BotPurgeSource
	for _, v := range c.sources {
		if v.bot == bot {
			source = new(BotPurgeSource)
			if json.Unmarshal(v.raw, source) != nil {
				return nil, ErrSpaceLifecycleConflict
			}
			break
		}
	}
	if source == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	var result []*spacev1.ReconcileBotPurgeInstallationRequest
	seenOps, seenInstallations := map[uuid.UUID]bool{}, map[string]bool{}
	for _, drain := range source.NativeDrains {
		native := new(spacev1.RemoveBotMemberRequest)
		digest := sha256.Sum256(drain.RequestBytes)
		if drain.OperationID == uuid.Nil || seenOps[drain.OperationID] || !bytes.Equal(digest[:], drain.RequestSHA256) || !purgeProofCanonical(drain.RequestBytes, native) {
			return nil, ErrSpaceLifecycleConflict
		}
		n := native.InstallationIntent
		if n == nil || n.BotId != source.BotID.String() || n.SpaceId != source.SpaceID.String() || n.ActorProfileId != source.ActorProfileID.String() || native.SpaceId != n.SpaceId || native.ProfileId != n.ActorProfileId || n.Generation == 0 || n.Generation > math.MaxInt64 || seenInstallations[n.InstallationId] {
			return nil, ErrSpaceLifecycleConflict
		}
		for _, id := range []string{n.BotId, n.SpaceId, n.ActorProfileId, n.OperationId, n.InstallationId, native.InstallOperationId} {
			if _, err := lifecycleID(id); err != nil {
				return nil, err
			}
		}
		if drain.OperationID.String() == n.OperationId || drain.OperationID.String() == native.InstallOperationId || drain.OperationID == c.operation {
			return nil, ErrSpaceLifecycleConflict
		}
		matched := false
		if i := source.Intent; i != nil && !i.LegacyEvidence && i.BotID == source.BotID && i.SpaceID == source.SpaceID && i.ActorProfileID == source.ActorProfileID && i.InstallationID.String() == n.InstallationId && i.OperationID.String() == native.InstallOperationId && uint64(i.Generation) == n.Generation && source.NativeCancelOperationID.String() == n.OperationId {
			if source.NativeDrainOperationID != drain.OperationID || (i.CancelOperationID != uuid.Nil && i.CancelOperationID != source.NativeCancelOperationID) {
				return nil, ErrSpaceLifecycleConflict
			}
			matched = true
		}
		for _, v := range source.Retained {
			t := v.Target
			if !t.LegacyEvidence && t.BotID == source.BotID && t.SpaceID == source.SpaceID && t.ActorProfileID == source.ActorProfileID && t.InstallationID.String() == n.InstallationId && t.InstallOperationID.String() == native.InstallOperationId && t.BotGeneration > 0 && uint64(t.BotGeneration) == n.Generation && t.CancelOperationID.String() == n.OperationId {
				matched = true
			}
		}
		if !matched {
			return nil, ErrSpaceLifecycleConflict
		}
		seenOps[drain.OperationID], seenInstallations[n.InstallationId] = true, true
		result = append(result, &spacev1.ReconcileBotPurgeInstallationRequest{ProtocolVersion: 1, OperationId: drain.OperationID.String(), SpaceId: c.space.String(), DeletionOperationId: c.operation.String(), Generation: uint64(c.generation), Manifest: proto.Clone(c.purge.Manifest).(*commonv1.ManifestBinding), BotId: bot.String(), InstallationId: n.InstallationId, NativeCancellation: native})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OperationId < result[j].OperationId })
	return result, nil
}

func ValidateBotPurgeNativeReceipt(q *spacev1.ReconcileBotPurgeInstallationRequest, r *spacev1.ReconcileBotPurgeInstallationResponse) error {
	if q == nil || r == nil || r.Receipt == nil || !purgeProofKnown(q) || !purgeProofKnown(r) {
		return ErrSpaceLifecycleConflict
	}
	if q.ProtocolVersion != 1 || q.Generation == 0 || q.Generation > math.MaxInt64 || q.Manifest == nil || len(q.Manifest.ManifestSha256) != sha256.Size || q.Manifest.ItemCount > math.MaxInt64 || q.NativeCancellation == nil || q.NativeCancellation.InstallationIntent == nil {
		return ErrSpaceLifecycleConflict
	}
	native := q.NativeCancellation
	n := native.InstallationIntent
	for _, id := range []string{q.OperationId, q.SpaceId, q.DeletionOperationId, q.BotId, q.InstallationId, q.Manifest.ManifestId, n.OperationId, native.InstallOperationId, n.ActorProfileId} {
		if _, err := lifecycleID(id); err != nil {
			return err
		}
	}
	if n.BotId != q.BotId || n.InstallationId != q.InstallationId || n.SpaceId != q.SpaceId || native.SpaceId != q.SpaceId || native.ProfileId != n.ActorProfileId || n.Generation == 0 || n.Generation > math.MaxInt64 || q.OperationId == n.OperationId || q.OperationId == native.InstallOperationId || q.OperationId == q.DeletionOperationId {
		return ErrSpaceLifecycleConflict
	}
	v := r.Receipt
	if _, err := lifecycleID(v.ReceiptId); err != nil {
		return err
	}
	if v.ProtocolVersion != 1 || q.ProtocolVersion != 1 || v.OperationId != q.OperationId || v.SpaceId != q.SpaceId || v.DeletionOperationId != q.DeletionOperationId || v.Generation != q.Generation || !proto.Equal(v.Manifest, q.Manifest) || v.BotId != q.BotId || v.InstallationId != q.InstallationId || !proto.Equal(v.NativeCancellation, q.NativeCancellation) || v.CompletedAt == nil || v.CompletedAt.CheckValid() != nil || v.CompletedAt.Seconds <= 0 {
		return ErrSpaceLifecycleConflict
	}
	switch v.Disposition {
	case spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED, spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED:
	default:
		return ErrSpaceLifecycleConflict
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	if !bytes.Equal(v.RequestSha256, digest[:]) {
		return ErrSpaceLifecycleConflict
	}
	return nil
}

func (c *BotPurgeClaim) validateNativeObservations(bot uuid.UUID, values []purgeNativeObservation) error {
	requests, err := c.NativeDrainRequests(bot)
	if err != nil {
		return err
	}
	expected := map[string][]byte{}
	for _, q := range requests {
		raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
		if e != nil {
			return e
		}
		expected[q.OperationId] = raw
	}
	var prior string
	for _, v := range values {
		key := v.OperationID.String()
		body, exists := expected[key]
		digest := sha256.Sum256(v.RequestBytes)
		if !exists || (prior != "" && prior >= key) || !bytes.Equal(body, v.RequestBytes) || !bytes.Equal(digest[:], v.RequestSHA256) || (v.ReceiptBytes == nil) != (v.ReceiptSHA256 == nil) {
			return ErrSpaceLifecycleConflict
		}
		prior = key
		if v.ReceiptBytes != nil {
			receiptSHA := sha256.Sum256(v.ReceiptBytes)
			q, r := new(spacev1.ReconcileBotPurgeInstallationRequest), new(spacev1.ReconcileBotPurgeInstallationResponse)
			if !bytes.Equal(receiptSHA[:], v.ReceiptSHA256) || !purgeProofCanonical(v.RequestBytes, q) || !purgeProofCanonical(v.ReceiptBytes, r) || ValidateBotPurgeNativeReceipt(q, r) != nil {
				return ErrSpaceLifecycleConflict
			}
		}
	}
	return nil
}

func (s *BotStore) purgeObservationTx(ctx context.Context, tx pgx.Tx, c *BotPurgeClaim, bot uuid.UUID) (*purgeCoverageObservation, error) {
	var raw, digest []byte
	err := tx.QueryRow(ctx, `SELECT coverage_bytes,coverage_sha256 FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, c.space, bot).Scan(&raw, &digest)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, ErrDeletionPending
	}
	sum := sha256.Sum256(raw)
	if !bytes.Equal(sum[:], digest) {
		return nil, ErrSpaceLifecycleConflict
	}
	shadow := *c
	shadow.sources = append([]botPurgeClaimSource(nil), c.sources...)
	for i := range shadow.sources {
		if shadow.sources[i].bot == bot {
			shadow.sources[i].coverage = raw
			shadow.sources[i].coverageSHA = digest
		}
	}
	proofs, err := shadow.CoverageObservation(bot)
	if err != nil {
		return nil, err
	}
	batches, err := c.ChatBatches(bot)
	if err != nil {
		return nil, err
	}
	if len(proofs) != len(batches) {
		return nil, ErrDeletionPending
	}
	var envelope purgeCoverageObservation
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	return &envelope, nil
}

func (s *BotStore) savePurgeObservationTx(ctx context.Context, tx pgx.Tx, c *BotPurgeClaim, bot uuid.UUID, envelope *purgeCoverageObservation) error {
	if err := c.validateNativeObservations(bot, envelope.Native); err != nil {
		return err
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	r, err := tx.Exec(ctx, `UPDATE bot_space_purge_sources s SET coverage_bytes=$3,coverage_sha256=$4 WHERE s.space_id=$1 AND s.bot_id=$2 AND s.state='PENDING' AND EXISTS(SELECT 1 FROM bot_space_purge_handoffs h WHERE h.space_id=s.space_id AND h.claim_generation=$5 AND h.claimed_until>clock_timestamp())`, c.space, bot, body, digest[:], c.claimGeneration)
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return nil
}

func (s *BotStore) SealBotPurgeNativeRequest(ctx context.Context, c *BotPurgeClaim, bot, operation uuid.UUID) (*BotPurgeNativeEffect, error) {
	requests, err := c.NativeDrainRequests(bot)
	if err != nil {
		return nil, err
	}
	var body []byte
	for _, q := range requests {
		if q.OperationId == operation.String() {
			body, err = (proto.MarshalOptions{Deterministic: true}).Marshal(q)
			break
		}
	}
	if err != nil {
		return nil, err
	}
	if body == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	tx, err := s.purgeClaimTx(ctx, c)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	envelope, err := s.purgeObservationTx(ctx, tx, c, bot)
	if err != nil {
		return nil, err
	}
	for _, v := range envelope.Native {
		if v.OperationID == operation {
			if !bytes.Equal(v.RequestBytes, body) {
				return nil, ErrSpaceLifecycleConflict
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return &BotPurgeNativeEffect{OperationID: operation, RequestBytes: append([]byte(nil), body...), ReceiptBytes: append([]byte(nil), v.ReceiptBytes...)}, nil
		}
	}
	digest := sha256.Sum256(body)
	envelope.Native = append(envelope.Native, purgeNativeObservation{OperationID: operation, RequestBytes: body, RequestSHA256: append([]byte(nil), digest[:]...)})
	sort.Slice(envelope.Native, func(i, j int) bool {
		return envelope.Native[i].OperationID.String() < envelope.Native[j].OperationID.String()
	})
	if err = s.savePurgeObservationTx(ctx, tx, c, bot, envelope); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &BotPurgeNativeEffect{OperationID: operation, RequestBytes: append([]byte(nil), body...)}, nil
}

func (s *BotStore) SaveBotPurgeNativeReceipt(ctx context.Context, c *BotPurgeClaim, bot, operation uuid.UUID, request, receipt []byte) error {
	// Own all transport bytes before any SQL wait. The sealed request is checked
	// again against the private committed source, not an external request pointer.
	request, receipt = append([]byte(nil), request...), append([]byte(nil), receipt...)
	q, r := new(spacev1.ReconcileBotPurgeInstallationRequest), new(spacev1.ReconcileBotPurgeInstallationResponse)
	if !purgeProofCanonical(request, q) || !purgeProofCanonical(receipt, r) || ValidateBotPurgeNativeReceipt(q, r) != nil {
		return ErrSpaceLifecycleConflict
	}
	tx, err := s.purgeClaimTx(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	envelope, err := s.purgeObservationTx(ctx, tx, c, bot)
	if err != nil {
		return err
	}
	for i := range envelope.Native {
		v := &envelope.Native[i]
		if v.OperationID != operation {
			continue
		}
		if !bytes.Equal(v.RequestBytes, request) {
			return ErrSpaceLifecycleConflict
		}
		if v.ReceiptBytes != nil {
			if !bytes.Equal(v.ReceiptBytes, receipt) {
				return ErrSpaceLifecycleConflict
			}
			return tx.Commit(ctx)
		}
		digest := sha256.Sum256(receipt)
		v.ReceiptBytes = receipt
		v.ReceiptSHA256 = append([]byte(nil), digest[:]...)
		if err = s.savePurgeObservationTx(ctx, tx, c, bot, envelope); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return ErrSpaceLifecycleConflict
}
