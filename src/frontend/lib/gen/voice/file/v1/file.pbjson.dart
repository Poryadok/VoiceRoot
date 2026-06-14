// This is a generated file - do not edit.
//
// Generated from voice/file/v1/file.proto.

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

@$core.Deprecated('Use fileLifecycleStatusDescriptor instead')
const FileLifecycleStatus$json = {
  '1': 'FileLifecycleStatus',
  '2': [
    {'1': 'FILE_LIFECYCLE_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'FILE_LIFECYCLE_STATUS_PENDING_UPLOAD', '2': 1},
    {'1': 'FILE_LIFECYCLE_STATUS_PROCESSING', '2': 2},
    {'1': 'FILE_LIFECYCLE_STATUS_READY', '2': 3},
    {'1': 'FILE_LIFECYCLE_STATUS_FAILED', '2': 4},
    {'1': 'FILE_LIFECYCLE_STATUS_DELETED', '2': 5},
  ],
};

/// Descriptor for `FileLifecycleStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileLifecycleStatusDescriptor = $convert.base64Decode(
    'ChNGaWxlTGlmZWN5Y2xlU3RhdHVzEiUKIUZJTEVfTElGRUNZQ0xFX1NUQVRVU19VTlNQRUNJRk'
    'lFRBAAEigKJEZJTEVfTElGRUNZQ0xFX1NUQVRVU19QRU5ESU5HX1VQTE9BRBABEiQKIEZJTEVf'
    'TElGRUNZQ0xFX1NUQVRVU19QUk9DRVNTSU5HEAISHwobRklMRV9MSUZFQ1lDTEVfU1RBVFVTX1'
    'JFQURZEAMSIAocRklMRV9MSUZFQ1lDTEVfU1RBVFVTX0ZBSUxFRBAEEiEKHUZJTEVfTElGRUNZ'
    'Q0xFX1NUQVRVU19ERUxFVEVEEAU=');

@$core.Deprecated('Use fileMediaCategoryDescriptor instead')
const FileMediaCategory$json = {
  '1': 'FileMediaCategory',
  '2': [
    {'1': 'FILE_MEDIA_CATEGORY_UNSPECIFIED', '2': 0},
    {'1': 'FILE_MEDIA_CATEGORY_IMAGE', '2': 1},
    {'1': 'FILE_MEDIA_CATEGORY_VIDEO', '2': 2},
    {'1': 'FILE_MEDIA_CATEGORY_AUDIO', '2': 3},
    {'1': 'FILE_MEDIA_CATEGORY_DOCUMENT', '2': 4},
    {'1': 'FILE_MEDIA_CATEGORY_OTHER', '2': 5},
  ],
};

/// Descriptor for `FileMediaCategory`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileMediaCategoryDescriptor = $convert.base64Decode(
    'ChFGaWxlTWVkaWFDYXRlZ29yeRIjCh9GSUxFX01FRElBX0NBVEVHT1JZX1VOU1BFQ0lGSUVEEA'
    'ASHQoZRklMRV9NRURJQV9DQVRFR09SWV9JTUFHRRABEh0KGUZJTEVfTUVESUFfQ0FURUdPUllf'
    'VklERU8QAhIdChlGSUxFX01FRElBX0NBVEVHT1JZX0FVRElPEAMSIAocRklMRV9NRURJQV9DQV'
    'RFR09SWV9ET0NVTUVOVBAEEh0KGUZJTEVfTUVESUFfQ0FURUdPUllfT1RIRVIQBQ==');

@$core.Deprecated('Use fileScanOutcomeDescriptor instead')
const FileScanOutcome$json = {
  '1': 'FileScanOutcome',
  '2': [
    {'1': 'FILE_SCAN_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'FILE_SCAN_OUTCOME_PENDING', '2': 1},
    {'1': 'FILE_SCAN_OUTCOME_CLEAN', '2': 2},
    {'1': 'FILE_SCAN_OUTCOME_INFECTED', '2': 3},
    {'1': 'FILE_SCAN_OUTCOME_ERROR', '2': 4},
    {'1': 'FILE_SCAN_OUTCOME_SKIPPED', '2': 5},
  ],
};

/// Descriptor for `FileScanOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileScanOutcomeDescriptor = $convert.base64Decode(
    'Cg9GaWxlU2Nhbk91dGNvbWUSIQodRklMRV9TQ0FOX09VVENPTUVfVU5TUEVDSUZJRUQQABIdCh'
    'lGSUxFX1NDQU5fT1VUQ09NRV9QRU5ESU5HEAESGwoXRklMRV9TQ0FOX09VVENPTUVfQ0xFQU4Q'
    'AhIeChpGSUxFX1NDQU5fT1VUQ09NRV9JTkZFQ1RFRBADEhsKF0ZJTEVfU0NBTl9PVVRDT01FX0'
    'VSUk9SEAQSHQoZRklMRV9TQ0FOX09VVENPTUVfU0tJUFBFRBAF');

