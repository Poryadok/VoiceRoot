// This is a generated file - do not edit.
//
// Generated from voice/user/v1/space_account_capture.proto.

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

@$core.Deprecated('Use spaceAccountCapturePurposeDescriptor instead')
const SpaceAccountCapturePurpose$json = {
  '1': 'SpaceAccountCapturePurpose',
  '2': [
    {'1': 'SPACE_ACCOUNT_CAPTURE_PURPOSE_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_ACCOUNT_CAPTURE_PURPOSE_LEGACY_QUALIFICATION', '2': 1},
    {'1': 'SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT', '2': 2},
    {'1': 'SPACE_ACCOUNT_CAPTURE_PURPOSE_COMMUNITY_PUBLICATION', '2': 3},
  ],
};

/// Descriptor for `SpaceAccountCapturePurpose`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceAccountCapturePurposeDescriptor = $convert.base64Decode(
    'ChpTcGFjZUFjY291bnRDYXB0dXJlUHVycG9zZRItCilTUEFDRV9BQ0NPVU5UX0NBUFRVUkVfUF'
    'VSUE9TRV9VTlNQRUNJRklFRBAAEjYKMlNQQUNFX0FDQ09VTlRfQ0FQVFVSRV9QVVJQT1NFX0xF'
    'R0FDWV9RVUFMSUZJQ0FUSU9OEAESLworU1BBQ0VfQUNDT1VOVF9DQVBUVVJFX1BVUlBPU0VfQk'
    'FOX1BSRUZMSUdIVBACEjcKM1NQQUNFX0FDQ09VTlRfQ0FQVFVSRV9QVVJQT1NFX0NPTU1VTklU'
    'WV9QVUJMSUNBVElPThAD');

@$core.Deprecated('Use spaceAccountCaptureOutcomeDescriptor instead')
const SpaceAccountCaptureOutcome$json = {
  '1': 'SpaceAccountCaptureOutcome',
  '2': [
    {'1': 'SPACE_ACCOUNT_CAPTURE_OUTCOME_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED', '2': 1},
    {'1': 'SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE', '2': 2},
    {'1': 'SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE', '2': 3},
  ],
};

/// Descriptor for `SpaceAccountCaptureOutcome`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceAccountCaptureOutcomeDescriptor = $convert.base64Decode(
    'ChpTcGFjZUFjY291bnRDYXB0dXJlT3V0Y29tZRItCilTUEFDRV9BQ0NPVU5UX0NBUFRVUkVfT1'
    'VUQ09NRV9VTlNQRUNJRklFRBAAEiwKKFNQQUNFX0FDQ09VTlRfQ0FQVFVSRV9PVVRDT01FX0FT'
    'U09DSUFURUQQARIyCi5TUEFDRV9BQ0NPVU5UX0NBUFRVUkVfT1VUQ09NRV9URVJNSU5BTF9QUk'
    '9GSUxFEAISLAooU1BBQ0VfQUNDT1VOVF9DQVBUVVJFX09VVENPTUVfSU5DT01QTEVURRAD');

@$core.Deprecated('Use spaceAccountCaptureOriginDescriptor instead')
const SpaceAccountCaptureOrigin$json = {
  '1': 'SpaceAccountCaptureOrigin',
  '2': [
    {'1': 'SPACE_ACCOUNT_CAPTURE_ORIGIN_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW', '2': 1},
    {'1': 'SPACE_ACCOUNT_CAPTURE_ORIGIN_OWNER_HISTORY', '2': 2},
  ],
};

/// Descriptor for `SpaceAccountCaptureOrigin`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceAccountCaptureOriginDescriptor = $convert.base64Decode(
    'ChlTcGFjZUFjY291bnRDYXB0dXJlT3JpZ2luEiwKKFNQQUNFX0FDQ09VTlRfQ0FQVFVSRV9PUk'
    'lHSU5fVU5TUEVDSUZJRUQQABI1CjFTUEFDRV9BQ0NPVU5UX0NBUFRVUkVfT1JJR0lOX0VYSVNU'
    'SU5HX0NVUlJFTlRfUk9XEAESLgoqU1BBQ0VfQUNDT1VOVF9DQVBUVVJFX09SSUdJTl9PV05FUl'
    '9ISVNUT1JZEAI=');

