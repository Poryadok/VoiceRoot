// This is a generated file - do not edit.
//
// Generated from voice/gameintegration/v1/game_event.proto.

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

@$core.Deprecated('Use gameEventPublicationStatusDescriptor instead')
const GameEventPublicationStatus$json = {
  '1': 'GameEventPublicationStatus',
  '2': [
    {'1': 'GAME_EVENT_PUBLICATION_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'GAME_EVENT_PUBLICATION_STATUS_PUBLISHED', '2': 1},
    {'1': 'GAME_EVENT_PUBLICATION_STATUS_EXPIRED', '2': 2},
  ],
};

/// Descriptor for `GameEventPublicationStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List gameEventPublicationStatusDescriptor = $convert.base64Decode(
    'ChpHYW1lRXZlbnRQdWJsaWNhdGlvblN0YXR1cxItCilHQU1FX0VWRU5UX1BVQkxJQ0FUSU9OX1'
    'NUQVRVU19VTlNQRUNJRklFRBAAEisKJ0dBTUVfRVZFTlRfUFVCTElDQVRJT05fU1RBVFVTX1BV'
    'QkxJU0hFRBABEikKJUdBTUVfRVZFTlRfUFVCTElDQVRJT05fU1RBVFVTX0VYUElSRUQQAg==');

@$core.Deprecated('Use bindingEventRecipientDescriptor instead')
const BindingEventRecipient$json = {
  '1': 'BindingEventRecipient',
  '2': [
    {'1': 'binding_id', '3': 1, '4': 1, '5': 9, '10': 'bindingId'},
    {'1': 'resolved_chat_id', '3': 2, '4': 1, '5': 9, '10': 'resolvedChatId'},
  ],
};

/// Descriptor for `BindingEventRecipient`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List bindingEventRecipientDescriptor = $convert.base64Decode(
    'ChVCaW5kaW5nRXZlbnRSZWNpcGllbnQSHQoKYmluZGluZ19pZBgBIAEoCVIJYmluZGluZ0lkEi'
    'gKEHJlc29sdmVkX2NoYXRfaWQYAiABKAlSDnJlc29sdmVkQ2hhdElk');

