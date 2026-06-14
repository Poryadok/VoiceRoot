// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat.proto.

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

@$core.Deprecated('Use chatTypeDescriptor instead')
const ChatType$json = {
  '1': 'ChatType',
  '2': [
    {'1': 'CHAT_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'CHAT_TYPE_DM', '2': 1},
    {'1': 'CHAT_TYPE_GROUP', '2': 2},
    {'1': 'CHAT_TYPE_CHANNEL', '2': 3},
  ],
};

/// Descriptor for `ChatType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List chatTypeDescriptor = $convert.base64Decode(
    'CghDaGF0VHlwZRIZChVDSEFUX1RZUEVfVU5TUEVDSUZJRUQQABIQCgxDSEFUX1RZUEVfRE0QAR'
    'ITCg9DSEFUX1RZUEVfR1JPVVAQAhIVChFDSEFUX1RZUEVfQ0hBTk5FTBAD');

@$core.Deprecated('Use chatDescriptor instead')
const Chat$json = {
  '1': 'Chat',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatType',
      '10': 'type'
    },
    {
      '1': 'space_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'spaceId',
      '17': true
    },
    {'1': 'name', '3': 4, '4': 1, '5': 9, '9': 1, '10': 'name', '17': true},
    {
      '1': 'avatar_url',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'avatarUrl',
      '17': true
    },
    {'1': 'topic', '3': 6, '4': 1, '5': 9, '9': 3, '10': 'topic', '17': true},
    {
      '1': 'creator_profile_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'creatorProfileId'
    },
    {'1': 'slow_mode_seconds', '3': 8, '4': 1, '5': 5, '10': 'slowModeSeconds'},
    {
      '1': 'last_message_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 4,
      '10': 'lastMessageAt',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'updated_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
    {'1': 'threads_enabled', '3': 12, '4': 1, '5': 8, '10': 'threadsEnabled'},
    {
      '1': 'allow_user_main_feed',
      '3': 13,
      '4': 1,
      '5': 8,
      '10': 'allowUserMainFeed'
    },
    {'1': 'e2e_enabled', '3': 14, '4': 1, '5': 8, '10': 'e2eEnabled'},
  ],
  '8': [
    {'1': '_space_id'},
    {'1': '_name'},
    {'1': '_avatar_url'},
    {'1': '_topic'},
    {'1': '_last_message_at'},
  ],
};

/// Descriptor for `Chat`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatDescriptor = $convert.base64Decode(
    'CgRDaGF0Eg4KAmlkGAEgASgJUgJpZBIrCgR0eXBlGAIgASgOMhcudm9pY2UuY2hhdC52MS5DaG'
    'F0VHlwZVIEdHlwZRIeCghzcGFjZV9pZBgDIAEoCUgAUgdzcGFjZUlkiAEBEhcKBG5hbWUYBCAB'
    'KAlIAVIEbmFtZYgBARIiCgphdmF0YXJfdXJsGAUgASgJSAJSCWF2YXRhclVybIgBARIZCgV0b3'
    'BpYxgGIAEoCUgDUgV0b3BpY4gBARIsChJjcmVhdG9yX3Byb2ZpbGVfaWQYByABKAlSEGNyZWF0'
    'b3JQcm9maWxlSWQSKgoRc2xvd19tb2RlX3NlY29uZHMYCCABKAVSD3Nsb3dNb2RlU2Vjb25kcx'
    'JHCg9sYXN0X21lc3NhZ2VfYXQYCSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSARS'
    'DWxhc3RNZXNzYWdlQXSIAQESOQoKY3JlYXRlZF9hdBgKIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi'
    '5UaW1lc3RhbXBSCWNyZWF0ZWRBdBI5Cgp1cGRhdGVkX2F0GAsgASgLMhouZ29vZ2xlLnByb3Rv'
    'YnVmLlRpbWVzdGFtcFIJdXBkYXRlZEF0EicKD3RocmVhZHNfZW5hYmxlZBgMIAEoCFIOdGhyZW'
    'Fkc0VuYWJsZWQSLwoUYWxsb3dfdXNlcl9tYWluX2ZlZWQYDSABKAhSEWFsbG93VXNlck1haW5G'
    'ZWVkEh8KC2UyZV9lbmFibGVkGA4gASgIUgplMmVFbmFibGVkQgsKCV9zcGFjZV9pZEIHCgVfbm'
    'FtZUINCgtfYXZhdGFyX3VybEIICgZfdG9waWNCEgoQX2xhc3RfbWVzc2FnZV9hdA==');

