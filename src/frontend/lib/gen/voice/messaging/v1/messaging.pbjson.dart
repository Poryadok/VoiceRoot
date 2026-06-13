// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/messaging.proto.

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

@$core.Deprecated('Use messageKindDescriptor instead')
const MessageKind$json = {
  '1': 'MessageKind',
  '2': [
    {'1': 'MESSAGE_KIND_UNSPECIFIED', '2': 0},
    {'1': 'MESSAGE_KIND_REGULAR', '2': 1},
    {'1': 'MESSAGE_KIND_SYSTEM', '2': 2},
    {'1': 'MESSAGE_KIND_FORWARD', '2': 3},
  ],
};

/// Descriptor for `MessageKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List messageKindDescriptor = $convert.base64Decode(
    'CgtNZXNzYWdlS2luZBIcChhNRVNTQUdFX0tJTkRfVU5TUEVDSUZJRUQQABIYChRNRVNTQUdFX0'
    'tJTkRfUkVHVUxBUhABEhcKE01FU1NBR0VfS0lORF9TWVNURU0QAhIYChRNRVNTQUdFX0tJTkRf'
    'Rk9SV0FSRBAD');

@$core.Deprecated('Use deleteScopeDescriptor instead')
const DeleteScope$json = {
  '1': 'DeleteScope',
  '2': [
    {'1': 'DELETE_SCOPE_UNSPECIFIED', '2': 0},
    {'1': 'DELETE_SCOPE_FOR_EVERYONE', '2': 1},
    {'1': 'DELETE_SCOPE_FOR_ME', '2': 2},
  ],
};

/// Descriptor for `DeleteScope`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List deleteScopeDescriptor = $convert.base64Decode(
    'CgtEZWxldGVTY29wZRIcChhERUxFVEVfU0NPUEVfVU5TUEVDSUZJRUQQABIdChlERUxFVEVfU0'
    'NPUEVfRk9SX0VWRVJZT05FEAESFwoTREVMRVRFX1NDT1BFX0ZPUl9NRRAC');

@$core.Deprecated('Use sharedMediaKindDescriptor instead')
const SharedMediaKind$json = {
  '1': 'SharedMediaKind',
  '2': [
    {'1': 'SHARED_MEDIA_KIND_UNSPECIFIED', '2': 0},
    {'1': 'SHARED_MEDIA_KIND_MEDIA', '2': 1},
    {'1': 'SHARED_MEDIA_KIND_FILES', '2': 2},
    {'1': 'SHARED_MEDIA_KIND_LINKS', '2': 3},
    {'1': 'SHARED_MEDIA_KIND_VOICE', '2': 4},
  ],
};

/// Descriptor for `SharedMediaKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List sharedMediaKindDescriptor = $convert.base64Decode(
    'Cg9TaGFyZWRNZWRpYUtpbmQSIQodU0hBUkVEX01FRElBX0tJTkRfVU5TUEVDSUZJRUQQABIbCh'
    'dTSEFSRURfTUVESUFfS0lORF9NRURJQRABEhsKF1NIQVJFRF9NRURJQV9LSU5EX0ZJTEVTEAIS'
    'GwoXU0hBUkVEX01FRElBX0tJTkRfTElOS1MQAxIbChdTSEFSRURfTUVESUFfS0lORF9WT0lDRR'
    'AE');

@$core.Deprecated('Use sendMessageRequestDescriptor instead')
const SendMessageRequest$json = {
  '1': 'SendMessageRequest',
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
    {
      '1': 'client_message_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'clientMessageId',
      '17': true
    },
    {'1': 'attachments_json', '3': 4, '4': 1, '5': 9, '10': 'attachmentsJson'},
    {'1': 'mentions_json', '3': 5, '4': 1, '5': 9, '10': 'mentionsJson'},
    {
      '1': 'thread_parent_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'threadParentId',
      '17': true
    },
    {
      '1': 'message_kind',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.messaging.v1.MessageKind',
      '9': 2,
      '10': 'messageKind',
      '17': true
    },
    {
      '1': 'posted_as_chat',
      '3': 8,
      '4': 1,
      '5': 8,
      '9': 3,
      '10': 'postedAsChat',
      '17': true
    },
  ],
  '8': [
    {'1': '_client_message_id'},
    {'1': '_thread_parent_id'},
    {'1': '_message_kind'},
    {'1': '_posted_as_chat'},
  ],
};

/// Descriptor for `SendMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageRequestDescriptor = $convert.base64Decode(
    'ChJTZW5kTWVzc2FnZVJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZlIEY2hhdBIYCgdjb250ZW50GAIgASgJUgdjb250ZW50Ei8KEWNsaWVudF9tZXNzYWdlX2lk'
    'GAMgASgJSABSD2NsaWVudE1lc3NhZ2VJZIgBARIpChBhdHRhY2htZW50c19qc29uGAQgASgJUg'
    '9hdHRhY2htZW50c0pzb24SIwoNbWVudGlvbnNfanNvbhgFIAEoCVIMbWVudGlvbnNKc29uEi0K'
    'EHRocmVhZF9wYXJlbnRfaWQYBiABKAlIAVIOdGhyZWFkUGFyZW50SWSIAQESRwoMbWVzc2FnZV'
    '9raW5kGAcgASgOMh8udm9pY2UubWVzc2FnaW5nLnYxLk1lc3NhZ2VLaW5kSAJSC21lc3NhZ2VL'
    'aW5kiAEBEikKDnBvc3RlZF9hc19jaGF0GAggASgISANSDHBvc3RlZEFzQ2hhdIgBAUIUChJfY2'
    'xpZW50X21lc3NhZ2VfaWRCEwoRX3RocmVhZF9wYXJlbnRfaWRCDwoNX21lc3NhZ2Vfa2luZEIR'
    'Cg9fcG9zdGVkX2FzX2NoYXQ=');

