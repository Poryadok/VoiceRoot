package catalogasset

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	filev1 "voice.app/voice/file/v1"
)

func readyFixture() (*filev1.ValidateCatalogAssetRequest, *filev1.ValidateCatalogAssetResponse, LifecycleState, time.Time) {
	now := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	expires := now.Add(time.Hour)
	req := &filev1.ValidateCatalogAssetRequest{ProtocolVersion: 1, SourceId: uuid.NewString(), ActorAccountId: uuid.NewString(), ActorProfileId: uuid.NewString(), Provider: "fixture", ProviderId: "asset-1", FileId: uuid.NewString()}
	response := &filev1.ValidateCatalogAssetResponse{SourceId: req.SourceId, ActorAccountId: req.ActorAccountId, ActorProfileId: req.ActorProfileId, Provider: req.Provider, ProviderId: req.ProviderId, FileId: req.FileId, ObjectRevision: 1, Intent: "gif", ScanResult: "clean", FileType: "video", SourceSha256: bytes.Repeat([]byte{1}, 32), DeliveredSha256: bytes.Repeat([]byte{2}, 32), DeliveredSizeBytes: 1024, DeliveredMimeType: "video/mp4"}
	return req, response, LifecycleState{Status: "ready", ExpiresAt: &expires, SourceSHA256: append([]byte(nil), response.SourceSha256...), ObjectRevision: 1}, now
}

func TestCatalogReadyAdmissionRequiresExactAuthoritativeSourceAndRepresentation(t *testing.T) {
	req, response, state, now := readyFixture()
	if err := ValidateReady(req, response, state, now); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*filev1.ValidateCatalogAssetResponse, *LifecycleState){
		"source":                func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.SourceId = uuid.NewString() },
		"account":               func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ActorAccountId = uuid.NewString() },
		"profile":               func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ActorProfileId = uuid.NewString() },
		"file":                  func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.FileId = uuid.NewString() },
		"provider":              func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.Provider = "forged" },
		"provider-id":           func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ProviderId = "other" },
		"pending":               func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.Status = "pending_upload" },
		"processing":            func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.Status = "processing" },
		"failed":                func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.Status = "failed" },
		"deleted":               func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.Status = "deleted" },
		"expired":               func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.Status = "expired" },
		"expiry-boundary":       func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.ExpiresAt = &now },
		"e2e":                   func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.IsE2E = true },
		"wrong-intent":          func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.Intent = "" },
		"wrong-type":            func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.FileType = "image" },
		"pending-scan":          func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ScanResult = "pending" },
		"infected":              func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ScanResult = "infected" },
		"scan-error":            func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ScanResult = "error" },
		"missing-source-digest": func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.SourceSha256 = nil },
		"changed-source-bytes": func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) {
			s.SourceSHA256 = bytes.Repeat([]byte{3}, 32)
		},
		"missing-delivery-digest":  func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.DeliveredSha256 = nil },
		"revision-zero":            func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.ObjectRevision = 0 },
		"stale-revision":           func(_ *filev1.ValidateCatalogAssetResponse, s *LifecycleState) { s.ObjectRevision++ },
		"empty-representation":     func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.DeliveredSizeBytes = 0 },
		"oversized-representation": func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.DeliveredSizeBytes = (5 << 20) + 1 },
		"wrong-representation":     func(r *filev1.ValidateCatalogAssetResponse, _ *LifecycleState) { r.DeliveredMimeType = "image/gif" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := proto.Clone(response).(*filev1.ValidateCatalogAssetResponse)
			changed := state
			change(candidate, &changed)
			if err := ValidateReady(req, candidate, changed, now); err == nil {
				t.Fatal("invalid catalog snapshot admitted")
			}
		})
	}
}

func TestCatalogReadyAdmissionRejectsInvalidIdentityAndAcceptsDocumentedVariants(t *testing.T) {
	req, response, state, now := readyFixture()
	for _, change := range []func(*filev1.ValidateCatalogAssetRequest){
		func(r *filev1.ValidateCatalogAssetRequest) { r.ProtocolVersion = 2 },
		func(r *filev1.ValidateCatalogAssetRequest) { r.SourceId = "not-a-uuid" },
		func(r *filev1.ValidateCatalogAssetRequest) { r.ActorAccountId = uuid.Nil.String() },
		func(r *filev1.ValidateCatalogAssetRequest) { r.ActorProfileId = "" },
		func(r *filev1.ValidateCatalogAssetRequest) { r.FileId = "00000000-0000-4000-8000-00000000000A" },
	} {
		invalid := proto.Clone(req).(*filev1.ValidateCatalogAssetRequest)
		change(invalid)
		if err := ValidateReady(invalid, response, state, now); err == nil {
			t.Fatal("invalid request admitted")
		}
	}
	if err := ValidateReady(nil, response, state, now); err == nil {
		t.Fatal("nil request admitted")
	}
	if err := ValidateReady(req, nil, state, now); err == nil {
		t.Fatal("nil snapshot admitted")
	}
	response.ScanResult = "skipped"
	response.DeliveredMimeType = "video/webm"
	state.ExpiresAt = nil
	if err := ValidateReady(req, response, state, now); err != nil {
		t.Fatal(err)
	}
}
