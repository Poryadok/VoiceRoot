// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actor.proto.

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

@$core.Deprecated('Use lfpActorActionKindDescriptor instead')
const LfpActorActionKind$json = {
  '1': 'LfpActorActionKind',
  '2': [
    {'1': 'LFP_ACTOR_ACTION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'LFP_ACTOR_ACTION_KIND_PUBLISH_OFFER', '2': 1},
    {'1': 'LFP_ACTOR_ACTION_KIND_RESPOND', '2': 2},
    {'1': 'LFP_ACTOR_ACTION_KIND_ACCEPT', '2': 3},
    {'1': 'LFP_ACTOR_ACTION_KIND_DECLINE', '2': 4},
  ],
};

/// Descriptor for `LfpActorActionKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List lfpActorActionKindDescriptor = $convert.base64Decode(
    'ChJMZnBBY3RvckFjdGlvbktpbmQSJQohTEZQX0FDVE9SX0FDVElPTl9LSU5EX1VOU1BFQ0lGSU'
    'VEEAASJwojTEZQX0FDVE9SX0FDVElPTl9LSU5EX1BVQkxJU0hfT0ZGRVIQARIhCh1MRlBfQUNU'
    'T1JfQUNUSU9OX0tJTkRfUkVTUE9ORBACEiAKHExGUF9BQ1RPUl9BQ1RJT05fS0lORF9BQ0NFUF'
    'QQAxIhCh1MRlBfQUNUT1JfQUNUSU9OX0tJTkRfREVDTElORRAE');

@$core.Deprecated('Use lfpActorKindDescriptor instead')
const LfpActorKind$json = {
  '1': 'LfpActorKind',
  '2': [
    {'1': 'LFP_ACTOR_KIND_UNSPECIFIED', '2': 0},
    {'1': 'LFP_ACTOR_KIND_HUMAN_DELEGATED_USER', '2': 1},
  ],
};

/// Descriptor for `LfpActorKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List lfpActorKindDescriptor = $convert.base64Decode(
    'CgxMZnBBY3RvcktpbmQSHgoaTEZQX0FDVE9SX0tJTkRfVU5TUEVDSUZJRUQQABInCiNMRlBfQU'
    'NUT1JfS0lORF9IVU1BTl9ERUxFR0FURURfVVNFUhAB');

@$core.Deprecated('Use lfpActorAccountTypeDescriptor instead')
const LfpActorAccountType$json = {
  '1': 'LfpActorAccountType',
  '2': [
    {'1': 'LFP_ACTOR_ACCOUNT_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'LFP_ACTOR_ACCOUNT_TYPE_REGULAR', '2': 1},
    {'1': 'LFP_ACTOR_ACCOUNT_TYPE_GUEST', '2': 2},
  ],
};

/// Descriptor for `LfpActorAccountType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List lfpActorAccountTypeDescriptor = $convert.base64Decode(
    'ChNMZnBBY3RvckFjY291bnRUeXBlEiYKIkxGUF9BQ1RPUl9BQ0NPVU5UX1RZUEVfVU5TUEVDSU'
    'ZJRUQQABIiCh5MRlBfQUNUT1JfQUNDT1VOVF9UWVBFX1JFR1VMQVIQARIgChxMRlBfQUNUT1Jf'
    'QUNDT1VOVF9UWVBFX0dVRVNUEAI=');

@$core.Deprecated('Use lfpActorProvenanceMaterialDescriptor instead')
const LfpActorProvenanceMaterial$json = {
  '1': 'LfpActorProvenanceMaterial',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'action_kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.LfpActorActionKind',
      '10': 'actionKind'
    },
    {
      '1': 'actor_kind',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.LfpActorKind',
      '10': 'actorKind'
    },
    {
      '1': 'account_type',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.story.v1.LfpActorAccountType',
      '10': 'accountType'
    },
    {'1': 'account_id', '3': 5, '4': 1, '5': 12, '10': 'accountId'},
    {'1': 'profile_id', '3': 6, '4': 1, '5': 12, '10': 'profileId'},
    {'1': 'session_epoch', '3': 7, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {
      '1': 'action_operation_id',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'actionOperationId'
    },
    {'1': 'issuer', '3': 9, '4': 1, '5': 9, '10': 'issuer'},
    {'1': 'audience', '3': 10, '4': 1, '5': 9, '10': 'audience'},
    {'1': 'full_rpc', '3': 11, '4': 1, '5': 9, '10': 'fullRpc'},
    {
      '1': 'principal_request_sha256',
      '3': 12,
      '4': 1,
      '5': 12,
      '10': 'principalRequestSha256'
    },
  ],
};