@$core.Deprecated('Use editMessageRequestDescriptor instead')
const EditMessageRequest$json = {
  '1': 'EditMessageRequest',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'content', '3': 2, '4': 1, '5': 9, '10': 'content'},
  ],
};

/// Descriptor for `EditMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List editMessageRequestDescriptor = $convert.base64Decode(
    'ChJFZGl0TWVzc2FnZVJlcXVlc3QSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEhgKB2'
    'NvbnRlbnQYAiABKAlSB2NvbnRlbnQ=');

@$core.Deprecated('Use deleteMessageRequestDescriptor instead')
const DeleteMessageRequest$json = {
  '1': 'DeleteMessageRequest',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {
      '1': 'scope',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.messaging.v1.DeleteScope',
      '9': 0,
      '10': 'scope',
      '17': true
    },
  ],
  '8': [
    {'1': '_scope'},
  ],
};

/// Descriptor for `DeleteMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteMessageRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVNZXNzYWdlUmVxdWVzdBIdCgptZXNzYWdlX2lkGAEgASgJUgltZXNzYWdlSWQSOg'
    'oFc2NvcGUYAiABKA4yHy52b2ljZS5tZXNzYWdpbmcudjEuRGVsZXRlU2NvcGVIAFIFc2NvcGWI'
    'AQFCCAoGX3Njb3Bl');

@$core.Deprecated('Use getMessagesRequestDescriptor instead')
const GetMessagesRequest$json = {
  '1': 'GetMessagesRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {
      '1': 'after_message_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'afterMessageId',
      '17': true
    },
    {
      '1': 'before_message_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'beforeMessageId',
      '17': true
    },
    {
      '1': 'last_message_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'lastMessageId',
      '17': true
    },
    {
      '1': 'page',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
  '8': [
    {'1': '_after_message_id'},
    {'1': '_before_message_id'},
    {'1': '_last_message_id'},
  ],
};

/// Descriptor for `GetMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMessagesRequestDescriptor = $convert.base64Decode(
    'ChJHZXRNZXNzYWdlc1JlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZlIEY2hhdBItChBhZnRlcl9tZXNzYWdlX2lkGAIgASgJSABSDmFmdGVyTWVzc2FnZUlkiAEB'
    'Ei8KEWJlZm9yZV9tZXNzYWdlX2lkGAMgASgJSAFSD2JlZm9yZU1lc3NhZ2VJZIgBARIrCg9sYX'
    'N0X21lc3NhZ2VfaWQYBCABKAlIAlINbGFzdE1lc3NhZ2VJZIgBARI2CgRwYWdlGAUgASgLMiIu'
    'dm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYWdlQhMKEV9hZnRlcl9tZXNzYW'
    'dlX2lkQhQKEl9iZWZvcmVfbWVzc2FnZV9pZEISChBfbGFzdF9tZXNzYWdlX2lk');

@$core.Deprecated('Use getMessageRequestDescriptor instead')
const GetMessageRequest$json = {
  '1': 'GetMessageRequest',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
  ],
};

/// Descriptor for `GetMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMessageRequestDescriptor = $convert.base64Decode(
    'ChFHZXRNZXNzYWdlUmVxdWVzdBIdCgptZXNzYWdlX2lkGAEgASgJUgltZXNzYWdlSWQ=');

@$core.Deprecated('Use messageListDescriptor instead')
const MessageList$json = {
  '1': 'MessageList',
  '2': [
    {
      '1': 'messages',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '10': 'messages'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
    {'1': 'has_more', '3': 3, '4': 1, '5': 8, '10': 'hasMore'},
    {
      '1': 'page',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageResponse',
      '9': 0,
      '10': 'page',
      '17': true
    },
  ],
  '8': [
    {'1': '_page'},
  ],
};

/// Descriptor for `MessageList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageListDescriptor = $convert.base64Decode(
    'CgtNZXNzYWdlTGlzdBI3CghtZXNzYWdlcxgBIAMoCzIbLnZvaWNlLm1lc3NhZ2luZy52MS5NZX'
    'NzYWdlUghtZXNzYWdlcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvchIZCghoYXNf'
    'bW9yZRgDIAEoCFIHaGFzTW9yZRI8CgRwYWdlGAQgASgLMiMudm9pY2UuY29tbW9uLnYxLkN1cn'
    'NvclBhZ2VSZXNwb25zZUgAUgRwYWdliAEBQgcKBV9wYWdl');

