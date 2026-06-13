// This is a generated file - do not edit.
//
// Generated from voice/calls/v1/calls.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for room_type strings (StartCallRequest, CallSession).
class VoiceSessionKind extends $pb.ProtobufEnum {
  static const VoiceSessionKind VOICE_SESSION_KIND_UNSPECIFIED =
      VoiceSessionKind._(
          0, _omitEnumNames ? '' : 'VOICE_SESSION_KIND_UNSPECIFIED');
  static const VoiceSessionKind VOICE_SESSION_KIND_CALL =
      VoiceSessionKind._(1, _omitEnumNames ? '' : 'VOICE_SESSION_KIND_CALL');
  static const VoiceSessionKind VOICE_SESSION_KIND_GROUP_VOICE =
      VoiceSessionKind._(
          2, _omitEnumNames ? '' : 'VOICE_SESSION_KIND_GROUP_VOICE');
  static const VoiceSessionKind VOICE_SESSION_KIND_VOICE_ROOM =
      VoiceSessionKind._(
          3, _omitEnumNames ? '' : 'VOICE_SESSION_KIND_VOICE_ROOM');

  static const $core.List<VoiceSessionKind> values = <VoiceSessionKind>[
    VOICE_SESSION_KIND_UNSPECIFIED,
    VOICE_SESSION_KIND_CALL,
    VOICE_SESSION_KIND_GROUP_VOICE,
    VOICE_SESSION_KIND_VOICE_ROOM,
  ];

  static final $core.List<VoiceSessionKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static VoiceSessionKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VoiceSessionKind._(super.value, super.name);
}

class CallMediaKind extends $pb.ProtobufEnum {
  static const CallMediaKind CALL_MEDIA_KIND_UNSPECIFIED =
      CallMediaKind._(0, _omitEnumNames ? '' : 'CALL_MEDIA_KIND_UNSPECIFIED');
  static const CallMediaKind CALL_MEDIA_KIND_AUDIO =
      CallMediaKind._(1, _omitEnumNames ? '' : 'CALL_MEDIA_KIND_AUDIO');
  static const CallMediaKind CALL_MEDIA_KIND_VIDEO =
      CallMediaKind._(2, _omitEnumNames ? '' : 'CALL_MEDIA_KIND_VIDEO');

  static const $core.List<CallMediaKind> values = <CallMediaKind>[
    CALL_MEDIA_KIND_UNSPECIFIED,
    CALL_MEDIA_KIND_AUDIO,
    CALL_MEDIA_KIND_VIDEO,
  ];

  static final $core.List<CallMediaKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static CallMediaKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CallMediaKind._(super.value, super.name);
}

class CallStatus extends $pb.ProtobufEnum {
  static const CallStatus CALL_STATUS_UNSPECIFIED =
      CallStatus._(0, _omitEnumNames ? '' : 'CALL_STATUS_UNSPECIFIED');
  static const CallStatus CALL_STATUS_RINGING =
      CallStatus._(1, _omitEnumNames ? '' : 'CALL_STATUS_RINGING');
  static const CallStatus CALL_STATUS_ACTIVE =
      CallStatus._(2, _omitEnumNames ? '' : 'CALL_STATUS_ACTIVE');
  static const CallStatus CALL_STATUS_DECLINED =
      CallStatus._(3, _omitEnumNames ? '' : 'CALL_STATUS_DECLINED');
  static const CallStatus CALL_STATUS_MISSED =
      CallStatus._(4, _omitEnumNames ? '' : 'CALL_STATUS_MISSED');
  static const CallStatus CALL_STATUS_ENDED =
      CallStatus._(5, _omitEnumNames ? '' : 'CALL_STATUS_ENDED');

  static const $core.List<CallStatus> values = <CallStatus>[
    CALL_STATUS_UNSPECIFIED,
    CALL_STATUS_RINGING,
    CALL_STATUS_ACTIVE,
    CALL_STATUS_DECLINED,
    CALL_STATUS_MISSED,
    CALL_STATUS_ENDED,
  ];

  static final $core.List<CallStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static CallStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CallStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
