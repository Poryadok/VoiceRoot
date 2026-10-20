package grpcsvc

import (
	"bytes"
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func buildGlobalRoleTerminalRequest(claim *store.BotDeletion) (*rolev1.RegisterBotGlobalTerminalRequest, error) {
	if cleanupPurpose(claim) != "global_delete" || validateDeletionDecision(claim) != nil {
		return nil, errDeletionEvidence
	}
	return &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: claim.BotID.String(), DecisionId: claim.OperationID.String()}, nil
}

func validateGlobalRoleTerminalReceipt(q *rolev1.RegisterBotGlobalTerminalRequest, response *rolev1.RegisterBotGlobalTerminalResponse) error {
	if q == nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || r.Participant != "role" || r.BotId != q.BotId || r.DecisionId != q.DecisionId || !deletionUUID(r.ReceiptId) || r.RegisteredAt == nil || r.RegisteredAt.CheckValid() != nil || r.RegisteredAt.Seconds <= 0 || len(r.RegisteredAt.ProtoReflect().GetUnknown()) != 0 || r.Outcome != rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) applyGlobalRoleTerminal(ctx context.Context, claim *store.BotDeletion, effect store.BotGlobalTerminalEffect) error {
	q, err := buildGlobalRoleTerminalRequest(claim)
	if err != nil || effect.Participant != "role" {
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil || (effect.RequestBytes != nil && !bytes.Equal(effect.RequestBytes, body)) {
		return errDeletionEvidence
	}
	if effect.State == "COMPLETE" {
		r := new(rolev1.RegisterBotGlobalTerminalResponse)
		if effect.RequestBytes == nil || proto.Unmarshal(effect.ReceiptBytes, r) != nil || validateGlobalRoleTerminalReceipt(q, r) != nil {
			return errDeletionEvidence
		}
		return nil
	}
	if s.CleanupRole == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected global Role terminal authority unavailable")
	}
	if err = s.Store.SealBotGlobalRoleRequest(ctx, claim, body); err != nil {
		return err
	}
	hash, err := principal.RequestHash(q)
	if err != nil {
		return err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, RequestID: q.DecisionId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot Role terminal principal unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", q.DecisionId))
	response, err := s.CleanupRole.RegisterBotGlobalTerminal(callCtx, q)
	if err != nil {
		return err
	}
	if validateGlobalRoleTerminalReceipt(q, response) != nil {
		return status.Error(codes.Unavailable, "Role global terminal acknowledgement is not qualified")
	}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	return s.Store.SaveBotGlobalRoleReceipt(ctx, claim, body, receipt)
}
