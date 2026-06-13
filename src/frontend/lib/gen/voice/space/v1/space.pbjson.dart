// This is a generated file - do not edit.
//
// Generated from voice/space/v1/space.proto.

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

@$core.Deprecated('Use spaceDescriptor instead')
const Space$json = {
  '1': 'Space',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'icon_url',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'iconUrl',
      '17': true
    },
    {
      '1': 'banner_url',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'bannerUrl',
      '17': true
    },
    {'1': 'visibility', '3': 6, '4': 1, '5': 9, '10': 'visibility'},
    {'1': 'owner_profile_id', '3': 7, '4': 1, '5': 9, '10': 'ownerProfileId'},
    {'1': 'member_count', '3': 8, '4': 1, '5': 5, '10': 'memberCount'},
    {'1': 'is_verified', '3': 9, '4': 1, '5': 8, '10': 'isVerified'},
    {
      '1': 'verification_type',
      '3': 10,
      '4': 1,
      '5': 9,
      '10': 'verificationType'
    },
    {
      '1': 'entry_requirement',
      '3': 11,
      '4': 1,
      '5': 9,
      '10': 'entryRequirement'
    },
    {
      '1': 'entry_questions_json',
      '3': 12,
      '4': 1,
      '5': 9,
      '10': 'entryQuestionsJson'
    },
    {'1': 'mm_config_json', '3': 13, '4': 1, '5': 9, '10': 'mmConfigJson'},
    {
      '1': 'created_at',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'updated_at',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
  ],
  '8': [
    {'1': '_icon_url'},
    {'1': '_banner_url'},
  ],
};

/// Descriptor for `Space`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceDescriptor = $convert.base64Decode(
    'CgVTcGFjZRIOCgJpZBgBIAEoCVICaWQSEgoEbmFtZRgCIAEoCVIEbmFtZRIgCgtkZXNjcmlwdG'
    'lvbhgDIAEoCVILZGVzY3JpcHRpb24SHgoIaWNvbl91cmwYBCABKAlIAFIHaWNvblVybIgBARIi'
    'CgpiYW5uZXJfdXJsGAUgASgJSAFSCWJhbm5lclVybIgBARIeCgp2aXNpYmlsaXR5GAYgASgJUg'
    'p2aXNpYmlsaXR5EigKEG93bmVyX3Byb2ZpbGVfaWQYByABKAlSDm93bmVyUHJvZmlsZUlkEiEK'
    'DG1lbWJlcl9jb3VudBgIIAEoBVILbWVtYmVyQ291bnQSHwoLaXNfdmVyaWZpZWQYCSABKAhSCm'
    'lzVmVyaWZpZWQSKwoRdmVyaWZpY2F0aW9uX3R5cGUYCiABKAlSEHZlcmlmaWNhdGlvblR5cGUS'
    'KwoRZW50cnlfcmVxdWlyZW1lbnQYCyABKAlSEGVudHJ5UmVxdWlyZW1lbnQSMAoUZW50cnlfcX'
    'Vlc3Rpb25zX2pzb24YDCABKAlSEmVudHJ5UXVlc3Rpb25zSnNvbhIkCg5tbV9jb25maWdfanNv'
    'bhgNIAEoCVIMbW1Db25maWdKc29uEjkKCmNyZWF0ZWRfYXQYDiABKAsyGi5nb29nbGUucHJvdG'
    '9idWYuVGltZXN0YW1wUgljcmVhdGVkQXQSOQoKdXBkYXRlZF9hdBgPIAEoCzIaLmdvb2dsZS5w'
    'cm90b2J1Zi5UaW1lc3RhbXBSCXVwZGF0ZWRBdEILCglfaWNvbl91cmxCDQoLX2Jhbm5lcl91cm'
    'w=');

@$core.Deprecated('Use createSpaceRequestDescriptor instead')
const CreateSpaceRequest$json = {
  '1': 'CreateSpaceRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'visibility', '3': 3, '4': 1, '5': 9, '10': 'visibility'},
  ],
};

/// Descriptor for `CreateSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createSpaceRequestDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVTcGFjZVJlcXVlc3QSEgoEbmFtZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbh'
    'gCIAEoCVILZGVzY3JpcHRpb24SHgoKdmlzaWJpbGl0eRgDIAEoCVIKdmlzaWJpbGl0eQ==');

@$core.Deprecated('Use updateSpaceRequestDescriptor instead')
const UpdateSpaceRequest$json = {
  '1': 'UpdateSpaceRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'description',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'description',
      '17': true
    },
    {
      '1': 'icon_url',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'iconUrl',
      '17': true
    },
    {
      '1': 'banner_url',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'bannerUrl',
      '17': true
    },
    {
      '1': 'visibility',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'visibility',
      '17': true
    },
    {
      '1': 'entry_requirement',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 5,
      '10': 'entryRequirement',
      '17': true
    },
    {
      '1': 'entry_questions_json',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 6,
      '10': 'entryQuestionsJson',
      '17': true
    },
    {
      '1': 'mm_config_json',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 7,
      '10': 'mmConfigJson',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_description'},
    {'1': '_icon_url'},
    {'1': '_banner_url'},
    {'1': '_visibility'},
    {'1': '_entry_requirement'},
    {'1': '_entry_questions_json'},
    {'1': '_mm_config_json'},
  ],
};

/// Descriptor for `UpdateSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateSpaceRequestDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVTcGFjZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSFwoEbmFtZR'
    'gCIAEoCUgAUgRuYW1liAEBEiUKC2Rlc2NyaXB0aW9uGAMgASgJSAFSC2Rlc2NyaXB0aW9uiAEB'
    'Eh4KCGljb25fdXJsGAQgASgJSAJSB2ljb25VcmyIAQESIgoKYmFubmVyX3VybBgFIAEoCUgDUg'
    'liYW5uZXJVcmyIAQESIwoKdmlzaWJpbGl0eRgGIAEoCUgEUgp2aXNpYmlsaXR5iAEBEjAKEWVu'
    'dHJ5X3JlcXVpcmVtZW50GAcgASgJSAVSEGVudHJ5UmVxdWlyZW1lbnSIAQESNQoUZW50cnlfcX'
    'Vlc3Rpb25zX2pzb24YCCABKAlIBlISZW50cnlRdWVzdGlvbnNKc29uiAEBEikKDm1tX2NvbmZp'
    'Z19qc29uGAkgASgJSAdSDG1tQ29uZmlnSnNvbogBAUIHCgVfbmFtZUIOCgxfZGVzY3JpcHRpb2'
    '5CCwoJX2ljb25fdXJsQg0KC19iYW5uZXJfdXJsQg0KC192aXNpYmlsaXR5QhQKEl9lbnRyeV9y'
    'ZXF1aXJlbWVudEIXChVfZW50cnlfcXVlc3Rpb25zX2pzb25CEQoPX21tX2NvbmZpZ19qc29u');

