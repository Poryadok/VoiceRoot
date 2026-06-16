// This is a generated file - do not edit.
//
// Generated from voice/bot/v1/bot.proto.

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

@$core.Deprecated('Use botLifecycleStatusDescriptor instead')
const BotLifecycleStatus$json = {
  '1': 'BotLifecycleStatus',
  '2': [
    {'1': 'BOT_LIFECYCLE_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'BOT_LIFECYCLE_STATUS_DRAFT', '2': 1},
    {'1': 'BOT_LIFECYCLE_STATUS_LIVE', '2': 2},
    {'1': 'BOT_LIFECYCLE_STATUS_DISABLED', '2': 3},
  ],
};

/// Descriptor for `BotLifecycleStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List botLifecycleStatusDescriptor = $convert.base64Decode(
    'ChJCb3RMaWZlY3ljbGVTdGF0dXMSJAogQk9UX0xJRkVDWUNMRV9TVEFUVVNfVU5TUEVDSUZJRU'
    'QQABIeChpCT1RfTElGRUNZQ0xFX1NUQVRVU19EUkFGVBABEh0KGUJPVF9MSUZFQ1lDTEVfU1RB'
    'VFVTX0xJVkUQAhIhCh1CT1RfTElGRUNZQ0xFX1NUQVRVU19ESVNBQkxFRBAD');

@$core.Deprecated('Use botDescriptor instead')
const Bot$json = {
  '1': 'Bot',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'owner_account_id', '3': 2, '4': 1, '5': 9, '10': 'ownerAccountId'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'avatar_url',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'avatarUrl',
      '17': true
    },
    {
      '1': 'webhook_url',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'webhookUrl',
      '17': true
    },
    {'1': 'is_polling_mode', '3': 7, '4': 1, '5': 8, '10': 'isPollingMode'},
    {'1': 'scopes_json', '3': 8, '4': 1, '5': 9, '10': 'scopesJson'},
    {'1': 'status', '3': 9, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'created_at',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'status_enum',
      '3': 11,
      '4': 1,
      '5': 14,
      '6': '.voice.bot.v1.BotLifecycleStatus',
      '9': 2,
      '10': 'statusEnum',
      '17': true
    },
    {
      '1': 'actor_profile_id',
      '3': 12,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'actorProfileId',
      '17': true
    },
    {'1': 'slug', '3': 13, '4': 1, '5': 9, '9': 4, '10': 'slug', '17': true},
  ],
  '8': [
    {'1': '_avatar_url'},
    {'1': '_webhook_url'},
    {'1': '_status_enum'},
    {'1': '_actor_profile_id'},
    {'1': '_slug'},
  ],
};

/// Descriptor for `Bot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botDescriptor = $convert.base64Decode(
    'CgNCb3QSDgoCaWQYASABKAlSAmlkEigKEG93bmVyX2FjY291bnRfaWQYAiABKAlSDm93bmVyQW'
    'Njb3VudElkEhIKBG5hbWUYAyABKAlSBG5hbWUSIAoLZGVzY3JpcHRpb24YBCABKAlSC2Rlc2Ny'
    'aXB0aW9uEiIKCmF2YXRhcl91cmwYBSABKAlIAFIJYXZhdGFyVXJsiAEBEiQKC3dlYmhvb2tfdX'
    'JsGAYgASgJSAFSCndlYmhvb2tVcmyIAQESJgoPaXNfcG9sbGluZ19tb2RlGAcgASgIUg1pc1Bv'
    'bGxpbmdNb2RlEh8KC3Njb3Blc19qc29uGAggASgJUgpzY29wZXNKc29uEhYKBnN0YXR1cxgJIA'
    'EoCVIGc3RhdHVzEjkKCmNyZWF0ZWRfYXQYCiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0'
    'YW1wUgljcmVhdGVkQXQSRgoLc3RhdHVzX2VudW0YCyABKA4yIC52b2ljZS5ib3QudjEuQm90TG'
    'lmZWN5Y2xlU3RhdHVzSAJSCnN0YXR1c0VudW2IAQESLQoQYWN0b3JfcHJvZmlsZV9pZBgMIAEo'
    'CUgDUg5hY3RvclByb2ZpbGVJZIgBARIXCgRzbHVnGA0gASgJSARSBHNsdWeIAQFCDQoLX2F2YX'
    'Rhcl91cmxCDgoMX3dlYmhvb2tfdXJsQg4KDF9zdGF0dXNfZW51bUITChFfYWN0b3JfcHJvZmls'
    'ZV9pZEIHCgVfc2x1Zw==');

@$core.Deprecated('Use registerBotRequestDescriptor instead')
const RegisterBotRequest$json = {
  '1': 'RegisterBotRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 2, '4': 1, '5': 9, '10': 'description'},
    {'1': 'scopes_json', '3': 3, '4': 1, '5': 9, '10': 'scopesJson'},
  ],
};

/// Descriptor for `RegisterBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerBotRequestDescriptor = $convert.base64Decode(
    'ChJSZWdpc3RlckJvdFJlcXVlc3QSEgoEbmFtZRgBIAEoCVIEbmFtZRIgCgtkZXNjcmlwdGlvbh'
    'gCIAEoCVILZGVzY3JpcHRpb24SHwoLc2NvcGVzX2pzb24YAyABKAlSCnNjb3Blc0pzb24=');

@$core.Deprecated('Use updateBotRequestDescriptor instead')
const UpdateBotRequest$json = {
  '1': 'UpdateBotRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'description',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'description',
      '17': true
    },
    {
      '1': 'avatar_url',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'avatarUrl',
      '17': true
    },
    {
      '1': 'scopes_json',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'scopesJson',
      '17': true
    },
  ],
  '8': [
    {'1': '_name'},
    {'1': '_description'},
    {'1': '_avatar_url'},
    {'1': '_scopes_json'},
  ],
};

/// Descriptor for `UpdateBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateBotRequestDescriptor = $convert.base64Decode(
    'ChBVcGRhdGVCb3RSZXF1ZXN0EhUKBmJvdF9pZBgBIAEoCVIFYm90SWQSFwoEbmFtZRgCIAEoCU'
    'gAUgRuYW1liAEBEiUKC2Rlc2NyaXB0aW9uGAMgASgJSAFSC2Rlc2NyaXB0aW9uiAEBEiIKCmF2'
    'YXRhcl91cmwYBCABKAlIAlIJYXZhdGFyVXJsiAEBEiQKC3Njb3Blc19qc29uGAUgASgJSANSCn'
    'Njb3Blc0pzb26IAQFCBwoFX25hbWVCDgoMX2Rlc2NyaXB0aW9uQg0KC19hdmF0YXJfdXJsQg4K'
    'DF9zY29wZXNfanNvbg==');

