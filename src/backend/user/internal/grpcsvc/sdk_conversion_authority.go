package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/principal"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// SdkConversionGRPC is registered only on Auth's dedicated, signed TLS listener.
// The ordinary User service intentionally embeds unimplemented methods.
type SdkConversionGRPC struct {
	userv1.UnimplementedUserServiceServer
	Profiles *store.ProfileStore
}

func requireAuthSdkPrincipal(ctx context.Context, method string, req proto.Message) error {
	verified, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Auth principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || !socialprincipal.AllowsMethod("auth", method) ||
		verified.Kind != "service" || verified.Issuer != "auth" ||
		verified.Subject != "service:auth" || verified.Audience != "user" ||
		verified.RPC != method || verified.RequestHash != hash ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid Auth principal binding")
	}
	return nil
}

func canonicalSDKUUID(raw string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(raw)
	if err != nil || parsed == uuid.Nil || parsed.String() != raw {
		return uuid.Nil, status.Error(codes.InvalidArgument, "invalid SDK conversion identifier")
	}
	return parsed, nil
}

func (s *SdkConversionGRPC) GetSdkProfileEligibility(ctx context.Context, req *userv1.GetSdkProfileEligibilityRequest) (*userv1.GetSdkProfileEligibilityResponse, error) {
	if err := requireAuthSdkPrincipal(ctx, userv1.UserService_GetSdkProfileEligibility_FullMethodName, req); err != nil {
		return nil, err
	}
	accountID, err := canonicalSDKUUID(req.GetAccountId())
	if err != nil {
		return nil, err
	}
	profileID, err := canonicalSDKUUID(req.GetProfileId())
	if err != nil {
		return nil, err
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "SDK profile authority unavailable")
	}
	profile, err := s.Profiles.GetSdkProfileEligibility(ctx, accountID, profileID)
	if errors.Is(err, store.ErrSdkProfileMissing) {
		return nil, status.Error(codes.NotFound, "SDK profile missing")
	}
	if errors.Is(err, store.ErrSdkProfileStale) {
		return nil, status.Error(codes.FailedPrecondition, "SDK profile revision invalid")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "SDK profile authority unavailable")
	}
	return &userv1.GetSdkProfileEligibilityResponse{
		AccountId: profile.AccountID.String(), ProfileId: profile.ProfileID.String(),
		ProfileRevision: profile.Revision, Deleted: profile.Deleted, Frozen: profile.Frozen,
	}, nil
}

