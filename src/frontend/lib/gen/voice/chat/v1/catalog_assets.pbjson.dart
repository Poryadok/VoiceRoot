// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/catalog_assets.proto.

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

@$core.Deprecated('Use resolveGifAssetRequestDescriptor instead')
const ResolveGifAssetRequest$json = {
  '1': 'ResolveGifAssetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'provider', '3': 3, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 4, '4': 1, '5': 9, '10': 'providerId'},
    {
      '1': 'destination_chat',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `ResolveGifAssetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveGifAssetRequestDescriptor = $convert.base64Decode(
    'ChZSZXNvbHZlR2lmQXNzZXRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3Byb3'
    'RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlvbklkEhoKCHByb3Zp'
    'ZGVyGAMgASgJUghwcm92aWRlchIfCgtwcm92aWRlcl9pZBgEIAEoCVIKcHJvdmlkZXJJZBJBCh'
    'BkZXN0aW5hdGlvbl9jaGF0GAUgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmUg9kZXN0aW5h'
    'dGlvbkNoYXQ=');

@$core.Deprecated('Use resolveGifAssetResponseDescriptor instead')
const ResolveGifAssetResponse$json = {
  '1': 'ResolveGifAssetResponse',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'provider', '3': 2, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 3, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 4, '4': 1, '5': 9, '10': 'fileId'},
    {'1': 'status', '3': 5, '4': 1, '5': 9, '10': 'status'},
  ],
};

/// Descriptor for `ResolveGifAssetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveGifAssetResponseDescriptor = $convert.base64Decode(
    'ChdSZXNvbHZlR2lmQXNzZXRSZXNwb25zZRIbCglzb3VyY2VfaWQYASABKAlSCHNvdXJjZUlkEh'
    'oKCHByb3ZpZGVyGAIgASgJUghwcm92aWRlchIfCgtwcm92aWRlcl9pZBgDIAEoCVIKcHJvdmlk'
    'ZXJJZBIXCgdmaWxlX2lkGAQgASgJUgZmaWxlSWQSFgoGc3RhdHVzGAUgASgJUgZzdGF0dXM=');

@$core.Deprecated('Use validateGifAssetRequestDescriptor instead')
const ValidateGifAssetRequest$json = {
  '1': 'ValidateGifAssetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 6, '4': 1, '5': 9, '10': 'fileId'},
    {
      '1': 'destination_chat',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
  ],
};

/// Descriptor for `ValidateGifAssetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateGifAssetRequestDescriptor = $convert.base64Decode(
    'ChdWYWxpZGF0ZUdpZkFzc2V0UmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm'
    '90b2NvbFZlcnNpb24SKAoQYWN0b3JfYWNjb3VudF9pZBgCIAEoCVIOYWN0b3JBY2NvdW50SWQS'
    'KAoQYWN0b3JfcHJvZmlsZV9pZBgDIAEoCVIOYWN0b3JQcm9maWxlSWQSGgoIcHJvdmlkZXIYBC'
    'ABKAlSCHByb3ZpZGVyEh8KC3Byb3ZpZGVyX2lkGAUgASgJUgpwcm92aWRlcklkEhcKB2ZpbGVf'
    'aWQYBiABKAlSBmZpbGVJZBJBChBkZXN0aW5hdGlvbl9jaGF0GAcgASgLMhYudm9pY2UuY2hhdC'
    '52MS5DaGF0UmVmUg9kZXN0aW5hdGlvbkNoYXQ=');

@$core.Deprecated('Use validateGifAssetResponseDescriptor instead')
const ValidateGifAssetResponse$json = {
  '1': 'ValidateGifAssetResponse',
  '2': [
    {'1': 'source_id', '3': 1, '4': 1, '5': 9, '10': 'sourceId'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'provider', '3': 4, '4': 1, '5': 9, '10': 'provider'},
    {'1': 'provider_id', '3': 5, '4': 1, '5': 9, '10': 'providerId'},
    {'1': 'file_id', '3': 6, '4': 1, '5': 9, '10': 'fileId'},
  ],
};

/// Descriptor for `ValidateGifAssetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateGifAssetResponseDescriptor = $convert.base64Decode(
    'ChhWYWxpZGF0ZUdpZkFzc2V0UmVzcG9uc2USGwoJc291cmNlX2lkGAEgASgJUghzb3VyY2VJZB'
    'IoChBhY3Rvcl9hY2NvdW50X2lkGAIgASgJUg5hY3RvckFjY291bnRJZBIoChBhY3Rvcl9wcm9m'
    'aWxlX2lkGAMgASgJUg5hY3RvclByb2ZpbGVJZBIaCghwcm92aWRlchgEIAEoCVIIcHJvdmlkZX'
    'ISHwoLcHJvdmlkZXJfaWQYBSABKAlSCnByb3ZpZGVySWQSFwoHZmlsZV9pZBgGIAEoCVIGZmls'
    'ZUlk');