@$core.Deprecated('Use deleteBotRequestDescriptor instead')
const DeleteBotRequest$json = {
  '1': 'DeleteBotRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `DeleteBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteBotRequestDescriptor = $convert
    .base64Decode('ChBEZWxldGVCb3RSZXF1ZXN0EhUKBmJvdF9pZBgBIAEoCVIFYm90SWQ=');

@$core.Deprecated('Use getBotRequestDescriptor instead')
const GetBotRequest$json = {
  '1': 'GetBotRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `GetBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBotRequestDescriptor = $convert
    .base64Decode('Cg1HZXRCb3RSZXF1ZXN0EhUKBmJvdF9pZBgBIAEoCVIFYm90SWQ=');

@$core.Deprecated('Use getBotBySlugRequestDescriptor instead')
const GetBotBySlugRequest$json = {
  '1': 'GetBotBySlugRequest',
  '2': [
    {'1': 'slug', '3': 1, '4': 1, '5': 9, '10': 'slug'},
  ],
};

/// Descriptor for `GetBotBySlugRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBotBySlugRequestDescriptor = $convert
    .base64Decode('ChNHZXRCb3RCeVNsdWdSZXF1ZXN0EhIKBHNsdWcYASABKAlSBHNsdWc=');

@$core.Deprecated('Use listBotsRequestDescriptor instead')
const ListBotsRequest$json = {
  '1': 'ListBotsRequest',
};

/// Descriptor for `ListBotsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBotsRequestDescriptor =
    $convert.base64Decode('Cg9MaXN0Qm90c1JlcXVlc3Q=');

@$core.Deprecated('Use botListDescriptor instead')
const BotList$json = {
  '1': 'BotList',
  '2': [
    {
      '1': 'bots',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bots'
    },
  ],
};

/// Descriptor for `BotList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botListDescriptor = $convert.base64Decode(
    'CgdCb3RMaXN0EiUKBGJvdHMYASADKAsyES52b2ljZS5ib3QudjEuQm90UgRib3Rz');

@$core.Deprecated('Use regenerateTokenRequestDescriptor instead')
const RegenerateTokenRequest$json = {
  '1': 'RegenerateTokenRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `RegenerateTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List regenerateTokenRequestDescriptor =
    $convert.base64Decode(
        'ChZSZWdlbmVyYXRlVG9rZW5SZXF1ZXN0EhUKBmJvdF9pZBgBIAEoCVIFYm90SWQ=');

@$core.Deprecated('Use tokenResponseDescriptor instead')
const TokenResponse$json = {
  '1': 'TokenResponse',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
  ],
};

/// Descriptor for `TokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tokenResponseDescriptor = $convert
    .base64Decode('Cg1Ub2tlblJlc3BvbnNlEhQKBXRva2VuGAEgASgJUgV0b2tlbg==');

@$core.Deprecated('Use registerCommandsRequestDescriptor instead')
const RegisterCommandsRequest$json = {
  '1': 'RegisterCommandsRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'commands_json', '3': 2, '4': 1, '5': 9, '10': 'commandsJson'},
  ],
};

/// Descriptor for `RegisterCommandsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerCommandsRequestDescriptor =
    $convert.base64Decode(
        'ChdSZWdpc3RlckNvbW1hbmRzUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEiMKDWNvbW'
        '1hbmRzX2pzb24YAiABKAlSDGNvbW1hbmRzSnNvbg==');

@$core.Deprecated('Use getCommandsRequestDescriptor instead')
const GetCommandsRequest$json = {
  '1': 'GetCommandsRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `GetCommandsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCommandsRequestDescriptor =
    $convert.base64Decode(
        'ChJHZXRDb21tYW5kc1JlcXVlc3QSFQoGYm90X2lkGAEgASgJUgVib3RJZA==');

@$core.Deprecated('Use commandListDescriptor instead')
const CommandList$json = {
  '1': 'CommandList',
  '2': [
    {'1': 'commands_json', '3': 1, '4': 1, '5': 9, '10': 'commandsJson'},
  ],
};

/// Descriptor for `CommandList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List commandListDescriptor = $convert.base64Decode(
    'CgtDb21tYW5kTGlzdBIjCg1jb21tYW5kc19qc29uGAEgASgJUgxjb21tYW5kc0pzb24=');

@$core.Deprecated('Use setWebhookURLRequestDescriptor instead')
const SetWebhookURLRequest$json = {
  '1': 'SetWebhookURLRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'url', '3': 2, '4': 1, '5': 9, '10': 'url'},
  ],
};

/// Descriptor for `SetWebhookURLRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setWebhookURLRequestDescriptor = $convert.base64Decode(
    'ChRTZXRXZWJob29rVVJMUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEhAKA3VybBgCIA'
    'EoCVIDdXJs');

@$core.Deprecated('Use getWebhookURLRequestDescriptor instead')
const GetWebhookURLRequest$json = {
  '1': 'GetWebhookURLRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `GetWebhookURLRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getWebhookURLRequestDescriptor =
    $convert.base64Decode(
        'ChRHZXRXZWJob29rVVJMUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElk');

@$core.Deprecated('Use setChatWhitelistRequestDescriptor instead')
const SetChatWhitelistRequest$json = {
  '1': 'SetChatWhitelistRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {
      '1': 'allowed_chats',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'allowedChats'
    },
  ],
};

/// Descriptor for `SetChatWhitelistRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatWhitelistRequestDescriptor = $convert.base64Decode(
    'ChdTZXRDaGF0V2hpdGVsaXN0UmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEjsKDWFsbG'
    '93ZWRfY2hhdHMYAiADKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSDGFsbG93ZWRDaGF0cw==');

@$core.Deprecated('Use getChatWhitelistRequestDescriptor instead')
const GetChatWhitelistRequest$json = {
  '1': 'GetChatWhitelistRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `GetChatWhitelistRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatWhitelistRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRDaGF0V2hpdGVsaXN0UmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElk');

@$core.Deprecated('Use sendBotMessageRequestDescriptor instead')
const SendBotMessageRequest$json = {
  '1': 'SendBotMessageRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'content', '3': 3, '4': 1, '5': 9, '10': 'content'},
    {
      '1': 'thread_parent_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'threadParentId',
      '17': true
    },
    {
      '1': 'interaction_token',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'interactionToken',
      '17': true
    },
  ],
  '8': [
    {'1': '_thread_parent_id'},
    {'1': '_interaction_token'},
  ],
};

/// Descriptor for `SendBotMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendBotMessageRequestDescriptor = $convert.base64Decode(
    'ChVTZW5kQm90TWVzc2FnZVJlcXVlc3QSFQoGYm90X2lkGAEgASgJUgVib3RJZBIqCgRjaGF0GA'
    'IgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmUgRjaGF0EhgKB2NvbnRlbnQYAyABKAlSB2Nv'
    'bnRlbnQSLQoQdGhyZWFkX3BhcmVudF9pZBgEIAEoCUgAUg50aHJlYWRQYXJlbnRJZIgBARIwCh'
    'FpbnRlcmFjdGlvbl90b2tlbhgFIAEoCUgBUhBpbnRlcmFjdGlvblRva2VuiAEBQhMKEV90aHJl'
    'YWRfcGFyZW50X2lkQhQKEl9pbnRlcmFjdGlvbl90b2tlbg==');

