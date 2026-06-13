// This is a generated file - do not edit.
//
// Generated from voice/search/v1/search.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import '../../chat/v1/chat.pb.dart' as $1;
import '../../common/v1/common.pb.dart' as $2;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class SearchInChatRequest extends $pb.GeneratedMessage {
  factory SearchInChatRequest({
    $1.ChatRef? chat,
    $core.String? query,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (query != null) result.query = query;
    if (page != null) result.page = page;
    return result;
  }

  SearchInChatRequest._();

  factory SearchInChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchInChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchInChatRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'query')
    ..aOM<$2.CursorPageRequest>(3, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchInChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchInChatRequest copyWith(void Function(SearchInChatRequest) updates) =>
      super.copyWith((message) => updates(message as SearchInChatRequest))
          as SearchInChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchInChatRequest create() => SearchInChatRequest._();
  @$core.override
  SearchInChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchInChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchInChatRequest>(create);
  static SearchInChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get query => $_getSZ(1);
  @$pb.TagNumber(2)
  set query($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasQuery() => $_has(1);
  @$pb.TagNumber(2)
  void clearQuery() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.CursorPageRequest get page => $_getN(2);
  @$pb.TagNumber(3)
  set page($2.CursorPageRequest value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasPage() => $_has(2);
  @$pb.TagNumber(3)
  void clearPage() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.CursorPageRequest ensurePage() => $_ensure(2);
}

class SearchResults extends $pb.GeneratedMessage {
  factory SearchResults({
    $core.Iterable<SearchHit>? hits,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (hits != null) result.hits.addAll(hits);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  SearchResults._();

  factory SearchResults.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchResults.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchResults',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..pPM<SearchHit>(1, _omitFieldNames ? '' : 'hits',
        subBuilder: SearchHit.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchResults clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchResults copyWith(void Function(SearchResults) updates) =>
      super.copyWith((message) => updates(message as SearchResults))
          as SearchResults;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchResults create() => SearchResults._();
  @$core.override
  SearchResults createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchResults getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchResults>(create);
  static SearchResults? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SearchHit> get hits => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class SearchHit extends $pb.GeneratedMessage {
  factory SearchHit({
    $core.String? messageId,
    $core.String? snippet,
    $core.double? score,
    $core.String? chatId,
  }) {
    final result = create();
    if (messageId != null) result.messageId = messageId;
    if (snippet != null) result.snippet = snippet;
    if (score != null) result.score = score;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  SearchHit._();

  factory SearchHit.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchHit.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchHit',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'messageId')
    ..aOS(2, _omitFieldNames ? '' : 'snippet')
    ..aD(3, _omitFieldNames ? '' : 'score')
    ..aOS(4, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchHit clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchHit copyWith(void Function(SearchHit) updates) =>
      super.copyWith((message) => updates(message as SearchHit)) as SearchHit;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchHit create() => SearchHit._();
  @$core.override
  SearchHit createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchHit getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<SearchHit>(create);
  static SearchHit? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get messageId => $_getSZ(0);
  @$pb.TagNumber(1)
  set messageId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessageId() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessageId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get snippet => $_getSZ(1);
  @$pb.TagNumber(2)
  set snippet($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSnippet() => $_has(1);
  @$pb.TagNumber(2)
  void clearSnippet() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.double get score => $_getN(2);
  @$pb.TagNumber(3)
  set score($core.double value) => $_setDouble(2, value);
  @$pb.TagNumber(3)
  $core.bool hasScore() => $_has(2);
  @$pb.TagNumber(3)
  void clearScore() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get chatId => $_getSZ(3);
  @$pb.TagNumber(4)
  set chatId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasChatId() => $_has(3);
  @$pb.TagNumber(4)
  void clearChatId() => $_clearField(4);
}

class SearchGlobalRequest extends $pb.GeneratedMessage {
  factory SearchGlobalRequest({
    $core.String? query,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (query != null) result.query = query;
    if (page != null) result.page = page;
    return result;
  }

  SearchGlobalRequest._();

  factory SearchGlobalRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchGlobalRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchGlobalRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGlobalRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGlobalRequest copyWith(void Function(SearchGlobalRequest) updates) =>
      super.copyWith((message) => updates(message as SearchGlobalRequest))
          as SearchGlobalRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchGlobalRequest create() => SearchGlobalRequest._();
  @$core.override
  SearchGlobalRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchGlobalRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchGlobalRequest>(create);
  static SearchGlobalRequest? _defaultInstance;

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

class GlobalSearchResults extends $pb.GeneratedMessage {
  factory GlobalSearchResults({
    $core.Iterable<SearchHit>? messages,
    $core.Iterable<$core.String>? profileIds,
    $core.Iterable<$1.ChatRef>? matchedChats,
    $core.Iterable<$core.String>? spaceIds,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (messages != null) result.messages.addAll(messages);
    if (profileIds != null) result.profileIds.addAll(profileIds);
    if (matchedChats != null) result.matchedChats.addAll(matchedChats);
    if (spaceIds != null) result.spaceIds.addAll(spaceIds);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  GlobalSearchResults._();

  factory GlobalSearchResults.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GlobalSearchResults.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GlobalSearchResults',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..pPM<SearchHit>(1, _omitFieldNames ? '' : 'messages',
        subBuilder: SearchHit.create)
    ..pPS(2, _omitFieldNames ? '' : 'profileIds')
    ..pPM<$1.ChatRef>(3, _omitFieldNames ? '' : 'matchedChats',
        subBuilder: $1.ChatRef.create)
    ..pPS(4, _omitFieldNames ? '' : 'spaceIds')
    ..aOS(5, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GlobalSearchResults clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GlobalSearchResults copyWith(void Function(GlobalSearchResults) updates) =>
      super.copyWith((message) => updates(message as GlobalSearchResults))
          as GlobalSearchResults;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GlobalSearchResults create() => GlobalSearchResults._();
  @$core.override
  GlobalSearchResults createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GlobalSearchResults getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GlobalSearchResults>(create);
  static GlobalSearchResults? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<SearchHit> get messages => $_getList(0);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get profileIds => $_getList(1);

  @$pb.TagNumber(3)
  $pb.PbList<$1.ChatRef> get matchedChats => $_getList(2);

  @$pb.TagNumber(4)
  $pb.PbList<$core.String> get spaceIds => $_getList(3);

  @$pb.TagNumber(5)
  $core.String get nextCursor => $_getSZ(4);
  @$pb.TagNumber(5)
  set nextCursor($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasNextCursor() => $_has(4);
  @$pb.TagNumber(5)
  void clearNextCursor() => $_clearField(5);
}

class SearchUsersRequest extends $pb.GeneratedMessage {
  factory SearchUsersRequest({
    $core.String? query,
    $core.int? limit,
  }) {
    final result = create();
    if (query != null) result.query = query;
    if (limit != null) result.limit = limit;
    return result;
  }

  SearchUsersRequest._();

  factory SearchUsersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchUsersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchUsersRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aI(2, _omitFieldNames ? '' : 'limit')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchUsersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchUsersRequest copyWith(void Function(SearchUsersRequest) updates) =>
      super.copyWith((message) => updates(message as SearchUsersRequest))
          as SearchUsersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchUsersRequest create() => SearchUsersRequest._();
  @$core.override
  SearchUsersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchUsersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchUsersRequest>(create);
  static SearchUsersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get query => $_getSZ(0);
  @$pb.TagNumber(1)
  set query($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasQuery() => $_has(0);
  @$pb.TagNumber(1)
  void clearQuery() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get limit => $_getIZ(1);
  @$pb.TagNumber(2)
  set limit($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimit() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimit() => $_clearField(2);
}

class UserSearchResults extends $pb.GeneratedMessage {
  factory UserSearchResults({
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  UserSearchResults._();

  factory UserSearchResults.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UserSearchResults.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UserSearchResults',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSearchResults clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UserSearchResults copyWith(void Function(UserSearchResults) updates) =>
      super.copyWith((message) => updates(message as UserSearchResults))
          as UserSearchResults;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UserSearchResults create() => UserSearchResults._();
  @$core.override
  UserSearchResults createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UserSearchResults getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UserSearchResults>(create);
  static UserSearchResults? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get profileIds => $_getList(0);
}

class SearchSpacesRequest extends $pb.GeneratedMessage {
  factory SearchSpacesRequest({
    $core.String? query,
    $2.CursorPageRequest? page,
  }) {
    final result = create();
    if (query != null) result.query = query;
    if (page != null) result.page = page;
    return result;
  }

  SearchSpacesRequest._();

  factory SearchSpacesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchSpacesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchSpacesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'query')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSpacesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSpacesRequest copyWith(void Function(SearchSpacesRequest) updates) =>
      super.copyWith((message) => updates(message as SearchSpacesRequest))
          as SearchSpacesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchSpacesRequest create() => SearchSpacesRequest._();
  @$core.override
  SearchSpacesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchSpacesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchSpacesRequest>(create);
  static SearchSpacesRequest? _defaultInstance;

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

class SpaceSearchResults extends $pb.GeneratedMessage {
  factory SpaceSearchResults({
    $core.Iterable<$core.String>? spaceIds,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (spaceIds != null) result.spaceIds.addAll(spaceIds);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  SpaceSearchResults._();

  factory SpaceSearchResults.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceSearchResults.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceSearchResults',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'spaceIds')
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceSearchResults clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceSearchResults copyWith(void Function(SpaceSearchResults) updates) =>
      super.copyWith((message) => updates(message as SpaceSearchResults))
          as SpaceSearchResults;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceSearchResults create() => SpaceSearchResults._();
  @$core.override
  SpaceSearchResults createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceSearchResults getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceSearchResults>(create);
  static SpaceSearchResults? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get spaceIds => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class ReindexChatRequest extends $pb.GeneratedMessage {
  factory ReindexChatRequest({
    $1.ChatRef? chat,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    return result;
  }

  ReindexChatRequest._();

  factory ReindexChatRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReindexChatRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReindexChatRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<$1.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $1.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReindexChatRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReindexChatRequest copyWith(void Function(ReindexChatRequest) updates) =>
      super.copyWith((message) => updates(message as ReindexChatRequest))
          as ReindexChatRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReindexChatRequest create() => ReindexChatRequest._();
  @$core.override
  ReindexChatRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReindexChatRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReindexChatRequest>(create);
  static ReindexChatRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($1.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.ChatRef ensureChat() => $_ensure(0);
}

class SearchInChatResponse extends $pb.GeneratedMessage {
  factory SearchInChatResponse({
    SearchResults? searchResults,
  }) {
    final result = create();
    if (searchResults != null) result.searchResults = searchResults;
    return result;
  }

  SearchInChatResponse._();

  factory SearchInChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchInChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchInChatResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<SearchResults>(1, _omitFieldNames ? '' : 'searchResults',
        subBuilder: SearchResults.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchInChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchInChatResponse copyWith(void Function(SearchInChatResponse) updates) =>
      super.copyWith((message) => updates(message as SearchInChatResponse))
          as SearchInChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchInChatResponse create() => SearchInChatResponse._();
  @$core.override
  SearchInChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchInChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchInChatResponse>(create);
  static SearchInChatResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SearchResults get searchResults => $_getN(0);
  @$pb.TagNumber(1)
  set searchResults(SearchResults value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSearchResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearSearchResults() => $_clearField(1);
  @$pb.TagNumber(1)
  SearchResults ensureSearchResults() => $_ensure(0);
}

class SearchGlobalResponse extends $pb.GeneratedMessage {
  factory SearchGlobalResponse({
    GlobalSearchResults? globalSearchResults,
  }) {
    final result = create();
    if (globalSearchResults != null)
      result.globalSearchResults = globalSearchResults;
    return result;
  }

  SearchGlobalResponse._();

  factory SearchGlobalResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchGlobalResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchGlobalResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<GlobalSearchResults>(1, _omitFieldNames ? '' : 'globalSearchResults',
        subBuilder: GlobalSearchResults.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGlobalResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchGlobalResponse copyWith(void Function(SearchGlobalResponse) updates) =>
      super.copyWith((message) => updates(message as SearchGlobalResponse))
          as SearchGlobalResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchGlobalResponse create() => SearchGlobalResponse._();
  @$core.override
  SearchGlobalResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchGlobalResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchGlobalResponse>(create);
  static SearchGlobalResponse? _defaultInstance;

  @$pb.TagNumber(1)
  GlobalSearchResults get globalSearchResults => $_getN(0);
  @$pb.TagNumber(1)
  set globalSearchResults(GlobalSearchResults value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGlobalSearchResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearGlobalSearchResults() => $_clearField(1);
  @$pb.TagNumber(1)
  GlobalSearchResults ensureGlobalSearchResults() => $_ensure(0);
}

class SearchUsersResponse extends $pb.GeneratedMessage {
  factory SearchUsersResponse({
    UserSearchResults? userSearchResults,
  }) {
    final result = create();
    if (userSearchResults != null) result.userSearchResults = userSearchResults;
    return result;
  }

  SearchUsersResponse._();

  factory SearchUsersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchUsersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchUsersResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<UserSearchResults>(1, _omitFieldNames ? '' : 'userSearchResults',
        subBuilder: UserSearchResults.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchUsersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchUsersResponse copyWith(void Function(SearchUsersResponse) updates) =>
      super.copyWith((message) => updates(message as SearchUsersResponse))
          as SearchUsersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchUsersResponse create() => SearchUsersResponse._();
  @$core.override
  SearchUsersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchUsersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchUsersResponse>(create);
  static SearchUsersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  UserSearchResults get userSearchResults => $_getN(0);
  @$pb.TagNumber(1)
  set userSearchResults(UserSearchResults value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasUserSearchResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearUserSearchResults() => $_clearField(1);
  @$pb.TagNumber(1)
  UserSearchResults ensureUserSearchResults() => $_ensure(0);
}

class SearchSpacesResponse extends $pb.GeneratedMessage {
  factory SearchSpacesResponse({
    SpaceSearchResults? spaceSearchResults,
  }) {
    final result = create();
    if (spaceSearchResults != null)
      result.spaceSearchResults = spaceSearchResults;
    return result;
  }

  SearchSpacesResponse._();

  factory SearchSpacesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SearchSpacesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SearchSpacesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceSearchResults>(1, _omitFieldNames ? '' : 'spaceSearchResults',
        subBuilder: SpaceSearchResults.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSpacesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SearchSpacesResponse copyWith(void Function(SearchSpacesResponse) updates) =>
      super.copyWith((message) => updates(message as SearchSpacesResponse))
          as SearchSpacesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SearchSpacesResponse create() => SearchSpacesResponse._();
  @$core.override
  SearchSpacesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SearchSpacesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SearchSpacesResponse>(create);
  static SearchSpacesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceSearchResults get spaceSearchResults => $_getN(0);
  @$pb.TagNumber(1)
  set spaceSearchResults(SpaceSearchResults value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceSearchResults() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceSearchResults() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceSearchResults ensureSpaceSearchResults() => $_ensure(0);
}

class ReindexChatResponse extends $pb.GeneratedMessage {
  factory ReindexChatResponse() => create();

  ReindexChatResponse._();

  factory ReindexChatResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReindexChatResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReindexChatResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.search.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReindexChatResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReindexChatResponse copyWith(void Function(ReindexChatResponse) updates) =>
      super.copyWith((message) => updates(message as ReindexChatResponse))
          as ReindexChatResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReindexChatResponse create() => ReindexChatResponse._();
  @$core.override
  ReindexChatResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReindexChatResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReindexChatResponse>(create);
  static ReindexChatResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
