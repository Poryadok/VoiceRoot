// This is a generated file - do not edit.
//
// Generated from voice/calls/v1/calls.proto.

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

@$core.Deprecated('Use voiceSessionKindDescriptor instead')
const VoiceSessionKind$json = {
  '1': 'VoiceSessionKind',
  '2': [
    {'1': 'VOICE_SESSION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'VOICE_SESSION_KIND_CALL', '2': 1},
    {'1': 'VOICE_SESSION_KIND_GROUP_VOICE', '2': 2},
    {'1': 'VOICE_SESSION_KIND_VOICE_ROOM', '2': 3},
  ],
};

/// Descriptor for `VoiceSessionKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List voiceSessionKindDescriptor = $convert.base64Decode(
    'ChBWb2ljZVNlc3Npb25LaW5kEiIKHlZPSUNFX1NFU1NJT05fS0lORF9VTlNQRUNJRklFRBAAEh'
    'sKF1ZPSUNFX1NFU1NJT05fS0lORF9DQUxMEAESIgoeVk9JQ0VfU0VTU0lPTl9LSU5EX0dST1VQ'
    'X1ZPSUNFEAISIQodVk9JQ0VfU0VTU0lPTl9LSU5EX1ZPSUNFX1JPT00QAw==');

@$core.Deprecated('Use callMediaKindDescriptor instead')
const CallMediaKind$json = {
  '1': 'CallMediaKind',
  '2': [
    {'1': 'CALL_MEDIA_KIND_UNSPECIFIED', '2': 0},
    {'1': 'CALL_MEDIA_KIND_AUDIO', '2': 1},
    {'1': 'CALL_MEDIA_KIND_VIDEO', '2': 2},
  ],
};

/// Descriptor for `CallMediaKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List callMediaKindDescriptor = $convert.base64Decode(
    'Cg1DYWxsTWVkaWFLaW5kEh8KG0NBTExfTUVESUFfS0lORF9VTlNQRUNJRklFRBAAEhkKFUNBTE'
    'xfTUVESUFfS0lORF9BVURJTxABEhkKFUNBTExfTUVESUFfS0lORF9WSURFTxAC');

@$core.Deprecated('Use callStatusDescriptor instead')
const CallStatus$json = {
  '1': 'CallStatus',
  '2': [
    {'1': 'CALL_STATUS_UNSPECIFIED', '2': 0},
    {'1': 'CALL_STATUS_RINGING', '2': 1},
    {'1': 'CALL_STATUS_ACTIVE', '2': 2},
    {'1': 'CALL_STATUS_DECLINED', '2': 3},
    {'1': 'CALL_STATUS_MISSED', '2': 4},
    {'1': 'CALL_STATUS_ENDED', '2': 5},
  ],
};

/// Descriptor for `CallStatus`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List callStatusDescriptor = $convert.base64Decode(
    'CgpDYWxsU3RhdHVzEhsKF0NBTExfU1RBVFVTX1VOU1BFQ0lGSUVEEAASFwoTQ0FMTF9TVEFUVV'
    'NfUklOR0lORxABEhYKEkNBTExfU1RBVFVTX0FDVElWRRACEhgKFENBTExfU1RBVFVTX0RFQ0xJ'
    'TkVEEAMSFgoSQ0FMTF9TVEFUVVNfTUlTU0VEEAQSFQoRQ0FMTF9TVEFUVVNfRU5ERUQQBQ==');

