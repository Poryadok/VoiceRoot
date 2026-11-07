// This is a generated file - do not edit.
//
// Generated from voice/user/v1/auth_profile_authority.proto.

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

@$core.Deprecated('Use authProfileMatchOutcomeDescriptor instead')
const AuthProfileMatchOutcome$json = {
  '1': 'AuthProfileMatchOutcome',
  '2': [
    {'1': 'AUTH_PROFILE_MATCH_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'AUTH_PROFILE_MATCH_OUTCOME_MATCH', '2': 1},
    {'1': 'AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT', '2': 2},
    {'1': 'AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE', '2': 3},
  ],
};

/// Descriptor for `AuthProfileMatchOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List authProfileMatchOutcomeDescriptor = $convert.base64Decode(
    'ChdBdXRoUHJvZmlsZU1hdGNoT3V0Y29tZRIqCiZBVVRIX1BST0ZJTEVfTUFUQ0hfT1VUQ09NRV'
    '9VTlNQRUNJRklFRBAAEiQKIEFVVEhfUFJPRklMRV9NQVRDSF9PVVRDT01FX01BVENIEAESMAos'
    'QVVUSF9QUk9GSUxFX01BVENIX09VVENPTUVfRElGRkVSRU5UX0FDQ09VTlQQAhIpCiVBVVRIX1'
    'BST0ZJTEVfTUFUQ0hfT1VUQ09NRV9JTkNPTVBMRVRFEAM=');

@$core.Deprecated('Use authOwnershipTargetOutcomeDescriptor instead')
const AuthOwnershipTargetOutcome$json = {
  '1': 'AuthOwnershipTargetOutcome',
  '2': [
    {'1': 'AUTH_OWNERSHIP_TARGET_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE', '2': 1},
    {'1': 'AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE', '2': 2},
    {'1': 'AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE', '2': 3},
  ],
};

/// Descriptor for `AuthOwnershipTargetOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List authOwnershipTargetOutcomeDescriptor = $convert.base64Decode(
    'ChpBdXRoT3duZXJzaGlwVGFyZ2V0T3V0Y29tZRItCilBVVRIX09XTkVSU0hJUF9UQVJHRVRfT1'
    'VUQ09NRV9VTlNQRUNJRklFRBAAEjEKLUFVVEhfT1dORVJTSElQX1RBUkdFVF9PVVRDT01FX0NV'
    'UlJFTlRfUFJPRklMRRABEjIKLkFVVEhfT1dORVJTSElQX1RBUkdFVF9PVVRDT01FX1RFUk1JTk'
    'FMX1BST0ZJTEUQAhIsCihBVVRIX09XTkVSU0hJUF9UQVJHRVRfT1VUQ09NRV9JTkNPTVBMRVRF'
    'EAM=');

@$core.Deprecated('Use authProfileSourcePinDescriptor instead')
const AuthProfileSourcePin$json = {
  '1': 'AuthProfileSourcePin',
  '2': [
    {
      '1': 'user_source_generation',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'userSourceGeneration'
    },
    {
      '1': 'history_generation',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'historyGeneration'
    },
    {'1': 'keyset_revision', '3': 3, '4': 1, '5': 4, '10': 'keysetRevision'},
  ],
};

/// Descriptor for `AuthProfileSourcePin`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authProfileSourcePinDescriptor = $convert.base64Decode(
    'ChRBdXRoUHJvZmlsZVNvdXJjZVBpbhI0ChZ1c2VyX3NvdXJjZV9nZW5lcmF0aW9uGAEgASgJUh'
    'R1c2VyU291cmNlR2VuZXJhdGlvbhItChJoaXN0b3J5X2dlbmVyYXRpb24YAiABKAlSEWhpc3Rv'
    'cnlHZW5lcmF0aW9uEicKD2tleXNldF9yZXZpc2lvbhgDIAEoBFIOa2V5c2V0UmV2aXNpb24=');

@$core.Deprecated('Use authProfileSourceCutDescriptor instead')
const AuthProfileSourceCut$json = {
  '1': 'AuthProfileSourceCut',
  '2': [
    {
      '1': 'source',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileSourcePin',
      '10': 'source'
    },
    {
      '1': 'user_authority_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'userAuthorityRevision'
    },
    {'1': 'through_revision', '3': 3, '4': 1, '5': 4, '10': 'throughRevision'},
    {'1': 'head_digest', '3': 4, '4': 1, '5': 12, '10': 'headDigest'},
    {'1': 'retained_floor', '3': 5, '4': 1, '5': 4, '10': 'retainedFloor'},
    {
      '1': 'unresolved_erasure_issued',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'unresolvedErasureIssued'
    },
    {
      '1': 'verified_erasure_covered',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'verifiedErasureCovered'
    },
    {
      '1': 'observed_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'observedAt'
    },
  ],
};

