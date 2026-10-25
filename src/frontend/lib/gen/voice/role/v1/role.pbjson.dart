// This is a generated file - do not edit.
//
// Generated from voice/role/v1/role.proto.

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

@$core.Deprecated('Use botGlobalTerminalOutcomeDescriptor instead')
const BotGlobalTerminalOutcome$json = {
  '1': 'BotGlobalTerminalOutcome',
  '2': [
    {'1': 'BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED', '2': 0},
    {
      '1': 'BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED',
      '2': 1
    },
  ],
};

/// Descriptor for `BotGlobalTerminalOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List botGlobalTerminalOutcomeDescriptor = $convert.base64Decode(
    'ChhCb3RHbG9iYWxUZXJtaW5hbE91dGNvbWUSKwonQk9UX0dMT0JBTF9URVJNSU5BTF9PVVRDT0'
    '1FX1VOU1BFQ0lGSUVEEAASPwo7Qk9UX0dMT0JBTF9URVJNSU5BTF9PVVRDT01FX0JPVF9BQ1RP'
    'Ul9ST0xFX01VVEFUSU9OU19GRU5DRUQQAQ==');

@$core.Deprecated('Use ownershipTransferStateDescriptor instead')
const OwnershipTransferState$json = {
  '1': 'OwnershipTransferState',
  '2': [
    {'1': 'OWNERSHIP_TRANSFER_STATE_UNSPECIFIED', '2': 0},
    {'1': 'OWNERSHIP_TRANSFER_STATE_PREPARED', '2': 1},
    {'1': 'OWNERSHIP_TRANSFER_STATE_FINALIZED', '2': 2},
    {'1': 'OWNERSHIP_TRANSFER_STATE_ABORTED', '2': 3},
  ],
};

/// Descriptor for `OwnershipTransferState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List ownershipTransferStateDescriptor = $convert.base64Decode(
    'ChZPd25lcnNoaXBUcmFuc2ZlclN0YXRlEigKJE9XTkVSU0hJUF9UUkFOU0ZFUl9TVEFURV9VTl'
    'NQRUNJRklFRBAAEiUKIU9XTkVSU0hJUF9UUkFOU0ZFUl9TVEFURV9QUkVQQVJFRBABEiYKIk9X'
    'TkVSU0hJUF9UUkFOU0ZFUl9TVEFURV9GSU5BTElaRUQQAhIkCiBPV05FUlNISVBfVFJBTlNGRV'
    'JfU1RBVEVfQUJPUlRFRBAD');

@$core.Deprecated('Use roleRetirementStateDescriptor instead')
const RoleRetirementState$json = {
  '1': 'RoleRetirementState',
  '2': [
    {'1': 'ROLE_RETIREMENT_STATE_UNSPECIFIED', '2': 0},
    {'1': 'ROLE_RETIREMENT_STATE_RETIRED', '2': 1},
  ],
};

/// Descriptor for `RoleRetirementState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List roleRetirementStateDescriptor = $convert.base64Decode(
    'ChNSb2xlUmV0aXJlbWVudFN0YXRlEiUKIVJPTEVfUkVUSVJFTUVOVF9TVEFURV9VTlNQRUNJRk'
    'lFRBAAEiEKHVJPTEVfUkVUSVJFTUVOVF9TVEFURV9SRVRJUkVEEAE=');

@$core.Deprecated('Use gameSessionGrantOutcomeDescriptor instead')
const GameSessionGrantOutcome$json = {
  '1': 'GameSessionGrantOutcome',
  '2': [
    {'1': 'GAME_SESSION_GRANT_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'GAME_SESSION_GRANT_OUTCOME_APPLIED', '2': 1},
    {'1': 'GAME_SESSION_GRANT_OUTCOME_REPLAYED', '2': 2},
    {'1': 'GAME_SESSION_GRANT_OUTCOME_STALE', '2': 3},
    {'1': 'GAME_SESSION_GRANT_OUTCOME_REVOKED', '2': 4},
  ],
};

/// Descriptor for `GameSessionGrantOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List gameSessionGrantOutcomeDescriptor = $convert.base64Decode(
    'ChdHYW1lU2Vzc2lvbkdyYW50T3V0Y29tZRIqCiZHQU1FX1NFU1NJT05fR1JBTlRfT1VUQ09NRV'
    '9VTlNQRUNJRklFRBAAEiYKIkdBTUVfU0VTU0lPTl9HUkFOVF9PVVRDT01FX0FQUExJRUQQARIn'
    'CiNHQU1FX1NFU1NJT05fR1JBTlRfT1VUQ09NRV9SRVBMQVlFRBACEiQKIEdBTUVfU0VTU0lPTl'
    '9HUkFOVF9PVVRDT01FX1NUQUxFEAMSJgoiR0FNRV9TRVNTSU9OX0dSQU5UX09VVENPTUVfUkVW'
    'T0tFRBAE');

@$core.Deprecated('Use botRemovalNativeEvidenceDescriptor instead')
const BotRemovalNativeEvidence$json = {
  '1': 'BotRemovalNativeEvidence',
  '2': [
    {'1': 'installation_id', '3': 1, '4': 1, '5': 9, '10': 'installationId'},
    {
      '1': 'install_operation_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'installOperationId'
    },
    {'1': 'generation', '3': 3, '4': 1, '5': 4, '10': 'generation'},
  ],
};

/// Descriptor for `BotRemovalNativeEvidence`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botRemovalNativeEvidenceDescriptor = $convert.base64Decode(
    'ChhCb3RSZW1vdmFsTmF0aXZlRXZpZGVuY2USJwoPaW5zdGFsbGF0aW9uX2lkGAEgASgJUg5pbn'
    'N0YWxsYXRpb25JZBIwChRpbnN0YWxsX29wZXJhdGlvbl9pZBgCIAEoCVISaW5zdGFsbE9wZXJh'
    'dGlvbklkEh4KCmdlbmVyYXRpb24YAyABKARSCmdlbmVyYXRpb24=');

@$core.Deprecated('Use botRemovalLegacyInstallationEvidenceDescriptor instead')
const BotRemovalLegacyInstallationEvidence$json = {
  '1': 'BotRemovalLegacyInstallationEvidence',
  '2': [
    {'1': 'installation_id', '3': 1, '4': 1, '5': 9, '10': 'installationId'},
    {'1': 'snapshot_id', '3': 2, '4': 1, '5': 9, '10': 'snapshotId'},
    {'1': 'snapshot_sha256', '3': 3, '4': 1, '5': 12, '10': 'snapshotSha256'},
  ],
};

/// Descriptor for `BotRemovalLegacyInstallationEvidence`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botRemovalLegacyInstallationEvidenceDescriptor =
    $convert.base64Decode(
        'CiRCb3RSZW1vdmFsTGVnYWN5SW5zdGFsbGF0aW9uRXZpZGVuY2USJwoPaW5zdGFsbGF0aW9uX2'
        'lkGAEgASgJUg5pbnN0YWxsYXRpb25JZBIfCgtzbmFwc2hvdF9pZBgCIAEoCVIKc25hcHNob3RJ'
        'ZBInCg9zbmFwc2hvdF9zaGEyNTYYAyABKAxSDnNuYXBzaG90U2hhMjU2');

@$core.Deprecated('Use botRemovalOrphanWhitelistEvidenceDescriptor instead')
const BotRemovalOrphanWhitelistEvidence$json = {
  '1': 'BotRemovalOrphanWhitelistEvidence',
  '2': [
    {'1': 'snapshot_id', '3': 1, '4': 1, '5': 9, '10': 'snapshotId'},
    {'1': 'snapshot_sha256', '3': 2, '4': 1, '5': 12, '10': 'snapshotSha256'},
  ],
};

/// Descriptor for `BotRemovalOrphanWhitelistEvidence`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botRemovalOrphanWhitelistEvidenceDescriptor =
    $convert.base64Decode(
        'CiFCb3RSZW1vdmFsT3JwaGFuV2hpdGVsaXN0RXZpZGVuY2USHwoLc25hcHNob3RfaWQYASABKA'
        'lSCnNuYXBzaG90SWQSJwoPc25hcHNob3Rfc2hhMjU2GAIgASgMUg5zbmFwc2hvdFNoYTI1Ng==');

@$core.Deprecated('Use applyBotInstallationRemovalRequestDescriptor instead')
const ApplyBotInstallationRemovalRequest$json = {
  '1': 'ApplyBotInstallationRemovalRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'decision_id', '3': 3, '4': 1, '5': 9, '10': 'decisionId'},
    {'1': 'decision_sha256', '3': 4, '4': 1, '5': 12, '10': 'decisionSha256'},
    {'1': 'purpose', '3': 5, '4': 1, '5': 9, '10': 'purpose'},
    {'1': 'bot_id', '3': 6, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'actor_profile_id', '3': 7, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'space_id', '3': 8, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'frozen_scope_sha256',
      '3': 9,
      '4': 1,
      '5': 12,
      '10': 'frozenScopeSha256'
    },
    {
      '1': 'native',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.BotRemovalNativeEvidence',
      '9': 0,
      '10': 'native'
    },
    {
      '1': 'legacy_installation',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.BotRemovalLegacyInstallationEvidence',
      '9': 0,
      '10': 'legacyInstallation'
    },
    {
      '1': 'orphan_whitelist',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.BotRemovalOrphanWhitelistEvidence',
      '9': 0,
      '10': 'orphanWhitelist'
    },
  ],
  '8': [
    {'1': 'evidence'},
  ],
};