@$core.Deprecated('Use startCallRequestDescriptor instead')
const StartCallRequest$json = {
  '1': 'StartCallRequest',
  '2': [
    {'1': 'room_type', '3': 1, '4': 1, '5': 9, '10': 'roomType'},
    {
      '1': 'linked_chat',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'linkedChat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
    {
      '1': 'space',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '9': 2,
      '10': 'space',
      '17': true
    },
    {
      '1': 'room_type_enum',
      '3': 5,
      '4': 1,
      '5': 14,
      '6': '.voice.calls.v1.VoiceSessionKind',
      '9': 3,
      '10': 'roomTypeEnum',
      '17': true
    },
    {
      '1': 'callee_profile_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'calleeProfileId',
      '17': true
    },
    {
      '1': 'media_kind',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.calls.v1.CallMediaKind',
      '9': 5,
      '10': 'mediaKind',
      '17': true
    },
  ],
  '8': [
    {'1': '_linked_chat'},
    {'1': '_voice_room_id'},
    {'1': '_space'},
    {'1': '_room_type_enum'},
    {'1': '_callee_profile_id'},
    {'1': '_media_kind'},
  ],
};

/// Descriptor for `StartCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startCallRequestDescriptor = $convert.base64Decode(
    'ChBTdGFydENhbGxSZXF1ZXN0EhsKCXJvb21fdHlwZRgBIAEoCVIIcm9vbVR5cGUSPAoLbGlua2'
    'VkX2NoYXQYAiABKAsyFi52b2ljZS5jaGF0LnYxLkNoYXRSZWZIAFIKbGlua2VkQ2hhdIgBARIn'
    'Cg12b2ljZV9yb29tX2lkGAMgASgJSAFSC3ZvaWNlUm9vbUlkiAEBEjMKBXNwYWNlGAQgASgLMh'
    'gudm9pY2Uuc3BhY2UudjEuU3BhY2VSZWZIAlIFc3BhY2WIAQESSwoOcm9vbV90eXBlX2VudW0Y'
    'BSABKA4yIC52b2ljZS5jYWxscy52MS5Wb2ljZVNlc3Npb25LaW5kSANSDHJvb21UeXBlRW51bY'
    'gBARIvChFjYWxsZWVfcHJvZmlsZV9pZBgGIAEoCUgEUg9jYWxsZWVQcm9maWxlSWSIAQESQQoK'
    'bWVkaWFfa2luZBgHIAEoDjIdLnZvaWNlLmNhbGxzLnYxLkNhbGxNZWRpYUtpbmRIBVIJbWVkaW'
    'FLaW5kiAEBQg4KDF9saW5rZWRfY2hhdEIQCg5fdm9pY2Vfcm9vbV9pZEIICgZfc3BhY2VCEQoP'
    'X3Jvb21fdHlwZV9lbnVtQhQKEl9jYWxsZWVfcHJvZmlsZV9pZEINCgtfbWVkaWFfa2luZA==');

@$core.Deprecated('Use acceptCallRequestDescriptor instead')
const AcceptCallRequest$json = {
  '1': 'AcceptCallRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `AcceptCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptCallRequestDescriptor = $convert.base64Decode(
    'ChFBY2NlcHRDYWxsUmVxdWVzdBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQ=');

