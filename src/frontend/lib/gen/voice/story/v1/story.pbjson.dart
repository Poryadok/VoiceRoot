// This is a generated file - do not edit.
//
// Generated from voice/story/v1/story.proto.

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

@$core.Deprecated('Use storyMediaTypeDescriptor instead')
const StoryMediaType$json = {
  '1': 'StoryMediaType',
  '2': [
    {'1': 'STORY_MEDIA_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'STORY_MEDIA_TYPE_PHOTO', '2': 1},
    {'1': 'STORY_MEDIA_TYPE_VIDEO', '2': 2},
    {'1': 'STORY_MEDIA_TYPE_TEXT', '2': 3},
  ],
};

/// Descriptor for `StoryMediaType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storyMediaTypeDescriptor = $convert.base64Decode(
    'Cg5TdG9yeU1lZGlhVHlwZRIgChxTVE9SWV9NRURJQV9UWVBFX1VOU1BFQ0lGSUVEEAASGgoWU1'
    'RPUllfTUVESUFfVFlQRV9QSE9UTxABEhoKFlNUT1JZX01FRElBX1RZUEVfVklERU8QAhIZChVT'
    'VE9SWV9NRURJQV9UWVBFX1RFWFQQAw==');

@$core.Deprecated('Use storyAudienceDescriptor instead')
const StoryAudience$json = {
  '1': 'StoryAudience',
  '2': [
    {'1': 'STORY_AUDIENCE_UNSPECIFIED', '2': 0},
    {'1': 'STORY_AUDIENCE_PUBLIC', '2': 1},
    {'1': 'STORY_AUDIENCE_FRIENDS', '2': 2},
    {'1': 'STORY_AUDIENCE_CLOSE_FRIENDS', '2': 3},
    {'1': 'STORY_AUDIENCE_CUSTOM', '2': 4},
  ],
};

/// Descriptor for `StoryAudience`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List storyAudienceDescriptor = $convert.base64Decode(
    'Cg1TdG9yeUF1ZGllbmNlEh4KGlNUT1JZX0FVRElFTkNFX1VOU1BFQ0lGSUVEEAASGQoVU1RPUl'
    'lfQVVESUVOQ0VfUFVCTElDEAESGgoWU1RPUllfQVVESUVOQ0VfRlJJRU5EUxACEiAKHFNUT1JZ'
    'X0FVRElFTkNFX0NMT1NFX0ZSSUVORFMQAxIZChVTVE9SWV9BVURJRU5DRV9DVVNUT00QBA==');

@$core.Deprecated('Use storyDescriptor instead')
const Story$json = {
  '1': 'Story',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'author_profile_id', '3': 2, '4': 1, '5': 9, '10': 'authorProfileId'},
    {'1': 'type', '3': 3, '4': 1, '5': 9, '10': 'type'},
    {
      '1': 'media_file_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'mediaFileId',
      '17': true
    },
    {
      '1': 'text_content',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'textContent',
      '17': true
    },
    {
      '1': 'text_style_json',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'textStyleJson',
      '17': true
    },
    {
      '1': 'game_tag',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'gameTag',
      '17': true
    },
    {
      '1': 'is_looking_for_party',
      '3': 8,
      '4': 1,
      '5': 8,
      '10': 'isLookingForParty'
    },
    {
      '1': 'lfp_criteria_json',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'lfpCriteriaJson',
      '17': true
    },
    {
      '1': 'mention_profile_ids_json',
      '3': 10,
      '4': 1,
      '5': 9,
      '10': 'mentionProfileIdsJson'
    },
    {'1': 'view_count', '3': 11, '4': 1, '5': 5, '10': 'viewCount'},
    {'1': 'visibility', '3': 12, '4': 1, '5': 9, '10': 'visibility'},
    {
      '1': 'expires_at',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {
      '1': 'archived_until',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'archivedUntil'
    },
    {
      '1': 'created_at',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'deleted_at',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 5,
      '10': 'deletedAt',
      '17': true
    },
    {
      '1': 'type_enum',
      '3': 17,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryMediaType',
      '9': 6,
      '10': 'typeEnum',
      '17': true
    },
    {
      '1': 'visibility_enum',
      '3': 18,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryAudience',
      '9': 7,
      '10': 'visibilityEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_media_file_id'},
    {'1': '_text_content'},
    {'1': '_text_style_json'},
    {'1': '_game_tag'},
    {'1': '_lfp_criteria_json'},
    {'1': '_deleted_at'},
    {'1': '_type_enum'},
    {'1': '_visibility_enum'},
  ],
};