@$core.Deprecated('Use spaceAccountAssociationOriginDescriptor instead')
const SpaceAccountAssociationOrigin$json = {
  '1': 'SpaceAccountAssociationOrigin',
  '2': [
    {'1': 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_UNSPECIFIED', '2': 0},
    {'1': 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH', '2': 1},
    {'1': 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW', '2': 2},
    {'1': 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_VERIFIED_HISTORY', '2': 3},
  ],
};

/// Descriptor for `SpaceAccountAssociationOrigin`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List spaceAccountAssociationOriginDescriptor = $convert.base64Decode(
    'Ch1TcGFjZUFjY291bnRBc3NvY2lhdGlvbk9yaWdpbhIwCixTUEFDRV9BQ0NPVU5UX0FTU09DSU'
    'FUSU9OX09SSUdJTl9VTlNQRUNJRklFRBAAEioKJlNQQUNFX0FDQ09VTlRfQVNTT0NJQVRJT05f'
    'T1JJR0lOX0JJUlRIEAESMQotU1BBQ0VfQUNDT1VOVF9BU1NPQ0lBVElPTl9PUklHSU5fUkVUQU'
    'lORURfTk9XEAISNQoxU1BBQ0VfQUNDT1VOVF9BU1NPQ0lBVElPTl9PUklHSU5fVkVSSUZJRURf'
    'SElTVE9SWRAD');

@$core.Deprecated('Use spaceOrdinaryPublicationIdentityDescriptor instead')
const SpaceOrdinaryPublicationIdentity$json = {
  '1': 'SpaceOrdinaryPublicationIdentity',
  '2': [
    {'1': 'generation', '3': 1, '4': 1, '5': 4, '10': 'generation'},
    {'1': 'publication_id', '3': 2, '4': 1, '5': 9, '10': 'publicationId'},
  ],
};

/// Descriptor for `SpaceOrdinaryPublicationIdentity`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceOrdinaryPublicationIdentityDescriptor =
    $convert.base64Decode(
        'CiBTcGFjZU9yZGluYXJ5UHVibGljYXRpb25JZGVudGl0eRIeCgpnZW5lcmF0aW9uGAEgASgEUg'
        'pnZW5lcmF0aW9uEiUKDnB1YmxpY2F0aW9uX2lkGAIgASgJUg1wdWJsaWNhdGlvbklk');

@$core.Deprecated('Use spaceCommunityPublicationIdentityDescriptor instead')
const SpaceCommunityPublicationIdentity$json = {
  '1': 'SpaceCommunityPublicationIdentity',
  '2': [
    {'1': 'owner_generation', '3': 1, '4': 1, '5': 4, '10': 'ownerGeneration'},
    {
      '1': 'member_generation',
      '3': 2,
      '4': 1,
      '5': 4,
      '10': 'memberGeneration'
    },
    {'1': 'episode_id', '3': 3, '4': 1, '5': 9, '10': 'episodeId'},
  ],
};

/// Descriptor for `SpaceCommunityPublicationIdentity`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceCommunityPublicationIdentityDescriptor =
    $convert.base64Decode(
        'CiFTcGFjZUNvbW11bml0eVB1YmxpY2F0aW9uSWRlbnRpdHkSKQoQb3duZXJfZ2VuZXJhdGlvbh'
        'gBIAEoBFIPb3duZXJHZW5lcmF0aW9uEisKEW1lbWJlcl9nZW5lcmF0aW9uGAIgASgEUhBtZW1i'
        'ZXJHZW5lcmF0aW9uEh0KCmVwaXNvZGVfaWQYAyABKAlSCWVwaXNvZGVJZA==');

@$core.Deprecated('Use spaceProspectiveCommunityIdentityDescriptor instead')
const SpaceProspectiveCommunityIdentity$json = {
  '1': 'SpaceProspectiveCommunityIdentity',
  '2': [
    {'1': 'operation_id', '3': 1, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'owner_generation', '3': 2, '4': 1, '5': 4, '10': 'ownerGeneration'},
    {'1': 'source_revision', '3': 3, '4': 1, '5': 4, '10': 'sourceRevision'},
  ],
};

/// Descriptor for `SpaceProspectiveCommunityIdentity`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceProspectiveCommunityIdentityDescriptor =
    $convert.base64Decode(
        'CiFTcGFjZVByb3NwZWN0aXZlQ29tbXVuaXR5SWRlbnRpdHkSIQoMb3BlcmF0aW9uX2lkGAEgAS'
        'gJUgtvcGVyYXRpb25JZBIpChBvd25lcl9nZW5lcmF0aW9uGAIgASgEUg9vd25lckdlbmVyYXRp'
        'b24SJwoPc291cmNlX3JldmlzaW9uGAMgASgEUg5zb3VyY2VSZXZpc2lvbg==');

@$core.Deprecated('Use spaceAccountCaptureEntryDescriptor instead')
const SpaceAccountCaptureEntry$json = {
  '1': 'SpaceAccountCaptureEntry',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'ordinary',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceOrdinaryPublicationIdentity',
      '9': 0,
      '10': 'ordinary'
    },
    {
      '1': 'community',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceCommunityPublicationIdentity',
      '9': 0,
      '10': 'community'
    },
    {
      '1': 'prospective_community',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceProspectiveCommunityIdentity',
      '9': 0,
      '10': 'prospectiveCommunity'
    },
    {
      '1': 'expected_association_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'expectedAssociationId'
    },
    {
      '1': 'expected_association_revision',
      '3': 6,
      '4': 1,
      '5': 4,
      '10': 'expectedAssociationRevision'
    },
  ],
  '8': [
    {'1': 'source'},
  ],
};

/// Descriptor for `SpaceAccountCaptureEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceAccountCaptureEntryDescriptor = $convert.base64Decode(
    'ChhTcGFjZUFjY291bnRDYXB0dXJlRW50cnkSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZU'
    'lkEk0KCG9yZGluYXJ5GAIgASgLMi8udm9pY2UudXNlci52MS5TcGFjZU9yZGluYXJ5UHVibGlj'
    'YXRpb25JZGVudGl0eUgAUghvcmRpbmFyeRJQCgljb21tdW5pdHkYAyABKAsyMC52b2ljZS51c2'
    'VyLnYxLlNwYWNlQ29tbXVuaXR5UHVibGljYXRpb25JZGVudGl0eUgAUgljb21tdW5pdHkSZwoV'
    'cHJvc3BlY3RpdmVfY29tbXVuaXR5GAQgASgLMjAudm9pY2UudXNlci52MS5TcGFjZVByb3NwZW'
    'N0aXZlQ29tbXVuaXR5SWRlbnRpdHlIAFIUcHJvc3BlY3RpdmVDb21tdW5pdHkSNgoXZXhwZWN0'
    'ZWRfYXNzb2NpYXRpb25faWQYBSABKAlSFWV4cGVjdGVkQXNzb2NpYXRpb25JZBJCCh1leHBlY3'
    'RlZF9hc3NvY2lhdGlvbl9yZXZpc2lvbhgGIAEoBFIbZXhwZWN0ZWRBc3NvY2lhdGlvblJldmlz'
    'aW9uQggKBnNvdXJjZQ==');