@$core.Deprecated('Use declineCallRequestDescriptor instead')
const DeclineCallRequest$json = {
  '1': 'DeclineCallRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `DeclineCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineCallRequestDescriptor =
    $convert.base64Decode(
        'ChJEZWNsaW5lQ2FsbFJlcXVlc3QSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlk');

@$core.Deprecated('Use joinCallRequestDescriptor instead')
const JoinCallRequest$json = {
  '1': 'JoinCallRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `JoinCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinCallRequestDescriptor = $convert
    .base64Decode('Cg9Kb2luQ2FsbFJlcXVlc3QSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlk');

@$core.Deprecated('Use leaveCallRequestDescriptor instead')
const LeaveCallRequest$json = {
  '1': 'LeaveCallRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `LeaveCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveCallRequestDescriptor = $convert.base64Decode(
    'ChBMZWF2ZUNhbGxSZXF1ZXN0EhcKB3Jvb21faWQYASABKAlSBnJvb21JZA==');

@$core.Deprecated('Use endCallRequestDescriptor instead')
const EndCallRequest$json = {
  '1': 'EndCallRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `EndCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List endCallRequestDescriptor = $convert
    .base64Decode('Cg5FbmRDYWxsUmVxdWVzdBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQ=');

@$core.Deprecated('Use callSessionDescriptor instead')
const CallSession$json = {
  '1': 'CallSession',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'livekit_room_name', '3': 2, '4': 1, '5': 9, '10': 'livekitRoomName'},
    {'1': 'room_type', '3': 3, '4': 1, '5': 9, '10': 'roomType'},
    {
      '1': 'linked_chat',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '9': 0,
      '10': 'linkedChat',
      '17': true
    },
    {
      '1': 'voice_room_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'voiceRoomId',
      '17': true
    },
    {
      '1': 'started_at',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'startedAt'
    },
    {
      '1': 'room_type_enum',
      '3': 7,
      '4': 1,
      '5': 14,
      '6': '.voice.calls.v1.VoiceSessionKind',
      '9': 2,
      '10': 'roomTypeEnum',
      '17': true
    },
    {
      '1': 'initiator_profile_id',
      '3': 8,
      '4': 1,
      '5': 9,
      '10': 'initiatorProfileId'
    },
    {'1': 'callee_profile_id', '3': 9, '4': 1, '5': 9, '10': 'calleeProfileId'},
    {
      '1': 'media_kind',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.voice.calls.v1.CallMediaKind',
      '10': 'mediaKind'
    },
    {
      '1': 'status',
      '3': 11,
      '4': 1,
      '5': 14,
      '6': '.voice.calls.v1.CallStatus',
      '10': 'status'
    },
    {
      '1': 'expires_at',
      '3': 12,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {
      '1': 'ended_at',
      '3': 13,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'endedAt'
    },
  ],
  '8': [
    {'1': '_linked_chat'},
    {'1': '_voice_room_id'},
    {'1': '_room_type_enum'},
  ],
};

/// Descriptor for `CallSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List callSessionDescriptor = $convert.base64Decode(
    'CgtDYWxsU2Vzc2lvbhIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQSKgoRbGl2ZWtpdF9yb29tX2'
    '5hbWUYAiABKAlSD2xpdmVraXRSb29tTmFtZRIbCglyb29tX3R5cGUYAyABKAlSCHJvb21UeXBl'
    'EjwKC2xpbmtlZF9jaGF0GAQgASgLMhYudm9pY2UuY2hhdC52MS5DaGF0UmVmSABSCmxpbmtlZE'
    'NoYXSIAQESJwoNdm9pY2Vfcm9vbV9pZBgFIAEoCUgBUgt2b2ljZVJvb21JZIgBARI5CgpzdGFy'
    'dGVkX2F0GAYgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJc3RhcnRlZEF0EksKDn'
    'Jvb21fdHlwZV9lbnVtGAcgASgOMiAudm9pY2UuY2FsbHMudjEuVm9pY2VTZXNzaW9uS2luZEgC'
    'Ugxyb29tVHlwZUVudW2IAQESMAoUaW5pdGlhdG9yX3Byb2ZpbGVfaWQYCCABKAlSEmluaXRpYX'
    'RvclByb2ZpbGVJZBIqChFjYWxsZWVfcHJvZmlsZV9pZBgJIAEoCVIPY2FsbGVlUHJvZmlsZUlk'
    'EjwKCm1lZGlhX2tpbmQYCiABKA4yHS52b2ljZS5jYWxscy52MS5DYWxsTWVkaWFLaW5kUgltZW'
    'RpYUtpbmQSMgoGc3RhdHVzGAsgASgOMhoudm9pY2UuY2FsbHMudjEuQ2FsbFN0YXR1c1IGc3Rh'
    'dHVzEjkKCmV4cGlyZXNfYXQYDCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUglleH'
    'BpcmVzQXQSNQoIZW5kZWRfYXQYDSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgdl'
    'bmRlZEF0Qg4KDF9saW5rZWRfY2hhdEIQCg5fdm9pY2Vfcm9vbV9pZEIRCg9fcm9vbV90eXBlX2'
    'VudW0=');

@$core.Deprecated('Use joinVoiceRoomRequestDescriptor instead')
const JoinVoiceRoomRequest$json = {
  '1': 'JoinVoiceRoomRequest',
  '2': [
    {'1': 'voice_room_id', '3': 1, '4': 1, '5': 9, '10': 'voiceRoomId'},
    {
      '1': 'space',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '10': 'space'
    },
  ],
};

/// Descriptor for `JoinVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinVoiceRoomRequestDescriptor = $convert.base64Decode(
    'ChRKb2luVm9pY2VSb29tUmVxdWVzdBIiCg12b2ljZV9yb29tX2lkGAEgASgJUgt2b2ljZVJvb2'
    '1JZBIuCgVzcGFjZRgCIAEoCzIYLnZvaWNlLnNwYWNlLnYxLlNwYWNlUmVmUgVzcGFjZQ==');

