// This is a generated file - do not edit.
//
// Generated from voice/space/v1/member_admission.proto.

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

@$core.Deprecated('Use memberAdmissionLifecycleStateDescriptor instead')
const MemberAdmissionLifecycleState$json = {
  '1': 'MemberAdmissionLifecycleState',
  '2': [
    {'1': 'MEMBER_ADMISSION_LIFECYCLE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE', '2': 1},
    {'1': 'MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN', '2': 2},
    {'1': 'MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED', '2': 3},
    {'1': 'MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED', '2': 4},
  ],
};

/// Descriptor for `MemberAdmissionLifecycleState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List memberAdmissionLifecycleStateDescriptor = $convert.base64Decode(
    'Ch1NZW1iZXJBZG1pc3Npb25MaWZlY3ljbGVTdGF0ZRIwCixNRU1CRVJfQURNSVNTSU9OX0xJRk'
    'VDWUNMRV9TVEFURV9VTlNQRUNJRklFRBAAEikKJU1FTUJFUl9BRE1JU1NJT05fTElGRUNZQ0xF'
    'X1NUQVRFX0xJVkUQARIrCidNRU1CRVJfQURNSVNTSU9OX0xJRkVDWUNMRV9TVEFURV9GUk9aRU'
    '4QAhIyCi5NRU1CRVJfQURNSVNTSU9OX0xJRkVDWUNMRV9TVEFURV9QVVJHRV9ERUNJREVEEAMS'
    'LAooTUVNQkVSX0FETUlTU0lPTl9MSUZFQ1lDTEVfU1RBVEVfUkVUSVJFRBAE');

@$core.Deprecated('Use memberAdmissionOwnershipStateDescriptor instead')
const MemberAdmissionOwnershipState$json = {
  '1': 'MemberAdmissionOwnershipState',
  '2': [
    {'1': 'MEMBER_ADMISSION_OWNERSHIP_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE', '2': 1},
    {'1': 'MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED', '2': 2},
    {'1': 'MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED', '2': 3},
  ],
};

/// Descriptor for `MemberAdmissionOwnershipState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List memberAdmissionOwnershipStateDescriptor = $convert.base64Decode(
    'Ch1NZW1iZXJBZG1pc3Npb25Pd25lcnNoaXBTdGF0ZRIwCixNRU1CRVJfQURNSVNTSU9OX09XTk'
    'VSU0hJUF9TVEFURV9VTlNQRUNJRklFRBAAEi4KKk1FTUJFUl9BRE1JU1NJT05fT1dORVJTSElQ'
    'X1NUQVRFX0FWQUlMQUJMRRABEi0KKU1FTUJFUl9BRE1JU1NJT05fT1dORVJTSElQX1NUQVRFX1'
    'BSRVBBUkVEEAISLAooTUVNQkVSX0FETUlTU0lPTl9PV05FUlNISVBfU1RBVEVfUkVUSVJFRBAD');

@$core.Deprecated('Use memberAdmissionSourceStateDescriptor instead')
const MemberAdmissionSourceState$json = {
  '1': 'MemberAdmissionSourceState',
  '2': [
    {'1': 'MEMBER_ADMISSION_SOURCE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'MEMBER_ADMISSION_SOURCE_STATE_NONE', '2': 1},
    {'1': 'MEMBER_ADMISSION_SOURCE_STATE_PREPARING', '2': 2},
    {'1': 'MEMBER_ADMISSION_SOURCE_STATE_ACTIVE', '2': 3},
    {'1': 'MEMBER_ADMISSION_SOURCE_STATE_RETIRED', '2': 4},
  ],
};

/// Descriptor for `MemberAdmissionSourceState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List memberAdmissionSourceStateDescriptor = $convert.base64Decode(
    'ChpNZW1iZXJBZG1pc3Npb25Tb3VyY2VTdGF0ZRItCilNRU1CRVJfQURNSVNTSU9OX1NPVVJDRV'
    '9TVEFURV9VTlNQRUNJRklFRBAAEiYKIk1FTUJFUl9BRE1JU1NJT05fU09VUkNFX1NUQVRFX05P'
    'TkUQARIrCidNRU1CRVJfQURNSVNTSU9OX1NPVVJDRV9TVEFURV9QUkVQQVJJTkcQAhIoCiRNRU'
    '1CRVJfQURNSVNTSU9OX1NPVVJDRV9TVEFURV9BQ1RJVkUQAxIpCiVNRU1CRVJfQURNSVNTSU9O'
    'X1NPVVJDRV9TVEFURV9SRVRJUkVEEAQ=');

