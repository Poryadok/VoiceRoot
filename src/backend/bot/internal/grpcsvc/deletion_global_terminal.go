package grpcsvc

import (
	"bytes"
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func buildGlobalSpaceTerminalRequest(claim *store.BotDeletion) (*spacev1.RegisterBotGlobalTerminalRequest, error) {
	if cleanupPurpose(claim) != "global_delete" || validateDeletionDecision(claim) != nil {
		return nil, errDeletionEvidence
	}
	return &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: claim.BotID.String(), DecisionId: claim.OperationID.String()}, nil
}

func validateGlobalSpaceTerminalReceipt(q *spacev1.RegisterBotGlobalTerminalRequest, response *spacev1.RegisterBotGlobalTerminalResponse) error {
	if q == nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || r.BotId != q.BotId || r.DecisionId != q.DecisionId || !deletionUUID(r.ReceiptId) || r.RegisteredAt == nil || r.RegisteredAt.CheckValid() != nil || r.RegisteredAt.Seconds <= 0 || len(r.RegisteredAt.ProtoReflect().GetUnknown()) != 0 || r.Outcome != spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) applyGlobalSpaceTerminal(ctx context.Context, claim *store.BotDeletion, effect store.BotGlobalTerminalEffect) error {
	q, err := buildGlobalSpaceTerminalRequest(claim)
	if err != nil || effect.Participant != "space" {
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil || (effect.RequestBytes != nil && !bytes.Equal(effect.RequestBytes, body)) {
		return errDeletionEvidence
	}
	if effect.State == "COMPLETE" {
		r := new(spacev1.RegisterBotGlobalTerminalResponse)
		if effect.RequestBytes == nil || proto.Unmarshal(effect.ReceiptBytes, r) != nil || validateGlobalSpaceTerminalReceipt(q, r) != nil {
			return errDeletionEvidence
		}
		return nil
	}
	if s.CleanupSpace == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected global Space terminal authority unavailable")
	}
	if err = s.Store.SealBotGlobalSpaceRequest(ctx, claim, body); err != nil {
		return err
	}
	hash, err := principal.RequestHash(q)
	if err != nil {
		return err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, RequestID: q.DecisionId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot global terminal principal unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", q.DecisionId))
	response, err := s.CleanupSpace.RegisterBotGlobalTerminal(callCtx, q)
	if err != nil {
		return err
	}
	if validateGlobalSpaceTerminalReceipt(q, response) != nil {
		return status.Error(codes.Unavailable, "Space global terminal acknowledgement is not qualified")
	}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	return s.Store.SaveBotGlobalSpaceReceipt(ctx, claim, body, receipt)
}
