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
    {'1': 'FILE_LIFECYCLE_STATUS_EXPIRED', '2': 6},
  ],
};

/// Descriptor for `FileLifecycleStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileLifecycleStatusDescriptor = $convert.base64Decode(
    'ChNGaWxlTGlmZWN5Y2xlU3RhdHVzEiUKIUZJTEVfTElGRUNZQ0xFX1NUQVRVU19VTlNQRUNJRk'
    'lFRBAAEigKJEZJTEVfTElGRUNZQ0xFX1NUQVRVU19QRU5ESU5HX1VQTE9BRBABEiQKIEZJTEVf'
    'TElGRUNZQ0xFX1NUQVRVU19QUk9DRVNTSU5HEAISHwobRklMRV9MSUZFQ1lDTEVfU1RBVFVTX1'
    'JFQURZEAMSIAocRklMRV9MSUZFQ1lDTEVfU1RBVFVTX0ZBSUxFRBAEEiEKHUZJTEVfTElGRUNZ'
    'Q0xFX1NUQVRVU19ERUxFVEVEEAUSIQodRklMRV9MSUZFQ1lDTEVfU1RBVFVTX0VYUElSRUQQBg'
    '==');

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

@$core.Deprecated('Use fileReferenceOwnerTypeDescriptor instead')
const FileReferenceOwnerType$json = {
  '1': 'FileReferenceOwnerType',
  '2': [
    {'1': 'FILE_REFERENCE_OWNER_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_MESSAGE', '2': 1},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_STORY', '2': 2},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_PROFILE_AVATAR', '2': 3},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_CHAT_AVATAR', '2': 4},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_SPACE_AVATAR', '2': 5},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_STICKER', '2': 6},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_GIF_ASSET', '2': 7},
    {'1': 'FILE_REFERENCE_OWNER_TYPE_UPLOAD_SESSION', '2': 8},
  ],
};

/// Descriptor for `FileReferenceOwnerType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileReferenceOwnerTypeDescriptor = $convert.base64Decode(
    'ChZGaWxlUmVmZXJlbmNlT3duZXJUeXBlEikKJUZJTEVfUkVGRVJFTkNFX09XTkVSX1RZUEVfVU'
    '5TUEVDSUZJRUQQABIlCiFGSUxFX1JFRkVSRU5DRV9PV05FUl9UWVBFX01FU1NBR0UQARIjCh9G'
    'SUxFX1JFRkVSRU5DRV9PV05FUl9UWVBFX1NUT1JZEAISLAooRklMRV9SRUZFUkVOQ0VfT1dORV'
    'JfVFlQRV9QUk9GSUxFX0FWQVRBUhADEikKJUZJTEVfUkVGRVJFTkNFX09XTkVSX1RZUEVfQ0hB'
    'VF9BVkFUQVIQBBIqCiZGSUxFX1JFRkVSRU5DRV9PV05FUl9UWVBFX1NQQUNFX0FWQVRBUhAFEi'
    'UKIUZJTEVfUkVGRVJFTkNFX09XTkVSX1RZUEVfU1RJQ0tFUhAGEicKI0ZJTEVfUkVGRVJFTkNF'
    'X09XTkVSX1RZUEVfR0lGX0FTU0VUEAcSLAooRklMRV9SRUZFUkVOQ0VfT1dORVJfVFlQRV9VUE'
    'xPQURfU0VTU0lPThAI');

@$core.Deprecated('Use fileReadSurfaceDescriptor instead')
const FileReadSurface$json = {
  '1': 'FileReadSurface',
  '2': [
    {'1': 'FILE_READ_SURFACE_UNSPECIFIED', '2': 0},
    {'1': 'FILE_READ_SURFACE_URL', '2': 1},
    {'1': 'FILE_READ_SURFACE_METADATA', '2': 2},
  ],
};

/// Descriptor for `FileReadSurface`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileReadSurfaceDescriptor = $convert.base64Decode(
    'Cg9GaWxlUmVhZFN1cmZhY2USIQodRklMRV9SRUFEX1NVUkZBQ0VfVU5TUEVDSUZJRUQQABIZCh'
    'VGSUxFX1JFQURfU1VSRkFDRV9VUkwQARIeChpGSUxFX1JFQURfU1VSRkFDRV9NRVRBREFUQRAC');

@$core.Deprecated('Use fileReferenceProducerIdDescriptor instead')
const FileReferenceProducerId$json = {
  '1': 'FileReferenceProducerId',
  '2': [
    {'1': 'FILE_REFERENCE_PRODUCER_ID_UNSPECIFIED', '2': 0},
    {'1': 'FILE_REFERENCE_PRODUCER_ID_SPACE', '2': 1},
    {'1': 'FILE_REFERENCE_PRODUCER_ID_CHAT', '2': 2},
    {'1': 'FILE_REFERENCE_PRODUCER_ID_MESSAGING', '2': 3},
  ],
};