@$core.Deprecated('Use memberAdmissionPublicationKindDescriptor instead')
const MemberAdmissionPublicationKind$json = {
  '1': 'MemberAdmissionPublicationKind',
  '2': [
    {'1': 'MEMBER_ADMISSION_PUBLICATION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY', '2': 1},
    {'1': 'MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED', '2': 2},
    {'1': 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE', '2': 3},
    {
      '1': 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE',
      '2': 4
    },
    {
      '1': 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY',
      '2': 5
    },
    {'1': 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL', '2': 6},
  ],
};

/// Descriptor for `MemberAdmissionPublicationKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List memberAdmissionPublicationKindDescriptor = $convert.base64Decode(
    'Ch5NZW1iZXJBZG1pc3Npb25QdWJsaWNhdGlvbktpbmQSMQotTUVNQkVSX0FETUlTU0lPTl9QVU'
    'JMSUNBVElPTl9LSU5EX1VOU1BFQ0lGSUVEEAASOgo2TUVNQkVSX0FETUlTU0lPTl9QVUJMSUNB'
    'VElPTl9LSU5EX0FETUlTU0lPTl9ST0xFX1JFQURZEAESNQoxTUVNQkVSX0FETUlTU0lPTl9QVU'
    'JMSUNBVElPTl9LSU5EX0xFR0FDWV9NSUdSQVRFRBACEjkKNU1FTUJFUl9BRE1JU1NJT05fUFVC'
    'TElDQVRJT05fS0lORF9OQVRJVkVfT1dORVJfQ1JFQVRFEAMSQwo/TUVNQkVSX0FETUlTU0lPTl'
    '9QVUJMSUNBVElPTl9LSU5EX05BVElWRV9DT01NVU5JVFlfT1dORVJfQ1JFQVRFEAQSRQpBTUVN'
    'QkVSX0FETUlTU0lPTl9QVUJMSUNBVElPTl9LSU5EX05BVElWRV9DT01NVU5JVFlfT1dORVJfUk'
    'VDT1ZFUlkQBRI4CjRNRU1CRVJfQURNSVNTSU9OX1BVQkxJQ0FUSU9OX0tJTkRfTkFUSVZFX0JP'
    'VF9JTlNUQUxMEAY=');

@$core.Deprecated('Use getMemberAdmissionAuthorityRequestDescriptor instead')
const GetMemberAdmissionAuthorityRequest$json = {
  '1': 'GetMemberAdmissionAuthorityRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'expected_generation',
      '3': 4,
      '4': 1,
      '5': 4,
      '9': 0,
      '10': 'expectedGeneration',
      '17': true
    },
    {
      '1': 'expected_admission_operation_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'expectedAdmissionOperationId',
      '17': true
    },
    {
      '1': 'chat_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'chatId',
      '17': true
    },
    {
      '1': 'bot_message_send_binding',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.BotMessageSendBinding',
      '9': 3,
      '10': 'botMessageSendBinding',
      '17': true
    },
  ],
  '8': [
    {'1': '_expected_generation'},
    {'1': '_expected_admission_operation_id'},
    {'1': '_chat_id'},
    {'1': '_bot_message_send_binding'},
  ],
};

/// Descriptor for `GetMemberAdmissionAuthorityRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberAdmissionAuthorityRequestDescriptor = $convert.base64Decode(
    'CiJHZXRNZW1iZXJBZG1pc3Npb25BdXRob3JpdHlSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBIdCgpw'
    'cm9maWxlX2lkGAMgASgJUglwcm9maWxlSWQSNAoTZXhwZWN0ZWRfZ2VuZXJhdGlvbhgEIAEoBE'
    'gAUhJleHBlY3RlZEdlbmVyYXRpb26IAQESSgofZXhwZWN0ZWRfYWRtaXNzaW9uX29wZXJhdGlv'
    'bl9pZBgFIAEoCUgBUhxleHBlY3RlZEFkbWlzc2lvbk9wZXJhdGlvbklkiAEBEhwKB2NoYXRfaW'
    'QYBiABKAlIAlIGY2hhdElkiAEBEmQKGGJvdF9tZXNzYWdlX3NlbmRfYmluZGluZxgHIAEoCzIm'
    'LnZvaWNlLmNvbW1vbi52MS5Cb3RNZXNzYWdlU2VuZEJpbmRpbmdIA1IVYm90TWVzc2FnZVNlbm'
    'RCaW5kaW5niAEBQhYKFF9leHBlY3RlZF9nZW5lcmF0aW9uQiIKIF9leHBlY3RlZF9hZG1pc3Np'
    'b25fb3BlcmF0aW9uX2lkQgoKCF9jaGF0X2lkQhsKGV9ib3RfbWVzc2FnZV9zZW5kX2JpbmRpbm'
    'c=');

