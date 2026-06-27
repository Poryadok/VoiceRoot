// This is a generated file - do not edit.
//
// Generated from voice/social/v1/social.proto.

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

@$core.Deprecated('Use sendFriendInvitationRequestDescriptor instead')
const SendFriendInvitationRequest$json = {
  '1': 'SendFriendInvitationRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
  ],
};

/// Descriptor for `SendFriendInvitationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendFriendInvitationRequestDescriptor =
    $convert.base64Decode(
        'ChtTZW5kRnJpZW5kSW52aXRhdGlvblJlcXVlc3QSKgoRdGFyZ2V0X3Byb2ZpbGVfaWQYASABKA'
        'lSD3RhcmdldFByb2ZpbGVJZA==');

@$core.Deprecated('Use acceptFriendInvitationRequestDescriptor instead')
const AcceptFriendInvitationRequest$json = {
  '1': 'AcceptFriendInvitationRequest',
  '2': [
    {
      '1': 'requester_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'requesterProfileId'
    },
  ],
};

/// Descriptor for `AcceptFriendInvitationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptFriendInvitationRequestDescriptor =
    $convert.base64Decode(
        'Ch1BY2NlcHRGcmllbmRJbnZpdGF0aW9uUmVxdWVzdBIwChRyZXF1ZXN0ZXJfcHJvZmlsZV9pZB'
        'gBIAEoCVIScmVxdWVzdGVyUHJvZmlsZUlk');

@$core.Deprecated('Use declineFriendInvitationRequestDescriptor instead')
const DeclineFriendInvitationRequest$json = {
  '1': 'DeclineFriendInvitationRequest',
  '2': [
    {
      '1': 'requester_profile_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'requesterProfileId'
    },
  ],
};

/// Descriptor for `DeclineFriendInvitationRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineFriendInvitationRequestDescriptor =
    $convert.base64Decode(
        'Ch5EZWNsaW5lRnJpZW5kSW52aXRhdGlvblJlcXVlc3QSMAoUcmVxdWVzdGVyX3Byb2ZpbGVfaW'
        'QYASABKAlSEnJlcXVlc3RlclByb2ZpbGVJZA==');

@$core.Deprecated('Use removeFriendRequestDescriptor instead')
const RemoveFriendRequest$json = {
  '1': 'RemoveFriendRequest',
  '2': [
    {'1': 'friend_profile_id', '3': 1, '4': 1, '5': 9, '10': 'friendProfileId'},
  ],
};

/// Descriptor for `RemoveFriendRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFriendRequestDescriptor = $convert.base64Decode(
    'ChNSZW1vdmVGcmllbmRSZXF1ZXN0EioKEWZyaWVuZF9wcm9maWxlX2lkGAEgASgJUg9mcmllbm'
    'RQcm9maWxlSWQ=');

@$core.Deprecated('Use listFriendsRequestDescriptor instead')
const ListFriendsRequest$json = {
  '1': 'ListFriendsRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListFriendsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFriendsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0RnJpZW5kc1JlcXVlc3QSNgoEcGFnZRgBIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5DdX'
    'Jzb3JQYWdlUmVxdWVzdFIEcGFnZQ==');