/// Descriptor for `FileReferenceProducerId`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List fileReferenceProducerIdDescriptor = $convert.base64Decode(
    'ChdGaWxlUmVmZXJlbmNlUHJvZHVjZXJJZBIqCiZGSUxFX1JFRkVSRU5DRV9QUk9EVUNFUl9JRF'
    '9VTlNQRUNJRklFRBAAEiQKIEZJTEVfUkVGRVJFTkNFX1BST0RVQ0VSX0lEX1NQQUNFEAESIwof'
    'RklMRV9SRUZFUkVOQ0VfUFJPRFVDRVJfSURfQ0hBVBACEigKJEZJTEVfUkVGRVJFTkNFX1BST0'
    'RVQ0VSX0lEX01FU1NBR0lORxAD');

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
    {
      '1': 'context_story',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.StoryRef',
      '9': 2,
      '10': 'contextStory',
      '17': true
    },
  ],
  '8': [
    {'1': '_context_chat'},
    {'1': '_is_e2e'},
    {'1': '_context_story'},
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
    'BSC2NvbnRleHRDaGF0iAEBEhoKBmlzX2UyZRgGIAEoCEgBUgVpc0UyZYgBARJCCg1jb250ZXh0'
    'X3N0b3J5GAcgASgLMhgudm9pY2Uuc3RvcnkudjEuU3RvcnlSZWZIAlIMY29udGV4dFN0b3J5iA'
    'EBQg8KDV9jb250ZXh0X2NoYXRCCQoHX2lzX2UyZUIQCg5fY29udGV4dF9zdG9yeUoECAUQBlIJ'
    'Y2hhdF90eXBl');

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
    {
      '1': 'access',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileAccessSelector',
      '10': 'access'
    },
  ],
};

/// Descriptor for `GetFileURLRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileURLRequestDescriptor = $convert.base64Decode(
    'ChFHZXRGaWxlVVJMUmVxdWVzdBIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQSOQoGYWNjZXNzGA'
    'IgASgLMiEudm9pY2UuZmlsZS52MS5GaWxlQWNjZXNzU2VsZWN0b3JSBmFjY2Vzcw==');

@$core.Deprecated('Use getFileMetadataRequestDescriptor instead')
const GetFileMetadataRequest$json = {
  '1': 'GetFileMetadataRequest',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {
      '1': 'access',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileAccessSelector',
      '10': 'access'
    },
  ],
};

/// Descriptor for `GetFileMetadataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFileMetadataRequestDescriptor = $convert.base64Decode(
    'ChZHZXRGaWxlTWV0YWRhdGFSZXF1ZXN0EhcKB2ZpbGVfaWQYASABKAlSBmZpbGVJZBI5CgZhY2'
    'Nlc3MYAiABKAsyIS52b2ljZS5maWxlLnYxLkZpbGVBY2Nlc3NTZWxlY3RvclIGYWNjZXNz');

@$core.Deprecated('Use getBulkMetadataRequestDescriptor instead')
const GetBulkMetadataRequest$json = {
  '1': 'GetBulkMetadataRequest',
  '2': [
    {
      '1': 'file_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '8': {'3': true},
      '10': 'fileIds',
    },
    {
      '1': 'items',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.FileAccessItem',
      '10': 'items'
    },
  ],
};

/// Descriptor for `GetBulkMetadataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkMetadataRequestDescriptor = $convert.base64Decode(
    'ChZHZXRCdWxrTWV0YWRhdGFSZXF1ZXN0Eh0KCGZpbGVfaWRzGAEgAygJQgIYAVIHZmlsZUlkcx'
    'IzCgVpdGVtcxgCIAMoCzIdLnZvaWNlLmZpbGUudjEuRmlsZUFjY2Vzc0l0ZW1SBWl0ZW1z');

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

@$core.Deprecated('Use fileReferenceProducerDeclarationDescriptor instead')
const FileReferenceProducerDeclaration$json = {
  '1': 'FileReferenceProducerDeclaration',
  '2': [
    {
      '1': 'producer_id',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {
      '1': 'expected_total_count',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'expectedTotalCount'
    },
    {
      '1': 'expected_references_sha256',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'expectedReferencesSha256'
    },
  ],
};

/// Descriptor for `FileReferenceProducerDeclaration`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileReferenceProducerDeclarationDescriptor =
    $convert.base64Decode(
        'CiBGaWxlUmVmZXJlbmNlUHJvZHVjZXJEZWNsYXJhdGlvbhJHCgtwcm9kdWNlcl9pZBgBIAEoDj'
        'ImLnZvaWNlLmZpbGUudjEuRmlsZVJlZmVyZW5jZVByb2R1Y2VySWRSCnByb2R1Y2VySWQSMAoU'
        'ZXhwZWN0ZWRfdG90YWxfY291bnQYAiABKARSEmV4cGVjdGVkVG90YWxDb3VudBI8ChpleHBlY3'
        'RlZF9yZWZlcmVuY2VzX3NoYTI1NhgDIAEoDFIYZXhwZWN0ZWRSZWZlcmVuY2VzU2hhMjU2');

@$core.Deprecated('Use fileReferenceKeyDescriptor instead')
const FileReferenceKey$json = {
  '1': 'FileReferenceKey',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {
      '1': 'owner_type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceOwnerType',
      '10': 'ownerType'
    },
    {'1': 'owner_id', '3': 3, '4': 1, '5': 9, '10': 'ownerId'},
    {
      '1': 'subresource_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'subresourceId',
      '17': true
    },
    {
      '1': 'scope_space_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'scopeSpaceId',
      '17': true
    },
  ],
  '8': [
    {'1': '_subresource_id'},
    {'1': '_scope_space_id'},
  ],
};

