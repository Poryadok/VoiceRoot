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

class GameSessionResourceKind extends $pb.ProtobufEnum {
  static const GameSessionResourceKind GAME_SESSION_RESOURCE_KIND_UNSPECIFIED =
      GameSessionResourceKind._(
          0, _omitEnumNames ? '' : 'GAME_SESSION_RESOURCE_KIND_UNSPECIFIED');
  static const GameSessionResourceKind GAME_SESSION_RESOURCE_KIND_PARTY =
      GameSessionResourceKind._(
          1, _omitEnumNames ? '' : 'GAME_SESSION_RESOURCE_KIND_PARTY');
  static const GameSessionResourceKind GAME_SESSION_RESOURCE_KIND_MATCH =
      GameSessionResourceKind._(
          2, _omitEnumNames ? '' : 'GAME_SESSION_RESOURCE_KIND_MATCH');
  static const GameSessionResourceKind
      GAME_SESSION_RESOURCE_KIND_FLEET_SESSION = GameSessionResourceKind._(
          3, _omitEnumNames ? '' : 'GAME_SESSION_RESOURCE_KIND_FLEET_SESSION');

  static const $core.List<GameSessionResourceKind> values =
      <GameSessionResourceKind>[
    GAME_SESSION_RESOURCE_KIND_UNSPECIFIED,
    GAME_SESSION_RESOURCE_KIND_PARTY,
    GAME_SESSION_RESOURCE_KIND_MATCH,
    GAME_SESSION_RESOURCE_KIND_FLEET_SESSION,
  ];

