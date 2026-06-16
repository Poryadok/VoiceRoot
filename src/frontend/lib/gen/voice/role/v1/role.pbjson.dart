// This is a generated file - do not edit.
//
// Generated from voice/role/v1/role.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use bootstrapSpaceRolesRequestDescriptor instead')
const BootstrapSpaceRolesRequest$json = {
  '1': 'BootstrapSpaceRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'owner_profile_id', '3': 2, '4': 1, '5': 9, '10': 'ownerProfileId'},
  ],
};

/// Descriptor for `BootstrapSpaceRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bootstrapSpaceRolesRequestDescriptor =
    $convert.base64Decode(
        'ChpCb290c3RyYXBTcGFjZVJvbGVzUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZB'
        'IoChBvd25lcl9wcm9maWxlX2lkGAIgASgJUg5vd25lclByb2ZpbGVJZA==');

@$core.Deprecated('Use bootstrapSpaceRolesResponseDescriptor instead')
const BootstrapSpaceRolesResponse$json = {
  '1': 'BootstrapSpaceRolesResponse',
};

/// Descriptor for `BootstrapSpaceRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bootstrapSpaceRolesResponseDescriptor =
    $convert.base64Decode('ChtCb290c3RyYXBTcGFjZVJvbGVzUmVzcG9uc2U=');

@$core.Deprecated('Use deleteRolesCreatedByProfileRequestDescriptor instead')
const DeleteRolesCreatedByProfileRequest$json = {
  '1': 'DeleteRolesCreatedByProfileRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'created_by_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'createdByProfileId'
    },
  ],
};

/// Descriptor for `DeleteRolesCreatedByProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRolesCreatedByProfileRequestDescriptor =
    $convert.base64Decode(
        'CiJEZWxldGVSb2xlc0NyZWF0ZWRCeVByb2ZpbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUg'
        'dzcGFjZUlkEjEKFWNyZWF0ZWRfYnlfcHJvZmlsZV9pZBgCIAEoCVISY3JlYXRlZEJ5UHJvZmls'
        'ZUlk');

@$core.Deprecated('Use deleteRolesCreatedByProfileResponseDescriptor instead')
const DeleteRolesCreatedByProfileResponse$json = {
  '1': 'DeleteRolesCreatedByProfileResponse',
};

/// Descriptor for `DeleteRolesCreatedByProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRolesCreatedByProfileResponseDescriptor =
    $convert
        .base64Decode('CiNEZWxldGVSb2xlc0NyZWF0ZWRCeVByb2ZpbGVSZXNwb25zZQ==');

@$core.Deprecated('Use roleDescriptor instead')
const Role$json = {
  '1': 'Role',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'permissions_mask', '3': 4, '4': 1, '5': 4, '10': 'permissionsMask'},
    {'1': 'position', '3': 5, '4': 1, '5': 5, '10': 'position'},
    {'1': 'managed', '3': 6, '4': 1, '5': 8, '10': 'managed'},
    {
      '1': 'created_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'created_by_profile_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'createdByProfileId',
      '17': true
    },
  ],
  '8': [
    {'1': '_created_by_profile_id'},
  ],
};

/// Descriptor for `Role`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleDescriptor = $convert.base64Decode(
    'CgRSb2xlEg4KAmlkGAEgASgJUgJpZBIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBISCgRuYW'
    '1lGAMgASgJUgRuYW1lEikKEHBlcm1pc3Npb25zX21hc2sYBCABKARSD3Blcm1pc3Npb25zTWFz'
    'axIaCghwb3NpdGlvbhgFIAEoBVIIcG9zaXRpb24SGAoHbWFuYWdlZBgGIAEoCFIHbWFuYWdlZB'
    'I5CgpjcmVhdGVkX2F0GAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRl'
    'ZEF0EjYKFWNyZWF0ZWRfYnlfcHJvZmlsZV9pZBgIIAEoCUgAUhJjcmVhdGVkQnlQcm9maWxlSW'
    'SIAQFCGAoWX2NyZWF0ZWRfYnlfcHJvZmlsZV9pZA==');

@$core.Deprecated('Use createRoleRequestDescriptor instead')
const CreateRoleRequest$json = {
  '1': 'CreateRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'permissions_mask', '3': 3, '4': 1, '5': 4, '10': 'permissionsMask'},
    {'1': 'position', '3': 4, '4': 1, '5': 5, '10': 'position'},
  ],
};