@$core.Deprecated('Use friendListDescriptor instead')
const FriendList$json = {
  '1': 'FriendList',
  '2': [
    {
      '1': 'friends',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.social.v1.FriendEdge',
      '10': 'friends'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `FriendList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List friendListDescriptor = $convert.base64Decode(
    'CgpGcmllbmRMaXN0EjUKB2ZyaWVuZHMYASADKAsyGy52b2ljZS5zb2NpYWwudjEuRnJpZW5kRW'
    'RnZVIHZnJpZW5kcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use friendEdgeDescriptor instead')
const FriendEdge$json = {
  '1': 'FriendEdge',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'friends_since',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'friendsSince'
    },
  ],
};

/// Descriptor for `FriendEdge`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List friendEdgeDescriptor = $convert.base64Decode(
    'CgpGcmllbmRFZGdlEh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZBI/Cg1mcmllbmRzX3'
    'NpbmNlGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIMZnJpZW5kc1NpbmNl');

@$core.Deprecated('Use listFriendRequestsRequestDescriptor instead')
const ListFriendRequestsRequest$json = {
  '1': 'ListFriendRequestsRequest',
};

/// Descriptor for `ListFriendRequestsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFriendRequestsRequestDescriptor =
    $convert.base64Decode('ChlMaXN0RnJpZW5kUmVxdWVzdHNSZXF1ZXN0');

@$core.Deprecated('Use friendRequestListDescriptor instead')
const FriendRequestList$json = {
  '1': 'FriendRequestList',
  '2': [
    {
      '1': 'incoming',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.social.v1.PendingFriendRequest',
      '10': 'incoming'
    },
    {
      '1': 'outgoing',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.social.v1.PendingFriendRequest',
      '10': 'outgoing'
    },
  ],
};

/// Descriptor for `FriendRequestList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List friendRequestListDescriptor = $convert.base64Decode(
    'ChFGcmllbmRSZXF1ZXN0TGlzdBJBCghpbmNvbWluZxgBIAMoCzIlLnZvaWNlLnNvY2lhbC52MS'
    '5QZW5kaW5nRnJpZW5kUmVxdWVzdFIIaW5jb21pbmcSQQoIb3V0Z29pbmcYAiADKAsyJS52b2lj'
    'ZS5zb2NpYWwudjEuUGVuZGluZ0ZyaWVuZFJlcXVlc3RSCG91dGdvaW5n');

@$core.Deprecated('Use pendingFriendRequestDescriptor instead')
const PendingFriendRequest$json = {
  '1': 'PendingFriendRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'created_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
};

/// Descriptor for `PendingFriendRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pendingFriendRequestDescriptor = $convert.base64Decode(
    'ChRQZW5kaW5nRnJpZW5kUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSWQSOQ'
    'oKY3JlYXRlZF9hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWNyZWF0ZWRB'
    'dA==');

@$core.Deprecated('Use addContactRequestDescriptor instead')
const AddContactRequest$json = {
  '1': 'AddContactRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
    {'1': 'source', '3': 2, '4': 1, '5': 9, '10': 'source'},
  ],
};

/// Descriptor for `AddContactRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addContactRequestDescriptor = $convert.base64Decode(
    'ChFBZGRDb250YWN0UmVxdWVzdBIqChF0YXJnZXRfcHJvZmlsZV9pZBgBIAEoCVIPdGFyZ2V0UH'
    'JvZmlsZUlkEhYKBnNvdXJjZRgCIAEoCVIGc291cmNl');

@$core.Deprecated('Use removeContactRequestDescriptor instead')
const RemoveContactRequest$json = {
  '1': 'RemoveContactRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
  ],
};

/// Descriptor for `RemoveContactRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeContactRequestDescriptor = $convert.base64Decode(
    'ChRSZW1vdmVDb250YWN0UmVxdWVzdBIqChF0YXJnZXRfcHJvZmlsZV9pZBgBIAEoCVIPdGFyZ2'
    'V0UHJvZmlsZUlk');

@$core.Deprecated('Use listContactsRequestDescriptor instead')
const ListContactsRequest$json = {
  '1': 'ListContactsRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListContactsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listContactsRequestDescriptor = $convert.base64Decode(
    'ChNMaXN0Q29udGFjdHNSZXF1ZXN0EjYKBHBhZ2UYASABKAsyIi52b2ljZS5jb21tb24udjEuQ3'
    'Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use contactListDescriptor instead')