/// Descriptor for `Story`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyDescriptor = $convert.base64Decode(
    'CgVTdG9yeRIOCgJpZBgBIAEoCVICaWQSKgoRYXV0aG9yX3Byb2ZpbGVfaWQYAiABKAlSD2F1dG'
    'hvclByb2ZpbGVJZBISCgR0eXBlGAMgASgJUgR0eXBlEicKDW1lZGlhX2ZpbGVfaWQYBCABKAlI'
    'AFILbWVkaWFGaWxlSWSIAQESJgoMdGV4dF9jb250ZW50GAUgASgJSAFSC3RleHRDb250ZW50iA'
    'EBEisKD3RleHRfc3R5bGVfanNvbhgGIAEoCUgCUg10ZXh0U3R5bGVKc29uiAEBEh4KCGdhbWVf'
    'dGFnGAcgASgJSANSB2dhbWVUYWeIAQESLwoUaXNfbG9va2luZ19mb3JfcGFydHkYCCABKAhSEW'
    'lzTG9va2luZ0ZvclBhcnR5Ei8KEWxmcF9jcml0ZXJpYV9qc29uGAkgASgJSARSD2xmcENyaXRl'
    'cmlhSnNvbogBARI3ChhtZW50aW9uX3Byb2ZpbGVfaWRzX2pzb24YCiABKAlSFW1lbnRpb25Qcm'
    '9maWxlSWRzSnNvbhIdCgp2aWV3X2NvdW50GAsgASgFUgl2aWV3Q291bnQSHgoKdmlzaWJpbGl0'
    'eRgMIAEoCVIKdmlzaWJpbGl0eRI5CgpleHBpcmVzX2F0GA0gASgLMhouZ29vZ2xlLnByb3RvYn'
    'VmLlRpbWVzdGFtcFIJZXhwaXJlc0F0EkEKDmFyY2hpdmVkX3VudGlsGA4gASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcFINYXJjaGl2ZWRVbnRpbBI5CgpjcmVhdGVkX2F0GA8gASgLMh'
    'ouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRlZEF0Ej4KCmRlbGV0ZWRfYXQYECAB'
    'KAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAVSCWRlbGV0ZWRBdIgBARJACgl0eXBlX2'
    'VudW0YESABKA4yHi52b2ljZS5zdG9yeS52MS5TdG9yeU1lZGlhVHlwZUgGUgh0eXBlRW51bYgB'
    'ARJLCg92aXNpYmlsaXR5X2VudW0YEiABKA4yHS52b2ljZS5zdG9yeS52MS5TdG9yeUF1ZGllbm'
    'NlSAdSDnZpc2liaWxpdHlFbnVtiAEBQhAKDl9tZWRpYV9maWxlX2lkQg8KDV90ZXh0X2NvbnRl'
    'bnRCEgoQX3RleHRfc3R5bGVfanNvbkILCglfZ2FtZV90YWdCFAoSX2xmcF9jcml0ZXJpYV9qc2'
    '9uQg0KC19kZWxldGVkX2F0QgwKCl90eXBlX2VudW1CEgoQX3Zpc2liaWxpdHlfZW51bQ==');