/// Descriptor for `ApplyBotInstallationRemovalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyBotInstallationRemovalRequestDescriptor = $convert.base64Decode(
    'CiJBcHBseUJvdEluc3RhbGxhdGlvblJlbW92YWxSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlv'
    'bklkEh8KC2RlY2lzaW9uX2lkGAMgASgJUgpkZWNpc2lvbklkEicKD2RlY2lzaW9uX3NoYTI1Nh'
    'gEIAEoDFIOZGVjaXNpb25TaGEyNTYSGAoHcHVycG9zZRgFIAEoCVIHcHVycG9zZRIVCgZib3Rf'
    'aWQYBiABKAlSBWJvdElkEigKEGFjdG9yX3Byb2ZpbGVfaWQYByABKAlSDmFjdG9yUHJvZmlsZU'
    'lkEhkKCHNwYWNlX2lkGAggASgJUgdzcGFjZUlkEi4KE2Zyb3plbl9zY29wZV9zaGEyNTYYCSAB'
    'KAxSEWZyb3plblNjb3BlU2hhMjU2EkEKBm5hdGl2ZRgKIAEoCzInLnZvaWNlLnJvbGUudjEuQm'
    '90UmVtb3ZhbE5hdGl2ZUV2aWRlbmNlSABSBm5hdGl2ZRJmChNsZWdhY3lfaW5zdGFsbGF0aW9u'
    'GAsgASgLMjMudm9pY2Uucm9sZS52MS5Cb3RSZW1vdmFsTGVnYWN5SW5zdGFsbGF0aW9uRXZpZG'
    'VuY2VIAFISbGVnYWN5SW5zdGFsbGF0aW9uEl0KEG9ycGhhbl93aGl0ZWxpc3QYDCABKAsyMC52'
    'b2ljZS5yb2xlLnYxLkJvdFJlbW92YWxPcnBoYW5XaGl0ZWxpc3RFdmlkZW5jZUgAUg9vcnBoYW'
    '5XaGl0ZWxpc3RCCgoIZXZpZGVuY2U=');

@$core.Deprecated('Use botInstallationRemovalReceiptDescriptor instead')
const BotInstallationRemovalReceipt$json = {
  '1': 'BotInstallationRemovalReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'decision_id', '3': 4, '4': 1, '5': 9, '10': 'decisionId'},
    {'1': 'purpose', '3': 5, '4': 1, '5': 9, '10': 'purpose'},
    {'1': 'bot_id', '3': 6, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'actor_profile_id', '3': 7, '4': 1, '5': 9, '10': 'actorProfileId'},
    {'1': 'space_id', '3': 8, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'evidence_kind', '3': 9, '4': 1, '5': 9, '10': 'evidenceKind'},
    {'1': 'evidence_sha256', '3': 10, '4': 1, '5': 12, '10': 'evidenceSha256'},
    {
      '1': 'frozen_scope_sha256',
      '3': 11,
      '4': 1,
      '5': 12,
      '10': 'frozenScopeSha256'
    },
    {'1': 'request_sha256', '3': 12, '4': 1, '5': 12, '10': 'requestSha256'},
    {'1': 'participant', '3': 13, '4': 1, '5': 9, '10': 'participant'},
    {'1': 'outcome', '3': 14, '4': 1, '5': 9, '10': 'outcome'},
    {
      '1': 'completed_at',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'completedAt'
    },
    {
      '1': 'deleted_role_count',
      '3': 16,
      '4': 1,
      '5': 4,
      '10': 'deletedRoleCount'
    },
    {
      '1': 'removed_assignment_count',
      '3': 17,
      '4': 1,
      '5': 4,
      '10': 'removedAssignmentCount'
    },
  ],
};

/// Descriptor for `BotInstallationRemovalReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botInstallationRemovalReceiptDescriptor = $convert.base64Decode(
    'Ch1Cb3RJbnN0YWxsYXRpb25SZW1vdmFsUmVjZWlwdBIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
    'gNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVjZWlwdF9pZBgCIAEoCVIJcmVjZWlwdElkEiEKDG9w'
    'ZXJhdGlvbl9pZBgDIAEoCVILb3BlcmF0aW9uSWQSHwoLZGVjaXNpb25faWQYBCABKAlSCmRlY2'
    'lzaW9uSWQSGAoHcHVycG9zZRgFIAEoCVIHcHVycG9zZRIVCgZib3RfaWQYBiABKAlSBWJvdElk'
    'EigKEGFjdG9yX3Byb2ZpbGVfaWQYByABKAlSDmFjdG9yUHJvZmlsZUlkEhkKCHNwYWNlX2lkGA'
    'ggASgJUgdzcGFjZUlkEiMKDWV2aWRlbmNlX2tpbmQYCSABKAlSDGV2aWRlbmNlS2luZBInCg9l'
    'dmlkZW5jZV9zaGEyNTYYCiABKAxSDmV2aWRlbmNlU2hhMjU2Ei4KE2Zyb3plbl9zY29wZV9zaG'
    'EyNTYYCyABKAxSEWZyb3plblNjb3BlU2hhMjU2EiUKDnJlcXVlc3Rfc2hhMjU2GAwgASgMUg1y'
    'ZXF1ZXN0U2hhMjU2EiAKC3BhcnRpY2lwYW50GA0gASgJUgtwYXJ0aWNpcGFudBIYCgdvdXRjb2'
    '1lGA4gASgJUgdvdXRjb21lEj0KDGNvbXBsZXRlZF9hdBgPIAEoCzIaLmdvb2dsZS5wcm90b2J1'
    'Zi5UaW1lc3RhbXBSC2NvbXBsZXRlZEF0EiwKEmRlbGV0ZWRfcm9sZV9jb3VudBgQIAEoBFIQZG'
    'VsZXRlZFJvbGVDb3VudBI4ChhyZW1vdmVkX2Fzc2lnbm1lbnRfY291bnQYESABKARSFnJlbW92'
    'ZWRBc3NpZ25tZW50Q291bnQ=');

@$core.Deprecated('Use applyBotInstallationRemovalResponseDescriptor instead')
const ApplyBotInstallationRemovalResponse$json = {
  '1': 'ApplyBotInstallationRemovalResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.BotInstallationRemovalReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ApplyBotInstallationRemovalResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyBotInstallationRemovalResponseDescriptor =
    $convert.base64Decode(
        'CiNBcHBseUJvdEluc3RhbGxhdGlvblJlbW92YWxSZXNwb25zZRJGCgdyZWNlaXB0GAEgASgLMi'
        'wudm9pY2Uucm9sZS52MS5Cb3RJbnN0YWxsYXRpb25SZW1vdmFsUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated('Use registerBotGlobalTerminalRequestDescriptor instead')
const RegisterBotGlobalTerminalRequest$json = {
  '1': 'RegisterBotGlobalTerminalRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'bot_id', '3': 2, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'decision_id', '3': 3, '4': 1, '5': 9, '10': 'decisionId'},
  ],
};

/// Descriptor for `RegisterBotGlobalTerminalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerBotGlobalTerminalRequestDescriptor =
    $convert.base64Decode(
        'CiBSZWdpc3RlckJvdEdsb2JhbFRlcm1pbmFsUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
        'EgASgNUg9wcm90b2NvbFZlcnNpb24SFQoGYm90X2lkGAIgASgJUgVib3RJZBIfCgtkZWNpc2lv'
        'bl9pZBgDIAEoCVIKZGVjaXNpb25JZA==');

@$core.Deprecated('Use botGlobalTerminalReceiptDescriptor instead')
const BotGlobalTerminalReceipt$json = {
  '1': 'BotGlobalTerminalReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'participant', '3': 2, '4': 1, '5': 9, '10': 'participant'},
    {'1': 'bot_id', '3': 3, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'decision_id', '3': 4, '4': 1, '5': 9, '10': 'decisionId'},
    {'1': 'receipt_id', '3': 5, '4': 1, '5': 9, '10': 'receiptId'},
    {
      '1': 'registered_at',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'registeredAt'
    },
    {
      '1': 'outcome',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.role.v1.BotGlobalTerminalOutcome',
      '10': 'outcome'
    },
  ],
};

/// Descriptor for `BotGlobalTerminalReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botGlobalTerminalReceiptDescriptor = $convert.base64Decode(
    'ChhCb3RHbG9iYWxUZXJtaW5hbFJlY2VpcHQSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcH'
    'JvdG9jb2xWZXJzaW9uEiAKC3BhcnRpY2lwYW50GAIgASgJUgtwYXJ0aWNpcGFudBIVCgZib3Rf'
    'aWQYAyABKAlSBWJvdElkEh8KC2RlY2lzaW9uX2lkGAQgASgJUgpkZWNpc2lvbklkEh0KCnJlY2'
    'VpcHRfaWQYBSABKAlSCXJlY2VpcHRJZBI/Cg1yZWdpc3RlcmVkX2F0GAYgASgLMhouZ29vZ2xl'
    'LnByb3RvYnVmLlRpbWVzdGFtcFIMcmVnaXN0ZXJlZEF0EkEKB291dGNvbWUYByABKA4yJy52b2'
    'ljZS5yb2xlLnYxLkJvdEdsb2JhbFRlcm1pbmFsT3V0Y29tZVIHb3V0Y29tZQ==');

@$core.Deprecated('Use registerBotGlobalTerminalResponseDescriptor instead')
const RegisterBotGlobalTerminalResponse$json = {
  '1': 'RegisterBotGlobalTerminalResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.BotGlobalTerminalReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `RegisterBotGlobalTerminalResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerBotGlobalTerminalResponseDescriptor =
    $convert.base64Decode(
        'CiFSZWdpc3RlckJvdEdsb2JhbFRlcm1pbmFsUmVzcG9uc2USQQoHcmVjZWlwdBgBIAEoCzInLn'
        'ZvaWNlLnJvbGUudjEuQm90R2xvYmFsVGVybWluYWxSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use ensureDefaultMemberRoleRequestDescriptor instead')
const EnsureDefaultMemberRoleRequest$json = {
  '1': 'EnsureDefaultMemberRoleRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'membership_generation',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'membershipGeneration'
    },
    {
      '1': 'admission_operation_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'admissionOperationId'
    },
    {
      '1': 'admission_binding_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'admissionBindingSha256'
    },
  ],
};

/// Descriptor for `EnsureDefaultMemberRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ensureDefaultMemberRoleRequestDescriptor = $convert.base64Decode(
    'Ch5FbnN1cmVEZWZhdWx0TWVtYmVyUm9sZVJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIA'
    'EoDVIPcHJvdG9jb2xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEh0KCnByb2Zp'
    'bGVfaWQYAyABKAlSCXByb2ZpbGVJZBIzChVtZW1iZXJzaGlwX2dlbmVyYXRpb24YBCABKARSFG'
    '1lbWJlcnNoaXBHZW5lcmF0aW9uEjQKFmFkbWlzc2lvbl9vcGVyYXRpb25faWQYBSABKAlSFGFk'
    'bWlzc2lvbk9wZXJhdGlvbklkEjgKGGFkbWlzc2lvbl9iaW5kaW5nX3NoYTI1NhgGIAEoDFIWYW'
    'RtaXNzaW9uQmluZGluZ1NoYTI1Ng==');