@$core.Deprecated('Use createDMRequestDescriptor instead')
const CreateDMRequest$json = {
  '1': 'CreateDMRequest',
  '2': [
    {'1': 'other_profile_id', '3': 1, '4': 1, '5': 9, '10': 'otherProfileId'},
  ],
};

/// Descriptor for `CreateDMRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createDMRequestDescriptor = $convert.base64Decode(
    'Cg9DcmVhdGVETVJlcXVlc3QSKAoQb3RoZXJfcHJvZmlsZV9pZBgBIAEoCVIOb3RoZXJQcm9maW'
    'xlSWQ=');

@$core.Deprecated('Use getDMRequestDescriptor instead')
const GetDMRequest$json = {
  '1': 'GetDMRequest',
  '2': [
    {'1': 'other_profile_id', '3': 1, '4': 1, '5': 9, '10': 'otherProfileId'},
  ],
};

/// Descriptor for `GetDMRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDMRequestDescriptor = $convert.base64Decode(
    'CgxHZXRETVJlcXVlc3QSKAoQb3RoZXJfcHJvZmlsZV9pZBgBIAEoCVIOb3RoZXJQcm9maWxlSW'
    'Q=');

@$core.Deprecated('Use createChatRequestDescriptor instead')
const CreateChatRequest$json = {
  '1': 'CreateChatRequest',
  '2': [
    {
      '1': 'type',
      '3': 1,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatType',
      '10': 'type'
    },
    {
      '1': 'space_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'spaceId',
      '17': true
    },
    {'1': 'name', '3': 3, '4': 1, '5': 9, '9': 1, '10': 'name', '17': true},
    {'1': 'topic', '3': 4, '4': 1, '5': 9, '9': 2, '10': 'topic', '17': true},
  ],
  '8': [
    {'1': '_space_id'},
    {'1': '_name'},
    {'1': '_topic'},
  ],
};

/// Descriptor for `CreateChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createChatRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVDaGF0UmVxdWVzdBIrCgR0eXBlGAEgASgOMhcudm9pY2UuY2hhdC52MS5DaGF0VH'
    'lwZVIEdHlwZRIeCghzcGFjZV9pZBgCIAEoCUgAUgdzcGFjZUlkiAEBEhcKBG5hbWUYAyABKAlI'
    'AVIEbmFtZYgBARIZCgV0b3BpYxgEIAEoCUgCUgV0b3BpY4gBAUILCglfc3BhY2VfaWRCBwoFX2'
    '5hbWVCCAoGX3RvcGlj');

@$core.Deprecated('Use updateChatRequestDescriptor instead')
const UpdateChatRequest$json = {
  '1': 'UpdateChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {'1': 'topic', '3': 3, '4': 1, '5': 9, '9': 1, '10': 'topic', '17': true},
    {
      '1': 'slow_mode_seconds',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 2,
      '10': 'slowModeSeconds',
      '17': true
    },
    {
      '1': 'avatar_url',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'avatarUrl',
      '17': true
    },
    {
      '1': 'threads_enabled',
      '3': 6,
      '4': 1,
      '5': 8,
      '9': 4,
      '10': 'threadsEnabled',
      '17': true
    },
    {
      '1': 'allow_user_main_feed',
      '3': 7,
      '4': 1,
      '5': 8,
      '9': 5,
      '10': 'allowUserMainFeed',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_topic'},
    {'1': '_slow_mode_seconds'},
    {'1': '_avatar_url'},
    {'1': '_threads_enabled'},
    {'1': '_allow_user_main_feed'},
  ],
};

