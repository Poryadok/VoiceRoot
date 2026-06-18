// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user.proto.

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

@$core.Deprecated('Use presenceOnlineStatusDescriptor instead')
const PresenceOnlineStatus$json = {
  '1': 'PresenceOnlineStatus',
  '2': [
    {'1': 'PRESENCE_ONLINE_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'PRESENCE_ONLINE_STATUS_ONLINE', '2': 1},
    {'1': 'PRESENCE_ONLINE_STATUS_IDLE', '2': 2},
    {'1': 'PRESENCE_ONLINE_STATUS_DND', '2': 3},
    {'1': 'PRESENCE_ONLINE_STATUS_INVISIBLE', '2': 4},
  ],
};

/// Descriptor for `PresenceOnlineStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List presenceOnlineStatusDescriptor = $convert.base64Decode(
    'ChRQcmVzZW5jZU9ubGluZVN0YXR1cxImCiJQUkVTRU5DRV9PTkxJTkVfU1RBVFVTX1VOU1BFQ0'
    'lGSUVEEAASIQodUFJFU0VOQ0VfT05MSU5FX1NUQVRVU19PTkxJTkUQARIfChtQUkVTRU5DRV9P'
    'TkxJTkVfU1RBVFVTX0lETEUQAhIeChpQUkVTRU5DRV9PTkxJTkVfU1RBVFVTX0RORBADEiQKIF'
    'BSRVNFTkNFX09OTElORV9TVEFUVVNfSU5WSVNJQkxFEAQ=');

@$core.Deprecated('Use privacyPresetDescriptor instead')
const PrivacyPreset$json = {
  '1': 'PrivacyPreset',
  '2': [
    {'1': 'PRIVACY_PRESET_UNSPECIFIED', '2': 0},
    {'1': 'PRIVACY_PRESET_PERSONAL', '2': 1},
    {'1': 'PRIVACY_PRESET_GAMING', '2': 2},
    {'1': 'PRIVACY_PRESET_WORK', '2': 3},
  ],
};

/// Descriptor for `PrivacyPreset`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List privacyPresetDescriptor = $convert.base64Decode(
    'Cg1Qcml2YWN5UHJlc2V0Eh4KGlBSSVZBQ1lfUFJFU0VUX1VOU1BFQ0lGSUVEEAASGwoXUFJJVk'
    'FDWV9QUkVTRVRfUEVSU09OQUwQARIZChVQUklWQUNZX1BSRVNFVF9HQU1JTkcQAhIXChNQUklW'
    'QUNZX1BSRVNFVF9XT1JLEAM=');

@$core.Deprecated('Use ensurePrimaryProfileRequestDescriptor instead')
const EnsurePrimaryProfileRequest$json = {
  '1': 'EnsurePrimaryProfileRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'profileId',
      '17': true
    },
    {'1': 'display_hint', '3': 3, '4': 1, '5': 9, '10': 'displayHint'},
  ],
  '8': [
    {'1': '_profile_id'},
  ],
};

/// Descriptor for `EnsurePrimaryProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ensurePrimaryProfileRequestDescriptor =
    $convert.base64Decode(
        'ChtFbnN1cmVQcmltYXJ5UHJvZmlsZVJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3'
        'VudElkEiIKCnByb2ZpbGVfaWQYAiABKAlIAFIJcHJvZmlsZUlkiAEBEiEKDGRpc3BsYXlfaGlu'
        'dBgDIAEoCVILZGlzcGxheUhpbnRCDQoLX3Byb2ZpbGVfaWQ=');

@$core.Deprecated('Use ensurePrimaryProfileResponseDescriptor instead')
const EnsurePrimaryProfileResponse$json = {
  '1': 'EnsurePrimaryProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `EnsurePrimaryProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ensurePrimaryProfileResponseDescriptor =
    $convert.base64Decode(
        'ChxFbnN1cmVQcmltYXJ5UHJvZmlsZVJlc3BvbnNlEjAKB3Byb2ZpbGUYASABKAsyFi52b2ljZS'
        '51c2VyLnYxLlByb2ZpbGVSB3Byb2ZpbGU=');

@$core.Deprecated('Use getProfileRequestDescriptor instead')
const GetProfileRequest$json = {
  '1': 'GetProfileRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'profileId'},
    {'1': 'username', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'username'},
  ],
  '8': [
    {'1': 'by'},
  ],
};

/// Descriptor for `GetProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileRequestDescriptor = $convert.base64Decode(
    'ChFHZXRQcm9maWxlUmVxdWVzdBIfCgpwcm9maWxlX2lkGAEgASgJSABSCXByb2ZpbGVJZBIcCg'
    'h1c2VybmFtZRgCIAEoCUgAUgh1c2VybmFtZUIECgJieQ==');

@$core.Deprecated('Use getProfilesRequestDescriptor instead')
const GetProfilesRequest$json = {
  '1': 'GetProfilesRequest',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `GetProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfilesRequestDescriptor = $convert.base64Decode(
    'ChJHZXRQcm9maWxlc1JlcXVlc3QSHwoLcHJvZmlsZV9pZHMYASADKAlSCnByb2ZpbGVJZHM=');

@$core.Deprecated('Use profileListDescriptor instead')
const ProfileList$json = {
  '1': 'ProfileList',
  '2': [
    {
      '1': 'profiles',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profiles'
    },
  ],
};

/// Descriptor for `ProfileList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileListDescriptor = $convert.base64Decode(
    'CgtQcm9maWxlTGlzdBIyCghwcm9maWxlcxgBIAMoCzIWLnZvaWNlLnVzZXIudjEuUHJvZmlsZV'
    'IIcHJvZmlsZXM=');

@$core.Deprecated('Use profileDescriptor instead')
const Profile$json = {
  '1': 'Profile',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'username', '3': 3, '4': 1, '5': 9, '10': 'username'},
    {'1': 'discriminator', '3': 4, '4': 1, '5': 9, '10': 'discriminator'},
    {'1': 'display_name', '3': 5, '4': 1, '5': 9, '10': 'displayName'},
    {
      '1': 'avatar_url',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'avatarUrl',
      '17': true
    },
    {
      '1': 'banner_url',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'bannerUrl',
      '17': true
    },
    {'1': 'bio', '3': 8, '4': 1, '5': 9, '9': 2, '10': 'bio', '17': true},
    {
      '1': 'custom_status',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'customStatus',
      '17': true
    },
    {'1': 'locale', '3': 10, '4': 1, '5': 9, '10': 'locale'},
    {'1': 'theme', '3': 11, '4': 1, '5': 9, '10': 'theme'},
    {'1': 'is_primary', '3': 12, '4': 1, '5': 8, '10': 'isPrimary'},
    {
      '1': 'verification_type',
      '3': 13,
      '4': 1,
      '5': 9,
      '10': 'verificationType'
    },
    {
      '1': 'verification_badge',
      '3': 14,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'verificationBadge',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'updated_at',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
    {
      '1': 'frozen_at',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 5,
      '10': 'frozenAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_avatar_url'},
    {'1': '_banner_url'},
    {'1': '_bio'},
    {'1': '_custom_status'},
    {'1': '_verification_badge'},
    {'1': '_frozen_at'},
  ],
};