@$core.Deprecated('Use leaveVoiceRoomRequestDescriptor instead')
const LeaveVoiceRoomRequest$json = {
  '1': 'LeaveVoiceRoomRequest',
  '2': [
    {'1': 'voice_room_id', '3': 1, '4': 1, '5': 9, '10': 'voiceRoomId'},
  ],
};

/// Descriptor for `LeaveVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveVoiceRoomRequestDescriptor = $convert.base64Decode(
    'ChVMZWF2ZVZvaWNlUm9vbVJlcXVlc3QSIgoNdm9pY2Vfcm9vbV9pZBgBIAEoCVILdm9pY2VSb2'
    '9tSWQ=');

@$core.Deprecated('Use moveToVoiceRoomRequestDescriptor instead')
const MoveToVoiceRoomRequest$json = {
  '1': 'MoveToVoiceRoomRequest',
  '2': [
    {
      '1': 'from_voice_room_id',
      '3': 1,
      '4': 1,
      '5': 9,
      '10': 'fromVoiceRoomId'
    },
    {'1': 'to_voice_room_id', '3': 2, '4': 1, '5': 9, '10': 'toVoiceRoomId'},
    {
      '1': 'space',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '10': 'space'
    },
  ],
};

/// Descriptor for `MoveToVoiceRoomRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List moveToVoiceRoomRequestDescriptor = $convert.base64Decode(
    'ChZNb3ZlVG9Wb2ljZVJvb21SZXF1ZXN0EisKEmZyb21fdm9pY2Vfcm9vbV9pZBgBIAEoCVIPZn'
    'JvbVZvaWNlUm9vbUlkEicKEHRvX3ZvaWNlX3Jvb21faWQYAiABKAlSDXRvVm9pY2VSb29tSWQS'
    'LgoFc3BhY2UYAyABKAsyGC52b2ljZS5zcGFjZS52MS5TcGFjZVJlZlIFc3BhY2U=');

@$core.Deprecated('Use voiceSessionDescriptor instead')
const VoiceSession$json = {
  '1': 'VoiceSession',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'livekit_room_name', '3': 2, '4': 1, '5': 9, '10': 'livekitRoomName'},
    {'1': 'voice_room_id', '3': 3, '4': 1, '5': 9, '10': 'voiceRoomId'},
  ],
};

/// Descriptor for `VoiceSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceSessionDescriptor = $convert.base64Decode(
    'CgxWb2ljZVNlc3Npb24SFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEioKEWxpdmVraXRfcm9vbV'
    '9uYW1lGAIgASgJUg9saXZla2l0Um9vbU5hbWUSIgoNdm9pY2Vfcm9vbV9pZBgDIAEoCVILdm9p'
    'Y2VSb29tSWQ=');

@$core.Deprecated('Use getJoinTokenRequestDescriptor instead')
const GetJoinTokenRequest$json = {
  '1': 'GetJoinTokenRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `GetJoinTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getJoinTokenRequestDescriptor =
    $convert.base64Decode(
        'ChNHZXRKb2luVG9rZW5SZXF1ZXN0EhcKB3Jvb21faWQYASABKAlSBnJvb21JZA==');

@$core.Deprecated('Use updateVoiceStateRequestDescriptor instead')
const UpdateVoiceStateRequest$json = {
  '1': 'UpdateVoiceStateRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {
      '1': 'is_muted',
      '3': 2,
      '4': 1,
      '5': 8,
      '9': 0,
      '10': 'isMuted',
      '17': true
    },
    {
      '1': 'is_deafened',
      '3': 3,
      '4': 1,
      '5': 8,
      '9': 1,
      '10': 'isDeafened',
      '17': true
    },
    {
      '1': 'is_video_on',
      '3': 4,
      '4': 1,
      '5': 8,
      '9': 2,
      '10': 'isVideoOn',
      '17': true
    },
  ],
  '8': [
    {'1': '_is_muted'},
    {'1': '_is_deafened'},
    {'1': '_is_video_on'},
  ],
};

