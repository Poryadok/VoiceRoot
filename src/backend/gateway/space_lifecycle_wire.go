package main

import (
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
)

// Lifecycle intents reject unknown fields and duplicate protobuf aliases.
func readLifecycleProtoJSON(r *http.Request, req proto.Message) error {
	if r.URL.RawQuery != "" || r.Body == nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if err != nil || len(body) == 0 || len(body) > 64<<10 {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(body, req); err != nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	return nil
}

func canonicalLifecycleUUID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

// Never forward factor distinctions, proof content or internal service errors.
func writeSpaceLifecycleError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	httpCode := grpcCodeToHTTP(code)
	if code == codes.FailedPrecondition {
		httpCode = http.StatusConflict
	}
	if code == codes.DeadlineExceeded {
		code = codes.Unavailable
		httpCode = http.StatusServiceUnavailable
	}
	message := map[codes.Code]string{
		codes.InvalidArgument: "invalid lifecycle request", codes.NotFound: "space not found",
		codes.Unauthenticated: "authentication required", codes.PermissionDenied: "lifecycle request not permitted",
		codes.AlreadyExists: "operation binding conflict", codes.FailedPrecondition: "lifecycle state conflict",
		codes.ResourceExhausted: "too many lifecycle requests", codes.Unavailable: "lifecycle service unavailable",
	}[code]
	if message == "" {
		message = "internal error"
	}
	writeJSON(w, httpCode, map[string]string{"error_code": grpcCodeToErrorCode(code), "message": message})
}

func (t *transcoder) serveSpaceDeletionProof(w http.ResponseWriter, r *http.Request) bool {
	if strings.TrimSpace(r.Header.Get("X-Voice-Account-Type")) != "regular" {
		writeSpaceLifecycleError(w, status.Error(codes.PermissionDenied, "regular account required"))
		return true
	}
	req := &authv1.IssueSpaceDeletionProofRequest{}
	if err := readLifecycleProtoJSON(r, req); err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	if !canonicalLifecycleUUID(req.SpaceId) || !canonicalLifecycleUUID(req.OperationId) || req.ConfirmationName == "" || req.Password == "" || (req.TotpCode != "" && req.BackupCode != "") {
		writeSpaceLifecycleError(w, status.Error(codes.InvalidArgument, "invalid proof intent"))
		return true
	}
	if t.clients.auth == nil {
		writeSpaceLifecycleError(w, status.Error(codes.Unavailable, "Auth unavailable"))
		return true
	}
	resp, err := t.clients.auth.IssueSpaceDeletionProof(authGRPCContext(withGRPCMetadata(r.Context(), r), r), req)
	if err != nil {
		writeSpaceLifecycleError(w, err)
		return true
	}
	if resp == nil || resp.Proof == "" || resp.ExpiresAt == nil || resp.ExpiresAt.CheckValid() != nil {
		writeSpaceLifecycleError(w, status.Error(codes.Unavailable, "invalid Auth response"))
		return true
	}
	writeProtoJSON(w, http.StatusOK, resp)
	return true
}