@$core.Deprecated('Use editBotMessageRequestDescriptor instead')
const EditBotMessageRequest$json = {
  '1': 'EditBotMessageRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'message_id', '3': 2, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'content', '3': 3, '4': 1, '5': 9, '10': 'content'},
  ],
};

/// Descriptor for `EditBotMessageRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List editBotMessageRequestDescriptor = $convert.base64Decode(
    'ChVFZGl0Qm90TWVzc2FnZVJlcXVlc3QSFQoGYm90X2lkGAEgASgJUgVib3RJZBIdCgptZXNzYW'
    'dlX2lkGAIgASgJUgltZXNzYWdlSWQSGAoHY29udGVudBgDIAEoCVIHY29udGVudA==');

@$core.Deprecated('Use sendEphemeralRequestDescriptor instead')
const SendEphemeralRequest$json = {
  '1': 'SendEphemeralRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'target_profile_id', '3': 3, '4': 1, '5': 9, '10': 'targetProfileId'},
    {'1': 'content', '3': 4, '4': 1, '5': 9, '10': 'content'},
  ],
};

/// Descriptor for `SendEphemeralRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendEphemeralRequestDescriptor = $convert.base64Decode(
    'ChRTZW5kRXBoZW1lcmFsUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEioKBGNoYXQYAi'
    'ABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZSBGNoYXQSKgoRdGFyZ2V0X3Byb2ZpbGVfaWQY'
    'AyABKAlSD3RhcmdldFByb2ZpbGVJZBIYCgdjb250ZW50GAQgASgJUgdjb250ZW50');

@$core.Deprecated('Use deferResponseRequestDescriptor instead')
const DeferResponseRequest$json = {
  '1': 'DeferResponseRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {
      '1': 'interaction_token',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'interactionToken'
    },
  ],
};

/// Descriptor for `DeferResponseRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deferResponseRequestDescriptor = $convert.base64Decode(
    'ChREZWZlclJlc3BvbnNlUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEisKEWludGVyYW'
    'N0aW9uX3Rva2VuGAIgASgJUhBpbnRlcmFjdGlvblRva2Vu');

@$core.Deprecated('Use pollEventsRequestDescriptor instead')
const PollEventsRequest$json = {
  '1': 'PollEventsRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'cursor', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'cursor', '17': true},
  ],
  '8': [
    {'1': '_cursor'},
  ],
};

/// Descriptor for `PollEventsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pollEventsRequestDescriptor = $convert.base64Decode(
    'ChFQb2xsRXZlbnRzUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEhsKBmN1cnNvchgCIA'
    'EoCUgAUgZjdXJzb3KIAQFCCQoHX2N1cnNvcg==');

@$core.Deprecated('Use botEventDescriptor instead')
const BotEvent$json = {
  '1': 'BotEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'event_type', '3': 2, '4': 1, '5': 9, '10': 'eventType'},
    {'1': 'payload_json', '3': 3, '4': 1, '5': 9, '10': 'payloadJson'},
    {
      '1': 'created_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
};

/// Descriptor for `BotEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List botEventDescriptor = $convert.base64Decode(
    'CghCb3RFdmVudBIZCghldmVudF9pZBgBIAEoCVIHZXZlbnRJZBIdCgpldmVudF90eXBlGAIgAS'
    'gJUglldmVudFR5cGUSIQoMcGF5bG9hZF9qc29uGAMgASgJUgtwYXlsb2FkSnNvbhI5CgpjcmVh'
    'dGVkX2F0GAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRlZEF0');

@$core.Deprecated('Use registerBotResponseDescriptor instead')
const RegisterBotResponse$json = {
  '1': 'RegisterBotResponse',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
    {
      '1': 'token_response',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.TokenResponse',
      '9': 0,
      '10': 'tokenResponse',
      '17': true
    },
  ],
  '8': [
    {'1': '_token_response'},
  ],
};

/// Descriptor for `RegisterBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerBotResponseDescriptor = $convert.base64Decode(
    'ChNSZWdpc3RlckJvdFJlc3BvbnNlEiMKA2JvdBgBIAEoCzIRLnZvaWNlLmJvdC52MS5Cb3RSA2'
    'JvdBJHCg50b2tlbl9yZXNwb25zZRgCIAEoCzIbLnZvaWNlLmJvdC52MS5Ub2tlblJlc3BvbnNl'
    'SABSDXRva2VuUmVzcG9uc2WIAQFCEQoPX3Rva2VuX3Jlc3BvbnNl');

@$core.Deprecated('Use updateBotResponseDescriptor instead')
const UpdateBotResponse$json = {
  '1': 'UpdateBotResponse',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
  ],
};

/// Descriptor for `UpdateBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateBotResponseDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVCb3RSZXNwb25zZRIjCgNib3QYASABKAsyES52b2ljZS5ib3QudjEuQm90UgNib3'
    'Q=');

@$core.Deprecated('Use deleteBotResponseDescriptor instead')
const DeleteBotResponse$json = {
  '1': 'DeleteBotResponse',
};

/// Descriptor for `DeleteBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteBotResponseDescriptor =
    $convert.base64Decode('ChFEZWxldGVCb3RSZXNwb25zZQ==');

@$core.Deprecated('Use getBotResponseDescriptor instead')
const GetBotResponse$json = {
  '1': 'GetBotResponse',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
  ],
};

/// Descriptor for `GetBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBotResponseDescriptor = $convert.base64Decode(
    'Cg5HZXRCb3RSZXNwb25zZRIjCgNib3QYASABKAsyES52b2ljZS5ib3QudjEuQm90UgNib3Q=');

@$core.Deprecated('Use listBotsResponseDescriptor instead')
const ListBotsResponse$json = {
  '1': 'ListBotsResponse',
  '2': [
    {
      '1': 'bot_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.BotList',
      '10': 'botList'
    },
  ],
};

/// Descriptor for `ListBotsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBotsResponseDescriptor = $convert.base64Decode(
    'ChBMaXN0Qm90c1Jlc3BvbnNlEjAKCGJvdF9saXN0GAEgASgLMhUudm9pY2UuYm90LnYxLkJvdE'
    'xpc3RSB2JvdExpc3Q=');

@$core.Deprecated('Use regenerateTokenResponseDescriptor instead')
const RegenerateTokenResponse$json = {
  '1': 'RegenerateTokenResponse',
  '2': [
    {
      '1': 'token_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.TokenResponse',
      '10': 'tokenResponse'
    },
  ],
};

/// Descriptor for `RegenerateTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List regenerateTokenResponseDescriptor =
    $convert.base64Decode(
        'ChdSZWdlbmVyYXRlVG9rZW5SZXNwb25zZRJCCg50b2tlbl9yZXNwb25zZRgBIAEoCzIbLnZvaW'
        'NlLmJvdC52MS5Ub2tlblJlc3BvbnNlUg10b2tlblJlc3BvbnNl');

@$core.Deprecated('Use registerCommandsResponseDescriptor instead')
const RegisterCommandsResponse$json = {
  '1': 'RegisterCommandsResponse',
};

