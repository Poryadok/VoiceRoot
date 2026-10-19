// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_consent.proto.

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

@$core.Deprecated('Use lfpConsentResponseTypeDescriptor instead')
const LfpConsentResponseType$json = {
  '1': 'LfpConsentResponseType',
  '2': [
    {'1': 'LFP_CONSENT_RESPONSE_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'LFP_CONSENT_RESPONSE_TYPE_JOIN', '2': 1},
    {'1': 'LFP_CONSENT_RESPONSE_TYPE_INVITE', '2': 2},
  ],
};

/// Descriptor for `LfpConsentResponseType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List lfpConsentResponseTypeDescriptor = $convert.base64Decode(
    'ChZMZnBDb25zZW50UmVzcG9uc2VUeXBlEikKJUxGUF9DT05TRU5UX1JFU1BPTlNFX1RZUEVfVU'
    '5TUEVDSUZJRUQQABIiCh5MRlBfQ09OU0VOVF9SRVNQT05TRV9UWVBFX0pPSU4QARIkCiBMRlBf'
    'Q09OU0VOVF9SRVNQT05TRV9UWVBFX0lOVklURRAC');

@$core.Deprecated('Use lfpConsentMaterialDescriptor instead')
const LfpConsentMaterial$json = {
  '1': 'LfpConsentMaterial',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {'1': 'consent_id', '3': 2, '4': 1, '5': 12, '10': 'consentId'},
    {
      '1': 'response_operation_id',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'responseOperationId'
    },
    {'1': 'story_id', '3': 4, '4': 1, '5': 12, '10': 'storyId'},
    {
      '1': 'story_incarnation',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'storyIncarnation'
    },
    {'1': 'offer_revision', '3': 6, '4': 1, '5': 4, '10': 'offerRevision'},
    {
      '1': 'author_profile_id',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'authorProfileId'
    },
    {
      '1': 'responder_account_id',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'responderAccountId'
    },
    {
      '1': 'responder_profile_id',
      '3': 9,
      '4': 1,
      '5': 12,
      '10': 'responderProfileId'
    },
    {
      '1': 'responder_session_epoch',
      '3': 10,
      '4': 1,
      '5': 3,
      '10': 'responderSessionEpoch'
    },
    {
      '1': 'response_type',
      '3': 11,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.LfpConsentResponseType',
      '10': 'responseType'
    },
    {'1': 'criteria_sha256', '3': 12, '4': 1, '5': 12, '10': 'criteriaSha256'},
    {
      '1': 'offer_payload_sha256',
      '3': 13,
      '4': 1,
      '5': 12,
      '10': 'offerPayloadSha256'
    },
    {
      '1': 'created_at_unix_millis',
      '3': 14,
      '4': 1,
      '5': 3,
      '10': 'createdAtUnixMillis'
    },
    {
      '1': 'offer_expires_at_unix_millis',
      '3': 15,
      '4': 1,
      '5': 3,
      '10': 'offerExpiresAtUnixMillis'
    },
    {
      '1': 'principal_request_sha256',
      '3': 16,
      '4': 1,
      '5': 12,
      '10': 'principalRequestSha256'
    },
  ],
};

