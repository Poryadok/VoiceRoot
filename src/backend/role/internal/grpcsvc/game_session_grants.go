package grpcsvc

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func gameGrantPrincipal(ctx context.Context, rpc, issuer, subject, requestID string, request proto.Message) ([32]byte, error) {
	var zero [32]byte
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != issuer || verified.Subject != subject ||
		verified.Audience != "role" || verified.RPC != rpc || verified.RequestID == "" || verified.RequestID != requestID {
		return zero, status.Error(codes.PermissionDenied, "exact verified service principal required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return zero, status.Error(codes.InvalidArgument, "invalid grant request")
	}
	if verified.RequestHash != hash {
		return zero, status.Error(codes.PermissionDenied, "verified request binding required")
	}
	requestHash, err := decodeRequestSHA256(hash)
	if err != nil {
		return zero, status.Error(codes.InvalidArgument, "invalid grant request hash")
	}
	return requestHash, nil
}

func decodeRequestSHA256(raw string) ([32]byte, error) {
	var out [32]byte
	hexDigest := strings.TrimPrefix(raw, "sha256:")
	if len(hexDigest) != 64 || raw != "sha256:"+hexDigest {
		return out, errors.New("invalid request hash")
	}
	decoded, err := hex.DecodeString(hexDigest)
	if err != nil || hex.EncodeToString(decoded) != hexDigest {
		return out, errors.New("invalid request hash")
	}
	copy(out[:], decoded)
	return out, nil
}

func parseCanonicalGrantUUID(field, raw string) (uuid.UUID, error) {
	id, err := parseUUIDField(field, raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid canonical %s", field)
	}
	return id, nil
}

func gameSessionGrantStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrGameSessionGrantInvalid):
		return status.Error(codes.InvalidArgument, "invalid game-session grant request")
	case errors.Is(err, store.ErrGameSessionGrantConflict):
		return status.Error(codes.AlreadyExists, "game-session grant operation conflicts with recorded request")
	case errors.Is(err, store.ErrGameSessionGrantRevoked):
		return status.Error(codes.FailedPrecondition, "game-session grants are terminally revoked")
	default:
		return status.Error(codes.Unavailable, "game-session grant persistence unavailable")
	}
}

func (s *RoleGRPC) ApplyGameSessionGrants(ctx context.Context, request *rolev1.ApplyGameSessionGrantsRequest) (*rolev1.ApplyGameSessionGrantsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "grant request required")
	}
	applicationID, err := parseCanonicalGrantUUID("application_id", request.ApplicationId)
	if err != nil {
		return nil, err
	}
	environmentID, err := parseCanonicalGrantUUID("environment_id", request.EnvironmentId)
	if err != nil {
		return nil, err
	}
	sessionID, err := parseCanonicalGrantUUID("session_id", request.SessionId)
	if err != nil {
		return nil, err
	}
	voiceRoomID, err := parseCanonicalGrantUUID("voice_room_id", request.VoiceRoomId)
	if err != nil {
		return nil, err
	}
	operationID, err := parseCanonicalGrantUUID("operation_id", request.OperationId)
	if err != nil {
		return nil, err
	}
	if request.RosterRevision == 0 || request.RosterRevision > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid roster_revision")
	}
	profiles := make([]uuid.UUID, len(request.ProfileIds))
	for index, raw := range request.ProfileIds {
		profiles[index], err = parseCanonicalGrantUUID("profile_id", raw)
		if err != nil {
			return nil, err
		}
		if index > 0 && request.ProfileIds[index-1] >= raw {
			return nil, status.Error(codes.InvalidArgument, "profile_ids must be sorted and unique")
		}
	}
	requestHash, err := gameGrantPrincipal(ctx, rolev1.RoleService_ApplyGameSessionGrants_FullMethodName,
		"gameintegration", "service:gameintegration", operationID.String(), request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "game-session grant persistence unavailable")
	}
	receipt, err := s.Store.ApplyGameSessionGrants(ctx, store.GameSessionGrantApply{
		ApplicationID: applicationID, EnvironmentID: environmentID, SessionID: sessionID,
		VoiceRoomID: voiceRoomID, OperationID: operationID, RosterRevision: int64(request.RosterRevision),
		ProfileIDs: profiles, RequestSHA256: requestHash,
	})
	if err != nil {
		if errors.Is(err, store.ErrGameSessionGrantConflict) {
			return nil, annotateGameGrantOwnerRejection(gameSessionGrantStoreError(err), request, gisowner.CategoryOperationConflict)
		}
		if errors.Is(err, store.ErrGameSessionGrantRevoked) {
			return nil, annotateGameGrantOwnerRejection(gameSessionGrantStoreError(err), request, gisowner.CategoryTerminalRevoked)
		}
		return nil, gameSessionGrantStoreError(err)
	}
	return &rolev1.ApplyGameSessionGrantsResponse{Receipt: gameSessionGrantReceiptToProto(receipt)}, nil
}

func annotateGameGrantOwnerRejection(err error, request proto.Message, category string) error {
	apply, ok := request.(*rolev1.ApplyGameSessionGrantsRequest)
	if !ok {
		return err
	}
	hash, hashErr := principal.RequestHash(request)
	if hashErr != nil {
		return err
	}
	return gisowner.Annotate(err, gisowner.RoleDomain, gisowner.RoleApplyRPC, category, apply.GetOperationId(), hash)
}

