// This is a generated file - do not edit.
//
// Generated from voice/moderation/v1/moderation.proto.

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

@$core.Deprecated('Use reportDescriptor instead')
const Report$json = {
  '1': 'Report',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'reporter_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'reporterProfileId'
    },
    {'1': 'target_type', '3': 3, '4': 1, '5': 9, '10': 'targetType'},
    {'1': 'target_id', '3': 4, '4': 1, '5': 9, '10': 'targetId'},
    {'1': 'category', '3': 5, '4': 1, '5': 9, '10': 'category'},
    {
      '1': 'description',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'description',
      '17': true
    },
    {'1': 'evidence_json', '3': 7, '4': 1, '5': 9, '10': 'evidenceJson'},
    {'1': 'status', '3': 8, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'assigned_to_profile_id',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'assignedToProfileId',
      '17': true
    },
    {
      '1': 'resolved_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 2,
      '10': 'resolvedAt',
      '17': true
    },
    {'1': 'resolution_json', '3': 11, '4': 1, '5': 9, '10': 'resolutionJson'},
    {
      '1': 'created_at',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
  '8': [
    {'1': '_description'},
    {'1': '_assigned_to_profile_id'},
    {'1': '_resolved_at'},
  ],
};

/// Descriptor for `Report`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportDescriptor = $convert.base64Decode(
    'CgZSZXBvcnQSDgoCaWQYASABKAlSAmlkEi4KE3JlcG9ydGVyX3Byb2ZpbGVfaWQYAiABKAlSEX'
    'JlcG9ydGVyUHJvZmlsZUlkEh8KC3RhcmdldF90eXBlGAMgASgJUgp0YXJnZXRUeXBlEhsKCXRh'
    'cmdldF9pZBgEIAEoCVIIdGFyZ2V0SWQSGgoIY2F0ZWdvcnkYBSABKAlSCGNhdGVnb3J5EiUKC2'
    'Rlc2NyaXB0aW9uGAYgASgJSABSC2Rlc2NyaXB0aW9uiAEBEiMKDWV2aWRlbmNlX2pzb24YByAB'
    'KAlSDGV2aWRlbmNlSnNvbhIWCgZzdGF0dXMYCCABKAlSBnN0YXR1cxI4ChZhc3NpZ25lZF90b1'
    '9wcm9maWxlX2lkGAkgASgJSAFSE2Fzc2lnbmVkVG9Qcm9maWxlSWSIAQESQAoLcmVzb2x2ZWRf'
    'YXQYCiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAJSCnJlc29sdmVkQXSIAQESJw'
    'oPcmVzb2x1dGlvbl9qc29uGAsgASgJUg5yZXNvbHV0aW9uSnNvbhI5CgpjcmVhdGVkX2F0GAwg'
    'ASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRlZEF0Qg4KDF9kZXNjcmlwdG'
    'lvbkIZChdfYXNzaWduZWRfdG9fcHJvZmlsZV9pZEIOCgxfcmVzb2x2ZWRfYXQ=');

@$core.Deprecated('Use createReportRequestDescriptor instead')
const CreateReportRequest$json = {
  '1': 'CreateReportRequest',
  '2': [
    {'1': 'target_type', '3': 1, '4': 1, '5': 9, '10': 'targetType'},
    {'1': 'target_id', '3': 2, '4': 1, '5': 9, '10': 'targetId'},
    {'1': 'category', '3': 3, '4': 1, '5': 9, '10': 'category'},
    {
      '1': 'description',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'description',
      '17': true
    },
    {'1': 'evidence_json', '3': 5, '4': 1, '5': 9, '10': 'evidenceJson'},
  ],
  '8': [
    {'1': '_description'},
  ],
};