/// Descriptor for `RegisterCommandsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerCommandsResponseDescriptor =
    $convert.base64Decode('ChhSZWdpc3RlckNvbW1hbmRzUmVzcG9uc2U=');

@$core.Deprecated('Use getCommandsResponseDescriptor instead')
const GetCommandsResponse$json = {
  '1': 'GetCommandsResponse',
  '2': [
    {
      '1': 'command_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.CommandList',
      '10': 'commandList'
    },
  ],
};

/// Descriptor for `GetCommandsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCommandsResponseDescriptor = $convert.base64Decode(
    'ChNHZXRDb21tYW5kc1Jlc3BvbnNlEjwKDGNvbW1hbmRfbGlzdBgBIAEoCzIZLnZvaWNlLmJvdC'
    '52MS5Db21tYW5kTGlzdFILY29tbWFuZExpc3Q=');

@$core.Deprecated('Use setWebhookURLResponseDescriptor instead')
const SetWebhookURLResponse$json = {
  '1': 'SetWebhookURLResponse',
};

/// Descriptor for `SetWebhookURLResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setWebhookURLResponseDescriptor =
    $convert.base64Decode('ChVTZXRXZWJob29rVVJMUmVzcG9uc2U=');

@$core.Deprecated('Use getWebhookURLResponseDescriptor instead')
const GetWebhookURLResponse$json = {
  '1': 'GetWebhookURLResponse',
  '2': [
    {'1': 'url', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'url', '17': true},
  ],
  '8': [
    {'1': '_url'},
  ],
};

/// Descriptor for `GetWebhookURLResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getWebhookURLResponseDescriptor = $convert.base64Decode(
    'ChVHZXRXZWJob29rVVJMUmVzcG9uc2USFQoDdXJsGAEgASgJSABSA3VybIgBAUIGCgRfdXJs');

@$core.Deprecated('Use setChatWhitelistResponseDescriptor instead')
const SetChatWhitelistResponse$json = {
  '1': 'SetChatWhitelistResponse',
};

/// Descriptor for `SetChatWhitelistResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setChatWhitelistResponseDescriptor =
    $convert.base64Decode('ChhTZXRDaGF0V2hpdGVsaXN0UmVzcG9uc2U=');

@$core.Deprecated('Use getChatWhitelistResponseDescriptor instead')
const GetChatWhitelistResponse$json = {
  '1': 'GetChatWhitelistResponse',
  '2': [
    {
      '1': 'allowed_chats',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'allowedChats'
    },
  ],
};

/// Descriptor for `GetChatWhitelistResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatWhitelistResponseDescriptor =
    $convert.base64Decode(
        'ChhHZXRDaGF0V2hpdGVsaXN0UmVzcG9uc2USOwoNYWxsb3dlZF9jaGF0cxgBIAMoCzIWLnZvaW'
        'NlLmNoYXQudjEuQ2hhdFJlZlIMYWxsb3dlZENoYXRz');

@$core.Deprecated('Use sendBotMessageResponseDescriptor instead')
const SendBotMessageResponse$json = {
  '1': 'SendBotMessageResponse',
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

/// Descriptor for `SendBotMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendBotMessageResponseDescriptor =
    $convert.base64Decode(
        'ChZTZW5kQm90TWVzc2FnZVJlc3BvbnNlEjUKB21lc3NhZ2UYASABKAsyGy52b2ljZS5tZXNzYW'
        'dpbmcudjEuTWVzc2FnZVIHbWVzc2FnZQ==');

@$core.Deprecated('Use editBotMessageResponseDescriptor instead')
const EditBotMessageResponse$json = {
  '1': 'EditBotMessageResponse',
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

/// Descriptor for `EditBotMessageResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List editBotMessageResponseDescriptor =
    $convert.base64Decode(
        'ChZFZGl0Qm90TWVzc2FnZVJlc3BvbnNlEjUKB21lc3NhZ2UYASABKAsyGy52b2ljZS5tZXNzYW'
        'dpbmcudjEuTWVzc2FnZVIHbWVzc2FnZQ==');

@$core.Deprecated('Use sendEphemeralResponseDescriptor instead')
const SendEphemeralResponse$json = {
  '1': 'SendEphemeralResponse',
};

/// Descriptor for `SendEphemeralResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sendEphemeralResponseDescriptor =
    $convert.base64Decode('ChVTZW5kRXBoZW1lcmFsUmVzcG9uc2U=');

@$core.Deprecated('Use deferResponseResponseDescriptor instead')
const DeferResponseResponse$json = {
  '1': 'DeferResponseResponse',
};

/// Descriptor for `DeferResponseResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deferResponseResponseDescriptor =
    $convert.base64Decode('ChVEZWZlclJlc3BvbnNlUmVzcG9uc2U=');

@$core.Deprecated('Use pollEventsResponseDescriptor instead')
const PollEventsResponse$json = {
  '1': 'PollEventsResponse',
  '2': [
    {
      '1': 'bot_event',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.BotEvent',
      '10': 'botEvent'
    },
  ],
};

/// Descriptor for `PollEventsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List pollEventsResponseDescriptor = $convert.base64Decode(
    'ChJQb2xsRXZlbnRzUmVzcG9uc2USMwoJYm90X2V2ZW50GAEgASgLMhYudm9pY2UuYm90LnYxLk'
    'JvdEV2ZW50Ughib3RFdmVudA==');

@$core.Deprecated('Use validateManifestRequestDescriptor instead')
const ValidateManifestRequest$json = {
  '1': 'ValidateManifestRequest',
  '2': [
    {'1': 'manifest_yaml', '3': 1, '4': 1, '5': 9, '10': 'manifestYaml'},
  ],
};

/// Descriptor for `ValidateManifestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateManifestRequestDescriptor =
    $convert.base64Decode(
        'ChdWYWxpZGF0ZU1hbmlmZXN0UmVxdWVzdBIjCg1tYW5pZmVzdF95YW1sGAEgASgJUgxtYW5pZm'
        'VzdFlhbWw=');

@$core.Deprecated('Use validateManifestResponseDescriptor instead')
const ValidateManifestResponse$json = {
  '1': 'ValidateManifestResponse',
  '2': [
    {'1': 'valid', '3': 1, '4': 1, '5': 8, '10': 'valid'},
    {'1': 'errors', '3': 2, '4': 3, '5': 9, '10': 'errors'},
    {
      '1': 'normalized_manifest_json',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'normalizedManifestJson',
      '17': true
    },
  ],
  '8': [
    {'1': '_normalized_manifest_json'},
  ],
};

/// Descriptor for `ValidateManifestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateManifestResponseDescriptor = $convert.base64Decode(
    'ChhWYWxpZGF0ZU1hbmlmZXN0UmVzcG9uc2USFAoFdmFsaWQYASABKAhSBXZhbGlkEhYKBmVycm'
    '9ycxgCIAMoCVIGZXJyb3JzEj0KGG5vcm1hbGl6ZWRfbWFuaWZlc3RfanNvbhgDIAEoCUgAUhZu'
    'b3JtYWxpemVkTWFuaWZlc3RKc29uiAEBQhsKGV9ub3JtYWxpemVkX21hbmlmZXN0X2pzb24=');

