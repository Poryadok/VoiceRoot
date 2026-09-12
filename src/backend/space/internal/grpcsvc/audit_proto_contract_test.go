package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	spacev1 "voice.app/voice/space/v1"
)

type be116AuditFieldSpec struct {
	name             protoreflect.Name
	number           protoreflect.FieldNumber
	kind             protoreflect.Kind
	typeName         protoreflect.FullName
	explicitOptional bool
}

func TestBE116AuditProtoContract(t *testing.T) {
	file := spacev1.File_voice_space_v1_space_proto

	t.Run("GetAuditLogRequest filters", func(t *testing.T) {
		request := requireBE116AuditMessage(t, file, "GetAuditLogRequest")
		requireBE116AuditExactFields(t, request, []be116AuditFieldSpec{
			{name: "space_id", number: 1, kind: protoreflect.StringKind},
			{name: "page", number: 2, kind: protoreflect.MessageKind, typeName: "voice.common.v1.CursorPageRequest"},
			{name: "actor_profile_id", number: 3, kind: protoreflect.StringKind, explicitOptional: true},
			{name: "action", number: 4, kind: protoreflect.StringKind, explicitOptional: true},
			{name: "from", number: 5, kind: protoreflect.MessageKind, typeName: "google.protobuf.Timestamp"},
			{name: "to", number: 6, kind: protoreflect.MessageKind, typeName: "google.protobuf.Timestamp"},
		})
	})

	t.Run("AppendAuditEvent RPC", func(t *testing.T) {
		service := file.Services().ByName("SpaceService")
		require.NotNil(t, service, "%s must define SpaceService", file.Path())

		method := service.Methods().ByName("AppendAuditEvent")
		require.NotNil(t, method, "%s must define AppendAuditEvent", service.FullName())
		require.Equal(t, protoreflect.FullName("voice.space.v1.AppendAuditEventRequest"), method.Input().FullName())
		require.Equal(t, protoreflect.FullName("voice.space.v1.AppendAuditEventResponse"), method.Output().FullName())
	})

	t.Run("AppendAuditEvent messages", func(t *testing.T) {
		request := requireBE116AuditMessage(t, file, "AppendAuditEventRequest")
		requireBE116AuditExactFields(t, request, []be116AuditFieldSpec{
			{name: "audit_event_id", number: 1, kind: protoreflect.StringKind},
			{name: "space_id", number: 2, kind: protoreflect.StringKind},
			{name: "actor_profile_id", number: 3, kind: protoreflect.StringKind},
			{name: "action", number: 4, kind: protoreflect.StringKind},
			{name: "target_type", number: 5, kind: protoreflect.StringKind},
			{name: "target_id", number: 6, kind: protoreflect.StringKind},
			{name: "details_json", number: 7, kind: protoreflect.StringKind},
			{name: "occurred_at", number: 8, kind: protoreflect.MessageKind, typeName: "google.protobuf.Timestamp"},
		})

		response := requireBE116AuditMessage(t, file, "AppendAuditEventResponse")
		require.Zero(t, response.Fields().Len(), "%s must remain empty", response.FullName())
	})
}

func requireBE116AuditMessage(t *testing.T, file protoreflect.FileDescriptor, name protoreflect.Name) protoreflect.MessageDescriptor {
	t.Helper()
	message := file.Messages().ByName(name)
	require.NotNil(t, message, "%s must define %s", file.Path(), name)
	return message
}

func requireBE116AuditExactFields(t *testing.T, message protoreflect.MessageDescriptor, expected []be116AuditFieldSpec) {
	t.Helper()
	require.Equal(t, len(expected), message.Fields().Len(), "%s field allowlist is frozen", message.FullName())
	for _, spec := range expected {
		field := message.Fields().ByName(spec.name)
		require.NotNil(t, field, "%s must define %s", message.FullName(), spec.name)
		require.Equal(t, spec.number, field.Number(), "%s.%s field number", message.FullName(), spec.name)
		require.Equal(t, spec.kind, field.Kind(), "%s.%s wire kind", message.FullName(), spec.name)
		require.Equal(t, protoreflect.Optional, field.Cardinality(), "%s.%s cardinality", message.FullName(), spec.name)
		require.Equal(t, spec.explicitOptional, field.HasOptionalKeyword(), "%s.%s explicit optional", message.FullName(), spec.name)
		if spec.explicitOptional {
			require.NotNil(t, field.ContainingOneof(), "%s.%s optional presence", message.FullName(), spec.name)
			require.True(t, field.ContainingOneof().IsSynthetic(), "%s.%s must use proto3 optional presence", message.FullName(), spec.name)
		} else {
			require.Nil(t, field.ContainingOneof(), "%s.%s must be an ordinary proto3 field", message.FullName(), spec.name)
		}
		if spec.kind == protoreflect.MessageKind {
			require.NotNil(t, field.Message(), "%s.%s message descriptor", message.FullName(), spec.name)
			require.Equal(t, spec.typeName, field.Message().FullName(), "%s.%s message target", message.FullName(), spec.name)
		}
	}
}