@$core.Deprecated('Use deleteSpaceRequestDescriptor instead')
const DeleteSpaceRequest$json = {
  '1': 'DeleteSpaceRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `DeleteSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteSpaceRequestDescriptor =
    $convert.base64Decode(
        'ChJEZWxldGVTcGFjZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use getSpaceRequestDescriptor instead')
const GetSpaceRequest$json = {
  '1': 'GetSpaceRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `GetSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceRequestDescriptor = $convert.base64Decode(
    'Cg9HZXRTcGFjZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use listMySpacesRequestDescriptor instead')
const ListMySpacesRequest$json = {
  '1': 'ListMySpacesRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListMySpacesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMySpacesRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0TXlTcGFjZXNSZXF1ZXN0EjYKBHBhZ2UYASABKAsyIi52b2ljZS5jb21tb24udjEuQ3'
    'Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use spaceListDescriptor instead')
const SpaceList$json = {
  '1': 'SpaceList',
  '2': [
    {
      '1': 'spaces',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.Space',
      '10': 'spaces'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `SpaceList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceListDescriptor = $convert.base64Decode(
    'CglTcGFjZUxpc3QSLQoGc3BhY2VzGAEgAygLMhUudm9pY2Uuc3BhY2UudjEuU3BhY2VSBnNwYW'
    'NlcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use searchPublicSpacesRequestDescriptor instead')
const SearchPublicSpacesRequest$json = {
  '1': 'SearchPublicSpacesRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `SearchPublicSpacesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchPublicSpacesRequestDescriptor =
    $convert.base64Decode(
        'ChlTZWFyY2hQdWJsaWNTcGFjZXNSZXF1ZXN0EhQKBXF1ZXJ5GAEgASgJUgVxdWVyeRI2CgRwYW'
        'dlGAIgASgLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYWdl');

@$core.Deprecated('Use voiceRoomDescriptor instead')
const VoiceRoom$json = {
  '1': 'VoiceRoom',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {
      '1': 'created_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
};

/// Descriptor for `VoiceRoom`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceRoomDescriptor = $convert.base64Decode(
    'CglWb2ljZVJvb20SDgoCaWQYASABKAlSAmlkEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEh'
    'IKBG5hbWUYAyABKAlSBG5hbWUSOQoKY3JlYXRlZF9hdBgEIAEoCzIaLmdvb2dsZS5wcm90b2J1'
    'Zi5UaW1lc3RhbXBSCWNyZWF0ZWRBdA==');

@$core.Deprecated('Use createVoiceRoomRequestDescriptor instead')
const CreateVoiceRoomRequest$json = {
  '1': 'CreateVoiceRoomRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `CreateVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createVoiceRoomRequestDescriptor =
    $convert.base64Decode(
        'ChZDcmVhdGVWb2ljZVJvb21SZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEhIKBG'
        '5hbWUYAiABKAlSBG5hbWU=');

@$core.Deprecated('Use updateVoiceRoomRequestDescriptor instead')
const UpdateVoiceRoomRequest$json = {
  '1': 'UpdateVoiceRoomRequest',
  '2': [
    {'1': 'voice_room_id', '3': 1, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
  ],
  '8': [
    {'1': '_name'},
  ],
};

/// Descriptor for `UpdateVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateVoiceRoomRequestDescriptor =
    $convert.base64Decode(
        'ChZVcGRhdGVWb2ljZVJvb21SZXF1ZXN0EiIKDXZvaWNlX3Jvb21faWQYASABKAlSC3ZvaWNlUm'
        '9vbUlkEhcKBG5hbWUYAiABKAlIAFIEbmFtZYgBAUIHCgVfbmFtZQ==');

@$core.Deprecated('Use deleteVoiceRoomRequestDescriptor instead')
const DeleteVoiceRoomRequest$json = {
  '1': 'DeleteVoiceRoomRequest',
  '2': [
    {'1': 'voice_room_id', '3': 1, '4': 1, '5': 9, '10': 'voiceRoomId'},
  ],
};

/// Descriptor for `DeleteVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteVoiceRoomRequestDescriptor =
    $convert.base64Decode(
        'ChZEZWxldGVWb2ljZVJvb21SZXF1ZXN0EiIKDXZvaWNlX3Jvb21faWQYASABKAlSC3ZvaWNlUm'
        '9vbUlk');

@$core.Deprecated('Use spaceTreeNodeDescriptor instead')
const SpaceTreeNode$json = {
  '1': 'SpaceTreeNode',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'category_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'categoryId',
      '17': true
    },
    {'1': 'kind', '3': 4, '4': 1, '5': 9, '10': 'kind'},
    {
      '1': 'linked_chat',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 1,
      '10': 'linkedChat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'voiceRoomId',
      '17': true
    },
    {'1': 'sort_order', '3': 7, '4': 1, '5': 5, '10': 'sortOrder'},
    {'1': 'is_system', '3': 8, '4': 1, '5': 8, '10': 'isSystem'},
    {
      '1': 'display_name',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'displayName',
      '17': true
    },
  ],
  '8': [
    {'1': '_category_id'},
    {'1': '_linked_chat'},
    {'1': '_voice_room_id'},
    {'1': '_display_name'},
  ],
};

/// Descriptor for `SpaceTreeNode`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceTreeNodeDescriptor = $convert.base64Decode(
    'Cg1TcGFjZVRyZWVOb2RlEg4KAmlkGAEgASgJUgJpZBIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2'
    'VJZBIkCgtjYXRlZ29yeV9pZBgDIAEoCUgAUgpjYXRlZ29yeUlkiAEBEhIKBGtpbmQYBCABKAlS'
    'BGtpbmQSPAoLbGlua2VkX2NoYXQYBSABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZIAVIKbG'
    'lua2VkQ2hhdIgBARInCg12b2ljZV9yb29tX2lkGAYgASgJSAJSC3ZvaWNlUm9vbUlkiAEBEh0K'
    'CnNvcnRfb3JkZXIYByABKAVSCXNvcnRPcmRlchIbCglpc19zeXN0ZW0YCCABKAhSCGlzU3lzdG'
    'VtEiYKDGRpc3BsYXlfbmFtZRgJIAEoCUgDUgtkaXNwbGF5TmFtZYgBAUIOCgxfY2F0ZWdvcnlf'
    'aWRCDgoMX2xpbmtlZF9jaGF0QhAKDl92b2ljZV9yb29tX2lkQg8KDV9kaXNwbGF5X25hbWU=');

@$core.Deprecated('Use upsertTreeNodeRequestDescriptor instead')
const UpsertTreeNodeRequest$json = {
  '1': 'UpsertTreeNodeRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'node_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'nodeId',
      '17': true
    },
    {
      '1': 'category_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'categoryId',
      '17': true
    },
    {'1': 'kind', '3': 4, '4': 1, '5': 9, '10': 'kind'},
    {
      '1': 'linked_chat',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 2,
      '10': 'linkedChat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'voiceRoomId',
      '17': true
    },
    {
      '1': 'sort_order',
      '3': 7,
      '4': 1,
      '5': 5,
      '9': 4,
      '10': 'sortOrder',
      '17': true
    },
    {
      '1': 'is_system',
      '3': 8,
      '4': 1,
      '5': 8,
      '9': 5,
      '10': 'isSystem',
      '17': true
    },
  ],
  '8': [
    {'1': '_node_id'},
    {'1': '_category_id'},
    {'1': '_linked_chat'},
    {'1': '_voice_room_id'},
    {'1': '_sort_order'},
    {'1': '_is_system'},
  ],
};

/// Descriptor for `UpsertTreeNodeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertTreeNodeRequestDescriptor = $convert.base64Decode(
    'ChVVcHNlcnRUcmVlTm9kZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHAoHbm'
    '9kZV9pZBgCIAEoCUgAUgZub2RlSWSIAQESJAoLY2F0ZWdvcnlfaWQYAyABKAlIAVIKY2F0ZWdv'
    'cnlJZIgBARISCgRraW5kGAQgASgJUgRraW5kEjwKC2xpbmtlZF9jaGF0GAUgASgLMhYudm9pY2'
    'UuY2hhdC52MS5DaGF0UmVmSAJSCmxpbmtlZENoYXSIAQESJwoNdm9pY2Vfcm9vbV9pZBgGIAEo'
    'CUgDUgt2b2ljZVJvb21JZIgBARIiCgpzb3J0X29yZGVyGAcgASgFSARSCXNvcnRPcmRlcogBAR'
    'IgCglpc19zeXN0ZW0YCCABKAhIBVIIaXNTeXN0ZW2IAQFCCgoIX25vZGVfaWRCDgoMX2NhdGVn'
    'b3J5X2lkQg4KDF9saW5rZWRfY2hhdEIQCg5fdm9pY2Vfcm9vbV9pZEINCgtfc29ydF9vcmRlck'
    'IMCgpfaXNfc3lzdGVt');

