package main

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

func spaceAdmissionRoute(r *http.Request) (kind, space, child string) {
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		return "", "", ""
	}
	p := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/"), "/")
	if len(p) == 3 && p[0] == "invites" && p[1] != "" && p[2] == "join-attempts" && r.Method == http.MethodPost {
		return "invite-start", "", p[1]
	}
	if len(p) < 3 || p[0] != "spaces" || p[1] == "" {
		return "", "", ""
	}
	space = p[1]
	switch {
	case len(p) == 3 && p[2] == "entry-policy" && r.Method == http.MethodPut:
		return "policy", space, ""
	case len(p) == 3 && p[2] == "join-attempts" && r.Method == http.MethodPost:
		return "start", space, ""
	case len(p) == 4 && p[2] == "join-attempts" && p[3] != "" && r.Method == http.MethodGet:
		return "attempt", space, p[3]
	case len(p) == 5 && p[2] == "join-attempts" && p[3] != "" && p[4] == "submit" && r.Method == http.MethodPost:
		return "submit", space, p[3]
	case len(p) == 3 && p[2] == "join-requests" && r.Method == http.MethodGet:
		return "requests", space, ""
	case len(p) == 4 && p[2] == "join-requests" && p[3] != "" && r.Method == http.MethodGet:
		return "request", space, p[3]
	case len(p) == 5 && p[2] == "join-requests" && p[3] != "" && p[4] == "approve" && r.Method == http.MethodPost:
		return "approve", space, p[3]
	default:
		return "", "", ""
	}
}
func isSpaceAdmissionRequest(r *http.Request) bool {
	kind, _, _ := spaceAdmissionRoute(r)
	return kind != ""
}

func writeSpaceAdmissionError(w http.ResponseWriter, err error) {
	code := status.Code(err)
	if code == codes.DeadlineExceeded || code == codes.Unimplemented {
		code = codes.Unavailable
	}
	statusCode := grpcCodeToHTTP(code)
	if code == codes.FailedPrecondition {
		statusCode = http.StatusConflict
	}
	message := map[codes.Code]string{codes.InvalidArgument: "invalid admission request", codes.Unauthenticated: "authentication required", codes.PermissionDenied: "admission denied", codes.NotFound: "join request not found", codes.AlreadyExists: "operation binding conflict", codes.FailedPrecondition: "admission state conflict", codes.ResourceExhausted: "admission limit reached", codes.Unavailable: "admission unavailable"}[code]
	if message == "" {
		message = "internal error"
	}
	writeJSON(w, statusCode, map[string]string{"error_code": grpcCodeToErrorCode(code), "message": message})
}

func emptyAdmissionRead(r *http.Request, query bool) error {
	if !query && r.URL.RawQuery != "" {
		return status.Error(codes.InvalidArgument, "unexpected query")
	}
	if r.Body != nil {
		b, err := io.ReadAll(io.LimitReader(r.Body, 1))
		if err != nil || len(b) != 0 {
			return status.Error(codes.InvalidArgument, "empty body required")
		}
	}
	return nil
}

func readEntryPolicyREST(r *http.Request) (*spacev1.SetEntryPolicyRequest, error) {
	bad := status.Error(codes.InvalidArgument, "invalid settings request")
	if r.Body == nil || r.URL.RawQuery != "" {
		return nil, bad
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if err != nil || len(raw) == 0 || len(raw) > 64<<10 {
		return nil, bad
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, bad
	}
	fields := map[string]json.RawMessage{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return nil, bad
		}
		key, ok := token.(string)
		if !ok || fields[key] != nil {
			return nil, bad
		}
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, bad
		}
		fields[key] = value
	}
	if _, err = d.Token(); err != nil {
		return nil, bad
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return nil, bad
	}
	if len(fields) != 3 || fields["operation_id"] == nil || fields["expected_policy_version"] == nil || fields["policy"] == nil {
		return nil, bad
	}
	policy := bytes.TrimSpace(fields["policy"])
	if len(policy) == 0 || policy[0] != '{' {
		return nil, bad
	}
	encodedPolicy, err := json.Marshal([]byte(policy))
	if err != nil {
		return nil, bad
	}
	wire, err := json.Marshal(map[string]json.RawMessage{"operation_id": fields["operation_id"], "expected_policy_version": fields["expected_policy_version"], "private_policy_json": encodedPolicy})
	if err != nil {
		return nil, bad
	}
	var req spacev1.SetEntryPolicyRequest
	if (protojson.UnmarshalOptions{DiscardUnknown: false}).Unmarshal(wire, &req) != nil || !canonicalLifecycleUUID(req.OperationId) || req.ExpectedPolicyVersion == 0 || req.ExpectedPolicyVersion > math.MaxInt64 {
		return nil, bad
	}
	return &req, nil
}