/// Descriptor for `AuthProfileSourceCut`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authProfileSourceCutDescriptor = $convert.base64Decode(
    'ChRBdXRoUHJvZmlsZVNvdXJjZUN1dBI7CgZzb3VyY2UYASABKAsyIy52b2ljZS51c2VyLnYxLk'
    'F1dGhQcm9maWxlU291cmNlUGluUgZzb3VyY2USNgoXdXNlcl9hdXRob3JpdHlfcmV2aXNpb24Y'
    'AiABKARSFXVzZXJBdXRob3JpdHlSZXZpc2lvbhIpChB0aHJvdWdoX3JldmlzaW9uGAMgASgEUg'
    '90aHJvdWdoUmV2aXNpb24SHwoLaGVhZF9kaWdlc3QYBCABKAxSCmhlYWREaWdlc3QSJQoOcmV0'
    'YWluZWRfZmxvb3IYBSABKARSDXJldGFpbmVkRmxvb3ISOgoZdW5yZXNvbHZlZF9lcmFzdXJlX2'
    'lzc3VlZBgGIAEoBFIXdW5yZXNvbHZlZEVyYXN1cmVJc3N1ZWQSOAoYdmVyaWZpZWRfZXJhc3Vy'
    'ZV9jb3ZlcmVkGAcgASgEUhZ2ZXJpZmllZEVyYXN1cmVDb3ZlcmVkEjsKC29ic2VydmVkX2F0GA'
    'ggASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIKb2JzZXJ2ZWRBdA==');

@$core.Deprecated('Use matchAuthProfileReferencesRequestDescriptor instead')
const MatchAuthProfileReferencesRequest$json = {
  '1': 'MatchAuthProfileReferencesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'inventory_id', '3': 3, '4': 1, '5': 9, '10': 'inventoryId'},
    {
      '1': 'inventory_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'inventoryRevision'
    },
    {'1': 'account_id', '3': 5, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'owner_handle', '3': 6, '4': 1, '5': 9, '10': 'ownerHandle'},
    {'1': 'deletion_cycle_id', '3': 7, '4': 1, '5': 9, '10': 'deletionCycleId'},
    {'1': 'profile_ids', '3': 8, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'expected_source',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileSourcePin',
      '10': 'expectedSource'
    },
  ],
};

/// Descriptor for `MatchAuthProfileReferencesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchAuthProfileReferencesRequestDescriptor = $convert.base64Decode(
    'CiFNYXRjaEF1dGhQcm9maWxlUmVmZXJlbmNlc1JlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbh'
    'gBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEh0KCnJlcXVlc3RfaWQYAiABKAlSCXJlcXVlc3RJZBIh'
    'CgxpbnZlbnRvcnlfaWQYAyABKAlSC2ludmVudG9yeUlkEi0KEmludmVudG9yeV9yZXZpc2lvbh'
    'gEIAEoBFIRaW52ZW50b3J5UmV2aXNpb24SHQoKYWNjb3VudF9pZBgFIAEoCVIJYWNjb3VudElk'
    'EiEKDG93bmVyX2hhbmRsZRgGIAEoCVILb3duZXJIYW5kbGUSKgoRZGVsZXRpb25fY3ljbGVfaW'
    'QYByABKAlSD2RlbGV0aW9uQ3ljbGVJZBIfCgtwcm9maWxlX2lkcxgIIAMoCVIKcHJvZmlsZUlk'
    'cxJMCg9leHBlY3RlZF9zb3VyY2UYCSABKAsyIy52b2ljZS51c2VyLnYxLkF1dGhQcm9maWxlU2'
    '91cmNlUGluUg5leHBlY3RlZFNvdXJjZQ==');

@$core.Deprecated('Use authProfileReferenceMatchDescriptor instead')
const AuthProfileReferenceMatch$json = {
  '1': 'AuthProfileReferenceMatch',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'outcome',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.AuthProfileMatchOutcome',
      '10': 'outcome'
    },
    {'1': 'association_id', '3': 3, '4': 1, '5': 9, '10': 'associationId'},
    {
      '1': 'association_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'associationRevision'
    },
    {'1': 'capture_origin', '3': 5, '4': 1, '5': 9, '10': 'captureOrigin'},
  ],
};