@$core.Deprecated('Use requestUploadRequestDescriptor instead')
const RequestUploadRequest$json = {
  '1': 'RequestUploadRequest',
  '2': [
    {'1': 'original_name', '3': 1, '4': 1, '5': 9, '10': 'originalName'},
    {'1': 'mime_type', '3': 2, '4': 1, '5': 9, '10': 'mimeType'},
    {'1': 'size_bytes', '3': 3, '4': 1, '5': 3, '10': 'sizeBytes'},
    {
      '1': 'context_chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'contextChat',
      '17': true
    },
    {'1': 'is_e2e', '3': 6, '4': 1, '5': 8, '9': 1, '10': 'isE2e', '17': true},
  ],
  '8': [
    {'1': '_context_chat'},
    {'1': '_is_e2e'},
  ],
  '9': [
    {'1': 5, '2': 6},
  ],
  '10': ['chat_type'],
};

/// Descriptor for `RequestUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List requestUploadRequestDescriptor = $convert.base64Decode(
    'ChRSZXF1ZXN0VXBsb2FkUmVxdWVzdBIjCg1vcmlnaW5hbF9uYW1lGAEgASgJUgxvcmlnaW5hbE'
    '5hbWUSGwoJbWltZV90eXBlGAIgASgJUghtaW1lVHlwZRIdCgpzaXplX2J5dGVzGAMgASgDUglz'
    'aXplQnl0ZXMSPgoMY29udGV4dF9jaGF0GAQgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmSA'
    'BSC2NvbnRleHRDaGF0iAEBEhoKBmlzX2UyZRgGIAEoCEgBUgVpc0UyZYgBAUIPCg1fY29udGV4'
    'dF9jaGF0QgkKB19pc19lMmVKBAgFEAZSCWNoYXRfdHlwZQ==');

@$core.Deprecated('Use uploadResponseDescriptor instead')
const UploadResponse$json = {
  '1': 'UploadResponse',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'presigned_put_url', '3': 2, '4': 1, '5': 9, '10': 'presignedPutUrl'},
    {'1': 'r2_key', '3': 3, '4': 1, '5': 9, '10': 'r2Key'},
  ],
};

/// Descriptor for `UploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List uploadResponseDescriptor = $convert.base64Decode(
    'Cg5VcGxvYWRSZXNwb25zZRIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQSKgoRcHJlc2lnbmVkX3'
    'B1dF91cmwYAiABKAlSD3ByZXNpZ25lZFB1dFVybBIVCgZyMl9rZXkYAyABKAlSBXIyS2V5');

@$core.Deprecated('Use confirmUploadRequestDescriptor instead')
const ConfirmUploadRequest$json = {
  '1': 'ConfirmUploadRequest',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'sha256_hash', '3': 2, '4': 1, '5': 9, '10': 'sha256Hash'},
  ],
};

/// Descriptor for `ConfirmUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List confirmUploadRequestDescriptor = $convert.base64Decode(
    'ChRDb25maXJtVXBsb2FkUmVxdWVzdBIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQSHwoLc2hhMj'
    'U2X2hhc2gYAiABKAlSCnNoYTI1Nkhhc2g=');

