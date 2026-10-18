// This is a generated file - do not edit.
//
// Generated from voice/role/v1/space_chat_permissions.proto.

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

@$core.Deprecated('Use getChatPermissionStateRequestDescriptor instead')
const GetChatPermissionStateRequest$json = {
  '1': 'GetChatPermissionStateRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'subject_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'subjectProfileId'
    },
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `GetChatPermissionStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatPermissionStateRequestDescriptor = $convert.base64Decode(
    'Ch1HZXRDaGF0UGVybWlzc2lvblN0YXRlUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
    'gNUg9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSFwoHY2hhdF9p'
    'ZBgDIAEoCVIGY2hhdElkEiwKEnN1YmplY3RfcHJvZmlsZV9pZBgEIAEoCVIQc3ViamVjdFByb2'
    'ZpbGVJZBIhCgxvcGVyYXRpb25faWQYBSABKAlSC29wZXJhdGlvbklk');

@$core.Deprecated('Use spaceChatMembershipGrantDescriptor instead')
const SpaceChatMembershipGrant$json = {
  '1': 'SpaceChatMembershipGrant',
  '2': [
    {
      '1': 'membership_binding',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'membershipBinding'
    },
    {
      '1': 'explicit_post_grant',
      '3': 2,
      '4': 1,
      '5': 8,
      '10': 'explicitPostGrant'
    },
  ],
};

/// Descriptor for `SpaceChatMembershipGrant`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceChatMembershipGrantDescriptor = $convert.base64Decode(
    'ChhTcGFjZUNoYXRNZW1iZXJzaGlwR3JhbnQSTwoSbWVtYmVyc2hpcF9iaW5kaW5nGAEgASgLMi'
    'Audm9pY2Uucm9sZS52MS5NZW1iZXJzaGlwQmluZGluZ1IRbWVtYmVyc2hpcEJpbmRpbmcSLgoT'
    'ZXhwbGljaXRfcG9zdF9ncmFudBgCIAEoCFIRZXhwbGljaXRQb3N0R3JhbnQ=');

@$core.Deprecated('Use getChatPermissionStateResponseDescriptor instead')
const GetChatPermissionStateResponse$json = {
  '1': 'GetChatPermissionStateResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'subject_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'subjectProfileId'
    },
    {
      '1': 'chat_scope_incarnation',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'chatScopeIncarnation'
    },
    {'1': 'can_post_as_chat', '3': 6, '4': 1, '5': 8, '10': 'canPostAsChat'},
    {'1': 'can_add_members', '3': 7, '4': 1, '5': 8, '10': 'canAddMembers'},
    {
      '1': 'can_manage_permissions',
      '3': 8,
      '4': 1,
      '5': 8,
      '10': 'canManagePermissions'
    },
    {
      '1': 'member_additions_enabled',
      '3': 9,
      '4': 1,
      '5': 8,
      '10': 'memberAdditionsEnabled'
    },
    {
      '1': 'effective_explicit_post_grant',
      '3': 10,
      '4': 1,
      '5': 8,
      '10': 'effectiveExplicitPostGrant'
    },
    {
      '1': 'membership_grants',
      '3': 11,
      '4': 3,
      '5': 11,
      '6': '.voice.role.v1.SpaceChatMembershipGrant',
      '10': 'membershipGrants'
    },
    {
      '1': 'role_authorization_revision',
      '3': 12,
      '4': 1,
      '5': 4,
      '10': 'roleAuthorizationRevision'
    },
    {
      '1': 'space_authority_revision',
      '3': 13,
      '4': 1,
      '5': 4,
      '10': 'spaceAuthorityRevision'
    },
    {
      '1': 'chat_authority_revision',
      '3': 14,
      '4': 1,
      '5': 4,
      '10': 'chatAuthorityRevision'
    },
    {
      '1': 'space_lifecycle_generation',
      '3': 15,
      '4': 1,
      '5': 4,
      '10': 'spaceLifecycleGeneration'
    },
    {
      '1': 'ownership_generation',
      '3': 16,
      '4': 1,
      '5': 4,
      '10': 'ownershipGeneration'
    },
    {
      '1': 'chat_lifecycle_generation',
      '3': 17,
      '4': 1,
      '5': 4,
      '10': 'chatLifecycleGeneration'
    },
  ],
};

/// Descriptor for `GetChatPermissionStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatPermissionStateResponseDescriptor = $convert.base64Decode(
    'Ch5HZXRDaGF0UGVybWlzc2lvblN0YXRlUmVzcG9uc2USKQoQcHJvdG9jb2xfdmVyc2lvbhgBIA'
    'EoDVIPcHJvdG9jb2xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEhcKB2NoYXRf'
    'aWQYAyABKAlSBmNoYXRJZBIsChJzdWJqZWN0X3Byb2ZpbGVfaWQYBCABKAlSEHN1YmplY3RQcm'
    '9maWxlSWQSNAoWY2hhdF9zY29wZV9pbmNhcm5hdGlvbhgFIAEoCVIUY2hhdFNjb3BlSW5jYXJu'
    'YXRpb24SJwoQY2FuX3Bvc3RfYXNfY2hhdBgGIAEoCFINY2FuUG9zdEFzQ2hhdBImCg9jYW5fYW'
    'RkX21lbWJlcnMYByABKAhSDWNhbkFkZE1lbWJlcnMSNAoWY2FuX21hbmFnZV9wZXJtaXNzaW9u'
    'cxgIIAEoCFIUY2FuTWFuYWdlUGVybWlzc2lvbnMSOAoYbWVtYmVyX2FkZGl0aW9uc19lbmFibG'
    'VkGAkgASgIUhZtZW1iZXJBZGRpdGlvbnNFbmFibGVkEkEKHWVmZmVjdGl2ZV9leHBsaWNpdF9w'
    'b3N0X2dyYW50GAogASgIUhplZmZlY3RpdmVFeHBsaWNpdFBvc3RHcmFudBJUChFtZW1iZXJzaG'
    'lwX2dyYW50cxgLIAMoCzInLnZvaWNlLnJvbGUudjEuU3BhY2VDaGF0TWVtYmVyc2hpcEdyYW50'
    'UhBtZW1iZXJzaGlwR3JhbnRzEj4KG3JvbGVfYXV0aG9yaXphdGlvbl9yZXZpc2lvbhgMIAEoBF'
    'IZcm9sZUF1dGhvcml6YXRpb25SZXZpc2lvbhI4ChhzcGFjZV9hdXRob3JpdHlfcmV2aXNpb24Y'
    'DSABKARSFnNwYWNlQXV0aG9yaXR5UmV2aXNpb24SNgoXY2hhdF9hdXRob3JpdHlfcmV2aXNpb2'
    '4YDiABKARSFWNoYXRBdXRob3JpdHlSZXZpc2lvbhI8ChpzcGFjZV9saWZlY3ljbGVfZ2VuZXJh'
    'dGlvbhgPIAEoBFIYc3BhY2VMaWZlY3ljbGVHZW5lcmF0aW9uEjEKFG93bmVyc2hpcF9nZW5lcm'
    'F0aW9uGBAgASgEUhNvd25lcnNoaXBHZW5lcmF0aW9uEjoKGWNoYXRfbGlmZWN5Y2xlX2dlbmVy'
    'YXRpb24YESABKARSF2NoYXRMaWZlY3ljbGVHZW5lcmF0aW9u');

