// This is a generated file - do not edit.
//
// Generated from voice/file/v1/catalog_assets.proto.

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

@$core.Deprecated('Use importCatalogAssetRequestDescriptor instead')
const ImportCatalogAssetRequest$json = {
  '1': 'ImportCatalogAssetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'source_id', '3': 3, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'source_url', '3': 6, '4': 1, '5': 9, '10': 'sourceUrl'},
    {'1': 'actor_account_id', '3': 7, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 8, '4': 1, '5': 9, '10': 'actorProfileId'},
    {
      '1': 'destination_chat',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `ImportCatalogAssetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importCatalogAssetRequestDescriptor = $convert.base64Decode(
    'ChlJbXBvcnRDYXRhbG9nQXNzZXRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3'
    'Byb3RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlvbklkEhsKCXNv'
    'dXJjZV9pZBgDIAEoCVIIc291cmNlSWQSGgoIcHJvdmlkZXIYBCABKAlSCHByb3ZpZGVyEh8KC3'
    'Byb3ZpZGVyX2lkGAUgASgJUgpwcm92aWRlcklkEh0KCnNvdXJjZV91cmwYBiABKAlSCXNvdXJj'
    'ZVVybBIoChBhY3Rvcl9hY2NvdW50X2lkGAcgASgJUg5hY3RvckFjY291bnRJZBIoChBhY3Rvcl'
    '9wcm9maWxlX2lkGAggASgJUg5hY3RvclByb2ZpbGVJZBJBChBkZXN0aW5hdGlvbl9jaGF0GAkg'
    'ASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmUg9kZXN0aW5hdGlvbkNoYXQ=');

@$core.Deprecated('Use importCatalogAssetResponseDescriptor instead')
const ImportCatalogAssetResponse$json = {
  '1': 'ImportCatalogAssetResponse',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 6, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'status', '3': 7, '4': 1, '5': 9, '10': 'status'},
    {'1': 'operation_id', '3': 8, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'destination_chat',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `ImportCatalogAssetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List importCatalogAssetResponseDescriptor = $convert.base64Decode(
    'ChpJbXBvcnRDYXRhbG9nQXNzZXRSZXNwb25zZRIbCglzb3VyY2VfaWQYASABKAlSCHNvdXJjZU'
    'lkEigKEGFjdG9yX2FjY291bnRfaWQYAiABKAlSDmFjdG9yQWNjb3VudElkEigKEGFjdG9yX3By'
    'b2ZpbGVfaWQYAyABKAlSDmFjdG9yUHJvZmlsZUlkEhoKCHByb3ZpZGVyGAQgASgJUghwcm92aW'
    'RlchIfCgtwcm92aWRlcl9pZBgFIAEoCVIKcHJvdmlkZXJJZBIXCgdmaWxlX2lkGAYgASgJUgZm'
    'aWxlSWQSFgoGc3RhdHVzGAcgASgJUgZzdGF0dXMSIQoMb3BlcmF0aW9uX2lkGAggASgJUgtvcG'
    'VyYXRpb25JZBJBChBkZXN0aW5hdGlvbl9jaGF0GAkgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0'
    'UmVmUg9kZXN0aW5hdGlvbkNoYXQ=');

@$core.Deprecated('Use validateCatalogAssetRequestDescriptor instead')
const ValidateCatalogAssetRequest$json = {
  '1': 'ValidateCatalogAssetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'source_id', '3': 2, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'provider', '3': 3, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 4, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 5, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'actor_account_id', '3': 6, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 7, '4': 1, '5': 9, '10': 'actorProfileId'},
  ],
};

/// Descriptor for `ValidateCatalogAssetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateCatalogAssetRequestDescriptor = $convert.base64Decode(
    'ChtWYWxpZGF0ZUNhdGFsb2dBc3NldFJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDV'
    'IPcHJvdG9jb2xWZXJzaW9uEhsKCXNvdXJjZV9pZBgCIAEoCVIIc291cmNlSWQSGgoIcHJvdmlk'
    'ZXIYAyABKAlSCHByb3ZpZGVyEh8KC3Byb3ZpZGVyX2lkGAQgASgJUgpwcm92aWRlcklkEhcKB2'
    'ZpbGVfaWQYBSABKAlSBmZpbGVJZBIoChBhY3Rvcl9hY2NvdW50X2lkGAYgASgJUg5hY3RvckFj'
    'Y291bnRJZBIoChBhY3Rvcl9wcm9maWxlX2lkGAcgASgJUg5hY3RvclByb2ZpbGVJZA==');