func admissionStateName(state spacev1.JoinAdmissionState) (string, bool) {
	switch state {
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING:
		return "collecting", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING:
		return "pending", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PROCESSING:
		return "processing", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED:
		return "admitted", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_FAILED:
		return "failed", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_EXPIRED:
		return "expired", true
	case spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_INVALIDATED:
		return "invalidated", true
	default:
		return "", false
	}
}
func validAdmissionResult(result *spacev1.JoinAdmissionResult, space, profile string) bool {
	if result == nil {
		return false
	}
	if _, ok := admissionStateName(result.State); !ok {
		return false
	}
	if result.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED {
		return result.SpaceMembership != nil && result.SpaceMembership.SpaceId == space && canonicalLifecycleUUID(result.SpaceMembership.ProfileId) && (profile == "" || result.SpaceMembership.ProfileId == profile) && result.SpaceMembership.JoinedAt != nil && result.SpaceMembership.JoinedAt.IsValid() && result.RequestId == ""
	}
	if result.SpaceMembership != nil {
		return false
	}
	if result.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING {
		return canonicalLifecycleUUID(result.RequestId)
	}
	return result.RequestId == "" || canonicalLifecycleUUID(result.RequestId)
}

// REST uses documented finite state names. The generated DTOs contain no private
// policy/answer digest fields; structured protobuf timestamps remain RFC3339.
func writeAdmissionProto(w http.ResponseWriter, httpCode int, message proto.Message) {
	raw, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(message)
	if err != nil {
		writeSpaceAdmissionError(w, status.Error(codes.Unavailable, "invalid response"))
		return
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		writeSpaceAdmissionError(w, status.Error(codes.Unavailable, "invalid response"))
		return
	}
	var convert func(any) bool
	convert = func(v any) bool {
		switch item := v.(type) {
		case map[string]any:
			for key, child := range item {
				if key == "state" {
					name, ok := child.(string)
					if !ok {
						return false
					}
					n, ok := spacev1.JoinAdmissionState_value[name]
					if !ok {
						return false
					}
					state, ok := admissionStateName(spacev1.JoinAdmissionState(n))
					if !ok {
						return false
					}
					item[key] = state
				} else if !convert(child) {
					return false
				}
			}
		case []any:
			for _, child := range item {
				if !convert(child) {
					return false
				}
			}
		}
		return true
	}
	if !convert(value) {
		writeSpaceAdmissionError(w, status.Error(codes.Unavailable, "invalid response"))
		return
	}
	writeJSON(w, httpCode, value)
}

