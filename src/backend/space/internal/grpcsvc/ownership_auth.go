package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/store"
)

func (s *SpaceGRPC) ownershipAuthContext(ctx context.Context, request proto.Message, method string, operationID uuid.UUID) (context.Context, error) {
	if s == nil || s.PrincipalIssuer == nil {
		return nil, status.Error(codes.Unavailable, "ownership auth principal runtime unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid ownership auth request")
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: method, RequestID: operationID.String(), RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "ownership auth principal signing unavailable")
	}
	// An explicit metadata context prevents propagated user credentials or raw identity from reaching Auth.
	return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operationID.String())), nil
}

// consumeOwnershipAuthProof is a private coordinator seam. It never mutates Space state and is unused while public transfer is disabled.
func (s *SpaceGRPC) consumeOwnershipAuthProof(ctx context.Context, binding store.OwnershipBinding, proof string) (store.OwnershipAuthReceipt, error) {
	if s == nil || s.OwnershipAuth == nil {
		return store.OwnershipAuthReceipt{}, status.Error(codes.Unavailable, "ownership auth transport unavailable")
	}
	digest := sha256.Sum256([]byte(proof))
	if proof == "" || binding.ProofDigest != hex.EncodeToString(digest[:]) {
		return store.OwnershipAuthReceipt{}, status.Error(codes.PermissionDenied, "ownership proof binding mismatch")
	}
	consume := &authv1.ConsumeOwnershipTransferProofRequest{AccountId: binding.AccountID.String(), ProfileId: binding.ActorProfileID.String(), SpaceId: binding.SpaceID.String(), NewOwnerProfileId: binding.NewOwnerProfileID.String(), OperationId: binding.OperationID.String(), SessionEpoch: binding.SessionEpoch, Proof: proof}
	signed, err := s.ownershipAuthContext(ctx, consume, authv1.AuthService_ConsumeOwnershipTransferProof_FullMethodName, binding.OperationID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, err
	}
	response, err := s.OwnershipAuth.ConsumeOwnershipTransferProof(signed, consume)
	if err == nil {
		return ownershipAuthReceiptFromConsume(binding, response)
	}
	if !ambiguousOwnershipAuthError(err) {
		return store.OwnershipAuthReceipt{}, err
	}
	lookup := &authv1.GetOwnershipTransferReceiptRequest{AccountId: binding.AccountID.String(), ProfileId: binding.ActorProfileID.String(), SpaceId: binding.SpaceID.String(), NewOwnerProfileId: binding.NewOwnerProfileID.String(), OperationId: binding.OperationID.String(), SessionEpoch: binding.SessionEpoch, ProofDigest: binding.ProofDigest}
	signed, signErr := s.ownershipAuthContext(ctx, lookup, authv1.AuthService_GetOwnershipTransferReceipt_FullMethodName, binding.OperationID)
	if signErr != nil {
		return store.OwnershipAuthReceipt{}, signErr
	}
	recovered, lookupErr := s.OwnershipAuth.GetOwnershipTransferReceipt(signed, lookup)
	if lookupErr != nil {
		return store.OwnershipAuthReceipt{}, lookupErr
	}
	return ownershipAuthReceiptFromLookup(binding, recovered)
}

func ambiguousOwnershipAuthError(err error) bool {
	return status.Code(err) == codes.Unavailable || status.Code(err) == codes.DeadlineExceeded || status.Code(err) == codes.Canceled || err == context.DeadlineExceeded
}

func ownershipAuthReceiptFromConsume(binding store.OwnershipBinding, response *authv1.ConsumeOwnershipTransferProofResponse) (store.OwnershipAuthReceipt, error) {
	if response == nil {
		return store.OwnershipAuthReceipt{}, status.Error(codes.Unavailable, "missing ownership auth receipt")
	}
	return parseOwnershipAuthReceipt(binding, response.GetReceiptId(), response.GetAccountId(), response.GetProfileId(), response.GetSpaceId(), response.GetNewOwnerProfileId(), response.GetOperationId(), response.GetSessionEpoch(), response.GetConsumedAt(), response.GetVerifiedFactors())
}

func ownershipAuthReceiptFromLookup(binding store.OwnershipBinding, response *authv1.GetOwnershipTransferReceiptResponse) (store.OwnershipAuthReceipt, error) {
	if response == nil {
		return store.OwnershipAuthReceipt{}, status.Error(codes.Unavailable, "missing ownership auth receipt")
	}
	return parseOwnershipAuthReceipt(binding, response.GetReceiptId(), response.GetAccountId(), response.GetProfileId(), response.GetSpaceId(), response.GetNewOwnerProfileId(), response.GetOperationId(), response.GetSessionEpoch(), response.GetConsumedAt(), response.GetVerifiedFactors())
}

func parseOwnershipAuthReceipt(binding store.OwnershipBinding, receiptID, accountID, profileID, spaceID, newOwnerID, operationID string, epoch int64, consumedAt *timestamppb.Timestamp, factors []string) (store.OwnershipAuthReceipt, error) {
	if consumedAt == nil || consumedAt.CheckValid() != nil || consumedAt.Nanos%1000 != 0 || !validOwnershipFactors(factors) {
		return store.OwnershipAuthReceipt{}, status.Error(codes.Unavailable, "invalid ownership auth receipt")
	}
	parse := func(value string) (uuid.UUID, error) { return uuid.Parse(value) }
	receipt, err := parse(receiptID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	account, err := parse(accountID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	profile, err := parse(profileID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	space, err := parse(spaceID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	newOwner, err := parse(newOwnerID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	operation, err := parse(operationID)
	if err != nil {
		return store.OwnershipAuthReceipt{}, invalidOwnershipAuthReceipt(err)
	}
	at := consumedAt.AsTime().UTC()
	if receipt == uuid.Nil || account != binding.AccountID || profile != binding.ActorProfileID || space != binding.SpaceID || newOwner != binding.NewOwnerProfileID || operation != binding.OperationID || epoch != binding.SessionEpoch || epoch <= 0 || at.IsZero() || at.Location() != time.UTC {
		return store.OwnershipAuthReceipt{}, status.Error(codes.Unavailable, "ownership auth receipt binding mismatch")
	}
	return store.OwnershipAuthReceipt{ReceiptID: receipt, AccountID: account, ProfileID: profile, SpaceID: space, NewOwnerProfileID: newOwner, OperationID: operation, SessionEpoch: epoch, ConsumedAt: at, VerifiedFactors: append([]string(nil), factors...)}, nil
}

func validOwnershipFactors(factors []string) bool {
	if len(factors) < 1 || len(factors) > 2 || factors[0] != "password" {
		return false
	}
	if len(factors) == 1 {
		return true
	}
	return factors[1] == "totp" || factors[1] == "backup_code"
}

func invalidOwnershipAuthReceipt(err error) error {
	return status.Error(codes.Unavailable, fmt.Sprintf("invalid ownership auth receipt: %v", err))
}
