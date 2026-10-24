// This is a generated file - do not edit.
//
// Generated from voice/identityerasure/v1/identity_erasure.proto.

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

@$core.Deprecated('Use eligibilityDescriptor instead')
const Eligibility$json = {
  '1': 'Eligibility',
  '2': [
    {'1': 'ELIGIBILITY_UNSPECIFIED', '2': 0},
    {'1': 'ELIGIBILITY_ELIGIBLE', '2': 1},
    {'1': 'ELIGIBILITY_INELIGIBLE', '2': 2},
    {'1': 'ELIGIBILITY_UNAVAILABLE', '2': 3},
  ],
};

/// Descriptor for `Eligibility`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List eligibilityDescriptor = $convert.base64Decode(
    'CgtFbGlnaWJpbGl0eRIbChdFTElHSUJJTElUWV9VTlNQRUNJRklFRBAAEhgKFEVMSUdJQklMSV'
    'RZX0VMSUdJQkxFEAESGgoWRUxJR0lCSUxJVFlfSU5FTElHSUJMRRACEhsKF0VMSUdJQklMSVRZ'
    'X1VOQVZBSUxBQkxFEAM=');

@$core.Deprecated('Use erasureDispositionDescriptor instead')
const ErasureDisposition$json = {
  '1': 'ErasureDisposition',
  '2': [
    {'1': 'ERASURE_DISPOSITION_UNSPECIFIED', '2': 0},
    {'1': 'ERASURE_DISPOSITION_NOT_IRREVERSIBLE', '2': 1},
    {'1': 'ERASURE_DISPOSITION_IRREVERSIBLE', '2': 2},
    {'1': 'ERASURE_DISPOSITION_INCOMPLETE', '2': 3},
  ],
};

/// Descriptor for `ErasureDisposition`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List erasureDispositionDescriptor = $convert.base64Decode(
    'ChJFcmFzdXJlRGlzcG9zaXRpb24SIwofRVJBU1VSRV9ESVNQT1NJVElPTl9VTlNQRUNJRklFRB'
    'AAEigKJEVSQVNVUkVfRElTUE9TSVRJT05fTk9UX0lSUkVWRVJTSUJMRRABEiQKIEVSQVNVUkVf'
    'RElTUE9TSVRJT05fSVJSRVZFUlNJQkxFEAISIgoeRVJBU1VSRV9ESVNQT1NJVElPTl9JTkNPTV'
    'BMRVRFEAM=');

@$core.Deprecated('Use accountLifecycleStateDescriptor instead')
const AccountLifecycleState$json = {
  '1': 'AccountLifecycleState',
  '2': [
    {'1': 'ACCOUNT_LIFECYCLE_STATE_UNSPECIFIED', '2': 0},
    {'1': 'ACCOUNT_LIFECYCLE_STATE_ACTIVE', '2': 1},
    {'1': 'ACCOUNT_LIFECYCLE_STATE_RECOVERABLE', '2': 2},
    {'1': 'ACCOUNT_LIFECYCLE_STATE_TERMINAL', '2': 3},
  ],
};

/// Descriptor for `AccountLifecycleState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List accountLifecycleStateDescriptor = $convert.base64Decode(
    'ChVBY2NvdW50TGlmZWN5Y2xlU3RhdGUSJwojQUNDT1VOVF9MSUZFQ1lDTEVfU1RBVEVfVU5TUE'
    'VDSUZJRUQQABIiCh5BQ0NPVU5UX0xJRkVDWUNMRV9TVEFURV9BQ1RJVkUQARInCiNBQ0NPVU5U'
    'X0xJRkVDWUNMRV9TVEFURV9SRUNPVkVSQUJMRRACEiQKIEFDQ09VTlRfTElGRUNZQ0xFX1NUQV'
    'RFX1RFUk1JTkFMEAM=');

