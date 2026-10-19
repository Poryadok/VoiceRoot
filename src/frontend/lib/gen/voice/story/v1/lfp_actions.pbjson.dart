// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actions.proto.

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

@$core.Deprecated('Use lfpConsentOfferViewDescriptor instead')
const LfpConsentOfferView$json = {
  '1': 'LfpConsentOfferView',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {'1': 'story_id', '3': 2, '4': 1, '5': 12, '10': 'storyId'},
    {
      '1': 'story_incarnation',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'storyIncarnation'
    },
    {'1': 'offer_revision', '3': 4, '4': 1, '5': 4, '10': 'offerRevision'},
    {
      '1': 'author_profile_id',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'authorProfileId'
    },
    {'1': 'criteria_sha256', '3': 6, '4': 1, '5': 12, '10': 'criteriaSha256'},
    {
      '1': 'offer_payload_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'offerPayloadSha256'
    },
    {
      '1': 'offer_expires_at_unix_millis',
      '3': 8,
      '4': 1,
      '5': 3,
      '10': 'offerExpiresAtUnixMillis'
    },
  ],
};

/// Descriptor for `LfpConsentOfferView`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpConsentOfferViewDescriptor = $convert.base64Decode(
    'ChNMZnBDb25zZW50T2ZmZXJWaWV3EikKEHByb3RvY29sX3ZlcnNpb24YASABKARSD3Byb3RvY2'
    '9sVmVyc2lvbhIZCghzdG9yeV9pZBgCIAEoDFIHc3RvcnlJZBIrChFzdG9yeV9pbmNhcm5hdGlv'
    'bhgDIAEoDFIQc3RvcnlJbmNhcm5hdGlvbhIlCg5vZmZlcl9yZXZpc2lvbhgEIAEoBFINb2ZmZX'
    'JSZXZpc2lvbhIqChFhdXRob3JfcHJvZmlsZV9pZBgFIAEoDFIPYXV0aG9yUHJvZmlsZUlkEicK'
    'D2NyaXRlcmlhX3NoYTI1NhgGIAEoDFIOY3JpdGVyaWFTaGEyNTYSMAoUb2ZmZXJfcGF5bG9hZF'
    '9zaGEyNTYYByABKAxSEm9mZmVyUGF5bG9hZFNoYTI1NhI+ChxvZmZlcl9leHBpcmVzX2F0X3Vu'
    'aXhfbWlsbGlzGAggASgDUhhvZmZlckV4cGlyZXNBdFVuaXhNaWxsaXM=');

@$core.Deprecated('Use publishLfpConsentOfferRequestDescriptor instead')
const PublishLfpConsentOfferRequest$json = {
  '1': 'PublishLfpConsentOfferRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'publication_operation_id',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'publicationOperationId'
    },
    {'1': 'story_id', '3': 3, '4': 1, '5': 12, '10': 'storyId'},
  ],
};

/// Descriptor for `PublishLfpConsentOfferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishLfpConsentOfferRequestDescriptor =
    $convert.base64Decode(
        'Ch1QdWJsaXNoTGZwQ29uc2VudE9mZmVyUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
        'gEUg9wcm90b2NvbFZlcnNpb24SOAoYcHVibGljYXRpb25fb3BlcmF0aW9uX2lkGAIgASgMUhZw'
        'dWJsaWNhdGlvbk9wZXJhdGlvbklkEhkKCHN0b3J5X2lkGAMgASgMUgdzdG9yeUlk');

@$core.Deprecated('Use publishLfpConsentOfferResponseDescriptor instead')
const PublishLfpConsentOfferResponse$json = {
  '1': 'PublishLfpConsentOfferResponse',
  '2': [
    {
      '1': 'offer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOfferView',
      '10': 'offer'
    },
  ],
};

/// Descriptor for `PublishLfpConsentOfferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List publishLfpConsentOfferResponseDescriptor =
    $convert.base64Decode(
        'Ch5QdWJsaXNoTGZwQ29uc2VudE9mZmVyUmVzcG9uc2USOQoFb2ZmZXIYASABKAsyIy52b2ljZS'
        '5zdG9yeS52MS5MZnBDb25zZW50T2ZmZXJWaWV3UgVvZmZlcg==');

@$core.Deprecated('Use observeLfpConsentOfferRequestDescriptor instead')
const ObserveLfpConsentOfferRequest$json = {
  '1': 'ObserveLfpConsentOfferRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {'1': 'story_id', '3': 2, '4': 1, '5': 12, '10': 'storyId'},
  ],
};

/// Descriptor for `ObserveLfpConsentOfferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeLfpConsentOfferRequestDescriptor =
    $convert.base64Decode(
        'Ch1PYnNlcnZlTGZwQ29uc2VudE9mZmVyUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
        'gEUg9wcm90b2NvbFZlcnNpb24SGQoIc3RvcnlfaWQYAiABKAxSB3N0b3J5SWQ=');

@$core.Deprecated('Use observeLfpConsentOfferResponseDescriptor instead')
const ObserveLfpConsentOfferResponse$json = {
  '1': 'ObserveLfpConsentOfferResponse',
  '2': [
    {
      '1': 'offer',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOfferView',
      '10': 'offer'
    },
  ],
};

/// Descriptor for `ObserveLfpConsentOfferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeLfpConsentOfferResponseDescriptor =
    $convert.base64Decode(
        'Ch5PYnNlcnZlTGZwQ29uc2VudE9mZmVyUmVzcG9uc2USOQoFb2ZmZXIYASABKAsyIy52b2ljZS'
        '5zdG9yeS52MS5MZnBDb25zZW50T2ZmZXJWaWV3UgVvZmZlcg==');

@$core.Deprecated('Use respondWithLfpConsentRequestDescriptor instead')
const RespondWithLfpConsentRequest$json = {
  '1': 'RespondWithLfpConsentRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'response_operation_id',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'responseOperationId'
    },
    {
      '1': 'offer',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOfferView',
      '10': 'offer'
    },
    {
      '1': 'response_type',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.LfpConsentResponseType',
      '10': 'responseType'
    },
  ],
};

/// Descriptor for `RespondWithLfpConsentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List respondWithLfpConsentRequestDescriptor = $convert.base64Decode(
    'ChxSZXNwb25kV2l0aExmcENvbnNlbnRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    'RSD3Byb3RvY29sVmVyc2lvbhIyChVyZXNwb25zZV9vcGVyYXRpb25faWQYAiABKAxSE3Jlc3Bv'
    'bnNlT3BlcmF0aW9uSWQSOQoFb2ZmZXIYAyABKAsyIy52b2ljZS5zdG9yeS52MS5MZnBDb25zZW'
    '50T2ZmZXJWaWV3UgVvZmZlchJLCg1yZXNwb25zZV90eXBlGAQgASgOMiYudm9pY2Uuc3Rvcnku'
    'djEuTGZwQ29uc2VudFJlc3BvbnNlVHlwZVIMcmVzcG9uc2VUeXBl');

@$core.Deprecated('Use respondWithLfpConsentResponseDescriptor instead')
const RespondWithLfpConsentResponse$json = {
  '1': 'RespondWithLfpConsentResponse',
};

/// Descriptor for `RespondWithLfpConsentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List respondWithLfpConsentResponseDescriptor =
    $convert.base64Decode('Ch1SZXNwb25kV2l0aExmcENvbnNlbnRSZXNwb25zZQ==');