/// Descriptor for `FileReferenceKey`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileReferenceKeyDescriptor = $convert.base64Decode(
    'ChBGaWxlUmVmZXJlbmNlS2V5EhcKB2ZpbGVfaWQYASABKAlSBmZpbGVJZBJECgpvd25lcl90eX'
    'BlGAIgASgOMiUudm9pY2UuZmlsZS52MS5GaWxlUmVmZXJlbmNlT3duZXJUeXBlUglvd25lclR5'
    'cGUSGQoIb3duZXJfaWQYAyABKAlSB293bmVySWQSKgoOc3VicmVzb3VyY2VfaWQYBCABKAlIAF'
    'INc3VicmVzb3VyY2VJZIgBARIpCg5zY29wZV9zcGFjZV9pZBgFIAEoCUgBUgxzY29wZVNwYWNl'
    'SWSIAQFCEQoPX3N1YnJlc291cmNlX2lkQhEKD19zY29wZV9zcGFjZV9pZA==');

@$core.Deprecated('Use fileAccessSelectorDescriptor instead')
const FileAccessSelector$json = {
  '1': 'FileAccessSelector',
  '2': [
    {
      '1': 'reference',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '9': 0,
      '10': 'reference'
    },
    {
      '1': 'capability_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'capabilityId'
    },
  ],
  '8': [
    {'1': 'selector'},
  ],
};

/// Descriptor for `FileAccessSelector`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileAccessSelectorDescriptor = $convert.base64Decode(
    'ChJGaWxlQWNjZXNzU2VsZWN0b3ISPwoJcmVmZXJlbmNlGAEgASgLMh8udm9pY2UuZmlsZS52MS'
    '5GaWxlUmVmZXJlbmNlS2V5SABSCXJlZmVyZW5jZRIlCg1jYXBhYmlsaXR5X2lkGAIgASgJSABS'
    'DGNhcGFiaWxpdHlJZEIKCghzZWxlY3Rvcg==');

@$core.Deprecated('Use fileAccessItemDescriptor instead')
const FileAccessItem$json = {
  '1': 'FileAccessItem',
  '2': [
    {'1': 'file_id', '3': 1, '4': 1, '5': 9, '10': 'fileId'},
    {
      '1': 'access',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileAccessSelector',
      '10': 'access'
    },
  ],
};

/// Descriptor for `FileAccessItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List fileAccessItemDescriptor = $convert.base64Decode(
    'Cg5GaWxlQWNjZXNzSXRlbRIXCgdmaWxlX2lkGAEgASgJUgZmaWxlSWQSOQoGYWNjZXNzGAIgAS'
    'gLMiEudm9pY2UuZmlsZS52MS5GaWxlQWNjZXNzU2VsZWN0b3JSBmFjY2Vzcw==');

@$core.Deprecated('Use issueFileAccessCapabilityRequestDescriptor instead')
const IssueFileAccessCapabilityRequest$json = {
  '1': 'IssueFileAccessCapabilityRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'reference',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '10': 'reference'
    },
    {
      '1': 'subject_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'subjectProfileId'
    },
    {
      '1': 'allowed_surfaces',
      '3': 5,
      '4': 3,
      '5': 14,
      '6': '.voice.file.v1.FileReadSurface',
      '10': 'allowedSurfaces'
    },
    {
      '1': 'expires_at',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `IssueFileAccessCapabilityRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueFileAccessCapabilityRequestDescriptor = $convert.base64Decode(
    'CiBJc3N1ZUZpbGVBY2Nlc3NDYXBhYmlsaXR5UmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
    'EgASgNUg9wcm90b2NvbFZlcnNpb24SIQoMb3BlcmF0aW9uX2lkGAIgASgJUgtvcGVyYXRpb25J'
    'ZBI9CglyZWZlcmVuY2UYAyABKAsyHy52b2ljZS5maWxlLnYxLkZpbGVSZWZlcmVuY2VLZXlSCX'
    'JlZmVyZW5jZRIsChJzdWJqZWN0X3Byb2ZpbGVfaWQYBCABKAlSEHN1YmplY3RQcm9maWxlSWQS'
    'SQoQYWxsb3dlZF9zdXJmYWNlcxgFIAMoDjIeLnZvaWNlLmZpbGUudjEuRmlsZVJlYWRTdXJmYW'
    'NlUg9hbGxvd2VkU3VyZmFjZXMSOQoKZXhwaXJlc19hdBgGIAEoCzIaLmdvb2dsZS5wcm90b2J1'
    'Zi5UaW1lc3RhbXBSCWV4cGlyZXNBdA==');

@$core.Deprecated('Use issueFileAccessCapabilityResponseDescriptor instead')
const IssueFileAccessCapabilityResponse$json = {
  '1': 'IssueFileAccessCapabilityResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.IssueFileAccessCapabilityReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `IssueFileAccessCapabilityResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueFileAccessCapabilityResponseDescriptor =
    $convert.base64Decode(
        'CiFJc3N1ZUZpbGVBY2Nlc3NDYXBhYmlsaXR5UmVzcG9uc2USSQoHcmVjZWlwdBgBIAEoCzIvLn'
        'ZvaWNlLmZpbGUudjEuSXNzdWVGaWxlQWNjZXNzQ2FwYWJpbGl0eVJlY2VpcHRSB3JlY2VpcHQ=');

@$core.Deprecated('Use issueFileAccessCapabilityReceiptDescriptor instead')
const IssueFileAccessCapabilityReceipt$json = {
  '1': 'IssueFileAccessCapabilityReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'capability_id', '3': 4, '4': 1, '5': 9, '10': 'capabilityId'},
    {
      '1': 'reference',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '10': 'reference'
    },
    {
      '1': 'subject_profile_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'subjectProfileId'
    },
    {
      '1': 'allowed_surfaces',
      '3': 7,
      '4': 3,
      '5': 14,
      '6': '.voice.file.v1.FileReadSurface',
      '10': 'allowedSurfaces'
    },
    {
      '1': 'expires_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {'1': 'request_sha256', '3': 9, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `IssueFileAccessCapabilityReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueFileAccessCapabilityReceiptDescriptor = $convert.base64Decode(
    'CiBJc3N1ZUZpbGVBY2Nlc3NDYXBhYmlsaXR5UmVjZWlwdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
    'EgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJcmVjZWlwdElkEiEK'
    'DG9wZXJhdGlvbl9pZBgDIAEoCVILb3BlcmF0aW9uSWQSIwoNY2FwYWJpbGl0eV9pZBgEIAEoCV'
    'IMY2FwYWJpbGl0eUlkEj0KCXJlZmVyZW5jZRgFIAEoCzIfLnZvaWNlLmZpbGUudjEuRmlsZVJl'
    'ZmVyZW5jZUtleVIJcmVmZXJlbmNlEiwKEnN1YmplY3RfcHJvZmlsZV9pZBgGIAEoCVIQc3Viam'
    'VjdFByb2ZpbGVJZBJJChBhbGxvd2VkX3N1cmZhY2VzGAcgAygOMh4udm9pY2UuZmlsZS52MS5G'
    'aWxlUmVhZFN1cmZhY2VSD2FsbG93ZWRTdXJmYWNlcxI5CgpleHBpcmVzX2F0GAggASgLMhouZ2'
    '9vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJZXhwaXJlc0F0EiUKDnJlcXVlc3Rfc2hhMjU2GAkg'
    'ASgMUg1yZXF1ZXN0U2hhMjU2Ej0KDGNvbXBsZXRlZF9hdBgKIAEoCzIaLmdvb2dsZS5wcm90b2'
    'J1Zi5UaW1lc3RhbXBSC2NvbXBsZXRlZEF0');

@$core.Deprecated(
    'Use prepareSpaceDeletionReferenceManifestRequestDescriptor instead')
const PrepareSpaceDeletionReferenceManifestRequest$json = {
  '1': 'PrepareSpaceDeletionReferenceManifestRequest',
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
      '1': 'chat_manifest',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.ManifestBinding',
      '10': 'chatManifest'
    },
  ],
};