@$core.Deprecated('Use removeTreeNodeRequestDescriptor instead')
const RemoveTreeNodeRequest$json = {
  '1': 'RemoveTreeNodeRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'node_id', '3': 2, '4': 1, '5': 9, '10': 'nodeId'},
  ],
};

/// Descriptor for `RemoveTreeNodeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeTreeNodeRequestDescriptor = $convert.base64Decode(
    'ChVSZW1vdmVUcmVlTm9kZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSFwoHbm'
    '9kZV9pZBgCIAEoCVIGbm9kZUlk');

@$core.Deprecated('Use categoryDescriptor instead')
const Category$json = {
  '1': 'Category',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'sort_order', '3': 4, '4': 1, '5': 5, '10': 'sortOrder'},
  ],
};

/// Descriptor for `Category`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List categoryDescriptor = $convert.base64Decode(
    'CghDYXRlZ29yeRIOCgJpZBgBIAEoCVICaWQSGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSEg'
    'oEbmFtZRgDIAEoCVIEbmFtZRIdCgpzb3J0X29yZGVyGAQgASgFUglzb3J0T3JkZXI=');

@$core.Deprecated('Use createCategoryRequestDescriptor instead')
const CreateCategoryRequest$json = {
  '1': 'CreateCategoryRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'sort_order', '3': 3, '4': 1, '5': 5, '10': 'sortOrder'},
  ],
};

/// Descriptor for `CreateCategoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCategoryRequestDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVDYXRlZ29yeVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSEgoEbm'
    'FtZRgCIAEoCVIEbmFtZRIdCgpzb3J0X29yZGVyGAMgASgFUglzb3J0T3JkZXI=');

@$core.Deprecated('Use updateCategoryRequestDescriptor instead')
const UpdateCategoryRequest$json = {
  '1': 'UpdateCategoryRequest',
  '2': [
    {'1': 'category_id', '3': 1, '4': 1, '5': 9, '10': 'categoryId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'sort_order',
      '3': 3,
      '4': 1,
      '5': 5,
      '9': 1,
      '10': 'sortOrder',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_sort_order'},
  ],
};

/// Descriptor for `UpdateCategoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCategoryRequestDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVDYXRlZ29yeVJlcXVlc3QSHwoLY2F0ZWdvcnlfaWQYASABKAlSCmNhdGVnb3J5SW'
    'QSFwoEbmFtZRgCIAEoCUgAUgRuYW1liAEBEiIKCnNvcnRfb3JkZXIYAyABKAVIAVIJc29ydE9y'
    'ZGVyiAEBQgcKBV9uYW1lQg0KC19zb3J0X29yZGVy');

@$core.Deprecated('Use deleteCategoryRequestDescriptor instead')
const DeleteCategoryRequest$json = {
  '1': 'DeleteCategoryRequest',
  '2': [
    {'1': 'category_id', '3': 1, '4': 1, '5': 9, '10': 'categoryId'},
  ],
};

/// Descriptor for `DeleteCategoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCategoryRequestDescriptor = $convert.base64Decode(
    'ChVEZWxldGVDYXRlZ29yeVJlcXVlc3QSHwoLY2F0ZWdvcnlfaWQYASABKAlSCmNhdGVnb3J5SW'
    'Q=');

@$core.Deprecated('Use reorderSpaceTreeRequestDescriptor instead')
const ReorderSpaceTreeRequest$json = {
  '1': 'ReorderSpaceTreeRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'ordered_node_ids', '3': 2, '4': 3, '5': 9, '10': 'orderedNodeIds'},
  ],
};

/// Descriptor for `ReorderSpaceTreeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderSpaceTreeRequestDescriptor =
    $convert.base64Decode(
        'ChdSZW9yZGVyU3BhY2VUcmVlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIoCh'
        'BvcmRlcmVkX25vZGVfaWRzGAIgAygJUg5vcmRlcmVkTm9kZUlkcw==');

@$core.Deprecated('Use listSpaceTreeRequestDescriptor instead')
const ListSpaceTreeRequest$json = {
  '1': 'ListSpaceTreeRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListSpaceTreeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpaceTreeRequestDescriptor =
    $convert.base64Decode(
        'ChRMaXN0U3BhY2VUcmVlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZA==');

@$core.Deprecated('Use inviteDescriptor instead')
const Invite$json = {
  '1': 'Invite',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'code', '3': 3, '4': 1, '5': 9, '10': 'code'},
    {
      '1': 'creator_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'creatorProfileId'
    },
    {
      '1': 'max_uses',
      '3': 5,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'maxUses',
      '17': true
    },
    {'1': 'use_count', '3': 6, '4': 1, '5': 5, '10': 'useCount'},
    {
      '1': 'expires_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'expiresAt',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'revoked_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 2,
      '10': 'revokedAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_max_uses'},
    {'1': '_expires_at'},
    {'1': '_revoked_at'},
  ],
};