// sdkTombstoneHash hashes the exact frozen request body excluding request_hash.
// encoding/json sorts map keys by ASCII; all values are canonical UUIDs or
// positive integers, yielding the agreed compact JSON across Auth and User.
func sdkTombstoneHash(req *userv1.RecordSdkAuthorTombstoneRequest) (string, error) {
	body := map[string]any{
		"version":                   req.GetVersion(),
		"operation_id":              req.GetOperationId(),
		"source_account_id":         req.GetSourceAccountId(),
		"source_actor_id":           req.GetSourceActorId(),
		"target_account_id":         req.GetTargetAccountId(),
		"target_profile_id":         req.GetTargetProfileId(),
		"expected_profile_revision": req.GetExpectedProfileRevision(),
		"frozen_binding_id":         req.GetFrozenBindingId(),
		"frozen_authority_epoch":    req.GetFrozenAuthorityEpoch(),
		"freeze_receipt_id":         req.GetFreezeReceiptId(),
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (s *SdkConversionGRPC) RecordSdkAuthorTombstone(ctx context.Context, req *userv1.RecordSdkAuthorTombstoneRequest) (*userv1.RecordSdkAuthorTombstoneResponse, error) {
	if err := requireAuthSdkPrincipal(ctx, userv1.UserService_RecordSdkAuthorTombstone_FullMethodName, req); err != nil {
		return nil, err
	}
	if req.GetVersion() != 1 || req.GetExpectedProfileRevision() == 0 ||
		req.GetExpectedProfileRevision() > math.MaxInt64 ||
		req.GetFrozenAuthorityEpoch() == 0 || req.GetFrozenAuthorityEpoch() > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid SDK tombstone version or revision")
	}
	operationID, err := canonicalSDKUUID(req.GetOperationId())
	if err != nil {
		return nil, err
	}
	sourceAccountID, err := canonicalSDKUUID(req.GetSourceAccountId())
	if err != nil {
		return nil, err
	}
	sourceActorID, err := canonicalSDKUUID(req.GetSourceActorId())
	if err != nil {
		return nil, err
	}
	targetAccountID, err := canonicalSDKUUID(req.GetTargetAccountId())
	if err != nil {
		return nil, err
	}
	targetProfileID, err := canonicalSDKUUID(req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	frozenBindingID, err := canonicalSDKUUID(req.GetFrozenBindingId())
	if err != nil {
		return nil, err
	}
	freezeReceiptID, err := canonicalSDKUUID(req.GetFreezeReceiptId())
	if err != nil {
		return nil, err
	}
	if sourceAccountID == targetAccountID {
		return nil, status.Error(codes.InvalidArgument, "SDK source and target accounts must differ")
	}
	canonicalHash, err := sdkTombstoneHash(req)
	if err != nil || req.GetRequestHash() != canonicalHash || strings.ToLower(req.GetRequestHash()) != req.GetRequestHash() {
		return nil, status.Error(codes.InvalidArgument, "invalid SDK tombstone request hash")
	}
	if s == nil || s.Profiles == nil {
		return nil, status.Error(codes.Unavailable, "SDK author authority unavailable")
	}
	receipt, err := s.Profiles.RecordSdkAuthorTombstone(ctx, store.SdkAuthorTombstoneInput{
		OperationID: operationID, SourceAccountID: sourceAccountID, SourceActorID: sourceActorID,
		TargetAccountID: targetAccountID, TargetProfileID: targetProfileID,
		ExpectedProfileRevision: req.GetExpectedProfileRevision(), FrozenBindingID: frozenBindingID,
		FrozenAuthorityEpoch: req.GetFrozenAuthorityEpoch(), FreezeReceiptID: freezeReceiptID,
		RequestHash: canonicalHash,
	})
	switch {
	case errors.Is(err, store.ErrSdkProfileMissing):
		return nil, status.Error(codes.NotFound, "SDK target profile missing")
	case errors.Is(err, store.ErrSdkProfileStale):
		return nil, status.Error(codes.FailedPrecondition, "SDK target profile changed")
	case errors.Is(err, store.ErrSdkAuthorConflict):
		return nil, status.Error(codes.AlreadyExists, "SDK author receipt conflict")
	case err != nil:
		return nil, status.Error(codes.Unavailable, "SDK author authority unavailable")
	}
	return &userv1.RecordSdkAuthorTombstoneResponse{
		Version: 1, ReceiptId: receipt.ReceiptID.String(), OperationId: receipt.OperationID.String(),
		SourceAccountId: receipt.SourceAccountID.String(), SourceActorId: receipt.SourceActorID.String(),
		TargetAccountId: receipt.TargetAccountID.String(), TargetProfileId: receipt.TargetProfileID.String(),
		ProfileRevision: receipt.ProfileRevision, TombstoneRevision: receipt.TombstoneRevision,
		FrozenBindingId: receipt.FrozenBindingID.String(), FrozenAuthorityEpoch: receipt.FrozenAuthorityEpoch,
		FreezeReceiptId: receipt.FreezeReceiptID.String(), RequestHash: receipt.RequestHash,
		CommittedAt: timestamppb.New(receipt.CommittedAt),
	}, nil
}

func RegisterSdkConversionServer(server grpc.ServiceRegistrar, profiles *store.ProfileStore) {
	desc := userv1.UserService_ServiceDesc
	desc.Methods = nil
	desc.Streams = nil
	for _, method := range userv1.UserService_ServiceDesc.Methods {
		if method.MethodName == "GetSdkProfileEligibility" || method.MethodName == "RecordSdkAuthorTombstone" {
			desc.Methods = append(desc.Methods, method)
		}
	}
	server.RegisterService(&desc, &SdkConversionGRPC{Profiles: profiles})
}
