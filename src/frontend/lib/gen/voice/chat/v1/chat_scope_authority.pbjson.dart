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

@$core.Deprecated('Use spaceReportAttachmentDispositionDescriptor instead')
const SpaceReportAttachmentDisposition$json = {
  '1': 'SpaceReportAttachmentDisposition',
  '2': [
    {'1': 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN', '2': 1},
    {'1': 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED', '2': 2},
    {'1': 'SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED', '2': 3},
  ],
};

/// Descriptor for `SpaceReportAttachmentDisposition`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceReportAttachmentDispositionDescriptor = $convert.base64Decode(
    'CiBTcGFjZVJlcG9ydEF0dGFjaG1lbnREaXNwb3NpdGlvbhIzCi9TUEFDRV9SRVBPUlRfQVRUQU'
    'NITUVOVF9ESVNQT1NJVElPTl9VTlNQRUNJRklFRBAAEi8KK1NQQUNFX1JFUE9SVF9BVFRBQ0hN'
    'RU5UX0RJU1BPU0lUSU9OX1VOS05PV04QARIxCi1TUEFDRV9SRVBPUlRfQVRUQUNITUVOVF9ESV'
    'NQT1NJVElPTl9VTkNIQU5HRUQQAhIvCitTUEFDRV9SRVBPUlRfQVRUQUNITUVOVF9ESVNQT1NJ'
    'VElPTl9SRVRJUkVEEAM=');

@$core.Deprecated('Use spaceReportAttachmentRetirementReasonDescriptor instead')
const SpaceReportAttachmentRetirementReason$json = {
  '1': 'SpaceReportAttachmentRetirementReason',
  '2': [
    {'1': 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED', '2': 1},
    {'1': 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED', '2': 2},
  ],
};

/// Descriptor for `SpaceReportAttachmentRetirementReason`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceReportAttachmentRetirementReasonDescriptor =
    $convert.base64Decode(
        'CiVTcGFjZVJlcG9ydEF0dGFjaG1lbnRSZXRpcmVtZW50UmVhc29uEjkKNVNQQUNFX1JFUE9SVF'
        '9BVFRBQ0hNRU5UX1JFVElSRU1FTlRfUkVBU09OX1VOU1BFQ0lGSUVEEAASPAo4U1BBQ0VfUkVQ'
        'T1JUX0FUVEFDSE1FTlRfUkVUSVJFTUVOVF9SRUFTT05fVEFSR0VUX0RFTEVURUQQARI6CjZTUE'
        'FDRV9SRVBPUlRfQVRUQUNITUVOVF9SRVRJUkVNRU5UX1JFQVNPTl9UQVJHRVRfTU9WRUQQAg==');

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
    {
      '1': 'report_attachment_generation',
      '3': 14,
      '4': 1,
      '5': 4,
      '10': 'reportAttachmentGeneration'
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
    'VNYW5hZ2VkS2luZFILbWFuYWdlZEtpbmQSQAoccmVwb3J0X2F0dGFjaG1lbnRfZ2VuZXJhdGlv'
    'bhgOIAEoBFIacmVwb3J0QXR0YWNobWVudEdlbmVyYXRpb24=');

@$core.Deprecated('Use readSpaceReportAttachmentRequestDescriptor instead')
const ReadSpaceReportAttachmentRequest$json = {
  '1': 'ReadSpaceReportAttachmentRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'expected_space_id', '3': 4, '4': 1, '5': 9, '10': 'expectedSpaceId'},
    {
      '1': 'saved_attachment_generation',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'savedAttachmentGeneration'
    },
  ],
};

/// Descriptor for `ReadSpaceReportAttachmentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readSpaceReportAttachmentRequestDescriptor = $convert.base64Decode(
    'CiBSZWFkU3BhY2VSZXBvcnRBdHRhY2htZW50UmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
    'EgASgNUg9wcm90b2NvbFZlcnNpb24SIQoMb3BlcmF0aW9uX2lkGAIgASgJUgtvcGVyYXRpb25J'
    'ZBIXCgdjaGF0X2lkGAMgASgJUgZjaGF0SWQSKgoRZXhwZWN0ZWRfc3BhY2VfaWQYBCABKAlSD2'
    'V4cGVjdGVkU3BhY2VJZBI+ChtzYXZlZF9hdHRhY2htZW50X2dlbmVyYXRpb24YBSABKARSGXNh'
    'dmVkQXR0YWNobWVudEdlbmVyYXRpb24=');

@$core.Deprecated('Use readSpaceReportAttachmentResponseDescriptor instead')
const ReadSpaceReportAttachmentResponse$json = {
  '1': 'ReadSpaceReportAttachmentResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'expected_space_id', '3': 4, '4': 1, '5': 9, '10': 'expectedSpaceId'},
    {
      '1': 'saved_attachment_generation',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'savedAttachmentGeneration'
    },
    {
      '1': 'disposition',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.SpaceReportAttachmentDisposition',
      '10': 'disposition'
    },
    {
      '1': 'reason',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.SpaceReportAttachmentRetirementReason',
      '10': 'reason'
    },
    {'1': 'owner_event_id', '3': 8, '4': 1, '5': 9, '10': 'ownerEventId'},
    {'1': 'owner_revision', '3': 9, '4': 1, '5': 4, '10': 'ownerRevision'},
  ],
};

/// Descriptor for `ReadSpaceReportAttachmentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readSpaceReportAttachmentResponseDescriptor = $convert.base64Decode(
    'CiFSZWFkU3BhY2VSZXBvcnRBdHRhY2htZW50UmVzcG9uc2USKQoQcHJvdG9jb2xfdmVyc2lvbh'
    'gBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEiEKDG9wZXJhdGlvbl9pZBgCIAEoCVILb3BlcmF0aW9u'
    'SWQSFwoHY2hhdF9pZBgDIAEoCVIGY2hhdElkEioKEWV4cGVjdGVkX3NwYWNlX2lkGAQgASgJUg'
    '9leHBlY3RlZFNwYWNlSWQSPgobc2F2ZWRfYXR0YWNobWVudF9nZW5lcmF0aW9uGAUgASgEUhlz'
    'YXZlZEF0dGFjaG1lbnRHZW5lcmF0aW9uElEKC2Rpc3Bvc2l0aW9uGAYgASgOMi8udm9pY2UuY2'
    'hhdC52MS5TcGFjZVJlcG9ydEF0dGFjaG1lbnREaXNwb3NpdGlvblILZGlzcG9zaXRpb24STAoG'
    'cmVhc29uGAcgASgOMjQudm9pY2UuY2hhdC52MS5TcGFjZVJlcG9ydEF0dGFjaG1lbnRSZXRpcm'
    'VtZW50UmVhc29uUgZyZWFzb24SJAoOb3duZXJfZXZlbnRfaWQYCCABKAlSDG93bmVyRXZlbnRJ'
    'ZBIlCg5vd25lcl9yZXZpc2lvbhgJIAEoBFINb3duZXJSZXZpc2lvbg==');