@$core.Deprecated('Use nativeMembershipPublicationBindingDescriptor instead')
const NativeMembershipPublicationBinding$json = {
  '1': 'NativeMembershipPublicationBinding',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'publication_operation_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'publicationOperationId'
    },
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 4, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'ordinary_generation',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'ordinaryGeneration'
    },
    {
      '1': 'kind',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionPublicationKind',
      '10': 'kind'
    },
    {
      '1': 'initiating_service',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'initiatingService'
    },
    {
      '1': 'initiating_account_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'initiatingAccountId',
      '17': true
    },
    {
      '1': 'initiating_profile_id',
      '3': 9,
      '4': 1,
      '5': 9,
      '10': 'initiatingProfileId'
    },
    {'1': 'request_sha256', '3': 10, '4': 1, '5': 12, '10': 'requestSha256'},
    {
      '1': 'producer_operation_id',
      '3': 11,
      '4': 1,
      '5': 9,
      '10': 'producerOperationId'
    },
    {
      '1': 'producer_evidence_sha256',
      '3': 12,
      '4': 1,
      '5': 12,
      '10': 'producerEvidenceSha256'
    },
    {
      '1': 'application_id',
      '3': 13,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'applicationId',
      '17': true
    },
    {
      '1': 'environment_id',
      '3': 14,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'environmentId',
      '17': true
    },
    {
      '1': 'corporation_key',
      '3': 15,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'corporationKey',
      '17': true
    },
    {
      '1': 'previous_owner_profile_id',
      '3': 16,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'previousOwnerProfileId',
      '17': true
    },
    {
      '1': 'previous_owner_account_id',
      '3': 17,
      '4': 1,
      '5': 9,
      '9': 5,
      '10': 'previousOwnerAccountId',
      '17': true
    },
    {
      '1': 'expected_owner_generation',
      '3': 18,
      '4': 1,
      '5': 4,
      '9': 6,
      '10': 'expectedOwnerGeneration',
      '17': true
    },
    {
      '1': 'result_owner_generation',
      '3': 19,
      '4': 1,
      '5': 4,
      '9': 7,
      '10': 'resultOwnerGeneration',
      '17': true
    },
    {'1': 'bot_id', '3': 20, '4': 1, '5': 9, '9': 8, '10': 'botId', '17': true},
    {
      '1': 'installation_id',
      '3': 21,
      '4': 1,
      '5': 9,
      '9': 9,
      '10': 'installationId',
      '17': true
    },
    {
      '1': 'published_at',
      '3': 22,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'publishedAt'
    },
  ],
  '8': [
    {'1': '_initiating_account_id'},
    {'1': '_application_id'},
    {'1': '_environment_id'},
    {'1': '_corporation_key'},
    {'1': '_previous_owner_profile_id'},
    {'1': '_previous_owner_account_id'},
    {'1': '_expected_owner_generation'},
    {'1': '_result_owner_generation'},
    {'1': '_bot_id'},
    {'1': '_installation_id'},
  ],
};