/// Descriptor for `LfpActorProvenanceMaterial`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpActorProvenanceMaterialDescriptor = $convert.base64Decode(
    'ChpMZnBBY3RvclByb3ZlbmFuY2VNYXRlcmlhbBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgEUg'
    '9wcm90b2NvbFZlcnNpb24SQwoLYWN0aW9uX2tpbmQYAiABKA4yIi52b2ljZS5zdG9yeS52MS5M'
    'ZnBBY3RvckFjdGlvbktpbmRSCmFjdGlvbktpbmQSOwoKYWN0b3Jfa2luZBgDIAEoDjIcLnZvaW'
    'NlLnN0b3J5LnYxLkxmcEFjdG9yS2luZFIJYWN0b3JLaW5kEkYKDGFjY291bnRfdHlwZRgEIAEo'
    'DjIjLnZvaWNlLnN0b3J5LnYxLkxmcEFjdG9yQWNjb3VudFR5cGVSC2FjY291bnRUeXBlEh0KCm'
    'FjY291bnRfaWQYBSABKAxSCWFjY291bnRJZBIdCgpwcm9maWxlX2lkGAYgASgMUglwcm9maWxl'
    'SWQSIwoNc2Vzc2lvbl9lcG9jaBgHIAEoA1IMc2Vzc2lvbkVwb2NoEi4KE2FjdGlvbl9vcGVyYX'
    'Rpb25faWQYCCABKAxSEWFjdGlvbk9wZXJhdGlvbklkEhYKBmlzc3VlchgJIAEoCVIGaXNzdWVy'
    'EhoKCGF1ZGllbmNlGAogASgJUghhdWRpZW5jZRIZCghmdWxsX3JwYxgLIAEoCVIHZnVsbFJwYx'
    'I4ChhwcmluY2lwYWxfcmVxdWVzdF9zaGEyNTYYDCABKAxSFnByaW5jaXBhbFJlcXVlc3RTaGEy'
    'NTY=');

@$core.Deprecated('Use lfpConsentOriginDescriptor instead')
const LfpConsentOrigin$json = {
  '1': 'LfpConsentOrigin',
  '2': [
    {'1': 'environment_id', '3': 1, '4': 1, '5': 12, '10': 'environmentId'},
    {'1': 'installation_id', '3': 2, '4': 1, '5': 12, '10': 'installationId'},
    {
      '1': 'authority_incarnation',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'authorityIncarnation'
    },
    {
      '1': 'database_binding_sha256',
      '3': 4,
      '4': 1,
      '5': 12,
      '10': 'databaseBindingSha256'
    },
  ],
};

/// Descriptor for `LfpConsentOrigin`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpConsentOriginDescriptor = $convert.base64Decode(
    'ChBMZnBDb25zZW50T3JpZ2luEiUKDmVudmlyb25tZW50X2lkGAEgASgMUg1lbnZpcm9ubWVudE'
    'lkEicKD2luc3RhbGxhdGlvbl9pZBgCIAEoDFIOaW5zdGFsbGF0aW9uSWQSMwoVYXV0aG9yaXR5'
    'X2luY2FybmF0aW9uGAMgASgMUhRhdXRob3JpdHlJbmNhcm5hdGlvbhI2ChdkYXRhYmFzZV9iaW'
    '5kaW5nX3NoYTI1NhgEIAEoDFIVZGF0YWJhc2VCaW5kaW5nU2hhMjU2');

@$core.Deprecated('Use lfpActorBoundConsentEnvelopeDescriptor instead')
const LfpActorBoundConsentEnvelope$json = {
  '1': 'LfpActorBoundConsentEnvelope',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'canonical_v1_consent',
      '3': 2,
      '4': 1,
      '5': 12,
      '10': 'canonicalV1Consent'
    },
    {'1': 'canonical_actor', '3': 3, '4': 1, '5': 12, '10': 'canonicalActor'},
    {
      '1': 'origin',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOrigin',
      '10': 'origin'
    },
    {'1': 'consent_sha256', '3': 5, '4': 1, '5': 12, '10': 'consentSha256'},
    {'1': 'actor_sha256', '3': 6, '4': 1, '5': 12, '10': 'actorSha256'},
    {
      '1': 'qualified_consent_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'qualifiedConsentSha256'
    },
  ],
};

