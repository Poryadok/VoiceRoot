// This is a generated file - do not edit.
//
// Generated from voice/common/v1/space_lifecycle.proto.

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

@$core.Deprecated('Use participantIdDescriptor instead')
const ParticipantId$json = {
  '1': 'ParticipantId',
  '2': [
    {'1': 'PARTICIPANT_ID_UNSPECIFIED', '2': 0},
    {'1': 'PARTICIPANT_ID_ROLE', '2': 1},
    {'1': 'PARTICIPANT_ID_CHAT', '2': 2},
    {'1': 'PARTICIPANT_ID_MESSAGING', '2': 3},
    {'1': 'PARTICIPANT_ID_FILE', '2': 4},
    {'1': 'PARTICIPANT_ID_VOICE', '2': 5},
    {'1': 'PARTICIPANT_ID_MATCHMAKING', '2': 6},
    {'1': 'PARTICIPANT_ID_SEARCH', '2': 7},
    {'1': 'PARTICIPANT_ID_SUBSCRIPTION', '2': 8},
    {'1': 'PARTICIPANT_ID_BOT', '2': 9},
    {'1': 'PARTICIPANT_ID_NOTIFICATION', '2': 10},
  ],
};

/// Descriptor for `ParticipantId`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List participantIdDescriptor = $convert.base64Decode(
    'Cg1QYXJ0aWNpcGFudElkEh4KGlBBUlRJQ0lQQU5UX0lEX1VOU1BFQ0lGSUVEEAASFwoTUEFSVE'
    'lDSVBBTlRfSURfUk9MRRABEhcKE1BBUlRJQ0lQQU5UX0lEX0NIQVQQAhIcChhQQVJUSUNJUEFO'
    'VF9JRF9NRVNTQUdJTkcQAxIXChNQQVJUSUNJUEFOVF9JRF9GSUxFEAQSGAoUUEFSVElDSVBBTl'
    'RfSURfVk9JQ0UQBRIeChpQQVJUSUNJUEFOVF9JRF9NQVRDSE1BS0lORxAGEhkKFVBBUlRJQ0lQ'
    'QU5UX0lEX1NFQVJDSBAHEh8KG1BBUlRJQ0lQQU5UX0lEX1NVQlNDUklQVElPThAIEhYKElBBUl'
    'RJQ0lQQU5UX0lEX0JPVBAJEh8KG1BBUlRJQ0lQQU5UX0lEX05PVElGSUNBVElPThAK');

@$core.Deprecated('Use lifecycleFenceStateDescriptor instead')
const LifecycleFenceState$json = {
  '1': 'LifecycleFenceState',
  '2': [
    {'1': 'LIFECYCLE_FENCE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'LIFECYCLE_FENCE_STATE_FROZEN', '2': 1},
    {'1': 'LIFECYCLE_FENCE_STATE_LIVE', '2': 2},
    {'1': 'LIFECYCLE_FENCE_STATE_PURGE_DECIDED', '2': 3},
  ],
};

/// Descriptor for `LifecycleFenceState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List lifecycleFenceStateDescriptor = $convert.base64Decode(
    'ChNMaWZlY3ljbGVGZW5jZVN0YXRlEiUKIUxJRkVDWUNMRV9GRU5DRV9TVEFURV9VTlNQRUNJRk'
    'lFRBAAEiAKHExJRkVDWUNMRV9GRU5DRV9TVEFURV9GUk9aRU4QARIeChpMSUZFQ1lDTEVfRkVO'
    'Q0VfU1RBVEVfTElWRRACEicKI0xJRkVDWUNMRV9GRU5DRV9TVEFURV9QVVJHRV9ERUNJREVEEA'
    'M=');

@$core.Deprecated('Use purgeReceiptStateDescriptor instead')
const PurgeReceiptState$json = {
  '1': 'PurgeReceiptState',
  '2': [
    {'1': 'PURGE_RECEIPT_STATE_UNSPECIFIED', '2': 0},
    {'1': 'PURGE_RECEIPT_STATE_COMPLETED', '2': 1},
  ],
};

