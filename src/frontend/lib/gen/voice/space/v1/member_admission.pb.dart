// This is a generated file - do not edit.
//
// Generated from voice/space/v1/member_admission.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/duration.pb.dart'
    as $1;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $2;

import '../../role/v1/member_admission.pb.dart' as $0;
import 'member_admission.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'member_admission.pbenum.dart';

/// Own-DB authority snapshot, never an ACL decision or a portable capability.
/// expected_generation and expected_admission_operation_id are present together.
class GetMemberAdmissionAuthorityRequest extends $pb.GeneratedMessage {
  factory GetMemberAdmissionAuthorityRequest({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? profileId,
    $fixnum.Int64? expectedGeneration,
    $core.String? expectedAdmissionOperationId,
    $core.String? chatId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (expectedGeneration != null)
      result.expectedGeneration = expectedGeneration;
    if (expectedAdmissionOperationId != null)
      result.expectedAdmissionOperationId = expectedAdmissionOperationId;
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  GetMemberAdmissionAuthorityRequest._();

  factory GetMemberAdmissionAuthorityRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMemberAdmissionAuthorityRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMemberAdmissionAuthorityRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'profileId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'expectedGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'expectedAdmissionOperationId')
    ..aOS(6, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberAdmissionAuthorityRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberAdmissionAuthorityRequest copyWith(
          void Function(GetMemberAdmissionAuthorityRequest) updates) =>
      super.copyWith((message) =>
              updates(message as GetMemberAdmissionAuthorityRequest))
          as GetMemberAdmissionAuthorityRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMemberAdmissionAuthorityRequest create() =>
      GetMemberAdmissionAuthorityRequest._();
  @$core.override
  GetMemberAdmissionAuthorityRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMemberAdmissionAuthorityRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMemberAdmissionAuthorityRequest>(
          create);
  static GetMemberAdmissionAuthorityRequest? _defaultInstance;

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
  $core.String get profileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set profileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get expectedGeneration => $_getI64(3);
  @$pb.TagNumber(4)
  set expectedGeneration($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedGeneration() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedGeneration() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get expectedAdmissionOperationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set expectedAdmissionOperationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExpectedAdmissionOperationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpectedAdmissionOperationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get chatId => $_getSZ(5);
  @$pb.TagNumber(6)
  set chatId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasChatId() => $_has(5);
  @$pb.TagNumber(6)
  void clearChatId() => $_clearField(6);
}

class MemberAdmissionLifecycleAuthority extends $pb.GeneratedMessage {
  factory MemberAdmissionLifecycleAuthority({
    $fixnum.Int64? generation,
    MemberAdmissionLifecycleState? state,
  }) {
    final result = create();
    if (generation != null) result.generation = generation;
    if (state != null) result.state = state;
    return result;
  }

  MemberAdmissionLifecycleAuthority._();

  factory MemberAdmissionLifecycleAuthority.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberAdmissionLifecycleAuthority.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberAdmissionLifecycleAuthority',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<MemberAdmissionLifecycleState>(2, _omitFieldNames ? '' : 'state',
        enumValues: MemberAdmissionLifecycleState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionLifecycleAuthority clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionLifecycleAuthority copyWith(
          void Function(MemberAdmissionLifecycleAuthority) updates) =>
      super.copyWith((message) =>
              updates(message as MemberAdmissionLifecycleAuthority))
          as MemberAdmissionLifecycleAuthority;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberAdmissionLifecycleAuthority create() =>
      MemberAdmissionLifecycleAuthority._();
  @$core.override
  MemberAdmissionLifecycleAuthority createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberAdmissionLifecycleAuthority getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberAdmissionLifecycleAuthority>(
          create);
  static MemberAdmissionLifecycleAuthority? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get generation => $_getI64(0);
  @$pb.TagNumber(1)
  set generation($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  MemberAdmissionLifecycleState get state => $_getN(1);
  @$pb.TagNumber(2)
  set state(MemberAdmissionLifecycleState value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);
}

class MemberAdmissionOwnershipAuthority extends $pb.GeneratedMessage {
  factory MemberAdmissionOwnershipAuthority({
    $fixnum.Int64? generation,
    MemberAdmissionOwnershipState? state,
    $core.String? currentOwnerProfileId,
  }) {
    final result = create();
    if (generation != null) result.generation = generation;
    if (state != null) result.state = state;
    if (currentOwnerProfileId != null)
      result.currentOwnerProfileId = currentOwnerProfileId;
    return result;
  }

  MemberAdmissionOwnershipAuthority._();

  factory MemberAdmissionOwnershipAuthority.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberAdmissionOwnershipAuthority.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberAdmissionOwnershipAuthority',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'generation', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<MemberAdmissionOwnershipState>(2, _omitFieldNames ? '' : 'state',
        enumValues: MemberAdmissionOwnershipState.values)
    ..aOS(3, _omitFieldNames ? '' : 'currentOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionOwnershipAuthority clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionOwnershipAuthority copyWith(
          void Function(MemberAdmissionOwnershipAuthority) updates) =>
      super.copyWith((message) =>
              updates(message as MemberAdmissionOwnershipAuthority))
          as MemberAdmissionOwnershipAuthority;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberAdmissionOwnershipAuthority create() =>
      MemberAdmissionOwnershipAuthority._();
  @$core.override
  MemberAdmissionOwnershipAuthority createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberAdmissionOwnershipAuthority getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberAdmissionOwnershipAuthority>(
          create);
  static MemberAdmissionOwnershipAuthority? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get generation => $_getI64(0);
  @$pb.TagNumber(1)
  set generation($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  MemberAdmissionOwnershipState get state => $_getN(1);
  @$pb.TagNumber(2)
  set state(MemberAdmissionOwnershipState value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get currentOwnerProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set currentOwnerProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCurrentOwnerProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCurrentOwnerProfileId() => $_clearField(3);
}

class MemberAdmissionOrdinaryAuthority extends $pb.GeneratedMessage {
  factory MemberAdmissionOrdinaryAuthority({
    $fixnum.Int64? headGeneration,
    $fixnum.Int64? retiredThrough,
    MemberAdmissionSourceState? state,
    $core.bool? explicitLegacy,
    $0.MembershipBinding? publishedBinding,
    $core.List<$core.int>? roleReceiptSha256,
    $0.MembershipBinding? preparingBinding,
    $core.List<$core.int>? preparingAdmissionBindingSha256,
  }) {
    final result = create();
    if (headGeneration != null) result.headGeneration = headGeneration;
    if (retiredThrough != null) result.retiredThrough = retiredThrough;
    if (state != null) result.state = state;
    if (explicitLegacy != null) result.explicitLegacy = explicitLegacy;
    if (publishedBinding != null) result.publishedBinding = publishedBinding;
    if (roleReceiptSha256 != null) result.roleReceiptSha256 = roleReceiptSha256;
    if (preparingBinding != null) result.preparingBinding = preparingBinding;
    if (preparingAdmissionBindingSha256 != null)
      result.preparingAdmissionBindingSha256 = preparingAdmissionBindingSha256;
    return result;
  }

  MemberAdmissionOrdinaryAuthority._();

  factory MemberAdmissionOrdinaryAuthority.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberAdmissionOrdinaryAuthority.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberAdmissionOrdinaryAuthority',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'headGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'retiredThrough', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<MemberAdmissionSourceState>(3, _omitFieldNames ? '' : 'state',
        enumValues: MemberAdmissionSourceState.values)
    ..aOB(4, _omitFieldNames ? '' : 'explicitLegacy')
    ..aOM<$0.MembershipBinding>(5, _omitFieldNames ? '' : 'publishedBinding',
        subBuilder: $0.MembershipBinding.create)
    ..a<$core.List<$core.int>>(
        6, _omitFieldNames ? '' : 'roleReceiptSha256', $pb.PbFieldType.OY)
    ..aOM<$0.MembershipBinding>(7, _omitFieldNames ? '' : 'preparingBinding',
        subBuilder: $0.MembershipBinding.create)
    ..a<$core.List<$core.int>>(
        8,
        _omitFieldNames ? '' : 'preparingAdmissionBindingSha256',
        $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionOrdinaryAuthority clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionOrdinaryAuthority copyWith(
          void Function(MemberAdmissionOrdinaryAuthority) updates) =>
      super.copyWith(
              (message) => updates(message as MemberAdmissionOrdinaryAuthority))
          as MemberAdmissionOrdinaryAuthority;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberAdmissionOrdinaryAuthority create() =>
      MemberAdmissionOrdinaryAuthority._();
  @$core.override
  MemberAdmissionOrdinaryAuthority createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberAdmissionOrdinaryAuthority getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberAdmissionOrdinaryAuthority>(
          create);
  static MemberAdmissionOrdinaryAuthority? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get headGeneration => $_getI64(0);
  @$pb.TagNumber(1)
  set headGeneration($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasHeadGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearHeadGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get retiredThrough => $_getI64(1);
  @$pb.TagNumber(2)
  set retiredThrough($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRetiredThrough() => $_has(1);
  @$pb.TagNumber(2)
  void clearRetiredThrough() => $_clearField(2);

  @$pb.TagNumber(3)
  MemberAdmissionSourceState get state => $_getN(2);
  @$pb.TagNumber(3)
  set state(MemberAdmissionSourceState value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasState() => $_has(2);
  @$pb.TagNumber(3)
  void clearState() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get explicitLegacy => $_getBF(3);
  @$pb.TagNumber(4)
  set explicitLegacy($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExplicitLegacy() => $_has(3);
  @$pb.TagNumber(4)
  void clearExplicitLegacy() => $_clearField(4);

  @$pb.TagNumber(5)
  $0.MembershipBinding get publishedBinding => $_getN(4);
  @$pb.TagNumber(5)
  set publishedBinding($0.MembershipBinding value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPublishedBinding() => $_has(4);
  @$pb.TagNumber(5)
  void clearPublishedBinding() => $_clearField(5);
  @$pb.TagNumber(5)
  $0.MembershipBinding ensurePublishedBinding() => $_ensure(4);

  @$pb.TagNumber(6)
  $core.List<$core.int> get roleReceiptSha256 => $_getN(5);
  @$pb.TagNumber(6)
  set roleReceiptSha256($core.List<$core.int> value) => $_setBytes(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRoleReceiptSha256() => $_has(5);
  @$pb.TagNumber(6)
  void clearRoleReceiptSha256() => $_clearField(6);

  /// PREPARING is inert; this identifies the current candidate for Ensure only.
  @$pb.TagNumber(7)
  $0.MembershipBinding get preparingBinding => $_getN(6);
  @$pb.TagNumber(7)
  set preparingBinding($0.MembershipBinding value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasPreparingBinding() => $_has(6);
  @$pb.TagNumber(7)
  void clearPreparingBinding() => $_clearField(7);
  @$pb.TagNumber(7)
  $0.MembershipBinding ensurePreparingBinding() => $_ensure(6);

  @$pb.TagNumber(8)
  $core.List<$core.int> get preparingAdmissionBindingSha256 => $_getN(7);
  @$pb.TagNumber(8)
  set preparingAdmissionBindingSha256($core.List<$core.int> value) =>
      $_setBytes(7, value);
  @$pb.TagNumber(8)
  $core.bool hasPreparingAdmissionBindingSha256() => $_has(7);
  @$pb.TagNumber(8)
  void clearPreparingAdmissionBindingSha256() => $_clearField(8);
}

class MemberAdmissionCommunityAuthority extends $pb.GeneratedMessage {
  factory MemberAdmissionCommunityAuthority({
    MemberAdmissionSourceState? state,
    $0.MembershipBinding? binding,
    $fixnum.Int64? retiredThrough,
    $fixnum.Int64? rosterSourceRevision,
    $1.Duration? validityRemaining,
  }) {
    final result = create();
    if (state != null) result.state = state;
    if (binding != null) result.binding = binding;
    if (retiredThrough != null) result.retiredThrough = retiredThrough;
    if (rosterSourceRevision != null)
      result.rosterSourceRevision = rosterSourceRevision;
    if (validityRemaining != null) result.validityRemaining = validityRemaining;
    return result;
  }

  MemberAdmissionCommunityAuthority._();

  factory MemberAdmissionCommunityAuthority.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberAdmissionCommunityAuthority.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberAdmissionCommunityAuthority',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aE<MemberAdmissionSourceState>(1, _omitFieldNames ? '' : 'state',
        enumValues: MemberAdmissionSourceState.values)
    ..aOM<$0.MembershipBinding>(2, _omitFieldNames ? '' : 'binding',
        subBuilder: $0.MembershipBinding.create)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'retiredThrough', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'rosterSourceRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$1.Duration>(5, _omitFieldNames ? '' : 'validityRemaining',
        subBuilder: $1.Duration.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionCommunityAuthority clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionCommunityAuthority copyWith(
          void Function(MemberAdmissionCommunityAuthority) updates) =>
      super.copyWith((message) =>
              updates(message as MemberAdmissionCommunityAuthority))
          as MemberAdmissionCommunityAuthority;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberAdmissionCommunityAuthority create() =>
      MemberAdmissionCommunityAuthority._();
  @$core.override
  MemberAdmissionCommunityAuthority createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberAdmissionCommunityAuthority getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MemberAdmissionCommunityAuthority>(
          create);
  static MemberAdmissionCommunityAuthority? _defaultInstance;

  @$pb.TagNumber(1)
  MemberAdmissionSourceState get state => $_getN(0);
  @$pb.TagNumber(1)
  set state(MemberAdmissionSourceState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasState() => $_has(0);
  @$pb.TagNumber(1)
  void clearState() => $_clearField(1);

  @$pb.TagNumber(2)
  $0.MembershipBinding get binding => $_getN(1);
  @$pb.TagNumber(2)
  set binding($0.MembershipBinding value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasBinding() => $_has(1);
  @$pb.TagNumber(2)
  void clearBinding() => $_clearField(2);
  @$pb.TagNumber(2)
  $0.MembershipBinding ensureBinding() => $_ensure(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get retiredThrough => $_getI64(2);
  @$pb.TagNumber(3)
  set retiredThrough($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRetiredThrough() => $_has(2);
  @$pb.TagNumber(3)
  void clearRetiredThrough() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get rosterSourceRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set rosterSourceRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRosterSourceRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearRosterSourceRevision() => $_clearField(4);

  /// Minimum of current owner and member leases, sampled after the Space fence.
  @$pb.TagNumber(5)
  $1.Duration get validityRemaining => $_getN(4);
  @$pb.TagNumber(5)
  set validityRemaining($1.Duration value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasValidityRemaining() => $_has(4);
  @$pb.TagNumber(5)
  void clearValidityRemaining() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Duration ensureValidityRemaining() => $_ensure(4);
}

class MemberAdmissionTreeRelationAuthority extends $pb.GeneratedMessage {
  factory MemberAdmissionTreeRelationAuthority({
    $core.String? chatId,
    $core.bool? present,
    $fixnum.Int64? revision,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    if (present != null) result.present = present;
    if (revision != null) result.revision = revision;
    return result;
  }

  MemberAdmissionTreeRelationAuthority._();

  factory MemberAdmissionTreeRelationAuthority.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MemberAdmissionTreeRelationAuthority.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MemberAdmissionTreeRelationAuthority',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..aOB(2, _omitFieldNames ? '' : 'present')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionTreeRelationAuthority clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MemberAdmissionTreeRelationAuthority copyWith(
          void Function(MemberAdmissionTreeRelationAuthority) updates) =>
      super.copyWith((message) =>
              updates(message as MemberAdmissionTreeRelationAuthority))
          as MemberAdmissionTreeRelationAuthority;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MemberAdmissionTreeRelationAuthority create() =>
      MemberAdmissionTreeRelationAuthority._();
  @$core.override
  MemberAdmissionTreeRelationAuthority createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MemberAdmissionTreeRelationAuthority getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          MemberAdmissionTreeRelationAuthority>(create);
  static MemberAdmissionTreeRelationAuthority? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get present => $_getBF(1);
  @$pb.TagNumber(2)
  set present($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPresent() => $_has(1);
  @$pb.TagNumber(2)
  void clearPresent() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get revision => $_getI64(2);
  @$pb.TagNumber(3)
  set revision($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRevision() => $_has(2);
  @$pb.TagNumber(3)
  void clearRevision() => $_clearField(3);
}

class GetMemberAdmissionAuthorityResponse extends $pb.GeneratedMessage {
  factory GetMemberAdmissionAuthorityResponse({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? profileId,
    $fixnum.Int64? spaceAuthorityRevision,
    MemberAdmissionLifecycleAuthority? lifecycle,
    MemberAdmissionOwnershipAuthority? ownership,
    MemberAdmissionOrdinaryAuthority? ordinary,
    MemberAdmissionCommunityAuthority? community,
    MemberAdmissionTreeRelationAuthority? treeRelation,
    $2.Timestamp? sampledAt,
    $1.Duration? validityRemaining,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (spaceAuthorityRevision != null)
      result.spaceAuthorityRevision = spaceAuthorityRevision;
    if (lifecycle != null) result.lifecycle = lifecycle;
    if (ownership != null) result.ownership = ownership;
    if (ordinary != null) result.ordinary = ordinary;
    if (community != null) result.community = community;
    if (treeRelation != null) result.treeRelation = treeRelation;
    if (sampledAt != null) result.sampledAt = sampledAt;
    if (validityRemaining != null) result.validityRemaining = validityRemaining;
    return result;
  }

  GetMemberAdmissionAuthorityResponse._();

  factory GetMemberAdmissionAuthorityResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMemberAdmissionAuthorityResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMemberAdmissionAuthorityResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'profileId')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'spaceAuthorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<MemberAdmissionLifecycleAuthority>(
        5, _omitFieldNames ? '' : 'lifecycle',
        subBuilder: MemberAdmissionLifecycleAuthority.create)
    ..aOM<MemberAdmissionOwnershipAuthority>(
        6, _omitFieldNames ? '' : 'ownership',
        subBuilder: MemberAdmissionOwnershipAuthority.create)
    ..aOM<MemberAdmissionOrdinaryAuthority>(
        7, _omitFieldNames ? '' : 'ordinary',
        subBuilder: MemberAdmissionOrdinaryAuthority.create)
    ..aOM<MemberAdmissionCommunityAuthority>(
        8, _omitFieldNames ? '' : 'community',
        subBuilder: MemberAdmissionCommunityAuthority.create)
    ..aOM<MemberAdmissionTreeRelationAuthority>(
        9, _omitFieldNames ? '' : 'treeRelation',
        subBuilder: MemberAdmissionTreeRelationAuthority.create)
    ..aOM<$2.Timestamp>(10, _omitFieldNames ? '' : 'sampledAt',
        subBuilder: $2.Timestamp.create)
    ..aOM<$1.Duration>(11, _omitFieldNames ? '' : 'validityRemaining',
        subBuilder: $1.Duration.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberAdmissionAuthorityResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberAdmissionAuthorityResponse copyWith(
          void Function(GetMemberAdmissionAuthorityResponse) updates) =>
      super.copyWith((message) =>
              updates(message as GetMemberAdmissionAuthorityResponse))
          as GetMemberAdmissionAuthorityResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMemberAdmissionAuthorityResponse create() =>
      GetMemberAdmissionAuthorityResponse._();
  @$core.override
  GetMemberAdmissionAuthorityResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMemberAdmissionAuthorityResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          GetMemberAdmissionAuthorityResponse>(create);
  static GetMemberAdmissionAuthorityResponse? _defaultInstance;

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
  $core.String get profileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set profileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get spaceAuthorityRevision => $_getI64(3);
  @$pb.TagNumber(4)
  set spaceAuthorityRevision($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSpaceAuthorityRevision() => $_has(3);
  @$pb.TagNumber(4)
  void clearSpaceAuthorityRevision() => $_clearField(4);

  @$pb.TagNumber(5)
  MemberAdmissionLifecycleAuthority get lifecycle => $_getN(4);
  @$pb.TagNumber(5)
  set lifecycle(MemberAdmissionLifecycleAuthority value) =>
      $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasLifecycle() => $_has(4);
  @$pb.TagNumber(5)
  void clearLifecycle() => $_clearField(5);
  @$pb.TagNumber(5)
  MemberAdmissionLifecycleAuthority ensureLifecycle() => $_ensure(4);

  @$pb.TagNumber(6)
  MemberAdmissionOwnershipAuthority get ownership => $_getN(5);
  @$pb.TagNumber(6)
  set ownership(MemberAdmissionOwnershipAuthority value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasOwnership() => $_has(5);
  @$pb.TagNumber(6)
  void clearOwnership() => $_clearField(6);
  @$pb.TagNumber(6)
  MemberAdmissionOwnershipAuthority ensureOwnership() => $_ensure(5);

  @$pb.TagNumber(7)
  MemberAdmissionOrdinaryAuthority get ordinary => $_getN(6);
  @$pb.TagNumber(7)
  set ordinary(MemberAdmissionOrdinaryAuthority value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasOrdinary() => $_has(6);
  @$pb.TagNumber(7)
  void clearOrdinary() => $_clearField(7);
  @$pb.TagNumber(7)
  MemberAdmissionOrdinaryAuthority ensureOrdinary() => $_ensure(6);

  @$pb.TagNumber(8)
  MemberAdmissionCommunityAuthority get community => $_getN(7);
  @$pb.TagNumber(8)
  set community(MemberAdmissionCommunityAuthority value) =>
      $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCommunity() => $_has(7);
  @$pb.TagNumber(8)
  void clearCommunity() => $_clearField(8);
  @$pb.TagNumber(8)
  MemberAdmissionCommunityAuthority ensureCommunity() => $_ensure(7);

  @$pb.TagNumber(9)
  MemberAdmissionTreeRelationAuthority get treeRelation => $_getN(8);
  @$pb.TagNumber(9)
  set treeRelation(MemberAdmissionTreeRelationAuthority value) =>
      $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasTreeRelation() => $_has(8);
  @$pb.TagNumber(9)
  void clearTreeRelation() => $_clearField(9);
  @$pb.TagNumber(9)
  MemberAdmissionTreeRelationAuthority ensureTreeRelation() => $_ensure(8);

  @$pb.TagNumber(10)
  $2.Timestamp get sampledAt => $_getN(9);
  @$pb.TagNumber(10)
  set sampledAt($2.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasSampledAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearSampledAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $2.Timestamp ensureSampledAt() => $_ensure(9);

  /// Owner-selected observation bound, <=5s and no greater than any leased input.
  /// Role starts its monotonic deadline BEFORE dispatch and subtracts the full
  /// roundtrip/elapsed time. Missing/expired/unknown bounds fail closed.
  @$pb.TagNumber(11)
  $1.Duration get validityRemaining => $_getN(10);
  @$pb.TagNumber(11)
  set validityRemaining($1.Duration value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasValidityRemaining() => $_has(10);
  @$pb.TagNumber(11)
  void clearValidityRemaining() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Duration ensureValidityRemaining() => $_ensure(10);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