/// Descriptor for `LfpConsentMaterial`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpConsentMaterialDescriptor = $convert.base64Decode(
    'ChJMZnBDb25zZW50TWF0ZXJpYWwSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoBFIPcHJvdG9jb2'
    'xWZXJzaW9uEh0KCmNvbnNlbnRfaWQYAiABKAxSCWNvbnNlbnRJZBIyChVyZXNwb25zZV9vcGVy'
    'YXRpb25faWQYAyABKAxSE3Jlc3BvbnNlT3BlcmF0aW9uSWQSGQoIc3RvcnlfaWQYBCABKAxSB3'
    'N0b3J5SWQSKwoRc3RvcnlfaW5jYXJuYXRpb24YBSABKAxSEHN0b3J5SW5jYXJuYXRpb24SJQoO'
    'b2ZmZXJfcmV2aXNpb24YBiABKARSDW9mZmVyUmV2aXNpb24SKgoRYXV0aG9yX3Byb2ZpbGVfaW'
    'QYByABKAxSD2F1dGhvclByb2ZpbGVJZBIwChRyZXNwb25kZXJfYWNjb3VudF9pZBgIIAEoDFIS'
    'cmVzcG9uZGVyQWNjb3VudElkEjAKFHJlc3BvbmRlcl9wcm9maWxlX2lkGAkgASgMUhJyZXNwb2'
    '5kZXJQcm9maWxlSWQSNgoXcmVzcG9uZGVyX3Nlc3Npb25fZXBvY2gYCiABKANSFXJlc3BvbmRl'
    'clNlc3Npb25FcG9jaBJLCg1yZXNwb25zZV90eXBlGAsgASgOMiYudm9pY2Uuc3RvcnkudjEuTG'
    'ZwQ29uc2VudFJlc3BvbnNlVHlwZVIMcmVzcG9uc2VUeXBlEicKD2NyaXRlcmlhX3NoYTI1NhgM'
    'IAEoDFIOY3JpdGVyaWFTaGEyNTYSMAoUb2ZmZXJfcGF5bG9hZF9zaGEyNTYYDSABKAxSEm9mZm'
    'VyUGF5bG9hZFNoYTI1NhIzChZjcmVhdGVkX2F0X3VuaXhfbWlsbGlzGA4gASgDUhNjcmVhdGVk'
    'QXRVbml4TWlsbGlzEj4KHG9mZmVyX2V4cGlyZXNfYXRfdW5peF9taWxsaXMYDyABKANSGG9mZm'
    'VyRXhwaXJlc0F0VW5peE1pbGxpcxI4ChhwcmluY2lwYWxfcmVxdWVzdF9zaGEyNTYYECABKAxS'
    'FnByaW5jaXBhbFJlcXVlc3RTaGEyNTY=');

@$core.Deprecated('Use registerLfpConsentResponseDescriptor instead')
const RegisterLfpConsentResponse$json = {
  '1': 'RegisterLfpConsentResponse',
  '2': [
    {
      '1': 'registration',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentRegistration',
      '10': 'registration'
    },
  ],
};

/// Descriptor for `RegisterLfpConsentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerLfpConsentResponseDescriptor =
    $convert.base64Decode(
        'ChpSZWdpc3RlckxmcENvbnNlbnRSZXNwb25zZRJKCgxyZWdpc3RyYXRpb24YASABKAsyJi52b2'
        'ljZS5zdG9yeS52MS5MZnBDb25zZW50UmVnaXN0cmF0aW9uUgxyZWdpc3RyYXRpb24=');

@$core.Deprecated('Use readLfpConsentRegistrationResponseDescriptor instead')
const ReadLfpConsentRegistrationResponse$json = {
  '1': 'ReadLfpConsentRegistrationResponse',
  '2': [
    {
      '1': 'registration',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentRegistration',
      '10': 'registration'
    },
  ],
};

/// Descriptor for `ReadLfpConsentRegistrationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readLfpConsentRegistrationResponseDescriptor =
    $convert.base64Decode(
        'CiJSZWFkTGZwQ29uc2VudFJlZ2lzdHJhdGlvblJlc3BvbnNlEkoKDHJlZ2lzdHJhdGlvbhgBIA'
        'EoCzImLnZvaWNlLnN0b3J5LnYxLkxmcENvbnNlbnRSZWdpc3RyYXRpb25SDHJlZ2lzdHJhdGlv'
        'bg==');

@$core.Deprecated('Use registerLfpConsentRequestDescriptor instead')
const RegisterLfpConsentRequest$json = {
  '1': 'RegisterLfpConsentRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'registration_operation_id',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'registrationOperationId'
    },
    {
      '1': 'canonical_consent',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'canonicalConsent'
    },
    {'1': 'consent_sha256', '3': 4, '4': 1, '5': 12, '10': 'consentSha256'},
  ],
};

/// Descriptor for `RegisterLfpConsentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerLfpConsentRequestDescriptor = $convert.base64Decode(
    'ChlSZWdpc3RlckxmcENvbnNlbnRSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKARSD3'
    'Byb3RvY29sVmVyc2lvbhI6ChlyZWdpc3RyYXRpb25fb3BlcmF0aW9uX2lkGAIgASgMUhdyZWdp'
    'c3RyYXRpb25PcGVyYXRpb25JZBIrChFjYW5vbmljYWxfY29uc2VudBgDIAEoDFIQY2Fub25pY2'
    'FsQ29uc2VudBIlCg5jb25zZW50X3NoYTI1NhgEIAEoDFINY29uc2VudFNoYTI1Ng==');

