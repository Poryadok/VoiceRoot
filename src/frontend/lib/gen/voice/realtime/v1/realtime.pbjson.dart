// This is a generated file - do not edit.
//
// Generated from voice/realtime/v1/realtime.proto.

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

@$core.Deprecated('Use applySpaceLifecycleFenceRequestDescriptor instead')
const ApplySpaceLifecycleFenceRequest$json = {
  '1': 'ApplySpaceLifecycleFenceRequest',
  '2': [
    {
      '1': 'fence',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpaceLifecycleFenceRequest',
      '10': 'fence'
    },
  ],
};

/// Descriptor for `ApplySpaceLifecycleFenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySpaceLifecycleFenceRequestDescriptor =
    $convert.base64Decode(
        'Ch9BcHBseVNwYWNlTGlmZWN5Y2xlRmVuY2VSZXF1ZXN0EkEKBWZlbmNlGAEgASgLMisudm9pY2'
        'UuY29tbW9uLnYxLlNwYWNlTGlmZWN5Y2xlRmVuY2VSZXF1ZXN0UgVmZW5jZQ==');

@$core.Deprecated('Use applySpaceLifecycleFenceResponseDescriptor instead')
const ApplySpaceLifecycleFenceResponse$json = {
  '1': 'ApplySpaceLifecycleFenceResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpaceLifecycleFenceReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ApplySpaceLifecycleFenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySpaceLifecycleFenceResponseDescriptor =
    $convert.base64Decode(
        'CiBBcHBseVNwYWNlTGlmZWN5Y2xlRmVuY2VSZXNwb25zZRJFCgdyZWNlaXB0GAEgASgLMisudm'
        '9pY2UuY29tbW9uLnYxLlNwYWNlTGlmZWN5Y2xlRmVuY2VSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use purgeSpaceRequestDescriptor instead')
const PurgeSpaceRequest$json = {
  '1': 'PurgeSpaceRequest',
  '2': [
    {
      '1': 'purge',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpacePurgeRequest',
      '10': 'purge'
    },
  ],
};

/// Descriptor for `PurgeSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List purgeSpaceRequestDescriptor = $convert.base64Decode(
    'ChFQdXJnZVNwYWNlUmVxdWVzdBI4CgVwdXJnZRgBIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5TcG'
    'FjZVB1cmdlUmVxdWVzdFIFcHVyZ2U=');

@$core.Deprecated('Use purgeSpaceResponseDescriptor instead')
const PurgeSpaceResponse$json = {
  '1': 'PurgeSpaceResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpacePurgeReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `PurgeSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List purgeSpaceResponseDescriptor = $convert.base64Decode(
    'ChJQdXJnZVNwYWNlUmVzcG9uc2USPAoHcmVjZWlwdBgBIAEoCzIiLnZvaWNlLmNvbW1vbi52MS'
    '5TcGFjZVB1cmdlUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated('Use importSpacePurgeManifestPageRequestDescriptor instead')
const ImportSpacePurgeManifestPageRequest$json = {
  '1': 'ImportSpacePurgeManifestPageRequest',
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
    {
      '1': 'schedule_generation',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'scheduleGeneration'
    },
    {
      '1': 'page',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.SpacePurgeManifestPage',
      '10': 'page'
    },
    {'1': 'seals_manifest', '3': 6, '4': 1, '5': 8, '10': 'sealsManifest'},
  ],
};

/// Descriptor for `ImportSpacePurgeManifestPageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importSpacePurgeManifestPageRequestDescriptor = $convert.base64Decode(
    'CiNJbXBvcnRTcGFjZVB1cmdlTWFuaWZlc3RQYWdlUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW'
    '9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSMgoV'
    'ZGVsZXRpb25fb3BlcmF0aW9uX2lkGAMgASgJUhNkZWxldGlvbk9wZXJhdGlvbklkEi8KE3NjaG'
    'VkdWxlX2dlbmVyYXRpb24YBCABKARSEnNjaGVkdWxlR2VuZXJhdGlvbhI5CgRwYWdlGAUgASgL'
    'MiUudm9pY2UuY2hhdC52MS5TcGFjZVB1cmdlTWFuaWZlc3RQYWdlUgRwYWdlEiUKDnNlYWxzX2'
    '1hbmlmZXN0GAYgASgIUg1zZWFsc01hbmlmZXN0');