@$core.Deprecated('Use directChatEventRecipientDescriptor instead')
const DirectChatEventRecipient$json = {
  '1': 'DirectChatEventRecipient',
  '2': [
    {'1': 'chat_id', '3': 1, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `DirectChatEventRecipient`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List directChatEventRecipientDescriptor =
    $convert.base64Decode(
        'ChhEaXJlY3RDaGF0RXZlbnRSZWNpcGllbnQSFwoHY2hhdF9pZBgBIAEoCVIGY2hhdElk');

@$core.Deprecated('Use verifiedGameEventIntentDescriptor instead')
const VerifiedGameEventIntent$json = {
  '1': 'VerifiedGameEventIntent',
  '2': [
    {'1': 'operation_id', '3': 1, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'app_id', '3': 2, '4': 1, '5': 9, '10': 'appId'},
    {
      '1': 'app_owner_account_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'appOwnerAccountId'
    },
    {'1': 'environment_id', '3': 4, '4': 1, '5': 9, '10': 'environmentId'},
    {'1': 'installation_id', '3': 5, '4': 1, '5': 9, '10': 'installationId'},
    {'1': 'bot_id', '3': 6, '4': 1, '5': 9, '10': 'botId'},
    {'1': 'event_id', '3': 7, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'payload_hash', '3': 8, '4': 1, '5': 9, '10': 'payloadHash'},
    {
      '1': 'binding',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.voice.gameintegration.v1.BindingEventRecipient',
      '9': 0,
      '10': 'binding'
    },
    {
      '1': 'direct_chat',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.voice.gameintegration.v1.DirectChatEventRecipient',
      '9': 0,
      '10': 'directChat'
    },
    {
      '1': 'authority_revision',
      '3': 11,
      '4': 1,
      '5': 4,
      '10': 'authorityRevision'
    },
    {'1': 'schema_version', '3': 12, '4': 1, '5': 13, '10': 'schemaVersion'},
    {
      '1': 'occurred_at',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {
      '1': 'expires_at',
      '3': 14,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {
      '1': 'character_binding_id',
      '3': 15,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'characterBindingId',
      '17': true
    },
    {
      '1': 'state_version',
      '3': 16,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'stateVersion',
      '17': true
    },
    {'1': 'fallback_text', '3': 17, '4': 1, '5': 9, '10': 'fallbackText'},
    {
      '1': 'client_message_id',
      '3': 18,
      '4': 1,
      '5': 9,
      '10': 'clientMessageId'
    },
    {'1': 'event_type', '3': 19, '4': 1, '5': 9, '10': 'eventType'},
    {
      '1': 'card',
      '3': 20,
      '4': 1,
      '5': 11,
      '6': '.voice.gameintegration.v1.GameCard',
      '10': 'card'
    },
  ],
  '8': [
    {'1': 'recipient'},
    {'1': '_character_binding_id'},
    {'1': '_state_version'},
  ],
};

/// Descriptor for `VerifiedGameEventIntent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifiedGameEventIntentDescriptor = $convert.base64Decode(
    'ChdWZXJpZmllZEdhbWVFdmVudEludGVudBIhCgxvcGVyYXRpb25faWQYASABKAlSC29wZXJhdG'
    'lvbklkEhUKBmFwcF9pZBgCIAEoCVIFYXBwSWQSLwoUYXBwX293bmVyX2FjY291bnRfaWQYAyAB'
    'KAlSEWFwcE93bmVyQWNjb3VudElkEiUKDmVudmlyb25tZW50X2lkGAQgASgJUg1lbnZpcm9ubW'
    'VudElkEicKD2luc3RhbGxhdGlvbl9pZBgFIAEoCVIOaW5zdGFsbGF0aW9uSWQSFQoGYm90X2lk'
    'GAYgASgJUgVib3RJZBIZCghldmVudF9pZBgHIAEoCVIHZXZlbnRJZBIhCgxwYXlsb2FkX2hhc2'
    'gYCCABKAlSC3BheWxvYWRIYXNoEksKB2JpbmRpbmcYCSABKAsyLy52b2ljZS5nYW1laW50ZWdy'
    'YXRpb24udjEuQmluZGluZ0V2ZW50UmVjaXBpZW50SABSB2JpbmRpbmcSVQoLZGlyZWN0X2NoYX'
    'QYCiABKAsyMi52b2ljZS5nYW1laW50ZWdyYXRpb24udjEuRGlyZWN0Q2hhdEV2ZW50UmVjaXBp'
    'ZW50SABSCmRpcmVjdENoYXQSLQoSYXV0aG9yaXR5X3JldmlzaW9uGAsgASgEUhFhdXRob3JpdH'
    'lSZXZpc2lvbhIlCg5zY2hlbWFfdmVyc2lvbhgMIAEoDVINc2NoZW1hVmVyc2lvbhI7CgtvY2N1'
    'cnJlZF9hdBgNIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCm9jY3VycmVkQXQSOQ'
    'oKZXhwaXJlc19hdBgOIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWV4cGlyZXNB'
    'dBI1ChRjaGFyYWN0ZXJfYmluZGluZ19pZBgPIAEoCUgBUhJjaGFyYWN0ZXJCaW5kaW5nSWSIAQ'
    'ESKAoNc3RhdGVfdmVyc2lvbhgQIAEoCUgCUgxzdGF0ZVZlcnNpb26IAQESIwoNZmFsbGJhY2tf'
    'dGV4dBgRIAEoCVIMZmFsbGJhY2tUZXh0EioKEWNsaWVudF9tZXNzYWdlX2lkGBIgASgJUg9jbG'
    'llbnRNZXNzYWdlSWQSHQoKZXZlbnRfdHlwZRgTIAEoCVIJZXZlbnRUeXBlEjYKBGNhcmQYFCAB'
    'KAsyIi52b2ljZS5nYW1laW50ZWdyYXRpb24udjEuR2FtZUNhcmRSBGNhcmRCCwoJcmVjaXBpZW'
    '50QhcKFV9jaGFyYWN0ZXJfYmluZGluZ19pZEIQCg5fc3RhdGVfdmVyc2lvbg==');

@$core.Deprecated('Use gameCardDescriptor instead')
const GameCard$json = {
  '1': 'GameCard',
  '2': [
    {'1': 'schema_version', '3': 1, '4': 1, '5': 13, '10': 'schemaVersion'},
    {'1': 'revision', '3': 2, '4': 1, '5': 4, '10': 'revision'},
    {'1': 'title', '3': 3, '4': 1, '5': 9, '10': 'title'},
    {'1': 'safe_summary', '3': 4, '4': 1, '5': 9, '10': 'safeSummary'},
    {
      '1': 'facts',
      '3': 5,
      '4': 3,
      '5': 11,
      '6': '.voice.gameintegration.v1.GameCardFact',
      '10': 'facts'
    },
    {
      '1': 'actions',
      '3': 6,
      '4': 3,
      '5': 11,
      '6': '.voice.gameintegration.v1.GameCardAction',
      '10': 'actions'
    },
    {
      '1': 'media_reference_ids',
      '3': 7,
      '4': 3,
      '5': 9,
      '10': 'mediaReferenceIds'
    },
  ],
};

/// Descriptor for `GameCard`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameCardDescriptor = $convert.base64Decode(
    'CghHYW1lQ2FyZBIlCg5zY2hlbWFfdmVyc2lvbhgBIAEoDVINc2NoZW1hVmVyc2lvbhIaCghyZX'
    'Zpc2lvbhgCIAEoBFIIcmV2aXNpb24SFAoFdGl0bGUYAyABKAlSBXRpdGxlEiEKDHNhZmVfc3Vt'
    'bWFyeRgEIAEoCVILc2FmZVN1bW1hcnkSPAoFZmFjdHMYBSADKAsyJi52b2ljZS5nYW1laW50ZW'
    'dyYXRpb24udjEuR2FtZUNhcmRGYWN0UgVmYWN0cxJCCgdhY3Rpb25zGAYgAygLMigudm9pY2Uu'
    'Z2FtZWludGVncmF0aW9uLnYxLkdhbWVDYXJkQWN0aW9uUgdhY3Rpb25zEi4KE21lZGlhX3JlZm'
    'VyZW5jZV9pZHMYByADKAlSEW1lZGlhUmVmZXJlbmNlSWRz');

@$core.Deprecated('Use gameCardFactDescriptor instead')
const GameCardFact$json = {
  '1': 'GameCardFact',
  '2': [
    {'1': 'label', '3': 1, '4': 1, '5': 9, '10': 'label'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
};

/// Descriptor for `GameCardFact`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameCardFactDescriptor = $convert.base64Decode(
    'CgxHYW1lQ2FyZEZhY3QSFAoFbGFiZWwYASABKAlSBWxhYmVsEhQKBXZhbHVlGAIgASgJUgV2YW'
    'x1ZQ==');

@$core.Deprecated('Use gameCardActionDescriptor instead')
const GameCardAction$json = {
  '1': 'GameCardAction',
  '2': [
    {'1': 'action_id', '3': 1, '4': 1, '5': 9, '10': 'actionId'},
    {'1': 'action_type', '3': 2, '4': 1, '5': 9, '10': 'actionType'},
    {'1': 'label', '3': 3, '4': 1, '5': 9, '10': 'label'},
    {'1': 'arguments_json', '3': 4, '4': 1, '5': 9, '10': 'argumentsJson'},
    {'1': 'state_version', '3': 5, '4': 1, '5': 9, '10': 'stateVersion'},
    {
      '1': 'expires_at',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `GameCardAction`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameCardActionDescriptor = $convert.base64Decode(
    'Cg5HYW1lQ2FyZEFjdGlvbhIbCglhY3Rpb25faWQYASABKAlSCGFjdGlvbklkEh8KC2FjdGlvbl'
    '90eXBlGAIgASgJUgphY3Rpb25UeXBlEhQKBWxhYmVsGAMgASgJUgVsYWJlbBIlCg5hcmd1bWVu'
    'dHNfanNvbhgEIAEoCVINYXJndW1lbnRzSnNvbhIjCg1zdGF0ZV92ZXJzaW9uGAUgASgJUgxzdG'
    'F0ZVZlcnNpb24SOQoKZXhwaXJlc19hdBgGIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3Rh'
    'bXBSCWV4cGlyZXNBdA==');
