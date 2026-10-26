// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/subscription_lifecycle.proto.

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

@$core.Deprecated('Use subscriptionAccountCaptureCoverageDescriptor instead')
const SubscriptionAccountCaptureCoverage$json = {
  '1': 'SubscriptionAccountCaptureCoverage',
  '2': [
    {'1': 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNSPECIFIED', '2': 0},
    {'1': 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED', '2': 1},
    {'1': 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED', '2': 2},
  ],
};

/// Descriptor for `SubscriptionAccountCaptureCoverage`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List subscriptionAccountCaptureCoverageDescriptor =
    $convert.base64Decode(
        'CiJTdWJzY3JpcHRpb25BY2NvdW50Q2FwdHVyZUNvdmVyYWdlEjUKMVNVQlNDUklQVElPTl9BQ0'
        'NPVU5UX0NBUFRVUkVfQ09WRVJBR0VfVU5TUEVDSUZJRUQQABI0CjBTVUJTQ1JJUFRJT05fQUND'
        'T1VOVF9DQVBUVVJFX0NPVkVSQUdFX1VOUkVDT1JERUQQARIyCi5TVUJTQ1JJUFRJT05fQUNDT1'
        'VOVF9DQVBUVVJFX0NPVkVSQUdFX0NBUFRVUkVEEAI=');

@$core.Deprecated('Use subscriptionAccountLifecycleKindDescriptor instead')
const SubscriptionAccountLifecycleKind$json = {
  '1': 'SubscriptionAccountLifecycleKind',
  '2': [
    {'1': 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_UNSPECIFIED', '2': 0},
    {'1': 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED', '2': 1},
    {'1': 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED', '2': 2},
  ],
};

/// Descriptor for `SubscriptionAccountLifecycleKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List subscriptionAccountLifecycleKindDescriptor =
    $convert.base64Decode(
        'CiBTdWJzY3JpcHRpb25BY2NvdW50TGlmZWN5Y2xlS2luZBIzCi9TVUJTQ1JJUFRJT05fQUNDT1'
        'VOVF9MSUZFQ1lDTEVfS0lORF9VTlNQRUNJRklFRBAAEjgKNFNVQlNDUklQVElPTl9BQ0NPVU5U'
        'X0xJRkVDWUNMRV9LSU5EX0RFTEVURV9TQ0hFRFVMRUQQARI5CjVTVUJTQ1JJUFRJT05fQUNDT1'
        'VOVF9MSUZFQ1lDTEVfS0lORF9SRVNUT1JFX1JFUVVFU1RFRBAC');

@$core.Deprecated('Use observeSubscriptionAccountRequestDescriptor instead')
const ObserveSubscriptionAccountRequest$json = {
  '1': 'ObserveSubscriptionAccountRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'minimum_lifecycle_revision',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'minimumLifecycleRevision'
    },
  ],
};

/// Descriptor for `ObserveSubscriptionAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeSubscriptionAccountRequestDescriptor =
    $convert.base64Decode(
        'CiFPYnNlcnZlU3Vic2NyaXB0aW9uQWNjb3VudFJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCV'
        'IJYWNjb3VudElkEjwKGm1pbmltdW1fbGlmZWN5Y2xlX3JldmlzaW9uGAIgASgEUhhtaW5pbXVt'
        'TGlmZWN5Y2xlUmV2aXNpb24=');

@$core.Deprecated('Use subscriptionAccountLifecycleFactDescriptor instead')
const SubscriptionAccountLifecycleFact$json = {
  '1': 'SubscriptionAccountLifecycleFact',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'source_revision', '3': 2, '4': 1, '5': 4, '10': 'sourceRevision'},
    {'1': 'event_id', '3': 3, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'deletion_cycle_id', '3': 4, '4': 1, '5': 9, '10': 'deletionCycleId'},
    {
      '1': 'kind',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.auth.v1.SubscriptionAccountLifecycleKind',
      '10': 'kind'
    },
    {'1': 'session_epoch', '3': 6, '4': 1, '5': 4, '10': 'sessionEpoch'},
    {
      '1': 'scheduled_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'scheduledAt'
    },
    {
      '1': 'purge_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'purgeAt'
    },
    {
      '1': 'effective_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'effectiveAt'
    },
    {
      '1': 'captured_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'capturedAt'
    },
  ],
};

/// Descriptor for `SubscriptionAccountLifecycleFact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscriptionAccountLifecycleFactDescriptor = $convert.base64Decode(
    'CiBTdWJzY3JpcHRpb25BY2NvdW50TGlmZWN5Y2xlRmFjdBIdCgphY2NvdW50X2lkGAEgASgJUg'
    'lhY2NvdW50SWQSJwoPc291cmNlX3JldmlzaW9uGAIgASgEUg5zb3VyY2VSZXZpc2lvbhIZCghl'
    'dmVudF9pZBgDIAEoCVIHZXZlbnRJZBIqChFkZWxldGlvbl9jeWNsZV9pZBgEIAEoCVIPZGVsZX'
    'Rpb25DeWNsZUlkEkMKBGtpbmQYBSABKA4yLy52b2ljZS5hdXRoLnYxLlN1YnNjcmlwdGlvbkFj'
    'Y291bnRMaWZlY3ljbGVLaW5kUgRraW5kEiMKDXNlc3Npb25fZXBvY2gYBiABKARSDHNlc3Npb2'
    '5FcG9jaBI9CgxzY2hlZHVsZWRfYXQYByABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1w'
    'UgtzY2hlZHVsZWRBdBI1CghwdXJnZV9hdBgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3'
    'RhbXBSB3B1cmdlQXQSPQoMZWZmZWN0aXZlX2F0GAkgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRp'
    'bWVzdGFtcFILZWZmZWN0aXZlQXQSOwoLY2FwdHVyZWRfYXQYCiABKAsyGi5nb29nbGUucHJvdG'
    '9idWYuVGltZXN0YW1wUgpjYXB0dXJlZEF0');

@$core.Deprecated('Use observeSubscriptionAccountResponseDescriptor instead')
const ObserveSubscriptionAccountResponse$json = {
  '1': 'ObserveSubscriptionAccountResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'account_type', '3': 3, '4': 1, '5': 9, '10': 'accountType'},
    {'1': 'account_status', '3': 4, '4': 1, '5': 9, '10': 'accountStatus'},
    {
      '1': 'regular_email_verification_pending',
      '3': 5,
      '4': 1,
      '5': 8,
      '10': 'regularEmailVerificationPending'
    },
    {'1': 'session_epoch', '3': 6, '4': 1, '5': 4, '10': 'sessionEpoch'},
    {
      '1': 'security_revision',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'securityRevision'
    },
    {
      '1': 'deleted_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'deletedAt'
    },
    {
      '1': 'observed_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'observedAt'
    },
    {
      '1': 'capture_coverage',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.voice.auth.v1.SubscriptionAccountCaptureCoverage',
      '10': 'captureCoverage'
    },
    {
      '1': 'head',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.SubscriptionAccountLifecycleFact',
      '10': 'head'
    },
    {
      '1': 'facts',
      '3': 12,
      '4': 3,
      '5': 11,
      '6': '.voice.auth.v1.SubscriptionAccountLifecycleFact',
      '10': 'facts'
    },
  ],
};

