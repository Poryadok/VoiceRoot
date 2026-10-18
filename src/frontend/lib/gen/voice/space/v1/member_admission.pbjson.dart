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
  ],
  '8': [
    {'1': '_expected_generation'},
    {'1': '_expected_admission_operation_id'},
    {'1': '_chat_id'},
  ],
};

/// Descriptor for `GetMemberAdmissionAuthorityRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberAdmissionAuthorityRequestDescriptor = $convert.base64Decode(
    'CiJHZXRNZW1iZXJBZG1pc3Npb25BdXRob3JpdHlSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBIdCgpw'
    'cm9maWxlX2lkGAMgASgJUglwcm9maWxlSWQSNAoTZXhwZWN0ZWRfZ2VuZXJhdGlvbhgEIAEoBE'
    'gAUhJleHBlY3RlZEdlbmVyYXRpb26IAQESSgofZXhwZWN0ZWRfYWRtaXNzaW9uX29wZXJhdGlv'
    'bl9pZBgFIAEoCUgBUhxleHBlY3RlZEFkbWlzc2lvbk9wZXJhdGlvbklkiAEBEhwKB2NoYXRfaW'
    'QYBiABKAlIAlIGY2hhdElkiAEBQhYKFF9leHBlY3RlZF9nZW5lcmF0aW9uQiIKIF9leHBlY3Rl'
    'ZF9hZG1pc3Npb25fb3BlcmF0aW9uX2lkQgoKCF9jaGF0X2lk');

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
    'dTaGEyNTY=');

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
  ],
  '8': [
    {'1': '_tree_relation'},
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
    'JhdGlvblIRdmFsaWRpdHlSZW1haW5pbmdCEAoOX3RyZWVfcmVsYXRpb24=');