@$core.Deprecated('Use messageDescriptor instead')
const Message$json = {
  '1': 'Message',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'sender_profile_id', '3': 4, '4': 1, '5': 9, '10': 'senderProfileId'},
    {'1': 'posted_as_chat', '3': 5, '4': 1, '5': 8, '10': 'postedAsChat'},
    {
      '1': 'display_chat_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'displayChatId',
      '17': true
    },
    {'1': 'content', '3': 7, '4': 1, '5': 9, '10': 'content'},
    {'1': 'type', '3': 8, '4': 1, '5': 9, '10': 'type'},
    {
      '1': 'thread_parent_id',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'threadParentId',
      '17': true
    },
    {
      '1': 'forward_from_id',
      '3': 10,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'forwardFromId',
      '17': true
    },
    {
      '1': 'forward_from_sender',
      '3': 11,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'forwardFromSender',
      '17': true
    },
    {'1': 'attachments_json', '3': 12, '4': 1, '5': 9, '10': 'attachmentsJson'},
    {'1': 'mentions_json', '3': 13, '4': 1, '5': 9, '10': 'mentionsJson'},
    {
      '1': 'edited_at',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 4,
      '10': 'editedAt',
      '17': true
    },
    {
      '1': 'deleted_at',
      '3': 15,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 5,
      '10': 'deletedAt',
      '17': true
    },
    {
      '1': 'created_at',
      '3': 16,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'message_kind',
      '3': 17,
      '4': 1,
      '5': 14,
      '6': '.voice.messaging.v1.MessageKind',
      '9': 6,
      '10': 'messageKind',
      '17': true
    },
    {'1': 'reactions_json', '3': 18, '4': 1, '5': 9, '10': 'reactionsJson'},
    {
      '1': 'is_pinned',
      '3': 19,
      '4': 1,
      '5': 8,
      '9': 7,
      '10': 'isPinned',
      '17': true
    },
  ],
  '8': [
    {'1': '_display_chat_id'},
    {'1': '_thread_parent_id'},
    {'1': '_forward_from_id'},
    {'1': '_forward_from_sender'},
    {'1': '_edited_at'},
    {'1': '_deleted_at'},
    {'1': '_message_kind'},
    {'1': '_is_pinned'},
  ],
  '9': [
    {'1': 3, '2': 4},
  ],
  '10': ['chat_type'],
};

/// Descriptor for `Message`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List messageDescriptor = $convert.base64Decode(
    'CgdNZXNzYWdlEg4KAmlkGAEgASgJUgJpZBIqCgRjaGF0GAIgASgLMhYudm9pY2UuY2hhdC52MS'
    '5DaGF0UmVmUgRjaGF0EioKEXNlbmRlcl9wcm9maWxlX2lkGAQgASgJUg9zZW5kZXJQcm9maWxl'
    'SWQSJAoOcG9zdGVkX2FzX2NoYXQYBSABKAhSDHBvc3RlZEFzQ2hhdBIrCg9kaXNwbGF5X2NoYX'
    'RfaWQYBiABKAlIAFINZGlzcGxheUNoYXRJZIgBARIYCgdjb250ZW50GAcgASgJUgdjb250ZW50'
    'EhIKBHR5cGUYCCABKAlSBHR5cGUSLQoQdGhyZWFkX3BhcmVudF9pZBgJIAEoCUgBUg50aHJlYW'
    'RQYXJlbnRJZIgBARIrCg9mb3J3YXJkX2Zyb21faWQYCiABKAlIAlINZm9yd2FyZEZyb21JZIgB'
    'ARIzChNmb3J3YXJkX2Zyb21fc2VuZGVyGAsgASgJSANSEWZvcndhcmRGcm9tU2VuZGVyiAEBEi'
    'kKEGF0dGFjaG1lbnRzX2pzb24YDCABKAlSD2F0dGFjaG1lbnRzSnNvbhIjCg1tZW50aW9uc19q'
    'c29uGA0gASgJUgxtZW50aW9uc0pzb24SPAoJZWRpdGVkX2F0GA4gASgLMhouZ29vZ2xlLnByb3'
    'RvYnVmLlRpbWVzdGFtcEgEUghlZGl0ZWRBdIgBARI+CgpkZWxldGVkX2F0GA8gASgLMhouZ29v'
    'Z2xlLnByb3RvYnVmLlRpbWVzdGFtcEgFUglkZWxldGVkQXSIAQESOQoKY3JlYXRlZF9hdBgQIA'
    'EoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWNyZWF0ZWRBdBJHCgxtZXNzYWdlX2tp'
    'bmQYESABKA4yHy52b2ljZS5tZXNzYWdpbmcudjEuTWVzc2FnZUtpbmRIBlILbWVzc2FnZUtpbm'
    'SIAQESJQoOcmVhY3Rpb25zX2pzb24YEiABKAlSDXJlYWN0aW9uc0pzb24SIAoJaXNfcGlubmVk'
    'GBMgASgISAdSCGlzUGlubmVkiAEBQhIKEF9kaXNwbGF5X2NoYXRfaWRCEwoRX3RocmVhZF9wYX'
    'JlbnRfaWRCEgoQX2ZvcndhcmRfZnJvbV9pZEIWChRfZm9yd2FyZF9mcm9tX3NlbmRlckIMCgpf'
    'ZWRpdGVkX2F0Qg0KC19kZWxldGVkX2F0Qg8KDV9tZXNzYWdlX2tpbmRCDAoKX2lzX3Bpbm5lZE'
    'oECAMQBFIJY2hhdF90eXBl');