/// Descriptor for `Profile`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileDescriptor = $convert.base64Decode(
    'CgdQcm9maWxlEg4KAmlkGAEgASgJUgJpZBIdCgphY2NvdW50X2lkGAIgASgJUglhY2NvdW50SW'
    'QSGgoIdXNlcm5hbWUYAyABKAlSCHVzZXJuYW1lEiQKDWRpc2NyaW1pbmF0b3IYBCABKAlSDWRp'
    'c2NyaW1pbmF0b3ISIQoMZGlzcGxheV9uYW1lGAUgASgJUgtkaXNwbGF5TmFtZRIiCgphdmF0YX'
    'JfdXJsGAYgASgJSABSCWF2YXRhclVybIgBARIiCgpiYW5uZXJfdXJsGAcgASgJSAFSCWJhbm5l'
    'clVybIgBARIVCgNiaW8YCCABKAlIAlIDYmlviAEBEigKDWN1c3RvbV9zdGF0dXMYCSABKAlIA1'
    'IMY3VzdG9tU3RhdHVziAEBEhYKBmxvY2FsZRgKIAEoCVIGbG9jYWxlEhQKBXRoZW1lGAsgASgJ'
    'UgV0aGVtZRIdCgppc19wcmltYXJ5GAwgASgIUglpc1ByaW1hcnkSKwoRdmVyaWZpY2F0aW9uX3'
    'R5cGUYDSABKAlSEHZlcmlmaWNhdGlvblR5cGUSMgoSdmVyaWZpY2F0aW9uX2JhZGdlGA4gASgJ'
    'SARSEXZlcmlmaWNhdGlvbkJhZGdliAEBEjkKCmNyZWF0ZWRfYXQYDyABKAsyGi5nb29nbGUucH'
    'JvdG9idWYuVGltZXN0YW1wUgljcmVhdGVkQXQSOQoKdXBkYXRlZF9hdBgQIAEoCzIaLmdvb2ds'
    'ZS5wcm90b2J1Zi5UaW1lc3RhbXBSCXVwZGF0ZWRBdBI8Cglmcm96ZW5fYXQYESABKAsyGi5nb2'
    '9nbGUucHJvdG9idWYuVGltZXN0YW1wSAVSCGZyb3plbkF0iAEBQg0KC19hdmF0YXJfdXJsQg0K'
    'C19iYW5uZXJfdXJsQgYKBF9iaW9CEAoOX2N1c3RvbV9zdGF0dXNCFQoTX3ZlcmlmaWNhdGlvbl'
    '9iYWRnZUIMCgpfZnJvemVuX2F0');

@$core.Deprecated('Use updateProfileRequestDescriptor instead')
const UpdateProfileRequest$json = {
  '1': 'UpdateProfileRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'display_name',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'displayName',
      '17': true
    },
    {
      '1': 'avatar_url',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'avatarUrl',
      '17': true
    },
    {
      '1': 'banner_url',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'bannerUrl',
      '17': true
    },
    {'1': 'bio', '3': 5, '4': 1, '5': 9, '9': 3, '10': 'bio', '17': true},
    {
      '1': 'custom_status',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'customStatus',
      '17': true
    },
    {'1': 'locale', '3': 7, '4': 1, '5': 9, '9': 5, '10': 'locale', '17': true},
    {'1': 'theme', '3': 8, '4': 1, '5': 9, '9': 6, '10': 'theme', '17': true},
  ],
  '8': [
    {'1': '_display_name'},
    {'1': '_avatar_url'},
    {'1': '_banner_url'},
    {'1': '_bio'},
    {'1': '_custom_status'},
    {'1': '_locale'},
    {'1': '_theme'},
  ],
};

/// Descriptor for `UpdateProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProfileRequestDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVQcm9maWxlUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSJg'
    'oMZGlzcGxheV9uYW1lGAIgASgJSABSC2Rpc3BsYXlOYW1liAEBEiIKCmF2YXRhcl91cmwYAyAB'
    'KAlIAVIJYXZhdGFyVXJsiAEBEiIKCmJhbm5lcl91cmwYBCABKAlIAlIJYmFubmVyVXJsiAEBEh'
    'UKA2JpbxgFIAEoCUgDUgNiaW+IAQESKAoNY3VzdG9tX3N0YXR1cxgGIAEoCUgEUgxjdXN0b21T'
    'dGF0dXOIAQESGwoGbG9jYWxlGAcgASgJSAVSBmxvY2FsZYgBARIZCgV0aGVtZRgIIAEoCUgGUg'
    'V0aGVtZYgBAUIPCg1fZGlzcGxheV9uYW1lQg0KC19hdmF0YXJfdXJsQg0KC19iYW5uZXJfdXJs'
    'QgYKBF9iaW9CEAoOX2N1c3RvbV9zdGF0dXNCCQoHX2xvY2FsZUIICgZfdGhlbWU=');

@$core.Deprecated('Use createProfileRequestDescriptor instead')
const CreateProfileRequest$json = {
  '1': 'CreateProfileRequest',
  '2': [
    {'1': 'display_name', '3': 1, '4': 1, '5': 9, '10': 'displayName'},
    {
      '1': 'username',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'username',
      '17': true
    },
  ],
  '8': [
    {'1': '_username'},
  ],
};

/// Descriptor for `CreateProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createProfileRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVQcm9maWxlUmVxdWVzdBIhCgxkaXNwbGF5X25hbWUYASABKAlSC2Rpc3BsYXlOYW'
    '1lEh8KCHVzZXJuYW1lGAIgASgJSABSCHVzZXJuYW1liAEBQgsKCV91c2VybmFtZQ==');

@$core.Deprecated('Use deleteProfileRequestDescriptor instead')
const DeleteProfileRequest$json = {
  '1': 'DeleteProfileRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `DeleteProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteProfileRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVQcm9maWxlUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use switchProfileRequestDescriptor instead')
const SwitchProfileRequest$json = {
  '1': 'SwitchProfileRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `SwitchProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List switchProfileRequestDescriptor = $convert.base64Decode(
    'ChRTd2l0Y2hQcm9maWxlUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use listMyProfilesRequestDescriptor instead')
const ListMyProfilesRequest$json = {
  '1': 'ListMyProfilesRequest',
};

/// Descriptor for `ListMyProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyProfilesRequestDescriptor =
    $convert.base64Decode('ChVMaXN0TXlQcm9maWxlc1JlcXVlc3Q=');