@$core.Deprecated('Use readLfpConsentRegistrationRequestDescriptor instead')
const ReadLfpConsentRegistrationRequest$json = {
  '1': 'ReadLfpConsentRegistrationRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'registration_operation_id',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'registrationOperationId'
    },
    {'1': 'consent_id', '3': 3, '4': 1, '5': 12, '10': 'consentId'},
    {'1': 'consent_sha256', '3': 4, '4': 1, '5': 12, '10': 'consentSha256'},
  ],
};

/// Descriptor for `ReadLfpConsentRegistrationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readLfpConsentRegistrationRequestDescriptor =
    $convert.base64Decode(
        'CiFSZWFkTGZwQ29uc2VudFJlZ2lzdHJhdGlvblJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbh'
        'gBIAEoBFIPcHJvdG9jb2xWZXJzaW9uEjoKGXJlZ2lzdHJhdGlvbl9vcGVyYXRpb25faWQYAiAB'
        'KAxSF3JlZ2lzdHJhdGlvbk9wZXJhdGlvbklkEh0KCmNvbnNlbnRfaWQYAyABKAxSCWNvbnNlbn'
        'RJZBIlCg5jb25zZW50X3NoYTI1NhgEIAEoDFINY29uc2VudFNoYTI1Ng==');

@$core.Deprecated('Use lfpConsentRegistrationDescriptor instead')
const LfpConsentRegistration$json = {
  '1': 'LfpConsentRegistration',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'registration_operation_id',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'registrationOperationId'
    },
    {'1': 'consent_id', '3': 3, '4': 1, '5': 12, '10': 'consentId'},
    {'1': 'consent_sha256', '3': 4, '4': 1, '5': 12, '10': 'consentSha256'},
    {'1': 'registration_id', '3': 5, '4': 1, '5': 12, '10': 'registrationId'},
    {
      '1': 'registration_generation',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'registrationGeneration'
    },
    {
      '1': 'captured_search_intent_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'capturedSearchIntentSha256'
    },
    {
      '1': 'registration_sha256',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'registrationSha256'
    },
  ],
};

/// Descriptor for `LfpConsentRegistration`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpConsentRegistrationDescriptor = $convert.base64Decode(
    'ChZMZnBDb25zZW50UmVnaXN0cmF0aW9uEikKEHByb3RvY29sX3ZlcnNpb24YASABKARSD3Byb3'
    'RvY29sVmVyc2lvbhI6ChlyZWdpc3RyYXRpb25fb3BlcmF0aW9uX2lkGAIgASgMUhdyZWdpc3Ry'
    'YXRpb25PcGVyYXRpb25JZBIdCgpjb25zZW50X2lkGAMgASgMUgljb25zZW50SWQSJQoOY29uc2'
    'VudF9zaGEyNTYYBCABKAxSDWNvbnNlbnRTaGEyNTYSJwoPcmVnaXN0cmF0aW9uX2lkGAUgASgM'
    'Ug5yZWdpc3RyYXRpb25JZBI3ChdyZWdpc3RyYXRpb25fZ2VuZXJhdGlvbhgGIAEoBFIWcmVnaX'
    'N0cmF0aW9uR2VuZXJhdGlvbhJBCh1jYXB0dXJlZF9zZWFyY2hfaW50ZW50X3NoYTI1NhgHIAEo'
    'DFIaY2FwdHVyZWRTZWFyY2hJbnRlbnRTaGEyNTYSLwoTcmVnaXN0cmF0aW9uX3NoYTI1NhgIIA'
    'EoDFIScmVnaXN0cmF0aW9uU2hhMjU2');

@$core.Deprecated('Use validateLfpConsentOfferRequestDescriptor instead')
const ValidateLfpConsentOfferRequest$json = {
  '1': 'ValidateLfpConsentOfferRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {'1': 'consent_id', '3': 2, '4': 1, '5': 12, '10': 'consentId'},
    {'1': 'consent_sha256', '3': 3, '4': 1, '5': 12, '10': 'consentSha256'},
    {'1': 'story_id', '3': 4, '4': 1, '5': 12, '10': 'storyId'},
    {
      '1': 'story_incarnation',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'storyIncarnation'
    },
    {'1': 'offer_revision', '3': 6, '4': 1, '5': 4, '10': 'offerRevision'},
    {
      '1': 'offer_payload_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'offerPayloadSha256'
    },
    {'1': 'criteria_sha256', '3': 8, '4': 1, '5': 12, '10': 'criteriaSha256'},
  ],
};