/// Descriptor for `UpdateChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateChatRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVDaGF0UmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQSFwoEbmFtZRgCIA'
    'EoCUgAUgRuYW1liAEBEhkKBXRvcGljGAMgASgJSAFSBXRvcGljiAEBEi8KEXNsb3dfbW9kZV9z'
    'ZWNvbmRzGAQgASgFSAJSD3Nsb3dNb2RlU2Vjb25kc4gBARIiCgphdmF0YXJfdXJsGAUgASgJSA'
    'NSCWF2YXRhclVybIgBARIsCg90aHJlYWRzX2VuYWJsZWQYBiABKAhIBFIOdGhyZWFkc0VuYWJs'
    'ZWSIAQESNAoUYWxsb3dfdXNlcl9tYWluX2ZlZWQYByABKAhIBVIRYWxsb3dVc2VyTWFpbkZlZW'
    'SIAQFCBwoFX25hbWVCCAoGX3RvcGljQhQKEl9zbG93X21vZGVfc2Vjb25kc0INCgtfYXZhdGFy'
    'X3VybEISChBfdGhyZWFkc19lbmFibGVkQhcKFV9hbGxvd191c2VyX21haW5fZmVlZA==');

@$core.Deprecated('Use deleteChatRequestDescriptor instead')
const DeleteChatRequest$json = {
  '1': 'DeleteChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `DeleteChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteChatRequestDescriptor = $convert.base64Decode(
    'ChFEZWxldGVDaGF0UmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQ=');

@$core.Deprecated('Use addMembersRequestDescriptor instead')
const AddMembersRequest$json = {
  '1': 'AddMembersRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'profile_ids', '3': 2, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `AddMembersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addMembersRequestDescriptor = $convert.base64Decode(
    'ChFBZGRNZW1iZXJzUmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQSHwoLcHJvZmlsZV'
    '9pZHMYAiADKAlSCnByb2ZpbGVJZHM=');

@$core.Deprecated('Use removeMemberRequestDescriptor instead')
const RemoveMemberRequest$json = {
  '1': 'RemoveMemberRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `RemoveMemberRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberRequestDescriptor = $convert.base64Decode(
    'ChNSZW1vdmVNZW1iZXJSZXF1ZXN0EhcKB2NoYXRfaWQYASABKAlSBmNoYXRJZBIdCgpwcm9maW'
    'xlX2lkGAIgASgJUglwcm9maWxlSWQ=');

@$core.Deprecated('Use leaveChatRequestDescriptor instead')
const LeaveChatRequest$json = {
  '1': 'LeaveChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `LeaveChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveChatRequestDescriptor = $convert.base64Decode(
    'ChBMZWF2ZUNoYXRSZXF1ZXN0EhcKB2NoYXRfaWQYASABKAlSBmNoYXRJZA==');

@$core.Deprecated('Use transferGroupOwnershipRequestDescriptor instead')
const TransferGroupOwnershipRequest$json = {
  '1': 'TransferGroupOwnershipRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'new_owner_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
  ],
};

/// Descriptor for `TransferGroupOwnershipRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transferGroupOwnershipRequestDescriptor =
    $convert.base64Decode(
        'Ch1UcmFuc2Zlckdyb3VwT3duZXJzaGlwUmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SW'
        'QSLwoUbmV3X293bmVyX3Byb2ZpbGVfaWQYAiABKAlSEW5ld093bmVyUHJvZmlsZUlk');

@$core.Deprecated('Use transferGroupOwnershipResponseDescriptor instead')
const TransferGroupOwnershipResponse$json = {
  '1': 'TransferGroupOwnershipResponse',
};

/// Descriptor for `TransferGroupOwnershipResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List transferGroupOwnershipResponseDescriptor =
    $convert.base64Decode('Ch5UcmFuc2Zlckdyb3VwT3duZXJzaGlwUmVzcG9uc2U=');

@$core.Deprecated('Use listMembersRequestDescriptor instead')
const ListMembersRequest$json = {
  '1': 'ListMembersRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
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

/// Descriptor for `ListMembersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0TWVtYmVyc1JlcXVlc3QSFwoHY2hhdF9pZBgBIAEoCVIGY2hhdElkEjYKBHBhZ2UYAi'
    'ABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use memberListDescriptor instead')