/// Descriptor for `AuthProfileReferenceMatch`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authProfileReferenceMatchDescriptor = $convert.base64Decode(
    'ChlBdXRoUHJvZmlsZVJlZmVyZW5jZU1hdGNoEh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbG'
    'VJZBJACgdvdXRjb21lGAIgASgOMiYudm9pY2UudXNlci52MS5BdXRoUHJvZmlsZU1hdGNoT3V0'
    'Y29tZVIHb3V0Y29tZRIlCg5hc3NvY2lhdGlvbl9pZBgDIAEoCVINYXNzb2NpYXRpb25JZBIxCh'
    'Rhc3NvY2lhdGlvbl9yZXZpc2lvbhgEIAEoBFITYXNzb2NpYXRpb25SZXZpc2lvbhIlCg5jYXB0'
    'dXJlX29yaWdpbhgFIAEoCVINY2FwdHVyZU9yaWdpbg==');

@$core.Deprecated('Use matchAuthProfileReferencesResponseDescriptor instead')
const MatchAuthProfileReferencesResponse$json = {
  '1': 'MatchAuthProfileReferencesResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'inventory_id', '3': 3, '4': 1, '5': 9, '10': 'inventoryId'},
    {
      '1': 'inventory_revision',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'inventoryRevision'
    },
    {'1': 'account_id', '3': 5, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'owner_handle', '3': 6, '4': 1, '5': 9, '10': 'ownerHandle'},
    {'1': 'deletion_cycle_id', '3': 7, '4': 1, '5': 9, '10': 'deletionCycleId'},
    {'1': 'request_digest', '3': 8, '4': 1, '5': 12, '10': 'requestDigest'},
    {
      '1': 'results',
      '3': 9,
      '4': 3,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileReferenceMatch',
      '10': 'results'
    },
    {
      '1': 'source_cut',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileSourceCut',
      '10': 'sourceCut'
    },
    {
      '1': 'all_requested_resolved',
      '3': 11,
      '4': 1,
      '5': 8,
      '10': 'allRequestedResolved'
    },
  ],
};

/// Descriptor for `MatchAuthProfileReferencesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchAuthProfileReferencesResponseDescriptor = $convert.base64Decode(
    'CiJNYXRjaEF1dGhQcm9maWxlUmVmZXJlbmNlc1Jlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQS'
    'IQoMaW52ZW50b3J5X2lkGAMgASgJUgtpbnZlbnRvcnlJZBItChJpbnZlbnRvcnlfcmV2aXNpb2'
    '4YBCABKARSEWludmVudG9yeVJldmlzaW9uEh0KCmFjY291bnRfaWQYBSABKAlSCWFjY291bnRJ'
    'ZBIhCgxvd25lcl9oYW5kbGUYBiABKAlSC293bmVySGFuZGxlEioKEWRlbGV0aW9uX2N5Y2xlX2'
    'lkGAcgASgJUg9kZWxldGlvbkN5Y2xlSWQSJQoOcmVxdWVzdF9kaWdlc3QYCCABKAxSDXJlcXVl'
    'c3REaWdlc3QSQgoHcmVzdWx0cxgJIAMoCzIoLnZvaWNlLnVzZXIudjEuQXV0aFByb2ZpbGVSZW'
    'ZlcmVuY2VNYXRjaFIHcmVzdWx0cxJCCgpzb3VyY2VfY3V0GAogASgLMiMudm9pY2UudXNlci52'
    'MS5BdXRoUHJvZmlsZVNvdXJjZUN1dFIJc291cmNlQ3V0EjQKFmFsbF9yZXF1ZXN0ZWRfcmVzb2'
    'x2ZWQYCyABKAhSFGFsbFJlcXVlc3RlZFJlc29sdmVk');

@$core.Deprecated('Use resolveAuthOwnershipTargetRequestDescriptor instead')
const ResolveAuthOwnershipTargetRequest$json = {
  '1': 'ResolveAuthOwnershipTargetRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'profile_id', '3': 4, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'expected_source',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileSourcePin',
      '10': 'expectedSource'
    },
  ],
};

/// Descriptor for `ResolveAuthOwnershipTargetRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveAuthOwnershipTargetRequestDescriptor = $convert.base64Decode(
    'CiFSZXNvbHZlQXV0aE93bmVyc2hpcFRhcmdldFJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbh'
    'gBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEh0KCnJlcXVlc3RfaWQYAiABKAlSCXJlcXVlc3RJZBIh'
    'CgxvcGVyYXRpb25faWQYAyABKAlSC29wZXJhdGlvbklkEh0KCnByb2ZpbGVfaWQYBCABKAlSCX'
    'Byb2ZpbGVJZBJMCg9leHBlY3RlZF9zb3VyY2UYBSABKAsyIy52b2ljZS51c2VyLnYxLkF1dGhQ'
    'cm9maWxlU291cmNlUGluUg5leHBlY3RlZFNvdXJjZQ==');

