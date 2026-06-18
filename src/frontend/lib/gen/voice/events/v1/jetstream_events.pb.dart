// This is a generated file - do not edit.
//
// Generated from voice/events/v1/jetstream_events.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

enum UserStreamEvent_Payload {
  userRegistered,
  userLoggedIn,
  userLoggedOut,
  user2faEnabled,
  userGuestConverted,
  userAccountDeleted,
  userAccountRestored,
  profileCreated,
  profileSwitched,
  settingsChanged,
  presenceChange,
  notSet
}

class UserStreamEvent extends $pb.GeneratedMessage {
  factory UserStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    UserRegistered? userRegistered,
    UserLoggedIn? userLoggedIn,
    UserLoggedOut? userLoggedOut,
    UserTwoFaEnabled? user2faEnabled,
    UserGuestConverted? userGuestConverted,
    UserAccountDeleted? userAccountDeleted,
    UserAccountRestored? userAccountRestored,
    ProfileCreated? profileCreated,
    ProfileSwitched? profileSwitched,
    SettingsChanged? settingsChanged,
    PresenceChange? presenceChange,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (userRegistered != null) result.userRegistered = userRegistered;
    if (userLoggedIn != null) result.userLoggedIn = userLoggedIn;
    if (userLoggedOut != null) result.userLoggedOut = userLoggedOut;
    if (user2faEnabled != null) result.user2faEnabled = user2faEnabled;
    if (userGuestConverted != null)
      result.userGuestConverted = userGuestConverted;
    if (userAccountDeleted != null)
      result.userAccountDeleted = userAccountDeleted;
    if (userAccountRestored != null)
      result.userAccountRestored = userAccountRestored;
    if (profileCreated != null) result.profileCreated = profileCreated;
    if (profileSwitched != null) result.profileSwitched = profileSwitched;
    if (settingsChanged != null) result.settingsChanged = settingsChanged;
    if (presenceChange != null) result.presenceChange = presenceChange;
    return result;
  }

  UserStreamEvent._();

  factory UserStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, UserStreamEvent_Payload>
      _UserStreamEvent_PayloadByTag = {
    10: UserStreamEvent_Payload.userRegistered,
    11: UserStreamEvent_Payload.userLoggedIn,
    12: UserStreamEvent_Payload.userLoggedOut,
    13: UserStreamEvent_Payload.user2faEnabled,
    14: UserStreamEvent_Payload.userGuestConverted,
    15: UserStreamEvent_Payload.userAccountDeleted,
    16: UserStreamEvent_Payload.userAccountRestored,
    17: UserStreamEvent_Payload.profileCreated,
    18: UserStreamEvent_Payload.profileSwitched,
    19: UserStreamEvent_Payload.settingsChanged,
    20: UserStreamEvent_Payload.presenceChange,
    0: UserStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<UserRegistered>(10, _omitFieldNames ? '' : 'userRegistered',
        subBuilder: UserRegistered.create)
    ..aOM<UserLoggedIn>(11, _omitFieldNames ? '' : 'userLoggedIn',
        subBuilder: UserLoggedIn.create)
    ..aOM<UserLoggedOut>(12, _omitFieldNames ? '' : 'userLoggedOut',
        subBuilder: UserLoggedOut.create)
    ..aOM<UserTwoFaEnabled>(13, _omitFieldNames ? '' : 'user2faEnabled',
        protoName: 'user_2fa_enabled', subBuilder: UserTwoFaEnabled.create)
    ..aOM<UserGuestConverted>(14, _omitFieldNames ? '' : 'userGuestConverted',
        subBuilder: UserGuestConverted.create)
    ..aOM<UserAccountDeleted>(15, _omitFieldNames ? '' : 'userAccountDeleted',
        subBuilder: UserAccountDeleted.create)
    ..aOM<UserAccountRestored>(16, _omitFieldNames ? '' : 'userAccountRestored',
        subBuilder: UserAccountRestored.create)
    ..aOM<ProfileCreated>(17, _omitFieldNames ? '' : 'profileCreated',
        subBuilder: ProfileCreated.create)
    ..aOM<ProfileSwitched>(18, _omitFieldNames ? '' : 'profileSwitched',
        subBuilder: ProfileSwitched.create)
    ..aOM<SettingsChanged>(19, _omitFieldNames ? '' : 'settingsChanged',
        subBuilder: SettingsChanged.create)
    ..aOM<PresenceChange>(20, _omitFieldNames ? '' : 'presenceChange',
        subBuilder: PresenceChange.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserStreamEvent copyWith(void Function(UserStreamEvent) updates) =>
      super.copyWith((message) => updates(message as UserStreamEvent))
          as UserStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserStreamEvent create() => UserStreamEvent._();
  @$core.override
  UserStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserStreamEvent>(create);
  static UserStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
  @$pb.TagNumber(19)
  @$pb.TagNumber(20)
  UserStreamEvent_Payload whichPayload() =>
      _UserStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
  @$pb.TagNumber(19)
  @$pb.TagNumber(20)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  UserRegistered get userRegistered => $_getN(2);
  @$pb.TagNumber(10)
  set userRegistered(UserRegistered value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasUserRegistered() => $_has(2);
  @$pb.TagNumber(10)
  void clearUserRegistered() => $_clearField(10);
  @$pb.TagNumber(10)
  UserRegistered ensureUserRegistered() => $_ensure(2);

  @$pb.TagNumber(11)
  UserLoggedIn get userLoggedIn => $_getN(3);
  @$pb.TagNumber(11)
  set userLoggedIn(UserLoggedIn value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasUserLoggedIn() => $_has(3);
  @$pb.TagNumber(11)
  void clearUserLoggedIn() => $_clearField(11);
  @$pb.TagNumber(11)
  UserLoggedIn ensureUserLoggedIn() => $_ensure(3);

  @$pb.TagNumber(12)
  UserLoggedOut get userLoggedOut => $_getN(4);
  @$pb.TagNumber(12)
  set userLoggedOut(UserLoggedOut value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasUserLoggedOut() => $_has(4);
  @$pb.TagNumber(12)
  void clearUserLoggedOut() => $_clearField(12);
  @$pb.TagNumber(12)
  UserLoggedOut ensureUserLoggedOut() => $_ensure(4);

  @$pb.TagNumber(13)
  UserTwoFaEnabled get user2faEnabled => $_getN(5);
  @$pb.TagNumber(13)
  set user2faEnabled(UserTwoFaEnabled value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasUser2faEnabled() => $_has(5);
  @$pb.TagNumber(13)
  void clearUser2faEnabled() => $_clearField(13);
  @$pb.TagNumber(13)
  UserTwoFaEnabled ensureUser2faEnabled() => $_ensure(5);

  @$pb.TagNumber(14)
  UserGuestConverted get userGuestConverted => $_getN(6);
  @$pb.TagNumber(14)
  set userGuestConverted(UserGuestConverted value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasUserGuestConverted() => $_has(6);
  @$pb.TagNumber(14)
  void clearUserGuestConverted() => $_clearField(14);
  @$pb.TagNumber(14)
  UserGuestConverted ensureUserGuestConverted() => $_ensure(6);

  @$pb.TagNumber(15)
  UserAccountDeleted get userAccountDeleted => $_getN(7);
  @$pb.TagNumber(15)
  set userAccountDeleted(UserAccountDeleted value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasUserAccountDeleted() => $_has(7);
  @$pb.TagNumber(15)
  void clearUserAccountDeleted() => $_clearField(15);
  @$pb.TagNumber(15)
  UserAccountDeleted ensureUserAccountDeleted() => $_ensure(7);

  @$pb.TagNumber(16)
  UserAccountRestored get userAccountRestored => $_getN(8);
  @$pb.TagNumber(16)
  set userAccountRestored(UserAccountRestored value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasUserAccountRestored() => $_has(8);
  @$pb.TagNumber(16)
  void clearUserAccountRestored() => $_clearField(16);
  @$pb.TagNumber(16)
  UserAccountRestored ensureUserAccountRestored() => $_ensure(8);

  @$pb.TagNumber(17)
  ProfileCreated get profileCreated => $_getN(9);
  @$pb.TagNumber(17)
  set profileCreated(ProfileCreated value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasProfileCreated() => $_has(9);
  @$pb.TagNumber(17)
  void clearProfileCreated() => $_clearField(17);
  @$pb.TagNumber(17)
  ProfileCreated ensureProfileCreated() => $_ensure(9);

  @$pb.TagNumber(18)
  ProfileSwitched get profileSwitched => $_getN(10);
  @$pb.TagNumber(18)
  set profileSwitched(ProfileSwitched value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasProfileSwitched() => $_has(10);
  @$pb.TagNumber(18)
  void clearProfileSwitched() => $_clearField(18);
  @$pb.TagNumber(18)
  ProfileSwitched ensureProfileSwitched() => $_ensure(10);

  @$pb.TagNumber(19)
  SettingsChanged get settingsChanged => $_getN(11);
  @$pb.TagNumber(19)
  set settingsChanged(SettingsChanged value) => $_setField(19, value);
  @$pb.TagNumber(19)
  $core.bool hasSettingsChanged() => $_has(11);
  @$pb.TagNumber(19)
  void clearSettingsChanged() => $_clearField(19);
  @$pb.TagNumber(19)
  SettingsChanged ensureSettingsChanged() => $_ensure(11);

  @$pb.TagNumber(20)
  PresenceChange get presenceChange => $_getN(12);
  @$pb.TagNumber(20)
  set presenceChange(PresenceChange value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasPresenceChange() => $_has(12);
  @$pb.TagNumber(20)
  void clearPresenceChange() => $_clearField(20);
  @$pb.TagNumber(20)
  PresenceChange ensurePresenceChange() => $_ensure(12);
}

class UserRegistered extends $pb.GeneratedMessage {
  factory UserRegistered({
    $core.String? accountId,
    $core.String? type,
    $core.String? method,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (type != null) result.type = type;
    if (method != null) result.method = method;
    return result;
  }

  UserRegistered._();

  factory UserRegistered.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserRegistered.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserRegistered',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..aOS(3, _omitFieldNames ? '' : 'method')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserRegistered clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserRegistered copyWith(void Function(UserRegistered) updates) =>
      super.copyWith((message) => updates(message as UserRegistered))
          as UserRegistered;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserRegistered create() => UserRegistered._();
  @$core.override
  UserRegistered createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserRegistered getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserRegistered>(create);
  static UserRegistered? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get method => $_getSZ(2);
  @$pb.TagNumber(3)
  set method($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMethod() => $_has(2);
  @$pb.TagNumber(3)
  void clearMethod() => $_clearField(3);
}

class UserLoggedIn extends $pb.GeneratedMessage {
  factory UserLoggedIn({
    $core.String? accountId,
    $core.String? deviceInfoJson,
    $core.String? ip,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (deviceInfoJson != null) result.deviceInfoJson = deviceInfoJson;
    if (ip != null) result.ip = ip;
    return result;
  }

  UserLoggedIn._();

  factory UserLoggedIn.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserLoggedIn.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserLoggedIn',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'deviceInfoJson')
    ..aOS(3, _omitFieldNames ? '' : 'ip')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserLoggedIn clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserLoggedIn copyWith(void Function(UserLoggedIn) updates) =>
      super.copyWith((message) => updates(message as UserLoggedIn))
          as UserLoggedIn;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserLoggedIn create() => UserLoggedIn._();
  @$core.override
  UserLoggedIn createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserLoggedIn getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserLoggedIn>(create);
  static UserLoggedIn? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get deviceInfoJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set deviceInfoJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeviceInfoJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeviceInfoJson() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get ip => $_getSZ(2);
  @$pb.TagNumber(3)
  set ip($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIp() => $_has(2);
  @$pb.TagNumber(3)
  void clearIp() => $_clearField(3);
}

class UserLoggedOut extends $pb.GeneratedMessage {
  factory UserLoggedOut({
    $core.String? accountId,
    $core.String? deviceInfoJson,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (deviceInfoJson != null) result.deviceInfoJson = deviceInfoJson;
    return result;
  }

  UserLoggedOut._();

  factory UserLoggedOut.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserLoggedOut.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserLoggedOut',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'deviceInfoJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserLoggedOut clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserLoggedOut copyWith(void Function(UserLoggedOut) updates) =>
      super.copyWith((message) => updates(message as UserLoggedOut))
          as UserLoggedOut;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserLoggedOut create() => UserLoggedOut._();
  @$core.override
  UserLoggedOut createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserLoggedOut getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserLoggedOut>(create);
  static UserLoggedOut? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get deviceInfoJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set deviceInfoJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeviceInfoJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeviceInfoJson() => $_clearField(2);
}

class UserTwoFaEnabled extends $pb.GeneratedMessage {
  factory UserTwoFaEnabled({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  UserTwoFaEnabled._();

  factory UserTwoFaEnabled.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserTwoFaEnabled.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserTwoFaEnabled',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserTwoFaEnabled clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserTwoFaEnabled copyWith(void Function(UserTwoFaEnabled) updates) =>
      super.copyWith((message) => updates(message as UserTwoFaEnabled))
          as UserTwoFaEnabled;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserTwoFaEnabled create() => UserTwoFaEnabled._();
  @$core.override
  UserTwoFaEnabled createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserTwoFaEnabled getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserTwoFaEnabled>(create);
  static UserTwoFaEnabled? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class UserGuestConverted extends $pb.GeneratedMessage {
  factory UserGuestConverted({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  UserGuestConverted._();

  factory UserGuestConverted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserGuestConverted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserGuestConverted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserGuestConverted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserGuestConverted copyWith(void Function(UserGuestConverted) updates) =>
      super.copyWith((message) => updates(message as UserGuestConverted))
          as UserGuestConverted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserGuestConverted create() => UserGuestConverted._();
  @$core.override
  UserGuestConverted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserGuestConverted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserGuestConverted>(create);
  static UserGuestConverted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class UserAccountDeleted extends $pb.GeneratedMessage {
  factory UserAccountDeleted({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  UserAccountDeleted._();

  factory UserAccountDeleted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserAccountDeleted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserAccountDeleted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserAccountDeleted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserAccountDeleted copyWith(void Function(UserAccountDeleted) updates) =>
      super.copyWith((message) => updates(message as UserAccountDeleted))
          as UserAccountDeleted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserAccountDeleted create() => UserAccountDeleted._();
  @$core.override
  UserAccountDeleted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserAccountDeleted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserAccountDeleted>(create);
  static UserAccountDeleted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class UserAccountRestored extends $pb.GeneratedMessage {
  factory UserAccountRestored({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  UserAccountRestored._();

  factory UserAccountRestored.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserAccountRestored.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserAccountRestored',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserAccountRestored clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserAccountRestored copyWith(void Function(UserAccountRestored) updates) =>
      super.copyWith((message) => updates(message as UserAccountRestored))
          as UserAccountRestored;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserAccountRestored create() => UserAccountRestored._();
  @$core.override
  UserAccountRestored createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserAccountRestored getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserAccountRestored>(create);
  static UserAccountRestored? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class ProfileCreated extends $pb.GeneratedMessage {
  factory ProfileCreated({
    $core.String? profileId,
    $core.String? accountId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  ProfileCreated._();

  factory ProfileCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProfileCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProfileCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileCreated copyWith(void Function(ProfileCreated) updates) =>
      super.copyWith((message) => updates(message as ProfileCreated))
          as ProfileCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProfileCreated create() => ProfileCreated._();
  @$core.override
  ProfileCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProfileCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProfileCreated>(create);
  static ProfileCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);
}

class ProfileSwitched extends $pb.GeneratedMessage {
  factory ProfileSwitched({
    $core.String? profileId,
    $core.String? accountId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  ProfileSwitched._();

  factory ProfileSwitched.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProfileSwitched.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProfileSwitched',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileSwitched clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileSwitched copyWith(void Function(ProfileSwitched) updates) =>
      super.copyWith((message) => updates(message as ProfileSwitched))
          as ProfileSwitched;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProfileSwitched create() => ProfileSwitched._();
  @$core.override
  ProfileSwitched createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProfileSwitched getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProfileSwitched>(create);
  static ProfileSwitched? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);
}

class SettingsChanged extends $pb.GeneratedMessage {
  factory SettingsChanged({
    $core.String? profileId,
    $core.String? changedKeysJson,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (changedKeysJson != null) result.changedKeysJson = changedKeysJson;
    return result;
  }

  SettingsChanged._();

  factory SettingsChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SettingsChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SettingsChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'changedKeysJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SettingsChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SettingsChanged copyWith(void Function(SettingsChanged) updates) =>
      super.copyWith((message) => updates(message as SettingsChanged))
          as SettingsChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SettingsChanged create() => SettingsChanged._();
  @$core.override
  SettingsChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SettingsChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SettingsChanged>(create);
  static SettingsChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get changedKeysJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set changedKeysJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChangedKeysJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearChangedKeysJson() => $_clearField(2);
}

class PresenceChange extends $pb.GeneratedMessage {
  factory PresenceChange({
    $core.String? profileId,
    $core.String? status,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (status != null) result.status = status;
    return result;
  }

  PresenceChange._();

  factory PresenceChange.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PresenceChange.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PresenceChange',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PresenceChange clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PresenceChange copyWith(void Function(PresenceChange) updates) =>
      super.copyWith((message) => updates(message as PresenceChange))
          as PresenceChange;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PresenceChange create() => PresenceChange._();
  @$core.override
  PresenceChange createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PresenceChange getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PresenceChange>(create);
  static PresenceChange? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);
}

enum SocialStreamEvent_Payload {
  friendAdded,
  friendRemoved,
  contactSynced,
  userBlocked,
  notSet
}

class SocialStreamEvent extends $pb.GeneratedMessage {
  factory SocialStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    FriendAdded? friendAdded,
    FriendRemoved? friendRemoved,
    ContactSynced? contactSynced,
    UserBlocked? userBlocked,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (friendAdded != null) result.friendAdded = friendAdded;
    if (friendRemoved != null) result.friendRemoved = friendRemoved;
    if (contactSynced != null) result.contactSynced = contactSynced;
    if (userBlocked != null) result.userBlocked = userBlocked;
    return result;
  }

  SocialStreamEvent._();

  factory SocialStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SocialStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, SocialStreamEvent_Payload>
      _SocialStreamEvent_PayloadByTag = {
    10: SocialStreamEvent_Payload.friendAdded,
    11: SocialStreamEvent_Payload.friendRemoved,
    12: SocialStreamEvent_Payload.contactSynced,
    13: SocialStreamEvent_Payload.userBlocked,
    0: SocialStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SocialStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<FriendAdded>(10, _omitFieldNames ? '' : 'friendAdded',
        subBuilder: FriendAdded.create)
    ..aOM<FriendRemoved>(11, _omitFieldNames ? '' : 'friendRemoved',
        subBuilder: FriendRemoved.create)
    ..aOM<ContactSynced>(12, _omitFieldNames ? '' : 'contactSynced',
        subBuilder: ContactSynced.create)
    ..aOM<UserBlocked>(13, _omitFieldNames ? '' : 'userBlocked',
        subBuilder: UserBlocked.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SocialStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SocialStreamEvent copyWith(void Function(SocialStreamEvent) updates) =>
      super.copyWith((message) => updates(message as SocialStreamEvent))
          as SocialStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SocialStreamEvent create() => SocialStreamEvent._();
  @$core.override
  SocialStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SocialStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SocialStreamEvent>(create);
  static SocialStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  SocialStreamEvent_Payload whichPayload() =>
      _SocialStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  FriendAdded get friendAdded => $_getN(2);
  @$pb.TagNumber(10)
  set friendAdded(FriendAdded value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasFriendAdded() => $_has(2);
  @$pb.TagNumber(10)
  void clearFriendAdded() => $_clearField(10);
  @$pb.TagNumber(10)
  FriendAdded ensureFriendAdded() => $_ensure(2);

  @$pb.TagNumber(11)
  FriendRemoved get friendRemoved => $_getN(3);
  @$pb.TagNumber(11)
  set friendRemoved(FriendRemoved value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasFriendRemoved() => $_has(3);
  @$pb.TagNumber(11)
  void clearFriendRemoved() => $_clearField(11);
  @$pb.TagNumber(11)
  FriendRemoved ensureFriendRemoved() => $_ensure(3);

  @$pb.TagNumber(12)
  ContactSynced get contactSynced => $_getN(4);
  @$pb.TagNumber(12)
  set contactSynced(ContactSynced value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasContactSynced() => $_has(4);
  @$pb.TagNumber(12)
  void clearContactSynced() => $_clearField(12);
  @$pb.TagNumber(12)
  ContactSynced ensureContactSynced() => $_ensure(4);

  @$pb.TagNumber(13)
  UserBlocked get userBlocked => $_getN(5);
  @$pb.TagNumber(13)
  set userBlocked(UserBlocked value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasUserBlocked() => $_has(5);
  @$pb.TagNumber(13)
  void clearUserBlocked() => $_clearField(13);
  @$pb.TagNumber(13)
  UserBlocked ensureUserBlocked() => $_ensure(5);
}

class FriendAdded extends $pb.GeneratedMessage {
  factory FriendAdded({
    $core.String? requesterProfileId,
    $core.String? targetProfileId,
  }) {
    final result = create();
    if (requesterProfileId != null)
      result.requesterProfileId = requesterProfileId;
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    return result;
  }

  FriendAdded._();

  factory FriendAdded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FriendAdded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FriendAdded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'requesterProfileId')
    ..aOS(2, _omitFieldNames ? '' : 'targetProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendAdded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendAdded copyWith(void Function(FriendAdded) updates) =>
      super.copyWith((message) => updates(message as FriendAdded))
          as FriendAdded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FriendAdded create() => FriendAdded._();
  @$core.override
  FriendAdded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FriendAdded getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FriendAdded>(create);
  static FriendAdded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requesterProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requesterProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequesterProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequesterProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get targetProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set targetProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTargetProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTargetProfileId() => $_clearField(2);
}

class FriendRemoved extends $pb.GeneratedMessage {
  factory FriendRemoved({
    $core.String? profileIdA,
    $core.String? profileIdB,
  }) {
    final result = create();
    if (profileIdA != null) result.profileIdA = profileIdA;
    if (profileIdB != null) result.profileIdB = profileIdB;
    return result;
  }

  FriendRemoved._();

  factory FriendRemoved.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FriendRemoved.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FriendRemoved',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileIdA')
    ..aOS(2, _omitFieldNames ? '' : 'profileIdB')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendRemoved clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendRemoved copyWith(void Function(FriendRemoved) updates) =>
      super.copyWith((message) => updates(message as FriendRemoved))
          as FriendRemoved;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FriendRemoved create() => FriendRemoved._();
  @$core.override
  FriendRemoved createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FriendRemoved getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FriendRemoved>(create);
  static FriendRemoved? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileIdA => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileIdA($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileIdA() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileIdA() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileIdB => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileIdB($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileIdB() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileIdB() => $_clearField(2);
}

class ContactSynced extends $pb.GeneratedMessage {
  factory ContactSynced({
    $core.String? ownerProfileId,
    $core.int? count,
  }) {
    final result = create();
    if (ownerProfileId != null) result.ownerProfileId = ownerProfileId;
    if (count != null) result.count = count;
    return result;
  }

  ContactSynced._();

  factory ContactSynced.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ContactSynced.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ContactSynced',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'ownerProfileId')
    ..aI(2, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContactSynced clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContactSynced copyWith(void Function(ContactSynced) updates) =>
      super.copyWith((message) => updates(message as ContactSynced))
          as ContactSynced;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ContactSynced create() => ContactSynced._();
  @$core.override
  ContactSynced createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ContactSynced getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ContactSynced>(create);
  static ContactSynced? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get ownerProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set ownerProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwnerProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwnerProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get count => $_getIZ(1);
  @$pb.TagNumber(2)
  set count($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);
}

class UserBlocked extends $pb.GeneratedMessage {
  factory UserBlocked({
    $core.String? blockerAccountId,
    $core.String? blockedAccountId,
  }) {
    final result = create();
    if (blockerAccountId != null) result.blockerAccountId = blockerAccountId;
    if (blockedAccountId != null) result.blockedAccountId = blockedAccountId;
    return result;
  }

  UserBlocked._();

  factory UserBlocked.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserBlocked.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserBlocked',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'blockerAccountId')
    ..aOS(2, _omitFieldNames ? '' : 'blockedAccountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserBlocked clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserBlocked copyWith(void Function(UserBlocked) updates) =>
      super.copyWith((message) => updates(message as UserBlocked))
          as UserBlocked;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserBlocked create() => UserBlocked._();
  @$core.override
  UserBlocked createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserBlocked getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserBlocked>(create);
  static UserBlocked? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get blockerAccountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set blockerAccountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBlockerAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlockerAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get blockedAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set blockedAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBlockedAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearBlockedAccountId() => $_clearField(2);
}

enum RoleStreamEvent_Payload {
  roleAssignmentChanged,
  roleDefinitionChanged,
  notSet
}

class RoleStreamEvent extends $pb.GeneratedMessage {
  factory RoleStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    RoleAssignmentChanged? roleAssignmentChanged,
    RoleDefinitionChanged? roleDefinitionChanged,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (roleAssignmentChanged != null)
      result.roleAssignmentChanged = roleAssignmentChanged;
    if (roleDefinitionChanged != null)
      result.roleDefinitionChanged = roleDefinitionChanged;
    return result;
  }

  RoleStreamEvent._();

  factory RoleStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, RoleStreamEvent_Payload>
      _RoleStreamEvent_PayloadByTag = {
    10: RoleStreamEvent_Payload.roleAssignmentChanged,
    11: RoleStreamEvent_Payload.roleDefinitionChanged,
    0: RoleStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<RoleAssignmentChanged>(
        10, _omitFieldNames ? '' : 'roleAssignmentChanged',
        subBuilder: RoleAssignmentChanged.create)
    ..aOM<RoleDefinitionChanged>(
        11, _omitFieldNames ? '' : 'roleDefinitionChanged',
        subBuilder: RoleDefinitionChanged.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleStreamEvent copyWith(void Function(RoleStreamEvent) updates) =>
      super.copyWith((message) => updates(message as RoleStreamEvent))
          as RoleStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleStreamEvent create() => RoleStreamEvent._();
  @$core.override
  RoleStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RoleStreamEvent>(create);
  static RoleStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  RoleStreamEvent_Payload whichPayload() =>
      _RoleStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  RoleAssignmentChanged get roleAssignmentChanged => $_getN(2);
  @$pb.TagNumber(10)
  set roleAssignmentChanged(RoleAssignmentChanged value) =>
      $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasRoleAssignmentChanged() => $_has(2);
  @$pb.TagNumber(10)
  void clearRoleAssignmentChanged() => $_clearField(10);
  @$pb.TagNumber(10)
  RoleAssignmentChanged ensureRoleAssignmentChanged() => $_ensure(2);

  @$pb.TagNumber(11)
  RoleDefinitionChanged get roleDefinitionChanged => $_getN(3);
  @$pb.TagNumber(11)
  set roleDefinitionChanged(RoleDefinitionChanged value) =>
      $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasRoleDefinitionChanged() => $_has(3);
  @$pb.TagNumber(11)
  void clearRoleDefinitionChanged() => $_clearField(11);
  @$pb.TagNumber(11)
  RoleDefinitionChanged ensureRoleDefinitionChanged() => $_ensure(3);
}

class RoleAssignmentChanged extends $pb.GeneratedMessage {
  factory RoleAssignmentChanged({
    $core.String? spaceId,
    $core.String? profileId,
    $core.Iterable<$core.String>? roleIds,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (roleIds != null) result.roleIds.addAll(roleIds);
    return result;
  }

  RoleAssignmentChanged._();

  factory RoleAssignmentChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleAssignmentChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleAssignmentChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..pPS(3, _omitFieldNames ? '' : 'roleIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleAssignmentChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleAssignmentChanged copyWith(
          void Function(RoleAssignmentChanged) updates) =>
      super.copyWith((message) => updates(message as RoleAssignmentChanged))
          as RoleAssignmentChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleAssignmentChanged create() => RoleAssignmentChanged._();
  @$core.override
  RoleAssignmentChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleAssignmentChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RoleAssignmentChanged>(create);
  static RoleAssignmentChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

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

class RoleDefinitionChanged extends $pb.GeneratedMessage {
  factory RoleDefinitionChanged({
    $core.String? spaceId,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  RoleDefinitionChanged._();

  factory RoleDefinitionChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleDefinitionChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleDefinitionChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleDefinitionChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleDefinitionChanged copyWith(
          void Function(RoleDefinitionChanged) updates) =>
      super.copyWith((message) => updates(message as RoleDefinitionChanged))
          as RoleDefinitionChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleDefinitionChanged create() => RoleDefinitionChanged._();
  @$core.override
  RoleDefinitionChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleDefinitionChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RoleDefinitionChanged>(create);
  static RoleDefinitionChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get roleId => $_getSZ(1);
  @$pb.TagNumber(2)
  set roleId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRoleId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRoleId() => $_clearField(2);
}

enum MessageStreamEvent_Payload {
  messageSent,
  messageEdited,
  messageDeleted,
  reactionAdded,
  messageRead,
  reactionRemoved,
  mentionAdded,
  messagePinned,
  messageUnpinned,
  notSet
}

class MessageStreamEvent extends $pb.GeneratedMessage {
  factory MessageStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    MessageSent? messageSent,
    MessageEdited? messageEdited,
    MessageDeleted? messageDeleted,
    ReactionAdded? reactionAdded,
    MessageRead? messageRead,
    ReactionRemoved? reactionRemoved,
    MentionAdded? mentionAdded,
    MessagePinned? messagePinned,
    MessageUnpinned? messageUnpinned,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (messageSent != null) result.messageSent = messageSent;
    if (messageEdited != null) result.messageEdited = messageEdited;
    if (messageDeleted != null) result.messageDeleted = messageDeleted;
    if (reactionAdded != null) result.reactionAdded = reactionAdded;
    if (messageRead != null) result.messageRead = messageRead;
    if (reactionRemoved != null) result.reactionRemoved = reactionRemoved;
    if (mentionAdded != null) result.mentionAdded = mentionAdded;
    if (messagePinned != null) result.messagePinned = messagePinned;
    if (messageUnpinned != null) result.messageUnpinned = messageUnpinned;
    return result;
  }

  MessageStreamEvent._();

  factory MessageStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MessageStreamEvent_Payload>
      _MessageStreamEvent_PayloadByTag = {
    10: MessageStreamEvent_Payload.messageSent,
    11: MessageStreamEvent_Payload.messageEdited,
    12: MessageStreamEvent_Payload.messageDeleted,
    13: MessageStreamEvent_Payload.reactionAdded,
    14: MessageStreamEvent_Payload.messageRead,
    15: MessageStreamEvent_Payload.reactionRemoved,
    16: MessageStreamEvent_Payload.mentionAdded,
    17: MessageStreamEvent_Payload.messagePinned,
    18: MessageStreamEvent_Payload.messageUnpinned,
    0: MessageStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14, 15, 16, 17, 18])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<MessageSent>(10, _omitFieldNames ? '' : 'messageSent',
        subBuilder: MessageSent.create)
    ..aOM<MessageEdited>(11, _omitFieldNames ? '' : 'messageEdited',
        subBuilder: MessageEdited.create)
    ..aOM<MessageDeleted>(12, _omitFieldNames ? '' : 'messageDeleted',
        subBuilder: MessageDeleted.create)
    ..aOM<ReactionAdded>(13, _omitFieldNames ? '' : 'reactionAdded',
        subBuilder: ReactionAdded.create)
    ..aOM<MessageRead>(14, _omitFieldNames ? '' : 'messageRead',
        subBuilder: MessageRead.create)
    ..aOM<ReactionRemoved>(15, _omitFieldNames ? '' : 'reactionRemoved',
        subBuilder: ReactionRemoved.create)
    ..aOM<MentionAdded>(16, _omitFieldNames ? '' : 'mentionAdded',
        subBuilder: MentionAdded.create)
    ..aOM<MessagePinned>(17, _omitFieldNames ? '' : 'messagePinned',
        subBuilder: MessagePinned.create)
    ..aOM<MessageUnpinned>(18, _omitFieldNames ? '' : 'messageUnpinned',
        subBuilder: MessageUnpinned.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageStreamEvent copyWith(void Function(MessageStreamEvent) updates) =>
      super.copyWith((message) => updates(message as MessageStreamEvent))
          as MessageStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageStreamEvent create() => MessageStreamEvent._();
  @$core.override
  MessageStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageStreamEvent>(create);
  static MessageStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
  MessageStreamEvent_Payload whichPayload() =>
      _MessageStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  MessageSent get messageSent => $_getN(2);
  @$pb.TagNumber(10)
  set messageSent(MessageSent value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasMessageSent() => $_has(2);
  @$pb.TagNumber(10)
  void clearMessageSent() => $_clearField(10);
  @$pb.TagNumber(10)
  MessageSent ensureMessageSent() => $_ensure(2);

  @$pb.TagNumber(11)
  MessageEdited get messageEdited => $_getN(3);
  @$pb.TagNumber(11)
  set messageEdited(MessageEdited value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasMessageEdited() => $_has(3);
  @$pb.TagNumber(11)
  void clearMessageEdited() => $_clearField(11);
  @$pb.TagNumber(11)
  MessageEdited ensureMessageEdited() => $_ensure(3);

  @$pb.TagNumber(12)
  MessageDeleted get messageDeleted => $_getN(4);
  @$pb.TagNumber(12)
  set messageDeleted(MessageDeleted value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasMessageDeleted() => $_has(4);
  @$pb.TagNumber(12)
  void clearMessageDeleted() => $_clearField(12);
  @$pb.TagNumber(12)
  MessageDeleted ensureMessageDeleted() => $_ensure(4);

  @$pb.TagNumber(13)
  ReactionAdded get reactionAdded => $_getN(5);
  @$pb.TagNumber(13)
  set reactionAdded(ReactionAdded value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasReactionAdded() => $_has(5);
  @$pb.TagNumber(13)
  void clearReactionAdded() => $_clearField(13);
  @$pb.TagNumber(13)
  ReactionAdded ensureReactionAdded() => $_ensure(5);

  @$pb.TagNumber(14)
  MessageRead get messageRead => $_getN(6);
  @$pb.TagNumber(14)
  set messageRead(MessageRead value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasMessageRead() => $_has(6);
  @$pb.TagNumber(14)
  void clearMessageRead() => $_clearField(14);
  @$pb.TagNumber(14)
  MessageRead ensureMessageRead() => $_ensure(6);

  @$pb.TagNumber(15)
  ReactionRemoved get reactionRemoved => $_getN(7);
  @$pb.TagNumber(15)
  set reactionRemoved(ReactionRemoved value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasReactionRemoved() => $_has(7);
  @$pb.TagNumber(15)
  void clearReactionRemoved() => $_clearField(15);
  @$pb.TagNumber(15)
  ReactionRemoved ensureReactionRemoved() => $_ensure(7);

  @$pb.TagNumber(16)
  MentionAdded get mentionAdded => $_getN(8);
  @$pb.TagNumber(16)
  set mentionAdded(MentionAdded value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasMentionAdded() => $_has(8);
  @$pb.TagNumber(16)
  void clearMentionAdded() => $_clearField(16);
  @$pb.TagNumber(16)
  MentionAdded ensureMentionAdded() => $_ensure(8);

  @$pb.TagNumber(17)
  MessagePinned get messagePinned => $_getN(9);
  @$pb.TagNumber(17)
  set messagePinned(MessagePinned value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasMessagePinned() => $_has(9);
  @$pb.TagNumber(17)
  void clearMessagePinned() => $_clearField(17);
  @$pb.TagNumber(17)
  MessagePinned ensureMessagePinned() => $_ensure(9);

  @$pb.TagNumber(18)
  MessageUnpinned get messageUnpinned => $_getN(10);
  @$pb.TagNumber(18)
  set messageUnpinned(MessageUnpinned value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasMessageUnpinned() => $_has(10);
  @$pb.TagNumber(18)
  void clearMessageUnpinned() => $_clearField(18);
  @$pb.TagNumber(18)
  MessageUnpinned ensureMessageUnpinned() => $_ensure(10);
}

class MessageSent extends $pb.GeneratedMessage {
  factory MessageSent({
    $core.String? messageId,
    $core.String? chatId,
    $core.String? senderProfileId,
    $core.bool? hasMentions,
    $core.String? threadParentId,
    $core.bool? isE2e,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    if (hasMentions != null) result.hasMentions = hasMentions;
    if (threadParentId != null) result.threadParentId = threadParentId;
    if (isE2e != null) result.isE2e = isE2e;
    return result;
  }

  MessageSent._();

  factory MessageSent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageSent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageSent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'senderProfileId')
    ..aOB(4, _omitFieldNames ? '' : 'hasMentions')
    ..aOS(5, _omitFieldNames ? '' : 'threadParentId')
    ..aOB(6, _omitFieldNames ? '' : 'isE2e')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageSent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageSent copyWith(void Function(MessageSent) updates) =>
      super.copyWith((message) => updates(message as MessageSent))
          as MessageSent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageSent create() => MessageSent._();
  @$core.override
  MessageSent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageSent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageSent>(create);
  static MessageSent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get senderProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set senderProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSenderProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSenderProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get hasMentions => $_getBF(3);
  @$pb.TagNumber(4)
  set hasMentions($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasHasMentions() => $_has(3);
  @$pb.TagNumber(4)
  void clearHasMentions() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get threadParentId => $_getSZ(4);
  @$pb.TagNumber(5)
  set threadParentId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasThreadParentId() => $_has(4);
  @$pb.TagNumber(5)
  void clearThreadParentId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get isE2e => $_getBF(5);
  @$pb.TagNumber(6)
  set isE2e($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasIsE2e() => $_has(5);
  @$pb.TagNumber(6)
  void clearIsE2e() => $_clearField(6);
}

class MentionAdded extends $pb.GeneratedMessage {
  factory MentionAdded({
    $core.String? messageId,
    $core.String? chatId,
    $core.String? senderProfileId,
    $core.Iterable<$core.String>? mentionedProfileIds,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    if (mentionedProfileIds != null)
      result.mentionedProfileIds.addAll(mentionedProfileIds);
    return result;
  }

  MentionAdded._();

  factory MentionAdded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MentionAdded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MentionAdded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'senderProfileId')
    ..pPS(4, _omitFieldNames ? '' : 'mentionedProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MentionAdded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MentionAdded copyWith(void Function(MentionAdded) updates) =>
      super.copyWith((message) => updates(message as MentionAdded))
          as MentionAdded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MentionAdded create() => MentionAdded._();
  @$core.override
  MentionAdded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MentionAdded getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MentionAdded>(create);
  static MentionAdded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get senderProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set senderProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSenderProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSenderProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get mentionedProfileIds => $_getList(3);
}

class MessageEdited extends $pb.GeneratedMessage {
  factory MessageEdited({
    $core.String? messageId,
    $core.String? chatId,
    $core.bool? isE2e,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (isE2e != null) result.isE2e = isE2e;
    return result;
  }

  MessageEdited._();

  factory MessageEdited.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageEdited.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageEdited',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOB(6, _omitFieldNames ? '' : 'isE2e')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageEdited clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageEdited copyWith(void Function(MessageEdited) updates) =>
      super.copyWith((message) => updates(message as MessageEdited))
          as MessageEdited;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageEdited create() => MessageEdited._();
  @$core.override
  MessageEdited createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageEdited getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageEdited>(create);
  static MessageEdited? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(6)
  $core.bool get isE2e => $_getBF(2);
  @$pb.TagNumber(6)
  set isE2e($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(6)
  $core.bool hasIsE2e() => $_has(2);
  @$pb.TagNumber(6)
  void clearIsE2e() => $_clearField(6);
}

class MessageDeleted extends $pb.GeneratedMessage {
  factory MessageDeleted({
    $core.String? messageId,
    $core.String? chatId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  MessageDeleted._();

  factory MessageDeleted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageDeleted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageDeleted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageDeleted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageDeleted copyWith(void Function(MessageDeleted) updates) =>
      super.copyWith((message) => updates(message as MessageDeleted))
          as MessageDeleted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageDeleted create() => MessageDeleted._();
  @$core.override
  MessageDeleted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageDeleted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageDeleted>(create);
  static MessageDeleted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);
}

class ReactionAdded extends $pb.GeneratedMessage {
  factory ReactionAdded({
    $core.String? messageId,
    $core.String? profileId,
    $core.String? emoji,
    $core.String? chatId,
    $core.String? messageAuthorProfileId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (profileId != null) result.profileId = profileId;
    if (emoji != null) result.emoji = emoji;
    if (chatId != null) result.chatId = chatId;
    if (messageAuthorProfileId != null)
      result.messageAuthorProfileId = messageAuthorProfileId;
    return result;
  }

  ReactionAdded._();

  factory ReactionAdded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReactionAdded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReactionAdded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'emoji')
    ..aOS(4, _omitFieldNames ? '' : 'chatId')
    ..aOS(5, _omitFieldNames ? '' : 'messageAuthorProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactionAdded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactionAdded copyWith(void Function(ReactionAdded) updates) =>
      super.copyWith((message) => updates(message as ReactionAdded))
          as ReactionAdded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReactionAdded create() => ReactionAdded._();
  @$core.override
  ReactionAdded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReactionAdded getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReactionAdded>(create);
  static ReactionAdded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get emoji => $_getSZ(2);
  @$pb.TagNumber(3)
  set emoji($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEmoji() => $_has(2);
  @$pb.TagNumber(3)
  void clearEmoji() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get chatId => $_getSZ(3);
  @$pb.TagNumber(4)
  set chatId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasChatId() => $_has(3);
  @$pb.TagNumber(4)
  void clearChatId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get messageAuthorProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set messageAuthorProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMessageAuthorProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearMessageAuthorProfileId() => $_clearField(5);
}

class ReactionRemoved extends $pb.GeneratedMessage {
  factory ReactionRemoved({
    $core.String? messageId,
    $core.String? profileId,
    $core.String? emoji,
    $core.String? chatId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (profileId != null) result.profileId = profileId;
    if (emoji != null) result.emoji = emoji;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  ReactionRemoved._();

  factory ReactionRemoved.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReactionRemoved.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReactionRemoved',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'emoji')
    ..aOS(4, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactionRemoved clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactionRemoved copyWith(void Function(ReactionRemoved) updates) =>
      super.copyWith((message) => updates(message as ReactionRemoved))
          as ReactionRemoved;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReactionRemoved create() => ReactionRemoved._();
  @$core.override
  ReactionRemoved createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReactionRemoved getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReactionRemoved>(create);
  static ReactionRemoved? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get emoji => $_getSZ(2);
  @$pb.TagNumber(3)
  set emoji($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEmoji() => $_has(2);
  @$pb.TagNumber(3)
  void clearEmoji() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get chatId => $_getSZ(3);
  @$pb.TagNumber(4)
  set chatId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasChatId() => $_has(3);
  @$pb.TagNumber(4)
  void clearChatId() => $_clearField(4);
}

class MessageRead extends $pb.GeneratedMessage {
  factory MessageRead({
    $core.String? messageId,
    $core.String? chatId,
    $core.String? profileId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  MessageRead._();

  factory MessageRead.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageRead.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageRead',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageRead clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageRead copyWith(void Function(MessageRead) updates) =>
      super.copyWith((message) => updates(message as MessageRead))
          as MessageRead;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageRead create() => MessageRead._();
  @$core.override
  MessageRead createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageRead getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageRead>(create);
  static MessageRead? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get profileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set profileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfileId() => $_clearField(3);
}

class MessagePinned extends $pb.GeneratedMessage {
  factory MessagePinned({
    $core.String? messageId,
    $core.String? chatId,
    $core.String? pinnedBy,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (pinnedBy != null) result.pinnedBy = pinnedBy;
    return result;
  }

  MessagePinned._();

  factory MessagePinned.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessagePinned.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessagePinned',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'pinnedBy')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessagePinned clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessagePinned copyWith(void Function(MessagePinned) updates) =>
      super.copyWith((message) => updates(message as MessagePinned))
          as MessagePinned;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessagePinned create() => MessagePinned._();
  @$core.override
  MessagePinned createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessagePinned getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessagePinned>(create);
  static MessagePinned? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pinnedBy => $_getSZ(2);
  @$pb.TagNumber(3)
  set pinnedBy($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPinnedBy() => $_has(2);
  @$pb.TagNumber(3)
  void clearPinnedBy() => $_clearField(3);
}

class MessageUnpinned extends $pb.GeneratedMessage {
  factory MessageUnpinned({
    $core.String? messageId,
    $core.String? chatId,
    $core.String? unpinnedBy,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (chatId != null) result.chatId = chatId;
    if (unpinnedBy != null) result.unpinnedBy = unpinnedBy;
    return result;
  }

  MessageUnpinned._();

  factory MessageUnpinned.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MessageUnpinned.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MessageUnpinned',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'unpinnedBy')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageUnpinned clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MessageUnpinned copyWith(void Function(MessageUnpinned) updates) =>
      super.copyWith((message) => updates(message as MessageUnpinned))
          as MessageUnpinned;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MessageUnpinned create() => MessageUnpinned._();
  @$core.override
  MessageUnpinned createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MessageUnpinned getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MessageUnpinned>(create);
  static MessageUnpinned? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get unpinnedBy => $_getSZ(2);
  @$pb.TagNumber(3)
  set unpinnedBy($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUnpinnedBy() => $_has(2);
  @$pb.TagNumber(3)
  void clearUnpinnedBy() => $_clearField(3);
}

enum ChatStreamEvent_Payload {
  chatCreated,
  chatMemberChanged,
  spaceTreeChanged,
  spaceCreated,
  notSet
}

class ChatStreamEvent extends $pb.GeneratedMessage {
  factory ChatStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    ChatCreated? chatCreated,
    ChatMemberChanged? chatMemberChanged,
    SpaceTreeChanged? spaceTreeChanged,
    SpaceCreated? spaceCreated,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (chatCreated != null) result.chatCreated = chatCreated;
    if (chatMemberChanged != null) result.chatMemberChanged = chatMemberChanged;
    if (spaceTreeChanged != null) result.spaceTreeChanged = spaceTreeChanged;
    if (spaceCreated != null) result.spaceCreated = spaceCreated;
    return result;
  }

  ChatStreamEvent._();

  factory ChatStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ChatStreamEvent_Payload>
      _ChatStreamEvent_PayloadByTag = {
    10: ChatStreamEvent_Payload.chatCreated,
    11: ChatStreamEvent_Payload.chatMemberChanged,
    12: ChatStreamEvent_Payload.spaceTreeChanged,
    13: ChatStreamEvent_Payload.spaceCreated,
    0: ChatStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<ChatCreated>(10, _omitFieldNames ? '' : 'chatCreated',
        subBuilder: ChatCreated.create)
    ..aOM<ChatMemberChanged>(11, _omitFieldNames ? '' : 'chatMemberChanged',
        subBuilder: ChatMemberChanged.create)
    ..aOM<SpaceTreeChanged>(12, _omitFieldNames ? '' : 'spaceTreeChanged',
        subBuilder: SpaceTreeChanged.create)
    ..aOM<SpaceCreated>(13, _omitFieldNames ? '' : 'spaceCreated',
        subBuilder: SpaceCreated.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatStreamEvent copyWith(void Function(ChatStreamEvent) updates) =>
      super.copyWith((message) => updates(message as ChatStreamEvent))
          as ChatStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatStreamEvent create() => ChatStreamEvent._();
  @$core.override
  ChatStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatStreamEvent>(create);
  static ChatStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  ChatStreamEvent_Payload whichPayload() =>
      _ChatStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  ChatCreated get chatCreated => $_getN(2);
  @$pb.TagNumber(10)
  set chatCreated(ChatCreated value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasChatCreated() => $_has(2);
  @$pb.TagNumber(10)
  void clearChatCreated() => $_clearField(10);
  @$pb.TagNumber(10)
  ChatCreated ensureChatCreated() => $_ensure(2);

  @$pb.TagNumber(11)
  ChatMemberChanged get chatMemberChanged => $_getN(3);
  @$pb.TagNumber(11)
  set chatMemberChanged(ChatMemberChanged value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasChatMemberChanged() => $_has(3);
  @$pb.TagNumber(11)
  void clearChatMemberChanged() => $_clearField(11);
  @$pb.TagNumber(11)
  ChatMemberChanged ensureChatMemberChanged() => $_ensure(3);

  @$pb.TagNumber(12)
  SpaceTreeChanged get spaceTreeChanged => $_getN(4);
  @$pb.TagNumber(12)
  set spaceTreeChanged(SpaceTreeChanged value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasSpaceTreeChanged() => $_has(4);
  @$pb.TagNumber(12)
  void clearSpaceTreeChanged() => $_clearField(12);
  @$pb.TagNumber(12)
  SpaceTreeChanged ensureSpaceTreeChanged() => $_ensure(4);

  @$pb.TagNumber(13)
  SpaceCreated get spaceCreated => $_getN(5);
  @$pb.TagNumber(13)
  set spaceCreated(SpaceCreated value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasSpaceCreated() => $_has(5);
  @$pb.TagNumber(13)
  void clearSpaceCreated() => $_clearField(13);
  @$pb.TagNumber(13)
  SpaceCreated ensureSpaceCreated() => $_ensure(5);
}

class ChatCreated extends $pb.GeneratedMessage {
  factory ChatCreated({
    $core.String? chatId,
    $core.String? type,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (type != null) result.type = type;
    return result;
  }

  ChatCreated._();

  factory ChatCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatCreated copyWith(void Function(ChatCreated) updates) =>
      super.copyWith((message) => updates(message as ChatCreated))
          as ChatCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatCreated create() => ChatCreated._();
  @$core.override
  ChatCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatCreated>(create);
  static ChatCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);
}

class ChatMemberChanged extends $pb.GeneratedMessage {
  factory ChatMemberChanged({
    $core.String? chatId,
    $core.String? profileId,
    $core.String? change,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (profileId != null) result.profileId = profileId;
    if (change != null) result.change = change;
    return result;
  }

  ChatMemberChanged._();

  factory ChatMemberChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ChatMemberChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ChatMemberChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'change')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatMemberChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ChatMemberChanged copyWith(void Function(ChatMemberChanged) updates) =>
      super.copyWith((message) => updates(message as ChatMemberChanged))
          as ChatMemberChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ChatMemberChanged create() => ChatMemberChanged._();
  @$core.override
  ChatMemberChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ChatMemberChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ChatMemberChanged>(create);
  static ChatMemberChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get change => $_getSZ(2);
  @$pb.TagNumber(3)
  set change($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChange() => $_has(2);
  @$pb.TagNumber(3)
  void clearChange() => $_clearField(3);
}

class SpaceTreeChanged extends $pb.GeneratedMessage {
  factory SpaceTreeChanged({
    $core.String? spaceId,
    $core.String? nodeId,
    $core.String? change,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (nodeId != null) result.nodeId = nodeId;
    if (change != null) result.change = change;
    return result;
  }

  SpaceTreeChanged._();

  factory SpaceTreeChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceTreeChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceTreeChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'nodeId')
    ..aOS(3, _omitFieldNames ? '' : 'change')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTreeChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTreeChanged copyWith(void Function(SpaceTreeChanged) updates) =>
      super.copyWith((message) => updates(message as SpaceTreeChanged))
          as SpaceTreeChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceTreeChanged create() => SpaceTreeChanged._();
  @$core.override
  SpaceTreeChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceTreeChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceTreeChanged>(create);
  static SpaceTreeChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get nodeId => $_getSZ(1);
  @$pb.TagNumber(2)
  set nodeId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNodeId() => $_has(1);
  @$pb.TagNumber(2)
  void clearNodeId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get change => $_getSZ(2);
  @$pb.TagNumber(3)
  set change($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChange() => $_has(2);
  @$pb.TagNumber(3)
  void clearChange() => $_clearField(3);
}

class SpaceCreated extends $pb.GeneratedMessage {
  factory SpaceCreated({
    $core.String? spaceId,
    $core.String? ownerProfileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (ownerProfileId != null) result.ownerProfileId = ownerProfileId;
    return result;
  }

  SpaceCreated._();

  factory SpaceCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'ownerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceCreated copyWith(void Function(SpaceCreated) updates) =>
      super.copyWith((message) => updates(message as SpaceCreated))
          as SpaceCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceCreated create() => SpaceCreated._();
  @$core.override
  SpaceCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceCreated>(create);
  static SpaceCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get ownerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set ownerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerProfileId() => $_clearField(2);
}

enum VoiceStreamEvent_Payload {
  callStarted,
  callEnded,
  screenShareStarted,
  callIncoming,
  callAccepted,
  callDeclined,
  callMissed,
  voiceStateChanged,
  screenShareStopped,
  notSet
}

class VoiceStreamEvent extends $pb.GeneratedMessage {
  factory VoiceStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    CallStarted? callStarted,
    CallEnded? callEnded,
    ScreenShareStarted? screenShareStarted,
    CallIncoming? callIncoming,
    CallAccepted? callAccepted,
    CallDeclined? callDeclined,
    CallMissed? callMissed,
    VoiceStateChanged? voiceStateChanged,
    ScreenShareStopped? screenShareStopped,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (callStarted != null) result.callStarted = callStarted;
    if (callEnded != null) result.callEnded = callEnded;
    if (screenShareStarted != null)
      result.screenShareStarted = screenShareStarted;
    if (callIncoming != null) result.callIncoming = callIncoming;
    if (callAccepted != null) result.callAccepted = callAccepted;
    if (callDeclined != null) result.callDeclined = callDeclined;
    if (callMissed != null) result.callMissed = callMissed;
    if (voiceStateChanged != null) result.voiceStateChanged = voiceStateChanged;
    if (screenShareStopped != null)
      result.screenShareStopped = screenShareStopped;
    return result;
  }

  VoiceStreamEvent._();

  factory VoiceStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, VoiceStreamEvent_Payload>
      _VoiceStreamEvent_PayloadByTag = {
    10: VoiceStreamEvent_Payload.callStarted,
    11: VoiceStreamEvent_Payload.callEnded,
    12: VoiceStreamEvent_Payload.screenShareStarted,
    13: VoiceStreamEvent_Payload.callIncoming,
    14: VoiceStreamEvent_Payload.callAccepted,
    15: VoiceStreamEvent_Payload.callDeclined,
    16: VoiceStreamEvent_Payload.callMissed,
    17: VoiceStreamEvent_Payload.voiceStateChanged,
    18: VoiceStreamEvent_Payload.screenShareStopped,
    0: VoiceStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14, 15, 16, 17, 18])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<CallStarted>(10, _omitFieldNames ? '' : 'callStarted',
        subBuilder: CallStarted.create)
    ..aOM<CallEnded>(11, _omitFieldNames ? '' : 'callEnded',
        subBuilder: CallEnded.create)
    ..aOM<ScreenShareStarted>(12, _omitFieldNames ? '' : 'screenShareStarted',
        subBuilder: ScreenShareStarted.create)
    ..aOM<CallIncoming>(13, _omitFieldNames ? '' : 'callIncoming',
        subBuilder: CallIncoming.create)
    ..aOM<CallAccepted>(14, _omitFieldNames ? '' : 'callAccepted',
        subBuilder: CallAccepted.create)
    ..aOM<CallDeclined>(15, _omitFieldNames ? '' : 'callDeclined',
        subBuilder: CallDeclined.create)
    ..aOM<CallMissed>(16, _omitFieldNames ? '' : 'callMissed',
        subBuilder: CallMissed.create)
    ..aOM<VoiceStateChanged>(17, _omitFieldNames ? '' : 'voiceStateChanged',
        subBuilder: VoiceStateChanged.create)
    ..aOM<ScreenShareStopped>(18, _omitFieldNames ? '' : 'screenShareStopped',
        subBuilder: ScreenShareStopped.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceStreamEvent copyWith(void Function(VoiceStreamEvent) updates) =>
      super.copyWith((message) => updates(message as VoiceStreamEvent))
          as VoiceStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceStreamEvent create() => VoiceStreamEvent._();
  @$core.override
  VoiceStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceStreamEvent>(create);
  static VoiceStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
  VoiceStreamEvent_Payload whichPayload() =>
      _VoiceStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  @$pb.TagNumber(17)
  @$pb.TagNumber(18)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  CallStarted get callStarted => $_getN(2);
  @$pb.TagNumber(10)
  set callStarted(CallStarted value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasCallStarted() => $_has(2);
  @$pb.TagNumber(10)
  void clearCallStarted() => $_clearField(10);
  @$pb.TagNumber(10)
  CallStarted ensureCallStarted() => $_ensure(2);

  @$pb.TagNumber(11)
  CallEnded get callEnded => $_getN(3);
  @$pb.TagNumber(11)
  set callEnded(CallEnded value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCallEnded() => $_has(3);
  @$pb.TagNumber(11)
  void clearCallEnded() => $_clearField(11);
  @$pb.TagNumber(11)
  CallEnded ensureCallEnded() => $_ensure(3);

  @$pb.TagNumber(12)
  ScreenShareStarted get screenShareStarted => $_getN(4);
  @$pb.TagNumber(12)
  set screenShareStarted(ScreenShareStarted value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasScreenShareStarted() => $_has(4);
  @$pb.TagNumber(12)
  void clearScreenShareStarted() => $_clearField(12);
  @$pb.TagNumber(12)
  ScreenShareStarted ensureScreenShareStarted() => $_ensure(4);

  @$pb.TagNumber(13)
  CallIncoming get callIncoming => $_getN(5);
  @$pb.TagNumber(13)
  set callIncoming(CallIncoming value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasCallIncoming() => $_has(5);
  @$pb.TagNumber(13)
  void clearCallIncoming() => $_clearField(13);
  @$pb.TagNumber(13)
  CallIncoming ensureCallIncoming() => $_ensure(5);

  @$pb.TagNumber(14)
  CallAccepted get callAccepted => $_getN(6);
  @$pb.TagNumber(14)
  set callAccepted(CallAccepted value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasCallAccepted() => $_has(6);
  @$pb.TagNumber(14)
  void clearCallAccepted() => $_clearField(14);
  @$pb.TagNumber(14)
  CallAccepted ensureCallAccepted() => $_ensure(6);

  @$pb.TagNumber(15)
  CallDeclined get callDeclined => $_getN(7);
  @$pb.TagNumber(15)
  set callDeclined(CallDeclined value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasCallDeclined() => $_has(7);
  @$pb.TagNumber(15)
  void clearCallDeclined() => $_clearField(15);
  @$pb.TagNumber(15)
  CallDeclined ensureCallDeclined() => $_ensure(7);

  @$pb.TagNumber(16)
  CallMissed get callMissed => $_getN(8);
  @$pb.TagNumber(16)
  set callMissed(CallMissed value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasCallMissed() => $_has(8);
  @$pb.TagNumber(16)
  void clearCallMissed() => $_clearField(16);
  @$pb.TagNumber(16)
  CallMissed ensureCallMissed() => $_ensure(8);

  @$pb.TagNumber(17)
  VoiceStateChanged get voiceStateChanged => $_getN(9);
  @$pb.TagNumber(17)
  set voiceStateChanged(VoiceStateChanged value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasVoiceStateChanged() => $_has(9);
  @$pb.TagNumber(17)
  void clearVoiceStateChanged() => $_clearField(17);
  @$pb.TagNumber(17)
  VoiceStateChanged ensureVoiceStateChanged() => $_ensure(9);

  @$pb.TagNumber(18)
  ScreenShareStopped get screenShareStopped => $_getN(10);
  @$pb.TagNumber(18)
  set screenShareStopped(ScreenShareStopped value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasScreenShareStopped() => $_has(10);
  @$pb.TagNumber(18)
  void clearScreenShareStopped() => $_clearField(18);
  @$pb.TagNumber(18)
  ScreenShareStopped ensureScreenShareStopped() => $_ensure(10);
}

class CallStarted extends $pb.GeneratedMessage {
  factory CallStarted({
    $core.String? roomId,
    $core.Iterable<$core.String>? profileIds,
    $core.String? chatId,
    $core.String? initiatorProfileId,
    $core.String? calleeProfileId,
    $core.String? mediaKind,
    $core.String? livekitRoomName,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (chatId != null) result.chatId = chatId;
    if (initiatorProfileId != null)
      result.initiatorProfileId = initiatorProfileId;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    return result;
  }

  CallStarted._();

  factory CallStarted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallStarted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallStarted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..pPS(2, _omitFieldNames ? '' : 'profileIds')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'initiatorProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'calleeProfileId')
    ..aOS(6, _omitFieldNames ? '' : 'mediaKind')
    ..aOS(7, _omitFieldNames ? '' : 'livekitRoomName')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallStarted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallStarted copyWith(void Function(CallStarted) updates) =>
      super.copyWith((message) => updates(message as CallStarted))
          as CallStarted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallStarted create() => CallStarted._();
  @$core.override
  CallStarted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallStarted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallStarted>(create);
  static CallStarted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get profileIds => $_getList(1);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get initiatorProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set initiatorProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasInitiatorProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearInitiatorProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get calleeProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set calleeProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCalleeProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearCalleeProfileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get mediaKind => $_getSZ(5);
  @$pb.TagNumber(6)
  set mediaKind($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasMediaKind() => $_has(5);
  @$pb.TagNumber(6)
  void clearMediaKind() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get livekitRoomName => $_getSZ(6);
  @$pb.TagNumber(7)
  set livekitRoomName($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasLivekitRoomName() => $_has(6);
  @$pb.TagNumber(7)
  void clearLivekitRoomName() => $_clearField(7);
}

class CallEnded extends $pb.GeneratedMessage {
  factory CallEnded({
    $core.String? roomId,
    $core.int? durationSeconds,
    $core.Iterable<$core.String>? profileIds,
    $core.String? reason,
    $core.String? endedByProfileId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (durationSeconds != null) result.durationSeconds = durationSeconds;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (reason != null) result.reason = reason;
    if (endedByProfileId != null) result.endedByProfileId = endedByProfileId;
    return result;
  }

  CallEnded._();

  factory CallEnded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallEnded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallEnded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aI(2, _omitFieldNames ? '' : 'durationSeconds')
    ..pPS(3, _omitFieldNames ? '' : 'profileIds')
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..aOS(5, _omitFieldNames ? '' : 'endedByProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallEnded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallEnded copyWith(void Function(CallEnded) updates) =>
      super.copyWith((message) => updates(message as CallEnded)) as CallEnded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallEnded create() => CallEnded._();
  @$core.override
  CallEnded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallEnded getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<CallEnded>(create);
  static CallEnded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get durationSeconds => $_getIZ(1);
  @$pb.TagNumber(2)
  set durationSeconds($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDurationSeconds() => $_has(1);
  @$pb.TagNumber(2)
  void clearDurationSeconds() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get profileIds => $_getList(2);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get endedByProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set endedByProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEndedByProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearEndedByProfileId() => $_clearField(5);
}

class ScreenShareStarted extends $pb.GeneratedMessage {
  factory ScreenShareStarted({
    $core.String? roomId,
    $core.String? profileId,
    $core.String? streamId,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileId != null) result.profileId = profileId;
    if (streamId != null) result.streamId = streamId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  ScreenShareStarted._();

  factory ScreenShareStarted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ScreenShareStarted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScreenShareStarted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'streamId')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareStarted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareStarted copyWith(void Function(ScreenShareStarted) updates) =>
      super.copyWith((message) => updates(message as ScreenShareStarted))
          as ScreenShareStarted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ScreenShareStarted create() => ScreenShareStarted._();
  @$core.override
  ScreenShareStarted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ScreenShareStarted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScreenShareStarted>(create);
  static ScreenShareStarted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get streamId => $_getSZ(2);
  @$pb.TagNumber(3)
  set streamId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStreamId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStreamId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);
}

class ScreenShareStopped extends $pb.GeneratedMessage {
  factory ScreenShareStopped({
    $core.String? roomId,
    $core.String? profileId,
    $core.String? streamId,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileId != null) result.profileId = profileId;
    if (streamId != null) result.streamId = streamId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  ScreenShareStopped._();

  factory ScreenShareStopped.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ScreenShareStopped.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ScreenShareStopped',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'streamId')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareStopped clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ScreenShareStopped copyWith(void Function(ScreenShareStopped) updates) =>
      super.copyWith((message) => updates(message as ScreenShareStopped))
          as ScreenShareStopped;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ScreenShareStopped create() => ScreenShareStopped._();
  @$core.override
  ScreenShareStopped createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ScreenShareStopped getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ScreenShareStopped>(create);
  static ScreenShareStopped? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get streamId => $_getSZ(2);
  @$pb.TagNumber(3)
  set streamId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStreamId() => $_has(2);
  @$pb.TagNumber(3)
  void clearStreamId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);
}

class CallIncoming extends $pb.GeneratedMessage {
  factory CallIncoming({
    $core.String? roomId,
    $core.String? chatId,
    $core.String? initiatorProfileId,
    $core.String? calleeProfileId,
    $core.String? mediaKind,
    $core.String? livekitRoomName,
    $0.Timestamp? expiresAt,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (chatId != null) result.chatId = chatId;
    if (initiatorProfileId != null)
      result.initiatorProfileId = initiatorProfileId;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    if (mediaKind != null) result.mediaKind = mediaKind;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  CallIncoming._();

  factory CallIncoming.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallIncoming.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallIncoming',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'initiatorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'calleeProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'mediaKind')
    ..aOS(6, _omitFieldNames ? '' : 'livekitRoomName')
    ..aOM<$0.Timestamp>(7, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $0.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallIncoming clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallIncoming copyWith(void Function(CallIncoming) updates) =>
      super.copyWith((message) => updates(message as CallIncoming))
          as CallIncoming;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallIncoming create() => CallIncoming._();
  @$core.override
  CallIncoming createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallIncoming getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallIncoming>(create);
  static CallIncoming? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get initiatorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set initiatorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInitiatorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearInitiatorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get calleeProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set calleeProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCalleeProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearCalleeProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get mediaKind => $_getSZ(4);
  @$pb.TagNumber(5)
  set mediaKind($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMediaKind() => $_has(4);
  @$pb.TagNumber(5)
  void clearMediaKind() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get livekitRoomName => $_getSZ(5);
  @$pb.TagNumber(6)
  set livekitRoomName($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasLivekitRoomName() => $_has(5);
  @$pb.TagNumber(6)
  void clearLivekitRoomName() => $_clearField(6);

  @$pb.TagNumber(7)
  $0.Timestamp get expiresAt => $_getN(6);
  @$pb.TagNumber(7)
  set expiresAt($0.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasExpiresAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpiresAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.Timestamp ensureExpiresAt() => $_ensure(6);
}

class CallAccepted extends $pb.GeneratedMessage {
  factory CallAccepted({
    $core.String? roomId,
    $core.String? chatId,
    $core.String? acceptedByProfileId,
    $core.Iterable<$core.String>? profileIds,
    $core.String? mediaKind,
    $core.String? livekitRoomName,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (chatId != null) result.chatId = chatId;
    if (acceptedByProfileId != null)
      result.acceptedByProfileId = acceptedByProfileId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (mediaKind != null) result.mediaKind = mediaKind;
    if (livekitRoomName != null) result.livekitRoomName = livekitRoomName;
    return result;
  }

  CallAccepted._();

  factory CallAccepted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallAccepted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallAccepted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'acceptedByProfileId')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..aOS(5, _omitFieldNames ? '' : 'mediaKind')
    ..aOS(6, _omitFieldNames ? '' : 'livekitRoomName')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallAccepted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallAccepted copyWith(void Function(CallAccepted) updates) =>
      super.copyWith((message) => updates(message as CallAccepted))
          as CallAccepted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallAccepted create() => CallAccepted._();
  @$core.override
  CallAccepted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallAccepted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallAccepted>(create);
  static CallAccepted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get acceptedByProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set acceptedByProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAcceptedByProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAcceptedByProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get mediaKind => $_getSZ(4);
  @$pb.TagNumber(5)
  set mediaKind($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMediaKind() => $_has(4);
  @$pb.TagNumber(5)
  void clearMediaKind() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get livekitRoomName => $_getSZ(5);
  @$pb.TagNumber(6)
  set livekitRoomName($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasLivekitRoomName() => $_has(5);
  @$pb.TagNumber(6)
  void clearLivekitRoomName() => $_clearField(6);
}

class CallDeclined extends $pb.GeneratedMessage {
  factory CallDeclined({
    $core.String? roomId,
    $core.String? chatId,
    $core.String? declinedByProfileId,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (chatId != null) result.chatId = chatId;
    if (declinedByProfileId != null)
      result.declinedByProfileId = declinedByProfileId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  CallDeclined._();

  factory CallDeclined.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallDeclined.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallDeclined',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'declinedByProfileId')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallDeclined clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallDeclined copyWith(void Function(CallDeclined) updates) =>
      super.copyWith((message) => updates(message as CallDeclined))
          as CallDeclined;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallDeclined create() => CallDeclined._();
  @$core.override
  CallDeclined createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallDeclined getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallDeclined>(create);
  static CallDeclined? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get declinedByProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set declinedByProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeclinedByProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeclinedByProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);
}

class CallMissed extends $pb.GeneratedMessage {
  factory CallMissed({
    $core.String? roomId,
    $core.String? chatId,
    $core.String? initiatorProfileId,
    $core.String? calleeProfileId,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (chatId != null) result.chatId = chatId;
    if (initiatorProfileId != null)
      result.initiatorProfileId = initiatorProfileId;
    if (calleeProfileId != null) result.calleeProfileId = calleeProfileId;
    return result;
  }

  CallMissed._();

  factory CallMissed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CallMissed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CallMissed',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'initiatorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'calleeProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallMissed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CallMissed copyWith(void Function(CallMissed) updates) =>
      super.copyWith((message) => updates(message as CallMissed)) as CallMissed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CallMissed create() => CallMissed._();
  @$core.override
  CallMissed createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CallMissed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CallMissed>(create);
  static CallMissed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get initiatorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set initiatorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasInitiatorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearInitiatorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get calleeProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set calleeProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCalleeProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearCalleeProfileId() => $_clearField(4);
}

class VoiceStateChanged extends $pb.GeneratedMessage {
  factory VoiceStateChanged({
    $core.String? roomId,
    $core.String? profileId,
    $core.bool? isMuted,
    $core.bool? isDeafened,
    $core.bool? isVideoOn,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (roomId != null) result.roomId = roomId;
    if (profileId != null) result.profileId = profileId;
    if (isMuted != null) result.isMuted = isMuted;
    if (isDeafened != null) result.isDeafened = isDeafened;
    if (isVideoOn != null) result.isVideoOn = isVideoOn;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  VoiceStateChanged._();

  factory VoiceStateChanged.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceStateChanged.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceStateChanged',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOB(3, _omitFieldNames ? '' : 'isMuted')
    ..aOB(4, _omitFieldNames ? '' : 'isDeafened')
    ..aOB(5, _omitFieldNames ? '' : 'isVideoOn')
    ..pPS(6, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceStateChanged clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceStateChanged copyWith(void Function(VoiceStateChanged) updates) =>
      super.copyWith((message) => updates(message as VoiceStateChanged))
          as VoiceStateChanged;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceStateChanged create() => VoiceStateChanged._();
  @$core.override
  VoiceStateChanged createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceStateChanged getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceStateChanged>(create);
  static VoiceStateChanged? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isMuted => $_getBF(2);
  @$pb.TagNumber(3)
  set isMuted($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsMuted() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsMuted() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get isDeafened => $_getBF(3);
  @$pb.TagNumber(4)
  set isDeafened($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIsDeafened() => $_has(3);
  @$pb.TagNumber(4)
  void clearIsDeafened() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get isVideoOn => $_getBF(4);
  @$pb.TagNumber(5)
  set isVideoOn($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIsVideoOn() => $_has(4);
  @$pb.TagNumber(5)
  void clearIsVideoOn() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get profileIds => $_getList(5);
}

enum ModerationStreamEvent_Payload {
  reportCreated,
  sanctionApplied,
  appealSubmitted,
  notSet
}

class ModerationStreamEvent extends $pb.GeneratedMessage {
  factory ModerationStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    ReportCreated? reportCreated,
    SanctionApplied? sanctionApplied,
    AppealSubmitted? appealSubmitted,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (reportCreated != null) result.reportCreated = reportCreated;
    if (sanctionApplied != null) result.sanctionApplied = sanctionApplied;
    if (appealSubmitted != null) result.appealSubmitted = appealSubmitted;
    return result;
  }

  ModerationStreamEvent._();

  factory ModerationStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ModerationStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, ModerationStreamEvent_Payload>
      _ModerationStreamEvent_PayloadByTag = {
    10: ModerationStreamEvent_Payload.reportCreated,
    11: ModerationStreamEvent_Payload.sanctionApplied,
    12: ModerationStreamEvent_Payload.appealSubmitted,
    0: ModerationStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ModerationStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<ReportCreated>(10, _omitFieldNames ? '' : 'reportCreated',
        subBuilder: ReportCreated.create)
    ..aOM<SanctionApplied>(11, _omitFieldNames ? '' : 'sanctionApplied',
        subBuilder: SanctionApplied.create)
    ..aOM<AppealSubmitted>(12, _omitFieldNames ? '' : 'appealSubmitted',
        subBuilder: AppealSubmitted.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ModerationStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ModerationStreamEvent copyWith(
          void Function(ModerationStreamEvent) updates) =>
      super.copyWith((message) => updates(message as ModerationStreamEvent))
          as ModerationStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ModerationStreamEvent create() => ModerationStreamEvent._();
  @$core.override
  ModerationStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ModerationStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ModerationStreamEvent>(create);
  static ModerationStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  ModerationStreamEvent_Payload whichPayload() =>
      _ModerationStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  ReportCreated get reportCreated => $_getN(2);
  @$pb.TagNumber(10)
  set reportCreated(ReportCreated value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasReportCreated() => $_has(2);
  @$pb.TagNumber(10)
  void clearReportCreated() => $_clearField(10);
  @$pb.TagNumber(10)
  ReportCreated ensureReportCreated() => $_ensure(2);

  @$pb.TagNumber(11)
  SanctionApplied get sanctionApplied => $_getN(3);
  @$pb.TagNumber(11)
  set sanctionApplied(SanctionApplied value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasSanctionApplied() => $_has(3);
  @$pb.TagNumber(11)
  void clearSanctionApplied() => $_clearField(11);
  @$pb.TagNumber(11)
  SanctionApplied ensureSanctionApplied() => $_ensure(3);

  @$pb.TagNumber(12)
  AppealSubmitted get appealSubmitted => $_getN(4);
  @$pb.TagNumber(12)
  set appealSubmitted(AppealSubmitted value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasAppealSubmitted() => $_has(4);
  @$pb.TagNumber(12)
  void clearAppealSubmitted() => $_clearField(12);
  @$pb.TagNumber(12)
  AppealSubmitted ensureAppealSubmitted() => $_ensure(4);
}

class ReportCreated extends $pb.GeneratedMessage {
  factory ReportCreated({
    $core.String? reportId,
    $core.String? reporterProfileId,
  }) {
    final result = create();
    if (reportId != null) result.reportId = reportId;
    if (reporterProfileId != null) result.reporterProfileId = reporterProfileId;
    return result;
  }

  ReportCreated._();

  factory ReportCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReportCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..aOS(2, _omitFieldNames ? '' : 'reporterProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportCreated copyWith(void Function(ReportCreated) updates) =>
      super.copyWith((message) => updates(message as ReportCreated))
          as ReportCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReportCreated create() => ReportCreated._();
  @$core.override
  ReportCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReportCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReportCreated>(create);
  static ReportCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reporterProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set reporterProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReporterProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReporterProfileId() => $_clearField(2);
}

class SanctionApplied extends $pb.GeneratedMessage {
  factory SanctionApplied({
    $core.String? sanctionId,
    $core.String? targetAccountId,
    $core.String? type,
  }) {
    final result = create();
    if (sanctionId != null) result.sanctionId = sanctionId;
    if (targetAccountId != null) result.targetAccountId = targetAccountId;
    if (type != null) result.type = type;
    return result;
  }

  SanctionApplied._();

  factory SanctionApplied.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SanctionApplied.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SanctionApplied',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sanctionId')
    ..aOS(2, _omitFieldNames ? '' : 'targetAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SanctionApplied clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SanctionApplied copyWith(void Function(SanctionApplied) updates) =>
      super.copyWith((message) => updates(message as SanctionApplied))
          as SanctionApplied;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SanctionApplied create() => SanctionApplied._();
  @$core.override
  SanctionApplied createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SanctionApplied getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SanctionApplied>(create);
  static SanctionApplied? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sanctionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sanctionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSanctionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanctionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get targetAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set targetAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTargetAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTargetAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get type => $_getSZ(2);
  @$pb.TagNumber(3)
  set type($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);
}

class AppealSubmitted extends $pb.GeneratedMessage {
  factory AppealSubmitted({
    $core.String? appealId,
    $core.String? sanctionId,
  }) {
    final result = create();
    if (appealId != null) result.appealId = appealId;
    if (sanctionId != null) result.sanctionId = sanctionId;
    return result;
  }

  AppealSubmitted._();

  factory AppealSubmitted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AppealSubmitted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AppealSubmitted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'appealId')
    ..aOS(2, _omitFieldNames ? '' : 'sanctionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AppealSubmitted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AppealSubmitted copyWith(void Function(AppealSubmitted) updates) =>
      super.copyWith((message) => updates(message as AppealSubmitted))
          as AppealSubmitted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AppealSubmitted create() => AppealSubmitted._();
  @$core.override
  AppealSubmitted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AppealSubmitted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AppealSubmitted>(create);
  static AppealSubmitted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get appealId => $_getSZ(0);
  @$pb.TagNumber(1)
  set appealId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAppealId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppealId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sanctionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sanctionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSanctionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSanctionId() => $_clearField(2);
}

enum SubscriptionStreamEvent_Payload {
  planStarted,
  planCancelled,
  paymentSuccess,
  paymentFailed,
  notSet
}

class SubscriptionStreamEvent extends $pb.GeneratedMessage {
  factory SubscriptionStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    PlanStarted? planStarted,
    PlanCancelled? planCancelled,
    PaymentSuccess? paymentSuccess,
    PaymentFailed? paymentFailed,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (planStarted != null) result.planStarted = planStarted;
    if (planCancelled != null) result.planCancelled = planCancelled;
    if (paymentSuccess != null) result.paymentSuccess = paymentSuccess;
    if (paymentFailed != null) result.paymentFailed = paymentFailed;
    return result;
  }

  SubscriptionStreamEvent._();

  factory SubscriptionStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SubscriptionStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, SubscriptionStreamEvent_Payload>
      _SubscriptionStreamEvent_PayloadByTag = {
    10: SubscriptionStreamEvent_Payload.planStarted,
    11: SubscriptionStreamEvent_Payload.planCancelled,
    12: SubscriptionStreamEvent_Payload.paymentSuccess,
    13: SubscriptionStreamEvent_Payload.paymentFailed,
    0: SubscriptionStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubscriptionStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<PlanStarted>(10, _omitFieldNames ? '' : 'planStarted',
        subBuilder: PlanStarted.create)
    ..aOM<PlanCancelled>(11, _omitFieldNames ? '' : 'planCancelled',
        subBuilder: PlanCancelled.create)
    ..aOM<PaymentSuccess>(12, _omitFieldNames ? '' : 'paymentSuccess',
        subBuilder: PaymentSuccess.create)
    ..aOM<PaymentFailed>(13, _omitFieldNames ? '' : 'paymentFailed',
        subBuilder: PaymentFailed.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscriptionStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubscriptionStreamEvent copyWith(
          void Function(SubscriptionStreamEvent) updates) =>
      super.copyWith((message) => updates(message as SubscriptionStreamEvent))
          as SubscriptionStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubscriptionStreamEvent create() => SubscriptionStreamEvent._();
  @$core.override
  SubscriptionStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SubscriptionStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubscriptionStreamEvent>(create);
  static SubscriptionStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  SubscriptionStreamEvent_Payload whichPayload() =>
      _SubscriptionStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  PlanStarted get planStarted => $_getN(2);
  @$pb.TagNumber(10)
  set planStarted(PlanStarted value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasPlanStarted() => $_has(2);
  @$pb.TagNumber(10)
  void clearPlanStarted() => $_clearField(10);
  @$pb.TagNumber(10)
  PlanStarted ensurePlanStarted() => $_ensure(2);

  @$pb.TagNumber(11)
  PlanCancelled get planCancelled => $_getN(3);
  @$pb.TagNumber(11)
  set planCancelled(PlanCancelled value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasPlanCancelled() => $_has(3);
  @$pb.TagNumber(11)
  void clearPlanCancelled() => $_clearField(11);
  @$pb.TagNumber(11)
  PlanCancelled ensurePlanCancelled() => $_ensure(3);

  @$pb.TagNumber(12)
  PaymentSuccess get paymentSuccess => $_getN(4);
  @$pb.TagNumber(12)
  set paymentSuccess(PaymentSuccess value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasPaymentSuccess() => $_has(4);
  @$pb.TagNumber(12)
  void clearPaymentSuccess() => $_clearField(12);
  @$pb.TagNumber(12)
  PaymentSuccess ensurePaymentSuccess() => $_ensure(4);

  @$pb.TagNumber(13)
  PaymentFailed get paymentFailed => $_getN(5);
  @$pb.TagNumber(13)
  set paymentFailed(PaymentFailed value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasPaymentFailed() => $_has(5);
  @$pb.TagNumber(13)
  void clearPaymentFailed() => $_clearField(13);
  @$pb.TagNumber(13)
  PaymentFailed ensurePaymentFailed() => $_ensure(5);
}

class PlanStarted extends $pb.GeneratedMessage {
  factory PlanStarted({
    $core.String? accountId,
    $core.String? plan,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (plan != null) result.plan = plan;
    return result;
  }

  PlanStarted._();

  factory PlanStarted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PlanStarted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PlanStarted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'plan')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlanStarted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlanStarted copyWith(void Function(PlanStarted) updates) =>
      super.copyWith((message) => updates(message as PlanStarted))
          as PlanStarted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PlanStarted create() => PlanStarted._();
  @$core.override
  PlanStarted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PlanStarted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PlanStarted>(create);
  static PlanStarted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get plan => $_getSZ(1);
  @$pb.TagNumber(2)
  set plan($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPlan() => $_has(1);
  @$pb.TagNumber(2)
  void clearPlan() => $_clearField(2);
}

class PlanCancelled extends $pb.GeneratedMessage {
  factory PlanCancelled({
    $core.String? accountId,
    $core.String? plan,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (plan != null) result.plan = plan;
    return result;
  }

  PlanCancelled._();

  factory PlanCancelled.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PlanCancelled.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PlanCancelled',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'plan')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlanCancelled clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlanCancelled copyWith(void Function(PlanCancelled) updates) =>
      super.copyWith((message) => updates(message as PlanCancelled))
          as PlanCancelled;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PlanCancelled create() => PlanCancelled._();
  @$core.override
  PlanCancelled createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PlanCancelled getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PlanCancelled>(create);
  static PlanCancelled? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get plan => $_getSZ(1);
  @$pb.TagNumber(2)
  set plan($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPlan() => $_has(1);
  @$pb.TagNumber(2)
  void clearPlan() => $_clearField(2);
}

class PaymentSuccess extends $pb.GeneratedMessage {
  factory PaymentSuccess({
    $core.String? accountId,
    $core.String? provider,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (provider != null) result.provider = provider;
    return result;
  }

  PaymentSuccess._();

  factory PaymentSuccess.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PaymentSuccess.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PaymentSuccess',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentSuccess clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentSuccess copyWith(void Function(PaymentSuccess) updates) =>
      super.copyWith((message) => updates(message as PaymentSuccess))
          as PaymentSuccess;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PaymentSuccess create() => PaymentSuccess._();
  @$core.override
  PaymentSuccess createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PaymentSuccess getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PaymentSuccess>(create);
  static PaymentSuccess? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get provider => $_getSZ(1);
  @$pb.TagNumber(2)
  set provider($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProvider() => $_has(1);
  @$pb.TagNumber(2)
  void clearProvider() => $_clearField(2);
}

class PaymentFailed extends $pb.GeneratedMessage {
  factory PaymentFailed({
    $core.String? accountId,
    $core.String? provider,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (provider != null) result.provider = provider;
    return result;
  }

  PaymentFailed._();

  factory PaymentFailed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PaymentFailed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PaymentFailed',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'provider')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentFailed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PaymentFailed copyWith(void Function(PaymentFailed) updates) =>
      super.copyWith((message) => updates(message as PaymentFailed))
          as PaymentFailed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PaymentFailed create() => PaymentFailed._();
  @$core.override
  PaymentFailed createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PaymentFailed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PaymentFailed>(create);
  static PaymentFailed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get provider => $_getSZ(1);
  @$pb.TagNumber(2)
  set provider($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProvider() => $_has(1);
  @$pb.TagNumber(2)
  void clearProvider() => $_clearField(2);
}

enum FileStreamEvent_Payload { fileUploaded, fileScanResult, notSet }

class FileStreamEvent extends $pb.GeneratedMessage {
  factory FileStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    FileUploaded? fileUploaded,
    FileScanResult? fileScanResult,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (fileUploaded != null) result.fileUploaded = fileUploaded;
    if (fileScanResult != null) result.fileScanResult = fileScanResult;
    return result;
  }

  FileStreamEvent._();

  factory FileStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, FileStreamEvent_Payload>
      _FileStreamEvent_PayloadByTag = {
    10: FileStreamEvent_Payload.fileUploaded,
    11: FileStreamEvent_Payload.fileScanResult,
    0: FileStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<FileUploaded>(10, _omitFieldNames ? '' : 'fileUploaded',
        subBuilder: FileUploaded.create)
    ..aOM<FileScanResult>(11, _omitFieldNames ? '' : 'fileScanResult',
        subBuilder: FileScanResult.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileStreamEvent copyWith(void Function(FileStreamEvent) updates) =>
      super.copyWith((message) => updates(message as FileStreamEvent))
          as FileStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileStreamEvent create() => FileStreamEvent._();
  @$core.override
  FileStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileStreamEvent>(create);
  static FileStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  FileStreamEvent_Payload whichPayload() =>
      _FileStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  FileUploaded get fileUploaded => $_getN(2);
  @$pb.TagNumber(10)
  set fileUploaded(FileUploaded value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasFileUploaded() => $_has(2);
  @$pb.TagNumber(10)
  void clearFileUploaded() => $_clearField(10);
  @$pb.TagNumber(10)
  FileUploaded ensureFileUploaded() => $_ensure(2);

  @$pb.TagNumber(11)
  FileScanResult get fileScanResult => $_getN(3);
  @$pb.TagNumber(11)
  set fileScanResult(FileScanResult value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasFileScanResult() => $_has(3);
  @$pb.TagNumber(11)
  void clearFileScanResult() => $_clearField(11);
  @$pb.TagNumber(11)
  FileScanResult ensureFileScanResult() => $_ensure(3);
}

class FileUploaded extends $pb.GeneratedMessage {
  factory FileUploaded({
    $core.String? fileId,
    $core.String? uploaderProfileId,
  }) {
    final result = create();
    if (fileId != null) result.fileId = fileId;
    if (uploaderProfileId != null) result.uploaderProfileId = uploaderProfileId;
    return result;
  }

  FileUploaded._();

  factory FileUploaded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileUploaded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileUploaded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOS(2, _omitFieldNames ? '' : 'uploaderProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileUploaded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileUploaded copyWith(void Function(FileUploaded) updates) =>
      super.copyWith((message) => updates(message as FileUploaded))
          as FileUploaded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileUploaded create() => FileUploaded._();
  @$core.override
  FileUploaded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileUploaded getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileUploaded>(create);
  static FileUploaded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get uploaderProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set uploaderProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUploaderProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearUploaderProfileId() => $_clearField(2);
}

class FileScanResult extends $pb.GeneratedMessage {
  factory FileScanResult({
    $core.String? fileId,
    $core.String? result,
  }) {
    final result$ = create();
    if (fileId != null) result$.fileId = fileId;
    if (result != null) result$.result = result;
    return result$;
  }

  FileScanResult._();

  factory FileScanResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FileScanResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FileScanResult',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'fileId')
    ..aOS(2, _omitFieldNames ? '' : 'result')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileScanResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FileScanResult copyWith(void Function(FileScanResult) updates) =>
      super.copyWith((message) => updates(message as FileScanResult))
          as FileScanResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FileScanResult create() => FileScanResult._();
  @$core.override
  FileScanResult createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FileScanResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FileScanResult>(create);
  static FileScanResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get fileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set fileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get result => $_getSZ(1);
  @$pb.TagNumber(2)
  set result($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResult() => $_has(1);
  @$pb.TagNumber(2)
  void clearResult() => $_clearField(2);
}

enum MatchmakingStreamEvent_Payload {
  searchStarted,
  matchFound,
  matchTimeout,
  ratingSubmitted,
  searchCancelled,
  matchCompleted,
  searchNudge,
  notSet
}

class MatchmakingStreamEvent extends $pb.GeneratedMessage {
  factory MatchmakingStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    SearchStarted? searchStarted,
    MatchFound? matchFound,
    MatchTimeout? matchTimeout,
    RatingSubmitted? ratingSubmitted,
    SearchCancelled? searchCancelled,
    MatchCompleted? matchCompleted,
    SearchNudge? searchNudge,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (searchStarted != null) result.searchStarted = searchStarted;
    if (matchFound != null) result.matchFound = matchFound;
    if (matchTimeout != null) result.matchTimeout = matchTimeout;
    if (ratingSubmitted != null) result.ratingSubmitted = ratingSubmitted;
    if (searchCancelled != null) result.searchCancelled = searchCancelled;
    if (matchCompleted != null) result.matchCompleted = matchCompleted;
    if (searchNudge != null) result.searchNudge = searchNudge;
    return result;
  }

  MatchmakingStreamEvent._();

  factory MatchmakingStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchmakingStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MatchmakingStreamEvent_Payload>
      _MatchmakingStreamEvent_PayloadByTag = {
    10: MatchmakingStreamEvent_Payload.searchStarted,
    11: MatchmakingStreamEvent_Payload.matchFound,
    12: MatchmakingStreamEvent_Payload.matchTimeout,
    13: MatchmakingStreamEvent_Payload.ratingSubmitted,
    14: MatchmakingStreamEvent_Payload.searchCancelled,
    15: MatchmakingStreamEvent_Payload.matchCompleted,
    16: MatchmakingStreamEvent_Payload.searchNudge,
    0: MatchmakingStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchmakingStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14, 15, 16])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<SearchStarted>(10, _omitFieldNames ? '' : 'searchStarted',
        subBuilder: SearchStarted.create)
    ..aOM<MatchFound>(11, _omitFieldNames ? '' : 'matchFound',
        subBuilder: MatchFound.create)
    ..aOM<MatchTimeout>(12, _omitFieldNames ? '' : 'matchTimeout',
        subBuilder: MatchTimeout.create)
    ..aOM<RatingSubmitted>(13, _omitFieldNames ? '' : 'ratingSubmitted',
        subBuilder: RatingSubmitted.create)
    ..aOM<SearchCancelled>(14, _omitFieldNames ? '' : 'searchCancelled',
        subBuilder: SearchCancelled.create)
    ..aOM<MatchCompleted>(15, _omitFieldNames ? '' : 'matchCompleted',
        subBuilder: MatchCompleted.create)
    ..aOM<SearchNudge>(16, _omitFieldNames ? '' : 'searchNudge',
        subBuilder: SearchNudge.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchmakingStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchmakingStreamEvent copyWith(
          void Function(MatchmakingStreamEvent) updates) =>
      super.copyWith((message) => updates(message as MatchmakingStreamEvent))
          as MatchmakingStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchmakingStreamEvent create() => MatchmakingStreamEvent._();
  @$core.override
  MatchmakingStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchmakingStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchmakingStreamEvent>(create);
  static MatchmakingStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  MatchmakingStreamEvent_Payload whichPayload() =>
      _MatchmakingStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  SearchStarted get searchStarted => $_getN(2);
  @$pb.TagNumber(10)
  set searchStarted(SearchStarted value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasSearchStarted() => $_has(2);
  @$pb.TagNumber(10)
  void clearSearchStarted() => $_clearField(10);
  @$pb.TagNumber(10)
  SearchStarted ensureSearchStarted() => $_ensure(2);

  @$pb.TagNumber(11)
  MatchFound get matchFound => $_getN(3);
  @$pb.TagNumber(11)
  set matchFound(MatchFound value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasMatchFound() => $_has(3);
  @$pb.TagNumber(11)
  void clearMatchFound() => $_clearField(11);
  @$pb.TagNumber(11)
  MatchFound ensureMatchFound() => $_ensure(3);

  @$pb.TagNumber(12)
  MatchTimeout get matchTimeout => $_getN(4);
  @$pb.TagNumber(12)
  set matchTimeout(MatchTimeout value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasMatchTimeout() => $_has(4);
  @$pb.TagNumber(12)
  void clearMatchTimeout() => $_clearField(12);
  @$pb.TagNumber(12)
  MatchTimeout ensureMatchTimeout() => $_ensure(4);

  @$pb.TagNumber(13)
  RatingSubmitted get ratingSubmitted => $_getN(5);
  @$pb.TagNumber(13)
  set ratingSubmitted(RatingSubmitted value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasRatingSubmitted() => $_has(5);
  @$pb.TagNumber(13)
  void clearRatingSubmitted() => $_clearField(13);
  @$pb.TagNumber(13)
  RatingSubmitted ensureRatingSubmitted() => $_ensure(5);

  @$pb.TagNumber(14)
  SearchCancelled get searchCancelled => $_getN(6);
  @$pb.TagNumber(14)
  set searchCancelled(SearchCancelled value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasSearchCancelled() => $_has(6);
  @$pb.TagNumber(14)
  void clearSearchCancelled() => $_clearField(14);
  @$pb.TagNumber(14)
  SearchCancelled ensureSearchCancelled() => $_ensure(6);

  @$pb.TagNumber(15)
  MatchCompleted get matchCompleted => $_getN(7);
  @$pb.TagNumber(15)
  set matchCompleted(MatchCompleted value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasMatchCompleted() => $_has(7);
  @$pb.TagNumber(15)
  void clearMatchCompleted() => $_clearField(15);
  @$pb.TagNumber(15)
  MatchCompleted ensureMatchCompleted() => $_ensure(7);

  @$pb.TagNumber(16)
  SearchNudge get searchNudge => $_getN(8);
  @$pb.TagNumber(16)
  set searchNudge(SearchNudge value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasSearchNudge() => $_has(8);
  @$pb.TagNumber(16)
  void clearSearchNudge() => $_clearField(16);
  @$pb.TagNumber(16)
  SearchNudge ensureSearchNudge() => $_ensure(8);
}

class SearchStarted extends $pb.GeneratedMessage {
  factory SearchStarted({
    $core.String? sessionId,
    $core.String? profileId,
    $core.String? gameId,
    $core.String? mode,
    $core.String? region,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    if (profileId != null) result.profileId = profileId;
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    if (region != null) result.region = region;
    return result;
  }

  SearchStarted._();

  factory SearchStarted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchStarted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchStarted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'gameId')
    ..aOS(4, _omitFieldNames ? '' : 'mode')
    ..aOS(5, _omitFieldNames ? '' : 'region')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchStarted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchStarted copyWith(void Function(SearchStarted) updates) =>
      super.copyWith((message) => updates(message as SearchStarted))
          as SearchStarted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchStarted create() => SearchStarted._();
  @$core.override
  SearchStarted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchStarted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchStarted>(create);
  static SearchStarted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get gameId => $_getSZ(2);
  @$pb.TagNumber(3)
  set gameId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGameId() => $_has(2);
  @$pb.TagNumber(3)
  void clearGameId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mode => $_getSZ(3);
  @$pb.TagNumber(4)
  set mode($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMode() => $_has(3);
  @$pb.TagNumber(4)
  void clearMode() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get region => $_getSZ(4);
  @$pb.TagNumber(5)
  set region($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRegion() => $_has(4);
  @$pb.TagNumber(5)
  void clearRegion() => $_clearField(5);
}

class SearchCancelled extends $pb.GeneratedMessage {
  factory SearchCancelled({
    $core.String? sessionId,
    $core.String? profileId,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  SearchCancelled._();

  factory SearchCancelled.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchCancelled.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchCancelled',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchCancelled clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchCancelled copyWith(void Function(SearchCancelled) updates) =>
      super.copyWith((message) => updates(message as SearchCancelled))
          as SearchCancelled;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchCancelled create() => SearchCancelled._();
  @$core.override
  SearchCancelled createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchCancelled getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchCancelled>(create);
  static SearchCancelled? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class MatchFound extends $pb.GeneratedMessage {
  factory MatchFound({
    $core.String? matchId,
    $core.Iterable<$core.String>? profileIds,
    $core.String? gameId,
    $core.String? mode,
    $core.String? region,
    $core.Iterable<$core.String>? sessionIds,
    $core.String? chatId,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    if (region != null) result.region = region;
    if (sessionIds != null) result.sessionIds.addAll(sessionIds);
    if (chatId != null) result.chatId = chatId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  MatchFound._();

  factory MatchFound.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchFound.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchFound',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..pPS(2, _omitFieldNames ? '' : 'profileIds')
    ..aOS(3, _omitFieldNames ? '' : 'gameId')
    ..aOS(4, _omitFieldNames ? '' : 'mode')
    ..aOS(5, _omitFieldNames ? '' : 'region')
    ..pPS(6, _omitFieldNames ? '' : 'sessionIds')
    ..aOS(7, _omitFieldNames ? '' : 'chatId')
    ..aOS(8, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchFound clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchFound copyWith(void Function(MatchFound) updates) =>
      super.copyWith((message) => updates(message as MatchFound)) as MatchFound;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchFound create() => MatchFound._();
  @$core.override
  MatchFound createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchFound getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchFound>(create);
  static MatchFound? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get profileIds => $_getList(1);

  @$pb.TagNumber(3)
  $core.String get gameId => $_getSZ(2);
  @$pb.TagNumber(3)
  set gameId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGameId() => $_has(2);
  @$pb.TagNumber(3)
  void clearGameId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mode => $_getSZ(3);
  @$pb.TagNumber(4)
  set mode($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMode() => $_has(3);
  @$pb.TagNumber(4)
  void clearMode() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get region => $_getSZ(4);
  @$pb.TagNumber(5)
  set region($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRegion() => $_has(4);
  @$pb.TagNumber(5)
  void clearRegion() => $_clearField(5);

  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get sessionIds => $_getList(5);

  @$pb.TagNumber(7)
  $core.String get chatId => $_getSZ(6);
  @$pb.TagNumber(7)
  set chatId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasChatId() => $_has(6);
  @$pb.TagNumber(7)
  void clearChatId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get voiceRoomId => $_getSZ(7);
  @$pb.TagNumber(8)
  set voiceRoomId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasVoiceRoomId() => $_has(7);
  @$pb.TagNumber(8)
  void clearVoiceRoomId() => $_clearField(8);
}

class MatchTimeout extends $pb.GeneratedMessage {
  factory MatchTimeout({
    $core.String? sessionId,
    $core.String? profileId,
    $core.String? gameId,
    $core.String? mode,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    if (profileId != null) result.profileId = profileId;
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    return result;
  }

  MatchTimeout._();

  factory MatchTimeout.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchTimeout.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchTimeout',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'gameId')
    ..aOS(4, _omitFieldNames ? '' : 'mode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchTimeout clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchTimeout copyWith(void Function(MatchTimeout) updates) =>
      super.copyWith((message) => updates(message as MatchTimeout))
          as MatchTimeout;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchTimeout create() => MatchTimeout._();
  @$core.override
  MatchTimeout createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchTimeout getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchTimeout>(create);
  static MatchTimeout? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get gameId => $_getSZ(2);
  @$pb.TagNumber(3)
  set gameId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGameId() => $_has(2);
  @$pb.TagNumber(3)
  void clearGameId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mode => $_getSZ(3);
  @$pb.TagNumber(4)
  set mode($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMode() => $_has(3);
  @$pb.TagNumber(4)
  void clearMode() => $_clearField(4);
}

class SearchNudge extends $pb.GeneratedMessage {
  factory SearchNudge({
    $core.String? sessionId,
    $core.String? profileId,
    $core.String? gameId,
    $core.String? mode,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    if (profileId != null) result.profileId = profileId;
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    return result;
  }

  SearchNudge._();

  factory SearchNudge.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchNudge.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchNudge',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'gameId')
    ..aOS(4, _omitFieldNames ? '' : 'mode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchNudge clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchNudge copyWith(void Function(SearchNudge) updates) =>
      super.copyWith((message) => updates(message as SearchNudge))
          as SearchNudge;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchNudge create() => SearchNudge._();
  @$core.override
  SearchNudge createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchNudge getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchNudge>(create);
  static SearchNudge? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get gameId => $_getSZ(2);
  @$pb.TagNumber(3)
  set gameId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGameId() => $_has(2);
  @$pb.TagNumber(3)
  void clearGameId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get mode => $_getSZ(3);
  @$pb.TagNumber(4)
  set mode($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMode() => $_has(3);
  @$pb.TagNumber(4)
  void clearMode() => $_clearField(4);
}

class MatchCompleted extends $pb.GeneratedMessage {
  factory MatchCompleted({
    $core.String? matchId,
    $fixnum.Int64? durationSeconds,
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    if (durationSeconds != null) result.durationSeconds = durationSeconds;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  MatchCompleted._();

  factory MatchCompleted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchCompleted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchCompleted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..aInt64(2, _omitFieldNames ? '' : 'durationSeconds')
    ..pPS(3, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchCompleted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchCompleted copyWith(void Function(MatchCompleted) updates) =>
      super.copyWith((message) => updates(message as MatchCompleted))
          as MatchCompleted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchCompleted create() => MatchCompleted._();
  @$core.override
  MatchCompleted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchCompleted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MatchCompleted>(create);
  static MatchCompleted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get durationSeconds => $_getI64(1);
  @$pb.TagNumber(2)
  set durationSeconds($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDurationSeconds() => $_has(1);
  @$pb.TagNumber(2)
  void clearDurationSeconds() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get profileIds => $_getList(2);
}

class RatingSubmitted extends $pb.GeneratedMessage {
  factory RatingSubmitted({
    $core.String? matchId,
    $core.String? raterProfileId,
    $core.String? ratedProfileId,
    $core.int? stars,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    if (raterProfileId != null) result.raterProfileId = raterProfileId;
    if (ratedProfileId != null) result.ratedProfileId = ratedProfileId;
    if (stars != null) result.stars = stars;
    return result;
  }

  RatingSubmitted._();

  factory RatingSubmitted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RatingSubmitted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RatingSubmitted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..aOS(2, _omitFieldNames ? '' : 'raterProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'ratedProfileId')
    ..aI(4, _omitFieldNames ? '' : 'stars')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RatingSubmitted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RatingSubmitted copyWith(void Function(RatingSubmitted) updates) =>
      super.copyWith((message) => updates(message as RatingSubmitted))
          as RatingSubmitted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RatingSubmitted create() => RatingSubmitted._();
  @$core.override
  RatingSubmitted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RatingSubmitted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RatingSubmitted>(create);
  static RatingSubmitted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get raterProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set raterProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRaterProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRaterProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get ratedProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set ratedProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRatedProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRatedProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get stars => $_getIZ(3);
  @$pb.TagNumber(4)
  set stars($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStars() => $_has(3);
  @$pb.TagNumber(4)
  void clearStars() => $_clearField(4);
}

enum StoryStreamEvent_Payload {
  storyCreated,
  storyViewed,
  highlightAdded,
  storyReacted,
  storyExpired,
  storyHighlightCreated,
  storyLfpCreated,
  notSet
}

class StoryStreamEvent extends $pb.GeneratedMessage {
  factory StoryStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    StoryCreated? storyCreated,
    StoryViewed? storyViewed,
    HighlightAdded? highlightAdded,
    StoryReacted? storyReacted,
    StoryExpired? storyExpired,
    StoryHighlightCreated? storyHighlightCreated,
    StoryLfpCreated? storyLfpCreated,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (storyCreated != null) result.storyCreated = storyCreated;
    if (storyViewed != null) result.storyViewed = storyViewed;
    if (highlightAdded != null) result.highlightAdded = highlightAdded;
    if (storyReacted != null) result.storyReacted = storyReacted;
    if (storyExpired != null) result.storyExpired = storyExpired;
    if (storyHighlightCreated != null)
      result.storyHighlightCreated = storyHighlightCreated;
    if (storyLfpCreated != null) result.storyLfpCreated = storyLfpCreated;
    return result;
  }

  StoryStreamEvent._();

  factory StoryStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, StoryStreamEvent_Payload>
      _StoryStreamEvent_PayloadByTag = {
    10: StoryStreamEvent_Payload.storyCreated,
    11: StoryStreamEvent_Payload.storyViewed,
    12: StoryStreamEvent_Payload.highlightAdded,
    13: StoryStreamEvent_Payload.storyReacted,
    14: StoryStreamEvent_Payload.storyExpired,
    15: StoryStreamEvent_Payload.storyHighlightCreated,
    16: StoryStreamEvent_Payload.storyLfpCreated,
    0: StoryStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14, 15, 16])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<StoryCreated>(10, _omitFieldNames ? '' : 'storyCreated',
        subBuilder: StoryCreated.create)
    ..aOM<StoryViewed>(11, _omitFieldNames ? '' : 'storyViewed',
        subBuilder: StoryViewed.create)
    ..aOM<HighlightAdded>(12, _omitFieldNames ? '' : 'highlightAdded',
        subBuilder: HighlightAdded.create)
    ..aOM<StoryReacted>(13, _omitFieldNames ? '' : 'storyReacted',
        subBuilder: StoryReacted.create)
    ..aOM<StoryExpired>(14, _omitFieldNames ? '' : 'storyExpired',
        subBuilder: StoryExpired.create)
    ..aOM<StoryHighlightCreated>(
        15, _omitFieldNames ? '' : 'storyHighlightCreated',
        subBuilder: StoryHighlightCreated.create)
    ..aOM<StoryLfpCreated>(16, _omitFieldNames ? '' : 'storyLfpCreated',
        subBuilder: StoryLfpCreated.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryStreamEvent copyWith(void Function(StoryStreamEvent) updates) =>
      super.copyWith((message) => updates(message as StoryStreamEvent))
          as StoryStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryStreamEvent create() => StoryStreamEvent._();
  @$core.override
  StoryStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryStreamEvent>(create);
  static StoryStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
  StoryStreamEvent_Payload whichPayload() =>
      _StoryStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  @$pb.TagNumber(15)
  @$pb.TagNumber(16)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  StoryCreated get storyCreated => $_getN(2);
  @$pb.TagNumber(10)
  set storyCreated(StoryCreated value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasStoryCreated() => $_has(2);
  @$pb.TagNumber(10)
  void clearStoryCreated() => $_clearField(10);
  @$pb.TagNumber(10)
  StoryCreated ensureStoryCreated() => $_ensure(2);

  @$pb.TagNumber(11)
  StoryViewed get storyViewed => $_getN(3);
  @$pb.TagNumber(11)
  set storyViewed(StoryViewed value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasStoryViewed() => $_has(3);
  @$pb.TagNumber(11)
  void clearStoryViewed() => $_clearField(11);
  @$pb.TagNumber(11)
  StoryViewed ensureStoryViewed() => $_ensure(3);

  @$pb.TagNumber(12)
  HighlightAdded get highlightAdded => $_getN(4);
  @$pb.TagNumber(12)
  set highlightAdded(HighlightAdded value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasHighlightAdded() => $_has(4);
  @$pb.TagNumber(12)
  void clearHighlightAdded() => $_clearField(12);
  @$pb.TagNumber(12)
  HighlightAdded ensureHighlightAdded() => $_ensure(4);

  @$pb.TagNumber(13)
  StoryReacted get storyReacted => $_getN(5);
  @$pb.TagNumber(13)
  set storyReacted(StoryReacted value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasStoryReacted() => $_has(5);
  @$pb.TagNumber(13)
  void clearStoryReacted() => $_clearField(13);
  @$pb.TagNumber(13)
  StoryReacted ensureStoryReacted() => $_ensure(5);

  @$pb.TagNumber(14)
  StoryExpired get storyExpired => $_getN(6);
  @$pb.TagNumber(14)
  set storyExpired(StoryExpired value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasStoryExpired() => $_has(6);
  @$pb.TagNumber(14)
  void clearStoryExpired() => $_clearField(14);
  @$pb.TagNumber(14)
  StoryExpired ensureStoryExpired() => $_ensure(6);

  @$pb.TagNumber(15)
  StoryHighlightCreated get storyHighlightCreated => $_getN(7);
  @$pb.TagNumber(15)
  set storyHighlightCreated(StoryHighlightCreated value) =>
      $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasStoryHighlightCreated() => $_has(7);
  @$pb.TagNumber(15)
  void clearStoryHighlightCreated() => $_clearField(15);
  @$pb.TagNumber(15)
  StoryHighlightCreated ensureStoryHighlightCreated() => $_ensure(7);

  @$pb.TagNumber(16)
  StoryLfpCreated get storyLfpCreated => $_getN(8);
  @$pb.TagNumber(16)
  set storyLfpCreated(StoryLfpCreated value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasStoryLfpCreated() => $_has(8);
  @$pb.TagNumber(16)
  void clearStoryLfpCreated() => $_clearField(16);
  @$pb.TagNumber(16)
  StoryLfpCreated ensureStoryLfpCreated() => $_ensure(8);
}

class StoryCreated extends $pb.GeneratedMessage {
  factory StoryCreated({
    $core.String? storyId,
    $core.String? authorProfileId,
    $core.String? type,
    $core.String? gameTag,
    $core.Iterable<$core.String>? mentionProfileIds,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (type != null) result.type = type;
    if (gameTag != null) result.gameTag = gameTag;
    if (mentionProfileIds != null)
      result.mentionProfileIds.addAll(mentionProfileIds);
    return result;
  }

  StoryCreated._();

  factory StoryCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'authorProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..aOS(4, _omitFieldNames ? '' : 'gameTag')
    ..pPS(5, _omitFieldNames ? '' : 'mentionProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryCreated copyWith(void Function(StoryCreated) updates) =>
      super.copyWith((message) => updates(message as StoryCreated))
          as StoryCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryCreated create() => StoryCreated._();
  @$core.override
  StoryCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryCreated>(create);
  static StoryCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get authorProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set authorProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuthorProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuthorProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get type => $_getSZ(2);
  @$pb.TagNumber(3)
  set type($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get gameTag => $_getSZ(3);
  @$pb.TagNumber(4)
  set gameTag($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGameTag() => $_has(3);
  @$pb.TagNumber(4)
  void clearGameTag() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get mentionProfileIds => $_getList(4);
}

class StoryViewed extends $pb.GeneratedMessage {
  factory StoryViewed({
    $core.String? storyId,
    $core.String? viewerProfileId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (viewerProfileId != null) result.viewerProfileId = viewerProfileId;
    return result;
  }

  StoryViewed._();

  factory StoryViewed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryViewed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryViewed',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'viewerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryViewed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryViewed copyWith(void Function(StoryViewed) updates) =>
      super.copyWith((message) => updates(message as StoryViewed))
          as StoryViewed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryViewed create() => StoryViewed._();
  @$core.override
  StoryViewed createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryViewed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryViewed>(create);
  static StoryViewed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get viewerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set viewerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasViewerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearViewerProfileId() => $_clearField(2);
}

class StoryReacted extends $pb.GeneratedMessage {
  factory StoryReacted({
    $core.String? storyId,
    $core.String? reactorProfileId,
    $core.String? emoji,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (reactorProfileId != null) result.reactorProfileId = reactorProfileId;
    if (emoji != null) result.emoji = emoji;
    return result;
  }

  StoryReacted._();

  factory StoryReacted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryReacted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryReacted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'reactorProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'emoji')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryReacted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryReacted copyWith(void Function(StoryReacted) updates) =>
      super.copyWith((message) => updates(message as StoryReacted))
          as StoryReacted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryReacted create() => StoryReacted._();
  @$core.override
  StoryReacted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryReacted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryReacted>(create);
  static StoryReacted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reactorProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set reactorProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReactorProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReactorProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get emoji => $_getSZ(2);
  @$pb.TagNumber(3)
  set emoji($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEmoji() => $_has(2);
  @$pb.TagNumber(3)
  void clearEmoji() => $_clearField(3);
}

class StoryExpired extends $pb.GeneratedMessage {
  factory StoryExpired({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  StoryExpired._();

  factory StoryExpired.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryExpired.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryExpired',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryExpired clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryExpired copyWith(void Function(StoryExpired) updates) =>
      super.copyWith((message) => updates(message as StoryExpired))
          as StoryExpired;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryExpired create() => StoryExpired._();
  @$core.override
  StoryExpired createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryExpired getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryExpired>(create);
  static StoryExpired? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class StoryHighlightCreated extends $pb.GeneratedMessage {
  factory StoryHighlightCreated({
    $core.String? highlightId,
    $core.String? profileId,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  StoryHighlightCreated._();

  factory StoryHighlightCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryHighlightCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryHighlightCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryHighlightCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryHighlightCreated copyWith(
          void Function(StoryHighlightCreated) updates) =>
      super.copyWith((message) => updates(message as StoryHighlightCreated))
          as StoryHighlightCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryHighlightCreated create() => StoryHighlightCreated._();
  @$core.override
  StoryHighlightCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryHighlightCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryHighlightCreated>(create);
  static StoryHighlightCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class StoryLfpCreated extends $pb.GeneratedMessage {
  factory StoryLfpCreated({
    $core.String? storyId,
    $core.String? authorProfileId,
    $core.String? criteriaJson,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (criteriaJson != null) result.criteriaJson = criteriaJson;
    return result;
  }

  StoryLfpCreated._();

  factory StoryLfpCreated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryLfpCreated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryLfpCreated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'authorProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'criteriaJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryLfpCreated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryLfpCreated copyWith(void Function(StoryLfpCreated) updates) =>
      super.copyWith((message) => updates(message as StoryLfpCreated))
          as StoryLfpCreated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryLfpCreated create() => StoryLfpCreated._();
  @$core.override
  StoryLfpCreated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryLfpCreated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryLfpCreated>(create);
  static StoryLfpCreated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get authorProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set authorProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAuthorProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAuthorProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get criteriaJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set criteriaJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCriteriaJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearCriteriaJson() => $_clearField(3);
}

class HighlightAdded extends $pb.GeneratedMessage {
  factory HighlightAdded({
    $core.String? highlightId,
    $core.String? profileId,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  HighlightAdded._();

  factory HighlightAdded.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HighlightAdded.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HighlightAdded',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HighlightAdded clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HighlightAdded copyWith(void Function(HighlightAdded) updates) =>
      super.copyWith((message) => updates(message as HighlightAdded))
          as HighlightAdded;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HighlightAdded create() => HighlightAdded._();
  @$core.override
  HighlightAdded createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HighlightAdded getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HighlightAdded>(create);
  static HighlightAdded? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

enum FederationStreamEvent_Payload {
  nodeConnected,
  nodeDisconnected,
  eventSynced,
  syncFailed,
  nodeDefederated,
  notSet
}

class FederationStreamEvent extends $pb.GeneratedMessage {
  factory FederationStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    NodeConnected? nodeConnected,
    NodeDisconnected? nodeDisconnected,
    EventSynced? eventSynced,
    SyncFailed? syncFailed,
    NodeDefederated? nodeDefederated,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (nodeConnected != null) result.nodeConnected = nodeConnected;
    if (nodeDisconnected != null) result.nodeDisconnected = nodeDisconnected;
    if (eventSynced != null) result.eventSynced = eventSynced;
    if (syncFailed != null) result.syncFailed = syncFailed;
    if (nodeDefederated != null) result.nodeDefederated = nodeDefederated;
    return result;
  }

  FederationStreamEvent._();

  factory FederationStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FederationStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, FederationStreamEvent_Payload>
      _FederationStreamEvent_PayloadByTag = {
    10: FederationStreamEvent_Payload.nodeConnected,
    11: FederationStreamEvent_Payload.nodeDisconnected,
    12: FederationStreamEvent_Payload.eventSynced,
    13: FederationStreamEvent_Payload.syncFailed,
    14: FederationStreamEvent_Payload.nodeDefederated,
    0: FederationStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FederationStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13, 14])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<NodeConnected>(10, _omitFieldNames ? '' : 'nodeConnected',
        subBuilder: NodeConnected.create)
    ..aOM<NodeDisconnected>(11, _omitFieldNames ? '' : 'nodeDisconnected',
        subBuilder: NodeDisconnected.create)
    ..aOM<EventSynced>(12, _omitFieldNames ? '' : 'eventSynced',
        subBuilder: EventSynced.create)
    ..aOM<SyncFailed>(13, _omitFieldNames ? '' : 'syncFailed',
        subBuilder: SyncFailed.create)
    ..aOM<NodeDefederated>(14, _omitFieldNames ? '' : 'nodeDefederated',
        subBuilder: NodeDefederated.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationStreamEvent copyWith(
          void Function(FederationStreamEvent) updates) =>
      super.copyWith((message) => updates(message as FederationStreamEvent))
          as FederationStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FederationStreamEvent create() => FederationStreamEvent._();
  @$core.override
  FederationStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FederationStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FederationStreamEvent>(create);
  static FederationStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  @$pb.TagNumber(14)
  FederationStreamEvent_Payload whichPayload() =>
      _FederationStreamEvent_PayloadByTag[$_whichOneof(0)]!;
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  NodeConnected get nodeConnected => $_getN(2);
  @$pb.TagNumber(10)
  set nodeConnected(NodeConnected value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasNodeConnected() => $_has(2);
  @$pb.TagNumber(10)
  void clearNodeConnected() => $_clearField(10);
  @$pb.TagNumber(10)
  NodeConnected ensureNodeConnected() => $_ensure(2);

  @$pb.TagNumber(11)
  NodeDisconnected get nodeDisconnected => $_getN(3);
  @$pb.TagNumber(11)
  set nodeDisconnected(NodeDisconnected value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasNodeDisconnected() => $_has(3);
  @$pb.TagNumber(11)
  void clearNodeDisconnected() => $_clearField(11);
  @$pb.TagNumber(11)
  NodeDisconnected ensureNodeDisconnected() => $_ensure(3);

  @$pb.TagNumber(12)
  EventSynced get eventSynced => $_getN(4);
  @$pb.TagNumber(12)
  set eventSynced(EventSynced value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasEventSynced() => $_has(4);
  @$pb.TagNumber(12)
  void clearEventSynced() => $_clearField(12);
  @$pb.TagNumber(12)
  EventSynced ensureEventSynced() => $_ensure(4);

  @$pb.TagNumber(13)
  SyncFailed get syncFailed => $_getN(5);
  @$pb.TagNumber(13)
  set syncFailed(SyncFailed value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasSyncFailed() => $_has(5);
  @$pb.TagNumber(13)
  void clearSyncFailed() => $_clearField(13);
  @$pb.TagNumber(13)
  SyncFailed ensureSyncFailed() => $_ensure(5);

  @$pb.TagNumber(14)
  NodeDefederated get nodeDefederated => $_getN(6);
  @$pb.TagNumber(14)
  set nodeDefederated(NodeDefederated value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasNodeDefederated() => $_has(6);
  @$pb.TagNumber(14)
  void clearNodeDefederated() => $_clearField(14);
  @$pb.TagNumber(14)
  NodeDefederated ensureNodeDefederated() => $_ensure(6);
}

class NodeConnected extends $pb.GeneratedMessage {
  factory NodeConnected({
    $core.String? nodeId,
    $core.String? host,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (host != null) result.host = host;
    return result;
  }

  NodeConnected._();

  factory NodeConnected.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NodeConnected.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NodeConnected',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'host')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeConnected clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeConnected copyWith(void Function(NodeConnected) updates) =>
      super.copyWith((message) => updates(message as NodeConnected))
          as NodeConnected;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NodeConnected create() => NodeConnected._();
  @$core.override
  NodeConnected createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NodeConnected getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NodeConnected>(create);
  static NodeConnected? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get host => $_getSZ(1);
  @$pb.TagNumber(2)
  set host($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHost() => $_has(1);
  @$pb.TagNumber(2)
  void clearHost() => $_clearField(2);
}

class NodeDisconnected extends $pb.GeneratedMessage {
  factory NodeDisconnected({
    $core.String? nodeId,
    $core.String? reason,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (reason != null) result.reason = reason;
    return result;
  }

  NodeDisconnected._();

  factory NodeDisconnected.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NodeDisconnected.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NodeDisconnected',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeDisconnected clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeDisconnected copyWith(void Function(NodeDisconnected) updates) =>
      super.copyWith((message) => updates(message as NodeDisconnected))
          as NodeDisconnected;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NodeDisconnected create() => NodeDisconnected._();
  @$core.override
  NodeDisconnected createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NodeDisconnected getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NodeDisconnected>(create);
  static NodeDisconnected? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class EventSynced extends $pb.GeneratedMessage {
  factory EventSynced({
    $core.String? nodeId,
    $core.String? eventType,
    $core.String? direction,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (eventType != null) result.eventType = eventType;
    if (direction != null) result.direction = direction;
    return result;
  }

  EventSynced._();

  factory EventSynced.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EventSynced.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EventSynced',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'eventType')
    ..aOS(3, _omitFieldNames ? '' : 'direction')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventSynced clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EventSynced copyWith(void Function(EventSynced) updates) =>
      super.copyWith((message) => updates(message as EventSynced))
          as EventSynced;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EventSynced create() => EventSynced._();
  @$core.override
  EventSynced createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EventSynced getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EventSynced>(create);
  static EventSynced? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get eventType => $_getSZ(1);
  @$pb.TagNumber(2)
  set eventType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEventType() => $_has(1);
  @$pb.TagNumber(2)
  void clearEventType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get direction => $_getSZ(2);
  @$pb.TagNumber(3)
  set direction($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDirection() => $_has(2);
  @$pb.TagNumber(3)
  void clearDirection() => $_clearField(3);
}

class SyncFailed extends $pb.GeneratedMessage {
  factory SyncFailed({
    $core.String? nodeId,
    $core.String? error,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (error != null) result.error = error;
    return result;
  }

  SyncFailed._();

  factory SyncFailed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SyncFailed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SyncFailed',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncFailed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncFailed copyWith(void Function(SyncFailed) updates) =>
      super.copyWith((message) => updates(message as SyncFailed)) as SyncFailed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SyncFailed create() => SyncFailed._();
  @$core.override
  SyncFailed createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SyncFailed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SyncFailed>(create);
  static SyncFailed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get error => $_getSZ(1);
  @$pb.TagNumber(2)
  set error($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasError() => $_has(1);
  @$pb.TagNumber(2)
  void clearError() => $_clearField(2);
}

class NodeDefederated extends $pb.GeneratedMessage {
  factory NodeDefederated({
    $core.String? nodeId,
    $core.String? reason,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (reason != null) result.reason = reason;
    return result;
  }

  NodeDefederated._();

  factory NodeDefederated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory NodeDefederated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'NodeDefederated',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeDefederated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  NodeDefederated copyWith(void Function(NodeDefederated) updates) =>
      super.copyWith((message) => updates(message as NodeDefederated))
          as NodeDefederated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static NodeDefederated create() => NodeDefederated._();
  @$core.override
  NodeDefederated createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static NodeDefederated getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<NodeDefederated>(create);
  static NodeDefederated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

enum BotStreamEvent_Payload {
  botRegistered,
  commandExecuted,
  webhookDelivered,
  webhookFailed,
  notSet
}

class BotStreamEvent extends $pb.GeneratedMessage {
  factory BotStreamEvent({
    $core.String? eventId,
    $0.Timestamp? occurredAt,
    BotRegistered? botRegistered,
    CommandExecuted? commandExecuted,
    WebhookDelivered? webhookDelivered,
    WebhookFailed? webhookFailed,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (botRegistered != null) result.botRegistered = botRegistered;
    if (commandExecuted != null) result.commandExecuted = commandExecuted;
    if (webhookDelivered != null) result.webhookDelivered = webhookDelivered;
    if (webhookFailed != null) result.webhookFailed = webhookFailed;
    return result;
  }

  BotStreamEvent._();

  factory BotStreamEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BotStreamEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, BotStreamEvent_Payload>
      _BotStreamEvent_PayloadByTag = {
    10: BotStreamEvent_Payload.botRegistered,
    11: BotStreamEvent_Payload.commandExecuted,
    12: BotStreamEvent_Payload.webhookDelivered,
    13: BotStreamEvent_Payload.webhookFailed,
    0: BotStreamEvent_Payload.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BotStreamEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..oo(0, [10, 11, 12, 13])
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOM<$0.Timestamp>(2, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<BotRegistered>(10, _omitFieldNames ? '' : 'botRegistered',
        subBuilder: BotRegistered.create)
    ..aOM<CommandExecuted>(11, _omitFieldNames ? '' : 'commandExecuted',
        subBuilder: CommandExecuted.create)
    ..aOM<WebhookDelivered>(12, _omitFieldNames ? '' : 'webhookDelivered',
        subBuilder: WebhookDelivered.create)
    ..aOM<WebhookFailed>(13, _omitFieldNames ? '' : 'webhookFailed',
        subBuilder: WebhookFailed.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotStreamEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotStreamEvent copyWith(void Function(BotStreamEvent) updates) =>
      super.copyWith((message) => updates(message as BotStreamEvent))
          as BotStreamEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BotStreamEvent create() => BotStreamEvent._();
  @$core.override
  BotStreamEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BotStreamEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BotStreamEvent>(create);
  static BotStreamEvent? _defaultInstance;

  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
  BotStreamEvent_Payload whichPayload() =>
      _BotStreamEvent_PayloadByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(10)
  @$pb.TagNumber(11)
  @$pb.TagNumber(12)
  @$pb.TagNumber(13)
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
  $0.Timestamp get occurredAt => $_getN(1);
  @$pb.TagNumber(2)
  set occurredAt($0.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOccurredAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearOccurredAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.Timestamp ensureOccurredAt() => $_ensure(1);

  @$pb.TagNumber(10)
  BotRegistered get botRegistered => $_getN(2);
  @$pb.TagNumber(10)
  set botRegistered(BotRegistered value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasBotRegistered() => $_has(2);
  @$pb.TagNumber(10)
  void clearBotRegistered() => $_clearField(10);
  @$pb.TagNumber(10)
  BotRegistered ensureBotRegistered() => $_ensure(2);

  @$pb.TagNumber(11)
  CommandExecuted get commandExecuted => $_getN(3);
  @$pb.TagNumber(11)
  set commandExecuted(CommandExecuted value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCommandExecuted() => $_has(3);
  @$pb.TagNumber(11)
  void clearCommandExecuted() => $_clearField(11);
  @$pb.TagNumber(11)
  CommandExecuted ensureCommandExecuted() => $_ensure(3);

  @$pb.TagNumber(12)
  WebhookDelivered get webhookDelivered => $_getN(4);
  @$pb.TagNumber(12)
  set webhookDelivered(WebhookDelivered value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasWebhookDelivered() => $_has(4);
  @$pb.TagNumber(12)
  void clearWebhookDelivered() => $_clearField(12);
  @$pb.TagNumber(12)
  WebhookDelivered ensureWebhookDelivered() => $_ensure(4);

  @$pb.TagNumber(13)
  WebhookFailed get webhookFailed => $_getN(5);
  @$pb.TagNumber(13)
  set webhookFailed(WebhookFailed value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasWebhookFailed() => $_has(5);
  @$pb.TagNumber(13)
  void clearWebhookFailed() => $_clearField(13);
  @$pb.TagNumber(13)
  WebhookFailed ensureWebhookFailed() => $_ensure(5);
}

class BotRegistered extends $pb.GeneratedMessage {
  factory BotRegistered({
    $core.String? botId,
    $core.String? ownerAccountId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (ownerAccountId != null) result.ownerAccountId = ownerAccountId;
    return result;
  }

  BotRegistered._();

  factory BotRegistered.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BotRegistered.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BotRegistered',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'ownerAccountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotRegistered clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BotRegistered copyWith(void Function(BotRegistered) updates) =>
      super.copyWith((message) => updates(message as BotRegistered))
          as BotRegistered;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BotRegistered create() => BotRegistered._();
  @$core.override
  BotRegistered createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BotRegistered getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BotRegistered>(create);
  static BotRegistered? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get ownerAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set ownerAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerAccountId() => $_clearField(2);
}

class CommandExecuted extends $pb.GeneratedMessage {
  factory CommandExecuted({
    $core.String? botId,
    $core.String? command,
    $core.String? chatId,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (command != null) result.command = command;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  CommandExecuted._();

  factory CommandExecuted.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CommandExecuted.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CommandExecuted',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'command')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommandExecuted clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommandExecuted copyWith(void Function(CommandExecuted) updates) =>
      super.copyWith((message) => updates(message as CommandExecuted))
          as CommandExecuted;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CommandExecuted create() => CommandExecuted._();
  @$core.override
  CommandExecuted createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CommandExecuted getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CommandExecuted>(create);
  static CommandExecuted? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get command => $_getSZ(1);
  @$pb.TagNumber(2)
  set command($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCommand() => $_has(1);
  @$pb.TagNumber(2)
  void clearCommand() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);
}

class WebhookDelivered extends $pb.GeneratedMessage {
  factory WebhookDelivered({
    $core.String? botId,
    $core.String? deliveryId,
    $core.bool? success,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (deliveryId != null) result.deliveryId = deliveryId;
    if (success != null) result.success = success;
    return result;
  }

  WebhookDelivered._();

  factory WebhookDelivered.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory WebhookDelivered.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WebhookDelivered',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'deliveryId')
    ..aOB(3, _omitFieldNames ? '' : 'success')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WebhookDelivered clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WebhookDelivered copyWith(void Function(WebhookDelivered) updates) =>
      super.copyWith((message) => updates(message as WebhookDelivered))
          as WebhookDelivered;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static WebhookDelivered create() => WebhookDelivered._();
  @$core.override
  WebhookDelivered createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static WebhookDelivered getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WebhookDelivered>(create);
  static WebhookDelivered? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get deliveryId => $_getSZ(1);
  @$pb.TagNumber(2)
  set deliveryId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeliveryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeliveryId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get success => $_getBF(2);
  @$pb.TagNumber(3)
  set success($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSuccess() => $_has(2);
  @$pb.TagNumber(3)
  void clearSuccess() => $_clearField(3);
}

class WebhookFailed extends $pb.GeneratedMessage {
  factory WebhookFailed({
    $core.String? botId,
    $core.String? eventType,
    $core.String? error,
  }) {
    final result = create();
    if (botId != null) result.botId = botId;
    if (eventType != null) result.eventType = eventType;
    if (error != null) result.error = error;
    return result;
  }

  WebhookFailed._();

  factory WebhookFailed.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory WebhookFailed.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'WebhookFailed',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.events.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'botId')
    ..aOS(2, _omitFieldNames ? '' : 'eventType')
    ..aOS(3, _omitFieldNames ? '' : 'error')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WebhookFailed clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  WebhookFailed copyWith(void Function(WebhookFailed) updates) =>
      super.copyWith((message) => updates(message as WebhookFailed))
          as WebhookFailed;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static WebhookFailed create() => WebhookFailed._();
  @$core.override
  WebhookFailed createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static WebhookFailed getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<WebhookFailed>(create);
  static WebhookFailed? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get botId => $_getSZ(0);
  @$pb.TagNumber(1)
  set botId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBotId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBotId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get eventType => $_getSZ(1);
  @$pb.TagNumber(2)
  set eventType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEventType() => $_has(1);
  @$pb.TagNumber(2)
  void clearEventType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get error => $_getSZ(2);
  @$pb.TagNumber(3)
  set error($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasError() => $_has(2);
  @$pb.TagNumber(3)
  void clearError() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