const MemberList$json = {
  '1': 'MemberList',
  '2': [
    {
      '1': 'members',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatMember',
      '10': 'members'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `MemberList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List memberListDescriptor = $convert.base64Decode(
    'CgpNZW1iZXJMaXN0EjMKB21lbWJlcnMYASADKAsyGS52b2ljZS5jaGF0LnYxLkNoYXRNZW1iZX'
    'JSB21lbWJlcnMSHwoLbmV4dF9jdXJzb3IYAiABKAlSCm5leHRDdXJzb3I=');

@$core.Deprecated('Use chatMemberDescriptor instead')
const ChatMember$json = {
  '1': 'ChatMember',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role', '3': 2, '4': 1, '5': 9, '10': 'role'},
    {
      '1': 'joined_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'joinedAt'
    },
    {
      '1': 'muted_until',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'mutedUntil',
      '17': true
    },
    {'1': 'is_archived', '3': 5, '4': 1, '5': 8, '10': 'isArchived'},
  ],
  '8': [
    {'1': '_muted_until'},
  ],
};

/// Descriptor for `ChatMember`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatMemberDescriptor = $convert.base64Decode(
    'CgpDaGF0TWVtYmVyEh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZBISCgRyb2xlGAIgAS'
    'gJUgRyb2xlEjcKCWpvaW5lZF9hdBgDIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBS'
    'CGpvaW5lZEF0EkAKC211dGVkX3VudGlsGAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdG'
    'FtcEgAUgptdXRlZFVudGlsiAEBEh8KC2lzX2FyY2hpdmVkGAUgASgIUgppc0FyY2hpdmVkQg4K'
    'DF9tdXRlZF91bnRpbA==');

@$core.Deprecated('Use listChatsRequestDescriptor instead')
const ListChatsRequest$json = {
  '1': 'ListChatsRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
    {'1': 'inbox', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'inbox', '17': true},
  ],
  '8': [
    {'1': '_inbox'},
  ],
};

/// Descriptor for `ListChatsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listChatsRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0Q2hhdHNSZXF1ZXN0EjYKBHBhZ2UYASABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc2'
    '9yUGFnZVJlcXVlc3RSBHBhZ2USGQoFaW5ib3gYAiABKAlIAFIFaW5ib3iIAQFCCAoGX2luYm94');

@$core.Deprecated('Use chatListItemDescriptor instead')
const ChatListItem$json = {
  '1': 'ChatListItem',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
    {
      '1': 'last_message_preview',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'lastMessagePreview',
      '17': true
    },
    {'1': 'unread_count', '3': 3, '4': 1, '5': 3, '10': 'unreadCount'},
    {'1': 'inbox', '3': 4, '4': 1, '5': 9, '9': 1, '10': 'inbox', '17': true},
    {
      '1': 'is_stranger',
      '3': 5,
      '4': 1,
      '5': 8,
      '9': 2,
      '10': 'isStranger',
      '17': true
    },
    {
      '1': 'dm_peer_profile_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'dmPeerProfileId',
      '17': true
    },
  ],
  '8': [
    {'1': '_last_message_preview'},
    {'1': '_inbox'},
    {'1': '_is_stranger'},
    {'1': '_dm_peer_profile_id'},
  ],
};

/// Descriptor for `ChatListItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatListItemDescriptor = $convert.base64Decode(
    'CgxDaGF0TGlzdEl0ZW0SJwoEY2hhdBgBIAEoCzITLnZvaWNlLmNoYXQudjEuQ2hhdFIEY2hhdB'
    'I1ChRsYXN0X21lc3NhZ2VfcHJldmlldxgCIAEoCUgAUhJsYXN0TWVzc2FnZVByZXZpZXeIAQES'
    'IQoMdW5yZWFkX2NvdW50GAMgASgDUgt1bnJlYWRDb3VudBIZCgVpbmJveBgEIAEoCUgBUgVpbm'
    'JveIgBARIkCgtpc19zdHJhbmdlchgFIAEoCEgCUgppc1N0cmFuZ2VyiAEBEjAKEmRtX3BlZXJf'
    'cHJvZmlsZV9pZBgGIAEoCUgDUg9kbVBlZXJQcm9maWxlSWSIAQFCFwoVX2xhc3RfbWVzc2FnZV'
    '9wcmV2aWV3QggKBl9pbmJveEIOCgxfaXNfc3RyYW5nZXJCFQoTX2RtX3BlZXJfcHJvZmlsZV9p'
    'ZA==');

