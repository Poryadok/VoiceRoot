// This is a generated file - do not edit.
//
// Generated from voice/authority/v1/authority.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'authority.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'authority.pbenum.dart';

/// Subject lists are canonical UUIDs, sorted and unique. Empty lists are explicit.
/// An owner rejects fields outside its declared scope shape; it never widens it.
class SourceScope extends $pb.GeneratedMessage {
  factory SourceScope({
    $core.int? schemaVersion,
    $core.String? spaceId,
    $core.String? environmentId,
    $core.Iterable<$core.String>? profileIds,
    $core.Iterable<$core.String>? accountIds,
    $core.Iterable<$core.String>? voiceRoomIds,
  }) {
    final result = create();
    if (schemaVersion != null) result.schemaVersion = schemaVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (environmentId != null) result.environmentId = environmentId;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (accountIds != null) result.accountIds.addAll(accountIds);
    if (voiceRoomIds != null) result.voiceRoomIds.addAll(voiceRoomIds);
    return result;
  }

  SourceScope._();

  factory SourceScope.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SourceScope.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SourceScope',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.authority.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'schemaVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'environmentId')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..pPS(5, _omitFieldNames ? '' : 'accountIds')
    ..pPS(6, _omitFieldNames ? '' : 'voiceRoomIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SourceScope clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SourceScope copyWith(void Function(SourceScope) updates) =>
      super.copyWith((message) => updates(message as SourceScope))
          as SourceScope;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SourceScope create() => SourceScope._();
  @$core.override
  SourceScope createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SourceScope getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SourceScope>(create);
  static SourceScope? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get schemaVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set schemaVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSchemaVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearSchemaVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get environmentId => $_getSZ(2);
  @$pb.TagNumber(3)
  set environmentId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEnvironmentId() => $_has(2);
  @$pb.TagNumber(3)
  void clearEnvironmentId() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);

  @$pb.TagNumber(5)
  $pb.PbList<$core.String> get accountIds => $_getList(4);

  /// Role's exact SDK grant resource set, derived from complete owning resource
  /// state and checked again by the publisher. Other owners reject this field.
  @$pb.TagNumber(6)
  $pb.PbList<$core.String> get voiceRoomIds => $_getList(5);
}

/// @voice.unknown_fields=reject
class ReadSnapshotRequest extends $pb.GeneratedMessage {
  factory ReadSnapshotRequest({
    SourceScope? scope,
  }) {
    final result = create();
    if (scope != null) result.scope = scope;
    return result;
  }

  ReadSnapshotRequest._();

