// This is a generated file - do not edit.
//
// Generated from voice/role/v1/space_chat_permissions.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'member_admission.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class GetChatPermissionStateRequest extends $pb.GeneratedMessage {
  factory GetChatPermissionStateRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? chatId,
    $core.String? subjectProfileId,
    $core.String? operationId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (chatId != null) result.chatId = chatId;
    if (subjectProfileId != null) result.subjectProfileId = subjectProfileId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  GetChatPermissionStateRequest._();

  factory GetChatPermissionStateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatPermissionStateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatPermissionStateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'subjectProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatPermissionStateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatPermissionStateRequest copyWith(
          void Function(GetChatPermissionStateRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetChatPermissionStateRequest))
          as GetChatPermissionStateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatPermissionStateRequest create() =>
      GetChatPermissionStateRequest._();
  @$core.override
  GetChatPermissionStateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatPermissionStateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatPermissionStateRequest>(create);
  static GetChatPermissionStateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get subjectProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set subjectProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSubjectProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubjectProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get operationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set operationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOperationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearOperationId() => $_clearField(5);
}

class SpaceChatMembershipGrant extends $pb.GeneratedMessage {
  factory SpaceChatMembershipGrant({
    $1.MembershipBinding? membershipBinding,
    $core.bool? explicitPostGrant,
  }) {
    final result = create();
    if (membershipBinding != null) result.membershipBinding = membershipBinding;
    if (explicitPostGrant != null) result.explicitPostGrant = explicitPostGrant;
    return result;
  }

  SpaceChatMembershipGrant._();