/// Descriptor for `PrepareSpaceDeletionReferenceManifestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    prepareSpaceDeletionReferenceManifestRequestDescriptor =
    $convert.base64Decode(
        'CixQcmVwYXJlU3BhY2VEZWxldGlvblJlZmVyZW5jZU1hbmlmZXN0UmVxdWVzdBIpChBwcm90b2'
        'NvbF92ZXJzaW9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3Nw'
        'YWNlSWQSMgoVZGVsZXRpb25fb3BlcmF0aW9uX2lkGAMgASgJUhNkZWxldGlvbk9wZXJhdGlvbk'
        'lkEi8KE3NjaGVkdWxlX2dlbmVyYXRpb24YBCABKARSEnNjaGVkdWxlR2VuZXJhdGlvbhJFCg1j'
        'aGF0X21hbmlmZXN0GAUgASgLMiAudm9pY2UuY29tbW9uLnYxLk1hbmlmZXN0QmluZGluZ1IMY2'
        'hhdE1hbmlmZXN0');

@$core.Deprecated(
    'Use prepareSpaceDeletionReferenceManifestReceiptDescriptor instead')
const PrepareSpaceDeletionReferenceManifestReceipt$json = {
  '1': 'PrepareSpaceDeletionReferenceManifestReceipt',
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
    {
      '1': 'schedule_generation',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'scheduleGeneration'
    },
    {
      '1': 'applied_state',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.common.v1.LifecycleFenceState',
      '10': 'appliedState'
    },
    {'1': 'request_sha256', '3': 7, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'applied_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'appliedAt'
    },
  ],
};

/// Descriptor for `PrepareSpaceDeletionReferenceManifestReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List prepareSpaceDeletionReferenceManifestReceiptDescriptor = $convert.base64Decode(
    'CixQcmVwYXJlU3BhY2VEZWxldGlvblJlZmVyZW5jZU1hbmlmZXN0UmVjZWlwdBIpChBwcm90b2'
    'NvbF92ZXJzaW9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJ'
    'cmVjZWlwdElkEhkKCHNwYWNlX2lkGAMgASgJUgdzcGFjZUlkEjIKFWRlbGV0aW9uX29wZXJhdG'
    'lvbl9pZBgEIAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIvChNzY2hlZHVsZV9nZW5lcmF0aW9u'
    'GAUgASgEUhJzY2hlZHVsZUdlbmVyYXRpb24SSQoNYXBwbGllZF9zdGF0ZRgGIAEoDjIkLnZvaW'
    'NlLmNvbW1vbi52MS5MaWZlY3ljbGVGZW5jZVN0YXRlUgxhcHBsaWVkU3RhdGUSJQoOcmVxdWVz'
    'dF9zaGEyNTYYByABKAxSDXJlcXVlc3RTaGEyNTYSOQoKYXBwbGllZF9hdBgIIAEoCzIaLmdvb2'
    'dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWFwcGxpZWRBdA==');

@$core.Deprecated(
    'Use prepareSpaceDeletionReferenceManifestResponseDescriptor instead')
