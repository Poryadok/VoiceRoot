package grpcsvc

import (
	"testing"

	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRoleApplyConflictAndRevokedRejectionsAreTypedAndBound(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	request := &rolev1.ApplyGameSessionGrantsRequest{OperationId: operationID}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, category string
		storeErr       error
		code           codes.Code
	}{
		{"conflict", gisowner.CategoryOperationConflict, store.ErrGameSessionGrantConflict, codes.AlreadyExists},
		{"revoked", gisowner.CategoryTerminalRevoked, store.ErrGameSessionGrantRevoked, codes.FailedPrecondition},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := annotateGameGrantOwnerRejection(gameSessionGrantStoreError(test.storeErr), request, test.category)
			if status.Code(got) != test.code {
				t.Fatalf("status code = %s, want %s", status.Code(got), test.code)
			}
			if category, ok := gisowner.Match(got, gisowner.RoleDomain, gisowner.RoleApplyRPC, operationID, hash); !ok || category != test.category {
				t.Fatalf("Match() = %q, %v; want %q", category, ok, test.category)
			}
		})
	}
}

func TestRoleGenericStatusDoesNotCarryPermanentRejection(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	request := &rolev1.ApplyGameSessionGrantsRequest{OperationId: operationID}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	transient := status.Error(codes.Unavailable, "persistence unavailable")
	if got := annotateGameGrantOwnerRejection(transient, request, gisowner.CategoryOperationConflict); got != transient {
		t.Fatalf("annotation changed transient status: %v", got)
	}
	if category, ok := gisowner.Match(transient, gisowner.RoleDomain, gisowner.RoleApplyRPC, operationID, hash); ok {
		t.Fatalf("transient status matched permanent category %q", category)
	}
}
