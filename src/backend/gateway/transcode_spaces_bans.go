package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

// Include malformed mutation paths in the protected family so they cannot fall
// through to an ordinary client or REST upstream. GET ListBans is unchanged.
func isSpaceBanMutationRequest(r *http.Request) bool {
	const prefix = "/api/v1/spaces/"
	if (r.Method != http.MethodPost && r.Method != http.MethodDelete) || !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
	return len(parts) >= 2 && parts[1] == "bans"
}

func validateSpaceBanIntent(r *http.Request, operationID string) error {
	if !canonicalLifecycleUUID(operationID) {
		return status.Error(codes.InvalidArgument, "invalid moderation intent")
	}
	if values, present := r.Header[http.CanonicalHeaderKey("Idempotency-Key")]; present {
		if len(values) != 1 || !canonicalLifecycleUUID(values[0]) || values[0] != operationID {
			return status.Error(codes.InvalidArgument, "invalid moderation intent")
		}
	}
	return nil
}

// Scalar protobuf path IDs have no presence bit. Retain strict JSON decoding
// while distinguishing omitted IDs from explicitly empty/null body IDs.
func readSpaceBanJSON(r *http.Request, req proto.Message, pathIDs map[string]string) error {
	if r.Body == nil {
		return status.Error(codes.InvalidArgument, "invalid moderation request")
	}
	var body bytes.Buffer
	copy := *r
	copy.Body = io.NopCloser(io.TeeReader(r.Body, &body))
	if err := readLifecycleProtoJSON(&copy, req); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body.Bytes(), &fields); err != nil {
		return status.Error(codes.InvalidArgument, "invalid moderation request")
	}
	for name, expected := range pathIDs {
		if raw, present := fields[name]; present {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil || value != expected {
				return status.Error(codes.InvalidArgument, "invalid moderation request")
			}
		}
	}
	return nil
}

// Only this exact typed moderation reason refines FAILED_PRECONDITION to 409.
// Backend messages and unrecognized details never select a different mapping.
func writeSpaceBanError(w http.ResponseWriter, err error) {
	st := status.Convert(err)
	code := st.Code()
	if code == codes.DeadlineExceeded {
		code = codes.Unavailable
	}
	httpCode := grpcCodeToHTTP(code)
	message := map[codes.Code]string{
		codes.InvalidArgument: "invalid moderation request", codes.NotFound: "space not found",
		codes.Unauthenticated: "authentication required", codes.PermissionDenied: "moderation request not permitted",
		codes.AlreadyExists: "operation binding conflict", codes.FailedPrecondition: "moderation state conflict",
		codes.ResourceExhausted: "too many moderation requests", codes.Unavailable: "moderation service unavailable",
	}[code]
	if code == codes.FailedPrecondition {
		for _, detail := range st.Details() {
			if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "voice.space.moderation.v1" && info.Reason == "INTENT_IN_PROGRESS" {
				httpCode = http.StatusConflict
				message = "moderation operation in progress"
				break
			}
		}
	}
	if message == "" {
		message = "internal error"
	}
	w.Header().Set("X-Voice-GRPC-Code", code.String())
	writeJSON(w, httpCode, map[string]string{"error_code": grpcCodeToErrorCode(code), "message": message})
}

func (t *transcoder) serveSpacesBans(w http.ResponseWriter, r *http.Request, rest string) bool {
	// Preserve the existing ordinary ListBans route, including its trailing slash.
	if r.Method == http.MethodGet {
		if !strings.Contains(rest, "/bans") {
			return false
		}
		parts := strings.SplitN(rest, "/bans", 2)
		if strings.TrimPrefix(parts[1], "/") != "" {
			return false
		}
		ctx := withGRPCMetadata(r.Context(), r)
		resp, err := t.clients.space.ListBans(ctx, &spacev1.ListBansRequest{SpaceId: parts[0]})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true
	}
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		return false
	}
	parts := strings.Split(rest, "/")
	if len(parts) < 2 || parts[1] != "bans" {
		return false
	}
	spaceID := parts[0]
	bad := func() { writeSpaceBanError(w, status.Error(codes.InvalidArgument, "invalid moderation request")) }
	if !canonicalLifecycleUUID(spaceID) || (r.Method == http.MethodPost && len(parts) != 2) || (r.Method == http.MethodDelete && (len(parts) != 3 || !canonicalLifecycleUUID(parts[2]))) {
		bad()
		return true
	}

	switch r.Method {
	case http.MethodPost:
		req := &spacev1.BanMemberRequest{}
		if err := readSpaceBanJSON(r, req, map[string]string{"space_id": spaceID, "spaceId": spaceID}); err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		if (req.SpaceId != "" && req.SpaceId != spaceID) || !canonicalLifecycleUUID(req.AccountId) || (req.ProfileId != nil && !canonicalLifecycleUUID(*req.ProfileId)) {
			bad()
			return true
		}
		if err := validateSpaceBanIntent(r, req.OperationId); err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		req.SpaceId = spaceID
		ctx, err := t.spaceJoinContext(r, req, spacev1.SpaceService_BanMember_FullMethodName)
		if err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		_, err = t.clients.spaceLifecycle.BanMember(ctx, req)
		if err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	case http.MethodDelete:
		req := &spacev1.UnbanMemberRequest{}
		if err := readSpaceBanJSON(r, req, map[string]string{"space_id": spaceID, "spaceId": spaceID, "account_id": parts[2], "accountId": parts[2]}); err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		if (req.SpaceId != "" && req.SpaceId != spaceID) || (req.AccountId != "" && req.AccountId != parts[2]) {
			bad()
			return true
		}
		if err := validateSpaceBanIntent(r, req.OperationId); err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		req.SpaceId, req.AccountId = spaceID, parts[2]
		ctx, err := t.spaceJoinContext(r, req, spacev1.SpaceService_UnbanMember_FullMethodName)
		if err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		_, err = t.clients.spaceLifecycle.UnbanMember(ctx, req)
		if err != nil {
			writeSpaceBanError(w, err)
			return true
		}
		w.WriteHeader(http.StatusNoContent)
		return true

	default:
		return false
	}
}