/// Descriptor for `ObserveSubscriptionAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List observeSubscriptionAccountResponseDescriptor = $convert.base64Decode(
    'CiJPYnNlcnZlU3Vic2NyaXB0aW9uQWNjb3VudFJlc3BvbnNlEikKEHByb3RvY29sX3ZlcnNpb2'
    '4YASABKA1SD3Byb3RvY29sVmVyc2lvbhIdCgphY2NvdW50X2lkGAIgASgJUglhY2NvdW50SWQS'
    'IQoMYWNjb3VudF90eXBlGAMgASgJUgthY2NvdW50VHlwZRIlCg5hY2NvdW50X3N0YXR1cxgEIA'
    'EoCVINYWNjb3VudFN0YXR1cxJLCiJyZWd1bGFyX2VtYWlsX3ZlcmlmaWNhdGlvbl9wZW5kaW5n'
    'GAUgASgIUh9yZWd1bGFyRW1haWxWZXJpZmljYXRpb25QZW5kaW5nEiMKDXNlc3Npb25fZXBvY2'
    'gYBiABKARSDHNlc3Npb25FcG9jaBIrChFzZWN1cml0eV9yZXZpc2lvbhgHIAEoBFIQc2VjdXJp'
    'dHlSZXZpc2lvbhI5CgpkZWxldGVkX2F0GAggASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdG'
    'FtcFIJZGVsZXRlZEF0EjsKC29ic2VydmVkX2F0GAkgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRp'
    'bWVzdGFtcFIKb2JzZXJ2ZWRBdBJcChBjYXB0dXJlX2NvdmVyYWdlGAogASgOMjEudm9pY2UuYX'
    'V0aC52MS5TdWJzY3JpcHRpb25BY2NvdW50Q2FwdHVyZUNvdmVyYWdlUg9jYXB0dXJlQ292ZXJh'
    'Z2USQwoEaGVhZBgLIAEoCzIvLnZvaWNlLmF1dGgudjEuU3Vic2NyaXB0aW9uQWNjb3VudExpZm'
    'VjeWNsZUZhY3RSBGhlYWQSRQoFZmFjdHMYDCADKAsyLy52b2ljZS5hdXRoLnYxLlN1YnNjcmlw'
    'dGlvbkFjY291bnRMaWZlY3ljbGVGYWN0UgVmYWN0cw==');
