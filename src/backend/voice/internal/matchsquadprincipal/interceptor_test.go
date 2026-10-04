package matchsquadprincipal

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type recordingVerifier struct {
	called bool
	got    principal.Principal
	err    error
}

func (v *recordingVerifier) Verify(_ context.Context, token, method, requestID, requestHash string) (principal.Principal, error) {
	v.called = true
	if token != "service-token" || (method != CreateMethod && method != TeardownMethod && method != CompactMethod) || requestID == "" || requestHash == "" {
		return principal.Principal{}, status.Error(codes.Unauthenticated, "unexpected verification binding")
	}
	return v.got, v.err
}

func TestStrictUnaryInterceptorVerifiesCompactOperationBinding(t *testing.T) {
	req := &callsv1.CompactMatchSquadRoomRequest{
		ProtocolVersion: 1, OperationId: uuid.NewString(), TeardownAggregateId: uuid.NewString(),
		MatchId: uuid.NewString(), RoomId: uuid.NewString(), CreationReceiptId: uuid.NewString(),
		CreationRequestSha256: make([]byte, 32), TeardownOperationId: uuid.NewString(), TeardownReceiptId: uuid.NewString(),
		TeardownReceiptSha256: make([]byte, 32), ParticipantManifestSha256: make([]byte, 32),
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	verifier := &recordingVerifier{got: principal.Principal{
		Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice",
		RPC: CompactMethod, RequestID: req.GetOperationId(), RequestHash: hash,
	}}
	called := false
	_, err = StrictUnaryInterceptor(verifier)(serviceMetadata(req.GetOperationId()), req,
		&grpc.UnaryServerInfo{FullMethod: CompactMethod}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	if err != nil || !called || !verifier.called || operationID(req) != req.GetOperationId() {
		t.Fatalf("compact interceptor binding: err=%v handler=%v verifier=%v", err, called, verifier.called)
	}
}

func validCreateRequest() *callsv1.CreateMatchSquadRoomRequest {
	return &callsv1.CreateMatchSquadRoomRequest{
		ProtocolVersion: 1,
		OperationId:     uuid.NewString(),
		MatchId:         uuid.NewString(),
		ChatCreationReceipt: &chatv1.MatchSquadChatReceipt{
			ProtocolVersion:           1,
			ReceiptId:                 uuid.NewString(),
			OperationId:               uuid.NewString(),
			MatchId:                   uuid.NewString(),
			ChatId:                    uuid.NewString(),
			ParticipantManifestSha256: make([]byte, 32),
			RequestSha256:             make([]byte, 32),
		},
	}
}

func serviceMetadata(requestID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer service-token", "x-request-id", requestID))
}

func TestStrictUnaryInterceptorVerifiesExactServiceBinding(t *testing.T) {
	req := validCreateRequest()
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	verified := principal.Principal{
		Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice",
		RPC: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName, RequestID: req.OperationId,
		RequestHash: hash,
	}
	verifier := &recordingVerifier{got: verified}
	called := false
	interceptor := StrictUnaryInterceptor(verifier)
	_, err = interceptor(serviceMetadata(req.OperationId), req,
		&grpc.UnaryServerInfo{FullMethod: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName},
		func(ctx context.Context, _ any) (any, error) {
			called = true
			got, ok := principal.FromContext(ctx)
			if !ok || got.Subject != "service:matchmaking" {
				t.Fatal("verified service principal was not attached to handler context")
			}
			return nil, nil
		})
	if err != nil || !called || !verifier.called {
		t.Fatalf("interceptor result: err=%v handler=%v verifier=%v", err, called, verifier.called)
	}
}

func TestStrictUnaryInterceptorRejectsWrongMethodAndMalformedMetadataBeforeVerification(t *testing.T) {
	for _, tc := range []struct {
		name   string
		method string
		ctx    context.Context
	}{
		{name: "ordinary voice method", method: callsv1.VoiceService_StartCall_FullMethodName, ctx: serviceMetadata(uuid.NewString())},
		{name: "wrong request id", method: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName, ctx: serviceMetadata(uuid.NewString())},
		{name: "duplicate bearer", method: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName, ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer service-token", "authorization", "Bearer service-token", "x-request-id", "op"))},
		{name: "raw identity", method: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName, ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer service-token", "x-request-id", "op", "x-profile-id", uuid.NewString()))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := validCreateRequest()
			if tc.name == "wrong request id" {
				req.OperationId = uuid.NewString()
			}
			verifier := &recordingVerifier{}
			called := false
			_, err := StrictUnaryInterceptor(verifier)(tc.ctx, req, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(context.Context, any) (any, error) {
				called = true
				return nil, nil
			})
			if err == nil || called || verifier.called {
				t.Fatalf("invalid request reached verification/handler: err=%v handler=%v verifier=%v", err, called, verifier.called)
			}
		})
	}
}

func TestStrictUnaryInterceptorRejectsUnknownNestedRequestFields(t *testing.T) {
	req := validCreateRequest()
	req.ChatCreationReceipt.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	verifier := &recordingVerifier{}
	called := false
	_, err := StrictUnaryInterceptor(verifier)(serviceMetadata(req.OperationId), req,
		&grpc.UnaryServerInfo{FullMethod: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName},
		func(context.Context, any) (any, error) { called = true; return nil, nil })
	if status.Code(err) != codes.InvalidArgument || called || verifier.called {
		t.Fatalf("unknown nested field result: err=%v handler=%v verifier=%v", err, called, verifier.called)
	}
}

func TestRejectsUnverifiedOrWrongAudienceServicePrincipal(t *testing.T) {
	for _, principalValue := range []principal.Principal{
		{},
		{Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "voice"},
		{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "chat"},
		{Kind: "service", Issuer: "matchmaking", Subject: "service:matchmaking", Audience: "voice", ProfileID: uuid.NewString()},
	} {
		req := validCreateRequest()
		verifier := &recordingVerifier{got: principalValue}
		called := false
		_, err := StrictUnaryInterceptor(verifier)(serviceMetadata(req.OperationId), req,
			&grpc.UnaryServerInfo{FullMethod: callsv1.MatchSquadVoiceService_CreateMatchSquadRoom_FullMethodName},
			func(context.Context, any) (any, error) { called = true; return nil, nil })
		if status.Code(err) != codes.Unauthenticated || called {
			t.Fatalf("invalid principal reached handler: principal=%+v err=%v handler=%v", principalValue, err, called)
		}
	}
}