@$core.Deprecated('Use chatListDescriptor instead')
const ChatList$json = {
  '1': 'ChatList',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatListItem',
      '10': 'items'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `ChatList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatListDescriptor = $convert.base64Decode(
    'CghDaGF0TGlzdBIxCgVpdGVtcxgBIAMoCzIbLnZvaWNlLmNoYXQudjEuQ2hhdExpc3RJdGVtUg'
    'VpdGVtcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use getChatRequestDescriptor instead')
const GetChatRequest$json = {
  '1': 'GetChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `GetChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatRequestDescriptor = $convert
    .base64Decode('Cg5HZXRDaGF0UmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQ=');

@$core.Deprecated('Use listFoldersRequestDescriptor instead')
const ListFoldersRequest$json = {
  '1': 'ListFoldersRequest',
};

/// Descriptor for `ListFoldersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFoldersRequestDescriptor =
    $convert.base64Decode('ChJMaXN0Rm9sZGVyc1JlcXVlc3Q=');

@$core.Deprecated('Use folderListDescriptor instead')
const FolderList$json = {
  '1': 'FolderList',
  '2': [
    {
      '1': 'folders',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.Folder',
      '10': 'folders'
    },
  ],
};

/// Descriptor for `FolderList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List folderListDescriptor = $convert.base64Decode(
    'CgpGb2xkZXJMaXN0Ei8KB2ZvbGRlcnMYASADKAsyFS52b2ljZS5jaGF0LnYxLkZvbGRlclIHZm'
    '9sZGVycw==');

@$core.Deprecated('Use folderDescriptor instead')
const Folder$json = {
  '1': 'Folder',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'folder_type', '3': 3, '4': 1, '5': 9, '10': 'folderType'},
    {
      '1': 'filter_config_json',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'filterConfigJson'
    },
    {'1': 'sort_order', '3': 5, '4': 1, '5': 5, '10': 'sortOrder'},
  ],
};

/// Descriptor for `Folder`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List folderDescriptor = $convert.base64Decode(
    'CgZGb2xkZXISDgoCaWQYASABKAlSAmlkEhIKBG5hbWUYAiABKAlSBG5hbWUSHwoLZm9sZGVyX3'
    'R5cGUYAyABKAlSCmZvbGRlclR5cGUSLAoSZmlsdGVyX2NvbmZpZ19qc29uGAQgASgJUhBmaWx0'
    'ZXJDb25maWdKc29uEh0KCnNvcnRfb3JkZXIYBSABKAVSCXNvcnRPcmRlcg==');

@$core.Deprecated('Use createFolderRequestDescriptor instead')
const CreateFolderRequest$json = {
  '1': 'CreateFolderRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {
      '1': 'filter_config_json',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'filterConfigJson'
    },
  ],
};

/// Descriptor for `CreateFolderRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createFolderRequestDescriptor = $convert.base64Decode(
    'ChNDcmVhdGVGb2xkZXJSZXF1ZXN0EhIKBG5hbWUYASABKAlSBG5hbWUSLAoSZmlsdGVyX2Nvbm'
    'ZpZ19qc29uGAIgASgJUhBmaWx0ZXJDb25maWdKc29u');

@$core.Deprecated('Use updateFolderRequestDescriptor instead')
const UpdateFolderRequest$json = {
  '1': 'UpdateFolderRequest',
  '2': [
    {'1': 'folder_id', '3': 1, '4': 1, '5': 9, '10': 'folderId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'filter_config_json',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'filterConfigJson',
      '17': true
    },
    {
      '1': 'sort_order',
      '3': 4,
      '4': 1,
      '5': 5,
      '9': 2,
      '10': 'sortOrder',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_filter_config_json'},
    {'1': '_sort_order'},
  ],
};

/// Descriptor for `UpdateFolderRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateFolderRequestDescriptor = $convert.base64Decode(
    'ChNVcGRhdGVGb2xkZXJSZXF1ZXN0EhsKCWZvbGRlcl9pZBgBIAEoCVIIZm9sZGVySWQSFwoEbm'
    'FtZRgCIAEoCUgAUgRuYW1liAEBEjEKEmZpbHRlcl9jb25maWdfanNvbhgDIAEoCUgBUhBmaWx0'
    'ZXJDb25maWdKc29uiAEBEiIKCnNvcnRfb3JkZXIYBCABKAVIAlIJc29ydE9yZGVyiAEBQgcKBV'
    '9uYW1lQhUKE19maWx0ZXJfY29uZmlnX2pzb25CDQoLX3NvcnRfb3JkZXI=');

