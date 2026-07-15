// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user.proto.

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
    as $1;

import '../../common/v1/common.pb.dart' as $2;
import 'user.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'user.pbenum.dart';

class EnsurePrimaryProfileRequest extends $pb.GeneratedMessage {
  factory EnsurePrimaryProfileRequest({
    $core.String? accountId,
    $core.String? profileId,
    $core.String? displayHint,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (displayHint != null) result.displayHint = displayHint;
    return result;
  }

  EnsurePrimaryProfileRequest._();

  factory EnsurePrimaryProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EnsurePrimaryProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EnsurePrimaryProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'displayHint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnsurePrimaryProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnsurePrimaryProfileRequest copyWith(
          void Function(EnsurePrimaryProfileRequest) updates) =>
      super.copyWith(
              (message) => updates(message as EnsurePrimaryProfileRequest))
          as EnsurePrimaryProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EnsurePrimaryProfileRequest create() =>
      EnsurePrimaryProfileRequest._();
  @$core.override
  EnsurePrimaryProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EnsurePrimaryProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EnsurePrimaryProfileRequest>(create);
  static EnsurePrimaryProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  /// Optional explicit profile UUID (Auth may pre-generate); if empty, User generates id.
  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  /// Hint for display_name / username base (e.g. email local part).
  @$pb.TagNumber(3)
  $core.String get displayHint => $_getSZ(2);
  @$pb.TagNumber(3)
  set displayHint($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDisplayHint() => $_has(2);
  @$pb.TagNumber(3)
  void clearDisplayHint() => $_clearField(3);
}

class EnsurePrimaryProfileResponse extends $pb.GeneratedMessage {
  factory EnsurePrimaryProfileResponse({
    Profile? profile,
  }) {
    final result = create();
    if (profile != null) result.profile = profile;
    return result;
  }

  EnsurePrimaryProfileResponse._();