@$core.Deprecated('Use ownerCutDescriptor instead')
const OwnerCut$json = {
  '1': 'OwnerCut',
  '2': [
    {'1': 'owner', '3': 1, '4': 1, '5': 9, '10': 'owner'},
    {'1': 'stream_id', '3': 2, '4': 1, '5': 9, '10': 'streamId'},
    {
      '1': 'history_generation',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'historyGeneration'
    },
    {'1': 'through_revision', '3': 4, '4': 1, '5': 4, '10': 'throughRevision'},
    {
      '1': 'selector_keyset_revision',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'selectorKeysetRevision'
    },
    {
      '1': 'selector_key_versions',
      '3': 6,
      '4': 3,
      '5': 9,
      '10': 'selectorKeyVersions'
    },
    {'1': 'fact_count', '3': 7, '4': 1, '5': 4, '10': 'factCount'},
    {'1': 'digest', '3': 8, '4': 1, '5': 12, '10': 'digest'},
    {'1': 'complete', '3': 9, '4': 1, '5': 8, '10': 'complete'},
  ],
};

/// Descriptor for `OwnerCut`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ownerCutDescriptor = $convert.base64Decode(
    'CghPd25lckN1dBIUCgVvd25lchgBIAEoCVIFb3duZXISGwoJc3RyZWFtX2lkGAIgASgJUghzdH'
    'JlYW1JZBItChJoaXN0b3J5X2dlbmVyYXRpb24YAyABKAlSEWhpc3RvcnlHZW5lcmF0aW9uEikK'
    'EHRocm91Z2hfcmV2aXNpb24YBCABKARSD3Rocm91Z2hSZXZpc2lvbhI4ChhzZWxlY3Rvcl9rZX'
    'lzZXRfcmV2aXNpb24YBSABKARSFnNlbGVjdG9yS2V5c2V0UmV2aXNpb24SMgoVc2VsZWN0b3Jf'
    'a2V5X3ZlcnNpb25zGAYgAygJUhNzZWxlY3RvcktleVZlcnNpb25zEh0KCmZhY3RfY291bnQYBy'
    'ABKARSCWZhY3RDb3VudBIWCgZkaWdlc3QYCCABKAxSBmRpZ2VzdBIaCghjb21wbGV0ZRgJIAEo'
    'CFIIY29tcGxldGU=');

@$core.Deprecated('Use recoveryBindingDescriptor instead')
const RecoveryBinding$json = {
  '1': 'RecoveryBinding',
  '2': [
    {'1': 'restore_target', '3': 1, '4': 1, '5': 9, '10': 'restoreTarget'},
    {
      '1': 'restore_generation',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'restoreGeneration'
    },
    {'1': 'challenge', '3': 3, '4': 1, '5': 12, '10': 'challenge'},
    {'1': 'role_stream_id', '3': 4, '4': 1, '5': 9, '10': 'roleStreamId'},
    {
      '1': 'role_through_revision',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'roleThroughRevision'
    },
    {
      '1': 'role_checkpoint_digest',
      '3': 6,
      '4': 1,
      '5': 12,
      '10': 'roleCheckpointDigest'
    },
    {
      '1': 'role_keyset_revision',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'roleKeysetRevision'
    },
    {
      '1': 'inventory_generation',
      '3': 8,
      '4': 1,
      '5': 4,
      '10': 'inventoryGeneration'
    },
    {'1': 'inventory_digest', '3': 9, '4': 1, '5': 12, '10': 'inventoryDigest'},
  ],
};

/// Descriptor for `RecoveryBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List recoveryBindingDescriptor = $convert.base64Decode(
    'Cg9SZWNvdmVyeUJpbmRpbmcSJQoOcmVzdG9yZV90YXJnZXQYASABKAlSDXJlc3RvcmVUYXJnZX'
    'QSLQoScmVzdG9yZV9nZW5lcmF0aW9uGAIgASgJUhFyZXN0b3JlR2VuZXJhdGlvbhIcCgljaGFs'
    'bGVuZ2UYAyABKAxSCWNoYWxsZW5nZRIkCg5yb2xlX3N0cmVhbV9pZBgEIAEoCVIMcm9sZVN0cm'
    'VhbUlkEjIKFXJvbGVfdGhyb3VnaF9yZXZpc2lvbhgFIAEoBFITcm9sZVRocm91Z2hSZXZpc2lv'
    'bhI0ChZyb2xlX2NoZWNrcG9pbnRfZGlnZXN0GAYgASgMUhRyb2xlQ2hlY2twb2ludERpZ2VzdB'
    'IwChRyb2xlX2tleXNldF9yZXZpc2lvbhgHIAEoCVIScm9sZUtleXNldFJldmlzaW9uEjEKFGlu'
    'dmVudG9yeV9nZW5lcmF0aW9uGAggASgEUhNpbnZlbnRvcnlHZW5lcmF0aW9uEikKEGludmVudG'
    '9yeV9kaWdlc3QYCSABKAxSD2ludmVudG9yeURpZ2VzdA==');