@$core.Deprecated('Use applyManifestRequestDescriptor instead')
const ApplyManifestRequest$json = {
  '1': 'ApplyManifestRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'manifest_yaml', '3': 2, '4': 1, '5': 9, '10': 'manifestYaml'},
  ],
};

/// Descriptor for `ApplyManifestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyManifestRequestDescriptor = $convert.base64Decode(
    'ChRBcHBseU1hbmlmZXN0UmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElkEiMKDW1hbmlmZX'
    'N0X3lhbWwYAiABKAlSDG1hbmlmZXN0WWFtbA==');

@$core.Deprecated('Use applyManifestResponseDescriptor instead')
const ApplyManifestResponse$json = {
  '1': 'ApplyManifestResponse',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
  ],
};

/// Descriptor for `ApplyManifestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyManifestResponseDescriptor = $convert.base64Decode(
    'ChVBcHBseU1hbmlmZXN0UmVzcG9uc2USIwoDYm90GAEgASgLMhEudm9pY2UuYm90LnYxLkJvdF'
    'IDYm90');

@$core.Deprecated('Use installBotInSpaceRequestDescriptor instead')
const InstallBotInSpaceRequest$json = {
  '1': 'InstallBotInSpaceRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'allowed_chats',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'allowedChats'
    },
    {
      '1': 'acknowledge_privileged_scopes',
      '3': 4,
      '4': 1,
      '5': 8,
      '10': 'acknowledgePrivilegedScopes'
    },
  ],
};

/// Descriptor for `InstallBotInSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List installBotInSpaceRequestDescriptor = $convert.base64Decode(
    'ChhJbnN0YWxsQm90SW5TcGFjZVJlcXVlc3QSFQoGYm90X2lkGAEgASgJUgVib3RJZBIZCghzcG'
    'FjZV9pZBgCIAEoCVIHc3BhY2VJZBI7Cg1hbGxvd2VkX2NoYXRzGAMgAygLMhYudm9pY2UuY2hh'
    'dC52MS5DaGF0UmVmUgxhbGxvd2VkQ2hhdHMSQgodYWNrbm93bGVkZ2VfcHJpdmlsZWdlZF9zY2'
    '9wZXMYBCABKAhSG2Fja25vd2xlZGdlUHJpdmlsZWdlZFNjb3Blcw==');

@$core.Deprecated('Use installBotInSpaceResponseDescriptor instead')
const InstallBotInSpaceResponse$json = {
  '1': 'InstallBotInSpaceResponse',
  '2': [
    {'1': 'installation_id', '3': 1, '4': 1, '5': 9, '10': 'installationId'},
  ],
};

/// Descriptor for `InstallBotInSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List installBotInSpaceResponseDescriptor =
    $convert.base64Decode(
        'ChlJbnN0YWxsQm90SW5TcGFjZVJlc3BvbnNlEicKD2luc3RhbGxhdGlvbl9pZBgBIAEoCVIOaW'
        '5zdGFsbGF0aW9uSWQ=');

@$core.Deprecated('Use uninstallBotFromSpaceRequestDescriptor instead')
const UninstallBotFromSpaceRequest$json = {
  '1': 'UninstallBotFromSpaceRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `UninstallBotFromSpaceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List uninstallBotFromSpaceRequestDescriptor =
    $convert.base64Decode(
        'ChxVbmluc3RhbGxCb3RGcm9tU3BhY2VSZXF1ZXN0EhUKBmJvdF9pZBgBIAEoCVIFYm90SWQSGQ'
        'oIc3BhY2VfaWQYAiABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use uninstallBotFromSpaceResponseDescriptor instead')
const UninstallBotFromSpaceResponse$json = {
  '1': 'UninstallBotFromSpaceResponse',
};

/// Descriptor for `UninstallBotFromSpaceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List uninstallBotFromSpaceResponseDescriptor =
    $convert.base64Decode('Ch1Vbmluc3RhbGxCb3RGcm9tU3BhY2VSZXNwb25zZQ==');

@$core.Deprecated('Use listInstalledBotsRequestDescriptor instead')
const ListInstalledBotsRequest$json = {
  '1': 'ListInstalledBotsRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListInstalledBotsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInstalledBotsRequestDescriptor =
    $convert.base64Decode(
        'ChhMaXN0SW5zdGFsbGVkQm90c1JlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use installedBotDescriptor instead')
const InstalledBot$json = {
  '1': 'InstalledBot',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
    {'1': 'installation_id', '3': 2, '4': 1, '5': 9, '10': 'installationId'},
    {
      '1': 'allowed_chats',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'allowedChats'
    },
    {'1': 'online', '3': 4, '4': 1, '5': 8, '10': 'online'},
  ],
};

/// Descriptor for `InstalledBot`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List installedBotDescriptor = $convert.base64Decode(
    'CgxJbnN0YWxsZWRCb3QSIwoDYm90GAEgASgLMhEudm9pY2UuYm90LnYxLkJvdFIDYm90EicKD2'
    'luc3RhbGxhdGlvbl9pZBgCIAEoCVIOaW5zdGFsbGF0aW9uSWQSOwoNYWxsb3dlZF9jaGF0cxgD'
    'IAMoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZlIMYWxsb3dlZENoYXRzEhYKBm9ubGluZRgEIA'
    'EoCFIGb25saW5l');

@$core.Deprecated('Use listInstalledBotsResponseDescriptor instead')
const ListInstalledBotsResponse$json = {
  '1': 'ListInstalledBotsResponse',
  '2': [
    {
      '1': 'installed_bots',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.InstalledBot',
      '10': 'installedBots'
    },
  ],
};

/// Descriptor for `ListInstalledBotsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listInstalledBotsResponseDescriptor =
    $convert.base64Decode(
        'ChlMaXN0SW5zdGFsbGVkQm90c1Jlc3BvbnNlEkEKDmluc3RhbGxlZF9ib3RzGAEgAygLMhoudm'
        '9pY2UuYm90LnYxLkluc3RhbGxlZEJvdFINaW5zdGFsbGVkQm90cw==');

@$core.Deprecated('Use listBotsInChatRequestDescriptor instead')
const ListBotsInChatRequest$json = {
  '1': 'ListBotsInChatRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'space_id', '3': 2, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `ListBotsInChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBotsInChatRequestDescriptor = $convert.base64Decode(
    'ChVMaXN0Qm90c0luQ2hhdFJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2'
    'hhdFJlZlIEY2hhdBIZCghzcGFjZV9pZBgCIAEoCVIHc3BhY2VJZA==');

@$core.Deprecated('Use chatBotEntryDescriptor instead')
const ChatBotEntry$json = {
  '1': 'ChatBotEntry',
  '2': [
    {
      '1': 'bot',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.bot.v1.Bot',
      '10': 'bot'
    },
    {'1': 'enabled', '3': 2, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'whitelisted', '3': 3, '4': 1, '5': 8, '10': 'whitelisted'},
  ],
};