@$core.Deprecated('Use searchProfilesRequestDescriptor instead')
const SearchProfilesRequest$json = {
  '1': 'SearchProfilesRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
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

/// Descriptor for `SearchProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchProfilesRequestDescriptor = $convert.base64Decode(
    'ChVTZWFyY2hQcm9maWxlc1JlcXVlc3QSFAoFcXVlcnkYASABKAlSBXF1ZXJ5EjYKBHBhZ2UYAi'
    'ABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use searchProfilesResponseDescriptor instead')
const SearchProfilesResponse$json = {
  '1': 'SearchProfilesResponse',
  '2': [
    {
      '1': 'profile_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.ProfileList',
      '10': 'profileList'
    },
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageResponse',
      '10': 'page'
    },
  ],
};

/// Descriptor for `SearchProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchProfilesResponseDescriptor = $convert.base64Decode(
    'ChZTZWFyY2hQcm9maWxlc1Jlc3BvbnNlEj0KDHByb2ZpbGVfbGlzdBgBIAEoCzIaLnZvaWNlLn'
    'VzZXIudjEuUHJvZmlsZUxpc3RSC3Byb2ZpbGVMaXN0EjcKBHBhZ2UYAiABKAsyIy52b2ljZS5j'
    'b21tb24udjEuQ3Vyc29yUGFnZVJlc3BvbnNlUgRwYWdl');

@$core.Deprecated('Use getPrivacySettingsRequestDescriptor instead')
const GetPrivacySettingsRequest$json = {
  '1': 'GetPrivacySettingsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetPrivacySettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPrivacySettingsRequestDescriptor =
    $convert.base64Decode(
        'ChlHZXRQcml2YWN5U2V0dGluZ3NSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbG'
        'VJZA==');

@$core.Deprecated('Use updatePrivacySettingsRequestDescriptor instead')
const UpdatePrivacySettingsRequest$json = {
  '1': 'UpdatePrivacySettingsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'settings',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacySettings',
      '10': 'settings'
    },
  ],
};

/// Descriptor for `UpdatePrivacySettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePrivacySettingsRequestDescriptor =
    $convert.base64Decode(
        'ChxVcGRhdGVQcml2YWN5U2V0dGluZ3NSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2'
        'ZpbGVJZBI6CghzZXR0aW5ncxgCIAEoCzIeLnZvaWNlLnVzZXIudjEuUHJpdmFjeVNldHRpbmdz'
        'UghzZXR0aW5ncw==');

@$core.Deprecated('Use privacyAudienceDescriptor instead')
const PrivacyAudience$json = {
  '1': 'PrivacyAudience',
  '2': [
    {'1': 'friends', '3': 1, '4': 1, '5': 8, '10': 'friends'},
    {
      '1': 'friends_of_friends',
      '3': 2,
      '4': 1,
      '5': 8,
      '10': 'friendsOfFriends'
    },
    {'1': 'space_members', '3': 3, '4': 1, '5': 8, '10': 'spaceMembers'},
    {'1': 'space_ids', '3': 4, '4': 3, '5': 9, '10': 'spaceIds'},
    {'1': 'include_guests', '3': 5, '4': 1, '5': 8, '10': 'includeGuests'},
  ],
};

/// Descriptor for `PrivacyAudience`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List privacyAudienceDescriptor = $convert.base64Decode(
    'Cg9Qcml2YWN5QXVkaWVuY2USGAoHZnJpZW5kcxgBIAEoCFIHZnJpZW5kcxIsChJmcmllbmRzX2'
    '9mX2ZyaWVuZHMYAiABKAhSEGZyaWVuZHNPZkZyaWVuZHMSIwoNc3BhY2VfbWVtYmVycxgDIAEo'
    'CFIMc3BhY2VNZW1iZXJzEhsKCXNwYWNlX2lkcxgEIAMoCVIIc3BhY2VJZHMSJQoOaW5jbHVkZV'
    '9ndWVzdHMYBSABKAhSDWluY2x1ZGVHdWVzdHM=');

@$core.Deprecated('Use privacySettingsDescriptor instead')
const PrivacySettings$json = {
  '1': 'PrivacySettings',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'preset', '3': 2, '4': 1, '5': 9, '10': 'preset'},
    {
      '1': 'show_online',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'showOnline'
    },
    {
      '1': 'show_game_status',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'showGameStatus'
    },
    {
      '1': 'show_mm_rating',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'showMmRating'
    },
    {
      '1': 'show_phone',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'showPhone'
    },
    {
      '1': 'show_stories',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'showStories'
    },
    {
      '1': 'allow_dm',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowDm'
    },
    {
      '1': 'allow_friend_requests',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowFriendRequests'
    },
    {'1': 'allow_guest_dm', '3': 10, '4': 1, '5': 8, '10': 'allowGuestDm'},
    {
      '1': 'updated_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
    {
      '1': 'preset_enum',
      '3': 12,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.PrivacyPreset',
      '9': 0,
      '10': 'presetEnum',
      '17': true
    },
    {
      '1': 'allow_phone_search',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowPhoneSearch'
    },
    {
      '1': 'allow_calls',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowCalls'
    },
    {
      '1': 'allow_chat_space_invites',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowChatSpaceInvites'
    },
    {
      '1': 'allow_files',
      '3': 17,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowFiles'
    },
    {
      '1': 'allow_voice_messages',
      '3': 18,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacyAudience',
      '10': 'allowVoiceMessages'
    },
  ],
  '8': [
    {'1': '_preset_enum'},
  ],
};