@$core.Deprecated('Use deleteFolderRequestDescriptor instead')
const DeleteFolderRequest$json = {
  '1': 'DeleteFolderRequest',
  '2': [
    {'1': 'folder_id', '3': 1, '4': 1, '5': 9, '10': 'folderId'},
  ],
};

/// Descriptor for `DeleteFolderRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteFolderRequestDescriptor =
    $convert.base64Decode(
        'ChNEZWxldGVGb2xkZXJSZXF1ZXN0EhsKCWZvbGRlcl9pZBgBIAEoCVIIZm9sZGVySWQ=');

@$core.Deprecated('Use acceptDMRequestRequestDescriptor instead')
const AcceptDMRequestRequest$json = {
  '1': 'AcceptDMRequestRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `AcceptDMRequestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptDMRequestRequestDescriptor =
    $convert.base64Decode(
        'ChZBY2NlcHRETVJlcXVlc3RSZXF1ZXN0EhcKB2NoYXRfaWQYASABKAlSBmNoYXRJZA==');

@$core.Deprecated('Use declineDMRequestRequestDescriptor instead')
const DeclineDMRequestRequest$json = {
  '1': 'DeclineDMRequestRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `DeclineDMRequestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineDMRequestRequestDescriptor =
    $convert.base64Decode(
        'ChdEZWNsaW5lRE1SZXF1ZXN0UmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQ=');

@$core.Deprecated('Use muteChatRequestDescriptor instead')
const MuteChatRequest$json = {
  '1': 'MuteChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {
      '1': 'muted_until',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'mutedUntil',
      '17': true
    },
  ],
  '8': [
    {'1': '_muted_until'},
  ],
};

/// Descriptor for `MuteChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List muteChatRequestDescriptor = $convert.base64Decode(
    'Cg9NdXRlQ2hhdFJlcXVlc3QSFwoHY2hhdF9pZBgBIAEoCVIGY2hhdElkEkAKC211dGVkX3VudG'
    'lsGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgAUgptdXRlZFVudGlsiAEBQg4K'
    'DF9tdXRlZF91bnRpbA==');

@$core.Deprecated('Use archiveChatRequestDescriptor instead')
const ArchiveChatRequest$json = {
  '1': 'ArchiveChatRequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
    {'1': 'archived', '3': 2, '4': 1, '5': 8, '10': 'archived'},
  ],
};

/// Descriptor for `ArchiveChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List archiveChatRequestDescriptor = $convert.base64Decode(
    'ChJBcmNoaXZlQ2hhdFJlcXVlc3QSFwoHY2hhdF9pZBgBIAEoCVIGY2hhdElkEhoKCGFyY2hpdm'
    'VkGAIgASgIUghhcmNoaXZlZA==');