/// Descriptor for `CreateReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReportRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVSZXBvcnRSZXF1ZXN0Eh8KC3RhcmdldF90eXBlGAEgASgJUgp0YXJnZXRUeXBlEh'
    'sKCXRhcmdldF9pZBgCIAEoCVIIdGFyZ2V0SWQSGgoIY2F0ZWdvcnkYAyABKAlSCGNhdGVnb3J5'
    'EiUKC2Rlc2NyaXB0aW9uGAQgASgJSABSC2Rlc2NyaXB0aW9uiAEBEiMKDWV2aWRlbmNlX2pzb2'
    '4YBSABKAlSDGV2aWRlbmNlSnNvbkIOCgxfZGVzY3JpcHRpb24=');

@$core.Deprecated('Use getReportRequestDescriptor instead')
const GetReportRequest$json = {
  '1': 'GetReportRequest',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '10': 'reportId'},
  ],
};

/// Descriptor for `GetReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReportRequestDescriptor = $convert.base64Decode(
    'ChBHZXRSZXBvcnRSZXF1ZXN0EhsKCXJlcG9ydF9pZBgBIAEoCVIIcmVwb3J0SWQ=');

@$core.Deprecated('Use listReportsRequestDescriptor instead')
const ListReportsRequest$json = {
  '1': 'ListReportsRequest',
  '2': [
    {'1': 'status_filter', '3': 1, '4': 1, '5': 9, '10': 'statusFilter'},
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListReportsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listReportsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0UmVwb3J0c1JlcXVlc3QSIwoNc3RhdHVzX2ZpbHRlchgBIAEoCVIMc3RhdHVzRmlsdG'
    'VyEjYKBHBhZ2UYAiABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc29yUGFnZVJlcXVlc3RSBHBh'
    'Z2U=');

@$core.Deprecated('Use reportListDescriptor instead')
const ReportList$json = {
  '1': 'ReportList',
  '2': [
    {
      '1': 'reports',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.moderation.v1.Report',
      '10': 'reports'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `ReportList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reportListDescriptor = $convert.base64Decode(
    'CgpSZXBvcnRMaXN0EjUKB3JlcG9ydHMYASADKAsyGy52b2ljZS5tb2RlcmF0aW9uLnYxLlJlcG'
    '9ydFIHcmVwb3J0cxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use resolveReportRequestDescriptor instead')
const ResolveReportRequest$json = {
  '1': 'ResolveReportRequest',
  '2': [
    {'1': 'report_id', '3': 1, '4': 1, '5': 9, '10': 'reportId'},
    {'1': 'resolution_json', '3': 2, '4': 1, '5': 9, '10': 'resolutionJson'},
    {'1': 'new_status', '3': 3, '4': 1, '5': 9, '10': 'newStatus'},
  ],
};

/// Descriptor for `ResolveReportRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveReportRequestDescriptor = $convert.base64Decode(
    'ChRSZXNvbHZlUmVwb3J0UmVxdWVzdBIbCglyZXBvcnRfaWQYASABKAlSCHJlcG9ydElkEicKD3'
    'Jlc29sdXRpb25fanNvbhgCIAEoCVIOcmVzb2x1dGlvbkpzb24SHQoKbmV3X3N0YXR1cxgDIAEo'
    'CVIJbmV3U3RhdHVz');

@$core.Deprecated('Use sanctionDescriptor instead')
const Sanction$json = {
  '1': 'Sanction',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'target_account_id', '3': 2, '4': 1, '5': 9, '10': 'targetAccountId'},
    {'1': 'type', '3': 3, '4': 1, '5': 9, '10': 'type'},
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'report_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'reportId',
      '17': true
    },
    {
      '1': 'issued_by_profile_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '10': 'issuedByProfileId'
    },
    {
      '1': 'expires_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'expiresAt',
      '17': true
    },
    {
      '1': 'revoked_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 2,
      '10': 'revokedAt',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
  '8': [
    {'1': '_report_id'},
    {'1': '_expires_at'},
    {'1': '_revoked_at'},
  ],
};

/// Descriptor for `Sanction`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sanctionDescriptor = $convert.base64Decode(
    'CghTYW5jdGlvbhIOCgJpZBgBIAEoCVICaWQSKgoRdGFyZ2V0X2FjY291bnRfaWQYAiABKAlSD3'
    'RhcmdldEFjY291bnRJZBISCgR0eXBlGAMgASgJUgR0eXBlEhYKBnJlYXNvbhgEIAEoCVIGcmVh'
    'c29uEiAKCXJlcG9ydF9pZBgFIAEoCUgAUghyZXBvcnRJZIgBARIvChRpc3N1ZWRfYnlfcHJvZm'
    'lsZV9pZBgGIAEoCVIRaXNzdWVkQnlQcm9maWxlSWQSPgoKZXhwaXJlc19hdBgHIAEoCzIaLmdv'
    'b2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAVIJZXhwaXJlc0F0iAEBEj4KCnJldm9rZWRfYXQYCC'
    'ABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAJSCXJldm9rZWRBdIgBARI5CgpjcmVh'
    'dGVkX2F0GAkgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRlZEF0QgwKCl'
    '9yZXBvcnRfaWRCDQoLX2V4cGlyZXNfYXRCDQoLX3Jldm9rZWRfYXQ=');

@$core.Deprecated('Use applySanctionRequestDescriptor instead')
const ApplySanctionRequest$json = {
  '1': 'ApplySanctionRequest',
  '2': [
    {'1': 'target_account_id', '3': 1, '4': 1, '5': 9, '10': 'targetAccountId'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '10': 'reason'},
    {
      '1': 'report_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'reportId',
      '17': true
    },
    {
      '1': 'expires_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'expiresAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_report_id'},
    {'1': '_expires_at'},
  ],
};

/// Descriptor for `ApplySanctionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySanctionRequestDescriptor = $convert.base64Decode(
    'ChRBcHBseVNhbmN0aW9uUmVxdWVzdBIqChF0YXJnZXRfYWNjb3VudF9pZBgBIAEoCVIPdGFyZ2'
    'V0QWNjb3VudElkEhIKBHR5cGUYAiABKAlSBHR5cGUSFgoGcmVhc29uGAMgASgJUgZyZWFzb24S'
    'IAoJcmVwb3J0X2lkGAQgASgJSABSCHJlcG9ydElkiAEBEj4KCmV4cGlyZXNfYXQYBSABKAsyGi'
    '5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAFSCWV4cGlyZXNBdIgBAUIMCgpfcmVwb3J0X2lk'
    'Qg0KC19leHBpcmVzX2F0');

@$core.Deprecated('Use revokeSanctionRequestDescriptor instead')
const RevokeSanctionRequest$json = {
  '1': 'RevokeSanctionRequest',
  '2': [
    {'1': 'sanction_id', '3': 1, '4': 1, '5': 9, '10': 'sanctionId'},
  ],
};

/// Descriptor for `RevokeSanctionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSanctionRequestDescriptor = $convert.base64Decode(
    'ChVSZXZva2VTYW5jdGlvblJlcXVlc3QSHwoLc2FuY3Rpb25faWQYASABKAlSCnNhbmN0aW9uSW'
    'Q=');

@$core.Deprecated('Use getAccountSanctionsRequestDescriptor instead')
const GetAccountSanctionsRequest$json = {
  '1': 'GetAccountSanctionsRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `GetAccountSanctionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAccountSanctionsRequestDescriptor =
    $convert.base64Decode(
        'ChpHZXRBY2NvdW50U2FuY3Rpb25zUmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW'
        '50SWQ=');

@$core.Deprecated('Use sanctionListDescriptor instead')
const SanctionList$json = {
  '1': 'SanctionList',
  '2': [
    {
      '1': 'sanctions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.moderation.v1.Sanction',
      '10': 'sanctions'
    },
  ],
};

/// Descriptor for `SanctionList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sanctionListDescriptor = $convert.base64Decode(
    'CgxTYW5jdGlvbkxpc3QSOwoJc2FuY3Rpb25zGAEgAygLMh0udm9pY2UubW9kZXJhdGlvbi52MS'
    '5TYW5jdGlvblIJc2FuY3Rpb25z');

@$core.Deprecated('Use getActiveSanctionRequestDescriptor instead')
const GetActiveSanctionRequest$json = {
  '1': 'GetActiveSanctionRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `GetActiveSanctionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getActiveSanctionRequestDescriptor =
    $convert.base64Decode(
        'ChhHZXRBY3RpdmVTYW5jdGlvblJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudE'
        'lk');

@$core.Deprecated('Use appealDescriptor instead')
const Appeal$json = {
  '1': 'Appeal',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'sanction_id', '3': 2, '4': 1, '5': 9, '10': 'sanctionId'},
    {
      '1': 'appellant_account_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'appellantAccountId'
    },
    {'1': 'reason', '3': 4, '4': 1, '5': 9, '10': 'reason'},
    {'1': 'status', '3': 5, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'reviewed_by_profile_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'reviewedByProfileId',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
  '8': [
    {'1': '_reviewed_by_profile_id'},
  ],
};

/// Descriptor for `Appeal`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List appealDescriptor = $convert.base64Decode(
    'CgZBcHBlYWwSDgoCaWQYASABKAlSAmlkEh8KC3NhbmN0aW9uX2lkGAIgASgJUgpzYW5jdGlvbk'
    'lkEjAKFGFwcGVsbGFudF9hY2NvdW50X2lkGAMgASgJUhJhcHBlbGxhbnRBY2NvdW50SWQSFgoG'
    'cmVhc29uGAQgASgJUgZyZWFzb24SFgoGc3RhdHVzGAUgASgJUgZzdGF0dXMSOAoWcmV2aWV3ZW'
    'RfYnlfcHJvZmlsZV9pZBgGIAEoCUgAUhNyZXZpZXdlZEJ5UHJvZmlsZUlkiAEBEjkKCmNyZWF0'
    'ZWRfYXQYByABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgljcmVhdGVkQXRCGQoXX3'
    'Jldmlld2VkX2J5X3Byb2ZpbGVfaWQ=');

@$core.Deprecated('Use submitAppealRequestDescriptor instead')
const SubmitAppealRequest$json = {
  '1': 'SubmitAppealRequest',
  '2': [
    {'1': 'sanction_id', '3': 1, '4': 1, '5': 9, '10': 'sanctionId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `SubmitAppealRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAppealRequestDescriptor = $convert.base64Decode(
    'ChNTdWJtaXRBcHBlYWxSZXF1ZXN0Eh8KC3NhbmN0aW9uX2lkGAEgASgJUgpzYW5jdGlvbklkEh'
    'YKBnJlYXNvbhgCIAEoCVIGcmVhc29u');

@$core.Deprecated('Use reviewAppealRequestDescriptor instead')
const ReviewAppealRequest$json = {
  '1': 'ReviewAppealRequest',
  '2': [
    {'1': 'appeal_id', '3': 1, '4': 1, '5': 9, '10': 'appealId'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'moderator_note',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'moderatorNote',
      '17': true
    },
  ],
  '8': [
    {'1': '_moderator_note'},
  ],
};

/// Descriptor for `ReviewAppealRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reviewAppealRequestDescriptor = $convert.base64Decode(
    'ChNSZXZpZXdBcHBlYWxSZXF1ZXN0EhsKCWFwcGVhbF9pZBgBIAEoCVIIYXBwZWFsSWQSFgoGc3'
    'RhdHVzGAIgASgJUgZzdGF0dXMSKgoObW9kZXJhdG9yX25vdGUYAyABKAlIAFINbW9kZXJhdG9y'
    'Tm90ZYgBAUIRCg9fbW9kZXJhdG9yX25vdGU=');

@$core.Deprecated('Use getAppealRequestDescriptor instead')
const GetAppealRequest$json = {
  '1': 'GetAppealRequest',
  '2': [
    {'1': 'appeal_id', '3': 1, '4': 1, '5': 9, '10': 'appealId'},
  ],
};

/// Descriptor for `GetAppealRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAppealRequestDescriptor = $convert.base64Decode(
    'ChBHZXRBcHBlYWxSZXF1ZXN0EhsKCWFwcGVhbF9pZBgBIAEoCVIIYXBwZWFsSWQ=');

@$core.Deprecated('Use checkMessageRequestDescriptor instead')
const CheckMessageRequest$json = {
  '1': 'CheckMessageRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'content', '3': 2, '4': 1, '5': 9, '10': 'content'},
    {'1': 'sender_profile_id', '3': 3, '4': 1, '5': 9, '10': 'senderProfileId'},
  ],
};