const ContactList$json = {
  '1': 'ContactList',
  '2': [
    {
      '1': 'contacts',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.social.v1.Contact',
      '10': 'contacts'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `ContactList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List contactListDescriptor = $convert.base64Decode(
    'CgtDb250YWN0TGlzdBI0Cghjb250YWN0cxgBIAMoCzIYLnZvaWNlLnNvY2lhbC52MS5Db250YW'
    'N0Ughjb250YWN0cxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use contactDescriptor instead')
const Contact$json = {
  '1': 'Contact',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'source', '3': 2, '4': 1, '5': 9, '10': 'source'},
    {'1': 'is_favorite', '3': 3, '4': 1, '5': 8, '10': 'isFavorite'},
  ],
};

/// Descriptor for `Contact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List contactDescriptor = $convert.base64Decode(
    'CgdDb250YWN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZBIWCgZzb3VyY2UYAiABKA'
    'lSBnNvdXJjZRIfCgtpc19mYXZvcml0ZRgDIAEoCFIKaXNGYXZvcml0ZQ==');

@$core.Deprecated('Use syncPhoneContactsRequestDescriptor instead')
const SyncPhoneContactsRequest$json = {
  '1': 'SyncPhoneContactsRequest',
  '2': [
    {
      '1': 'hashed_phone_numbers',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'hashedPhoneNumbers'
    },
  ],
};

/// Descriptor for `SyncPhoneContactsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List syncPhoneContactsRequestDescriptor =
    $convert.base64Decode(
        'ChhTeW5jUGhvbmVDb250YWN0c1JlcXVlc3QSMAoUaGFzaGVkX3Bob25lX251bWJlcnMYASADKA'
        'lSEmhhc2hlZFBob25lTnVtYmVycw==');

@$core.Deprecated('Use syncPhoneContactsResponseDescriptor instead')
const SyncPhoneContactsResponse$json = {
  '1': 'SyncPhoneContactsResponse',
  '2': [
    {
      '1': 'matched_profile_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'matchedProfileIds'
    },
  ],
};

/// Descriptor for `SyncPhoneContactsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List syncPhoneContactsResponseDescriptor =
    $convert.base64Decode(
        'ChlTeW5jUGhvbmVDb250YWN0c1Jlc3BvbnNlEi4KE21hdGNoZWRfcHJvZmlsZV9pZHMYASADKA'
        'lSEW1hdGNoZWRQcm9maWxlSWRz');

@$core.Deprecated('Use setFavoriteRequestDescriptor instead')
const SetFavoriteRequest$json = {
  '1': 'SetFavoriteRequest',
  '2': [
    {'1': 'friend_profile_id', '3': 1, '4': 1, '5': 9, '10': 'friendProfileId'},
    {'1': 'favorite', '3': 2, '4': 1, '5': 8, '10': 'favorite'},
  ],
};

/// Descriptor for `SetFavoriteRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setFavoriteRequestDescriptor = $convert.base64Decode(
    'ChJTZXRGYXZvcml0ZVJlcXVlc3QSKgoRZnJpZW5kX3Byb2ZpbGVfaWQYASABKAlSD2ZyaWVuZF'
    'Byb2ZpbGVJZBIaCghmYXZvcml0ZRgCIAEoCFIIZmF2b3JpdGU=');

@$core.Deprecated('Use listFavoritesRequestDescriptor instead')
const ListFavoritesRequest$json = {
  '1': 'ListFavoritesRequest',
};

/// Descriptor for `ListFavoritesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFavoritesRequestDescriptor =
    $convert.base64Decode('ChRMaXN0RmF2b3JpdGVzUmVxdWVzdA==');

@$core.Deprecated('Use blockAccountRequestDescriptor instead')
const BlockAccountRequest$json = {
  '1': 'BlockAccountRequest',
  '2': [
    {
      '1': 'blocked_account_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'blockedAccountId'
    },
  ],
};

/// Descriptor for `BlockAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List blockAccountRequestDescriptor = $convert.base64Decode(
    'ChNCbG9ja0FjY291bnRSZXF1ZXN0EiwKEmJsb2NrZWRfYWNjb3VudF9pZBgBIAEoCVIQYmxvY2'
    'tlZEFjY291bnRJZA==');

@$core.Deprecated('Use unblockAccountRequestDescriptor instead')
const UnblockAccountRequest$json = {
  '1': 'UnblockAccountRequest',
  '2': [
    {
      '1': 'blocked_account_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'blockedAccountId'
    },
  ],
};

/// Descriptor for `UnblockAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unblockAccountRequestDescriptor = $convert.base64Decode(
    'ChVVbmJsb2NrQWNjb3VudFJlcXVlc3QSLAoSYmxvY2tlZF9hY2NvdW50X2lkGAEgASgJUhBibG'
    '9ja2VkQWNjb3VudElk');

@$core.Deprecated('Use listBlockedRequestDescriptor instead')
const ListBlockedRequest$json = {
  '1': 'ListBlockedRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListBlockedRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBlockedRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0QmxvY2tlZFJlcXVlc3QSNgoEcGFnZRgBIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5DdX'
    'Jzb3JQYWdlUmVxdWVzdFIEcGFnZQ==');

