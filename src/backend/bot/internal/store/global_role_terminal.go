package store

import (
	"bytes"
	"context"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

func (s *BotStore) SealBotGlobalRoleRequest(ctx context.Context, claim *BotDeletion, body []byte) error {
	return s.sealBotGlobalTerminalRequest(ctx, claim, "role", body)
}

func (s *BotStore) SaveBotGlobalRoleReceipt(ctx context.Context, claim *BotDeletion, body, receipt []byte) error {
	body, receipt = bytes.Clone(body), bytes.Clone(receipt)
	if err := validateGlobalTerminalRequest(claim, "role", body); err != nil {
		return err
	}
	if len(receipt) == 0 || len(receipt) > 4096 {
		return ErrInstallationConflict
	}
	r := new(rolev1.RegisterBotGlobalTerminalResponse)
	if proto.Unmarshal(receipt, r) != nil || len(r.ProtoReflect().GetUnknown()) != 0 || r.Receipt == nil {
		return ErrInstallationConflict
	}
	proof := r.Receipt
	id, idErr := uuid.Parse(proof.ReceiptId)
	canonical, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	if err != nil || !bytes.Equal(canonical, receipt) || len(proof.ProtoReflect().GetUnknown()) != 0 || proof.ProtocolVersion != 1 || proof.Participant != "role" || proof.BotId != claim.BotID.String() || proof.DecisionId != claim.OperationID.String() || idErr != nil || id == uuid.Nil || id.String() != proof.ReceiptId || proof.RegisteredAt == nil || proof.RegisteredAt.CheckValid() != nil || proof.RegisteredAt.Seconds <= 0 || len(proof.RegisteredAt.ProtoReflect().GetUnknown()) != 0 || proof.Outcome != rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED {
		return ErrInstallationConflict
	}
	return s.saveBotGlobalTerminalReceipt(ctx, claim, "role", body, receipt)
}