const PrepareSpaceDeletionReferenceManifestResponse$json = {
  '1': 'PrepareSpaceDeletionReferenceManifestResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.PrepareSpaceDeletionReferenceManifestReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `PrepareSpaceDeletionReferenceManifestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    prepareSpaceDeletionReferenceManifestResponseDescriptor =
    $convert.base64Decode(
        'Ci1QcmVwYXJlU3BhY2VEZWxldGlvblJlZmVyZW5jZU1hbmlmZXN0UmVzcG9uc2USVQoHcmVjZW'
        'lwdBgBIAEoCzI7LnZvaWNlLmZpbGUudjEuUHJlcGFyZVNwYWNlRGVsZXRpb25SZWZlcmVuY2VN'
        'YW5pZmVzdFJlY2VpcHRSB3JlY2VpcHQ=');

@$core.Deprecated(
    'Use registerSpaceDeletionReferenceChunkRequestDescriptor instead')
const RegisterSpaceDeletionReferenceChunkRequest$json = {
  '1': 'RegisterSpaceDeletionReferenceChunkRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'producer_id',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'deletion_operation_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'space_id', '3': 5, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'schedule_generation',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'scheduleGeneration'
    },
    {'1': 'chunk_index', '3': 7, '4': 1, '5': 4, '10': 'chunkIndex'},
    {
      '1': 'references',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '10': 'references'
    },
    {
      '1': 'expected_total_count',
      '3': 9,
      '4': 1,
      '5': 4,
      '10': 'expectedTotalCount'
    },
    {
      '1': 'expected_references_sha256',
      '3': 10,
      '4': 1,
      '5': 12,
      '10': 'expectedReferencesSha256'
    },
    {'1': 'seals_producer', '3': 11, '4': 1, '5': 8, '10': 'sealsProducer'},
  ],
};

/// Descriptor for `RegisterSpaceDeletionReferenceChunkRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerSpaceDeletionReferenceChunkRequestDescriptor = $convert.base64Decode(
    'CipSZWdpc3RlclNwYWNlRGVsZXRpb25SZWZlcmVuY2VDaHVua1JlcXVlc3QSKQoQcHJvdG9jb2'
    'xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEkcKC3Byb2R1Y2VyX2lkGAIgASgOMiYu'
    'dm9pY2UuZmlsZS52MS5GaWxlUmVmZXJlbmNlUHJvZHVjZXJJZFIKcHJvZHVjZXJJZBIhCgxvcG'
    'VyYXRpb25faWQYAyABKAlSC29wZXJhdGlvbklkEjIKFWRlbGV0aW9uX29wZXJhdGlvbl9pZBgE'
    'IAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIZCghzcGFjZV9pZBgFIAEoCVIHc3BhY2VJZBIvCh'
    'NzY2hlZHVsZV9nZW5lcmF0aW9uGAYgASgEUhJzY2hlZHVsZUdlbmVyYXRpb24SHwoLY2h1bmtf'
    'aW5kZXgYByABKARSCmNodW5rSW5kZXgSPwoKcmVmZXJlbmNlcxgIIAMoCzIfLnZvaWNlLmZpbG'
    'UudjEuRmlsZVJlZmVyZW5jZUtleVIKcmVmZXJlbmNlcxIwChRleHBlY3RlZF90b3RhbF9jb3Vu'
    'dBgJIAEoBFISZXhwZWN0ZWRUb3RhbENvdW50EjwKGmV4cGVjdGVkX3JlZmVyZW5jZXNfc2hhMj'
    'U2GAogASgMUhhleHBlY3RlZFJlZmVyZW5jZXNTaGEyNTYSJQoOc2VhbHNfcHJvZHVjZXIYCyAB'
    'KAhSDXNlYWxzUHJvZHVjZXI=');

@$core.Deprecated(
    'Use registerSpaceDeletionReferenceChunkReceiptDescriptor instead')