@$core.Deprecated('Use defaultMemberRoleReceiptDescriptor instead')
const DefaultMemberRoleReceipt$json = {
  '1': 'DefaultMemberRoleReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'membership_generation',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'membershipGeneration'
    },
    {
      '1': 'admission_operation_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'admissionOperationId'
    },
    {
      '1': 'admission_binding_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'admissionBindingSha256'
    },
    {'1': 'role_id', '3': 7, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'receipt_sha256', '3': 8, '4': 1, '5': 12, '10': 'receiptSha256'},
  ],
};

/// Descriptor for `DefaultMemberRoleReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List defaultMemberRoleReceiptDescriptor = $convert.base64Decode(
    'ChhEZWZhdWx0TWVtYmVyUm9sZVJlY2VpcHQSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcH'
    'JvdG9jb2xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEh0KCnByb2ZpbGVfaWQY'
    'AyABKAlSCXByb2ZpbGVJZBIzChVtZW1iZXJzaGlwX2dlbmVyYXRpb24YBCABKARSFG1lbWJlcn'
    'NoaXBHZW5lcmF0aW9uEjQKFmFkbWlzc2lvbl9vcGVyYXRpb25faWQYBSABKAlSFGFkbWlzc2lv'
    'bk9wZXJhdGlvbklkEjgKGGFkbWlzc2lvbl9iaW5kaW5nX3NoYTI1NhgGIAEoDFIWYWRtaXNzaW'
    '9uQmluZGluZ1NoYTI1NhIXCgdyb2xlX2lkGAcgASgJUgZyb2xlSWQSJQoOcmVjZWlwdF9zaGEy'
    'NTYYCCABKAxSDXJlY2VpcHRTaGEyNTY=');

@$core.Deprecated('Use ensureDefaultMemberRoleResponseDescriptor instead')
const EnsureDefaultMemberRoleResponse$json = {
  '1': 'EnsureDefaultMemberRoleResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.DefaultMemberRoleReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `EnsureDefaultMemberRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ensureDefaultMemberRoleResponseDescriptor =
    $convert.base64Decode(
        'Ch9FbnN1cmVEZWZhdWx0TWVtYmVyUm9sZVJlc3BvbnNlEkEKB3JlY2VpcHQYASABKAsyJy52b2'
        'ljZS5yb2xlLnYxLkRlZmF1bHRNZW1iZXJSb2xlUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated('Use removeMemberRolesRequestDescriptor instead')
const RemoveMemberRolesRequest$json = {
  '1': 'RemoveMemberRolesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'removal_generation_floor',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'removalGenerationFloor'
    },
    {
      '1': 'removal_operation_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'removalOperationId'
    },
    {
      '1': 'removal_binding_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'removalBindingSha256'
    },
  ],
};

/// Descriptor for `RemoveMemberRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberRolesRequestDescriptor = $convert.base64Decode(
    'ChhSZW1vdmVNZW1iZXJSb2xlc1JlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcH'
    'JvdG9jb2xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEh0KCnByb2ZpbGVfaWQY'
    'AyABKAlSCXByb2ZpbGVJZBI4ChhyZW1vdmFsX2dlbmVyYXRpb25fZmxvb3IYBCABKARSFnJlbW'
    '92YWxHZW5lcmF0aW9uRmxvb3ISMAoUcmVtb3ZhbF9vcGVyYXRpb25faWQYBSABKAlSEnJlbW92'
    'YWxPcGVyYXRpb25JZBI0ChZyZW1vdmFsX2JpbmRpbmdfc2hhMjU2GAYgASgMUhRyZW1vdmFsQm'
    'luZGluZ1NoYTI1Ng==');

@$core.Deprecated('Use memberRolesRemovalReceiptDescriptor instead')
const MemberRolesRemovalReceipt$json = {
  '1': 'MemberRolesRemovalReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'removal_generation_floor',
      '3': 4,
      '4': 1,
      '5': 4,
      '10': 'removalGenerationFloor'
    },
    {
      '1': 'removal_operation_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'removalOperationId'
    },
    {
      '1': 'removal_binding_sha256',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'removalBindingSha256'
    },
    {'1': 'receipt_sha256', '3': 7, '4': 1, '5': 12, '10': 'receiptSha256'},
  ],
};

/// Descriptor for `MemberRolesRemovalReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberRolesRemovalReceiptDescriptor = $convert.base64Decode(
    'ChlNZW1iZXJSb2xlc1JlbW92YWxSZWNlaXB0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3'
    'Byb3RvY29sVmVyc2lvbhIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBIdCgpwcm9maWxlX2lk'
    'GAMgASgJUglwcm9maWxlSWQSOAoYcmVtb3ZhbF9nZW5lcmF0aW9uX2Zsb29yGAQgASgEUhZyZW'
    '1vdmFsR2VuZXJhdGlvbkZsb29yEjAKFHJlbW92YWxfb3BlcmF0aW9uX2lkGAUgASgJUhJyZW1v'
    'dmFsT3BlcmF0aW9uSWQSNAoWcmVtb3ZhbF9iaW5kaW5nX3NoYTI1NhgGIAEoDFIUcmVtb3ZhbE'
    'JpbmRpbmdTaGEyNTYSJQoOcmVjZWlwdF9zaGEyNTYYByABKAxSDXJlY2VpcHRTaGEyNTY=');

@$core.Deprecated('Use removeMemberRolesResponseDescriptor instead')
const RemoveMemberRolesResponse$json = {
  '1': 'RemoveMemberRolesResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.MemberRolesRemovalReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `RemoveMemberRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberRolesResponseDescriptor =
    $convert.base64Decode(
        'ChlSZW1vdmVNZW1iZXJSb2xlc1Jlc3BvbnNlEkIKB3JlY2VpcHQYASABKAsyKC52b2ljZS5yb2'
        'xlLnYxLk1lbWJlclJvbGVzUmVtb3ZhbFJlY2VpcHRSB3JlY2VpcHQ=');

@$core.Deprecated('Use bootstrapSpaceRolesRequestDescriptor instead')
const BootstrapSpaceRolesRequest$json = {
  '1': 'BootstrapSpaceRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'owner_profile_id', '3': 2, '4': 1, '5': 9, '10': 'ownerProfileId'},
  ],
};

/// Descriptor for `BootstrapSpaceRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bootstrapSpaceRolesRequestDescriptor =
    $convert.base64Decode(
        'ChpCb290c3RyYXBTcGFjZVJvbGVzUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZB'
        'IoChBvd25lcl9wcm9maWxlX2lkGAIgASgJUg5vd25lclByb2ZpbGVJZA==');

@$core.Deprecated('Use bootstrapSpaceRolesResponseDescriptor instead')
const BootstrapSpaceRolesResponse$json = {
  '1': 'BootstrapSpaceRolesResponse',
};

/// Descriptor for `BootstrapSpaceRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bootstrapSpaceRolesResponseDescriptor =
    $convert.base64Decode('ChtCb290c3RyYXBTcGFjZVJvbGVzUmVzcG9uc2U=');

@$core.Deprecated('Use deleteRolesCreatedByProfileRequestDescriptor instead')
const DeleteRolesCreatedByProfileRequest$json = {
  '1': 'DeleteRolesCreatedByProfileRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'created_by_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'createdByProfileId'
    },
  ],
};

/// Descriptor for `DeleteRolesCreatedByProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRolesCreatedByProfileRequestDescriptor =
    $convert.base64Decode(
        'CiJEZWxldGVSb2xlc0NyZWF0ZWRCeVByb2ZpbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUg'
        'dzcGFjZUlkEjEKFWNyZWF0ZWRfYnlfcHJvZmlsZV9pZBgCIAEoCVISY3JlYXRlZEJ5UHJvZmls'
        'ZUlk');

@$core.Deprecated('Use deleteRolesCreatedByProfileResponseDescriptor instead')
const DeleteRolesCreatedByProfileResponse$json = {
  '1': 'DeleteRolesCreatedByProfileResponse',
};

/// Descriptor for `DeleteRolesCreatedByProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRolesCreatedByProfileResponseDescriptor =
    $convert
        .base64Decode('CiNEZWxldGVSb2xlc0NyZWF0ZWRCeVByb2ZpbGVSZXNwb25zZQ==');

@$core.Deprecated('Use applyOwnershipTransferRequestDescriptor instead')
const ApplyOwnershipTransferRequest$json = {
  '1': 'ApplyOwnershipTransferRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'old_owner_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'oldOwnerProfileId'
    },
    {
      '1': 'new_owner_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `ApplyOwnershipTransferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyOwnershipTransferRequestDescriptor = $convert.base64Decode(
    'Ch1BcHBseU93bmVyc2hpcFRyYW5zZmVyUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2'
    'VJZBIvChRvbGRfb3duZXJfcHJvZmlsZV9pZBgCIAEoCVIRb2xkT3duZXJQcm9maWxlSWQSLwoU'
    'bmV3X293bmVyX3Byb2ZpbGVfaWQYAyABKAlSEW5ld093bmVyUHJvZmlsZUlkEiEKDG9wZXJhdG'
    'lvbl9pZBgEIAEoCVILb3BlcmF0aW9uSWQ=');