  factory SpaceChatMembershipGrant.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceChatMembershipGrant.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceChatMembershipGrant',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<$1.MembershipBinding>(1, _omitFieldNames ? '' : 'membershipBinding',
        subBuilder: $1.MembershipBinding.create)
    ..aOB(2, _omitFieldNames ? '' : 'explicitPostGrant')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceChatMembershipGrant clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceChatMembershipGrant copyWith(
          void Function(SpaceChatMembershipGrant) updates) =>
      super.copyWith((message) => updates(message as SpaceChatMembershipGrant))
          as SpaceChatMembershipGrant;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceChatMembershipGrant create() => SpaceChatMembershipGrant._();
  @$core.override
  SpaceChatMembershipGrant createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceChatMembershipGrant getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceChatMembershipGrant>(create);
  static SpaceChatMembershipGrant? _defaultInstance;

  @$pb.TagNumber(1)
  $1.MembershipBinding get membershipBinding => $_getN(0);
  @$pb.TagNumber(1)
  set membershipBinding($1.MembershipBinding value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasMembershipBinding() => $_has(0);
  @$pb.TagNumber(1)
  void clearMembershipBinding() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.MembershipBinding ensureMembershipBinding() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.bool get explicitPostGrant => $_getBF(1);
  @$pb.TagNumber(2)
  set explicitPostGrant($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasExplicitPostGrant() => $_has(1);
  @$pb.TagNumber(2)
  void clearExplicitPostGrant() => $_clearField(2);
}

/// Request-bound observation only. Every send, invite and destination load must
/// obtain fresh authority; a later operation must not consume this as a bearer.
class GetChatPermissionStateResponse extends $pb.GeneratedMessage {
  factory GetChatPermissionStateResponse({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? chatId,
    $core.String? subjectProfileId,
    $core.String? chatScopeIncarnation,
    $core.bool? canPostAsChat,
    $core.bool? canAddMembers,
    $core.bool? canManagePermissions,
    $core.bool? memberAdditionsEnabled,
    $core.bool? effectiveExplicitPostGrant,
    $core.Iterable<SpaceChatMembershipGrant>? membershipGrants,
    $fixnum.Int64? roleAuthorizationRevision,
    $fixnum.Int64? spaceAuthorityRevision,
    $fixnum.Int64? chatAuthorityRevision,
    $fixnum.Int64? spaceLifecycleGeneration,
    $fixnum.Int64? ownershipGeneration,
    $fixnum.Int64? chatLifecycleGeneration,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (chatId != null) result.chatId = chatId;
    if (subjectProfileId != null) result.subjectProfileId = subjectProfileId;
    if (chatScopeIncarnation != null)
      result.chatScopeIncarnation = chatScopeIncarnation;
    if (canPostAsChat != null) result.canPostAsChat = canPostAsChat;
    if (canAddMembers != null) result.canAddMembers = canAddMembers;
    if (canManagePermissions != null)
      result.canManagePermissions = canManagePermissions;
    if (memberAdditionsEnabled != null)
      result.memberAdditionsEnabled = memberAdditionsEnabled;
    if (effectiveExplicitPostGrant != null)
      result.effectiveExplicitPostGrant = effectiveExplicitPostGrant;
    if (membershipGrants != null)
      result.membershipGrants.addAll(membershipGrants);
    if (roleAuthorizationRevision != null)
      result.roleAuthorizationRevision = roleAuthorizationRevision;
    if (spaceAuthorityRevision != null)
      result.spaceAuthorityRevision = spaceAuthorityRevision;
    if (chatAuthorityRevision != null)
      result.chatAuthorityRevision = chatAuthorityRevision;
    if (spaceLifecycleGeneration != null)
      result.spaceLifecycleGeneration = spaceLifecycleGeneration;
    if (ownershipGeneration != null)
      result.ownershipGeneration = ownershipGeneration;
    if (chatLifecycleGeneration != null)
      result.chatLifecycleGeneration = chatLifecycleGeneration;
    return result;
  }

  GetChatPermissionStateResponse._();

  factory GetChatPermissionStateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatPermissionStateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatPermissionStateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'subjectProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'chatScopeIncarnation')
    ..aOB(6, _omitFieldNames ? '' : 'canPostAsChat')
    ..aOB(7, _omitFieldNames ? '' : 'canAddMembers')
    ..aOB(8, _omitFieldNames ? '' : 'canManagePermissions')
    ..aOB(9, _omitFieldNames ? '' : 'memberAdditionsEnabled')
    ..aOB(10, _omitFieldNames ? '' : 'effectiveExplicitPostGrant')
    ..pPM<SpaceChatMembershipGrant>(
        11, _omitFieldNames ? '' : 'membershipGrants',
        subBuilder: SpaceChatMembershipGrant.create)
    ..a<$fixnum.Int64>(12, _omitFieldNames ? '' : 'roleAuthorizationRevision',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(13, _omitFieldNames ? '' : 'spaceAuthorityRevision',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        14, _omitFieldNames ? '' : 'chatAuthorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(15, _omitFieldNames ? '' : 'spaceLifecycleGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        16, _omitFieldNames ? '' : 'ownershipGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(17, _omitFieldNames ? '' : 'chatLifecycleGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatPermissionStateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatPermissionStateResponse copyWith(
          void Function(GetChatPermissionStateResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetChatPermissionStateResponse))
          as GetChatPermissionStateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatPermissionStateResponse create() =>
      GetChatPermissionStateResponse._();
  @$core.override
  GetChatPermissionStateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatPermissionStateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatPermissionStateResponse>(create);
  static GetChatPermissionStateResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get subjectProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set subjectProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSubjectProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubjectProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get chatScopeIncarnation => $_getSZ(4);
  @$pb.TagNumber(5)
  set chatScopeIncarnation($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasChatScopeIncarnation() => $_has(4);
  @$pb.TagNumber(5)
  void clearChatScopeIncarnation() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get canPostAsChat => $_getBF(5);
  @$pb.TagNumber(6)
  set canPostAsChat($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCanPostAsChat() => $_has(5);
  @$pb.TagNumber(6)
  void clearCanPostAsChat() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get canAddMembers => $_getBF(6);
  @$pb.TagNumber(7)
  set canAddMembers($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCanAddMembers() => $_has(6);
  @$pb.TagNumber(7)
  void clearCanAddMembers() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get canManagePermissions => $_getBF(7);
  @$pb.TagNumber(8)
  set canManagePermissions($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasCanManagePermissions() => $_has(7);
  @$pb.TagNumber(8)
  void clearCanManagePermissions() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get memberAdditionsEnabled => $_getBF(8);
  @$pb.TagNumber(9)
  set memberAdditionsEnabled($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasMemberAdditionsEnabled() => $_has(8);
  @$pb.TagNumber(9)
  void clearMemberAdditionsEnabled() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get effectiveExplicitPostGrant => $_getBF(9);
  @$pb.TagNumber(10)
  set effectiveExplicitPostGrant($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasEffectiveExplicitPostGrant() => $_has(9);
  @$pb.TagNumber(10)
  void clearEffectiveExplicitPostGrant() => $_clearField(10);

  @$pb.TagNumber(11)
  $pb.PbList<SpaceChatMembershipGrant> get membershipGrants => $_getList(10);

  @$pb.TagNumber(12)
  $fixnum.Int64 get roleAuthorizationRevision => $_getI64(11);
  @$pb.TagNumber(12)
  set roleAuthorizationRevision($fixnum.Int64 value) => $_setInt64(11, value);
  @$pb.TagNumber(12)
  $core.bool hasRoleAuthorizationRevision() => $_has(11);
  @$pb.TagNumber(12)
  void clearRoleAuthorizationRevision() => $_clearField(12);

  @$pb.TagNumber(13)
  $fixnum.Int64 get spaceAuthorityRevision => $_getI64(12);
  @$pb.TagNumber(13)
  set spaceAuthorityRevision($fixnum.Int64 value) => $_setInt64(12, value);
  @$pb.TagNumber(13)
  $core.bool hasSpaceAuthorityRevision() => $_has(12);
  @$pb.TagNumber(13)
  void clearSpaceAuthorityRevision() => $_clearField(13);

  @$pb.TagNumber(14)
  $fixnum.Int64 get chatAuthorityRevision => $_getI64(13);
  @$pb.TagNumber(14)
  set chatAuthorityRevision($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasChatAuthorityRevision() => $_has(13);
  @$pb.TagNumber(14)
  void clearChatAuthorityRevision() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get spaceLifecycleGeneration => $_getI64(14);
  @$pb.TagNumber(15)
  set spaceLifecycleGeneration($fixnum.Int64 value) => $_setInt64(14, value);
  @$pb.TagNumber(15)
  $core.bool hasSpaceLifecycleGeneration() => $_has(14);
  @$pb.TagNumber(15)
  void clearSpaceLifecycleGeneration() => $_clearField(15);

  @$pb.TagNumber(16)
  $fixnum.Int64 get ownershipGeneration => $_getI64(15);
  @$pb.TagNumber(16)
  set ownershipGeneration($fixnum.Int64 value) => $_setInt64(15, value);
  @$pb.TagNumber(16)
  $core.bool hasOwnershipGeneration() => $_has(15);
  @$pb.TagNumber(16)
  void clearOwnershipGeneration() => $_clearField(16);

  @$pb.TagNumber(17)
  $fixnum.Int64 get chatLifecycleGeneration => $_getI64(16);
  @$pb.TagNumber(17)
  set chatLifecycleGeneration($fixnum.Int64 value) => $_setInt64(16, value);
  @$pb.TagNumber(17)
  $core.bool hasChatLifecycleGeneration() => $_has(16);
  @$pb.TagNumber(17)
  void clearChatLifecycleGeneration() => $_clearField(17);
}

class SetSpaceChatAddMembersRequest extends $pb.GeneratedMessage {
  factory SetSpaceChatAddMembersRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? chatId,
    $core.bool? enabled,
    $core.String? operationId,
    $core.String? expectedChatScopeIncarnation,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (chatId != null) result.chatId = chatId;
    if (enabled != null) result.enabled = enabled;
    if (operationId != null) result.operationId = operationId;
    if (expectedChatScopeIncarnation != null)
      result.expectedChatScopeIncarnation = expectedChatScopeIncarnation;
    return result;
  }

  SetSpaceChatAddMembersRequest._();

  factory SetSpaceChatAddMembersRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetSpaceChatAddMembersRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetSpaceChatAddMembersRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOB(4, _omitFieldNames ? '' : 'enabled')
    ..aOS(5, _omitFieldNames ? '' : 'operationId')
    ..aOS(6, _omitFieldNames ? '' : 'expectedChatScopeIncarnation')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatAddMembersRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatAddMembersRequest copyWith(
          void Function(SetSpaceChatAddMembersRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SetSpaceChatAddMembersRequest))
          as SetSpaceChatAddMembersRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetSpaceChatAddMembersRequest create() =>
      SetSpaceChatAddMembersRequest._();
  @$core.override
  SetSpaceChatAddMembersRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetSpaceChatAddMembersRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetSpaceChatAddMembersRequest>(create);
  static SetSpaceChatAddMembersRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get enabled => $_getBF(3);
  @$pb.TagNumber(4)
  set enabled($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEnabled() => $_has(3);
  @$pb.TagNumber(4)
  void clearEnabled() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get operationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set operationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOperationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearOperationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get expectedChatScopeIncarnation => $_getSZ(5);
  @$pb.TagNumber(6)
  set expectedChatScopeIncarnation($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasExpectedChatScopeIncarnation() => $_has(5);
  @$pb.TagNumber(6)
  void clearExpectedChatScopeIncarnation() => $_clearField(6);
}

class SetSpaceChatPostAsChatGrantRequest extends $pb.GeneratedMessage {
  factory SetSpaceChatPostAsChatGrantRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? chatId,
    $core.String? targetProfileId,
    $core.bool? allowed,
    $core.String? operationId,
    $core.String? expectedChatScopeIncarnation,
    $1.MembershipBinding? targetMembershipBinding,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (chatId != null) result.chatId = chatId;
    if (targetProfileId != null) result.targetProfileId = targetProfileId;
    if (allowed != null) result.allowed = allowed;
    if (operationId != null) result.operationId = operationId;
    if (expectedChatScopeIncarnation != null)
      result.expectedChatScopeIncarnation = expectedChatScopeIncarnation;
    if (targetMembershipBinding != null)
      result.targetMembershipBinding = targetMembershipBinding;
    return result;
  }

  SetSpaceChatPostAsChatGrantRequest._();

  factory SetSpaceChatPostAsChatGrantRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetSpaceChatPostAsChatGrantRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetSpaceChatPostAsChatGrantRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'targetProfileId')
    ..aOB(5, _omitFieldNames ? '' : 'allowed')
    ..aOS(6, _omitFieldNames ? '' : 'operationId')
    ..aOS(7, _omitFieldNames ? '' : 'expectedChatScopeIncarnation')
    ..aOM<$1.MembershipBinding>(
        8, _omitFieldNames ? '' : 'targetMembershipBinding',
        subBuilder: $1.MembershipBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatPostAsChatGrantRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatPostAsChatGrantRequest copyWith(
          void Function(SetSpaceChatPostAsChatGrantRequest) updates) =>
      super.copyWith((message) =>
              updates(message as SetSpaceChatPostAsChatGrantRequest))
          as SetSpaceChatPostAsChatGrantRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetSpaceChatPostAsChatGrantRequest create() =>
      SetSpaceChatPostAsChatGrantRequest._();
  @$core.override
  SetSpaceChatPostAsChatGrantRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetSpaceChatPostAsChatGrantRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetSpaceChatPostAsChatGrantRequest>(
          create);
  static SetSpaceChatPostAsChatGrantRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get targetProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set targetProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTargetProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearTargetProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get allowed => $_getBF(4);
  @$pb.TagNumber(5)
  set allowed($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasAllowed() => $_has(4);
  @$pb.TagNumber(5)
  void clearAllowed() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get operationId => $_getSZ(5);
  @$pb.TagNumber(6)
  set operationId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasOperationId() => $_has(5);
  @$pb.TagNumber(6)
  void clearOperationId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get expectedChatScopeIncarnation => $_getSZ(6);
  @$pb.TagNumber(7)
  set expectedChatScopeIncarnation($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasExpectedChatScopeIncarnation() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpectedChatScopeIncarnation() => $_clearField(7);

  /// Omitted + exactly one eligible source selects that source once. BOTH with
  /// no selector fails MEMBERSHIP_SOURCE_REQUIRED for allow AND revoke. A stale
  /// selector fails MEMBERSHIP_BINDING_CHANGED. Two sources are two operations.
  @$pb.TagNumber(8)
  $1.MembershipBinding get targetMembershipBinding => $_getN(7);
  @$pb.TagNumber(8)
  set targetMembershipBinding($1.MembershipBinding value) =>
      $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasTargetMembershipBinding() => $_has(7);
  @$pb.TagNumber(8)
  void clearTargetMembershipBinding() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.MembershipBinding ensureTargetMembershipBinding() => $_ensure(7);
}

/// Durable historical evidence, never a current membership or permission. An
/// exact completed retry returns this outcome without reapplying its mutation.
class SpaceChatMutationOutcome extends $pb.GeneratedMessage {
  factory SpaceChatMutationOutcome({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? chatScopeIncarnation,
    $core.bool? appliedValue,
    $1.MembershipBinding? selectedMembershipBinding,
    $fixnum.Int64? roleAuthorizationRevision,
    $core.bool? historical,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (chatScopeIncarnation != null)
      result.chatScopeIncarnation = chatScopeIncarnation;
    if (appliedValue != null) result.appliedValue = appliedValue;
    if (selectedMembershipBinding != null)
      result.selectedMembershipBinding = selectedMembershipBinding;
    if (roleAuthorizationRevision != null)
      result.roleAuthorizationRevision = roleAuthorizationRevision;
    if (historical != null) result.historical = historical;
    return result;
  }

  SpaceChatMutationOutcome._();

  factory SpaceChatMutationOutcome.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceChatMutationOutcome.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceChatMutationOutcome',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'chatScopeIncarnation')
    ..aOB(4, _omitFieldNames ? '' : 'appliedValue')
    ..aOM<$1.MembershipBinding>(
        5, _omitFieldNames ? '' : 'selectedMembershipBinding',
        subBuilder: $1.MembershipBinding.create)
    ..a<$fixnum.Int64>(6, _omitFieldNames ? '' : 'roleAuthorizationRevision',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOB(7, _omitFieldNames ? '' : 'historical')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceChatMutationOutcome clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceChatMutationOutcome copyWith(
          void Function(SpaceChatMutationOutcome) updates) =>
      super.copyWith((message) => updates(message as SpaceChatMutationOutcome))
          as SpaceChatMutationOutcome;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceChatMutationOutcome create() => SpaceChatMutationOutcome._();
  @$core.override
  SpaceChatMutationOutcome createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceChatMutationOutcome getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceChatMutationOutcome>(create);
  static SpaceChatMutationOutcome? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatScopeIncarnation => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatScopeIncarnation($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatScopeIncarnation() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatScopeIncarnation() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get appliedValue => $_getBF(3);
  @$pb.TagNumber(4)
  set appliedValue($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAppliedValue() => $_has(3);
  @$pb.TagNumber(4)
  void clearAppliedValue() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.MembershipBinding get selectedMembershipBinding => $_getN(4);
  @$pb.TagNumber(5)
  set selectedMembershipBinding($1.MembershipBinding value) =>
      $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasSelectedMembershipBinding() => $_has(4);
  @$pb.TagNumber(5)
  void clearSelectedMembershipBinding() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.MembershipBinding ensureSelectedMembershipBinding() => $_ensure(4);

  @$pb.TagNumber(6)
  $fixnum.Int64 get roleAuthorizationRevision => $_getI64(5);
  @$pb.TagNumber(6)
  set roleAuthorizationRevision($fixnum.Int64 value) => $_setInt64(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRoleAuthorizationRevision() => $_has(5);
  @$pb.TagNumber(6)
  void clearRoleAuthorizationRevision() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get historical => $_getBF(6);
  @$pb.TagNumber(7)
  set historical($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasHistorical() => $_has(6);
  @$pb.TagNumber(7)
  void clearHistorical() => $_clearField(7);
}

class SetSpaceChatAddMembersResponse extends $pb.GeneratedMessage {
  factory SetSpaceChatAddMembersResponse({
    SpaceChatMutationOutcome? outcome,
  }) {
    final result = create();
    if (outcome != null) result.outcome = outcome;
    return result;
  }

  SetSpaceChatAddMembersResponse._();

  factory SetSpaceChatAddMembersResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetSpaceChatAddMembersResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetSpaceChatAddMembersResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceChatMutationOutcome>(1, _omitFieldNames ? '' : 'outcome',
        subBuilder: SpaceChatMutationOutcome.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatAddMembersResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatAddMembersResponse copyWith(
          void Function(SetSpaceChatAddMembersResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SetSpaceChatAddMembersResponse))
          as SetSpaceChatAddMembersResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetSpaceChatAddMembersResponse create() =>
      SetSpaceChatAddMembersResponse._();
  @$core.override
  SetSpaceChatAddMembersResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetSpaceChatAddMembersResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetSpaceChatAddMembersResponse>(create);
  static SetSpaceChatAddMembersResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceChatMutationOutcome get outcome => $_getN(0);
  @$pb.TagNumber(1)
  set outcome(SpaceChatMutationOutcome value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOutcome() => $_has(0);
  @$pb.TagNumber(1)
  void clearOutcome() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceChatMutationOutcome ensureOutcome() => $_ensure(0);
}

class SetSpaceChatPostAsChatGrantResponse extends $pb.GeneratedMessage {
  factory SetSpaceChatPostAsChatGrantResponse({
    SpaceChatMutationOutcome? outcome,
  }) {
    final result = create();
    if (outcome != null) result.outcome = outcome;
    return result;
  }

  SetSpaceChatPostAsChatGrantResponse._();

  factory SetSpaceChatPostAsChatGrantResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetSpaceChatPostAsChatGrantResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetSpaceChatPostAsChatGrantResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceChatMutationOutcome>(1, _omitFieldNames ? '' : 'outcome',
        subBuilder: SpaceChatMutationOutcome.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatPostAsChatGrantResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetSpaceChatPostAsChatGrantResponse copyWith(
          void Function(SetSpaceChatPostAsChatGrantResponse) updates) =>
      super.copyWith((message) =>
              updates(message as SetSpaceChatPostAsChatGrantResponse))
          as SetSpaceChatPostAsChatGrantResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetSpaceChatPostAsChatGrantResponse create() =>
      SetSpaceChatPostAsChatGrantResponse._();
  @$core.override
  SetSpaceChatPostAsChatGrantResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetSpaceChatPostAsChatGrantResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          SetSpaceChatPostAsChatGrantResponse>(create);
  static SetSpaceChatPostAsChatGrantResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceChatMutationOutcome get outcome => $_getN(0);
  @$pb.TagNumber(1)
  set outcome(SpaceChatMutationOutcome value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOutcome() => $_has(0);
  @$pb.TagNumber(1)
  void clearOutcome() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceChatMutationOutcome ensureOutcome() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