const RegisterSpaceDeletionReferenceChunkReceipt$json = {
  '1': 'RegisterSpaceDeletionReferenceChunkReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'deletion_operation_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'space_id', '3': 5, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'schedule_generation',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'scheduleGeneration'
    },
    {'1': 'chunk_index', '3': 7, '4': 1, '5': 4, '10': 'chunkIndex'},
    {'1': 'accepted_count', '3': 8, '4': 1, '5': 4, '10': 'acceptedCount'},
    {
      '1': 'producer_id',
      '3': 9,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {
      '1': 'expected_total_count',
      '3': 10,
      '4': 1,
      '5': 4,
      '10': 'expectedTotalCount'
    },
    {
      '1': 'expected_references_sha256',
      '3': 11,
      '4': 1,
      '5': 12,
      '10': 'expectedReferencesSha256'
    },
    {'1': 'producer_sealed', '3': 12, '4': 1, '5': 8, '10': 'producerSealed'},
    {'1': 'request_sha256', '3': 13, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `RegisterSpaceDeletionReferenceChunkReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerSpaceDeletionReferenceChunkReceiptDescriptor = $convert.base64Decode(
    'CipSZWdpc3RlclNwYWNlRGVsZXRpb25SZWZlcmVuY2VDaHVua1JlY2VpcHQSKQoQcHJvdG9jb2'
    'xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEh0KCnJlY2VpcHRfaWQYAiABKAlSCXJl'
    'Y2VpcHRJZBIhCgxvcGVyYXRpb25faWQYAyABKAlSC29wZXJhdGlvbklkEjIKFWRlbGV0aW9uX2'
    '9wZXJhdGlvbl9pZBgEIAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIZCghzcGFjZV9pZBgFIAEo'
    'CVIHc3BhY2VJZBIvChNzY2hlZHVsZV9nZW5lcmF0aW9uGAYgASgEUhJzY2hlZHVsZUdlbmVyYX'
    'Rpb24SHwoLY2h1bmtfaW5kZXgYByABKARSCmNodW5rSW5kZXgSJQoOYWNjZXB0ZWRfY291bnQY'
    'CCABKARSDWFjY2VwdGVkQ291bnQSRwoLcHJvZHVjZXJfaWQYCSABKA4yJi52b2ljZS5maWxlLn'
    'YxLkZpbGVSZWZlcmVuY2VQcm9kdWNlcklkUgpwcm9kdWNlcklkEjAKFGV4cGVjdGVkX3RvdGFs'
    'X2NvdW50GAogASgEUhJleHBlY3RlZFRvdGFsQ291bnQSPAoaZXhwZWN0ZWRfcmVmZXJlbmNlc1'
    '9zaGEyNTYYCyABKAxSGGV4cGVjdGVkUmVmZXJlbmNlc1NoYTI1NhInCg9wcm9kdWNlcl9zZWFs'
    'ZWQYDCABKAhSDnByb2R1Y2VyU2VhbGVkEiUKDnJlcXVlc3Rfc2hhMjU2GA0gASgMUg1yZXF1ZX'
    'N0U2hhMjU2Ej0KDGNvbXBsZXRlZF9hdBgOIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3Rh'
    'bXBSC2NvbXBsZXRlZEF0');

@$core.Deprecated(
    'Use registerSpaceDeletionReferenceChunkResponseDescriptor instead')
const RegisterSpaceDeletionReferenceChunkResponse$json = {
  '1': 'RegisterSpaceDeletionReferenceChunkResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.RegisterSpaceDeletionReferenceChunkReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `RegisterSpaceDeletionReferenceChunkResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    registerSpaceDeletionReferenceChunkResponseDescriptor =
    $convert.base64Decode(
        'CitSZWdpc3RlclNwYWNlRGVsZXRpb25SZWZlcmVuY2VDaHVua1Jlc3BvbnNlElMKB3JlY2VpcH'
        'QYASABKAsyOS52b2ljZS5maWxlLnYxLlJlZ2lzdGVyU3BhY2VEZWxldGlvblJlZmVyZW5jZUNo'
        'dW5rUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated(
    'Use releaseSpaceDeletionProducerReferencesRequestDescriptor instead')
const ReleaseSpaceDeletionProducerReferencesRequest$json = {
  '1': 'ReleaseSpaceDeletionProducerReferencesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'deletion_operation_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'purge_generation', '3': 4, '4': 1, '5': 4, '10': 'purgeGeneration'},
    {
      '1': 'source_schedule_generation',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'sourceScheduleGeneration'
    },
    {
      '1': 'producer_id',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {
      '1': 'expected_references_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'expectedReferencesSha256'
    },
  ],
};

/// Descriptor for `ReleaseSpaceDeletionProducerReferencesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List releaseSpaceDeletionProducerReferencesRequestDescriptor = $convert.base64Decode(
    'Ci1SZWxlYXNlU3BhY2VEZWxldGlvblByb2R1Y2VyUmVmZXJlbmNlc1JlcXVlc3QSKQoQcHJvdG'
    '9jb2xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEjIKFWRlbGV0aW9uX29wZXJhdGlv'
    'bl9pZBgCIAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIZCghzcGFjZV9pZBgDIAEoCVIHc3BhY2'
    'VJZBIpChBwdXJnZV9nZW5lcmF0aW9uGAQgASgEUg9wdXJnZUdlbmVyYXRpb24SPAoac291cmNl'
    'X3NjaGVkdWxlX2dlbmVyYXRpb24YBSABKARSGHNvdXJjZVNjaGVkdWxlR2VuZXJhdGlvbhJHCg'
    'twcm9kdWNlcl9pZBgGIAEoDjImLnZvaWNlLmZpbGUudjEuRmlsZVJlZmVyZW5jZVByb2R1Y2Vy'
    'SWRSCnByb2R1Y2VySWQSPAoaZXhwZWN0ZWRfcmVmZXJlbmNlc19zaGEyNTYYByABKAxSGGV4cG'
    'VjdGVkUmVmZXJlbmNlc1NoYTI1Ng==');

@$core.Deprecated(
    'Use releaseSpaceDeletionProducerReferencesReceiptDescriptor instead')
