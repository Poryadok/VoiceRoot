package main

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func isSpaceCreateRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && (r.URL.Path == "/api/v1/spaces" || r.URL.Path == "/api/v1/spaces/")
}

func (t *transcoder) createSpaceContext(r *http.Request, req *spacev1.CreateSpaceRequest) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "verified creator required")
	}
	if claims.AuthenticatedAccountType != "regular" {
		return nil, status.Error(codes.PermissionDenied, "explicit regular account required")
	}
	if t.lifecycleIssuer == nil || t.clients.spaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Space creator transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid creator request")
	}
	requestID := uuid.NewString()
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{
		Audience: "space", RPC: spacev1.SpaceService_CreateSpace_FullMethodName,
		RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID,
		SessionEpoch: claims.SessionEpoch, AccountType: "regular", ClientExpiresAt: claims.ExpiresAt,
	})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "creator credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

func (t *transcoder) serveSpaceCreate(w http.ResponseWriter, r *http.Request) {
	req := &spacev1.CreateSpaceRequest{}
	// Retain the existing body bound while rejecting unknown/duplicate proto
	// fields: a discarded owner/actor field cannot become signed authority.
	if r.Body == nil {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid_body"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (4<<20)+1))
	if err != nil || len(body) > 4<<20 || (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(body, req) != nil {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid_json"))
		return
	}
	ctx, err := t.createSpaceContext(r, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	// One dispatch only. An ambiguous result is never a fresh-token retry of
	// this non-idempotent POST, nor a downgrade to the ordinary client.
	resp, err := t.clients.spaceLifecycle.CreateSpace(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProtoJSON(w, http.StatusOK, resp)
}