@$core.Deprecated('Use blockedListDescriptor instead')
const BlockedList$json = {
  '1': 'BlockedList',
  '2': [
    {
      '1': 'blocked',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.social.v1.BlockedAccount',
      '10': 'blocked'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `BlockedList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List blockedListDescriptor = $convert.base64Decode(
    'CgtCbG9ja2VkTGlzdBI5CgdibG9ja2VkGAEgAygLMh8udm9pY2Uuc29jaWFsLnYxLkJsb2NrZW'
    'RBY2NvdW50UgdibG9ja2VkEh8KC25leHRfY3Vyc29yGAIgASgJUgpuZXh0Q3Vyc29y');

@$core.Deprecated('Use blockedAccountDescriptor instead')
const BlockedAccount$json = {
  '1': 'BlockedAccount',
  '2': [
    {
      '1': 'blocked_account_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'blockedAccountId'
    },
    {
      '1': 'created_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
};

/// Descriptor for `BlockedAccount`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List blockedAccountDescriptor = $convert.base64Decode(
    'Cg5CbG9ja2VkQWNjb3VudBIsChJibG9ja2VkX2FjY291bnRfaWQYASABKAlSEGJsb2NrZWRBY2'
    'NvdW50SWQSOQoKY3JlYXRlZF9hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBS'
    'CWNyZWF0ZWRBdA==');

@$core.Deprecated('Use isBlockedRequestDescriptor instead')
const IsBlockedRequest$json = {
  '1': 'IsBlockedRequest',
  '2': [
    {'1': 'account_id_a', '3': 1, '4': 1, '5': 9, '10': 'accountIdA'},
    {'1': 'account_id_b', '3': 2, '4': 1, '5': 9, '10': 'accountIdB'},
  ],
};

/// Descriptor for `IsBlockedRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List isBlockedRequestDescriptor = $convert.base64Decode(
    'ChBJc0Jsb2NrZWRSZXF1ZXN0EiAKDGFjY291bnRfaWRfYRgBIAEoCVIKYWNjb3VudElkQRIgCg'
    'xhY2NvdW50X2lkX2IYAiABKAlSCmFjY291bnRJZEI=');

@$core.Deprecated('Use isBlockedResponseDescriptor instead')
const IsBlockedResponse$json = {
  '1': 'IsBlockedResponse',
  '2': [
    {'1': 'blocked', '3': 1, '4': 1, '5': 8, '10': 'blocked'},
  ],
};

/// Descriptor for `IsBlockedResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List isBlockedResponseDescriptor = $convert.base64Decode(
    'ChFJc0Jsb2NrZWRSZXNwb25zZRIYCgdibG9ja2VkGAEgASgIUgdibG9ja2Vk');

@$core.Deprecated('Use areFriendsRequestDescriptor instead')
const AreFriendsRequest$json = {
  '1': 'AreFriendsRequest',
  '2': [
    {'1': 'profile_id_a', '3': 1, '4': 1, '5': 9, '10': 'profileIdA'},
    {'1': 'profile_id_b', '3': 2, '4': 1, '5': 9, '10': 'profileIdB'},
  ],
};

/// Descriptor for `AreFriendsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List areFriendsRequestDescriptor = $convert.base64Decode(
    'ChFBcmVGcmllbmRzUmVxdWVzdBIgCgxwcm9maWxlX2lkX2EYASABKAlSCnByb2ZpbGVJZEESIA'
    'oMcHJvZmlsZV9pZF9iGAIgASgJUgpwcm9maWxlSWRC');

@$core.Deprecated('Use areFriendsResponseDescriptor instead')
const AreFriendsResponse$json = {
  '1': 'AreFriendsResponse',
  '2': [
    {'1': 'friends', '3': 1, '4': 1, '5': 8, '10': 'friends'},
  ],
};

/// Descriptor for `AreFriendsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List areFriendsResponseDescriptor =
    $convert.base64Decode(
        'ChJBcmVGcmllbmRzUmVzcG9uc2USGAoHZnJpZW5kcxgBIAEoCFIHZnJpZW5kcw==');

@$core.Deprecated('Use areFriendsOfFriendsRequestDescriptor instead')
const AreFriendsOfFriendsRequest$json = {
  '1': 'AreFriendsOfFriendsRequest',
  '2': [
    {'1': 'profile_id_a', '3': 1, '4': 1, '5': 9, '10': 'profileIdA'},
    {'1': 'profile_id_b', '3': 2, '4': 1, '5': 9, '10': 'profileIdB'},
  ],
};

/// Descriptor for `AreFriendsOfFriendsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List areFriendsOfFriendsRequestDescriptor =
    $convert.base64Decode(
        'ChpBcmVGcmllbmRzT2ZGcmllbmRzUmVxdWVzdBIgCgxwcm9maWxlX2lkX2EYASABKAlSCnByb2'
        'ZpbGVJZEESIAoMcHJvZmlsZV9pZF9iGAIgASgJUgpwcm9maWxlSWRC');

@$core.Deprecated('Use areFriendsOfFriendsResponseDescriptor instead')
const AreFriendsOfFriendsResponse$json = {
  '1': 'AreFriendsOfFriendsResponse',
  '2': [
    {'1': 'friends', '3': 1, '4': 1, '5': 8, '10': 'friends'},
  ],
};

/// Descriptor for `AreFriendsOfFriendsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List areFriendsOfFriendsResponseDescriptor =
    $convert.base64Decode(
        'ChtBcmVGcmllbmRzT2ZGcmllbmRzUmVzcG9uc2USGAoHZnJpZW5kcxgBIAEoCFIHZnJpZW5kcw'
        '==');

@$core.Deprecated('Use getFriendsOfFriendsRequestDescriptor instead')
const GetFriendsOfFriendsRequest$json = {
  '1': 'GetFriendsOfFriendsRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetFriendsOfFriendsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFriendsOfFriendsRequestDescriptor =
    $convert.base64Decode(
        'ChpHZXRGcmllbmRzT2ZGcmllbmRzUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maW'
        'xlSWQ=');

@$core.Deprecated('Use profileIdListDescriptor instead')
const ProfileIdList$json = {
  '1': 'ProfileIdList',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `ProfileIdList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List profileIdListDescriptor = $convert.base64Decode(
    'Cg1Qcm9maWxlSWRMaXN0Eh8KC3Byb2ZpbGVfaWRzGAEgAygJUgpwcm9maWxlSWRz');

@$core.Deprecated('Use sendFriendInvitationResponseDescriptor instead')
const SendFriendInvitationResponse$json = {
  '1': 'SendFriendInvitationResponse',
};

/// Descriptor for `SendFriendInvitationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendFriendInvitationResponseDescriptor =
    $convert.base64Decode('ChxTZW5kRnJpZW5kSW52aXRhdGlvblJlc3BvbnNl');

@$core.Deprecated('Use acceptFriendInvitationResponseDescriptor instead')
const AcceptFriendInvitationResponse$json = {
  '1': 'AcceptFriendInvitationResponse',
};

/// Descriptor for `AcceptFriendInvitationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptFriendInvitationResponseDescriptor =
    $convert.base64Decode('Ch5BY2NlcHRGcmllbmRJbnZpdGF0aW9uUmVzcG9uc2U=');

@$core.Deprecated('Use declineFriendInvitationResponseDescriptor instead')
const DeclineFriendInvitationResponse$json = {
  '1': 'DeclineFriendInvitationResponse',
};

/// Descriptor for `DeclineFriendInvitationResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineFriendInvitationResponseDescriptor =
    $convert.base64Decode('Ch9EZWNsaW5lRnJpZW5kSW52aXRhdGlvblJlc3BvbnNl');

@$core.Deprecated('Use removeFriendResponseDescriptor instead')
const RemoveFriendResponse$json = {
  '1': 'RemoveFriendResponse',
};

/// Descriptor for `RemoveFriendResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeFriendResponseDescriptor =
    $convert.base64Decode('ChRSZW1vdmVGcmllbmRSZXNwb25zZQ==');

@$core.Deprecated('Use listFriendsResponseDescriptor instead')
const ListFriendsResponse$json = {
  '1': 'ListFriendsResponse',
  '2': [
    {
      '1': 'friend_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.FriendList',
      '10': 'friendList'
    },
  ],
};

/// Descriptor for `ListFriendsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFriendsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0RnJpZW5kc1Jlc3BvbnNlEjwKC2ZyaWVuZF9saXN0GAEgASgLMhsudm9pY2Uuc29jaW'
    'FsLnYxLkZyaWVuZExpc3RSCmZyaWVuZExpc3Q=');

@$core.Deprecated('Use listFriendRequestsResponseDescriptor instead')
const ListFriendRequestsResponse$json = {
  '1': 'ListFriendRequestsResponse',
  '2': [
    {
      '1': 'friend_request_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.FriendRequestList',
      '10': 'friendRequestList'
    },
  ],
};

/// Descriptor for `ListFriendRequestsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFriendRequestsResponseDescriptor =
    $convert.base64Decode(
        'ChpMaXN0RnJpZW5kUmVxdWVzdHNSZXNwb25zZRJSChNmcmllbmRfcmVxdWVzdF9saXN0GAEgAS'
        'gLMiIudm9pY2Uuc29jaWFsLnYxLkZyaWVuZFJlcXVlc3RMaXN0UhFmcmllbmRSZXF1ZXN0TGlz'
        'dA==');

@$core.Deprecated('Use addContactResponseDescriptor instead')
const AddContactResponse$json = {
  '1': 'AddContactResponse',
};

/// Descriptor for `AddContactResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addContactResponseDescriptor =
    $convert.base64Decode('ChJBZGRDb250YWN0UmVzcG9uc2U=');

@$core.Deprecated('Use removeContactResponseDescriptor instead')
const RemoveContactResponse$json = {
  '1': 'RemoveContactResponse',
};

/// Descriptor for `RemoveContactResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeContactResponseDescriptor =
    $convert.base64Decode('ChVSZW1vdmVDb250YWN0UmVzcG9uc2U=');

@$core.Deprecated('Use listContactsResponseDescriptor instead')
const ListContactsResponse$json = {
  '1': 'ListContactsResponse',
  '2': [
    {
      '1': 'contact_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.ContactList',
      '10': 'contactList'
    },
  ],
};

/// Descriptor for `ListContactsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listContactsResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0Q29udGFjdHNSZXNwb25zZRI/Cgxjb250YWN0X2xpc3QYASABKAsyHC52b2ljZS5zb2'
    'NpYWwudjEuQ29udGFjdExpc3RSC2NvbnRhY3RMaXN0');

@$core.Deprecated('Use setFavoriteResponseDescriptor instead')
const SetFavoriteResponse$json = {
  '1': 'SetFavoriteResponse',
};

/// Descriptor for `SetFavoriteResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setFavoriteResponseDescriptor =
    $convert.base64Decode('ChNTZXRGYXZvcml0ZVJlc3BvbnNl');

@$core.Deprecated('Use listFavoritesResponseDescriptor instead')
const ListFavoritesResponse$json = {
  '1': 'ListFavoritesResponse',
  '2': [
    {
      '1': 'friend_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.FriendList',
      '10': 'friendList'
    },
  ],
};

/// Descriptor for `ListFavoritesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFavoritesResponseDescriptor = $convert.base64Decode(
    'ChVMaXN0RmF2b3JpdGVzUmVzcG9uc2USPAoLZnJpZW5kX2xpc3QYASABKAsyGy52b2ljZS5zb2'
    'NpYWwudjEuRnJpZW5kTGlzdFIKZnJpZW5kTGlzdA==');