@$core.Deprecated('Use observationWindowDescriptor instead')
const ObservationWindow$json = {
  '1': 'ObservationWindow',
  '2': [
    {
      '1': 'observed_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'observedAt'
    },
    {
      '1': 'valid_until',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'validUntil'
    },
  ],
};

/// Descriptor for `ObservationWindow`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observationWindowDescriptor = $convert.base64Decode(
    'ChFPYnNlcnZhdGlvbldpbmRvdxI7CgtvYnNlcnZlZF9hdBgBIAEoCzIaLmdvb2dsZS5wcm90b2'
    'J1Zi5UaW1lc3RhbXBSCm9ic2VydmVkQXQSOwoLdmFsaWRfdW50aWwYAiABKAsyGi5nb29nbGUu'
    'cHJvdG9idWYuVGltZXN0YW1wUgp2YWxpZFVudGls');

@$core.Deprecated('Use accountLifecycleFactDescriptor instead')
const AccountLifecycleFact$json = {
  '1': 'AccountLifecycleFact',
  '2': [
    {'1': 'owner_handle', '3': 1, '4': 1, '5': 9, '10': 'ownerHandle'},
    {
      '1': 'lifecycle_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'lifecycleRevision'
    },
    {
      '1': 'state',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.identityerasure.v1.AccountLifecycleState',
      '10': 'state'
    },
    {'1': 'deletion_cycle', '3': 4, '4': 1, '5': 9, '10': 'deletionCycle'},
    {
      '1': 'erase_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'eraseAt'
    },
    {
      '1': 'eligibility',
      '3': 6,
      '4': 1,
      '5': 14,
      '6': '.voice.identityerasure.v1.Eligibility',
      '10': 'eligibility'
    },
    {
      '1': 'disposition',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.identityerasure.v1.ErasureDisposition',
      '10': 'disposition'
    },
    {
      '1': 'owner_cut',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
  ],
};

/// Descriptor for `AccountLifecycleFact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List accountLifecycleFactDescriptor = $convert.base64Decode(
    'ChRBY2NvdW50TGlmZWN5Y2xlRmFjdBIhCgxvd25lcl9oYW5kbGUYASABKAlSC293bmVySGFuZG'
    'xlEi0KEmxpZmVjeWNsZV9yZXZpc2lvbhgCIAEoBFIRbGlmZWN5Y2xlUmV2aXNpb24SRQoFc3Rh'
    'dGUYAyABKA4yLy52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuQWNjb3VudExpZmVjeWNsZVN0YX'
    'RlUgVzdGF0ZRIlCg5kZWxldGlvbl9jeWNsZRgEIAEoCVINZGVsZXRpb25DeWNsZRI1CghlcmFz'
    'ZV9hdBgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSB2VyYXNlQXQSRwoLZWxpZ2'
    'liaWxpdHkYBiABKA4yJS52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuRWxpZ2liaWxpdHlSC2Vs'
    'aWdpYmlsaXR5Ek4KC2Rpc3Bvc2l0aW9uGAcgASgOMiwudm9pY2UuaWRlbnRpdHllcmFzdXJlLn'
    'YxLkVyYXN1cmVEaXNwb3NpdGlvblILZGlzcG9zaXRpb24SPwoJb3duZXJfY3V0GAggASgLMiIu'
    'dm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLk93bmVyQ3V0Ughvd25lckN1dA==');

@$core.Deprecated('Use accountObservationDescriptor instead')
const AccountObservation$json = {
  '1': 'AccountObservation',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'lifecycle',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountLifecycleFact',
      '10': 'lifecycle'
    },
  ],
};

/// Descriptor for `AccountObservation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List accountObservationDescriptor = $convert.base64Decode(
    'ChJBY2NvdW50T2JzZXJ2YXRpb24SHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElkEkwKCW'
    'xpZmVjeWNsZRgCIAEoCzIuLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5BY2NvdW50TGlmZWN5'
    'Y2xlRmFjdFIJbGlmZWN5Y2xl');

