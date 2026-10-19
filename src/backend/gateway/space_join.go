package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func isSpaceJoinRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	for _, prefix := range []string{"/api/v1/spaces/", "/api/v1/invites/"} {
		if strings.HasPrefix(r.URL.Path, prefix) {
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
			return len(parts) == 2 && parts[0] != "" && parts[1] == "join"
		}
	}
	return false
}

func (t *transcoder) spaceJoinContext(r *http.Request, req proto.Message, method string) (context.Context, error) {
	claims, ok := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	if !ok || !canonicalLifecycleUUID(claims.UserID) || !canonicalLifecycleUUID(claims.ProfileID) || claims.SessionEpoch <= 0 || !claims.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "verified admission actor required")
	}
	if claims.AuthenticatedAccountType != "regular" && claims.AuthenticatedAccountType != "guest" {
		return nil, status.Error(codes.PermissionDenied, "explicit account type required")
	}
	if t.lifecycleIssuer == nil || t.clients.spaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "Space admission transport unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid admission request")
	}
	requestID := uuid.NewString()
	token, err := t.lifecycleIssuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: method, RequestID: requestID, RequestHash: hash, AccountID: claims.UserID, ProfileID: claims.ProfileID, SessionEpoch: claims.SessionEpoch, AccountType: claims.AuthenticatedAccountType, ClientExpiresAt: claims.ExpiresAt})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "admission credential unavailable")
	}
	return metadata.NewOutgoingContext(r.Context(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID)), nil
}

// bindJoinOperation never accepts an unsigned operation metadata header. A wire
// revision without the target field must fail closed instead of losing the key.
func bindJoinOperation(r *http.Request, req proto.Message) error {
	values, present := r.Header[http.CanonicalHeaderKey("Idempotency-Key")]
	if !present {
		return nil
	}
	if len(values) != 1 {
		return status.Error(codes.InvalidArgument, "invalid idempotency key")
	}
	id, err := uuid.Parse(values[0])
	if err != nil || id == uuid.Nil || strings.TrimSpace(values[0]) != values[0] {
		return status.Error(codes.InvalidArgument, "invalid idempotency key")
	}
	field := req.ProtoReflect().Descriptor().Fields().ByName("operation_id")
	if field == nil || field.Kind() != protoreflect.StringKind || !field.HasPresence() {
		return status.Error(codes.Unavailable, "Space admission operation transport unavailable")
	}
	req.ProtoReflect().Set(field, protoreflect.ValueOfString(id.String()))
	return nil
}

func (t *transcoder) serveSpaceJoin(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(body) != 0 {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "empty body required"))
			return
		}
	}
	var req proto.Message
	method := spacev1.SpaceService_JoinSpace_FullMethodName
	if strings.HasPrefix(r.URL.Path, "/api/v1/invites/") {
		req = &spacev1.JoinByInviteRequest{Code: strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/invites/"), "/join")}
		method = spacev1.SpaceService_JoinByInvite_FullMethodName
	} else {
		spaceID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/spaces/"), "/join")
		if !canonicalLifecycleUUID(spaceID) {
			writeGRPCError(w, status.Error(codes.InvalidArgument, "invalid space ID"))
			return
		}
		req = &spacev1.JoinSpaceRequest{SpaceId: spaceID}
	}
	if err := bindJoinOperation(r, req); err != nil {
		writeGRPCError(w, err)
		return
	}
	ctx, err := t.spaceJoinContext(r, req, method)
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	// One protected dispatch. Space owns eligibility, policy and durable outcome;
	// an ambiguous response is never retried or downgraded to an ordinary client.
	var resp proto.Message
	if direct, ok := req.(*spacev1.JoinSpaceRequest); ok {
		resp, err = t.clients.spaceLifecycle.JoinSpace(ctx, direct)
	} else {
		resp, err = t.clients.spaceLifecycle.JoinByInvite(ctx, req.(*spacev1.JoinByInviteRequest))
	}
	if err != nil {
		writeGRPCError(w, err)
		return
	}
	writeProtoJSON(w, http.StatusOK, resp)
}