/// Descriptor for `Invite`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List inviteDescriptor = $convert.base64Decode(
    'CgZJbnZpdGUSDgoCaWQYASABKAlSAmlkEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEhIKBG'
    'NvZGUYAyABKAlSBGNvZGUSLAoSY3JlYXRvcl9wcm9maWxlX2lkGAQgASgJUhBjcmVhdG9yUHJv'
    'ZmlsZUlkEh4KCG1heF91c2VzGAUgASgFSABSB21heFVzZXOIAQESGwoJdXNlX2NvdW50GAYgAS'
    'gFUgh1c2VDb3VudBI+CgpleHBpcmVzX2F0GAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVz'
    'dGFtcEgBUglleHBpcmVzQXSIAQESOQoKY3JlYXRlZF9hdBgIIAEoCzIaLmdvb2dsZS5wcm90b2'
    'J1Zi5UaW1lc3RhbXBSCWNyZWF0ZWRBdBI+CgpyZXZva2VkX2F0GAkgASgLMhouZ29vZ2xlLnBy'
    'b3RvYnVmLlRpbWVzdGFtcEgCUglyZXZva2VkQXSIAQFCCwoJX21heF91c2VzQg0KC19leHBpcm'
    'VzX2F0Qg0KC19yZXZva2VkX2F0');

@$core.Deprecated('Use createInviteRequestDescriptor instead')
const CreateInviteRequest$json = {
  '1': 'CreateInviteRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'max_uses',
      '3': 2,
      '4': 1,
      '5': 5,
      '9': 0,
      '10': 'maxUses',
      '17': true
    },
    {
      '1': 'expires_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'expiresAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_max_uses'},
    {'1': '_expires_at'},
  ],
};

/// Descriptor for `CreateInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createInviteRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVJbnZpdGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh4KCG1heF'
    '91c2VzGAIgASgFSABSB21heFVzZXOIAQESPgoKZXhwaXJlc19hdBgDIAEoCzIaLmdvb2dsZS5w'
    'cm90b2J1Zi5UaW1lc3RhbXBIAVIJZXhwaXJlc0F0iAEBQgsKCV9tYXhfdXNlc0INCgtfZXhwaX'
    'Jlc19hdA==');

@$core.Deprecated('Use revokeInviteRequestDescriptor instead')
const RevokeInviteRequest$json = {
  '1': 'RevokeInviteRequest',
  '2': [
    {'1': 'invite_id', '3': 1, '4': 1, '5': 9, '10': 'inviteId'},
  ],
};

/// Descriptor for `RevokeInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeInviteRequestDescriptor =
    $convert.base64Decode(
        'ChNSZXZva2VJbnZpdGVSZXF1ZXN0EhsKCWludml0ZV9pZBgBIAEoCVIIaW52aXRlSWQ=');

@$core.Deprecated('Use getInviteRequestDescriptor instead')
const GetInviteRequest$json = {
  '1': 'GetInviteRequest',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
  ],
};

/// Descriptor for `GetInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getInviteRequestDescriptor = $convert
    .base64Decode('ChBHZXRJbnZpdGVSZXF1ZXN0EhIKBGNvZGUYASABKAlSBGNvZGU=');

@$core.Deprecated('Use listInvitesRequestDescriptor instead')
const ListInvitesRequest$json = {
  '1': 'ListInvitesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListInvitesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInvitesRequestDescriptor =
    $convert.base64Decode(
        'ChJMaXN0SW52aXRlc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use inviteListDescriptor instead')
const InviteList$json = {
  '1': 'InviteList',
  '2': [
    {
      '1': 'invites',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.Invite',
      '10': 'invites'
    },
  ],
};

/// Descriptor for `InviteList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List inviteListDescriptor = $convert.base64Decode(
    'CgpJbnZpdGVMaXN0EjAKB2ludml0ZXMYASADKAsyFi52b2ljZS5zcGFjZS52MS5JbnZpdGVSB2'
    'ludml0ZXM=');

@$core.Deprecated('Use joinByInviteRequestDescriptor instead')
const JoinByInviteRequest$json = {
  '1': 'JoinByInviteRequest',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
  ],
};

/// Descriptor for `JoinByInviteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinByInviteRequestDescriptor = $convert
    .base64Decode('ChNKb2luQnlJbnZpdGVSZXF1ZXN0EhIKBGNvZGUYASABKAlSBGNvZGU=');

@$core.Deprecated('Use spaceMembershipDescriptor instead')
const SpaceMembership$json = {
  '1': 'SpaceMembership',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'joined_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'joinedAt'
    },
    {
      '1': 'nickname',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'nickname',
      '17': true
    },
    {'1': 'role_names', '3': 5, '4': 3, '5': 9, '10': 'roleNames'},
  ],
  '8': [
    {'1': '_nickname'},
  ],
};

/// Descriptor for `SpaceMembership`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceMembershipDescriptor = $convert.base64Decode(
    'Cg9TcGFjZU1lbWJlcnNoaXASGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHQoKcHJvZmlsZV'
    '9pZBgCIAEoCVIJcHJvZmlsZUlkEjcKCWpvaW5lZF9hdBgDIAEoCzIaLmdvb2dsZS5wcm90b2J1'
    'Zi5UaW1lc3RhbXBSCGpvaW5lZEF0Eh8KCG5pY2tuYW1lGAQgASgJSABSCG5pY2tuYW1liAEBEh'
    '0KCnJvbGVfbmFtZXMYBSADKAlSCXJvbGVOYW1lc0ILCglfbmlja25hbWU=');

@$core.Deprecated('Use joinSpaceRequestDescriptor instead')
const JoinSpaceRequest$json = {
  '1': 'JoinSpaceRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `JoinSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinSpaceRequestDescriptor = $convert.base64Decode(
    'ChBKb2luU3BhY2VSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlk');

@$core.Deprecated('Use leaveSpaceRequestDescriptor instead')
const LeaveSpaceRequest$json = {
  '1': 'LeaveSpaceRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `LeaveSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveSpaceRequestDescriptor = $convert.base64Decode(
    'ChFMZWF2ZVNwYWNlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZA==');

@$core.Deprecated('Use kickMemberRequestDescriptor instead')
const KickMemberRequest$json = {
  '1': 'KickMemberRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `KickMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kickMemberRequestDescriptor = $convert.base64Decode(
    'ChFLaWNrTWVtYmVyUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use banMemberRequestDescriptor instead')
const BanMemberRequest$json = {
  '1': 'BanMemberRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'reason', '17': true},
    {
      '1': 'profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'profileId',
      '17': true
    },
  ],
  '8': [
    {'1': '_reason'},
    {'1': '_profile_id'},
  ],
};

/// Descriptor for `BanMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banMemberRequestDescriptor = $convert.base64Decode(
    'ChBCYW5NZW1iZXJSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh0KCmFjY291bn'
    'RfaWQYAiABKAlSCWFjY291bnRJZBIbCgZyZWFzb24YAyABKAlIAFIGcmVhc29uiAEBEiIKCnBy'
    'b2ZpbGVfaWQYBCABKAlIAVIJcHJvZmlsZUlkiAEBQgkKB19yZWFzb25CDQoLX3Byb2ZpbGVfaW'
    'Q=');

@$core.Deprecated('Use unbanMemberRequestDescriptor instead')
const UnbanMemberRequest$json = {
  '1': 'UnbanMemberRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `UnbanMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unbanMemberRequestDescriptor = $convert.base64Decode(
    'ChJVbmJhbk1lbWJlclJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHQoKYWNjb3'
    'VudF9pZBgCIAEoCVIJYWNjb3VudElk');