@$core.Deprecated('Use fileMetadataDescriptor instead')
const FileMetadata$json = {
  '1': 'FileMetadata',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'uploader_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'uploaderProfileId'
    },
    {'1': 'original_name', '3': 3, '4': 1, '5': 9, '10': 'originalName'},
    {'1': 'mime_type', '3': 4, '4': 1, '5': 9, '10': 'mimeType'},
    {'1': 'size_bytes', '3': 5, '4': 1, '5': 3, '10': 'sizeBytes'},
    {'1': 'sha256_hash', '3': 6, '4': 1, '5': 9, '10': 'sha256Hash'},
    {'1': 'r2_key', '3': 7, '4': 1, '5': 9, '10': 'r2Key'},
    {'1': 'status', '3': 8, '4': 1, '5': 9, '10': 'status'},
    {'1': 'file_type', '3': 9, '4': 1, '5': 9, '10': 'fileType'},
    {'1': 'width', '3': 10, '4': 1, '5': 5, '9': 0, '10': 'width', '17': true},
    {
      '1': 'height',
      '3': 11,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'height',
      '17': true
    },
    {
      '1': 'duration_seconds',
      '3': 12,
      '4': 1,
      '5': 5,
      '9': 2,
      '10': 'durationSeconds',
      '17': true
    },
    {
      '1': 'thumbnail_r2_key',
      '3': 13,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'thumbnailR2Key',
      '17': true
    },
    {
      '1': 'converted_r2_key',
      '3': 14,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'convertedR2Key',
      '17': true
    },
    {
      '1': 'chat',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 5,
      '10': 'chat',
      '17': true
    },
    {'1': 'is_e2e', '3': 17, '4': 1, '5': 8, '10': 'isE2e'},
    {
      '1': 'expires_at',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 6,
      '10': 'expiresAt',
      '17': true
    },
    {'1': 'scan_result', '3': 19, '4': 1, '5': 9, '10': 'scanResult'},
    {
      '1': 'created_at',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'status_enum',
      '3': 21,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileLifecycleStatus',
      '9': 7,
      '10': 'statusEnum',
      '17': true
    },
    {
      '1': 'file_type_enum',
      '3': 22,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileMediaCategory',
      '9': 8,
      '10': 'fileTypeEnum',
      '17': true
    },
    {
      '1': 'scan_result_enum',
      '3': 23,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileScanOutcome',
      '9': 9,
      '10': 'scanResultEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_width'},
    {'1': '_height'},
    {'1': '_duration_seconds'},
    {'1': '_thumbnail_r2_key'},
    {'1': '_converted_r2_key'},
    {'1': '_chat'},
    {'1': '_expires_at'},
    {'1': '_status_enum'},
    {'1': '_file_type_enum'},
    {'1': '_scan_result_enum'},
  ],
  '9': [
    {'1': 16, '2': 17},
  ],
  '10': ['chat_type'],
};