@$core.Deprecated('Use validateCatalogAssetResponseDescriptor instead')
const ValidateCatalogAssetResponse$json = {
  '1': 'ValidateCatalogAssetResponse',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 6, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'object_revision', '3': 7, '4': 1, '5': 4, '10': 'objectRevision'},
    {'1': 'intent', '3': 8, '4': 1, '5': 9, '10': 'intent'},
    {'1': 'scan_result', '3': 9, '4': 1, '5': 9, '10': 'scanResult'},
    {'1': 'file_type', '3': 10, '4': 1, '5': 9, '10': 'fileType'},
    {'1': 'source_sha256', '3': 11, '4': 1, '5': 12, '10': 'sourceSha256'},
    {
      '1': 'delivered_sha256',
      '3': 12,
      '4': 1,
      '5': 12,
      '10': 'deliveredSha256'
    },
    {
      '1': 'delivered_size_bytes',
      '3': 13,
      '4': 1,
      '5': 3,
      '10': 'deliveredSizeBytes'
    },
    {
      '1': 'delivered_mime_type',
      '3': 14,
      '4': 1,
      '5': 9,
      '10': 'deliveredMimeType'
    },
  ],
};

/// Descriptor for `ValidateCatalogAssetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateCatalogAssetResponseDescriptor = $convert.base64Decode(
    'ChxWYWxpZGF0ZUNhdGFsb2dBc3NldFJlc3BvbnNlEhsKCXNvdXJjZV9pZBgBIAEoCVIIc291cm'
    'NlSWQSKAoQYWN0b3JfYWNjb3VudF9pZBgCIAEoCVIOYWN0b3JBY2NvdW50SWQSKAoQYWN0b3Jf'
    'cHJvZmlsZV9pZBgDIAEoCVIOYWN0b3JQcm9maWxlSWQSGgoIcHJvdmlkZXIYBCABKAlSCHByb3'
    'ZpZGVyEh8KC3Byb3ZpZGVyX2lkGAUgASgJUgpwcm92aWRlcklkEhcKB2ZpbGVfaWQYBiABKAlS'
    'BmZpbGVJZBInCg9vYmplY3RfcmV2aXNpb24YByABKARSDm9iamVjdFJldmlzaW9uEhYKBmludG'
    'VudBgIIAEoCVIGaW50ZW50Eh8KC3NjYW5fcmVzdWx0GAkgASgJUgpzY2FuUmVzdWx0EhsKCWZp'
    'bGVfdHlwZRgKIAEoCVIIZmlsZVR5cGUSIwoNc291cmNlX3NoYTI1NhgLIAEoDFIMc291cmNlU2'
    'hhMjU2EikKEGRlbGl2ZXJlZF9zaGEyNTYYDCABKAxSD2RlbGl2ZXJlZFNoYTI1NhIwChRkZWxp'
    'dmVyZWRfc2l6ZV9ieXRlcxgNIAEoA1ISZGVsaXZlcmVkU2l6ZUJ5dGVzEi4KE2RlbGl2ZXJlZF'
    '9taW1lX3R5cGUYDiABKAlSEWRlbGl2ZXJlZE1pbWVUeXBl');

@$core.Deprecated('Use lookupCatalogAssetRequestDescriptor instead')
const LookupCatalogAssetRequest$json = {
  '1': 'LookupCatalogAssetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'file_id', '3': 4, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `LookupCatalogAssetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lookupCatalogAssetRequestDescriptor = $convert.base64Decode(
    'ChlMb29rdXBDYXRhbG9nQXNzZXRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3'
    'Byb3RvY29sVmVyc2lvbhIoChBhY3Rvcl9hY2NvdW50X2lkGAIgASgJUg5hY3RvckFjY291bnRJ'
    'ZBIoChBhY3Rvcl9wcm9maWxlX2lkGAMgASgJUg5hY3RvclByb2ZpbGVJZBIXCgdmaWxlX2lkGA'
    'QgASgJUgZmaWxlSWQ=');

@$core.Deprecated('Use lookupCatalogAssetResponseDescriptor instead')
const LookupCatalogAssetResponse$json = {
  '1': 'LookupCatalogAssetResponse',
  '2': [
    {
      '1': 'catalog',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.ValidateCatalogAssetResponse',
      '9': 0,
      '10': 'catalog'
    },
    {
      '1': 'ordinary_file',
      '3': 2,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'ordinaryFile'
    },
  ],
  '8': [
    {'1': 'classification'},
  ],
};

/// Descriptor for `LookupCatalogAssetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lookupCatalogAssetResponseDescriptor =
    $convert.base64Decode(
        'ChpMb29rdXBDYXRhbG9nQXNzZXRSZXNwb25zZRJHCgdjYXRhbG9nGAEgASgLMisudm9pY2UuZm'
        'lsZS52MS5WYWxpZGF0ZUNhdGFsb2dBc3NldFJlc3BvbnNlSABSB2NhdGFsb2cSJQoNb3JkaW5h'
        'cnlfZmlsZRgCIAEoCEgAUgxvcmRpbmFyeUZpbGVCEAoOY2xhc3NpZmljYXRpb24=');