@$core.Deprecated('Use compensateOwnershipTransferRequestDescriptor instead')
const CompensateOwnershipTransferRequest$json = {
  '1': 'CompensateOwnershipTransferRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'old_owner_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'oldOwnerProfileId'
    },
    {
      '1': 'new_owner_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `CompensateOwnershipTransferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List compensateOwnershipTransferRequestDescriptor =
    $convert.base64Decode(
        'CiJDb21wZW5zYXRlT3duZXJzaGlwVHJhbnNmZXJSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUg'
        'dzcGFjZUlkEi8KFG9sZF9vd25lcl9wcm9maWxlX2lkGAIgASgJUhFvbGRPd25lclByb2ZpbGVJ'
        'ZBIvChRuZXdfb3duZXJfcHJvZmlsZV9pZBgDIAEoCVIRbmV3T3duZXJQcm9maWxlSWQSIQoMb3'
        'BlcmF0aW9uX2lkGAQgASgJUgtvcGVyYXRpb25JZA==');

@$core.Deprecated('Use applyOwnershipTransferResponseDescriptor instead')
const ApplyOwnershipTransferResponse$json = {
  '1': 'ApplyOwnershipTransferResponse',
  '2': [
    {
      '1': 'current_owner_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'currentOwnerProfileId'
    },
  ],
};

/// Descriptor for `ApplyOwnershipTransferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyOwnershipTransferResponseDescriptor =
    $convert.base64Decode(
        'Ch5BcHBseU93bmVyc2hpcFRyYW5zZmVyUmVzcG9uc2USNwoYY3VycmVudF9vd25lcl9wcm9maW'
        'xlX2lkGAEgASgJUhVjdXJyZW50T3duZXJQcm9maWxlSWQ=');

@$core.Deprecated('Use compensateOwnershipTransferResponseDescriptor instead')
const CompensateOwnershipTransferResponse$json = {
  '1': 'CompensateOwnershipTransferResponse',
  '2': [
    {
      '1': 'current_owner_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'currentOwnerProfileId'
    },
  ],
};

/// Descriptor for `CompensateOwnershipTransferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List compensateOwnershipTransferResponseDescriptor =
    $convert.base64Decode(
        'CiNDb21wZW5zYXRlT3duZXJzaGlwVHJhbnNmZXJSZXNwb25zZRI3ChhjdXJyZW50X293bmVyX3'
        'Byb2ZpbGVfaWQYASABKAlSFWN1cnJlbnRPd25lclByb2ZpbGVJZA==');

@$core.Deprecated('Use ownershipTransferIntentDescriptor instead')
const OwnershipTransferIntent$json = {
  '1': 'OwnershipTransferIntent',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'old_owner_profile_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'oldOwnerProfileId'
    },
    {
      '1': 'new_owner_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `OwnershipTransferIntent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ownershipTransferIntentDescriptor = $convert.base64Decode(
    'ChdPd25lcnNoaXBUcmFuc2ZlckludGVudBIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm'
    '90b2NvbFZlcnNpb24SGQoIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQSLwoUb2xkX293bmVyX3By'
    'b2ZpbGVfaWQYAyABKAlSEW9sZE93bmVyUHJvZmlsZUlkEi8KFG5ld19vd25lcl9wcm9maWxlX2'
    'lkGAQgASgJUhFuZXdPd25lclByb2ZpbGVJZBIhCgxvcGVyYXRpb25faWQYBSABKAlSC29wZXJh'
    'dGlvbklk');

@$core.Deprecated('Use ownershipTransferReceiptDescriptor instead')
const OwnershipTransferReceipt$json = {
  '1': 'OwnershipTransferReceipt',
  '2': [
    {
      '1': 'intent',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferIntent',
      '10': 'intent'
    },
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.role.v1.OwnershipTransferState',
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

/// Descriptor for `OwnershipTransferReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ownershipTransferReceiptDescriptor = $convert.base64Decode(
    'ChhPd25lcnNoaXBUcmFuc2ZlclJlY2VpcHQSPgoGaW50ZW50GAEgASgLMiYudm9pY2Uucm9sZS'
    '52MS5Pd25lcnNoaXBUcmFuc2ZlckludGVudFIGaW50ZW50EjsKBXN0YXRlGAIgASgOMiUudm9p'
    'Y2Uucm9sZS52MS5Pd25lcnNoaXBUcmFuc2ZlclN0YXRlUgVzdGF0ZRI3ChhjdXJyZW50X293bm'
    'VyX3Byb2ZpbGVfaWQYAyABKAlSFWN1cnJlbnRPd25lclByb2ZpbGVJZA==');

@$core
    .Deprecated('Use getOwnershipTransferCapabilitiesRequestDescriptor instead')
const GetOwnershipTransferCapabilitiesRequest$json = {
  '1': 'GetOwnershipTransferCapabilitiesRequest',
};

/// Descriptor for `GetOwnershipTransferCapabilitiesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOwnershipTransferCapabilitiesRequestDescriptor =
    $convert.base64Decode(
        'CidHZXRPd25lcnNoaXBUcmFuc2ZlckNhcGFiaWxpdGllc1JlcXVlc3Q=');

@$core.Deprecated(
    'Use getOwnershipTransferCapabilitiesResponseDescriptor instead')
const GetOwnershipTransferCapabilitiesResponse$json = {
  '1': 'GetOwnershipTransferCapabilitiesResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'supported_methods',
      '3': 2,
      '4': 3,
      '5': 9,
      '10': 'supportedMethods'
    },
  ],
};

/// Descriptor for `GetOwnershipTransferCapabilitiesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOwnershipTransferCapabilitiesResponseDescriptor =
    $convert.base64Decode(
        'CihHZXRPd25lcnNoaXBUcmFuc2ZlckNhcGFiaWxpdGllc1Jlc3BvbnNlEikKEHByb3RvY29sX3'
        'ZlcnNpb24YASABKA1SD3Byb3RvY29sVmVyc2lvbhIrChFzdXBwb3J0ZWRfbWV0aG9kcxgCIAMo'
        'CVIQc3VwcG9ydGVkTWV0aG9kcw==');

@$core.Deprecated('Use prepareOwnershipTransferRequestDescriptor instead')
const PrepareOwnershipTransferRequest$json = {
  '1': 'PrepareOwnershipTransferRequest',
  '2': [
    {
      '1': 'intent',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferIntent',
      '10': 'intent'
    },
  ],
};

/// Descriptor for `PrepareOwnershipTransferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List prepareOwnershipTransferRequestDescriptor =
    $convert.base64Decode(
        'Ch9QcmVwYXJlT3duZXJzaGlwVHJhbnNmZXJSZXF1ZXN0Ej4KBmludGVudBgBIAEoCzImLnZvaW'
        'NlLnJvbGUudjEuT3duZXJzaGlwVHJhbnNmZXJJbnRlbnRSBmludGVudA==');

@$core.Deprecated('Use prepareOwnershipTransferResponseDescriptor instead')
const PrepareOwnershipTransferResponse$json = {
  '1': 'PrepareOwnershipTransferResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `PrepareOwnershipTransferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List prepareOwnershipTransferResponseDescriptor =
    $convert.base64Decode(
        'CiBQcmVwYXJlT3duZXJzaGlwVHJhbnNmZXJSZXNwb25zZRJBCgdyZWNlaXB0GAEgASgLMicudm'
        '9pY2Uucm9sZS52MS5Pd25lcnNoaXBUcmFuc2ZlclJlY2VpcHRSB3JlY2VpcHQ=');

@$core.Deprecated('Use finalizeOwnershipTransferRequestDescriptor instead')
const FinalizeOwnershipTransferRequest$json = {
  '1': 'FinalizeOwnershipTransferRequest',
  '2': [
    {
      '1': 'intent',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferIntent',
      '10': 'intent'
    },
  ],
};

/// Descriptor for `FinalizeOwnershipTransferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List finalizeOwnershipTransferRequestDescriptor =
    $convert.base64Decode(
        'CiBGaW5hbGl6ZU93bmVyc2hpcFRyYW5zZmVyUmVxdWVzdBI+CgZpbnRlbnQYASABKAsyJi52b2'
        'ljZS5yb2xlLnYxLk93bmVyc2hpcFRyYW5zZmVySW50ZW50UgZpbnRlbnQ=');

@$core.Deprecated('Use finalizeOwnershipTransferResponseDescriptor instead')
const FinalizeOwnershipTransferResponse$json = {
  '1': 'FinalizeOwnershipTransferResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `FinalizeOwnershipTransferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List finalizeOwnershipTransferResponseDescriptor =
    $convert.base64Decode(
        'CiFGaW5hbGl6ZU93bmVyc2hpcFRyYW5zZmVyUmVzcG9uc2USQQoHcmVjZWlwdBgBIAEoCzInLn'
        'ZvaWNlLnJvbGUudjEuT3duZXJzaGlwVHJhbnNmZXJSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use abortOwnershipTransferRequestDescriptor instead')
const AbortOwnershipTransferRequest$json = {
  '1': 'AbortOwnershipTransferRequest',
  '2': [
    {
      '1': 'intent',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferIntent',
      '10': 'intent'
    },
  ],
};

/// Descriptor for `AbortOwnershipTransferRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List abortOwnershipTransferRequestDescriptor =
    $convert.base64Decode(
        'Ch1BYm9ydE93bmVyc2hpcFRyYW5zZmVyUmVxdWVzdBI+CgZpbnRlbnQYASABKAsyJi52b2ljZS'
        '5yb2xlLnYxLk93bmVyc2hpcFRyYW5zZmVySW50ZW50UgZpbnRlbnQ=');

@$core.Deprecated('Use abortOwnershipTransferResponseDescriptor instead')
const AbortOwnershipTransferResponse$json = {
  '1': 'AbortOwnershipTransferResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OwnershipTransferReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `AbortOwnershipTransferResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List abortOwnershipTransferResponseDescriptor =
    $convert.base64Decode(
        'Ch5BYm9ydE93bmVyc2hpcFRyYW5zZmVyUmVzcG9uc2USQQoHcmVjZWlwdBgBIAEoCzInLnZvaW'
        'NlLnJvbGUudjEuT3duZXJzaGlwVHJhbnNmZXJSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use roleDescriptor instead')
const Role$json = {
  '1': 'Role',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'permissions_mask', '3': 4, '4': 1, '5': 4, '10': 'permissionsMask'},
    {'1': 'position', '3': 5, '4': 1, '5': 5, '10': 'position'},
    {'1': 'managed', '3': 6, '4': 1, '5': 8, '10': 'managed'},
    {
      '1': 'created_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'created_by_profile_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'createdByProfileId',
      '17': true
    },
    {'1': 'color', '3': 9, '4': 1, '5': 9, '9': 1, '10': 'color', '17': true},
    {'1': 'is_mentionable', '3': 10, '4': 1, '5': 8, '10': 'isMentionable'},
  ],
  '8': [
    {'1': '_created_by_profile_id'},
    {'1': '_color'},
  ],
};

/// Descriptor for `Role`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleDescriptor = $convert.base64Decode(
    'CgRSb2xlEg4KAmlkGAEgASgJUgJpZBIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZBISCgRuYW'
    '1lGAMgASgJUgRuYW1lEikKEHBlcm1pc3Npb25zX21hc2sYBCABKARSD3Blcm1pc3Npb25zTWFz'
    'axIaCghwb3NpdGlvbhgFIAEoBVIIcG9zaXRpb24SGAoHbWFuYWdlZBgGIAEoCFIHbWFuYWdlZB'
    'I5CgpjcmVhdGVkX2F0GAcgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRl'
    'ZEF0EjYKFWNyZWF0ZWRfYnlfcHJvZmlsZV9pZBgIIAEoCUgAUhJjcmVhdGVkQnlQcm9maWxlSW'
    'SIAQESGQoFY29sb3IYCSABKAlIAVIFY29sb3KIAQESJQoOaXNfbWVudGlvbmFibGUYCiABKAhS'
    'DWlzTWVudGlvbmFibGVCGAoWX2NyZWF0ZWRfYnlfcHJvZmlsZV9pZEIICgZfY29sb3I=');