@$core.Deprecated('Use setSpaceChatAddMembersRequestDescriptor instead')
const SetSpaceChatAddMembersRequest$json = {
  '1': 'SetSpaceChatAddMembersRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'enabled', '3': 4, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'expected_chat_scope_incarnation',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'expectedChatScopeIncarnation'
    },
  ],
};

/// Descriptor for `SetSpaceChatAddMembersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setSpaceChatAddMembersRequestDescriptor = $convert.base64Decode(
    'Ch1TZXRTcGFjZUNoYXRBZGRNZW1iZXJzUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
    'gNUg9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSFwoHY2hhdF9p'
    'ZBgDIAEoCVIGY2hhdElkEhgKB2VuYWJsZWQYBCABKAhSB2VuYWJsZWQSIQoMb3BlcmF0aW9uX2'
    'lkGAUgASgJUgtvcGVyYXRpb25JZBJFCh9leHBlY3RlZF9jaGF0X3Njb3BlX2luY2FybmF0aW9u'
    'GAYgASgJUhxleHBlY3RlZENoYXRTY29wZUluY2FybmF0aW9u');

@$core.Deprecated('Use setSpaceChatPostAsChatGrantRequestDescriptor instead')
const SetSpaceChatPostAsChatGrantRequest$json = {
  '1': 'SetSpaceChatPostAsChatGrantRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'target_profile_id', '3': 4, '4': 1, '5': 9, '10': 'targetProfileId'},
    {'1': 'allowed', '3': 5, '4': 1, '5': 8, '10': 'allowed'},
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'expected_chat_scope_incarnation',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'expectedChatScopeIncarnation'
    },
    {
      '1': 'target_membership_binding',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'targetMembershipBinding'
    },
  ],
};