  factory ReadSnapshotRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadSnapshotRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadSnapshotRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.authority.v1'),
      createEmptyInstance: create)
    ..aOM<SourceScope>(1, _omitFieldNames ? '' : 'scope',
        subBuilder: SourceScope.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSnapshotRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSnapshotRequest copyWith(void Function(ReadSnapshotRequest) updates) =>
      super.copyWith((message) => updates(message as ReadSnapshotRequest))
          as ReadSnapshotRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadSnapshotRequest create() => ReadSnapshotRequest._();
  @$core.override
  ReadSnapshotRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadSnapshotRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadSnapshotRequest>(create);
  static ReadSnapshotRequest? _defaultInstance;

  @$pb.TagNumber(1)
  SourceScope get scope => $_getN(0);
  @$pb.TagNumber(1)
  set scope(SourceScope value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScope() => $_has(0);
  @$pb.TagNumber(1)
  void clearScope() => $_clearField(1);
  @$pb.TagNumber(1)
  SourceScope ensureScope() => $_ensure(0);
}

/// canonical_state is the fixed, versioned owner JSON schema. Clients reject
/// unknown authority fields, missing fields, wrong owner/scope and partial state.
class ReadSnapshotResponse extends $pb.GeneratedMessage {
  factory ReadSnapshotResponse({
    SourceScope? scope,
    AuthorityOwner? owner,
    $fixnum.Int64? revision,
    $core.bool? complete,
    $core.List<$core.int>? canonicalState,
    $fixnum.Int64? validUntilUnixMillis,
  }) {
    final result = create();
    if (scope != null) result.scope = scope;
    if (owner != null) result.owner = owner;
    if (revision != null) result.revision = revision;
    if (complete != null) result.complete = complete;
    if (canonicalState != null) result.canonicalState = canonicalState;
    if (validUntilUnixMillis != null)
      result.validUntilUnixMillis = validUntilUnixMillis;
    return result;
  }

  ReadSnapshotResponse._();

  factory ReadSnapshotResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadSnapshotResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadSnapshotResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.authority.v1'),
      createEmptyInstance: create)
    ..aOM<SourceScope>(1, _omitFieldNames ? '' : 'scope',
        subBuilder: SourceScope.create)
    ..aE<AuthorityOwner>(2, _omitFieldNames ? '' : 'owner',
        enumValues: AuthorityOwner.values)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOB(4, _omitFieldNames ? '' : 'complete')
    ..a<$core.List<$core.int>>(
        5, _omitFieldNames ? '' : 'canonicalState', $pb.PbFieldType.OY)
    ..aInt64(6, _omitFieldNames ? '' : 'validUntilUnixMillis')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSnapshotResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSnapshotResponse copyWith(void Function(ReadSnapshotResponse) updates) =>
      super.copyWith((message) => updates(message as ReadSnapshotResponse))
          as ReadSnapshotResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadSnapshotResponse create() => ReadSnapshotResponse._();
  @$core.override
  ReadSnapshotResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadSnapshotResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadSnapshotResponse>(create);
  static ReadSnapshotResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SourceScope get scope => $_getN(0);
  @$pb.TagNumber(1)
  set scope(SourceScope value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScope() => $_has(0);
  @$pb.TagNumber(1)
  void clearScope() => $_clearField(1);
  @$pb.TagNumber(1)
  SourceScope ensureScope() => $_ensure(0);

  @$pb.TagNumber(2)
  AuthorityOwner get owner => $_getN(1);
  @$pb.TagNumber(2)
  set owner(AuthorityOwner value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOwner() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwner() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get revision => $_getI64(2);
  @$pb.TagNumber(3)
  set revision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearRevision() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get complete => $_getBF(3);
  @$pb.TagNumber(4)
  set complete($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasComplete() => $_has(3);
  @$pb.TagNumber(4)
  void clearComplete() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.List<$core.int> get canonicalState => $_getN(4);
  @$pb.TagNumber(5)
  set canonicalState($core.List<$core.int> value) => $_setBytes(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCanonicalState() => $_has(4);
  @$pb.TagNumber(5)
  void clearCanonicalState() => $_clearField(5);

  /// Zero means no earlier time-dependent cutoff than the publisher's own bound.
  @$pb.TagNumber(6)
  $fixnum.Int64 get validUntilUnixMillis => $_getI64(5);
  @$pb.TagNumber(6)
  set validUntilUnixMillis($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasValidUntilUnixMillis() => $_has(5);
  @$pb.TagNumber(6)
  void clearValidUntilUnixMillis() => $_clearField(6);
}

/// @voice.unknown_fields=reject
class ReadRevisionRequest extends $pb.GeneratedMessage {
  factory ReadRevisionRequest({
    SourceScope? scope,
  }) {
    final result = create();
    if (scope != null) result.scope = scope;
    return result;
  }

  ReadRevisionRequest._();

  factory ReadRevisionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadRevisionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadRevisionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.authority.v1'),
      createEmptyInstance: create)
    ..aOM<SourceScope>(1, _omitFieldNames ? '' : 'scope',
        subBuilder: SourceScope.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRevisionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRevisionRequest copyWith(void Function(ReadRevisionRequest) updates) =>
      super.copyWith((message) => updates(message as ReadRevisionRequest))
          as ReadRevisionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadRevisionRequest create() => ReadRevisionRequest._();
  @$core.override
  ReadRevisionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadRevisionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadRevisionRequest>(create);
  static ReadRevisionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  SourceScope get scope => $_getN(0);
  @$pb.TagNumber(1)
  set scope(SourceScope value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScope() => $_has(0);
  @$pb.TagNumber(1)
  void clearScope() => $_clearField(1);
  @$pb.TagNumber(1)
  SourceScope ensureScope() => $_ensure(0);
}

class ReadRevisionResponse extends $pb.GeneratedMessage {
  factory ReadRevisionResponse({
    SourceScope? scope,
    AuthorityOwner? owner,
    $fixnum.Int64? revision,
  }) {
    final result = create();
    if (scope != null) result.scope = scope;
    if (owner != null) result.owner = owner;
    if (revision != null) result.revision = revision;
    return result;
  }

  ReadRevisionResponse._();

  factory ReadRevisionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadRevisionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadRevisionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.authority.v1'),
      createEmptyInstance: create)
    ..aOM<SourceScope>(1, _omitFieldNames ? '' : 'scope',
        subBuilder: SourceScope.create)
    ..aE<AuthorityOwner>(2, _omitFieldNames ? '' : 'owner',
        enumValues: AuthorityOwner.values)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRevisionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadRevisionResponse copyWith(void Function(ReadRevisionResponse) updates) =>
      super.copyWith((message) => updates(message as ReadRevisionResponse))
          as ReadRevisionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadRevisionResponse create() => ReadRevisionResponse._();
  @$core.override
  ReadRevisionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadRevisionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadRevisionResponse>(create);
  static ReadRevisionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SourceScope get scope => $_getN(0);
  @$pb.TagNumber(1)
  set scope(SourceScope value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasScope() => $_has(0);
  @$pb.TagNumber(1)
  void clearScope() => $_clearField(1);
  @$pb.TagNumber(1)
  SourceScope ensureScope() => $_ensure(0);

  @$pb.TagNumber(2)
  AuthorityOwner get owner => $_getN(1);
  @$pb.TagNumber(2)
  set owner(AuthorityOwner value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasOwner() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwner() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get revision => $_getI64(2);
  @$pb.TagNumber(3)
  set revision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearRevision() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
