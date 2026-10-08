package gisowner

import (
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPermanentRejectionBindsOwnerMethodOperationAndHash(t *testing.T) {
	const (
		operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
		requestHash = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	err := Annotate(status.Error(codes.AlreadyExists, "conflict"), ChatDomain, ChatProvisionRPC,
		CategoryResourceConflict, operationID, requestHash)
	if category, ok := Match(err, ChatDomain, ChatProvisionRPC, operationID, requestHash); !ok || category != CategoryResourceConflict {
		t.Fatalf("Match() = %q, %v; want resource conflict", category, ok)
	}
	for name, match := range map[string]struct {
		domain, rpc, operationID, requestHash string
	}{
		"other owner": {VoiceDomain, ChatProvisionRPC, operationID, requestHash},
		"other method": {ChatDomain, ChatRosterRPC, operationID, requestHash},
		"other operation": {ChatDomain, ChatProvisionRPC, "other", requestHash},
		"other hash": {ChatDomain, ChatProvisionRPC, operationID, "sha256:other"},
	} {
		t.Run(name, func(t *testing.T) {
			if category, ok := Match(err, match.domain, match.rpc, match.operationID, match.requestHash); ok {
				t.Fatalf("Match() = %q, true; want no match", category)
			}
		})
	}
}

func TestPermanentRejectionRequiresTypedStatusAndAllowListedCategory(t *testing.T) {
	const (
		operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
		requestHash = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	)
	t.Run("same public code without details remains retryable", func(t *testing.T) {
		err := status.Error(codes.AlreadyExists, "conflict")
		if category, ok := Match(err, ChatDomain, ChatProvisionRPC, operationID, requestHash); ok {
			t.Fatalf("Match() = %q, true; want no match", category)
		}
	})
	t.Run("wrong status code is not annotated", func(t *testing.T) {
		err := status.Error(codes.Unavailable, "temporary")
		annotated := Annotate(err, ChatDomain, ChatProvisionRPC, CategoryOperationConflict, operationID, requestHash)
		if annotated != err {
			t.Fatalf("Annotate() changed transient error: %v", annotated)
		}
	})
	t.Run("unsupported method category is not annotated", func(t *testing.T) {
		err := status.Error(codes.FailedPrecondition, "revoked")
		annotated := Annotate(err, RoleDomain, RoleApplyRPC, CategoryResourceMissing, operationID, requestHash)
		if annotated != err {
			t.Fatalf("Annotate() changed unsupported category: %v", annotated)
		}
	})
	t.Run("wrong reason remains retryable", func(t *testing.T) {
		base := status.New(codes.AlreadyExists, "conflict")
		bad, err := base.WithDetails(&errdetails.ErrorInfo{Reason: "OTHER_REASON", Domain: ChatDomain, Metadata: map[string]string{
			metadataRPC: ChatProvisionRPC, metadataCategory: CategoryOperationConflict,
			metadataOperationID: operationID, metadataRequestHash: requestHash,
		}})
		if err != nil {
			t.Fatal(err)
		}
		if category, ok := Match(bad.Err(), ChatDomain, ChatProvisionRPC, operationID, requestHash); ok {
			t.Fatalf("Match() = %q, true; want no match", category)
		}
	})
}