@$core.Deprecated('Use enableChatE2ERequestDescriptor instead')
const EnableChatE2ERequest$json = {
  '1': 'EnableChatE2ERequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `EnableChatE2ERequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List enableChatE2ERequestDescriptor =
    $convert.base64Decode(
        'ChRFbmFibGVDaGF0RTJFUmVxdWVzdBIXCgdjaGF0X2lkGAEgASgJUgZjaGF0SWQ=');

@$core.Deprecated('Use disableChatE2ERequestDescriptor instead')
const DisableChatE2ERequest$json = {
  '1': 'DisableChatE2ERequest',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `DisableChatE2ERequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List disableChatE2ERequestDescriptor =
    $convert.base64Decode(
        'ChVEaXNhYmxlQ2hhdEUyRVJlcXVlc3QSFwoHY2hhdF9pZBgBIAEoCVIGY2hhdElk');

@$core.Deprecated('Use chatRefDescriptor instead')
const ChatRef$json = {
  '1': 'ChatRef',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'type',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.chat.v1.ChatType',
      '9': 0,
      '10': 'type',
      '17': true
    },
  ],
  '8': [
    {'1': '_type'},
  ],
};

/// Descriptor for `ChatRef`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatRefDescriptor = $convert.base64Decode(
    'CgdDaGF0UmVmEg4KAmlkGAEgASgJUgJpZBIwCgR0eXBlGAIgASgOMhcudm9pY2UuY2hhdC52MS'
    '5DaGF0VHlwZUgAUgR0eXBliAEBQgcKBV90eXBl');

@$core.Deprecated('Use createDMResponseDescriptor instead')
const CreateDMResponse$json = {
  '1': 'CreateDMResponse',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `CreateDMResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createDMResponseDescriptor = $convert.base64Decode(
    'ChBDcmVhdGVETVJlc3BvbnNlEicKBGNoYXQYASABKAsyEy52b2ljZS5jaGF0LnYxLkNoYXRSBG'
    'NoYXQ=');

@$core.Deprecated('Use getDMResponseDescriptor instead')
const GetDMResponse$json = {
  '1': 'GetDMResponse',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `GetDMResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDMResponseDescriptor = $convert.base64Decode(
    'Cg1HZXRETVJlc3BvbnNlEicKBGNoYXQYASABKAsyEy52b2ljZS5jaGF0LnYxLkNoYXRSBGNoYX'
    'Q=');

@$core.Deprecated('Use createChatResponseDescriptor instead')
const CreateChatResponse$json = {
  '1': 'CreateChatResponse',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `CreateChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createChatResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVDaGF0UmVzcG9uc2USJwoEY2hhdBgBIAEoCzITLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'IEY2hhdA==');

@$core.Deprecated('Use updateChatResponseDescriptor instead')
const UpdateChatResponse$json = {
  '1': 'UpdateChatResponse',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `UpdateChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateChatResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVDaGF0UmVzcG9uc2USJwoEY2hhdBgBIAEoCzITLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'IEY2hhdA==');

@$core.Deprecated('Use deleteChatResponseDescriptor instead')
const DeleteChatResponse$json = {
  '1': 'DeleteChatResponse',
};

/// Descriptor for `DeleteChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteChatResponseDescriptor =
    $convert.base64Decode('ChJEZWxldGVDaGF0UmVzcG9uc2U=');

@$core.Deprecated('Use addMembersResponseDescriptor instead')
const AddMembersResponse$json = {
  '1': 'AddMembersResponse',
};

/// Descriptor for `AddMembersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addMembersResponseDescriptor =
    $convert.base64Decode('ChJBZGRNZW1iZXJzUmVzcG9uc2U=');

@$core.Deprecated('Use removeMemberResponseDescriptor instead')
const RemoveMemberResponse$json = {
  '1': 'RemoveMemberResponse',
};

/// Descriptor for `RemoveMemberResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeMemberResponseDescriptor =
    $convert.base64Decode('ChRSZW1vdmVNZW1iZXJSZXNwb25zZQ==');

@$core.Deprecated('Use leaveChatResponseDescriptor instead')
const LeaveChatResponse$json = {
  '1': 'LeaveChatResponse',
};

/// Descriptor for `LeaveChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveChatResponseDescriptor =
    $convert.base64Decode('ChFMZWF2ZUNoYXRSZXNwb25zZQ==');

@$core.Deprecated('Use listMembersResponseDescriptor instead')
const ListMembersResponse$json = {
  '1': 'ListMembersResponse',
  '2': [
    {
      '1': 'member_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.MemberList',
      '10': 'memberList'
    },
  ],
};

/// Descriptor for `ListMembersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listMembersResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0TWVtYmVyc1Jlc3BvbnNlEjoKC21lbWJlcl9saXN0GAEgASgLMhkudm9pY2UuY2hhdC'
    '52MS5NZW1iZXJMaXN0UgptZW1iZXJMaXN0');

@$core.Deprecated('Use listChatsResponseDescriptor instead')
const ListChatsResponse$json = {
  '1': 'ListChatsResponse',
  '2': [
    {
      '1': 'chat_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatList',
      '10': 'chatList'
    },
  ],
};

/// Descriptor for `ListChatsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listChatsResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0Q2hhdHNSZXNwb25zZRI0CgljaGF0X2xpc3QYASABKAsyFy52b2ljZS5jaGF0LnYxLk'
    'NoYXRMaXN0UghjaGF0TGlzdA==');

@$core.Deprecated('Use getChatResponseDescriptor instead')
const GetChatResponse$json = {
  '1': 'GetChatResponse',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Chat',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `GetChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatResponseDescriptor = $convert.base64Decode(
    'Cg9HZXRDaGF0UmVzcG9uc2USJwoEY2hhdBgBIAEoCzITLnZvaWNlLmNoYXQudjEuQ2hhdFIEY2'
    'hhdA==');

@$core.Deprecated('Use listFoldersResponseDescriptor instead')
const ListFoldersResponse$json = {
  '1': 'ListFoldersResponse',
  '2': [
    {
      '1': 'folder_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.FolderList',
      '10': 'folderList'
    },
  ],
};

/// Descriptor for `ListFoldersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listFoldersResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0Rm9sZGVyc1Jlc3BvbnNlEjoKC2ZvbGRlcl9saXN0GAEgASgLMhkudm9pY2UuY2hhdC'
    '52MS5Gb2xkZXJMaXN0Ugpmb2xkZXJMaXN0');