@$core.Deprecated('Use getThreadMessagesRequestDescriptor instead')
const GetThreadMessagesRequest$json = {
  '1': 'GetThreadMessagesRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'thread_parent_id', '3': 2, '4': 1, '5': 9, '10': 'threadParentId'},
    {
      '1': 'page',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `GetThreadMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadMessagesRequestDescriptor = $convert.base64Decode(
    'ChhHZXRUaHJlYWRNZXNzYWdlc1JlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudj'
    'EuQ2hhdFJlZlIEY2hhdBIoChB0aHJlYWRfcGFyZW50X2lkGAIgASgJUg50aHJlYWRQYXJlbnRJ'
    'ZBI2CgRwYWdlGAMgASgLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYW'
    'dl');

@$core.Deprecated('Use threadSummaryDescriptor instead')
const ThreadSummary$json = {
  '1': 'ThreadSummary',
  '2': [
    {'1': 'thread_parent_id', '3': 1, '4': 1, '5': 9, '10': 'threadParentId'},
    {'1': 'reply_count', '3': 2, '4': 1, '5': 5, '10': 'replyCount'},
    {
      '1': 'last_reply_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'lastReplyAt',
      '17': true
    },
    {
      '1': 'last_reply_preview',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'lastReplyPreview',
      '17': true
    },
  ],
  '8': [
    {'1': '_last_reply_at'},
    {'1': '_last_reply_preview'},
  ],
};

/// Descriptor for `ThreadSummary`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadSummaryDescriptor = $convert.base64Decode(
    'Cg1UaHJlYWRTdW1tYXJ5EigKEHRocmVhZF9wYXJlbnRfaWQYASABKAlSDnRocmVhZFBhcmVudE'
    'lkEh8KC3JlcGx5X2NvdW50GAIgASgFUgpyZXBseUNvdW50EkMKDWxhc3RfcmVwbHlfYXQYAyAB'
    'KAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSABSC2xhc3RSZXBseUF0iAEBEjEKEmxhc3'
    'RfcmVwbHlfcHJldmlldxgEIAEoCUgBUhBsYXN0UmVwbHlQcmV2aWV3iAEBQhAKDl9sYXN0X3Jl'
    'cGx5X2F0QhUKE19sYXN0X3JlcGx5X3ByZXZpZXc=');

@$core.Deprecated('Use listThreadsRequestDescriptor instead')
const ListThreadsRequest$json = {
  '1': 'ListThreadsRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
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

/// Descriptor for `ListThreadsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadsRequestDescriptor = $convert.base64Decode(
    'ChJMaXN0VGhyZWFkc1JlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZlIEY2hhdBI2CgRwYWdlGAIgASgLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1'
    'ZXN0UgRwYWdl');

@$core.Deprecated('Use threadListDescriptor instead')
const ThreadList$json = {
  '1': 'ThreadList',
  '2': [
    {
      '1': 'threads',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.messaging.v1.ThreadSummary',
      '10': 'threads'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `ThreadList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List threadListDescriptor = $convert.base64Decode(
    'CgpUaHJlYWRMaXN0EjsKB3RocmVhZHMYASADKAsyIS52b2ljZS5tZXNzYWdpbmcudjEuVGhyZW'
    'FkU3VtbWFyeVIHdGhyZWFkcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use addReactionRequestDescriptor instead')
const AddReactionRequest$json = {
  '1': 'AddReactionRequest',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'emoji', '3': 2, '4': 1, '5': 9, '10': 'emoji'},
  ],
};

/// Descriptor for `AddReactionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addReactionRequestDescriptor = $convert.base64Decode(
    'ChJBZGRSZWFjdGlvblJlcXVlc3QSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEhQKBW'
    'Vtb2ppGAIgASgJUgVlbW9qaQ==');

@$core.Deprecated('Use removeReactionRequestDescriptor instead')
const RemoveReactionRequest$json = {
  '1': 'RemoveReactionRequest',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'emoji', '3': 2, '4': 1, '5': 9, '10': 'emoji'},
  ],
};

/// Descriptor for `RemoveReactionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeReactionRequestDescriptor = $convert.base64Decode(
    'ChVSZW1vdmVSZWFjdGlvblJlcXVlc3QSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEh'
    'QKBWVtb2ppGAIgASgJUgVlbW9qaQ==');

@$core.Deprecated('Use pinMessageRequestDescriptor instead')
const PinMessageRequest$json = {
  '1': 'PinMessageRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'message_id', '3': 2, '4': 1, '5': 9, '10': 'messageId'},
  ],
};

/// Descriptor for `PinMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pinMessageRequestDescriptor = $convert.base64Decode(
    'ChFQaW5NZXNzYWdlUmVxdWVzdBIqCgRjaGF0GAEgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0Um'
    'VmUgRjaGF0Eh0KCm1lc3NhZ2VfaWQYAiABKAlSCW1lc3NhZ2VJZA==');

@$core.Deprecated('Use unpinMessageRequestDescriptor instead')
const UnpinMessageRequest$json = {
  '1': 'UnpinMessageRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'message_id', '3': 2, '4': 1, '5': 9, '10': 'messageId'},
  ],
};

/// Descriptor for `UnpinMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpinMessageRequestDescriptor = $convert.base64Decode(
    'ChNVbnBpbk1lc3NhZ2VSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLkNoYX'
    'RSZWZSBGNoYXQSHQoKbWVzc2FnZV9pZBgCIAEoCVIJbWVzc2FnZUlk');

