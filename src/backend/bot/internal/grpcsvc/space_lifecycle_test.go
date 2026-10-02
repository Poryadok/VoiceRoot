package grpcsvc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/principal"
)

type botLifecycleStoreStub struct {
	fenceCalls int
	purgeCalls int
	fence      *commonv1.SpaceLifecycleFenceRequest
	purge      *commonv1.SpacePurgeRequest
}

func (s *botLifecycleStoreStub) ApplySpaceLifecycleFence(_ context.Context, request *commonv1.SpaceLifecycleFenceRequest) (*commonv1.SpaceLifecycleFenceReceipt, error) {
	s.fenceCalls++
	s.fence = request
	return &commonv1.SpaceLifecycleFenceReceipt{ReceiptId: "fence-receipt"}, nil
}

func (s *botLifecycleStoreStub) PurgeSpace(_ context.Context, request *commonv1.SpacePurgeRequest) (*commonv1.SpacePurgeReceipt, error) {
	s.purgeCalls++
	s.purge = request
	return &commonv1.SpacePurgeReceipt{ReceiptId: "purge-receipt"}, nil
}

func TestSpaceLifecycleRPCsRequireExactSpacePrincipalAndForwardDurableRequests(t *testing.T) {
	store := &botLifecycleStoreStub{}
	service := NewBotGRPC(nil, nil)
	service.LifecycleStore = store

	fence := &commonv1.SpaceLifecycleFenceRequest{SpaceId: "space-id"}
	fenceRequest := &botv1.ApplySpaceLifecycleFenceRequest{Fence: fence}
	fenceHash, err := principal.RequestHash(fenceRequest)
	if err != nil {
		t.Fatal(err)
	}
	fenceMethod := botv1.BotService_ApplySpaceLifecycleFence_FullMethodName
	fenceContext := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot",
		RPC: fenceMethod, RequestID: "fence-request", RequestHash: fenceHash,
	})
	fenceResponse, err := service.ApplySpaceLifecycleFence(fenceContext, fenceRequest)
	if err != nil || fenceResponse.GetReceipt().GetReceiptId() != "fence-receipt" {
		t.Fatalf("ApplySpaceLifecycleFence() response=%v err=%v", fenceResponse, err)
	}
	if store.fenceCalls != 1 || store.fence != fence {
		t.Fatalf("fence was not forwarded exactly once: calls=%d request=%p", store.fenceCalls, store.fence)
	}

	wrongHashContext := principal.WithVerified(fenceContext, principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot",
		RPC: fenceMethod, RequestID: "fence-request", RequestHash: "wrong",
	})
	if _, err := service.ApplySpaceLifecycleFence(wrongHashContext, fenceRequest); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong request hash error=%v, want PermissionDenied", err)
	}
	if store.fenceCalls != 1 {
		t.Fatalf("wrong request hash reached store: calls=%d", store.fenceCalls)
	}

	purge := &commonv1.SpacePurgeRequest{SpaceId: "space-id"}
	purgeRequest := &botv1.PurgeSpaceRequest{Purge: purge}
	purgeHash, err := principal.RequestHash(purgeRequest)
	if err != nil {
		t.Fatal(err)
	}
	purgeMethod := botv1.BotService_PurgeSpace_FullMethodName
	purgeContext := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot",
		RPC: purgeMethod, RequestID: "purge-request", RequestHash: purgeHash,
	})
	purgeResponse, err := service.PurgeSpace(purgeContext, purgeRequest)
	if err != nil || purgeResponse.GetReceipt().GetReceiptId() != "purge-receipt" {
		t.Fatalf("PurgeSpace() response=%v err=%v", purgeResponse, err)
	}
	if store.purgeCalls != 1 || store.purge != purge {
		t.Fatalf("purge was not forwarded exactly once: calls=%d request=%p", store.purgeCalls, store.purge)
	}
}

func TestSpaceLifecycleRPCsRejectMissingAndWrongServicePrincipal(t *testing.T) {
	service := NewBotGRPC(nil, nil)
	request := &botv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{SpaceId: "space-id"}}
	if _, err := service.ApplySpaceLifecycleFence(context.Background(), request); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing principal error=%v, want Unauthenticated", err)
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	wrongCaller := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "gameintegration", Subject: "service:gameintegration", Audience: "bot",
		RPC: botv1.BotService_ApplySpaceLifecycleFence_FullMethodName, RequestID: "request", RequestHash: hash,
	})
	if _, err := service.ApplySpaceLifecycleFence(wrongCaller, request); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("wrong principal error=%v, want PermissionDenied", err)
	}
}

func TestSpaceLifecycleRPCsRejectNilAndUnknownFieldRequests(t *testing.T) {
	service := NewBotGRPC(nil, nil)
	if _, err := service.ApplySpaceLifecycleFence(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil fence request error=%v, want InvalidArgument", err)
	}
	request := &botv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{SpaceId: "space-id"}}
	request.ProtoReflect().SetUnknown([]byte{0x78, 0x01})
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	verified := principal.WithVerified(context.Background(), principal.Principal{
		Kind: "service", Issuer: "space", Subject: "service:space", Audience: "bot",
		RPC: botv1.BotService_ApplySpaceLifecycleFence_FullMethodName, RequestID: "request", RequestHash: hash,
	})
	if _, err := service.ApplySpaceLifecycleFence(verified, request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown field error=%v, want InvalidArgument", err)
	}
}
