package grpcsvc

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
)

// consumeSpaceDeletionProof recovers only Auth's durable receipt after an
// ambiguous consume. It returns the deterministic typed response expected by
// SpaceStore; callers must persist it before advancing lifecycle state.
func (s *SpaceGRPC) consumeSpaceDeletionProof(
	ctx context.Context,
	accountID, profileID uuid.UUID,
	epoch int64,
	request *spacev1.DeleteSpaceRequest,
) ([]byte, [sha256.Size]byte, error) {
	var emptyHash [sha256.Size]byte
	if s == nil || s.OwnershipAuth == nil || accountID == uuid.Nil || profileID == uuid.Nil || epoch <= 0 ||
		request == nil || request.GetConfirmationName() == "" || request.GetProof() == "" {
		return nil, emptyHash, status.Error(codes.Unavailable, "space deletion proof authority unavailable")
	}
	spaceID, err := uuid.Parse(request.GetSpaceId())
	if err != nil || spaceID == uuid.Nil || spaceID.String() != request.GetSpaceId() {
		return nil, emptyHash, status.Error(codes.InvalidArgument, "invalid space deletion request")
	}
	operationID, err := uuid.Parse(request.GetOperationId())
	if err != nil || operationID == uuid.Nil || operationID.String() != request.GetOperationId() {
		return nil, emptyHash, status.Error(codes.InvalidArgument, "invalid space deletion request")
	}
	proofDigest := sha256.Sum256([]byte(request.GetProof()))
	confirmationDigest := sha256.Sum256([]byte(request.GetConfirmationName()))
	consume := &authv1.ConsumeSpaceDeletionProofRequest{
		ProtocolVersion:  1,
		AccountId:        accountID.String(),
		ProfileId:        profileID.String(),
		SessionEpoch:     epoch,
		SpaceId:          spaceID.String(),
		OperationId:      operationID.String(),
		ConfirmationName: request.GetConfirmationName(),
		Proof:            request.GetProof(),
	}
	signed, err := s.ownershipAuthContext(ctx, consume, authv1.AuthService_ConsumeSpaceDeletionProof_FullMethodName, operationID)
	if err != nil {
		return nil, emptyHash, err
	}
	response, err := s.OwnershipAuth.ConsumeSpaceDeletionProof(signed, consume)
	if err != nil {
		if !ambiguousOwnershipAuthError(err) {
			return nil, emptyHash, err
		}
		lookup := &authv1.GetSpaceDeletionProofReceiptRequest{
			ProtocolVersion:        1,
			AccountId:              accountID.String(),
			ProfileId:              profileID.String(),
			SessionEpoch:           epoch,
			SpaceId:                spaceID.String(),
			OperationId:            operationID.String(),
			ConfirmationNameSha256: confirmationDigest[:],
			ProofDigestSha256:      proofDigest[:],
		}
		signed, err = s.ownershipAuthContext(ctx, lookup, authv1.AuthService_GetSpaceDeletionProofReceipt_FullMethodName, operationID)
		if err != nil {
			return nil, emptyHash, err
		}
		recovered, lookupErr := s.OwnershipAuth.GetSpaceDeletionProofReceipt(signed, lookup)
		if lookupErr != nil {
			return nil, emptyHash, lookupErr
		}
		if recovered == nil {
			return nil, emptyHash, status.Error(codes.Unavailable, "missing space deletion proof receipt")
		}
		response = &authv1.ConsumeSpaceDeletionProofResponse{Receipt: recovered.GetReceipt()}
	}
	if response == nil || !validSpaceDeletionProofReceipt(response.GetReceipt(), accountID, profileID, epoch, spaceID, operationID, proofDigest, confirmationDigest) {
		return nil, emptyHash, status.Error(codes.Unavailable, "invalid space deletion proof receipt")
	}
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, emptyHash, status.Error(codes.Unavailable, "invalid space deletion proof receipt")
	}
	hash := lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", wire)
	return wire, hash, nil
}

func validSpaceDeletionProofReceipt(
	receipt *authv1.SpaceDeletionProofReceipt,
	accountID, profileID uuid.UUID,
	epoch int64,
	spaceID, operationID uuid.UUID,
	proofDigest, confirmationDigest [sha256.Size]byte,
) bool {
	if receipt == nil || receipt.GetProtocolVersion() != 1 || receipt.GetConsumedAt() == nil ||
		receipt.GetConsumedAt().CheckValid() != nil || receipt.GetConsumedAt().GetNanos()%1000 != 0 ||
		len(receipt.GetBindingSha256()) != sha256.Size ||
		subtle.ConstantTimeCompare(receipt.GetConfirmationNameSha256(), confirmationDigest[:]) != 1 {
		return false
	}
	receiptID, err := uuid.Parse(receipt.GetReceiptId())
	if err != nil || receiptID == uuid.Nil || receiptID.String() != receipt.GetReceiptId() || receipt.GetOperationId() != operationID.String() {
		return false
	}
	factors := receipt.GetVerifiedFactors()
	if len(factors) < 1 || len(factors) > 2 || factors[0] != authv1.VerifiedFactor_VERIFIED_FACTOR_PASSWORD ||
		(len(factors) == 2 && factors[1] != authv1.VerifiedFactor_VERIFIED_FACTOR_TOTP && factors[1] != authv1.VerifiedFactor_VERIFIED_FACTOR_BACKUP_CODE) {
		return false
	}
	binding := &authv1.SpaceDeletionProofBinding{
		ProtocolVersion:        1,
		AccountId:              accountID.String(),
		ProfileId:              profileID.String(),
		SessionEpoch:           epoch,
		SpaceId:                spaceID.String(),
		OperationId:            operationID.String(),
		ConfirmationNameSha256: confirmationDigest[:],
		ProofDigestSha256:      proofDigest[:],
		VerifiedFactors:        factors,
		Purpose:                authv1.ProofPurpose_PROOF_PURPOSE_SPACE_DELETE,
	}
	bindingBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(binding)
	if err != nil {
		return false
	}
	expectedBinding := lifecycleEvidenceHash("voice.auth.v1.SpaceDeletionProofBinding", bindingBytes)
	return subtle.ConstantTimeCompare(receipt.GetBindingSha256(), expectedBinding[:]) == 1
}

func lifecycleEvidenceHash(fqn string, payload []byte) [sha256.Size]byte {
	hasher := sha256.New()
	_, _ = hasher.Write([]byte(fqn))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write(payload)
	var digest [sha256.Size]byte
	copy(digest[:], hasher.Sum(nil))
	return digest
}