@$core.Deprecated('Use getPinnedMessagesRequestDescriptor instead')
const GetPinnedMessagesRequest$json = {
  '1': 'GetPinnedMessagesRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `GetPinnedMessagesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPinnedMessagesRequestDescriptor =
    $convert.base64Decode(
        'ChhHZXRQaW5uZWRNZXNzYWdlc1JlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudj'
        'EuQ2hhdFJlZlIEY2hhdA==');

@$core.Deprecated('Use forwardMessageRequestDescriptor instead')
const ForwardMessageRequest$json = {
  '1': 'ForwardMessageRequest',
  '2': [
    {'1': 'source_message_id', '3': 1, '4': 1, '5': 9, '10': 'sourceMessageId'},
    {
      '1': 'target_chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'targetChat'
    },
    {
      '1': 'commentary',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'commentary',
      '17': true
    },
  ],
  '8': [
    {'1': '_commentary'},
  ],
};

/// Descriptor for `ForwardMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List forwardMessageRequestDescriptor = $convert.base64Decode(
    'ChVGb3J3YXJkTWVzc2FnZVJlcXVlc3QSKgoRc291cmNlX21lc3NhZ2VfaWQYASABKAlSD3NvdX'
    'JjZU1lc3NhZ2VJZBI3Cgt0YXJnZXRfY2hhdBgCIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJl'
    'ZlIKdGFyZ2V0Q2hhdBIjCgpjb21tZW50YXJ5GAMgASgJSABSCmNvbW1lbnRhcnmIAQFCDQoLX2'
    'NvbW1lbnRhcnk=');

@$core.Deprecated('Use markReadRequestDescriptor instead')
const MarkReadRequest$json = {
  '1': 'MarkReadRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {
      '1': 'last_read_message_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'lastReadMessageId'
    },
  ],
};

/// Descriptor for `MarkReadRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markReadRequestDescriptor = $convert.base64Decode(
    'Cg9NYXJrUmVhZFJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZl'
    'IEY2hhdBIvChRsYXN0X3JlYWRfbWVzc2FnZV9pZBgCIAEoCVIRbGFzdFJlYWRNZXNzYWdlSWQ=');

@$core.Deprecated('Use getReadStateRequestDescriptor instead')
const GetReadStateRequest$json = {
  '1': 'GetReadStateRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `GetReadStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReadStateRequestDescriptor = $convert.base64Decode(
    'ChNHZXRSZWFkU3RhdGVSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLkNoYX'
    'RSZWZSBGNoYXQ=');

@$core.Deprecated('Use readStateDescriptor instead')
const ReadState$json = {
  '1': 'ReadState',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'last_read_message_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'lastReadMessageId'
    },
    {
      '1': 'updated_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
  ],
};

/// Descriptor for `ReadState`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List readStateDescriptor = $convert.base64Decode(
    'CglSZWFkU3RhdGUSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZlIEY2hhdB'
    'IdCgpwcm9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSLwoUbGFzdF9yZWFkX21lc3NhZ2VfaWQY'
    'AyABKAlSEWxhc3RSZWFkTWVzc2FnZUlkEjkKCnVwZGF0ZWRfYXQYBCABKAsyGi5nb29nbGUucH'
    'JvdG9idWYuVGltZXN0YW1wUgl1cGRhdGVkQXQ=');

@$core.Deprecated('Use getBulkReadStateRequestDescriptor instead')
const GetBulkReadStateRequest$json = {
  '1': 'GetBulkReadStateRequest',
  '2': [
    {
      '1': 'chats',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chats'
    },
  ],
};

/// Descriptor for `GetBulkReadStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkReadStateRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRCdWxrUmVhZFN0YXRlUmVxdWVzdBIsCgVjaGF0cxgBIAMoCzIWLnZvaWNlLmNoYXQudj'
        'EuQ2hhdFJlZlIFY2hhdHM=');

@$core.Deprecated('Use getChatListMetadataRequestDescriptor instead')
const GetChatListMetadataRequest$json = {
  '1': 'GetChatListMetadataRequest',
  '2': [
    {
      '1': 'chats',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chats'
    },
  ],
};

/// Descriptor for `GetChatListMetadataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatListMetadataRequestDescriptor =
    $convert.base64Decode(
        'ChpHZXRDaGF0TGlzdE1ldGFkYXRhUmVxdWVzdBIsCgVjaGF0cxgBIAMoCzIWLnZvaWNlLmNoYX'
        'QudjEuQ2hhdFJlZlIFY2hhdHM=');

@$core.Deprecated('Use chatListMetadataDescriptor instead')
const ChatListMetadata$json = {
  '1': 'ChatListMetadata',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
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
    {
      '1': 'last_message_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'lastMessageAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_last_message_preview'},
    {'1': '_last_message_at'},
  ],
};

/// Descriptor for `ChatListMetadata`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatListMetadataDescriptor = $convert.base64Decode(
    'ChBDaGF0TGlzdE1ldGFkYXRhEioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZW'
    'ZSBGNoYXQSNQoUbGFzdF9tZXNzYWdlX3ByZXZpZXcYAiABKAlIAFISbGFzdE1lc3NhZ2VQcmV2'
    'aWV3iAEBEiEKDHVucmVhZF9jb3VudBgDIAEoA1ILdW5yZWFkQ291bnQSRwoPbGFzdF9tZXNzYW'
    'dlX2F0GAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgBUg1sYXN0TWVzc2FnZUF0'
    'iAEBQhcKFV9sYXN0X21lc3NhZ2VfcHJldmlld0ISChBfbGFzdF9tZXNzYWdlX2F0');