@$core.Deprecated('Use authorizeCatalogDestinationRequestDescriptor instead')
const AuthorizeCatalogDestinationRequest$json = {
  '1': 'AuthorizeCatalogDestinationRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'actor_account_id', '3': 2, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 3, '4': 1, '5': 9, '10': 'actorProfileId'},
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

/// Descriptor for `AuthorizeCatalogDestinationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authorizeCatalogDestinationRequestDescriptor =
    $convert.base64Decode(
        'CiJBdXRob3JpemVDYXRhbG9nRGVzdGluYXRpb25SZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb2'
        '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIoChBhY3Rvcl9hY2NvdW50X2lkGAIgASgJUg5hY3Rv'
        'ckFjY291bnRJZBIoChBhY3Rvcl9wcm9maWxlX2lkGAMgASgJUg5hY3RvclByb2ZpbGVJZBJBCh'
        'BkZXN0aW5hdGlvbl9jaGF0GAQgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmUg9kZXN0aW5h'
        'dGlvbkNoYXQ=');

@$core.Deprecated('Use authorizeCatalogDestinationResponseDescriptor instead')
const AuthorizeCatalogDestinationResponse$json = {
  '1': 'AuthorizeCatalogDestinationResponse',
  '2': [
    {'1': 'actor_account_id', '3': 1, '4': 1, '5': 9, '10': 'actorAccountId'},
    {'1': 'actor_profile_id', '3': 2, '4': 1, '5': 9, '10': 'actorProfileId'},
    {
      '1': 'destination_chat',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'destinationChat'
    },
    {
      '1': 'observed_chat_type',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatType',
      '10': 'observedChatType'
    },
    {
      '1': 'chat_account_generation',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'chatAccountGeneration'
    },
    {
      '1': 'chat_profile_generation',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'chatProfileGeneration'
    },
  ],
};

/// Descriptor for `AuthorizeCatalogDestinationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authorizeCatalogDestinationResponseDescriptor = $convert.base64Decode(
    'CiNBdXRob3JpemVDYXRhbG9nRGVzdGluYXRpb25SZXNwb25zZRIoChBhY3Rvcl9hY2NvdW50X2'
    'lkGAEgASgJUg5hY3RvckFjY291bnRJZBIoChBhY3Rvcl9wcm9maWxlX2lkGAIgASgJUg5hY3Rv'
    'clByb2ZpbGVJZBJBChBkZXN0aW5hdGlvbl9jaGF0GAMgASgLMhYudm9pY2UuY2hhdC52MS5DaG'
    'F0UmVmUg9kZXN0aW5hdGlvbkNoYXQSRQoSb2JzZXJ2ZWRfY2hhdF90eXBlGAQgASgOMhcudm9p'
    'Y2UuY2hhdC52MS5DaGF0VHlwZVIQb2JzZXJ2ZWRDaGF0VHlwZRI2ChdjaGF0X2FjY291bnRfZ2'
    'VuZXJhdGlvbhgFIAEoCVIVY2hhdEFjY291bnRHZW5lcmF0aW9uEjYKF2NoYXRfcHJvZmlsZV9n'
    'ZW5lcmF0aW9uGAYgASgJUhVjaGF0UHJvZmlsZUdlbmVyYXRpb24=');