@$core.Deprecated('Use createFolderResponseDescriptor instead')
const CreateFolderResponse$json = {
  '1': 'CreateFolderResponse',
  '2': [
    {
      '1': 'folder',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Folder',
      '10': 'folder'
    },
  ],
};

/// Descriptor for `CreateFolderResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createFolderResponseDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVGb2xkZXJSZXNwb25zZRItCgZmb2xkZXIYASABKAsyFS52b2ljZS5jaGF0LnYxLk'
    'ZvbGRlclIGZm9sZGVy');

@$core.Deprecated('Use updateFolderResponseDescriptor instead')
const UpdateFolderResponse$json = {
  '1': 'UpdateFolderResponse',
  '2': [
    {
      '1': 'folder',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.Folder',
      '10': 'folder'
    },
  ],
};

/// Descriptor for `UpdateFolderResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateFolderResponseDescriptor = $convert.base64Decode(
    'ChRVcGRhdGVGb2xkZXJSZXNwb25zZRItCgZmb2xkZXIYASABKAsyFS52b2ljZS5jaGF0LnYxLk'
    'ZvbGRlclIGZm9sZGVy');

@$core.Deprecated('Use deleteFolderResponseDescriptor instead')
const DeleteFolderResponse$json = {
  '1': 'DeleteFolderResponse',
};

/// Descriptor for `DeleteFolderResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteFolderResponseDescriptor =
    $convert.base64Decode('ChREZWxldGVGb2xkZXJSZXNwb25zZQ==');

@$core.Deprecated('Use acceptDMRequestResponseDescriptor instead')
const AcceptDMRequestResponse$json = {
  '1': 'AcceptDMRequestResponse',
};

/// Descriptor for `AcceptDMRequestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptDMRequestResponseDescriptor =
    $convert.base64Decode('ChdBY2NlcHRETVJlcXVlc3RSZXNwb25zZQ==');

@$core.Deprecated('Use declineDMRequestResponseDescriptor instead')
const DeclineDMRequestResponse$json = {
  '1': 'DeclineDMRequestResponse',
};

/// Descriptor for `DeclineDMRequestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineDMRequestResponseDescriptor =
    $convert.base64Decode('ChhEZWNsaW5lRE1SZXF1ZXN0UmVzcG9uc2U=');

@$core.Deprecated('Use muteChatResponseDescriptor instead')
const MuteChatResponse$json = {
  '1': 'MuteChatResponse',
};

/// Descriptor for `MuteChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List muteChatResponseDescriptor =
    $convert.base64Decode('ChBNdXRlQ2hhdFJlc3BvbnNl');

@$core.Deprecated('Use archiveChatResponseDescriptor instead')
const ArchiveChatResponse$json = {
  '1': 'ArchiveChatResponse',
};

/// Descriptor for `ArchiveChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List archiveChatResponseDescriptor =
    $convert.base64Decode('ChNBcmNoaXZlQ2hhdFJlc3BvbnNl');

@$core.Deprecated('Use enableChatE2EResponseDescriptor instead')
const EnableChatE2EResponse$json = {
  '1': 'EnableChatE2EResponse',
};

/// Descriptor for `EnableChatE2EResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List enableChatE2EResponseDescriptor =
    $convert.base64Decode('ChVFbmFibGVDaGF0RTJFUmVzcG9uc2U=');

@$core.Deprecated('Use disableChatE2EResponseDescriptor instead')
const DisableChatE2EResponse$json = {
  '1': 'DisableChatE2EResponse',
};

/// Descriptor for `DisableChatE2EResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List disableChatE2EResponseDescriptor =
    $convert.base64Decode('ChZEaXNhYmxlQ2hhdEUyRVJlc3BvbnNl');