@$core.Deprecated('Use storyRefDescriptor instead')
const StoryRef$json = {
  '1': 'StoryRef',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `StoryRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyRefDescriptor = $convert
    .base64Decode('CghTdG9yeVJlZhIZCghzdG9yeV9pZBgBIAEoCVIHc3RvcnlJZA==');

@$core.Deprecated('Use createStoryRequestDescriptor instead')
const CreateStoryRequest$json = {
  '1': 'CreateStoryRequest',
  '2': [
    {'1': 'type', '3': 1, '4': 1, '5': 9, '10': 'type'},
    {
      '1': 'media_file_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'mediaFileId',
      '17': true
    },
    {
      '1': 'text_content',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'textContent',
      '17': true
    },
    {
      '1': 'text_style_json',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'textStyleJson',
      '17': true
    },
    {
      '1': 'game_tag',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'gameTag',
      '17': true
    },
    {'1': 'visibility', '3': 6, '4': 1, '5': 9, '10': 'visibility'},
    {
      '1': 'type_enum',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryMediaType',
      '9': 4,
      '10': 'typeEnum',
      '17': true
    },
    {
      '1': 'visibility_enum',
      '3': 8,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryAudience',
      '9': 5,
      '10': 'visibilityEnum',
      '17': true
    },
    {
      '1': 'mention_profile_ids',
      '3': 9,
      '4': 3,
      '5': 9,
      '10': 'mentionProfileIds'
    },
  ],
  '8': [
    {'1': '_media_file_id'},
    {'1': '_text_content'},
    {'1': '_text_style_json'},
    {'1': '_game_tag'},
    {'1': '_type_enum'},
    {'1': '_visibility_enum'},
  ],
};

/// Descriptor for `CreateStoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createStoryRequestDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVTdG9yeVJlcXVlc3QSEgoEdHlwZRgBIAEoCVIEdHlwZRInCg1tZWRpYV9maWxlX2'
    'lkGAIgASgJSABSC21lZGlhRmlsZUlkiAEBEiYKDHRleHRfY29udGVudBgDIAEoCUgBUgt0ZXh0'
    'Q29udGVudIgBARIrCg90ZXh0X3N0eWxlX2pzb24YBCABKAlIAlINdGV4dFN0eWxlSnNvbogBAR'
    'IeCghnYW1lX3RhZxgFIAEoCUgDUgdnYW1lVGFniAEBEh4KCnZpc2liaWxpdHkYBiABKAlSCnZp'
    'c2liaWxpdHkSQAoJdHlwZV9lbnVtGAcgASgOMh4udm9pY2Uuc3RvcnkudjEuU3RvcnlNZWRpYV'
    'R5cGVIBFIIdHlwZUVudW2IAQESSwoPdmlzaWJpbGl0eV9lbnVtGAggASgOMh0udm9pY2Uuc3Rv'
    'cnkudjEuU3RvcnlBdWRpZW5jZUgFUg52aXNpYmlsaXR5RW51bYgBARIuChNtZW50aW9uX3Byb2'
    'ZpbGVfaWRzGAkgAygJUhFtZW50aW9uUHJvZmlsZUlkc0IQCg5fbWVkaWFfZmlsZV9pZEIPCg1f'
    'dGV4dF9jb250ZW50QhIKEF90ZXh0X3N0eWxlX2pzb25CCwoJX2dhbWVfdGFnQgwKCl90eXBlX2'
    'VudW1CEgoQX3Zpc2liaWxpdHlfZW51bQ==');

@$core.Deprecated('Use deleteStoryRequestDescriptor instead')
const DeleteStoryRequest$json = {
  '1': 'DeleteStoryRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `DeleteStoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteStoryRequestDescriptor =
    $convert.base64Decode(
        'ChJEZWxldGVTdG9yeVJlcXVlc3QSGQoIc3RvcnlfaWQYASABKAlSB3N0b3J5SWQ=');

@$core.Deprecated('Use getStoryRequestDescriptor instead')
const GetStoryRequest$json = {
  '1': 'GetStoryRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `GetStoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryRequestDescriptor = $convert.base64Decode(
    'Cg9HZXRTdG9yeVJlcXVlc3QSGQoIc3RvcnlfaWQYASABKAlSB3N0b3J5SWQ=');

@$core.Deprecated('Use getStoryFeedRequestDescriptor instead')
const GetStoryFeedRequest$json = {
  '1': 'GetStoryFeedRequest',
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

/// Descriptor for `GetStoryFeedRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryFeedRequestDescriptor = $convert.base64Decode(
    'ChNHZXRTdG9yeUZlZWRSZXF1ZXN0EjYKBHBhZ2UYASABKAsyIi52b2ljZS5jb21tb24udjEuQ3'
    'Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use getProfileStoriesRequestDescriptor instead')
const GetProfileStoriesRequest$json = {
  '1': 'GetProfileStoriesRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetProfileStoriesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileStoriesRequestDescriptor =
    $convert.base64Decode(
        'ChhHZXRQcm9maWxlU3Rvcmllc1JlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZU'
        'lk');

@$core.Deprecated('Use storyListDescriptor instead')
const StoryList$json = {
  '1': 'StoryList',
  '2': [
    {
      '1': 'stories',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'stories'
    },
  ],
};

/// Descriptor for `StoryList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyListDescriptor = $convert.base64Decode(
    'CglTdG9yeUxpc3QSLwoHc3RvcmllcxgBIAMoCzIVLnZvaWNlLnN0b3J5LnYxLlN0b3J5UgdzdG'
    '9yaWVz');

@$core.Deprecated('Use markViewedRequestDescriptor instead')
const MarkViewedRequest$json = {
  '1': 'MarkViewedRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
    {'1': 'anonymous', '3': 2, '4': 1, '5': 8, '10': 'anonymous'},
  ],
};

/// Descriptor for `MarkViewedRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markViewedRequestDescriptor = $convert.base64Decode(
    'ChFNYXJrVmlld2VkUmVxdWVzdBIZCghzdG9yeV9pZBgBIAEoCVIHc3RvcnlJZBIcCglhbm9ueW'
    '1vdXMYAiABKAhSCWFub255bW91cw==');