@$core.Deprecated('Use listMembersRequestDescriptor instead')
const ListMembersRequest$json = {
  '1': 'ListMembersRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListMembersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0TWVtYmVyc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSNgoEcGFnZR'
    'gCIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5DdXJzb3JQYWdlUmVxdWVzdFIEcGFnZQ==');

@$core.Deprecated('Use spaceMemberListDescriptor instead')
const SpaceMemberList$json = {
  '1': 'SpaceMemberList',
  '2': [
    {
      '1': 'members',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.SpaceMembership',
      '10': 'members'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `SpaceMemberList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceMemberListDescriptor = $convert.base64Decode(
    'Cg9TcGFjZU1lbWJlckxpc3QSOQoHbWVtYmVycxgBIAMoCzIfLnZvaWNlLnNwYWNlLnYxLlNwYW'
    'NlTWVtYmVyc2hpcFIHbWVtYmVycxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use listBansRequestDescriptor instead')
const ListBansRequest$json = {
  '1': 'ListBansRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListBansRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBansRequestDescriptor = $convert.base64Decode(
    'Cg9MaXN0QmFuc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use banListDescriptor instead')
const BanList$json = {
  '1': 'BanList',
  '2': [
    {
      '1': 'bans',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.SpaceBan',
      '10': 'bans'
    },
  ],
};

/// Descriptor for `BanList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banListDescriptor = $convert.base64Decode(
    'CgdCYW5MaXN0EiwKBGJhbnMYASADKAsyGC52b2ljZS5zcGFjZS52MS5TcGFjZUJhblIEYmFucw'
    '==');

@$core.Deprecated('Use spaceBanDescriptor instead')
const SpaceBan$json = {
  '1': 'SpaceBan',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'banned_by_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'bannedByProfileId'
    },
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '9': 0, '10': 'reason', '17': true},
    {
      '1': 'banned_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'bannedAt'
    },
  ],
  '8': [
    {'1': '_reason'},
  ],
};

/// Descriptor for `SpaceBan`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceBanDescriptor = $convert.base64Decode(
    'CghTcGFjZUJhbhIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgphY2NvdW50X2lkGAIgAS'
    'gJUglhY2NvdW50SWQSLwoUYmFubmVkX2J5X3Byb2ZpbGVfaWQYAyABKAlSEWJhbm5lZEJ5UHJv'
    'ZmlsZUlkEhsKBnJlYXNvbhgEIAEoCUgAUgZyZWFzb26IAQESNwoJYmFubmVkX2F0GAUgASgLMh'
    'ouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIIYmFubmVkQXRCCQoHX3JlYXNvbg==');

@$core.Deprecated('Use timeoutMemberRequestDescriptor instead')
const TimeoutMemberRequest$json = {
  '1': 'TimeoutMemberRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'duration_seconds', '3': 3, '4': 1, '5': 5, '10': 'durationSeconds'},
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '9': 0, '10': 'reason', '17': true},
  ],
  '8': [
    {'1': '_reason'},
  ],
};

/// Descriptor for `TimeoutMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List timeoutMemberRequestDescriptor = $convert.base64Decode(
    'ChRUaW1lb3V0TWVtYmVyUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm'
    '9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSKQoQZHVyYXRpb25fc2Vjb25kcxgDIAEoBVIPZHVy'
    'YXRpb25TZWNvbmRzEhsKBnJlYXNvbhgEIAEoCUgAUgZyZWFzb26IAQFCCQoHX3JlYXNvbg==');

@$core.Deprecated('Use removeMemberTimeoutRequestDescriptor instead')
const RemoveMemberTimeoutRequest$json = {
  '1': 'RemoveMemberTimeoutRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `RemoveMemberTimeoutRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberTimeoutRequestDescriptor =
    $convert.base64Decode(
        'ChpSZW1vdmVNZW1iZXJUaW1lb3V0UmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZB'
        'IdCgpwcm9maWxlX2lkGAIgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use transferOwnershipRequestDescriptor instead')
const TransferOwnershipRequest$json = {
  '1': 'TransferOwnershipRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
  ],
};

/// Descriptor for `TransferOwnershipRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transferOwnershipRequestDescriptor =
    $convert.base64Decode(
        'ChhUcmFuc2Zlck93bmVyc2hpcFJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSLw'
        'oUbmV3X293bmVyX3Byb2ZpbGVfaWQYAiABKAlSEW5ld093bmVyUHJvZmlsZUlk');

@$core.Deprecated('Use listTemplatesRequestDescriptor instead')
const ListTemplatesRequest$json = {
  '1': 'ListTemplatesRequest',
};

/// Descriptor for `ListTemplatesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTemplatesRequestDescriptor =
    $convert.base64Decode('ChRMaXN0VGVtcGxhdGVzUmVxdWVzdA==');

@$core.Deprecated('Use templateListDescriptor instead')
const TemplateList$json = {
  '1': 'TemplateList',
  '2': [
    {
      '1': 'templates',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.SpaceTemplate',
      '10': 'templates'
    },
  ],
};

/// Descriptor for `TemplateList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List templateListDescriptor = $convert.base64Decode(
    'CgxUZW1wbGF0ZUxpc3QSOwoJdGVtcGxhdGVzGAEgAygLMh0udm9pY2Uuc3BhY2UudjEuU3BhY2'
    'VUZW1wbGF0ZVIJdGVtcGxhdGVz');

@$core.Deprecated('Use spaceTemplateDescriptor instead')
const SpaceTemplate$json = {
  '1': 'SpaceTemplate',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 3, '4': 1, '5': 9, '10': 'description'},
  ],
};

/// Descriptor for `SpaceTemplate`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceTemplateDescriptor = $convert.base64Decode(
    'Cg1TcGFjZVRlbXBsYXRlEg4KAmlkGAEgASgJUgJpZBISCgRuYW1lGAIgASgJUgRuYW1lEiAKC2'
    'Rlc2NyaXB0aW9uGAMgASgJUgtkZXNjcmlwdGlvbg==');

