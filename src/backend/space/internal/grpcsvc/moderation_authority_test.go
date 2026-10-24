package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/authctx"
	"voice/backend/space/internal/store"
)

// Raw account/profile headers and a transport request ID cannot manufacture a
// request-bound opaque moderation capability. Nil database/Role/User clients
// make any premature dependency call distinguishable from authentication denial.
func TestSpaceModerationOrdinaryHeadersCannotAuthorizeBanOrUnban(t *testing.T) {
	space, account, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, method := range []string{"BanMember", "UnbanMember"} {
		t.Run(method, func(t *testing.T) {
			for _, headers := range []struct {
				name   string
				values []string
			}{
				{name: "missing proof"},
				{name: "legacy actor headers", values: []string{authctx.HeaderUserID, account, authctx.HeaderProfileID, profile}},
				{name: "unsigned transport metadata", values: []string{authctx.HeaderUserID, account, authctx.HeaderProfileID, profile, "x-request-id", uuid.NewString(), "authorization", "Bearer unverified"}},
			} {
				t.Run(headers.name, func(t *testing.T) {
					ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(headers.values...))
					var request proto.Message
					if method == "BanMember" {
						request = &spacev1.BanMemberRequest{SpaceId: space, AccountId: account}
					} else {
						request = &spacev1.UnbanMemberRequest{SpaceId: space, AccountId: account}
					}
					// Consume the actual producer SDK when composed, without inventing
					// a field on the older SDK or using transport UUID as intent identity.
					if field := request.ProtoReflect().Descriptor().Fields().ByName("operation_id"); field != nil {
						request.ProtoReflect().Set(field, protoreflect.ValueOfString(uuid.NewString()))
					}
					server := &SpaceGRPC{Store: &store.SpaceStore{}}
					var err error
					if method == "BanMember" {
						_, err = server.BanMember(ctx, request.(*spacev1.BanMemberRequest))
					} else {
						_, err = server.UnbanMember(ctx, request.(*spacev1.UnbanMemberRequest))
					}
					require.Equal(t, codes.Unauthenticated, status.Code(err), "ordinary metadata must fail before database or Role/User work")
				})
			}
		})
	}
}
