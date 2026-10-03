// This is a generated file - do not edit.
//
// Generated from voice/authority/v1/authority.proto.

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

@$core.Deprecated('Use authorityOwnerDescriptor instead')
const AuthorityOwner$json = {
  '1': 'AuthorityOwner',
  '2': [
    {'1': 'AUTHORITY_OWNER_UNSPECIFIED', '2': 0},
    {'1': 'AUTHORITY_OWNER_SPACE', '2': 1},
    {'1': 'AUTHORITY_OWNER_ROLE', '2': 2},
    {'1': 'AUTHORITY_OWNER_AUTH', '2': 3},
    {'1': 'AUTHORITY_OWNER_USER', '2': 4},
    {'1': 'AUTHORITY_OWNER_GAME_INTEGRATION', '2': 5},
  ],
};

/// Descriptor for `AuthorityOwner`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List authorityOwnerDescriptor = $convert.base64Decode(
    'Cg5BdXRob3JpdHlPd25lchIfChtBVVRIT1JJVFlfT1dORVJfVU5TUEVDSUZJRUQQABIZChVBVV'
    'RIT1JJVFlfT1dORVJfU1BBQ0UQARIYChRBVVRIT1JJVFlfT1dORVJfUk9MRRACEhgKFEFVVEhP'
    'UklUWV9PV05FUl9BVVRIEAMSGAoUQVVUSE9SSVRZX09XTkVSX1VTRVIQBBIkCiBBVVRIT1JJVF'
    'lfT1dORVJfR0FNRV9JTlRFR1JBVElPThAF');

@$core.Deprecated('Use sourceScopeDescriptor instead')
const SourceScope$json = {
  '1': 'SourceScope',
  '2': [
    {'1': 'schema_version', '3': 1, '4': 1, '5': 13, '10': 'schemaVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'environment_id', '3': 3, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
    {'1': 'account_ids', '3': 5, '4': 3, '5': 9, '10': 'accountIds'},
    {'1': 'voice_room_ids', '3': 6, '4': 3, '5': 9, '10': 'voiceRoomIds'},
  ],
};

/// Descriptor for `SourceScope`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sourceScopeDescriptor = $convert.base64Decode(
    'CgtTb3VyY2VTY29wZRIlCg5zY2hlbWFfdmVyc2lvbhgBIAEoDVINc2NoZW1hVmVyc2lvbhIZCg'
    'hzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBIlCg5lbnZpcm9ubWVudF9pZBgDIAEoCVINZW52aXJv'
    'bm1lbnRJZBIfCgtwcm9maWxlX2lkcxgEIAMoCVIKcHJvZmlsZUlkcxIfCgthY2NvdW50X2lkcx'
    'gFIAMoCVIKYWNjb3VudElkcxIkCg52b2ljZV9yb29tX2lkcxgGIAMoCVIMdm9pY2VSb29tSWRz');

@$core.Deprecated('Use readSnapshotRequestDescriptor instead')
const ReadSnapshotRequest$json = {
  '1': 'ReadSnapshotRequest',
  '2': [
    {
      '1': 'scope',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.authority.v1.SourceScope',
      '10': 'scope'
    },
  ],
};

/// Descriptor for `ReadSnapshotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readSnapshotRequestDescriptor = $convert.base64Decode(
    'ChNSZWFkU25hcHNob3RSZXF1ZXN0EjUKBXNjb3BlGAEgASgLMh8udm9pY2UuYXV0aG9yaXR5Ln'
    'YxLlNvdXJjZVNjb3BlUgVzY29wZQ==');

@$core.Deprecated('Use readSnapshotResponseDescriptor instead')
const ReadSnapshotResponse$json = {
  '1': 'ReadSnapshotResponse',
  '2': [
    {
      '1': 'scope',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.authority.v1.SourceScope',
      '10': 'scope'
    },
    {
      '1': 'owner',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.authority.v1.AuthorityOwner',
      '10': 'owner'
    },
    {'1': 'revision', '3': 3, '4': 1, '5': 4, '10': 'revision'},
    {'1': 'complete', '3': 4, '4': 1, '5': 8, '10': 'complete'},
    {'1': 'canonical_state', '3': 5, '4': 1, '5': 12, '10': 'canonicalState'},
    {
      '1': 'valid_until_unix_millis',
      '3': 6,
      '4': 1,
      '5': 3,
      '10': 'validUntilUnixMillis'
    },
  ],
};

/// Descriptor for `ReadSnapshotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readSnapshotResponseDescriptor = $convert.base64Decode(
    'ChRSZWFkU25hcHNob3RSZXNwb25zZRI1CgVzY29wZRgBIAEoCzIfLnZvaWNlLmF1dGhvcml0eS'
    '52MS5Tb3VyY2VTY29wZVIFc2NvcGUSOAoFb3duZXIYAiABKA4yIi52b2ljZS5hdXRob3JpdHku'
    'djEuQXV0aG9yaXR5T3duZXJSBW93bmVyEhoKCHJldmlzaW9uGAMgASgEUghyZXZpc2lvbhIaCg'
    'hjb21wbGV0ZRgEIAEoCFIIY29tcGxldGUSJwoPY2Fub25pY2FsX3N0YXRlGAUgASgMUg5jYW5v'
    'bmljYWxTdGF0ZRI1Chd2YWxpZF91bnRpbF91bml4X21pbGxpcxgGIAEoA1IUdmFsaWRVbnRpbF'
    'VuaXhNaWxsaXM=');

@$core.Deprecated('Use readRevisionRequestDescriptor instead')
const ReadRevisionRequest$json = {
  '1': 'ReadRevisionRequest',
  '2': [
    {
      '1': 'scope',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.authority.v1.SourceScope',
      '10': 'scope'
    },
  ],
};

/// Descriptor for `ReadRevisionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readRevisionRequestDescriptor = $convert.base64Decode(
    'ChNSZWFkUmV2aXNpb25SZXF1ZXN0EjUKBXNjb3BlGAEgASgLMh8udm9pY2UuYXV0aG9yaXR5Ln'
    'YxLlNvdXJjZVNjb3BlUgVzY29wZQ==');

@$core.Deprecated('Use readRevisionResponseDescriptor instead')
const ReadRevisionResponse$json = {
  '1': 'ReadRevisionResponse',
  '2': [
    {
      '1': 'scope',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.authority.v1.SourceScope',
      '10': 'scope'
    },
    {
      '1': 'owner',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.authority.v1.AuthorityOwner',
      '10': 'owner'
    },
    {'1': 'revision', '3': 3, '4': 1, '5': 4, '10': 'revision'},
  ],
};

/// Descriptor for `ReadRevisionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readRevisionResponseDescriptor = $convert.base64Decode(
    'ChRSZWFkUmV2aXNpb25SZXNwb25zZRI1CgVzY29wZRgBIAEoCzIfLnZvaWNlLmF1dGhvcml0eS'
    '52MS5Tb3VyY2VTY29wZVIFc2NvcGUSOAoFb3duZXIYAiABKA4yIi52b2ljZS5hdXRob3JpdHku'
    'djEuQXV0aG9yaXR5T3duZXJSBW93bmVyEhoKCHJldmlzaW9uGAMgASgEUghyZXZpc2lvbg==');
