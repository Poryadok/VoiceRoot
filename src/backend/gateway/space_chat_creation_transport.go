package main

import (
	"context"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func isSpaceChatCreationRequest(r *http.Request) bool {
	if r == nil || r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/api/v1/spaces/") {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/"), "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] == "chats"
}

func (t *transcoder) compositeOutcomeContext(r *http.Request, request *spacev1.GetSpaceChatCreationOutcomeRequest) (context.Context, error) {
	if r == nil || request == nil || !canonicalLifecycleUUID(request.SpaceId) || !canonicalLifecycleUUID(request.OperationId) {
		return nil, status.Error(codes.InvalidArgument, "invalid composite lookup")
	}
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	accountType := claims.AuthenticatedAccountType
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) || (accountType != "regular" && accountType != "guest") {
		return nil, status.Error(codes.Unauthenticated, "verified explicit composite actor required")
	}
	if t == nil || t.lifecycleIssuer == nil || t.clients.spaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "protected Space outcome transport unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid composite lookup")
	}
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, RequestID: request.OperationId, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: accountType, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "composite outcome credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId)), nil
}