@$core.Deprecated('Use createRoleRequestDescriptor instead')
const CreateRoleRequest$json = {
  '1': 'CreateRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'permissions_mask', '3': 3, '4': 1, '5': 4, '10': 'permissionsMask'},
    {'1': 'position', '3': 4, '4': 1, '5': 5, '10': 'position'},
  ],
};

/// Descriptor for `CreateRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createRoleRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBISCgRuYW1lGA'
    'IgASgJUgRuYW1lEikKEHBlcm1pc3Npb25zX21hc2sYAyABKARSD3Blcm1pc3Npb25zTWFzaxIa'
    'Cghwb3NpdGlvbhgEIAEoBVIIcG9zaXRpb24=');

@$core.Deprecated('Use updateRoleRequestDescriptor instead')
const UpdateRoleRequest$json = {
  '1': 'UpdateRoleRequest',
  '2': [
    {'1': 'role_id', '3': 1, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'permissions_mask',
      '3': 3,
      '4': 1,
      '5': 4,
      '9': 1,
      '10': 'permissionsMask',
      '17': true
    },
    {
      '1': 'position',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 2,
      '10': 'position',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_permissions_mask'},
    {'1': '_position'},
  ],
};

/// Descriptor for `UpdateRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateRoleRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVSb2xlUmVxdWVzdBIXCgdyb2xlX2lkGAEgASgJUgZyb2xlSWQSFwoEbmFtZRgCIA'
    'EoCUgAUgRuYW1liAEBEi4KEHBlcm1pc3Npb25zX21hc2sYAyABKARIAVIPcGVybWlzc2lvbnNN'
    'YXNriAEBEh8KCHBvc2l0aW9uGAQgASgFSAJSCHBvc2l0aW9uiAEBQgcKBV9uYW1lQhMKEV9wZX'
    'JtaXNzaW9uc19tYXNrQgsKCV9wb3NpdGlvbg==');

@$core.Deprecated('Use deleteRoleRequestDescriptor instead')
const DeleteRoleRequest$json = {
  '1': 'DeleteRoleRequest',
  '2': [
    {'1': 'role_id', '3': 1, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `DeleteRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRoleRequestDescriptor = $convert.base64Decode(
    'ChFEZWxldGVSb2xlUmVxdWVzdBIXCgdyb2xlX2lkGAEgASgJUgZyb2xlSWQ=');

@$core.Deprecated('Use listRolesRequestDescriptor instead')
const ListRolesRequest$json = {
  '1': 'ListRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRolesRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0Um9sZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlk');

@$core.Deprecated('Use roleListDescriptor instead')
const RoleList$json = {
  '1': 'RoleList',
  '2': [
    {
      '1': 'roles',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'roles'
    },
  ],
};

/// Descriptor for `RoleList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List roleListDescriptor = $convert.base64Decode(
    'CghSb2xlTGlzdBIpCgVyb2xlcxgBIAMoCzITLnZvaWNlLnJvbGUudjEuUm9sZVIFcm9sZXM=');

@$core.Deprecated('Use reorderRolesRequestDescriptor instead')
const ReorderRolesRequest$json = {
  '1': 'ReorderRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'ordered_role_ids', '3': 2, '4': 3, '5': 9, '10': 'orderedRoleIds'},
  ],
};

/// Descriptor for `ReorderRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderRolesRequestDescriptor = $convert.base64Decode(
    'ChNSZW9yZGVyUm9sZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEigKEG9yZG'
    'VyZWRfcm9sZV9pZHMYAiADKAlSDm9yZGVyZWRSb2xlSWRz');

@$core.Deprecated('Use assignRoleRequestDescriptor instead')
const AssignRoleRequest$json = {
  '1': 'AssignRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `AssignRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignRoleRequestDescriptor = $convert.base64Decode(
    'ChFBc3NpZ25Sb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use revokeRoleRequestDescriptor instead')
const RevokeRoleRequest$json = {
  '1': 'RevokeRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RevokeRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeRoleRequestDescriptor = $convert.base64Decode(
    'ChFSZXZva2VSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use getMemberRolesRequestDescriptor instead')
const GetMemberRolesRequest$json = {
  '1': 'GetMemberRolesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetMemberRolesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberRolesRequestDescriptor = $convert.base64Decode(
    'ChVHZXRNZW1iZXJSb2xlc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQSHQoKcH'
    'JvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use setChatOverrideRequestDescriptor instead')
const SetChatOverrideRequest$json = {
  '1': 'SetChatOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetChatOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatOverrideRequestDescriptor = $convert.base64Decode(
    'ChZTZXRDaGF0T3ZlcnJpZGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEioKBG'
    'NoYXQYAiABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSGwoJZGVueV9tYXNrGAMg'
    'ASgEUghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd01hc2sSFwoHcm9sZV9pZB'
    'gFIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use removeChatOverrideRequestDescriptor instead')
const RemoveChatOverrideRequest$json = {
  '1': 'RemoveChatOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RemoveChatOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeChatOverrideRequestDescriptor = $convert.base64Decode(
    'ChlSZW1vdmVDaGF0T3ZlcnJpZGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEi'
    'oKBGNoYXQYAiABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSFwoHcm9sZV9pZBgD'
    'IAEoCVIGcm9sZUlk');

@$core.Deprecated('Use getChatOverridesRequestDescriptor instead')
const GetChatOverridesRequest$json = {
  '1': 'GetChatOverridesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'filter_chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'filterChat',
      '17': true
    },
  ],
  '8': [
    {'1': '_filter_chat'},
  ],
};

/// Descriptor for `GetChatOverridesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatOverridesRequestDescriptor = $convert.base64Decode(
    'ChdHZXRDaGF0T3ZlcnJpZGVzUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBI8Cg'
    'tmaWx0ZXJfY2hhdBgCIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZkgAUgpmaWx0ZXJDaGF0'
    'iAEBQg4KDF9maWx0ZXJfY2hhdA==');

@$core.Deprecated('Use setVoiceRoomOverrideRequestDescriptor instead')
const SetVoiceRoomOverrideRequest$json = {
  '1': 'SetVoiceRoomOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetVoiceRoomOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVoiceRoomOverrideRequestDescriptor = $convert.base64Decode(
    'ChtTZXRWb2ljZVJvb21PdmVycmlkZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSW'
    'QSIgoNdm9pY2Vfcm9vbV9pZBgCIAEoCVILdm9pY2VSb29tSWQSGwoJZGVueV9tYXNrGAMgASgE'
    'UghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd01hc2sSFwoHcm9sZV9pZBgFIA'
    'EoCVIGcm9sZUlk');