/// Descriptor for `ChatBotEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List chatBotEntryDescriptor = $convert.base64Decode(
    'CgxDaGF0Qm90RW50cnkSIwoDYm90GAEgASgLMhEudm9pY2UuYm90LnYxLkJvdFIDYm90EhgKB2'
    'VuYWJsZWQYAiABKAhSB2VuYWJsZWQSIAoLd2hpdGVsaXN0ZWQYAyABKAhSC3doaXRlbGlzdGVk');

@$core.Deprecated('Use listBotsInChatResponseDescriptor instead')
const ListBotsInChatResponse$json = {
  '1': 'ListBotsInChatResponse',
  '2': [
    {
      '1': 'bots',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.ChatBotEntry',
      '10': 'bots'
    },
  ],
};

/// Descriptor for `ListBotsInChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listBotsInChatResponseDescriptor =
    $convert.base64Decode(
        'ChZMaXN0Qm90c0luQ2hhdFJlc3BvbnNlEi4KBGJvdHMYASADKAsyGi52b2ljZS5ib3QudjEuQ2'
        'hhdEJvdEVudHJ5UgRib3Rz');

@$core.Deprecated('Use setBotChatEnabledRequestDescriptor instead')
const SetBotChatEnabledRequest$json = {
  '1': 'SetBotChatEnabledRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {
      '1': 'chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'enabled', '3': 3, '4': 1, '5': 8, '10': 'enabled'},
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
  ],
};

/// Descriptor for `SetBotChatEnabledRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setBotChatEnabledRequestDescriptor = $convert.base64Decode(
    'ChhTZXRCb3RDaGF0RW5hYmxlZFJlcXVlc3QSFQoGYm90X2lkGAEgASgJUgVib3RJZBIqCgRjaG'
    'F0GAIgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmUgRjaGF0EhgKB2VuYWJsZWQYAyABKAhS'
    'B2VuYWJsZWQSGQoIc3BhY2VfaWQYBCABKAlSB3NwYWNlSWQ=');

@$core.Deprecated('Use setBotChatEnabledResponseDescriptor instead')
const SetBotChatEnabledResponse$json = {
  '1': 'SetBotChatEnabledResponse',
};

/// Descriptor for `SetBotChatEnabledResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setBotChatEnabledResponseDescriptor =
    $convert.base64Decode('ChlTZXRCb3RDaGF0RW5hYmxlZFJlc3BvbnNl');

@$core.Deprecated('Use slashCommandOptionDescriptor instead')
const SlashCommandOption$json = {
  '1': 'SlashCommandOption',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
    {'1': 'required', '3': 3, '4': 1, '5': 8, '10': 'required'},
    {'1': 'autocomplete', '3': 4, '4': 1, '5': 8, '10': 'autocomplete'},
  ],
};

/// Descriptor for `SlashCommandOption`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List slashCommandOptionDescriptor = $convert.base64Decode(
    'ChJTbGFzaENvbW1hbmRPcHRpb24SEgoEbmFtZRgBIAEoCVIEbmFtZRISCgR0eXBlGAIgASgJUg'
    'R0eXBlEhoKCHJlcXVpcmVkGAMgASgIUghyZXF1aXJlZBIiCgxhdXRvY29tcGxldGUYBCABKAhS'
    'DGF1dG9jb21wbGV0ZQ==');

@$core.Deprecated('Use slashCommandDescriptor instead')
const SlashCommand$json = {
  '1': 'SlashCommand',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'bot_name', '3': 2, '4': 1, '5': 9, '10': 'botName'},
    {'1': 'name', '3': 3, '4': 1, '5': 9, '10': 'name'},
    {'1': 'description', '3': 4, '4': 1, '5': 9, '10': 'description'},
    {
      '1': 'options',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.SlashCommandOption',
      '10': 'options'
    },
    {
      '1': 'group_name',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'groupName',
      '17': true
    },
    {'1': 'online', '3': 7, '4': 1, '5': 8, '10': 'online'},
  ],
  '8': [
    {'1': '_group_name'},
  ],
};

/// Descriptor for `SlashCommand`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List slashCommandDescriptor = $convert.base64Decode(
    'CgxTbGFzaENvbW1hbmQSFQoGYm90X2lkGAEgASgJUgVib3RJZBIZCghib3RfbmFtZRgCIAEoCV'
    'IHYm90TmFtZRISCgRuYW1lGAMgASgJUgRuYW1lEiAKC2Rlc2NyaXB0aW9uGAQgASgJUgtkZXNj'
    'cmlwdGlvbhI6CgdvcHRpb25zGAUgAygLMiAudm9pY2UuYm90LnYxLlNsYXNoQ29tbWFuZE9wdG'
    'lvblIHb3B0aW9ucxIiCgpncm91cF9uYW1lGAYgASgJSABSCWdyb3VwTmFtZYgBARIWCgZvbmxp'
    'bmUYByABKAhSBm9ubGluZUINCgtfZ3JvdXBfbmFtZQ==');

@$core.Deprecated('Use listSlashCommandsForChatRequestDescriptor instead')
const ListSlashCommandsForChatRequest$json = {
  '1': 'ListSlashCommandsForChatRequest',
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

/// Descriptor for `ListSlashCommandsForChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSlashCommandsForChatRequestDescriptor =
    $convert.base64Decode(
        'Ch9MaXN0U2xhc2hDb21tYW5kc0ZvckNoYXRSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS'
        '5jaGF0LnYxLkNoYXRSZWZSBGNoYXQ=');

@$core.Deprecated('Use listSlashCommandsForChatResponseDescriptor instead')
const ListSlashCommandsForChatResponse$json = {
  '1': 'ListSlashCommandsForChatResponse',
  '2': [
    {
      '1': 'commands',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.SlashCommand',
      '10': 'commands'
    },
  ],
};

/// Descriptor for `ListSlashCommandsForChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSlashCommandsForChatResponseDescriptor =
    $convert.base64Decode(
        'CiBMaXN0U2xhc2hDb21tYW5kc0ZvckNoYXRSZXNwb25zZRI2Cghjb21tYW5kcxgBIAMoCzIaLn'
        'ZvaWNlLmJvdC52MS5TbGFzaENvbW1hbmRSCGNvbW1hbmRz');

@$core.Deprecated('Use executeSlashInteractionRequestDescriptor instead')
const ExecuteSlashInteractionRequest$json = {
  '1': 'ExecuteSlashInteractionRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'bot_id', '3': 2, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'command_name', '3': 3, '4': 1, '5': 9, '10': 'commandName'},
    {'1': 'options_json', '3': 4, '4': 1, '5': 9, '10': 'optionsJson'},
  ],
};

/// Descriptor for `ExecuteSlashInteractionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List executeSlashInteractionRequestDescriptor =
    $convert.base64Decode(
        'Ch5FeGVjdXRlU2xhc2hJbnRlcmFjdGlvblJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLm'
        'NoYXQudjEuQ2hhdFJlZlIEY2hhdBIVCgZib3RfaWQYAiABKAlSBWJvdElkEiEKDGNvbW1hbmRf'
        'bmFtZRgDIAEoCVILY29tbWFuZE5hbWUSIQoMb3B0aW9uc19qc29uGAQgASgJUgtvcHRpb25zSn'
        'Nvbg==');