/// Descriptor for `UpdateVoiceStateRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateVoiceStateRequestDescriptor = $convert.base64Decode(
    'ChdVcGRhdGVWb2ljZVN0YXRlUmVxdWVzdBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQSHgoIaX'
    'NfbXV0ZWQYAiABKAhIAFIHaXNNdXRlZIgBARIkCgtpc19kZWFmZW5lZBgDIAEoCEgBUgppc0Rl'
    'YWZlbmVkiAEBEiMKC2lzX3ZpZGVvX29uGAQgASgISAJSCWlzVmlkZW9PbogBAUILCglfaXNfbX'
    'V0ZWRCDgoMX2lzX2RlYWZlbmVkQg4KDF9pc192aWRlb19vbg==');

@$core.Deprecated('Use getVoiceStatesRequestDescriptor instead')
const GetVoiceStatesRequest$json = {
  '1': 'GetVoiceStatesRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {
      '1': 'voice_room_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'voiceRoomId',
      '17': true
    },
  ],
  '8': [
    {'1': '_voice_room_id'},
  ],
};

/// Descriptor for `GetVoiceStatesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceStatesRequestDescriptor = $convert.base64Decode(
    'ChVHZXRWb2ljZVN0YXRlc1JlcXVlc3QSFwoHcm9vbV9pZBgBIAEoCVIGcm9vbUlkEicKDXZvaW'
    'NlX3Jvb21faWQYAiABKAlIAFILdm9pY2VSb29tSWSIAQFCEAoOX3ZvaWNlX3Jvb21faWQ=');

@$core.Deprecated('Use voiceParticipantStateDescriptor instead')
const VoiceParticipantState$json = {
  '1': 'VoiceParticipantState',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'is_muted', '3': 2, '4': 1, '5': 8, '10': 'isMuted'},
    {'1': 'is_deafened', '3': 3, '4': 1, '5': 8, '10': 'isDeafened'},
    {'1': 'is_video_on', '3': 4, '4': 1, '5': 8, '10': 'isVideoOn'},
    {'1': 'is_screen_sharing', '3': 5, '4': 1, '5': 8, '10': 'isScreenSharing'},
    {'1': 'is_commander', '3': 6, '4': 1, '5': 8, '10': 'isCommander'},
    {'1': 'hand_raised', '3': 7, '4': 1, '5': 8, '10': 'handRaised'},
  ],
};

/// Descriptor for `VoiceParticipantState`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List voiceParticipantStateDescriptor = $convert.base64Decode(
    'ChVWb2ljZVBhcnRpY2lwYW50U3RhdGUSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEh'
    'kKCGlzX211dGVkGAIgASgIUgdpc011dGVkEh8KC2lzX2RlYWZlbmVkGAMgASgIUgppc0RlYWZl'
    'bmVkEh4KC2lzX3ZpZGVvX29uGAQgASgIUglpc1ZpZGVvT24SKgoRaXNfc2NyZWVuX3NoYXJpbm'
    'cYBSABKAhSD2lzU2NyZWVuU2hhcmluZxIhCgxpc19jb21tYW5kZXIYBiABKAhSC2lzQ29tbWFu'
    'ZGVyEh8KC2hhbmRfcmFpc2VkGAcgASgIUgpoYW5kUmFpc2Vk');

@$core.Deprecated('Use getActiveCallRequestDescriptor instead')
const GetActiveCallRequest$json = {
  '1': 'GetActiveCallRequest',
};

/// Descriptor for `GetActiveCallRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getActiveCallRequestDescriptor =
    $convert.base64Decode('ChRHZXRBY3RpdmVDYWxsUmVxdWVzdA==');

@$core.Deprecated('Use startScreenShareRequestDescriptor instead')
const StartScreenShareRequest$json = {
  '1': 'StartScreenShareRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `StartScreenShareRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startScreenShareRequestDescriptor =
    $convert.base64Decode(
        'ChdTdGFydFNjcmVlblNoYXJlUmVxdWVzdBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQ=');

@$core.Deprecated('Use screenShareSessionDescriptor instead')
const ScreenShareSession$json = {
  '1': 'ScreenShareSession',
  '2': [
    {'1': 'stream_id', '3': 1, '4': 1, '5': 9, '10': 'streamId'},
  ],
};

/// Descriptor for `ScreenShareSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List screenShareSessionDescriptor =
    $convert.base64Decode(
        'ChJTY3JlZW5TaGFyZVNlc3Npb24SGwoJc3RyZWFtX2lkGAEgASgJUghzdHJlYW1JZA==');

@$core.Deprecated('Use stopScreenShareRequestDescriptor instead')
const StopScreenShareRequest$json = {
  '1': 'StopScreenShareRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {
      '1': 'stream_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'streamId',
      '17': true
    },
  ],
  '8': [
    {'1': '_stream_id'},
  ],
};