@$core.Deprecated('Use observeAccountsRequestDescriptor instead')
const ObserveAccountsRequest$json = {
  '1': 'ObserveAccountsRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'account_ids', '3': 3, '4': 3, '5': 9, '10': 'accountIds'},
    {
      '1': 'recovery',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveAccountsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeAccountsRequestDescriptor = $convert.base64Decode(
    'ChZPYnNlcnZlQWNjb3VudHNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3Byb3'
    'RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQSHwoLYWNjb3VudF9p'
    'ZHMYAyADKAlSCmFjY291bnRJZHMSRQoIcmVjb3ZlcnkYBCABKAsyKS52b2ljZS5pZGVudGl0eW'
    'VyYXN1cmUudjEuUmVjb3ZlcnlCaW5kaW5nUghyZWNvdmVyeQ==');

@$core.Deprecated('Use observeAccountsResponseDescriptor instead')
const ObserveAccountsResponse$json = {
  '1': 'ObserveAccountsResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'window',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.ObservationWindow',
      '10': 'window'
    },
    {
      '1': 'accounts',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountObservation',
      '10': 'accounts'
    },
    {
      '1': 'owner_cut',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
    {
      '1': 'recovery',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveAccountsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeAccountsResponseDescriptor = $convert.base64Decode(
    'ChdPYnNlcnZlQWNjb3VudHNSZXNwb25zZRIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm'
    '90b2NvbFZlcnNpb24SHQoKcmVxdWVzdF9pZBgCIAEoCVIJcmVxdWVzdElkEkMKBndpbmRvdxgD'
    'IAEoCzIrLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5PYnNlcnZhdGlvbldpbmRvd1IGd2luZG'
    '93EkgKCGFjY291bnRzGAQgAygLMiwudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLkFjY291bnRP'
    'YnNlcnZhdGlvblIIYWNjb3VudHMSPwoJb3duZXJfY3V0GAUgASgLMiIudm9pY2UuaWRlbnRpdH'
    'llcmFzdXJlLnYxLk93bmVyQ3V0Ughvd25lckN1dBJFCghyZWNvdmVyeRgGIAEoCzIpLnZvaWNl'
    'LmlkZW50aXR5ZXJhc3VyZS52MS5SZWNvdmVyeUJpbmRpbmdSCHJlY292ZXJ5');

@$core.Deprecated('Use resolveAccountHandleRequestDescriptor instead')
const ResolveAccountHandleRequest$json = {
  '1': 'ResolveAccountHandleRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'account_id', '3': 3, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `ResolveAccountHandleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveAccountHandleRequestDescriptor =
    $convert.base64Decode(
        'ChtSZXNvbHZlQWNjb3VudEhhbmRsZVJlcXVlc3QSKQoQcHJvdG9jb2xfdmVyc2lvbhgBIAEoDV'
        'IPcHJvdG9jb2xWZXJzaW9uEh0KCnJlcXVlc3RfaWQYAiABKAlSCXJlcXVlc3RJZBIdCgphY2Nv'
        'dW50X2lkGAMgASgJUglhY2NvdW50SWQ=');

@$core.Deprecated('Use resolveAccountHandleResponseDescriptor instead')
const ResolveAccountHandleResponse$json = {
  '1': 'ResolveAccountHandleResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'account_id', '3': 3, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'window',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.ObservationWindow',
      '10': 'window'
    },
    {
      '1': 'lifecycle',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountLifecycleFact',
      '10': 'lifecycle'
    },
  ],
};

/// Descriptor for `ResolveAccountHandleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveAccountHandleResponseDescriptor = $convert.base64Decode(
    'ChxSZXNvbHZlQWNjb3VudEhhbmRsZVJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQSHQoKYWNj'
    'b3VudF9pZBgDIAEoCVIJYWNjb3VudElkEkMKBndpbmRvdxgEIAEoCzIrLnZvaWNlLmlkZW50aX'
    'R5ZXJhc3VyZS52MS5PYnNlcnZhdGlvbldpbmRvd1IGd2luZG93EkwKCWxpZmVjeWNsZRgFIAEo'
    'CzIuLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5BY2NvdW50TGlmZWN5Y2xlRmFjdFIJbGlmZW'
    'N5Y2xl');

@$core.Deprecated('Use observeAccountHandlesRequestDescriptor instead')
const ObserveAccountHandlesRequest$json = {
  '1': 'ObserveAccountHandlesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'owner_handles', '3': 3, '4': 3, '5': 9, '10': 'ownerHandles'},
    {
      '1': 'required_auth_cut',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'requiredAuthCut'
    },
    {
      '1': 'recovery',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveAccountHandlesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeAccountHandlesRequestDescriptor = $convert.base64Decode(
    'ChxPYnNlcnZlQWNjb3VudEhhbmRsZXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
    '1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQSIwoNb3du'
    'ZXJfaGFuZGxlcxgDIAMoCVIMb3duZXJIYW5kbGVzEk4KEXJlcXVpcmVkX2F1dGhfY3V0GAQgAS'
    'gLMiIudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLk93bmVyQ3V0Ug9yZXF1aXJlZEF1dGhDdXQS'
    'RQoIcmVjb3ZlcnkYBSABKAsyKS52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuUmVjb3ZlcnlCaW'
    '5kaW5nUghyZWNvdmVyeQ==');

@$core.Deprecated('Use observeAccountHandlesResponseDescriptor instead')
const ObserveAccountHandlesResponse$json = {
  '1': 'ObserveAccountHandlesResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'window',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.ObservationWindow',
      '10': 'window'
    },
    {
      '1': 'accounts',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountLifecycleFact',
      '10': 'accounts'
    },
    {
      '1': 'owner_cut',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
    {
      '1': 'recovery',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveAccountHandlesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeAccountHandlesResponseDescriptor = $convert.base64Decode(
    'Ch1PYnNlcnZlQWNjb3VudEhhbmRsZXNSZXNwb25zZRIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
    'gNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVxdWVzdF9pZBgCIAEoCVIJcmVxdWVzdElkEkMKBndp'
    'bmRvdxgDIAEoCzIrLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5PYnNlcnZhdGlvbldpbmRvd1'
    'IGd2luZG93EkoKCGFjY291bnRzGAQgAygLMi4udm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLkFj'
    'Y291bnRMaWZlY3ljbGVGYWN0UghhY2NvdW50cxI/Cglvd25lcl9jdXQYBSABKAsyIi52b2ljZS'
    '5pZGVudGl0eWVyYXN1cmUudjEuT3duZXJDdXRSCG93bmVyQ3V0EkUKCHJlY292ZXJ5GAYgASgL'
    'Mikudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLlJlY292ZXJ5QmluZGluZ1IIcmVjb3Zlcnk=');

@$core.Deprecated('Use profileObservationDescriptor instead')
const ProfileObservation$json = {
  '1': 'ProfileObservation',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'lifecycle_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'lifecycleRevision'
    },
    {
      '1': 'eligibility',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.identityerasure.v1.Eligibility',
      '10': 'eligibility'
    },
    {
      '1': 'disposition',
      '3': 4,
      '4': 1,
      '5': 14,
      '6': '.voice.identityerasure.v1.ErasureDisposition',
      '10': 'disposition'
    },
    {
      '1': 'erase_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'eraseAt'
    },
    {
      '1': 'parent_owner_handle',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'parentOwnerHandle'
    },
    {
      '1': 'terminal_parent',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountLifecycleFact',
      '10': 'terminalParent'
    },
    {
      '1': 'owner_cut',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
  ],
};

/// Descriptor for `ProfileObservation`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileObservationDescriptor = $convert.base64Decode(
    'ChJQcm9maWxlT2JzZXJ2YXRpb24SHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEi0KEm'
    'xpZmVjeWNsZV9yZXZpc2lvbhgCIAEoBFIRbGlmZWN5Y2xlUmV2aXNpb24SRwoLZWxpZ2liaWxp'
    'dHkYAyABKA4yJS52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuRWxpZ2liaWxpdHlSC2VsaWdpYm'
    'lsaXR5Ek4KC2Rpc3Bvc2l0aW9uGAQgASgOMiwudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLkVy'
    'YXN1cmVEaXNwb3NpdGlvblILZGlzcG9zaXRpb24SNQoIZXJhc2VfYXQYBSABKAsyGi5nb29nbG'
    'UucHJvdG9idWYuVGltZXN0YW1wUgdlcmFzZUF0Ei4KE3BhcmVudF9vd25lcl9oYW5kbGUYBiAB'
    'KAlSEXBhcmVudE93bmVySGFuZGxlElcKD3Rlcm1pbmFsX3BhcmVudBgHIAEoCzIuLnZvaWNlLm'
    'lkZW50aXR5ZXJhc3VyZS52MS5BY2NvdW50TGlmZWN5Y2xlRmFjdFIOdGVybWluYWxQYXJlbnQS'
    'PwoJb3duZXJfY3V0GAggASgLMiIudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLk93bmVyQ3V0Ug'
    'hvd25lckN1dA==');

@$core.Deprecated('Use observeProfilesRequestDescriptor instead')
const ObserveProfilesRequest$json = {
  '1': 'ObserveProfilesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'profile_ids', '3': 3, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'recovery',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeProfilesRequestDescriptor = $convert.base64Decode(
    'ChZPYnNlcnZlUHJvZmlsZXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3Byb3'
    'RvY29sVmVyc2lvbhIdCgpyZXF1ZXN0X2lkGAIgASgJUglyZXF1ZXN0SWQSHwoLcHJvZmlsZV9p'
    'ZHMYAyADKAlSCnByb2ZpbGVJZHMSRQoIcmVjb3ZlcnkYBCABKAsyKS52b2ljZS5pZGVudGl0eW'
    'VyYXN1cmUudjEuUmVjb3ZlcnlCaW5kaW5nUghyZWNvdmVyeQ==');

@$core.Deprecated('Use observeProfilesResponseDescriptor instead')
const ObserveProfilesResponse$json = {
  '1': 'ObserveProfilesResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'window',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.ObservationWindow',
      '10': 'window'
    },
    {
      '1': 'profiles',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.identityerasure.v1.ProfileObservation',
      '10': 'profiles'
    },
    {
      '1': 'owner_cut',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
    {
      '1': 'recovery',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ObserveProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeProfilesResponseDescriptor = $convert.base64Decode(
    'ChdPYnNlcnZlUHJvZmlsZXNSZXNwb25zZRIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm'
    '90b2NvbFZlcnNpb24SHQoKcmVxdWVzdF9pZBgCIAEoCVIJcmVxdWVzdElkEkMKBndpbmRvdxgD'
    'IAEoCzIrLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5PYnNlcnZhdGlvbldpbmRvd1IGd2luZG'
    '93EkgKCHByb2ZpbGVzGAQgAygLMiwudm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLlByb2ZpbGVP'
    'YnNlcnZhdGlvblIIcHJvZmlsZXMSPwoJb3duZXJfY3V0GAUgASgLMiIudm9pY2UuaWRlbnRpdH'
    'llcmFzdXJlLnYxLk93bmVyQ3V0Ughvd25lckN1dBJFCghyZWNvdmVyeRgGIAEoCzIpLnZvaWNl'
    'LmlkZW50aXR5ZXJhc3VyZS52MS5SZWNvdmVyeUJpbmRpbmdSCHJlY292ZXJ5');

@$core.Deprecated('Use resolveHistoricalCoverageRequestDescriptor instead')
const ResolveHistoricalCoverageRequest$json = {
  '1': 'ResolveHistoricalCoverageRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {'1': 'profile_ids', '3': 3, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'required_accounts',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.identityerasure.v1.AccountLifecycleFact',
      '10': 'requiredAccounts'
    },
    {
      '1': 'required_auth_cut',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'requiredAuthCut'
    },
    {
      '1': 'recovery',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
    {
      '1': 'required_user_cut',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'requiredUserCut'
    },
  ],
};

/// Descriptor for `ResolveHistoricalCoverageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveHistoricalCoverageRequestDescriptor = $convert.base64Decode(
    'CiBSZXNvbHZlSGlzdG9yaWNhbENvdmVyYWdlUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
    'EgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKcmVxdWVzdF9pZBgCIAEoCVIJcmVxdWVzdElkEh8K'
    'C3Byb2ZpbGVfaWRzGAMgAygJUgpwcm9maWxlSWRzElsKEXJlcXVpcmVkX2FjY291bnRzGAQgAy'
    'gLMi4udm9pY2UuaWRlbnRpdHllcmFzdXJlLnYxLkFjY291bnRMaWZlY3ljbGVGYWN0UhByZXF1'
    'aXJlZEFjY291bnRzEk4KEXJlcXVpcmVkX2F1dGhfY3V0GAUgASgLMiIudm9pY2UuaWRlbnRpdH'
    'llcmFzdXJlLnYxLk93bmVyQ3V0Ug9yZXF1aXJlZEF1dGhDdXQSRQoIcmVjb3ZlcnkYBiABKAsy'
    'KS52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuUmVjb3ZlcnlCaW5kaW5nUghyZWNvdmVyeRJOCh'
    'FyZXF1aXJlZF91c2VyX2N1dBgHIAEoCzIiLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52MS5Pd25l'
    'ckN1dFIPcmVxdWlyZWRVc2VyQ3V0');

@$core.Deprecated('Use resolveHistoricalCoverageResponseDescriptor instead')
const ResolveHistoricalCoverageResponse$json = {
  '1': 'ResolveHistoricalCoverageResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'request_id', '3': 2, '4': 1, '5': 9, '10': 'requestId'},
    {
      '1': 'window',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.ObservationWindow',
      '10': 'window'
    },
    {
      '1': 'profiles',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.identityerasure.v1.ProfileObservation',
      '10': 'profiles'
    },
    {
      '1': 'owner_cut',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'ownerCut'
    },
    {
      '1': 'imported_auth_cut',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.OwnerCut',
      '10': 'importedAuthCut'
    },
    {
      '1': 'matched_association_count',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'matchedAssociationCount'
    },
    {
      '1': 'association_digest',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'associationDigest'
    },
    {'1': 'complete', '3': 9, '4': 1, '5': 8, '10': 'complete'},
    {
      '1': 'recovery',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.identityerasure.v1.RecoveryBinding',
      '10': 'recovery'
    },
  ],
};

/// Descriptor for `ResolveHistoricalCoverageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveHistoricalCoverageResponseDescriptor = $convert.base64Decode(
    'CiFSZXNvbHZlSGlzdG9yaWNhbENvdmVyYWdlUmVzcG9uc2USKQoQcHJvdG9jb2xfdmVyc2lvbh'
    'gBIAEoDVIPcHJvdG9jb2xWZXJzaW9uEh0KCnJlcXVlc3RfaWQYAiABKAlSCXJlcXVlc3RJZBJD'
    'CgZ3aW5kb3cYAyABKAsyKy52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuT2JzZXJ2YXRpb25XaW'
    '5kb3dSBndpbmRvdxJICghwcm9maWxlcxgEIAMoCzIsLnZvaWNlLmlkZW50aXR5ZXJhc3VyZS52'
    'MS5Qcm9maWxlT2JzZXJ2YXRpb25SCHByb2ZpbGVzEj8KCW93bmVyX2N1dBgFIAEoCzIiLnZvaW'
    'NlLmlkZW50aXR5ZXJhc3VyZS52MS5Pd25lckN1dFIIb3duZXJDdXQSTgoRaW1wb3J0ZWRfYXV0'
    'aF9jdXQYBiABKAsyIi52b2ljZS5pZGVudGl0eWVyYXN1cmUudjEuT3duZXJDdXRSD2ltcG9ydG'
    'VkQXV0aEN1dBI6ChltYXRjaGVkX2Fzc29jaWF0aW9uX2NvdW50GAcgASgEUhdtYXRjaGVkQXNz'
    'b2NpYXRpb25Db3VudBItChJhc3NvY2lhdGlvbl9kaWdlc3QYCCABKAxSEWFzc29jaWF0aW9uRG'
    'lnZXN0EhoKCGNvbXBsZXRlGAkgASgIUghjb21wbGV0ZRJFCghyZWNvdmVyeRgKIAEoCzIpLnZv'
    'aWNlLmlkZW50aXR5ZXJhc3VyZS52MS5SZWNvdmVyeUJpbmRpbmdSCHJlY292ZXJ5');
