package sessionowners

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/gameintegration/internal/registry"
	"voice/backend/pkg/gisowner"
)

func TestClassifyOwnerErrorRequiresExactOwnerMethodOperationAndHash(t *testing.T) {
	request := registry.SessionOwnerRequest{
		OperationID: uuid.MustParse("6f2f5cd2-8712-484e-87d6-2da943a6f589"),
		RequestHash:  "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	typed := gisowner.Annotate(status.Error(codes.AlreadyExists, "conflict"), gisowner.RoleDomain,
		gisowner.RoleApplyRPC, gisowner.CategoryOperationConflict, request.OperationID.String(), request.RequestHash)
	got := classifyOwnerError(typed, request, gisowner.RoleDomain, gisowner.RoleApplyRPC)
	var rejection *registry.PermanentOwnerRejection
	if !errors.As(got, &rejection) || rejection.Category != gisowner.CategoryOperationConflict {
		t.Fatalf("classified error = %#v; want permanent operation conflict", got)
	}

	for name, test := range map[string]struct {
		err     error
		domain  string
		rpc     string
		request registry.SessionOwnerRequest
	}{
		"same code without details": {status.Error(codes.AlreadyExists, "conflict"), gisowner.RoleDomain, gisowner.RoleApplyRPC, request},
		"wrong owner": {typed, gisowner.ChatDomain, gisowner.RoleApplyRPC, request},
		"wrong method": {typed, gisowner.RoleDomain, gisowner.VoiceProvisionRPC, request},
		"wrong operation": {typed, gisowner.RoleDomain, gisowner.RoleApplyRPC, registry.SessionOwnerRequest{OperationID: uuid.New(), RequestHash: request.RequestHash}},
		"wrong hash": {typed, gisowner.RoleDomain, gisowner.RoleApplyRPC, registry.SessionOwnerRequest{OperationID: request.OperationID, RequestHash: "sha256:1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}},
	} {
		t.Run(name, func(t *testing.T) {
			got := classifyOwnerError(test.err, test.request, test.domain, test.rpc)
			if errors.As(got, &rejection) {
				t.Fatalf("classified mismatched owner response as permanent: %#v", rejection)
			}
		})
	}
}