@$core.Deprecated('Use spaceAccountCaptureCohortDescriptor instead')
const SpaceAccountCaptureCohort$json = {
  '1': 'SpaceAccountCaptureCohort',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.user.v1.SpaceAccountCaptureEntry',
      '10': 'entries'
    },
  ],
};

/// Descriptor for `SpaceAccountCaptureCohort`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceAccountCaptureCohortDescriptor =
    $convert.base64Decode(
        'ChlTcGFjZUFjY291bnRDYXB0dXJlQ29ob3J0EkEKB2VudHJpZXMYASADKAsyJy52b2ljZS51c2'
        'VyLnYxLlNwYWNlQWNjb3VudENhcHR1cmVFbnRyeVIHZW50cmllcw==');

@$core.Deprecated(
    'Use captureSpaceMemberAccountBindingsRequestDescriptor instead')
const CaptureSpaceMemberAccountBindingsRequest$json = {
  '1': 'CaptureSpaceMemberAccountBindingsRequest',
  '2': [
    {'1': 'protocol', '3': 1, '4': 1, '5': 13, '10': 'protocol'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'purpose',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.SpaceAccountCapturePurpose',
      '10': 'purpose'
    },
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'cohort_digest', '3': 5, '4': 1, '5': 12, '10': 'cohortDigest'},
    {
      '1': 'cohort',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceAccountCaptureCohort',
      '10': 'cohort'
    },
  ],
};