func (t *transcoder) serveSpaceAdmission(w http.ResponseWriter, r *http.Request) {
	bad := func() { writeSpaceAdmissionError(w, status.Error(codes.InvalidArgument, "invalid admission request")) }
	kind, space, child := spaceAdmissionRoute(r)
	if kind == "" || (kind != "invite-start" && !canonicalLifecycleUUID(space)) {
		bad()
		return
	}
	if (kind == "attempt" || kind == "submit" || kind == "request" || kind == "approve") && !canonicalLifecycleUUID(child) {
		bad()
		return
	}
	var req proto.Message
	var method string
	switch kind {
	case "start", "invite-start":
		v := &spacev1.StartJoinAttemptRequest{}
		if readLifecycleProtoJSON(r, v) != nil || v.Target != nil || !canonicalLifecycleUUID(v.OperationId) {
			bad()
			return
		}
		if kind == "start" {
			v.Target = &spacev1.StartJoinAttemptRequest_SpaceId{SpaceId: space}
		} else {
			v.Target = &spacev1.StartJoinAttemptRequest_InviteCode{InviteCode: child}
		}
		req = v
		method = spacev1.SpaceService_StartJoinAttempt_FullMethodName
	case "attempt":
		if emptyAdmissionRead(r, false) != nil {
			bad()
			return
		}
		req = &spacev1.GetJoinAttemptRequest{SpaceId: space, AttemptId: child}
		method = spacev1.SpaceService_GetJoinAttempt_FullMethodName
	case "submit":
		v := &spacev1.SubmitJoinAttemptRequest{}
		if readLifecycleProtoJSON(r, v) != nil || v.SpaceId != "" || v.AttemptId != "" || !canonicalLifecycleUUID(v.OperationId) {
			bad()
			return
		}
		v.SpaceId = space
		v.AttemptId = child
		req = v
		method = spacev1.SpaceService_SubmitJoinAttempt_FullMethodName
	case "requests":
		if emptyAdmissionRead(r, true) != nil {
			bad()
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			bad()
			return
		}
		for key, values := range query {
			if (key != "limit" && key != "cursor") || len(values) != 1 {
				bad()
				return
			}
		}
		limit := uint64(0)
		if values, ok := query["limit"]; ok {
			if values[0] == "" {
				bad()
				return
			}
			for _, b := range values[0] {
				if b < '0' || b > '9' {
					bad()
					return
				}
			}
			limit, err = strconv.ParseUint(values[0], 10, 32)
			if err != nil || limit > 100 {
				bad()
				return
			}
		}
		req = &spacev1.ListJoinRequestsRequest{SpaceId: space, Limit: uint32(limit), Cursor: query.Get("cursor")}
		method = spacev1.SpaceService_ListJoinRequests_FullMethodName
	case "request":
		if emptyAdmissionRead(r, false) != nil {
			bad()
			return
		}
		req = &spacev1.GetJoinRequestRequest{SpaceId: space, RequestId: child}
		method = spacev1.SpaceService_GetJoinRequest_FullMethodName
	case "approve":
		v := &spacev1.ApproveJoinRequestRequest{}
		if readLifecycleProtoJSON(r, v) != nil || v.SpaceId != "" || v.RequestId != "" || !canonicalLifecycleUUID(v.OperationId) || (v.AttemptId != nil && !canonicalLifecycleUUID(*v.AttemptId)) {
			bad()
			return
		}
		v.SpaceId = space
		v.RequestId = child
		req = v
		method = spacev1.SpaceService_ApproveJoinRequest_FullMethodName
	case "policy":
		v, err := readEntryPolicyREST(r)
		if err != nil {
			bad()
			return
		}
		v.SpaceId = space
		req = v
		method = spacev1.SpaceService_SetEntryPolicy_FullMethodName
	}
	ctx, err := t.spaceJoinContext(r, req, method)
	if err != nil {
		writeSpaceAdmissionError(w, err)
		return
	}
	claims, _ := r.Context().Value(verifiedUserClaimsKey{}).(tokenClaims)
	invalid := func() { writeSpaceAdmissionError(w, status.Error(codes.Unavailable, "invalid admission response")) }
	switch request := req.(type) {
	case *spacev1.StartJoinAttemptRequest:
		response, err := t.clients.spaceLifecycle.StartJoinAttempt(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || response.Attempt == nil {
			invalid()
			return
		}
		v := response.Attempt
		if !canonicalLifecycleUUID(v.AttemptId) || !canonicalLifecycleUUID(v.SpaceId) || (space != "" && v.SpaceId != space) || v.PolicyVersion <= 0 || v.ExpiresAt == nil || !v.ExpiresAt.IsValid() || v.Requirements == nil || !validAdmissionResult(v.Result, v.SpaceId, claims.ProfileID) || v.Result.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED {
			invalid()
			return
		}
		writeAdmissionProto(w, http.StatusCreated, v)
	case *spacev1.GetJoinAttemptRequest:
		response, err := t.clients.spaceLifecycle.GetJoinAttempt(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || response.Attempt == nil {
			invalid()
			return
		}
		v := response.Attempt
		if v.AttemptId != child || v.SpaceId != space || v.PolicyVersion <= 0 || v.ExpiresAt == nil || !v.ExpiresAt.IsValid() || v.Requirements == nil || !validAdmissionResult(v.Result, space, claims.ProfileID) {
			invalid()
			return
		}
		writeAdmissionProto(w, http.StatusOK, v)
	case *spacev1.SubmitJoinAttemptRequest:
		response, err := t.clients.spaceLifecycle.SubmitJoinAttempt(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || !validAdmissionResult(response.Result, space, claims.ProfileID) {
			invalid()
			return
		}
		code := http.StatusOK
		if response.Result.State == spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING {
			code = http.StatusAccepted
		} else if response.Result.State != spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED {
			invalid()
			return
		}
		writeAdmissionProto(w, code, response.Result)
	case *spacev1.ListJoinRequestsRequest:
		response, err := t.clients.spaceLifecycle.ListJoinRequests(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil {
			invalid()
			return
		}
		for _, q := range response.Requests {
			if q == nil || q.SpaceId != space || !canonicalLifecycleUUID(q.RequestId) || !canonicalLifecycleUUID(q.ApplicantProfileId) || q.CreatedAt == nil || !q.CreatedAt.IsValid() || q.AttemptCount == 0 {
				invalid()
				return
			}
		}
		writeAdmissionProto(w, http.StatusOK, response)
	case *spacev1.GetJoinRequestRequest:
		response, err := t.clients.spaceLifecycle.GetJoinRequest(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || response.Request == nil || response.Request.RequestId != child || response.Request.SpaceId != space {
			invalid()
			return
		}
		writeAdmissionProto(w, http.StatusOK, response)
	case *spacev1.ApproveJoinRequestRequest:
		response, err := t.clients.spaceLifecycle.ApproveJoinRequest(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || !validAdmissionResult(response.Result, space, "") || response.Result.State != spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED {
			invalid()
			return
		}
		writeAdmissionProto(w, http.StatusOK, response.Result)
	case *spacev1.SetEntryPolicyRequest:
		response, err := t.clients.spaceLifecycle.SetEntryPolicy(ctx, request)
		if err != nil {
			writeSpaceAdmissionError(w, err)
			return
		}
		if response == nil || response.OperationId != request.OperationId || response.EntryPolicyVersion < request.ExpectedPolicyVersion || response.EntryPolicyVersion > math.MaxInt64 || (response.EntryPolicyVersion != request.ExpectedPolicyVersion && response.EntryPolicyVersion != request.ExpectedPolicyVersion+1) {
			invalid()
			return
		}
		writeAdmissionProto(w, http.StatusOK, response)
	}
}
