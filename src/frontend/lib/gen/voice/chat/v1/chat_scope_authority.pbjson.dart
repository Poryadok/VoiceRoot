// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_scope_authority.proto.

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

@$core.Deprecated('Use chatScopeManagedKindDescriptor instead')
const ChatScopeManagedKind$json = {
  '1': 'ChatScopeManagedKind',
  '2': [
    {'1': 'CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED', '2': 0},
    {'1': 'CHAT_SCOPE_MANAGED_KIND_ORDINARY', '2': 1},
    {'1': 'CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION', '2': 2},
    {'1': 'CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD', '2': 3},
  ],
};

/// Descriptor for `ChatScopeManagedKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List chatScopeManagedKindDescriptor = $convert.base64Decode(
    'ChRDaGF0U2NvcGVNYW5hZ2VkS2luZBInCiNDSEFUX1NDT1BFX01BTkFHRURfS0lORF9VTlNQRU'
    'NJRklFRBAAEiQKIENIQVRfU0NPUEVfTUFOQUdFRF9LSU5EX09SRElOQVJZEAESLAooQ0hBVF9T'
    'Q09QRV9NQU5BR0VEX0tJTkRfR0FNRV9JTlRFR1JBVElPThACEicKI0NIQVRfU0NPUEVfTUFOQU'
    'dFRF9LSU5EX01BVENIX1NRVUFEEAM=');

@$core.Deprecated('Use chatScopeLifecycleStateDescriptor instead')
const ChatScopeLifecycleState$json = {
  '1': 'ChatScopeLifecycleState',
  '2': [
    {'1': 'CHAT_SCOPE_LIFECYCLE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'CHAT_SCOPE_LIFECYCLE_STATE_LIVE', '2': 1},
    {'1': 'CHAT_SCOPE_LIFECYCLE_STATE_FROZEN', '2': 2},
    {'1': 'CHAT_SCOPE_LIFECYCLE_STATE_PURGE_DECIDED', '2': 3},
    {'1': 'CHAT_SCOPE_LIFECYCLE_STATE_PURGED', '2': 4},
  ],
};

/// Descriptor for `ChatScopeLifecycleState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List chatScopeLifecycleStateDescriptor = $convert.base64Decode(
    'ChdDaGF0U2NvcGVMaWZlY3ljbGVTdGF0ZRIqCiZDSEFUX1NDT1BFX0xJRkVDWUNMRV9TVEFURV'
    '9VTlNQRUNJRklFRBAAEiMKH0NIQVRfU0NPUEVfTElGRUNZQ0xFX1NUQVRFX0xJVkUQARIlCiFD'
    'SEFUX1NDT1BFX0xJRkVDWUNMRV9TVEFURV9GUk9aRU4QAhIsCihDSEFUX1NDT1BFX0xJRkVDWU'
    'NMRV9TVEFURV9QVVJHRV9ERUNJREVEEAMSJQohQ0hBVF9TQ09QRV9MSUZFQ1lDTEVfU1RBVEVf'
    'UFVSR0VEEAQ=');

@$core.Deprecated('Use getSpaceChatScopeAuthorityRequestDescriptor instead')
const GetSpaceChatScopeAuthorityRequest$json = {
  '1': 'GetSpaceChatScopeAuthorityRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'expected_space_id', '3': 3, '4': 1, '5': 9, '10': 'expectedSpaceId'},
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `GetSpaceChatScopeAuthorityRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceChatScopeAuthorityRequestDescriptor =
    $convert.base64Decode(
        'CiFHZXRTcGFjZUNoYXRTY29wZUF1dGhvcml0eVJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbh'
        'gBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEhcKB2NoYXRfaWQYAiABKAlSBmNoYXRJZBIqChFleHBl'
        'Y3RlZF9zcGFjZV9pZBgDIAEoCVIPZXhwZWN0ZWRTcGFjZUlkEiEKDG9wZXJhdGlvbl9pZBgEIA'
        'EoCVILb3BlcmF0aW9uSWQ=');

@$core.Deprecated('Use getSpaceChatScopeAuthorityResponseDescriptor instead')
const GetSpaceChatScopeAuthorityResponse$json = {
  '1': 'GetSpaceChatScopeAuthorityResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'expected_space_id', '3': 3, '4': 1, '5': 9, '10': 'expectedSpaceId'},
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'exists', '3': 5, '4': 1, '5': 8, '10': 'exists'},
    {'1': 'live', '3': 6, '4': 1, '5': 8, '10': 'live'},
    {
      '1': 'chat_type',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatType',
      '10': 'chatType'
    },
    {'1': 'actual_space_id', '3': 8, '4': 1, '5': 9, '10': 'actualSpaceId'},
    {
      '1': 'chat_scope_incarnation',
      '3': 9,
      '4': 1,
      '5': 9,
      '10': 'chatScopeIncarnation'
    },
    {
      '1': 'chat_authority_revision',
      '3': 10,
      '4': 1,
      '5': 4,
      '10': 'chatAuthorityRevision'
    },
    {
      '1': 'lifecycle_generation',
      '3': 11,
      '4': 1,
      '5': 4,
      '10': 'lifecycleGeneration'
    },
    {
      '1': 'lifecycle_state',
      '3': 12,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatScopeLifecycleState',
      '10': 'lifecycleState'
    },
    {
      '1': 'managed_kind',
      '3': 13,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatScopeManagedKind',
      '10': 'managedKind'
    },
  ],
};

/// Descriptor for `GetSpaceChatScopeAuthorityResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceChatScopeAuthorityResponseDescriptor = $convert.base64Decode(
    'CiJHZXRTcGFjZUNoYXRTY29wZUF1dGhvcml0eVJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIXCgdjaGF0X2lkGAIgASgJUgZjaGF0SWQSKgoRZXhw'
    'ZWN0ZWRfc3BhY2VfaWQYAyABKAlSD2V4cGVjdGVkU3BhY2VJZBIhCgxvcGVyYXRpb25faWQYBC'
    'ABKAlSC29wZXJhdGlvbklkEhYKBmV4aXN0cxgFIAEoCFIGZXhpc3RzEhIKBGxpdmUYBiABKAhS'
    'BGxpdmUSNAoJY2hhdF90eXBlGAcgASgOMhcudm9pY2UuY2hhdC52MS5DaGF0VHlwZVIIY2hhdF'
    'R5cGUSJgoPYWN0dWFsX3NwYWNlX2lkGAggASgJUg1hY3R1YWxTcGFjZUlkEjQKFmNoYXRfc2Nv'
    'cGVfaW5jYXJuYXRpb24YCSABKAlSFGNoYXRTY29wZUluY2FybmF0aW9uEjYKF2NoYXRfYXV0aG'
    '9yaXR5X3JldmlzaW9uGAogASgEUhVjaGF0QXV0aG9yaXR5UmV2aXNpb24SMQoUbGlmZWN5Y2xl'
    'X2dlbmVyYXRpb24YCyABKARSE2xpZmVjeWNsZUdlbmVyYXRpb24STwoPbGlmZWN5Y2xlX3N0YX'
    'RlGAwgASgOMiYudm9pY2UuY2hhdC52MS5DaGF0U2NvcGVMaWZlY3ljbGVTdGF0ZVIObGlmZWN5'
    'Y2xlU3RhdGUSRgoMbWFuYWdlZF9raW5kGA0gASgOMiMudm9pY2UuY2hhdC52MS5DaGF0U2NvcG'
    'VNYW5hZ2VkS2luZFILbWFuYWdlZEtpbmQ=');