/// Descriptor for `CheckMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkMessageRequestDescriptor = $convert.base64Decode(
    'ChNDaGVja01lc3NhZ2VSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLkNoYX'
    'RSZWZSBGNoYXQSGAoHY29udGVudBgCIAEoCVIHY29udGVudBIqChFzZW5kZXJfcHJvZmlsZV9p'
    'ZBgDIAEoCVIPc2VuZGVyUHJvZmlsZUlk');

@$core.Deprecated('Use checkResultDescriptor instead')
const CheckResult$json = {
  '1': 'CheckResult',
  '2': [
    {'1': 'allowed', '3': 1, '4': 1, '5': 8, '10': 'allowed'},
    {
      '1': 'block_reason',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'blockReason',
      '17': true
    },
  ],
  '8': [
    {'1': '_block_reason'},
  ],
};

/// Descriptor for `CheckResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkResultDescriptor = $convert.base64Decode(
    'CgtDaGVja1Jlc3VsdBIYCgdhbGxvd2VkGAEgASgIUgdhbGxvd2VkEiYKDGJsb2NrX3JlYXNvbh'
    'gCIAEoCUgAUgtibG9ja1JlYXNvbogBAUIPCg1fYmxvY2tfcmVhc29u');

@$core.Deprecated('Use getAutoModStatsRequestDescriptor instead')
const GetAutoModStatsRequest$json = {
  '1': 'GetAutoModStatsRequest',
};