/// Descriptor for `FileMetadata`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileMetadataDescriptor = $convert.base64Decode(
    'CgxGaWxlTWV0YWRhdGESDgoCaWQYASABKAlSAmlkEi4KE3VwbG9hZGVyX3Byb2ZpbGVfaWQYAi'
    'ABKAlSEXVwbG9hZGVyUHJvZmlsZUlkEiMKDW9yaWdpbmFsX25hbWUYAyABKAlSDG9yaWdpbmFs'
    'TmFtZRIbCgltaW1lX3R5cGUYBCABKAlSCG1pbWVUeXBlEh0KCnNpemVfYnl0ZXMYBSABKANSCX'
    'NpemVCeXRlcxIfCgtzaGEyNTZfaGFzaBgGIAEoCVIKc2hhMjU2SGFzaBIVCgZyMl9rZXkYByAB'
    'KAlSBXIyS2V5EhYKBnN0YXR1cxgIIAEoCVIGc3RhdHVzEhsKCWZpbGVfdHlwZRgJIAEoCVIIZm'
    'lsZVR5cGUSGQoFd2lkdGgYCiABKAVIAFIFd2lkdGiIAQESGwoGaGVpZ2h0GAsgASgFSAFSBmhl'
    'aWdodIgBARIuChBkdXJhdGlvbl9zZWNvbmRzGAwgASgFSAJSD2R1cmF0aW9uU2Vjb25kc4gBAR'
    'ItChB0aHVtYm5haWxfcjJfa2V5GA0gASgJSANSDnRodW1ibmFpbFIyS2V5iAEBEi0KEGNvbnZl'
    'cnRlZF9yMl9rZXkYDiABKAlIBFIOY29udmVydGVkUjJLZXmIAQESLwoEY2hhdBgPIAEoCzIWLn'
    'ZvaWNlLmNoYXQudjEuQ2hhdFJlZkgFUgRjaGF0iAEBEhUKBmlzX2UyZRgRIAEoCFIFaXNFMmUS'
    'PgoKZXhwaXJlc19hdBgSIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIBlIJZXhwaX'
    'Jlc0F0iAEBEh8KC3NjYW5fcmVzdWx0GBMgASgJUgpzY2FuUmVzdWx0EjkKCmNyZWF0ZWRfYXQY'
    'FCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgljcmVhdGVkQXQSSAoLc3RhdHVzX2'
    'VudW0YFSABKA4yIi52b2ljZS5maWxlLnYxLkZpbGVMaWZlY3ljbGVTdGF0dXNIB1IKc3RhdHVz'
    'RW51bYgBARJLCg5maWxlX3R5cGVfZW51bRgWIAEoDjIgLnZvaWNlLmZpbGUudjEuRmlsZU1lZG'
    'lhQ2F0ZWdvcnlICFIMZmlsZVR5cGVFbnVtiAEBEk0KEHNjYW5fcmVzdWx0X2VudW0YFyABKA4y'
    'Hi52b2ljZS5maWxlLnYxLkZpbGVTY2FuT3V0Y29tZUgJUg5zY2FuUmVzdWx0RW51bYgBAUIICg'
    'Zfd2lkdGhCCQoHX2hlaWdodEITChFfZHVyYXRpb25fc2Vjb25kc0ITChFfdGh1bWJuYWlsX3Iy'
    'X2tleUITChFfY29udmVydGVkX3IyX2tleUIHCgVfY2hhdEINCgtfZXhwaXJlc19hdEIOCgxfc3'
    'RhdHVzX2VudW1CEQoPX2ZpbGVfdHlwZV9lbnVtQhMKEV9zY2FuX3Jlc3VsdF9lbnVtSgQIEBAR'
    'UgljaGF0X3R5cGU=');