/// Descriptor for `PurgeReceiptState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List purgeReceiptStateDescriptor = $convert.base64Decode(
    'ChFQdXJnZVJlY2VpcHRTdGF0ZRIjCh9QVVJHRV9SRUNFSVBUX1NUQVRFX1VOU1BFQ0lGSUVEEA'
    'ASIQodUFVSR0VfUkVDRUlQVF9TVEFURV9DT01QTEVURUQQAQ==');

@$core.Deprecated('Use manifestBindingDescriptor instead')
const ManifestBinding$json = {
  '1': 'ManifestBinding',
  '2': [
    {'1': 'manifest_id', '3': 1, '4': 1, '5': 9, '10': 'manifestId'},
    {'1': 'manifest_sha256', '3': 2, '4': 1, '5': 12, '10': 'manifestSha256'},
    {'1': 'item_count', '3': 3, '4': 1, '5': 4, '10': 'itemCount'},
  ],
};

/// Descriptor for `ManifestBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List manifestBindingDescriptor = $convert.base64Decode(
    'Cg9NYW5pZmVzdEJpbmRpbmcSHwoLbWFuaWZlc3RfaWQYASABKAlSCm1hbmlmZXN0SWQSJwoPbW'
    'FuaWZlc3Rfc2hhMjU2GAIgASgMUg5tYW5pZmVzdFNoYTI1NhIdCgppdGVtX2NvdW50GAMgASgE'
    'UglpdGVtQ291bnQ=');

@$core.Deprecated('Use spaceLifecycleFenceRequestDescriptor instead')
const SpaceLifecycleFenceRequest$json = {
  '1': 'SpaceLifecycleFenceRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 4, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'desired_state',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.LifecycleFenceState',
      '10': 'desiredState'
    },
    {
      '1': 'manifest',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.ManifestBinding',
      '10': 'manifest'
    },
  ],
};

/// Descriptor for `SpaceLifecycleFenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceLifecycleFenceRequestDescriptor = $convert.base64Decode(
    'ChpTcGFjZUxpZmVjeWNsZUZlbmNlUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg'
    '9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSMgoVZGVsZXRpb25f'
    'b3BlcmF0aW9uX2lkGAMgASgJUhNkZWxldGlvbk9wZXJhdGlvbklkEh4KCmdlbmVyYXRpb24YBC'
    'ABKARSCmdlbmVyYXRpb24SSQoNZGVzaXJlZF9zdGF0ZRgFIAEoDjIkLnZvaWNlLmNvbW1vbi52'
    'MS5MaWZlY3ljbGVGZW5jZVN0YXRlUgxkZXNpcmVkU3RhdGUSPAoIbWFuaWZlc3QYBiABKAsyIC'
    '52b2ljZS5jb21tb24udjEuTWFuaWZlc3RCaW5kaW5nUghtYW5pZmVzdA==');

@$core.Deprecated('Use spaceLifecycleFenceReceiptDescriptor instead')
const SpaceLifecycleFenceReceipt$json = {
  '1': 'SpaceLifecycleFenceReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 5, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'participant_id',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.ParticipantId',
      '10': 'participantId'
    },
    {
      '1': 'applied_state',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.LifecycleFenceState',
      '10': 'appliedState'
    },
    {'1': 'request_sha256', '3': 8, '4': 1, '5': 12, '10': 'requestSha256'},
    {'1': 'manifest_sha256', '3': 9, '4': 1, '5': 12, '10': 'manifestSha256'},
    {
      '1': 'applied_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'appliedAt'
    },
  ],
};