@$core.Deprecated('Use removeVoiceRoomOverrideRequestDescriptor instead')
const RemoveVoiceRoomOverrideRequest$json = {
  '1': 'RemoveVoiceRoomOverrideRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RemoveVoiceRoomOverrideRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeVoiceRoomOverrideRequestDescriptor =
    $convert.base64Decode(
        'Ch5SZW1vdmVWb2ljZVJvb21PdmVycmlkZVJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYW'
        'NlSWQSIgoNdm9pY2Vfcm9vbV9pZBgCIAEoCVILdm9pY2VSb29tSWQSFwoHcm9sZV9pZBgDIAEo'
        'CVIGcm9sZUlk');

@$core.Deprecated('Use getVoiceRoomOverridesRequestDescriptor instead')
const GetVoiceRoomOverridesRequest$json = {
  '1': 'GetVoiceRoomOverridesRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'voice_room_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `GetVoiceRoomOverridesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceRoomOverridesRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRWb2ljZVJvb21PdmVycmlkZXNSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZU'
        'lkEicKDXZvaWNlX3Jvb21faWQYAiABKAlIAFILdm9pY2VSb29tSWSIAQFCEAoOX3ZvaWNlX3Jv'
        'b21faWQ=');

@$core.Deprecated('Use overrideListDescriptor instead')
const OverrideList$json = {
  '1': 'OverrideList',
  '2': [
    {
      '1': 'overrides',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.role.v1.PermissionOverride',
      '10': 'overrides'
    },
  ],
};

/// Descriptor for `OverrideList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List overrideListDescriptor = $convert.base64Decode(
    'CgxPdmVycmlkZUxpc3QSPwoJb3ZlcnJpZGVzGAEgAygLMiEudm9pY2Uucm9sZS52MS5QZXJtaX'
    'NzaW9uT3ZlcnJpZGVSCW92ZXJyaWRlcw==');

@$core.Deprecated('Use permissionOverrideDescriptor instead')
const PermissionOverride$json = {
  '1': 'PermissionOverride',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
    {'1': 'deny_mask', '3': 3, '4': 1, '5': 4, '10': 'denyMask'},
    {'1': 'allow_mask', '3': 4, '4': 1, '5': 4, '10': 'allowMask'},
    {'1': 'role_id', '3': 5, '4': 1, '5': 9, '10': 'roleId'},
    {'1': 'role_name', '3': 6, '4': 1, '5': 9, '10': 'roleName'},
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `PermissionOverride`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List permissionOverrideDescriptor = $convert.base64Decode(
    'ChJQZXJtaXNzaW9uT3ZlcnJpZGUSLwoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZkgAUgRjaGF0iAEBEicKDXZvaWNlX3Jvb21faWQYAiABKAlIAVILdm9pY2VSb29tSWSIAQES'
    'GwoJZGVueV9tYXNrGAMgASgEUghkZW55TWFzaxIdCgphbGxvd19tYXNrGAQgASgEUglhbGxvd0'
    '1hc2sSFwoHcm9sZV9pZBgFIAEoCVIGcm9sZUlkEhsKCXJvbGVfbmFtZRgGIAEoCVIIcm9sZU5h'
    'bWVCBwoFX2NoYXRCEAoOX3ZvaWNlX3Jvb21faWQ=');

@$core.Deprecated('Use setDefaultJoinRoleRequestDescriptor instead')
const SetDefaultJoinRoleRequest$json = {
  '1': 'SetDefaultJoinRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'role_id', '3': 2, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `SetDefaultJoinRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setDefaultJoinRoleRequestDescriptor =
    $convert.base64Decode(
        'ChlTZXREZWZhdWx0Sm9pblJvbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh'
        'cKB3JvbGVfaWQYAiABKAlSBnJvbGVJZA==');

@$core.Deprecated('Use getDefaultJoinRoleRequestDescriptor instead')
const GetDefaultJoinRoleRequest$json = {
  '1': 'GetDefaultJoinRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `GetDefaultJoinRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDefaultJoinRoleRequestDescriptor =
    $convert.base64Decode(
        'ChlHZXREZWZhdWx0Sm9pblJvbGVSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlk');

@$core.Deprecated('Use checkPermissionRequestDescriptor instead')
const CheckPermissionRequest$json = {
  '1': 'CheckPermissionRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'permission_name', '3': 3, '4': 1, '5': 9, '10': 'permissionName'},
    {
      '1': 'chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `CheckPermissionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkPermissionRequestDescriptor = $convert.base64Decode(
    'ChZDaGVja1Blcm1pc3Npb25SZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUgdzcGFjZUlkEh0KCn'
    'Byb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBInCg9wZXJtaXNzaW9uX25hbWUYAyABKAlSDnBl'
    'cm1pc3Npb25OYW1lEi8KBGNoYXQYBCABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZIAFIEY2'
    'hhdIgBARInCg12b2ljZV9yb29tX2lkGAUgASgJSAFSC3ZvaWNlUm9vbUlkiAEBQgcKBV9jaGF0'
    'QhAKDl92b2ljZV9yb29tX2lk');

@$core.Deprecated('Use checkPermissionResponseDescriptor instead')
const CheckPermissionResponse$json = {
  '1': 'CheckPermissionResponse',
  '2': [
    {'1': 'allowed', '3': 1, '4': 1, '5': 8, '10': 'allowed'},
  ],
};

/// Descriptor for `CheckPermissionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkPermissionResponseDescriptor =
    $convert.base64Decode(
        'ChdDaGVja1Blcm1pc3Npb25SZXNwb25zZRIYCgdhbGxvd2VkGAEgASgIUgdhbGxvd2Vk');

@$core.Deprecated('Use resolveVoiceRoomGrantsRequestDescriptor instead')
const ResolveVoiceRoomGrantsRequest$json = {
  '1': 'ResolveVoiceRoomGrantsRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `ResolveVoiceRoomGrantsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveVoiceRoomGrantsRequestDescriptor =
    $convert.base64Decode(
        'Ch1SZXNvbHZlVm9pY2VSb29tR3JhbnRzUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2'
        'VJZBIiCg12b2ljZV9yb29tX2lkGAIgASgJUgt2b2ljZVJvb21JZBIdCgpwcm9maWxlX2lkGAMg'
        'ASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use voiceRoomGrantsDescriptor instead')
const VoiceRoomGrants$json = {
  '1': 'VoiceRoomGrants',
  '2': [
    {'1': 'can_join', '3': 1, '4': 1, '5': 8, '10': 'canJoin'},
    {'1': 'can_publish_audio', '3': 2, '4': 1, '5': 8, '10': 'canPublishAudio'},
    {'1': 'can_publish_video', '3': 3, '4': 1, '5': 8, '10': 'canPublishVideo'},
    {
      '1': 'can_publish_screen_share',
      '3': 4,
      '4': 1,
      '5': 8,
      '10': 'canPublishScreenShare'
    },
    {'1': 'can_subscribe', '3': 5, '4': 1, '5': 8, '10': 'canSubscribe'},
    {'1': 'can_mute_others', '3': 6, '4': 1, '5': 8, '10': 'canMuteOthers'},
    {'1': 'can_deafen_others', '3': 7, '4': 1, '5': 8, '10': 'canDeafenOthers'},
    {'1': 'can_move_others', '3': 8, '4': 1, '5': 8, '10': 'canMoveOthers'},
    {'1': 'can_use_ptt', '3': 9, '4': 1, '5': 8, '10': 'canUsePtt'},
    {'1': 'priority_speaker', '3': 10, '4': 1, '5': 8, '10': 'prioritySpeaker'},
  ],
};

/// Descriptor for `VoiceRoomGrants`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceRoomGrantsDescriptor = $convert.base64Decode(
    'Cg9Wb2ljZVJvb21HcmFudHMSGQoIY2FuX2pvaW4YASABKAhSB2NhbkpvaW4SKgoRY2FuX3B1Ym'
    'xpc2hfYXVkaW8YAiABKAhSD2NhblB1Ymxpc2hBdWRpbxIqChFjYW5fcHVibGlzaF92aWRlbxgD'
    'IAEoCFIPY2FuUHVibGlzaFZpZGVvEjcKGGNhbl9wdWJsaXNoX3NjcmVlbl9zaGFyZRgEIAEoCF'
    'IVY2FuUHVibGlzaFNjcmVlblNoYXJlEiMKDWNhbl9zdWJzY3JpYmUYBSABKAhSDGNhblN1YnNj'
    'cmliZRImCg9jYW5fbXV0ZV9vdGhlcnMYBiABKAhSDWNhbk11dGVPdGhlcnMSKgoRY2FuX2RlYW'
    'Zlbl9vdGhlcnMYByABKAhSD2NhbkRlYWZlbk90aGVycxImCg9jYW5fbW92ZV9vdGhlcnMYCCAB'
    'KAhSDWNhbk1vdmVPdGhlcnMSHgoLY2FuX3VzZV9wdHQYCSABKAhSCWNhblVzZVB0dBIpChBwcm'
    'lvcml0eV9zcGVha2VyGAogASgIUg9wcmlvcml0eVNwZWFrZXI=');

@$core.Deprecated('Use resolveVoiceRoomGrantsResponseDescriptor instead')
const ResolveVoiceRoomGrantsResponse$json = {
  '1': 'ResolveVoiceRoomGrantsResponse',
  '2': [
    {
      '1': 'grants',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.VoiceRoomGrants',
      '10': 'grants'
    },
    {'1': 'policy_epoch', '3': 2, '4': 1, '5': 4, '10': 'policyEpoch'},
  ],
};

/// Descriptor for `ResolveVoiceRoomGrantsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveVoiceRoomGrantsResponseDescriptor =
    $convert.base64Decode(
        'Ch5SZXNvbHZlVm9pY2VSb29tR3JhbnRzUmVzcG9uc2USNgoGZ3JhbnRzGAEgASgLMh4udm9pY2'
        'Uucm9sZS52MS5Wb2ljZVJvb21HcmFudHNSBmdyYW50cxIhCgxwb2xpY3lfZXBvY2gYAiABKARS'
        'C3BvbGljeUVwb2No');

@$core.Deprecated('Use getEffectivePermissionsRequestDescriptor instead')
const GetEffectivePermissionsRequest$json = {
  '1': 'GetEffectivePermissionsRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'chat',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'chat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_chat'},
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `GetEffectivePermissionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getEffectivePermissionsRequestDescriptor =
    $convert.base64Decode(
        'Ch5HZXRFZmZlY3RpdmVQZXJtaXNzaW9uc1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYW'
        'NlSWQSHQoKcHJvZmlsZV9pZBgCIAEoCVIJcHJvZmlsZUlkEi8KBGNoYXQYAyABKAsyFi52b2lj'
        'ZS5jaGF0LnYxLkNoYXRSZWZIAFIEY2hhdIgBARInCg12b2ljZV9yb29tX2lkGAQgASgJSAFSC3'
        'ZvaWNlUm9vbUlkiAEBQgcKBV9jaGF0QhAKDl92b2ljZV9yb29tX2lk');

@$core.Deprecated('Use permissionSetDescriptor instead')
const PermissionSet$json = {
  '1': 'PermissionSet',
  '2': [
    {'1': 'effective_mask', '3': 1, '4': 1, '5': 4, '10': 'effectiveMask'},
    {'1': 'permission_names', '3': 2, '4': 3, '5': 9, '10': 'permissionNames'},
  ],
};

/// Descriptor for `PermissionSet`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List permissionSetDescriptor = $convert.base64Decode(
    'Cg1QZXJtaXNzaW9uU2V0EiUKDmVmZmVjdGl2ZV9tYXNrGAEgASgEUg1lZmZlY3RpdmVNYXNrEi'
    'kKEHBlcm1pc3Npb25fbmFtZXMYAiADKAlSD3Blcm1pc3Npb25OYW1lcw==');

@$core.Deprecated('Use createRoleResponseDescriptor instead')
const CreateRoleResponse$json = {
  '1': 'CreateRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `CreateRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createRoleResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVSb2xlUmVzcG9uc2USJwoEcm9sZRgBIAEoCzITLnZvaWNlLnJvbGUudjEuUm9sZV'
    'IEcm9sZQ==');

@$core.Deprecated('Use updateRoleResponseDescriptor instead')
const UpdateRoleResponse$json = {
  '1': 'UpdateRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `UpdateRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateRoleResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVSb2xlUmVzcG9uc2USJwoEcm9sZRgBIAEoCzITLnZvaWNlLnJvbGUudjEuUm9sZV'
    'IEcm9sZQ==');

@$core.Deprecated('Use deleteRoleResponseDescriptor instead')
const DeleteRoleResponse$json = {
  '1': 'DeleteRoleResponse',
};

/// Descriptor for `DeleteRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteRoleResponseDescriptor =
    $convert.base64Decode('ChJEZWxldGVSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use listRolesResponseDescriptor instead')
const ListRolesResponse$json = {
  '1': 'ListRolesResponse',
  '2': [
    {
      '1': 'role_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.RoleList',
      '10': 'roleList'
    },
  ],
};

/// Descriptor for `ListRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listRolesResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0Um9sZXNSZXNwb25zZRI0Cglyb2xlX2xpc3QYASABKAsyFy52b2ljZS5yb2xlLnYxLl'
    'JvbGVMaXN0Ughyb2xlTGlzdA==');

@$core.Deprecated('Use reorderRolesResponseDescriptor instead')
const ReorderRolesResponse$json = {
  '1': 'ReorderRolesResponse',
};

/// Descriptor for `ReorderRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reorderRolesResponseDescriptor =
    $convert.base64Decode('ChRSZW9yZGVyUm9sZXNSZXNwb25zZQ==');

@$core.Deprecated('Use assignRoleResponseDescriptor instead')
const AssignRoleResponse$json = {
  '1': 'AssignRoleResponse',
};

/// Descriptor for `AssignRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignRoleResponseDescriptor =
    $convert.base64Decode('ChJBc3NpZ25Sb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use revokeRoleResponseDescriptor instead')
const RevokeRoleResponse$json = {
  '1': 'RevokeRoleResponse',
};

/// Descriptor for `RevokeRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeRoleResponseDescriptor =
    $convert.base64Decode('ChJSZXZva2VSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use getMemberRolesResponseDescriptor instead')
const GetMemberRolesResponse$json = {
  '1': 'GetMemberRolesResponse',
  '2': [
    {
      '1': 'role_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.RoleList',
      '10': 'roleList'
    },
  ],
};

/// Descriptor for `GetMemberRolesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMemberRolesResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRNZW1iZXJSb2xlc1Jlc3BvbnNlEjQKCXJvbGVfbGlzdBgBIAEoCzIXLnZvaWNlLnJvbG'
        'UudjEuUm9sZUxpc3RSCHJvbGVMaXN0');

@$core.Deprecated('Use setChatOverrideResponseDescriptor instead')
const SetChatOverrideResponse$json = {
  '1': 'SetChatOverrideResponse',
};

/// Descriptor for `SetChatOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatOverrideResponseDescriptor =
    $convert.base64Decode('ChdTZXRDaGF0T3ZlcnJpZGVSZXNwb25zZQ==');

@$core.Deprecated('Use removeChatOverrideResponseDescriptor instead')
const RemoveChatOverrideResponse$json = {
  '1': 'RemoveChatOverrideResponse',
};

/// Descriptor for `RemoveChatOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeChatOverrideResponseDescriptor =
    $convert.base64Decode('ChpSZW1vdmVDaGF0T3ZlcnJpZGVSZXNwb25zZQ==');

@$core.Deprecated('Use getChatOverridesResponseDescriptor instead')
const GetChatOverridesResponse$json = {
  '1': 'GetChatOverridesResponse',
  '2': [
    {
      '1': 'override_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OverrideList',
      '10': 'overrideList'
    },
  ],
};

/// Descriptor for `GetChatOverridesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatOverridesResponseDescriptor =
    $convert.base64Decode(
        'ChhHZXRDaGF0T3ZlcnJpZGVzUmVzcG9uc2USQAoNb3ZlcnJpZGVfbGlzdBgBIAEoCzIbLnZvaW'
        'NlLnJvbGUudjEuT3ZlcnJpZGVMaXN0UgxvdmVycmlkZUxpc3Q=');

@$core.Deprecated('Use setVoiceRoomOverrideResponseDescriptor instead')
const SetVoiceRoomOverrideResponse$json = {
  '1': 'SetVoiceRoomOverrideResponse',
};

/// Descriptor for `SetVoiceRoomOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVoiceRoomOverrideResponseDescriptor =
    $convert.base64Decode('ChxTZXRWb2ljZVJvb21PdmVycmlkZVJlc3BvbnNl');

@$core.Deprecated('Use removeVoiceRoomOverrideResponseDescriptor instead')
const RemoveVoiceRoomOverrideResponse$json = {
  '1': 'RemoveVoiceRoomOverrideResponse',
};

/// Descriptor for `RemoveVoiceRoomOverrideResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeVoiceRoomOverrideResponseDescriptor =
    $convert.base64Decode('Ch9SZW1vdmVWb2ljZVJvb21PdmVycmlkZVJlc3BvbnNl');

@$core.Deprecated('Use getVoiceRoomOverridesResponseDescriptor instead')
const GetVoiceRoomOverridesResponse$json = {
  '1': 'GetVoiceRoomOverridesResponse',
  '2': [
    {
      '1': 'override_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.OverrideList',
      '10': 'overrideList'
    },
  ],
};

/// Descriptor for `GetVoiceRoomOverridesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceRoomOverridesResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRWb2ljZVJvb21PdmVycmlkZXNSZXNwb25zZRJACg1vdmVycmlkZV9saXN0GAEgASgLMh'
        'sudm9pY2Uucm9sZS52MS5PdmVycmlkZUxpc3RSDG92ZXJyaWRlTGlzdA==');

@$core.Deprecated('Use getEffectivePermissionsResponseDescriptor instead')
const GetEffectivePermissionsResponse$json = {
  '1': 'GetEffectivePermissionsResponse',
  '2': [
    {
      '1': 'permission_set',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.PermissionSet',
      '10': 'permissionSet'
    },
  ],
};

/// Descriptor for `GetEffectivePermissionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getEffectivePermissionsResponseDescriptor =
    $convert.base64Decode(
        'Ch9HZXRFZmZlY3RpdmVQZXJtaXNzaW9uc1Jlc3BvbnNlEkMKDnBlcm1pc3Npb25fc2V0GAEgAS'
        'gLMhwudm9pY2Uucm9sZS52MS5QZXJtaXNzaW9uU2V0Ug1wZXJtaXNzaW9uU2V0');

@$core.Deprecated('Use setDefaultJoinRoleResponseDescriptor instead')
const SetDefaultJoinRoleResponse$json = {
  '1': 'SetDefaultJoinRoleResponse',
};

/// Descriptor for `SetDefaultJoinRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setDefaultJoinRoleResponseDescriptor =
    $convert.base64Decode('ChpTZXREZWZhdWx0Sm9pblJvbGVSZXNwb25zZQ==');

@$core.Deprecated('Use getDefaultJoinRoleResponseDescriptor instead')
const GetDefaultJoinRoleResponse$json = {
  '1': 'GetDefaultJoinRoleResponse',
  '2': [
    {
      '1': 'role',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.Role',
      '10': 'role'
    },
  ],
};

/// Descriptor for `GetDefaultJoinRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDefaultJoinRoleResponseDescriptor =
    $convert.base64Decode(
        'ChpHZXREZWZhdWx0Sm9pblJvbGVSZXNwb25zZRInCgRyb2xlGAEgASgLMhMudm9pY2Uucm9sZS'
        '52MS5Sb2xlUgRyb2xl');

@$core.Deprecated('Use applySpaceLifecycleFenceRequestDescriptor instead')
const ApplySpaceLifecycleFenceRequest$json = {
  '1': 'ApplySpaceLifecycleFenceRequest',
  '2': [
    {
      '1': 'fence',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpaceLifecycleFenceRequest',
      '10': 'fence'
    },
  ],
};

/// Descriptor for `ApplySpaceLifecycleFenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySpaceLifecycleFenceRequestDescriptor =
    $convert.base64Decode(
        'Ch9BcHBseVNwYWNlTGlmZWN5Y2xlRmVuY2VSZXF1ZXN0EkEKBWZlbmNlGAEgASgLMisudm9pY2'
        'UuY29tbW9uLnYxLlNwYWNlTGlmZWN5Y2xlRmVuY2VSZXF1ZXN0UgVmZW5jZQ==');

@$core.Deprecated('Use applySpaceLifecycleFenceResponseDescriptor instead')
const ApplySpaceLifecycleFenceResponse$json = {
  '1': 'ApplySpaceLifecycleFenceResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.SpaceLifecycleFenceReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ApplySpaceLifecycleFenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySpaceLifecycleFenceResponseDescriptor =
    $convert.base64Decode(
        'CiBBcHBseVNwYWNlTGlmZWN5Y2xlRmVuY2VSZXNwb25zZRJFCgdyZWNlaXB0GAEgASgLMisudm'
        '9pY2UuY29tbW9uLnYxLlNwYWNlTGlmZWN5Y2xlRmVuY2VSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use retireSpaceRequestDescriptor instead')
const RetireSpaceRequest$json = {
  '1': 'RetireSpaceRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 4, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'purge_decided_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'purgeDecidedAt'
    },
    {
      '1': 'manifest',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.ManifestBinding',
      '10': 'manifest'
    },
  ],
};

/// Descriptor for `RetireSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List retireSpaceRequestDescriptor = $convert.base64Decode(
    'ChJSZXRpcmVTcGFjZVJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2'
    'xWZXJzaW9uEhkKCHNwYWNlX2lkGAIgASgJUgdzcGFjZUlkEjIKFWRlbGV0aW9uX29wZXJhdGlv'
    'bl9pZBgDIAEoCVITZGVsZXRpb25PcGVyYXRpb25JZBIeCgpnZW5lcmF0aW9uGAQgASgEUgpnZW'
    '5lcmF0aW9uEkQKEHB1cmdlX2RlY2lkZWRfYXQYBSABKAsyGi5nb29nbGUucHJvdG9idWYuVGlt'
    'ZXN0YW1wUg5wdXJnZURlY2lkZWRBdBI8CghtYW5pZmVzdBgGIAEoCzIgLnZvaWNlLmNvbW1vbi'
    '52MS5NYW5pZmVzdEJpbmRpbmdSCG1hbmlmZXN0');

@$core.Deprecated('Use retireSpaceReceiptDescriptor instead')
const RetireSpaceReceipt$json = {
  '1': 'RetireSpaceReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'deletion_operation_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'deletionOperationId'
    },
    {'1': 'generation', '3': 5, '4': 1, '5': 4, '10': 'generation'},
    {
      '1': 'state',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.role.v1.RoleRetirementState',
      '10': 'state'
    },
    {'1': 'request_sha256', '3': 7, '4': 1, '5': 12, '10': 'requestSha256'},
    {'1': 'manifest_sha256', '3': 8, '4': 1, '5': 12, '10': 'manifestSha256'},
    {
      '1': 'retired_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'retiredAt'
    },
  ],
};