/// Descriptor for `GetAutoModStatsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAutoModStatsRequestDescriptor =
    $convert.base64Decode('ChZHZXRBdXRvTW9kU3RhdHNSZXF1ZXN0');

@$core.Deprecated('Use autoModStatsDescriptor instead')
const AutoModStats$json = {
  '1': 'AutoModStats',
  '2': [
    {'1': 'messages_checked', '3': 1, '4': 1, '5': 3, '10': 'messagesChecked'},
    {'1': 'blocked', '3': 2, '4': 1, '5': 3, '10': 'blocked'},
  ],
};

/// Descriptor for `AutoModStats`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List autoModStatsDescriptor = $convert.base64Decode(
    'CgxBdXRvTW9kU3RhdHMSKQoQbWVzc2FnZXNfY2hlY2tlZBgBIAEoA1IPbWVzc2FnZXNDaGVja2'
    'VkEhgKB2Jsb2NrZWQYAiABKANSB2Jsb2NrZWQ=');

@$core.Deprecated('Use isShadowBannedRequestDescriptor instead')
const IsShadowBannedRequest$json = {
  '1': 'IsShadowBannedRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `IsShadowBannedRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List isShadowBannedRequestDescriptor = $convert.base64Decode(
    'ChVJc1NoYWRvd0Jhbm5lZFJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElk');