/// Descriptor for `LfpActorBoundConsentEnvelope`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpActorBoundConsentEnvelopeDescriptor = $convert.base64Decode(
    'ChxMZnBBY3RvckJvdW5kQ29uc2VudEVudmVsb3BlEikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    'RSD3Byb3RvY29sVmVyc2lvbhIwChRjYW5vbmljYWxfdjFfY29uc2VudBgCIAEoDFISY2Fub25p'
    'Y2FsVjFDb25zZW50EicKD2Nhbm9uaWNhbF9hY3RvchgDIAEoDFIOY2Fub25pY2FsQWN0b3ISOA'
    'oGb3JpZ2luGAQgASgLMiAudm9pY2Uuc3RvcnkudjEuTGZwQ29uc2VudE9yaWdpblIGb3JpZ2lu'
    'EiUKDmNvbnNlbnRfc2hhMjU2GAUgASgMUg1jb25zZW50U2hhMjU2EiEKDGFjdG9yX3NoYTI1Nh'
    'gGIAEoDFILYWN0b3JTaGEyNTYSOAoYcXVhbGlmaWVkX2NvbnNlbnRfc2hhMjU2GAcgASgMUhZx'
    'dWFsaWZpZWRDb25zZW50U2hhMjU2');

@$core.Deprecated('Use lfpActorBoundRegistrationAckDescriptor instead')
const LfpActorBoundRegistrationAck$json = {
  '1': 'LfpActorBoundRegistrationAck',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'registration',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentRegistration',
      '10': 'registration'
    },
    {
      '1': 'origin',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOrigin',
      '10': 'origin'
    },
    {
      '1': 'qualified_consent_sha256',
      '3': 4,
      '4': 1,
      '5': 12,
      '10': 'qualifiedConsentSha256'
    },
    {
      '1': 'actor_bound_registration_sha256',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'actorBoundRegistrationSha256'
    },
  ],
};

/// Descriptor for `LfpActorBoundRegistrationAck`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lfpActorBoundRegistrationAckDescriptor = $convert.base64Decode(
    'ChxMZnBBY3RvckJvdW5kUmVnaXN0cmF0aW9uQWNrEikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    'RSD3Byb3RvY29sVmVyc2lvbhJKCgxyZWdpc3RyYXRpb24YAiABKAsyJi52b2ljZS5zdG9yeS52'
    'MS5MZnBDb25zZW50UmVnaXN0cmF0aW9uUgxyZWdpc3RyYXRpb24SOAoGb3JpZ2luGAMgASgLMi'
    'Audm9pY2Uuc3RvcnkudjEuTGZwQ29uc2VudE9yaWdpblIGb3JpZ2luEjgKGHF1YWxpZmllZF9j'
    'b25zZW50X3NoYTI1NhgEIAEoDFIWcXVhbGlmaWVkQ29uc2VudFNoYTI1NhJFCh9hY3Rvcl9ib3'
    'VuZF9yZWdpc3RyYXRpb25fc2hhMjU2GAUgASgMUhxhY3RvckJvdW5kUmVnaXN0cmF0aW9uU2hh'
    'MjU2');

@$core.Deprecated('Use registerActorBoundLfpConsentRequestDescriptor instead')
const RegisterActorBoundLfpConsentRequest$json = {
  '1': 'RegisterActorBoundLfpConsentRequest',
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
      '1': 'envelope',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpActorBoundConsentEnvelope',
      '10': 'envelope'
    },
  ],
};