@$core.Deprecated('Use catalogImportOperationSelectorDescriptor instead')
const CatalogImportOperationSelector$json = {
  '1': 'CatalogImportOperationSelector',
  '2': [
    {'1': 'operation_id', '3': 1, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 3, '4': 1, '5': 9, '10': 'providerId'},
    {
      '1': 'destination_chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `CatalogImportOperationSelector`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List catalogImportOperationSelectorDescriptor =
    $convert.base64Decode(
        'Ch5DYXRhbG9nSW1wb3J0T3BlcmF0aW9uU2VsZWN0b3ISIQoMb3BlcmF0aW9uX2lkGAEgASgJUg'
        'tvcGVyYXRpb25JZBIaCghwcm92aWRlchgCIAEoCVIIcHJvdmlkZXISHwoLcHJvdmlkZXJfaWQY'
        'AyABKAlSCnByb3ZpZGVySWQSQQoQZGVzdGluYXRpb25fY2hhdBgEIAEoCzIWLnZvaWNlLmNoYX'
        'QudjEuQ2hhdFJlZlIPZGVzdGluYXRpb25DaGF0');

@$core.Deprecated('Use catalogAssetSelectorDescriptor instead')
const CatalogAssetSelector$json = {
  '1': 'CatalogAssetSelector',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 3, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 4, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `CatalogAssetSelector`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List catalogAssetSelectorDescriptor = $convert.base64Decode(
    'ChRDYXRhbG9nQXNzZXRTZWxlY3RvchIbCglzb3VyY2VfaWQYASABKAlSCHNvdXJjZUlkEhoKCH'
    'Byb3ZpZGVyGAIgASgJUghwcm92aWRlchIfCgtwcm92aWRlcl9pZBgDIAEoCVIKcHJvdmlkZXJJ'
    'ZBIXCgdmaWxlX2lkGAQgASgJUgZmaWxlSWQ=');

@$core.Deprecated('Use getCatalogAssetStatusRequestDescriptor instead')
const GetCatalogAssetStatusRequest$json = {
  '1': 'GetCatalogAssetStatusRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {
      '1': 'operation',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.CatalogImportOperationSelector',
      '9': 0,
      '10': 'operation'
    },
    {
      '1': 'asset',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.CatalogAssetSelector',
      '9': 0,
      '10': 'asset'
    },
  ],
  '8': [
    {'1': 'selector'},
  ],
};

/// Descriptor for `GetCatalogAssetStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCatalogAssetStatusRequestDescriptor = $convert.base64Decode(
    'ChxHZXRDYXRhbG9nQXNzZXRTdGF0dXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIoChBhY3Rvcl9hY2NvdW50X2lkGAIgASgJUg5hY3RvckFjY291'
    'bnRJZBIoChBhY3Rvcl9wcm9maWxlX2lkGAMgASgJUg5hY3RvclByb2ZpbGVJZBJNCglvcGVyYX'
    'Rpb24YBCABKAsyLS52b2ljZS5maWxlLnYxLkNhdGFsb2dJbXBvcnRPcGVyYXRpb25TZWxlY3Rv'
    'ckgAUglvcGVyYXRpb24SOwoFYXNzZXQYBSABKAsyIy52b2ljZS5maWxlLnYxLkNhdGFsb2dBc3'
    'NldFNlbGVjdG9ySABSBWFzc2V0QgoKCHNlbGVjdG9y');

@$core.Deprecated('Use getCatalogAssetStatusResponseDescriptor instead')
const GetCatalogAssetStatusResponse$json = {
  '1': 'GetCatalogAssetStatusResponse',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 6, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'status', '3': 7, '4': 1, '5': 9, '10': 'status'},
    {'1': 'operation_id', '3': 8, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'destination_chat',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `GetCatalogAssetStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCatalogAssetStatusResponseDescriptor = $convert.base64Decode(
    'Ch1HZXRDYXRhbG9nQXNzZXRTdGF0dXNSZXNwb25zZRIbCglzb3VyY2VfaWQYASABKAlSCHNvdX'
    'JjZUlkEigKEGFjdG9yX2FjY291bnRfaWQYAiABKAlSDmFjdG9yQWNjb3VudElkEigKEGFjdG9y'
    'X3Byb2ZpbGVfaWQYAyABKAlSDmFjdG9yUHJvZmlsZUlkEhoKCHByb3ZpZGVyGAQgASgJUghwcm'
    '92aWRlchIfCgtwcm92aWRlcl9pZBgFIAEoCVIKcHJvdmlkZXJJZBIXCgdmaWxlX2lkGAYgASgJ'
    'UgZmaWxlSWQSFgoGc3RhdHVzGAcgASgJUgZzdGF0dXMSIQoMb3BlcmF0aW9uX2lkGAggASgJUg'
    'tvcGVyYXRpb25JZBJBChBkZXN0aW5hdGlvbl9jaGF0GAkgASgLMhYudm9pY2UuY2hhdC52MS5D'
    'aGF0UmVmUg9kZXN0aW5hdGlvbkNoYXQ=');