/// Descriptor for `CaptureSpaceMemberAccountBindingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List captureSpaceMemberAccountBindingsRequestDescriptor =
    $convert.base64Decode(
        'CihDYXB0dXJlU3BhY2VNZW1iZXJBY2NvdW50QmluZGluZ3NSZXF1ZXN0EhoKCHByb3RvY29sGA'
        'EgASgNUghwcm90b2NvbBIhCgxvcGVyYXRpb25faWQYAiABKAlSC29wZXJhdGlvbklkEkMKB3B1'
        'cnBvc2UYAyABKA4yKS52b2ljZS51c2VyLnYxLlNwYWNlQWNjb3VudENhcHR1cmVQdXJwb3NlUg'
        'dwdXJwb3NlEhkKCHNwYWNlX2lkGAQgASgJUgdzcGFjZUlkEiMKDWNvaG9ydF9kaWdlc3QYBSAB'
        'KAxSDGNvaG9ydERpZ2VzdBJACgZjb2hvcnQYBiABKAsyKC52b2ljZS51c2VyLnYxLlNwYWNlQW'
        'Njb3VudENhcHR1cmVDb2hvcnRSBmNvaG9ydA==');

@$core.Deprecated('Use spaceAccountIdentityCutDescriptor instead')
const SpaceAccountIdentityCut$json = {
  '1': 'SpaceAccountIdentityCut',
  '2': [
    {'1': 'owner', '3': 1, '4': 1, '5': 9, '10': 'owner'},
    {'1': 'stream', '3': 2, '4': 1, '5': 9, '10': 'stream'},
    {
      '1': 'history_generation',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'historyGeneration'
    },
    {'1': 'through_revision', '3': 4, '4': 1, '5': 4, '10': 'throughRevision'},
    {'1': 'keyset_revision', '3': 5, '4': 1, '5': 4, '10': 'keysetRevision'},
    {'1': 'digest', '3': 6, '4': 1, '5': 12, '10': 'digest'},
    {'1': 'retained_floor', '3': 7, '4': 1, '5': 4, '10': 'retainedFloor'},
  ],
};

/// Descriptor for `SpaceAccountIdentityCut`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceAccountIdentityCutDescriptor = $convert.base64Decode(
    'ChdTcGFjZUFjY291bnRJZGVudGl0eUN1dBIUCgVvd25lchgBIAEoCVIFb3duZXISFgoGc3RyZW'
    'FtGAIgASgJUgZzdHJlYW0SLQoSaGlzdG9yeV9nZW5lcmF0aW9uGAMgASgJUhFoaXN0b3J5R2Vu'
    'ZXJhdGlvbhIpChB0aHJvdWdoX3JldmlzaW9uGAQgASgEUg90aHJvdWdoUmV2aXNpb24SJwoPa2'
    'V5c2V0X3JldmlzaW9uGAUgASgEUg5rZXlzZXRSZXZpc2lvbhIWCgZkaWdlc3QYBiABKAxSBmRp'
    'Z2VzdBIlCg5yZXRhaW5lZF9mbG9vchgHIAEoBFINcmV0YWluZWRGbG9vcg==');