  static final $core.List<GameSessionResourceKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static GameSessionResourceKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const GameSessionResourceKind._(super.value, super.name);
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

class VoiceRoomLifecycleMethod extends $pb.ProtobufEnum {
  static const VoiceRoomLifecycleMethod
      VOICE_ROOM_LIFECYCLE_METHOD_UNSPECIFIED = VoiceRoomLifecycleMethod._(
          0, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_METHOD_UNSPECIFIED');
  static const VoiceRoomLifecycleMethod VOICE_ROOM_LIFECYCLE_METHOD_JOIN =
      VoiceRoomLifecycleMethod._(
          1, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_METHOD_JOIN');
  static const VoiceRoomLifecycleMethod VOICE_ROOM_LIFECYCLE_METHOD_LEAVE =
      VoiceRoomLifecycleMethod._(
          2, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_METHOD_LEAVE');
  static const VoiceRoomLifecycleMethod VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE =
      VoiceRoomLifecycleMethod._(
          3, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE');
  static const VoiceRoomLifecycleMethod
      VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE = VoiceRoomLifecycleMethod._(4,
          _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE');

  static const $core.List<VoiceRoomLifecycleMethod> values =
      <VoiceRoomLifecycleMethod>[
    VOICE_ROOM_LIFECYCLE_METHOD_UNSPECIFIED,
    VOICE_ROOM_LIFECYCLE_METHOD_JOIN,
    VOICE_ROOM_LIFECYCLE_METHOD_LEAVE,
    VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE,
    VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE,
  ];

  static final $core.List<VoiceRoomLifecycleMethod?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static VoiceRoomLifecycleMethod? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VoiceRoomLifecycleMethod._(super.value, super.name);
}

class VoiceRoomLifecycleOutcome extends $pb.ProtobufEnum {
  static const VoiceRoomLifecycleOutcome
      VOICE_ROOM_LIFECYCLE_OUTCOME_UNSPECIFIED = VoiceRoomLifecycleOutcome._(
          0, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_OUTCOME_UNSPECIFIED');
  static const VoiceRoomLifecycleOutcome VOICE_ROOM_LIFECYCLE_OUTCOME_JOINED =
      VoiceRoomLifecycleOutcome._(
          1, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_OUTCOME_JOINED');
  static const VoiceRoomLifecycleOutcome VOICE_ROOM_LIFECYCLE_OUTCOME_LEFT =
      VoiceRoomLifecycleOutcome._(
          2, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_OUTCOME_LEFT');
  static const VoiceRoomLifecycleOutcome VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED =
      VoiceRoomLifecycleOutcome._(
          3, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED');
  static const VoiceRoomLifecycleOutcome VOICE_ROOM_LIFECYCLE_OUTCOME_NO_OP =
      VoiceRoomLifecycleOutcome._(
          4, _omitEnumNames ? '' : 'VOICE_ROOM_LIFECYCLE_OUTCOME_NO_OP');

  static const $core.List<VoiceRoomLifecycleOutcome> values =
      <VoiceRoomLifecycleOutcome>[
    VOICE_ROOM_LIFECYCLE_OUTCOME_UNSPECIFIED,
    VOICE_ROOM_LIFECYCLE_OUTCOME_JOINED,
    VOICE_ROOM_LIFECYCLE_OUTCOME_LEFT,
    VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED,
    VOICE_ROOM_LIFECYCLE_OUTCOME_NO_OP,
  ];

  static final $core.List<VoiceRoomLifecycleOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static VoiceRoomLifecycleOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VoiceRoomLifecycleOutcome._(super.value, super.name);
}

class MatchSquadTeardownStatus extends $pb.ProtobufEnum {
  static const MatchSquadTeardownStatus
      MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED = MatchSquadTeardownStatus._(
          0, _omitEnumNames ? '' : 'MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED');
  static const MatchSquadTeardownStatus MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED =
      MatchSquadTeardownStatus._(
          1, _omitEnumNames ? '' : 'MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED');

  static const $core.List<MatchSquadTeardownStatus> values =
      <MatchSquadTeardownStatus>[
    MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED,
    MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED,
  ];

  static final $core.List<MatchSquadTeardownStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static MatchSquadTeardownStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MatchSquadTeardownStatus._(super.value, super.name);
}

class MatchSquadRoomCompactionStatus extends $pb.ProtobufEnum {
  static const MatchSquadRoomCompactionStatus
      MATCH_SQUAD_ROOM_COMPACTION_STATUS_UNSPECIFIED =
      MatchSquadRoomCompactionStatus._(
          0,
          _omitEnumNames
              ? ''
              : 'MATCH_SQUAD_ROOM_COMPACTION_STATUS_UNSPECIFIED');
  static const MatchSquadRoomCompactionStatus
      MATCH_SQUAD_ROOM_COMPACTION_STATUS_COMPACTED =
      MatchSquadRoomCompactionStatus._(1,
          _omitEnumNames ? '' : 'MATCH_SQUAD_ROOM_COMPACTION_STATUS_COMPACTED');

  static const $core.List<MatchSquadRoomCompactionStatus> values =
      <MatchSquadRoomCompactionStatus>[
    MATCH_SQUAD_ROOM_COMPACTION_STATUS_UNSPECIFIED,
    MATCH_SQUAD_ROOM_COMPACTION_STATUS_COMPACTED,
  ];

  static final $core.List<MatchSquadRoomCompactionStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static MatchSquadRoomCompactionStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MatchSquadRoomCompactionStatus._(super.value, super.name);
}

class MatchSquadMembershipState extends $pb.ProtobufEnum {
  static const MatchSquadMembershipState
      MATCH_SQUAD_MEMBERSHIP_STATE_UNSPECIFIED = MatchSquadMembershipState._(
          0, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_UNSPECIFIED');
  static const MatchSquadMembershipState MATCH_SQUAD_MEMBERSHIP_STATE_JOINING =
      MatchSquadMembershipState._(
          1, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINING');
  static const MatchSquadMembershipState MATCH_SQUAD_MEMBERSHIP_STATE_JOINED =
      MatchSquadMembershipState._(
          2, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_JOINED');
  static const MatchSquadMembershipState
      MATCH_SQUAD_MEMBERSHIP_STATE_RECONNECTING = MatchSquadMembershipState._(
          3, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_RECONNECTING');
  static const MatchSquadMembershipState MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING =
      MatchSquadMembershipState._(
          4, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING');
  static const MatchSquadMembershipState MATCH_SQUAD_MEMBERSHIP_STATE_LEFT =
      MatchSquadMembershipState._(
          5, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_LEFT');
  static const MatchSquadMembershipState MATCH_SQUAD_MEMBERSHIP_STATE_EJECTED =
      MatchSquadMembershipState._(
          6, _omitEnumNames ? '' : 'MATCH_SQUAD_MEMBERSHIP_STATE_EJECTED');

  static const $core.List<MatchSquadMembershipState> values =
      <MatchSquadMembershipState>[
    MATCH_SQUAD_MEMBERSHIP_STATE_UNSPECIFIED,
    MATCH_SQUAD_MEMBERSHIP_STATE_JOINING,
    MATCH_SQUAD_MEMBERSHIP_STATE_JOINED,
    MATCH_SQUAD_MEMBERSHIP_STATE_RECONNECTING,
    MATCH_SQUAD_MEMBERSHIP_STATE_LEAVING,
    MATCH_SQUAD_MEMBERSHIP_STATE_LEFT,
    MATCH_SQUAD_MEMBERSHIP_STATE_EJECTED,
  ];

  static final $core.List<MatchSquadMembershipState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static MatchSquadMembershipState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MatchSquadMembershipState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