/// Descriptor for `StopScreenShareRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List stopScreenShareRequestDescriptor =
    $convert.base64Decode(
        'ChZTdG9wU2NyZWVuU2hhcmVSZXF1ZXN0EhcKB3Jvb21faWQYASABKAlSBnJvb21JZBIgCglzdH'
        'JlYW1faWQYAiABKAlIAFIIc3RyZWFtSWSIAQFCDAoKX3N0cmVhbV9pZA==');

@$core.Deprecated('Use setCommanderModeRequestDescriptor instead')
const SetCommanderModeRequest$json = {
  '1': 'SetCommanderModeRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'enabled', '3': 2, '4': 1, '5': 8, '10': 'enabled'},
  ],
};

/// Descriptor for `SetCommanderModeRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCommanderModeRequestDescriptor =
    $convert.base64Decode(
        'ChdTZXRDb21tYW5kZXJNb2RlUmVxdWVzdBIXCgdyb29tX2lkGAEgASgJUgZyb29tSWQSGAoHZW'
        '5hYmxlZBgCIAEoCFIHZW5hYmxlZA==');

@$core.Deprecated('Use raiseHandRequestDescriptor instead')
const RaiseHandRequest$json = {
  '1': 'RaiseHandRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `RaiseHandRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List raiseHandRequestDescriptor = $convert.base64Decode(
    'ChBSYWlzZUhhbmRSZXF1ZXN0EhcKB3Jvb21faWQYASABKAlSBnJvb21JZA==');

@$core.Deprecated('Use lowerHandRequestDescriptor instead')
const LowerHandRequest$json = {
  '1': 'LowerHandRequest',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `LowerHandRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lowerHandRequestDescriptor = $convert.base64Decode(
    'ChBMb3dlckhhbmRSZXF1ZXN0EhcKB3Jvb21faWQYASABKAlSBnJvb21JZA==');

@$core.Deprecated('Use startCallResponseDescriptor instead')
const StartCallResponse$json = {
  '1': 'StartCallResponse',
  '2': [
    {
      '1': 'call_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.CallSession',
      '10': 'callSession'
    },
  ],
};

/// Descriptor for `StartCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startCallResponseDescriptor = $convert.base64Decode(
    'ChFTdGFydENhbGxSZXNwb25zZRI+CgxjYWxsX3Nlc3Npb24YASABKAsyGy52b2ljZS5jYWxscy'
    '52MS5DYWxsU2Vzc2lvblILY2FsbFNlc3Npb24=');

@$core.Deprecated('Use acceptCallResponseDescriptor instead')
const AcceptCallResponse$json = {
  '1': 'AcceptCallResponse',
  '2': [
    {
      '1': 'call_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.CallSession',
      '10': 'callSession'
    },
  ],
};

/// Descriptor for `AcceptCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List acceptCallResponseDescriptor = $convert.base64Decode(
    'ChJBY2NlcHRDYWxsUmVzcG9uc2USPgoMY2FsbF9zZXNzaW9uGAEgASgLMhsudm9pY2UuY2FsbH'
    'MudjEuQ2FsbFNlc3Npb25SC2NhbGxTZXNzaW9u');

@$core.Deprecated('Use declineCallResponseDescriptor instead')
const DeclineCallResponse$json = {
  '1': 'DeclineCallResponse',
  '2': [
    {
      '1': 'call_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.CallSession',
      '10': 'callSession'
    },
  ],
};

/// Descriptor for `DeclineCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List declineCallResponseDescriptor = $convert.base64Decode(
    'ChNEZWNsaW5lQ2FsbFJlc3BvbnNlEj4KDGNhbGxfc2Vzc2lvbhgBIAEoCzIbLnZvaWNlLmNhbG'
    'xzLnYxLkNhbGxTZXNzaW9uUgtjYWxsU2Vzc2lvbg==');