@$core.Deprecated('Use isShadowBannedResponseDescriptor instead')
const IsShadowBannedResponse$json = {
  '1': 'IsShadowBannedResponse',
  '2': [
    {'1': 'shadow_banned', '3': 1, '4': 1, '5': 8, '10': 'shadowBanned'},
  ],
};

/// Descriptor for `IsShadowBannedResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List isShadowBannedResponseDescriptor =
    $convert.base64Decode(
        'ChZJc1NoYWRvd0Jhbm5lZFJlc3BvbnNlEiMKDXNoYWRvd19iYW5uZWQYASABKAhSDHNoYWRvd0'
        'Jhbm5lZA==');

@$core.Deprecated('Use createReportResponseDescriptor instead')
const CreateReportResponse$json = {
  '1': 'CreateReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Report',
      '10': 'report'
    },
  ],
};

/// Descriptor for `CreateReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createReportResponseDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVSZXBvcnRSZXNwb25zZRIzCgZyZXBvcnQYASABKAsyGy52b2ljZS5tb2RlcmF0aW'
    '9uLnYxLlJlcG9ydFIGcmVwb3J0');

@$core.Deprecated('Use getReportResponseDescriptor instead')
const GetReportResponse$json = {
  '1': 'GetReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Report',
      '10': 'report'
    },
  ],
};