@$core.Deprecated('Use blockAccountResponseDescriptor instead')
const BlockAccountResponse$json = {
  '1': 'BlockAccountResponse',
};

/// Descriptor for `BlockAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List blockAccountResponseDescriptor =
    $convert.base64Decode('ChRCbG9ja0FjY291bnRSZXNwb25zZQ==');

@$core.Deprecated('Use unblockAccountResponseDescriptor instead')
const UnblockAccountResponse$json = {
  '1': 'UnblockAccountResponse',
};

/// Descriptor for `UnblockAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unblockAccountResponseDescriptor =
    $convert.base64Decode('ChZVbmJsb2NrQWNjb3VudFJlc3BvbnNl');

@$core.Deprecated('Use listBlockedResponseDescriptor instead')
const ListBlockedResponse$json = {
  '1': 'ListBlockedResponse',
  '2': [
    {
      '1': 'blocked_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.BlockedList',
      '10': 'blockedList'
    },
  ],
};

/// Descriptor for `ListBlockedResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBlockedResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0QmxvY2tlZFJlc3BvbnNlEj8KDGJsb2NrZWRfbGlzdBgBIAEoCzIcLnZvaWNlLnNvY2'
    'lhbC52MS5CbG9ja2VkTGlzdFILYmxvY2tlZExpc3Q=');

@$core.Deprecated('Use getFriendsOfFriendsResponseDescriptor instead')
const GetFriendsOfFriendsResponse$json = {
  '1': 'GetFriendsOfFriendsResponse',
  '2': [
    {
      '1': 'profile_id_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.social.v1.ProfileIdList',
      '10': 'profileIdList'
    },
  ],
};

/// Descriptor for `GetFriendsOfFriendsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFriendsOfFriendsResponseDescriptor =
    $convert.base64Decode(
        'ChtHZXRGcmllbmRzT2ZGcmllbmRzUmVzcG9uc2USRgoPcHJvZmlsZV9pZF9saXN0GAEgASgLMh'
        '4udm9pY2Uuc29jaWFsLnYxLlByb2ZpbGVJZExpc3RSDXByb2ZpbGVJZExpc3Q=');
