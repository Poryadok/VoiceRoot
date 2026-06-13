// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/s2s.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 's2s.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 's2s.pbenum.dart';

class AuthenticateUserRequest extends $pb.GeneratedMessage {
  factory AuthenticateUserRequest({
    $core.String? authToken,
    $core.String? spaceId,
  }) {
    final result = create();
    if (authToken != null) result.authToken = authToken;
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  AuthenticateUserRequest._();

  factory AuthenticateUserRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthenticateUserRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthenticateUserRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'authToken')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthenticateUserRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthenticateUserRequest copyWith(
          void Function(AuthenticateUserRequest) updates) =>
      super.copyWith((message) => updates(message as AuthenticateUserRequest))
          as AuthenticateUserRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthenticateUserRequest create() => AuthenticateUserRequest._();
  @$core.override
  AuthenticateUserRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthenticateUserRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthenticateUserRequest>(create);
  static AuthenticateUserRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get authToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set authToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAuthToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearAuthToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);
}

class AuthenticateUserResponse extends $pb.GeneratedMessage {
  factory AuthenticateUserResponse({
    $core.bool? ok,
    $core.String? accountId,
    $core.String? profileId,
    $core.Iterable<$core.String>? roleIds,
    $fixnum.Int64? expiresAt,
    $core.String? error,
  }) {
    final result = create();
    if (ok != null) result.ok = ok;
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (roleIds != null) result.roleIds.addAll(roleIds);
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (error != null) result.error = error;
    return result;
  }

  AuthenticateUserResponse._();