/// Descriptor for `NativeMembershipPublicationBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List nativeMembershipPublicationBindingDescriptor = $convert.base64Decode(
    'CiJOYXRpdmVNZW1iZXJzaGlwUHVibGljYXRpb25CaW5kaW5nEikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhI4ChhwdWJsaWNhdGlvbl9vcGVyYXRpb25faWQYAiAB'
    'KAlSFnB1YmxpY2F0aW9uT3BlcmF0aW9uSWQSGQoIc3BhY2VfaWQYAyABKAlSB3NwYWNlSWQSHQ'
    'oKcHJvZmlsZV9pZBgEIAEoCVIJcHJvZmlsZUlkEi8KE29yZGluYXJ5X2dlbmVyYXRpb24YBSAB'
    'KARSEm9yZGluYXJ5R2VuZXJhdGlvbhJCCgRraW5kGAYgASgOMi4udm9pY2Uuc3BhY2UudjEuTW'
    'VtYmVyQWRtaXNzaW9uUHVibGljYXRpb25LaW5kUgRraW5kEi0KEmluaXRpYXRpbmdfc2Vydmlj'
    'ZRgHIAEoCVIRaW5pdGlhdGluZ1NlcnZpY2USNwoVaW5pdGlhdGluZ19hY2NvdW50X2lkGAggAS'
    'gJSABSE2luaXRpYXRpbmdBY2NvdW50SWSIAQESMgoVaW5pdGlhdGluZ19wcm9maWxlX2lkGAkg'
    'ASgJUhNpbml0aWF0aW5nUHJvZmlsZUlkEiUKDnJlcXVlc3Rfc2hhMjU2GAogASgMUg1yZXF1ZX'
    'N0U2hhMjU2EjIKFXByb2R1Y2VyX29wZXJhdGlvbl9pZBgLIAEoCVITcHJvZHVjZXJPcGVyYXRp'
    'b25JZBI4Chhwcm9kdWNlcl9ldmlkZW5jZV9zaGEyNTYYDCABKAxSFnByb2R1Y2VyRXZpZGVuY2'
    'VTaGEyNTYSKgoOYXBwbGljYXRpb25faWQYDSABKAlIAVINYXBwbGljYXRpb25JZIgBARIqCg5l'
    'bnZpcm9ubWVudF9pZBgOIAEoCUgCUg1lbnZpcm9ubWVudElkiAEBEiwKD2NvcnBvcmF0aW9uX2'
    'tleRgPIAEoCUgDUg5jb3Jwb3JhdGlvbktleYgBARI+ChlwcmV2aW91c19vd25lcl9wcm9maWxl'
    'X2lkGBAgASgJSARSFnByZXZpb3VzT3duZXJQcm9maWxlSWSIAQESPgoZcHJldmlvdXNfb3duZX'
    'JfYWNjb3VudF9pZBgRIAEoCUgFUhZwcmV2aW91c093bmVyQWNjb3VudElkiAEBEj8KGWV4cGVj'
    'dGVkX293bmVyX2dlbmVyYXRpb24YEiABKARIBlIXZXhwZWN0ZWRPd25lckdlbmVyYXRpb26IAQ'
    'ESOwoXcmVzdWx0X293bmVyX2dlbmVyYXRpb24YEyABKARIB1IVcmVzdWx0T3duZXJHZW5lcmF0'
    'aW9uiAEBEhoKBmJvdF9pZBgUIAEoCUgIUgVib3RJZIgBARIsCg9pbnN0YWxsYXRpb25faWQYFS'
    'ABKAlICVIOaW5zdGFsbGF0aW9uSWSIAQESPQoMcHVibGlzaGVkX2F0GBYgASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcFILcHVibGlzaGVkQXRCGAoWX2luaXRpYXRpbmdfYWNjb3VudF'
    '9pZEIRCg9fYXBwbGljYXRpb25faWRCEQoPX2Vudmlyb25tZW50X2lkQhIKEF9jb3Jwb3JhdGlv'
    'bl9rZXlCHAoaX3ByZXZpb3VzX293bmVyX3Byb2ZpbGVfaWRCHAoaX3ByZXZpb3VzX293bmVyX2'
    'FjY291bnRfaWRCHAoaX2V4cGVjdGVkX293bmVyX2dlbmVyYXRpb25CGgoYX3Jlc3VsdF9vd25l'
    'cl9nZW5lcmF0aW9uQgkKB19ib3RfaWRCEgoQX2luc3RhbGxhdGlvbl9pZA==');

@$core.Deprecated('Use memberAdmissionNativePublicationProofDescriptor instead')
const MemberAdmissionNativePublicationProof$json = {
  '1': 'MemberAdmissionNativePublicationProof',
  '2': [
    {
      '1': 'kind',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionPublicationKind',
      '10': 'kind'
    },
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'binding_sha256', '3': 3, '4': 1, '5': 12, '10': 'bindingSha256'},
  ],
};

/// Descriptor for `MemberAdmissionNativePublicationProof`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionNativePublicationProofDescriptor =
    $convert.base64Decode(
        'CiVNZW1iZXJBZG1pc3Npb25OYXRpdmVQdWJsaWNhdGlvblByb29mEkIKBGtpbmQYASABKA4yLi'
        '52b2ljZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25QdWJsaWNhdGlvbktpbmRSBGtpbmQSIQoM'
        'b3BlcmF0aW9uX2lkGAIgASgJUgtvcGVyYXRpb25JZBIlCg5iaW5kaW5nX3NoYTI1NhgDIAEoDF'
        'INYmluZGluZ1NoYTI1Ng==');