/// Descriptor for `ValidateLfpConsentOfferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateLfpConsentOfferRequestDescriptor = $convert.base64Decode(
    'Ch5WYWxpZGF0ZUxmcENvbnNlbnRPZmZlclJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIA'
    'EoBFIPcHJvdG9jb2xWZXJzaW9uEh0KCmNvbnNlbnRfaWQYAiABKAxSCWNvbnNlbnRJZBIlCg5j'
    'b25zZW50X3NoYTI1NhgDIAEoDFINY29uc2VudFNoYTI1NhIZCghzdG9yeV9pZBgEIAEoDFIHc3'
    'RvcnlJZBIrChFzdG9yeV9pbmNhcm5hdGlvbhgFIAEoDFIQc3RvcnlJbmNhcm5hdGlvbhIlCg5v'
    'ZmZlcl9yZXZpc2lvbhgGIAEoBFINb2ZmZXJSZXZpc2lvbhIwChRvZmZlcl9wYXlsb2FkX3NoYT'
    'I1NhgHIAEoDFISb2ZmZXJQYXlsb2FkU2hhMjU2EicKD2NyaXRlcmlhX3NoYTI1NhgIIAEoDFIO'
    'Y3JpdGVyaWFTaGEyNTY=');

@$core.Deprecated('Use validateLfpConsentOfferResponseDescriptor instead')
const ValidateLfpConsentOfferResponse$json = {
  '1': 'ValidateLfpConsentOfferResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {'1': 'consent_id', '3': 2, '4': 1, '5': 12, '10': 'consentId'},
    {'1': 'consent_sha256', '3': 3, '4': 1, '5': 12, '10': 'consentSha256'},
    {'1': 'story_id', '3': 4, '4': 1, '5': 12, '10': 'storyId'},
    {
      '1': 'story_incarnation',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'storyIncarnation'
    },
    {'1': 'offer_revision', '3': 6, '4': 1, '5': 4, '10': 'offerRevision'},
    {
      '1': 'offer_payload_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'offerPayloadSha256'
    },
    {'1': 'criteria_sha256', '3': 8, '4': 1, '5': 12, '10': 'criteriaSha256'},
    {
      '1': 'offer_expires_at_unix_millis',
      '3': 9,
      '4': 1,
      '5': 3,
      '10': 'offerExpiresAtUnixMillis'
    },
  ],
};

/// Descriptor for `ValidateLfpConsentOfferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateLfpConsentOfferResponseDescriptor = $convert.base64Decode(
    'Ch9WYWxpZGF0ZUxmcENvbnNlbnRPZmZlclJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb24YAS'
    'ABKARSD3Byb3RvY29sVmVyc2lvbhIdCgpjb25zZW50X2lkGAIgASgMUgljb25zZW50SWQSJQoO'
    'Y29uc2VudF9zaGEyNTYYAyABKAxSDWNvbnNlbnRTaGEyNTYSGQoIc3RvcnlfaWQYBCABKAxSB3'
    'N0b3J5SWQSKwoRc3RvcnlfaW5jYXJuYXRpb24YBSABKAxSEHN0b3J5SW5jYXJuYXRpb24SJQoO'
    'b2ZmZXJfcmV2aXNpb24YBiABKARSDW9mZmVyUmV2aXNpb24SMAoUb2ZmZXJfcGF5bG9hZF9zaG'
    'EyNTYYByABKAxSEm9mZmVyUGF5bG9hZFNoYTI1NhInCg9jcml0ZXJpYV9zaGEyNTYYCCABKAxS'
    'DmNyaXRlcmlhU2hhMjU2Ej4KHG9mZmVyX2V4cGlyZXNfYXRfdW5peF9taWxsaXMYCSABKANSGG'
    '9mZmVyRXhwaXJlc0F0VW5peE1pbGxpcw==');
