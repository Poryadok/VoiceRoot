package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
)

func TestRetireSpace_RejectsUnknownAuthorityFieldsWithoutStorage(t *testing.T) {
	f := r23RetirementFixture{
		spaceID: uuid.New(), deletionOperationID: uuid.New(), manifestID: uuid.New(),
		purgeDecidedAt: time.Unix(1_700_000_000, 123_000_000).UTC(), manifestSHA: make([]byte, 32),
	}
	request := f.request()
	svc := &RoleGRPC{}
	_, err := svc.RetireSpace(r23VerifiedContext(t, rolev1.RoleService_RetireSpace_FullMethodName, request), request)
	require.Equal(t, codes.Unavailable, status.Code(err), "valid control must reach the unconfigured persistence boundary")

	for _, tc := range []struct {
		name   string
		mutate func(*rolev1.RetireSpaceRequest)
	}{
		{"request", func(r *rolev1.RetireSpaceRequest) { r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) }},
		{"manifest", func(r *rolev1.RetireSpaceRequest) { r.Manifest.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01}) }},
		{"purge_decided_at", func(r *rolev1.RetireSpaceRequest) {
			r.PurgeDecidedAt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := proto.Clone(request).(*rolev1.RetireSpaceRequest)
			tc.mutate(req)
			response, err := svc.RetireSpace(r23VerifiedContext(t, rolev1.RoleService_RetireSpace_FullMethodName, req), req)
			require.Equal(t, codes.InvalidArgument, status.Code(err), "unknown authority fields must deny before persistence")
			require.Nil(t, response)
		})
	}
}