/// Descriptor for `PrivacySettings`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List privacySettingsDescriptor = $convert.base64Decode(
    'Cg9Qcml2YWN5U2V0dGluZ3MSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEhYKBnByZX'
    'NldBgCIAEoCVIGcHJlc2V0Ej8KC3Nob3dfb25saW5lGAMgASgLMh4udm9pY2UudXNlci52MS5Q'
    'cml2YWN5QXVkaWVuY2VSCnNob3dPbmxpbmUSSAoQc2hvd19nYW1lX3N0YXR1cxgEIAEoCzIeLn'
    'ZvaWNlLnVzZXIudjEuUHJpdmFjeUF1ZGllbmNlUg5zaG93R2FtZVN0YXR1cxJECg5zaG93X21t'
    'X3JhdGluZxgFIAEoCzIeLnZvaWNlLnVzZXIudjEuUHJpdmFjeUF1ZGllbmNlUgxzaG93TW1SYX'
    'RpbmcSPQoKc2hvd19waG9uZRgGIAEoCzIeLnZvaWNlLnVzZXIudjEuUHJpdmFjeUF1ZGllbmNl'
    'UglzaG93UGhvbmUSQQoMc2hvd19zdG9yaWVzGAcgASgLMh4udm9pY2UudXNlci52MS5Qcml2YW'
    'N5QXVkaWVuY2VSC3Nob3dTdG9yaWVzEjkKCGFsbG93X2RtGAggASgLMh4udm9pY2UudXNlci52'
    'MS5Qcml2YWN5QXVkaWVuY2VSB2FsbG93RG0SUgoVYWxsb3dfZnJpZW5kX3JlcXVlc3RzGAkgAS'
    'gLMh4udm9pY2UudXNlci52MS5Qcml2YWN5QXVkaWVuY2VSE2FsbG93RnJpZW5kUmVxdWVzdHMS'
    'JAoOYWxsb3dfZ3Vlc3RfZG0YCiABKAhSDGFsbG93R3Vlc3REbRI5Cgp1cGRhdGVkX2F0GAsgAS'
    'gLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJdXBkYXRlZEF0EkIKC3ByZXNldF9lbnVt'
    'GAwgASgOMhwudm9pY2UudXNlci52MS5Qcml2YWN5UHJlc2V0SABSCnByZXNldEVudW2IAQESTA'
    'oSYWxsb3dfcGhvbmVfc2VhcmNoGA4gASgLMh4udm9pY2UudXNlci52MS5Qcml2YWN5QXVkaWVu'
    'Y2VSEGFsbG93UGhvbmVTZWFyY2gSPwoLYWxsb3dfY2FsbHMYDyABKAsyHi52b2ljZS51c2VyLn'
    'YxLlByaXZhY3lBdWRpZW5jZVIKYWxsb3dDYWxscxJXChhhbGxvd19jaGF0X3NwYWNlX2ludml0'
    'ZXMYECABKAsyHi52b2ljZS51c2VyLnYxLlByaXZhY3lBdWRpZW5jZVIVYWxsb3dDaGF0U3BhY2'
    'VJbnZpdGVzEj8KC2FsbG93X2ZpbGVzGBEgASgLMh4udm9pY2UudXNlci52MS5Qcml2YWN5QXVk'
    'aWVuY2VSCmFsbG93RmlsZXMSUAoUYWxsb3dfdm9pY2VfbWVzc2FnZXMYEiABKAsyHi52b2ljZS'
    '51c2VyLnYxLlByaXZhY3lBdWRpZW5jZVISYWxsb3dWb2ljZU1lc3NhZ2VzQg4KDF9wcmVzZXRf'
    'ZW51bQ==');

@$core.Deprecated('Use updatePresenceRequestDescriptor instead')
const UpdatePresenceRequest$json = {
  '1': 'UpdatePresenceRequest',
  '2': [
    {'1': 'status', '3': 1, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'game_title',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'gameTitle',
      '17': true
    },
    {
      '1': 'custom_status',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'customStatus',
      '17': true
    },
    {
      '1': 'call_info_json',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'callInfoJson',
      '17': true
    },
    {
      '1': 'status_enum',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.PresenceOnlineStatus',
      '9': 3,
      '10': 'statusEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_game_title'},
    {'1': '_custom_status'},
    {'1': '_call_info_json'},
    {'1': '_status_enum'},
  ],
};

/// Descriptor for `UpdatePresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePresenceRequestDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVQcmVzZW5jZVJlcXVlc3QSFgoGc3RhdHVzGAEgASgJUgZzdGF0dXMSIgoKZ2FtZV'
    '90aXRsZRgCIAEoCUgAUglnYW1lVGl0bGWIAQESKAoNY3VzdG9tX3N0YXR1cxgDIAEoCUgBUgxj'
    'dXN0b21TdGF0dXOIAQESKQoOY2FsbF9pbmZvX2pzb24YBCABKAlIAlIMY2FsbEluZm9Kc29uiA'
    'EBEkkKC3N0YXR1c19lbnVtGAUgASgOMiMudm9pY2UudXNlci52MS5QcmVzZW5jZU9ubGluZVN0'
    'YXR1c0gDUgpzdGF0dXNFbnVtiAEBQg0KC19nYW1lX3RpdGxlQhAKDl9jdXN0b21fc3RhdHVzQh'
    'EKD19jYWxsX2luZm9fanNvbkIOCgxfc3RhdHVzX2VudW0=');

@$core.Deprecated('Use getPresenceRequestDescriptor instead')
const GetPresenceRequest$json = {
  '1': 'GetPresenceRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetPresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPresenceRequestDescriptor =
    $convert.base64Decode(
        'ChJHZXRQcmVzZW5jZVJlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use presenceStatusDescriptor instead')
const PresenceStatus$json = {
  '1': 'PresenceStatus',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'game_title',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'gameTitle',
      '17': true
    },
    {
      '1': 'custom_status',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'customStatus',
      '17': true
    },
    {
      '1': 'last_seen',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'lastSeen'
    },
    {
      '1': 'call_info_json',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'callInfoJson',
      '17': true
    },
    {
      '1': 'status_enum',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.user.v1.PresenceOnlineStatus',
      '9': 3,
      '10': 'statusEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_game_title'},
    {'1': '_custom_status'},
    {'1': '_call_info_json'},
    {'1': '_status_enum'},
  ],
};

/// Descriptor for `PresenceStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List presenceStatusDescriptor = $convert.base64Decode(
    'Cg5QcmVzZW5jZVN0YXR1cxIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSFgoGc3RhdH'
    'VzGAIgASgJUgZzdGF0dXMSIgoKZ2FtZV90aXRsZRgDIAEoCUgAUglnYW1lVGl0bGWIAQESKAoN'
    'Y3VzdG9tX3N0YXR1cxgEIAEoCUgBUgxjdXN0b21TdGF0dXOIAQESNwoJbGFzdF9zZWVuGAUgAS'
    'gLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIIbGFzdFNlZW4SKQoOY2FsbF9pbmZvX2pz'
    'b24YBiABKAlIAlIMY2FsbEluZm9Kc29uiAEBEkkKC3N0YXR1c19lbnVtGAcgASgOMiMudm9pY2'
    'UudXNlci52MS5QcmVzZW5jZU9ubGluZVN0YXR1c0gDUgpzdGF0dXNFbnVtiAEBQg0KC19nYW1l'
    'X3RpdGxlQhAKDl9jdXN0b21fc3RhdHVzQhEKD19jYWxsX2luZm9fanNvbkIOCgxfc3RhdHVzX2'
    'VudW0=');

