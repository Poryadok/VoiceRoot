// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_lifecycle.proto.

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

@$core.Deprecated('Use chatLifecycleStateDescriptor instead')
const ChatLifecycleState$json = {
  '1': 'ChatLifecycleState',
  '2': [
    {'1': 'CHAT_LIFECYCLE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'CHAT_LIFECYCLE_STATE_LIVE', '2': 1},
    {'1': 'CHAT_LIFECYCLE_STATE_FROZEN', '2': 2},
    {'1': 'CHAT_LIFECYCLE_STATE_PURGE_DECIDED', '2': 3},
    {'1': 'CHAT_LIFECYCLE_STATE_PURGED', '2': 4},
  ],
};

/// Descriptor for `ChatLifecycleState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List chatLifecycleStateDescriptor = $convert.base64Decode(
    'ChJDaGF0TGlmZWN5Y2xlU3RhdGUSJAogQ0hBVF9MSUZFQ1lDTEVfU1RBVEVfVU5TUEVDSUZJRU'
    'QQABIdChlDSEFUX0xJRkVDWUNMRV9TVEFURV9MSVZFEAESHwobQ0hBVF9MSUZFQ1lDTEVfU1RB'
    'VEVfRlJPWkVOEAISJgoiQ0hBVF9MSUZFQ1lDTEVfU1RBVEVfUFVSR0VfREVDSURFRBADEh8KG0'
    'NIQVRfTElGRUNZQ0xFX1NUQVRFX1BVUkdFRBAE');

@$core.Deprecated('Use resolveChatLifecycleRequestDescriptor instead')
const ResolveChatLifecycleRequest$json = {
  '1': 'ResolveChatLifecycleRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'chat_id', '3': 3, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `ResolveChatLifecycleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveChatLifecycleRequestDescriptor =
    $convert.base64Decode(
        'ChtSZXNvbHZlQ2hhdExpZmVjeWNsZVJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDV'
        'IPcHJvdG9jb2xWZXJzaW9uEiEKDG9wZXJhdGlvbl9pZBgCIAEoCVILb3BlcmF0aW9uSWQSFwoH'
        'Y2hhdF9pZBgDIAEoCVIGY2hhdElk');

@$core.Deprecated('Use resolveChatLifecycleResponseDescriptor instead')
const ResolveChatLifecycleResponse$json = {
  '1': 'ResolveChatLifecycleResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'chat_id', '3': 2, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'generation', '3': 4, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'state',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatLifecycleState',
      '10': 'state'
    },
    {
      '1': 'deletion_operation_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'schema_version', '3': 7, '4': 1, '5': 13, '10': 'schemaVersion'},
  ],
};

/// Descriptor for `ResolveChatLifecycleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveChatLifecycleResponseDescriptor = $convert.base64Decode(
    'ChxSZXNvbHZlQ2hhdExpZmVjeWNsZVJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIXCgdjaGF0X2lkGAIgASgJUgZjaGF0SWQSGQoIc3BhY2VfaWQY'
    'AyABKAlSB3NwYWNlSWQSHgoKZ2VuZXJhdGlvbhgEIAEoBFIKZ2VuZXJhdGlvbhI3CgVzdGF0ZR'
    'gFIAEoDjIhLnZvaWNlLmNoYXQudjEuQ2hhdExpZmVjeWNsZVN0YXRlUgVzdGF0ZRIyChVkZWxl'
    'dGlvbl9vcGVyYXRpb25faWQYBiABKAlSE2RlbGV0aW9uT3BlcmF0aW9uSWQSJQoOc2NoZW1hX3'
    'ZlcnNpb24YByABKA1SDXNjaGVtYVZlcnNpb24=');