/// Descriptor for `GetReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReportResponseDescriptor = $convert.base64Decode(
    'ChFHZXRSZXBvcnRSZXNwb25zZRIzCgZyZXBvcnQYASABKAsyGy52b2ljZS5tb2RlcmF0aW9uLn'
    'YxLlJlcG9ydFIGcmVwb3J0');

@$core.Deprecated('Use listReportsResponseDescriptor instead')
const ListReportsResponse$json = {
  '1': 'ListReportsResponse',
  '2': [
    {
      '1': 'report_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.ReportList',
      '10': 'reportList'
    },
  ],
};

/// Descriptor for `ListReportsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listReportsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0UmVwb3J0c1Jlc3BvbnNlEkAKC3JlcG9ydF9saXN0GAEgASgLMh8udm9pY2UubW9kZX'
    'JhdGlvbi52MS5SZXBvcnRMaXN0UgpyZXBvcnRMaXN0');

@$core.Deprecated('Use resolveReportResponseDescriptor instead')
const ResolveReportResponse$json = {
  '1': 'ResolveReportResponse',
  '2': [
    {
      '1': 'report',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Report',
      '10': 'report'
    },
  ],
};

/// Descriptor for `ResolveReportResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolveReportResponseDescriptor = $convert.base64Decode(
    'ChVSZXNvbHZlUmVwb3J0UmVzcG9uc2USMwoGcmVwb3J0GAEgASgLMhsudm9pY2UubW9kZXJhdG'
    'lvbi52MS5SZXBvcnRSBnJlcG9ydA==');

@$core.Deprecated('Use applySanctionResponseDescriptor instead')
const ApplySanctionResponse$json = {
  '1': 'ApplySanctionResponse',
  '2': [
    {
      '1': 'sanction',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Sanction',
      '10': 'sanction'
    },
  ],
};

/// Descriptor for `ApplySanctionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applySanctionResponseDescriptor = $convert.base64Decode(
    'ChVBcHBseVNhbmN0aW9uUmVzcG9uc2USOQoIc2FuY3Rpb24YASABKAsyHS52b2ljZS5tb2Rlcm'
    'F0aW9uLnYxLlNhbmN0aW9uUghzYW5jdGlvbg==');

@$core.Deprecated('Use revokeSanctionResponseDescriptor instead')
const RevokeSanctionResponse$json = {
  '1': 'RevokeSanctionResponse',
};

/// Descriptor for `RevokeSanctionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSanctionResponseDescriptor =
    $convert.base64Decode('ChZSZXZva2VTYW5jdGlvblJlc3BvbnNl');

@$core.Deprecated('Use getAccountSanctionsResponseDescriptor instead')
const GetAccountSanctionsResponse$json = {
  '1': 'GetAccountSanctionsResponse',
  '2': [
    {
      '1': 'sanction_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.SanctionList',
      '10': 'sanctionList'
    },
  ],
};

/// Descriptor for `GetAccountSanctionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAccountSanctionsResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRBY2NvdW50U2FuY3Rpb25zUmVzcG9uc2USRgoNc2FuY3Rpb25fbGlzdBgBIAEoCzIhLn'
        'ZvaWNlLm1vZGVyYXRpb24udjEuU2FuY3Rpb25MaXN0UgxzYW5jdGlvbkxpc3Q=');