@$core.Deprecated('Use getFileURLRequestDescriptor instead')
const GetFileURLRequest$json = {
  '1': 'GetFileURLRequest',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `GetFileURLRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileURLRequestDescriptor = $convert.base64Decode(
    'ChFHZXRGaWxlVVJMUmVxdWVzdBIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQ=');

@$core.Deprecated('Use getFileMetadataRequestDescriptor instead')
const GetFileMetadataRequest$json = {
  '1': 'GetFileMetadataRequest',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `GetFileMetadataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileMetadataRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRGaWxlTWV0YWRhdGFSZXF1ZXN0EhcKB2ZpbGVfaWQYASABKAlSBmZpbGVJZA==');

@$core.Deprecated('Use getBulkMetadataRequestDescriptor instead')
const GetBulkMetadataRequest$json = {
  '1': 'GetBulkMetadataRequest',
  '2': [
    {'1': 'file_ids', '3': 1, '4': 3, '5': 9, '10': 'fileIds'},
  ],
};

/// Descriptor for `GetBulkMetadataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkMetadataRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRCdWxrTWV0YWRhdGFSZXF1ZXN0EhkKCGZpbGVfaWRzGAEgAygJUgdmaWxlSWRz');

@$core.Deprecated('Use bulkFileMetadataDescriptor instead')
const BulkFileMetadata$json = {
  '1': 'BulkFileMetadata',
  '2': [
    {
      '1': 'by_file_id',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.BulkFileMetadata.ByFileIdEntry',
      '10': 'byFileId'
    },
  ],
  '3': [BulkFileMetadata_ByFileIdEntry$json],
};

@$core.Deprecated('Use bulkFileMetadataDescriptor instead')
const BulkFileMetadata_ByFileIdEntry$json = {
  '1': 'ByFileIdEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileMetadata',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `BulkFileMetadata`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bulkFileMetadataDescriptor = $convert.base64Decode(
    'ChBCdWxrRmlsZU1ldGFkYXRhEksKCmJ5X2ZpbGVfaWQYASADKAsyLS52b2ljZS5maWxlLnYxLk'
    'J1bGtGaWxlTWV0YWRhdGEuQnlGaWxlSWRFbnRyeVIIYnlGaWxlSWQaWAoNQnlGaWxlSWRFbnRy'
    'eRIQCgNrZXkYASABKAlSA2tleRIxCgV2YWx1ZRgCIAEoCzIbLnZvaWNlLmZpbGUudjEuRmlsZU'
    '1ldGFkYXRhUgV2YWx1ZToCOAE=');

@$core.Deprecated('Use deleteFileRequestDescriptor instead')
const DeleteFileRequest$json = {
  '1': 'DeleteFileRequest',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `DeleteFileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteFileRequestDescriptor = $convert.base64Decode(
    'ChFEZWxldGVGaWxlUmVxdWVzdBIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQ=');

@$core.Deprecated('Use listFilesRequestDescriptor instead')
const ListFilesRequest$json = {
  '1': 'ListFilesRequest',
  '2': [
    {
      '1': 'filter_chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'filterChat',
      '17': true
    },
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
  '8': [
    {'1': '_filter_chat'},
  ],
};

/// Descriptor for `ListFilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFilesRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0RmlsZXNSZXF1ZXN0EjwKC2ZpbHRlcl9jaGF0GAEgASgLMhYudm9pY2UuY2hhdC52MS'
    '5DaGF0UmVmSABSCmZpbHRlckNoYXSIAQESNgoEcGFnZRgCIAEoCzIiLnZvaWNlLmNvbW1vbi52'
    'MS5DdXJzb3JQYWdlUmVxdWVzdFIEcGFnZUIOCgxfZmlsdGVyX2NoYXQ=');

@$core.Deprecated('Use fileListDescriptor instead')
const FileList$json = {
  '1': 'FileList',
  '2': [
    {
      '1': 'files',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.FileMetadata',
      '10': 'files'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
    {
      '1': 'page',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageResponse',
      '9': 0,
      '10': 'page',
      '17': true
    },
  ],
  '8': [
    {'1': '_page'},
  ],
};

/// Descriptor for `FileList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileListDescriptor = $convert.base64Decode(
    'CghGaWxlTGlzdBIxCgVmaWxlcxgBIAMoCzIbLnZvaWNlLmZpbGUudjEuRmlsZU1ldGFkYXRhUg'
    'VmaWxlcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvchI8CgRwYWdlGAMgASgLMiMu'
    'dm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXNwb25zZUgAUgRwYWdliAEBQgcKBV9wYWdl');

@$core.Deprecated('Use checkQuotaRequestDescriptor instead')
const CheckQuotaRequest$json = {
  '1': 'CheckQuotaRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `CheckQuotaRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkQuotaRequestDescriptor = $convert.base64Decode(
    'ChFDaGVja1F1b3RhUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use quotaResponseDescriptor instead')
const QuotaResponse$json = {
  '1': 'QuotaResponse',
  '2': [
    {'1': 'bytes_used', '3': 1, '4': 1, '5': 3, '10': 'bytesUsed'},
    {'1': 'bytes_limit', '3': 2, '4': 1, '5': 3, '10': 'bytesLimit'},
  ],
};

/// Descriptor for `QuotaResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List quotaResponseDescriptor = $convert.base64Decode(
    'Cg1RdW90YVJlc3BvbnNlEh0KCmJ5dGVzX3VzZWQYASABKANSCWJ5dGVzVXNlZBIfCgtieXRlc1'
    '9saW1pdBgCIAEoA1IKYnl0ZXNMaW1pdA==');

@$core.Deprecated('Use requestUploadResponseDescriptor instead')
const RequestUploadResponse$json = {
  '1': 'RequestUploadResponse',
  '2': [
    {
      '1': 'upload_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.UploadResponse',
      '10': 'uploadResponse'
    },
  ],
};

/// Descriptor for `RequestUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List requestUploadResponseDescriptor = $convert.base64Decode(
    'ChVSZXF1ZXN0VXBsb2FkUmVzcG9uc2USRgoPdXBsb2FkX3Jlc3BvbnNlGAEgASgLMh0udm9pY2'
    'UuZmlsZS52MS5VcGxvYWRSZXNwb25zZVIOdXBsb2FkUmVzcG9uc2U=');

@$core.Deprecated('Use confirmUploadResponseDescriptor instead')
const ConfirmUploadResponse$json = {
  '1': 'ConfirmUploadResponse',
  '2': [
    {
      '1': 'file_metadata',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileMetadata',
      '10': 'fileMetadata'
    },
  ],
};