@$core.Deprecated('Use getViewersRequestDescriptor instead')
const GetViewersRequest$json = {
  '1': 'GetViewersRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `GetViewersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getViewersRequestDescriptor = $convert.base64Decode(
    'ChFHZXRWaWV3ZXJzUmVxdWVzdBIZCghzdG9yeV9pZBgBIAEoCVIHc3RvcnlJZA==');

@$core.Deprecated('Use viewerListDescriptor instead')
const ViewerList$json = {
  '1': 'ViewerList',
  '2': [
    {
      '1': 'viewer_profile_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'viewerProfileIds'
    },
  ],
};

/// Descriptor for `ViewerList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List viewerListDescriptor = $convert.base64Decode(
    'CgpWaWV3ZXJMaXN0EiwKEnZpZXdlcl9wcm9maWxlX2lkcxgBIAMoCVIQdmlld2VyUHJvZmlsZU'
    'lkcw==');

@$core.Deprecated('Use reactToStoryRequestDescriptor instead')
const ReactToStoryRequest$json = {
  '1': 'ReactToStoryRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
    {'1': 'emoji', '3': 2, '4': 1, '5': 9, '10': 'emoji'},
  ],
};

/// Descriptor for `ReactToStoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reactToStoryRequestDescriptor = $convert.base64Decode(
    'ChNSZWFjdFRvU3RvcnlSZXF1ZXN0EhkKCHN0b3J5X2lkGAEgASgJUgdzdG9yeUlkEhQKBWVtb2'
    'ppGAIgASgJUgVlbW9qaQ==');

@$core.Deprecated('Use storyReactionDescriptor instead')
const StoryReaction$json = {
  '1': 'StoryReaction',
  '2': [
    {
      '1': 'reactor_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'reactorProfileId'
    },
    {'1': 'emoji', '3': 2, '4': 1, '5': 9, '10': 'emoji'},
  ],
};

/// Descriptor for `StoryReaction`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyReactionDescriptor = $convert.base64Decode(
    'Cg1TdG9yeVJlYWN0aW9uEiwKEnJlYWN0b3JfcHJvZmlsZV9pZBgBIAEoCVIQcmVhY3RvclByb2'
    'ZpbGVJZBIUCgVlbW9qaRgCIAEoCVIFZW1vamk=');

@$core.Deprecated('Use getStoryReactionsRequestDescriptor instead')
const GetStoryReactionsRequest$json = {
  '1': 'GetStoryReactionsRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `GetStoryReactionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryReactionsRequestDescriptor =
    $convert.base64Decode(
        'ChhHZXRTdG9yeVJlYWN0aW9uc1JlcXVlc3QSGQoIc3RvcnlfaWQYASABKAlSB3N0b3J5SWQ=');

@$core.Deprecated('Use replyToStoryRequestDescriptor instead')
const ReplyToStoryRequest$json = {
  '1': 'ReplyToStoryRequest',
  '2': [
    {'1': 'story_id', '3': 1, '4': 1, '5': 9, '10': 'storyId'},
    {'1': 'text', '3': 2, '4': 1, '5': 9, '10': 'text'},
  ],
};

/// Descriptor for `ReplyToStoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List replyToStoryRequestDescriptor = $convert.base64Decode(
    'ChNSZXBseVRvU3RvcnlSZXF1ZXN0EhkKCHN0b3J5X2lkGAEgASgJUgdzdG9yeUlkEhIKBHRleH'
    'QYAiABKAlSBHRleHQ=');

@$core.Deprecated('Use getArchiveRequestDescriptor instead')
const GetArchiveRequest$json = {
  '1': 'GetArchiveRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetArchiveRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getArchiveRequestDescriptor = $convert.base64Decode(
    'ChFHZXRBcmNoaXZlUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use highlightDescriptor instead')
const Highlight$json = {
  '1': 'Highlight',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'story_ids', '3': 4, '4': 3, '5': 9, '10': 'storyIds'},
    {'1': 'visibility', '3': 5, '4': 1, '5': 9, '10': 'visibility'},
    {
      '1': 'visibility_enum',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryAudience',
      '9': 0,
      '10': 'visibilityEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_visibility_enum'},
  ],
};

/// Descriptor for `Highlight`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List highlightDescriptor = $convert.base64Decode(
    'CglIaWdobGlnaHQSDgoCaWQYASABKAlSAmlkEh0KCnByb2ZpbGVfaWQYAiABKAlSCXByb2ZpbG'
    'VJZBISCgRuYW1lGAMgASgJUgRuYW1lEhsKCXN0b3J5X2lkcxgEIAMoCVIIc3RvcnlJZHMSHgoK'
    'dmlzaWJpbGl0eRgFIAEoCVIKdmlzaWJpbGl0eRJLCg92aXNpYmlsaXR5X2VudW0YBiABKA4yHS'
    '52b2ljZS5zdG9yeS52MS5TdG9yeUF1ZGllbmNlSABSDnZpc2liaWxpdHlFbnVtiAEBQhIKEF92'
    'aXNpYmlsaXR5X2VudW0=');