@$core.Deprecated('Use getBulkPresenceRequestDescriptor instead')
const GetBulkPresenceRequest$json = {
  '1': 'GetBulkPresenceRequest',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `GetBulkPresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkPresenceRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRCdWxrUHJlc2VuY2VSZXF1ZXN0Eh8KC3Byb2ZpbGVfaWRzGAEgAygJUgpwcm9maWxlSW'
        'Rz');

@$core.Deprecated('Use getSettingsRequestDescriptor instead')
const GetSettingsRequest$json = {
  '1': 'GetSettingsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSettingsRequestDescriptor =
    $convert.base64Decode(
        'ChJHZXRTZXR0aW5nc1JlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use updateSettingsRequestDescriptor instead')
const UpdateSettingsRequest$json = {
  '1': 'UpdateSettingsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'settings',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSettings',
      '10': 'settings'
    },
  ],
};

/// Descriptor for `UpdateSettingsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateSettingsRequestDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVTZXR0aW5nc1JlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEj'
    'cKCHNldHRpbmdzGAIgASgLMhsudm9pY2UudXNlci52MS5Vc2VyU2V0dGluZ3NSCHNldHRpbmdz');

@$core.Deprecated('Use userSettingsDescriptor instead')
const UserSettings$json = {
  '1': 'UserSettings',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'language', '3': 2, '4': 1, '5': 9, '10': 'language'},
    {'1': 'theme', '3': 3, '4': 1, '5': 9, '10': 'theme'},
    {
      '1': 'notification_prefs_json',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'notificationPrefsJson'
    },
  ],
};

/// Descriptor for `UserSettings`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userSettingsDescriptor = $convert.base64Decode(
    'CgxVc2VyU2V0dGluZ3MSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEhoKCGxhbmd1YW'
    'dlGAIgASgJUghsYW5ndWFnZRIUCgV0aGVtZRgDIAEoCVIFdGhlbWUSNgoXbm90aWZpY2F0aW9u'
    'X3ByZWZzX2pzb24YBCABKAlSFW5vdGlmaWNhdGlvblByZWZzSnNvbg==');

@$core.Deprecated('Use getOnboardingStateRequestDescriptor instead')
const GetOnboardingStateRequest$json = {
  '1': 'GetOnboardingStateRequest',
};

/// Descriptor for `GetOnboardingStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOnboardingStateRequestDescriptor =
    $convert.base64Decode('ChlHZXRPbmJvYXJkaW5nU3RhdGVSZXF1ZXN0');

@$core.Deprecated('Use completeOnboardingStepRequestDescriptor instead')
const CompleteOnboardingStepRequest$json = {
  '1': 'CompleteOnboardingStepRequest',
  '2': [
    {'1': 'step_id', '3': 1, '4': 1, '5': 9, '10': 'stepId'},
  ],
};

/// Descriptor for `CompleteOnboardingStepRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeOnboardingStepRequestDescriptor =
    $convert.base64Decode(
        'Ch1Db21wbGV0ZU9uYm9hcmRpbmdTdGVwUmVxdWVzdBIXCgdzdGVwX2lkGAEgASgJUgZzdGVwSW'
        'Q=');

@$core.Deprecated('Use onboardingStateDescriptor instead')
const OnboardingState$json = {
  '1': 'OnboardingState',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'completed_steps', '3': 2, '4': 3, '5': 9, '10': 'completedSteps'},
    {'1': 'completed', '3': 3, '4': 1, '5': 8, '10': 'completed'},
    {
      '1': 'completed_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'completedAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_completed_at'},
  ],
};

/// Descriptor for `OnboardingState`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List onboardingStateDescriptor = $convert.base64Decode(
    'Cg9PbmJvYXJkaW5nU3RhdGUSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEicKD2NvbX'
    'BsZXRlZF9zdGVwcxgCIAMoCVIOY29tcGxldGVkU3RlcHMSHAoJY29tcGxldGVkGAMgASgIUglj'
    'b21wbGV0ZWQSQgoMY29tcGxldGVkX2F0GAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdG'
    'FtcEgAUgtjb21wbGV0ZWRBdIgBAUIPCg1fY29tcGxldGVkX2F0');

@$core.Deprecated('Use createAvatarPresignedUploadRequestDescriptor instead')
const CreateAvatarPresignedUploadRequest$json = {
  '1': 'CreateAvatarPresignedUploadRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'content_type', '3': 2, '4': 1, '5': 9, '10': 'contentType'},
    {'1': 'content_length', '3': 3, '4': 1, '5': 3, '10': 'contentLength'},
  ],
};

/// Descriptor for `CreateAvatarPresignedUploadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createAvatarPresignedUploadRequestDescriptor =
    $convert.base64Decode(
        'CiJDcmVhdGVBdmF0YXJQcmVzaWduZWRVcGxvYWRSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKA'
        'lSCXByb2ZpbGVJZBIhCgxjb250ZW50X3R5cGUYAiABKAlSC2NvbnRlbnRUeXBlEiUKDmNvbnRl'
        'bnRfbGVuZ3RoGAMgASgDUg1jb250ZW50TGVuZ3Ro');

@$core.Deprecated('Use createAvatarPresignedUploadResponseDescriptor instead')
const CreateAvatarPresignedUploadResponse$json = {
  '1': 'CreateAvatarPresignedUploadResponse',
  '2': [
    {'1': 'http_method', '3': 1, '4': 1, '5': 9, '10': 'httpMethod'},
    {'1': 'upload_url', '3': 2, '4': 1, '5': 9, '10': 'uploadUrl'},
    {
      '1': 'required_headers',
      '3': 3,
      '4': 3,
      '5': 11,
      '6':
          '.voice.user.v1.CreateAvatarPresignedUploadResponse.RequiredHeadersEntry',
      '10': 'requiredHeaders'
    },
    {'1': 'max_bytes', '3': 4, '4': 1, '5': 3, '10': 'maxBytes'},
    {
      '1': 'expires_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {'1': 'public_url', '3': 6, '4': 1, '5': 9, '10': 'publicUrl'},
    {'1': 'object_key', '3': 7, '4': 1, '5': 9, '10': 'objectKey'},
  ],
  '3': [CreateAvatarPresignedUploadResponse_RequiredHeadersEntry$json],
};

@$core.Deprecated('Use createAvatarPresignedUploadResponseDescriptor instead')
const CreateAvatarPresignedUploadResponse_RequiredHeadersEntry$json = {
  '1': 'RequiredHeadersEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `CreateAvatarPresignedUploadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createAvatarPresignedUploadResponseDescriptor = $convert.base64Decode(
    'CiNDcmVhdGVBdmF0YXJQcmVzaWduZWRVcGxvYWRSZXNwb25zZRIfCgtodHRwX21ldGhvZBgBIA'
    'EoCVIKaHR0cE1ldGhvZBIdCgp1cGxvYWRfdXJsGAIgASgJUgl1cGxvYWRVcmwScgoQcmVxdWly'
    'ZWRfaGVhZGVycxgDIAMoCzJHLnZvaWNlLnVzZXIudjEuQ3JlYXRlQXZhdGFyUHJlc2lnbmVkVX'
    'Bsb2FkUmVzcG9uc2UuUmVxdWlyZWRIZWFkZXJzRW50cnlSD3JlcXVpcmVkSGVhZGVycxIbCglt'
    'YXhfYnl0ZXMYBCABKANSCG1heEJ5dGVzEjkKCmV4cGlyZXNfYXQYBSABKAsyGi5nb29nbGUucH'
    'JvdG9idWYuVGltZXN0YW1wUglleHBpcmVzQXQSHQoKcHVibGljX3VybBgGIAEoCVIJcHVibGlj'
    'VXJsEh0KCm9iamVjdF9rZXkYByABKAlSCW9iamVjdEtleRpCChRSZXF1aXJlZEhlYWRlcnNFbn'
    'RyeRIQCgNrZXkYASABKAlSA2tleRIUCgV2YWx1ZRgCIAEoCVIFdmFsdWU6AjgB');

@$core.Deprecated('Use getVerificationStatusRequestDescriptor instead')
const GetVerificationStatusRequest$json = {
  '1': 'GetVerificationStatusRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetVerificationStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVerificationStatusRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRWZXJpZmljYXRpb25TdGF0dXNSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2'
        'ZpbGVJZA==');

@$core.Deprecated('Use verificationStatusDescriptor instead')
const VerificationStatus$json = {
  '1': 'VerificationStatus',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'verification_type',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'verificationType'
    },
    {'1': 'badge', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'badge', '17': true},
  ],
  '8': [
    {'1': '_badge'},
  ],
};

