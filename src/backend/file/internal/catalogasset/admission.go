package catalogasset

import (
	"bytes"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	filev1 "voice.app/voice/file/v1"
)

var ErrAdmission = errors.New("catalog asset is not available")

// LifecycleState is read from the locked authoritative File row. The source
// digest comes from that row, independently of the catalog provenance binding.
// This predicate grants neither destination nor viewer permission.
type LifecycleState struct {
	BlobState      string
	BlobDeleting   bool
	Status         string
	IsE2E          bool
	ExpiresAt      *time.Time
	SourceSHA256   []byte
	ObjectRevision uint64
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

// ValidateReady verifies the exact expected binding and closed File predicate.
// The caller must separately authenticate the source, actor and destination.
func ValidateReady(request *filev1.ValidateCatalogAssetRequest, snapshot *filev1.ValidateCatalogAssetResponse, state LifecycleState, now time.Time) error {
	if request == nil || snapshot == nil || request.ProtocolVersion != 1 || len(request.ProtoReflect().GetUnknown()) != 0 {
		return ErrAdmission
	}
	for _, id := range []string{request.SourceId, request.ActorAccountId, request.ActorProfileId, request.FileId} {
		if !canonicalID(id) {
			return ErrAdmission
		}
	}
	if request.Provider == "" || request.ProviderId == "" || strings.TrimSpace(request.Provider) != request.Provider || strings.TrimSpace(request.ProviderId) != request.ProviderId {
		return ErrAdmission
	}
	if snapshot.SourceId != request.SourceId || snapshot.ActorAccountId != request.ActorAccountId || snapshot.ActorProfileId != request.ActorProfileId || snapshot.FileId != request.FileId || snapshot.Provider != request.Provider || snapshot.ProviderId != request.ProviderId {
		return ErrAdmission
	}
	if state.BlobState != "LIVE" || state.BlobDeleting {
		return ErrAdmission
	}
	if state.Status != "ready" || state.IsE2E || state.ExpiresAt != nil && !now.Before(*state.ExpiresAt) || snapshot.ObjectRevision == 0 || snapshot.ObjectRevision != state.ObjectRevision {
		return ErrAdmission
	}
	if snapshot.Intent != "gif" || snapshot.FileType != "video" || snapshot.ScanResult != "clean" && snapshot.ScanResult != "skipped" {
		return ErrAdmission
	}
	if len(snapshot.SourceSha256) != 32 || len(state.SourceSHA256) != 32 || !bytes.Equal(snapshot.SourceSha256, state.SourceSHA256) || len(snapshot.DeliveredSha256) != 32 || snapshot.DeliveredSizeBytes <= 0 || snapshot.DeliveredSizeBytes > 5<<20 {
		return ErrAdmission
	}
	if snapshot.DeliveredMimeType != "video/mp4" && snapshot.DeliveredMimeType != "video/webm" {
		return ErrAdmission
	}
	return nil
}