/// Descriptor for `SetSpaceChatPostAsChatGrantRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setSpaceChatPostAsChatGrantRequestDescriptor = $convert.base64Decode(
    'CiJTZXRTcGFjZUNoYXRQb3N0QXNDaGF0R3JhbnRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBIXCgdj'
    'aGF0X2lkGAMgASgJUgZjaGF0SWQSKgoRdGFyZ2V0X3Byb2ZpbGVfaWQYBCABKAlSD3RhcmdldF'
    'Byb2ZpbGVJZBIYCgdhbGxvd2VkGAUgASgIUgdhbGxvd2VkEiEKDG9wZXJhdGlvbl9pZBgGIAEo'
    'CVILb3BlcmF0aW9uSWQSRQofZXhwZWN0ZWRfY2hhdF9zY29wZV9pbmNhcm5hdGlvbhgHIAEoCV'
    'IcZXhwZWN0ZWRDaGF0U2NvcGVJbmNhcm5hdGlvbhJcChl0YXJnZXRfbWVtYmVyc2hpcF9iaW5k'
    'aW5nGAggASgLMiAudm9pY2Uucm9sZS52MS5NZW1iZXJzaGlwQmluZGluZ1IXdGFyZ2V0TWVtYm'
    'Vyc2hpcEJpbmRpbmc=');

@$core.Deprecated('Use spaceChatMutationOutcomeDescriptor instead')
const SpaceChatMutationOutcome$json = {
  '1': 'SpaceChatMutationOutcome',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'chat_scope_incarnation',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'chatScopeIncarnation'
    },
    {'1': 'applied_value', '3': 4, '4': 1, '5': 8, '10': 'appliedValue'},
    {
      '1': 'selected_membership_binding',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'selectedMembershipBinding'
    },
    {
      '1': 'role_authorization_revision',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'roleAuthorizationRevision'
    },
    {'1': 'historical', '3': 7, '4': 1, '5': 8, '10': 'historical'},
  ],
};

/// Descriptor for `SpaceChatMutationOutcome`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceChatMutationOutcomeDescriptor = $convert.base64Decode(
    'ChhTcGFjZUNoYXRNdXRhdGlvbk91dGNvbWUSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcH'
    'JvdG9jb2xWZXJzaW9uEiEKDG9wZXJhdGlvbl9pZBgCIAEoCVILb3BlcmF0aW9uSWQSNAoWY2hh'
    'dF9zY29wZV9pbmNhcm5hdGlvbhgDIAEoCVIUY2hhdFNjb3BlSW5jYXJuYXRpb24SIwoNYXBwbG'
    'llZF92YWx1ZRgEIAEoCFIMYXBwbGllZFZhbHVlEmAKG3NlbGVjdGVkX21lbWJlcnNoaXBfYmlu'
    'ZGluZxgFIAEoCzIgLnZvaWNlLnJvbGUudjEuTWVtYmVyc2hpcEJpbmRpbmdSGXNlbGVjdGVkTW'
    'VtYmVyc2hpcEJpbmRpbmcSPgobcm9sZV9hdXRob3JpemF0aW9uX3JldmlzaW9uGAYgASgEUhly'
    'b2xlQXV0aG9yaXphdGlvblJldmlzaW9uEh4KCmhpc3RvcmljYWwYByABKAhSCmhpc3RvcmljYW'
    'w=');

@$core.Deprecated('Use setSpaceChatAddMembersResponseDescriptor instead')
const SetSpaceChatAddMembersResponse$json = {
  '1': 'SetSpaceChatAddMembersResponse',
  '2': [
    {
      '1': 'outcome',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.SpaceChatMutationOutcome',
      '10': 'outcome'
    },
  ],
};

/// Descriptor for `SetSpaceChatAddMembersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setSpaceChatAddMembersResponseDescriptor =
    $convert.base64Decode(
        'Ch5TZXRTcGFjZUNoYXRBZGRNZW1iZXJzUmVzcG9uc2USQQoHb3V0Y29tZRgBIAEoCzInLnZvaW'
        'NlLnJvbGUudjEuU3BhY2VDaGF0TXV0YXRpb25PdXRjb21lUgdvdXRjb21l');

@$core.Deprecated('Use setSpaceChatPostAsChatGrantResponseDescriptor instead')
const SetSpaceChatPostAsChatGrantResponse$json = {
  '1': 'SetSpaceChatPostAsChatGrantResponse',
  '2': [
    {
      '1': 'outcome',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.SpaceChatMutationOutcome',
      '10': 'outcome'
    },
  ],
};

/// Descriptor for `SetSpaceChatPostAsChatGrantResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setSpaceChatPostAsChatGrantResponseDescriptor =
    $convert.base64Decode(
        'CiNTZXRTcGFjZUNoYXRQb3N0QXNDaGF0R3JhbnRSZXNwb25zZRJBCgdvdXRjb21lGAEgASgLMi'
        'cudm9pY2Uucm9sZS52MS5TcGFjZUNoYXRNdXRhdGlvbk91dGNvbWVSB291dGNvbWU=');