@$core.Deprecated('Use spaceAccountCaptureResultDescriptor instead')
const SpaceAccountCaptureResult$json = {
  '1': 'SpaceAccountCaptureResult',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceAccountCaptureEntry',
      '10': 'entry'
    },
    {
      '1': 'outcome',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.SpaceAccountCaptureOutcome',
      '10': 'outcome'
    },
    {'1': 'account_id', '3': 3, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'association_id', '3': 4, '4': 1, '5': 9, '10': 'associationId'},
    {
      '1': 'association_revision',
      '3': 5,
      '4': 1,
      '5': 4,
      '10': 'associationRevision'
    },
    {'1': 'profile_revision', '3': 6, '4': 1, '5': 4, '10': 'profileRevision'},
    {'1': 'deleted', '3': 7, '4': 1, '5': 8, '10': 'deleted'},
    {'1': 'frozen', '3': 8, '4': 1, '5': 8, '10': 'frozen'},
    {
      '1': 'origin',
      '3': 9,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.SpaceAccountCaptureOrigin',
      '10': 'origin'
    },
    {'1': 'owner_receipt_id', '3': 10, '4': 1, '5': 9, '10': 'ownerReceiptId'},
    {
      '1': 'owner_receipt_digest',
      '3': 11,
      '4': 1,
      '5': 12,
      '10': 'ownerReceiptDigest'
    },
    {
      '1': 'terminal_operation_id',
      '3': 12,
      '4': 1,
      '5': 9,
      '10': 'terminalOperationId'
    },
    {
      '1': 'terminal_revision',
      '3': 13,
      '4': 1,
      '5': 4,
      '10': 'terminalRevision'
    },
    {
      '1': 'terminal_receipt_id',
      '3': 14,
      '4': 1,
      '5': 9,
      '10': 'terminalReceiptId'
    },
    {
      '1': 'terminal_receipt_digest',
      '3': 15,
      '4': 1,
      '5': 12,
      '10': 'terminalReceiptDigest'
    },
    {
      '1': 'capture_cut',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'captureCut'
    },
    {
      '1': 'association_origin',
      '3': 17,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.SpaceAccountAssociationOrigin',
      '10': 'associationOrigin'
    },
    {'1': 'account_inactive', '3': 18, '4': 1, '5': 8, '10': 'accountInactive'},
  ],
};

/// Descriptor for `SpaceAccountCaptureResult`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceAccountCaptureResultDescriptor = $convert.base64Decode(
    'ChlTcGFjZUFjY291bnRDYXB0dXJlUmVzdWx0Ej0KBWVudHJ5GAEgASgLMicudm9pY2UudXNlci'
    '52MS5TcGFjZUFjY291bnRDYXB0dXJlRW50cnlSBWVudHJ5EkMKB291dGNvbWUYAiABKA4yKS52'
    'b2ljZS51c2VyLnYxLlNwYWNlQWNjb3VudENhcHR1cmVPdXRjb21lUgdvdXRjb21lEh0KCmFjY2'
    '91bnRfaWQYAyABKAlSCWFjY291bnRJZBIlCg5hc3NvY2lhdGlvbl9pZBgEIAEoCVINYXNzb2Np'
    'YXRpb25JZBIxChRhc3NvY2lhdGlvbl9yZXZpc2lvbhgFIAEoBFITYXNzb2NpYXRpb25SZXZpc2'
    'lvbhIpChBwcm9maWxlX3JldmlzaW9uGAYgASgEUg9wcm9maWxlUmV2aXNpb24SGAoHZGVsZXRl'
    'ZBgHIAEoCFIHZGVsZXRlZBIWCgZmcm96ZW4YCCABKAhSBmZyb3plbhJACgZvcmlnaW4YCSABKA'
    '4yKC52b2ljZS51c2VyLnYxLlNwYWNlQWNjb3VudENhcHR1cmVPcmlnaW5SBm9yaWdpbhIoChBv'
    'd25lcl9yZWNlaXB0X2lkGAogASgJUg5vd25lclJlY2VpcHRJZBIwChRvd25lcl9yZWNlaXB0X2'
    'RpZ2VzdBgLIAEoDFISb3duZXJSZWNlaXB0RGlnZXN0EjIKFXRlcm1pbmFsX29wZXJhdGlvbl9p'
    'ZBgMIAEoCVITdGVybWluYWxPcGVyYXRpb25JZBIrChF0ZXJtaW5hbF9yZXZpc2lvbhgNIAEoBF'
    'IQdGVybWluYWxSZXZpc2lvbhIuChN0ZXJtaW5hbF9yZWNlaXB0X2lkGA4gASgJUhF0ZXJtaW5h'
    'bFJlY2VpcHRJZBI2Chd0ZXJtaW5hbF9yZWNlaXB0X2RpZ2VzdBgPIAEoDFIVdGVybWluYWxSZW'
    'NlaXB0RGlnZXN0EjsKC2NhcHR1cmVfY3V0GBAgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVz'
    'dGFtcFIKY2FwdHVyZUN1dBJbChJhc3NvY2lhdGlvbl9vcmlnaW4YESABKA4yLC52b2ljZS51c2'
    'VyLnYxLlNwYWNlQWNjb3VudEFzc29jaWF0aW9uT3JpZ2luUhFhc3NvY2lhdGlvbk9yaWdpbhIp'
    'ChBhY2NvdW50X2luYWN0aXZlGBIgASgIUg9hY2NvdW50SW5hY3RpdmU=');