const ReleaseSpaceDeletionProducerReferencesReceipt$json = {
  '1': 'ReleaseSpaceDeletionProducerReferencesReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {
      '1': 'deletion_operation_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'purge_generation', '3': 5, '4': 1, '5': 4, '10': 'purgeGeneration'},
    {
      '1': 'source_schedule_generation',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'sourceScheduleGeneration'
    },
    {
      '1': 'producer_id',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {'1': 'released_count', '3': 8, '4': 1, '5': 4, '10': 'releasedCount'},
    {
      '1': 'expected_references_sha256',
      '3': 9,
      '4': 1,
      '5': 12,
      '10': 'expectedReferencesSha256'
    },
    {'1': 'request_sha256', '3': 10, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `ReleaseSpaceDeletionProducerReferencesReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List releaseSpaceDeletionProducerReferencesReceiptDescriptor = $convert.base64Decode(
    'Ci1SZWxlYXNlU3BhY2VEZWxldGlvblByb2R1Y2VyUmVmZXJlbmNlc1JlY2VpcHQSKQoQcHJvdG'
    '9jb2xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEh0KCnJlY2VpcHRfaWQYAiABKAlS'
    'CXJlY2VpcHRJZBIyChVkZWxldGlvbl9vcGVyYXRpb25faWQYAyABKAlSE2RlbGV0aW9uT3Blcm'
    'F0aW9uSWQSGQoIc3BhY2VfaWQYBCABKAlSB3NwYWNlSWQSKQoQcHVyZ2VfZ2VuZXJhdGlvbhgF'
    'IAEoBFIPcHVyZ2VHZW5lcmF0aW9uEjwKGnNvdXJjZV9zY2hlZHVsZV9nZW5lcmF0aW9uGAYgAS'
    'gEUhhzb3VyY2VTY2hlZHVsZUdlbmVyYXRpb24SRwoLcHJvZHVjZXJfaWQYByABKA4yJi52b2lj'
    'ZS5maWxlLnYxLkZpbGVSZWZlcmVuY2VQcm9kdWNlcklkUgpwcm9kdWNlcklkEiUKDnJlbGVhc2'
    'VkX2NvdW50GAggASgEUg1yZWxlYXNlZENvdW50EjwKGmV4cGVjdGVkX3JlZmVyZW5jZXNfc2hh'
    'MjU2GAkgASgMUhhleHBlY3RlZFJlZmVyZW5jZXNTaGEyNTYSJQoOcmVxdWVzdF9zaGEyNTYYCi'
    'ABKAxSDXJlcXVlc3RTaGEyNTYSPQoMY29tcGxldGVkX2F0GAsgASgLMhouZ29vZ2xlLnByb3Rv'
    'YnVmLlRpbWVzdGFtcFILY29tcGxldGVkQXQ=');

@$core.Deprecated(
    'Use releaseSpaceDeletionProducerReferencesResponseDescriptor instead')
const ReleaseSpaceDeletionProducerReferencesResponse$json = {
  '1': 'ReleaseSpaceDeletionProducerReferencesResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.ReleaseSpaceDeletionProducerReferencesReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ReleaseSpaceDeletionProducerReferencesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    releaseSpaceDeletionProducerReferencesResponseDescriptor =
    $convert.base64Decode(
        'Ci5SZWxlYXNlU3BhY2VEZWxldGlvblByb2R1Y2VyUmVmZXJlbmNlc1Jlc3BvbnNlElYKB3JlY2'
        'VpcHQYASABKAsyPC52b2ljZS5maWxlLnYxLlJlbGVhc2VTcGFjZURlbGV0aW9uUHJvZHVjZXJS'
        'ZWZlcmVuY2VzUmVjZWlwdFIHcmVjZWlwdA==');

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

@$core.Deprecated('Use acquireFileReferencesRequestDescriptor instead')
const AcquireFileReferencesRequest$json = {
  '1': 'AcquireFileReferencesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'producer_id',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {
      '1': 'references',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '10': 'references'
    },
  ],
};

/// Descriptor for `AcquireFileReferencesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acquireFileReferencesRequestDescriptor = $convert.base64Decode(
    'ChxBY3F1aXJlRmlsZVJlZmVyZW5jZXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlvbklkEkcK'
    'C3Byb2R1Y2VyX2lkGAMgASgOMiYudm9pY2UuZmlsZS52MS5GaWxlUmVmZXJlbmNlUHJvZHVjZX'
    'JJZFIKcHJvZHVjZXJJZBI/CgpyZWZlcmVuY2VzGAQgAygLMh8udm9pY2UuZmlsZS52MS5GaWxl'
    'UmVmZXJlbmNlS2V5UgpyZWZlcmVuY2Vz');

@$core.Deprecated('Use acquireFileReferencesReceiptDescriptor instead')
const AcquireFileReferencesReceipt$json = {
  '1': 'AcquireFileReferencesReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'producer_id',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {'1': 'reference_count', '3': 5, '4': 1, '5': 4, '10': 'referenceCount'},
    {'1': 'request_sha256', '3': 6, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `AcquireFileReferencesReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acquireFileReferencesReceiptDescriptor = $convert.base64Decode(
    'ChxBY3F1aXJlRmlsZVJlZmVyZW5jZXNSZWNlaXB0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZWNlaXB0X2lkGAIgASgJUglyZWNlaXB0SWQSIQoMb3Bl'
    'cmF0aW9uX2lkGAMgASgJUgtvcGVyYXRpb25JZBJHCgtwcm9kdWNlcl9pZBgEIAEoDjImLnZvaW'
    'NlLmZpbGUudjEuRmlsZVJlZmVyZW5jZVByb2R1Y2VySWRSCnByb2R1Y2VySWQSJwoPcmVmZXJl'
    'bmNlX2NvdW50GAUgASgEUg5yZWZlcmVuY2VDb3VudBIlCg5yZXF1ZXN0X3NoYTI1NhgGIAEoDF'
    'INcmVxdWVzdFNoYTI1NhI9Cgxjb21wbGV0ZWRfYXQYByABKAsyGi5nb29nbGUucHJvdG9idWYu'
    'VGltZXN0YW1wUgtjb21wbGV0ZWRBdA==');

@$core.Deprecated('Use acquireFileReferencesResponseDescriptor instead')
const AcquireFileReferencesResponse$json = {
  '1': 'AcquireFileReferencesResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.AcquireFileReferencesReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `AcquireFileReferencesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acquireFileReferencesResponseDescriptor =
    $convert.base64Decode(
        'Ch1BY3F1aXJlRmlsZVJlZmVyZW5jZXNSZXNwb25zZRJFCgdyZWNlaXB0GAEgASgLMisudm9pY2'
        'UuZmlsZS52MS5BY3F1aXJlRmlsZVJlZmVyZW5jZXNSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use releaseFileReferencesRequestDescriptor instead')
const ReleaseFileReferencesRequest$json = {
  '1': 'ReleaseFileReferencesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'producer_id',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {
      '1': 'references',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.file.v1.FileReferenceKey',
      '10': 'references'
    },
  ],
};

/// Descriptor for `ReleaseFileReferencesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List releaseFileReferencesRequestDescriptor = $convert.base64Decode(
    'ChxSZWxlYXNlRmlsZVJlZmVyZW5jZXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlvbklkEkcK'
    'C3Byb2R1Y2VyX2lkGAMgASgOMiYudm9pY2UuZmlsZS52MS5GaWxlUmVmZXJlbmNlUHJvZHVjZX'
    'JJZFIKcHJvZHVjZXJJZBI/CgpyZWZlcmVuY2VzGAQgAygLMh8udm9pY2UuZmlsZS52MS5GaWxl'
    'UmVmZXJlbmNlS2V5UgpyZWZlcmVuY2Vz');

@$core.Deprecated('Use releaseFileReferencesReceiptDescriptor instead')
const ReleaseFileReferencesReceipt$json = {
  '1': 'ReleaseFileReferencesReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'producer_id',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.file.v1.FileReferenceProducerId',
      '10': 'producerId'
    },
    {'1': 'released_count', '3': 5, '4': 1, '5': 4, '10': 'releasedCount'},
    {'1': 'request_sha256', '3': 6, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'completed_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
  ],
};

/// Descriptor for `ReleaseFileReferencesReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List releaseFileReferencesReceiptDescriptor = $convert.base64Decode(
    'ChxSZWxlYXNlRmlsZVJlZmVyZW5jZXNSZWNlaXB0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZWNlaXB0X2lkGAIgASgJUglyZWNlaXB0SWQSIQoMb3Bl'
    'cmF0aW9uX2lkGAMgASgJUgtvcGVyYXRpb25JZBJHCgtwcm9kdWNlcl9pZBgEIAEoDjImLnZvaW'
    'NlLmZpbGUudjEuRmlsZVJlZmVyZW5jZVByb2R1Y2VySWRSCnByb2R1Y2VySWQSJQoOcmVsZWFz'
    'ZWRfY291bnQYBSABKARSDXJlbGVhc2VkQ291bnQSJQoOcmVxdWVzdF9zaGEyNTYYBiABKAxSDX'
    'JlcXVlc3RTaGEyNTYSPQoMY29tcGxldGVkX2F0GAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRp'
    'bWVzdGFtcFILY29tcGxldGVkQXQ=');