@$core.Deprecated('Use memberAdmissionLifecycleAuthorityDescriptor instead')
const MemberAdmissionLifecycleAuthority$json = {
  '1': 'MemberAdmissionLifecycleAuthority',
  '2': [
    {'1': 'generation', '3': 1, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionLifecycleState',
      '10': 'state'
    },
  ],
};

/// Descriptor for `MemberAdmissionLifecycleAuthority`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionLifecycleAuthorityDescriptor =
    $convert.base64Decode(
        'CiFNZW1iZXJBZG1pc3Npb25MaWZlY3ljbGVBdXRob3JpdHkSHgoKZ2VuZXJhdGlvbhgBIAEoBF'
        'IKZ2VuZXJhdGlvbhJDCgVzdGF0ZRgCIAEoDjItLnZvaWNlLnNwYWNlLnYxLk1lbWJlckFkbWlz'
        'c2lvbkxpZmVjeWNsZVN0YXRlUgVzdGF0ZQ==');

@$core.Deprecated('Use memberAdmissionOwnershipAuthorityDescriptor instead')
const MemberAdmissionOwnershipAuthority$json = {
  '1': 'MemberAdmissionOwnershipAuthority',
  '2': [
    {'1': 'generation', '3': 1, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionOwnershipState',
      '10': 'state'
    },
    {
      '1': 'current_owner_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'currentOwnerProfileId'
    },
  ],
};

/// Descriptor for `MemberAdmissionOwnershipAuthority`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionOwnershipAuthorityDescriptor =
    $convert.base64Decode(
        'CiFNZW1iZXJBZG1pc3Npb25Pd25lcnNoaXBBdXRob3JpdHkSHgoKZ2VuZXJhdGlvbhgBIAEoBF'
        'IKZ2VuZXJhdGlvbhJDCgVzdGF0ZRgCIAEoDjItLnZvaWNlLnNwYWNlLnYxLk1lbWJlckFkbWlz'
        'c2lvbk93bmVyc2hpcFN0YXRlUgVzdGF0ZRI3ChhjdXJyZW50X293bmVyX3Byb2ZpbGVfaWQYAy'
        'ABKAlSFWN1cnJlbnRPd25lclByb2ZpbGVJZA==');

@$core.Deprecated('Use memberAdmissionOrdinaryAuthorityDescriptor instead')
const MemberAdmissionOrdinaryAuthority$json = {
  '1': 'MemberAdmissionOrdinaryAuthority',
  '2': [
    {'1': 'head_generation', '3': 1, '4': 1, '5': 4, '10': 'headGeneration'},
    {'1': 'retired_through', '3': 2, '4': 1, '5': 4, '10': 'retiredThrough'},
    {
      '1': 'state',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionSourceState',
      '10': 'state'
    },
    {'1': 'explicit_legacy', '3': 4, '4': 1, '5': 8, '10': 'explicitLegacy'},
    {
      '1': 'published_binding',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'publishedBinding'
    },
    {
      '1': 'role_receipt_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'roleReceiptSha256'
    },
    {
      '1': 'preparing_binding',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'preparingBinding'
    },
    {
      '1': 'preparing_admission_binding_sha256',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'preparingAdmissionBindingSha256'
    },
    {
      '1': 'publication_kind',
      '3': 9,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionPublicationKind',
      '10': 'publicationKind'
    },
    {
      '1': 'native_publication',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionNativePublicationProof',
      '10': 'nativePublication'
    },
  ],
};