/// Descriptor for `RetireSpaceReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List retireSpaceReceiptDescriptor = $convert.base64Decode(
    'ChJSZXRpcmVTcGFjZVJlY2VpcHQSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDVIPcHJvdG9jb2'
    'xWZXJzaW9uEh0KCnJlY2VpcHRfaWQYAiABKAlSCXJlY2VpcHRJZBIZCghzcGFjZV9pZBgDIAEo'
    'CVIHc3BhY2VJZBIyChVkZWxldGlvbl9vcGVyYXRpb25faWQYBCABKAlSE2RlbGV0aW9uT3Blcm'
    'F0aW9uSWQSHgoKZ2VuZXJhdGlvbhgFIAEoBFIKZ2VuZXJhdGlvbhI4CgVzdGF0ZRgGIAEoDjIi'
    'LnZvaWNlLnJvbGUudjEuUm9sZVJldGlyZW1lbnRTdGF0ZVIFc3RhdGUSJQoOcmVxdWVzdF9zaG'
    'EyNTYYByABKAxSDXJlcXVlc3RTaGEyNTYSJwoPbWFuaWZlc3Rfc2hhMjU2GAggASgMUg5tYW5p'
    'ZmVzdFNoYTI1NhI5CgpyZXRpcmVkX2F0GAkgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdG'
    'FtcFIJcmV0aXJlZEF0');