func (s *RoleGRPC) RevokeGameSessionGrants(ctx context.Context, request *rolev1.RevokeGameSessionGrantsRequest) (*rolev1.RevokeGameSessionGrantsResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "revoke request required")
	}
	applicationID, err := parseCanonicalGrantUUID("application_id", request.ApplicationId)
	if err != nil {
		return nil, err
	}
	environmentID, err := parseCanonicalGrantUUID("environment_id", request.EnvironmentId)
	if err != nil {
		return nil, err
	}
	sessionID, err := parseCanonicalGrantUUID("session_id", request.SessionId)
	if err != nil {
		return nil, err
	}
	operationID, err := parseCanonicalGrantUUID("operation_id", request.OperationId)
	if err != nil {
		return nil, err
	}
	requestHash, err := gameGrantPrincipal(ctx, rolev1.RoleService_RevokeGameSessionGrants_FullMethodName,
		"gameintegration", "service:gameintegration", operationID.String(), request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "game-session grant persistence unavailable")
	}
	receipt, err := s.Store.RevokeGameSessionGrants(ctx, applicationID, environmentID, sessionID, operationID, requestHash)
	if err != nil {
		return nil, gameSessionGrantStoreError(err)
	}
	return &rolev1.RevokeGameSessionGrantsResponse{Receipt: gameSessionGrantReceiptToProto(receipt)}, nil
}

func (s *RoleGRPC) CheckGameSessionGrant(ctx context.Context, request *rolev1.CheckGameSessionGrantRequest) (*rolev1.CheckGameSessionGrantResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "check request required")
	}
	applicationID, err := parseCanonicalGrantUUID("application_id", request.ApplicationId)
	if err != nil {
		return nil, err
	}
	environmentID, err := parseCanonicalGrantUUID("environment_id", request.EnvironmentId)
	if err != nil {
		return nil, err
	}
	sessionID, err := parseCanonicalGrantUUID("session_id", request.SessionId)
	if err != nil {
		return nil, err
	}
	voiceRoomID, err := parseCanonicalGrantUUID("voice_room_id", request.VoiceRoomId)
	if err != nil {
		return nil, err
	}
	profileID, err := parseCanonicalGrantUUID("profile_id", request.ProfileId)
	if err != nil {
		return nil, err
	}
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "voice" || verified.Subject != "service:voice" ||
		verified.Audience != "role" || verified.RPC != rolev1.RoleService_CheckGameSessionGrant_FullMethodName || verified.RequestID == "" {
		return nil, status.Error(codes.PermissionDenied, "exact verified Voice principal required")
	}
	if _, err := parseCanonicalGrantUUID("request_id", verified.RequestID); err != nil {
		return nil, status.Error(codes.PermissionDenied, "fresh canonical Voice request ID required")
	}
	requestHash, err := gameGrantPrincipal(ctx, rolev1.RoleService_CheckGameSessionGrant_FullMethodName,
		"voice", "service:voice", verified.RequestID, request)
	if err != nil {
		return nil, err
	}
	_ = requestHash // Check has no durable operation receipt; the signed hash still binds every input.
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "game-session grant persistence unavailable")
	}
	allowed, err := s.Store.CheckGameSessionGrant(ctx, applicationID, environmentID, sessionID, voiceRoomID, profileID)
	if err != nil {
		return nil, gameSessionGrantStoreError(err)
	}
	return &rolev1.CheckGameSessionGrantResponse{Allowed: allowed}, nil
}

func gameSessionGrantReceiptToProto(receipt store.GameSessionGrantReceipt) *rolev1.GameSessionGrantReceipt {
	outcomes := map[store.GameSessionGrantOutcome]rolev1.GameSessionGrantOutcome{
		store.GameSessionGrantApplied: rolev1.GameSessionGrantOutcome_GAME_SESSION_GRANT_OUTCOME_APPLIED,
		store.GameSessionGrantReplay:  rolev1.GameSessionGrantOutcome_GAME_SESSION_GRANT_OUTCOME_REPLAYED,
		store.GameSessionGrantStale:   rolev1.GameSessionGrantOutcome_GAME_SESSION_GRANT_OUTCOME_STALE,
		store.GameSessionGrantRevoked: rolev1.GameSessionGrantOutcome_GAME_SESSION_GRANT_OUTCOME_REVOKED,
	}
	return &rolev1.GameSessionGrantReceipt{
		ReceiptId: receipt.ReceiptID.String(), OperationId: receipt.OperationID.String(),
		ApplicationId: receipt.ApplicationID.String(), EnvironmentId: receipt.EnvironmentID.String(), SessionId: receipt.SessionID.String(),
		RequestSha256: append([]byte(nil), receipt.RequestSHA256[:]...), RosterRevision: uint64(receipt.RosterRevision),
		AppliedProfileSetSha256: append([]byte(nil), receipt.AppliedProfileSetSHA256[:]...), Outcome: outcomes[receipt.Outcome],
	}
}
