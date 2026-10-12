package grpcsvc

import (
	"testing"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/store"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestManagedChatOwnerRejectionsAreTypedAndRequestBound(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	request := &chatv1.ProvisionManagedChatRequest{OperationId: operationID}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	annotated := managedChatOwnerStatus(store.ErrManagedResourceConflict, request)
	if status.Code(annotated) != codes.AlreadyExists {
		t.Fatalf("status code = %s, want AlreadyExists", status.Code(annotated))
	}
	if category, ok := gisowner.Match(annotated, gisowner.ChatDomain, gisowner.ChatProvisionRPC, operationID, hash); !ok || category != gisowner.CategoryResourceConflict {
		t.Fatalf("Match() = %q, %v; want typed resource conflict", category, ok)
	}
}

func TestManagedChatRosterMissingResourceIsTypedButPrincipalFailureIsNot(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	request := &chatv1.SyncManagedChatMembersRequest{OperationId: operationID, ChatId: "00000000-0000-4000-8000-000000000002"}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	missing := managedChatOwnerStatus(store.ErrManagedChatNotFound, request)
	if category, ok := gisowner.Match(missing, gisowner.ChatDomain, gisowner.ChatRosterRPC, operationID, hash); !ok || category != gisowner.CategoryResourceMissing {
		t.Fatalf("Match(missing) = %q, %v; want typed missing resource", category, ok)
	}
	principalFailure := managedChatOwnerStatus(store.ErrManagedChatPrincipalRequired, request)
	if status.Code(principalFailure) != codes.PermissionDenied {
		t.Fatalf("principal failure code = %s, want PermissionDenied", status.Code(principalFailure))
	}
	if category, ok := gisowner.Match(principalFailure, gisowner.ChatDomain, gisowner.ChatRosterRPC, operationID, hash); ok {
		t.Fatalf("principal failure matched permanent category %q", category)
	}
}