@$core.Deprecated('Use sendMessageResponseDescriptor instead')
const SendMessageResponse$json = {
  '1': 'SendMessageResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `SendMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendMessageResponseDescriptor = $convert.base64Decode(
    'ChNTZW5kTWVzc2FnZVJlc3BvbnNlEjUKB21lc3NhZ2UYASABKAsyGy52b2ljZS5tZXNzYWdpbm'
    'cudjEuTWVzc2FnZVIHbWVzc2FnZQ==');

@$core.Deprecated('Use editMessageResponseDescriptor instead')
const EditMessageResponse$json = {
  '1': 'EditMessageResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `EditMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List editMessageResponseDescriptor = $convert.base64Decode(
    'ChNFZGl0TWVzc2FnZVJlc3BvbnNlEjUKB21lc3NhZ2UYASABKAsyGy52b2ljZS5tZXNzYWdpbm'
    'cudjEuTWVzc2FnZVIHbWVzc2FnZQ==');

@$core.Deprecated('Use deleteMessageResponseDescriptor instead')
const DeleteMessageResponse$json = {
  '1': 'DeleteMessageResponse',
};

/// Descriptor for `DeleteMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteMessageResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVNZXNzYWdlUmVzcG9uc2U=');

@$core.Deprecated('Use getMessagesResponseDescriptor instead')
const GetMessagesResponse$json = {
  '1': 'GetMessagesResponse',
  '2': [
    {
      '1': 'message_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.MessageList',
      '10': 'messageList'
    },
  ],
};

/// Descriptor for `GetMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMessagesResponseDescriptor = $convert.base64Decode(
    'ChNHZXRNZXNzYWdlc1Jlc3BvbnNlEkIKDG1lc3NhZ2VfbGlzdBgBIAEoCzIfLnZvaWNlLm1lc3'
    'NhZ2luZy52MS5NZXNzYWdlTGlzdFILbWVzc2FnZUxpc3Q=');

@$core.Deprecated('Use getMessageResponseDescriptor instead')
const GetMessageResponse$json = {
  '1': 'GetMessageResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `GetMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMessageResponseDescriptor = $convert.base64Decode(
    'ChJHZXRNZXNzYWdlUmVzcG9uc2USNQoHbWVzc2FnZRgBIAEoCzIbLnZvaWNlLm1lc3NhZ2luZy'
    '52MS5NZXNzYWdlUgdtZXNzYWdl');

@$core.Deprecated('Use getThreadMessagesResponseDescriptor instead')
const GetThreadMessagesResponse$json = {
  '1': 'GetThreadMessagesResponse',
  '2': [
    {
      '1': 'message_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.MessageList',
      '10': 'messageList'
    },
  ],
};

/// Descriptor for `GetThreadMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getThreadMessagesResponseDescriptor =
    $convert.base64Decode(
        'ChlHZXRUaHJlYWRNZXNzYWdlc1Jlc3BvbnNlEkIKDG1lc3NhZ2VfbGlzdBgBIAEoCzIfLnZvaW'
        'NlLm1lc3NhZ2luZy52MS5NZXNzYWdlTGlzdFILbWVzc2FnZUxpc3Q=');

@$core.Deprecated('Use listThreadsResponseDescriptor instead')
const ListThreadsResponse$json = {
  '1': 'ListThreadsResponse',
  '2': [
    {
      '1': 'thread_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.ThreadList',
      '10': 'threadList'
    },
  ],
};

/// Descriptor for `ListThreadsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listThreadsResponseDescriptor = $convert.base64Decode(
    'ChNMaXN0VGhyZWFkc1Jlc3BvbnNlEj8KC3RocmVhZF9saXN0GAEgASgLMh4udm9pY2UubWVzc2'
    'FnaW5nLnYxLlRocmVhZExpc3RSCnRocmVhZExpc3Q=');

@$core.Deprecated('Use addReactionResponseDescriptor instead')
const AddReactionResponse$json = {
  '1': 'AddReactionResponse',
};

/// Descriptor for `AddReactionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List addReactionResponseDescriptor =
    $convert.base64Decode('ChNBZGRSZWFjdGlvblJlc3BvbnNl');

@$core.Deprecated('Use removeReactionResponseDescriptor instead')
const RemoveReactionResponse$json = {
  '1': 'RemoveReactionResponse',
};

/// Descriptor for `RemoveReactionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List removeReactionResponseDescriptor =
    $convert.base64Decode('ChZSZW1vdmVSZWFjdGlvblJlc3BvbnNl');

@$core.Deprecated('Use pinMessageResponseDescriptor instead')
const PinMessageResponse$json = {
  '1': 'PinMessageResponse',
};

/// Descriptor for `PinMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pinMessageResponseDescriptor =
    $convert.base64Decode('ChJQaW5NZXNzYWdlUmVzcG9uc2U=');

@$core.Deprecated('Use unpinMessageResponseDescriptor instead')
const UnpinMessageResponse$json = {
  '1': 'UnpinMessageResponse',
};

/// Descriptor for `UnpinMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unpinMessageResponseDescriptor =
    $convert.base64Decode('ChRVbnBpbk1lc3NhZ2VSZXNwb25zZQ==');

@$core.Deprecated('Use getPinnedMessagesResponseDescriptor instead')
const GetPinnedMessagesResponse$json = {
  '1': 'GetPinnedMessagesResponse',
  '2': [
    {
      '1': 'message_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.MessageList',
      '10': 'messageList'
    },
  ],
};

