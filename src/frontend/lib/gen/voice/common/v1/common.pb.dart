// This is a generated file - do not edit.
//
// Generated from voice/common/v1/common.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Generic cursor page (Messaging history, admin lists, etc.).
class CursorPageRequest extends $pb.GeneratedMessage {
  factory CursorPageRequest({
    $core.String? cursor,
    $core.int? pageSize,
  }) {
    final result = create();
    if (cursor != null) result.cursor = cursor;
    if (pageSize != null) result.pageSize = pageSize;
    return result;
  }

  CursorPageRequest._();

  factory CursorPageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CursorPageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CursorPageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'cursor')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CursorPageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CursorPageRequest copyWith(void Function(CursorPageRequest) updates) =>
      super.copyWith((message) => updates(message as CursorPageRequest))
          as CursorPageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CursorPageRequest create() => CursorPageRequest._();
  @$core.override
  CursorPageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CursorPageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CursorPageRequest>(create);
  static CursorPageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get cursor => $_getSZ(0);
  @$pb.TagNumber(1)
  set cursor($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCursor() => $_has(0);
  @$pb.TagNumber(1)
  void clearCursor() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);
}

class CursorPageResponse extends $pb.GeneratedMessage {
  factory CursorPageResponse({
    $core.String? nextCursor,
    $core.bool? hasMore,
  }) {
    final result = create();
    if (nextCursor != null) result.nextCursor = nextCursor;
    if (hasMore != null) result.hasMore = hasMore;
    return result;
  }

  CursorPageResponse._();

  factory CursorPageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CursorPageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CursorPageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nextCursor')
    ..aOB(2, _omitFieldNames ? '' : 'hasMore')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CursorPageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CursorPageResponse copyWith(void Function(CursorPageResponse) updates) =>
      super.copyWith((message) => updates(message as CursorPageResponse))
          as CursorPageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CursorPageResponse create() => CursorPageResponse._();
  @$core.override
  CursorPageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CursorPageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CursorPageResponse>(create);
  static CursorPageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nextCursor => $_getSZ(0);
  @$pb.TagNumber(1)
  set nextCursor($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNextCursor() => $_has(0);
  @$pb.TagNumber(1)
  void clearNextCursor() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get hasMore => $_getBF(1);
  @$pb.TagNumber(2)
  set hasMore($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHasMore() => $_has(1);
  @$pb.TagNumber(2)
  void clearHasMore() => $_clearField(2);
}

/// Offset/limit helper where cursor is not required.
class PageRequest extends $pb.GeneratedMessage {
  factory PageRequest({
    $core.int? offset,
    $core.int? limit,
  }) {
    final result = create();
    if (offset != null) result.offset = offset;
    if (limit != null) result.limit = limit;
    return result;
  }

  PageRequest._();

  factory PageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'offset')
    ..aI(2, _omitFieldNames ? '' : 'limit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PageRequest copyWith(void Function(PageRequest) updates) =>
      super.copyWith((message) => updates(message as PageRequest))
          as PageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PageRequest create() => PageRequest._();
  @$core.override
  PageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PageRequest>(create);
  static PageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get offset => $_getIZ(0);
  @$pb.TagNumber(1)
  set offset($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOffset() => $_has(0);
  @$pb.TagNumber(1)
  void clearOffset() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get limit => $_getIZ(1);
  @$pb.TagNumber(2)
  set limit($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimit() => $_clearField(2);
}

class PageResponse extends $pb.GeneratedMessage {
  factory PageResponse({
    $core.int? totalCount,
  }) {
    final result = create();
    if (totalCount != null) result.totalCount = totalCount;
    return result;
  }

  PageResponse._();

  factory PageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.common.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'totalCount')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PageResponse copyWith(void Function(PageResponse) updates) =>
      super.copyWith((message) => updates(message as PageResponse))
          as PageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PageResponse create() => PageResponse._();
  @$core.override
  PageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PageResponse>(create);
  static PageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get totalCount => $_getIZ(0);
  @$pb.TagNumber(1)
  set totalCount($core.int value) => $_setSignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotalCount() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotalCount() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