/// Descriptor for `MemberAdmissionOrdinaryAuthority`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionOrdinaryAuthorityDescriptor = $convert.base64Decode(
    'CiBNZW1iZXJBZG1pc3Npb25PcmRpbmFyeUF1dGhvcml0eRInCg9oZWFkX2dlbmVyYXRpb24YAS'
    'ABKARSDmhlYWRHZW5lcmF0aW9uEicKD3JldGlyZWRfdGhyb3VnaBgCIAEoBFIOcmV0aXJlZFRo'
    'cm91Z2gSQAoFc3RhdGUYAyABKA4yKi52b2ljZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25Tb3'
    'VyY2VTdGF0ZVIFc3RhdGUSJwoPZXhwbGljaXRfbGVnYWN5GAQgASgIUg5leHBsaWNpdExlZ2Fj'
    'eRJNChFwdWJsaXNoZWRfYmluZGluZxgFIAEoCzIgLnZvaWNlLnJvbGUudjEuTWVtYmVyc2hpcE'
    'JpbmRpbmdSEHB1Ymxpc2hlZEJpbmRpbmcSLgoTcm9sZV9yZWNlaXB0X3NoYTI1NhgGIAEoDFIR'
    'cm9sZVJlY2VpcHRTaGEyNTYSTQoRcHJlcGFyaW5nX2JpbmRpbmcYByABKAsyIC52b2ljZS5yb2'
    'xlLnYxLk1lbWJlcnNoaXBCaW5kaW5nUhBwcmVwYXJpbmdCaW5kaW5nEksKInByZXBhcmluZ19h'
    'ZG1pc3Npb25fYmluZGluZ19zaGEyNTYYCCABKAxSH3ByZXBhcmluZ0FkbWlzc2lvbkJpbmRpbm'
    'dTaGEyNTYSWQoQcHVibGljYXRpb25fa2luZBgJIAEoDjIuLnZvaWNlLnNwYWNlLnYxLk1lbWJl'
    'ckFkbWlzc2lvblB1YmxpY2F0aW9uS2luZFIPcHVibGljYXRpb25LaW5kEmQKEm5hdGl2ZV9wdW'
    'JsaWNhdGlvbhgKIAEoCzI1LnZvaWNlLnNwYWNlLnYxLk1lbWJlckFkbWlzc2lvbk5hdGl2ZVB1'
    'YmxpY2F0aW9uUHJvb2ZSEW5hdGl2ZVB1YmxpY2F0aW9u');

@$core.Deprecated('Use memberAdmissionCommunityAuthorityDescriptor instead')
const MemberAdmissionCommunityAuthority$json = {
  '1': 'MemberAdmissionCommunityAuthority',
  '2': [
    {
      '1': 'state',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.voice.space.v1.MemberAdmissionSourceState',
      '10': 'state'
    },
    {
      '1': 'binding',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MembershipBinding',
      '10': 'binding'
    },
    {'1': 'retired_through', '3': 3, '4': 1, '5': 4, '10': 'retiredThrough'},
    {
      '1': 'roster_source_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'rosterSourceRevision'
    },
    {
      '1': 'validity_remaining',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'validityRemaining'
    },
  ],
};

/// Descriptor for `MemberAdmissionCommunityAuthority`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionCommunityAuthorityDescriptor = $convert.base64Decode(
    'CiFNZW1iZXJBZG1pc3Npb25Db21tdW5pdHlBdXRob3JpdHkSQAoFc3RhdGUYASABKA4yKi52b2'
    'ljZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25Tb3VyY2VTdGF0ZVIFc3RhdGUSOgoHYmluZGlu'
    'ZxgCIAEoCzIgLnZvaWNlLnJvbGUudjEuTWVtYmVyc2hpcEJpbmRpbmdSB2JpbmRpbmcSJwoPcm'
    'V0aXJlZF90aHJvdWdoGAMgASgEUg5yZXRpcmVkVGhyb3VnaBI0ChZyb3N0ZXJfc291cmNlX3Jl'
    'dmlzaW9uGAQgASgEUhRyb3N0ZXJTb3VyY2VSZXZpc2lvbhJIChJ2YWxpZGl0eV9yZW1haW5pbm'
    'cYBSABKAsyGS5nb29nbGUucHJvdG9idWYuRHVyYXRpb25SEXZhbGlkaXR5UmVtYWluaW5n');

@$core.Deprecated('Use memberAdmissionTreeRelationAuthorityDescriptor instead')
const MemberAdmissionTreeRelationAuthority$json = {
  '1': 'MemberAdmissionTreeRelationAuthority',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'present', '3': 2, '4': 1, '5': 8, '10': 'present'},
    {'1': 'revision', '3': 3, '4': 1, '5': 4, '10': 'revision'},
  ],
};

