package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	userv1 "voice.app/voice/user/v1"
)

// multi-profile/verification (docs/features/multi-profile.md) contract tests — fail until protos and codegen include the fields/RPCs.

func TestPhase13_ProfileProto_HasFrozenAtField(t *testing.T) {
	t.Parallel()
	fd := (&userv1.Profile{}).ProtoReflect().Descriptor().Fields().ByName("frozen_at")
	require.NotNil(t, fd, "Profile.frozen_at must be exposed on the Profile proto")
	require.Equal(t, protoreflect.MessageKind, fd.Kind())
}

func TestPhase13_UserService_HasSetVerificationRPC(t *testing.T) {
	t.Parallel()
	requireContainsGRPCMethod(t, "SetVerification")
}

func TestPhase13_UserService_HasClearVerificationRPC(t *testing.T) {
	t.Parallel()
	requireContainsGRPCMethod(t, "ClearVerification")
}

func requireContainsGRPCMethod(t *testing.T, name string) {
	t.Helper()
	for _, m := range userv1.UserService_ServiceDesc.Methods {
		if m.MethodName == name {
			return
		}
	}
	t.Fatalf("UserService must expose %s RPC (S2S verification)", name)
}