/// Descriptor for `ConfirmUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List confirmUploadResponseDescriptor = $convert.base64Decode(
    'ChVDb25maXJtVXBsb2FkUmVzcG9uc2USQAoNZmlsZV9tZXRhZGF0YRgBIAEoCzIbLnZvaWNlLm'
    'ZpbGUudjEuRmlsZU1ldGFkYXRhUgxmaWxlTWV0YWRhdGE=');

@$core.Deprecated('Use getFileURLResponseDescriptor instead')
const GetFileURLResponse$json = {
  '1': 'GetFileURLResponse',
  '2': [
    {'1': 'presigned_get_url', '3': 1, '4': 1, '5': 9, '10': 'presignedGetUrl'},
    {
      '1': 'expires_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `GetFileURLResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileURLResponseDescriptor = $convert.base64Decode(
    'ChJHZXRGaWxlVVJMUmVzcG9uc2USKgoRcHJlc2lnbmVkX2dldF91cmwYASABKAlSD3ByZXNpZ2'
    '5lZEdldFVybBI5CgpleHBpcmVzX2F0GAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFt'
    'cFIJZXhwaXJlc0F0');

@$core.Deprecated('Use getFileMetadataResponseDescriptor instead')
const GetFileMetadataResponse$json = {
  '1': 'GetFileMetadataResponse',
  '2': [
    {
      '1': 'file_metadata',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileMetadata',
      '10': 'fileMetadata'
    },
  ],
};

/// Descriptor for `GetFileMetadataResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileMetadataResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRGaWxlTWV0YWRhdGFSZXNwb25zZRJACg1maWxlX21ldGFkYXRhGAEgASgLMhsudm9pY2'
        'UuZmlsZS52MS5GaWxlTWV0YWRhdGFSDGZpbGVNZXRhZGF0YQ==');

@$core.Deprecated('Use getBulkMetadataResponseDescriptor instead')
const GetBulkMetadataResponse$json = {
  '1': 'GetBulkMetadataResponse',
  '2': [
    {
      '1': 'bulk_file_metadata',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.BulkFileMetadata',
      '10': 'bulkFileMetadata'
    },
  ],
};

/// Descriptor for `GetBulkMetadataResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkMetadataResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRCdWxrTWV0YWRhdGFSZXNwb25zZRJNChJidWxrX2ZpbGVfbWV0YWRhdGEYASABKAsyHy'
        '52b2ljZS5maWxlLnYxLkJ1bGtGaWxlTWV0YWRhdGFSEGJ1bGtGaWxlTWV0YWRhdGE=');

@$core.Deprecated('Use deleteFileResponseDescriptor instead')
const DeleteFileResponse$json = {
  '1': 'DeleteFileResponse',
};

/// Descriptor for `DeleteFileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteFileResponseDescriptor =
    $convert.base64Decode('ChJEZWxldGVGaWxlUmVzcG9uc2U=');

@$core.Deprecated('Use listFilesResponseDescriptor instead')
const ListFilesResponse$json = {
  '1': 'ListFilesResponse',
  '2': [
    {
      '1': 'file_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileList',
      '10': 'fileList'
    },
  ],
};

/// Descriptor for `ListFilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFilesResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0RmlsZXNSZXNwb25zZRI0CglmaWxlX2xpc3QYASABKAsyFy52b2ljZS5maWxlLnYxLk'
    'ZpbGVMaXN0UghmaWxlTGlzdA==');

@$core.Deprecated('Use checkQuotaResponseDescriptor instead')
const CheckQuotaResponse$json = {
  '1': 'CheckQuotaResponse',
  '2': [
    {
      '1': 'quota_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.QuotaResponse',
      '10': 'quotaResponse'
    },
  ],
};

/// Descriptor for `CheckQuotaResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkQuotaResponseDescriptor = $convert.base64Decode(
    'ChJDaGVja1F1b3RhUmVzcG9uc2USQwoOcXVvdGFfcmVzcG9uc2UYASABKAsyHC52b2ljZS5maW'
    'xlLnYxLlF1b3RhUmVzcG9uc2VSDXF1b3RhUmVzcG9uc2U=');