@$core.Deprecated('Use createHighlightRequestDescriptor instead')
const CreateHighlightRequest$json = {
  '1': 'CreateHighlightRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'visibility', '3': 2, '4': 1, '5': 9, '10': 'visibility'},
    {
      '1': 'visibility_enum',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryAudience',
      '9': 0,
      '10': 'visibilityEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_visibility_enum'},
  ],
};

/// Descriptor for `CreateHighlightRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createHighlightRequestDescriptor = $convert.base64Decode(
    'ChZDcmVhdGVIaWdobGlnaHRSZXF1ZXN0EhIKBG5hbWUYASABKAlSBG5hbWUSHgoKdmlzaWJpbG'
    'l0eRgCIAEoCVIKdmlzaWJpbGl0eRJLCg92aXNpYmlsaXR5X2VudW0YAyABKA4yHS52b2ljZS5z'
    'dG9yeS52MS5TdG9yeUF1ZGllbmNlSABSDnZpc2liaWxpdHlFbnVtiAEBQhIKEF92aXNpYmlsaX'
    'R5X2VudW0=');

@$core.Deprecated('Use updateHighlightRequestDescriptor instead')
const UpdateHighlightRequest$json = {
  '1': 'UpdateHighlightRequest',
  '2': [
    {'1': 'highlight_id', '3': 1, '4': 1, '5': 9, '10': 'highlightId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'visibility',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'visibility',
      '17': true
    },
    {
      '1': 'visibility_enum',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.StoryAudience',
      '9': 2,
      '10': 'visibilityEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_visibility'},
    {'1': '_visibility_enum'},
  ],
};

/// Descriptor for `UpdateHighlightRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateHighlightRequestDescriptor = $convert.base64Decode(
    'ChZVcGRhdGVIaWdobGlnaHRSZXF1ZXN0EiEKDGhpZ2hsaWdodF9pZBgBIAEoCVILaGlnaGxpZ2'
    'h0SWQSFwoEbmFtZRgCIAEoCUgAUgRuYW1liAEBEiMKCnZpc2liaWxpdHkYAyABKAlIAVIKdmlz'
    'aWJpbGl0eYgBARJLCg92aXNpYmlsaXR5X2VudW0YBCABKA4yHS52b2ljZS5zdG9yeS52MS5TdG'
    '9yeUF1ZGllbmNlSAJSDnZpc2liaWxpdHlFbnVtiAEBQgcKBV9uYW1lQg0KC192aXNpYmlsaXR5'
    'QhIKEF92aXNpYmlsaXR5X2VudW0=');

@$core.Deprecated('Use deleteHighlightRequestDescriptor instead')
const DeleteHighlightRequest$json = {
  '1': 'DeleteHighlightRequest',
  '2': [
    {'1': 'highlight_id', '3': 1, '4': 1, '5': 9, '10': 'highlightId'},
  ],
};

/// Descriptor for `DeleteHighlightRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteHighlightRequestDescriptor =
    $convert.base64Decode(
        'ChZEZWxldGVIaWdobGlnaHRSZXF1ZXN0EiEKDGhpZ2hsaWdodF9pZBgBIAEoCVILaGlnaGxpZ2'
        'h0SWQ=');