/// Descriptor for `CreateRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createRoleRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBISCgRuYW1lGA'
    'IgASgJUgRuYW1lEikKEHBlcm1pc3Npb25zX21hc2sYAyABKARSD3Blcm1pc3Npb25zTWFzaxIa'
    'Cghwb3NpdGlvbhgEIAEoBVIIcG9zaXRpb24=');

@$core.Deprecated('Use updateRoleRequestDescriptor instead')
const UpdateRoleRequest$json = {
  '1': 'UpdateRoleRequest',
  '2': [
    {'1': 'role_id', '3': 1, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'permissions_mask',
      '3': 3,
      '4': 1,
      '5': 4,
      '9': 1,
      '10': 'permissionsMask',
      '17': true
    },
    {
      '1': 'position',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 2,
      '10': 'position',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_permissions_mask'},
    {'1': '_position'},
  ],
};

/// Descriptor for `UpdateRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateRoleRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVSb2xlUmVxdWVzdBIXCgdyb2xlX2lkGAEgASgJUgZyb2xlSWQSFwoEbmFtZRgCIA'
    'EoCUgAUgRuYW1liAEBEi4KEHBlcm1pc3Npb25zX21hc2sYAyABKARIAVIPcGVybWlzc2lvbnNN'
    'YXNriAEBEh8KCHBvc2l0aW9uGAQgASgFSAJSCHBvc2l0aW9uiAEBQgcKBV9uYW1lQhMKEV9wZX'
    'JtaXNzaW9uc19tYXNrQgsKCV9wb3NpdGlvbg==');

@$core.Deprecated('Use deleteRoleRequestDescriptor instead')
const DeleteRoleRequest$json = {
  '1': 'DeleteRoleRequest',
  '2': [
    {'1': 'role_id', '3': 1, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `DeleteRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRoleRequestDescriptor = $convert.base64Decode(
    'ChFEZWxldGVSb2xlUmVxdWVzdBIXCgdyb2xlX2lkGAEgASgJUgZyb2xlSWQ=');

@$core.Deprecated('Use listRolesRequestDescriptor instead')
const ListRolesRequest$json = {
  '1': 'ListRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRolesRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0Um9sZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlk');

@$core.Deprecated('Use roleListDescriptor instead')
const RoleList$json = {
  '1': 'RoleList',
  '2': [
    {
      '1': 'roles',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'roles'
    },
  ],
};

/// Descriptor for `RoleList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleListDescriptor = $convert.base64Decode(
    'CghSb2xlTGlzdBIpCgVyb2xlcxgBIAMoCzITLnZvaWNlLnJvbGUudjEuUm9sZVIFcm9sZXM=');

@$core.Deprecated('Use reorderRolesRequestDescriptor instead')
const ReorderRolesRequest$json = {
  '1': 'ReorderRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'ordered_role_ids', '3': 2, '4': 3, '5': 9, '10': 'orderedRoleIds'},
  ],
};

/// Descriptor for `ReorderRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderRolesRequestDescriptor = $convert.base64Decode(
    'ChNSZW9yZGVyUm9sZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEigKEG9yZG'
    'VyZWRfcm9sZV9pZHMYAiADKAlSDm9yZGVyZWRSb2xlSWRz');

@$core.Deprecated('Use assignRoleRequestDescriptor instead')
const AssignRoleRequest$json = {
  '1': 'AssignRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `AssignRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignRoleRequestDescriptor = $convert.base64Decode(
    'ChFBc3NpZ25Sb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use revokeRoleRequestDescriptor instead')
const RevokeRoleRequest$json = {
  '1': 'RevokeRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RevokeRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeRoleRequestDescriptor = $convert.base64Decode(
    'ChFSZXZva2VSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use getMemberRolesRequestDescriptor instead')
const GetMemberRolesRequest$json = {
  '1': 'GetMemberRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetMemberRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberRolesRequestDescriptor = $convert.base64Decode(
    'ChVHZXRNZW1iZXJSb2xlc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHQoKcH'
    'JvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use setChatOverrideRequestDescriptor instead')
const SetChatOverrideRequest$json = {
  '1': 'SetChatOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetChatOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatOverrideRequestDescriptor = $convert.base64Decode(
    'ChZTZXRDaGF0T3ZlcnJpZGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEioKBG'
    'NoYXQYAiABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSGwoJZGVueV9tYXNrGAMg'
    'ASgEUghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd01hc2sSFwoHcm9sZV9pZB'
    'gFIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use removeChatOverrideRequestDescriptor instead')
const RemoveChatOverrideRequest$json = {
  '1': 'RemoveChatOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RemoveChatOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeChatOverrideRequestDescriptor = $convert.base64Decode(
    'ChlSZW1vdmVDaGF0T3ZlcnJpZGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEi'
    'oKBGNoYXQYAiABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSFwoHcm9sZV9pZBgD'
    'IAEoCVIGcm9sZUlk');

@$core.Deprecated('Use getChatOverridesRequestDescriptor instead')
const GetChatOverridesRequest$json = {
  '1': 'GetChatOverridesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'filter_chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'filterChat',
      '17': true
    },
  ],
  '8': [
    {'1': '_filter_chat'},
  ],
};

