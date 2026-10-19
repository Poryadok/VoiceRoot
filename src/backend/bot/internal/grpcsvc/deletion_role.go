package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

var errDeletionEvidence = errors.New("invalid frozen Bot deletion authority or participant receipt")

func deletionUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

// Recovery authority is the exact committed decision, not a still-live user
// bearer token. Expired admission timestamps therefore remain historical data.
func validateDeletionSource(op *store.BotDeletion, effect store.BotDeletionEffect) error {
	if op == nil || op.OperationID == uuid.Nil || op.BotID == uuid.Nil || op.OwnerAccountID == uuid.Nil || op.ActorProfileID == uuid.Nil || op.State != "RUNNING" || op.ClaimGeneration <= 0 || effect.EffectID == uuid.Nil {
		return errDeletionEvidence
	}
	decisionSHA := sha256.Sum256(op.DecisionBytes)
	if !bytes.Equal(decisionSHA[:], op.DecisionSHA256) {
		return errDeletionEvidence
	}
	var decision struct {
		OperationID, BotID, OwnerAccountID, ActorProfileID uuid.UUID
		Purpose                                            string
		Admission                                          *store.DeletionAdmission
	}
	decoder := json.NewDecoder(bytes.NewReader(op.DecisionBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decision); err != nil {
		return errDeletionEvidence
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || decision.OperationID != op.OperationID || decision.BotID != op.BotID || decision.OwnerAccountID != op.OwnerAccountID || decision.ActorProfileID != op.ActorProfileID || decision.Purpose != "global_delete" || decision.Admission == nil {
		return errDeletionEvidence
	}
	a := decision.Admission
	if a.AccountID != op.OwnerAccountID || a.ProfileID == uuid.Nil || a.SessionEpoch <= 0 || !deletionUUID(a.RequestID) || a.Method != botv1.BotService_DeleteBot_FullMethodName || a.ExpiresAt.IsZero() || (a.AccountType != "regular" && a.AccountType != "guest" && a.AccountType != "sdk-account") {
		return errDeletionEvidence
	}
	var original botv1.DeleteBotRequest
	if proto.Unmarshal(a.RequestBytes, &original) != nil || len(original.ProtoReflect().GetUnknown()) != 0 || original.GetBotId() != op.BotID.String() {
		return errDeletionEvidence
	}
	hash, err := principal.RequestHash(&original)
	canonical, marshalErr := (proto.MarshalOptions{Deterministic: true}).Marshal(&original)
	if err != nil || marshalErr != nil || hash != a.RequestHash || !bytes.Equal(canonical, a.RequestBytes) {
		return errDeletionEvidence
	}
	t := effect.Target
	scopeSHA := sha256.Sum256(t.ScopeBytes)
	scope, err := json.Marshal(t)
	if err != nil || !bytes.Equal(scope, t.ScopeBytes) || !bytes.Equal(scopeSHA[:], t.ScopeSHA256) || t.TargetID == uuid.Nil || t.OperationID != op.OperationID || t.BotID != op.BotID || t.ActorProfileID != op.ActorProfileID || t.SpaceID == uuid.Nil {
		return errDeletionEvidence
	}
	return nil
}

func buildRoleDeletionRequest(op *store.BotDeletion, effect store.BotDeletionEffect) (*rolev1.ApplyBotInstallationRemovalRequest, error) {
	if err := validateDeletionSource(op, effect); err != nil {
		return nil, err
	}
	if effect.Kind != "ROLE_REMOVE" || effect.ChatID != nil {
		return nil, errDeletionEvidence
	}
	t := effect.Target
	r := &rolev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: effect.EffectID.String(), DecisionId: op.OperationID.String(), DecisionSha256: append([]byte(nil), op.DecisionSHA256...), Purpose: "global_delete", BotId: op.BotID.String(), ActorProfileId: op.ActorProfileID.String(), SpaceId: t.SpaceID.String(), FrozenScopeSha256: append([]byte(nil), t.ScopeSHA256...)}
	switch {
	case !t.LegacyEvidence:
		if t.InstallationID == uuid.Nil || t.InstallOperationID == uuid.Nil || t.BotGeneration <= 0 {
			return nil, errDeletionEvidence
		}
		r.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_Native{Native: &rolev1.BotRemovalNativeEvidence{InstallationId: t.InstallationID.String(), InstallOperationId: t.InstallOperationID.String(), Generation: uint64(t.BotGeneration)}}
	case t.InstallationID != uuid.Nil:
		r.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_LegacyInstallation{LegacyInstallation: &rolev1.BotRemovalLegacyInstallationEvidence{InstallationId: t.InstallationID.String(), SnapshotId: t.TargetID.String(), SnapshotSha256: append([]byte(nil), t.ScopeSHA256...)}}
	default:
		if t.InstallOperationID != uuid.Nil || t.BotGeneration != 0 || len(t.Chats) == 0 {
			return nil, errDeletionEvidence
		}
		r.Evidence = &rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist{OrphanWhitelist: &rolev1.BotRemovalOrphanWhitelistEvidence{SnapshotId: t.TargetID.String(), SnapshotSha256: append([]byte(nil), t.ScopeSHA256...)}}
	}
	return r, nil
}

func roleRemovalEvidence(request *rolev1.ApplyBotInstallationRemovalRequest) (proto.Message, string, error) {
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 {
		return nil, "", errDeletionEvidence
	}
	var evidence proto.Message
	var kind string
	switch e := request.Evidence.(type) {
	case *rolev1.ApplyBotInstallationRemovalRequest_Native:
		evidence, kind = e.Native, "native"
	case *rolev1.ApplyBotInstallationRemovalRequest_LegacyInstallation:
		evidence, kind = e.LegacyInstallation, "legacy_installation"
	case *rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist:
		evidence, kind = e.OrphanWhitelist, "orphan_whitelist"
	default:
		return nil, "", errDeletionEvidence
	}
	if evidence == nil || !evidence.ProtoReflect().IsValid() || len(evidence.ProtoReflect().GetUnknown()) != 0 {
		return nil, "", errDeletionEvidence
	}
	return evidence, kind, nil
}

func validateRoleDeletionReceipt(request *rolev1.ApplyBotInstallationRemovalRequest, response *rolev1.ApplyBotInstallationRemovalResponse) error {
	evidence, kind, err := roleRemovalEvidence(request)
	if err != nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !deletionUUID(r.ReceiptId) || r.OperationId != request.OperationId || r.DecisionId != request.DecisionId || r.Purpose != request.Purpose || r.BotId != request.BotId || r.ActorProfileId != request.ActorProfileId || r.SpaceId != request.SpaceId || r.EvidenceKind != kind || r.Participant != "role" || r.Outcome != "COMPLETED" || !bytes.Equal(r.FrozenScopeSha256, request.FrozenScopeSha256) || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 || r.DeletedRoleCount > math.MaxInt64 || r.RemovedAssignmentCount > math.MaxInt64 {
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return errDeletionEvidence
	}
	requestSHA := sha256.Sum256(body)
	body, err = (proto.MarshalOptions{Deterministic: true}).Marshal(evidence)
	if err != nil {
		return errDeletionEvidence
	}
	evidenceSHA := sha256.Sum256(body)
	if !bytes.Equal(requestSHA[:], r.RequestSha256) || !bytes.Equal(evidenceSHA[:], r.EvidenceSha256) {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) applyRoleDeletion(ctx context.Context, claim *store.BotDeletion, effect store.BotDeletionEffect) error {
	if s == nil || s.Store == nil || s.CleanupRole == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected Role cleanup authority unavailable")
	}
	request, err := buildRoleDeletionRequest(claim, effect)
	if err != nil {
		return status.Error(codes.FailedPrecondition, "frozen Bot cleanup evidence is not qualified")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	if len(effect.RequestBytes) != 0 && !bytes.Equal(effect.RequestBytes, body) {
		return store.ErrInstallationConflict
	}
	if err = s.Store.SealBotDeletionRequest(ctx, claim, effect.EffectID, body); err != nil {
		return err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot cleanup principal unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
	response, err := s.CleanupRole.ApplyBotInstallationRemoval(callCtx, request)
	if err != nil {
		return err
	}
	if validateRoleDeletionReceipt(request, response) != nil {
		return status.Error(codes.Unavailable, "Role cleanup acknowledgement is not qualified")
	}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	return s.Store.SaveBotDeletionReceipt(ctx, claim, effect.EffectID, body, receipt)
}