/// Descriptor for `MemberAdmissionTreeRelationAuthority`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberAdmissionTreeRelationAuthorityDescriptor =
    $convert.base64Decode(
        'CiRNZW1iZXJBZG1pc3Npb25UcmVlUmVsYXRpb25BdXRob3JpdHkSFwoHY2hhdF9pZBgBIAEoCV'
        'IGY2hhdElkEhgKB3ByZXNlbnQYAiABKAhSB3ByZXNlbnQSGgoIcmV2aXNpb24YAyABKARSCHJl'
        'dmlzaW9u');

@$core.Deprecated('Use getMemberAdmissionAuthorityResponseDescriptor instead')
const GetMemberAdmissionAuthorityResponse$json = {
  '1': 'GetMemberAdmissionAuthorityResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'space_authority_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'spaceAuthorityRevision'
    },
    {
      '1': 'lifecycle',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionLifecycleAuthority',
      '10': 'lifecycle'
    },
    {
      '1': 'ownership',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionOwnershipAuthority',
      '10': 'ownership'
    },
    {
      '1': 'ordinary',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionOrdinaryAuthority',
      '10': 'ordinary'
    },
    {
      '1': 'community',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionCommunityAuthority',
      '10': 'community'
    },
    {
      '1': 'tree_relation',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.MemberAdmissionTreeRelationAuthority',
      '9': 0,
      '10': 'treeRelation',
      '17': true
    },
    {
      '1': 'sampled_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'sampledAt'
    },
    {
      '1': 'validity_remaining',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Duration',
      '10': 'validityRemaining'
    },
    {
      '1': 'bot_message_send_binding',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.BotMessageSendBinding',
      '9': 1,
      '10': 'botMessageSendBinding',
      '17': true
    },
  ],
  '8': [
    {'1': '_tree_relation'},
    {'1': '_bot_message_send_binding'},
  ],
};

/// Descriptor for `GetMemberAdmissionAuthorityResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberAdmissionAuthorityResponseDescriptor = $convert.base64Decode(
    'CiNHZXRNZW1iZXJBZG1pc3Npb25BdXRob3JpdHlSZXNwb25zZRIpChBwcm90b2NvbF92ZXJzaW'
    '9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSHQoK'
    'cHJvZmlsZV9pZBgDIAEoCVIJcHJvZmlsZUlkEjgKGHNwYWNlX2F1dGhvcml0eV9yZXZpc2lvbh'
    'gEIAEoBFIWc3BhY2VBdXRob3JpdHlSZXZpc2lvbhJPCglsaWZlY3ljbGUYBSABKAsyMS52b2lj'
    'ZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25MaWZlY3ljbGVBdXRob3JpdHlSCWxpZmVjeWNsZR'
    'JPCglvd25lcnNoaXAYBiABKAsyMS52b2ljZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25Pd25l'
    'cnNoaXBBdXRob3JpdHlSCW93bmVyc2hpcBJMCghvcmRpbmFyeRgHIAEoCzIwLnZvaWNlLnNwYW'
    'NlLnYxLk1lbWJlckFkbWlzc2lvbk9yZGluYXJ5QXV0aG9yaXR5UghvcmRpbmFyeRJPCgljb21t'
    'dW5pdHkYCCABKAsyMS52b2ljZS5zcGFjZS52MS5NZW1iZXJBZG1pc3Npb25Db21tdW5pdHlBdX'
    'Rob3JpdHlSCWNvbW11bml0eRJeCg10cmVlX3JlbGF0aW9uGAkgASgLMjQudm9pY2Uuc3BhY2Uu'
    'djEuTWVtYmVyQWRtaXNzaW9uVHJlZVJlbGF0aW9uQXV0aG9yaXR5SABSDHRyZWVSZWxhdGlvbo'
    'gBARI5CgpzYW1wbGVkX2F0GAogASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJc2Ft'
    'cGxlZEF0EkgKEnZhbGlkaXR5X3JlbWFpbmluZxgLIAEoCzIZLmdvb2dsZS5wcm90b2J1Zi5EdX'
    'JhdGlvblIRdmFsaWRpdHlSZW1haW5pbmcSZAoYYm90X21lc3NhZ2Vfc2VuZF9iaW5kaW5nGAwg'
    'ASgLMiYudm9pY2UuY29tbW9uLnYxLkJvdE1lc3NhZ2VTZW5kQmluZGluZ0gBUhVib3RNZXNzYW'
    'dlU2VuZEJpbmRpbmeIAQFCEAoOX3RyZWVfcmVsYXRpb25CGwoZX2JvdF9tZXNzYWdlX3NlbmRf'
    'YmluZGluZw==');
