// This is a generated file - do not edit.
//
// Generated from voice/social/v1/social.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import '../../common/v1/common.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class SendFriendInvitationRequest extends $pb.GeneratedMessage {
  factory SendFriendInvitationRequest({
    $core.String? targetProfileId,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    return result;
  }

  SendFriendInvitationRequest._();

  factory SendFriendInvitationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendFriendInvitationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendFriendInvitationRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendFriendInvitationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendFriendInvitationRequest copyWith(
          void Function(SendFriendInvitationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SendFriendInvitationRequest))
          as SendFriendInvitationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendFriendInvitationRequest create() =>
      SendFriendInvitationRequest._();
  @$core.override
  SendFriendInvitationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendFriendInvitationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendFriendInvitationRequest>(create);
  static SendFriendInvitationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);
}

class AcceptFriendInvitationRequest extends $pb.GeneratedMessage {
  factory AcceptFriendInvitationRequest({
    $core.String? requesterProfileId,
  }) {
    final result = create();
    if (requesterProfileId != null)
      result.requesterProfileId = requesterProfileId;
    return result;
  }

  AcceptFriendInvitationRequest._();

  factory AcceptFriendInvitationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptFriendInvitationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptFriendInvitationRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'requesterProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptFriendInvitationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptFriendInvitationRequest copyWith(
          void Function(AcceptFriendInvitationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AcceptFriendInvitationRequest))
          as AcceptFriendInvitationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptFriendInvitationRequest create() =>
      AcceptFriendInvitationRequest._();
  @$core.override
  AcceptFriendInvitationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptFriendInvitationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptFriendInvitationRequest>(create);
  static AcceptFriendInvitationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requesterProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requesterProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequesterProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequesterProfileId() => $_clearField(1);
}

class DeclineFriendInvitationRequest extends $pb.GeneratedMessage {
  factory DeclineFriendInvitationRequest({
    $core.String? requesterProfileId,
  }) {
    final result = create();
    if (requesterProfileId != null)
      result.requesterProfileId = requesterProfileId;
    return result;
  }

  DeclineFriendInvitationRequest._();

  factory DeclineFriendInvitationRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineFriendInvitationRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineFriendInvitationRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'requesterProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineFriendInvitationRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineFriendInvitationRequest copyWith(
          void Function(DeclineFriendInvitationRequest) updates) =>
      super.copyWith(
              (message) => updates(message as DeclineFriendInvitationRequest))
          as DeclineFriendInvitationRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineFriendInvitationRequest create() =>
      DeclineFriendInvitationRequest._();
  @$core.override
  DeclineFriendInvitationRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineFriendInvitationRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineFriendInvitationRequest>(create);
  static DeclineFriendInvitationRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get requesterProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set requesterProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRequesterProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRequesterProfileId() => $_clearField(1);
}

class RemoveFriendRequest extends $pb.GeneratedMessage {
  factory RemoveFriendRequest({
    $core.String? friendProfileId,
  }) {
    final result = create();
    if (friendProfileId != null) result.friendProfileId = friendProfileId;
    return result;
  }

  RemoveFriendRequest._();

  factory RemoveFriendRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveFriendRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFriendRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'friendProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFriendRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFriendRequest copyWith(void Function(RemoveFriendRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveFriendRequest))
          as RemoveFriendRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveFriendRequest create() => RemoveFriendRequest._();
  @$core.override
  RemoveFriendRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveFriendRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFriendRequest>(create);
  static RemoveFriendRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get friendProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set friendProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFriendProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriendProfileId() => $_clearField(1);
}

