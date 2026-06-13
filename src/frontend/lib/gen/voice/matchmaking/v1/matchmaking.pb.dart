// This is a generated file - do not edit.
//
// Generated from voice/matchmaking/v1/matchmaking.proto.

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

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class Game extends $pb.GeneratedMessage {
  factory Game({
    $core.String? id,
    $core.String? name,
    $core.String? iconUrl,
    $core.String? externalId,
    $core.String? configJson,
    $core.String? status,
    $core.String? createdByProfileId,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (iconUrl != null) result.iconUrl = iconUrl;
    if (externalId != null) result.externalId = externalId;
    if (configJson != null) result.configJson = configJson;
    if (status != null) result.status = status;
    if (createdByProfileId != null)
      result.createdByProfileId = createdByProfileId;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  Game._();

  factory Game.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Game.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Game',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'iconUrl')
    ..aOS(4, _omitFieldNames ? '' : 'externalId')
    ..aOS(5, _omitFieldNames ? '' : 'configJson')
    ..aOS(6, _omitFieldNames ? '' : 'status')
    ..aOS(7, _omitFieldNames ? '' : 'createdByProfileId')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Game clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Game copyWith(void Function(Game) updates) =>
      super.copyWith((message) => updates(message as Game)) as Game;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Game create() => Game._();
  @$core.override
  Game createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Game getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Game>(create);
  static Game? _defaultInstance;

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
  $core.String get iconUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set iconUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIconUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearIconUrl() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get externalId => $_getSZ(3);
  @$pb.TagNumber(4)
  set externalId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExternalId() => $_has(3);
  @$pb.TagNumber(4)
  void clearExternalId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get configJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set configJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasConfigJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearConfigJson() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get status => $_getSZ(5);
  @$pb.TagNumber(6)
  set status($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get createdByProfileId => $_getSZ(6);
  @$pb.TagNumber(7)
  set createdByProfileId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCreatedByProfileId() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreatedByProfileId() => $_clearField(7);

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

class ListGamesRequest extends $pb.GeneratedMessage {
  factory ListGamesRequest({
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ListGamesRequest._();

  factory ListGamesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListGamesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListGamesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<$2.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListGamesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListGamesRequest copyWith(void Function(ListGamesRequest) updates) =>
      super.copyWith((message) => updates(message as ListGamesRequest))
          as ListGamesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListGamesRequest create() => ListGamesRequest._();
  @$core.override
  ListGamesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListGamesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListGamesRequest>(create);
  static ListGamesRequest? _defaultInstance;

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

class GameList extends $pb.GeneratedMessage {
  factory GameList({
    $core.Iterable<Game>? games,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (games != null) result.games.addAll(games);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  GameList._();

  factory GameList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GameList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GameList',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..pPM<Game>(1, _omitFieldNames ? '' : 'games', subBuilder: Game.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameList copyWith(void Function(GameList) updates) =>
      super.copyWith((message) => updates(message as GameList)) as GameList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GameList create() => GameList._();
  @$core.override
  GameList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GameList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GameList>(create);
  static GameList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Game> get games => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class GetGameRequest extends $pb.GeneratedMessage {
  factory GetGameRequest({
    $core.String? gameId,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    return result;
  }

  GetGameRequest._();

  factory GetGameRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetGameRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetGameRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetGameRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetGameRequest copyWith(void Function(GetGameRequest) updates) =>
      super.copyWith((message) => updates(message as GetGameRequest))
          as GetGameRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetGameRequest create() => GetGameRequest._();
  @$core.override
  GetGameRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetGameRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetGameRequest>(create);
  static GetGameRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);
}

class CreateGameRequest extends $pb.GeneratedMessage {
  factory CreateGameRequest({
    $core.String? name,
    $core.String? configJson,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (configJson != null) result.configJson = configJson;
    return result;
  }

  CreateGameRequest._();

  factory CreateGameRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateGameRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateGameRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'configJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateGameRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateGameRequest copyWith(void Function(CreateGameRequest) updates) =>
      super.copyWith((message) => updates(message as CreateGameRequest))
          as CreateGameRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateGameRequest create() => CreateGameRequest._();
  @$core.override
  CreateGameRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateGameRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateGameRequest>(create);
  static CreateGameRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get configJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set configJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasConfigJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearConfigJson() => $_clearField(2);
}

class UpdateGameRequest extends $pb.GeneratedMessage {
  factory UpdateGameRequest({
    $core.String? gameId,
    $core.String? name,
    $core.String? configJson,
    $core.String? status,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    if (name != null) result.name = name;
    if (configJson != null) result.configJson = configJson;
    if (status != null) result.status = status;
    return result;
  }

  UpdateGameRequest._();

  factory UpdateGameRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateGameRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateGameRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'configJson')
    ..aOS(4, _omitFieldNames ? '' : 'status')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateGameRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateGameRequest copyWith(void Function(UpdateGameRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateGameRequest))
          as UpdateGameRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateGameRequest create() => UpdateGameRequest._();
  @$core.override
  UpdateGameRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateGameRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateGameRequest>(create);
  static UpdateGameRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get configJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set configJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasConfigJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearConfigJson() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get status => $_getSZ(3);
  @$pb.TagNumber(4)
  set status($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasStatus() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatus() => $_clearField(4);
}

class SearchGamesRequest extends $pb.GeneratedMessage {
  factory SearchGamesRequest({
    $core.String? query,
  }) {
    final result = create();
    if (query != null) result.query = query;
    return result;
  }

  SearchGamesRequest._();

  factory SearchGamesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchGamesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchGamesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGamesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGamesRequest copyWith(void Function(SearchGamesRequest) updates) =>
      super.copyWith((message) => updates(message as SearchGamesRequest))
          as SearchGamesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchGamesRequest create() => SearchGamesRequest._();
  @$core.override
  SearchGamesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchGamesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchGamesRequest>(create);
  static SearchGamesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);
}

class StartSearchRequest extends $pb.GeneratedMessage {
  factory StartSearchRequest({
    $core.String? gameId,
    $core.String? mode,
    $core.String? criteriaJson,
    $core.String? partyId,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    if (criteriaJson != null) result.criteriaJson = criteriaJson;
    if (partyId != null) result.partyId = partyId;
    return result;
  }

  StartSearchRequest._();

  factory StartSearchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartSearchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartSearchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..aOS(2, _omitFieldNames ? '' : 'mode')
    ..aOS(3, _omitFieldNames ? '' : 'criteriaJson')
    ..aOS(4, _omitFieldNames ? '' : 'partyId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartSearchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartSearchRequest copyWith(void Function(StartSearchRequest) updates) =>
      super.copyWith((message) => updates(message as StartSearchRequest))
          as StartSearchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartSearchRequest create() => StartSearchRequest._();
  @$core.override
  StartSearchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartSearchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartSearchRequest>(create);
  static StartSearchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get mode => $_getSZ(1);
  @$pb.TagNumber(2)
  set mode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMode() => $_has(1);
  @$pb.TagNumber(2)
  void clearMode() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get criteriaJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set criteriaJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCriteriaJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearCriteriaJson() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get partyId => $_getSZ(3);
  @$pb.TagNumber(4)
  set partyId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPartyId() => $_has(3);
  @$pb.TagNumber(4)
  void clearPartyId() => $_clearField(4);
}

class SearchSession extends $pb.GeneratedMessage {
  factory SearchSession({
    $core.String? id,
    $core.String? profileId,
    $core.String? partyId,
    $core.String? gameId,
    $core.String? mode,
    $core.String? criteriaJson,
    $core.String? status,
    $1.Timestamp? timeoutAt,
    $1.Timestamp? matchedAt,
    $core.String? matchId,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (profileId != null) result.profileId = profileId;
    if (partyId != null) result.partyId = partyId;
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    if (criteriaJson != null) result.criteriaJson = criteriaJson;
    if (status != null) result.status = status;
    if (timeoutAt != null) result.timeoutAt = timeoutAt;
    if (matchedAt != null) result.matchedAt = matchedAt;
    if (matchId != null) result.matchId = matchId;
    return result;
  }

  SearchSession._();

  factory SearchSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchSession',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'partyId')
    ..aOS(4, _omitFieldNames ? '' : 'gameId')
    ..aOS(5, _omitFieldNames ? '' : 'mode')
    ..aOS(6, _omitFieldNames ? '' : 'criteriaJson')
    ..aOS(7, _omitFieldNames ? '' : 'status')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'timeoutAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'matchedAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(10, _omitFieldNames ? '' : 'matchId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSession copyWith(void Function(SearchSession) updates) =>
      super.copyWith((message) => updates(message as SearchSession))
          as SearchSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchSession create() => SearchSession._();
  @$core.override
  SearchSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchSession>(create);
  static SearchSession? _defaultInstance;

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
  $core.String get partyId => $_getSZ(2);
  @$pb.TagNumber(3)
  set partyId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPartyId() => $_has(2);
  @$pb.TagNumber(3)
  void clearPartyId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get gameId => $_getSZ(3);
  @$pb.TagNumber(4)
  set gameId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGameId() => $_has(3);
  @$pb.TagNumber(4)
  void clearGameId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get mode => $_getSZ(4);
  @$pb.TagNumber(5)
  set mode($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasMode() => $_has(4);
  @$pb.TagNumber(5)
  void clearMode() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get criteriaJson => $_getSZ(5);
  @$pb.TagNumber(6)
  set criteriaJson($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCriteriaJson() => $_has(5);
  @$pb.TagNumber(6)
  void clearCriteriaJson() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get status => $_getSZ(6);
  @$pb.TagNumber(7)
  set status($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasStatus() => $_has(6);
  @$pb.TagNumber(7)
  void clearStatus() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get timeoutAt => $_getN(7);
  @$pb.TagNumber(8)
  set timeoutAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasTimeoutAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearTimeoutAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureTimeoutAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get matchedAt => $_getN(8);
  @$pb.TagNumber(9)
  set matchedAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasMatchedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearMatchedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureMatchedAt() => $_ensure(8);

  @$pb.TagNumber(10)
  $core.String get matchId => $_getSZ(9);
  @$pb.TagNumber(10)
  set matchId($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasMatchId() => $_has(9);
  @$pb.TagNumber(10)
  void clearMatchId() => $_clearField(10);
}

class CancelSearchRequest extends $pb.GeneratedMessage {
  factory CancelSearchRequest({
    $core.String? sessionId,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  CancelSearchRequest._();

  factory CancelSearchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CancelSearchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelSearchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSearchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSearchRequest copyWith(void Function(CancelSearchRequest) updates) =>
      super.copyWith((message) => updates(message as CancelSearchRequest))
          as CancelSearchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CancelSearchRequest create() => CancelSearchRequest._();
  @$core.override
  CancelSearchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CancelSearchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelSearchRequest>(create);
  static CancelSearchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

class GetSearchStatusRequest extends $pb.GeneratedMessage {
  factory GetSearchStatusRequest({
    $core.String? sessionId,
  }) {
    final result = create();
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  GetSearchStatusRequest._();

  factory GetSearchStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSearchStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSearchStatusRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSearchStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSearchStatusRequest copyWith(
          void Function(GetSearchStatusRequest) updates) =>
      super.copyWith((message) => updates(message as GetSearchStatusRequest))
          as GetSearchStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSearchStatusRequest create() => GetSearchStatusRequest._();
  @$core.override
  GetSearchStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSearchStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSearchStatusRequest>(create);
  static GetSearchStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sessionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sessionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSessionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSessionId() => $_clearField(1);
}

class Match extends $pb.GeneratedMessage {
  factory Match({
    $core.String? id,
    $core.String? gameId,
    $core.String? mode,
    $core.Iterable<$core.String>? profileIds,
    $1.Timestamp? createdAt,
    $core.String? voiceRoomId,
    $core.String? chatId,
    $core.String? status,
    $core.String? region,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (gameId != null) result.gameId = gameId;
    if (mode != null) result.mode = mode;
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (createdAt != null) result.createdAt = createdAt;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (chatId != null) result.chatId = chatId;
    if (status != null) result.status = status;
    if (region != null) result.region = region;
    return result;
  }

  Match._();

  factory Match.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Match.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Match',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'gameId')
    ..aOS(3, _omitFieldNames ? '' : 'mode')
    ..pPS(4, _omitFieldNames ? '' : 'profileIds')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(6, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(7, _omitFieldNames ? '' : 'chatId')
    ..aOS(8, _omitFieldNames ? '' : 'status')
    ..aOS(9, _omitFieldNames ? '' : 'region')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Match clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Match copyWith(void Function(Match) updates) =>
      super.copyWith((message) => updates(message as Match)) as Match;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Match create() => Match._();
  @$core.override
  Match createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Match getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Match>(create);
  static Match? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get gameId => $_getSZ(1);
  @$pb.TagNumber(2)
  set gameId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGameId() => $_has(1);
  @$pb.TagNumber(2)
  void clearGameId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get mode => $_getSZ(2);
  @$pb.TagNumber(3)
  set mode($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMode() => $_has(2);
  @$pb.TagNumber(3)
  void clearMode() => $_clearField(3);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get profileIds => $_getList(3);

  @$pb.TagNumber(5)
  $1.Timestamp get createdAt => $_getN(4);
  @$pb.TagNumber(5)
  set createdAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCreatedAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearCreatedAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureCreatedAt() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.String get voiceRoomId => $_getSZ(5);
  @$pb.TagNumber(6)
  set voiceRoomId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasVoiceRoomId() => $_has(5);
  @$pb.TagNumber(6)
  void clearVoiceRoomId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get chatId => $_getSZ(6);
  @$pb.TagNumber(7)
  set chatId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasChatId() => $_has(6);
  @$pb.TagNumber(7)
  void clearChatId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get status => $_getSZ(7);
  @$pb.TagNumber(8)
  set status($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasStatus() => $_has(7);
  @$pb.TagNumber(8)
  void clearStatus() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get region => $_getSZ(8);
  @$pb.TagNumber(9)
  set region($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRegion() => $_has(8);
  @$pb.TagNumber(9)
  void clearRegion() => $_clearField(9);
}

class GetMatchRequest extends $pb.GeneratedMessage {
  factory GetMatchRequest({
    $core.String? matchId,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    return result;
  }

  GetMatchRequest._();

  factory GetMatchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchRequest copyWith(void Function(GetMatchRequest) updates) =>
      super.copyWith((message) => updates(message as GetMatchRequest))
          as GetMatchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchRequest create() => GetMatchRequest._();
  @$core.override
  GetMatchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchRequest>(create);
  static GetMatchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);
}

class RespondToMatchRequest extends $pb.GeneratedMessage {
  factory RespondToMatchRequest({
    $core.String? matchId,
    $core.bool? accept,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    if (accept != null) result.accept = accept;
    return result;
  }

  RespondToMatchRequest._();

  factory RespondToMatchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RespondToMatchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RespondToMatchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..aOB(2, _omitFieldNames ? '' : 'accept')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondToMatchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondToMatchRequest copyWith(
          void Function(RespondToMatchRequest) updates) =>
      super.copyWith((message) => updates(message as RespondToMatchRequest))
          as RespondToMatchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RespondToMatchRequest create() => RespondToMatchRequest._();
  @$core.override
  RespondToMatchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RespondToMatchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RespondToMatchRequest>(create);
  static RespondToMatchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get accept => $_getBF(1);
  @$pb.TagNumber(2)
  set accept($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccept() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccept() => $_clearField(2);
}

class RespondToMatchResponse extends $pb.GeneratedMessage {
  factory RespondToMatchResponse({
    Match? match,
    SearchSession? searchSession,
  }) {
    final result = create();
    if (match != null) result.match = match;
    if (searchSession != null) result.searchSession = searchSession;
    return result;
  }

  RespondToMatchResponse._();

  factory RespondToMatchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RespondToMatchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RespondToMatchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Match>(1, _omitFieldNames ? '' : 'match', subBuilder: Match.create)
    ..aOM<SearchSession>(2, _omitFieldNames ? '' : 'searchSession',
        subBuilder: SearchSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondToMatchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RespondToMatchResponse copyWith(
          void Function(RespondToMatchResponse) updates) =>
      super.copyWith((message) => updates(message as RespondToMatchResponse))
          as RespondToMatchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RespondToMatchResponse create() => RespondToMatchResponse._();
  @$core.override
  RespondToMatchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RespondToMatchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RespondToMatchResponse>(create);
  static RespondToMatchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Match get match => $_getN(0);
  @$pb.TagNumber(1)
  set match(Match value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMatch() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatch() => $_clearField(1);
  @$pb.TagNumber(1)
  Match ensureMatch() => $_ensure(0);

  @$pb.TagNumber(2)
  SearchSession get searchSession => $_getN(1);
  @$pb.TagNumber(2)
  set searchSession(SearchSession value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSearchSession() => $_has(1);
  @$pb.TagNumber(2)
  void clearSearchSession() => $_clearField(2);
  @$pb.TagNumber(2)
  SearchSession ensureSearchSession() => $_ensure(1);
}

class GetMatchHistoryRequest extends $pb.GeneratedMessage {
  factory GetMatchHistoryRequest({
    $core.String? profileId,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (page != null) result.page = page;
    return result;
  }

  GetMatchHistoryRequest._();

  factory GetMatchHistoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchHistoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchHistoryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchHistoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchHistoryRequest copyWith(
          void Function(GetMatchHistoryRequest) updates) =>
      super.copyWith((message) => updates(message as GetMatchHistoryRequest))
          as GetMatchHistoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchHistoryRequest create() => GetMatchHistoryRequest._();
  @$core.override
  GetMatchHistoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchHistoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchHistoryRequest>(create);
  static GetMatchHistoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

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

class MatchList extends $pb.GeneratedMessage {
  factory MatchList({
    $core.Iterable<Match>? matches,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (matches != null) result.matches.addAll(matches);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  MatchList._();

  factory MatchList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MatchList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MatchList',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..pPM<Match>(1, _omitFieldNames ? '' : 'matches', subBuilder: Match.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MatchList copyWith(void Function(MatchList) updates) =>
      super.copyWith((message) => updates(message as MatchList)) as MatchList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MatchList create() => MatchList._();
  @$core.override
  MatchList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MatchList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<MatchList>(create);
  static MatchList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Match> get matches => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class CompleteMatchRequest extends $pb.GeneratedMessage {
  factory CompleteMatchRequest({
    $core.String? matchId,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    return result;
  }

  CompleteMatchRequest._();

  factory CompleteMatchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteMatchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteMatchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMatchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMatchRequest copyWith(void Function(CompleteMatchRequest) updates) =>
      super.copyWith((message) => updates(message as CompleteMatchRequest))
          as CompleteMatchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteMatchRequest create() => CompleteMatchRequest._();
  @$core.override
  CompleteMatchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteMatchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteMatchRequest>(create);
  static CompleteMatchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);
}

class CompleteMatchResponse extends $pb.GeneratedMessage {
  factory CompleteMatchResponse({
    Match? match,
  }) {
    final result = create();
    if (match != null) result.match = match;
    return result;
  }

  CompleteMatchResponse._();

  factory CompleteMatchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompleteMatchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompleteMatchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Match>(1, _omitFieldNames ? '' : 'match', subBuilder: Match.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMatchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompleteMatchResponse copyWith(
          void Function(CompleteMatchResponse) updates) =>
      super.copyWith((message) => updates(message as CompleteMatchResponse))
          as CompleteMatchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompleteMatchResponse create() => CompleteMatchResponse._();
  @$core.override
  CompleteMatchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompleteMatchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompleteMatchResponse>(create);
  static CompleteMatchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Match get match => $_getN(0);
  @$pb.TagNumber(1)
  set match(Match value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMatch() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatch() => $_clearField(1);
  @$pb.TagNumber(1)
  Match ensureMatch() => $_ensure(0);
}

class RateMatchRequest extends $pb.GeneratedMessage {
  factory RateMatchRequest({
    $core.String? matchId,
    $core.String? ratedProfileId,
    $core.int? stars,
  }) {
    final result = create();
    if (matchId != null) result.matchId = matchId;
    if (ratedProfileId != null) result.ratedProfileId = ratedProfileId;
    if (stars != null) result.stars = stars;
    return result;
  }

  RateMatchRequest._();

  factory RateMatchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RateMatchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RateMatchRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'matchId')
    ..aOS(2, _omitFieldNames ? '' : 'ratedProfileId')
    ..aI(3, _omitFieldNames ? '' : 'stars')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateMatchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateMatchRequest copyWith(void Function(RateMatchRequest) updates) =>
      super.copyWith((message) => updates(message as RateMatchRequest))
          as RateMatchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RateMatchRequest create() => RateMatchRequest._();
  @$core.override
  RateMatchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RateMatchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RateMatchRequest>(create);
  static RateMatchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get matchId => $_getSZ(0);
  @$pb.TagNumber(1)
  set matchId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get ratedProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set ratedProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRatedProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRatedProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get stars => $_getIZ(2);
  @$pb.TagNumber(3)
  set stars($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasStars() => $_has(2);
  @$pb.TagNumber(3)
  void clearStars() => $_clearField(3);
}

class GetPlayerRatingRequest extends $pb.GeneratedMessage {
  factory GetPlayerRatingRequest({
    $core.String? profileId,
    $core.String? gameId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (gameId != null) result.gameId = gameId;
    return result;
  }

  GetPlayerRatingRequest._();

  factory GetPlayerRatingRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPlayerRatingRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPlayerRatingRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'gameId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerRatingRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerRatingRequest copyWith(
          void Function(GetPlayerRatingRequest) updates) =>
      super.copyWith((message) => updates(message as GetPlayerRatingRequest))
          as GetPlayerRatingRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPlayerRatingRequest create() => GetPlayerRatingRequest._();
  @$core.override
  GetPlayerRatingRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPlayerRatingRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPlayerRatingRequest>(create);
  static GetPlayerRatingRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get gameId => $_getSZ(1);
  @$pb.TagNumber(2)
  set gameId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGameId() => $_has(1);
  @$pb.TagNumber(2)
  void clearGameId() => $_clearField(2);
}

class PlayerRating extends $pb.GeneratedMessage {
  factory PlayerRating({
    $core.String? profileId,
    $core.String? gameId,
    $core.double? ratingValue,
    $core.int? gamesPlayed,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (gameId != null) result.gameId = gameId;
    if (ratingValue != null) result.ratingValue = ratingValue;
    if (gamesPlayed != null) result.gamesPlayed = gamesPlayed;
    return result;
  }

  PlayerRating._();

  factory PlayerRating.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PlayerRating.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PlayerRating',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'gameId')
    ..aD(3, _omitFieldNames ? '' : 'ratingValue')
    ..aI(4, _omitFieldNames ? '' : 'gamesPlayed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlayerRating clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlayerRating copyWith(void Function(PlayerRating) updates) =>
      super.copyWith((message) => updates(message as PlayerRating))
          as PlayerRating;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PlayerRating create() => PlayerRating._();
  @$core.override
  PlayerRating createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PlayerRating getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PlayerRating>(create);
  static PlayerRating? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get gameId => $_getSZ(1);
  @$pb.TagNumber(2)
  set gameId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasGameId() => $_has(1);
  @$pb.TagNumber(2)
  void clearGameId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.double get ratingValue => $_getN(2);
  @$pb.TagNumber(3)
  set ratingValue($core.double value) => $_setDouble(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRatingValue() => $_has(2);
  @$pb.TagNumber(3)
  void clearRatingValue() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get gamesPlayed => $_getIZ(3);
  @$pb.TagNumber(4)
  set gamesPlayed($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGamesPlayed() => $_has(3);
  @$pb.TagNumber(4)
  void clearGamesPlayed() => $_clearField(4);
}

class BanFromMMRequest extends $pb.GeneratedMessage {
  factory BanFromMMRequest({
    $core.String? targetProfileId,
    $core.String? reason,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (reason != null) result.reason = reason;
    return result;
  }

  BanFromMMRequest._();

  factory BanFromMMRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanFromMMRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanFromMMRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanFromMMRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanFromMMRequest copyWith(void Function(BanFromMMRequest) updates) =>
      super.copyWith((message) => updates(message as BanFromMMRequest))
          as BanFromMMRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanFromMMRequest create() => BanFromMMRequest._();
  @$core.override
  BanFromMMRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanFromMMRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BanFromMMRequest>(create);
  static BanFromMMRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class UnbanFromMMRequest extends $pb.GeneratedMessage {
  factory UnbanFromMMRequest({
    $core.String? targetProfileId,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    return result;
  }

  UnbanFromMMRequest._();

  factory UnbanFromMMRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnbanFromMMRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnbanFromMMRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanFromMMRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanFromMMRequest copyWith(void Function(UnbanFromMMRequest) updates) =>
      super.copyWith((message) => updates(message as UnbanFromMMRequest))
          as UnbanFromMMRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnbanFromMMRequest create() => UnbanFromMMRequest._();
  @$core.override
  UnbanFromMMRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnbanFromMMRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnbanFromMMRequest>(create);
  static UnbanFromMMRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);
}

class GetMMBanStatusRequest extends $pb.GeneratedMessage {
  factory GetMMBanStatusRequest({
    $core.String? targetProfileId,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    return result;
  }

  GetMMBanStatusRequest._();

  factory GetMMBanStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMMBanStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMMBanStatusRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMMBanStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMMBanStatusRequest copyWith(
          void Function(GetMMBanStatusRequest) updates) =>
      super.copyWith((message) => updates(message as GetMMBanStatusRequest))
          as GetMMBanStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMMBanStatusRequest create() => GetMMBanStatusRequest._();
  @$core.override
  GetMMBanStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMMBanStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMMBanStatusRequest>(create);
  static GetMMBanStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);
}

class MMBanStatus extends $pb.GeneratedMessage {
  factory MMBanStatus({
    $core.bool? banned,
    $1.Timestamp? until,
  }) {
    final result = create();
    if (banned != null) result.banned = banned;
    if (until != null) result.until = until;
    return result;
  }

  MMBanStatus._();

  factory MMBanStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MMBanStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MMBanStatus',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'banned')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'until',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MMBanStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MMBanStatus copyWith(void Function(MMBanStatus) updates) =>
      super.copyWith((message) => updates(message as MMBanStatus))
          as MMBanStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MMBanStatus create() => MMBanStatus._();
  @$core.override
  MMBanStatus createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MMBanStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MMBanStatus>(create);
  static MMBanStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get banned => $_getBF(0);
  @$pb.TagNumber(1)
  set banned($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBanned() => $_has(0);
  @$pb.TagNumber(1)
  void clearBanned() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get until => $_getN(1);
  @$pb.TagNumber(2)
  set until($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasUntil() => $_has(1);
  @$pb.TagNumber(2)
  void clearUntil() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureUntil() => $_ensure(1);
}

class ListGamesResponse extends $pb.GeneratedMessage {
  factory ListGamesResponse({
    GameList? gameList,
  }) {
    final result = create();
    if (gameList != null) result.gameList = gameList;
    return result;
  }

  ListGamesResponse._();

  factory ListGamesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListGamesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListGamesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<GameList>(1, _omitFieldNames ? '' : 'gameList',
        subBuilder: GameList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListGamesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListGamesResponse copyWith(void Function(ListGamesResponse) updates) =>
      super.copyWith((message) => updates(message as ListGamesResponse))
          as ListGamesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListGamesResponse create() => ListGamesResponse._();
  @$core.override
  ListGamesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListGamesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListGamesResponse>(create);
  static ListGamesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  GameList get gameList => $_getN(0);
  @$pb.TagNumber(1)
  set gameList(GameList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGameList() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameList() => $_clearField(1);
  @$pb.TagNumber(1)
  GameList ensureGameList() => $_ensure(0);
}

class GetGameResponse extends $pb.GeneratedMessage {
  factory GetGameResponse({
    Game? game,
  }) {
    final result = create();
    if (game != null) result.game = game;
    return result;
  }

  GetGameResponse._();

  factory GetGameResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetGameResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetGameResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Game>(1, _omitFieldNames ? '' : 'game', subBuilder: Game.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetGameResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetGameResponse copyWith(void Function(GetGameResponse) updates) =>
      super.copyWith((message) => updates(message as GetGameResponse))
          as GetGameResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetGameResponse create() => GetGameResponse._();
  @$core.override
  GetGameResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetGameResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetGameResponse>(create);
  static GetGameResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Game get game => $_getN(0);
  @$pb.TagNumber(1)
  set game(Game value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGame() => $_has(0);
  @$pb.TagNumber(1)
  void clearGame() => $_clearField(1);
  @$pb.TagNumber(1)
  Game ensureGame() => $_ensure(0);
}

class CreateGameResponse extends $pb.GeneratedMessage {
  factory CreateGameResponse({
    Game? game,
  }) {
    final result = create();
    if (game != null) result.game = game;
    return result;
  }

  CreateGameResponse._();

  factory CreateGameResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateGameResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateGameResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Game>(1, _omitFieldNames ? '' : 'game', subBuilder: Game.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateGameResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateGameResponse copyWith(void Function(CreateGameResponse) updates) =>
      super.copyWith((message) => updates(message as CreateGameResponse))
          as CreateGameResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateGameResponse create() => CreateGameResponse._();
  @$core.override
  CreateGameResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateGameResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateGameResponse>(create);
  static CreateGameResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Game get game => $_getN(0);
  @$pb.TagNumber(1)
  set game(Game value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGame() => $_has(0);
  @$pb.TagNumber(1)
  void clearGame() => $_clearField(1);
  @$pb.TagNumber(1)
  Game ensureGame() => $_ensure(0);
}

class UpdateGameResponse extends $pb.GeneratedMessage {
  factory UpdateGameResponse({
    Game? game,
  }) {
    final result = create();
    if (game != null) result.game = game;
    return result;
  }

  UpdateGameResponse._();

  factory UpdateGameResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateGameResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateGameResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Game>(1, _omitFieldNames ? '' : 'game', subBuilder: Game.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateGameResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateGameResponse copyWith(void Function(UpdateGameResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateGameResponse))
          as UpdateGameResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateGameResponse create() => UpdateGameResponse._();
  @$core.override
  UpdateGameResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateGameResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateGameResponse>(create);
  static UpdateGameResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Game get game => $_getN(0);
  @$pb.TagNumber(1)
  set game(Game value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGame() => $_has(0);
  @$pb.TagNumber(1)
  void clearGame() => $_clearField(1);
  @$pb.TagNumber(1)
  Game ensureGame() => $_ensure(0);
}

class SearchGamesResponse extends $pb.GeneratedMessage {
  factory SearchGamesResponse({
    GameList? gameList,
  }) {
    final result = create();
    if (gameList != null) result.gameList = gameList;
    return result;
  }

  SearchGamesResponse._();

  factory SearchGamesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchGamesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchGamesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<GameList>(1, _omitFieldNames ? '' : 'gameList',
        subBuilder: GameList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGamesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGamesResponse copyWith(void Function(SearchGamesResponse) updates) =>
      super.copyWith((message) => updates(message as SearchGamesResponse))
          as SearchGamesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchGamesResponse create() => SearchGamesResponse._();
  @$core.override
  SearchGamesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchGamesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchGamesResponse>(create);
  static SearchGamesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  GameList get gameList => $_getN(0);
  @$pb.TagNumber(1)
  set gameList(GameList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGameList() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameList() => $_clearField(1);
  @$pb.TagNumber(1)
  GameList ensureGameList() => $_ensure(0);
}

class StartSearchResponse extends $pb.GeneratedMessage {
  factory StartSearchResponse({
    SearchSession? searchSession,
  }) {
    final result = create();
    if (searchSession != null) result.searchSession = searchSession;
    return result;
  }

  StartSearchResponse._();

  factory StartSearchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory StartSearchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'StartSearchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<SearchSession>(1, _omitFieldNames ? '' : 'searchSession',
        subBuilder: SearchSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartSearchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  StartSearchResponse copyWith(void Function(StartSearchResponse) updates) =>
      super.copyWith((message) => updates(message as StartSearchResponse))
          as StartSearchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static StartSearchResponse create() => StartSearchResponse._();
  @$core.override
  StartSearchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static StartSearchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<StartSearchResponse>(create);
  static StartSearchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SearchSession get searchSession => $_getN(0);
  @$pb.TagNumber(1)
  set searchSession(SearchSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSearchSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSearchSession() => $_clearField(1);
  @$pb.TagNumber(1)
  SearchSession ensureSearchSession() => $_ensure(0);
}

class CancelSearchResponse extends $pb.GeneratedMessage {
  factory CancelSearchResponse() => create();

  CancelSearchResponse._();

  factory CancelSearchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CancelSearchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelSearchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSearchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSearchResponse copyWith(void Function(CancelSearchResponse) updates) =>
      super.copyWith((message) => updates(message as CancelSearchResponse))
          as CancelSearchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CancelSearchResponse create() => CancelSearchResponse._();
  @$core.override
  CancelSearchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CancelSearchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelSearchResponse>(create);
  static CancelSearchResponse? _defaultInstance;
}

class GetSearchStatusResponse extends $pb.GeneratedMessage {
  factory GetSearchStatusResponse({
    SearchSession? searchSession,
  }) {
    final result = create();
    if (searchSession != null) result.searchSession = searchSession;
    return result;
  }

  GetSearchStatusResponse._();

  factory GetSearchStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSearchStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSearchStatusResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<SearchSession>(1, _omitFieldNames ? '' : 'searchSession',
        subBuilder: SearchSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSearchStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSearchStatusResponse copyWith(
          void Function(GetSearchStatusResponse) updates) =>
      super.copyWith((message) => updates(message as GetSearchStatusResponse))
          as GetSearchStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSearchStatusResponse create() => GetSearchStatusResponse._();
  @$core.override
  GetSearchStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSearchStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSearchStatusResponse>(create);
  static GetSearchStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SearchSession get searchSession => $_getN(0);
  @$pb.TagNumber(1)
  set searchSession(SearchSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSearchSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSearchSession() => $_clearField(1);
  @$pb.TagNumber(1)
  SearchSession ensureSearchSession() => $_ensure(0);
}

class GetMatchResponse extends $pb.GeneratedMessage {
  factory GetMatchResponse({
    Match? match,
  }) {
    final result = create();
    if (match != null) result.match = match;
    return result;
  }

  GetMatchResponse._();

  factory GetMatchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<Match>(1, _omitFieldNames ? '' : 'match', subBuilder: Match.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchResponse copyWith(void Function(GetMatchResponse) updates) =>
      super.copyWith((message) => updates(message as GetMatchResponse))
          as GetMatchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchResponse create() => GetMatchResponse._();
  @$core.override
  GetMatchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchResponse>(create);
  static GetMatchResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Match get match => $_getN(0);
  @$pb.TagNumber(1)
  set match(Match value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMatch() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatch() => $_clearField(1);
  @$pb.TagNumber(1)
  Match ensureMatch() => $_ensure(0);
}

class GetMatchHistoryResponse extends $pb.GeneratedMessage {
  factory GetMatchHistoryResponse({
    MatchList? matchList,
  }) {
    final result = create();
    if (matchList != null) result.matchList = matchList;
    return result;
  }

  GetMatchHistoryResponse._();

  factory GetMatchHistoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMatchHistoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMatchHistoryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<MatchList>(1, _omitFieldNames ? '' : 'matchList',
        subBuilder: MatchList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchHistoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMatchHistoryResponse copyWith(
          void Function(GetMatchHistoryResponse) updates) =>
      super.copyWith((message) => updates(message as GetMatchHistoryResponse))
          as GetMatchHistoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMatchHistoryResponse create() => GetMatchHistoryResponse._();
  @$core.override
  GetMatchHistoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMatchHistoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMatchHistoryResponse>(create);
  static GetMatchHistoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MatchList get matchList => $_getN(0);
  @$pb.TagNumber(1)
  set matchList(MatchList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMatchList() => $_has(0);
  @$pb.TagNumber(1)
  void clearMatchList() => $_clearField(1);
  @$pb.TagNumber(1)
  MatchList ensureMatchList() => $_ensure(0);
}

class RateMatchResponse extends $pb.GeneratedMessage {
  factory RateMatchResponse() => create();

  RateMatchResponse._();

  factory RateMatchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RateMatchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RateMatchResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateMatchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RateMatchResponse copyWith(void Function(RateMatchResponse) updates) =>
      super.copyWith((message) => updates(message as RateMatchResponse))
          as RateMatchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RateMatchResponse create() => RateMatchResponse._();
  @$core.override
  RateMatchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RateMatchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RateMatchResponse>(create);
  static RateMatchResponse? _defaultInstance;
}

class GetPlayerRatingResponse extends $pb.GeneratedMessage {
  factory GetPlayerRatingResponse({
    PlayerRating? playerRating,
  }) {
    final result = create();
    if (playerRating != null) result.playerRating = playerRating;
    return result;
  }

  GetPlayerRatingResponse._();

  factory GetPlayerRatingResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPlayerRatingResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPlayerRatingResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<PlayerRating>(1, _omitFieldNames ? '' : 'playerRating',
        subBuilder: PlayerRating.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerRatingResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerRatingResponse copyWith(
          void Function(GetPlayerRatingResponse) updates) =>
      super.copyWith((message) => updates(message as GetPlayerRatingResponse))
          as GetPlayerRatingResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPlayerRatingResponse create() => GetPlayerRatingResponse._();
  @$core.override
  GetPlayerRatingResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPlayerRatingResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPlayerRatingResponse>(create);
  static GetPlayerRatingResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PlayerRating get playerRating => $_getN(0);
  @$pb.TagNumber(1)
  set playerRating(PlayerRating value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPlayerRating() => $_has(0);
  @$pb.TagNumber(1)
  void clearPlayerRating() => $_clearField(1);
  @$pb.TagNumber(1)
  PlayerRating ensurePlayerRating() => $_ensure(0);
}

class BanFromMMResponse extends $pb.GeneratedMessage {
  factory BanFromMMResponse() => create();

  BanFromMMResponse._();

  factory BanFromMMResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BanFromMMResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BanFromMMResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanFromMMResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BanFromMMResponse copyWith(void Function(BanFromMMResponse) updates) =>
      super.copyWith((message) => updates(message as BanFromMMResponse))
          as BanFromMMResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BanFromMMResponse create() => BanFromMMResponse._();
  @$core.override
  BanFromMMResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BanFromMMResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BanFromMMResponse>(create);
  static BanFromMMResponse? _defaultInstance;
}

class UnbanFromMMResponse extends $pb.GeneratedMessage {
  factory UnbanFromMMResponse() => create();

  UnbanFromMMResponse._();

  factory UnbanFromMMResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnbanFromMMResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnbanFromMMResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanFromMMResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnbanFromMMResponse copyWith(void Function(UnbanFromMMResponse) updates) =>
      super.copyWith((message) => updates(message as UnbanFromMMResponse))
          as UnbanFromMMResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnbanFromMMResponse create() => UnbanFromMMResponse._();
  @$core.override
  UnbanFromMMResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnbanFromMMResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnbanFromMMResponse>(create);
  static UnbanFromMMResponse? _defaultInstance;
}

class GetMMBanStatusResponse extends $pb.GeneratedMessage {
  factory GetMMBanStatusResponse({
    MMBanStatus? mmBanStatus,
  }) {
    final result = create();
    if (mmBanStatus != null) result.mmBanStatus = mmBanStatus;
    return result;
  }

  GetMMBanStatusResponse._();

  factory GetMMBanStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMMBanStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMMBanStatusResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<MMBanStatus>(1, _omitFieldNames ? '' : 'mmBanStatus',
        subBuilder: MMBanStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMMBanStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMMBanStatusResponse copyWith(
          void Function(GetMMBanStatusResponse) updates) =>
      super.copyWith((message) => updates(message as GetMMBanStatusResponse))
          as GetMMBanStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMMBanStatusResponse create() => GetMMBanStatusResponse._();
  @$core.override
  GetMMBanStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMMBanStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMMBanStatusResponse>(create);
  static GetMMBanStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  MMBanStatus get mmBanStatus => $_getN(0);
  @$pb.TagNumber(1)
  set mmBanStatus(MMBanStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMmBanStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearMmBanStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  MMBanStatus ensureMmBanStatus() => $_ensure(0);
}

class PlayerGameEntry extends $pb.GeneratedMessage {
  factory PlayerGameEntry({
    $core.String? gameId,
    $core.String? region,
    $core.String? role,
    $core.String? rank,
    $1.Timestamp? updatedAt,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    if (region != null) result.region = region;
    if (role != null) result.role = role;
    if (rank != null) result.rank = rank;
    if (updatedAt != null) result.updatedAt = updatedAt;
    return result;
  }

  PlayerGameEntry._();

  factory PlayerGameEntry.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PlayerGameEntry.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PlayerGameEntry',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..aOS(2, _omitFieldNames ? '' : 'region')
    ..aOS(3, _omitFieldNames ? '' : 'role')
    ..aOS(4, _omitFieldNames ? '' : 'rank')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'updatedAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlayerGameEntry clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PlayerGameEntry copyWith(void Function(PlayerGameEntry) updates) =>
      super.copyWith((message) => updates(message as PlayerGameEntry))
          as PlayerGameEntry;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PlayerGameEntry create() => PlayerGameEntry._();
  @$core.override
  PlayerGameEntry createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PlayerGameEntry getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PlayerGameEntry>(create);
  static PlayerGameEntry? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get region => $_getSZ(1);
  @$pb.TagNumber(2)
  set region($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRegion() => $_has(1);
  @$pb.TagNumber(2)
  void clearRegion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get role => $_getSZ(2);
  @$pb.TagNumber(3)
  set role($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRole() => $_has(2);
  @$pb.TagNumber(3)
  void clearRole() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get rank => $_getSZ(3);
  @$pb.TagNumber(4)
  set rank($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRank() => $_has(3);
  @$pb.TagNumber(4)
  void clearRank() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get updatedAt => $_getN(4);
  @$pb.TagNumber(5)
  set updatedAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasUpdatedAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearUpdatedAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureUpdatedAt() => $_ensure(4);
}

class GetMyPlayerProfileRequest extends $pb.GeneratedMessage {
  factory GetMyPlayerProfileRequest() => create();

  GetMyPlayerProfileRequest._();

  factory GetMyPlayerProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMyPlayerProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyPlayerProfileRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPlayerProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPlayerProfileRequest copyWith(
          void Function(GetMyPlayerProfileRequest) updates) =>
      super.copyWith((message) => updates(message as GetMyPlayerProfileRequest))
          as GetMyPlayerProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMyPlayerProfileRequest create() => GetMyPlayerProfileRequest._();
  @$core.override
  GetMyPlayerProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMyPlayerProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyPlayerProfileRequest>(create);
  static GetMyPlayerProfileRequest? _defaultInstance;
}

class GetMyPlayerProfileResponse extends $pb.GeneratedMessage {
  factory GetMyPlayerProfileResponse({
    $core.Iterable<PlayerGameEntry>? entries,
  }) {
    final result = create();
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  GetMyPlayerProfileResponse._();

  factory GetMyPlayerProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMyPlayerProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMyPlayerProfileResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..pPM<PlayerGameEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: PlayerGameEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPlayerProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMyPlayerProfileResponse copyWith(
          void Function(GetMyPlayerProfileResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetMyPlayerProfileResponse))
          as GetMyPlayerProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMyPlayerProfileResponse create() => GetMyPlayerProfileResponse._();
  @$core.override
  GetMyPlayerProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMyPlayerProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMyPlayerProfileResponse>(create);
  static GetMyPlayerProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PlayerGameEntry> get entries => $_getList(0);
}

class GetPlayerProfileRequest extends $pb.GeneratedMessage {
  factory GetPlayerProfileRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetPlayerProfileRequest._();

  factory GetPlayerProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPlayerProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPlayerProfileRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerProfileRequest copyWith(
          void Function(GetPlayerProfileRequest) updates) =>
      super.copyWith((message) => updates(message as GetPlayerProfileRequest))
          as GetPlayerProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPlayerProfileRequest create() => GetPlayerProfileRequest._();
  @$core.override
  GetPlayerProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPlayerProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPlayerProfileRequest>(create);
  static GetPlayerProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class GetPlayerProfileResponse extends $pb.GeneratedMessage {
  factory GetPlayerProfileResponse({
    $core.Iterable<PlayerGameEntry>? entries,
  }) {
    final result = create();
    if (entries != null) result.entries.addAll(entries);
    return result;
  }

  GetPlayerProfileResponse._();

  factory GetPlayerProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetPlayerProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetPlayerProfileResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..pPM<PlayerGameEntry>(1, _omitFieldNames ? '' : 'entries',
        subBuilder: PlayerGameEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetPlayerProfileResponse copyWith(
          void Function(GetPlayerProfileResponse) updates) =>
      super.copyWith((message) => updates(message as GetPlayerProfileResponse))
          as GetPlayerProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetPlayerProfileResponse create() => GetPlayerProfileResponse._();
  @$core.override
  GetPlayerProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetPlayerProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetPlayerProfileResponse>(create);
  static GetPlayerProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PlayerGameEntry> get entries => $_getList(0);
}

class UpsertPlayerGameEntryRequest extends $pb.GeneratedMessage {
  factory UpsertPlayerGameEntryRequest({
    $core.String? gameId,
    $core.String? region,
    $core.String? role,
    $core.String? rank,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    if (region != null) result.region = region;
    if (role != null) result.role = role;
    if (rank != null) result.rank = rank;
    return result;
  }

  UpsertPlayerGameEntryRequest._();

  factory UpsertPlayerGameEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpsertPlayerGameEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertPlayerGameEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..aOS(2, _omitFieldNames ? '' : 'region')
    ..aOS(3, _omitFieldNames ? '' : 'role')
    ..aOS(4, _omitFieldNames ? '' : 'rank')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertPlayerGameEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertPlayerGameEntryRequest copyWith(
          void Function(UpsertPlayerGameEntryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as UpsertPlayerGameEntryRequest))
          as UpsertPlayerGameEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpsertPlayerGameEntryRequest create() =>
      UpsertPlayerGameEntryRequest._();
  @$core.override
  UpsertPlayerGameEntryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpsertPlayerGameEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertPlayerGameEntryRequest>(create);
  static UpsertPlayerGameEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get region => $_getSZ(1);
  @$pb.TagNumber(2)
  set region($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRegion() => $_has(1);
  @$pb.TagNumber(2)
  void clearRegion() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get role => $_getSZ(2);
  @$pb.TagNumber(3)
  set role($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRole() => $_has(2);
  @$pb.TagNumber(3)
  void clearRole() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get rank => $_getSZ(3);
  @$pb.TagNumber(4)
  set rank($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRank() => $_has(3);
  @$pb.TagNumber(4)
  void clearRank() => $_clearField(4);
}

class UpsertPlayerGameEntryResponse extends $pb.GeneratedMessage {
  factory UpsertPlayerGameEntryResponse({
    PlayerGameEntry? entry,
  }) {
    final result = create();
    if (entry != null) result.entry = entry;
    return result;
  }

  UpsertPlayerGameEntryResponse._();

  factory UpsertPlayerGameEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpsertPlayerGameEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpsertPlayerGameEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOM<PlayerGameEntry>(1, _omitFieldNames ? '' : 'entry',
        subBuilder: PlayerGameEntry.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertPlayerGameEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpsertPlayerGameEntryResponse copyWith(
          void Function(UpsertPlayerGameEntryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as UpsertPlayerGameEntryResponse))
          as UpsertPlayerGameEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpsertPlayerGameEntryResponse create() =>
      UpsertPlayerGameEntryResponse._();
  @$core.override
  UpsertPlayerGameEntryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpsertPlayerGameEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpsertPlayerGameEntryResponse>(create);
  static UpsertPlayerGameEntryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PlayerGameEntry get entry => $_getN(0);
  @$pb.TagNumber(1)
  set entry(PlayerGameEntry value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEntry() => $_has(0);
  @$pb.TagNumber(1)
  void clearEntry() => $_clearField(1);
  @$pb.TagNumber(1)
  PlayerGameEntry ensureEntry() => $_ensure(0);
}

class DeletePlayerGameEntryRequest extends $pb.GeneratedMessage {
  factory DeletePlayerGameEntryRequest({
    $core.String? gameId,
  }) {
    final result = create();
    if (gameId != null) result.gameId = gameId;
    return result;
  }

  DeletePlayerGameEntryRequest._();

  factory DeletePlayerGameEntryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeletePlayerGameEntryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeletePlayerGameEntryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'gameId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePlayerGameEntryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePlayerGameEntryRequest copyWith(
          void Function(DeletePlayerGameEntryRequest) updates) =>
      super.copyWith(
              (message) => updates(message as DeletePlayerGameEntryRequest))
          as DeletePlayerGameEntryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeletePlayerGameEntryRequest create() =>
      DeletePlayerGameEntryRequest._();
  @$core.override
  DeletePlayerGameEntryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeletePlayerGameEntryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeletePlayerGameEntryRequest>(create);
  static DeletePlayerGameEntryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get gameId => $_getSZ(0);
  @$pb.TagNumber(1)
  set gameId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGameId() => $_has(0);
  @$pb.TagNumber(1)
  void clearGameId() => $_clearField(1);
}

class DeletePlayerGameEntryResponse extends $pb.GeneratedMessage {
  factory DeletePlayerGameEntryResponse() => create();

  DeletePlayerGameEntryResponse._();

  factory DeletePlayerGameEntryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeletePlayerGameEntryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeletePlayerGameEntryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.matchmaking.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePlayerGameEntryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeletePlayerGameEntryResponse copyWith(
          void Function(DeletePlayerGameEntryResponse) updates) =>
      super.copyWith(
              (message) => updates(message as DeletePlayerGameEntryResponse))
          as DeletePlayerGameEntryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeletePlayerGameEntryResponse create() =>
      DeletePlayerGameEntryResponse._();
  @$core.override
  DeletePlayerGameEntryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeletePlayerGameEntryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeletePlayerGameEntryResponse>(create);
  static DeletePlayerGameEntryResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