@$core.Deprecated('Use retireSpaceResponseDescriptor instead')
const RetireSpaceResponse$json = {
  '1': 'RetireSpaceResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.RetireSpaceReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `RetireSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List retireSpaceResponseDescriptor = $convert.base64Decode(
    'ChNSZXRpcmVTcGFjZVJlc3BvbnNlEjsKB3JlY2VpcHQYASABKAsyIS52b2ljZS5yb2xlLnYxLl'
    'JldGlyZVNwYWNlUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated('Use applyGameSessionGrantsRequestDescriptor instead')
const ApplyGameSessionGrantsRequest$json = {
  '1': 'ApplyGameSessionGrantsRequest',
  '2': [
    {'1': 'application_id', '3': 1, '4': 1, '5': 9, '10': 'applicationId'},
    {'1': 'environment_id', '3': 2, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'session_id', '3': 3, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'voice_room_id', '3': 4, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'roster_revision', '3': 6, '4': 1, '5': 4, '10': 'rosterRevision'},
    {'1': 'profile_ids', '3': 7, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `ApplyGameSessionGrantsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyGameSessionGrantsRequestDescriptor = $convert.base64Decode(
    'Ch1BcHBseUdhbWVTZXNzaW9uR3JhbnRzUmVxdWVzdBIlCg5hcHBsaWNhdGlvbl9pZBgBIAEoCV'
    'INYXBwbGljYXRpb25JZBIlCg5lbnZpcm9ubWVudF9pZBgCIAEoCVINZW52aXJvbm1lbnRJZBId'
    'CgpzZXNzaW9uX2lkGAMgASgJUglzZXNzaW9uSWQSIgoNdm9pY2Vfcm9vbV9pZBgEIAEoCVILdm'
    '9pY2VSb29tSWQSIQoMb3BlcmF0aW9uX2lkGAUgASgJUgtvcGVyYXRpb25JZBInCg9yb3N0ZXJf'
    'cmV2aXNpb24YBiABKARSDnJvc3RlclJldmlzaW9uEh8KC3Byb2ZpbGVfaWRzGAcgAygJUgpwcm'
    '9maWxlSWRz');

@$core.Deprecated('Use revokeGameSessionGrantsRequestDescriptor instead')
const RevokeGameSessionGrantsRequest$json = {
  '1': 'RevokeGameSessionGrantsRequest',
  '2': [
    {'1': 'application_id', '3': 1, '4': 1, '5': 9, '10': 'applicationId'},
    {'1': 'environment_id', '3': 2, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'session_id', '3': 3, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
  ],
};

/// Descriptor for `RevokeGameSessionGrantsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeGameSessionGrantsRequestDescriptor =
    $convert.base64Decode(
        'Ch5SZXZva2VHYW1lU2Vzc2lvbkdyYW50c1JlcXVlc3QSJQoOYXBwbGljYXRpb25faWQYASABKA'
        'lSDWFwcGxpY2F0aW9uSWQSJQoOZW52aXJvbm1lbnRfaWQYAiABKAlSDWVudmlyb25tZW50SWQS'
        'HQoKc2Vzc2lvbl9pZBgDIAEoCVIJc2Vzc2lvbklkEiEKDG9wZXJhdGlvbl9pZBgEIAEoCVILb3'
        'BlcmF0aW9uSWQ=');

@$core.Deprecated('Use checkGameSessionGrantRequestDescriptor instead')
const CheckGameSessionGrantRequest$json = {
  '1': 'CheckGameSessionGrantRequest',
  '2': [
    {'1': 'application_id', '3': 1, '4': 1, '5': 9, '10': 'applicationId'},
    {'1': 'environment_id', '3': 2, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'session_id', '3': 3, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'voice_room_id', '3': 4, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {'1': 'profile_id', '3': 5, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `CheckGameSessionGrantRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkGameSessionGrantRequestDescriptor = $convert.base64Decode(
    'ChxDaGVja0dhbWVTZXNzaW9uR3JhbnRSZXF1ZXN0EiUKDmFwcGxpY2F0aW9uX2lkGAEgASgJUg'
    '1hcHBsaWNhdGlvbklkEiUKDmVudmlyb25tZW50X2lkGAIgASgJUg1lbnZpcm9ubWVudElkEh0K'
    'CnNlc3Npb25faWQYAyABKAlSCXNlc3Npb25JZBIiCg12b2ljZV9yb29tX2lkGAQgASgJUgt2b2'
    'ljZVJvb21JZBIdCgpwcm9maWxlX2lkGAUgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use gameSessionGrantReceiptDescriptor instead')
const GameSessionGrantReceipt$json = {
  '1': 'GameSessionGrantReceipt',
  '2': [
    {'1': 'receipt_id', '3': 1, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'application_id', '3': 3, '4': 1, '5': 9, '10': 'applicationId'},
    {'1': 'environment_id', '3': 4, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'session_id', '3': 5, '4': 1, '5': 9, '10': 'sessionId'},
    {'1': 'request_sha256', '3': 6, '4': 1, '5': 12, '10': 'requestSha256'},
    {'1': 'roster_revision', '3': 7, '4': 1, '5': 4, '10': 'rosterRevision'},
    {
      '1': 'applied_profile_set_sha256',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'appliedProfileSetSha256'
    },
    {
      '1': 'outcome',
      '3': 9,
      '4': 1,
      '5': 14,
      '6': '.voice.role.v1.GameSessionGrantOutcome',
      '10': 'outcome'
    },
  ],
};

/// Descriptor for `GameSessionGrantReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameSessionGrantReceiptDescriptor = $convert.base64Decode(
    'ChdHYW1lU2Vzc2lvbkdyYW50UmVjZWlwdBIdCgpyZWNlaXB0X2lkGAEgASgJUglyZWNlaXB0SW'
    'QSIQoMb3BlcmF0aW9uX2lkGAIgASgJUgtvcGVyYXRpb25JZBIlCg5hcHBsaWNhdGlvbl9pZBgD'
    'IAEoCVINYXBwbGljYXRpb25JZBIlCg5lbnZpcm9ubWVudF9pZBgEIAEoCVINZW52aXJvbm1lbn'
    'RJZBIdCgpzZXNzaW9uX2lkGAUgASgJUglzZXNzaW9uSWQSJQoOcmVxdWVzdF9zaGEyNTYYBiAB'
    'KAxSDXJlcXVlc3RTaGEyNTYSJwoPcm9zdGVyX3JldmlzaW9uGAcgASgEUg5yb3N0ZXJSZXZpc2'
    'lvbhI7ChphcHBsaWVkX3Byb2ZpbGVfc2V0X3NoYTI1NhgIIAEoDFIXYXBwbGllZFByb2ZpbGVT'
    'ZXRTaGEyNTYSQAoHb3V0Y29tZRgJIAEoDjImLnZvaWNlLnJvbGUudjEuR2FtZVNlc3Npb25Hcm'
    'FudE91dGNvbWVSB291dGNvbWU=');

@$core.Deprecated('Use applyGameSessionGrantsResponseDescriptor instead')
const ApplyGameSessionGrantsResponse$json = {
  '1': 'ApplyGameSessionGrantsResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.GameSessionGrantReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ApplyGameSessionGrantsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyGameSessionGrantsResponseDescriptor =
    $convert.base64Decode(
        'Ch5BcHBseUdhbWVTZXNzaW9uR3JhbnRzUmVzcG9uc2USQAoHcmVjZWlwdBgBIAEoCzImLnZvaW'
        'NlLnJvbGUudjEuR2FtZVNlc3Npb25HcmFudFJlY2VpcHRSB3JlY2VpcHQ=');

@$core.Deprecated('Use revokeGameSessionGrantsResponseDescriptor instead')
const RevokeGameSessionGrantsResponse$json = {
  '1': 'RevokeGameSessionGrantsResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.role.v1.GameSessionGrantReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `RevokeGameSessionGrantsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeGameSessionGrantsResponseDescriptor =
    $convert.base64Decode(
        'Ch9SZXZva2VHYW1lU2Vzc2lvbkdyYW50c1Jlc3BvbnNlEkAKB3JlY2VpcHQYASABKAsyJi52b2'
        'ljZS5yb2xlLnYxLkdhbWVTZXNzaW9uR3JhbnRSZWNlaXB0UgdyZWNlaXB0');

@$core.Deprecated('Use checkGameSessionGrantResponseDescriptor instead')
const CheckGameSessionGrantResponse$json = {
  '1': 'CheckGameSessionGrantResponse',
  '2': [
    {'1': 'allowed', '3': 1, '4': 1, '5': 8, '10': 'allowed'},
  ],
};

/// Descriptor for `CheckGameSessionGrantResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkGameSessionGrantResponseDescriptor =
    $convert.base64Decode(
        'Ch1DaGVja0dhbWVTZXNzaW9uR3JhbnRSZXNwb25zZRIYCgdhbGxvd2VkGAEgASgIUgdhbGxvd2'
        'Vk');