class ListFriendsRequest extends $pb.GeneratedMessage {
  factory ListFriendsRequest({
    $1.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ListFriendsRequest._();

  factory ListFriendsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFriendsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFriendsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<$1.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $1.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendsRequest copyWith(void Function(ListFriendsRequest) updates) =>
      super.copyWith((message) => updates(message as ListFriendsRequest))
          as ListFriendsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFriendsRequest create() => ListFriendsRequest._();
  @$core.override
  ListFriendsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFriendsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFriendsRequest>(create);
  static ListFriendsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.CursorPageRequest get page => $_getN(0);
  @$pb.TagNumber(1)
  set page($1.CursorPageRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.CursorPageRequest ensurePage() => $_ensure(0);
}

class FriendList extends $pb.GeneratedMessage {
  factory FriendList({
    $core.Iterable<FriendEdge>? friends,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (friends != null) result.friends.addAll(friends);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  FriendList._();

  factory FriendList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FriendList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FriendList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPM<FriendEdge>(1, _omitFieldNames ? '' : 'friends',
        subBuilder: FriendEdge.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendList copyWith(void Function(FriendList) updates) =>
      super.copyWith((message) => updates(message as FriendList)) as FriendList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FriendList create() => FriendList._();
  @$core.override
  FriendList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FriendList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FriendList>(create);
  static FriendList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<FriendEdge> get friends => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class FriendEdge extends $pb.GeneratedMessage {
  factory FriendEdge({
    $core.String? profileId,
    $2.Timestamp? friendsSince,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (friendsSince != null) result.friendsSince = friendsSince;
    return result;
  }

  FriendEdge._();

  factory FriendEdge.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FriendEdge.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FriendEdge',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'friendsSince',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendEdge clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendEdge copyWith(void Function(FriendEdge) updates) =>
      super.copyWith((message) => updates(message as FriendEdge)) as FriendEdge;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FriendEdge create() => FriendEdge._();
  @$core.override
  FriendEdge createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FriendEdge getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FriendEdge>(create);
  static FriendEdge? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.Timestamp get friendsSince => $_getN(1);
  @$pb.TagNumber(2)
  set friendsSince($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFriendsSince() => $_has(1);
  @$pb.TagNumber(2)
  void clearFriendsSince() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureFriendsSince() => $_ensure(1);
}

class ListFriendRequestsRequest extends $pb.GeneratedMessage {
  factory ListFriendRequestsRequest() => create();

  ListFriendRequestsRequest._();

  factory ListFriendRequestsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFriendRequestsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFriendRequestsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendRequestsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendRequestsRequest copyWith(
          void Function(ListFriendRequestsRequest) updates) =>
      super.copyWith((message) => updates(message as ListFriendRequestsRequest))
          as ListFriendRequestsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFriendRequestsRequest create() => ListFriendRequestsRequest._();
  @$core.override
  ListFriendRequestsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFriendRequestsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFriendRequestsRequest>(create);
  static ListFriendRequestsRequest? _defaultInstance;
}

class FriendRequestList extends $pb.GeneratedMessage {
  factory FriendRequestList({
    $core.Iterable<PendingFriendRequest>? incoming,
    $core.Iterable<PendingFriendRequest>? outgoing,
  }) {
    final result = create();
    if (incoming != null) result.incoming.addAll(incoming);
    if (outgoing != null) result.outgoing.addAll(outgoing);
    return result;
  }

  FriendRequestList._();

  factory FriendRequestList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FriendRequestList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FriendRequestList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPM<PendingFriendRequest>(1, _omitFieldNames ? '' : 'incoming',
        subBuilder: PendingFriendRequest.create)
    ..pPM<PendingFriendRequest>(2, _omitFieldNames ? '' : 'outgoing',
        subBuilder: PendingFriendRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendRequestList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FriendRequestList copyWith(void Function(FriendRequestList) updates) =>
      super.copyWith((message) => updates(message as FriendRequestList))
          as FriendRequestList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FriendRequestList create() => FriendRequestList._();
  @$core.override
  FriendRequestList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FriendRequestList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FriendRequestList>(create);
  static FriendRequestList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PendingFriendRequest> get incoming => $_getList(0);

  @$pb.TagNumber(2)
  $pb.PbList<PendingFriendRequest> get outgoing => $_getList(1);
}

class PendingFriendRequest extends $pb.GeneratedMessage {
  factory PendingFriendRequest({
    $core.String? profileId,
    $2.Timestamp? createdAt,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  PendingFriendRequest._();

  factory PendingFriendRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PendingFriendRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PendingFriendRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PendingFriendRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PendingFriendRequest copyWith(void Function(PendingFriendRequest) updates) =>
      super.copyWith((message) => updates(message as PendingFriendRequest))
          as PendingFriendRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PendingFriendRequest create() => PendingFriendRequest._();
  @$core.override
  PendingFriendRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PendingFriendRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PendingFriendRequest>(create);
  static PendingFriendRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.Timestamp get createdAt => $_getN(1);
  @$pb.TagNumber(2)
  set createdAt($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreatedAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreatedAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureCreatedAt() => $_ensure(1);
}

class AddContactRequest extends $pb.GeneratedMessage {
  factory AddContactRequest({
    $core.String? targetProfileId,
    $core.String? source,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (source != null) result.source = source;
    return result;
  }

  AddContactRequest._();

  factory AddContactRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddContactRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddContactRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..aOS(2, _omitFieldNames ? '' : 'source')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddContactRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddContactRequest copyWith(void Function(AddContactRequest) updates) =>
      super.copyWith((message) => updates(message as AddContactRequest))
          as AddContactRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddContactRequest create() => AddContactRequest._();
  @$core.override
  AddContactRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddContactRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddContactRequest>(create);
  static AddContactRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get source => $_getSZ(1);
  @$pb.TagNumber(2)
  set source($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSource() => $_has(1);
  @$pb.TagNumber(2)
  void clearSource() => $_clearField(2);
}

class RemoveContactRequest extends $pb.GeneratedMessage {
  factory RemoveContactRequest({
    $core.String? targetProfileId,
  }) {
    final result = create();
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    return result;
  }

  RemoveContactRequest._();

  factory RemoveContactRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveContactRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveContactRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveContactRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveContactRequest copyWith(void Function(RemoveContactRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveContactRequest))
          as RemoveContactRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveContactRequest create() => RemoveContactRequest._();
  @$core.override
  RemoveContactRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveContactRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveContactRequest>(create);
  static RemoveContactRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetProfileId() => $_clearField(1);
}

class ListContactsRequest extends $pb.GeneratedMessage {
  factory ListContactsRequest({
    $1.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ListContactsRequest._();

  factory ListContactsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListContactsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListContactsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<$1.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $1.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListContactsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListContactsRequest copyWith(void Function(ListContactsRequest) updates) =>
      super.copyWith((message) => updates(message as ListContactsRequest))
          as ListContactsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListContactsRequest create() => ListContactsRequest._();
  @$core.override
  ListContactsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListContactsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListContactsRequest>(create);
  static ListContactsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.CursorPageRequest get page => $_getN(0);
  @$pb.TagNumber(1)
  set page($1.CursorPageRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.CursorPageRequest ensurePage() => $_ensure(0);
}

class ContactList extends $pb.GeneratedMessage {
  factory ContactList({
    $core.Iterable<Contact>? contacts,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (contacts != null) result.contacts.addAll(contacts);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  ContactList._();

  factory ContactList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ContactList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ContactList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPM<Contact>(1, _omitFieldNames ? '' : 'contacts',
        subBuilder: Contact.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContactList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ContactList copyWith(void Function(ContactList) updates) =>
      super.copyWith((message) => updates(message as ContactList))
          as ContactList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ContactList create() => ContactList._();
  @$core.override
  ContactList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ContactList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ContactList>(create);
  static ContactList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Contact> get contacts => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class Contact extends $pb.GeneratedMessage {
  factory Contact({
    $core.String? profileId,
    $core.String? source,
    $core.bool? isFavorite,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    if (source != null) result.source = source;
    if (isFavorite != null) result.isFavorite = isFavorite;
    return result;
  }

  Contact._();

  factory Contact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Contact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Contact',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..aOS(2, _omitFieldNames ? '' : 'source')
    ..aOB(3, _omitFieldNames ? '' : 'isFavorite')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Contact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Contact copyWith(void Function(Contact) updates) =>
      super.copyWith((message) => updates(message as Contact)) as Contact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Contact create() => Contact._();
  @$core.override
  Contact createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Contact getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Contact>(create);
  static Contact? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get source => $_getSZ(1);
  @$pb.TagNumber(2)
  set source($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSource() => $_has(1);
  @$pb.TagNumber(2)
  void clearSource() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get isFavorite => $_getBF(2);
  @$pb.TagNumber(3)
  set isFavorite($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIsFavorite() => $_has(2);
  @$pb.TagNumber(3)
  void clearIsFavorite() => $_clearField(3);
}

class SyncPhoneContactsRequest extends $pb.GeneratedMessage {
  factory SyncPhoneContactsRequest({
    $core.Iterable<$core.String>? hashedPhoneNumbers,
  }) {
    final result = create();
    if (hashedPhoneNumbers != null)
      result.hashedPhoneNumbers.addAll(hashedPhoneNumbers);
    return result;
  }

  SyncPhoneContactsRequest._();

  factory SyncPhoneContactsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SyncPhoneContactsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SyncPhoneContactsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'hashedPhoneNumbers')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncPhoneContactsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncPhoneContactsRequest copyWith(
          void Function(SyncPhoneContactsRequest) updates) =>
      super.copyWith((message) => updates(message as SyncPhoneContactsRequest))
          as SyncPhoneContactsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SyncPhoneContactsRequest create() => SyncPhoneContactsRequest._();
  @$core.override
  SyncPhoneContactsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SyncPhoneContactsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SyncPhoneContactsRequest>(create);
  static SyncPhoneContactsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get hashedPhoneNumbers => $_getList(0);
}

class SyncPhoneContactsResponse extends $pb.GeneratedMessage {
  factory SyncPhoneContactsResponse({
    $core.Iterable<$core.String>? matchedProfileIds,
  }) {
    final result = create();
    if (matchedProfileIds != null)
      result.matchedProfileIds.addAll(matchedProfileIds);
    return result;
  }

  SyncPhoneContactsResponse._();

  factory SyncPhoneContactsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SyncPhoneContactsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SyncPhoneContactsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'matchedProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncPhoneContactsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SyncPhoneContactsResponse copyWith(
          void Function(SyncPhoneContactsResponse) updates) =>
      super.copyWith((message) => updates(message as SyncPhoneContactsResponse))
          as SyncPhoneContactsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SyncPhoneContactsResponse create() => SyncPhoneContactsResponse._();
  @$core.override
  SyncPhoneContactsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SyncPhoneContactsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SyncPhoneContactsResponse>(create);
  static SyncPhoneContactsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get matchedProfileIds => $_getList(0);
}

class SetFavoriteRequest extends $pb.GeneratedMessage {
  factory SetFavoriteRequest({
    $core.String? friendProfileId,
    $core.bool? favorite,
  }) {
    final result = create();
    if (friendProfileId != null) result.friendProfileId = friendProfileId;
    if (favorite != null) result.favorite = favorite;
    return result;
  }

  SetFavoriteRequest._();

  factory SetFavoriteRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetFavoriteRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetFavoriteRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'friendProfileId')
    ..aOB(2, _omitFieldNames ? '' : 'favorite')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetFavoriteRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetFavoriteRequest copyWith(void Function(SetFavoriteRequest) updates) =>
      super.copyWith((message) => updates(message as SetFavoriteRequest))
          as SetFavoriteRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetFavoriteRequest create() => SetFavoriteRequest._();
  @$core.override
  SetFavoriteRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetFavoriteRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetFavoriteRequest>(create);
  static SetFavoriteRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get friendProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set friendProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFriendProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriendProfileId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get favorite => $_getBF(1);
  @$pb.TagNumber(2)
  set favorite($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasFavorite() => $_has(1);
  @$pb.TagNumber(2)
  void clearFavorite() => $_clearField(2);
}

class ListFavoritesRequest extends $pb.GeneratedMessage {
  factory ListFavoritesRequest() => create();

  ListFavoritesRequest._();

  factory ListFavoritesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFavoritesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFavoritesRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesRequest copyWith(void Function(ListFavoritesRequest) updates) =>
      super.copyWith((message) => updates(message as ListFavoritesRequest))
          as ListFavoritesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFavoritesRequest create() => ListFavoritesRequest._();
  @$core.override
  ListFavoritesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFavoritesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFavoritesRequest>(create);
  static ListFavoritesRequest? _defaultInstance;
}

/// Account-level block: applies to all profiles of the peer account. Body: blocked party's account UUID (auth_db.accounts.id); blocker from request context.
class BlockAccountRequest extends $pb.GeneratedMessage {
  factory BlockAccountRequest({
    $core.String? blockedAccountId,
  }) {
    final result = create();
    if (blockedAccountId != null) result.blockedAccountId = blockedAccountId;
    return result;
  }

  BlockAccountRequest._();

  factory BlockAccountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BlockAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BlockAccountRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'blockedAccountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockAccountRequest copyWith(void Function(BlockAccountRequest) updates) =>
      super.copyWith((message) => updates(message as BlockAccountRequest))
          as BlockAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BlockAccountRequest create() => BlockAccountRequest._();
  @$core.override
  BlockAccountRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BlockAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BlockAccountRequest>(create);
  static BlockAccountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get blockedAccountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set blockedAccountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBlockedAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlockedAccountId() => $_clearField(1);
}

class UnblockAccountRequest extends $pb.GeneratedMessage {
  factory UnblockAccountRequest({
    $core.String? blockedAccountId,
  }) {
    final result = create();
    if (blockedAccountId != null) result.blockedAccountId = blockedAccountId;
    return result;
  }

  UnblockAccountRequest._();

  factory UnblockAccountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnblockAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnblockAccountRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'blockedAccountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnblockAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnblockAccountRequest copyWith(
          void Function(UnblockAccountRequest) updates) =>
      super.copyWith((message) => updates(message as UnblockAccountRequest))
          as UnblockAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnblockAccountRequest create() => UnblockAccountRequest._();
  @$core.override
  UnblockAccountRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnblockAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnblockAccountRequest>(create);
  static UnblockAccountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get blockedAccountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set blockedAccountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBlockedAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlockedAccountId() => $_clearField(1);
}

class ListBlockedRequest extends $pb.GeneratedMessage {
  factory ListBlockedRequest({
    $1.CursorPageRequest? page,
  }) {
    final result = create();
    if (page != null) result.page = page;
    return result;
  }

  ListBlockedRequest._();

  factory ListBlockedRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBlockedRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBlockedRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<$1.CursorPageRequest>(1, _omitFieldNames ? '' : 'page',
        subBuilder: $1.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBlockedRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBlockedRequest copyWith(void Function(ListBlockedRequest) updates) =>
      super.copyWith((message) => updates(message as ListBlockedRequest))
          as ListBlockedRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBlockedRequest create() => ListBlockedRequest._();
  @$core.override
  ListBlockedRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBlockedRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBlockedRequest>(create);
  static ListBlockedRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.CursorPageRequest get page => $_getN(0);
  @$pb.TagNumber(1)
  set page($1.CursorPageRequest value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPage() => $_has(0);
  @$pb.TagNumber(1)
  void clearPage() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.CursorPageRequest ensurePage() => $_ensure(0);
}

class BlockedList extends $pb.GeneratedMessage {
  factory BlockedList({
    $core.Iterable<BlockedAccount>? blocked,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (blocked != null) result.blocked.addAll(blocked);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  BlockedList._();

  factory BlockedList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BlockedList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BlockedList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPM<BlockedAccount>(1, _omitFieldNames ? '' : 'blocked',
        subBuilder: BlockedAccount.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockedList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockedList copyWith(void Function(BlockedList) updates) =>
      super.copyWith((message) => updates(message as BlockedList))
          as BlockedList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BlockedList create() => BlockedList._();
  @$core.override
  BlockedList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BlockedList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BlockedList>(create);
  static BlockedList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<BlockedAccount> get blocked => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class BlockedAccount extends $pb.GeneratedMessage {
  factory BlockedAccount({
    $core.String? blockedAccountId,
    $2.Timestamp? createdAt,
  }) {
    final result = create();
    if (blockedAccountId != null) result.blockedAccountId = blockedAccountId;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  BlockedAccount._();

  factory BlockedAccount.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BlockedAccount.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BlockedAccount',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'blockedAccountId')
    ..aOM<$2.Timestamp>(2, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $2.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockedAccount clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockedAccount copyWith(void Function(BlockedAccount) updates) =>
      super.copyWith((message) => updates(message as BlockedAccount))
          as BlockedAccount;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BlockedAccount create() => BlockedAccount._();
  @$core.override
  BlockedAccount createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BlockedAccount getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BlockedAccount>(create);
  static BlockedAccount? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get blockedAccountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set blockedAccountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBlockedAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlockedAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.Timestamp get createdAt => $_getN(1);
  @$pb.TagNumber(2)
  set createdAt($2.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCreatedAt() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreatedAt() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.Timestamp ensureCreatedAt() => $_ensure(1);
}

class IsBlockedRequest extends $pb.GeneratedMessage {
  factory IsBlockedRequest({
    $core.String? accountIdA,
    $core.String? accountIdB,
  }) {
    final result = create();
    if (accountIdA != null) result.accountIdA = accountIdA;
    if (accountIdB != null) result.accountIdB = accountIdB;
    return result;
  }

  IsBlockedRequest._();

  factory IsBlockedRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IsBlockedRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IsBlockedRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountIdA')
    ..aOS(2, _omitFieldNames ? '' : 'accountIdB')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsBlockedRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsBlockedRequest copyWith(void Function(IsBlockedRequest) updates) =>
      super.copyWith((message) => updates(message as IsBlockedRequest))
          as IsBlockedRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IsBlockedRequest create() => IsBlockedRequest._();
  @$core.override
  IsBlockedRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IsBlockedRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IsBlockedRequest>(create);
  static IsBlockedRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountIdA => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountIdA($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountIdA() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountIdA() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountIdB => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountIdB($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountIdB() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountIdB() => $_clearField(2);
}

class IsBlockedResponse extends $pb.GeneratedMessage {
  factory IsBlockedResponse({
    $core.bool? blocked,
  }) {
    final result = create();
    if (blocked != null) result.blocked = blocked;
    return result;
  }

  IsBlockedResponse._();

  factory IsBlockedResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IsBlockedResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IsBlockedResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'blocked')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsBlockedResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsBlockedResponse copyWith(void Function(IsBlockedResponse) updates) =>
      super.copyWith((message) => updates(message as IsBlockedResponse))
          as IsBlockedResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IsBlockedResponse create() => IsBlockedResponse._();
  @$core.override
  IsBlockedResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IsBlockedResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IsBlockedResponse>(create);
  static IsBlockedResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get blocked => $_getBF(0);
  @$pb.TagNumber(1)
  set blocked($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBlocked() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlocked() => $_clearField(1);
}

class AreFriendsRequest extends $pb.GeneratedMessage {
  factory AreFriendsRequest({
    $core.String? profileIdA,
    $core.String? profileIdB,
  }) {
    final result = create();
    if (profileIdA != null) result.profileIdA = profileIdA;
    if (profileIdB != null) result.profileIdB = profileIdB;
    return result;
  }

  AreFriendsRequest._();

  factory AreFriendsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AreFriendsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AreFriendsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileIdA')
    ..aOS(2, _omitFieldNames ? '' : 'profileIdB')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreFriendsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreFriendsRequest copyWith(void Function(AreFriendsRequest) updates) =>
      super.copyWith((message) => updates(message as AreFriendsRequest))
          as AreFriendsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AreFriendsRequest create() => AreFriendsRequest._();
  @$core.override
  AreFriendsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AreFriendsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AreFriendsRequest>(create);
  static AreFriendsRequest? _defaultInstance;

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

class AreFriendsResponse extends $pb.GeneratedMessage {
  factory AreFriendsResponse({
    $core.bool? friends,
  }) {
    final result = create();
    if (friends != null) result.friends = friends;
    return result;
  }

  AreFriendsResponse._();

  factory AreFriendsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AreFriendsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AreFriendsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'friends')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreFriendsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AreFriendsResponse copyWith(void Function(AreFriendsResponse) updates) =>
      super.copyWith((message) => updates(message as AreFriendsResponse))
          as AreFriendsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AreFriendsResponse create() => AreFriendsResponse._();
  @$core.override
  AreFriendsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AreFriendsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AreFriendsResponse>(create);
  static AreFriendsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get friends => $_getBF(0);
  @$pb.TagNumber(1)
  set friends($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFriends() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriends() => $_clearField(1);
}

class GetFriendsOfFriendsRequest extends $pb.GeneratedMessage {
  factory GetFriendsOfFriendsRequest({
    $core.String? profileId,
  }) {
    final result = create();
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetFriendsOfFriendsRequest._();

  factory GetFriendsOfFriendsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFriendsOfFriendsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFriendsOfFriendsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFriendsOfFriendsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFriendsOfFriendsRequest copyWith(
          void Function(GetFriendsOfFriendsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetFriendsOfFriendsRequest))
          as GetFriendsOfFriendsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFriendsOfFriendsRequest create() => GetFriendsOfFriendsRequest._();
  @$core.override
  GetFriendsOfFriendsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFriendsOfFriendsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFriendsOfFriendsRequest>(create);
  static GetFriendsOfFriendsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get profileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set profileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileId() => $_clearField(1);
}

class ProfileIdList extends $pb.GeneratedMessage {
  factory ProfileIdList({
    $core.Iterable<$core.String>? profileIds,
  }) {
    final result = create();
    if (profileIds != null) result.profileIds.addAll(profileIds);
    return result;
  }

  ProfileIdList._();

  factory ProfileIdList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ProfileIdList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ProfileIdList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'profileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileIdList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ProfileIdList copyWith(void Function(ProfileIdList) updates) =>
      super.copyWith((message) => updates(message as ProfileIdList))
          as ProfileIdList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ProfileIdList create() => ProfileIdList._();
  @$core.override
  ProfileIdList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ProfileIdList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ProfileIdList>(create);
  static ProfileIdList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get profileIds => $_getList(0);
}

class SendFriendInvitationResponse extends $pb.GeneratedMessage {
  factory SendFriendInvitationResponse() => create();

  SendFriendInvitationResponse._();

  factory SendFriendInvitationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SendFriendInvitationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SendFriendInvitationResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendFriendInvitationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SendFriendInvitationResponse copyWith(
          void Function(SendFriendInvitationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SendFriendInvitationResponse))
          as SendFriendInvitationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SendFriendInvitationResponse create() =>
      SendFriendInvitationResponse._();
  @$core.override
  SendFriendInvitationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SendFriendInvitationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SendFriendInvitationResponse>(create);
  static SendFriendInvitationResponse? _defaultInstance;
}

class AcceptFriendInvitationResponse extends $pb.GeneratedMessage {
  factory AcceptFriendInvitationResponse() => create();

  AcceptFriendInvitationResponse._();

  factory AcceptFriendInvitationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AcceptFriendInvitationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AcceptFriendInvitationResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptFriendInvitationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AcceptFriendInvitationResponse copyWith(
          void Function(AcceptFriendInvitationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AcceptFriendInvitationResponse))
          as AcceptFriendInvitationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AcceptFriendInvitationResponse create() =>
      AcceptFriendInvitationResponse._();
  @$core.override
  AcceptFriendInvitationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AcceptFriendInvitationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AcceptFriendInvitationResponse>(create);
  static AcceptFriendInvitationResponse? _defaultInstance;
}

class DeclineFriendInvitationResponse extends $pb.GeneratedMessage {
  factory DeclineFriendInvitationResponse() => create();

  DeclineFriendInvitationResponse._();

  factory DeclineFriendInvitationResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeclineFriendInvitationResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeclineFriendInvitationResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineFriendInvitationResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeclineFriendInvitationResponse copyWith(
          void Function(DeclineFriendInvitationResponse) updates) =>
      super.copyWith(
              (message) => updates(message as DeclineFriendInvitationResponse))
          as DeclineFriendInvitationResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeclineFriendInvitationResponse create() =>
      DeclineFriendInvitationResponse._();
  @$core.override
  DeclineFriendInvitationResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeclineFriendInvitationResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeclineFriendInvitationResponse>(
          create);
  static DeclineFriendInvitationResponse? _defaultInstance;
}

class RemoveFriendResponse extends $pb.GeneratedMessage {
  factory RemoveFriendResponse() => create();

  RemoveFriendResponse._();

  factory RemoveFriendResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveFriendResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveFriendResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFriendResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveFriendResponse copyWith(void Function(RemoveFriendResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveFriendResponse))
          as RemoveFriendResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveFriendResponse create() => RemoveFriendResponse._();
  @$core.override
  RemoveFriendResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveFriendResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveFriendResponse>(create);
  static RemoveFriendResponse? _defaultInstance;
}

class ListFriendsResponse extends $pb.GeneratedMessage {
  factory ListFriendsResponse({
    FriendList? friendList,
  }) {
    final result = create();
    if (friendList != null) result.friendList = friendList;
    return result;
  }

  ListFriendsResponse._();

  factory ListFriendsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFriendsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFriendsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<FriendList>(1, _omitFieldNames ? '' : 'friendList',
        subBuilder: FriendList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendsResponse copyWith(void Function(ListFriendsResponse) updates) =>
      super.copyWith((message) => updates(message as ListFriendsResponse))
          as ListFriendsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFriendsResponse create() => ListFriendsResponse._();
  @$core.override
  ListFriendsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFriendsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFriendsResponse>(create);
  static ListFriendsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FriendList get friendList => $_getN(0);
  @$pb.TagNumber(1)
  set friendList(FriendList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFriendList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriendList() => $_clearField(1);
  @$pb.TagNumber(1)
  FriendList ensureFriendList() => $_ensure(0);
}

class ListFriendRequestsResponse extends $pb.GeneratedMessage {
  factory ListFriendRequestsResponse({
    FriendRequestList? friendRequestList,
  }) {
    final result = create();
    if (friendRequestList != null) result.friendRequestList = friendRequestList;
    return result;
  }

  ListFriendRequestsResponse._();

  factory ListFriendRequestsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFriendRequestsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFriendRequestsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<FriendRequestList>(1, _omitFieldNames ? '' : 'friendRequestList',
        subBuilder: FriendRequestList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendRequestsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFriendRequestsResponse copyWith(
          void Function(ListFriendRequestsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ListFriendRequestsResponse))
          as ListFriendRequestsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFriendRequestsResponse create() => ListFriendRequestsResponse._();
  @$core.override
  ListFriendRequestsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFriendRequestsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFriendRequestsResponse>(create);
  static ListFriendRequestsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FriendRequestList get friendRequestList => $_getN(0);
  @$pb.TagNumber(1)
  set friendRequestList(FriendRequestList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFriendRequestList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriendRequestList() => $_clearField(1);
  @$pb.TagNumber(1)
  FriendRequestList ensureFriendRequestList() => $_ensure(0);
}

class AddContactResponse extends $pb.GeneratedMessage {
  factory AddContactResponse() => create();

  AddContactResponse._();

  factory AddContactResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AddContactResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AddContactResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddContactResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AddContactResponse copyWith(void Function(AddContactResponse) updates) =>
      super.copyWith((message) => updates(message as AddContactResponse))
          as AddContactResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AddContactResponse create() => AddContactResponse._();
  @$core.override
  AddContactResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AddContactResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AddContactResponse>(create);
  static AddContactResponse? _defaultInstance;
}

class RemoveContactResponse extends $pb.GeneratedMessage {
  factory RemoveContactResponse() => create();

  RemoveContactResponse._();

  factory RemoveContactResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveContactResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveContactResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveContactResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveContactResponse copyWith(
          void Function(RemoveContactResponse) updates) =>
      super.copyWith((message) => updates(message as RemoveContactResponse))
          as RemoveContactResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveContactResponse create() => RemoveContactResponse._();
  @$core.override
  RemoveContactResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveContactResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveContactResponse>(create);
  static RemoveContactResponse? _defaultInstance;
}

class ListContactsResponse extends $pb.GeneratedMessage {
  factory ListContactsResponse({
    ContactList? contactList,
  }) {
    final result = create();
    if (contactList != null) result.contactList = contactList;
    return result;
  }

  ListContactsResponse._();

  factory ListContactsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListContactsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListContactsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<ContactList>(1, _omitFieldNames ? '' : 'contactList',
        subBuilder: ContactList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListContactsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListContactsResponse copyWith(void Function(ListContactsResponse) updates) =>
      super.copyWith((message) => updates(message as ListContactsResponse))
          as ListContactsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListContactsResponse create() => ListContactsResponse._();
  @$core.override
  ListContactsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListContactsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListContactsResponse>(create);
  static ListContactsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ContactList get contactList => $_getN(0);
  @$pb.TagNumber(1)
  set contactList(ContactList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasContactList() => $_has(0);
  @$pb.TagNumber(1)
  void clearContactList() => $_clearField(1);
  @$pb.TagNumber(1)
  ContactList ensureContactList() => $_ensure(0);
}

class SetFavoriteResponse extends $pb.GeneratedMessage {
  factory SetFavoriteResponse() => create();

  SetFavoriteResponse._();

  factory SetFavoriteResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetFavoriteResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetFavoriteResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetFavoriteResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetFavoriteResponse copyWith(void Function(SetFavoriteResponse) updates) =>
      super.copyWith((message) => updates(message as SetFavoriteResponse))
          as SetFavoriteResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetFavoriteResponse create() => SetFavoriteResponse._();
  @$core.override
  SetFavoriteResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetFavoriteResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetFavoriteResponse>(create);
  static SetFavoriteResponse? _defaultInstance;
}

class ListFavoritesResponse extends $pb.GeneratedMessage {
  factory ListFavoritesResponse({
    FriendList? friendList,
  }) {
    final result = create();
    if (friendList != null) result.friendList = friendList;
    return result;
  }

  ListFavoritesResponse._();

  factory ListFavoritesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListFavoritesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListFavoritesResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<FriendList>(1, _omitFieldNames ? '' : 'friendList',
        subBuilder: FriendList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListFavoritesResponse copyWith(
          void Function(ListFavoritesResponse) updates) =>
      super.copyWith((message) => updates(message as ListFavoritesResponse))
          as ListFavoritesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListFavoritesResponse create() => ListFavoritesResponse._();
  @$core.override
  ListFavoritesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListFavoritesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListFavoritesResponse>(create);
  static ListFavoritesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FriendList get friendList => $_getN(0);
  @$pb.TagNumber(1)
  set friendList(FriendList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFriendList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFriendList() => $_clearField(1);
  @$pb.TagNumber(1)
  FriendList ensureFriendList() => $_ensure(0);
}

class BlockAccountResponse extends $pb.GeneratedMessage {
  factory BlockAccountResponse() => create();

  BlockAccountResponse._();

  factory BlockAccountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BlockAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BlockAccountResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BlockAccountResponse copyWith(void Function(BlockAccountResponse) updates) =>
      super.copyWith((message) => updates(message as BlockAccountResponse))
          as BlockAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BlockAccountResponse create() => BlockAccountResponse._();
  @$core.override
  BlockAccountResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BlockAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BlockAccountResponse>(create);
  static BlockAccountResponse? _defaultInstance;
}

class UnblockAccountResponse extends $pb.GeneratedMessage {
  factory UnblockAccountResponse() => create();

  UnblockAccountResponse._();

  factory UnblockAccountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UnblockAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UnblockAccountResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnblockAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UnblockAccountResponse copyWith(
          void Function(UnblockAccountResponse) updates) =>
      super.copyWith((message) => updates(message as UnblockAccountResponse))
          as UnblockAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UnblockAccountResponse create() => UnblockAccountResponse._();
  @$core.override
  UnblockAccountResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UnblockAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UnblockAccountResponse>(create);
  static UnblockAccountResponse? _defaultInstance;
}

class ListBlockedResponse extends $pb.GeneratedMessage {
  factory ListBlockedResponse({
    BlockedList? blockedList,
  }) {
    final result = create();
    if (blockedList != null) result.blockedList = blockedList;
    return result;
  }

  ListBlockedResponse._();

  factory ListBlockedResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListBlockedResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListBlockedResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<BlockedList>(1, _omitFieldNames ? '' : 'blockedList',
        subBuilder: BlockedList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBlockedResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListBlockedResponse copyWith(void Function(ListBlockedResponse) updates) =>
      super.copyWith((message) => updates(message as ListBlockedResponse))
          as ListBlockedResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListBlockedResponse create() => ListBlockedResponse._();
  @$core.override
  ListBlockedResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListBlockedResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListBlockedResponse>(create);
  static ListBlockedResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BlockedList get blockedList => $_getN(0);
  @$pb.TagNumber(1)
  set blockedList(BlockedList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBlockedList() => $_has(0);
  @$pb.TagNumber(1)
  void clearBlockedList() => $_clearField(1);
  @$pb.TagNumber(1)
  BlockedList ensureBlockedList() => $_ensure(0);
}

class GetFriendsOfFriendsResponse extends $pb.GeneratedMessage {
  factory GetFriendsOfFriendsResponse({
    ProfileIdList? profileIdList,
  }) {
    final result = create();
    if (profileIdList != null) result.profileIdList = profileIdList;
    return result;
  }

  GetFriendsOfFriendsResponse._();

  factory GetFriendsOfFriendsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFriendsOfFriendsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFriendsOfFriendsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.social.v1'),
      createEmptyInstance: create)
    ..aOM<ProfileIdList>(1, _omitFieldNames ? '' : 'profileIdList',
        subBuilder: ProfileIdList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFriendsOfFriendsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFriendsOfFriendsResponse copyWith(
          void Function(GetFriendsOfFriendsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetFriendsOfFriendsResponse))
          as GetFriendsOfFriendsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFriendsOfFriendsResponse create() =>
      GetFriendsOfFriendsResponse._();
  @$core.override
  GetFriendsOfFriendsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFriendsOfFriendsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFriendsOfFriendsResponse>(create);
  static GetFriendsOfFriendsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ProfileIdList get profileIdList => $_getN(0);
  @$pb.TagNumber(1)
  set profileIdList(ProfileIdList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasProfileIdList() => $_has(0);
  @$pb.TagNumber(1)
  void clearProfileIdList() => $_clearField(1);
  @$pb.TagNumber(1)
  ProfileIdList ensureProfileIdList() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