@$core.Deprecated('Use joinCallResponseDescriptor instead')
const JoinCallResponse$json = {
  '1': 'JoinCallResponse',
  '2': [
    {
      '1': 'call_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.CallSession',
      '10': 'callSession'
    },
  ],
};

/// Descriptor for `JoinCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinCallResponseDescriptor = $convert.base64Decode(
    'ChBKb2luQ2FsbFJlc3BvbnNlEj4KDGNhbGxfc2Vzc2lvbhgBIAEoCzIbLnZvaWNlLmNhbGxzLn'
    'YxLkNhbGxTZXNzaW9uUgtjYWxsU2Vzc2lvbg==');

@$core.Deprecated('Use leaveCallResponseDescriptor instead')
const LeaveCallResponse$json = {
  '1': 'LeaveCallResponse',
};

/// Descriptor for `LeaveCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveCallResponseDescriptor =
    $convert.base64Decode('ChFMZWF2ZUNhbGxSZXNwb25zZQ==');

@$core.Deprecated('Use endCallResponseDescriptor instead')
const EndCallResponse$json = {
  '1': 'EndCallResponse',
};

/// Descriptor for `EndCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List endCallResponseDescriptor =
    $convert.base64Decode('Cg9FbmRDYWxsUmVzcG9uc2U=');

@$core.Deprecated('Use joinVoiceRoomResponseDescriptor instead')
const JoinVoiceRoomResponse$json = {
  '1': 'JoinVoiceRoomResponse',
  '2': [
    {
      '1': 'voice_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.VoiceSession',
      '10': 'voiceSession'
    },
  ],
};

/// Descriptor for `JoinVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List joinVoiceRoomResponseDescriptor = $convert.base64Decode(
    'ChVKb2luVm9pY2VSb29tUmVzcG9uc2USQQoNdm9pY2Vfc2Vzc2lvbhgBIAEoCzIcLnZvaWNlLm'
    'NhbGxzLnYxLlZvaWNlU2Vzc2lvblIMdm9pY2VTZXNzaW9u');

@$core.Deprecated('Use leaveVoiceRoomResponseDescriptor instead')
const LeaveVoiceRoomResponse$json = {
  '1': 'LeaveVoiceRoomResponse',
};

/// Descriptor for `LeaveVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List leaveVoiceRoomResponseDescriptor =
    $convert.base64Decode('ChZMZWF2ZVZvaWNlUm9vbVJlc3BvbnNl');

@$core.Deprecated('Use moveToVoiceRoomResponseDescriptor instead')
const MoveToVoiceRoomResponse$json = {
  '1': 'MoveToVoiceRoomResponse',
  '2': [
    {
      '1': 'voice_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.VoiceSession',
      '10': 'voiceSession'
    },
  ],
};

/// Descriptor for `MoveToVoiceRoomResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List moveToVoiceRoomResponseDescriptor =
    $convert.base64Decode(
        'ChdNb3ZlVG9Wb2ljZVJvb21SZXNwb25zZRJBCg12b2ljZV9zZXNzaW9uGAEgASgLMhwudm9pY2'
        'UuY2FsbHMudjEuVm9pY2VTZXNzaW9uUgx2b2ljZVNlc3Npb24=');

@$core.Deprecated('Use getJoinTokenResponseDescriptor instead')
const GetJoinTokenResponse$json = {
  '1': 'GetJoinTokenResponse',
  '2': [
    {'1': 'jwt', '3': 1, '4': 1, '5': 9, '10': 'jwt'},
    {
      '1': 'expires_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {'1': 'livekit_url', '3': 3, '4': 1, '5': 9, '10': 'livekitUrl'},
  ],
};

/// Descriptor for `GetJoinTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getJoinTokenResponseDescriptor = $convert.base64Decode(
    'ChRHZXRKb2luVG9rZW5SZXNwb25zZRIQCgNqd3QYASABKAlSA2p3dBI5CgpleHBpcmVzX2F0GA'
    'IgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJZXhwaXJlc0F0Eh8KC2xpdmVraXRf'
    'dXJsGAMgASgJUgpsaXZla2l0VXJs');