@$core.Deprecated(
    'Use captureSpaceMemberAccountBindingsResponseDescriptor instead')
const CaptureSpaceMemberAccountBindingsResponse$json = {
  '1': 'CaptureSpaceMemberAccountBindingsResponse',
  '2': [
    {'1': 'protocol', '3': 1, '4': 1, '5': 13, '10': 'protocol'},
    {'1': 'operation_id', '3': 2, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'purpose',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.SpaceAccountCapturePurpose',
      '10': 'purpose'
    },
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'cohort_digest', '3': 5, '4': 1, '5': 12, '10': 'cohortDigest'},
    {'1': 'request_digest', '3': 6, '4': 1, '5': 12, '10': 'requestDigest'},
    {
      '1': 'owner_source_revision',
      '3': 7,
      '4': 1,
      '5': 4,
      '10': 'ownerSourceRevision'
    },
    {
      '1': 'results',
      '3': 8,
      '4': 3,
      '5': 11,
      '6': '.voice.user.v1.SpaceAccountCaptureResult',
      '10': 'results'
    },
    {
      '1': 'user_source_generation',
      '3': 9,
      '4': 1,
      '5': 9,
      '10': 'userSourceGeneration'
    },
    {
      '1': 'identity_cut',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.SpaceAccountIdentityCut',
      '10': 'identityCut'
    },
  ],
};

/// Descriptor for `CaptureSpaceMemberAccountBindingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List captureSpaceMemberAccountBindingsResponseDescriptor = $convert.base64Decode(
    'CilDYXB0dXJlU3BhY2VNZW1iZXJBY2NvdW50QmluZGluZ3NSZXNwb25zZRIaCghwcm90b2NvbB'
    'gBIAEoDVIIcHJvdG9jb2wSIQoMb3BlcmF0aW9uX2lkGAIgASgJUgtvcGVyYXRpb25JZBJDCgdw'
    'dXJwb3NlGAMgASgOMikudm9pY2UudXNlci52MS5TcGFjZUFjY291bnRDYXB0dXJlUHVycG9zZV'
    'IHcHVycG9zZRIZCghzcGFjZV9pZBgEIAEoCVIHc3BhY2VJZBIjCg1jb2hvcnRfZGlnZXN0GAUg'
    'ASgMUgxjb2hvcnREaWdlc3QSJQoOcmVxdWVzdF9kaWdlc3QYBiABKAxSDXJlcXVlc3REaWdlc3'
    'QSMgoVb3duZXJfc291cmNlX3JldmlzaW9uGAcgASgEUhNvd25lclNvdXJjZVJldmlzaW9uEkIK'
    'B3Jlc3VsdHMYCCADKAsyKC52b2ljZS51c2VyLnYxLlNwYWNlQWNjb3VudENhcHR1cmVSZXN1bH'
    'RSB3Jlc3VsdHMSNAoWdXNlcl9zb3VyY2VfZ2VuZXJhdGlvbhgJIAEoCVIUdXNlclNvdXJjZUdl'
    'bmVyYXRpb24SSQoMaWRlbnRpdHlfY3V0GAogASgLMiYudm9pY2UudXNlci52MS5TcGFjZUFjY2'
    '91bnRJZGVudGl0eUN1dFILaWRlbnRpdHlDdXQ=');
