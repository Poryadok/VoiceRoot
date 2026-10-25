// This is a generated file - do not edit.
//
// Generated from voice/common/v1/bot_message.proto.

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

@$core.Deprecated('Use botMessageSendBindingDescriptor instead')
const BotMessageSendBinding$json = {
  '1': 'BotMessageSendBinding',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'bot_id', '3': 2, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'installation_id', '3': 5, '4': 1, '5': 9, '10': 'installationId'},
    {
      '1': 'install_operation_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'installOperationId'
    },
    {
      '1': 'installation_generation',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'installationGeneration'
    },
    {'1': 'chat_id', '3': 8, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `BotMessageSendBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botMessageSendBindingDescriptor = $convert.base64Decode(
    'ChVCb3RNZXNzYWdlU2VuZEJpbmRpbmcSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcHJvdG'
    '9jb2xWZXJzaW9uEhUKBmJvdF9pZBgCIAEoCVIFYm90SWQSKAoQYWN0b3JfcHJvZmlsZV9pZBgD'
    'IAEoCVIOYWN0b3JQcm9maWxlSWQSGQoIc3BhY2VfaWQYBCABKAlSB3NwYWNlSWQSJwoPaW5zdG'
    'FsbGF0aW9uX2lkGAUgASgJUg5pbnN0YWxsYXRpb25JZBIwChRpbnN0YWxsX29wZXJhdGlvbl9p'
    'ZBgGIAEoCVISaW5zdGFsbE9wZXJhdGlvbklkEjcKF2luc3RhbGxhdGlvbl9nZW5lcmF0aW9uGA'
    'cgASgEUhZpbnN0YWxsYXRpb25HZW5lcmF0aW9uEhcKB2NoYXRfaWQYCCABKAlSBmNoYXRJZA==');
