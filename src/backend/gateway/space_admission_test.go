package main

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type admissionInvokeConn struct {
	call func(context.Context, string, proto.Message, proto.Message) error
}

func (c *admissionInvokeConn) Invoke(ctx context.Context, method string, args, reply any, _ ...grpc.CallOption) error {
	return c.call(ctx, method, args.(proto.Message), reply.(proto.Message))
}
func (*admissionInvokeConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, status.Error(codes.Unavailable, "no stream")
}

func TestSpaceAdmissionProtectedRoutesBindFinalDTO(t *testing.T) {
	f := newCreatorFixture(t)
	space, attempt, requestID, operation := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, kind := range []string{"regular", "guest"} {
		f.payload["account_type"] = kind
		for _, tc := range []struct {
			method, path, body, rpc string
			want                    int
		}{
			{http.MethodPost, "/api/v1/spaces/" + space + "/join-attempts", `{"operation_id":"` + operation + `"}`, spacev1.SpaceService_StartJoinAttempt_FullMethodName, 201},
			{http.MethodPost, "/api/v1/invites/exact-code/join-attempts", `{"operation_id":"` + operation + `"}`, spacev1.SpaceService_StartJoinAttempt_FullMethodName, 201},
			{http.MethodGet, "/api/v1/spaces/" + space + "/join-attempts/" + attempt, "", spacev1.SpaceService_GetJoinAttempt_FullMethodName, 200},
			{http.MethodPost, "/api/v1/spaces/" + space + "/join-attempts/" + attempt + "/submit", `{"operation_id":"` + operation + `","answers":[]}`, spacev1.SpaceService_SubmitJoinAttempt_FullMethodName, 202},
			{http.MethodGet, "/api/v1/spaces/" + space + "/join-requests?limit=10&cursor=opaque", "", spacev1.SpaceService_ListJoinRequests_FullMethodName, 200},
			{http.MethodGet, "/api/v1/spaces/" + space + "/join-requests/" + requestID, "", spacev1.SpaceService_GetJoinRequest_FullMethodName, 200},
			{http.MethodPost, "/api/v1/spaces/" + space + "/join-requests/" + requestID + "/approve", fmt.Sprintf(`{"operation_id":%q,"attempt_id":%q}`, operation, attempt), spacev1.SpaceService_ApproveJoinRequest_FullMethodName, 200},
			{http.MethodPut, "/api/v1/spaces/" + space + "/entry-policy", fmt.Sprintf(`{"operation_id":%q,"expected_policy_version":1,"policy":{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":false,"questions":[]}}`, operation), spacev1.SpaceService_SetEntryPolicy_FullMethodName, 200},
		} {
			t.Run(kind+tc.path, func(t *testing.T) {
				calls, ordinaryCalls := 0, 0
				conn := &admissionInvokeConn{call: func(ctx context.Context, method string, req, reply proto.Message) error {
					calls++
					require.Equal(t, tc.rpc, method)
					md, _ := metadata.FromOutgoingContext(ctx)
					require.Len(t, md, 2)
					hash, err := principal.RequestHash(req)
					require.NoError(t, err)
					verify := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}
					p, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), verify)
					require.NoError(t, err)
					require.Equal(t, kind, p.AccountType)
					require.Equal(t, f.payload["profile_id"], p.ProfileID)
					verify.ExpectedRequestHash = "changed"
					_, err = principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), verify)
					require.Error(t, err)
					view := &spacev1.JoinAttemptView{AttemptId: attempt, SpaceId: space, PolicyVersion: 1, ExpiresAt: timestamppb.New(time.Now().Add(time.Minute)), Requirements: &spacev1.JoinRequirements{}, Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING}}
					switch r := req.(type) {
					case *spacev1.StartJoinAttemptRequest:
						require.Equal(t, operation, r.OperationId)
						proto.Merge(reply, &spacev1.StartJoinAttemptResponse{Attempt: view})
					case *spacev1.GetJoinAttemptRequest:
						require.Equal(t, space, r.SpaceId)
						require.Equal(t, attempt, r.AttemptId)
						proto.Merge(reply, &spacev1.GetJoinAttemptResponse{Attempt: view})
					case *spacev1.SubmitJoinAttemptRequest:
						require.Equal(t, space, r.SpaceId)
						require.Equal(t, attempt, r.AttemptId)
						require.Equal(t, operation, r.OperationId)
						proto.Merge(reply, &spacev1.SubmitJoinAttemptResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, RequestId: requestID}})
					case *spacev1.ListJoinRequestsRequest:
						require.EqualValues(t, 10, r.Limit)
						require.Equal(t, "opaque", r.Cursor)
						proto.Merge(reply, &spacev1.ListJoinRequestsResponse{})
					case *spacev1.GetJoinRequestRequest:
						require.Equal(t, requestID, r.RequestId)
						proto.Merge(reply, &spacev1.GetJoinRequestResponse{Request: &spacev1.JoinRequestSummary{RequestId: requestID, SpaceId: space, ApplicantProfileId: uuid.NewString(), CreatedAt: timestamppb.Now(), AttemptCount: 1}})
					case *spacev1.ApproveJoinRequestRequest:
						require.Equal(t, requestID, r.RequestId)
						require.Equal(t, attempt, r.GetAttemptId())
						proto.Merge(reply, &spacev1.ApproveJoinRequestResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_ADMITTED, SpaceMembership: &spacev1.SpaceMembership{SpaceId: space, ProfileId: uuid.NewString(), JoinedAt: timestamppb.Now()}}})
					case *spacev1.SetEntryPolicyRequest:
						require.Equal(t, operation, r.OperationId)
						require.EqualValues(t, 1, r.ExpectedPolicyVersion)
						require.Contains(t, string(r.PrivatePolicyJson), `"questions":[]`)
						proto.Merge(reply, &spacev1.SetEntryPolicyResponse{OperationId: operation, EntryPolicyVersion: 2})
					default:
						require.Fail(t, "unexpected admission DTO")
					}
					return nil
				}}
				cfg := f.config(spacev1.NewSpaceServiceClient(conn))
				cfg.transcoder.clients.space = spacev1.NewSpaceServiceClient(&admissionInvokeConn{call: func(context.Context, string, proto.Message, proto.Message) error {
					ordinaryCalls++
					return status.Error(codes.Internal, "ordinary bypass")
				}})
				r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
				r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
				r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
				w := httptest.NewRecorder()
				newGateway(cfg).ServeHTTP(w, r)
				require.Equal(t, tc.want, w.Code, w.Body.String())
				require.Equal(t, 1, calls)
				require.Zero(t, ordinaryCalls)
				require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
				require.NotContains(t, w.Body.String(), "expected_answer")
				require.NotContains(t, w.Body.String(), "captcha_token")
				if tc.want == 202 {
					require.Contains(t, w.Body.String(), `"state":"pending"`)
				}
			})
		}
	}
}