@$core.Deprecated('Use createFromTemplateRequestDescriptor instead')
const CreateFromTemplateRequest$json = {
  '1': 'CreateFromTemplateRequest',
  '2': [
    {'1': 'template_id', '3': 1, '4': 1, '5': 9, '10': 'templateId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
  ],
};

/// Descriptor for `CreateFromTemplateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createFromTemplateRequestDescriptor =
    $convert.base64Decode(
        'ChlDcmVhdGVGcm9tVGVtcGxhdGVSZXF1ZXN0Eh8KC3RlbXBsYXRlX2lkGAEgASgJUgp0ZW1wbG'
        'F0ZUlkEhIKBG5hbWUYAiABKAlSBG5hbWU=');

@$core.Deprecated('Use getAuditLogRequestDescriptor instead')
const GetAuditLogRequest$json = {
  '1': 'GetAuditLogRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `GetAuditLogRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAuditLogRequestDescriptor = $convert.base64Decode(
    'ChJHZXRBdWRpdExvZ1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSNgoEcGFnZR'
    'gCIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5DdXJzb3JQYWdlUmVxdWVzdFIEcGFnZQ==');

@$core.Deprecated('Use auditLogListDescriptor instead')
const AuditLogList$json = {
  '1': 'AuditLogList',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.AuditLogEntry',
      '10': 'entries'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `AuditLogList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List auditLogListDescriptor = $convert.base64Decode(
    'CgxBdWRpdExvZ0xpc3QSNwoHZW50cmllcxgBIAMoCzIdLnZvaWNlLnNwYWNlLnYxLkF1ZGl0TG'
    '9nRW50cnlSB2VudHJpZXMSHwoLbmV4dF9jdXJzb3IYAiABKAlSCm5leHRDdXJzb3I=');

@$core.Deprecated('Use auditLogEntryDescriptor instead')
const AuditLogEntry$json = {
  '1': 'AuditLogEntry',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'action', '3': 4, '4': 1, '5': 9, '10': 'action'},
    {'1': 'target_type', '3': 5, '4': 1, '5': 9, '10': 'targetType'},
    {'1': 'target_id', '3': 6, '4': 1, '5': 9, '10': 'targetId'},
    {'1': 'details_json', '3': 7, '4': 1, '5': 9, '10': 'detailsJson'},
    {
      '1': 'created_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
};

/// Descriptor for `AuditLogEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List auditLogEntryDescriptor = $convert.base64Decode(
    'Cg1BdWRpdExvZ0VudHJ5Eg4KAmlkGAEgASgJUgJpZBIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2'
    'VJZBIoChBhY3Rvcl9wcm9maWxlX2lkGAMgASgJUg5hY3RvclByb2ZpbGVJZBIWCgZhY3Rpb24Y'
    'BCABKAlSBmFjdGlvbhIfCgt0YXJnZXRfdHlwZRgFIAEoCVIKdGFyZ2V0VHlwZRIbCgl0YXJnZX'
    'RfaWQYBiABKAlSCHRhcmdldElkEiEKDGRldGFpbHNfanNvbhgHIAEoCVILZGV0YWlsc0pzb24S'
    'OQoKY3JlYXRlZF9hdBgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWNyZWF0ZW'
    'RBdA==');

@$core.Deprecated('Use spaceRefDescriptor instead')
const SpaceRef$json = {
  '1': 'SpaceRef',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
  ],
};

/// Descriptor for `SpaceRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceRefDescriptor =
    $convert.base64Decode('CghTcGFjZVJlZhIOCgJpZBgBIAEoCVICaWQ=');

@$core.Deprecated('Use createSpaceResponseDescriptor instead')
const CreateSpaceResponse$json = {
  '1': 'CreateSpaceResponse',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Space',
      '10': 'space'
    },
  ],
};

/// Descriptor for `CreateSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createSpaceResponseDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVTcGFjZVJlc3BvbnNlEisKBXNwYWNlGAEgASgLMhUudm9pY2Uuc3BhY2UudjEuU3'
    'BhY2VSBXNwYWNl');

@$core.Deprecated('Use updateSpaceResponseDescriptor instead')
const UpdateSpaceResponse$json = {
  '1': 'UpdateSpaceResponse',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Space',
      '10': 'space'
    },
  ],
};

/// Descriptor for `UpdateSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateSpaceResponseDescriptor = $convert.base64Decode(
    'ChNVcGRhdGVTcGFjZVJlc3BvbnNlEisKBXNwYWNlGAEgASgLMhUudm9pY2Uuc3BhY2UudjEuU3'
    'BhY2VSBXNwYWNl');

@$core.Deprecated('Use deleteSpaceResponseDescriptor instead')
const DeleteSpaceResponse$json = {
  '1': 'DeleteSpaceResponse',
};

/// Descriptor for `DeleteSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteSpaceResponseDescriptor =
    $convert.base64Decode('ChNEZWxldGVTcGFjZVJlc3BvbnNl');

@$core.Deprecated('Use getSpaceResponseDescriptor instead')
const GetSpaceResponse$json = {
  '1': 'GetSpaceResponse',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Space',
      '10': 'space'
    },
  ],
};

/// Descriptor for `GetSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceResponseDescriptor = $convert.base64Decode(
    'ChBHZXRTcGFjZVJlc3BvbnNlEisKBXNwYWNlGAEgASgLMhUudm9pY2Uuc3BhY2UudjEuU3BhY2'
    'VSBXNwYWNl');

@$core.Deprecated('Use listMySpacesResponseDescriptor instead')
const ListMySpacesResponse$json = {
  '1': 'ListMySpacesResponse',
  '2': [
    {
      '1': 'space_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceList',
      '10': 'spaceList'
    },
  ],
};

/// Descriptor for `ListMySpacesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMySpacesResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0TXlTcGFjZXNSZXNwb25zZRI4CgpzcGFjZV9saXN0GAEgASgLMhkudm9pY2Uuc3BhY2'
    'UudjEuU3BhY2VMaXN0UglzcGFjZUxpc3Q=');

@$core.Deprecated('Use searchPublicSpacesResponseDescriptor instead')
const SearchPublicSpacesResponse$json = {
  '1': 'SearchPublicSpacesResponse',
  '2': [
    {
      '1': 'space_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceList',
      '10': 'spaceList'
    },
  ],
};

/// Descriptor for `SearchPublicSpacesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchPublicSpacesResponseDescriptor =
    $convert.base64Decode(
        'ChpTZWFyY2hQdWJsaWNTcGFjZXNSZXNwb25zZRI4CgpzcGFjZV9saXN0GAEgASgLMhkudm9pY2'
        'Uuc3BhY2UudjEuU3BhY2VMaXN0UglzcGFjZUxpc3Q=');

@$core.Deprecated('Use createVoiceRoomResponseDescriptor instead')
const CreateVoiceRoomResponse$json = {
  '1': 'CreateVoiceRoomResponse',
  '2': [
    {
      '1': 'voice_room',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.VoiceRoom',
      '10': 'voiceRoom'
    },
  ],
};

/// Descriptor for `CreateVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createVoiceRoomResponseDescriptor =
    $convert.base64Decode(
        'ChdDcmVhdGVWb2ljZVJvb21SZXNwb25zZRI4Cgp2b2ljZV9yb29tGAEgASgLMhkudm9pY2Uuc3'
        'BhY2UudjEuVm9pY2VSb29tUgl2b2ljZVJvb20=');

@$core.Deprecated('Use updateVoiceRoomResponseDescriptor instead')
const UpdateVoiceRoomResponse$json = {
  '1': 'UpdateVoiceRoomResponse',
  '2': [
    {
      '1': 'voice_room',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.VoiceRoom',
      '10': 'voiceRoom'
    },
  ],
};

/// Descriptor for `UpdateVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateVoiceRoomResponseDescriptor =
    $convert.base64Decode(
        'ChdVcGRhdGVWb2ljZVJvb21SZXNwb25zZRI4Cgp2b2ljZV9yb29tGAEgASgLMhkudm9pY2Uuc3'
        'BhY2UudjEuVm9pY2VSb29tUgl2b2ljZVJvb20=');