@$core.Deprecated('Use addToHighlightRequestDescriptor instead')
const AddToHighlightRequest$json = {
  '1': 'AddToHighlightRequest',
  '2': [
    {'1': 'highlight_id', '3': 1, '4': 1, '5': 9, '10': 'highlightId'},
    {'1': 'story_id', '3': 2, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `AddToHighlightRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addToHighlightRequestDescriptor = $convert.base64Decode(
    'ChVBZGRUb0hpZ2hsaWdodFJlcXVlc3QSIQoMaGlnaGxpZ2h0X2lkGAEgASgJUgtoaWdobGlnaH'
    'RJZBIZCghzdG9yeV9pZBgCIAEoCVIHc3RvcnlJZA==');

@$core.Deprecated('Use removeFromHighlightRequestDescriptor instead')
const RemoveFromHighlightRequest$json = {
  '1': 'RemoveFromHighlightRequest',
  '2': [
    {'1': 'highlight_id', '3': 1, '4': 1, '5': 9, '10': 'highlightId'},
    {'1': 'story_id', '3': 2, '4': 1, '5': 9, '10': 'storyId'},
  ],
};

/// Descriptor for `RemoveFromHighlightRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFromHighlightRequestDescriptor =
    $convert.base64Decode(
        'ChpSZW1vdmVGcm9tSGlnaGxpZ2h0UmVxdWVzdBIhCgxoaWdobGlnaHRfaWQYASABKAlSC2hpZ2'
        'hsaWdodElkEhkKCHN0b3J5X2lkGAIgASgJUgdzdG9yeUlk');

@$core.Deprecated('Use getHighlightsRequestDescriptor instead')
const GetHighlightsRequest$json = {
  '1': 'GetHighlightsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetHighlightsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getHighlightsRequestDescriptor = $convert.base64Decode(
    'ChRHZXRIaWdobGlnaHRzUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use highlightListDescriptor instead')
const HighlightList$json = {
  '1': 'HighlightList',
  '2': [
    {
      '1': 'highlights',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.Highlight',
      '10': 'highlights'
    },
  ],
};

/// Descriptor for `HighlightList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List highlightListDescriptor = $convert.base64Decode(
    'Cg1IaWdobGlnaHRMaXN0EjkKCmhpZ2hsaWdodHMYASADKAsyGS52b2ljZS5zdG9yeS52MS5IaW'
    'dobGlnaHRSCmhpZ2hsaWdodHM=');

@$core.Deprecated('Use createLookingForPartyRequestDescriptor instead')
const CreateLookingForPartyRequest$json = {
  '1': 'CreateLookingForPartyRequest',
  '2': [
    {'1': 'criteria_json', '3': 1, '4': 1, '5': 9, '10': 'criteriaJson'},
    {
      '1': 'media_file_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'mediaFileId',
      '17': true
    },
  ],
  '8': [
    {'1': '_media_file_id'},
  ],
};

/// Descriptor for `CreateLookingForPartyRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createLookingForPartyRequestDescriptor =
    $convert.base64Decode(
        'ChxDcmVhdGVMb29raW5nRm9yUGFydHlSZXF1ZXN0EiMKDWNyaXRlcmlhX2pzb24YASABKAlSDG'
        'NyaXRlcmlhSnNvbhInCg1tZWRpYV9maWxlX2lkGAIgASgJSABSC21lZGlhRmlsZUlkiAEBQhAK'
        'Dl9tZWRpYV9maWxlX2lk');

@$core.Deprecated('Use createStoryResponseDescriptor instead')
const CreateStoryResponse$json = {
  '1': 'CreateStoryResponse',
  '2': [
    {
      '1': 'story',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'story'
    },
  ],
};

/// Descriptor for `CreateStoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createStoryResponseDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVTdG9yeVJlc3BvbnNlEisKBXN0b3J5GAEgASgLMhUudm9pY2Uuc3RvcnkudjEuU3'
    'RvcnlSBXN0b3J5');

@$core.Deprecated('Use deleteStoryResponseDescriptor instead')
const DeleteStoryResponse$json = {
  '1': 'DeleteStoryResponse',
};

/// Descriptor for `DeleteStoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteStoryResponseDescriptor =
    $convert.base64Decode('ChNEZWxldGVTdG9yeVJlc3BvbnNl');

@$core.Deprecated('Use getStoryResponseDescriptor instead')
const GetStoryResponse$json = {
  '1': 'GetStoryResponse',
  '2': [
    {
      '1': 'story',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'story'
    },
  ],
};

/// Descriptor for `GetStoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryResponseDescriptor = $convert.base64Decode(
    'ChBHZXRTdG9yeVJlc3BvbnNlEisKBXN0b3J5GAEgASgLMhUudm9pY2Uuc3RvcnkudjEuU3Rvcn'
    'lSBXN0b3J5');

@$core.Deprecated('Use storyFeedGroupDescriptor instead')
const StoryFeedGroup$json = {
  '1': 'StoryFeedGroup',
  '2': [
    {'1': 'author_profile_id', '3': 1, '4': 1, '5': 9, '10': 'authorProfileId'},
    {
      '1': 'stories',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'stories'
    },
  ],
};

/// Descriptor for `StoryFeedGroup`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List storyFeedGroupDescriptor = $convert.base64Decode(
    'Cg5TdG9yeUZlZWRHcm91cBIqChFhdXRob3JfcHJvZmlsZV9pZBgBIAEoCVIPYXV0aG9yUHJvZm'
    'lsZUlkEi8KB3N0b3JpZXMYAiADKAsyFS52b2ljZS5zdG9yeS52MS5TdG9yeVIHc3Rvcmllcw==');