@$core.Deprecated('Use getActiveSanctionResponseDescriptor instead')
const GetActiveSanctionResponse$json = {
  '1': 'GetActiveSanctionResponse',
  '2': [
    {
      '1': 'sanction',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Sanction',
      '10': 'sanction'
    },
  ],
};

/// Descriptor for `GetActiveSanctionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getActiveSanctionResponseDescriptor =
    $convert.base64Decode(
        'ChlHZXRBY3RpdmVTYW5jdGlvblJlc3BvbnNlEjkKCHNhbmN0aW9uGAEgASgLMh0udm9pY2UubW'
        '9kZXJhdGlvbi52MS5TYW5jdGlvblIIc2FuY3Rpb24=');

@$core.Deprecated('Use submitAppealResponseDescriptor instead')
const SubmitAppealResponse$json = {
  '1': 'SubmitAppealResponse',
  '2': [
    {
      '1': 'appeal',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Appeal',
      '10': 'appeal'
    },
  ],
};

/// Descriptor for `SubmitAppealResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List submitAppealResponseDescriptor = $convert.base64Decode(
    'ChRTdWJtaXRBcHBlYWxSZXNwb25zZRIzCgZhcHBlYWwYASABKAsyGy52b2ljZS5tb2RlcmF0aW'
    '9uLnYxLkFwcGVhbFIGYXBwZWFs');

@$core.Deprecated('Use reviewAppealResponseDescriptor instead')
const ReviewAppealResponse$json = {
  '1': 'ReviewAppealResponse',
  '2': [
    {
      '1': 'appeal',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Appeal',
      '10': 'appeal'
    },
  ],
};

/// Descriptor for `ReviewAppealResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reviewAppealResponseDescriptor = $convert.base64Decode(
    'ChRSZXZpZXdBcHBlYWxSZXNwb25zZRIzCgZhcHBlYWwYASABKAsyGy52b2ljZS5tb2RlcmF0aW'
    '9uLnYxLkFwcGVhbFIGYXBwZWFs');

@$core.Deprecated('Use getAppealResponseDescriptor instead')
const GetAppealResponse$json = {
  '1': 'GetAppealResponse',
  '2': [
    {
      '1': 'appeal',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.Appeal',
      '10': 'appeal'
    },
  ],
};

/// Descriptor for `GetAppealResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAppealResponseDescriptor = $convert.base64Decode(
    'ChFHZXRBcHBlYWxSZXNwb25zZRIzCgZhcHBlYWwYASABKAsyGy52b2ljZS5tb2RlcmF0aW9uLn'
    'YxLkFwcGVhbFIGYXBwZWFs');

@$core.Deprecated('Use checkMessageResponseDescriptor instead')
const CheckMessageResponse$json = {
  '1': 'CheckMessageResponse',
  '2': [
    {
      '1': 'check_result',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.CheckResult',
      '10': 'checkResult'
    },
  ],
};

/// Descriptor for `CheckMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkMessageResponseDescriptor = $convert.base64Decode(
    'ChRDaGVja01lc3NhZ2VSZXNwb25zZRJDCgxjaGVja19yZXN1bHQYASABKAsyIC52b2ljZS5tb2'
    'RlcmF0aW9uLnYxLkNoZWNrUmVzdWx0UgtjaGVja1Jlc3VsdA==');

@$core.Deprecated('Use getAutoModStatsResponseDescriptor instead')
const GetAutoModStatsResponse$json = {
  '1': 'GetAutoModStatsResponse',
  '2': [
    {
      '1': 'auto_mod_stats',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.moderation.v1.AutoModStats',
      '10': 'autoModStats'
    },
  ],
};

/// Descriptor for `GetAutoModStatsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getAutoModStatsResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRBdXRvTW9kU3RhdHNSZXNwb25zZRJHCg5hdXRvX21vZF9zdGF0cxgBIAEoCzIhLnZvaW'
        'NlLm1vZGVyYXRpb24udjEuQXV0b01vZFN0YXRzUgxhdXRvTW9kU3RhdHM=');