@$core.Deprecated('Use resolveAuthOwnershipTargetResponseDescriptor instead')
const ResolveAuthOwnershipTargetResponse$json = {
  '1': 'ResolveAuthOwnershipTargetResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'profile_id', '3': 4, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'request_digest', '3': 5, '4': 1, '5': 12, '10': 'requestDigest'},
    {
      '1': 'outcome',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.AuthOwnershipTargetOutcome',
      '10': 'outcome'
    },
    {'1': 'account_id', '3': 7, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_revision', '3': 8, '4': 1, '5': 4, '10': 'profileRevision'},
    {'1': 'deleted', '3': 9, '4': 1, '5': 8, '10': 'deleted'},
    {'1': 'frozen', '3': 10, '4': 1, '5': 8, '10': 'frozen'},
    {'1': 'account_inactive', '3': 11, '4': 1, '5': 8, '10': 'accountInactive'},
    {'1': 'association_id', '3': 12, '4': 1, '5': 9, '10': 'associationId'},
    {
      '1': 'association_revision',
      '3': 13,
      '4': 1,
      '5': 4,
      '10': 'associationRevision'
    },
    {
      '1': 'terminal_operation_id',
      '3': 14,
      '4': 1,
      '5': 9,
      '10': 'terminalOperationId'
    },
    {
      '1': 'terminal_revision',
      '3': 15,
      '4': 1,
      '5': 4,
      '10': 'terminalRevision'
    },
    {
      '1': 'terminal_receipt_id',
      '3': 16,
      '4': 1,
      '5': 9,
      '10': 'terminalReceiptId'
    },
    {
      '1': 'terminal_receipt_digest',
      '3': 17,
      '4': 1,
      '5': 12,
      '10': 'terminalReceiptDigest'
    },
    {
      '1': 'source_cut',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.AuthProfileSourceCut',
      '10': 'sourceCut'
    },
  ],
};

/// Descriptor for `ResolveAuthOwnershipTargetResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveAuthOwnershipTargetResponseDescriptor = $convert.base64Decode(
    'CiJSZXNvbHZlQXV0aE93bmVyc2hpcFRhcmdldFJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQS'
    'IQoMb3BlcmF0aW9uX2lkGAMgASgJUgtvcGVyYXRpb25JZBIdCgpwcm9maWxlX2lkGAQgASgJUg'
    'lwcm9maWxlSWQSJQoOcmVxdWVzdF9kaWdlc3QYBSABKAxSDXJlcXVlc3REaWdlc3QSQwoHb3V0'
    'Y29tZRgGIAEoDjIpLnZvaWNlLnVzZXIudjEuQXV0aE93bmVyc2hpcFRhcmdldE91dGNvbWVSB2'
    '91dGNvbWUSHQoKYWNjb3VudF9pZBgHIAEoCVIJYWNjb3VudElkEikKEHByb2ZpbGVfcmV2aXNp'
    'b24YCCABKARSD3Byb2ZpbGVSZXZpc2lvbhIYCgdkZWxldGVkGAkgASgIUgdkZWxldGVkEhYKBm'
    'Zyb3plbhgKIAEoCFIGZnJvemVuEikKEGFjY291bnRfaW5hY3RpdmUYCyABKAhSD2FjY291bnRJ'
    'bmFjdGl2ZRIlCg5hc3NvY2lhdGlvbl9pZBgMIAEoCVINYXNzb2NpYXRpb25JZBIxChRhc3NvY2'
    'lhdGlvbl9yZXZpc2lvbhgNIAEoBFITYXNzb2NpYXRpb25SZXZpc2lvbhIyChV0ZXJtaW5hbF9v'
    'cGVyYXRpb25faWQYDiABKAlSE3Rlcm1pbmFsT3BlcmF0aW9uSWQSKwoRdGVybWluYWxfcmV2aX'
    'Npb24YDyABKARSEHRlcm1pbmFsUmV2aXNpb24SLgoTdGVybWluYWxfcmVjZWlwdF9pZBgQIAEo'
    'CVIRdGVybWluYWxSZWNlaXB0SWQSNgoXdGVybWluYWxfcmVjZWlwdF9kaWdlc3QYESABKAxSFX'
    'Rlcm1pbmFsUmVjZWlwdERpZ2VzdBJCCgpzb3VyY2VfY3V0GBIgASgLMiMudm9pY2UudXNlci52'
    'MS5BdXRoUHJvZmlsZVNvdXJjZUN1dFIJc291cmNlQ3V0');