@$core.Deprecated('Use getStoryFeedResponseDescriptor instead')
const GetStoryFeedResponse$json = {
  '1': 'GetStoryFeedResponse',
  '2': [
    {
      '1': 'stories',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'stories'
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
    {
      '1': 'feed_groups',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.StoryFeedGroup',
      '10': 'feedGroups'
    },
  ],
  '8': [
    {'1': '_page'},
  ],
};

/// Descriptor for `GetStoryFeedResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryFeedResponseDescriptor = $convert.base64Decode(
    'ChRHZXRTdG9yeUZlZWRSZXNwb25zZRIvCgdzdG9yaWVzGAEgAygLMhUudm9pY2Uuc3Rvcnkudj'
    'EuU3RvcnlSB3N0b3JpZXMSHwoLbmV4dF9jdXJzb3IYAiABKAlSCm5leHRDdXJzb3ISPAoEcGFn'
    'ZRgDIAEoCzIjLnZvaWNlLmNvbW1vbi52MS5DdXJzb3JQYWdlUmVzcG9uc2VIAFIEcGFnZYgBAR'
    'I/CgtmZWVkX2dyb3VwcxgEIAMoCzIeLnZvaWNlLnN0b3J5LnYxLlN0b3J5RmVlZEdyb3VwUgpm'
    'ZWVkR3JvdXBzQgcKBV9wYWdl');

@$core.Deprecated('Use getProfileStoriesResponseDescriptor instead')
const GetProfileStoriesResponse$json = {
  '1': 'GetProfileStoriesResponse',
  '2': [
    {
      '1': 'story_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.StoryList',
      '10': 'storyList'
    },
  ],
};

/// Descriptor for `GetProfileStoriesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileStoriesResponseDescriptor =
    $convert.base64Decode(
        'ChlHZXRQcm9maWxlU3Rvcmllc1Jlc3BvbnNlEjgKCnN0b3J5X2xpc3QYASABKAsyGS52b2ljZS'
        '5zdG9yeS52MS5TdG9yeUxpc3RSCXN0b3J5TGlzdA==');

@$core.Deprecated('Use markViewedResponseDescriptor instead')
const MarkViewedResponse$json = {
  '1': 'MarkViewedResponse',
};

/// Descriptor for `MarkViewedResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markViewedResponseDescriptor =
    $convert.base64Decode('ChJNYXJrVmlld2VkUmVzcG9uc2U=');

@$core.Deprecated('Use getViewersResponseDescriptor instead')
const GetViewersResponse$json = {
  '1': 'GetViewersResponse',
  '2': [
    {
      '1': 'viewer_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.ViewerList',
      '10': 'viewerList'
    },
  ],
};

/// Descriptor for `GetViewersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getViewersResponseDescriptor = $convert.base64Decode(
    'ChJHZXRWaWV3ZXJzUmVzcG9uc2USOwoLdmlld2VyX2xpc3QYASABKAsyGi52b2ljZS5zdG9yeS'
    '52MS5WaWV3ZXJMaXN0Ugp2aWV3ZXJMaXN0');

@$core.Deprecated('Use reactToStoryResponseDescriptor instead')
const ReactToStoryResponse$json = {
  '1': 'ReactToStoryResponse',
};

/// Descriptor for `ReactToStoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reactToStoryResponseDescriptor =
    $convert.base64Decode('ChRSZWFjdFRvU3RvcnlSZXNwb25zZQ==');

@$core.Deprecated('Use getStoryReactionsResponseDescriptor instead')
const GetStoryReactionsResponse$json = {
  '1': 'GetStoryReactionsResponse',
  '2': [
    {
      '1': 'reactions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.story.v1.StoryReaction',
      '10': 'reactions'
    },
  ],
};

/// Descriptor for `GetStoryReactionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getStoryReactionsResponseDescriptor =
    $convert.base64Decode(
        'ChlHZXRTdG9yeVJlYWN0aW9uc1Jlc3BvbnNlEjsKCXJlYWN0aW9ucxgBIAMoCzIdLnZvaWNlLn'
        'N0b3J5LnYxLlN0b3J5UmVhY3Rpb25SCXJlYWN0aW9ucw==');

@$core.Deprecated('Use replyToStoryResponseDescriptor instead')
const ReplyToStoryResponse$json = {
  '1': 'ReplyToStoryResponse',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'message_id', '3': 2, '4': 1, '5': 9, '10': 'messageId'},
  ],
};

/// Descriptor for `ReplyToStoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List replyToStoryResponseDescriptor = $convert.base64Decode(
    'ChRSZXBseVRvU3RvcnlSZXNwb25zZRIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQSHQoKbWVzc2'
    'FnZV9pZBgCIAEoCVIJbWVzc2FnZUlk');