@$core.Deprecated('Use updateVoiceStateResponseDescriptor instead')
const UpdateVoiceStateResponse$json = {
  '1': 'UpdateVoiceStateResponse',
};

/// Descriptor for `UpdateVoiceStateResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateVoiceStateResponseDescriptor =
    $convert.base64Decode('ChhVcGRhdGVWb2ljZVN0YXRlUmVzcG9uc2U=');

@$core.Deprecated('Use getVoiceStatesResponseDescriptor instead')
const GetVoiceStatesResponse$json = {
  '1': 'GetVoiceStatesResponse',
  '2': [
    {
      '1': 'participants',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.calls.v1.VoiceParticipantState',
      '10': 'participants'
    },
  ],
};

/// Descriptor for `GetVoiceStatesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getVoiceStatesResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRWb2ljZVN0YXRlc1Jlc3BvbnNlEkkKDHBhcnRpY2lwYW50cxgBIAMoCzIlLnZvaWNlLm'
        'NhbGxzLnYxLlZvaWNlUGFydGljaXBhbnRTdGF0ZVIMcGFydGljaXBhbnRz');

@$core.Deprecated('Use getActiveCallResponseDescriptor instead')
const GetActiveCallResponse$json = {
  '1': 'GetActiveCallResponse',
  '2': [
    {
      '1': 'call_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.CallSession',
      '10': 'callSession'
    },
  ],
};

/// Descriptor for `GetActiveCallResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getActiveCallResponseDescriptor = $convert.base64Decode(
    'ChVHZXRBY3RpdmVDYWxsUmVzcG9uc2USPgoMY2FsbF9zZXNzaW9uGAEgASgLMhsudm9pY2UuY2'
    'FsbHMudjEuQ2FsbFNlc3Npb25SC2NhbGxTZXNzaW9u');

@$core.Deprecated('Use startScreenShareResponseDescriptor instead')
const StartScreenShareResponse$json = {
  '1': 'StartScreenShareResponse',
  '2': [
    {
      '1': 'screen_share_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.calls.v1.ScreenShareSession',
      '10': 'screenShareSession'
    },
  ],
};

/// Descriptor for `StartScreenShareResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startScreenShareResponseDescriptor = $convert.base64Decode(
    'ChhTdGFydFNjcmVlblNoYXJlUmVzcG9uc2USVAoUc2NyZWVuX3NoYXJlX3Nlc3Npb24YASABKA'
    'syIi52b2ljZS5jYWxscy52MS5TY3JlZW5TaGFyZVNlc3Npb25SEnNjcmVlblNoYXJlU2Vzc2lv'
    'bg==');

@$core.Deprecated('Use stopScreenShareResponseDescriptor instead')
const StopScreenShareResponse$json = {
  '1': 'StopScreenShareResponse',
};

/// Descriptor for `StopScreenShareResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List stopScreenShareResponseDescriptor =
    $convert.base64Decode('ChdTdG9wU2NyZWVuU2hhcmVSZXNwb25zZQ==');

@$core.Deprecated('Use setCommanderModeResponseDescriptor instead')
const SetCommanderModeResponse$json = {
  '1': 'SetCommanderModeResponse',
};

/// Descriptor for `SetCommanderModeResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setCommanderModeResponseDescriptor =
    $convert.base64Decode('ChhTZXRDb21tYW5kZXJNb2RlUmVzcG9uc2U=');

@$core.Deprecated('Use raiseHandResponseDescriptor instead')
const RaiseHandResponse$json = {
  '1': 'RaiseHandResponse',
};

/// Descriptor for `RaiseHandResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List raiseHandResponseDescriptor =
    $convert.base64Decode('ChFSYWlzZUhhbmRSZXNwb25zZQ==');

@$core.Deprecated('Use lowerHandResponseDescriptor instead')
const LowerHandResponse$json = {
  '1': 'LowerHandResponse',
};

/// Descriptor for `LowerHandResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List lowerHandResponseDescriptor =
    $convert.base64Decode('ChFMb3dlckhhbmRSZXNwb25zZQ==');
