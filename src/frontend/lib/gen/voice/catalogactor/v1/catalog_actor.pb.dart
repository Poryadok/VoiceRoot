// This is a generated file - do not edit.
//
// Generated from voice/catalogactor/v1/catalog_actor.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import 'catalog_actor.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'catalog_actor.pbenum.dart';

class CatalogActor extends $pb.GeneratedMessage {
  factory CatalogActor({
    $core.String? accountId,
    $core.String? profileId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  CatalogActor._();

  factory CatalogActor.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CatalogActor.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CatalogActor',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.catalogactor.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogActor clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogActor copyWith(void Function(CatalogActor) updates) =>
      super.copyWith((message) => updates(message as CatalogActor))
          as CatalogActor;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CatalogActor create() => CatalogActor._();
  @$core.override
  CatalogActor createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CatalogActor getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CatalogActor>(create);
  static CatalogActor? _defaultInstance;

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

/// @voice.unknown_fields=reject
class GetCatalogActorStatesRequest extends $pb.GeneratedMessage {
  factory GetCatalogActorStatesRequest({
    $core.int? protocolVersion,
    $core.Iterable<CatalogActor>? actors,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (actors != null) result.actors.addAll(actors);
    return result;
  }

  GetCatalogActorStatesRequest._();

  factory GetCatalogActorStatesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCatalogActorStatesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCatalogActorStatesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.catalogactor.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..pPM<CatalogActor>(2, _omitFieldNames ? '' : 'actors',
        subBuilder: CatalogActor.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogActorStatesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogActorStatesRequest copyWith(
          void Function(GetCatalogActorStatesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetCatalogActorStatesRequest))
          as GetCatalogActorStatesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCatalogActorStatesRequest create() =>
      GetCatalogActorStatesRequest._();
  @$core.override
  GetCatalogActorStatesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCatalogActorStatesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCatalogActorStatesRequest>(create);
  static GetCatalogActorStatesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  /// 1..100 canonical nonnil UUID pairs; profile IDs must be distinct.
  @$pb.TagNumber(2)
  $pb.PbList<CatalogActor> get actors => $_getList(1);
}

class CatalogActorStateEntry extends $pb.GeneratedMessage {
  factory CatalogActorStateEntry({
    CatalogActor? actor,
    CatalogActorState? state,
  }) {
    final result = create();
    if (actor != null) result.actor = actor;
    if (state != null) result.state = state;
    return result;
  }

  CatalogActorStateEntry._();

  factory CatalogActorStateEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CatalogActorStateEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CatalogActorStateEntry',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.catalogactor.v1'),
      createEmptyInstance: create)
    ..aOM<CatalogActor>(1, _omitFieldNames ? '' : 'actor',
        subBuilder: CatalogActor.create)
    ..aE<CatalogActorState>(2, _omitFieldNames ? '' : 'state',
        enumValues: CatalogActorState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogActorStateEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CatalogActorStateEntry copyWith(
          void Function(CatalogActorStateEntry) updates) =>
      super.copyWith((message) => updates(message as CatalogActorStateEntry))
          as CatalogActorStateEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CatalogActorStateEntry create() => CatalogActorStateEntry._();
  @$core.override
  CatalogActorStateEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CatalogActorStateEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CatalogActorStateEntry>(create);
  static CatalogActorStateEntry? _defaultInstance;

  /// Exact request echo; never reveals the actual owner of a mismatched profile.
  @$pb.TagNumber(1)
  CatalogActor get actor => $_getN(0);
  @$pb.TagNumber(1)
  set actor(CatalogActor value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasActor() => $_has(0);
  @$pb.TagNumber(1)
  void clearActor() => $_clearField(1);
  @$pb.TagNumber(1)
  CatalogActor ensureActor() => $_ensure(0);

  @$pb.TagNumber(2)
  CatalogActorState get state => $_getN(1);
  @$pb.TagNumber(2)
  set state(CatalogActorState value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);
}

class GetCatalogActorStatesResponse extends $pb.GeneratedMessage {
  factory GetCatalogActorStatesResponse({
    $core.int? protocolVersion,
    $core.Iterable<CatalogActorStateEntry>? states,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (states != null) result.states.addAll(states);
    return result;
  }

  GetCatalogActorStatesResponse._();

  factory GetCatalogActorStatesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetCatalogActorStatesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetCatalogActorStatesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.catalogactor.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..pPM<CatalogActorStateEntry>(2, _omitFieldNames ? '' : 'states',
        subBuilder: CatalogActorStateEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogActorStatesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetCatalogActorStatesResponse copyWith(
          void Function(GetCatalogActorStatesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetCatalogActorStatesResponse))
          as GetCatalogActorStatesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetCatalogActorStatesResponse create() =>
      GetCatalogActorStatesResponse._();
  @$core.override
  GetCatalogActorStatesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetCatalogActorStatesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetCatalogActorStatesResponse>(create);
  static GetCatalogActorStatesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  /// Exactly one entry per input, in input order. UNKNOWN is never deletion proof.
  @$pb.TagNumber(2)
  $pb.PbList<CatalogActorStateEntry> get states => $_getList(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