@$core.Deprecated('Use getArchiveResponseDescriptor instead')
const GetArchiveResponse$json = {
  '1': 'GetArchiveResponse',
  '2': [
    {
      '1': 'story_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.StoryList',
      '10': 'storyList'
    },
  ],
};

/// Descriptor for `GetArchiveResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getArchiveResponseDescriptor = $convert.base64Decode(
    'ChJHZXRBcmNoaXZlUmVzcG9uc2USOAoKc3RvcnlfbGlzdBgBIAEoCzIZLnZvaWNlLnN0b3J5Ln'
    'YxLlN0b3J5TGlzdFIJc3RvcnlMaXN0');

@$core.Deprecated('Use createHighlightResponseDescriptor instead')
const CreateHighlightResponse$json = {
  '1': 'CreateHighlightResponse',
  '2': [
    {
      '1': 'highlight',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.Highlight',
      '10': 'highlight'
    },
  ],
};

/// Descriptor for `CreateHighlightResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createHighlightResponseDescriptor =
    $convert.base64Decode(
        'ChdDcmVhdGVIaWdobGlnaHRSZXNwb25zZRI3CgloaWdobGlnaHQYASABKAsyGS52b2ljZS5zdG'
        '9yeS52MS5IaWdobGlnaHRSCWhpZ2hsaWdodA==');

@$core.Deprecated('Use updateHighlightResponseDescriptor instead')
const UpdateHighlightResponse$json = {
  '1': 'UpdateHighlightResponse',
  '2': [
    {
      '1': 'highlight',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.Highlight',
      '10': 'highlight'
    },
  ],
};

/// Descriptor for `UpdateHighlightResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateHighlightResponseDescriptor =
    $convert.base64Decode(
        'ChdVcGRhdGVIaWdobGlnaHRSZXNwb25zZRI3CgloaWdobGlnaHQYASABKAsyGS52b2ljZS5zdG'
        '9yeS52MS5IaWdobGlnaHRSCWhpZ2hsaWdodA==');

@$core.Deprecated('Use deleteHighlightResponseDescriptor instead')
const DeleteHighlightResponse$json = {
  '1': 'DeleteHighlightResponse',
};

/// Descriptor for `DeleteHighlightResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteHighlightResponseDescriptor =
    $convert.base64Decode('ChdEZWxldGVIaWdobGlnaHRSZXNwb25zZQ==');

@$core.Deprecated('Use addToHighlightResponseDescriptor instead')
const AddToHighlightResponse$json = {
  '1': 'AddToHighlightResponse',
};

/// Descriptor for `AddToHighlightResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addToHighlightResponseDescriptor =
    $convert.base64Decode('ChZBZGRUb0hpZ2hsaWdodFJlc3BvbnNl');

@$core.Deprecated('Use removeFromHighlightResponseDescriptor instead')
const RemoveFromHighlightResponse$json = {
  '1': 'RemoveFromHighlightResponse',
};

/// Descriptor for `RemoveFromHighlightResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFromHighlightResponseDescriptor =
    $convert.base64Decode('ChtSZW1vdmVGcm9tSGlnaGxpZ2h0UmVzcG9uc2U=');

@$core.Deprecated('Use getHighlightsResponseDescriptor instead')
const GetHighlightsResponse$json = {
  '1': 'GetHighlightsResponse',
  '2': [
    {
      '1': 'highlight_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.HighlightList',
      '10': 'highlightList'
    },
  ],
};

/// Descriptor for `GetHighlightsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getHighlightsResponseDescriptor = $convert.base64Decode(
    'ChVHZXRIaWdobGlnaHRzUmVzcG9uc2USRAoOaGlnaGxpZ2h0X2xpc3QYASABKAsyHS52b2ljZS'
    '5zdG9yeS52MS5IaWdobGlnaHRMaXN0Ug1oaWdobGlnaHRMaXN0');

@$core.Deprecated('Use createLookingForPartyResponseDescriptor instead')
const CreateLookingForPartyResponse$json = {
  '1': 'CreateLookingForPartyResponse',
  '2': [
    {
      '1': 'story',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.Story',
      '10': 'story'
    },
  ],
};

/// Descriptor for `CreateLookingForPartyResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createLookingForPartyResponseDescriptor =
    $convert.base64Decode(
        'Ch1DcmVhdGVMb29raW5nRm9yUGFydHlSZXNwb25zZRIrCgVzdG9yeRgBIAEoCzIVLnZvaWNlLn'
        'N0b3J5LnYxLlN0b3J5UgVzdG9yeQ==');