@$core.Deprecated('Use executeSlashInteractionResponseDescriptor instead')
const ExecuteSlashInteractionResponse$json = {
  '1': 'ExecuteSlashInteractionResponse',
  '2': [
    {
      '1': 'interaction_token',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'interactionToken'
    },
    {
      '1': 'content',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'content',
      '17': true
    },
    {'1': 'is_ephemeral', '3': 3, '4': 1, '5': 8, '10': 'isEphemeral'},
    {'1': 'deferred', '3': 4, '4': 1, '5': 8, '10': 'deferred'},
    {
      '1': 'message',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '9': 1,
      '10': 'message',
      '17': true
    },
    {
      '1': 'error_code',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'errorCode',
      '17': true
    },
    {
      '1': 'error_message',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'errorMessage',
      '17': true
    },
  ],
  '8': [
    {'1': '_content'},
    {'1': '_message'},
    {'1': '_error_code'},
    {'1': '_error_message'},
  ],
};

/// Descriptor for `ExecuteSlashInteractionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List executeSlashInteractionResponseDescriptor = $convert.base64Decode(
    'Ch9FeGVjdXRlU2xhc2hJbnRlcmFjdGlvblJlc3BvbnNlEisKEWludGVyYWN0aW9uX3Rva2VuGA'
    'EgASgJUhBpbnRlcmFjdGlvblRva2VuEh0KB2NvbnRlbnQYAiABKAlIAFIHY29udGVudIgBARIh'
    'Cgxpc19lcGhlbWVyYWwYAyABKAhSC2lzRXBoZW1lcmFsEhoKCGRlZmVycmVkGAQgASgIUghkZW'
    'ZlcnJlZBI6CgdtZXNzYWdlGAUgASgLMhsudm9pY2UubWVzc2FnaW5nLnYxLk1lc3NhZ2VIAVIH'
    'bWVzc2FnZYgBARIiCgplcnJvcl9jb2RlGAYgASgJSAJSCWVycm9yQ29kZYgBARIoCg1lcnJvcl'
    '9tZXNzYWdlGAcgASgJSANSDGVycm9yTWVzc2FnZYgBAUIKCghfY29udGVudEIKCghfbWVzc2Fn'
    'ZUINCgtfZXJyb3JfY29kZUIQCg5fZXJyb3JfbWVzc2FnZQ==');

@$core.Deprecated('Use completeInteractionRequestDescriptor instead')
const CompleteInteractionRequest$json = {
  '1': 'CompleteInteractionRequest',
  '2': [
    {
      '1': 'interaction_token',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'interactionToken'
    },
    {'1': 'content', '3': 2, '4': 1, '5': 9, '10': 'content'},
    {'1': 'is_ephemeral', '3': 3, '4': 1, '5': 8, '10': 'isEphemeral'},
    {'1': 'deferred', '3': 4, '4': 1, '5': 8, '10': 'deferred'},
  ],
};

/// Descriptor for `CompleteInteractionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeInteractionRequestDescriptor =
    $convert.base64Decode(
        'ChpDb21wbGV0ZUludGVyYWN0aW9uUmVxdWVzdBIrChFpbnRlcmFjdGlvbl90b2tlbhgBIAEoCV'
        'IQaW50ZXJhY3Rpb25Ub2tlbhIYCgdjb250ZW50GAIgASgJUgdjb250ZW50EiEKDGlzX2VwaGVt'
        'ZXJhbBgDIAEoCFILaXNFcGhlbWVyYWwSGgoIZGVmZXJyZWQYBCABKAhSCGRlZmVycmVk');

@$core.Deprecated('Use completeInteractionResponseDescriptor instead')
const CompleteInteractionResponse$json = {
  '1': 'CompleteInteractionResponse',
  '2': [
    {
      '1': 'message',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.messaging.v1.Message',
      '9': 0,
      '10': 'message',
      '17': true
    },
  ],
  '8': [
    {'1': '_message'},
  ],
};

/// Descriptor for `CompleteInteractionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeInteractionResponseDescriptor =
    $convert.base64Decode(
        'ChtDb21wbGV0ZUludGVyYWN0aW9uUmVzcG9uc2USOgoHbWVzc2FnZRgBIAEoCzIbLnZvaWNlLm'
        '1lc3NhZ2luZy52MS5NZXNzYWdlSABSB21lc3NhZ2WIAQFCCgoIX21lc3NhZ2U=');

@$core.Deprecated('Use autocompleteSlashOptionRequestDescriptor instead')
const AutocompleteSlashOptionRequest$json = {
  '1': 'AutocompleteSlashOptionRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'bot_id', '3': 2, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'command_name', '3': 3, '4': 1, '5': 9, '10': 'commandName'},
    {'1': 'option_name', '3': 4, '4': 1, '5': 9, '10': 'optionName'},
    {'1': 'focused_value', '3': 5, '4': 1, '5': 9, '10': 'focusedValue'},
    {'1': 'options_json', '3': 6, '4': 1, '5': 9, '10': 'optionsJson'},
  ],
};

/// Descriptor for `AutocompleteSlashOptionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List autocompleteSlashOptionRequestDescriptor = $convert.base64Decode(
    'Ch5BdXRvY29tcGxldGVTbGFzaE9wdGlvblJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLm'
    'NoYXQudjEuQ2hhdFJlZlIEY2hhdBIVCgZib3RfaWQYAiABKAlSBWJvdElkEiEKDGNvbW1hbmRf'
    'bmFtZRgDIAEoCVILY29tbWFuZE5hbWUSHwoLb3B0aW9uX25hbWUYBCABKAlSCm9wdGlvbk5hbW'
    'USIwoNZm9jdXNlZF92YWx1ZRgFIAEoCVIMZm9jdXNlZFZhbHVlEiEKDG9wdGlvbnNfanNvbhgG'
    'IAEoCVILb3B0aW9uc0pzb24=');

@$core.Deprecated('Use autocompleteChoiceDescriptor instead')
const AutocompleteChoice$json = {
  '1': 'AutocompleteChoice',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
};

/// Descriptor for `AutocompleteChoice`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List autocompleteChoiceDescriptor = $convert.base64Decode(
    'ChJBdXRvY29tcGxldGVDaG9pY2USEgoEbmFtZRgBIAEoCVIEbmFtZRIUCgV2YWx1ZRgCIAEoCV'
    'IFdmFsdWU=');

@$core.Deprecated('Use autocompleteSlashOptionResponseDescriptor instead')
const AutocompleteSlashOptionResponse$json = {
  '1': 'AutocompleteSlashOptionResponse',
  '2': [
    {
      '1': 'choices',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.bot.v1.AutocompleteChoice',
      '10': 'choices'
    },
  ],
};

