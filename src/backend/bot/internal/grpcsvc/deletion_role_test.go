package grpcsvc

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	botv1 "voice.app/voice/bot/v1"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func deletionRoleSource(t *testing.T) (*store.BotDeletion, store.BotDeletionEffect) {
	t.Helper()
	op := &store.BotDeletion{OperationID: uuid.New(), BotID: uuid.New(), OwnerAccountID: uuid.New(), ActorProfileID: uuid.New(), State: "RUNNING", ClaimGeneration: 1}
	request := &botv1.DeleteBotRequest{BotId: op.BotID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	decision := struct {
		OperationID, BotID, OwnerAccountID, ActorProfileID uuid.UUID
		Purpose                                            string
		Admission                                          *store.DeletionAdmission
	}{op.OperationID, op.BotID, op.OwnerAccountID, op.ActorProfileID, "global_delete", &store.DeletionAdmission{AccountID: op.OwnerAccountID, ProfileID: uuid.New(), SessionEpoch: 9, AccountType: "regular", Method: botv1.BotService_DeleteBot_FullMethodName, RequestID: uuid.NewString(), RequestHash: hash, RequestBytes: body, ExpiresAt: time.Now().Add(-time.Hour)}}
	op.DecisionBytes, err = json.Marshal(decision)
	require.NoError(t, err)
	digest := sha256.Sum256(op.DecisionBytes)
	op.DecisionSHA256 = digest[:]
	target := store.BotDeletionTarget{TargetID: uuid.New(), OperationID: op.OperationID, BotID: op.BotID, SpaceID: uuid.New(), ActorProfileID: op.ActorProfileID, InstallationID: uuid.New(), InstallOperationID: uuid.New(), BotGeneration: 1, CancelOperationID: uuid.New(), Chats: []uuid.UUID{uuid.New()}}
	target.ScopeBytes, err = json.Marshal(target)
	require.NoError(t, err)
	scope := sha256.Sum256(target.ScopeBytes)
	target.ScopeSHA256 = scope[:]
	return op, store.BotDeletionEffect{EffectID: uuid.New(), Target: target, Kind: "ROLE_REMOVE", State: "PENDING"}
}

func TestRoleDeletionRequestUsesOnlyFrozenQualifiedSource(t *testing.T) {
	op, effect := deletionRoleSource(t)
	request, err := buildRoleDeletionRequest(op, effect)
	require.NoError(t, err, "expired initiating credential does not revoke committed recovery authority")
	require.Equal(t, op.OperationID.String(), request.GetDecisionId())
	require.Equal(t, effect.EffectID.String(), request.GetOperationId())
	require.Equal(t, effect.Target.InstallationID.String(), request.GetNative().GetInstallationId())
	require.Equal(t, effect.Target.InstallOperationID.String(), request.GetNative().GetInstallOperationId())
	require.Equal(t, op.DecisionSHA256, request.GetDecisionSha256())
	require.Equal(t, effect.Target.ScopeSHA256, request.GetFrozenScopeSha256())
	for _, mode := range []string{"root hash", "foreign actor", "scope hash", "scope mutation", "missing admission", "wrong method", "missing native operation", "zero generation"} {
		t.Run(mode, func(t *testing.T) {
			d, e := deletionRoleSource(t)
			switch mode {
			case "root hash":
				d.DecisionSHA256[0] ^= 1
			case "foreign actor":
				e.Target.ActorProfileID = uuid.New()
			case "scope hash":
				e.Target.ScopeSHA256[0] ^= 1
			case "scope mutation":
				e.Target.InstallationID = uuid.New()
			case "missing admission":
				d.DecisionBytes = []byte(`{}`)
				sum := sha256.Sum256(d.DecisionBytes)
				d.DecisionSHA256 = sum[:]
			case "wrong method":
				d.DecisionBytes = []byte(`{"Purpose":"space_uninstall"}`)
				sum := sha256.Sum256(d.DecisionBytes)
				d.DecisionSHA256 = sum[:]
			case "missing native operation":
				e.Target.InstallOperationID = uuid.Nil
				e.Target.ScopeBytes, _ = json.Marshal(e.Target)
				sum := sha256.Sum256(e.Target.ScopeBytes)
				e.Target.ScopeSHA256 = sum[:]
			case "zero generation":
				e.Target.BotGeneration = 0
				e.Target.ScopeBytes, _ = json.Marshal(e.Target)
				sum := sha256.Sum256(e.Target.ScopeBytes)
				e.Target.ScopeSHA256 = sum[:]
			}
			_, err := buildRoleDeletionRequest(d, e)
			require.Error(t, err)
		})
	}
	for _, orphan := range []bool{false, true} {
		d, e := deletionRoleSource(t)
		e.Target.LegacyEvidence = true
		if orphan {
			e.Target.InstallationID = uuid.Nil
			e.Target.InstallOperationID = uuid.Nil
			e.Target.BotGeneration = 0
		}
		e.Target.ScopeBytes, _ = json.Marshal(e.Target)
		sum := sha256.Sum256(e.Target.ScopeBytes)
		e.Target.ScopeSHA256 = sum[:]
		r, err := buildRoleDeletionRequest(d, e)
		require.NoError(t, err)
		if orphan {
			require.NotNil(t, r.GetOrphanWhitelist())
			require.Nil(t, r.GetNative())
		} else {
			require.NotNil(t, r.GetLegacyInstallation())
			require.Nil(t, r.GetNative())
		}
	}
}

func TestRoleDeletionReceiptRejectsWrongOrIncompleteBinding(t *testing.T) {
	op, effect := deletionRoleSource(t)
	request, err := buildRoleDeletionRequest(op, effect)
	require.NoError(t, err)
	evidenceBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request.GetNative())
	require.NoError(t, err)
	evidenceSHA := sha256.Sum256(evidenceBytes)
	requestBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	requestSHA := sha256.Sum256(requestBytes)
	response := &rolev1.ApplyBotInstallationRemovalResponse{Receipt: &rolev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: request.OperationId, DecisionId: request.DecisionId, Purpose: request.Purpose, BotId: request.BotId, ActorProfileId: request.ActorProfileId, SpaceId: request.SpaceId, EvidenceKind: "native", EvidenceSha256: evidenceSHA[:], FrozenScopeSha256: request.FrozenScopeSha256, RequestSha256: requestSHA[:], Participant: "role", Outcome: "COMPLETED", CompletedAt: timestamppb.Now()}}
	require.NoError(t, validateRoleDeletionReceipt(request, response))
	for _, mutate := range []func(*rolev1.ApplyBotInstallationRemovalResponse){
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt = nil },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.OperationId = uuid.NewString() },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.DecisionId = uuid.NewString() },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.ActorProfileId = uuid.NewString() },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.EvidenceKind = "orphan_whitelist" },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.RequestSha256[0] ^= 1 },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.EvidenceSha256[0] ^= 1 },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.Outcome = "PENDING" },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.Participant = "messaging" },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) { r.Receipt.CompletedAt = nil },
		func(r *rolev1.ApplyBotInstallationRemovalResponse) {
			r.Receipt.DeletedRoleCount = uint64(math.MaxInt64) + 1
		},
		func(r *rolev1.ApplyBotInstallationRemovalResponse) {
			r.Receipt.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
		},
	} {
		bad := proto.Clone(response).(*rolev1.ApplyBotInstallationRemovalResponse)
		mutate(bad)
		require.Error(t, validateRoleDeletionReceipt(request, bad))
	}
}