/// Descriptor for `GetPinnedMessagesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPinnedMessagesResponseDescriptor =
    $convert.base64Decode(
        'ChlHZXRQaW5uZWRNZXNzYWdlc1Jlc3BvbnNlEkIKDG1lc3NhZ2VfbGlzdBgBIAEoCzIfLnZvaW'
        'NlLm1lc3NhZ2luZy52MS5NZXNzYWdlTGlzdFILbWVzc2FnZUxpc3Q=');

@$core.Deprecated('Use forwardMessageResponseDescriptor instead')
const ForwardMessageResponse$json = {
  '1': 'ForwardMessageResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '10': 'message'
    },
  ],
};

/// Descriptor for `ForwardMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List forwardMessageResponseDescriptor =
    $convert.base64Decode(
        'ChZGb3J3YXJkTWVzc2FnZVJlc3BvbnNlEjUKB21lc3NhZ2UYASABKAsyGy52b2ljZS5tZXNzYW'
        'dpbmcudjEuTWVzc2FnZVIHbWVzc2FnZQ==');

@$core.Deprecated('Use markReadResponseDescriptor instead')
const MarkReadResponse$json = {
  '1': 'MarkReadResponse',
};

/// Descriptor for `MarkReadResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markReadResponseDescriptor =
    $convert.base64Decode('ChBNYXJrUmVhZFJlc3BvbnNl');

@$core.Deprecated('Use getReadStateResponseDescriptor instead')
const GetReadStateResponse$json = {
  '1': 'GetReadStateResponse',
  '2': [
    {
      '1': 'read_state',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.ReadState',
      '10': 'readState'
    },
  ],
};

/// Descriptor for `GetReadStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getReadStateResponseDescriptor = $convert.base64Decode(
    'ChRHZXRSZWFkU3RhdGVSZXNwb25zZRI8CgpyZWFkX3N0YXRlGAEgASgLMh0udm9pY2UubWVzc2'
    'FnaW5nLnYxLlJlYWRTdGF0ZVIJcmVhZFN0YXRl');

@$core.Deprecated('Use getBulkReadStateResponseDescriptor instead')
const GetBulkReadStateResponse$json = {
  '1': 'GetBulkReadStateResponse',
  '2': [
    {
      '1': 'by_chat_id',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.messaging.v1.GetBulkReadStateResponse.ByChatIdEntry',
      '10': 'byChatId'
    },
  ],
  '3': [GetBulkReadStateResponse_ByChatIdEntry$json],
};

@$core.Deprecated('Use getBulkReadStateResponseDescriptor instead')
const GetBulkReadStateResponse_ByChatIdEntry$json = {
  '1': 'ByChatIdEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.ReadState',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `GetBulkReadStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBulkReadStateResponseDescriptor = $convert.base64Decode(
    'ChhHZXRCdWxrUmVhZFN0YXRlUmVzcG9uc2USWAoKYnlfY2hhdF9pZBgBIAMoCzI6LnZvaWNlLm'
    '1lc3NhZ2luZy52MS5HZXRCdWxrUmVhZFN0YXRlUmVzcG9uc2UuQnlDaGF0SWRFbnRyeVIIYnlD'
    'aGF0SWQaWgoNQnlDaGF0SWRFbnRyeRIQCgNrZXkYASABKAlSA2tleRIzCgV2YWx1ZRgCIAEoCz'
    'IdLnZvaWNlLm1lc3NhZ2luZy52MS5SZWFkU3RhdGVSBXZhbHVlOgI4AQ==');

@$core.Deprecated('Use getChatListMetadataResponseDescriptor instead')
const GetChatListMetadataResponse$json = {
  '1': 'GetChatListMetadataResponse',
  '2': [
    {
      '1': 'by_chat_id',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.messaging.v1.GetChatListMetadataResponse.ByChatIdEntry',
      '10': 'byChatId'
    },
  ],
  '3': [GetChatListMetadataResponse_ByChatIdEntry$json],
};

@$core.Deprecated('Use getChatListMetadataResponseDescriptor instead')
const GetChatListMetadataResponse_ByChatIdEntry$json = {
  '1': 'ByChatIdEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {
      '1': 'value',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.ChatListMetadata',
      '10': 'value'
    },
  ],
  '7': {'7': true},
};

/// Descriptor for `GetChatListMetadataResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatListMetadataResponseDescriptor = $convert.base64Decode(
    'ChtHZXRDaGF0TGlzdE1ldGFkYXRhUmVzcG9uc2USWwoKYnlfY2hhdF9pZBgBIAMoCzI9LnZvaW'
    'NlLm1lc3NhZ2luZy52MS5HZXRDaGF0TGlzdE1ldGFkYXRhUmVzcG9uc2UuQnlDaGF0SWRFbnRy'
    'eVIIYnlDaGF0SWQaYQoNQnlDaGF0SWRFbnRyeRIQCgNrZXkYASABKAlSA2tleRI6CgV2YWx1ZR'
    'gCIAEoCzIkLnZvaWNlLm1lc3NhZ2luZy52MS5DaGF0TGlzdE1ldGFkYXRhUgV2YWx1ZToCOAE=');

@$core.Deprecated('Use listSharedMediaRequestDescriptor instead')
const ListSharedMediaRequest$json = {
  '1': 'ListSharedMediaRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.messaging.v1.SharedMediaKind',
      '10': 'kind'
    },
    {
      '1': 'page',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListSharedMediaRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSharedMediaRequestDescriptor = $convert.base64Decode(
    'ChZMaXN0U2hhcmVkTWVkaWFSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLk'
    'NoYXRSZWZSBGNoYXQSNwoEa2luZBgCIAEoDjIjLnZvaWNlLm1lc3NhZ2luZy52MS5TaGFyZWRN'
    'ZWRpYUtpbmRSBGtpbmQSNgoEcGFnZRgDIAEoCzIiLnZvaWNlLmNvbW1vbi52MS5DdXJzb3JQYW'
    'dlUmVxdWVzdFIEcGFnZQ==');

@$core.Deprecated('Use sharedMediaItemDescriptor instead')
const SharedMediaItem$json = {
  '1': 'SharedMediaItem',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'sender_profile_id', '3': 2, '4': 1, '5': 9, '10': 'senderProfileId'},
    {
      '1': 'created_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'file_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'fileId',
      '17': true
    },
    {
      '1': 'attachment_type',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'attachmentType',
      '17': true
    },
    {
      '1': 'external_url',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'externalUrl',
      '17': true
    },
    {'1': 'title', '3': 7, '4': 1, '5': 9, '9': 3, '10': 'title', '17': true},
    {'1': 'sort_order', '3': 8, '4': 1, '5': 5, '10': 'sortOrder'},
    {
      '1': 'original_name',
      '3': 9,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'originalName',
      '17': true
    },
    {
      '1': 'size_bytes',
      '3': 10,
      '4': 1,
      '5': 3,
      '9': 5,
      '10': 'sizeBytes',
      '17': true
    },
  ],
  '8': [
    {'1': '_file_id'},
    {'1': '_attachment_type'},
    {'1': '_external_url'},
    {'1': '_title'},
    {'1': '_original_name'},
    {'1': '_size_bytes'},
  ],
};

