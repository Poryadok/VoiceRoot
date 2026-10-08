// Package gisowner defines the private, durable rejection signal shared by GIS
// and the owner services used by session provisioning.
package gisowner

import (
	"encoding/hex"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	RejectionReason = "GIS_PERMANENT_OWNER_REJECTION"

	ChatDomain  = "voice.chat.v1"
	VoiceDomain = "voice.calls.v1"
	RoleDomain  = "voice.role.v1"

	ChatProvisionRPC = "/voice.chat.v1.GameIntegrationChatService/ProvisionManagedChat"
	ChatRosterRPC    = "/voice.chat.v1.GameIntegrationChatService/SyncManagedChatMembers"
	VoiceProvisionRPC = "/voice.calls.v1.GameSessionProvisioningService/ProvisionGameSessionRoom"
	VoiceRosterRPC    = "/voice.calls.v1.GameSessionProvisioningService/ApplyGameSessionRoster"
	RoleApplyRPC      = "/voice.role.v1.RoleService/ApplyGameSessionGrants"

	CategoryOperationConflict = "operation_conflict"
	CategoryResourceConflict  = "resource_conflict"
	CategoryResourceMissing   = "resource_missing"
	CategoryTerminalRevoked   = "terminal_revoked"
)

const (
	metadataRPC         = "rpc"
	metadataCategory    = "category"
	metadataOperationID = "operation_id"
	metadataRequestHash = "request_hash"
)

// Annotate adds an explicit owner-domain rejection to an already mapped gRPC
// status. It preserves the public status code and message. Invalid bindings,
// unsupported categories, or non-gRPC errors are returned unchanged.
func Annotate(err error, domain, rpc, category, operationID, requestHash string) error {
	if err == nil || !canonicalOperationID(operationID) || !canonicalRequestHash(requestHash) || !allowed(domain, rpc, category) {
		return err
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != categoryCode(category) {
		return err
	}
	with, detailErr := st.WithDetails(&errdetails.ErrorInfo{
		Reason: RejectionReason,
		Domain:  domain,
		Metadata: map[string]string{
			metadataRPC:         rpc,
			metadataCategory:    category,
			metadataOperationID: operationID,
			metadataRequestHash: requestHash,
		},
	})
	if detailErr != nil {
		return err
	}
	return with.Err()
}

// Match accepts only an allow-listed owner method/category with the exact
// operation and canonical request hash expected by GIS for this call.
func Match(err error, domain, rpc, operationID, requestHash string) (string, bool) {
	if err == nil || !canonicalOperationID(operationID) || !canonicalRequestHash(requestHash) {
		return "", false
	}
	st, ok := status.FromError(err)
	if !ok {
		return "", false
	}
	var matched string
	for _, detail := range st.Details() {
		info, ok := detail.(*errdetails.ErrorInfo)
		if !ok || info.GetReason() != RejectionReason || info.GetDomain() != domain {
			continue
		}
		metadata := info.GetMetadata()
		category := metadata[metadataCategory]
		if matched != "" || !allowed(domain, rpc, category) || st.Code() != categoryCode(category) ||
			metadata[metadataRPC] != rpc || metadata[metadataOperationID] != operationID ||
			metadata[metadataRequestHash] != requestHash {
			return "", false
		}
		matched = category
	}
	return matched, matched != ""
}

func canonicalOperationID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func canonicalRequestHash(value string) bool {
	if len(value) != len("sha256:")+sha256HexLength || !strings.HasPrefix(value, "sha256:") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[len("sha256:"):])
	return err == nil
}

const sha256HexLength = 64

func categoryCode(category string) codes.Code {
	switch category {
	case CategoryOperationConflict, CategoryResourceConflict:
		return codes.AlreadyExists
	case CategoryResourceMissing:
		return codes.NotFound
	case CategoryTerminalRevoked:
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

func allowed(domain, rpc, category string) bool {
	switch {
	case domain == ChatDomain && rpc == ChatProvisionRPC:
		return category == CategoryOperationConflict || category == CategoryResourceConflict
	case domain == ChatDomain && rpc == ChatRosterRPC:
		return category == CategoryOperationConflict || category == CategoryResourceConflict || category == CategoryResourceMissing
	case domain == VoiceDomain && (rpc == VoiceProvisionRPC || rpc == VoiceRosterRPC):
		return category == CategoryOperationConflict
	case domain == RoleDomain && rpc == RoleApplyRPC:
		return category == CategoryOperationConflict || category == CategoryTerminalRevoked
	default:
		return false
	}
}