@$core.Deprecated('Use importSpacePurgeManifestPageReceiptDescriptor instead')
const ImportSpacePurgeManifestPageReceipt$json = {
  '1': 'ImportSpacePurgeManifestPageReceipt',
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
      '1': 'manifest',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.ManifestBinding',
      '10': 'manifest'
    },
    {'1': 'page_index', '3': 7, '4': 1, '5': 4, '10': 'pageIndex'},
    {'1': 'accepted_count', '3': 8, '4': 1, '5': 4, '10': 'acceptedCount'},
    {'1': 'page_sha256', '3': 9, '4': 1, '5': 12, '10': 'pageSha256'},
    {'1': 'manifest_sealed', '3': 10, '4': 1, '5': 8, '10': 'manifestSealed'},
    {'1': 'request_sha256', '3': 11, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `ImportSpacePurgeManifestPageReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importSpacePurgeManifestPageReceiptDescriptor = $convert.base64Decode(
    'CiNJbXBvcnRTcGFjZVB1cmdlTWFuaWZlc3RQYWdlUmVjZWlwdBIpChBwcm90b2NvbF92ZXJzaW'
    '9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJcmVjZWlwdElk'
    'EhkKCHNwYWNlX2lkGAMgASgJUgdzcGFjZUlkEjIKFWRlbGV0aW9uX29wZXJhdGlvbl9pZBgEIA'
    'EoCVITZGVsZXRpb25PcGVyYXRpb25JZBIeCgpnZW5lcmF0aW9uGAUgASgEUgpnZW5lcmF0aW9u'
    'EjwKCG1hbmlmZXN0GAYgASgLMiAudm9pY2UuY29tbW9uLnYxLk1hbmlmZXN0QmluZGluZ1IIbW'
    'FuaWZlc3QSHQoKcGFnZV9pbmRleBgHIAEoBFIJcGFnZUluZGV4EiUKDmFjY2VwdGVkX2NvdW50'
    'GAggASgEUg1hY2NlcHRlZENvdW50Eh8KC3BhZ2Vfc2hhMjU2GAkgASgMUgpwYWdlU2hhMjU2Ei'
    'cKD21hbmlmZXN0X3NlYWxlZBgKIAEoCFIObWFuaWZlc3RTZWFsZWQSJQoOcmVxdWVzdF9zaGEy'
    'NTYYCyABKAxSDXJlcXVlc3RTaGEyNTYSPQoMY29tcGxldGVkX2F0GAwgASgLMhouZ29vZ2xlLn'
    'Byb3RvYnVmLlRpbWVzdGFtcFILY29tcGxldGVkQXQ=');

@$core.Deprecated('Use importSpacePurgeManifestPageResponseDescriptor instead')
const ImportSpacePurgeManifestPageResponse$json = {
  '1': 'ImportSpacePurgeManifestPageResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.realtime.v1.ImportSpacePurgeManifestPageReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ImportSpacePurgeManifestPageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importSpacePurgeManifestPageResponseDescriptor =
    $convert.base64Decode(
        'CiRJbXBvcnRTcGFjZVB1cmdlTWFuaWZlc3RQYWdlUmVzcG9uc2USUAoHcmVjZWlwdBgBIAEoCz'
        'I2LnZvaWNlLnJlYWx0aW1lLnYxLkltcG9ydFNwYWNlUHVyZ2VNYW5pZmVzdFBhZ2VSZWNlaXB0'
        'UgdyZWNlaXB0');