/// Descriptor for `SpaceLifecycleFenceReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceLifecycleFenceReceiptDescriptor = $convert.base64Decode(
    'ChpTcGFjZUxpZmVjeWNsZUZlbmNlUmVjZWlwdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg'
    '9wcm90b2NvbFZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJcmVjZWlwdElkEhkKCHNwYWNl'
    'X2lkGAMgASgJUgdzcGFjZUlkEjIKFWRlbGV0aW9uX29wZXJhdGlvbl9pZBgEIAEoCVITZGVsZX'
    'Rpb25PcGVyYXRpb25JZBIeCgpnZW5lcmF0aW9uGAUgASgEUgpnZW5lcmF0aW9uEkUKDnBhcnRp'
    'Y2lwYW50X2lkGAYgASgOMh4udm9pY2UuY29tbW9uLnYxLlBhcnRpY2lwYW50SWRSDXBhcnRpY2'
    'lwYW50SWQSSQoNYXBwbGllZF9zdGF0ZRgHIAEoDjIkLnZvaWNlLmNvbW1vbi52MS5MaWZlY3lj'
    'bGVGZW5jZVN0YXRlUgxhcHBsaWVkU3RhdGUSJQoOcmVxdWVzdF9zaGEyNTYYCCABKAxSDXJlcX'
    'Vlc3RTaGEyNTYSJwoPbWFuaWZlc3Rfc2hhMjU2GAkgASgMUg5tYW5pZmVzdFNoYTI1NhI5Cgph'
    'cHBsaWVkX2F0GAogASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJYXBwbGllZEF0');

@$core.Deprecated('Use spacePurgeRequestDescriptor instead')
const SpacePurgeRequest$json = {
  '1': 'SpacePurgeRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 4, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'purge_decided_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'purgeDecidedAt'
    },
    {
      '1': 'participant_id',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.ParticipantId',
      '10': 'participantId'
    },
    {
      '1': 'manifest',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.ManifestBinding',
      '10': 'manifest'
    },
  ],
};

/// Descriptor for `SpacePurgeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spacePurgeRequestDescriptor = $convert.base64Decode(
    'ChFTcGFjZVB1cmdlUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm90b2NvbF'
    'ZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSMgoVZGVsZXRpb25fb3BlcmF0aW9u'
    'X2lkGAMgASgJUhNkZWxldGlvbk9wZXJhdGlvbklkEh4KCmdlbmVyYXRpb24YBCABKARSCmdlbm'
    'VyYXRpb24SRAoQcHVyZ2VfZGVjaWRlZF9hdBgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1l'
    'c3RhbXBSDnB1cmdlRGVjaWRlZEF0EkUKDnBhcnRpY2lwYW50X2lkGAYgASgOMh4udm9pY2UuY2'
    '9tbW9uLnYxLlBhcnRpY2lwYW50SWRSDXBhcnRpY2lwYW50SWQSPAoIbWFuaWZlc3QYByABKAsy'
    'IC52b2ljZS5jb21tb24udjEuTWFuaWZlc3RCaW5kaW5nUghtYW5pZmVzdA==');

@$core.Deprecated('Use spacePurgeReceiptDescriptor instead')
const SpacePurgeReceipt$json = {
  '1': 'SpacePurgeReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 5, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'participant_id',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.ParticipantId',
      '10': 'participantId'
    },
    {
      '1': 'state',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.PurgeReceiptState',
      '10': 'state'
    },
    {'1': 'request_sha256', '3': 8, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `SpacePurgeReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spacePurgeReceiptDescriptor = $convert.base64Decode(
    'ChFTcGFjZVB1cmdlUmVjZWlwdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm90b2NvbF'
    'ZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJcmVjZWlwdElkEhkKCHNwYWNlX2lkGAMgASgJ'
    'UgdzcGFjZUlkEjIKFWRlbGV0aW9uX29wZXJhdGlvbl9pZBgEIAEoCVITZGVsZXRpb25PcGVyYX'
    'Rpb25JZBIeCgpnZW5lcmF0aW9uGAUgASgEUgpnZW5lcmF0aW9uEkUKDnBhcnRpY2lwYW50X2lk'
    'GAYgASgOMh4udm9pY2UuY29tbW9uLnYxLlBhcnRpY2lwYW50SWRSDXBhcnRpY2lwYW50SWQSOA'
    'oFc3RhdGUYByABKA4yIi52b2ljZS5jb21tb24udjEuUHVyZ2VSZWNlaXB0U3RhdGVSBXN0YXRl'
    'EiUKDnJlcXVlc3Rfc2hhMjU2GAggASgMUg1yZXF1ZXN0U2hhMjU2Ej0KDGNvbXBsZXRlZF9hdB'
    'gJIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSC2NvbXBsZXRlZEF0');
