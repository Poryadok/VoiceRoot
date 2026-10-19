package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

// This is the existing protected Matchmaking transport profile. An explicitly
// pre-cut Gateway with neither profile dependency still uses its ordinary MM
// client; a partially present protected composition never falls back to it.
func (t *transcoder) protectedPublicMatchmaking() bool {
	return t.matchmakingCompleteIssuer != nil || t.clients.matchmakingComplete != nil
}

func readPublicMatchmakingJSON(r *http.Request, msg proto.Message, decision bool) error {
	const max = 4 << 20
	raw, err := io.ReadAll(io.LimitReader(r.Body, max+1))
	if err != nil || len(raw) == 0 || len(raw) > max {
		return status.Error(codes.InvalidArgument, "invalid matchmaking request")
	}
	// protojson rejects duplicate fields (including snake/camel aliases), unknown
	// fields and trailing data. In proto3 false has no presence, so require the
	// actual decision field separately before accepting a terminal DECLINE.
	if err = (protojson.UnmarshalOptions{}).Unmarshal(raw, msg); err != nil {
		return status.Error(codes.InvalidArgument, "invalid matchmaking request")
	}
	if decision {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields["accept"] == nil || string(fields["accept"]) == "null" {
			return status.Error(codes.InvalidArgument, "match decision required")
		}
	}
	return nil
}

func (t *transcoder) publicMatchmakingContext(r *http.Request, req proto.Message, method, operation string) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "verified matchmaking actor required")
	}
	if claims.AuthenticatedAccountType != "regular" && claims.AuthenticatedAccountType != "guest" {
		return nil, status.Error(codes.PermissionDenied, "explicit human matchmaking actor required")
	}
	if !canonicalLifecycleUUID(operation) {
		return nil, status.Error(codes.InvalidArgument, "invalid matchmaking operation")
	}
	if t.matchmakingCompleteIssuer == nil || t.clients.matchmakingComplete == nil {
		return nil, status.Error(codes.Unavailable, "protected matchmaking transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid matchmaking request")
	}
	token, err := t.matchmakingCompleteIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: method, RequestID: operation, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: claims.AuthenticatedAccountType, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "matchmaking credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", operation)), nil
}

func (t *transcoder) serveProtectedMatchmakingSearch(w http.ResponseWriter, r *http.Request) bool {
	req := &matchmakingv1.StartSearchRequest{}
	if err := readPublicMatchmakingJSON(r, req, false); err != nil {
		writeGRPCError(w, err)
		return true
	}
	ctx, err := t.publicMatchmakingContext(r, req, matchmakingv1.MatchmakingService_StartSearch_FullMethodName, req.OperationId)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	resp, err := t.clients.matchmakingComplete.StartSearch(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
}

func (t *transcoder) serveProtectedMatchmakingResponse(w http.ResponseWriter, r *http.Request, matchID string) bool {
	req := &matchmakingv1.RespondToMatchRequest{}
	if err := readPublicMatchmakingJSON(r, req, true); err != nil {
		writeGRPCError(w, err)
		return true
	}
	if !canonicalLifecycleUUID(matchID) || (req.MatchId != "" && req.MatchId != matchID) {
		writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid match route"))
		return true
	}
	req.MatchId = matchID // Hash the final canonical request, including URL identity.
	ctx, err := t.publicMatchmakingContext(r, req, matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName, req.OperationId)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	resp, err := t.clients.matchmakingComplete.RespondToMatch(ctx, req)
	if err != nil {
		writeGRPCError(w, err)
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
}