/// Descriptor for `RegisterActorBoundLfpConsentRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerActorBoundLfpConsentRequestDescriptor =
    $convert.base64Decode(
        'CiNSZWdpc3RlckFjdG9yQm91bmRMZnBDb25zZW50UmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW'
        '9uGAEgASgEUg9wcm90b2NvbFZlcnNpb24SOgoZcmVnaXN0cmF0aW9uX29wZXJhdGlvbl9pZBgC'
        'IAEoDFIXcmVnaXN0cmF0aW9uT3BlcmF0aW9uSWQSSAoIZW52ZWxvcGUYAyABKAsyLC52b2ljZS'
        '5zdG9yeS52MS5MZnBBY3RvckJvdW5kQ29uc2VudEVudmVsb3BlUghlbnZlbG9wZQ==');

@$core.Deprecated('Use registerActorBoundLfpConsentResponseDescriptor instead')
const RegisterActorBoundLfpConsentResponse$json = {
  '1': 'RegisterActorBoundLfpConsentResponse',
  '2': [
    {
      '1': 'acknowledgement',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpActorBoundRegistrationAck',
      '10': 'acknowledgement'
    },
  ],
};

/// Descriptor for `RegisterActorBoundLfpConsentResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerActorBoundLfpConsentResponseDescriptor =
    $convert.base64Decode(
        'CiRSZWdpc3RlckFjdG9yQm91bmRMZnBDb25zZW50UmVzcG9uc2USVgoPYWNrbm93bGVkZ2VtZW'
        '50GAEgASgLMiwudm9pY2Uuc3RvcnkudjEuTGZwQWN0b3JCb3VuZFJlZ2lzdHJhdGlvbkFja1IP'
        'YWNrbm93bGVkZ2VtZW50');

@$core.Deprecated(
    'Use readActorBoundLfpConsentRegistrationRequestDescriptor instead')
const ReadActorBoundLfpConsentRegistrationRequest$json = {
  '1': 'ReadActorBoundLfpConsentRegistrationRequest',
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
    {'1': 'actor_sha256', '3': 5, '4': 1, '5': 12, '10': 'actorSha256'},
    {
      '1': 'qualified_consent_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'qualifiedConsentSha256'
    },
    {
      '1': 'origin',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOrigin',
      '10': 'origin'
    },
  ],
};

/// Descriptor for `ReadActorBoundLfpConsentRegistrationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    readActorBoundLfpConsentRegistrationRequestDescriptor =
    $convert.base64Decode(
        'CitSZWFkQWN0b3JCb3VuZExmcENvbnNlbnRSZWdpc3RyYXRpb25SZXF1ZXN0EikKEHByb3RvY2'
        '9sX3ZlcnNpb24YASABKARSD3Byb3RvY29sVmVyc2lvbhI6ChlyZWdpc3RyYXRpb25fb3BlcmF0'
        'aW9uX2lkGAIgASgMUhdyZWdpc3RyYXRpb25PcGVyYXRpb25JZBIdCgpjb25zZW50X2lkGAMgAS'
        'gMUgljb25zZW50SWQSJQoOY29uc2VudF9zaGEyNTYYBCABKAxSDWNvbnNlbnRTaGEyNTYSIQoM'
        'YWN0b3Jfc2hhMjU2GAUgASgMUgthY3RvclNoYTI1NhI4ChhxdWFsaWZpZWRfY29uc2VudF9zaG'
        'EyNTYYBiABKAxSFnF1YWxpZmllZENvbnNlbnRTaGEyNTYSOAoGb3JpZ2luGAcgASgLMiAudm9p'
        'Y2Uuc3RvcnkudjEuTGZwQ29uc2VudE9yaWdpblIGb3JpZ2lu');

@$core.Deprecated(
    'Use readActorBoundLfpConsentRegistrationResponseDescriptor instead')
const ReadActorBoundLfpConsentRegistrationResponse$json = {
  '1': 'ReadActorBoundLfpConsentRegistrationResponse',
  '2': [
    {
      '1': 'acknowledgement',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpActorBoundRegistrationAck',
      '10': 'acknowledgement'
    },
  ],
};

/// Descriptor for `ReadActorBoundLfpConsentRegistrationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    readActorBoundLfpConsentRegistrationResponseDescriptor =
    $convert.base64Decode(
        'CixSZWFkQWN0b3JCb3VuZExmcENvbnNlbnRSZWdpc3RyYXRpb25SZXNwb25zZRJWCg9hY2tub3'
        'dsZWRnZW1lbnQYASABKAsyLC52b2ljZS5zdG9yeS52MS5MZnBBY3RvckJvdW5kUmVnaXN0cmF0'
        'aW9uQWNrUg9hY2tub3dsZWRnZW1lbnQ=');

