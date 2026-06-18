// This is a generated file - do not edit.
//
// Generated from voice/space/v1/space.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../chat/v1/chat.pb.dart' as $3;
import '../../common/v1/common.pb.dart' as $2;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class Space extends $pb.GeneratedMessage {
  factory Space({
    $core.String? id,
    $core.String? name,
    $core.String? description,
    $core.String? iconUrl,
    $core.String? bannerUrl,
    $core.String? visibility,
    $core.String? ownerProfileId,
    $core.int? memberCount,
    $core.bool? isVerified,
    $core.String? verificationType,
    $core.String? entryRequirement,
    $core.String? entryQuestionsJson,
    $core.String? mmConfigJson,
    $1.Timestamp? createdAt,
    $1.Timestamp? updatedAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (iconUrl != null) result.iconUrl = iconUrl;
    if (bannerUrl != null) result.bannerUrl = bannerUrl;
    if (visibility != null) result.visibility = visibility;
    if (ownerProfileId != null) result.ownerProfileId = ownerProfileId;
    if (memberCount != null) result.memberCount = memberCount;
    if (isVerified != null) result.isVerified = isVerified;
    if (verificationType != null) result.verificationType = verificationType;
    if (entryRequirement != null) result.entryRequirement = entryRequirement;
    if (entryQuestionsJson != null)
      result.entryQuestionsJson = entryQuestionsJson;
    if (mmConfigJson != null) result.mmConfigJson = mmConfigJson;
    if (createdAt != null) result.createdAt = createdAt;
    if (updatedAt != null) result.updatedAt = updatedAt;
    return result;
  }

  Space._();

  factory Space.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Space.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Space',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'iconUrl')
    ..aOS(5, _omitFieldNames ? '' : 'bannerUrl')
    ..aOS(6, _omitFieldNames ? '' : 'visibility')
    ..aOS(7, _omitFieldNames ? '' : 'ownerProfileId')
    ..aI(8, _omitFieldNames ? '' : 'memberCount')
    ..aOB(9, _omitFieldNames ? '' : 'isVerified')
    ..aOS(10, _omitFieldNames ? '' : 'verificationType')
    ..aOS(11, _omitFieldNames ? '' : 'entryRequirement')
    ..aOS(12, _omitFieldNames ? '' : 'entryQuestionsJson')
    ..aOS(13, _omitFieldNames ? '' : 'mmConfigJson')
    ..aOM<$1.Timestamp>(14, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(15, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Space clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Space copyWith(void Function(Space) updates) =>
      super.copyWith((message) => updates(message as Space)) as Space;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Space create() => Space._();
  @$core.override
  Space createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Space getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Space>(create);
  static Space? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get iconUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set iconUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIconUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearIconUrl() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get bannerUrl => $_getSZ(4);
  @$pb.TagNumber(5)
  set bannerUrl($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBannerUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearBannerUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get visibility => $_getSZ(5);
  @$pb.TagNumber(6)
  set visibility($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVisibility() => $_has(5);
  @$pb.TagNumber(6)
  void clearVisibility() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get ownerProfileId => $_getSZ(6);
  @$pb.TagNumber(7)
  set ownerProfileId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasOwnerProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearOwnerProfileId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.int get memberCount => $_getIZ(7);
  @$pb.TagNumber(8)
  set memberCount($core.int value) => $_setSignedInt32(7, value);
  @$pb.TagNumber(8)
  $core.bool hasMemberCount() => $_has(7);
  @$pb.TagNumber(8)
  void clearMemberCount() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get isVerified => $_getBF(8);
  @$pb.TagNumber(9)
  set isVerified($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasIsVerified() => $_has(8);
  @$pb.TagNumber(9)
  void clearIsVerified() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get verificationType => $_getSZ(9);
  @$pb.TagNumber(10)
  set verificationType($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasVerificationType() => $_has(9);
  @$pb.TagNumber(10)
  void clearVerificationType() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get entryRequirement => $_getSZ(10);
  @$pb.TagNumber(11)
  set entryRequirement($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasEntryRequirement() => $_has(10);
  @$pb.TagNumber(11)
  void clearEntryRequirement() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get entryQuestionsJson => $_getSZ(11);
  @$pb.TagNumber(12)
  set entryQuestionsJson($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasEntryQuestionsJson() => $_has(11);
  @$pb.TagNumber(12)
  void clearEntryQuestionsJson() => $_clearField(12);

  @$pb.TagNumber(13)
  $core.String get mmConfigJson => $_getSZ(12);
  @$pb.TagNumber(13)
  set mmConfigJson($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasMmConfigJson() => $_has(12);
  @$pb.TagNumber(13)
  void clearMmConfigJson() => $_clearField(13);

  @$pb.TagNumber(14)
  $1.Timestamp get createdAt => $_getN(13);
  @$pb.TagNumber(14)
  set createdAt($1.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasCreatedAt() => $_has(13);
  @$pb.TagNumber(14)
  void clearCreatedAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $1.Timestamp ensureCreatedAt() => $_ensure(13);

  @$pb.TagNumber(15)
  $1.Timestamp get updatedAt => $_getN(14);
  @$pb.TagNumber(15)
  set updatedAt($1.Timestamp value) => $_setField(15, value);
  @$pb.TagNumber(15)
  $core.bool hasUpdatedAt() => $_has(14);
  @$pb.TagNumber(15)
  void clearUpdatedAt() => $_clearField(15);
  @$pb.TagNumber(15)
  $1.Timestamp ensureUpdatedAt() => $_ensure(14);
}

class CreateSpaceRequest extends $pb.GeneratedMessage {
  factory CreateSpaceRequest({
    $core.String? name,
    $core.String? description,
    $core.String? visibility,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (visibility != null) result.visibility = visibility;
    return result;
  }

  CreateSpaceRequest._();

  factory CreateSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'description')
    ..aOS(3, _omitFieldNames ? '' : 'visibility')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceRequest copyWith(void Function(CreateSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as CreateSpaceRequest))
          as CreateSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateSpaceRequest create() => CreateSpaceRequest._();
  @$core.override
  CreateSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateSpaceRequest>(create);
  static CreateSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get description => $_getSZ(1);
  @$pb.TagNumber(2)
  set description($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDescription() => $_has(1);
  @$pb.TagNumber(2)
  void clearDescription() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get visibility => $_getSZ(2);
  @$pb.TagNumber(3)
  set visibility($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVisibility() => $_has(2);
  @$pb.TagNumber(3)
  void clearVisibility() => $_clearField(3);
}

class UpdateSpaceRequest extends $pb.GeneratedMessage {
  factory UpdateSpaceRequest({
    $core.String? spaceId,
    $core.String? name,
    $core.String? description,
    $core.String? iconUrl,
    $core.String? bannerUrl,
    $core.String? visibility,
    $core.String? entryRequirement,
    $core.String? entryQuestionsJson,
    $core.String? mmConfigJson,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    if (iconUrl != null) result.iconUrl = iconUrl;
    if (bannerUrl != null) result.bannerUrl = bannerUrl;
    if (visibility != null) result.visibility = visibility;
    if (entryRequirement != null) result.entryRequirement = entryRequirement;
    if (entryQuestionsJson != null)
      result.entryQuestionsJson = entryQuestionsJson;
    if (mmConfigJson != null) result.mmConfigJson = mmConfigJson;
    return result;
  }

  UpdateSpaceRequest._();

  factory UpdateSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..aOS(4, _omitFieldNames ? '' : 'iconUrl')
    ..aOS(5, _omitFieldNames ? '' : 'bannerUrl')
    ..aOS(6, _omitFieldNames ? '' : 'visibility')
    ..aOS(7, _omitFieldNames ? '' : 'entryRequirement')
    ..aOS(8, _omitFieldNames ? '' : 'entryQuestionsJson')
    ..aOS(9, _omitFieldNames ? '' : 'mmConfigJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSpaceRequest copyWith(void Function(UpdateSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateSpaceRequest))
          as UpdateSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateSpaceRequest create() => UpdateSpaceRequest._();
  @$core.override
  UpdateSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateSpaceRequest>(create);
  static UpdateSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get iconUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set iconUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasIconUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearIconUrl() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get bannerUrl => $_getSZ(4);
  @$pb.TagNumber(5)
  set bannerUrl($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBannerUrl() => $_has(4);
  @$pb.TagNumber(5)
  void clearBannerUrl() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get visibility => $_getSZ(5);
  @$pb.TagNumber(6)
  set visibility($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVisibility() => $_has(5);
  @$pb.TagNumber(6)
  void clearVisibility() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get entryRequirement => $_getSZ(6);
  @$pb.TagNumber(7)
  set entryRequirement($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasEntryRequirement() => $_has(6);
  @$pb.TagNumber(7)
  void clearEntryRequirement() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get entryQuestionsJson => $_getSZ(7);
  @$pb.TagNumber(8)
  set entryQuestionsJson($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasEntryQuestionsJson() => $_has(7);
  @$pb.TagNumber(8)
  void clearEntryQuestionsJson() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get mmConfigJson => $_getSZ(8);
  @$pb.TagNumber(9)
  set mmConfigJson($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasMmConfigJson() => $_has(8);
  @$pb.TagNumber(9)
  void clearMmConfigJson() => $_clearField(9);
}

class DeleteSpaceRequest extends $pb.GeneratedMessage {
  factory DeleteSpaceRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  DeleteSpaceRequest._();

  factory DeleteSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteSpaceRequest copyWith(void Function(DeleteSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteSpaceRequest))
          as DeleteSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteSpaceRequest create() => DeleteSpaceRequest._();
  @$core.override
  DeleteSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteSpaceRequest>(create);
  static DeleteSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class GetSpaceRequest extends $pb.GeneratedMessage {
  factory GetSpaceRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  GetSpaceRequest._();

  factory GetSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceRequest copyWith(void Function(GetSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as GetSpaceRequest))
          as GetSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceRequest create() => GetSpaceRequest._();
  @$core.override
  GetSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceRequest>(create);
  static GetSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class ListMySpacesRequest extends $pb.GeneratedMessage {
  factory ListMySpacesRequest({
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ListMySpacesRequest._();

  factory ListMySpacesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMySpacesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMySpacesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<$2.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySpacesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySpacesRequest copyWith(void Function(ListMySpacesRequest) updates) =>
      super.copyWith((message) => updates(message as ListMySpacesRequest))
          as ListMySpacesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMySpacesRequest create() => ListMySpacesRequest._();
  @$core.override
  ListMySpacesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMySpacesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMySpacesRequest>(create);
  static ListMySpacesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.CursorPageRequest get page => $_getN(0);
  @$pb.TagNumber(1)
  set page($2.CursorPageRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.CursorPageRequest ensurePage() => $_ensure(0);
}

class SpaceList extends $pb.GeneratedMessage {
  factory SpaceList({
    $core.Iterable<Space>? spaces,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (spaces != null) result.spaces.addAll(spaces);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  SpaceList._();

  factory SpaceList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<Space>(1, _omitFieldNames ? '' : 'spaces', subBuilder: Space.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceList copyWith(void Function(SpaceList) updates) =>
      super.copyWith((message) => updates(message as SpaceList)) as SpaceList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceList create() => SpaceList._();
  @$core.override
  SpaceList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SpaceList>(create);
  static SpaceList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Space> get spaces => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class SearchPublicSpacesRequest extends $pb.GeneratedMessage {
  factory SearchPublicSpacesRequest({
    $core.String? query,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (query != null) result.query = query;
    if (page != null) result.page = page;
    return result;
  }

  SearchPublicSpacesRequest._();

  factory SearchPublicSpacesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchPublicSpacesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchPublicSpacesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchPublicSpacesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchPublicSpacesRequest copyWith(
          void Function(SearchPublicSpacesRequest) updates) =>
      super.copyWith((message) => updates(message as SearchPublicSpacesRequest))
          as SearchPublicSpacesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchPublicSpacesRequest create() => SearchPublicSpacesRequest._();
  @$core.override
  SearchPublicSpacesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchPublicSpacesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchPublicSpacesRequest>(create);
  static SearchPublicSpacesRequest? _defaultInstance;

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

class VoiceRoom extends $pb.GeneratedMessage {
  factory VoiceRoom({
    $core.String? id,
    $core.String? spaceId,
    $core.String? name,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  VoiceRoom._();

  factory VoiceRoom.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceRoom.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceRoom',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoom clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoom copyWith(void Function(VoiceRoom) updates) =>
      super.copyWith((message) => updates(message as VoiceRoom)) as VoiceRoom;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceRoom create() => VoiceRoom._();
  @$core.override
  VoiceRoom createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceRoom getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<VoiceRoom>(create);
  static VoiceRoom? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get createdAt => $_getN(3);
  @$pb.TagNumber(4)
  set createdAt($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasCreatedAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreatedAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureCreatedAt() => $_ensure(3);
}

class CreateVoiceRoomRequest extends $pb.GeneratedMessage {
  factory CreateVoiceRoomRequest({
    $core.String? spaceId,
    $core.String? name,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    return result;
  }

  CreateVoiceRoomRequest._();

  factory CreateVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateVoiceRoomRequest copyWith(
          void Function(CreateVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as CreateVoiceRoomRequest))
          as CreateVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateVoiceRoomRequest create() => CreateVoiceRoomRequest._();
  @$core.override
  CreateVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateVoiceRoomRequest>(create);
  static CreateVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class UpdateVoiceRoomRequest extends $pb.GeneratedMessage {
  factory UpdateVoiceRoomRequest({
    $core.String? voiceRoomId,
    $core.String? name,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (name != null) result.name = name;
    return result;
  }

  UpdateVoiceRoomRequest._();

  factory UpdateVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceRoomRequest copyWith(
          void Function(UpdateVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceRoomRequest))
          as UpdateVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceRoomRequest create() => UpdateVoiceRoomRequest._();
  @$core.override
  UpdateVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceRoomRequest>(create);
  static UpdateVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class DeleteVoiceRoomRequest extends $pb.GeneratedMessage {
  factory DeleteVoiceRoomRequest({
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  DeleteVoiceRoomRequest._();

  factory DeleteVoiceRoomRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteVoiceRoomRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteVoiceRoomRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteVoiceRoomRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteVoiceRoomRequest copyWith(
          void Function(DeleteVoiceRoomRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteVoiceRoomRequest))
          as DeleteVoiceRoomRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteVoiceRoomRequest create() => DeleteVoiceRoomRequest._();
  @$core.override
  DeleteVoiceRoomRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteVoiceRoomRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteVoiceRoomRequest>(create);
  static DeleteVoiceRoomRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get voiceRoomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set voiceRoomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoomId() => $_clearField(1);
}

class SpaceTreeNode extends $pb.GeneratedMessage {
  factory SpaceTreeNode({
    $core.String? id,
    $core.String? spaceId,
    $core.String? categoryId,
    $core.String? kind,
    $3.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $core.int? sortOrder,
    $core.bool? isSystem,
    $core.String? displayName,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (categoryId != null) result.categoryId = categoryId;
    if (kind != null) result.kind = kind;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (sortOrder != null) result.sortOrder = sortOrder;
    if (isSystem != null) result.isSystem = isSystem;
    if (displayName != null) result.displayName = displayName;
    return result;
  }

  SpaceTreeNode._();

  factory SpaceTreeNode.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceTreeNode.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceTreeNode',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'categoryId')
    ..aOS(4, _omitFieldNames ? '' : 'kind')
    ..aOM<$3.ChatRef>(5, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $3.ChatRef.create)
    ..aOS(6, _omitFieldNames ? '' : 'voiceRoomId')
    ..aI(7, _omitFieldNames ? '' : 'sortOrder')
    ..aOB(8, _omitFieldNames ? '' : 'isSystem')
    ..aOS(9, _omitFieldNames ? '' : 'displayName')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTreeNode clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTreeNode copyWith(void Function(SpaceTreeNode) updates) =>
      super.copyWith((message) => updates(message as SpaceTreeNode))
          as SpaceTreeNode;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceTreeNode create() => SpaceTreeNode._();
  @$core.override
  SpaceTreeNode createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceTreeNode getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceTreeNode>(create);
  static SpaceTreeNode? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get categoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set categoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get kind => $_getSZ(3);
  @$pb.TagNumber(4)
  set kind($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasKind() => $_has(3);
  @$pb.TagNumber(4)
  void clearKind() => $_clearField(4);

  @$pb.TagNumber(5)
  $3.ChatRef get linkedChat => $_getN(4);
  @$pb.TagNumber(5)
  set linkedChat($3.ChatRef value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLinkedChat() => $_has(4);
  @$pb.TagNumber(5)
  void clearLinkedChat() => $_clearField(5);
  @$pb.TagNumber(5)
  $3.ChatRef ensureLinkedChat() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get voiceRoomId => $_getSZ(5);
  @$pb.TagNumber(6)
  set voiceRoomId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVoiceRoomId() => $_has(5);
  @$pb.TagNumber(6)
  void clearVoiceRoomId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.int get sortOrder => $_getIZ(6);
  @$pb.TagNumber(7)
  set sortOrder($core.int value) => $_setSignedInt32(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSortOrder() => $_has(6);
  @$pb.TagNumber(7)
  void clearSortOrder() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get isSystem => $_getBF(7);
  @$pb.TagNumber(8)
  set isSystem($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasIsSystem() => $_has(7);
  @$pb.TagNumber(8)
  void clearIsSystem() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get displayName => $_getSZ(8);
  @$pb.TagNumber(9)
  set displayName($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasDisplayName() => $_has(8);
  @$pb.TagNumber(9)
  void clearDisplayName() => $_clearField(9);
}

class UpsertTreeNodeRequest extends $pb.GeneratedMessage {
  factory UpsertTreeNodeRequest({
    $core.String? spaceId,
    $core.String? nodeId,
    $core.String? categoryId,
    $core.String? kind,
    $3.ChatRef? linkedChat,
    $core.String? voiceRoomId,
    $core.int? sortOrder,
    $core.bool? isSystem,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (nodeId != null) result.nodeId = nodeId;
    if (categoryId != null) result.categoryId = categoryId;
    if (kind != null) result.kind = kind;
    if (linkedChat != null) result.linkedChat = linkedChat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (sortOrder != null) result.sortOrder = sortOrder;
    if (isSystem != null) result.isSystem = isSystem;
    return result;
  }

  UpsertTreeNodeRequest._();

  factory UpsertTreeNodeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpsertTreeNodeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertTreeNodeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'nodeId')
    ..aOS(3, _omitFieldNames ? '' : 'categoryId')
    ..aOS(4, _omitFieldNames ? '' : 'kind')
    ..aOM<$3.ChatRef>(5, _omitFieldNames ? '' : 'linkedChat',
        subBuilder: $3.ChatRef.create)
    ..aOS(6, _omitFieldNames ? '' : 'voiceRoomId')
    ..aI(7, _omitFieldNames ? '' : 'sortOrder')
    ..aOB(8, _omitFieldNames ? '' : 'isSystem')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertTreeNodeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertTreeNodeRequest copyWith(
          void Function(UpsertTreeNodeRequest) updates) =>
      super.copyWith((message) => updates(message as UpsertTreeNodeRequest))
          as UpsertTreeNodeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpsertTreeNodeRequest create() => UpsertTreeNodeRequest._();
  @$core.override
  UpsertTreeNodeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpsertTreeNodeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertTreeNodeRequest>(create);
  static UpsertTreeNodeRequest? _defaultInstance;

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
  $core.String get categoryId => $_getSZ(2);
  @$pb.TagNumber(3)
  set categoryId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategoryId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategoryId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get kind => $_getSZ(3);
  @$pb.TagNumber(4)
  set kind($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasKind() => $_has(3);
  @$pb.TagNumber(4)
  void clearKind() => $_clearField(4);

  @$pb.TagNumber(5)
  $3.ChatRef get linkedChat => $_getN(4);
  @$pb.TagNumber(5)
  set linkedChat($3.ChatRef value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLinkedChat() => $_has(4);
  @$pb.TagNumber(5)
  void clearLinkedChat() => $_clearField(5);
  @$pb.TagNumber(5)
  $3.ChatRef ensureLinkedChat() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get voiceRoomId => $_getSZ(5);
  @$pb.TagNumber(6)
  set voiceRoomId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVoiceRoomId() => $_has(5);
  @$pb.TagNumber(6)
  void clearVoiceRoomId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.int get sortOrder => $_getIZ(6);
  @$pb.TagNumber(7)
  set sortOrder($core.int value) => $_setSignedInt32(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSortOrder() => $_has(6);
  @$pb.TagNumber(7)
  void clearSortOrder() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get isSystem => $_getBF(7);
  @$pb.TagNumber(8)
  set isSystem($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasIsSystem() => $_has(7);
  @$pb.TagNumber(8)
  void clearIsSystem() => $_clearField(8);
}

class RemoveTreeNodeRequest extends $pb.GeneratedMessage {
  factory RemoveTreeNodeRequest({
    $core.String? spaceId,
    $core.String? nodeId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (nodeId != null) result.nodeId = nodeId;
    return result;
  }

  RemoveTreeNodeRequest._();

  factory RemoveTreeNodeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveTreeNodeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveTreeNodeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'nodeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveTreeNodeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveTreeNodeRequest copyWith(
          void Function(RemoveTreeNodeRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveTreeNodeRequest))
          as RemoveTreeNodeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveTreeNodeRequest create() => RemoveTreeNodeRequest._();
  @$core.override
  RemoveTreeNodeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveTreeNodeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveTreeNodeRequest>(create);
  static RemoveTreeNodeRequest? _defaultInstance;

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
}

class Category extends $pb.GeneratedMessage {
  factory Category({
    $core.String? id,
    $core.String? spaceId,
    $core.String? name,
    $core.int? sortOrder,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (sortOrder != null) result.sortOrder = sortOrder;
    return result;
  }

  Category._();

  factory Category.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Category.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Category',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..aI(4, _omitFieldNames ? '' : 'sortOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Category clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Category copyWith(void Function(Category) updates) =>
      super.copyWith((message) => updates(message as Category)) as Category;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Category create() => Category._();
  @$core.override
  Category createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Category getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Category>(create);
  static Category? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get sortOrder => $_getIZ(3);
  @$pb.TagNumber(4)
  set sortOrder($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSortOrder() => $_has(3);
  @$pb.TagNumber(4)
  void clearSortOrder() => $_clearField(4);
}

class CreateCategoryRequest extends $pb.GeneratedMessage {
  factory CreateCategoryRequest({
    $core.String? spaceId,
    $core.String? name,
    $core.int? sortOrder,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (sortOrder != null) result.sortOrder = sortOrder;
    return result;
  }

  CreateCategoryRequest._();

  factory CreateCategoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateCategoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCategoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aI(3, _omitFieldNames ? '' : 'sortOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCategoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCategoryRequest copyWith(
          void Function(CreateCategoryRequest) updates) =>
      super.copyWith((message) => updates(message as CreateCategoryRequest))
          as CreateCategoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateCategoryRequest create() => CreateCategoryRequest._();
  @$core.override
  CreateCategoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateCategoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCategoryRequest>(create);
  static CreateCategoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get sortOrder => $_getIZ(2);
  @$pb.TagNumber(3)
  set sortOrder($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSortOrder() => $_has(2);
  @$pb.TagNumber(3)
  void clearSortOrder() => $_clearField(3);
}

class UpdateCategoryRequest extends $pb.GeneratedMessage {
  factory UpdateCategoryRequest({
    $core.String? categoryId,
    $core.String? name,
    $core.int? sortOrder,
  }) {
    final result = create();
    if (categoryId != null) result.categoryId = categoryId;
    if (name != null) result.name = name;
    if (sortOrder != null) result.sortOrder = sortOrder;
    return result;
  }

  UpdateCategoryRequest._();

  factory UpdateCategoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateCategoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCategoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'categoryId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aI(3, _omitFieldNames ? '' : 'sortOrder')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCategoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCategoryRequest copyWith(
          void Function(UpdateCategoryRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateCategoryRequest))
          as UpdateCategoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateCategoryRequest create() => UpdateCategoryRequest._();
  @$core.override
  UpdateCategoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateCategoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCategoryRequest>(create);
  static UpdateCategoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get categoryId => $_getSZ(0);
  @$pb.TagNumber(1)
  set categoryId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCategoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get sortOrder => $_getIZ(2);
  @$pb.TagNumber(3)
  set sortOrder($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSortOrder() => $_has(2);
  @$pb.TagNumber(3)
  void clearSortOrder() => $_clearField(3);
}

class DeleteCategoryRequest extends $pb.GeneratedMessage {
  factory DeleteCategoryRequest({
    $core.String? categoryId,
  }) {
    final result = create();
    if (categoryId != null) result.categoryId = categoryId;
    return result;
  }

  DeleteCategoryRequest._();

  factory DeleteCategoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteCategoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCategoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'categoryId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCategoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCategoryRequest copyWith(
          void Function(DeleteCategoryRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteCategoryRequest))
          as DeleteCategoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteCategoryRequest create() => DeleteCategoryRequest._();
  @$core.override
  DeleteCategoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteCategoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCategoryRequest>(create);
  static DeleteCategoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get categoryId => $_getSZ(0);
  @$pb.TagNumber(1)
  set categoryId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCategoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategoryId() => $_clearField(1);
}

class ReorderSpaceTreeRequest extends $pb.GeneratedMessage {
  factory ReorderSpaceTreeRequest({
    $core.String? spaceId,
    $core.Iterable<$core.String>? orderedNodeIds,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (orderedNodeIds != null) result.orderedNodeIds.addAll(orderedNodeIds);
    return result;
  }

  ReorderSpaceTreeRequest._();

  factory ReorderSpaceTreeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReorderSpaceTreeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReorderSpaceTreeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..pPS(2, _omitFieldNames ? '' : 'orderedNodeIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderSpaceTreeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderSpaceTreeRequest copyWith(
          void Function(ReorderSpaceTreeRequest) updates) =>
      super.copyWith((message) => updates(message as ReorderSpaceTreeRequest))
          as ReorderSpaceTreeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReorderSpaceTreeRequest create() => ReorderSpaceTreeRequest._();
  @$core.override
  ReorderSpaceTreeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReorderSpaceTreeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReorderSpaceTreeRequest>(create);
  static ReorderSpaceTreeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get orderedNodeIds => $_getList(1);
}

class ListSpaceTreeRequest extends $pb.GeneratedMessage {
  factory ListSpaceTreeRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  ListSpaceTreeRequest._();

  factory ListSpaceTreeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSpaceTreeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSpaceTreeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpaceTreeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpaceTreeRequest copyWith(void Function(ListSpaceTreeRequest) updates) =>
      super.copyWith((message) => updates(message as ListSpaceTreeRequest))
          as ListSpaceTreeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSpaceTreeRequest create() => ListSpaceTreeRequest._();
  @$core.override
  ListSpaceTreeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSpaceTreeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSpaceTreeRequest>(create);
  static ListSpaceTreeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class Invite extends $pb.GeneratedMessage {
  factory Invite({
    $core.String? id,
    $core.String? spaceId,
    $core.String? code,
    $core.String? creatorProfileId,
    $core.int? maxUses,
    $core.int? useCount,
    $1.Timestamp? expiresAt,
    $1.Timestamp? createdAt,
    $1.Timestamp? revokedAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (code != null) result.code = code;
    if (creatorProfileId != null) result.creatorProfileId = creatorProfileId;
    if (maxUses != null) result.maxUses = maxUses;
    if (useCount != null) result.useCount = useCount;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (createdAt != null) result.createdAt = createdAt;
    if (revokedAt != null) result.revokedAt = revokedAt;
    return result;
  }

  Invite._();

  factory Invite.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Invite.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Invite',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'code')
    ..aOS(4, _omitFieldNames ? '' : 'creatorProfileId')
    ..aI(5, _omitFieldNames ? '' : 'maxUses')
    ..aI(6, _omitFieldNames ? '' : 'useCount')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'revokedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Invite clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Invite copyWith(void Function(Invite) updates) =>
      super.copyWith((message) => updates(message as Invite)) as Invite;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Invite create() => Invite._();
  @$core.override
  Invite createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Invite getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Invite>(create);
  static Invite? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get code => $_getSZ(2);
  @$pb.TagNumber(3)
  set code($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCode() => $_has(2);
  @$pb.TagNumber(3)
  void clearCode() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get creatorProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set creatorProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCreatorProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearCreatorProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get maxUses => $_getIZ(4);
  @$pb.TagNumber(5)
  set maxUses($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMaxUses() => $_has(4);
  @$pb.TagNumber(5)
  void clearMaxUses() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.int get useCount => $_getIZ(5);
  @$pb.TagNumber(6)
  set useCount($core.int value) => $_setSignedInt32(5, value);
  @$pb.TagNumber(6)
  $core.bool hasUseCount() => $_has(5);
  @$pb.TagNumber(6)
  void clearUseCount() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.Timestamp get expiresAt => $_getN(6);
  @$pb.TagNumber(7)
  set expiresAt($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasExpiresAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpiresAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureExpiresAt() => $_ensure(6);

  @$pb.TagNumber(8)
  $1.Timestamp get createdAt => $_getN(7);
  @$pb.TagNumber(8)
  set createdAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCreatedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreatedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureCreatedAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get revokedAt => $_getN(8);
  @$pb.TagNumber(9)
  set revokedAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasRevokedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearRevokedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureRevokedAt() => $_ensure(8);
}

class CreateInviteRequest extends $pb.GeneratedMessage {
  factory CreateInviteRequest({
    $core.String? spaceId,
    $core.int? maxUses,
    $1.Timestamp? expiresAt,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (maxUses != null) result.maxUses = maxUses;
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  CreateInviteRequest._();

  factory CreateInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateInviteRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aI(2, _omitFieldNames ? '' : 'maxUses')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateInviteRequest copyWith(void Function(CreateInviteRequest) updates) =>
      super.copyWith((message) => updates(message as CreateInviteRequest))
          as CreateInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateInviteRequest create() => CreateInviteRequest._();
  @$core.override
  CreateInviteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateInviteRequest>(create);
  static CreateInviteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get maxUses => $_getIZ(1);
  @$pb.TagNumber(2)
  set maxUses($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMaxUses() => $_has(1);
  @$pb.TagNumber(2)
  void clearMaxUses() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get expiresAt => $_getN(2);
  @$pb.TagNumber(3)
  set expiresAt($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasExpiresAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpiresAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureExpiresAt() => $_ensure(2);
}

class RevokeInviteRequest extends $pb.GeneratedMessage {
  factory RevokeInviteRequest({
    $core.String? inviteId,
  }) {
    final result = create();
    if (inviteId != null) result.inviteId = inviteId;
    return result;
  }

  RevokeInviteRequest._();

  factory RevokeInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeInviteRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'inviteId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeInviteRequest copyWith(void Function(RevokeInviteRequest) updates) =>
      super.copyWith((message) => updates(message as RevokeInviteRequest))
          as RevokeInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeInviteRequest create() => RevokeInviteRequest._();
  @$core.override
  RevokeInviteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeInviteRequest>(create);
  static RevokeInviteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get inviteId => $_getSZ(0);
  @$pb.TagNumber(1)
  set inviteId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasInviteId() => $_has(0);
  @$pb.TagNumber(1)
  void clearInviteId() => $_clearField(1);
}

class GetInviteRequest extends $pb.GeneratedMessage {
  factory GetInviteRequest({
    $core.String? code,
  }) {
    final result = create();
    if (code != null) result.code = code;
    return result;
  }

  GetInviteRequest._();

  factory GetInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetInviteRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInviteRequest copyWith(void Function(GetInviteRequest) updates) =>
      super.copyWith((message) => updates(message as GetInviteRequest))
          as GetInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetInviteRequest create() => GetInviteRequest._();
  @$core.override
  GetInviteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetInviteRequest>(create);
  static GetInviteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);
}

class ListInvitesRequest extends $pb.GeneratedMessage {
  factory ListInvitesRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  ListInvitesRequest._();

  factory ListInvitesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInvitesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInvitesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInvitesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInvitesRequest copyWith(void Function(ListInvitesRequest) updates) =>
      super.copyWith((message) => updates(message as ListInvitesRequest))
          as ListInvitesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInvitesRequest create() => ListInvitesRequest._();
  @$core.override
  ListInvitesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInvitesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInvitesRequest>(create);
  static ListInvitesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class InviteList extends $pb.GeneratedMessage {
  factory InviteList({
    $core.Iterable<Invite>? invites,
  }) {
    final result = create();
    if (invites != null) result.invites.addAll(invites);
    return result;
  }

  InviteList._();

  factory InviteList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory InviteList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'InviteList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<Invite>(1, _omitFieldNames ? '' : 'invites',
        subBuilder: Invite.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InviteList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  InviteList copyWith(void Function(InviteList) updates) =>
      super.copyWith((message) => updates(message as InviteList)) as InviteList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static InviteList create() => InviteList._();
  @$core.override
  InviteList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static InviteList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<InviteList>(create);
  static InviteList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Invite> get invites => $_getList(0);
}

class JoinByInviteRequest extends $pb.GeneratedMessage {
  factory JoinByInviteRequest({
    $core.String? code,
  }) {
    final result = create();
    if (code != null) result.code = code;
    return result;
  }

  JoinByInviteRequest._();

  factory JoinByInviteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinByInviteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinByInviteRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinByInviteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinByInviteRequest copyWith(void Function(JoinByInviteRequest) updates) =>
      super.copyWith((message) => updates(message as JoinByInviteRequest))
          as JoinByInviteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinByInviteRequest create() => JoinByInviteRequest._();
  @$core.override
  JoinByInviteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinByInviteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinByInviteRequest>(create);
  static JoinByInviteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);
}

class SpaceMembership extends $pb.GeneratedMessage {
  factory SpaceMembership({
    $core.String? spaceId,
    $core.String? profileId,
    $1.Timestamp? joinedAt,
    $core.String? nickname,
    $core.Iterable<$core.String>? roleNames,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (joinedAt != null) result.joinedAt = joinedAt;
    if (nickname != null) result.nickname = nickname;
    if (roleNames != null) result.roleNames.addAll(roleNames);
    return result;
  }

  SpaceMembership._();

  factory SpaceMembership.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceMembership.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceMembership',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'joinedAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(4, _omitFieldNames ? '' : 'nickname')
    ..pPS(5, _omitFieldNames ? '' : 'roleNames')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceMembership clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceMembership copyWith(void Function(SpaceMembership) updates) =>
      super.copyWith((message) => updates(message as SpaceMembership))
          as SpaceMembership;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceMembership create() => SpaceMembership._();
  @$core.override
  SpaceMembership createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceMembership getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceMembership>(create);
  static SpaceMembership? _defaultInstance;

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
  $1.Timestamp get joinedAt => $_getN(2);
  @$pb.TagNumber(3)
  set joinedAt($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasJoinedAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearJoinedAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureJoinedAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get nickname => $_getSZ(3);
  @$pb.TagNumber(4)
  set nickname($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNickname() => $_has(3);
  @$pb.TagNumber(4)
  void clearNickname() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get roleNames => $_getList(4);
}

class JoinSpaceRequest extends $pb.GeneratedMessage {
  factory JoinSpaceRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  JoinSpaceRequest._();

  factory JoinSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinSpaceRequest copyWith(void Function(JoinSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as JoinSpaceRequest))
          as JoinSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinSpaceRequest create() => JoinSpaceRequest._();
  @$core.override
  JoinSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinSpaceRequest>(create);
  static JoinSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class LeaveSpaceRequest extends $pb.GeneratedMessage {
  factory LeaveSpaceRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  LeaveSpaceRequest._();

  factory LeaveSpaceRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveSpaceRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveSpaceRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveSpaceRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveSpaceRequest copyWith(void Function(LeaveSpaceRequest) updates) =>
      super.copyWith((message) => updates(message as LeaveSpaceRequest))
          as LeaveSpaceRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveSpaceRequest create() => LeaveSpaceRequest._();
  @$core.override
  LeaveSpaceRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveSpaceRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveSpaceRequest>(create);
  static LeaveSpaceRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class KickMemberRequest extends $pb.GeneratedMessage {
  factory KickMemberRequest({
    $core.String? spaceId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  KickMemberRequest._();

  factory KickMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory KickMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KickMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KickMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KickMemberRequest copyWith(void Function(KickMemberRequest) updates) =>
      super.copyWith((message) => updates(message as KickMemberRequest))
          as KickMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static KickMemberRequest create() => KickMemberRequest._();
  @$core.override
  KickMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static KickMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<KickMemberRequest>(create);
  static KickMemberRequest? _defaultInstance;

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
}

class BanMemberRequest extends $pb.GeneratedMessage {
  factory BanMemberRequest({
    $core.String? spaceId,
    $core.String? accountId,
    $core.String? reason,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (accountId != null) result.accountId = accountId;
    if (reason != null) result.reason = reason;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  BanMemberRequest._();

  factory BanMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..aOS(4, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanMemberRequest copyWith(void Function(BanMemberRequest) updates) =>
      super.copyWith((message) => updates(message as BanMemberRequest))
          as BanMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanMemberRequest create() => BanMemberRequest._();
  @$core.override
  BanMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BanMemberRequest>(create);
  static BanMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get profileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set profileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearProfileId() => $_clearField(4);
}

class UnbanMemberRequest extends $pb.GeneratedMessage {
  factory UnbanMemberRequest({
    $core.String? spaceId,
    $core.String? accountId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  UnbanMemberRequest._();

  factory UnbanMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnbanMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnbanMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanMemberRequest copyWith(void Function(UnbanMemberRequest) updates) =>
      super.copyWith((message) => updates(message as UnbanMemberRequest))
          as UnbanMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnbanMemberRequest create() => UnbanMemberRequest._();
  @$core.override
  UnbanMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnbanMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnbanMemberRequest>(create);
  static UnbanMemberRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);
}

class ListMembersRequest extends $pb.GeneratedMessage {
  factory ListMembersRequest({
    $core.String? spaceId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (page != null) result.page = page;
    return result;
  }

  ListMembersRequest._();

  factory ListMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersRequest copyWith(void Function(ListMembersRequest) updates) =>
      super.copyWith((message) => updates(message as ListMembersRequest))
          as ListMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersRequest create() => ListMembersRequest._();
  @$core.override
  ListMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersRequest>(create);
  static ListMembersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

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

class SpaceMemberList extends $pb.GeneratedMessage {
  factory SpaceMemberList({
    $core.Iterable<SpaceMembership>? members,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (members != null) result.members.addAll(members);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  SpaceMemberList._();

  factory SpaceMemberList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceMemberList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceMemberList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<SpaceMembership>(1, _omitFieldNames ? '' : 'members',
        subBuilder: SpaceMembership.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceMemberList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceMemberList copyWith(void Function(SpaceMemberList) updates) =>
      super.copyWith((message) => updates(message as SpaceMemberList))
          as SpaceMemberList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceMemberList create() => SpaceMemberList._();
  @$core.override
  SpaceMemberList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceMemberList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceMemberList>(create);
  static SpaceMemberList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SpaceMembership> get members => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class ListBansRequest extends $pb.GeneratedMessage {
  factory ListBansRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  ListBansRequest._();

  factory ListBansRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBansRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBansRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBansRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBansRequest copyWith(void Function(ListBansRequest) updates) =>
      super.copyWith((message) => updates(message as ListBansRequest))
          as ListBansRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBansRequest create() => ListBansRequest._();
  @$core.override
  ListBansRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBansRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBansRequest>(create);
  static ListBansRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class BanList extends $pb.GeneratedMessage {
  factory BanList({
    $core.Iterable<SpaceBan>? bans,
  }) {
    final result = create();
    if (bans != null) result.bans.addAll(bans);
    return result;
  }

  BanList._();

  factory BanList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<SpaceBan>(1, _omitFieldNames ? '' : 'bans',
        subBuilder: SpaceBan.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanList copyWith(void Function(BanList) updates) =>
      super.copyWith((message) => updates(message as BanList)) as BanList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanList create() => BanList._();
  @$core.override
  BanList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<BanList>(create);
  static BanList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SpaceBan> get bans => $_getList(0);
}

class SpaceBan extends $pb.GeneratedMessage {
  factory SpaceBan({
    $core.String? spaceId,
    $core.String? accountId,
    $core.String? bannedByProfileId,
    $core.String? reason,
    $1.Timestamp? bannedAt,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (accountId != null) result.accountId = accountId;
    if (bannedByProfileId != null) result.bannedByProfileId = bannedByProfileId;
    if (reason != null) result.reason = reason;
    if (bannedAt != null) result.bannedAt = bannedAt;
    return result;
  }

  SpaceBan._();

  factory SpaceBan.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceBan.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceBan',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'bannedByProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'bannedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceBan clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceBan copyWith(void Function(SpaceBan) updates) =>
      super.copyWith((message) => updates(message as SpaceBan)) as SpaceBan;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceBan create() => SpaceBan._();
  @$core.override
  SpaceBan createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceBan getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SpaceBan>(create);
  static SpaceBan? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get bannedByProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set bannedByProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasBannedByProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearBannedByProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get bannedAt => $_getN(4);
  @$pb.TagNumber(5)
  set bannedAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasBannedAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearBannedAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureBannedAt() => $_ensure(4);
}

class TimeoutMemberRequest extends $pb.GeneratedMessage {
  factory TimeoutMemberRequest({
    $core.String? spaceId,
    $core.String? profileId,
    $core.int? durationSeconds,
    $core.String? reason,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (durationSeconds != null) result.durationSeconds = durationSeconds;
    if (reason != null) result.reason = reason;
    return result;
  }

  TimeoutMemberRequest._();

  factory TimeoutMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TimeoutMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TimeoutMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aI(3, _omitFieldNames ? '' : 'durationSeconds')
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TimeoutMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TimeoutMemberRequest copyWith(void Function(TimeoutMemberRequest) updates) =>
      super.copyWith((message) => updates(message as TimeoutMemberRequest))
          as TimeoutMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TimeoutMemberRequest create() => TimeoutMemberRequest._();
  @$core.override
  TimeoutMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TimeoutMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TimeoutMemberRequest>(create);
  static TimeoutMemberRequest? _defaultInstance;

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
  $core.int get durationSeconds => $_getIZ(2);
  @$pb.TagNumber(3)
  set durationSeconds($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDurationSeconds() => $_has(2);
  @$pb.TagNumber(3)
  void clearDurationSeconds() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);
}

class RemoveMemberTimeoutRequest extends $pb.GeneratedMessage {
  factory RemoveMemberTimeoutRequest({
    $core.String? spaceId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  RemoveMemberTimeoutRequest._();

  factory RemoveMemberTimeoutRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveMemberTimeoutRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveMemberTimeoutRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberTimeoutRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberTimeoutRequest copyWith(
          void Function(RemoveMemberTimeoutRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveMemberTimeoutRequest))
          as RemoveMemberTimeoutRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveMemberTimeoutRequest create() => RemoveMemberTimeoutRequest._();
  @$core.override
  RemoveMemberTimeoutRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveMemberTimeoutRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveMemberTimeoutRequest>(create);
  static RemoveMemberTimeoutRequest? _defaultInstance;

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
}

class TransferOwnershipRequest extends $pb.GeneratedMessage {
  factory TransferOwnershipRequest({
    $core.String? spaceId,
    $core.String? newOwnerProfileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (newOwnerProfileId != null) result.newOwnerProfileId = newOwnerProfileId;
    return result;
  }

  TransferOwnershipRequest._();

  factory TransferOwnershipRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TransferOwnershipRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransferOwnershipRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'newOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferOwnershipRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferOwnershipRequest copyWith(
          void Function(TransferOwnershipRequest) updates) =>
      super.copyWith((message) => updates(message as TransferOwnershipRequest))
          as TransferOwnershipRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TransferOwnershipRequest create() => TransferOwnershipRequest._();
  @$core.override
  TransferOwnershipRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TransferOwnershipRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransferOwnershipRequest>(create);
  static TransferOwnershipRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get newOwnerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set newOwnerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNewOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearNewOwnerProfileId() => $_clearField(2);
}

class AddBotMemberRequest extends $pb.GeneratedMessage {
  factory AddBotMemberRequest({
    $core.String? spaceId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  AddBotMemberRequest._();

  factory AddBotMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddBotMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddBotMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddBotMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddBotMemberRequest copyWith(void Function(AddBotMemberRequest) updates) =>
      super.copyWith((message) => updates(message as AddBotMemberRequest))
          as AddBotMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddBotMemberRequest create() => AddBotMemberRequest._();
  @$core.override
  AddBotMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddBotMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddBotMemberRequest>(create);
  static AddBotMemberRequest? _defaultInstance;

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
}

class RemoveBotMemberRequest extends $pb.GeneratedMessage {
  factory RemoveBotMemberRequest({
    $core.String? spaceId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  RemoveBotMemberRequest._();

  factory RemoveBotMemberRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveBotMemberRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveBotMemberRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveBotMemberRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveBotMemberRequest copyWith(
          void Function(RemoveBotMemberRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveBotMemberRequest))
          as RemoveBotMemberRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveBotMemberRequest create() => RemoveBotMemberRequest._();
  @$core.override
  RemoveBotMemberRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveBotMemberRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveBotMemberRequest>(create);
  static RemoveBotMemberRequest? _defaultInstance;

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
}

class ListTemplatesRequest extends $pb.GeneratedMessage {
  factory ListTemplatesRequest() => create();

  ListTemplatesRequest._();

  factory ListTemplatesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListTemplatesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTemplatesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTemplatesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTemplatesRequest copyWith(void Function(ListTemplatesRequest) updates) =>
      super.copyWith((message) => updates(message as ListTemplatesRequest))
          as ListTemplatesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListTemplatesRequest create() => ListTemplatesRequest._();
  @$core.override
  ListTemplatesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListTemplatesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListTemplatesRequest>(create);
  static ListTemplatesRequest? _defaultInstance;
}

class TemplateList extends $pb.GeneratedMessage {
  factory TemplateList({
    $core.Iterable<SpaceTemplate>? templates,
  }) {
    final result = create();
    if (templates != null) result.templates.addAll(templates);
    return result;
  }

  TemplateList._();

  factory TemplateList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TemplateList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TemplateList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<SpaceTemplate>(1, _omitFieldNames ? '' : 'templates',
        subBuilder: SpaceTemplate.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TemplateList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TemplateList copyWith(void Function(TemplateList) updates) =>
      super.copyWith((message) => updates(message as TemplateList))
          as TemplateList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TemplateList create() => TemplateList._();
  @$core.override
  TemplateList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TemplateList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TemplateList>(create);
  static TemplateList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SpaceTemplate> get templates => $_getList(0);
}

class SpaceTemplate extends $pb.GeneratedMessage {
  factory SpaceTemplate({
    $core.String? id,
    $core.String? name,
    $core.String? description,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (description != null) result.description = description;
    return result;
  }

  SpaceTemplate._();

  factory SpaceTemplate.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceTemplate.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceTemplate',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'description')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTemplate clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceTemplate copyWith(void Function(SpaceTemplate) updates) =>
      super.copyWith((message) => updates(message as SpaceTemplate))
          as SpaceTemplate;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceTemplate create() => SpaceTemplate._();
  @$core.override
  SpaceTemplate createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceTemplate getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceTemplate>(create);
  static SpaceTemplate? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get description => $_getSZ(2);
  @$pb.TagNumber(3)
  set description($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDescription() => $_has(2);
  @$pb.TagNumber(3)
  void clearDescription() => $_clearField(3);
}

class CreateFromTemplateRequest extends $pb.GeneratedMessage {
  factory CreateFromTemplateRequest({
    $core.String? templateId,
    $core.String? name,
  }) {
    final result = create();
    if (templateId != null) result.templateId = templateId;
    if (name != null) result.name = name;
    return result;
  }

  CreateFromTemplateRequest._();

  factory CreateFromTemplateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateFromTemplateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateFromTemplateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'templateId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFromTemplateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFromTemplateRequest copyWith(
          void Function(CreateFromTemplateRequest) updates) =>
      super.copyWith((message) => updates(message as CreateFromTemplateRequest))
          as CreateFromTemplateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateFromTemplateRequest create() => CreateFromTemplateRequest._();
  @$core.override
  CreateFromTemplateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateFromTemplateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateFromTemplateRequest>(create);
  static CreateFromTemplateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get templateId => $_getSZ(0);
  @$pb.TagNumber(1)
  set templateId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTemplateId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTemplateId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);
}

class GetAuditLogRequest extends $pb.GeneratedMessage {
  factory GetAuditLogRequest({
    $core.String? spaceId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (page != null) result.page = page;
    return result;
  }

  GetAuditLogRequest._();

  factory GetAuditLogRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAuditLogRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAuditLogRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAuditLogRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAuditLogRequest copyWith(void Function(GetAuditLogRequest) updates) =>
      super.copyWith((message) => updates(message as GetAuditLogRequest))
          as GetAuditLogRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAuditLogRequest create() => GetAuditLogRequest._();
  @$core.override
  GetAuditLogRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAuditLogRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAuditLogRequest>(create);
  static GetAuditLogRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

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

class AuditLogList extends $pb.GeneratedMessage {
  factory AuditLogList({
    $core.Iterable<AuditLogEntry>? entries,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (entries != null) result.entries.addAll(entries);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  AuditLogList._();

  factory AuditLogList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuditLogList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuditLogList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<AuditLogEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: AuditLogEntry.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuditLogList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuditLogList copyWith(void Function(AuditLogList) updates) =>
      super.copyWith((message) => updates(message as AuditLogList))
          as AuditLogList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuditLogList create() => AuditLogList._();
  @$core.override
  AuditLogList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuditLogList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuditLogList>(create);
  static AuditLogList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<AuditLogEntry> get entries => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class AuditLogEntry extends $pb.GeneratedMessage {
  factory AuditLogEntry({
    $core.String? id,
    $core.String? spaceId,
    $core.String? actorProfileId,
    $core.String? action,
    $core.String? targetType,
    $core.String? targetId,
    $core.String? detailsJson,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (actorProfileId != null) result.actorProfileId = actorProfileId;
    if (action != null) result.action = action;
    if (targetType != null) result.targetType = targetType;
    if (targetId != null) result.targetId = targetId;
    if (detailsJson != null) result.detailsJson = detailsJson;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  AuditLogEntry._();

  factory AuditLogEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuditLogEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuditLogEntry',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'actorProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'action')
    ..aOS(5, _omitFieldNames ? '' : 'targetType')
    ..aOS(6, _omitFieldNames ? '' : 'targetId')
    ..aOS(7, _omitFieldNames ? '' : 'detailsJson')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuditLogEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuditLogEntry copyWith(void Function(AuditLogEntry) updates) =>
      super.copyWith((message) => updates(message as AuditLogEntry))
          as AuditLogEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuditLogEntry create() => AuditLogEntry._();
  @$core.override
  AuditLogEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuditLogEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuditLogEntry>(create);
  static AuditLogEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get actorProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set actorProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasActorProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearActorProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get action => $_getSZ(3);
  @$pb.TagNumber(4)
  set action($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAction() => $_has(3);
  @$pb.TagNumber(4)
  void clearAction() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get targetType => $_getSZ(4);
  @$pb.TagNumber(5)
  set targetType($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTargetType() => $_has(4);
  @$pb.TagNumber(5)
  void clearTargetType() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get targetId => $_getSZ(5);
  @$pb.TagNumber(6)
  set targetId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTargetId() => $_has(5);
  @$pb.TagNumber(6)
  void clearTargetId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get detailsJson => $_getSZ(6);
  @$pb.TagNumber(7)
  set detailsJson($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasDetailsJson() => $_has(6);
  @$pb.TagNumber(7)
  void clearDetailsJson() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get createdAt => $_getN(7);
  @$pb.TagNumber(8)
  set createdAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCreatedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreatedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureCreatedAt() => $_ensure(7);
}

/// Logical reference to space_db.spaces for other services (cf. ChatRef).
class SpaceRef extends $pb.GeneratedMessage {
  factory SpaceRef({
    $core.String? id,
  }) {
    final result = create();
    if (id != null) result.id = id;
    return result;
  }

  SpaceRef._();

  factory SpaceRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceRef',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceRef copyWith(void Function(SpaceRef) updates) =>
      super.copyWith((message) => updates(message as SpaceRef)) as SpaceRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceRef create() => SpaceRef._();
  @$core.override
  SpaceRef createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceRef getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SpaceRef>(create);
  static SpaceRef? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);
}

class CreateSpaceResponse extends $pb.GeneratedMessage {
  factory CreateSpaceResponse({
    Space? space,
  }) {
    final result = create();
    if (space != null) result.space = space;
    return result;
  }

  CreateSpaceResponse._();

  factory CreateSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Space>(1, _omitFieldNames ? '' : 'space', subBuilder: Space.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceResponse copyWith(void Function(CreateSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as CreateSpaceResponse))
          as CreateSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateSpaceResponse create() => CreateSpaceResponse._();
  @$core.override
  CreateSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateSpaceResponse>(create);
  static CreateSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Space get space => $_getN(0);
  @$pb.TagNumber(1)
  set space(Space value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  Space ensureSpace() => $_ensure(0);
}

class UpdateSpaceResponse extends $pb.GeneratedMessage {
  factory UpdateSpaceResponse({
    Space? space,
  }) {
    final result = create();
    if (space != null) result.space = space;
    return result;
  }

  UpdateSpaceResponse._();

  factory UpdateSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Space>(1, _omitFieldNames ? '' : 'space', subBuilder: Space.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateSpaceResponse copyWith(void Function(UpdateSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateSpaceResponse))
          as UpdateSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateSpaceResponse create() => UpdateSpaceResponse._();
  @$core.override
  UpdateSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateSpaceResponse>(create);
  static UpdateSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Space get space => $_getN(0);
  @$pb.TagNumber(1)
  set space(Space value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  Space ensureSpace() => $_ensure(0);
}

class DeleteSpaceResponse extends $pb.GeneratedMessage {
  factory DeleteSpaceResponse() => create();

  DeleteSpaceResponse._();

  factory DeleteSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteSpaceResponse copyWith(void Function(DeleteSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteSpaceResponse))
          as DeleteSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteSpaceResponse create() => DeleteSpaceResponse._();
  @$core.override
  DeleteSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteSpaceResponse>(create);
  static DeleteSpaceResponse? _defaultInstance;
}

class GetSpaceResponse extends $pb.GeneratedMessage {
  factory GetSpaceResponse({
    Space? space,
  }) {
    final result = create();
    if (space != null) result.space = space;
    return result;
  }

  GetSpaceResponse._();

  factory GetSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Space>(1, _omitFieldNames ? '' : 'space', subBuilder: Space.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceResponse copyWith(void Function(GetSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as GetSpaceResponse))
          as GetSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceResponse create() => GetSpaceResponse._();
  @$core.override
  GetSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceResponse>(create);
  static GetSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Space get space => $_getN(0);
  @$pb.TagNumber(1)
  set space(Space value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  Space ensureSpace() => $_ensure(0);
}

class ListMySpacesResponse extends $pb.GeneratedMessage {
  factory ListMySpacesResponse({
    SpaceList? spaceList,
  }) {
    final result = create();
    if (spaceList != null) result.spaceList = spaceList;
    return result;
  }

  ListMySpacesResponse._();

  factory ListMySpacesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMySpacesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMySpacesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceList>(1, _omitFieldNames ? '' : 'spaceList',
        subBuilder: SpaceList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySpacesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMySpacesResponse copyWith(void Function(ListMySpacesResponse) updates) =>
      super.copyWith((message) => updates(message as ListMySpacesResponse))
          as ListMySpacesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMySpacesResponse create() => ListMySpacesResponse._();
  @$core.override
  ListMySpacesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMySpacesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMySpacesResponse>(create);
  static ListMySpacesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceList get spaceList => $_getN(0);
  @$pb.TagNumber(1)
  set spaceList(SpaceList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceList() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceList() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceList ensureSpaceList() => $_ensure(0);
}

class SearchPublicSpacesResponse extends $pb.GeneratedMessage {
  factory SearchPublicSpacesResponse({
    SpaceList? spaceList,
  }) {
    final result = create();
    if (spaceList != null) result.spaceList = spaceList;
    return result;
  }

  SearchPublicSpacesResponse._();

  factory SearchPublicSpacesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchPublicSpacesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchPublicSpacesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceList>(1, _omitFieldNames ? '' : 'spaceList',
        subBuilder: SpaceList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchPublicSpacesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchPublicSpacesResponse copyWith(
          void Function(SearchPublicSpacesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SearchPublicSpacesResponse))
          as SearchPublicSpacesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchPublicSpacesResponse create() => SearchPublicSpacesResponse._();
  @$core.override
  SearchPublicSpacesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchPublicSpacesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchPublicSpacesResponse>(create);
  static SearchPublicSpacesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceList get spaceList => $_getN(0);
  @$pb.TagNumber(1)
  set spaceList(SpaceList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceList() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceList() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceList ensureSpaceList() => $_ensure(0);
}

class CreateVoiceRoomResponse extends $pb.GeneratedMessage {
  factory CreateVoiceRoomResponse({
    VoiceRoom? voiceRoom,
  }) {
    final result = create();
    if (voiceRoom != null) result.voiceRoom = voiceRoom;
    return result;
  }

  CreateVoiceRoomResponse._();

  factory CreateVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceRoom>(1, _omitFieldNames ? '' : 'voiceRoom',
        subBuilder: VoiceRoom.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateVoiceRoomResponse copyWith(
          void Function(CreateVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as CreateVoiceRoomResponse))
          as CreateVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateVoiceRoomResponse create() => CreateVoiceRoomResponse._();
  @$core.override
  CreateVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateVoiceRoomResponse>(create);
  static CreateVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceRoom get voiceRoom => $_getN(0);
  @$pb.TagNumber(1)
  set voiceRoom(VoiceRoom value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoom() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoom() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceRoom ensureVoiceRoom() => $_ensure(0);
}

class UpdateVoiceRoomResponse extends $pb.GeneratedMessage {
  factory UpdateVoiceRoomResponse({
    VoiceRoom? voiceRoom,
  }) {
    final result = create();
    if (voiceRoom != null) result.voiceRoom = voiceRoom;
    return result;
  }

  UpdateVoiceRoomResponse._();

  factory UpdateVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceRoom>(1, _omitFieldNames ? '' : 'voiceRoom',
        subBuilder: VoiceRoom.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateVoiceRoomResponse copyWith(
          void Function(UpdateVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateVoiceRoomResponse))
          as UpdateVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateVoiceRoomResponse create() => UpdateVoiceRoomResponse._();
  @$core.override
  UpdateVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateVoiceRoomResponse>(create);
  static UpdateVoiceRoomResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceRoom get voiceRoom => $_getN(0);
  @$pb.TagNumber(1)
  set voiceRoom(VoiceRoom value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasVoiceRoom() => $_has(0);
  @$pb.TagNumber(1)
  void clearVoiceRoom() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceRoom ensureVoiceRoom() => $_ensure(0);
}

class DeleteVoiceRoomResponse extends $pb.GeneratedMessage {
  factory DeleteVoiceRoomResponse() => create();

  DeleteVoiceRoomResponse._();

  factory DeleteVoiceRoomResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteVoiceRoomResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteVoiceRoomResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteVoiceRoomResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteVoiceRoomResponse copyWith(
          void Function(DeleteVoiceRoomResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteVoiceRoomResponse))
          as DeleteVoiceRoomResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteVoiceRoomResponse create() => DeleteVoiceRoomResponse._();
  @$core.override
  DeleteVoiceRoomResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteVoiceRoomResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteVoiceRoomResponse>(create);
  static DeleteVoiceRoomResponse? _defaultInstance;
}

class UpsertTreeNodeResponse extends $pb.GeneratedMessage {
  factory UpsertTreeNodeResponse({
    SpaceTreeNode? spaceTreeNode,
  }) {
    final result = create();
    if (spaceTreeNode != null) result.spaceTreeNode = spaceTreeNode;
    return result;
  }

  UpsertTreeNodeResponse._();

  factory UpsertTreeNodeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpsertTreeNodeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertTreeNodeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceTreeNode>(1, _omitFieldNames ? '' : 'spaceTreeNode',
        subBuilder: SpaceTreeNode.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertTreeNodeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertTreeNodeResponse copyWith(
          void Function(UpsertTreeNodeResponse) updates) =>
      super.copyWith((message) => updates(message as UpsertTreeNodeResponse))
          as UpsertTreeNodeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpsertTreeNodeResponse create() => UpsertTreeNodeResponse._();
  @$core.override
  UpsertTreeNodeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpsertTreeNodeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertTreeNodeResponse>(create);
  static UpsertTreeNodeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceTreeNode get spaceTreeNode => $_getN(0);
  @$pb.TagNumber(1)
  set spaceTreeNode(SpaceTreeNode value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceTreeNode() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceTreeNode() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceTreeNode ensureSpaceTreeNode() => $_ensure(0);
}

class RemoveTreeNodeResponse extends $pb.GeneratedMessage {
  factory RemoveTreeNodeResponse() => create();

  RemoveTreeNodeResponse._();

  factory RemoveTreeNodeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveTreeNodeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveTreeNodeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveTreeNodeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveTreeNodeResponse copyWith(
          void Function(RemoveTreeNodeResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveTreeNodeResponse))
          as RemoveTreeNodeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveTreeNodeResponse create() => RemoveTreeNodeResponse._();
  @$core.override
  RemoveTreeNodeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveTreeNodeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveTreeNodeResponse>(create);
  static RemoveTreeNodeResponse? _defaultInstance;
}

class CreateCategoryResponse extends $pb.GeneratedMessage {
  factory CreateCategoryResponse({
    Category? category,
  }) {
    final result = create();
    if (category != null) result.category = category;
    return result;
  }

  CreateCategoryResponse._();

  factory CreateCategoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateCategoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCategoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Category>(1, _omitFieldNames ? '' : 'category',
        subBuilder: Category.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCategoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCategoryResponse copyWith(
          void Function(CreateCategoryResponse) updates) =>
      super.copyWith((message) => updates(message as CreateCategoryResponse))
          as CreateCategoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateCategoryResponse create() => CreateCategoryResponse._();
  @$core.override
  CreateCategoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateCategoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCategoryResponse>(create);
  static CreateCategoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Category get category => $_getN(0);
  @$pb.TagNumber(1)
  set category(Category value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCategory() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategory() => $_clearField(1);
  @$pb.TagNumber(1)
  Category ensureCategory() => $_ensure(0);
}

class UpdateCategoryResponse extends $pb.GeneratedMessage {
  factory UpdateCategoryResponse({
    Category? category,
  }) {
    final result = create();
    if (category != null) result.category = category;
    return result;
  }

  UpdateCategoryResponse._();

  factory UpdateCategoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateCategoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateCategoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Category>(1, _omitFieldNames ? '' : 'category',
        subBuilder: Category.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCategoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateCategoryResponse copyWith(
          void Function(UpdateCategoryResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateCategoryResponse))
          as UpdateCategoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateCategoryResponse create() => UpdateCategoryResponse._();
  @$core.override
  UpdateCategoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateCategoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateCategoryResponse>(create);
  static UpdateCategoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Category get category => $_getN(0);
  @$pb.TagNumber(1)
  set category(Category value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCategory() => $_has(0);
  @$pb.TagNumber(1)
  void clearCategory() => $_clearField(1);
  @$pb.TagNumber(1)
  Category ensureCategory() => $_ensure(0);
}

class DeleteCategoryResponse extends $pb.GeneratedMessage {
  factory DeleteCategoryResponse() => create();

  DeleteCategoryResponse._();

  factory DeleteCategoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteCategoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteCategoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCategoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteCategoryResponse copyWith(
          void Function(DeleteCategoryResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteCategoryResponse))
          as DeleteCategoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteCategoryResponse create() => DeleteCategoryResponse._();
  @$core.override
  DeleteCategoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteCategoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteCategoryResponse>(create);
  static DeleteCategoryResponse? _defaultInstance;
}

class ReorderSpaceTreeResponse extends $pb.GeneratedMessage {
  factory ReorderSpaceTreeResponse() => create();

  ReorderSpaceTreeResponse._();

  factory ReorderSpaceTreeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReorderSpaceTreeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReorderSpaceTreeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderSpaceTreeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderSpaceTreeResponse copyWith(
          void Function(ReorderSpaceTreeResponse) updates) =>
      super.copyWith((message) => updates(message as ReorderSpaceTreeResponse))
          as ReorderSpaceTreeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReorderSpaceTreeResponse create() => ReorderSpaceTreeResponse._();
  @$core.override
  ReorderSpaceTreeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReorderSpaceTreeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReorderSpaceTreeResponse>(create);
  static ReorderSpaceTreeResponse? _defaultInstance;
}

class ListSpaceTreeResponse extends $pb.GeneratedMessage {
  factory ListSpaceTreeResponse({
    $core.Iterable<Category>? categories,
    $core.Iterable<SpaceTreeNode>? nodes,
    $core.Iterable<VoiceRoom>? voiceRooms,
  }) {
    final result = create();
    if (categories != null) result.categories.addAll(categories);
    if (nodes != null) result.nodes.addAll(nodes);
    if (voiceRooms != null) result.voiceRooms.addAll(voiceRooms);
    return result;
  }

  ListSpaceTreeResponse._();

  factory ListSpaceTreeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListSpaceTreeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListSpaceTreeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..pPM<Category>(1, _omitFieldNames ? '' : 'categories',
        subBuilder: Category.create)
    ..pPM<SpaceTreeNode>(2, _omitFieldNames ? '' : 'nodes',
        subBuilder: SpaceTreeNode.create)
    ..pPM<VoiceRoom>(3, _omitFieldNames ? '' : 'voiceRooms',
        subBuilder: VoiceRoom.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpaceTreeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListSpaceTreeResponse copyWith(
          void Function(ListSpaceTreeResponse) updates) =>
      super.copyWith((message) => updates(message as ListSpaceTreeResponse))
          as ListSpaceTreeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListSpaceTreeResponse create() => ListSpaceTreeResponse._();
  @$core.override
  ListSpaceTreeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListSpaceTreeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListSpaceTreeResponse>(create);
  static ListSpaceTreeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Category> get categories => $_getList(0);

  @$pb.TagNumber(2)
  $pb.PbList<SpaceTreeNode> get nodes => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<VoiceRoom> get voiceRooms => $_getList(2);
}

class CreateInviteResponse extends $pb.GeneratedMessage {
  factory CreateInviteResponse({
    Invite? invite,
  }) {
    final result = create();
    if (invite != null) result.invite = invite;
    return result;
  }

  CreateInviteResponse._();

  factory CreateInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateInviteResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Invite>(1, _omitFieldNames ? '' : 'invite', subBuilder: Invite.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateInviteResponse copyWith(void Function(CreateInviteResponse) updates) =>
      super.copyWith((message) => updates(message as CreateInviteResponse))
          as CreateInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateInviteResponse create() => CreateInviteResponse._();
  @$core.override
  CreateInviteResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateInviteResponse>(create);
  static CreateInviteResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Invite get invite => $_getN(0);
  @$pb.TagNumber(1)
  set invite(Invite value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInvite() => $_has(0);
  @$pb.TagNumber(1)
  void clearInvite() => $_clearField(1);
  @$pb.TagNumber(1)
  Invite ensureInvite() => $_ensure(0);
}

class RevokeInviteResponse extends $pb.GeneratedMessage {
  factory RevokeInviteResponse() => create();

  RevokeInviteResponse._();

  factory RevokeInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeInviteResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeInviteResponse copyWith(void Function(RevokeInviteResponse) updates) =>
      super.copyWith((message) => updates(message as RevokeInviteResponse))
          as RevokeInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeInviteResponse create() => RevokeInviteResponse._();
  @$core.override
  RevokeInviteResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeInviteResponse>(create);
  static RevokeInviteResponse? _defaultInstance;
}

class GetInviteResponse extends $pb.GeneratedMessage {
  factory GetInviteResponse({
    Invite? invite,
  }) {
    final result = create();
    if (invite != null) result.invite = invite;
    return result;
  }

  GetInviteResponse._();

  factory GetInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetInviteResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Invite>(1, _omitFieldNames ? '' : 'invite', subBuilder: Invite.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetInviteResponse copyWith(void Function(GetInviteResponse) updates) =>
      super.copyWith((message) => updates(message as GetInviteResponse))
          as GetInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetInviteResponse create() => GetInviteResponse._();
  @$core.override
  GetInviteResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetInviteResponse>(create);
  static GetInviteResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Invite get invite => $_getN(0);
  @$pb.TagNumber(1)
  set invite(Invite value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInvite() => $_has(0);
  @$pb.TagNumber(1)
  void clearInvite() => $_clearField(1);
  @$pb.TagNumber(1)
  Invite ensureInvite() => $_ensure(0);
}

class ListInvitesResponse extends $pb.GeneratedMessage {
  factory ListInvitesResponse({
    InviteList? inviteList,
  }) {
    final result = create();
    if (inviteList != null) result.inviteList = inviteList;
    return result;
  }

  ListInvitesResponse._();

  factory ListInvitesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListInvitesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListInvitesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<InviteList>(1, _omitFieldNames ? '' : 'inviteList',
        subBuilder: InviteList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInvitesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListInvitesResponse copyWith(void Function(ListInvitesResponse) updates) =>
      super.copyWith((message) => updates(message as ListInvitesResponse))
          as ListInvitesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListInvitesResponse create() => ListInvitesResponse._();
  @$core.override
  ListInvitesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListInvitesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListInvitesResponse>(create);
  static ListInvitesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  InviteList get inviteList => $_getN(0);
  @$pb.TagNumber(1)
  set inviteList(InviteList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInviteList() => $_has(0);
  @$pb.TagNumber(1)
  void clearInviteList() => $_clearField(1);
  @$pb.TagNumber(1)
  InviteList ensureInviteList() => $_ensure(0);
}

class JoinByInviteResponse extends $pb.GeneratedMessage {
  factory JoinByInviteResponse({
    SpaceMembership? spaceMembership,
  }) {
    final result = create();
    if (spaceMembership != null) result.spaceMembership = spaceMembership;
    return result;
  }

  JoinByInviteResponse._();

  factory JoinByInviteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinByInviteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinByInviteResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceMembership>(1, _omitFieldNames ? '' : 'spaceMembership',
        subBuilder: SpaceMembership.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinByInviteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinByInviteResponse copyWith(void Function(JoinByInviteResponse) updates) =>
      super.copyWith((message) => updates(message as JoinByInviteResponse))
          as JoinByInviteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinByInviteResponse create() => JoinByInviteResponse._();
  @$core.override
  JoinByInviteResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinByInviteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinByInviteResponse>(create);
  static JoinByInviteResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceMembership get spaceMembership => $_getN(0);
  @$pb.TagNumber(1)
  set spaceMembership(SpaceMembership value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceMembership() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceMembership() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceMembership ensureSpaceMembership() => $_ensure(0);
}

class JoinSpaceResponse extends $pb.GeneratedMessage {
  factory JoinSpaceResponse({
    SpaceMembership? spaceMembership,
  }) {
    final result = create();
    if (spaceMembership != null) result.spaceMembership = spaceMembership;
    return result;
  }

  JoinSpaceResponse._();

  factory JoinSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory JoinSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'JoinSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceMembership>(1, _omitFieldNames ? '' : 'spaceMembership',
        subBuilder: SpaceMembership.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  JoinSpaceResponse copyWith(void Function(JoinSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as JoinSpaceResponse))
          as JoinSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static JoinSpaceResponse create() => JoinSpaceResponse._();
  @$core.override
  JoinSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static JoinSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<JoinSpaceResponse>(create);
  static JoinSpaceResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceMembership get spaceMembership => $_getN(0);
  @$pb.TagNumber(1)
  set spaceMembership(SpaceMembership value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceMembership() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceMembership() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceMembership ensureSpaceMembership() => $_ensure(0);
}

class LeaveSpaceResponse extends $pb.GeneratedMessage {
  factory LeaveSpaceResponse() => create();

  LeaveSpaceResponse._();

  factory LeaveSpaceResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LeaveSpaceResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LeaveSpaceResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveSpaceResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LeaveSpaceResponse copyWith(void Function(LeaveSpaceResponse) updates) =>
      super.copyWith((message) => updates(message as LeaveSpaceResponse))
          as LeaveSpaceResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LeaveSpaceResponse create() => LeaveSpaceResponse._();
  @$core.override
  LeaveSpaceResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LeaveSpaceResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LeaveSpaceResponse>(create);
  static LeaveSpaceResponse? _defaultInstance;
}

class KickMemberResponse extends $pb.GeneratedMessage {
  factory KickMemberResponse() => create();

  KickMemberResponse._();

  factory KickMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory KickMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'KickMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KickMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  KickMemberResponse copyWith(void Function(KickMemberResponse) updates) =>
      super.copyWith((message) => updates(message as KickMemberResponse))
          as KickMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static KickMemberResponse create() => KickMemberResponse._();
  @$core.override
  KickMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static KickMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<KickMemberResponse>(create);
  static KickMemberResponse? _defaultInstance;
}

class BanMemberResponse extends $pb.GeneratedMessage {
  factory BanMemberResponse() => create();

  BanMemberResponse._();

  factory BanMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanMemberResponse copyWith(void Function(BanMemberResponse) updates) =>
      super.copyWith((message) => updates(message as BanMemberResponse))
          as BanMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanMemberResponse create() => BanMemberResponse._();
  @$core.override
  BanMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BanMemberResponse>(create);
  static BanMemberResponse? _defaultInstance;
}

class UnbanMemberResponse extends $pb.GeneratedMessage {
  factory UnbanMemberResponse() => create();

  UnbanMemberResponse._();

  factory UnbanMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnbanMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnbanMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanMemberResponse copyWith(void Function(UnbanMemberResponse) updates) =>
      super.copyWith((message) => updates(message as UnbanMemberResponse))
          as UnbanMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnbanMemberResponse create() => UnbanMemberResponse._();
  @$core.override
  UnbanMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnbanMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnbanMemberResponse>(create);
  static UnbanMemberResponse? _defaultInstance;
}

class ListMembersResponse extends $pb.GeneratedMessage {
  factory ListMembersResponse({
    SpaceMemberList? spaceMemberList,
  }) {
    final result = create();
    if (spaceMemberList != null) result.spaceMemberList = spaceMemberList;
    return result;
  }

  ListMembersResponse._();

  factory ListMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceMemberList>(1, _omitFieldNames ? '' : 'spaceMemberList',
        subBuilder: SpaceMemberList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListMembersResponse copyWith(void Function(ListMembersResponse) updates) =>
      super.copyWith((message) => updates(message as ListMembersResponse))
          as ListMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListMembersResponse create() => ListMembersResponse._();
  @$core.override
  ListMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListMembersResponse>(create);
  static ListMembersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceMemberList get spaceMemberList => $_getN(0);
  @$pb.TagNumber(1)
  set spaceMemberList(SpaceMemberList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceMemberList() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceMemberList() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceMemberList ensureSpaceMemberList() => $_ensure(0);
}

class ListBansResponse extends $pb.GeneratedMessage {
  factory ListBansResponse({
    BanList? banList,
  }) {
    final result = create();
    if (banList != null) result.banList = banList;
    return result;
  }

  ListBansResponse._();

  factory ListBansResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBansResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBansResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<BanList>(1, _omitFieldNames ? '' : 'banList',
        subBuilder: BanList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBansResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBansResponse copyWith(void Function(ListBansResponse) updates) =>
      super.copyWith((message) => updates(message as ListBansResponse))
          as ListBansResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBansResponse create() => ListBansResponse._();
  @$core.override
  ListBansResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBansResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBansResponse>(create);
  static ListBansResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BanList get banList => $_getN(0);
  @$pb.TagNumber(1)
  set banList(BanList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBanList() => $_has(0);
  @$pb.TagNumber(1)
  void clearBanList() => $_clearField(1);
  @$pb.TagNumber(1)
  BanList ensureBanList() => $_ensure(0);
}

class TimeoutMemberResponse extends $pb.GeneratedMessage {
  factory TimeoutMemberResponse() => create();

  TimeoutMemberResponse._();

  factory TimeoutMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TimeoutMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TimeoutMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TimeoutMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TimeoutMemberResponse copyWith(
          void Function(TimeoutMemberResponse) updates) =>
      super.copyWith((message) => updates(message as TimeoutMemberResponse))
          as TimeoutMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TimeoutMemberResponse create() => TimeoutMemberResponse._();
  @$core.override
  TimeoutMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TimeoutMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TimeoutMemberResponse>(create);
  static TimeoutMemberResponse? _defaultInstance;
}

class RemoveMemberTimeoutResponse extends $pb.GeneratedMessage {
  factory RemoveMemberTimeoutResponse() => create();

  RemoveMemberTimeoutResponse._();

  factory RemoveMemberTimeoutResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveMemberTimeoutResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveMemberTimeoutResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberTimeoutResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveMemberTimeoutResponse copyWith(
          void Function(RemoveMemberTimeoutResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveMemberTimeoutResponse))
          as RemoveMemberTimeoutResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveMemberTimeoutResponse create() =>
      RemoveMemberTimeoutResponse._();
  @$core.override
  RemoveMemberTimeoutResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveMemberTimeoutResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveMemberTimeoutResponse>(create);
  static RemoveMemberTimeoutResponse? _defaultInstance;
}

class TransferOwnershipResponse extends $pb.GeneratedMessage {
  factory TransferOwnershipResponse() => create();

  TransferOwnershipResponse._();

  factory TransferOwnershipResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TransferOwnershipResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TransferOwnershipResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferOwnershipResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TransferOwnershipResponse copyWith(
          void Function(TransferOwnershipResponse) updates) =>
      super.copyWith((message) => updates(message as TransferOwnershipResponse))
          as TransferOwnershipResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TransferOwnershipResponse create() => TransferOwnershipResponse._();
  @$core.override
  TransferOwnershipResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TransferOwnershipResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TransferOwnershipResponse>(create);
  static TransferOwnershipResponse? _defaultInstance;
}

class AddBotMemberResponse extends $pb.GeneratedMessage {
  factory AddBotMemberResponse() => create();

  AddBotMemberResponse._();

  factory AddBotMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddBotMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddBotMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddBotMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddBotMemberResponse copyWith(void Function(AddBotMemberResponse) updates) =>
      super.copyWith((message) => updates(message as AddBotMemberResponse))
          as AddBotMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddBotMemberResponse create() => AddBotMemberResponse._();
  @$core.override
  AddBotMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddBotMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddBotMemberResponse>(create);
  static AddBotMemberResponse? _defaultInstance;
}

class RemoveBotMemberResponse extends $pb.GeneratedMessage {
  factory RemoveBotMemberResponse() => create();

  RemoveBotMemberResponse._();

  factory RemoveBotMemberResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveBotMemberResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveBotMemberResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveBotMemberResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveBotMemberResponse copyWith(
          void Function(RemoveBotMemberResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveBotMemberResponse))
          as RemoveBotMemberResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveBotMemberResponse create() => RemoveBotMemberResponse._();
  @$core.override
  RemoveBotMemberResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveBotMemberResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveBotMemberResponse>(create);
  static RemoveBotMemberResponse? _defaultInstance;
}

class ListTemplatesResponse extends $pb.GeneratedMessage {
  factory ListTemplatesResponse({
    TemplateList? templateList,
  }) {
    final result = create();
    if (templateList != null) result.templateList = templateList;
    return result;
  }

  ListTemplatesResponse._();

  factory ListTemplatesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListTemplatesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListTemplatesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<TemplateList>(1, _omitFieldNames ? '' : 'templateList',
        subBuilder: TemplateList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTemplatesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListTemplatesResponse copyWith(
          void Function(ListTemplatesResponse) updates) =>
      super.copyWith((message) => updates(message as ListTemplatesResponse))
          as ListTemplatesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListTemplatesResponse create() => ListTemplatesResponse._();
  @$core.override
  ListTemplatesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListTemplatesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListTemplatesResponse>(create);
  static ListTemplatesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  TemplateList get templateList => $_getN(0);
  @$pb.TagNumber(1)
  set templateList(TemplateList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasTemplateList() => $_has(0);
  @$pb.TagNumber(1)
  void clearTemplateList() => $_clearField(1);
  @$pb.TagNumber(1)
  TemplateList ensureTemplateList() => $_ensure(0);
}

class CreateFromTemplateResponse extends $pb.GeneratedMessage {
  factory CreateFromTemplateResponse({
    Space? space,
  }) {
    final result = create();
    if (space != null) result.space = space;
    return result;
  }

  CreateFromTemplateResponse._();

  factory CreateFromTemplateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateFromTemplateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateFromTemplateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<Space>(1, _omitFieldNames ? '' : 'space', subBuilder: Space.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFromTemplateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateFromTemplateResponse copyWith(
          void Function(CreateFromTemplateResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CreateFromTemplateResponse))
          as CreateFromTemplateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateFromTemplateResponse create() => CreateFromTemplateResponse._();
  @$core.override
  CreateFromTemplateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateFromTemplateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateFromTemplateResponse>(create);
  static CreateFromTemplateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Space get space => $_getN(0);
  @$pb.TagNumber(1)
  set space(Space value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  Space ensureSpace() => $_ensure(0);
}

class GetAuditLogResponse extends $pb.GeneratedMessage {
  factory GetAuditLogResponse({
    AuditLogList? auditLogList,
  }) {
    final result = create();
    if (auditLogList != null) result.auditLogList = auditLogList;
    return result;
  }

  GetAuditLogResponse._();

  factory GetAuditLogResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAuditLogResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAuditLogResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOM<AuditLogList>(1, _omitFieldNames ? '' : 'auditLogList',
        subBuilder: AuditLogList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAuditLogResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAuditLogResponse copyWith(void Function(GetAuditLogResponse) updates) =>
      super.copyWith((message) => updates(message as GetAuditLogResponse))
          as GetAuditLogResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAuditLogResponse create() => GetAuditLogResponse._();
  @$core.override
  GetAuditLogResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAuditLogResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAuditLogResponse>(create);
  static GetAuditLogResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuditLogList get auditLogList => $_getN(0);
  @$pb.TagNumber(1)
  set auditLogList(AuditLogList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAuditLogList() => $_has(0);
  @$pb.TagNumber(1)
  void clearAuditLogList() => $_clearField(1);
  @$pb.TagNumber(1)
  AuditLogList ensureAuditLogList() => $_ensure(0);
}

class AreCoMembersRequest extends $pb.GeneratedMessage {
  factory AreCoMembersRequest({
    $core.String? profileIdA,
    $core.String? profileIdB,
    $core.Iterable<$core.String>? spaceIds,
  }) {
    final result = create();
    if (profileIdA != null) result.profileIdA = profileIdA;
    if (profileIdB != null) result.profileIdB = profileIdB;
    if (spaceIds != null) result.spaceIds.addAll(spaceIds);
    return result;
  }

  AreCoMembersRequest._();

  factory AreCoMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AreCoMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AreCoMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileIdA')
    ..aOS(2, _omitFieldNames ? '' : 'profileIdB')
    ..pPS(3, _omitFieldNames ? '' : 'spaceIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreCoMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreCoMembersRequest copyWith(void Function(AreCoMembersRequest) updates) =>
      super.copyWith((message) => updates(message as AreCoMembersRequest))
          as AreCoMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AreCoMembersRequest create() => AreCoMembersRequest._();
  @$core.override
  AreCoMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AreCoMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AreCoMembersRequest>(create);
  static AreCoMembersRequest? _defaultInstance;

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

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get spaceIds => $_getList(2);
}

class AreCoMembersResponse extends $pb.GeneratedMessage {
  factory AreCoMembersResponse({
    $core.bool? coMembers,
  }) {
    final result = create();
    if (coMembers != null) result.coMembers = coMembers;
    return result;
  }

  AreCoMembersResponse._();

  factory AreCoMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AreCoMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AreCoMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'coMembers')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreCoMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreCoMembersResponse copyWith(void Function(AreCoMembersResponse) updates) =>
      super.copyWith((message) => updates(message as AreCoMembersResponse))
          as AreCoMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AreCoMembersResponse create() => AreCoMembersResponse._();
  @$core.override
  AreCoMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AreCoMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AreCoMembersResponse>(create);
  static AreCoMembersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get coMembers => $_getBF(0);
  @$pb.TagNumber(1)
  set coMembers($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCoMembers() => $_has(0);
  @$pb.TagNumber(1)
  void clearCoMembers() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
