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
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $3;

import '../../chat/v1/chat.pb.dart' as $1;
import '../../space/v1/space.pb.dart' as $2;
import 'calls.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'calls.pbenum.dart';

class StartCallRequest extends $pb.GeneratedMessage {
  factory StartCallRequest({
    $core.String? roomType,
    $1.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $2.SpaceRef? space,
    VoiceSessionKind? roomTypeEnum,
    $core.String? calleeProfileId,
    CallMediaKind? mediaKind,
  }) {
    final result = create();
    if (roomType != null) result.roomType = roomType;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (space != null) result.space = space;
    if (roomTypeEnum != null) result.roomTypeEnum = roomTypeEnum;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    return result;
  }

  StartCallRequest._();

  factory StartCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomType')
    ..aOM<$1.ChatRef>(2, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$2.SpaceRef>(4, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..aE<VoiceSessionKind>(5, _omitFieldNames ? '' : 'roomTypeEnum',
        enumValues: VoiceSessionKind.values)
    ..aOS(6, _omitFieldNames ? '' : 'calleeProfileId')
    ..aE<CallMediaKind>(7, _omitFieldNames ? '' : 'mediaKind',
        enumValues: CallMediaKind.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallRequest copyWith(void Function(StartCallRequest) updates) =>
      super.copyWith((message) => updates(message as StartCallRequest))
          as StartCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartCallRequest create() => StartCallRequest._();
  @$core.override
  StartCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCallRequest>(create);
  static StartCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomType => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomType() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomType() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.ChatRef get linkedChat => $_getN(1);
  @$pb.TagNumber(2)
  set linkedChat($1.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasLinkedChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearLinkedChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.ChatRef ensureLinkedChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get voiceRoomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set voiceRoomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVoiceRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearVoiceRoomId() => $_clearField(3);

  @$pb.TagNumber(4)
  $2.SpaceRef get space => $_getN(3);
  @$pb.TagNumber(4)
  set space($2.SpaceRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasSpace() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpace() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.SpaceRef ensureSpace() => $_ensure(3);

  @$pb.TagNumber(5)
  VoiceSessionKind get roomTypeEnum => $_getN(4);
  @$pb.TagNumber(5)
  set roomTypeEnum(VoiceSessionKind value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasRoomTypeEnum() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoomTypeEnum() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get calleeProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set calleeProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCalleeProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearCalleeProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  CallMediaKind get mediaKind => $_getN(6);
  @$pb.TagNumber(7)
  set mediaKind(CallMediaKind value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasMediaKind() => $_has(6);
  @$pb.TagNumber(7)
  void clearMediaKind() => $_clearField(7);
}

class AcceptCallRequest extends $pb.GeneratedMessage {
  factory AcceptCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  AcceptCallRequest._();

  factory AcceptCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallRequest copyWith(void Function(AcceptCallRequest) updates) =>
      super.copyWith((message) => updates(message as AcceptCallRequest))
          as AcceptCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptCallRequest create() => AcceptCallRequest._();
  @$core.override
  AcceptCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptCallRequest>(create);
  static AcceptCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class DeclineCallRequest extends $pb.GeneratedMessage {
  factory DeclineCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  DeclineCallRequest._();

  factory DeclineCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallRequest copyWith(void Function(DeclineCallRequest) updates) =>
      super.copyWith((message) => updates(message as DeclineCallRequest))
          as DeclineCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineCallRequest create() => DeclineCallRequest._();
  @$core.override
  DeclineCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineCallRequest>(create);
  static DeclineCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class JoinCallRequest extends $pb.GeneratedMessage {
  factory JoinCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  JoinCallRequest._();

  factory JoinCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallRequest copyWith(void Function(JoinCallRequest) updates) =>
      super.copyWith((message) => updates(message as JoinCallRequest))
          as JoinCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinCallRequest create() => JoinCallRequest._();
  @$core.override
  JoinCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinCallRequest>(create);
  static JoinCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class LeaveCallRequest extends $pb.GeneratedMessage {
  factory LeaveCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  LeaveCallRequest._();

  factory LeaveCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallRequest copyWith(void Function(LeaveCallRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveCallRequest))
          as LeaveCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveCallRequest create() => LeaveCallRequest._();
  @$core.override
  LeaveCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveCallRequest>(create);
  static LeaveCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class EndCallRequest extends $pb.GeneratedMessage {
  factory EndCallRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  EndCallRequest._();

  factory EndCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EndCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallRequest copyWith(void Function(EndCallRequest) updates) =>
      super.copyWith((message) => updates(message as EndCallRequest))
          as EndCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EndCallRequest create() => EndCallRequest._();
  @$core.override
  EndCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EndCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCallRequest>(create);
  static EndCallRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class CallSession extends $pb.GeneratedMessage {
  factory CallSession({
    $core.String? roomId,
    $core.String? livekitRoomName,
    $core.String? roomType,
    $1.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $3.Timestamp? startedAt,
    VoiceSessionKind? roomTypeEnum,
    $core.String? initiatorProfileId,
    $core.String? calleeProfileId,
    CallMediaKind? mediaKind,
    CallStatus? status,
    $3.Timestamp? expiresAt,
    $3.Timestamp? endedAt,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (roomType != null) result.roomType = roomType;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (startedAt != null) result.startedAt = startedAt;
    if (roomTypeEnum != null) result.roomTypeEnum = roomTypeEnum;
    if (initiatorProfileId != null)
      result.initiatorProfileId = initiatorProfileId;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    if (status != null) result.status = status;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (endedAt != null) result.endedAt = endedAt;
    return result;
  }

  CallSession._();

  factory CallSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOS(3, _omitFieldNames ? '' : 'roomType')
    ..aOM<$1.ChatRef>(4, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $1.ChatRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$3.Timestamp>(6, _omitFieldNames ? '' : 'startedAt',
        subBuilder: $3.Timestamp.create)
    ..aE<VoiceSessionKind>(7, _omitFieldNames ? '' : 'roomTypeEnum',
        enumValues: VoiceSessionKind.values)
    ..aOS(8, _omitFieldNames ? '' : 'initiatorProfileId')
    ..aOS(9, _omitFieldNames ? '' : 'calleeProfileId')
    ..aE<CallMediaKind>(10, _omitFieldNames ? '' : 'mediaKind',
        enumValues: CallMediaKind.values)
    ..aE<CallStatus>(11, _omitFieldNames ? '' : 'status',
        enumValues: CallStatus.values)
    ..aOM<$3.Timestamp>(12, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..aOM<$3.Timestamp>(13, _omitFieldNames ? '' : 'endedAt',
        subBuilder: $3.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallSession copyWith(void Function(CallSession) updates) =>
      super.copyWith((message) => updates(message as CallSession))
          as CallSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallSession create() => CallSession._();
  @$core.override
  CallSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallSession>(create);
  static CallSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get livekitRoomName => $_getSZ(1);
  @$pb.TagNumber(2)
  set livekitRoomName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLivekitRoomName() => $_has(1);
  @$pb.TagNumber(2)
  void clearLivekitRoomName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roomType => $_getSZ(2);
  @$pb.TagNumber(3)
  set roomType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoomType() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoomType() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.ChatRef get linkedChat => $_getN(3);
  @$pb.TagNumber(4)
  set linkedChat($1.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLinkedChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearLinkedChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.ChatRef ensureLinkedChat() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get voiceRoomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceRoomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceRoomId() => $_clearField(5);

  @$pb.TagNumber(6)
  $3.Timestamp get startedAt => $_getN(5);
  @$pb.TagNumber(6)
  set startedAt($3.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasStartedAt() => $_has(5);
  @$pb.TagNumber(6)
  void clearStartedAt() => $_clearField(6);
  @$pb.TagNumber(6)
  $3.Timestamp ensureStartedAt() => $_ensure(5);

  @$pb.TagNumber(7)
  VoiceSessionKind get roomTypeEnum => $_getN(6);
  @$pb.TagNumber(7)
  set roomTypeEnum(VoiceSessionKind value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasRoomTypeEnum() => $_has(6);
  @$pb.TagNumber(7)
  void clearRoomTypeEnum() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get initiatorProfileId => $_getSZ(7);
  @$pb.TagNumber(8)
  set initiatorProfileId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasInitiatorProfileId() => $_has(7);
  @$pb.TagNumber(8)
  void clearInitiatorProfileId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get calleeProfileId => $_getSZ(8);
  @$pb.TagNumber(9)
  set calleeProfileId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasCalleeProfileId() => $_has(8);
  @$pb.TagNumber(9)
  void clearCalleeProfileId() => $_clearField(9);

  @$pb.TagNumber(10)
  CallMediaKind get mediaKind => $_getN(9);
  @$pb.TagNumber(10)
  set mediaKind(CallMediaKind value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasMediaKind() => $_has(9);
  @$pb.TagNumber(10)
  void clearMediaKind() => $_clearField(10);

  @$pb.TagNumber(11)
  CallStatus get status => $_getN(10);
  @$pb.TagNumber(11)
  set status(CallStatus value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasStatus() => $_has(10);
  @$pb.TagNumber(11)
  void clearStatus() => $_clearField(11);

  @$pb.TagNumber(12)
  $3.Timestamp get expiresAt => $_getN(11);
  @$pb.TagNumber(12)
  set expiresAt($3.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasExpiresAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearExpiresAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $3.Timestamp ensureExpiresAt() => $_ensure(11);

  @$pb.TagNumber(13)
  $3.Timestamp get endedAt => $_getN(12);
  @$pb.TagNumber(13)
  set endedAt($3.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasEndedAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearEndedAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $3.Timestamp ensureEndedAt() => $_ensure(12);
}

class JoinVoiceRoomRequest extends $pb.GeneratedMessage {
  factory JoinVoiceRoomRequest({
    $core.String? voiceRoomId,
    $2.SpaceRef? space,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (space != null) result.space = space;
    return result;
  }

  JoinVoiceRoomRequest._();

  factory JoinVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOM<$2.SpaceRef>(2, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomRequest copyWith(void Function(JoinVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as JoinVoiceRoomRequest))
          as JoinVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomRequest create() => JoinVoiceRoomRequest._();
  @$core.override
  JoinVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinVoiceRoomRequest>(create);
  static JoinVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.SpaceRef get space => $_getN(1);
  @$pb.TagNumber(2)
  set space($2.SpaceRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSpace() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpace() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.SpaceRef ensureSpace() => $_ensure(1);
}

class LeaveVoiceRoomRequest extends $pb.GeneratedMessage {
  factory LeaveVoiceRoomRequest({
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  LeaveVoiceRoomRequest._();

  factory LeaveVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomRequest copyWith(
          void Function(LeaveVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveVoiceRoomRequest))
          as LeaveVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomRequest create() => LeaveVoiceRoomRequest._();
  @$core.override
  LeaveVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveVoiceRoomRequest>(create);
  static LeaveVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);
}

class MoveToVoiceRoomRequest extends $pb.GeneratedMessage {
  factory MoveToVoiceRoomRequest({
    $core.String? fromVoiceRoomId,
    $core.String? toVoiceRoomId,
    $2.SpaceRef? space,
  }) {
    final result = create();
    if (fromVoiceRoomId != null) result.fromVoiceRoomId = fromVoiceRoomId;
    if (toVoiceRoomId != null) result.toVoiceRoomId = toVoiceRoomId;
    if (space != null) result.space = space;
    return result;
  }

  MoveToVoiceRoomRequest._();

  factory MoveToVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveToVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveToVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fromVoiceRoomId')
    ..aOS(2, _omitFieldNames ? '' : 'toVoiceRoomId')
    ..aOM<$2.SpaceRef>(3, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomRequest copyWith(
          void Function(MoveToVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as MoveToVoiceRoomRequest))
          as MoveToVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomRequest create() => MoveToVoiceRoomRequest._();
  @$core.override
  MoveToVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveToVoiceRoomRequest>(create);
  static MoveToVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fromVoiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fromVoiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFromVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFromVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get toVoiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set toVoiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasToVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearToVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.SpaceRef get space => $_getN(2);
  @$pb.TagNumber(3)
  set space($2.SpaceRef value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasSpace() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpace() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.SpaceRef ensureSpace() => $_ensure(2);
}

class VoiceSession extends $pb.GeneratedMessage {
  factory VoiceSession({
    $core.String? roomId,
    $core.String? livekitRoomName,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  VoiceSession._();

  factory VoiceSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOS(3, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceSession copyWith(void Function(VoiceSession) updates) =>
      super.copyWith((message) => updates(message as VoiceSession))
          as VoiceSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceSession create() => VoiceSession._();
  @$core.override
  VoiceSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceSession>(create);
  static VoiceSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get livekitRoomName => $_getSZ(1);
  @$pb.TagNumber(2)
  set livekitRoomName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLivekitRoomName() => $_has(1);
  @$pb.TagNumber(2)
  void clearLivekitRoomName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get voiceRoomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set voiceRoomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVoiceRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearVoiceRoomId() => $_clearField(3);
}

class GetJoinTokenRequest extends $pb.GeneratedMessage {
  factory GetJoinTokenRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  GetJoinTokenRequest._();

  factory GetJoinTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJoinTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJoinTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenRequest copyWith(void Function(GetJoinTokenRequest) updates) =>
      super.copyWith((message) => updates(message as GetJoinTokenRequest))
          as GetJoinTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJoinTokenRequest create() => GetJoinTokenRequest._();
  @$core.override
  GetJoinTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJoinTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJoinTokenRequest>(create);
  static GetJoinTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class UpdateVoiceStateRequest extends $pb.GeneratedMessage {
  factory UpdateVoiceStateRequest({
    $core.String? roomId,
    $core.bool? isMuted,
    $core.bool? isDeafened,
    $core.bool? isVideoOn,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (isMuted != null) result.isMuted = isMuted;
    if (isDeafened != null) result.isDeafened = isDeafened;
    if (isVideoOn != null) result.isVideoOn = isVideoOn;
    return result;
  }

  UpdateVoiceStateRequest._();

  factory UpdateVoiceStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceStateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOB(2, _omitFieldNames ? '' : 'isMuted')
    ..aOB(3, _omitFieldNames ? '' : 'isDeafened')
    ..aOB(4, _omitFieldNames ? '' : 'isVideoOn')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateRequest copyWith(
          void Function(UpdateVoiceStateRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceStateRequest))
          as UpdateVoiceStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateRequest create() => UpdateVoiceStateRequest._();
  @$core.override
  UpdateVoiceStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceStateRequest>(create);
  static UpdateVoiceStateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get isMuted => $_getBF(1);
  @$pb.TagNumber(2)
  set isMuted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIsMuted() => $_has(1);
  @$pb.TagNumber(2)
  void clearIsMuted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isDeafened => $_getBF(2);
  @$pb.TagNumber(3)
  set isDeafened($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsDeafened() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsDeafened() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get isVideoOn => $_getBF(3);
  @$pb.TagNumber(4)
  set isVideoOn($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsVideoOn() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsVideoOn() => $_clearField(4);
}

class GetVoiceStatesRequest extends $pb.GeneratedMessage {
  factory GetVoiceStatesRequest({
    $core.String? roomId,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  GetVoiceStatesRequest._();

  factory GetVoiceStatesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceStatesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceStatesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesRequest copyWith(
          void Function(GetVoiceStatesRequest) updates) =>
      super.copyWith((message) => updates(message as GetVoiceStatesRequest))
          as GetVoiceStatesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesRequest create() => GetVoiceStatesRequest._();
  @$core.override
  GetVoiceStatesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceStatesRequest>(create);
  static GetVoiceStatesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);
}

class VoiceParticipantState extends $pb.GeneratedMessage {
  factory VoiceParticipantState({
    $core.String? profileId,
    $core.bool? isMuted,
    $core.bool? isDeafened,
    $core.bool? isVideoOn,
    $core.bool? isScreenSharing,
    $core.bool? isCommander,
    $core.bool? handRaised,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (isMuted != null) result.isMuted = isMuted;
    if (isDeafened != null) result.isDeafened = isDeafened;
    if (isVideoOn != null) result.isVideoOn = isVideoOn;
    if (isScreenSharing != null) result.isScreenSharing = isScreenSharing;
    if (isCommander != null) result.isCommander = isCommander;
    if (handRaised != null) result.handRaised = handRaised;
    return result;
  }

  VoiceParticipantState._();

  factory VoiceParticipantState.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceParticipantState.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceParticipantState',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOB(2, _omitFieldNames ? '' : 'isMuted')
    ..aOB(3, _omitFieldNames ? '' : 'isDeafened')
    ..aOB(4, _omitFieldNames ? '' : 'isVideoOn')
    ..aOB(5, _omitFieldNames ? '' : 'isScreenSharing')
    ..aOB(6, _omitFieldNames ? '' : 'isCommander')
    ..aOB(7, _omitFieldNames ? '' : 'handRaised')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceParticipantState clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceParticipantState copyWith(
          void Function(VoiceParticipantState) updates) =>
      super.copyWith((message) => updates(message as VoiceParticipantState))
          as VoiceParticipantState;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceParticipantState create() => VoiceParticipantState._();
  @$core.override
  VoiceParticipantState createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceParticipantState getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceParticipantState>(create);
  static VoiceParticipantState? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get isMuted => $_getBF(1);
  @$pb.TagNumber(2)
  set isMuted($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasIsMuted() => $_has(1);
  @$pb.TagNumber(2)
  void clearIsMuted() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isDeafened => $_getBF(2);
  @$pb.TagNumber(3)
  set isDeafened($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsDeafened() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsDeafened() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get isVideoOn => $_getBF(3);
  @$pb.TagNumber(4)
  set isVideoOn($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsVideoOn() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsVideoOn() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get isScreenSharing => $_getBF(4);
  @$pb.TagNumber(5)
  set isScreenSharing($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIsScreenSharing() => $_has(4);
  @$pb.TagNumber(5)
  void clearIsScreenSharing() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get isCommander => $_getBF(5);
  @$pb.TagNumber(6)
  set isCommander($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasIsCommander() => $_has(5);
  @$pb.TagNumber(6)
  void clearIsCommander() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get handRaised => $_getBF(6);
  @$pb.TagNumber(7)
  set handRaised($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasHandRaised() => $_has(6);
  @$pb.TagNumber(7)
  void clearHandRaised() => $_clearField(7);
}

class GetActiveCallRequest extends $pb.GeneratedMessage {
  factory GetActiveCallRequest() => create();

  GetActiveCallRequest._();

  factory GetActiveCallRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveCallRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveCallRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallRequest copyWith(void Function(GetActiveCallRequest) updates) =>
      super.copyWith((message) => updates(message as GetActiveCallRequest))
          as GetActiveCallRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveCallRequest create() => GetActiveCallRequest._();
  @$core.override
  GetActiveCallRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveCallRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveCallRequest>(create);
  static GetActiveCallRequest? _defaultInstance;
}

class StartScreenShareRequest extends $pb.GeneratedMessage {
  factory StartScreenShareRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  StartScreenShareRequest._();

  factory StartScreenShareRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartScreenShareRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartScreenShareRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareRequest copyWith(
          void Function(StartScreenShareRequest) updates) =>
      super.copyWith((message) => updates(message as StartScreenShareRequest))
          as StartScreenShareRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartScreenShareRequest create() => StartScreenShareRequest._();
  @$core.override
  StartScreenShareRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartScreenShareRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartScreenShareRequest>(create);
  static StartScreenShareRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class ScreenShareSession extends $pb.GeneratedMessage {
  factory ScreenShareSession({
    $core.String? streamId,
  }) {
    final result = create();
    if (streamId != null) result.streamId = streamId;
    return result;
  }

  ScreenShareSession._();

  factory ScreenShareSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ScreenShareSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScreenShareSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'streamId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareSession copyWith(void Function(ScreenShareSession) updates) =>
      super.copyWith((message) => updates(message as ScreenShareSession))
          as ScreenShareSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ScreenShareSession create() => ScreenShareSession._();
  @$core.override
  ScreenShareSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ScreenShareSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScreenShareSession>(create);
  static ScreenShareSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get streamId => $_getSZ(0);
  @$pb.TagNumber(1)
  set streamId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStreamId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStreamId() => $_clearField(1);
}

class StopScreenShareRequest extends $pb.GeneratedMessage {
  factory StopScreenShareRequest({
    $core.String? roomId,
    $core.String? streamId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (streamId != null) result.streamId = streamId;
    return result;
  }

  StopScreenShareRequest._();

  factory StopScreenShareRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StopScreenShareRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StopScreenShareRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'streamId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareRequest copyWith(
          void Function(StopScreenShareRequest) updates) =>
      super.copyWith((message) => updates(message as StopScreenShareRequest))
          as StopScreenShareRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StopScreenShareRequest create() => StopScreenShareRequest._();
  @$core.override
  StopScreenShareRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StopScreenShareRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StopScreenShareRequest>(create);
  static StopScreenShareRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get streamId => $_getSZ(1);
  @$pb.TagNumber(2)
  set streamId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStreamId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStreamId() => $_clearField(2);
}

class SetCommanderModeRequest extends $pb.GeneratedMessage {
  factory SetCommanderModeRequest({
    $core.String? roomId,
    $core.bool? enabled,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (enabled != null) result.enabled = enabled;
    return result;
  }

  SetCommanderModeRequest._();

  factory SetCommanderModeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetCommanderModeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCommanderModeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOB(2, _omitFieldNames ? '' : 'enabled')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeRequest copyWith(
          void Function(SetCommanderModeRequest) updates) =>
      super.copyWith((message) => updates(message as SetCommanderModeRequest))
          as SetCommanderModeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetCommanderModeRequest create() => SetCommanderModeRequest._();
  @$core.override
  SetCommanderModeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetCommanderModeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCommanderModeRequest>(create);
  static SetCommanderModeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get enabled => $_getBF(1);
  @$pb.TagNumber(2)
  set enabled($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEnabled() => $_has(1);
  @$pb.TagNumber(2)
  void clearEnabled() => $_clearField(2);
}

class RaiseHandRequest extends $pb.GeneratedMessage {
  factory RaiseHandRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  RaiseHandRequest._();

  factory RaiseHandRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RaiseHandRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RaiseHandRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandRequest copyWith(void Function(RaiseHandRequest) updates) =>
      super.copyWith((message) => updates(message as RaiseHandRequest))
          as RaiseHandRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RaiseHandRequest create() => RaiseHandRequest._();
  @$core.override
  RaiseHandRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RaiseHandRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RaiseHandRequest>(create);
  static RaiseHandRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class LowerHandRequest extends $pb.GeneratedMessage {
  factory LowerHandRequest({
    $core.String? roomId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  LowerHandRequest._();

  factory LowerHandRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LowerHandRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LowerHandRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandRequest copyWith(void Function(LowerHandRequest) updates) =>
      super.copyWith((message) => updates(message as LowerHandRequest))
          as LowerHandRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LowerHandRequest create() => LowerHandRequest._();
  @$core.override
  LowerHandRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LowerHandRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LowerHandRequest>(create);
  static LowerHandRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);
}

class StartCallResponse extends $pb.GeneratedMessage {
  factory StartCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  StartCallResponse._();

  factory StartCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartCallResponse copyWith(void Function(StartCallResponse) updates) =>
      super.copyWith((message) => updates(message as StartCallResponse))
          as StartCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartCallResponse create() => StartCallResponse._();
  @$core.override
  StartCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartCallResponse>(create);
  static StartCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class AcceptCallResponse extends $pb.GeneratedMessage {
  factory AcceptCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  AcceptCallResponse._();

  factory AcceptCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptCallResponse copyWith(void Function(AcceptCallResponse) updates) =>
      super.copyWith((message) => updates(message as AcceptCallResponse))
          as AcceptCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptCallResponse create() => AcceptCallResponse._();
  @$core.override
  AcceptCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptCallResponse>(create);
  static AcceptCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class DeclineCallResponse extends $pb.GeneratedMessage {
  factory DeclineCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  DeclineCallResponse._();

  factory DeclineCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineCallResponse copyWith(void Function(DeclineCallResponse) updates) =>
      super.copyWith((message) => updates(message as DeclineCallResponse))
          as DeclineCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineCallResponse create() => DeclineCallResponse._();
  @$core.override
  DeclineCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineCallResponse>(create);
  static DeclineCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class JoinCallResponse extends $pb.GeneratedMessage {
  factory JoinCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  JoinCallResponse._();

  factory JoinCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinCallResponse copyWith(void Function(JoinCallResponse) updates) =>
      super.copyWith((message) => updates(message as JoinCallResponse))
          as JoinCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinCallResponse create() => JoinCallResponse._();
  @$core.override
  JoinCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinCallResponse>(create);
  static JoinCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class LeaveCallResponse extends $pb.GeneratedMessage {
  factory LeaveCallResponse() => create();

  LeaveCallResponse._();

  factory LeaveCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveCallResponse copyWith(void Function(LeaveCallResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveCallResponse))
          as LeaveCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveCallResponse create() => LeaveCallResponse._();
  @$core.override
  LeaveCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveCallResponse>(create);
  static LeaveCallResponse? _defaultInstance;
}

class EndCallResponse extends $pb.GeneratedMessage {
  factory EndCallResponse() => create();

  EndCallResponse._();

  factory EndCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EndCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EndCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EndCallResponse copyWith(void Function(EndCallResponse) updates) =>
      super.copyWith((message) => updates(message as EndCallResponse))
          as EndCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EndCallResponse create() => EndCallResponse._();
  @$core.override
  EndCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EndCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EndCallResponse>(create);
  static EndCallResponse? _defaultInstance;
}

class JoinVoiceRoomResponse extends $pb.GeneratedMessage {
  factory JoinVoiceRoomResponse({
    VoiceSession? voiceSession,
  }) {
    final result = create();
    if (voiceSession != null) result.voiceSession = voiceSession;
    return result;
  }

  JoinVoiceRoomResponse._();

  factory JoinVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceSession>(1, _omitFieldNames ? '' : 'voiceSession',
        subBuilder: VoiceSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinVoiceRoomResponse copyWith(
          void Function(JoinVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as JoinVoiceRoomResponse))
          as JoinVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomResponse create() => JoinVoiceRoomResponse._();
  @$core.override
  JoinVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinVoiceRoomResponse>(create);
  static JoinVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceSession get voiceSession => $_getN(0);
  @$pb.TagNumber(1)
  set voiceSession(VoiceSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceSession() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceSession ensureVoiceSession() => $_ensure(0);
}

class LeaveVoiceRoomResponse extends $pb.GeneratedMessage {
  factory LeaveVoiceRoomResponse() => create();

  LeaveVoiceRoomResponse._();

  factory LeaveVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveVoiceRoomResponse copyWith(
          void Function(LeaveVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveVoiceRoomResponse))
          as LeaveVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomResponse create() => LeaveVoiceRoomResponse._();
  @$core.override
  LeaveVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveVoiceRoomResponse>(create);
  static LeaveVoiceRoomResponse? _defaultInstance;
}

class MoveToVoiceRoomResponse extends $pb.GeneratedMessage {
  factory MoveToVoiceRoomResponse({
    VoiceSession? voiceSession,
  }) {
    final result = create();
    if (voiceSession != null) result.voiceSession = voiceSession;
    return result;
  }

  MoveToVoiceRoomResponse._();

  factory MoveToVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MoveToVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MoveToVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceSession>(1, _omitFieldNames ? '' : 'voiceSession',
        subBuilder: VoiceSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MoveToVoiceRoomResponse copyWith(
          void Function(MoveToVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as MoveToVoiceRoomResponse))
          as MoveToVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomResponse create() => MoveToVoiceRoomResponse._();
  @$core.override
  MoveToVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MoveToVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MoveToVoiceRoomResponse>(create);
  static MoveToVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceSession get voiceSession => $_getN(0);
  @$pb.TagNumber(1)
  set voiceSession(VoiceSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceSession() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceSession ensureVoiceSession() => $_ensure(0);
}

/// JWT and wall-clock expiry (UTC). Public API uses google.protobuf.Timestamp per docs/REPOSITORIES.md.
class GetJoinTokenResponse extends $pb.GeneratedMessage {
  factory GetJoinTokenResponse({
    $core.String? jwt,
    $3.Timestamp? expiresAt,
    $core.String? livekitUrl,
  }) {
    final result = create();
    if (jwt != null) result.jwt = jwt;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (livekitUrl != null) result.livekitUrl = livekitUrl;
    return result;
  }

  GetJoinTokenResponse._();

  factory GetJoinTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJoinTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJoinTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'jwt')
    ..aOM<$3.Timestamp>(2, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $3.Timestamp.create)
    ..aOS(3, _omitFieldNames ? '' : 'livekitUrl')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJoinTokenResponse copyWith(void Function(GetJoinTokenResponse) updates) =>
      super.copyWith((message) => updates(message as GetJoinTokenResponse))
          as GetJoinTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJoinTokenResponse create() => GetJoinTokenResponse._();
  @$core.override
  GetJoinTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJoinTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJoinTokenResponse>(create);
  static GetJoinTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get jwt => $_getSZ(0);
  @$pb.TagNumber(1)
  set jwt($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasJwt() => $_has(0);
  @$pb.TagNumber(1)
  void clearJwt() => $_clearField(1);

  @$pb.TagNumber(2)
  $3.Timestamp get expiresAt => $_getN(1);
  @$pb.TagNumber(2)
  set expiresAt($3.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasExpiresAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearExpiresAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.Timestamp ensureExpiresAt() => $_ensure(1);

  /// WebSocket URL for LiveKit SDK connect (public ingress; not the internal service URL).
  @$pb.TagNumber(3)
  $core.String get livekitUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set livekitUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLivekitUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearLivekitUrl() => $_clearField(3);
}

class UpdateVoiceStateResponse extends $pb.GeneratedMessage {
  factory UpdateVoiceStateResponse() => create();

  UpdateVoiceStateResponse._();

  factory UpdateVoiceStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceStateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceStateResponse copyWith(
          void Function(UpdateVoiceStateResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceStateResponse))
          as UpdateVoiceStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateResponse create() => UpdateVoiceStateResponse._();
  @$core.override
  UpdateVoiceStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceStateResponse>(create);
  static UpdateVoiceStateResponse? _defaultInstance;
}

class GetVoiceStatesResponse extends $pb.GeneratedMessage {
  factory GetVoiceStatesResponse({
    $core.Iterable<VoiceParticipantState>? participants,
  }) {
    final result = create();
    if (participants != null) result.participants.addAll(participants);
    return result;
  }

  GetVoiceStatesResponse._();

  factory GetVoiceStatesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceStatesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceStatesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..pPM<VoiceParticipantState>(1, _omitFieldNames ? '' : 'participants',
        subBuilder: VoiceParticipantState.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceStatesResponse copyWith(
          void Function(GetVoiceStatesResponse) updates) =>
      super.copyWith((message) => updates(message as GetVoiceStatesResponse))
          as GetVoiceStatesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesResponse create() => GetVoiceStatesResponse._();
  @$core.override
  GetVoiceStatesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceStatesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceStatesResponse>(create);
  static GetVoiceStatesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<VoiceParticipantState> get participants => $_getList(0);
}

class GetActiveCallResponse extends $pb.GeneratedMessage {
  factory GetActiveCallResponse({
    CallSession? callSession,
  }) {
    final result = create();
    if (callSession != null) result.callSession = callSession;
    return result;
  }

  GetActiveCallResponse._();

  factory GetActiveCallResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveCallResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveCallResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<CallSession>(1, _omitFieldNames ? '' : 'callSession',
        subBuilder: CallSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveCallResponse copyWith(
          void Function(GetActiveCallResponse) updates) =>
      super.copyWith((message) => updates(message as GetActiveCallResponse))
          as GetActiveCallResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveCallResponse create() => GetActiveCallResponse._();
  @$core.override
  GetActiveCallResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveCallResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveCallResponse>(create);
  static GetActiveCallResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CallSession get callSession => $_getN(0);
  @$pb.TagNumber(1)
  set callSession(CallSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCallSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearCallSession() => $_clearField(1);
  @$pb.TagNumber(1)
  CallSession ensureCallSession() => $_ensure(0);
}

class StartScreenShareResponse extends $pb.GeneratedMessage {
  factory StartScreenShareResponse({
    ScreenShareSession? screenShareSession,
  }) {
    final result = create();
    if (screenShareSession != null)
      result.screenShareSession = screenShareSession;
    return result;
  }

  StartScreenShareResponse._();

  factory StartScreenShareResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartScreenShareResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartScreenShareResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..aOM<ScreenShareSession>(1, _omitFieldNames ? '' : 'screenShareSession',
        subBuilder: ScreenShareSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartScreenShareResponse copyWith(
          void Function(StartScreenShareResponse) updates) =>
      super.copyWith((message) => updates(message as StartScreenShareResponse))
          as StartScreenShareResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartScreenShareResponse create() => StartScreenShareResponse._();
  @$core.override
  StartScreenShareResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartScreenShareResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartScreenShareResponse>(create);
  static StartScreenShareResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ScreenShareSession get screenShareSession => $_getN(0);
  @$pb.TagNumber(1)
  set screenShareSession(ScreenShareSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScreenShareSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearScreenShareSession() => $_clearField(1);
  @$pb.TagNumber(1)
  ScreenShareSession ensureScreenShareSession() => $_ensure(0);
}

class StopScreenShareResponse extends $pb.GeneratedMessage {
  factory StopScreenShareResponse() => create();

  StopScreenShareResponse._();

  factory StopScreenShareResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StopScreenShareResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StopScreenShareResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StopScreenShareResponse copyWith(
          void Function(StopScreenShareResponse) updates) =>
      super.copyWith((message) => updates(message as StopScreenShareResponse))
          as StopScreenShareResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StopScreenShareResponse create() => StopScreenShareResponse._();
  @$core.override
  StopScreenShareResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StopScreenShareResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StopScreenShareResponse>(create);
  static StopScreenShareResponse? _defaultInstance;
}

class SetCommanderModeResponse extends $pb.GeneratedMessage {
  factory SetCommanderModeResponse() => create();

  SetCommanderModeResponse._();

  factory SetCommanderModeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetCommanderModeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetCommanderModeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetCommanderModeResponse copyWith(
          void Function(SetCommanderModeResponse) updates) =>
      super.copyWith((message) => updates(message as SetCommanderModeResponse))
          as SetCommanderModeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetCommanderModeResponse create() => SetCommanderModeResponse._();
  @$core.override
  SetCommanderModeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetCommanderModeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetCommanderModeResponse>(create);
  static SetCommanderModeResponse? _defaultInstance;
}

class RaiseHandResponse extends $pb.GeneratedMessage {
  factory RaiseHandResponse() => create();

  RaiseHandResponse._();

  factory RaiseHandResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RaiseHandResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RaiseHandResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RaiseHandResponse copyWith(void Function(RaiseHandResponse) updates) =>
      super.copyWith((message) => updates(message as RaiseHandResponse))
          as RaiseHandResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RaiseHandResponse create() => RaiseHandResponse._();
  @$core.override
  RaiseHandResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RaiseHandResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RaiseHandResponse>(create);
  static RaiseHandResponse? _defaultInstance;
}

class LowerHandResponse extends $pb.GeneratedMessage {
  factory LowerHandResponse() => create();

  LowerHandResponse._();

  factory LowerHandResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LowerHandResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LowerHandResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.calls.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LowerHandResponse copyWith(void Function(LowerHandResponse) updates) =>
      super.copyWith((message) => updates(message as LowerHandResponse))
          as LowerHandResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LowerHandResponse create() => LowerHandResponse._();
  @$core.override
  LowerHandResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LowerHandResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LowerHandResponse>(create);
  static LowerHandResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