/// Descriptor for `VerificationStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verificationStatusDescriptor = $convert.base64Decode(
    'ChJWZXJpZmljYXRpb25TdGF0dXMSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEisKEX'
    'ZlcmlmaWNhdGlvbl90eXBlGAIgASgJUhB2ZXJpZmljYXRpb25UeXBlEhkKBWJhZGdlGAMgASgJ'
    'SABSBWJhZGdliAEBQggKBl9iYWRnZQ==');

@$core.Deprecated('Use getProfileResponseDescriptor instead')
const GetProfileResponse$json = {
  '1': 'GetProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `GetProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfileResponseDescriptor = $convert.base64Decode(
    'ChJHZXRQcm9maWxlUmVzcG9uc2USMAoHcHJvZmlsZRgBIAEoCzIWLnZvaWNlLnVzZXIudjEuUH'
    'JvZmlsZVIHcHJvZmlsZQ==');

@$core.Deprecated('Use getProfilesResponseDescriptor instead')
const GetProfilesResponse$json = {
  '1': 'GetProfilesResponse',
  '2': [
    {
      '1': 'profile_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.ProfileList',
      '10': 'profileList'
    },
  ],
};

/// Descriptor for `GetProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getProfilesResponseDescriptor = $convert.base64Decode(
    'ChNHZXRQcm9maWxlc1Jlc3BvbnNlEj0KDHByb2ZpbGVfbGlzdBgBIAEoCzIaLnZvaWNlLnVzZX'
    'IudjEuUHJvZmlsZUxpc3RSC3Byb2ZpbGVMaXN0');

@$core.Deprecated('Use updateProfileResponseDescriptor instead')
const UpdateProfileResponse$json = {
  '1': 'UpdateProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `UpdateProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateProfileResponseDescriptor = $convert.base64Decode(
    'ChVVcGRhdGVQcm9maWxlUmVzcG9uc2USMAoHcHJvZmlsZRgBIAEoCzIWLnZvaWNlLnVzZXIudj'
    'EuUHJvZmlsZVIHcHJvZmlsZQ==');

@$core.Deprecated('Use createProfileResponseDescriptor instead')
const CreateProfileResponse$json = {
  '1': 'CreateProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `CreateProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createProfileResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVQcm9maWxlUmVzcG9uc2USMAoHcHJvZmlsZRgBIAEoCzIWLnZvaWNlLnVzZXIudj'
    'EuUHJvZmlsZVIHcHJvZmlsZQ==');

@$core.Deprecated('Use deleteProfileResponseDescriptor instead')
const DeleteProfileResponse$json = {
  '1': 'DeleteProfileResponse',
};

/// Descriptor for `DeleteProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteProfileResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVQcm9maWxlUmVzcG9uc2U=');

@$core.Deprecated('Use switchProfileResponseDescriptor instead')
const SwitchProfileResponse$json = {
  '1': 'SwitchProfileResponse',
  '2': [
    {
      '1': 'profile',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.Profile',
      '10': 'profile'
    },
  ],
};

/// Descriptor for `SwitchProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List switchProfileResponseDescriptor = $convert.base64Decode(
    'ChVTd2l0Y2hQcm9maWxlUmVzcG9uc2USMAoHcHJvZmlsZRgBIAEoCzIWLnZvaWNlLnVzZXIudj'
    'EuUHJvZmlsZVIHcHJvZmlsZQ==');

@$core.Deprecated('Use listMyProfilesResponseDescriptor instead')
const ListMyProfilesResponse$json = {
  '1': 'ListMyProfilesResponse',
  '2': [
    {
      '1': 'profile_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.ProfileList',
      '10': 'profileList'
    },
  ],
};

/// Descriptor for `ListMyProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMyProfilesResponseDescriptor =
    $convert.base64Decode(
        'ChZMaXN0TXlQcm9maWxlc1Jlc3BvbnNlEj0KDHByb2ZpbGVfbGlzdBgBIAEoCzIaLnZvaWNlLn'
        'VzZXIudjEuUHJvZmlsZUxpc3RSC3Byb2ZpbGVMaXN0');

@$core.Deprecated('Use getPrivacySettingsResponseDescriptor instead')
const GetPrivacySettingsResponse$json = {
  '1': 'GetPrivacySettingsResponse',
  '2': [
    {
      '1': 'privacy_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacySettings',
      '10': 'privacySettings'
    },
  ],
};

/// Descriptor for `GetPrivacySettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPrivacySettingsResponseDescriptor =
    $convert.base64Decode(
        'ChpHZXRQcml2YWN5U2V0dGluZ3NSZXNwb25zZRJJChBwcml2YWN5X3NldHRpbmdzGAEgASgLMh'
        '4udm9pY2UudXNlci52MS5Qcml2YWN5U2V0dGluZ3NSD3ByaXZhY3lTZXR0aW5ncw==');

@$core.Deprecated('Use updatePrivacySettingsResponseDescriptor instead')
const UpdatePrivacySettingsResponse$json = {
  '1': 'UpdatePrivacySettingsResponse',
  '2': [
    {
      '1': 'privacy_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PrivacySettings',
      '10': 'privacySettings'
    },
  ],
};

