// This is a generated file - do not edit.
//
// Generated from voice/story/v1/story.proto.

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

import '../../common/v1/common.pb.dart' as $2;
import 'story.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'story.pbenum.dart';

class Story extends $pb.GeneratedMessage {
  factory Story({
    $core.String? id,
    $core.String? authorProfileId,
    $core.String? type,
    $core.String? mediaFileId,
    $core.String? textContent,
    $core.String? textStyleJson,
    $core.String? gameTag,
    $core.bool? isLookingForParty,
    $core.String? lfpCriteriaJson,
    $core.String? mentionProfileIdsJson,
    $core.int? viewCount,
    $core.String? visibility,
    $1.Timestamp? expiresAt,
    $1.Timestamp? archivedUntil,
    $1.Timestamp? createdAt,
    $1.Timestamp? deletedAt,
    StoryMediaType? typeEnum,
    StoryAudience? visibilityEnum,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (type != null) result.type = type;
    if (mediaFileId != null) result.mediaFileId = mediaFileId;
    if (textContent != null) result.textContent = textContent;
    if (textStyleJson != null) result.textStyleJson = textStyleJson;
    if (gameTag != null) result.gameTag = gameTag;
    if (isLookingForParty != null) result.isLookingForParty = isLookingForParty;
    if (lfpCriteriaJson != null) result.lfpCriteriaJson = lfpCriteriaJson;
    if (mentionProfileIdsJson != null)
      result.mentionProfileIdsJson = mentionProfileIdsJson;
    if (viewCount != null) result.viewCount = viewCount;
    if (visibility != null) result.visibility = visibility;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (archivedUntil != null) result.archivedUntil = archivedUntil;
    if (createdAt != null) result.createdAt = createdAt;
    if (deletedAt != null) result.deletedAt = deletedAt;
    if (typeEnum != null) result.typeEnum = typeEnum;
    if (visibilityEnum != null) result.visibilityEnum = visibilityEnum;
    return result;
  }

  Story._();

  factory Story.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Story.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Story',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'authorProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..aOS(4, _omitFieldNames ? '' : 'mediaFileId')
    ..aOS(5, _omitFieldNames ? '' : 'textContent')
    ..aOS(6, _omitFieldNames ? '' : 'textStyleJson')
    ..aOS(7, _omitFieldNames ? '' : 'gameTag')
    ..aOB(8, _omitFieldNames ? '' : 'isLookingForParty')
    ..aOS(9, _omitFieldNames ? '' : 'lfpCriteriaJson')
    ..aOS(10, _omitFieldNames ? '' : 'mentionProfileIdsJson')
    ..aI(11, _omitFieldNames ? '' : 'viewCount')
    ..aOS(12, _omitFieldNames ? '' : 'visibility')
    ..aOM<$1.Timestamp>(13, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(14, _omitFieldNames ? '' : 'archivedUntil',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(15, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(16, _omitFieldNames ? '' : 'deletedAt',
        subBuilder: $1.Timestamp.create)
    ..aE<StoryMediaType>(17, _omitFieldNames ? '' : 'typeEnum',
        enumValues: StoryMediaType.values)
    ..aE<StoryAudience>(18, _omitFieldNames ? '' : 'visibilityEnum',
        enumValues: StoryAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Story clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Story copyWith(void Function(Story) updates) =>
      super.copyWith((message) => updates(message as Story)) as Story;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Story create() => Story._();
  @$core.override
  Story createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Story getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Story>(create);
  static Story? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

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
  $core.String get mediaFileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set mediaFileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasMediaFileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearMediaFileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get textContent => $_getSZ(4);
  @$pb.TagNumber(5)
  set textContent($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTextContent() => $_has(4);
  @$pb.TagNumber(5)
  void clearTextContent() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get textStyleJson => $_getSZ(5);
  @$pb.TagNumber(6)
  set textStyleJson($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasTextStyleJson() => $_has(5);
  @$pb.TagNumber(6)
  void clearTextStyleJson() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get gameTag => $_getSZ(6);
  @$pb.TagNumber(7)
  set gameTag($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasGameTag() => $_has(6);
  @$pb.TagNumber(7)
  void clearGameTag() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get isLookingForParty => $_getBF(7);
  @$pb.TagNumber(8)
  set isLookingForParty($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasIsLookingForParty() => $_has(7);
  @$pb.TagNumber(8)
  void clearIsLookingForParty() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get lfpCriteriaJson => $_getSZ(8);
  @$pb.TagNumber(9)
  set lfpCriteriaJson($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasLfpCriteriaJson() => $_has(8);
  @$pb.TagNumber(9)
  void clearLfpCriteriaJson() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get mentionProfileIdsJson => $_getSZ(9);
  @$pb.TagNumber(10)
  set mentionProfileIdsJson($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasMentionProfileIdsJson() => $_has(9);
  @$pb.TagNumber(10)
  void clearMentionProfileIdsJson() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.int get viewCount => $_getIZ(10);
  @$pb.TagNumber(11)
  set viewCount($core.int value) => $_setSignedInt32(10, value);
  @$pb.TagNumber(11)
  $core.bool hasViewCount() => $_has(10);
  @$pb.TagNumber(11)
  void clearViewCount() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.String get visibility => $_getSZ(11);
  @$pb.TagNumber(12)
  set visibility($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasVisibility() => $_has(11);
  @$pb.TagNumber(12)
  void clearVisibility() => $_clearField(12);

  @$pb.TagNumber(13)
  $1.Timestamp get expiresAt => $_getN(12);
  @$pb.TagNumber(13)
  set expiresAt($1.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasExpiresAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearExpiresAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $1.Timestamp ensureExpiresAt() => $_ensure(12);

  @$pb.TagNumber(14)
  $1.Timestamp get archivedUntil => $_getN(13);
  @$pb.TagNumber(14)
  set archivedUntil($1.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasArchivedUntil() => $_has(13);
  @$pb.TagNumber(14)
  void clearArchivedUntil() => $_clearField(14);
  @$pb.TagNumber(14)
  $1.Timestamp ensureArchivedUntil() => $_ensure(13);

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
  $1.Timestamp get deletedAt => $_getN(15);
  @$pb.TagNumber(16)
  set deletedAt($1.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasDeletedAt() => $_has(15);
  @$pb.TagNumber(16)
  void clearDeletedAt() => $_clearField(16);
  @$pb.TagNumber(16)
  $1.Timestamp ensureDeletedAt() => $_ensure(15);

  @$pb.TagNumber(17)
  StoryMediaType get typeEnum => $_getN(16);
  @$pb.TagNumber(17)
  set typeEnum(StoryMediaType value) => $_setField(17, value);
  @$pb.TagNumber(17)
  $core.bool hasTypeEnum() => $_has(16);
  @$pb.TagNumber(17)
  void clearTypeEnum() => $_clearField(17);

  @$pb.TagNumber(18)
  StoryAudience get visibilityEnum => $_getN(17);
  @$pb.TagNumber(18)
  set visibilityEnum(StoryAudience value) => $_setField(18, value);
  @$pb.TagNumber(18)
  $core.bool hasVisibilityEnum() => $_has(17);
  @$pb.TagNumber(18)
  void clearVisibilityEnum() => $_clearField(18);
}

class StoryRef extends $pb.GeneratedMessage {
  factory StoryRef({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  StoryRef._();

  factory StoryRef.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryRef.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryRef',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryRef clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryRef copyWith(void Function(StoryRef) updates) =>
      super.copyWith((message) => updates(message as StoryRef)) as StoryRef;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryRef create() => StoryRef._();
  @$core.override
  StoryRef createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryRef getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StoryRef>(create);
  static StoryRef? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class CreateStoryRequest extends $pb.GeneratedMessage {
  factory CreateStoryRequest({
    $core.String? type,
    $core.String? mediaFileId,
    $core.String? textContent,
    $core.String? textStyleJson,
    $core.String? gameTag,
    $core.String? visibility,
    StoryMediaType? typeEnum,
    StoryAudience? visibilityEnum,
    $core.Iterable<$core.String>? mentionProfileIds,
  }) {
    final result = create();
    if (type != null) result.type = type;
    if (mediaFileId != null) result.mediaFileId = mediaFileId;
    if (textContent != null) result.textContent = textContent;
    if (textStyleJson != null) result.textStyleJson = textStyleJson;
    if (gameTag != null) result.gameTag = gameTag;
    if (visibility != null) result.visibility = visibility;
    if (typeEnum != null) result.typeEnum = typeEnum;
    if (visibilityEnum != null) result.visibilityEnum = visibilityEnum;
    if (mentionProfileIds != null)
      result.mentionProfileIds.addAll(mentionProfileIds);
    return result;
  }

  CreateStoryRequest._();

  factory CreateStoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateStoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'type')
    ..aOS(2, _omitFieldNames ? '' : 'mediaFileId')
    ..aOS(3, _omitFieldNames ? '' : 'textContent')
    ..aOS(4, _omitFieldNames ? '' : 'textStyleJson')
    ..aOS(5, _omitFieldNames ? '' : 'gameTag')
    ..aOS(6, _omitFieldNames ? '' : 'visibility')
    ..aE<StoryMediaType>(7, _omitFieldNames ? '' : 'typeEnum',
        enumValues: StoryMediaType.values)
    ..aE<StoryAudience>(8, _omitFieldNames ? '' : 'visibilityEnum',
        enumValues: StoryAudience.values)
    ..pPS(9, _omitFieldNames ? '' : 'mentionProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoryRequest copyWith(void Function(CreateStoryRequest) updates) =>
      super.copyWith((message) => updates(message as CreateStoryRequest))
          as CreateStoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateStoryRequest create() => CreateStoryRequest._();
  @$core.override
  CreateStoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateStoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStoryRequest>(create);
  static CreateStoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get type => $_getSZ(0);
  @$pb.TagNumber(1)
  set type($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasType() => $_has(0);
  @$pb.TagNumber(1)
  void clearType() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mediaFileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaFileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaFileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaFileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get textContent => $_getSZ(2);
  @$pb.TagNumber(3)
  set textContent($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTextContent() => $_has(2);
  @$pb.TagNumber(3)
  void clearTextContent() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get textStyleJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set textStyleJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTextStyleJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearTextStyleJson() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get gameTag => $_getSZ(4);
  @$pb.TagNumber(5)
  set gameTag($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasGameTag() => $_has(4);
  @$pb.TagNumber(5)
  void clearGameTag() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get visibility => $_getSZ(5);
  @$pb.TagNumber(6)
  set visibility($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVisibility() => $_has(5);
  @$pb.TagNumber(6)
  void clearVisibility() => $_clearField(6);

  @$pb.TagNumber(7)
  StoryMediaType get typeEnum => $_getN(6);
  @$pb.TagNumber(7)
  set typeEnum(StoryMediaType value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasTypeEnum() => $_has(6);
  @$pb.TagNumber(7)
  void clearTypeEnum() => $_clearField(7);

  @$pb.TagNumber(8)
  StoryAudience get visibilityEnum => $_getN(7);
  @$pb.TagNumber(8)
  set visibilityEnum(StoryAudience value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasVisibilityEnum() => $_has(7);
  @$pb.TagNumber(8)
  void clearVisibilityEnum() => $_clearField(8);

  @$pb.TagNumber(9)
  $pb.PbList<$core.String> get mentionProfileIds => $_getList(8);
}

class DeleteStoryRequest extends $pb.GeneratedMessage {
  factory DeleteStoryRequest({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  DeleteStoryRequest._();

  factory DeleteStoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteStoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteStoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteStoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteStoryRequest copyWith(void Function(DeleteStoryRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteStoryRequest))
          as DeleteStoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteStoryRequest create() => DeleteStoryRequest._();
  @$core.override
  DeleteStoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteStoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteStoryRequest>(create);
  static DeleteStoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class GetStoryRequest extends $pb.GeneratedMessage {
  factory GetStoryRequest({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  GetStoryRequest._();

  factory GetStoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryRequest copyWith(void Function(GetStoryRequest) updates) =>
      super.copyWith((message) => updates(message as GetStoryRequest))
          as GetStoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryRequest create() => GetStoryRequest._();
  @$core.override
  GetStoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryRequest>(create);
  static GetStoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class GetStoryFeedRequest extends $pb.GeneratedMessage {
  factory GetStoryFeedRequest({
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  GetStoryFeedRequest._();

  factory GetStoryFeedRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryFeedRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryFeedRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<$2.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryFeedRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryFeedRequest copyWith(void Function(GetStoryFeedRequest) updates) =>
      super.copyWith((message) => updates(message as GetStoryFeedRequest))
          as GetStoryFeedRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryFeedRequest create() => GetStoryFeedRequest._();
  @$core.override
  GetStoryFeedRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryFeedRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryFeedRequest>(create);
  static GetStoryFeedRequest? _defaultInstance;

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

class GetProfileStoriesRequest extends $pb.GeneratedMessage {
  factory GetProfileStoriesRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetProfileStoriesRequest._();

  factory GetProfileStoriesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfileStoriesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileStoriesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileStoriesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileStoriesRequest copyWith(
          void Function(GetProfileStoriesRequest) updates) =>
      super.copyWith((message) => updates(message as GetProfileStoriesRequest))
          as GetProfileStoriesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfileStoriesRequest create() => GetProfileStoriesRequest._();
  @$core.override
  GetProfileStoriesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfileStoriesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfileStoriesRequest>(create);
  static GetProfileStoriesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class StoryList extends $pb.GeneratedMessage {
  factory StoryList({
    $core.Iterable<Story>? stories,
  }) {
    final result = create();
    if (stories != null) result.stories.addAll(stories);
    return result;
  }

  StoryList._();

  factory StoryList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..pPM<Story>(1, _omitFieldNames ? '' : 'stories', subBuilder: Story.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryList copyWith(void Function(StoryList) updates) =>
      super.copyWith((message) => updates(message as StoryList)) as StoryList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryList create() => StoryList._();
  @$core.override
  StoryList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<StoryList>(create);
  static StoryList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Story> get stories => $_getList(0);
}

class MarkViewedRequest extends $pb.GeneratedMessage {
  factory MarkViewedRequest({
    $core.String? storyId,
    $core.bool? anonymous,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (anonymous != null) result.anonymous = anonymous;
    return result;
  }

  MarkViewedRequest._();

  factory MarkViewedRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MarkViewedRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkViewedRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOB(2, _omitFieldNames ? '' : 'anonymous')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkViewedRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkViewedRequest copyWith(void Function(MarkViewedRequest) updates) =>
      super.copyWith((message) => updates(message as MarkViewedRequest))
          as MarkViewedRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MarkViewedRequest create() => MarkViewedRequest._();
  @$core.override
  MarkViewedRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MarkViewedRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkViewedRequest>(create);
  static MarkViewedRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get anonymous => $_getBF(1);
  @$pb.TagNumber(2)
  set anonymous($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAnonymous() => $_has(1);
  @$pb.TagNumber(2)
  void clearAnonymous() => $_clearField(2);
}

class GetViewersRequest extends $pb.GeneratedMessage {
  factory GetViewersRequest({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  GetViewersRequest._();

  factory GetViewersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetViewersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetViewersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetViewersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetViewersRequest copyWith(void Function(GetViewersRequest) updates) =>
      super.copyWith((message) => updates(message as GetViewersRequest))
          as GetViewersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetViewersRequest create() => GetViewersRequest._();
  @$core.override
  GetViewersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetViewersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetViewersRequest>(create);
  static GetViewersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class ViewerList extends $pb.GeneratedMessage {
  factory ViewerList({
    $core.Iterable<$core.String>? viewerProfileIds,
  }) {
    final result = create();
    if (viewerProfileIds != null)
      result.viewerProfileIds.addAll(viewerProfileIds);
    return result;
  }

  ViewerList._();

  factory ViewerList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ViewerList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ViewerList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'viewerProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ViewerList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ViewerList copyWith(void Function(ViewerList) updates) =>
      super.copyWith((message) => updates(message as ViewerList)) as ViewerList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ViewerList create() => ViewerList._();
  @$core.override
  ViewerList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ViewerList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ViewerList>(create);
  static ViewerList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get viewerProfileIds => $_getList(0);
}

class ReactToStoryRequest extends $pb.GeneratedMessage {
  factory ReactToStoryRequest({
    $core.String? storyId,
    $core.String? emoji,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (emoji != null) result.emoji = emoji;
    return result;
  }

  ReactToStoryRequest._();

  factory ReactToStoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReactToStoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReactToStoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'emoji')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactToStoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactToStoryRequest copyWith(void Function(ReactToStoryRequest) updates) =>
      super.copyWith((message) => updates(message as ReactToStoryRequest))
          as ReactToStoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReactToStoryRequest create() => ReactToStoryRequest._();
  @$core.override
  ReactToStoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReactToStoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReactToStoryRequest>(create);
  static ReactToStoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get emoji => $_getSZ(1);
  @$pb.TagNumber(2)
  set emoji($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEmoji() => $_has(1);
  @$pb.TagNumber(2)
  void clearEmoji() => $_clearField(2);
}

class StoryReaction extends $pb.GeneratedMessage {
  factory StoryReaction({
    $core.String? reactorProfileId,
    $core.String? emoji,
  }) {
    final result = create();
    if (reactorProfileId != null) result.reactorProfileId = reactorProfileId;
    if (emoji != null) result.emoji = emoji;
    return result;
  }

  StoryReaction._();

  factory StoryReaction.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryReaction.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryReaction',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reactorProfileId')
    ..aOS(2, _omitFieldNames ? '' : 'emoji')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryReaction clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryReaction copyWith(void Function(StoryReaction) updates) =>
      super.copyWith((message) => updates(message as StoryReaction))
          as StoryReaction;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryReaction create() => StoryReaction._();
  @$core.override
  StoryReaction createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryReaction getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryReaction>(create);
  static StoryReaction? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reactorProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reactorProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReactorProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReactorProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get emoji => $_getSZ(1);
  @$pb.TagNumber(2)
  set emoji($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEmoji() => $_has(1);
  @$pb.TagNumber(2)
  void clearEmoji() => $_clearField(2);
}

class GetStoryReactionsRequest extends $pb.GeneratedMessage {
  factory GetStoryReactionsRequest({
    $core.String? storyId,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  GetStoryReactionsRequest._();

  factory GetStoryReactionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryReactionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryReactionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryReactionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryReactionsRequest copyWith(
          void Function(GetStoryReactionsRequest) updates) =>
      super.copyWith((message) => updates(message as GetStoryReactionsRequest))
          as GetStoryReactionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryReactionsRequest create() => GetStoryReactionsRequest._();
  @$core.override
  GetStoryReactionsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryReactionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryReactionsRequest>(create);
  static GetStoryReactionsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);
}

class ReplyToStoryRequest extends $pb.GeneratedMessage {
  factory ReplyToStoryRequest({
    $core.String? storyId,
    $core.String? text,
  }) {
    final result = create();
    if (storyId != null) result.storyId = storyId;
    if (text != null) result.text = text;
    return result;
  }

  ReplyToStoryRequest._();

  factory ReplyToStoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReplyToStoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReplyToStoryRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'storyId')
    ..aOS(2, _omitFieldNames ? '' : 'text')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyToStoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyToStoryRequest copyWith(void Function(ReplyToStoryRequest) updates) =>
      super.copyWith((message) => updates(message as ReplyToStoryRequest))
          as ReplyToStoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReplyToStoryRequest create() => ReplyToStoryRequest._();
  @$core.override
  ReplyToStoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReplyToStoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReplyToStoryRequest>(create);
  static ReplyToStoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get storyId => $_getSZ(0);
  @$pb.TagNumber(1)
  set storyId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryId() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get text => $_getSZ(1);
  @$pb.TagNumber(2)
  set text($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasText() => $_has(1);
  @$pb.TagNumber(2)
  void clearText() => $_clearField(2);
}

class GetArchiveRequest extends $pb.GeneratedMessage {
  factory GetArchiveRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetArchiveRequest._();

  factory GetArchiveRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetArchiveRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetArchiveRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetArchiveRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetArchiveRequest copyWith(void Function(GetArchiveRequest) updates) =>
      super.copyWith((message) => updates(message as GetArchiveRequest))
          as GetArchiveRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetArchiveRequest create() => GetArchiveRequest._();
  @$core.override
  GetArchiveRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetArchiveRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetArchiveRequest>(create);
  static GetArchiveRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class Highlight extends $pb.GeneratedMessage {
  factory Highlight({
    $core.String? id,
    $core.String? profileId,
    $core.String? name,
    $core.Iterable<$core.String>? storyIds,
    $core.String? visibility,
    StoryAudience? visibilityEnum,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (profileId != null) result.profileId = profileId;
    if (name != null) result.name = name;
    if (storyIds != null) result.storyIds.addAll(storyIds);
    if (visibility != null) result.visibility = visibility;
    if (visibilityEnum != null) result.visibilityEnum = visibilityEnum;
    return result;
  }

  Highlight._();

  factory Highlight.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Highlight.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Highlight',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..pPS(4, _omitFieldNames ? '' : 'storyIds')
    ..aOS(5, _omitFieldNames ? '' : 'visibility')
    ..aE<StoryAudience>(6, _omitFieldNames ? '' : 'visibilityEnum',
        enumValues: StoryAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Highlight clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Highlight copyWith(void Function(Highlight) updates) =>
      super.copyWith((message) => updates(message as Highlight)) as Highlight;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Highlight create() => Highlight._();
  @$core.override
  Highlight createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Highlight getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Highlight>(create);
  static Highlight? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get storyIds => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get visibility => $_getSZ(4);
  @$pb.TagNumber(5)
  set visibility($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVisibility() => $_has(4);
  @$pb.TagNumber(5)
  void clearVisibility() => $_clearField(5);

  @$pb.TagNumber(6)
  StoryAudience get visibilityEnum => $_getN(5);
  @$pb.TagNumber(6)
  set visibilityEnum(StoryAudience value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasVisibilityEnum() => $_has(5);
  @$pb.TagNumber(6)
  void clearVisibilityEnum() => $_clearField(6);
}

class CreateHighlightRequest extends $pb.GeneratedMessage {
  factory CreateHighlightRequest({
    $core.String? name,
    $core.String? visibility,
    StoryAudience? visibilityEnum,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (visibility != null) result.visibility = visibility;
    if (visibilityEnum != null) result.visibilityEnum = visibilityEnum;
    return result;
  }

  CreateHighlightRequest._();

  factory CreateHighlightRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateHighlightRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateHighlightRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'visibility')
    ..aE<StoryAudience>(3, _omitFieldNames ? '' : 'visibilityEnum',
        enumValues: StoryAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateHighlightRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateHighlightRequest copyWith(
          void Function(CreateHighlightRequest) updates) =>
      super.copyWith((message) => updates(message as CreateHighlightRequest))
          as CreateHighlightRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateHighlightRequest create() => CreateHighlightRequest._();
  @$core.override
  CreateHighlightRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateHighlightRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateHighlightRequest>(create);
  static CreateHighlightRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get visibility => $_getSZ(1);
  @$pb.TagNumber(2)
  set visibility($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVisibility() => $_has(1);
  @$pb.TagNumber(2)
  void clearVisibility() => $_clearField(2);

  @$pb.TagNumber(3)
  StoryAudience get visibilityEnum => $_getN(2);
  @$pb.TagNumber(3)
  set visibilityEnum(StoryAudience value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasVisibilityEnum() => $_has(2);
  @$pb.TagNumber(3)
  void clearVisibilityEnum() => $_clearField(3);
}

class UpdateHighlightRequest extends $pb.GeneratedMessage {
  factory UpdateHighlightRequest({
    $core.String? highlightId,
    $core.String? name,
    $core.String? visibility,
    StoryAudience? visibilityEnum,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    if (name != null) result.name = name;
    if (visibility != null) result.visibility = visibility;
    if (visibilityEnum != null) result.visibilityEnum = visibilityEnum;
    return result;
  }

  UpdateHighlightRequest._();

  factory UpdateHighlightRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateHighlightRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateHighlightRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'visibility')
    ..aE<StoryAudience>(4, _omitFieldNames ? '' : 'visibilityEnum',
        enumValues: StoryAudience.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateHighlightRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateHighlightRequest copyWith(
          void Function(UpdateHighlightRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateHighlightRequest))
          as UpdateHighlightRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateHighlightRequest create() => UpdateHighlightRequest._();
  @$core.override
  UpdateHighlightRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateHighlightRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateHighlightRequest>(create);
  static UpdateHighlightRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get visibility => $_getSZ(2);
  @$pb.TagNumber(3)
  set visibility($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasVisibility() => $_has(2);
  @$pb.TagNumber(3)
  void clearVisibility() => $_clearField(3);

  @$pb.TagNumber(4)
  StoryAudience get visibilityEnum => $_getN(3);
  @$pb.TagNumber(4)
  set visibilityEnum(StoryAudience value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasVisibilityEnum() => $_has(3);
  @$pb.TagNumber(4)
  void clearVisibilityEnum() => $_clearField(4);
}

class DeleteHighlightRequest extends $pb.GeneratedMessage {
  factory DeleteHighlightRequest({
    $core.String? highlightId,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    return result;
  }

  DeleteHighlightRequest._();

  factory DeleteHighlightRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteHighlightRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteHighlightRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteHighlightRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteHighlightRequest copyWith(
          void Function(DeleteHighlightRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteHighlightRequest))
          as DeleteHighlightRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteHighlightRequest create() => DeleteHighlightRequest._();
  @$core.override
  DeleteHighlightRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteHighlightRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteHighlightRequest>(create);
  static DeleteHighlightRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);
}

class AddToHighlightRequest extends $pb.GeneratedMessage {
  factory AddToHighlightRequest({
    $core.String? highlightId,
    $core.String? storyId,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  AddToHighlightRequest._();

  factory AddToHighlightRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddToHighlightRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddToHighlightRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..aOS(2, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddToHighlightRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddToHighlightRequest copyWith(
          void Function(AddToHighlightRequest) updates) =>
      super.copyWith((message) => updates(message as AddToHighlightRequest))
          as AddToHighlightRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddToHighlightRequest create() => AddToHighlightRequest._();
  @$core.override
  AddToHighlightRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddToHighlightRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddToHighlightRequest>(create);
  static AddToHighlightRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get storyId => $_getSZ(1);
  @$pb.TagNumber(2)
  set storyId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoryId() => $_clearField(2);
}

class RemoveFromHighlightRequest extends $pb.GeneratedMessage {
  factory RemoveFromHighlightRequest({
    $core.String? highlightId,
    $core.String? storyId,
  }) {
    final result = create();
    if (highlightId != null) result.highlightId = highlightId;
    if (storyId != null) result.storyId = storyId;
    return result;
  }

  RemoveFromHighlightRequest._();

  factory RemoveFromHighlightRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveFromHighlightRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFromHighlightRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'highlightId')
    ..aOS(2, _omitFieldNames ? '' : 'storyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFromHighlightRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFromHighlightRequest copyWith(
          void Function(RemoveFromHighlightRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveFromHighlightRequest))
          as RemoveFromHighlightRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveFromHighlightRequest create() => RemoveFromHighlightRequest._();
  @$core.override
  RemoveFromHighlightRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveFromHighlightRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFromHighlightRequest>(create);
  static RemoveFromHighlightRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get highlightId => $_getSZ(0);
  @$pb.TagNumber(1)
  set highlightId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightId() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get storyId => $_getSZ(1);
  @$pb.TagNumber(2)
  set storyId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStoryId() => $_has(1);
  @$pb.TagNumber(2)
  void clearStoryId() => $_clearField(2);
}

class GetHighlightsRequest extends $pb.GeneratedMessage {
  factory GetHighlightsRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetHighlightsRequest._();

  factory GetHighlightsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetHighlightsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetHighlightsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHighlightsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHighlightsRequest copyWith(void Function(GetHighlightsRequest) updates) =>
      super.copyWith((message) => updates(message as GetHighlightsRequest))
          as GetHighlightsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetHighlightsRequest create() => GetHighlightsRequest._();
  @$core.override
  GetHighlightsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetHighlightsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetHighlightsRequest>(create);
  static GetHighlightsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class HighlightList extends $pb.GeneratedMessage {
  factory HighlightList({
    $core.Iterable<Highlight>? highlights,
  }) {
    final result = create();
    if (highlights != null) result.highlights.addAll(highlights);
    return result;
  }

  HighlightList._();

  factory HighlightList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HighlightList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HighlightList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..pPM<Highlight>(1, _omitFieldNames ? '' : 'highlights',
        subBuilder: Highlight.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HighlightList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HighlightList copyWith(void Function(HighlightList) updates) =>
      super.copyWith((message) => updates(message as HighlightList))
          as HighlightList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HighlightList create() => HighlightList._();
  @$core.override
  HighlightList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HighlightList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HighlightList>(create);
  static HighlightList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Highlight> get highlights => $_getList(0);
}

class CreateLookingForPartyRequest extends $pb.GeneratedMessage {
  factory CreateLookingForPartyRequest({
    $core.String? criteriaJson,
    $core.String? mediaFileId,
  }) {
    final result = create();
    if (criteriaJson != null) result.criteriaJson = criteriaJson;
    if (mediaFileId != null) result.mediaFileId = mediaFileId;
    return result;
  }

  CreateLookingForPartyRequest._();

  factory CreateLookingForPartyRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateLookingForPartyRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateLookingForPartyRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'criteriaJson')
    ..aOS(2, _omitFieldNames ? '' : 'mediaFileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateLookingForPartyRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateLookingForPartyRequest copyWith(
          void Function(CreateLookingForPartyRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CreateLookingForPartyRequest))
          as CreateLookingForPartyRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateLookingForPartyRequest create() =>
      CreateLookingForPartyRequest._();
  @$core.override
  CreateLookingForPartyRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateLookingForPartyRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateLookingForPartyRequest>(create);
  static CreateLookingForPartyRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get criteriaJson => $_getSZ(0);
  @$pb.TagNumber(1)
  set criteriaJson($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCriteriaJson() => $_has(0);
  @$pb.TagNumber(1)
  void clearCriteriaJson() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mediaFileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set mediaFileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMediaFileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMediaFileId() => $_clearField(2);
}

class CreateStoryResponse extends $pb.GeneratedMessage {
  factory CreateStoryResponse({
    Story? story,
  }) {
    final result = create();
    if (story != null) result.story = story;
    return result;
  }

  CreateStoryResponse._();

  factory CreateStoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateStoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateStoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<Story>(1, _omitFieldNames ? '' : 'story', subBuilder: Story.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateStoryResponse copyWith(void Function(CreateStoryResponse) updates) =>
      super.copyWith((message) => updates(message as CreateStoryResponse))
          as CreateStoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateStoryResponse create() => CreateStoryResponse._();
  @$core.override
  CreateStoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateStoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateStoryResponse>(create);
  static CreateStoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Story get story => $_getN(0);
  @$pb.TagNumber(1)
  set story(Story value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStory() => $_has(0);
  @$pb.TagNumber(1)
  void clearStory() => $_clearField(1);
  @$pb.TagNumber(1)
  Story ensureStory() => $_ensure(0);
}

class DeleteStoryResponse extends $pb.GeneratedMessage {
  factory DeleteStoryResponse() => create();

  DeleteStoryResponse._();

  factory DeleteStoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteStoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteStoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteStoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteStoryResponse copyWith(void Function(DeleteStoryResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteStoryResponse))
          as DeleteStoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteStoryResponse create() => DeleteStoryResponse._();
  @$core.override
  DeleteStoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteStoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteStoryResponse>(create);
  static DeleteStoryResponse? _defaultInstance;
}

class GetStoryResponse extends $pb.GeneratedMessage {
  factory GetStoryResponse({
    Story? story,
  }) {
    final result = create();
    if (story != null) result.story = story;
    return result;
  }

  GetStoryResponse._();

  factory GetStoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<Story>(1, _omitFieldNames ? '' : 'story', subBuilder: Story.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryResponse copyWith(void Function(GetStoryResponse) updates) =>
      super.copyWith((message) => updates(message as GetStoryResponse))
          as GetStoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryResponse create() => GetStoryResponse._();
  @$core.override
  GetStoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryResponse>(create);
  static GetStoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Story get story => $_getN(0);
  @$pb.TagNumber(1)
  set story(Story value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStory() => $_has(0);
  @$pb.TagNumber(1)
  void clearStory() => $_clearField(1);
  @$pb.TagNumber(1)
  Story ensureStory() => $_ensure(0);
}

class StoryFeedGroup extends $pb.GeneratedMessage {
  factory StoryFeedGroup({
    $core.String? authorProfileId,
    $core.Iterable<Story>? stories,
  }) {
    final result = create();
    if (authorProfileId != null) result.authorProfileId = authorProfileId;
    if (stories != null) result.stories.addAll(stories);
    return result;
  }

  StoryFeedGroup._();

  factory StoryFeedGroup.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StoryFeedGroup.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StoryFeedGroup',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'authorProfileId')
    ..pPM<Story>(2, _omitFieldNames ? '' : 'stories', subBuilder: Story.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryFeedGroup clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StoryFeedGroup copyWith(void Function(StoryFeedGroup) updates) =>
      super.copyWith((message) => updates(message as StoryFeedGroup))
          as StoryFeedGroup;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StoryFeedGroup create() => StoryFeedGroup._();
  @$core.override
  StoryFeedGroup createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StoryFeedGroup getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StoryFeedGroup>(create);
  static StoryFeedGroup? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get authorProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set authorProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAuthorProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAuthorProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<Story> get stories => $_getList(1);
}

class GetStoryFeedResponse extends $pb.GeneratedMessage {
  factory GetStoryFeedResponse({
    $core.Iterable<Story>? stories,
    $core.String? nextCursor,
    $2.CursorPageResponse? page,
    $core.Iterable<StoryFeedGroup>? feedGroups,
  }) {
    final result = create();
    if (stories != null) result.stories.addAll(stories);
    if (nextCursor != null) result.nextCursor = nextCursor;
    if (page != null) result.page = page;
    if (feedGroups != null) result.feedGroups.addAll(feedGroups);
    return result;
  }

  GetStoryFeedResponse._();

  factory GetStoryFeedResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryFeedResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryFeedResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..pPM<Story>(1, _omitFieldNames ? '' : 'stories', subBuilder: Story.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..aOM<$2.CursorPageResponse>(3, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageResponse.create)
    ..pPM<StoryFeedGroup>(4, _omitFieldNames ? '' : 'feedGroups',
        subBuilder: StoryFeedGroup.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryFeedResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryFeedResponse copyWith(void Function(GetStoryFeedResponse) updates) =>
      super.copyWith((message) => updates(message as GetStoryFeedResponse))
          as GetStoryFeedResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryFeedResponse create() => GetStoryFeedResponse._();
  @$core.override
  GetStoryFeedResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryFeedResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryFeedResponse>(create);
  static GetStoryFeedResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Story> get stories => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.CursorPageResponse get page => $_getN(2);
  @$pb.TagNumber(3)
  set page($2.CursorPageResponse value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPage() => $_has(2);
  @$pb.TagNumber(3)
  void clearPage() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.CursorPageResponse ensurePage() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbList<StoryFeedGroup> get feedGroups => $_getList(3);
}

class GetProfileStoriesResponse extends $pb.GeneratedMessage {
  factory GetProfileStoriesResponse({
    StoryList? storyList,
  }) {
    final result = create();
    if (storyList != null) result.storyList = storyList;
    return result;
  }

  GetProfileStoriesResponse._();

  factory GetProfileStoriesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetProfileStoriesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetProfileStoriesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<StoryList>(1, _omitFieldNames ? '' : 'storyList',
        subBuilder: StoryList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileStoriesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetProfileStoriesResponse copyWith(
          void Function(GetProfileStoriesResponse) updates) =>
      super.copyWith((message) => updates(message as GetProfileStoriesResponse))
          as GetProfileStoriesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetProfileStoriesResponse create() => GetProfileStoriesResponse._();
  @$core.override
  GetProfileStoriesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetProfileStoriesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetProfileStoriesResponse>(create);
  static GetProfileStoriesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  StoryList get storyList => $_getN(0);
  @$pb.TagNumber(1)
  set storyList(StoryList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryList() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryList() => $_clearField(1);
  @$pb.TagNumber(1)
  StoryList ensureStoryList() => $_ensure(0);
}

class MarkViewedResponse extends $pb.GeneratedMessage {
  factory MarkViewedResponse() => create();

  MarkViewedResponse._();

  factory MarkViewedResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MarkViewedResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MarkViewedResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkViewedResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MarkViewedResponse copyWith(void Function(MarkViewedResponse) updates) =>
      super.copyWith((message) => updates(message as MarkViewedResponse))
          as MarkViewedResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MarkViewedResponse create() => MarkViewedResponse._();
  @$core.override
  MarkViewedResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MarkViewedResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MarkViewedResponse>(create);
  static MarkViewedResponse? _defaultInstance;
}

class GetViewersResponse extends $pb.GeneratedMessage {
  factory GetViewersResponse({
    ViewerList? viewerList,
  }) {
    final result = create();
    if (viewerList != null) result.viewerList = viewerList;
    return result;
  }

  GetViewersResponse._();

  factory GetViewersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetViewersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetViewersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<ViewerList>(1, _omitFieldNames ? '' : 'viewerList',
        subBuilder: ViewerList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetViewersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetViewersResponse copyWith(void Function(GetViewersResponse) updates) =>
      super.copyWith((message) => updates(message as GetViewersResponse))
          as GetViewersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetViewersResponse create() => GetViewersResponse._();
  @$core.override
  GetViewersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetViewersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetViewersResponse>(create);
  static GetViewersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ViewerList get viewerList => $_getN(0);
  @$pb.TagNumber(1)
  set viewerList(ViewerList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasViewerList() => $_has(0);
  @$pb.TagNumber(1)
  void clearViewerList() => $_clearField(1);
  @$pb.TagNumber(1)
  ViewerList ensureViewerList() => $_ensure(0);
}

class ReactToStoryResponse extends $pb.GeneratedMessage {
  factory ReactToStoryResponse() => create();

  ReactToStoryResponse._();

  factory ReactToStoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReactToStoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReactToStoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactToStoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReactToStoryResponse copyWith(void Function(ReactToStoryResponse) updates) =>
      super.copyWith((message) => updates(message as ReactToStoryResponse))
          as ReactToStoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReactToStoryResponse create() => ReactToStoryResponse._();
  @$core.override
  ReactToStoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReactToStoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReactToStoryResponse>(create);
  static ReactToStoryResponse? _defaultInstance;
}

class GetStoryReactionsResponse extends $pb.GeneratedMessage {
  factory GetStoryReactionsResponse({
    $core.Iterable<StoryReaction>? reactions,
  }) {
    final result = create();
    if (reactions != null) result.reactions.addAll(reactions);
    return result;
  }

  GetStoryReactionsResponse._();

  factory GetStoryReactionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetStoryReactionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetStoryReactionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..pPM<StoryReaction>(1, _omitFieldNames ? '' : 'reactions',
        subBuilder: StoryReaction.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryReactionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetStoryReactionsResponse copyWith(
          void Function(GetStoryReactionsResponse) updates) =>
      super.copyWith((message) => updates(message as GetStoryReactionsResponse))
          as GetStoryReactionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetStoryReactionsResponse create() => GetStoryReactionsResponse._();
  @$core.override
  GetStoryReactionsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetStoryReactionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetStoryReactionsResponse>(create);
  static GetStoryReactionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<StoryReaction> get reactions => $_getList(0);
}

class ReplyToStoryResponse extends $pb.GeneratedMessage {
  factory ReplyToStoryResponse({
    $core.String? chatId,
    $core.String? messageId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (messageId != null) result.messageId = messageId;
    return result;
  }

  ReplyToStoryResponse._();

  factory ReplyToStoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReplyToStoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReplyToStoryResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOS(2, _omitFieldNames ? '' : 'messageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyToStoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReplyToStoryResponse copyWith(void Function(ReplyToStoryResponse) updates) =>
      super.copyWith((message) => updates(message as ReplyToStoryResponse))
          as ReplyToStoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReplyToStoryResponse create() => ReplyToStoryResponse._();
  @$core.override
  ReplyToStoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReplyToStoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReplyToStoryResponse>(create);
  static ReplyToStoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get messageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set messageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessageId() => $_clearField(2);
}

class GetArchiveResponse extends $pb.GeneratedMessage {
  factory GetArchiveResponse({
    StoryList? storyList,
  }) {
    final result = create();
    if (storyList != null) result.storyList = storyList;
    return result;
  }

  GetArchiveResponse._();

  factory GetArchiveResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetArchiveResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetArchiveResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<StoryList>(1, _omitFieldNames ? '' : 'storyList',
        subBuilder: StoryList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetArchiveResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetArchiveResponse copyWith(void Function(GetArchiveResponse) updates) =>
      super.copyWith((message) => updates(message as GetArchiveResponse))
          as GetArchiveResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetArchiveResponse create() => GetArchiveResponse._();
  @$core.override
  GetArchiveResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetArchiveResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetArchiveResponse>(create);
  static GetArchiveResponse? _defaultInstance;

  @$pb.TagNumber(1)
  StoryList get storyList => $_getN(0);
  @$pb.TagNumber(1)
  set storyList(StoryList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStoryList() => $_has(0);
  @$pb.TagNumber(1)
  void clearStoryList() => $_clearField(1);
  @$pb.TagNumber(1)
  StoryList ensureStoryList() => $_ensure(0);
}

class CreateHighlightResponse extends $pb.GeneratedMessage {
  factory CreateHighlightResponse({
    Highlight? highlight,
  }) {
    final result = create();
    if (highlight != null) result.highlight = highlight;
    return result;
  }

  CreateHighlightResponse._();

  factory CreateHighlightResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateHighlightResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateHighlightResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<Highlight>(1, _omitFieldNames ? '' : 'highlight',
        subBuilder: Highlight.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateHighlightResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateHighlightResponse copyWith(
          void Function(CreateHighlightResponse) updates) =>
      super.copyWith((message) => updates(message as CreateHighlightResponse))
          as CreateHighlightResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateHighlightResponse create() => CreateHighlightResponse._();
  @$core.override
  CreateHighlightResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateHighlightResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateHighlightResponse>(create);
  static CreateHighlightResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Highlight get highlight => $_getN(0);
  @$pb.TagNumber(1)
  set highlight(Highlight value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlight() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlight() => $_clearField(1);
  @$pb.TagNumber(1)
  Highlight ensureHighlight() => $_ensure(0);
}

class UpdateHighlightResponse extends $pb.GeneratedMessage {
  factory UpdateHighlightResponse({
    Highlight? highlight,
  }) {
    final result = create();
    if (highlight != null) result.highlight = highlight;
    return result;
  }

  UpdateHighlightResponse._();

  factory UpdateHighlightResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateHighlightResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateHighlightResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<Highlight>(1, _omitFieldNames ? '' : 'highlight',
        subBuilder: Highlight.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateHighlightResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateHighlightResponse copyWith(
          void Function(UpdateHighlightResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateHighlightResponse))
          as UpdateHighlightResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateHighlightResponse create() => UpdateHighlightResponse._();
  @$core.override
  UpdateHighlightResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateHighlightResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateHighlightResponse>(create);
  static UpdateHighlightResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Highlight get highlight => $_getN(0);
  @$pb.TagNumber(1)
  set highlight(Highlight value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlight() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlight() => $_clearField(1);
  @$pb.TagNumber(1)
  Highlight ensureHighlight() => $_ensure(0);
}

class DeleteHighlightResponse extends $pb.GeneratedMessage {
  factory DeleteHighlightResponse() => create();

  DeleteHighlightResponse._();

  factory DeleteHighlightResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteHighlightResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteHighlightResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteHighlightResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteHighlightResponse copyWith(
          void Function(DeleteHighlightResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteHighlightResponse))
          as DeleteHighlightResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteHighlightResponse create() => DeleteHighlightResponse._();
  @$core.override
  DeleteHighlightResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteHighlightResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteHighlightResponse>(create);
  static DeleteHighlightResponse? _defaultInstance;
}

class AddToHighlightResponse extends $pb.GeneratedMessage {
  factory AddToHighlightResponse() => create();

  AddToHighlightResponse._();

  factory AddToHighlightResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddToHighlightResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddToHighlightResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddToHighlightResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddToHighlightResponse copyWith(
          void Function(AddToHighlightResponse) updates) =>
      super.copyWith((message) => updates(message as AddToHighlightResponse))
          as AddToHighlightResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddToHighlightResponse create() => AddToHighlightResponse._();
  @$core.override
  AddToHighlightResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddToHighlightResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddToHighlightResponse>(create);
  static AddToHighlightResponse? _defaultInstance;
}

class RemoveFromHighlightResponse extends $pb.GeneratedMessage {
  factory RemoveFromHighlightResponse() => create();

  RemoveFromHighlightResponse._();

  factory RemoveFromHighlightResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveFromHighlightResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFromHighlightResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFromHighlightResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFromHighlightResponse copyWith(
          void Function(RemoveFromHighlightResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveFromHighlightResponse))
          as RemoveFromHighlightResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveFromHighlightResponse create() =>
      RemoveFromHighlightResponse._();
  @$core.override
  RemoveFromHighlightResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveFromHighlightResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFromHighlightResponse>(create);
  static RemoveFromHighlightResponse? _defaultInstance;
}

class GetHighlightsResponse extends $pb.GeneratedMessage {
  factory GetHighlightsResponse({
    HighlightList? highlightList,
  }) {
    final result = create();
    if (highlightList != null) result.highlightList = highlightList;
    return result;
  }

  GetHighlightsResponse._();

  factory GetHighlightsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetHighlightsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetHighlightsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<HighlightList>(1, _omitFieldNames ? '' : 'highlightList',
        subBuilder: HighlightList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHighlightsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetHighlightsResponse copyWith(
          void Function(GetHighlightsResponse) updates) =>
      super.copyWith((message) => updates(message as GetHighlightsResponse))
          as GetHighlightsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetHighlightsResponse create() => GetHighlightsResponse._();
  @$core.override
  GetHighlightsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetHighlightsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetHighlightsResponse>(create);
  static GetHighlightsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  HighlightList get highlightList => $_getN(0);
  @$pb.TagNumber(1)
  set highlightList(HighlightList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasHighlightList() => $_has(0);
  @$pb.TagNumber(1)
  void clearHighlightList() => $_clearField(1);
  @$pb.TagNumber(1)
  HighlightList ensureHighlightList() => $_ensure(0);
}

class CreateLookingForPartyResponse extends $pb.GeneratedMessage {
  factory CreateLookingForPartyResponse({
    Story? story,
  }) {
    final result = create();
    if (story != null) result.story = story;
    return result;
  }

  CreateLookingForPartyResponse._();

  factory CreateLookingForPartyResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateLookingForPartyResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateLookingForPartyResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.story.v1'),
      createEmptyInstance: create)
    ..aOM<Story>(1, _omitFieldNames ? '' : 'story', subBuilder: Story.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateLookingForPartyResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateLookingForPartyResponse copyWith(
          void Function(CreateLookingForPartyResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CreateLookingForPartyResponse))
          as CreateLookingForPartyResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateLookingForPartyResponse create() =>
      CreateLookingForPartyResponse._();
  @$core.override
  CreateLookingForPartyResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateLookingForPartyResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateLookingForPartyResponse>(create);
  static CreateLookingForPartyResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Story get story => $_getN(0);
  @$pb.TagNumber(1)
  set story(Story value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasStory() => $_has(0);
  @$pb.TagNumber(1)
  void clearStory() => $_clearField(1);
  @$pb.TagNumber(1)
  Story ensureStory() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