/// Descriptor for `SharedMediaItem`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sharedMediaItemDescriptor = $convert.base64Decode(
    'Cg9TaGFyZWRNZWRpYUl0ZW0SHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEioKEXNlbm'
    'Rlcl9wcm9maWxlX2lkGAIgASgJUg9zZW5kZXJQcm9maWxlSWQSOQoKY3JlYXRlZF9hdBgDIAEo'
    'CzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWNyZWF0ZWRBdBIcCgdmaWxlX2lkGAQgAS'
    'gJSABSBmZpbGVJZIgBARIsCg9hdHRhY2htZW50X3R5cGUYBSABKAlIAVIOYXR0YWNobWVudFR5'
    'cGWIAQESJgoMZXh0ZXJuYWxfdXJsGAYgASgJSAJSC2V4dGVybmFsVXJsiAEBEhkKBXRpdGxlGA'
    'cgASgJSANSBXRpdGxliAEBEh0KCnNvcnRfb3JkZXIYCCABKAVSCXNvcnRPcmRlchIoCg1vcmln'
    'aW5hbF9uYW1lGAkgASgJSARSDG9yaWdpbmFsTmFtZYgBARIiCgpzaXplX2J5dGVzGAogASgDSA'
    'VSCXNpemVCeXRlc4gBAUIKCghfZmlsZV9pZEISChBfYXR0YWNobWVudF90eXBlQg8KDV9leHRl'
    'cm5hbF91cmxCCAoGX3RpdGxlQhAKDl9vcmlnaW5hbF9uYW1lQg0KC19zaXplX2J5dGVz');

@$core.Deprecated('Use sharedMediaListDescriptor instead')
const SharedMediaList$json = {
  '1': 'SharedMediaList',
  '2': [
    {
      '1': 'items',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.messaging.v1.SharedMediaItem',
      '10': 'items'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
    {'1': 'has_more', '3': 3, '4': 1, '5': 8, '10': 'hasMore'},
    {
      '1': 'page',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageResponse',
      '9': 0,
      '10': 'page',
      '17': true
    },
  ],
  '8': [
    {'1': '_page'},
  ],
};

/// Descriptor for `SharedMediaList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sharedMediaListDescriptor = $convert.base64Decode(
    'Cg9TaGFyZWRNZWRpYUxpc3QSOQoFaXRlbXMYASADKAsyIy52b2ljZS5tZXNzYWdpbmcudjEuU2'
    'hhcmVkTWVkaWFJdGVtUgVpdGVtcxIfCgtuZXh0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvchIZ'
    'CghoYXNfbW9yZRgDIAEoCFIHaGFzTW9yZRI8CgRwYWdlGAQgASgLMiMudm9pY2UuY29tbW9uLn'
    'YxLkN1cnNvclBhZ2VSZXNwb25zZUgAUgRwYWdliAEBQgcKBV9wYWdl');

@$core.Deprecated('Use listSharedMediaResponseDescriptor instead')
const ListSharedMediaResponse$json = {
  '1': 'ListSharedMediaResponse',
  '2': [
    {
      '1': 'shared_media_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.SharedMediaList',
      '10': 'sharedMediaList'
    },
  ],
};

/// Descriptor for `ListSharedMediaResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSharedMediaResponseDescriptor = $convert.base64Decode(
    'ChdMaXN0U2hhcmVkTWVkaWFSZXNwb25zZRJPChFzaGFyZWRfbWVkaWFfbGlzdBgBIAEoCzIjLn'
    'ZvaWNlLm1lc3NhZ2luZy52MS5TaGFyZWRNZWRpYUxpc3RSD3NoYXJlZE1lZGlhTGlzdA==');
