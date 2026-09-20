package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"voice/backend/pkg/correlation"
)

var protoJSONMarshal = protojson.MarshalOptions{
	EmitUnpopulated: false,
	UseProtoNames:   true,
}

var protoJSONUnmarshal = protojson.UnmarshalOptions{
	DiscardUnknown: true,
}

func grpcMetadataFromRequest(r *http.Request) metadata.MD {
	md := metadata.MD{}
	for _, key := range []string{
		"x-voice-user-id",
		"x-voice-profile-id",
		"x-voice-roles",
		"x-voice-subscription-tier",
		"x-voice-account-type",
		correlation.GRPCMetadataKey,
	} {
		if v := strings.TrimSpace(r.Header.Get(key)); v != "" {
			md.Set(key, v)
		}
	}
	return md
}

func withGRPCMetadata(ctx context.Context, r *http.Request) context.Context {
	return metadata.NewOutgoingContext(ctx, grpcMetadataFromRequest(r))
}

func readProtoJSON(r *http.Request, msg proto.Message) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid_body")
	}
	if len(body) == 0 {
		return nil
	}
	if err := protoJSONUnmarshal.Unmarshal(body, msg); err != nil {
		return status.Error(codes.InvalidArgument, "invalid_json")
	}
	return nil
}

func writeProtoJSON(w http.ResponseWriter, httpStatus int, msg proto.Message) {
	if msg == nil {
		writeJSON(w, httpStatus, map[string]any{})
		return
	}
	b, err := protoJSONMarshal.Marshal(msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "encode_failed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_, _ = w.Write(b)
}

func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error_code": "internal",
			"message":    "internal error",
		})
		return
	}
	errorCode := grpcCodeToErrorCode(st.Code())
	if domain := grpcStatusDomainErrorCode(st); domain != "" {
		errorCode = domain
	}
	httpStatus := grpcCodeToHTTP(st.Code())
	if st.Code() == codes.FailedPrecondition && errorCode == "phone_contact_sync_unavailable" {
		httpStatus = http.StatusConflict
	}
	w.Header().Set("X-Voice-GRPC-Code", st.Code().String())
	writeJSON(w, httpStatus, map[string]string{
		"error_code": errorCode,
		"message":    st.Message(),
	})
}

// writeVoiceRoomMoveError is deliberately route-local: voice-room moves use a
// conflict response for obsolete roster state, while every unrelated gRPC
// FAILED_PRECONDITION keeps the generic HTTP 412 mapping above.
func writeVoiceRoomMoveError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error_code": "internal", "message": "internal error"})
		return
	}
	w.Header().Set("X-Voice-GRPC-Code", st.Code().String())
	type moveError struct {
		status        int
		code, message string
	}
	table := map[codes.Code]moveError{
		codes.InvalidArgument:    {http.StatusBadRequest, "invalid_argument", "invalid voice room move request"},
		codes.Unauthenticated:    {http.StatusUnauthorized, "unauthenticated", "authentication required"},
		codes.PermissionDenied:   {http.StatusForbidden, "permission_denied", "voice room move not permitted"},
		codes.NotFound:           {http.StatusNotFound, "not_found", "voice room not found"},
		codes.FailedPrecondition: {http.StatusConflict, "failed_precondition", "voice room move is no longer possible"},
		codes.ResourceExhausted:  {http.StatusTooManyRequests, "resource_exhausted", "voice room is full"},
		codes.Unavailable:        {http.StatusServiceUnavailable, "unavailable", "voice room roster unavailable"},
	}
	entry, found := table[st.Code()]
	if !found {
		entry = moveError{http.StatusInternalServerError, "internal", "internal error"}
	}
	writeJSON(w, entry.status, map[string]string{"error_code": entry.code, "message": entry.message})
}

// grpcStatusDomainErrorCode returns app error keys carried in gRPC status messages
// (e.g. AuthException codes), so Gateway clients see registration_conflict instead of unauthenticated.
func grpcStatusDomainErrorCode(st *status.Status) string {
	msg := strings.TrimSpace(st.Message())
	if msg == "" {
		return ""
	}
	for i, r := range msg {
		if i == 0 {
			if r < 'a' || r > 'z' {
				return ""
			}
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return ""
	}
	return msg
}

func grpcCodeToHTTP(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument, codes.OutOfRange:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func grpcCodeToErrorCode(code codes.Code) string {
	switch code {
	case codes.InvalidArgument:
		return "invalid_argument"
	case codes.NotFound:
		return "not_found"
	case codes.AlreadyExists:
		return "already_exists"
	case codes.PermissionDenied:
		return "permission_denied"
	case codes.Unauthenticated:
		return "unauthenticated"
	case codes.FailedPrecondition:
		return "failed_precondition"
	case codes.ResourceExhausted:
		return "resource_exhausted"
	case codes.Unavailable:
		return "unavailable"
	default:
		return "internal"
	}
}

func queryFirst(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

func decodeQueryJSON(dst any, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), dst)
}