/// Descriptor for `GetChatOverridesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatOverridesRequestDescriptor = $convert.base64Decode(
    'ChdHZXRDaGF0T3ZlcnJpZGVzUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBI8Cg'
    'tmaWx0ZXJfY2hhdBgCIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZkgAUgpmaWx0ZXJDaGF0'
    'iAEBQg4KDF9maWx0ZXJfY2hhdA==');

@$core.Deprecated('Use setVoiceRoomOverrideRequestDescriptor instead')
const SetVoiceRoomOverrideRequest$json = {
  '1': 'SetVoiceRoomOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetVoiceRoomOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVoiceRoomOverrideRequestDescriptor = $convert.base64Decode(
    'ChtTZXRWb2ljZVJvb21PdmVycmlkZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSW'
    'QSIgoNdm9pY2Vfcm9vbV9pZBgCIAEoCVILdm9pY2VSb29tSWQSGwoJZGVueV9tYXNrGAMgASgE'
    'UghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd01hc2sSFwoHcm9sZV9pZBgFIA'
    'EoCVIGcm9sZUlk');

@$core.Deprecated('Use removeVoiceRoomOverrideRequestDescriptor instead')
const RemoveVoiceRoomOverrideRequest$json = {
  '1': 'RemoveVoiceRoomOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RemoveVoiceRoomOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeVoiceRoomOverrideRequestDescriptor =
    $convert.base64Decode(
        'Ch5SZW1vdmVWb2ljZVJvb21PdmVycmlkZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYW'
        'NlSWQSIgoNdm9pY2Vfcm9vbV9pZBgCIAEoCVILdm9pY2VSb29tSWQSFwoHcm9sZV9pZBgDIAEo'
        'CVIGcm9sZUlk');

@$core.Deprecated('Use getVoiceRoomOverridesRequestDescriptor instead')
const GetVoiceRoomOverridesRequest$json = {
  '1': 'GetVoiceRoomOverridesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'voice_room_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `GetVoiceRoomOverridesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceRoomOverridesRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRWb2ljZVJvb21PdmVycmlkZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZU'
        'lkEicKDXZvaWNlX3Jvb21faWQYAiABKAlIAFILdm9pY2VSb29tSWSIAQFCEAoOX3ZvaWNlX3Jv'
        'b21faWQ=');

@$core.Deprecated('Use overrideListDescriptor instead')
const OverrideList$json = {
  '1': 'OverrideList',
  '2': [
    {
      '1': 'overrides',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.role.v1.PermissionOverride',
      '10': 'overrides'
    },
  ],
};

/// Descriptor for `OverrideList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List overrideListDescriptor = $convert.base64Decode(
    'CgxPdmVycmlkZUxpc3QSPwoJb3ZlcnJpZGVzGAEgAygLMiEudm9pY2Uucm9sZS52MS5QZXJtaX'
    'NzaW9uT3ZlcnJpZGVSCW92ZXJyaWRlcw==');

@$core.Deprecated('Use permissionOverrideDescriptor instead')
const PermissionOverride$json = {
  '1': 'PermissionOverride',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'role_name', '3': 6, '4': 1, '5': 9, '10': 'roleName'},
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `PermissionOverride`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List permissionOverrideDescriptor = $convert.base64Decode(
    'ChJQZXJtaXNzaW9uT3ZlcnJpZGUSLwoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZkgAUgRjaGF0iAEBEicKDXZvaWNlX3Jvb21faWQYAiABKAlIAVILdm9pY2VSb29tSWSIAQES'
    'GwoJZGVueV9tYXNrGAMgASgEUghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd0'
    '1hc2sSFwoHcm9sZV9pZBgFIAEoCVIGcm9sZUlkEhsKCXJvbGVfbmFtZRgGIAEoCVIIcm9sZU5h'
    'bWVCBwoFX2NoYXRCEAoOX3ZvaWNlX3Jvb21faWQ=');

@$core.Deprecated('Use setDefaultJoinRoleRequestDescriptor instead')
const SetDefaultJoinRoleRequest$json = {
  '1': 'SetDefaultJoinRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'role_id', '3': 2, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetDefaultJoinRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setDefaultJoinRoleRequestDescriptor =
    $convert.base64Decode(
        'ChlTZXREZWZhdWx0Sm9pblJvbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh'
        'cKB3JvbGVfaWQYAiABKAlSBnJvbGVJZA==');

@$core.Deprecated('Use getDefaultJoinRoleRequestDescriptor instead')
const GetDefaultJoinRoleRequest$json = {
  '1': 'GetDefaultJoinRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `GetDefaultJoinRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDefaultJoinRoleRequestDescriptor =
    $convert.base64Decode(
        'ChlHZXREZWZhdWx0Sm9pblJvbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlk');

@$core.Deprecated('Use checkPermissionRequestDescriptor instead')
const CheckPermissionRequest$json = {
  '1': 'CheckPermissionRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'permission_name', '3': 3, '4': 1, '5': 9, '10': 'permissionName'},
    {
      '1': 'chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `CheckPermissionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkPermissionRequestDescriptor = $convert.base64Decode(
    'ChZDaGVja1Blcm1pc3Npb25SZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh0KCn'
    'Byb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBInCg9wZXJtaXNzaW9uX25hbWUYAyABKAlSDnBl'
    'cm1pc3Npb25OYW1lEi8KBGNoYXQYBCABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZIAFIEY2'
    'hhdIgBARInCg12b2ljZV9yb29tX2lkGAUgASgJSAFSC3ZvaWNlUm9vbUlkiAEBQgcKBV9jaGF0'
    'QhAKDl92b2ljZV9yb29tX2lk');

@$core.Deprecated('Use checkPermissionResponseDescriptor instead')
const CheckPermissionResponse$json = {
  '1': 'CheckPermissionResponse',
  '2': [
    {'1': 'allowed', '3': 1, '4': 1, '5': 8, '10': 'allowed'},
  ],
};

/// Descriptor for `CheckPermissionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkPermissionResponseDescriptor =
    $convert.base64Decode(
        'ChdDaGVja1Blcm1pc3Npb25SZXNwb25zZRIYCgdhbGxvd2VkGAEgASgIUgdhbGxvd2Vk');

@$core.Deprecated('Use getEffectivePermissionsRequestDescriptor instead')
const GetEffectivePermissionsRequest$json = {
  '1': 'GetEffectivePermissionsRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'chat',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `GetEffectivePermissionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getEffectivePermissionsRequestDescriptor =
    $convert.base64Decode(
        'Ch5HZXRFZmZlY3RpdmVQZXJtaXNzaW9uc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYW'
        'NlSWQSHQoKcHJvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlkEi8KBGNoYXQYAyABKAsyFi52b2lj'
        'ZS5jaGF0LnYxLkNoYXRSZWZIAFIEY2hhdIgBARInCg12b2ljZV9yb29tX2lkGAQgASgJSAFSC3'
        'ZvaWNlUm9vbUlkiAEBQgcKBV9jaGF0QhAKDl92b2ljZV9yb29tX2lk');

@$core.Deprecated('Use permissionSetDescriptor instead')
const PermissionSet$json = {
  '1': 'PermissionSet',
  '2': [
    {'1': 'effective_mask', '3': 1, '4': 1, '5': 4, '10': 'effectiveMask'},
    {'1': 'permission_names', '3': 2, '4': 3, '5': 9, '10': 'permissionNames'},
  ],
};

/// Descriptor for `PermissionSet`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List permissionSetDescriptor = $convert.base64Decode(
    'Cg1QZXJtaXNzaW9uU2V0EiUKDmVmZmVjdGl2ZV9tYXNrGAEgASgEUg1lZmZlY3RpdmVNYXNrEi'
    'kKEHBlcm1pc3Npb25fbmFtZXMYAiADKAlSD3Blcm1pc3Npb25OYW1lcw==');

@$core.Deprecated('Use createRoleResponseDescriptor instead')
const CreateRoleResponse$json = {
  '1': 'CreateRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `CreateRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createRoleResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVSb2xlUmVzcG9uc2USJwoEcm9sZRgBIAEoCzITLnZvaWNlLnJvbGUudjEuUm9sZV'
    'IEcm9sZQ==');

@$core.Deprecated('Use updateRoleResponseDescriptor instead')
const UpdateRoleResponse$json = {
  '1': 'UpdateRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `UpdateRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateRoleResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVSb2xlUmVzcG9uc2USJwoEcm9sZRgBIAEoCzITLnZvaWNlLnJvbGUudjEuUm9sZV'
    'IEcm9sZQ==');

@$core.Deprecated('Use deleteRoleResponseDescriptor instead')
const DeleteRoleResponse$json = {
  '1': 'DeleteRoleResponse',
};

/// Descriptor for `DeleteRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRoleResponseDescriptor =
    $convert.base64Decode('ChJEZWxldGVSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use listRolesResponseDescriptor instead')
const ListRolesResponse$json = {
  '1': 'ListRolesResponse',
  '2': [
    {
      '1': 'role_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.RoleList',
      '10': 'roleList'
    },
  ],
};

/// Descriptor for `ListRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRolesResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0Um9sZXNSZXNwb25zZRI0Cglyb2xlX2xpc3QYASABKAsyFy52b2ljZS5yb2xlLnYxLl'
    'JvbGVMaXN0Ughyb2xlTGlzdA==');

@$core.Deprecated('Use reorderRolesResponseDescriptor instead')
const ReorderRolesResponse$json = {
  '1': 'ReorderRolesResponse',
};

/// Descriptor for `ReorderRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderRolesResponseDescriptor =
    $convert.base64Decode('ChRSZW9yZGVyUm9sZXNSZXNwb25zZQ==');

@$core.Deprecated('Use assignRoleResponseDescriptor instead')
const AssignRoleResponse$json = {
  '1': 'AssignRoleResponse',
};

/// Descriptor for `AssignRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignRoleResponseDescriptor =
    $convert.base64Decode('ChJBc3NpZ25Sb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use revokeRoleResponseDescriptor instead')
const RevokeRoleResponse$json = {
  '1': 'RevokeRoleResponse',
};

/// Descriptor for `RevokeRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeRoleResponseDescriptor =
    $convert.base64Decode('ChJSZXZva2VSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use getMemberRolesResponseDescriptor instead')
const GetMemberRolesResponse$json = {
  '1': 'GetMemberRolesResponse',
  '2': [
    {
      '1': 'role_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.RoleList',
      '10': 'roleList'
    },
  ],
};

/// Descriptor for `GetMemberRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberRolesResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRNZW1iZXJSb2xlc1Jlc3BvbnNlEjQKCXJvbGVfbGlzdBgBIAEoCzIXLnZvaWNlLnJvbG'
        'UudjEuUm9sZUxpc3RSCHJvbGVMaXN0');

@$core.Deprecated('Use setChatOverrideResponseDescriptor instead')
const SetChatOverrideResponse$json = {
  '1': 'SetChatOverrideResponse',
};

/// Descriptor for `SetChatOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatOverrideResponseDescriptor =
    $convert.base64Decode('ChdTZXRDaGF0T3ZlcnJpZGVSZXNwb25zZQ==');

@$core.Deprecated('Use removeChatOverrideResponseDescriptor instead')
const RemoveChatOverrideResponse$json = {
  '1': 'RemoveChatOverrideResponse',
};

/// Descriptor for `RemoveChatOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeChatOverrideResponseDescriptor =
    $convert.base64Decode('ChpSZW1vdmVDaGF0T3ZlcnJpZGVSZXNwb25zZQ==');

@$core.Deprecated('Use getChatOverridesResponseDescriptor instead')
const GetChatOverridesResponse$json = {
  '1': 'GetChatOverridesResponse',
  '2': [
    {
      '1': 'override_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OverrideList',
      '10': 'overrideList'
    },
  ],
};

/// Descriptor for `GetChatOverridesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatOverridesResponseDescriptor =
    $convert.base64Decode(
        'ChhHZXRDaGF0T3ZlcnJpZGVzUmVzcG9uc2USQAoNb3ZlcnJpZGVfbGlzdBgBIAEoCzIbLnZvaW'
        'NlLnJvbGUudjEuT3ZlcnJpZGVMaXN0UgxvdmVycmlkZUxpc3Q=');

@$core.Deprecated('Use setVoiceRoomOverrideResponseDescriptor instead')
const SetVoiceRoomOverrideResponse$json = {
  '1': 'SetVoiceRoomOverrideResponse',
};

/// Descriptor for `SetVoiceRoomOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVoiceRoomOverrideResponseDescriptor =
    $convert.base64Decode('ChxTZXRWb2ljZVJvb21PdmVycmlkZVJlc3BvbnNl');

@$core.Deprecated('Use removeVoiceRoomOverrideResponseDescriptor instead')
const RemoveVoiceRoomOverrideResponse$json = {
  '1': 'RemoveVoiceRoomOverrideResponse',
};

/// Descriptor for `RemoveVoiceRoomOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeVoiceRoomOverrideResponseDescriptor =
    $convert.base64Decode('Ch9SZW1vdmVWb2ljZVJvb21PdmVycmlkZVJlc3BvbnNl');

@$core.Deprecated('Use getVoiceRoomOverridesResponseDescriptor instead')
const GetVoiceRoomOverridesResponse$json = {
  '1': 'GetVoiceRoomOverridesResponse',
  '2': [
    {
      '1': 'override_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OverrideList',
      '10': 'overrideList'
    },
  ],
};

/// Descriptor for `GetVoiceRoomOverridesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceRoomOverridesResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRWb2ljZVJvb21PdmVycmlkZXNSZXNwb25zZRJACg1vdmVycmlkZV9saXN0GAEgASgLMh'
        'sudm9pY2Uucm9sZS52MS5PdmVycmlkZUxpc3RSDG92ZXJyaWRlTGlzdA==');

@$core.Deprecated('Use getEffectivePermissionsResponseDescriptor instead')
const GetEffectivePermissionsResponse$json = {
  '1': 'GetEffectivePermissionsResponse',
  '2': [
    {
      '1': 'permission_set',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.PermissionSet',
      '10': 'permissionSet'
    },
  ],
};

/// Descriptor for `GetEffectivePermissionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getEffectivePermissionsResponseDescriptor =
    $convert.base64Decode(
        'Ch9HZXRFZmZlY3RpdmVQZXJtaXNzaW9uc1Jlc3BvbnNlEkMKDnBlcm1pc3Npb25fc2V0GAEgAS'
        'gLMhwudm9pY2Uucm9sZS52MS5QZXJtaXNzaW9uU2V0Ug1wZXJtaXNzaW9uU2V0');

@$core.Deprecated('Use setDefaultJoinRoleResponseDescriptor instead')
const SetDefaultJoinRoleResponse$json = {
  '1': 'SetDefaultJoinRoleResponse',
};

/// Descriptor for `SetDefaultJoinRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setDefaultJoinRoleResponseDescriptor =
    $convert.base64Decode('ChpTZXREZWZhdWx0Sm9pblJvbGVSZXNwb25zZQ==');

@$core.Deprecated('Use getDefaultJoinRoleResponseDescriptor instead')
const GetDefaultJoinRoleResponse$json = {
  '1': 'GetDefaultJoinRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `GetDefaultJoinRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDefaultJoinRoleResponseDescriptor =
    $convert.base64Decode(
        'ChpHZXREZWZhdWx0Sm9pblJvbGVSZXNwb25zZRInCgRyb2xlGAEgASgLMhMudm9pY2Uucm9sZS'
        '52MS5Sb2xlUgRyb2xl');
