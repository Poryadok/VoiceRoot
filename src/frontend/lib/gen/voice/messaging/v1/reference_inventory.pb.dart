// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/reference_inventory.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import '../../common/v1/reference_inventory.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class BeginReferenceInventoryRequest extends $pb.GeneratedMessage {
  factory BeginReferenceInventoryRequest({
    $1.BeginReferenceInventoryRequest? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  BeginReferenceInventoryRequest._();

  factory BeginReferenceInventoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BeginReferenceInventoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginReferenceInventoryRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.BeginReferenceInventoryRequest>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.BeginReferenceInventoryRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryRequest copyWith(
          void Function(BeginReferenceInventoryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as BeginReferenceInventoryRequest))
          as BeginReferenceInventoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryRequest create() =>
      BeginReferenceInventoryRequest._();
  @$core.override
  BeginReferenceInventoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginReferenceInventoryRequest>(create);
  static BeginReferenceInventoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.BeginReferenceInventoryRequest get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.BeginReferenceInventoryRequest value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.BeginReferenceInventoryRequest ensureInventory() => $_ensure(0);
}

class BeginReferenceInventoryResponse extends $pb.GeneratedMessage {
  factory BeginReferenceInventoryResponse({
    $1.BeginReferenceInventoryResponse? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  BeginReferenceInventoryResponse._();

  factory BeginReferenceInventoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BeginReferenceInventoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BeginReferenceInventoryResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.BeginReferenceInventoryResponse>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.BeginReferenceInventoryResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BeginReferenceInventoryResponse copyWith(
          void Function(BeginReferenceInventoryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as BeginReferenceInventoryResponse))
          as BeginReferenceInventoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryResponse create() =>
      BeginReferenceInventoryResponse._();
  @$core.override
  BeginReferenceInventoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BeginReferenceInventoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BeginReferenceInventoryResponse>(
          create);
  static BeginReferenceInventoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $1.BeginReferenceInventoryResponse get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.BeginReferenceInventoryResponse value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.BeginReferenceInventoryResponse ensureInventory() => $_ensure(0);
}

class ReadReferenceInventoryPageRequest extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryPageRequest({
    $1.ReadReferenceInventoryPageRequest? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  ReadReferenceInventoryPageRequest._();

  factory ReadReferenceInventoryPageRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryPageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryPageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ReadReferenceInventoryPageRequest>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.ReadReferenceInventoryPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageRequest copyWith(
          void Function(ReadReferenceInventoryPageRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryPageRequest))
          as ReadReferenceInventoryPageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageRequest create() =>
      ReadReferenceInventoryPageRequest._();
  @$core.override
  ReadReferenceInventoryPageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadReferenceInventoryPageRequest>(
          create);
  static ReadReferenceInventoryPageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryPageRequest get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.ReadReferenceInventoryPageRequest value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryPageRequest ensureInventory() => $_ensure(0);
}

class ReadReferenceInventoryPageResponse extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryPageResponse({
    $1.ReadReferenceInventoryPageResponse? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  ReadReferenceInventoryPageResponse._();

  factory ReadReferenceInventoryPageResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryPageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryPageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ReadReferenceInventoryPageResponse>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.ReadReferenceInventoryPageResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryPageResponse copyWith(
          void Function(ReadReferenceInventoryPageResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryPageResponse))
          as ReadReferenceInventoryPageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageResponse create() =>
      ReadReferenceInventoryPageResponse._();
  @$core.override
  ReadReferenceInventoryPageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryPageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadReferenceInventoryPageResponse>(
          create);
  static ReadReferenceInventoryPageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryPageResponse get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.ReadReferenceInventoryPageResponse value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryPageResponse ensureInventory() => $_ensure(0);
}

class ReadReferenceInventoryChangesRequest extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryChangesRequest({
    $1.ReadReferenceInventoryChangesRequest? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  ReadReferenceInventoryChangesRequest._();

  factory ReadReferenceInventoryChangesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryChangesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryChangesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ReadReferenceInventoryChangesRequest>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.ReadReferenceInventoryChangesRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesRequest copyWith(
          void Function(ReadReferenceInventoryChangesRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryChangesRequest))
          as ReadReferenceInventoryChangesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesRequest create() =>
      ReadReferenceInventoryChangesRequest._();
  @$core.override
  ReadReferenceInventoryChangesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadReferenceInventoryChangesRequest>(create);
  static ReadReferenceInventoryChangesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryChangesRequest get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.ReadReferenceInventoryChangesRequest value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryChangesRequest ensureInventory() => $_ensure(0);
}

class ReadReferenceInventoryChangesResponse extends $pb.GeneratedMessage {
  factory ReadReferenceInventoryChangesResponse({
    $1.ReadReferenceInventoryChangesResponse? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  ReadReferenceInventoryChangesResponse._();

  factory ReadReferenceInventoryChangesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadReferenceInventoryChangesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadReferenceInventoryChangesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ReadReferenceInventoryChangesResponse>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.ReadReferenceInventoryChangesResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadReferenceInventoryChangesResponse copyWith(
          void Function(ReadReferenceInventoryChangesResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadReferenceInventoryChangesResponse))
          as ReadReferenceInventoryChangesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesResponse create() =>
      ReadReferenceInventoryChangesResponse._();
  @$core.override
  ReadReferenceInventoryChangesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadReferenceInventoryChangesResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ReadReferenceInventoryChangesResponse>(create);
  static ReadReferenceInventoryChangesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryChangesResponse get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.ReadReferenceInventoryChangesResponse value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ReadReferenceInventoryChangesResponse ensureInventory() => $_ensure(0);
}

class SealReferenceInventoryRequest extends $pb.GeneratedMessage {
  factory SealReferenceInventoryRequest({
    $1.SealReferenceInventoryRequest? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  SealReferenceInventoryRequest._();

  factory SealReferenceInventoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SealReferenceInventoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SealReferenceInventoryRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SealReferenceInventoryRequest>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.SealReferenceInventoryRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryRequest copyWith(
          void Function(SealReferenceInventoryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SealReferenceInventoryRequest))
          as SealReferenceInventoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryRequest create() =>
      SealReferenceInventoryRequest._();
  @$core.override
  SealReferenceInventoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SealReferenceInventoryRequest>(create);
  static SealReferenceInventoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.SealReferenceInventoryRequest get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.SealReferenceInventoryRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SealReferenceInventoryRequest ensureInventory() => $_ensure(0);
}

class SealReferenceInventoryResponse extends $pb.GeneratedMessage {
  factory SealReferenceInventoryResponse({
    $1.SealReferenceInventoryResponse? inventory,
  }) {
    final result = create();
    if (inventory != null) result.inventory = inventory;
    return result;
  }

  SealReferenceInventoryResponse._();

  factory SealReferenceInventoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SealReferenceInventoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SealReferenceInventoryResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOM<$1.SealReferenceInventoryResponse>(
        1, _omitFieldNames ? '' : 'inventory',
        subBuilder: $1.SealReferenceInventoryResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SealReferenceInventoryResponse copyWith(
          void Function(SealReferenceInventoryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SealReferenceInventoryResponse))
          as SealReferenceInventoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryResponse create() =>
      SealReferenceInventoryResponse._();
  @$core.override
  SealReferenceInventoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SealReferenceInventoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SealReferenceInventoryResponse>(create);
  static SealReferenceInventoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $1.SealReferenceInventoryResponse get inventory => $_getN(0);
  @$pb.TagNumber(1)
  set inventory($1.SealReferenceInventoryResponse value) =>
      $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasInventory() => $_has(0);
  @$pb.TagNumber(1)
  void clearInventory() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.SealReferenceInventoryResponse ensureInventory() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