@$core.Deprecated('Use releaseFileReferencesResponseDescriptor instead')
const ReleaseFileReferencesResponse$json = {
  '1': 'ReleaseFileReferencesResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.file.v1.ReleaseFileReferencesReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ReleaseFileReferencesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List releaseFileReferencesResponseDescriptor =
    $convert.base64Decode(
        'Ch1SZWxlYXNlRmlsZVJlZmVyZW5jZXNSZXNwb25zZRJFCgdyZWNlaXB0GAEgASgLMisudm9pY2'
        'UuZmlsZS52MS5SZWxlYXNlRmlsZVJlZmVyZW5jZXNSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use getSpacePurgeReceiptRequestDescriptor instead')
const GetSpacePurgeReceiptRequest$json = {
  '1': 'GetSpacePurgeReceiptRequest',
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
      '1': 'purge_request_sha256',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'purgeRequestSha256'
    },
    {'1': 'manifest_sha256', '3': 6, '4': 1, '5': 12, '10': 'manifestSha256'},
  ],
};

/// Descriptor for `GetSpacePurgeReceiptRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpacePurgeReceiptRequestDescriptor = $convert.base64Decode(
    'ChtHZXRTcGFjZVB1cmdlUmVjZWlwdFJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDV'
    'IPcHJvdG9jb2xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEjIKFWRlbGV0aW9u'
    'X29wZXJhdGlvbl9pZBgDIAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIeCgpnZW5lcmF0aW9uGA'
    'QgASgEUgpnZW5lcmF0aW9uEjAKFHB1cmdlX3JlcXVlc3Rfc2hhMjU2GAUgASgMUhJwdXJnZVJl'
    'cXVlc3RTaGEyNTYSJwoPbWFuaWZlc3Rfc2hhMjU2GAYgASgMUg5tYW5pZmVzdFNoYTI1Ng==');

@$core.Deprecated('Use getSpacePurgeReceiptResponseDescriptor instead')
const GetSpacePurgeReceiptResponse$json = {
  '1': 'GetSpacePurgeReceiptResponse',
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

/// Descriptor for `GetSpacePurgeReceiptResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpacePurgeReceiptResponseDescriptor =
    $convert.base64Decode(
        'ChxHZXRTcGFjZVB1cmdlUmVjZWlwdFJlc3BvbnNlEjwKB3JlY2VpcHQYASABKAsyIi52b2ljZS'
        '5jb21tb24udjEuU3BhY2VQdXJnZVJlY2VpcHRSB3JlY2VpcHQ=');
