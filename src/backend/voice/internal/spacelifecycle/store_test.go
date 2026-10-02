package spacelifecycle

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
)

func TestValidateFenceRequestRequiresCanonicalVersionedManifest(t *testing.T) {
	valid := &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: uuid.NewString(), DeletionOperationId: uuid.NewString(), Generation: 1,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN,
		Manifest:     &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 0},
	}
	require.NoError(t, validateFenceRequest(valid))

	tests := []struct {
		name   string
		mutate func(*commonv1.SpaceLifecycleFenceRequest)
	}{
		{"protocol", func(req *commonv1.SpaceLifecycleFenceRequest) { req.ProtocolVersion = 2 }},
		{"noncanonical space", func(req *commonv1.SpaceLifecycleFenceRequest) { req.SpaceId = "{" + req.SpaceId + "}" }},
		{"zero generation", func(req *commonv1.SpaceLifecycleFenceRequest) { req.Generation = 0 }},
		{"unspecified state", func(req *commonv1.SpaceLifecycleFenceRequest) {
			req.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_UNSPECIFIED
		}},
		{"bad manifest hash", func(req *commonv1.SpaceLifecycleFenceRequest) { req.Manifest.ManifestSha256 = []byte{1} }},
		{"missing manifest", func(req *commonv1.SpaceLifecycleFenceRequest) { req.Manifest = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := proto.Clone(valid).(*commonv1.SpaceLifecycleFenceRequest)
			test.mutate(req)
			require.Error(t, validateFenceRequest(req))
		})
	}
}