  factory EnsurePrimaryProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory EnsurePrimaryProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'EnsurePrimaryProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnsurePrimaryProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  EnsurePrimaryProfileResponse copyWith(
          void Function(EnsurePrimaryProfileResponse) updates) =>
      super.copyWith(
              (message) => updates(message as EnsurePrimaryProfileResponse))
          as EnsurePrimaryProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static EnsurePrimaryProfileResponse create() =>
      EnsurePrimaryProfileResponse._();
  @$core.override
  EnsurePrimaryProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static EnsurePrimaryProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<EnsurePrimaryProfileResponse>(create);
  static EnsurePrimaryProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

enum GetProfileRequest_By { profileId, username, notSet }

class GetProfileRequest extends $pb.GeneratedMessage {
  factory GetProfileRequest({
    $core.String? profileId,
    $core.String? username,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (username != null) result.username = username;
    return result;
  }

  GetProfileRequest._();

  factory GetProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, GetProfileRequest_By>
      _GetProfileRequest_ByByTag = {
    1: GetProfileRequest_By.profileId,
    2: GetProfileRequest_By.username,
    0: GetProfileRequest_By.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2])
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'username')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileRequest copyWith(void Function(GetProfileRequest) updates) =>
      super.copyWith((message) => updates(message as GetProfileRequest))
          as GetProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfileRequest create() => GetProfileRequest._();
  @$core.override
  GetProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfileRequest>(create);
  static GetProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  GetProfileRequest_By whichBy() =>
      _GetProfileRequest_ByByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearBy() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get username => $_getSZ(1);
  @$pb.TagNumber(2)
  set username($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUsername() => $_has(1);
  @$pb.TagNumber(2)
  void clearUsername() => $_clearField(2);
}

class GetProfilesRequest extends $pb.GeneratedMessage {
  factory GetProfilesRequest({
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  GetProfilesRequest._();

  factory GetProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfilesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfilesRequest copyWith(void Function(GetProfilesRequest) updates) =>
      super.copyWith((message) => updates(message as GetProfilesRequest))
          as GetProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfilesRequest create() => GetProfilesRequest._();
  @$core.override
  GetProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfilesRequest>(create);
  static GetProfilesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get profileIds => $_getList(0);
}

class ProfileList extends $pb.GeneratedMessage {
  factory ProfileList({
    $core.Iterable<Profile>? profiles,
  }) {
    final result = create();
    if (profiles != null) result.profiles.addAll(profiles);
    return result;
  }

  ProfileList._();

  factory ProfileList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProfileList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProfileList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..pPM<Profile>(1, _omitFieldNames ? '' : 'profiles',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileList copyWith(void Function(ProfileList) updates) =>
      super.copyWith((message) => updates(message as ProfileList))
          as ProfileList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProfileList create() => ProfileList._();
  @$core.override
  ProfileList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProfileList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProfileList>(create);
  static ProfileList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Profile> get profiles => $_getList(0);
}

class Profile extends $pb.GeneratedMessage {
  factory Profile({
    $core.String? id,
    $core.String? accountId,
    $core.String? username,
    $core.String? discriminator,
    $core.String? displayName,
    $core.String? avatarUrl,
    $core.String? bannerUrl,
    $core.String? bio,
    $core.String? customStatus,
    $core.String? locale,
    $core.String? theme,
    $core.bool? isPrimary,
    $core.String? verificationType,
    $core.String? verificationBadge,
    $1.Timestamp? createdAt,
    $1.Timestamp? updatedAt,
    $1.Timestamp? frozenAt,
    $core.String? accentColor,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (accountId != null) result.accountId = accountId;
    if (username != null) result.username = username;
    if (discriminator != null) result.discriminator = discriminator;
    if (displayName != null) result.displayName = displayName;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (bannerUrl != null) result.bannerUrl = bannerUrl;
    if (bio != null) result.bio = bio;
    if (customStatus != null) result.customStatus = customStatus;
    if (locale != null) result.locale = locale;
    if (theme != null) result.theme = theme;
    if (isPrimary != null) result.isPrimary = isPrimary;
    if (verificationType != null) result.verificationType = verificationType;
    if (verificationBadge != null) result.verificationBadge = verificationBadge;
    if (createdAt != null) result.createdAt = createdAt;
    if (updatedAt != null) result.updatedAt = updatedAt;
    if (frozenAt != null) result.frozenAt = frozenAt;
    if (accentColor != null) result.accentColor = accentColor;
    return result;
  }

  Profile._();

  factory Profile.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Profile.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Profile',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'username')
    ..aOS(4, _omitFieldNames ? '' : 'discriminator')
    ..aOS(5, _omitFieldNames ? '' : 'displayName')
    ..aOS(6, _omitFieldNames ? '' : 'avatarUrl')
    ..aOS(7, _omitFieldNames ? '' : 'bannerUrl')
    ..aOS(8, _omitFieldNames ? '' : 'bio')
    ..aOS(9, _omitFieldNames ? '' : 'customStatus')
    ..aOS(10, _omitFieldNames ? '' : 'locale')
    ..aOS(11, _omitFieldNames ? '' : 'theme')
    ..aOB(12, _omitFieldNames ? '' : 'isPrimary')
    ..aOS(13, _omitFieldNames ? '' : 'verificationType')
    ..aOS(14, _omitFieldNames ? '' : 'verificationBadge')
    ..aOM<$1.Timestamp>(15, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(16, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(17, _omitFieldNames ? '' : 'frozenAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(18, _omitFieldNames ? '' : 'accentColor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Profile clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Profile copyWith(void Function(Profile) updates) =>
      super.copyWith((message) => updates(message as Profile)) as Profile;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Profile create() => Profile._();
  @$core.override
  Profile createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Profile getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Profile>(create);
  static Profile? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get username => $_getSZ(2);
  @$pb.TagNumber(3)
  set username($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasUsername() => $_has(2);
  @$pb.TagNumber(3)
  void clearUsername() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get discriminator => $_getSZ(3);
  @$pb.TagNumber(4)
  set discriminator($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDiscriminator() => $_has(3);
  @$pb.TagNumber(4)
  void clearDiscriminator() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get displayName => $_getSZ(4);
  @$pb.TagNumber(5)
  set displayName($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDisplayName() => $_has(4);
  @$pb.TagNumber(5)
  void clearDisplayName() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get avatarUrl => $_getSZ(5);
  @$pb.TagNumber(6)
  set avatarUrl($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasAvatarUrl() => $_has(5);
  @$pb.TagNumber(6)
  void clearAvatarUrl() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get bannerUrl => $_getSZ(6);
  @$pb.TagNumber(7)
  set bannerUrl($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasBannerUrl() => $_has(6);
  @$pb.TagNumber(7)
  void clearBannerUrl() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get bio => $_getSZ(7);
  @$pb.TagNumber(8)
  set bio($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasBio() => $_has(7);
  @$pb.TagNumber(8)
  void clearBio() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get customStatus => $_getSZ(8);
  @$pb.TagNumber(9)
  set customStatus($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasCustomStatus() => $_has(8);
  @$pb.TagNumber(9)
  void clearCustomStatus() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get locale => $_getSZ(9);
  @$pb.TagNumber(10)
  set locale($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasLocale() => $_has(9);
  @$pb.TagNumber(10)
  void clearLocale() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get theme => $_getSZ(10);
  @$pb.TagNumber(11)
  set theme($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasTheme() => $_has(10);
  @$pb.TagNumber(11)
  void clearTheme() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.bool get isPrimary => $_getBF(11);
  @$pb.TagNumber(12)
  set isPrimary($core.bool value) => $_setBool(11, value);
  @$pb.TagNumber(12)
  $core.bool hasIsPrimary() => $_has(11);
  @$pb.TagNumber(12)
  void clearIsPrimary() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get verificationType => $_getSZ(12);
  @$pb.TagNumber(13)
  set verificationType($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasVerificationType() => $_has(12);
  @$pb.TagNumber(13)
  void clearVerificationType() => $_clearField(13);

  @$pb.TagNumber(14)
  $core.String get verificationBadge => $_getSZ(13);
  @$pb.TagNumber(14)
  set verificationBadge($core.String value) => $_setString(13, value);
  @$pb.TagNumber(14)
  $core.bool hasVerificationBadge() => $_has(13);
  @$pb.TagNumber(14)
  void clearVerificationBadge() => $_clearField(14);

  @$pb.TagNumber(15)
  $1.Timestamp get createdAt => $_getN(14);
  @$pb.TagNumber(15)
  set createdAt($1.Timestamp value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasCreatedAt() => $_has(14);
  @$pb.TagNumber(15)
  void clearCreatedAt() => $_clearField(15);
  @$pb.TagNumber(15)
  $1.Timestamp ensureCreatedAt() => $_ensure(14);

  @$pb.TagNumber(16)
  $1.Timestamp get updatedAt => $_getN(15);
  @$pb.TagNumber(16)
  set updatedAt($1.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasUpdatedAt() => $_has(15);
  @$pb.TagNumber(16)
  void clearUpdatedAt() => $_clearField(16);
  @$pb.TagNumber(16)
  $1.Timestamp ensureUpdatedAt() => $_ensure(15);

  @$pb.TagNumber(17)
  $1.Timestamp get frozenAt => $_getN(16);
  @$pb.TagNumber(17)
  set frozenAt($1.Timestamp value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasFrozenAt() => $_has(16);
  @$pb.TagNumber(17)
  void clearFrozenAt() => $_clearField(17);
  @$pb.TagNumber(17)
  $1.Timestamp ensureFrozenAt() => $_ensure(16);

  @$pb.TagNumber(18)
  $core.String get accentColor => $_getSZ(17);
  @$pb.TagNumber(18)
  set accentColor($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasAccentColor() => $_has(17);
  @$pb.TagNumber(18)
  void clearAccentColor() => $_clearField(18);
}

class UpdateProfileRequest extends $pb.GeneratedMessage {
  factory UpdateProfileRequest({
    $core.String? profileId,
    $core.String? displayName,
    $core.String? avatarUrl,
    $core.String? bannerUrl,
    $core.String? bio,
    $core.String? customStatus,
    $core.String? locale,
    $core.String? theme,
    $core.String? accentColor,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (displayName != null) result.displayName = displayName;
    if (avatarUrl != null) result.avatarUrl = avatarUrl;
    if (bannerUrl != null) result.bannerUrl = bannerUrl;
    if (bio != null) result.bio = bio;
    if (customStatus != null) result.customStatus = customStatus;
    if (locale != null) result.locale = locale;
    if (theme != null) result.theme = theme;
    if (accentColor != null) result.accentColor = accentColor;
    return result;
  }

  UpdateProfileRequest._();

  factory UpdateProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'displayName')
    ..aOS(3, _omitFieldNames ? '' : 'avatarUrl')
    ..aOS(4, _omitFieldNames ? '' : 'bannerUrl')
    ..aOS(5, _omitFieldNames ? '' : 'bio')
    ..aOS(6, _omitFieldNames ? '' : 'customStatus')
    ..aOS(7, _omitFieldNames ? '' : 'locale')
    ..aOS(8, _omitFieldNames ? '' : 'theme')
    ..aOS(9, _omitFieldNames ? '' : 'accentColor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileRequest copyWith(void Function(UpdateProfileRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateProfileRequest))
          as UpdateProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateProfileRequest create() => UpdateProfileRequest._();
  @$core.override
  UpdateProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProfileRequest>(create);
  static UpdateProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get displayName => $_getSZ(1);
  @$pb.TagNumber(2)
  set displayName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDisplayName() => $_has(1);
  @$pb.TagNumber(2)
  void clearDisplayName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get avatarUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set avatarUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAvatarUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearAvatarUrl() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get bannerUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set bannerUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBannerUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearBannerUrl() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get bio => $_getSZ(4);
  @$pb.TagNumber(5)
  set bio($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBio() => $_has(4);
  @$pb.TagNumber(5)
  void clearBio() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get customStatus => $_getSZ(5);
  @$pb.TagNumber(6)
  set customStatus($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCustomStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearCustomStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get locale => $_getSZ(6);
  @$pb.TagNumber(7)
  set locale($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasLocale() => $_has(6);
  @$pb.TagNumber(7)
  void clearLocale() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get theme => $_getSZ(7);
  @$pb.TagNumber(8)
  set theme($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasTheme() => $_has(7);
  @$pb.TagNumber(8)
  void clearTheme() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get accentColor => $_getSZ(8);
  @$pb.TagNumber(9)
  set accentColor($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasAccentColor() => $_has(8);
  @$pb.TagNumber(9)
  void clearAccentColor() => $_clearField(9);
}

class CreateProfileRequest extends $pb.GeneratedMessage {
  factory CreateProfileRequest({
    $core.String? displayName,
    $core.String? username,
    $core.String? preset,
    $core.String? accentColor,
  }) {
    final result = create();
    if (displayName != null) result.displayName = displayName;
    if (username != null) result.username = username;
    if (preset != null) result.preset = preset;
    if (accentColor != null) result.accentColor = accentColor;
    return result;
  }

  CreateProfileRequest._();

  factory CreateProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'displayName')
    ..aOS(2, _omitFieldNames ? '' : 'username')
    ..aOS(3, _omitFieldNames ? '' : 'preset')
    ..aOS(4, _omitFieldNames ? '' : 'accentColor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProfileRequest copyWith(void Function(CreateProfileRequest) updates) =>
      super.copyWith((message) => updates(message as CreateProfileRequest))
          as CreateProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateProfileRequest create() => CreateProfileRequest._();
  @$core.override
  CreateProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateProfileRequest>(create);
  static CreateProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get displayName => $_getSZ(0);
  @$pb.TagNumber(1)
  set displayName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDisplayName() => $_has(0);
  @$pb.TagNumber(1)
  void clearDisplayName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get username => $_getSZ(1);
  @$pb.TagNumber(2)
  set username($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUsername() => $_has(1);
  @$pb.TagNumber(2)
  void clearUsername() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get preset => $_getSZ(2);
  @$pb.TagNumber(3)
  set preset($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPreset() => $_has(2);
  @$pb.TagNumber(3)
  void clearPreset() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get accentColor => $_getSZ(3);
  @$pb.TagNumber(4)
  set accentColor($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAccentColor() => $_has(3);
  @$pb.TagNumber(4)
  void clearAccentColor() => $_clearField(4);
}

class DeleteProfileRequest extends $pb.GeneratedMessage {
  factory DeleteProfileRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  DeleteProfileRequest._();

  factory DeleteProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProfileRequest copyWith(void Function(DeleteProfileRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteProfileRequest))
          as DeleteProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteProfileRequest create() => DeleteProfileRequest._();
  @$core.override
  DeleteProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteProfileRequest>(create);
  static DeleteProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class SwitchProfileRequest extends $pb.GeneratedMessage {
  factory SwitchProfileRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  SwitchProfileRequest._();

  factory SwitchProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SwitchProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SwitchProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchProfileRequest copyWith(void Function(SwitchProfileRequest) updates) =>
      super.copyWith((message) => updates(message as SwitchProfileRequest))
          as SwitchProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SwitchProfileRequest create() => SwitchProfileRequest._();
  @$core.override
  SwitchProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SwitchProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SwitchProfileRequest>(create);
  static SwitchProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class ListMyProfilesRequest extends $pb.GeneratedMessage {
  factory ListMyProfilesRequest() => create();

  ListMyProfilesRequest._();

  factory ListMyProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMyProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyProfilesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProfilesRequest copyWith(
          void Function(ListMyProfilesRequest) updates) =>
      super.copyWith((message) => updates(message as ListMyProfilesRequest))
          as ListMyProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMyProfilesRequest create() => ListMyProfilesRequest._();
  @$core.override
  ListMyProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMyProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyProfilesRequest>(create);
  static ListMyProfilesRequest? _defaultInstance;
}

class SearchProfilesRequest extends $pb.GeneratedMessage {
  factory SearchProfilesRequest({
    $core.String? query,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (query != null) result.query = query;
    if (page != null) result.page = page;
    return result;
  }

  SearchProfilesRequest._();

  factory SearchProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchProfilesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProfilesRequest copyWith(
          void Function(SearchProfilesRequest) updates) =>
      super.copyWith((message) => updates(message as SearchProfilesRequest))
          as SearchProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchProfilesRequest create() => SearchProfilesRequest._();
  @$core.override
  SearchProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchProfilesRequest>(create);
  static SearchProfilesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.CursorPageRequest get page => $_getN(1);
  @$pb.TagNumber(2)
  set page($2.CursorPageRequest value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPage() => $_has(1);
  @$pb.TagNumber(2)
  void clearPage() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.CursorPageRequest ensurePage() => $_ensure(1);
}

class SearchProfilesResponse extends $pb.GeneratedMessage {
  factory SearchProfilesResponse({
    ProfileList? profileList,
    $2.CursorPageResponse? page,
  }) {
    final result = create();
    if (profileList != null) result.profileList = profileList;
    if (page != null) result.page = page;
    return result;
  }

  SearchProfilesResponse._();

  factory SearchProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchProfilesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<ProfileList>(1, _omitFieldNames ? '' : 'profileList',
        subBuilder: ProfileList.create)
    ..aOM<$2.CursorPageResponse>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchProfilesResponse copyWith(
          void Function(SearchProfilesResponse) updates) =>
      super.copyWith((message) => updates(message as SearchProfilesResponse))
          as SearchProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchProfilesResponse create() => SearchProfilesResponse._();
  @$core.override
  SearchProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchProfilesResponse>(create);
  static SearchProfilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ProfileList get profileList => $_getN(0);
  @$pb.TagNumber(1)
  set profileList(ProfileList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileList() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileList() => $_clearField(1);
  @$pb.TagNumber(1)
  ProfileList ensureProfileList() => $_ensure(0);

  @$pb.TagNumber(2)
  $2.CursorPageResponse get page => $_getN(1);
  @$pb.TagNumber(2)
  set page($2.CursorPageResponse value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPage() => $_has(1);
  @$pb.TagNumber(2)
  void clearPage() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.CursorPageResponse ensurePage() => $_ensure(1);
}

class GetPrivacySettingsRequest extends $pb.GeneratedMessage {
  factory GetPrivacySettingsRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetPrivacySettingsRequest._();

  factory GetPrivacySettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPrivacySettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPrivacySettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPrivacySettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPrivacySettingsRequest copyWith(
          void Function(GetPrivacySettingsRequest) updates) =>
      super.copyWith((message) => updates(message as GetPrivacySettingsRequest))
          as GetPrivacySettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPrivacySettingsRequest create() => GetPrivacySettingsRequest._();
  @$core.override
  GetPrivacySettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPrivacySettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPrivacySettingsRequest>(create);
  static GetPrivacySettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class UpdatePrivacySettingsRequest extends $pb.GeneratedMessage {
  factory UpdatePrivacySettingsRequest({
    $core.String? profileId,
    PrivacySettings? settings,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (settings != null) result.settings = settings;
    return result;
  }

  UpdatePrivacySettingsRequest._();

  factory UpdatePrivacySettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdatePrivacySettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePrivacySettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<PrivacySettings>(2, _omitFieldNames ? '' : 'settings',
        subBuilder: PrivacySettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePrivacySettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePrivacySettingsRequest copyWith(
          void Function(UpdatePrivacySettingsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as UpdatePrivacySettingsRequest))
          as UpdatePrivacySettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdatePrivacySettingsRequest create() =>
      UpdatePrivacySettingsRequest._();
  @$core.override
  UpdatePrivacySettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdatePrivacySettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePrivacySettingsRequest>(create);
  static UpdatePrivacySettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  PrivacySettings get settings => $_getN(1);
  @$pb.TagNumber(2)
  set settings(PrivacySettings value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSettings() => $_has(1);
  @$pb.TagNumber(2)
  void clearSettings() => $_clearField(2);
  @$pb.TagNumber(2)
  PrivacySettings ensureSettings() => $_ensure(1);
}

/// Multiselect audience union (privacy.md §«Контрол выбора аудитории»).
class PrivacyAudience extends $pb.GeneratedMessage {
  factory PrivacyAudience({
    $core.bool? friends,
    $core.bool? friendsOfFriends,
    $core.bool? spaceMembers,
    $core.Iterable<$core.String>? spaceIds,
    $core.bool? includeGuests,
  }) {
    final result = create();
    if (friends != null) result.friends = friends;
    if (friendsOfFriends != null) result.friendsOfFriends = friendsOfFriends;
    if (spaceMembers != null) result.spaceMembers = spaceMembers;
    if (spaceIds != null) result.spaceIds.addAll(spaceIds);
    if (includeGuests != null) result.includeGuests = includeGuests;
    return result;
  }

  PrivacyAudience._();

  factory PrivacyAudience.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrivacyAudience.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrivacyAudience',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'friends')
    ..aOB(2, _omitFieldNames ? '' : 'friendsOfFriends')
    ..aOB(3, _omitFieldNames ? '' : 'spaceMembers')
    ..pPS(4, _omitFieldNames ? '' : 'spaceIds')
    ..aOB(5, _omitFieldNames ? '' : 'includeGuests')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrivacyAudience clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrivacyAudience copyWith(void Function(PrivacyAudience) updates) =>
      super.copyWith((message) => updates(message as PrivacyAudience))
          as PrivacyAudience;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrivacyAudience create() => PrivacyAudience._();
  @$core.override
  PrivacyAudience createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PrivacyAudience getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PrivacyAudience>(create);
  static PrivacyAudience? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get friends => $_getBF(0);
  @$pb.TagNumber(1)
  set friends($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFriends() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriends() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get friendsOfFriends => $_getBF(1);
  @$pb.TagNumber(2)
  set friendsOfFriends($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFriendsOfFriends() => $_has(1);
  @$pb.TagNumber(2)
  void clearFriendsOfFriends() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get spaceMembers => $_getBF(2);
  @$pb.TagNumber(3)
  set spaceMembers($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceMembers() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceMembers() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get spaceIds => $_getList(3);

  @$pb.TagNumber(5)
  $core.bool get includeGuests => $_getBF(4);
  @$pb.TagNumber(5)
  set includeGuests($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasIncludeGuests() => $_has(4);
  @$pb.TagNumber(5)
  void clearIncludeGuests() => $_clearField(5);
}

class PrivacySettings extends $pb.GeneratedMessage {
  factory PrivacySettings({
    $core.String? profileId,
    $core.String? preset,
    PrivacyAudience? showOnline,
    PrivacyAudience? showGameStatus,
    PrivacyAudience? showMmRating,
    PrivacyAudience? showPhone,
    PrivacyAudience? showStories,
    PrivacyAudience? allowDm,
    PrivacyAudience? allowFriendRequests,
    $core.bool? allowGuestDm,
    $1.Timestamp? updatedAt,
    PrivacyPreset? presetEnum,
    PrivacyAudience? allowPhoneSearch,
    PrivacyAudience? allowCalls,
    PrivacyAudience? allowChatSpaceInvites,
    PrivacyAudience? allowFiles,
    PrivacyAudience? allowVoiceMessages,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (preset != null) result.preset = preset;
    if (showOnline != null) result.showOnline = showOnline;
    if (showGameStatus != null) result.showGameStatus = showGameStatus;
    if (showMmRating != null) result.showMmRating = showMmRating;
    if (showPhone != null) result.showPhone = showPhone;
    if (showStories != null) result.showStories = showStories;
    if (allowDm != null) result.allowDm = allowDm;
    if (allowFriendRequests != null)
      result.allowFriendRequests = allowFriendRequests;
    if (allowGuestDm != null) result.allowGuestDm = allowGuestDm;
    if (updatedAt != null) result.updatedAt = updatedAt;
    if (presetEnum != null) result.presetEnum = presetEnum;
    if (allowPhoneSearch != null) result.allowPhoneSearch = allowPhoneSearch;
    if (allowCalls != null) result.allowCalls = allowCalls;
    if (allowChatSpaceInvites != null)
      result.allowChatSpaceInvites = allowChatSpaceInvites;
    if (allowFiles != null) result.allowFiles = allowFiles;
    if (allowVoiceMessages != null)
      result.allowVoiceMessages = allowVoiceMessages;
    return result;
  }

  PrivacySettings._();

  factory PrivacySettings.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrivacySettings.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrivacySettings',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'preset')
    ..aOM<PrivacyAudience>(3, _omitFieldNames ? '' : 'showOnline',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(4, _omitFieldNames ? '' : 'showGameStatus',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(5, _omitFieldNames ? '' : 'showMmRating',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(6, _omitFieldNames ? '' : 'showPhone',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(7, _omitFieldNames ? '' : 'showStories',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(8, _omitFieldNames ? '' : 'allowDm',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(9, _omitFieldNames ? '' : 'allowFriendRequests',
        subBuilder: PrivacyAudience.create)
    ..aOB(10, _omitFieldNames ? '' : 'allowGuestDm')
    ..aOM<$1.Timestamp>(11, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $1.Timestamp.create)
    ..aE<PrivacyPreset>(12, _omitFieldNames ? '' : 'presetEnum',
        enumValues: PrivacyPreset.values)
    ..aOM<PrivacyAudience>(14, _omitFieldNames ? '' : 'allowPhoneSearch',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(15, _omitFieldNames ? '' : 'allowCalls',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(16, _omitFieldNames ? '' : 'allowChatSpaceInvites',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(17, _omitFieldNames ? '' : 'allowFiles',
        subBuilder: PrivacyAudience.create)
    ..aOM<PrivacyAudience>(18, _omitFieldNames ? '' : 'allowVoiceMessages',
        subBuilder: PrivacyAudience.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrivacySettings clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrivacySettings copyWith(void Function(PrivacySettings) updates) =>
      super.copyWith((message) => updates(message as PrivacySettings))
          as PrivacySettings;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrivacySettings create() => PrivacySettings._();
  @$core.override
  PrivacySettings createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PrivacySettings getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PrivacySettings>(create);
  static PrivacySettings? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get preset => $_getSZ(1);
  @$pb.TagNumber(2)
  set preset($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPreset() => $_has(1);
  @$pb.TagNumber(2)
  void clearPreset() => $_clearField(2);

  @$pb.TagNumber(3)
  PrivacyAudience get showOnline => $_getN(2);
  @$pb.TagNumber(3)
  set showOnline(PrivacyAudience value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasShowOnline() => $_has(2);
  @$pb.TagNumber(3)
  void clearShowOnline() => $_clearField(3);
  @$pb.TagNumber(3)
  PrivacyAudience ensureShowOnline() => $_ensure(2);

  @$pb.TagNumber(4)
  PrivacyAudience get showGameStatus => $_getN(3);
  @$pb.TagNumber(4)
  set showGameStatus(PrivacyAudience value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasShowGameStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearShowGameStatus() => $_clearField(4);
  @$pb.TagNumber(4)
  PrivacyAudience ensureShowGameStatus() => $_ensure(3);

  @$pb.TagNumber(5)
  PrivacyAudience get showMmRating => $_getN(4);
  @$pb.TagNumber(5)
  set showMmRating(PrivacyAudience value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasShowMmRating() => $_has(4);
  @$pb.TagNumber(5)
  void clearShowMmRating() => $_clearField(5);
  @$pb.TagNumber(5)
  PrivacyAudience ensureShowMmRating() => $_ensure(4);

  @$pb.TagNumber(6)
  PrivacyAudience get showPhone => $_getN(5);
  @$pb.TagNumber(6)
  set showPhone(PrivacyAudience value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasShowPhone() => $_has(5);
  @$pb.TagNumber(6)
  void clearShowPhone() => $_clearField(6);
  @$pb.TagNumber(6)
  PrivacyAudience ensureShowPhone() => $_ensure(5);

  @$pb.TagNumber(7)
  PrivacyAudience get showStories => $_getN(6);
  @$pb.TagNumber(7)
  set showStories(PrivacyAudience value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasShowStories() => $_has(6);
  @$pb.TagNumber(7)
  void clearShowStories() => $_clearField(7);
  @$pb.TagNumber(7)
  PrivacyAudience ensureShowStories() => $_ensure(6);

  @$pb.TagNumber(8)
  PrivacyAudience get allowDm => $_getN(7);
  @$pb.TagNumber(8)
  set allowDm(PrivacyAudience value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasAllowDm() => $_has(7);
  @$pb.TagNumber(8)
  void clearAllowDm() => $_clearField(8);
  @$pb.TagNumber(8)
  PrivacyAudience ensureAllowDm() => $_ensure(7);

  @$pb.TagNumber(9)
  PrivacyAudience get allowFriendRequests => $_getN(8);
  @$pb.TagNumber(9)
  set allowFriendRequests(PrivacyAudience value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasAllowFriendRequests() => $_has(8);
  @$pb.TagNumber(9)
  void clearAllowFriendRequests() => $_clearField(9);
  @$pb.TagNumber(9)
  PrivacyAudience ensureAllowFriendRequests() => $_ensure(8);

  @$pb.TagNumber(10)
  $core.bool get allowGuestDm => $_getBF(9);
  @$pb.TagNumber(10)
  set allowGuestDm($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasAllowGuestDm() => $_has(9);
  @$pb.TagNumber(10)
  void clearAllowGuestDm() => $_clearField(10);

  @$pb.TagNumber(11)
  $1.Timestamp get updatedAt => $_getN(10);
  @$pb.TagNumber(11)
  set updatedAt($1.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasUpdatedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearUpdatedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Timestamp ensureUpdatedAt() => $_ensure(10);

  @$pb.TagNumber(12)
  PrivacyPreset get presetEnum => $_getN(11);
  @$pb.TagNumber(12)
  set presetEnum(PrivacyPreset value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasPresetEnum() => $_has(11);
  @$pb.TagNumber(12)
  void clearPresetEnum() => $_clearField(12);

  @$pb.TagNumber(14)
  PrivacyAudience get allowPhoneSearch => $_getN(12);
  @$pb.TagNumber(14)
  set allowPhoneSearch(PrivacyAudience value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasAllowPhoneSearch() => $_has(12);
  @$pb.TagNumber(14)
  void clearAllowPhoneSearch() => $_clearField(14);
  @$pb.TagNumber(14)
  PrivacyAudience ensureAllowPhoneSearch() => $_ensure(12);

  @$pb.TagNumber(15)
  PrivacyAudience get allowCalls => $_getN(13);
  @$pb.TagNumber(15)
  set allowCalls(PrivacyAudience value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasAllowCalls() => $_has(13);
  @$pb.TagNumber(15)
  void clearAllowCalls() => $_clearField(15);
  @$pb.TagNumber(15)
  PrivacyAudience ensureAllowCalls() => $_ensure(13);

  @$pb.TagNumber(16)
  PrivacyAudience get allowChatSpaceInvites => $_getN(14);
  @$pb.TagNumber(16)
  set allowChatSpaceInvites(PrivacyAudience value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasAllowChatSpaceInvites() => $_has(14);
  @$pb.TagNumber(16)
  void clearAllowChatSpaceInvites() => $_clearField(16);
  @$pb.TagNumber(16)
  PrivacyAudience ensureAllowChatSpaceInvites() => $_ensure(14);

  @$pb.TagNumber(17)
  PrivacyAudience get allowFiles => $_getN(15);
  @$pb.TagNumber(17)
  set allowFiles(PrivacyAudience value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasAllowFiles() => $_has(15);
  @$pb.TagNumber(17)
  void clearAllowFiles() => $_clearField(17);
  @$pb.TagNumber(17)
  PrivacyAudience ensureAllowFiles() => $_ensure(15);

  @$pb.TagNumber(18)
  PrivacyAudience get allowVoiceMessages => $_getN(16);
  @$pb.TagNumber(18)
  set allowVoiceMessages(PrivacyAudience value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasAllowVoiceMessages() => $_has(16);
  @$pb.TagNumber(18)
  void clearAllowVoiceMessages() => $_clearField(18);
  @$pb.TagNumber(18)
  PrivacyAudience ensureAllowVoiceMessages() => $_ensure(16);
}

class UpdatePresenceRequest extends $pb.GeneratedMessage {
  factory UpdatePresenceRequest({
    $core.String? status,
    $core.String? gameTitle,
    $core.String? customStatus,
    $core.String? callInfoJson,
    PresenceOnlineStatus? statusEnum,
  }) {
    final result = create();
    if (status != null) result.status = status;
    if (gameTitle != null) result.gameTitle = gameTitle;
    if (customStatus != null) result.customStatus = customStatus;
    if (callInfoJson != null) result.callInfoJson = callInfoJson;
    if (statusEnum != null) result.statusEnum = statusEnum;
    return result;
  }

  UpdatePresenceRequest._();

  factory UpdatePresenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdatePresenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePresenceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'status')
    ..aOS(2, _omitFieldNames ? '' : 'gameTitle')
    ..aOS(3, _omitFieldNames ? '' : 'customStatus')
    ..aOS(4, _omitFieldNames ? '' : 'callInfoJson')
    ..aE<PresenceOnlineStatus>(5, _omitFieldNames ? '' : 'statusEnum',
        enumValues: PresenceOnlineStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePresenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePresenceRequest copyWith(
          void Function(UpdatePresenceRequest) updates) =>
      super.copyWith((message) => updates(message as UpdatePresenceRequest))
          as UpdatePresenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdatePresenceRequest create() => UpdatePresenceRequest._();
  @$core.override
  UpdatePresenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdatePresenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePresenceRequest>(create);
  static UpdatePresenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get status => $_getSZ(0);
  @$pb.TagNumber(1)
  set status($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatus() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get gameTitle => $_getSZ(1);
  @$pb.TagNumber(2)
  set gameTitle($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGameTitle() => $_has(1);
  @$pb.TagNumber(2)
  void clearGameTitle() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get customStatus => $_getSZ(2);
  @$pb.TagNumber(3)
  set customStatus($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCustomStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearCustomStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get callInfoJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set callInfoJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCallInfoJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearCallInfoJson() => $_clearField(4);

  @$pb.TagNumber(5)
  PresenceOnlineStatus get statusEnum => $_getN(4);
  @$pb.TagNumber(5)
  set statusEnum(PresenceOnlineStatus value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStatusEnum() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatusEnum() => $_clearField(5);
}

class GetPresenceRequest extends $pb.GeneratedMessage {
  factory GetPresenceRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetPresenceRequest._();

  factory GetPresenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPresenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPresenceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceRequest copyWith(void Function(GetPresenceRequest) updates) =>
      super.copyWith((message) => updates(message as GetPresenceRequest))
          as GetPresenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPresenceRequest create() => GetPresenceRequest._();
  @$core.override
  GetPresenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPresenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPresenceRequest>(create);
  static GetPresenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class PresenceStatus extends $pb.GeneratedMessage {
  factory PresenceStatus({
    $core.String? profileId,
    $core.String? status,
    $core.String? gameTitle,
    $core.String? customStatus,
    $1.Timestamp? lastSeen,
    $core.String? callInfoJson,
    PresenceOnlineStatus? statusEnum,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (status != null) result.status = status;
    if (gameTitle != null) result.gameTitle = gameTitle;
    if (customStatus != null) result.customStatus = customStatus;
    if (lastSeen != null) result.lastSeen = lastSeen;
    if (callInfoJson != null) result.callInfoJson = callInfoJson;
    if (statusEnum != null) result.statusEnum = statusEnum;
    return result;
  }

  PresenceStatus._();

  factory PresenceStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PresenceStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PresenceStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..aOS(3, _omitFieldNames ? '' : 'gameTitle')
    ..aOS(4, _omitFieldNames ? '' : 'customStatus')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'lastSeen',
        subBuilder: $1.Timestamp.create)
    ..aOS(6, _omitFieldNames ? '' : 'callInfoJson')
    ..aE<PresenceOnlineStatus>(7, _omitFieldNames ? '' : 'statusEnum',
        enumValues: PresenceOnlineStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PresenceStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PresenceStatus copyWith(void Function(PresenceStatus) updates) =>
      super.copyWith((message) => updates(message as PresenceStatus))
          as PresenceStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PresenceStatus create() => PresenceStatus._();
  @$core.override
  PresenceStatus createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PresenceStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PresenceStatus>(create);
  static PresenceStatus? _defaultInstance;

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

  @$pb.TagNumber(3)
  $core.String get gameTitle => $_getSZ(2);
  @$pb.TagNumber(3)
  set gameTitle($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasGameTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearGameTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get customStatus => $_getSZ(3);
  @$pb.TagNumber(4)
  set customStatus($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCustomStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearCustomStatus() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get lastSeen => $_getN(4);
  @$pb.TagNumber(5)
  set lastSeen($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLastSeen() => $_has(4);
  @$pb.TagNumber(5)
  void clearLastSeen() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureLastSeen() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get callInfoJson => $_getSZ(5);
  @$pb.TagNumber(6)
  set callInfoJson($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCallInfoJson() => $_has(5);
  @$pb.TagNumber(6)
  void clearCallInfoJson() => $_clearField(6);

  @$pb.TagNumber(7)
  PresenceOnlineStatus get statusEnum => $_getN(6);
  @$pb.TagNumber(7)
  set statusEnum(PresenceOnlineStatus value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasStatusEnum() => $_has(6);
  @$pb.TagNumber(7)
  void clearStatusEnum() => $_clearField(7);
}

class GetBulkPresenceRequest extends $pb.GeneratedMessage {
  factory GetBulkPresenceRequest({
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  GetBulkPresenceRequest._();

  factory GetBulkPresenceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkPresenceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkPresenceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkPresenceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkPresenceRequest copyWith(
          void Function(GetBulkPresenceRequest) updates) =>
      super.copyWith((message) => updates(message as GetBulkPresenceRequest))
          as GetBulkPresenceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkPresenceRequest create() => GetBulkPresenceRequest._();
  @$core.override
  GetBulkPresenceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkPresenceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkPresenceRequest>(create);
  static GetBulkPresenceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get profileIds => $_getList(0);
}

class GetSettingsRequest extends $pb.GeneratedMessage {
  factory GetSettingsRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetSettingsRequest._();

  factory GetSettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSettingsRequest copyWith(void Function(GetSettingsRequest) updates) =>
      super.copyWith((message) => updates(message as GetSettingsRequest))
          as GetSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSettingsRequest create() => GetSettingsRequest._();
  @$core.override
  GetSettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSettingsRequest>(create);
  static GetSettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class UpdateSettingsRequest extends $pb.GeneratedMessage {
  factory UpdateSettingsRequest({
    $core.String? profileId,
    UserSettings? settings,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (settings != null) result.settings = settings;
    return result;
  }

  UpdateSettingsRequest._();

  factory UpdateSettingsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateSettingsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateSettingsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<UserSettings>(2, _omitFieldNames ? '' : 'settings',
        subBuilder: UserSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSettingsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSettingsRequest copyWith(
          void Function(UpdateSettingsRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateSettingsRequest))
          as UpdateSettingsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateSettingsRequest create() => UpdateSettingsRequest._();
  @$core.override
  UpdateSettingsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateSettingsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateSettingsRequest>(create);
  static UpdateSettingsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  UserSettings get settings => $_getN(1);
  @$pb.TagNumber(2)
  set settings(UserSettings value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSettings() => $_has(1);
  @$pb.TagNumber(2)
  void clearSettings() => $_clearField(2);
  @$pb.TagNumber(2)
  UserSettings ensureSettings() => $_ensure(1);
}

class UserSettings extends $pb.GeneratedMessage {
  factory UserSettings({
    $core.String? profileId,
    $core.String? language,
    $core.String? theme,
    $core.String? notificationPrefsJson,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (language != null) result.language = language;
    if (theme != null) result.theme = theme;
    if (notificationPrefsJson != null)
      result.notificationPrefsJson = notificationPrefsJson;
    return result;
  }

  UserSettings._();

  factory UserSettings.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserSettings.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserSettings',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'language')
    ..aOS(3, _omitFieldNames ? '' : 'theme')
    ..aOS(4, _omitFieldNames ? '' : 'notificationPrefsJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSettings clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSettings copyWith(void Function(UserSettings) updates) =>
      super.copyWith((message) => updates(message as UserSettings))
          as UserSettings;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserSettings create() => UserSettings._();
  @$core.override
  UserSettings createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserSettings getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserSettings>(create);
  static UserSettings? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get language => $_getSZ(1);
  @$pb.TagNumber(2)
  set language($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLanguage() => $_has(1);
  @$pb.TagNumber(2)
  void clearLanguage() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get theme => $_getSZ(2);
  @$pb.TagNumber(3)
  set theme($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTheme() => $_has(2);
  @$pb.TagNumber(3)
  void clearTheme() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get notificationPrefsJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set notificationPrefsJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNotificationPrefsJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearNotificationPrefsJson() => $_clearField(4);
}

class GetOnboardingStateRequest extends $pb.GeneratedMessage {
  factory GetOnboardingStateRequest() => create();

  GetOnboardingStateRequest._();

  factory GetOnboardingStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetOnboardingStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetOnboardingStateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOnboardingStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOnboardingStateRequest copyWith(
          void Function(GetOnboardingStateRequest) updates) =>
      super.copyWith((message) => updates(message as GetOnboardingStateRequest))
          as GetOnboardingStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetOnboardingStateRequest create() => GetOnboardingStateRequest._();
  @$core.override
  GetOnboardingStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetOnboardingStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetOnboardingStateRequest>(create);
  static GetOnboardingStateRequest? _defaultInstance;
}

class CompleteOnboardingStepRequest extends $pb.GeneratedMessage {
  factory CompleteOnboardingStepRequest({
    $core.String? stepId,
  }) {
    final result = create();
    if (stepId != null) result.stepId = stepId;
    return result;
  }

  CompleteOnboardingStepRequest._();

  factory CompleteOnboardingStepRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteOnboardingStepRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteOnboardingStepRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'stepId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteOnboardingStepRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteOnboardingStepRequest copyWith(
          void Function(CompleteOnboardingStepRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CompleteOnboardingStepRequest))
          as CompleteOnboardingStepRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteOnboardingStepRequest create() =>
      CompleteOnboardingStepRequest._();
  @$core.override
  CompleteOnboardingStepRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteOnboardingStepRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteOnboardingStepRequest>(create);
  static CompleteOnboardingStepRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get stepId => $_getSZ(0);
  @$pb.TagNumber(1)
  set stepId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStepId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStepId() => $_clearField(1);
}

class OnboardingState extends $pb.GeneratedMessage {
  factory OnboardingState({
    $core.String? profileId,
    $core.Iterable<$core.String>? completedSteps,
    $core.bool? completed,
    $1.Timestamp? completedAt,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (completedSteps != null) result.completedSteps.addAll(completedSteps);
    if (completed != null) result.completed = completed;
    if (completedAt != null) result.completedAt = completedAt;
    return result;
  }

  OnboardingState._();

  factory OnboardingState.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OnboardingState.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OnboardingState',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..pPS(2, _omitFieldNames ? '' : 'completedSteps')
    ..aOB(3, _omitFieldNames ? '' : 'completed')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'completedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OnboardingState clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OnboardingState copyWith(void Function(OnboardingState) updates) =>
      super.copyWith((message) => updates(message as OnboardingState))
          as OnboardingState;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OnboardingState create() => OnboardingState._();
  @$core.override
  OnboardingState createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OnboardingState getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OnboardingState>(create);
  static OnboardingState? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get completedSteps => $_getList(1);

  @$pb.TagNumber(3)
  $core.bool get completed => $_getBF(2);
  @$pb.TagNumber(3)
  set completed($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCompleted() => $_has(2);
  @$pb.TagNumber(3)
  void clearCompleted() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get completedAt => $_getN(3);
  @$pb.TagNumber(4)
  set completedAt($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCompletedAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearCompletedAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureCompletedAt() => $_ensure(3);
}

class CreateAvatarPresignedUploadRequest extends $pb.GeneratedMessage {
  factory CreateAvatarPresignedUploadRequest({
    $core.String? profileId,
    $core.String? contentType,
    $fixnum.Int64? contentLength,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (contentType != null) result.contentType = contentType;
    if (contentLength != null) result.contentLength = contentLength;
    return result;
  }

  CreateAvatarPresignedUploadRequest._();

  factory CreateAvatarPresignedUploadRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateAvatarPresignedUploadRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateAvatarPresignedUploadRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'contentType')
    ..aInt64(3, _omitFieldNames ? '' : 'contentLength')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAvatarPresignedUploadRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAvatarPresignedUploadRequest copyWith(
          void Function(CreateAvatarPresignedUploadRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CreateAvatarPresignedUploadRequest))
          as CreateAvatarPresignedUploadRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateAvatarPresignedUploadRequest create() =>
      CreateAvatarPresignedUploadRequest._();
  @$core.override
  CreateAvatarPresignedUploadRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateAvatarPresignedUploadRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateAvatarPresignedUploadRequest>(
          create);
  static CreateAvatarPresignedUploadRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get contentType => $_getSZ(1);
  @$pb.TagNumber(2)
  set contentType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContentType() => $_has(1);
  @$pb.TagNumber(2)
  void clearContentType() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get contentLength => $_getI64(2);
  @$pb.TagNumber(3)
  set contentLength($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasContentLength() => $_has(2);
  @$pb.TagNumber(3)
  void clearContentLength() => $_clearField(3);
}

class CreateAvatarPresignedUploadResponse extends $pb.GeneratedMessage {
  factory CreateAvatarPresignedUploadResponse({
    $core.String? httpMethod,
    $core.String? uploadUrl,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? requiredHeaders,
    $fixnum.Int64? maxBytes,
    $1.Timestamp? expiresAt,
    $core.String? publicUrl,
    $core.String? objectKey,
  }) {
    final result = create();
    if (httpMethod != null) result.httpMethod = httpMethod;
    if (uploadUrl != null) result.uploadUrl = uploadUrl;
    if (requiredHeaders != null)
      result.requiredHeaders.addEntries(requiredHeaders);
    if (maxBytes != null) result.maxBytes = maxBytes;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (publicUrl != null) result.publicUrl = publicUrl;
    if (objectKey != null) result.objectKey = objectKey;
    return result;
  }

  CreateAvatarPresignedUploadResponse._();

  factory CreateAvatarPresignedUploadResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateAvatarPresignedUploadResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateAvatarPresignedUploadResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'httpMethod')
    ..aOS(2, _omitFieldNames ? '' : 'uploadUrl')
    ..m<$core.String, $core.String>(3, _omitFieldNames ? '' : 'requiredHeaders',
        entryClassName:
            'CreateAvatarPresignedUploadResponse.RequiredHeadersEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('voice.user.v1'))
    ..aInt64(4, _omitFieldNames ? '' : 'maxBytes')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(6, _omitFieldNames ? '' : 'publicUrl')
    ..aOS(7, _omitFieldNames ? '' : 'objectKey')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAvatarPresignedUploadResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateAvatarPresignedUploadResponse copyWith(
          void Function(CreateAvatarPresignedUploadResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CreateAvatarPresignedUploadResponse))
          as CreateAvatarPresignedUploadResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateAvatarPresignedUploadResponse create() =>
      CreateAvatarPresignedUploadResponse._();
  @$core.override
  CreateAvatarPresignedUploadResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateAvatarPresignedUploadResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CreateAvatarPresignedUploadResponse>(create);
  static CreateAvatarPresignedUploadResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get httpMethod => $_getSZ(0);
  @$pb.TagNumber(1)
  set httpMethod($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHttpMethod() => $_has(0);
  @$pb.TagNumber(1)
  void clearHttpMethod() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get uploadUrl => $_getSZ(1);
  @$pb.TagNumber(2)
  set uploadUrl($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasUploadUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearUploadUrl() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbMap<$core.String, $core.String> get requiredHeaders => $_getMap(2);

  @$pb.TagNumber(4)
  $fixnum.Int64 get maxBytes => $_getI64(3);
  @$pb.TagNumber(4)
  set maxBytes($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMaxBytes() => $_has(3);
  @$pb.TagNumber(4)
  void clearMaxBytes() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get expiresAt => $_getN(4);
  @$pb.TagNumber(5)
  set expiresAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasExpiresAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpiresAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureExpiresAt() => $_ensure(4);

  /// Value to persist via UpdateProfile.avatar_url after a successful PUT to upload_url.
  @$pb.TagNumber(6)
  $core.String get publicUrl => $_getSZ(5);
  @$pb.TagNumber(6)
  set publicUrl($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasPublicUrl() => $_has(5);
  @$pb.TagNumber(6)
  void clearPublicUrl() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get objectKey => $_getSZ(6);
  @$pb.TagNumber(7)
  set objectKey($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasObjectKey() => $_has(6);
  @$pb.TagNumber(7)
  void clearObjectKey() => $_clearField(7);
}

class GetVerificationStatusRequest extends $pb.GeneratedMessage {
  factory GetVerificationStatusRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetVerificationStatusRequest._();

  factory GetVerificationStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVerificationStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVerificationStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVerificationStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVerificationStatusRequest copyWith(
          void Function(GetVerificationStatusRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetVerificationStatusRequest))
          as GetVerificationStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVerificationStatusRequest create() =>
      GetVerificationStatusRequest._();
  @$core.override
  GetVerificationStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVerificationStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVerificationStatusRequest>(create);
  static GetVerificationStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class VerificationStatus extends $pb.GeneratedMessage {
  factory VerificationStatus({
    $core.String? profileId,
    $core.String? verificationType,
    $core.String? badge,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (verificationType != null) result.verificationType = verificationType;
    if (badge != null) result.badge = badge;
    return result;
  }

  VerificationStatus._();

  factory VerificationStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VerificationStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerificationStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'verificationType')
    ..aOS(3, _omitFieldNames ? '' : 'badge')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerificationStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerificationStatus copyWith(void Function(VerificationStatus) updates) =>
      super.copyWith((message) => updates(message as VerificationStatus))
          as VerificationStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VerificationStatus create() => VerificationStatus._();
  @$core.override
  VerificationStatus createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VerificationStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerificationStatus>(create);
  static VerificationStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get verificationType => $_getSZ(1);
  @$pb.TagNumber(2)
  set verificationType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVerificationType() => $_has(1);
  @$pb.TagNumber(2)
  void clearVerificationType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get badge => $_getSZ(2);
  @$pb.TagNumber(3)
  set badge($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBadge() => $_has(2);
  @$pb.TagNumber(3)
  void clearBadge() => $_clearField(3);
}

class GetProfileResponse extends $pb.GeneratedMessage {
  factory GetProfileResponse({
    Profile? profile,
  }) {
    final result = create();
    if (profile != null) result.profile = profile;
    return result;
  }

  GetProfileResponse._();

  factory GetProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileResponse copyWith(void Function(GetProfileResponse) updates) =>
      super.copyWith((message) => updates(message as GetProfileResponse))
          as GetProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfileResponse create() => GetProfileResponse._();
  @$core.override
  GetProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfileResponse>(create);
  static GetProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class GetProfilesResponse extends $pb.GeneratedMessage {
  factory GetProfilesResponse({
    ProfileList? profileList,
  }) {
    final result = create();
    if (profileList != null) result.profileList = profileList;
    return result;
  }

  GetProfilesResponse._();

  factory GetProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfilesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<ProfileList>(1, _omitFieldNames ? '' : 'profileList',
        subBuilder: ProfileList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfilesResponse copyWith(void Function(GetProfilesResponse) updates) =>
      super.copyWith((message) => updates(message as GetProfilesResponse))
          as GetProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfilesResponse create() => GetProfilesResponse._();
  @$core.override
  GetProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfilesResponse>(create);
  static GetProfilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ProfileList get profileList => $_getN(0);
  @$pb.TagNumber(1)
  set profileList(ProfileList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileList() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileList() => $_clearField(1);
  @$pb.TagNumber(1)
  ProfileList ensureProfileList() => $_ensure(0);
}

class UpdateProfileResponse extends $pb.GeneratedMessage {
  factory UpdateProfileResponse({
    Profile? profile,
  }) {
    final result = create();
    if (profile != null) result.profile = profile;
    return result;
  }

  UpdateProfileResponse._();

  factory UpdateProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateProfileResponse copyWith(
          void Function(UpdateProfileResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateProfileResponse))
          as UpdateProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateProfileResponse create() => UpdateProfileResponse._();
  @$core.override
  UpdateProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateProfileResponse>(create);
  static UpdateProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class CreateProfileResponse extends $pb.GeneratedMessage {
  factory CreateProfileResponse({
    Profile? profile,
  }) {
    final result = create();
    if (profile != null) result.profile = profile;
    return result;
  }

  CreateProfileResponse._();

  factory CreateProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateProfileResponse copyWith(
          void Function(CreateProfileResponse) updates) =>
      super.copyWith((message) => updates(message as CreateProfileResponse))
          as CreateProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateProfileResponse create() => CreateProfileResponse._();
  @$core.override
  CreateProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateProfileResponse>(create);
  static CreateProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class DeleteProfileResponse extends $pb.GeneratedMessage {
  factory DeleteProfileResponse() => create();

  DeleteProfileResponse._();

  factory DeleteProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteProfileResponse copyWith(
          void Function(DeleteProfileResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteProfileResponse))
          as DeleteProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteProfileResponse create() => DeleteProfileResponse._();
  @$core.override
  DeleteProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteProfileResponse>(create);
  static DeleteProfileResponse? _defaultInstance;
}

class SwitchProfileResponse extends $pb.GeneratedMessage {
  factory SwitchProfileResponse({
    Profile? profile,
  }) {
    final result = create();
    if (profile != null) result.profile = profile;
    return result;
  }

  SwitchProfileResponse._();

  factory SwitchProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SwitchProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SwitchProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<Profile>(1, _omitFieldNames ? '' : 'profile',
        subBuilder: Profile.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchProfileResponse copyWith(
          void Function(SwitchProfileResponse) updates) =>
      super.copyWith((message) => updates(message as SwitchProfileResponse))
          as SwitchProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SwitchProfileResponse create() => SwitchProfileResponse._();
  @$core.override
  SwitchProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SwitchProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SwitchProfileResponse>(create);
  static SwitchProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Profile get profile => $_getN(0);
  @$pb.TagNumber(1)
  set profile(Profile value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfile() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfile() => $_clearField(1);
  @$pb.TagNumber(1)
  Profile ensureProfile() => $_ensure(0);
}

class ListMyProfilesResponse extends $pb.GeneratedMessage {
  factory ListMyProfilesResponse({
    ProfileList? profileList,
  }) {
    final result = create();
    if (profileList != null) result.profileList = profileList;
    return result;
  }

  ListMyProfilesResponse._();

  factory ListMyProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMyProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMyProfilesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<ProfileList>(1, _omitFieldNames ? '' : 'profileList',
        subBuilder: ProfileList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMyProfilesResponse copyWith(
          void Function(ListMyProfilesResponse) updates) =>
      super.copyWith((message) => updates(message as ListMyProfilesResponse))
          as ListMyProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMyProfilesResponse create() => ListMyProfilesResponse._();
  @$core.override
  ListMyProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMyProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMyProfilesResponse>(create);
  static ListMyProfilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ProfileList get profileList => $_getN(0);
  @$pb.TagNumber(1)
  set profileList(ProfileList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileList() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileList() => $_clearField(1);
  @$pb.TagNumber(1)
  ProfileList ensureProfileList() => $_ensure(0);
}

class GetPrivacySettingsResponse extends $pb.GeneratedMessage {
  factory GetPrivacySettingsResponse({
    PrivacySettings? privacySettings,
  }) {
    final result = create();
    if (privacySettings != null) result.privacySettings = privacySettings;
    return result;
  }

  GetPrivacySettingsResponse._();

  factory GetPrivacySettingsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPrivacySettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPrivacySettingsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<PrivacySettings>(1, _omitFieldNames ? '' : 'privacySettings',
        subBuilder: PrivacySettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPrivacySettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPrivacySettingsResponse copyWith(
          void Function(GetPrivacySettingsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetPrivacySettingsResponse))
          as GetPrivacySettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPrivacySettingsResponse create() => GetPrivacySettingsResponse._();
  @$core.override
  GetPrivacySettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPrivacySettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPrivacySettingsResponse>(create);
  static GetPrivacySettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PrivacySettings get privacySettings => $_getN(0);
  @$pb.TagNumber(1)
  set privacySettings(PrivacySettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPrivacySettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearPrivacySettings() => $_clearField(1);
  @$pb.TagNumber(1)
  PrivacySettings ensurePrivacySettings() => $_ensure(0);
}

class UpdatePrivacySettingsResponse extends $pb.GeneratedMessage {
  factory UpdatePrivacySettingsResponse({
    PrivacySettings? privacySettings,
  }) {
    final result = create();
    if (privacySettings != null) result.privacySettings = privacySettings;
    return result;
  }

  UpdatePrivacySettingsResponse._();

  factory UpdatePrivacySettingsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdatePrivacySettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePrivacySettingsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<PrivacySettings>(1, _omitFieldNames ? '' : 'privacySettings',
        subBuilder: PrivacySettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePrivacySettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePrivacySettingsResponse copyWith(
          void Function(UpdatePrivacySettingsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UpdatePrivacySettingsResponse))
          as UpdatePrivacySettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdatePrivacySettingsResponse create() =>
      UpdatePrivacySettingsResponse._();
  @$core.override
  UpdatePrivacySettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdatePrivacySettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePrivacySettingsResponse>(create);
  static UpdatePrivacySettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PrivacySettings get privacySettings => $_getN(0);
  @$pb.TagNumber(1)
  set privacySettings(PrivacySettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPrivacySettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearPrivacySettings() => $_clearField(1);
  @$pb.TagNumber(1)
  PrivacySettings ensurePrivacySettings() => $_ensure(0);
}

class UpdatePresenceResponse extends $pb.GeneratedMessage {
  factory UpdatePresenceResponse() => create();

  UpdatePresenceResponse._();

  factory UpdatePresenceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdatePresenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdatePresenceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePresenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdatePresenceResponse copyWith(
          void Function(UpdatePresenceResponse) updates) =>
      super.copyWith((message) => updates(message as UpdatePresenceResponse))
          as UpdatePresenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdatePresenceResponse create() => UpdatePresenceResponse._();
  @$core.override
  UpdatePresenceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdatePresenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdatePresenceResponse>(create);
  static UpdatePresenceResponse? _defaultInstance;
}

class GetPresenceResponse extends $pb.GeneratedMessage {
  factory GetPresenceResponse({
    PresenceStatus? presenceStatus,
  }) {
    final result = create();
    if (presenceStatus != null) result.presenceStatus = presenceStatus;
    return result;
  }

  GetPresenceResponse._();

  factory GetPresenceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPresenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPresenceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<PresenceStatus>(1, _omitFieldNames ? '' : 'presenceStatus',
        subBuilder: PresenceStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPresenceResponse copyWith(void Function(GetPresenceResponse) updates) =>
      super.copyWith((message) => updates(message as GetPresenceResponse))
          as GetPresenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPresenceResponse create() => GetPresenceResponse._();
  @$core.override
  GetPresenceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPresenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPresenceResponse>(create);
  static GetPresenceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PresenceStatus get presenceStatus => $_getN(0);
  @$pb.TagNumber(1)
  set presenceStatus(PresenceStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPresenceStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearPresenceStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  PresenceStatus ensurePresenceStatus() => $_ensure(0);
}

class GetBulkPresenceResponse extends $pb.GeneratedMessage {
  factory GetBulkPresenceResponse({
    $core.Iterable<$core.MapEntry<$core.String, PresenceStatus>>? byProfileId,
  }) {
    final result = create();
    if (byProfileId != null) result.byProfileId.addEntries(byProfileId);
    return result;
  }

  GetBulkPresenceResponse._();

  factory GetBulkPresenceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBulkPresenceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBulkPresenceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..m<$core.String, PresenceStatus>(1, _omitFieldNames ? '' : 'byProfileId',
        entryClassName: 'GetBulkPresenceResponse.ByProfileIdEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OM,
        valueCreator: PresenceStatus.create,
        valueDefaultOrMaker: PresenceStatus.getDefault,
        packageName: const $pb.PackageName('voice.user.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkPresenceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBulkPresenceResponse copyWith(
          void Function(GetBulkPresenceResponse) updates) =>
      super.copyWith((message) => updates(message as GetBulkPresenceResponse))
          as GetBulkPresenceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBulkPresenceResponse create() => GetBulkPresenceResponse._();
  @$core.override
  GetBulkPresenceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBulkPresenceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBulkPresenceResponse>(create);
  static GetBulkPresenceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbMap<$core.String, PresenceStatus> get byProfileId => $_getMap(0);
}

class GetSettingsResponse extends $pb.GeneratedMessage {
  factory GetSettingsResponse({
    UserSettings? userSettings,
  }) {
    final result = create();
    if (userSettings != null) result.userSettings = userSettings;
    return result;
  }

  GetSettingsResponse._();

  factory GetSettingsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSettingsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<UserSettings>(1, _omitFieldNames ? '' : 'userSettings',
        subBuilder: UserSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSettingsResponse copyWith(void Function(GetSettingsResponse) updates) =>
      super.copyWith((message) => updates(message as GetSettingsResponse))
          as GetSettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSettingsResponse create() => GetSettingsResponse._();
  @$core.override
  GetSettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSettingsResponse>(create);
  static GetSettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UserSettings get userSettings => $_getN(0);
  @$pb.TagNumber(1)
  set userSettings(UserSettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasUserSettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearUserSettings() => $_clearField(1);
  @$pb.TagNumber(1)
  UserSettings ensureUserSettings() => $_ensure(0);
}

class UpdateSettingsResponse extends $pb.GeneratedMessage {
  factory UpdateSettingsResponse({
    UserSettings? userSettings,
  }) {
    final result = create();
    if (userSettings != null) result.userSettings = userSettings;
    return result;
  }

  UpdateSettingsResponse._();

  factory UpdateSettingsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateSettingsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateSettingsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<UserSettings>(1, _omitFieldNames ? '' : 'userSettings',
        subBuilder: UserSettings.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSettingsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSettingsResponse copyWith(
          void Function(UpdateSettingsResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateSettingsResponse))
          as UpdateSettingsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateSettingsResponse create() => UpdateSettingsResponse._();
  @$core.override
  UpdateSettingsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateSettingsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateSettingsResponse>(create);
  static UpdateSettingsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UserSettings get userSettings => $_getN(0);
  @$pb.TagNumber(1)
  set userSettings(UserSettings value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasUserSettings() => $_has(0);
  @$pb.TagNumber(1)
  void clearUserSettings() => $_clearField(1);
  @$pb.TagNumber(1)
  UserSettings ensureUserSettings() => $_ensure(0);
}

class GetOnboardingStateResponse extends $pb.GeneratedMessage {
  factory GetOnboardingStateResponse({
    OnboardingState? onboardingState,
  }) {
    final result = create();
    if (onboardingState != null) result.onboardingState = onboardingState;
    return result;
  }

  GetOnboardingStateResponse._();

  factory GetOnboardingStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetOnboardingStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetOnboardingStateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<OnboardingState>(1, _omitFieldNames ? '' : 'onboardingState',
        subBuilder: OnboardingState.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOnboardingStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOnboardingStateResponse copyWith(
          void Function(GetOnboardingStateResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetOnboardingStateResponse))
          as GetOnboardingStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetOnboardingStateResponse create() => GetOnboardingStateResponse._();
  @$core.override
  GetOnboardingStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetOnboardingStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetOnboardingStateResponse>(create);
  static GetOnboardingStateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  OnboardingState get onboardingState => $_getN(0);
  @$pb.TagNumber(1)
  set onboardingState(OnboardingState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOnboardingState() => $_has(0);
  @$pb.TagNumber(1)
  void clearOnboardingState() => $_clearField(1);
  @$pb.TagNumber(1)
  OnboardingState ensureOnboardingState() => $_ensure(0);
}

class CompleteOnboardingStepResponse extends $pb.GeneratedMessage {
  factory CompleteOnboardingStepResponse({
    OnboardingState? onboardingState,
  }) {
    final result = create();
    if (onboardingState != null) result.onboardingState = onboardingState;
    return result;
  }

  CompleteOnboardingStepResponse._();

  factory CompleteOnboardingStepResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteOnboardingStepResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteOnboardingStepResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<OnboardingState>(1, _omitFieldNames ? '' : 'onboardingState',
        subBuilder: OnboardingState.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteOnboardingStepResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteOnboardingStepResponse copyWith(
          void Function(CompleteOnboardingStepResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CompleteOnboardingStepResponse))
          as CompleteOnboardingStepResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteOnboardingStepResponse create() =>
      CompleteOnboardingStepResponse._();
  @$core.override
  CompleteOnboardingStepResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteOnboardingStepResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteOnboardingStepResponse>(create);
  static CompleteOnboardingStepResponse? _defaultInstance;

  @$pb.TagNumber(1)
  OnboardingState get onboardingState => $_getN(0);
  @$pb.TagNumber(1)
  set onboardingState(OnboardingState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOnboardingState() => $_has(0);
  @$pb.TagNumber(1)
  void clearOnboardingState() => $_clearField(1);
  @$pb.TagNumber(1)
  OnboardingState ensureOnboardingState() => $_ensure(0);
}

class GetVerificationStatusResponse extends $pb.GeneratedMessage {
  factory GetVerificationStatusResponse({
    VerificationStatus? verificationStatus,
  }) {
    final result = create();
    if (verificationStatus != null)
      result.verificationStatus = verificationStatus;
    return result;
  }

  GetVerificationStatusResponse._();

  factory GetVerificationStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVerificationStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVerificationStatusResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<VerificationStatus>(1, _omitFieldNames ? '' : 'verificationStatus',
        subBuilder: VerificationStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVerificationStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVerificationStatusResponse copyWith(
          void Function(GetVerificationStatusResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetVerificationStatusResponse))
          as GetVerificationStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVerificationStatusResponse create() =>
      GetVerificationStatusResponse._();
  @$core.override
  GetVerificationStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVerificationStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVerificationStatusResponse>(create);
  static GetVerificationStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VerificationStatus get verificationStatus => $_getN(0);
  @$pb.TagNumber(1)
  set verificationStatus(VerificationStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVerificationStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearVerificationStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  VerificationStatus ensureVerificationStatus() => $_ensure(0);
}

class SetVerificationRequest extends $pb.GeneratedMessage {
  factory SetVerificationRequest({
    $core.String? profileId,
    $core.String? verificationType,
    $core.String? badge,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (verificationType != null) result.verificationType = verificationType;
    if (badge != null) result.badge = badge;
    return result;
  }

  SetVerificationRequest._();

  factory SetVerificationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetVerificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetVerificationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'verificationType')
    ..aOS(3, _omitFieldNames ? '' : 'badge')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVerificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVerificationRequest copyWith(
          void Function(SetVerificationRequest) updates) =>
      super.copyWith((message) => updates(message as SetVerificationRequest))
          as SetVerificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetVerificationRequest create() => SetVerificationRequest._();
  @$core.override
  SetVerificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetVerificationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetVerificationRequest>(create);
  static SetVerificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get verificationType => $_getSZ(1);
  @$pb.TagNumber(2)
  set verificationType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVerificationType() => $_has(1);
  @$pb.TagNumber(2)
  void clearVerificationType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get badge => $_getSZ(2);
  @$pb.TagNumber(3)
  set badge($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBadge() => $_has(2);
  @$pb.TagNumber(3)
  void clearBadge() => $_clearField(3);
}

class SetVerificationResponse extends $pb.GeneratedMessage {
  factory SetVerificationResponse({
    VerificationStatus? verificationStatus,
  }) {
    final result = create();
    if (verificationStatus != null)
      result.verificationStatus = verificationStatus;
    return result;
  }

  SetVerificationResponse._();

  factory SetVerificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetVerificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetVerificationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<VerificationStatus>(1, _omitFieldNames ? '' : 'verificationStatus',
        subBuilder: VerificationStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVerificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVerificationResponse copyWith(
          void Function(SetVerificationResponse) updates) =>
      super.copyWith((message) => updates(message as SetVerificationResponse))
          as SetVerificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetVerificationResponse create() => SetVerificationResponse._();
  @$core.override
  SetVerificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetVerificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetVerificationResponse>(create);
  static SetVerificationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VerificationStatus get verificationStatus => $_getN(0);
  @$pb.TagNumber(1)
  set verificationStatus(VerificationStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVerificationStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearVerificationStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  VerificationStatus ensureVerificationStatus() => $_ensure(0);
}

class ClearVerificationRequest extends $pb.GeneratedMessage {
  factory ClearVerificationRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  ClearVerificationRequest._();

  factory ClearVerificationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ClearVerificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearVerificationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVerificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVerificationRequest copyWith(
          void Function(ClearVerificationRequest) updates) =>
      super.copyWith((message) => updates(message as ClearVerificationRequest))
          as ClearVerificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ClearVerificationRequest create() => ClearVerificationRequest._();
  @$core.override
  ClearVerificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ClearVerificationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearVerificationRequest>(create);
  static ClearVerificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class ClearVerificationResponse extends $pb.GeneratedMessage {
  factory ClearVerificationResponse({
    VerificationStatus? verificationStatus,
  }) {
    final result = create();
    if (verificationStatus != null)
      result.verificationStatus = verificationStatus;
    return result;
  }

  ClearVerificationResponse._();

  factory ClearVerificationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ClearVerificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClearVerificationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<VerificationStatus>(1, _omitFieldNames ? '' : 'verificationStatus',
        subBuilder: VerificationStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVerificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClearVerificationResponse copyWith(
          void Function(ClearVerificationResponse) updates) =>
      super.copyWith((message) => updates(message as ClearVerificationResponse))
          as ClearVerificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ClearVerificationResponse create() => ClearVerificationResponse._();
  @$core.override
  ClearVerificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ClearVerificationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ClearVerificationResponse>(create);
  static ClearVerificationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VerificationStatus get verificationStatus => $_getN(0);
  @$pb.TagNumber(1)
  set verificationStatus(VerificationStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVerificationStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearVerificationStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  VerificationStatus ensureVerificationStatus() => $_ensure(0);
}

class StartOrganizationVerificationRequest extends $pb.GeneratedMessage {
  factory StartOrganizationVerificationRequest({
    $core.String? profileId,
    $core.String? domain,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (domain != null) result.domain = domain;
    return result;
  }

  StartOrganizationVerificationRequest._();

  factory StartOrganizationVerificationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartOrganizationVerificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartOrganizationVerificationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'domain')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartOrganizationVerificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartOrganizationVerificationRequest copyWith(
          void Function(StartOrganizationVerificationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as StartOrganizationVerificationRequest))
          as StartOrganizationVerificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartOrganizationVerificationRequest create() =>
      StartOrganizationVerificationRequest._();
  @$core.override
  StartOrganizationVerificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartOrganizationVerificationRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          StartOrganizationVerificationRequest>(create);
  static StartOrganizationVerificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get domain => $_getSZ(1);
  @$pb.TagNumber(2)
  set domain($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDomain() => $_has(1);
  @$pb.TagNumber(2)
  void clearDomain() => $_clearField(2);
}

class StartOrganizationVerificationResponse extends $pb.GeneratedMessage {
  factory StartOrganizationVerificationResponse({
    $core.String? domain,
    $core.String? txtRecord,
  }) {
    final result = create();
    if (domain != null) result.domain = domain;
    if (txtRecord != null) result.txtRecord = txtRecord;
    return result;
  }

  StartOrganizationVerificationResponse._();

  factory StartOrganizationVerificationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartOrganizationVerificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartOrganizationVerificationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'domain')
    ..aOS(2, _omitFieldNames ? '' : 'txtRecord')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartOrganizationVerificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartOrganizationVerificationResponse copyWith(
          void Function(StartOrganizationVerificationResponse) updates) =>
      super.copyWith((message) =>
              updates(message as StartOrganizationVerificationResponse))
          as StartOrganizationVerificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartOrganizationVerificationResponse create() =>
      StartOrganizationVerificationResponse._();
  @$core.override
  StartOrganizationVerificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartOrganizationVerificationResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          StartOrganizationVerificationResponse>(create);
  static StartOrganizationVerificationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get domain => $_getSZ(0);
  @$pb.TagNumber(1)
  set domain($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDomain() => $_has(0);
  @$pb.TagNumber(1)
  void clearDomain() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get txtRecord => $_getSZ(1);
  @$pb.TagNumber(2)
  set txtRecord($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTxtRecord() => $_has(1);
  @$pb.TagNumber(2)
  void clearTxtRecord() => $_clearField(2);
}

class CheckOrganizationVerificationRequest extends $pb.GeneratedMessage {
  factory CheckOrganizationVerificationRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  CheckOrganizationVerificationRequest._();

  factory CheckOrganizationVerificationRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckOrganizationVerificationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckOrganizationVerificationRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckOrganizationVerificationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckOrganizationVerificationRequest copyWith(
          void Function(CheckOrganizationVerificationRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CheckOrganizationVerificationRequest))
          as CheckOrganizationVerificationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckOrganizationVerificationRequest create() =>
      CheckOrganizationVerificationRequest._();
  @$core.override
  CheckOrganizationVerificationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckOrganizationVerificationRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CheckOrganizationVerificationRequest>(create);
  static CheckOrganizationVerificationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class CheckOrganizationVerificationResponse extends $pb.GeneratedMessage {
  factory CheckOrganizationVerificationResponse({
    VerificationStatus? verificationStatus,
  }) {
    final result = create();
    if (verificationStatus != null)
      result.verificationStatus = verificationStatus;
    return result;
  }

  CheckOrganizationVerificationResponse._();

  factory CheckOrganizationVerificationResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckOrganizationVerificationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckOrganizationVerificationResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOM<VerificationStatus>(1, _omitFieldNames ? '' : 'verificationStatus',
        subBuilder: VerificationStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckOrganizationVerificationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckOrganizationVerificationResponse copyWith(
          void Function(CheckOrganizationVerificationResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CheckOrganizationVerificationResponse))
          as CheckOrganizationVerificationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckOrganizationVerificationResponse create() =>
      CheckOrganizationVerificationResponse._();
  @$core.override
  CheckOrganizationVerificationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckOrganizationVerificationResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CheckOrganizationVerificationResponse>(create);
  static CheckOrganizationVerificationResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VerificationStatus get verificationStatus => $_getN(0);
  @$pb.TagNumber(1)
  set verificationStatus(VerificationStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVerificationStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearVerificationStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  VerificationStatus ensureVerificationStatus() => $_ensure(0);
}

class ApplyDowngradeProfilesRequest extends $pb.GeneratedMessage {
  factory ApplyDowngradeProfilesRequest({
    $core.String? accountId,
    $core.Iterable<$core.String>? keptProfileIds,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (keptProfileIds != null) result.keptProfileIds.addAll(keptProfileIds);
    return result;
  }

  ApplyDowngradeProfilesRequest._();

  factory ApplyDowngradeProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyDowngradeProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyDowngradeProfilesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..pPS(2, _omitFieldNames ? '' : 'keptProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesRequest copyWith(
          void Function(ApplyDowngradeProfilesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyDowngradeProfilesRequest))
          as ApplyDowngradeProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesRequest create() =>
      ApplyDowngradeProfilesRequest._();
  @$core.override
  ApplyDowngradeProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyDowngradeProfilesRequest>(create);
  static ApplyDowngradeProfilesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get keptProfileIds => $_getList(1);
}

class ApplyDowngradeProfilesResponse extends $pb.GeneratedMessage {
  factory ApplyDowngradeProfilesResponse({
    $core.Iterable<$core.String>? keptProfileIds,
  }) {
    final result = create();
    if (keptProfileIds != null) result.keptProfileIds.addAll(keptProfileIds);
    return result;
  }

  ApplyDowngradeProfilesResponse._();

  factory ApplyDowngradeProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyDowngradeProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyDowngradeProfilesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.user.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'keptProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesResponse copyWith(
          void Function(ApplyDowngradeProfilesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyDowngradeProfilesResponse))
          as ApplyDowngradeProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesResponse create() =>
      ApplyDowngradeProfilesResponse._();
  @$core.override
  ApplyDowngradeProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyDowngradeProfilesResponse>(create);
  static ApplyDowngradeProfilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get keptProfileIds => $_getList(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