/// Descriptor for `AutocompleteSlashOptionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List autocompleteSlashOptionResponseDescriptor =
    $convert.base64Decode(
        'Ch9BdXRvY29tcGxldGVTbGFzaE9wdGlvblJlc3BvbnNlEjoKB2Nob2ljZXMYASADKAsyIC52b2'
        'ljZS5ib3QudjEuQXV0b2NvbXBsZXRlQ2hvaWNlUgdjaG9pY2Vz');

@$core.Deprecated('Use touchPresenceRequestDescriptor instead')
const TouchPresenceRequest$json = {
  '1': 'TouchPresenceRequest',
  '2': [
    {'1': 'bot_id', '3': 1, '4': 1, '5': 9, '10': 'botId'},
  ],
};

/// Descriptor for `TouchPresenceRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List touchPresenceRequestDescriptor =
    $convert.base64Decode(
        'ChRUb3VjaFByZXNlbmNlUmVxdWVzdBIVCgZib3RfaWQYASABKAlSBWJvdElk');

@$core.Deprecated('Use touchPresenceResponseDescriptor instead')
const TouchPresenceResponse$json = {
  '1': 'TouchPresenceResponse',
};

/// Descriptor for `TouchPresenceResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List touchPresenceResponseDescriptor =
    $convert.base64Decode('ChVUb3VjaFByZXNlbmNlUmVzcG9uc2U=');

@$core.Deprecated('Use assignBotRoleRequestDescriptor instead')
const AssignBotRoleRequest$json = {
  '1': 'AssignBotRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `AssignBotRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignBotRoleRequestDescriptor = $convert.base64Decode(
    'ChRBc3NpZ25Cb3RSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm'
    '9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use assignBotRoleResponseDescriptor instead')
const AssignBotRoleResponse$json = {
  '1': 'AssignBotRoleResponse',
};

/// Descriptor for `AssignBotRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List assignBotRoleResponseDescriptor =
    $convert.base64Decode('ChVBc3NpZ25Cb3RSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use revokeBotRoleRequestDescriptor instead')
const RevokeBotRoleRequest$json = {
  '1': 'RevokeBotRoleRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'role_id', '3': 3, '4': 1, '5': 9, '10': 'roleId'},
  ],
};

/// Descriptor for `RevokeBotRoleRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeBotRoleRequestDescriptor = $convert.base64Decode(
    'ChRSZXZva2VCb3RSb2xlUmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBIdCgpwcm'
    '9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSFwoHcm9sZV9pZBgDIAEoCVIGcm9sZUlk');

@$core.Deprecated('Use revokeBotRoleResponseDescriptor instead')
const RevokeBotRoleResponse$json = {
  '1': 'RevokeBotRoleResponse',
};

/// Descriptor for `RevokeBotRoleResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeBotRoleResponseDescriptor =
    $convert.base64Decode('ChVSZXZva2VCb3RSb2xlUmVzcG9uc2U=');

@$core.Deprecated('Use listSpaceMembersForBotRequestDescriptor instead')
const ListSpaceMembersForBotRequest$json = {
  '1': 'ListSpaceMembersForBotRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'cursor', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'cursor', '17': true},
  ],
  '8': [
    {'1': '_cursor'},
  ],
};

/// Descriptor for `ListSpaceMembersForBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpaceMembersForBotRequestDescriptor =
    $convert.base64Decode(
        'Ch1MaXN0U3BhY2VNZW1iZXJzRm9yQm90UmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2'
        'VJZBIbCgZjdXJzb3IYAiABKAlIAFIGY3Vyc29yiAEBQgkKB19jdXJzb3I=');

@$core.Deprecated('Use listSpaceMembersForBotResponseDescriptor instead')
const ListSpaceMembersForBotResponse$json = {
  '1': 'ListSpaceMembersForBotResponse',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'next_cursor',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'nextCursor',
      '17': true
    },
  ],
  '8': [
    {'1': '_next_cursor'},
  ],
};

/// Descriptor for `ListSpaceMembersForBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSpaceMembersForBotResponseDescriptor =
    $convert.base64Decode(
        'Ch5MaXN0U3BhY2VNZW1iZXJzRm9yQm90UmVzcG9uc2USHwoLcHJvZmlsZV9pZHMYASADKAlSCn'
        'Byb2ZpbGVJZHMSJAoLbmV4dF9jdXJzb3IYAiABKAlIAFIKbmV4dEN1cnNvcogBAUIOCgxfbmV4'
        'dF9jdXJzb3I=');

@$core.Deprecated('Use createBotChatRequestDescriptor instead')
const CreateBotChatRequest$json = {
  '1': 'CreateBotChatRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {'1': 'chat_type', '3': 3, '4': 1, '5': 9, '10': 'chatType'},
  ],
};

/// Descriptor for `CreateBotChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createBotChatRequestDescriptor = $convert.base64Decode(
    'ChRDcmVhdGVCb3RDaGF0UmVxdWVzdBIZCghzcGFjZV9pZBgBIAEoCVIHc3BhY2VJZBISCgRuYW'
    '1lGAIgASgJUgRuYW1lEhsKCWNoYXRfdHlwZRgDIAEoCVIIY2hhdFR5cGU=');

@$core.Deprecated('Use createBotChatResponseDescriptor instead')
const CreateBotChatResponse$json = {
  '1': 'CreateBotChatResponse',
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

/// Descriptor for `CreateBotChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createBotChatResponseDescriptor = $convert.base64Decode(
    'ChVDcmVhdGVCb3RDaGF0UmVzcG9uc2USKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2'
    'hhdFJlZlIEY2hhdA==');

@$core.Deprecated('Use getChatMessagesForBotRequestDescriptor instead')
const GetChatMessagesForBotRequest$json = {
  '1': 'GetChatMessagesForBotRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'cursor', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'cursor', '17': true},
  ],
  '8': [
    {'1': '_cursor'},
  ],
};

/// Descriptor for `GetChatMessagesForBotRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatMessagesForBotRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRDaGF0TWVzc2FnZXNGb3JCb3RSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaG'
        'F0LnYxLkNoYXRSZWZSBGNoYXQSGwoGY3Vyc29yGAIgASgJSABSBmN1cnNvcogBAUIJCgdfY3Vy'
        'c29y');

@$core.Deprecated('Use getChatMessagesForBotResponseDescriptor instead')
const GetChatMessagesForBotResponse$json = {
  '1': 'GetChatMessagesForBotResponse',
  '2': [
    {'1': 'message_ids', '3': 1, '4': 3, '5': 9, '10': 'messageIds'},
    {
      '1': 'next_cursor',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'nextCursor',
      '17': true
    },
  ],
  '8': [
    {'1': '_next_cursor'},
  ],
};

/// Descriptor for `GetChatMessagesForBotResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getChatMessagesForBotResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRDaGF0TWVzc2FnZXNGb3JCb3RSZXNwb25zZRIfCgttZXNzYWdlX2lkcxgBIAMoCVIKbW'
        'Vzc2FnZUlkcxIkCgtuZXh0X2N1cnNvchgCIAEoCUgAUgpuZXh0Q3Vyc29yiAEBQg4KDF9uZXh0'
        'X2N1cnNvcg==');