@$core.Deprecated('Use deleteVoiceRoomResponseDescriptor instead')
const DeleteVoiceRoomResponse$json = {
  '1': 'DeleteVoiceRoomResponse',
};

/// Descriptor for `DeleteVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteVoiceRoomResponseDescriptor =
    $convert.base64Decode('ChdEZWxldGVWb2ljZVJvb21SZXNwb25zZQ==');

@$core.Deprecated('Use upsertTreeNodeResponseDescriptor instead')
const UpsertTreeNodeResponse$json = {
  '1': 'UpsertTreeNodeResponse',
  '2': [
    {
      '1': 'space_tree_node',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceTreeNode',
      '10': 'spaceTreeNode'
    },
  ],
};

/// Descriptor for `UpsertTreeNodeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertTreeNodeResponseDescriptor =
    $convert.base64Decode(
        'ChZVcHNlcnRUcmVlTm9kZVJlc3BvbnNlEkUKD3NwYWNlX3RyZWVfbm9kZRgBIAEoCzIdLnZvaW'
        'NlLnNwYWNlLnYxLlNwYWNlVHJlZU5vZGVSDXNwYWNlVHJlZU5vZGU=');

@$core.Deprecated('Use removeTreeNodeResponseDescriptor instead')
const RemoveTreeNodeResponse$json = {
  '1': 'RemoveTreeNodeResponse',
};

/// Descriptor for `RemoveTreeNodeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeTreeNodeResponseDescriptor =
    $convert.base64Decode('ChZSZW1vdmVUcmVlTm9kZVJlc3BvbnNl');

@$core.Deprecated('Use createCategoryResponseDescriptor instead')
const CreateCategoryResponse$json = {
  '1': 'CreateCategoryResponse',
  '2': [
    {
      '1': 'category',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Category',
      '10': 'category'
    },
  ],
};

/// Descriptor for `CreateCategoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCategoryResponseDescriptor =
    $convert.base64Decode(
        'ChZDcmVhdGVDYXRlZ29yeVJlc3BvbnNlEjQKCGNhdGVnb3J5GAEgASgLMhgudm9pY2Uuc3BhY2'
        'UudjEuQ2F0ZWdvcnlSCGNhdGVnb3J5');

@$core.Deprecated('Use updateCategoryResponseDescriptor instead')
const UpdateCategoryResponse$json = {
  '1': 'UpdateCategoryResponse',
  '2': [
    {
      '1': 'category',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Category',
      '10': 'category'
    },
  ],
};

/// Descriptor for `UpdateCategoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateCategoryResponseDescriptor =
    $convert.base64Decode(
        'ChZVcGRhdGVDYXRlZ29yeVJlc3BvbnNlEjQKCGNhdGVnb3J5GAEgASgLMhgudm9pY2Uuc3BhY2'
        'UudjEuQ2F0ZWdvcnlSCGNhdGVnb3J5');

@$core.Deprecated('Use deleteCategoryResponseDescriptor instead')
const DeleteCategoryResponse$json = {
  '1': 'DeleteCategoryResponse',
};

/// Descriptor for `DeleteCategoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteCategoryResponseDescriptor =
    $convert.base64Decode('ChZEZWxldGVDYXRlZ29yeVJlc3BvbnNl');

@$core.Deprecated('Use reorderSpaceTreeResponseDescriptor instead')
const ReorderSpaceTreeResponse$json = {
  '1': 'ReorderSpaceTreeResponse',
};

/// Descriptor for `ReorderSpaceTreeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderSpaceTreeResponseDescriptor =
    $convert.base64Decode('ChhSZW9yZGVyU3BhY2VUcmVlUmVzcG9uc2U=');

@$core.Deprecated('Use listSpaceTreeResponseDescriptor instead')
const ListSpaceTreeResponse$json = {
  '1': 'ListSpaceTreeResponse',
  '2': [
    {
      '1': 'categories',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.Category',
      '10': 'categories'
    },
    {
      '1': 'nodes',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.SpaceTreeNode',
      '10': 'nodes'
    },
    {
      '1': 'voice_rooms',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.voice.space.v1.VoiceRoom',
      '10': 'voiceRooms'
    },
  ],
};

/// Descriptor for `ListSpaceTreeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpaceTreeResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0U3BhY2VUcmVlUmVzcG9uc2USOAoKY2F0ZWdvcmllcxgBIAMoCzIYLnZvaWNlLnNwYW'
    'NlLnYxLkNhdGVnb3J5UgpjYXRlZ29yaWVzEjMKBW5vZGVzGAIgAygLMh0udm9pY2Uuc3BhY2Uu'
    'djEuU3BhY2VUcmVlTm9kZVIFbm9kZXMSOgoLdm9pY2Vfcm9vbXMYAyADKAsyGS52b2ljZS5zcG'
    'FjZS52MS5Wb2ljZVJvb21SCnZvaWNlUm9vbXM=');

@$core.Deprecated('Use createInviteResponseDescriptor instead')
const CreateInviteResponse$json = {
  '1': 'CreateInviteResponse',
  '2': [
    {
      '1': 'invite',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Invite',
      '10': 'invite'
    },
  ],
};

/// Descriptor for `CreateInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createInviteResponseDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVJbnZpdGVSZXNwb25zZRIuCgZpbnZpdGUYASABKAsyFi52b2ljZS5zcGFjZS52MS'
    '5JbnZpdGVSBmludml0ZQ==');

@$core.Deprecated('Use revokeInviteResponseDescriptor instead')
const RevokeInviteResponse$json = {
  '1': 'RevokeInviteResponse',
};

/// Descriptor for `RevokeInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeInviteResponseDescriptor =
    $convert.base64Decode('ChRSZXZva2VJbnZpdGVSZXNwb25zZQ==');

@$core.Deprecated('Use getInviteResponseDescriptor instead')
const GetInviteResponse$json = {
  '1': 'GetInviteResponse',
  '2': [
    {
      '1': 'invite',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Invite',
      '10': 'invite'
    },
  ],
};

/// Descriptor for `GetInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getInviteResponseDescriptor = $convert.base64Decode(
    'ChFHZXRJbnZpdGVSZXNwb25zZRIuCgZpbnZpdGUYASABKAsyFi52b2ljZS5zcGFjZS52MS5Jbn'
    'ZpdGVSBmludml0ZQ==');

@$core.Deprecated('Use listInvitesResponseDescriptor instead')
const ListInvitesResponse$json = {
  '1': 'ListInvitesResponse',
  '2': [
    {
      '1': 'invite_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.InviteList',
      '10': 'inviteList'
    },
  ],
};

/// Descriptor for `ListInvitesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInvitesResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0SW52aXRlc1Jlc3BvbnNlEjsKC2ludml0ZV9saXN0GAEgASgLMhoudm9pY2Uuc3BhY2'
    'UudjEuSW52aXRlTGlzdFIKaW52aXRlTGlzdA==');