/// Descriptor for `UpdatePrivacySettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePrivacySettingsResponseDescriptor =
    $convert.base64Decode(
        'Ch1VcGRhdGVQcml2YWN5U2V0dGluZ3NSZXNwb25zZRJJChBwcml2YWN5X3NldHRpbmdzGAEgAS'
        'gLMh4udm9pY2UudXNlci52MS5Qcml2YWN5U2V0dGluZ3NSD3ByaXZhY3lTZXR0aW5ncw==');

@$core.Deprecated('Use updatePresenceResponseDescriptor instead')
const UpdatePresenceResponse$json = {
  '1': 'UpdatePresenceResponse',
};

/// Descriptor for `UpdatePresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updatePresenceResponseDescriptor =
    $convert.base64Decode('ChZVcGRhdGVQcmVzZW5jZVJlc3BvbnNl');

@$core.Deprecated('Use getPresenceResponseDescriptor instead')
const GetPresenceResponse$json = {
  '1': 'GetPresenceResponse',
  '2': [
    {
      '1': 'presence_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PresenceStatus',
      '10': 'presenceStatus'
    },
  ],
};

/// Descriptor for `GetPresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPresenceResponseDescriptor = $convert.base64Decode(
    'ChNHZXRQcmVzZW5jZVJlc3BvbnNlEkYKD3ByZXNlbmNlX3N0YXR1cxgBIAEoCzIdLnZvaWNlLn'
    'VzZXIudjEuUHJlc2VuY2VTdGF0dXNSDnByZXNlbmNlU3RhdHVz');

@$core.Deprecated('Use getBulkPresenceResponseDescriptor instead')
const GetBulkPresenceResponse$json = {
  '1': 'GetBulkPresenceResponse',
  '2': [
    {
      '1': 'by_profile_id',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.user.v1.GetBulkPresenceResponse.ByProfileIdEntry',
      '10': 'byProfileId'
    },
  ],
  '3': [GetBulkPresenceResponse_ByProfileIdEntry$json],
};

@$core.Deprecated('Use getBulkPresenceResponseDescriptor instead')
const GetBulkPresenceResponse_ByProfileIdEntry$json = {
  '1': 'ByProfileIdEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.PresenceStatus',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `GetBulkPresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkPresenceResponseDescriptor = $convert.base64Decode(
    'ChdHZXRCdWxrUHJlc2VuY2VSZXNwb25zZRJbCg1ieV9wcm9maWxlX2lkGAEgAygLMjcudm9pY2'
    'UudXNlci52MS5HZXRCdWxrUHJlc2VuY2VSZXNwb25zZS5CeVByb2ZpbGVJZEVudHJ5UgtieVBy'
    'b2ZpbGVJZBpdChBCeVByb2ZpbGVJZEVudHJ5EhAKA2tleRgBIAEoCVIDa2V5EjMKBXZhbHVlGA'
    'IgASgLMh0udm9pY2UudXNlci52MS5QcmVzZW5jZVN0YXR1c1IFdmFsdWU6AjgB');

@$core.Deprecated('Use getSettingsResponseDescriptor instead')
const GetSettingsResponse$json = {
  '1': 'GetSettingsResponse',
  '2': [
    {
      '1': 'user_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSettings',
      '10': 'userSettings'
    },
  ],
};

/// Descriptor for `GetSettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSettingsResponseDescriptor = $convert.base64Decode(
    'ChNHZXRTZXR0aW5nc1Jlc3BvbnNlEkAKDXVzZXJfc2V0dGluZ3MYASABKAsyGy52b2ljZS51c2'
    'VyLnYxLlVzZXJTZXR0aW5nc1IMdXNlclNldHRpbmdz');

@$core.Deprecated('Use updateSettingsResponseDescriptor instead')
const UpdateSettingsResponse$json = {
  '1': 'UpdateSettingsResponse',
  '2': [
    {
      '1': 'user_settings',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.UserSettings',
      '10': 'userSettings'
    },
  ],
};

/// Descriptor for `UpdateSettingsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateSettingsResponseDescriptor =
    $convert.base64Decode(
        'ChZVcGRhdGVTZXR0aW5nc1Jlc3BvbnNlEkAKDXVzZXJfc2V0dGluZ3MYASABKAsyGy52b2ljZS'
        '51c2VyLnYxLlVzZXJTZXR0aW5nc1IMdXNlclNldHRpbmdz');

@$core.Deprecated('Use getOnboardingStateResponseDescriptor instead')
const GetOnboardingStateResponse$json = {
  '1': 'GetOnboardingStateResponse',
  '2': [
    {
      '1': 'onboarding_state',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.OnboardingState',
      '10': 'onboardingState'
    },
  ],
};

/// Descriptor for `GetOnboardingStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOnboardingStateResponseDescriptor =
    $convert.base64Decode(
        'ChpHZXRPbmJvYXJkaW5nU3RhdGVSZXNwb25zZRJJChBvbmJvYXJkaW5nX3N0YXRlGAEgASgLMh'
        '4udm9pY2UudXNlci52MS5PbmJvYXJkaW5nU3RhdGVSD29uYm9hcmRpbmdTdGF0ZQ==');

@$core.Deprecated('Use completeOnboardingStepResponseDescriptor instead')
const CompleteOnboardingStepResponse$json = {
  '1': 'CompleteOnboardingStepResponse',
  '2': [
    {
      '1': 'onboarding_state',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.OnboardingState',
      '10': 'onboardingState'
    },
  ],
};

/// Descriptor for `CompleteOnboardingStepResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeOnboardingStepResponseDescriptor =
    $convert.base64Decode(
        'Ch5Db21wbGV0ZU9uYm9hcmRpbmdTdGVwUmVzcG9uc2USSQoQb25ib2FyZGluZ19zdGF0ZRgBIA'
        'EoCzIeLnZvaWNlLnVzZXIudjEuT25ib2FyZGluZ1N0YXRlUg9vbmJvYXJkaW5nU3RhdGU=');

@$core.Deprecated('Use getVerificationStatusResponseDescriptor instead')
const GetVerificationStatusResponse$json = {
  '1': 'GetVerificationStatusResponse',
  '2': [
    {
      '1': 'verification_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.VerificationStatus',
      '10': 'verificationStatus'
    },
  ],
};

/// Descriptor for `GetVerificationStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVerificationStatusResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRWZXJpZmljYXRpb25TdGF0dXNSZXNwb25zZRJSChN2ZXJpZmljYXRpb25fc3RhdHVzGA'
        'EgASgLMiEudm9pY2UudXNlci52MS5WZXJpZmljYXRpb25TdGF0dXNSEnZlcmlmaWNhdGlvblN0'
        'YXR1cw==');

@$core.Deprecated('Use setVerificationRequestDescriptor instead')
const SetVerificationRequest$json = {
  '1': 'SetVerificationRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'verification_type',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'verificationType'
    },
    {'1': 'badge', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'badge', '17': true},
  ],
  '8': [
    {'1': '_badge'},
  ],
};