@$core.Deprecated(
    'Use validateActorBoundLfpConsentOfferRequestDescriptor instead')
const ValidateActorBoundLfpConsentOfferRequest$json = {
  '1': 'ValidateActorBoundLfpConsentOfferRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'offer',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.ValidateLfpConsentOfferRequest',
      '10': 'offer'
    },
    {
      '1': 'qualified_consent_sha256',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'qualifiedConsentSha256'
    },
    {'1': 'actor_sha256', '3': 4, '4': 1, '5': 12, '10': 'actorSha256'},
    {
      '1': 'origin',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOrigin',
      '10': 'origin'
    },
  ],
};

/// Descriptor for `ValidateActorBoundLfpConsentOfferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateActorBoundLfpConsentOfferRequestDescriptor =
    $convert.base64Decode(
        'CihWYWxpZGF0ZUFjdG9yQm91bmRMZnBDb25zZW50T2ZmZXJSZXF1ZXN0EikKEHByb3RvY29sX3'
        'ZlcnNpb24YASABKARSD3Byb3RvY29sVmVyc2lvbhJECgVvZmZlchgCIAEoCzIuLnZvaWNlLnN0'
        'b3J5LnYxLlZhbGlkYXRlTGZwQ29uc2VudE9mZmVyUmVxdWVzdFIFb2ZmZXISOAoYcXVhbGlmaW'
        'VkX2NvbnNlbnRfc2hhMjU2GAMgASgMUhZxdWFsaWZpZWRDb25zZW50U2hhMjU2EiEKDGFjdG9y'
        'X3NoYTI1NhgEIAEoDFILYWN0b3JTaGEyNTYSOAoGb3JpZ2luGAUgASgLMiAudm9pY2Uuc3Rvcn'
        'kudjEuTGZwQ29uc2VudE9yaWdpblIGb3JpZ2lu');

@$core.Deprecated(
    'Use validateActorBoundLfpConsentOfferResponseDescriptor instead')
const ValidateActorBoundLfpConsentOfferResponse$json = {
  '1': 'ValidateActorBoundLfpConsentOfferResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 4, '10': 'protocolVersion'},
    {
      '1': 'offer',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.ValidateLfpConsentOfferResponse',
      '10': 'offer'
    },
    {
      '1': 'qualified_consent_sha256',
      '3': 3,
      '4': 1,
      '5': 12,
      '10': 'qualifiedConsentSha256'
    },
    {'1': 'actor_sha256', '3': 4, '4': 1, '5': 12, '10': 'actorSha256'},
    {
      '1': 'origin',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.story.v1.LfpConsentOrigin',
      '10': 'origin'
    },
    {
      '1': 'canonical_criteria_json',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'canonicalCriteriaJson'
    },
  ],
};

/// Descriptor for `ValidateActorBoundLfpConsentOfferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateActorBoundLfpConsentOfferResponseDescriptor = $convert.base64Decode(
    'CilWYWxpZGF0ZUFjdG9yQm91bmRMZnBDb25zZW50T2ZmZXJSZXNwb25zZRIpChBwcm90b2NvbF'
    '92ZXJzaW9uGAEgASgEUg9wcm90b2NvbFZlcnNpb24SRQoFb2ZmZXIYAiABKAsyLy52b2ljZS5z'
    'dG9yeS52MS5WYWxpZGF0ZUxmcENvbnNlbnRPZmZlclJlc3BvbnNlUgVvZmZlchI4ChhxdWFsaW'
    'ZpZWRfY29uc2VudF9zaGEyNTYYAyABKAxSFnF1YWxpZmllZENvbnNlbnRTaGEyNTYSIQoMYWN0'
    'b3Jfc2hhMjU2GAQgASgMUgthY3RvclNoYTI1NhI4CgZvcmlnaW4YBSABKAsyIC52b2ljZS5zdG'
    '9yeS52MS5MZnBDb25zZW50T3JpZ2luUgZvcmlnaW4SNgoXY2Fub25pY2FsX2NyaXRlcmlhX2pz'
    'b24YBiABKAxSFWNhbm9uaWNhbENyaXRlcmlhSnNvbg==');