@$core.Deprecated('Use joinByInviteResponseDescriptor instead')
const JoinByInviteResponse$json = {
  '1': 'JoinByInviteResponse',
  '2': [
    {
      '1': 'space_membership',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceMembership',
      '10': 'spaceMembership'
    },
  ],
};

/// Descriptor for `JoinByInviteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinByInviteResponseDescriptor = $convert.base64Decode(
    'ChRKb2luQnlJbnZpdGVSZXNwb25zZRJKChBzcGFjZV9tZW1iZXJzaGlwGAEgASgLMh8udm9pY2'
    'Uuc3BhY2UudjEuU3BhY2VNZW1iZXJzaGlwUg9zcGFjZU1lbWJlcnNoaXA=');

@$core.Deprecated('Use joinSpaceResponseDescriptor instead')
const JoinSpaceResponse$json = {
  '1': 'JoinSpaceResponse',
  '2': [
    {
      '1': 'space_membership',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceMembership',
      '10': 'spaceMembership'
    },
  ],
};

/// Descriptor for `JoinSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinSpaceResponseDescriptor = $convert.base64Decode(
    'ChFKb2luU3BhY2VSZXNwb25zZRJKChBzcGFjZV9tZW1iZXJzaGlwGAEgASgLMh8udm9pY2Uuc3'
    'BhY2UudjEuU3BhY2VNZW1iZXJzaGlwUg9zcGFjZU1lbWJlcnNoaXA=');

@$core.Deprecated('Use leaveSpaceResponseDescriptor instead')
const LeaveSpaceResponse$json = {
  '1': 'LeaveSpaceResponse',
};

/// Descriptor for `LeaveSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveSpaceResponseDescriptor =
    $convert.base64Decode('ChJMZWF2ZVNwYWNlUmVzcG9uc2U=');

@$core.Deprecated('Use kickMemberResponseDescriptor instead')
const KickMemberResponse$json = {
  '1': 'KickMemberResponse',
};

/// Descriptor for `KickMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List kickMemberResponseDescriptor =
    $convert.base64Decode('ChJLaWNrTWVtYmVyUmVzcG9uc2U=');

@$core.Deprecated('Use banMemberResponseDescriptor instead')
const BanMemberResponse$json = {
  '1': 'BanMemberResponse',
};

/// Descriptor for `BanMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banMemberResponseDescriptor =
    $convert.base64Decode('ChFCYW5NZW1iZXJSZXNwb25zZQ==');

@$core.Deprecated('Use unbanMemberResponseDescriptor instead')
const UnbanMemberResponse$json = {
  '1': 'UnbanMemberResponse',
};

/// Descriptor for `UnbanMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unbanMemberResponseDescriptor =
    $convert.base64Decode('ChNVbmJhbk1lbWJlclJlc3BvbnNl');

@$core.Deprecated('Use listMembersResponseDescriptor instead')
const ListMembersResponse$json = {
  '1': 'ListMembersResponse',
  '2': [
    {
      '1': 'space_member_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceMemberList',
      '10': 'spaceMemberList'
    },
  ],
};

/// Descriptor for `ListMembersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0TWVtYmVyc1Jlc3BvbnNlEksKEXNwYWNlX21lbWJlcl9saXN0GAEgASgLMh8udm9pY2'
    'Uuc3BhY2UudjEuU3BhY2VNZW1iZXJMaXN0Ug9zcGFjZU1lbWJlckxpc3Q=');

@$core.Deprecated('Use listBansResponseDescriptor instead')
const ListBansResponse$json = {
  '1': 'ListBansResponse',
  '2': [
    {
      '1': 'ban_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.BanList',
      '10': 'banList'
    },
  ],
};

/// Descriptor for `ListBansResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBansResponseDescriptor = $convert.base64Decode(
    'ChBMaXN0QmFuc1Jlc3BvbnNlEjIKCGJhbl9saXN0GAEgASgLMhcudm9pY2Uuc3BhY2UudjEuQm'
    'FuTGlzdFIHYmFuTGlzdA==');

@$core.Deprecated('Use timeoutMemberResponseDescriptor instead')
const TimeoutMemberResponse$json = {
  '1': 'TimeoutMemberResponse',
};

/// Descriptor for `TimeoutMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List timeoutMemberResponseDescriptor =
    $convert.base64Decode('ChVUaW1lb3V0TWVtYmVyUmVzcG9uc2U=');

@$core.Deprecated('Use removeMemberTimeoutResponseDescriptor instead')
const RemoveMemberTimeoutResponse$json = {
  '1': 'RemoveMemberTimeoutResponse',
};

/// Descriptor for `RemoveMemberTimeoutResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberTimeoutResponseDescriptor =
    $convert.base64Decode('ChtSZW1vdmVNZW1iZXJUaW1lb3V0UmVzcG9uc2U=');

@$core.Deprecated('Use transferOwnershipResponseDescriptor instead')
const TransferOwnershipResponse$json = {
  '1': 'TransferOwnershipResponse',
};

/// Descriptor for `TransferOwnershipResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transferOwnershipResponseDescriptor =
    $convert.base64Decode('ChlUcmFuc2Zlck93bmVyc2hpcFJlc3BvbnNl');

@$core.Deprecated('Use listTemplatesResponseDescriptor instead')
const ListTemplatesResponse$json = {
  '1': 'ListTemplatesResponse',
  '2': [
    {
      '1': 'template_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.TemplateList',
      '10': 'templateList'
    },
  ],
};

/// Descriptor for `ListTemplatesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listTemplatesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0VGVtcGxhdGVzUmVzcG9uc2USQQoNdGVtcGxhdGVfbGlzdBgBIAEoCzIcLnZvaWNlLn'
    'NwYWNlLnYxLlRlbXBsYXRlTGlzdFIMdGVtcGxhdGVMaXN0');

@$core.Deprecated('Use createFromTemplateResponseDescriptor instead')
const CreateFromTemplateResponse$json = {
  '1': 'CreateFromTemplateResponse',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.Space',
      '10': 'space'
    },
  ],
};

/// Descriptor for `CreateFromTemplateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createFromTemplateResponseDescriptor =
    $convert.base64Decode(
        'ChpDcmVhdGVGcm9tVGVtcGxhdGVSZXNwb25zZRIrCgVzcGFjZRgBIAEoCzIVLnZvaWNlLnNwYW'
        'NlLnYxLlNwYWNlUgVzcGFjZQ==');

@$core.Deprecated('Use getAuditLogResponseDescriptor instead')
const GetAuditLogResponse$json = {
  '1': 'GetAuditLogResponse',
  '2': [
    {
      '1': 'audit_log_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.AuditLogList',
      '10': 'auditLogList'
    },
  ],
};

/// Descriptor for `GetAuditLogResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAuditLogResponseDescriptor = $convert.base64Decode(
    'ChNHZXRBdWRpdExvZ1Jlc3BvbnNlEkIKDmF1ZGl0X2xvZ19saXN0GAEgASgLMhwudm9pY2Uuc3'
    'BhY2UudjEuQXVkaXRMb2dMaXN0UgxhdWRpdExvZ0xpc3Q=');