/// Descriptor for `SetVerificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVerificationRequestDescriptor = $convert.base64Decode(
    'ChZTZXRWZXJpZmljYXRpb25SZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZB'
    'IrChF2ZXJpZmljYXRpb25fdHlwZRgCIAEoCVIQdmVyaWZpY2F0aW9uVHlwZRIZCgViYWRnZRgD'
    'IAEoCUgAUgViYWRnZYgBAUIICgZfYmFkZ2U=');

@$core.Deprecated('Use setVerificationResponseDescriptor instead')
const SetVerificationResponse$json = {
  '1': 'SetVerificationResponse',
  '2': [
    {
      '1': 'verification_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.VerificationStatus',
      '10': 'verificationStatus'
    },
  ],
};

/// Descriptor for `SetVerificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setVerificationResponseDescriptor = $convert.base64Decode(
    'ChdTZXRWZXJpZmljYXRpb25SZXNwb25zZRJSChN2ZXJpZmljYXRpb25fc3RhdHVzGAEgASgLMi'
    'Eudm9pY2UudXNlci52MS5WZXJpZmljYXRpb25TdGF0dXNSEnZlcmlmaWNhdGlvblN0YXR1cw==');

@$core.Deprecated('Use clearVerificationRequestDescriptor instead')
const ClearVerificationRequest$json = {
  '1': 'ClearVerificationRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `ClearVerificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearVerificationRequestDescriptor =
    $convert.base64Decode(
        'ChhDbGVhclZlcmlmaWNhdGlvblJlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZU'
        'lk');

@$core.Deprecated('Use clearVerificationResponseDescriptor instead')
const ClearVerificationResponse$json = {
  '1': 'ClearVerificationResponse',
  '2': [
    {
      '1': 'verification_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.VerificationStatus',
      '10': 'verificationStatus'
    },
  ],
};

/// Descriptor for `ClearVerificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clearVerificationResponseDescriptor = $convert.base64Decode(
    'ChlDbGVhclZlcmlmaWNhdGlvblJlc3BvbnNlElIKE3ZlcmlmaWNhdGlvbl9zdGF0dXMYASABKA'
    'syIS52b2ljZS51c2VyLnYxLlZlcmlmaWNhdGlvblN0YXR1c1ISdmVyaWZpY2F0aW9uU3RhdHVz');

@$core.Deprecated('Use startOrganizationVerificationRequestDescriptor instead')
const StartOrganizationVerificationRequest$json = {
  '1': 'StartOrganizationVerificationRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'domain', '3': 2, '4': 1, '5': 9, '10': 'domain'},
  ],
};

/// Descriptor for `StartOrganizationVerificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startOrganizationVerificationRequestDescriptor =
    $convert.base64Decode(
        'CiRTdGFydE9yZ2FuaXphdGlvblZlcmlmaWNhdGlvblJlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIA'
        'EoCVIJcHJvZmlsZUlkEhYKBmRvbWFpbhgCIAEoCVIGZG9tYWlu');

@$core.Deprecated('Use startOrganizationVerificationResponseDescriptor instead')
const StartOrganizationVerificationResponse$json = {
  '1': 'StartOrganizationVerificationResponse',
  '2': [
    {'1': 'domain', '3': 1, '4': 1, '5': 9, '10': 'domain'},
    {'1': 'txt_record', '3': 2, '4': 1, '5': 9, '10': 'txtRecord'},
  ],
};

/// Descriptor for `StartOrganizationVerificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startOrganizationVerificationResponseDescriptor =
    $convert.base64Decode(
        'CiVTdGFydE9yZ2FuaXphdGlvblZlcmlmaWNhdGlvblJlc3BvbnNlEhYKBmRvbWFpbhgBIAEoCV'
        'IGZG9tYWluEh0KCnR4dF9yZWNvcmQYAiABKAlSCXR4dFJlY29yZA==');

@$core.Deprecated('Use checkOrganizationVerificationRequestDescriptor instead')
const CheckOrganizationVerificationRequest$json = {
  '1': 'CheckOrganizationVerificationRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `CheckOrganizationVerificationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkOrganizationVerificationRequestDescriptor =
    $convert.base64Decode(
        'CiRDaGVja09yZ2FuaXphdGlvblZlcmlmaWNhdGlvblJlcXVlc3QSHQoKcHJvZmlsZV9pZBgBIA'
        'EoCVIJcHJvZmlsZUlk');

@$core.Deprecated('Use checkOrganizationVerificationResponseDescriptor instead')
const CheckOrganizationVerificationResponse$json = {
  '1': 'CheckOrganizationVerificationResponse',
  '2': [
    {
      '1': 'verification_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.user.v1.VerificationStatus',
      '10': 'verificationStatus'
    },
  ],
};

/// Descriptor for `CheckOrganizationVerificationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkOrganizationVerificationResponseDescriptor =
    $convert.base64Decode(
        'CiVDaGVja09yZ2FuaXphdGlvblZlcmlmaWNhdGlvblJlc3BvbnNlElIKE3ZlcmlmaWNhdGlvbl'
        '9zdGF0dXMYASABKAsyIS52b2ljZS51c2VyLnYxLlZlcmlmaWNhdGlvblN0YXR1c1ISdmVyaWZp'
        'Y2F0aW9uU3RhdHVz');

@$core.Deprecated('Use applyDowngradeProfilesRequestDescriptor instead')
const ApplyDowngradeProfilesRequest$json = {
  '1': 'ApplyDowngradeProfilesRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'kept_profile_ids', '3': 2, '4': 3, '5': 9, '10': 'keptProfileIds'},
  ],
};

/// Descriptor for `ApplyDowngradeProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyDowngradeProfilesRequestDescriptor =
    $convert.base64Decode(
        'Ch1BcHBseURvd25ncmFkZVByb2ZpbGVzUmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2'
        'NvdW50SWQSKAoQa2VwdF9wcm9maWxlX2lkcxgCIAMoCVIOa2VwdFByb2ZpbGVJZHM=');

@$core.Deprecated('Use applyDowngradeProfilesResponseDescriptor instead')
const ApplyDowngradeProfilesResponse$json = {
  '1': 'ApplyDowngradeProfilesResponse',
  '2': [
    {'1': 'kept_profile_ids', '3': 1, '4': 3, '5': 9, '10': 'keptProfileIds'},
  ],
};

/// Descriptor for `ApplyDowngradeProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyDowngradeProfilesResponseDescriptor =
    $convert.base64Decode(
        'Ch5BcHBseURvd25ncmFkZVByb2ZpbGVzUmVzcG9uc2USKAoQa2VwdF9wcm9maWxlX2lkcxgBIA'
        'MoCVIOa2VwdFByb2ZpbGVJZHM=');