func TestSpaceAdmissionFailsClosedAndNeverFallsBack(t *testing.T) {
	for _, mode := range []string{"static", "chained static shadow", "chained valid", "no strict", "no protected", "no signer", "no transcoder", "duplicate bearer", "body selector", "duplicate body", "body alias duplicate", "duplicate query", "policy selector", "backend denied", "approval pending"} {
		t.Run(mode, func(t *testing.T) {
			f := newCreatorFixture(t)
			f.payload["account_type"] = "regular"
			space, operation := uuid.NewString(), uuid.NewString()
			method := http.MethodPost
			path := "/api/v1/spaces/" + space + "/join-attempts"
			body := `{"operation_id":"` + operation + `"}`
			want := 503
			calls, ordinary, proxy := 0, 0, 0
			conn := &admissionInvokeConn{call: func(ctx context.Context, _ string, req, reply proto.Message) error {
				calls++
				if mode == "backend denied" {
					return status.Error(codes.PermissionDenied, "question_id private_expected_answer=4 provider_token=secret")
				}
				if mode == "approval pending" {
					proto.Merge(reply, &spacev1.ApproveJoinRequestResponse{Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_PENDING, RequestId: uuid.NewString()}})
					return nil
				}
				md, _ := metadata.FromOutgoingContext(ctx)
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_StartJoinAttempt_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }})
				require.NoError(t, err)
				require.Equal(t, f.payload["profile_id"], p.ProfileID, "a static shadow cannot replace the verified JWT actor")
				proto.Merge(reply, &spacev1.StartJoinAttemptResponse{Attempt: &spacev1.JoinAttemptView{AttemptId: uuid.NewString(), SpaceId: space, PolicyVersion: 1, ExpiresAt: timestamppb.Now(), Requirements: &spacev1.JoinRequirements{}, Result: &spacev1.JoinAdmissionResult{State: spacev1.JoinAdmissionState_JOIN_ADMISSION_STATE_COLLECTING}}})
				return nil
			}}
			cfg := f.config(spacev1.NewSpaceServiceClient(conn))
			cfg.transcoder.clients.space = spacev1.NewSpaceServiceClient(&admissionInvokeConn{call: func(context.Context, string, proto.Message, proto.Message) error {
				ordinary++
				return status.Error(codes.Internal, "ordinary fallback")
			}})
			cfg.restUpstreams = map[string]http.Handler{"spaces": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxy++ })}
			token := signConfigJWT(t, "creator-key", f.key, f.payload)
			static := tokenClaims{UserID: f.payload["user_id"].(string), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: "regular", JTI: f.payload["jti"].(string), ExpiresAt: time.Now().Add(time.Minute)}
			switch mode {
			case "static":
				cfg.tokenValidator = staticTokenValidator{token: static}
				want = 401
			case "chained static shadow":
				token = "static-shadow-without-valid-jwt"
				cfg.tokenValidator = chainedTokenValidator{static: staticTokenValidator{token: static}, next: cfg.tokenValidator}
				want = 401
			case "chained valid":
				cfg.tokenValidator = chainedTokenValidator{static: staticTokenValidator{token: static}, next: cfg.tokenValidator}
				want = 201
			case "no strict":
				cfg.sessionEpochStrict = false
			case "no protected":
				cfg.transcoder.clients.spaceLifecycle = nil
			case "no signer":
				cfg.transcoder.lifecycleIssuer = nil
			case "no transcoder":
				cfg.transcoder = nil
			case "duplicate bearer":
				want = 401
			case "body selector":
				body = `{"operation_id":"` + operation + `","space_id":"` + space + `"}`
				want = 400
			case "duplicate body":
				body = `{"operation_id":"` + operation + `","operation_id":"` + operation + `"}`
				want = 400
			case "body alias duplicate":
				body = `{"operation_id":"` + operation + `","operationId":"` + operation + `"}`
				want = 400
			case "duplicate query":
				method = http.MethodGet
				path = "/api/v1/spaces/" + space + "/join-requests?limit=1&limit=2"
				body = ""
				want = 400
			case "policy selector":
				method = http.MethodPut
				path = "/api/v1/spaces/" + space + "/entry-policy"
				body = fmt.Sprintf(`{"operation_id":%q,"expected_policy_version":1,"policy":{},"actor_profile_id":%q}`, operation, uuid.NewString())
				want = 400
			case "backend denied":
				want = 403
			case "approval pending":
				path = "/api/v1/spaces/" + space + "/join-requests/" + uuid.NewString() + "/approve"
			}
			r := httptest.NewRequest(method, path, strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			if mode == "duplicate bearer" {
				r.Header.Add("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			newGateway(cfg).ServeHTTP(w, r)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, ordinary)
			require.Zero(t, proxy)
			require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
			require.NotContains(t, w.Body.String(), "private_expected_answer")
			require.NotContains(t, w.Body.String(), "provider_token")
			require.NotContains(t, w.Body.String(), "secret")
			if mode == "backend denied" || mode == "approval pending" || mode == "chained valid" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