  factory AuthenticateUserResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthenticateUserResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthenticateUserResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'ok')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'profileId')
    ..pPS(4, _omitFieldNames ? '' : 'roleIds')
    ..aInt64(5, _omitFieldNames ? '' : 'expiresAt')
    ..aOS(6, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthenticateUserResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthenticateUserResponse copyWith(
          void Function(AuthenticateUserResponse) updates) =>
      super.copyWith((message) => updates(message as AuthenticateUserResponse))
          as AuthenticateUserResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthenticateUserResponse create() => AuthenticateUserResponse._();
  @$core.override
  AuthenticateUserResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthenticateUserResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthenticateUserResponse>(create);
  static AuthenticateUserResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get ok => $_getBF(0);
  @$pb.TagNumber(1)
  set ok($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOk() => $_has(0);
  @$pb.TagNumber(1)
  void clearOk() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get profileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set profileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get roleIds => $_getList(3);

  @$pb.TagNumber(5)
  $fixnum.Int64 get expiresAt => $_getI64(4);
  @$pb.TagNumber(5)
  set expiresAt($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpiresAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpiresAt() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get error => $_getSZ(5);
  @$pb.TagNumber(6)
  set error($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasError() => $_has(5);
  @$pb.TagNumber(6)
  void clearError() => $_clearField(6);
}

enum EventStreamRequest_Payload { subscribe, heartbeat, ack, notSet }

/// Node → master (client stream).
class EventStreamRequest extends $pb.GeneratedMessage {
  factory EventStreamRequest({
    SubscribeRequest? subscribe,
    Heartbeat? heartbeat,
    Ack? ack,
  }) {
    final result = create();
    if (subscribe != null) result.subscribe = subscribe;
    if (heartbeat != null) result.heartbeat = heartbeat;
    if (ack != null) result.ack = ack;
    return result;
  }

  EventStreamRequest._();

  factory EventStreamRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EventStreamRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, EventStreamRequest_Payload>
      _EventStreamRequest_PayloadByTag = {
    1: EventStreamRequest_Payload.subscribe,
    2: EventStreamRequest_Payload.heartbeat,
    3: EventStreamRequest_Payload.ack,
    0: EventStreamRequest_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EventStreamRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2, 3])
    ..aOM<SubscribeRequest>(1, _omitFieldNames ? '' : 'subscribe',
        subBuilder: SubscribeRequest.create)
    ..aOM<Heartbeat>(2, _omitFieldNames ? '' : 'heartbeat',
        subBuilder: Heartbeat.create)
    ..aOM<Ack>(3, _omitFieldNames ? '' : 'ack', subBuilder: Ack.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventStreamRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventStreamRequest copyWith(void Function(EventStreamRequest) updates) =>
      super.copyWith((message) => updates(message as EventStreamRequest))
          as EventStreamRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EventStreamRequest create() => EventStreamRequest._();
  @$core.override
  EventStreamRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EventStreamRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EventStreamRequest>(create);
  static EventStreamRequest? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  EventStreamRequest_Payload whichPayload() =>
      _EventStreamRequest_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  void clearPayload() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  SubscribeRequest get subscribe => $_getN(0);
  @$pb.TagNumber(1)
  set subscribe(SubscribeRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscribe() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscribe() => $_clearField(1);
  @$pb.TagNumber(1)
  SubscribeRequest ensureSubscribe() => $_ensure(0);

  @$pb.TagNumber(2)
  Heartbeat get heartbeat => $_getN(1);
  @$pb.TagNumber(2)
  set heartbeat(Heartbeat value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasHeartbeat() => $_has(1);
  @$pb.TagNumber(2)
  void clearHeartbeat() => $_clearField(2);
  @$pb.TagNumber(2)
  Heartbeat ensureHeartbeat() => $_ensure(1);

  @$pb.TagNumber(3)
  Ack get ack => $_getN(2);
  @$pb.TagNumber(3)
  set ack(Ack value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasAck() => $_has(2);
  @$pb.TagNumber(3)
  void clearAck() => $_clearField(3);
  @$pb.TagNumber(3)
  Ack ensureAck() => $_ensure(2);
}

class SubscribeRequest extends $pb.GeneratedMessage {
  factory SubscribeRequest({
    $core.String? nodeId,
    $core.String? nodeSecret,
    $core.Iterable<$core.String>? spaceIds,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (nodeSecret != null) result.nodeSecret = nodeSecret;
    if (spaceIds != null) result.spaceIds.addAll(spaceIds);
    return result;
  }

  SubscribeRequest._();

  factory SubscribeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SubscribeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubscribeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'nodeSecret')
    ..pPS(3, _omitFieldNames ? '' : 'spaceIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscribeRequest copyWith(void Function(SubscribeRequest) updates) =>
      super.copyWith((message) => updates(message as SubscribeRequest))
          as SubscribeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubscribeRequest create() => SubscribeRequest._();
  @$core.override
  SubscribeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SubscribeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubscribeRequest>(create);
  static SubscribeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get nodeSecret => $_getSZ(1);
  @$pb.TagNumber(2)
  set nodeSecret($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNodeSecret() => $_has(1);
  @$pb.TagNumber(2)
  void clearNodeSecret() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get spaceIds => $_getList(2);
}

class Heartbeat extends $pb.GeneratedMessage {
  factory Heartbeat({
    $fixnum.Int64? timestamp,
  }) {
    final result = create();
    if (timestamp != null) result.timestamp = timestamp;
    return result;
  }

  Heartbeat._();

  factory Heartbeat.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Heartbeat.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Heartbeat',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aInt64(1, _omitFieldNames ? '' : 'timestamp')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Heartbeat clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Heartbeat copyWith(void Function(Heartbeat) updates) =>
      super.copyWith((message) => updates(message as Heartbeat)) as Heartbeat;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Heartbeat create() => Heartbeat._();
  @$core.override
  Heartbeat createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Heartbeat getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Heartbeat>(create);
  static Heartbeat? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get timestamp => $_getI64(0);
  @$pb.TagNumber(1)
  set timestamp($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTimestamp() => $_has(0);
  @$pb.TagNumber(1)
  void clearTimestamp() => $_clearField(1);
}

class Ack extends $pb.GeneratedMessage {
  factory Ack({
    $core.String? eventId,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    return result;
  }

  Ack._();

  factory Ack.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Ack.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Ack',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Ack clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Ack copyWith(void Function(Ack) updates) =>
      super.copyWith((message) => updates(message as Ack)) as Ack;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Ack create() => Ack._();
  @$core.override
  Ack createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Ack getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Ack>(create);
  static Ack? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);
}

enum EventStreamResponse_Payload {
  roleChanged,
  userBanned,
  userUnbanned,
  defederated,
  spaceDeleted,
  notSet
}

/// Master → node (server stream).
class EventStreamResponse extends $pb.GeneratedMessage {
  factory EventStreamResponse({
    $core.String? eventId,
    $fixnum.Int64? timestamp,
    $core.String? spaceId,
    RoleChangedEvent? roleChanged,
    UserBannedEvent? userBanned,
    UserUnbannedEvent? userUnbanned,
    DefederatedEvent? defederated,
    SpaceDeletedEvent? spaceDeleted,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (timestamp != null) result.timestamp = timestamp;
    if (spaceId != null) result.spaceId = spaceId;
    if (roleChanged != null) result.roleChanged = roleChanged;
    if (userBanned != null) result.userBanned = userBanned;
    if (userUnbanned != null) result.userUnbanned = userUnbanned;
    if (defederated != null) result.defederated = defederated;
    if (spaceDeleted != null) result.spaceDeleted = spaceDeleted;
    return result;
  }

  EventStreamResponse._();

  factory EventStreamResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EventStreamResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, EventStreamResponse_Payload>
      _EventStreamResponse_PayloadByTag = {
    10: EventStreamResponse_Payload.roleChanged,
    11: EventStreamResponse_Payload.userBanned,
    12: EventStreamResponse_Payload.userUnbanned,
    13: EventStreamResponse_Payload.defederated,
    14: EventStreamResponse_Payload.spaceDeleted,
    0: EventStreamResponse_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EventStreamResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aInt64(2, _omitFieldNames ? '' : 'timestamp')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOM<RoleChangedEvent>(10, _omitFieldNames ? '' : 'roleChanged',
        subBuilder: RoleChangedEvent.create)
    ..aOM<UserBannedEvent>(11, _omitFieldNames ? '' : 'userBanned',
        subBuilder: UserBannedEvent.create)
    ..aOM<UserUnbannedEvent>(12, _omitFieldNames ? '' : 'userUnbanned',
        subBuilder: UserUnbannedEvent.create)
    ..aOM<DefederatedEvent>(13, _omitFieldNames ? '' : 'defederated',
        subBuilder: DefederatedEvent.create)
    ..aOM<SpaceDeletedEvent>(14, _omitFieldNames ? '' : 'spaceDeleted',
        subBuilder: SpaceDeletedEvent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventStreamResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventStreamResponse copyWith(void Function(EventStreamResponse) updates) =>
      super.copyWith((message) => updates(message as EventStreamResponse))
          as EventStreamResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EventStreamResponse create() => EventStreamResponse._();
  @$core.override
  EventStreamResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EventStreamResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EventStreamResponse>(create);
  static EventStreamResponse? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  EventStreamResponse_Payload whichPayload() =>
      _EventStreamResponse_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  void clearPayload() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get timestamp => $_getI64(1);
  @$pb.TagNumber(2)
  set timestamp($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTimestamp() => $_has(1);
  @$pb.TagNumber(2)
  void clearTimestamp() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(10)
  RoleChangedEvent get roleChanged => $_getN(3);
  @$pb.TagNumber(10)
  set roleChanged(RoleChangedEvent value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasRoleChanged() => $_has(3);
  @$pb.TagNumber(10)
  void clearRoleChanged() => $_clearField(10);
  @$pb.TagNumber(10)
  RoleChangedEvent ensureRoleChanged() => $_ensure(3);

  @$pb.TagNumber(11)
  UserBannedEvent get userBanned => $_getN(4);
  @$pb.TagNumber(11)
  set userBanned(UserBannedEvent value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasUserBanned() => $_has(4);
  @$pb.TagNumber(11)
  void clearUserBanned() => $_clearField(11);
  @$pb.TagNumber(11)
  UserBannedEvent ensureUserBanned() => $_ensure(4);

  @$pb.TagNumber(12)
  UserUnbannedEvent get userUnbanned => $_getN(5);
  @$pb.TagNumber(12)
  set userUnbanned(UserUnbannedEvent value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasUserUnbanned() => $_has(5);
  @$pb.TagNumber(12)
  void clearUserUnbanned() => $_clearField(12);
  @$pb.TagNumber(12)
  UserUnbannedEvent ensureUserUnbanned() => $_ensure(5);

  @$pb.TagNumber(13)
  DefederatedEvent get defederated => $_getN(6);
  @$pb.TagNumber(13)
  set defederated(DefederatedEvent value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasDefederated() => $_has(6);
  @$pb.TagNumber(13)
  void clearDefederated() => $_clearField(13);
  @$pb.TagNumber(13)
  DefederatedEvent ensureDefederated() => $_ensure(6);

  @$pb.TagNumber(14)
  SpaceDeletedEvent get spaceDeleted => $_getN(7);
  @$pb.TagNumber(14)
  set spaceDeleted(SpaceDeletedEvent value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasSpaceDeleted() => $_has(7);
  @$pb.TagNumber(14)
  void clearSpaceDeleted() => $_clearField(14);
  @$pb.TagNumber(14)
  SpaceDeletedEvent ensureSpaceDeleted() => $_ensure(7);
}

class RoleChangedEvent extends $pb.GeneratedMessage {
  factory RoleChangedEvent({
    $core.String? accountId,
    $core.String? profileId,
    $core.Iterable<$core.String>? addedRoles,
    $core.Iterable<$core.String>? removedRoles,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (addedRoles != null) result.addedRoles.addAll(addedRoles);
    if (removedRoles != null) result.removedRoles.addAll(removedRoles);
    return result;
  }

  RoleChangedEvent._();

  factory RoleChangedEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleChangedEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleChangedEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..pPS(3, _omitFieldNames ? '' : 'addedRoles')
    ..pPS(4, _omitFieldNames ? '' : 'removedRoles')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleChangedEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleChangedEvent copyWith(void Function(RoleChangedEvent) updates) =>
      super.copyWith((message) => updates(message as RoleChangedEvent))
          as RoleChangedEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleChangedEvent create() => RoleChangedEvent._();
  @$core.override
  RoleChangedEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleChangedEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RoleChangedEvent>(create);
  static RoleChangedEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get addedRoles => $_getList(2);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get removedRoles => $_getList(3);
}

class UserBannedEvent extends $pb.GeneratedMessage {
  factory UserBannedEvent({
    $core.String? accountId,
    $core.String? profileId,
    $core.String? reason,
    $fixnum.Int64? bannedUntil,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (reason != null) result.reason = reason;
    if (bannedUntil != null) result.bannedUntil = bannedUntil;
    return result;
  }

  UserBannedEvent._();

  factory UserBannedEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserBannedEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserBannedEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..aInt64(4, _omitFieldNames ? '' : 'bannedUntil')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserBannedEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserBannedEvent copyWith(void Function(UserBannedEvent) updates) =>
      super.copyWith((message) => updates(message as UserBannedEvent))
          as UserBannedEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserBannedEvent create() => UserBannedEvent._();
  @$core.override
  UserBannedEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserBannedEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserBannedEvent>(create);
  static UserBannedEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get bannedUntil => $_getI64(3);
  @$pb.TagNumber(4)
  set bannedUntil($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBannedUntil() => $_has(3);
  @$pb.TagNumber(4)
  void clearBannedUntil() => $_clearField(4);
}

class UserUnbannedEvent extends $pb.GeneratedMessage {
  factory UserUnbannedEvent({
    $core.String? accountId,
    $core.String? profileId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  UserUnbannedEvent._();

  factory UserUnbannedEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserUnbannedEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserUnbannedEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserUnbannedEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserUnbannedEvent copyWith(void Function(UserUnbannedEvent) updates) =>
      super.copyWith((message) => updates(message as UserUnbannedEvent))
          as UserUnbannedEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserUnbannedEvent create() => UserUnbannedEvent._();
  @$core.override
  UserUnbannedEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserUnbannedEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserUnbannedEvent>(create);
  static UserUnbannedEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class DefederatedEvent extends $pb.GeneratedMessage {
  factory DefederatedEvent({
    $core.String? reason,
  }) {
    final result = create();
    if (reason != null) result.reason = reason;
    return result;
  }

  DefederatedEvent._();

  factory DefederatedEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DefederatedEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DefederatedEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederatedEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederatedEvent copyWith(void Function(DefederatedEvent) updates) =>
      super.copyWith((message) => updates(message as DefederatedEvent))
          as DefederatedEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DefederatedEvent create() => DefederatedEvent._();
  @$core.override
  DefederatedEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DefederatedEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DefederatedEvent>(create);
  static DefederatedEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reason => $_getSZ(0);
  @$pb.TagNumber(1)
  set reason($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReason() => $_has(0);
  @$pb.TagNumber(1)
  void clearReason() => $_clearField(1);
}

class SpaceDeletedEvent extends $pb.GeneratedMessage {
  factory SpaceDeletedEvent() => create();

  SpaceDeletedEvent._();

  factory SpaceDeletedEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceDeletedEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceDeletedEvent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceDeletedEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceDeletedEvent copyWith(void Function(SpaceDeletedEvent) updates) =>
      super.copyWith((message) => updates(message as SpaceDeletedEvent))
          as SpaceDeletedEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceDeletedEvent create() => SpaceDeletedEvent._();
  @$core.override
  SpaceDeletedEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceDeletedEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceDeletedEvent>(create);
  static SpaceDeletedEvent? _defaultInstance;
}

class SyncSnapshotRequest extends $pb.GeneratedMessage {
  factory SyncSnapshotRequest({
    $core.String? nodeId,
    $core.String? spaceId,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  SyncSnapshotRequest._();

  factory SyncSnapshotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SyncSnapshotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SyncSnapshotRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncSnapshotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncSnapshotRequest copyWith(void Function(SyncSnapshotRequest) updates) =>
      super.copyWith((message) => updates(message as SyncSnapshotRequest))
          as SyncSnapshotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SyncSnapshotRequest create() => SyncSnapshotRequest._();
  @$core.override
  SyncSnapshotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SyncSnapshotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SyncSnapshotRequest>(create);
  static SyncSnapshotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);
}

class SyncSnapshotResponse extends $pb.GeneratedMessage {
  factory SyncSnapshotResponse({
    $core.String? spaceId,
    $core.Iterable<RoleEntry>? roles,
    $core.Iterable<BanEntry>? bans,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (roles != null) result.roles.addAll(roles);
    if (bans != null) result.bans.addAll(bans);
    return result;
  }

  SyncSnapshotResponse._();

  factory SyncSnapshotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SyncSnapshotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SyncSnapshotResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..pPM<RoleEntry>(2, _omitFieldNames ? '' : 'roles',
        subBuilder: RoleEntry.create)
    ..pPM<BanEntry>(3, _omitFieldNames ? '' : 'bans',
        subBuilder: BanEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncSnapshotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncSnapshotResponse copyWith(void Function(SyncSnapshotResponse) updates) =>
      super.copyWith((message) => updates(message as SyncSnapshotResponse))
          as SyncSnapshotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SyncSnapshotResponse create() => SyncSnapshotResponse._();
  @$core.override
  SyncSnapshotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SyncSnapshotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SyncSnapshotResponse>(create);
  static SyncSnapshotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<RoleEntry> get roles => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<BanEntry> get bans => $_getList(2);
}

class RoleEntry extends $pb.GeneratedMessage {
  factory RoleEntry({
    $core.String? accountId,
    $core.String? profileId,
    $core.Iterable<$core.String>? roleIds,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (roleIds != null) result.roleIds.addAll(roleIds);
    return result;
  }

  RoleEntry._();

  factory RoleEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..pPS(3, _omitFieldNames ? '' : 'roleIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleEntry copyWith(void Function(RoleEntry) updates) =>
      super.copyWith((message) => updates(message as RoleEntry)) as RoleEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleEntry create() => RoleEntry._();
  @$core.override
  RoleEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RoleEntry>(create);
  static RoleEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get roleIds => $_getList(2);
}

class BanEntry extends $pb.GeneratedMessage {
  factory BanEntry({
    $core.String? accountId,
    $core.String? profileId,
    $fixnum.Int64? bannedUntil,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (bannedUntil != null) result.bannedUntil = bannedUntil;
    return result;
  }

  BanEntry._();

  factory BanEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aInt64(3, _omitFieldNames ? '' : 'bannedUntil')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanEntry copyWith(void Function(BanEntry) updates) =>
      super.copyWith((message) => updates(message as BanEntry)) as BanEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanEntry create() => BanEntry._();
  @$core.override
  BanEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanEntry getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BanEntry>(create);
  static BanEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get bannedUntil => $_getI64(2);
  @$pb.TagNumber(3)
  set bannedUntil($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBannedUntil() => $_has(2);
  @$pb.TagNumber(3)
  void clearBannedUntil() => $_clearField(3);
}

class NotifyUserRequest extends $pb.GeneratedMessage {
  factory NotifyUserRequest({
    $core.String? accountId,
    $core.String? spaceId,
    $core.String? type,
    $core.String? preview,
    $core.String? deepLink,
    FederationPushEventType? typeEnum,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (spaceId != null) result.spaceId = spaceId;
    if (type != null) result.type = type;
    if (preview != null) result.preview = preview;
    if (deepLink != null) result.deepLink = deepLink;
    if (typeEnum != null) result.typeEnum = typeEnum;
    return result;
  }

  NotifyUserRequest._();

  factory NotifyUserRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NotifyUserRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NotifyUserRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..aOS(4, _omitFieldNames ? '' : 'preview')
    ..aOS(5, _omitFieldNames ? '' : 'deepLink')
    ..aE<FederationPushEventType>(6, _omitFieldNames ? '' : 'typeEnum',
        enumValues: FederationPushEventType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotifyUserRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotifyUserRequest copyWith(void Function(NotifyUserRequest) updates) =>
      super.copyWith((message) => updates(message as NotifyUserRequest))
          as NotifyUserRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NotifyUserRequest create() => NotifyUserRequest._();
  @$core.override
  NotifyUserRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NotifyUserRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NotifyUserRequest>(create);
  static NotifyUserRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get type => $_getSZ(2);
  @$pb.TagNumber(3)
  set type($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get preview => $_getSZ(3);
  @$pb.TagNumber(4)
  set preview($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPreview() => $_has(3);
  @$pb.TagNumber(4)
  void clearPreview() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get deepLink => $_getSZ(4);
  @$pb.TagNumber(5)
  set deepLink($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDeepLink() => $_has(4);
  @$pb.TagNumber(5)
  void clearDeepLink() => $_clearField(5);

  @$pb.TagNumber(6)
  FederationPushEventType get typeEnum => $_getN(5);
  @$pb.TagNumber(6)
  set typeEnum(FederationPushEventType value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasTypeEnum() => $_has(5);
  @$pb.TagNumber(6)
  void clearTypeEnum() => $_clearField(6);
}

class NotifyUserResponse extends $pb.GeneratedMessage {
  factory NotifyUserResponse({
    $core.bool? accepted,
    $core.String? error,
  }) {
    final result = create();
    if (accepted != null) result.accepted = accepted;
    if (error != null) result.error = error;
    return result;
  }

  NotifyUserResponse._();

  factory NotifyUserResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NotifyUserResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NotifyUserResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'accepted')
    ..aOS(2, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotifyUserResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NotifyUserResponse copyWith(void Function(NotifyUserResponse) updates) =>
      super.copyWith((message) => updates(message as NotifyUserResponse))
          as NotifyUserResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NotifyUserResponse create() => NotifyUserResponse._();
  @$core.override
  NotifyUserResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NotifyUserResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NotifyUserResponse>(create);
  static NotifyUserResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get accepted => $_getBF(0);
  @$pb.TagNumber(1)
  set accepted($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccepted() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccepted() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get error => $_getSZ(1);
  @$pb.TagNumber(2)
  set error($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
