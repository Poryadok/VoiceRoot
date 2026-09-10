// This is a generated file - do not edit.
//
// Generated from voice/role/v1/role.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import '../../chat/v1/chat.pb.dart' as $2;
import 'role.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'role.pbenum.dart';

class BootstrapSpaceRolesRequest extends $pb.GeneratedMessage {
  factory BootstrapSpaceRolesRequest({
    $core.String? spaceId,
    $core.String? ownerProfileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (ownerProfileId != null) result.ownerProfileId = ownerProfileId;
    return result;
  }

  BootstrapSpaceRolesRequest._();

  factory BootstrapSpaceRolesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BootstrapSpaceRolesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BootstrapSpaceRolesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'ownerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BootstrapSpaceRolesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BootstrapSpaceRolesRequest copyWith(
          void Function(BootstrapSpaceRolesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as BootstrapSpaceRolesRequest))
          as BootstrapSpaceRolesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BootstrapSpaceRolesRequest create() => BootstrapSpaceRolesRequest._();
  @$core.override
  BootstrapSpaceRolesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BootstrapSpaceRolesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BootstrapSpaceRolesRequest>(create);
  static BootstrapSpaceRolesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get ownerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set ownerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOwnerProfileId() => $_clearField(2);
}

class BootstrapSpaceRolesResponse extends $pb.GeneratedMessage {
  factory BootstrapSpaceRolesResponse() => create();

  BootstrapSpaceRolesResponse._();

  factory BootstrapSpaceRolesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BootstrapSpaceRolesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BootstrapSpaceRolesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BootstrapSpaceRolesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BootstrapSpaceRolesResponse copyWith(
          void Function(BootstrapSpaceRolesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as BootstrapSpaceRolesResponse))
          as BootstrapSpaceRolesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BootstrapSpaceRolesResponse create() =>
      BootstrapSpaceRolesResponse._();
  @$core.override
  BootstrapSpaceRolesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BootstrapSpaceRolesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BootstrapSpaceRolesResponse>(create);
  static BootstrapSpaceRolesResponse? _defaultInstance;
}

class DeleteRolesCreatedByProfileRequest extends $pb.GeneratedMessage {
  factory DeleteRolesCreatedByProfileRequest({
    $core.String? spaceId,
    $core.String? createdByProfileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (createdByProfileId != null)
      result.createdByProfileId = createdByProfileId;
    return result;
  }

  DeleteRolesCreatedByProfileRequest._();

  factory DeleteRolesCreatedByProfileRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteRolesCreatedByProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteRolesCreatedByProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'createdByProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRolesCreatedByProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRolesCreatedByProfileRequest copyWith(
          void Function(DeleteRolesCreatedByProfileRequest) updates) =>
      super.copyWith((message) =>
              updates(message as DeleteRolesCreatedByProfileRequest))
          as DeleteRolesCreatedByProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteRolesCreatedByProfileRequest create() =>
      DeleteRolesCreatedByProfileRequest._();
  @$core.override
  DeleteRolesCreatedByProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteRolesCreatedByProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteRolesCreatedByProfileRequest>(
          create);
  static DeleteRolesCreatedByProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get createdByProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set createdByProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCreatedByProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCreatedByProfileId() => $_clearField(2);
}

class DeleteRolesCreatedByProfileResponse extends $pb.GeneratedMessage {
  factory DeleteRolesCreatedByProfileResponse() => create();

  DeleteRolesCreatedByProfileResponse._();

  factory DeleteRolesCreatedByProfileResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteRolesCreatedByProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteRolesCreatedByProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRolesCreatedByProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRolesCreatedByProfileResponse copyWith(
          void Function(DeleteRolesCreatedByProfileResponse) updates) =>
      super.copyWith((message) =>
              updates(message as DeleteRolesCreatedByProfileResponse))
          as DeleteRolesCreatedByProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteRolesCreatedByProfileResponse create() =>
      DeleteRolesCreatedByProfileResponse._();
  @$core.override
  DeleteRolesCreatedByProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteRolesCreatedByProfileResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          DeleteRolesCreatedByProfileResponse>(create);
  static DeleteRolesCreatedByProfileResponse? _defaultInstance;
}

class ApplyOwnershipTransferRequest extends $pb.GeneratedMessage {
  factory ApplyOwnershipTransferRequest({
    $core.String? spaceId,
    $core.String? oldOwnerProfileId,
    $core.String? newOwnerProfileId,
    $core.String? operationId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (oldOwnerProfileId != null) result.oldOwnerProfileId = oldOwnerProfileId;
    if (newOwnerProfileId != null) result.newOwnerProfileId = newOwnerProfileId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  ApplyOwnershipTransferRequest._();

  factory ApplyOwnershipTransferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyOwnershipTransferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyOwnershipTransferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'oldOwnerProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'newOwnerProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyOwnershipTransferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyOwnershipTransferRequest copyWith(
          void Function(ApplyOwnershipTransferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyOwnershipTransferRequest))
          as ApplyOwnershipTransferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyOwnershipTransferRequest create() =>
      ApplyOwnershipTransferRequest._();
  @$core.override
  ApplyOwnershipTransferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyOwnershipTransferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyOwnershipTransferRequest>(create);
  static ApplyOwnershipTransferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get oldOwnerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set oldOwnerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOldOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOldOwnerProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get newOwnerProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set newOwnerProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNewOwnerProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearNewOwnerProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get operationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set operationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperationId() => $_clearField(4);
}

class CompensateOwnershipTransferRequest extends $pb.GeneratedMessage {
  factory CompensateOwnershipTransferRequest({
    $core.String? spaceId,
    $core.String? oldOwnerProfileId,
    $core.String? newOwnerProfileId,
    $core.String? operationId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (oldOwnerProfileId != null) result.oldOwnerProfileId = oldOwnerProfileId;
    if (newOwnerProfileId != null) result.newOwnerProfileId = newOwnerProfileId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  CompensateOwnershipTransferRequest._();

  factory CompensateOwnershipTransferRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompensateOwnershipTransferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompensateOwnershipTransferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'oldOwnerProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'newOwnerProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompensateOwnershipTransferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompensateOwnershipTransferRequest copyWith(
          void Function(CompensateOwnershipTransferRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CompensateOwnershipTransferRequest))
          as CompensateOwnershipTransferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompensateOwnershipTransferRequest create() =>
      CompensateOwnershipTransferRequest._();
  @$core.override
  CompensateOwnershipTransferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompensateOwnershipTransferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CompensateOwnershipTransferRequest>(
          create);
  static CompensateOwnershipTransferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get oldOwnerProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set oldOwnerProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOldOwnerProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOldOwnerProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get newOwnerProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set newOwnerProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNewOwnerProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearNewOwnerProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get operationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set operationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperationId() => $_clearField(4);
}

class ApplyOwnershipTransferResponse extends $pb.GeneratedMessage {
  factory ApplyOwnershipTransferResponse({
    $core.String? currentOwnerProfileId,
  }) {
    final result = create();
    if (currentOwnerProfileId != null)
      result.currentOwnerProfileId = currentOwnerProfileId;
    return result;
  }

  ApplyOwnershipTransferResponse._();

  factory ApplyOwnershipTransferResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyOwnershipTransferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyOwnershipTransferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'currentOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyOwnershipTransferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyOwnershipTransferResponse copyWith(
          void Function(ApplyOwnershipTransferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyOwnershipTransferResponse))
          as ApplyOwnershipTransferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyOwnershipTransferResponse create() =>
      ApplyOwnershipTransferResponse._();
  @$core.override
  ApplyOwnershipTransferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyOwnershipTransferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyOwnershipTransferResponse>(create);
  static ApplyOwnershipTransferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get currentOwnerProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set currentOwnerProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCurrentOwnerProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCurrentOwnerProfileId() => $_clearField(1);
}

class CompensateOwnershipTransferResponse extends $pb.GeneratedMessage {
  factory CompensateOwnershipTransferResponse({
    $core.String? currentOwnerProfileId,
  }) {
    final result = create();
    if (currentOwnerProfileId != null)
      result.currentOwnerProfileId = currentOwnerProfileId;
    return result;
  }

  CompensateOwnershipTransferResponse._();

  factory CompensateOwnershipTransferResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CompensateOwnershipTransferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CompensateOwnershipTransferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'currentOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompensateOwnershipTransferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CompensateOwnershipTransferResponse copyWith(
          void Function(CompensateOwnershipTransferResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CompensateOwnershipTransferResponse))
          as CompensateOwnershipTransferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CompensateOwnershipTransferResponse create() =>
      CompensateOwnershipTransferResponse._();
  @$core.override
  CompensateOwnershipTransferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CompensateOwnershipTransferResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          CompensateOwnershipTransferResponse>(create);
  static CompensateOwnershipTransferResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get currentOwnerProfileId => $_getSZ(0);
  @$pb.TagNumber(1)
  set currentOwnerProfileId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCurrentOwnerProfileId() => $_has(0);
  @$pb.TagNumber(1)
  void clearCurrentOwnerProfileId() => $_clearField(1);
}

/// Immutable cross-action identity. Missing intent and protocol_version other than 2
/// are rejected before mutation. UUIDs retain the original old/new ordering.
/// The ledger hashes and retains the deterministic encoding, INCLUDING unknown
/// intent fields; receipts echo that exact intent rather than reconstructing IDs.
/// Space separately binds account/session/proof data in its durable journal.
class OwnershipTransferIntent extends $pb.GeneratedMessage {
  factory OwnershipTransferIntent({
    $core.int? protocolVersion,
    $core.String? spaceId,
    $core.String? oldOwnerProfileId,
    $core.String? newOwnerProfileId,
    $core.String? operationId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (spaceId != null) result.spaceId = spaceId;
    if (oldOwnerProfileId != null) result.oldOwnerProfileId = oldOwnerProfileId;
    if (newOwnerProfileId != null) result.newOwnerProfileId = newOwnerProfileId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  OwnershipTransferIntent._();

  factory OwnershipTransferIntent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OwnershipTransferIntent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OwnershipTransferIntent',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'oldOwnerProfileId')
    ..aOS(4, _omitFieldNames ? '' : 'newOwnerProfileId')
    ..aOS(5, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnershipTransferIntent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnershipTransferIntent copyWith(
          void Function(OwnershipTransferIntent) updates) =>
      super.copyWith((message) => updates(message as OwnershipTransferIntent))
          as OwnershipTransferIntent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OwnershipTransferIntent create() => OwnershipTransferIntent._();
  @$core.override
  OwnershipTransferIntent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OwnershipTransferIntent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OwnershipTransferIntent>(create);
  static OwnershipTransferIntent? _defaultInstance;

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
  $core.String get oldOwnerProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set oldOwnerProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasOldOwnerProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearOldOwnerProfileId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get newOwnerProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set newOwnerProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasNewOwnerProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearNewOwnerProfileId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get operationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set operationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasOperationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearOperationId() => $_clearField(5);
}

class OwnershipTransferReceipt extends $pb.GeneratedMessage {
  factory OwnershipTransferReceipt({
    OwnershipTransferIntent? intent,
    OwnershipTransferState? state,
    $core.String? currentOwnerProfileId,
  }) {
    final result = create();
    if (intent != null) result.intent = intent;
    if (state != null) result.state = state;
    if (currentOwnerProfileId != null)
      result.currentOwnerProfileId = currentOwnerProfileId;
    return result;
  }

  OwnershipTransferReceipt._();

  factory OwnershipTransferReceipt.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OwnershipTransferReceipt.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OwnershipTransferReceipt',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferIntent>(1, _omitFieldNames ? '' : 'intent',
        subBuilder: OwnershipTransferIntent.create)
    ..aE<OwnershipTransferState>(2, _omitFieldNames ? '' : 'state',
        enumValues: OwnershipTransferState.values)
    ..aOS(3, _omitFieldNames ? '' : 'currentOwnerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnershipTransferReceipt clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OwnershipTransferReceipt copyWith(
          void Function(OwnershipTransferReceipt) updates) =>
      super.copyWith((message) => updates(message as OwnershipTransferReceipt))
          as OwnershipTransferReceipt;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OwnershipTransferReceipt create() => OwnershipTransferReceipt._();
  @$core.override
  OwnershipTransferReceipt createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OwnershipTransferReceipt getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OwnershipTransferReceipt>(create);
  static OwnershipTransferReceipt? _defaultInstance;

  @$pb.TagNumber(1)
  OwnershipTransferIntent get intent => $_getN(0);
  @$pb.TagNumber(1)
  set intent(OwnershipTransferIntent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIntent() => $_has(0);
  @$pb.TagNumber(1)
  void clearIntent() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferIntent ensureIntent() => $_ensure(0);

  @$pb.TagNumber(2)
  OwnershipTransferState get state => $_getN(1);
  @$pb.TagNumber(2)
  set state(OwnershipTransferState value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasState() => $_has(1);
  @$pb.TagNumber(2)
  void clearState() => $_clearField(2);

  /// Empty while prepared; new owner when finalized; old owner when aborted.
  @$pb.TagNumber(3)
  $core.String get currentOwnerProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set currentOwnerProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCurrentOwnerProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearCurrentOwnerProfileId() => $_clearField(3);
}

class GetOwnershipTransferCapabilitiesRequest extends $pb.GeneratedMessage {
  factory GetOwnershipTransferCapabilitiesRequest() => create();

  GetOwnershipTransferCapabilitiesRequest._();

  factory GetOwnershipTransferCapabilitiesRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetOwnershipTransferCapabilitiesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetOwnershipTransferCapabilitiesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOwnershipTransferCapabilitiesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOwnershipTransferCapabilitiesRequest copyWith(
          void Function(GetOwnershipTransferCapabilitiesRequest) updates) =>
      super.copyWith((message) =>
              updates(message as GetOwnershipTransferCapabilitiesRequest))
          as GetOwnershipTransferCapabilitiesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetOwnershipTransferCapabilitiesRequest create() =>
      GetOwnershipTransferCapabilitiesRequest._();
  @$core.override
  GetOwnershipTransferCapabilitiesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetOwnershipTransferCapabilitiesRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          GetOwnershipTransferCapabilitiesRequest>(create);
  static GetOwnershipTransferCapabilitiesRequest? _defaultInstance;
}

/// An authenticated protocol 2 and all three exact full transition RPC paths are
/// required before journal reservation. Missing/incompatible capability denies;
/// no network-error or old-server fallback to v1 is permitted. Schema availability
/// alone does not advertise support or activate the production ownership flow.
class GetOwnershipTransferCapabilitiesResponse extends $pb.GeneratedMessage {
  factory GetOwnershipTransferCapabilitiesResponse({
    $core.int? protocolVersion,
    $core.Iterable<$core.String>? supportedMethods,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (supportedMethods != null)
      result.supportedMethods.addAll(supportedMethods);
    return result;
  }

  GetOwnershipTransferCapabilitiesResponse._();

  factory GetOwnershipTransferCapabilitiesResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetOwnershipTransferCapabilitiesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetOwnershipTransferCapabilitiesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..pPS(2, _omitFieldNames ? '' : 'supportedMethods')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOwnershipTransferCapabilitiesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetOwnershipTransferCapabilitiesResponse copyWith(
          void Function(GetOwnershipTransferCapabilitiesResponse) updates) =>
      super.copyWith((message) =>
              updates(message as GetOwnershipTransferCapabilitiesResponse))
          as GetOwnershipTransferCapabilitiesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetOwnershipTransferCapabilitiesResponse create() =>
      GetOwnershipTransferCapabilitiesResponse._();
  @$core.override
  GetOwnershipTransferCapabilitiesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetOwnershipTransferCapabilitiesResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          GetOwnershipTransferCapabilitiesResponse>(create);
  static GetOwnershipTransferCapabilitiesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get supportedMethods => $_getList(1);
}

/// Each action retains its first accepted full deterministic request hash,
/// INCLUDING unknown wrapper fields, separately from the shared intent hash.
/// JWT request_hash binds the full request; its rpc claim binds the exact action.
/// Changed intent or same-action request conflicts with ALREADY_EXISTS.
class PrepareOwnershipTransferRequest extends $pb.GeneratedMessage {
  factory PrepareOwnershipTransferRequest({
    OwnershipTransferIntent? intent,
  }) {
    final result = create();
    if (intent != null) result.intent = intent;
    return result;
  }

  PrepareOwnershipTransferRequest._();

  factory PrepareOwnershipTransferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrepareOwnershipTransferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrepareOwnershipTransferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferIntent>(1, _omitFieldNames ? '' : 'intent',
        subBuilder: OwnershipTransferIntent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareOwnershipTransferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareOwnershipTransferRequest copyWith(
          void Function(PrepareOwnershipTransferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as PrepareOwnershipTransferRequest))
          as PrepareOwnershipTransferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrepareOwnershipTransferRequest create() =>
      PrepareOwnershipTransferRequest._();
  @$core.override
  PrepareOwnershipTransferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PrepareOwnershipTransferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PrepareOwnershipTransferRequest>(
          create);
  static PrepareOwnershipTransferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  OwnershipTransferIntent get intent => $_getN(0);
  @$pb.TagNumber(1)
  set intent(OwnershipTransferIntent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIntent() => $_has(0);
  @$pb.TagNumber(1)
  void clearIntent() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferIntent ensureIntent() => $_ensure(0);
}

class PrepareOwnershipTransferResponse extends $pb.GeneratedMessage {
  factory PrepareOwnershipTransferResponse({
    OwnershipTransferReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  PrepareOwnershipTransferResponse._();

  factory PrepareOwnershipTransferResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PrepareOwnershipTransferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PrepareOwnershipTransferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: OwnershipTransferReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareOwnershipTransferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PrepareOwnershipTransferResponse copyWith(
          void Function(PrepareOwnershipTransferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as PrepareOwnershipTransferResponse))
          as PrepareOwnershipTransferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PrepareOwnershipTransferResponse create() =>
      PrepareOwnershipTransferResponse._();
  @$core.override
  PrepareOwnershipTransferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PrepareOwnershipTransferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PrepareOwnershipTransferResponse>(
          create);
  static PrepareOwnershipTransferResponse? _defaultInstance;

  /// Only PREPARED; terminal operations cannot be prepared again.
  @$pb.TagNumber(1)
  OwnershipTransferReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(OwnershipTransferReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferReceipt ensureReceipt() => $_ensure(0);
}

/// Same immutable intent and per-action full request binding rules as Prepare.
/// Only a durable Space commit decision may issue Finalize; absent/aborted denies.
class FinalizeOwnershipTransferRequest extends $pb.GeneratedMessage {
  factory FinalizeOwnershipTransferRequest({
    OwnershipTransferIntent? intent,
  }) {
    final result = create();
    if (intent != null) result.intent = intent;
    return result;
  }

  FinalizeOwnershipTransferRequest._();

  factory FinalizeOwnershipTransferRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FinalizeOwnershipTransferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FinalizeOwnershipTransferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferIntent>(1, _omitFieldNames ? '' : 'intent',
        subBuilder: OwnershipTransferIntent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FinalizeOwnershipTransferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FinalizeOwnershipTransferRequest copyWith(
          void Function(FinalizeOwnershipTransferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as FinalizeOwnershipTransferRequest))
          as FinalizeOwnershipTransferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FinalizeOwnershipTransferRequest create() =>
      FinalizeOwnershipTransferRequest._();
  @$core.override
  FinalizeOwnershipTransferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FinalizeOwnershipTransferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FinalizeOwnershipTransferRequest>(
          create);
  static FinalizeOwnershipTransferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  OwnershipTransferIntent get intent => $_getN(0);
  @$pb.TagNumber(1)
  set intent(OwnershipTransferIntent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIntent() => $_has(0);
  @$pb.TagNumber(1)
  void clearIntent() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferIntent ensureIntent() => $_ensure(0);
}

class FinalizeOwnershipTransferResponse extends $pb.GeneratedMessage {
  factory FinalizeOwnershipTransferResponse({
    OwnershipTransferReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  FinalizeOwnershipTransferResponse._();

  factory FinalizeOwnershipTransferResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FinalizeOwnershipTransferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FinalizeOwnershipTransferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: OwnershipTransferReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FinalizeOwnershipTransferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FinalizeOwnershipTransferResponse copyWith(
          void Function(FinalizeOwnershipTransferResponse) updates) =>
      super.copyWith((message) =>
              updates(message as FinalizeOwnershipTransferResponse))
          as FinalizeOwnershipTransferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FinalizeOwnershipTransferResponse create() =>
      FinalizeOwnershipTransferResponse._();
  @$core.override
  FinalizeOwnershipTransferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FinalizeOwnershipTransferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FinalizeOwnershipTransferResponse>(
          create);
  static FinalizeOwnershipTransferResponse? _defaultInstance;

  /// Only FINALIZED, including identical terminal retries.
  @$pb.TagNumber(1)
  OwnershipTransferReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(OwnershipTransferReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferReceipt ensureReceipt() => $_ensure(0);
}

/// Same immutable intent and per-action full request binding rules as Prepare.
/// Only a durable Space abort decision may issue Abort; absent Abort fences delayed
/// Prepare after sole-old-owner validation. A finalized operation cannot abort.
class AbortOwnershipTransferRequest extends $pb.GeneratedMessage {
  factory AbortOwnershipTransferRequest({
    OwnershipTransferIntent? intent,
  }) {
    final result = create();
    if (intent != null) result.intent = intent;
    return result;
  }

  AbortOwnershipTransferRequest._();

  factory AbortOwnershipTransferRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AbortOwnershipTransferRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AbortOwnershipTransferRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferIntent>(1, _omitFieldNames ? '' : 'intent',
        subBuilder: OwnershipTransferIntent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AbortOwnershipTransferRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AbortOwnershipTransferRequest copyWith(
          void Function(AbortOwnershipTransferRequest) updates) =>
      super.copyWith(
              (message) => updates(message as AbortOwnershipTransferRequest))
          as AbortOwnershipTransferRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AbortOwnershipTransferRequest create() =>
      AbortOwnershipTransferRequest._();
  @$core.override
  AbortOwnershipTransferRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AbortOwnershipTransferRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AbortOwnershipTransferRequest>(create);
  static AbortOwnershipTransferRequest? _defaultInstance;

  @$pb.TagNumber(1)
  OwnershipTransferIntent get intent => $_getN(0);
  @$pb.TagNumber(1)
  set intent(OwnershipTransferIntent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasIntent() => $_has(0);
  @$pb.TagNumber(1)
  void clearIntent() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferIntent ensureIntent() => $_ensure(0);
}

class AbortOwnershipTransferResponse extends $pb.GeneratedMessage {
  factory AbortOwnershipTransferResponse({
    OwnershipTransferReceipt? receipt,
  }) {
    final result = create();
    if (receipt != null) result.receipt = receipt;
    return result;
  }

  AbortOwnershipTransferResponse._();

  factory AbortOwnershipTransferResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AbortOwnershipTransferResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AbortOwnershipTransferResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OwnershipTransferReceipt>(1, _omitFieldNames ? '' : 'receipt',
        subBuilder: OwnershipTransferReceipt.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AbortOwnershipTransferResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AbortOwnershipTransferResponse copyWith(
          void Function(AbortOwnershipTransferResponse) updates) =>
      super.copyWith(
              (message) => updates(message as AbortOwnershipTransferResponse))
          as AbortOwnershipTransferResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AbortOwnershipTransferResponse create() =>
      AbortOwnershipTransferResponse._();
  @$core.override
  AbortOwnershipTransferResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AbortOwnershipTransferResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AbortOwnershipTransferResponse>(create);
  static AbortOwnershipTransferResponse? _defaultInstance;

  /// Only ABORTED, including identical terminal retries.
  @$pb.TagNumber(1)
  OwnershipTransferReceipt get receipt => $_getN(0);
  @$pb.TagNumber(1)
  set receipt(OwnershipTransferReceipt value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReceipt() => $_has(0);
  @$pb.TagNumber(1)
  void clearReceipt() => $_clearField(1);
  @$pb.TagNumber(1)
  OwnershipTransferReceipt ensureReceipt() => $_ensure(0);
}

class Role extends $pb.GeneratedMessage {
  factory Role({
    $core.String? id,
    $core.String? spaceId,
    $core.String? name,
    $fixnum.Int64? permissionsMask,
    $core.int? position,
    $core.bool? managed,
    $1.Timestamp? createdAt,
    $core.String? createdByProfileId,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (permissionsMask != null) result.permissionsMask = permissionsMask;
    if (position != null) result.position = position;
    if (managed != null) result.managed = managed;
    if (createdAt != null) result.createdAt = createdAt;
    if (createdByProfileId != null)
      result.createdByProfileId = createdByProfileId;
    return result;
  }

  Role._();

  factory Role.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Role.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Role',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'spaceId')
    ..aOS(3, _omitFieldNames ? '' : 'name')
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'permissionsMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(5, _omitFieldNames ? '' : 'position')
    ..aOB(6, _omitFieldNames ? '' : 'managed')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(8, _omitFieldNames ? '' : 'createdByProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Role clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Role copyWith(void Function(Role) updates) =>
      super.copyWith((message) => updates(message as Role)) as Role;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Role create() => Role._();
  @$core.override
  Role createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Role getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Role>(create);
  static Role? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get spaceId => $_getSZ(1);
  @$pb.TagNumber(2)
  set spaceId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSpaceId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpaceId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get name => $_getSZ(2);
  @$pb.TagNumber(3)
  set name($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasName() => $_has(2);
  @$pb.TagNumber(3)
  void clearName() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get permissionsMask => $_getI64(3);
  @$pb.TagNumber(4)
  set permissionsMask($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPermissionsMask() => $_has(3);
  @$pb.TagNumber(4)
  void clearPermissionsMask() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.int get position => $_getIZ(4);
  @$pb.TagNumber(5)
  set position($core.int value) => $_setSignedInt32(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPosition() => $_has(4);
  @$pb.TagNumber(5)
  void clearPosition() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get managed => $_getBF(5);
  @$pb.TagNumber(6)
  set managed($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasManaged() => $_has(5);
  @$pb.TagNumber(6)
  void clearManaged() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.Timestamp get createdAt => $_getN(6);
  @$pb.TagNumber(7)
  set createdAt($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasCreatedAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearCreatedAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureCreatedAt() => $_ensure(6);

  @$pb.TagNumber(8)
  $core.String get createdByProfileId => $_getSZ(7);
  @$pb.TagNumber(8)
  set createdByProfileId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasCreatedByProfileId() => $_has(7);
  @$pb.TagNumber(8)
  void clearCreatedByProfileId() => $_clearField(8);
}

class CreateRoleRequest extends $pb.GeneratedMessage {
  factory CreateRoleRequest({
    $core.String? spaceId,
    $core.String? name,
    $fixnum.Int64? permissionsMask,
    $core.int? position,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (name != null) result.name = name;
    if (permissionsMask != null) result.permissionsMask = permissionsMask;
    if (position != null) result.position = position;
    return result;
  }

  CreateRoleRequest._();

  factory CreateRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'permissionsMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(4, _omitFieldNames ? '' : 'position')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateRoleRequest copyWith(void Function(CreateRoleRequest) updates) =>
      super.copyWith((message) => updates(message as CreateRoleRequest))
          as CreateRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateRoleRequest create() => CreateRoleRequest._();
  @$core.override
  CreateRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateRoleRequest>(create);
  static CreateRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get permissionsMask => $_getI64(2);
  @$pb.TagNumber(3)
  set permissionsMask($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPermissionsMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearPermissionsMask() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get position => $_getIZ(3);
  @$pb.TagNumber(4)
  set position($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPosition() => $_has(3);
  @$pb.TagNumber(4)
  void clearPosition() => $_clearField(4);
}

class UpdateRoleRequest extends $pb.GeneratedMessage {
  factory UpdateRoleRequest({
    $core.String? roleId,
    $core.String? name,
    $fixnum.Int64? permissionsMask,
    $core.int? position,
  }) {
    final result = create();
    if (roleId != null) result.roleId = roleId;
    if (name != null) result.name = name;
    if (permissionsMask != null) result.permissionsMask = permissionsMask;
    if (position != null) result.position = position;
    return result;
  }

  UpdateRoleRequest._();

  factory UpdateRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roleId')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'permissionsMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(4, _omitFieldNames ? '' : 'position')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateRoleRequest copyWith(void Function(UpdateRoleRequest) updates) =>
      super.copyWith((message) => updates(message as UpdateRoleRequest))
          as UpdateRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateRoleRequest create() => UpdateRoleRequest._();
  @$core.override
  UpdateRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateRoleRequest>(create);
  static UpdateRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roleId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roleId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoleId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoleId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get name => $_getSZ(1);
  @$pb.TagNumber(2)
  set name($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasName() => $_has(1);
  @$pb.TagNumber(2)
  void clearName() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get permissionsMask => $_getI64(2);
  @$pb.TagNumber(3)
  set permissionsMask($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPermissionsMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearPermissionsMask() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get position => $_getIZ(3);
  @$pb.TagNumber(4)
  set position($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPosition() => $_has(3);
  @$pb.TagNumber(4)
  void clearPosition() => $_clearField(4);
}

class DeleteRoleRequest extends $pb.GeneratedMessage {
  factory DeleteRoleRequest({
    $core.String? roleId,
  }) {
    final result = create();
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  DeleteRoleRequest._();

  factory DeleteRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRoleRequest copyWith(void Function(DeleteRoleRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteRoleRequest))
          as DeleteRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteRoleRequest create() => DeleteRoleRequest._();
  @$core.override
  DeleteRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteRoleRequest>(create);
  static DeleteRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roleId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roleId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoleId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoleId() => $_clearField(1);
}

class ListRolesRequest extends $pb.GeneratedMessage {
  factory ListRolesRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  ListRolesRequest._();

  factory ListRolesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListRolesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRolesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRolesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRolesRequest copyWith(void Function(ListRolesRequest) updates) =>
      super.copyWith((message) => updates(message as ListRolesRequest))
          as ListRolesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListRolesRequest create() => ListRolesRequest._();
  @$core.override
  ListRolesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListRolesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListRolesRequest>(create);
  static ListRolesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class RoleList extends $pb.GeneratedMessage {
  factory RoleList({
    $core.Iterable<Role>? roles,
  }) {
    final result = create();
    if (roles != null) result.roles.addAll(roles);
    return result;
  }

  RoleList._();

  factory RoleList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RoleList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RoleList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..pPM<Role>(1, _omitFieldNames ? '' : 'roles', subBuilder: Role.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RoleList copyWith(void Function(RoleList) updates) =>
      super.copyWith((message) => updates(message as RoleList)) as RoleList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RoleList create() => RoleList._();
  @$core.override
  RoleList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RoleList getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<RoleList>(create);
  static RoleList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Role> get roles => $_getList(0);
}

class ReorderRolesRequest extends $pb.GeneratedMessage {
  factory ReorderRolesRequest({
    $core.String? spaceId,
    $core.Iterable<$core.String>? orderedRoleIds,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (orderedRoleIds != null) result.orderedRoleIds.addAll(orderedRoleIds);
    return result;
  }

  ReorderRolesRequest._();

  factory ReorderRolesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReorderRolesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReorderRolesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..pPS(2, _omitFieldNames ? '' : 'orderedRoleIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderRolesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderRolesRequest copyWith(void Function(ReorderRolesRequest) updates) =>
      super.copyWith((message) => updates(message as ReorderRolesRequest))
          as ReorderRolesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReorderRolesRequest create() => ReorderRolesRequest._();
  @$core.override
  ReorderRolesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReorderRolesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReorderRolesRequest>(create);
  static ReorderRolesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get orderedRoleIds => $_getList(1);
}

class AssignRoleRequest extends $pb.GeneratedMessage {
  factory AssignRoleRequest({
    $core.String? spaceId,
    $core.String? profileId,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  AssignRoleRequest._();

  factory AssignRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AssignRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssignRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssignRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssignRoleRequest copyWith(void Function(AssignRoleRequest) updates) =>
      super.copyWith((message) => updates(message as AssignRoleRequest))
          as AssignRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AssignRoleRequest create() => AssignRoleRequest._();
  @$core.override
  AssignRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AssignRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AssignRoleRequest>(create);
  static AssignRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roleId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roleId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoleId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoleId() => $_clearField(3);
}

class RevokeRoleRequest extends $pb.GeneratedMessage {
  factory RevokeRoleRequest({
    $core.String? spaceId,
    $core.String? profileId,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  RevokeRoleRequest._();

  factory RevokeRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeRoleRequest copyWith(void Function(RevokeRoleRequest) updates) =>
      super.copyWith((message) => updates(message as RevokeRoleRequest))
          as RevokeRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeRoleRequest create() => RevokeRoleRequest._();
  @$core.override
  RevokeRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeRoleRequest>(create);
  static RevokeRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roleId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roleId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoleId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoleId() => $_clearField(3);
}

class GetMemberRolesRequest extends $pb.GeneratedMessage {
  factory GetMemberRolesRequest({
    $core.String? spaceId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  GetMemberRolesRequest._();

  factory GetMemberRolesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMemberRolesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMemberRolesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberRolesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberRolesRequest copyWith(
          void Function(GetMemberRolesRequest) updates) =>
      super.copyWith((message) => updates(message as GetMemberRolesRequest))
          as GetMemberRolesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMemberRolesRequest create() => GetMemberRolesRequest._();
  @$core.override
  GetMemberRolesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMemberRolesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMemberRolesRequest>(create);
  static GetMemberRolesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class SetChatOverrideRequest extends $pb.GeneratedMessage {
  factory SetChatOverrideRequest({
    $core.String? spaceId,
    $2.ChatRef? chat,
    $fixnum.Int64? denyMask,
    $fixnum.Int64? allowMask,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (chat != null) result.chat = chat;
    if (denyMask != null) result.denyMask = denyMask;
    if (allowMask != null) result.allowMask = allowMask;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  SetChatOverrideRequest._();

  factory SetChatOverrideRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetChatOverrideRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetChatOverrideRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$2.ChatRef>(2, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'denyMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'allowMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatOverrideRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatOverrideRequest copyWith(
          void Function(SetChatOverrideRequest) updates) =>
      super.copyWith((message) => updates(message as SetChatOverrideRequest))
          as SetChatOverrideRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetChatOverrideRequest create() => SetChatOverrideRequest._();
  @$core.override
  SetChatOverrideRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetChatOverrideRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetChatOverrideRequest>(create);
  static SetChatOverrideRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.ChatRef get chat => $_getN(1);
  @$pb.TagNumber(2)
  set chat($2.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.ChatRef ensureChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $fixnum.Int64 get denyMask => $_getI64(2);
  @$pb.TagNumber(3)
  set denyMask($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDenyMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearDenyMask() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get allowMask => $_getI64(3);
  @$pb.TagNumber(4)
  set allowMask($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAllowMask() => $_has(3);
  @$pb.TagNumber(4)
  void clearAllowMask() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roleId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roleId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoleId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoleId() => $_clearField(5);
}

class RemoveChatOverrideRequest extends $pb.GeneratedMessage {
  factory RemoveChatOverrideRequest({
    $core.String? spaceId,
    $2.ChatRef? chat,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (chat != null) result.chat = chat;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  RemoveChatOverrideRequest._();

  factory RemoveChatOverrideRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveChatOverrideRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveChatOverrideRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$2.ChatRef>(2, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveChatOverrideRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveChatOverrideRequest copyWith(
          void Function(RemoveChatOverrideRequest) updates) =>
      super.copyWith((message) => updates(message as RemoveChatOverrideRequest))
          as RemoveChatOverrideRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveChatOverrideRequest create() => RemoveChatOverrideRequest._();
  @$core.override
  RemoveChatOverrideRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveChatOverrideRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveChatOverrideRequest>(create);
  static RemoveChatOverrideRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.ChatRef get chat => $_getN(1);
  @$pb.TagNumber(2)
  set chat($2.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.ChatRef ensureChat() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get roleId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roleId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoleId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoleId() => $_clearField(3);
}

class GetChatOverridesRequest extends $pb.GeneratedMessage {
  factory GetChatOverridesRequest({
    $core.String? spaceId,
    $2.ChatRef? filterChat,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (filterChat != null) result.filterChat = filterChat;
    return result;
  }

  GetChatOverridesRequest._();

  factory GetChatOverridesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatOverridesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatOverridesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$2.ChatRef>(2, _omitFieldNames ? '' : 'filterChat',
        subBuilder: $2.ChatRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatOverridesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatOverridesRequest copyWith(
          void Function(GetChatOverridesRequest) updates) =>
      super.copyWith((message) => updates(message as GetChatOverridesRequest))
          as GetChatOverridesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatOverridesRequest create() => GetChatOverridesRequest._();
  @$core.override
  GetChatOverridesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatOverridesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatOverridesRequest>(create);
  static GetChatOverridesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.ChatRef get filterChat => $_getN(1);
  @$pb.TagNumber(2)
  set filterChat($2.ChatRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFilterChat() => $_has(1);
  @$pb.TagNumber(2)
  void clearFilterChat() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.ChatRef ensureFilterChat() => $_ensure(1);
}

class SetVoiceRoomOverrideRequest extends $pb.GeneratedMessage {
  factory SetVoiceRoomOverrideRequest({
    $core.String? spaceId,
    $core.String? voiceRoomId,
    $fixnum.Int64? denyMask,
    $fixnum.Int64? allowMask,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (denyMask != null) result.denyMask = denyMask;
    if (allowMask != null) result.allowMask = allowMask;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  SetVoiceRoomOverrideRequest._();

  factory SetVoiceRoomOverrideRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetVoiceRoomOverrideRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetVoiceRoomOverrideRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'denyMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'allowMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVoiceRoomOverrideRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVoiceRoomOverrideRequest copyWith(
          void Function(SetVoiceRoomOverrideRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SetVoiceRoomOverrideRequest))
          as SetVoiceRoomOverrideRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetVoiceRoomOverrideRequest create() =>
      SetVoiceRoomOverrideRequest._();
  @$core.override
  SetVoiceRoomOverrideRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetVoiceRoomOverrideRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetVoiceRoomOverrideRequest>(create);
  static SetVoiceRoomOverrideRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get denyMask => $_getI64(2);
  @$pb.TagNumber(3)
  set denyMask($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDenyMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearDenyMask() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get allowMask => $_getI64(3);
  @$pb.TagNumber(4)
  set allowMask($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAllowMask() => $_has(3);
  @$pb.TagNumber(4)
  void clearAllowMask() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roleId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roleId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoleId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoleId() => $_clearField(5);
}

class RemoveVoiceRoomOverrideRequest extends $pb.GeneratedMessage {
  factory RemoveVoiceRoomOverrideRequest({
    $core.String? spaceId,
    $core.String? voiceRoomId,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  RemoveVoiceRoomOverrideRequest._();

  factory RemoveVoiceRoomOverrideRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveVoiceRoomOverrideRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveVoiceRoomOverrideRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(3, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveVoiceRoomOverrideRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveVoiceRoomOverrideRequest copyWith(
          void Function(RemoveVoiceRoomOverrideRequest) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveVoiceRoomOverrideRequest))
          as RemoveVoiceRoomOverrideRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveVoiceRoomOverrideRequest create() =>
      RemoveVoiceRoomOverrideRequest._();
  @$core.override
  RemoveVoiceRoomOverrideRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveVoiceRoomOverrideRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveVoiceRoomOverrideRequest>(create);
  static RemoveVoiceRoomOverrideRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get roleId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roleId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoleId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoleId() => $_clearField(3);
}

class GetVoiceRoomOverridesRequest extends $pb.GeneratedMessage {
  factory GetVoiceRoomOverridesRequest({
    $core.String? spaceId,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  GetVoiceRoomOverridesRequest._();

  factory GetVoiceRoomOverridesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceRoomOverridesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceRoomOverridesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceRoomOverridesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceRoomOverridesRequest copyWith(
          void Function(GetVoiceRoomOverridesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetVoiceRoomOverridesRequest))
          as GetVoiceRoomOverridesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceRoomOverridesRequest create() =>
      GetVoiceRoomOverridesRequest._();
  @$core.override
  GetVoiceRoomOverridesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceRoomOverridesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceRoomOverridesRequest>(create);
  static GetVoiceRoomOverridesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);
}

class OverrideList extends $pb.GeneratedMessage {
  factory OverrideList({
    $core.Iterable<PermissionOverride>? overrides,
  }) {
    final result = create();
    if (overrides != null) result.overrides.addAll(overrides);
    return result;
  }

  OverrideList._();

  factory OverrideList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OverrideList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OverrideList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..pPM<PermissionOverride>(1, _omitFieldNames ? '' : 'overrides',
        subBuilder: PermissionOverride.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OverrideList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OverrideList copyWith(void Function(OverrideList) updates) =>
      super.copyWith((message) => updates(message as OverrideList))
          as OverrideList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OverrideList create() => OverrideList._();
  @$core.override
  OverrideList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OverrideList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OverrideList>(create);
  static OverrideList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PermissionOverride> get overrides => $_getList(0);
}

class PermissionOverride extends $pb.GeneratedMessage {
  factory PermissionOverride({
    $2.ChatRef? chat,
    $core.String? voiceRoomId,
    $fixnum.Int64? denyMask,
    $fixnum.Int64? allowMask,
    $core.String? roleId,
    $core.String? roleName,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (denyMask != null) result.denyMask = denyMask;
    if (allowMask != null) result.allowMask = allowMask;
    if (roleId != null) result.roleId = roleId;
    if (roleName != null) result.roleName = roleName;
    return result;
  }

  PermissionOverride._();

  factory PermissionOverride.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PermissionOverride.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PermissionOverride',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<$2.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..a<$fixnum.Int64>(
        3, _omitFieldNames ? '' : 'denyMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        4, _omitFieldNames ? '' : 'allowMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(5, _omitFieldNames ? '' : 'roleId')
    ..aOS(6, _omitFieldNames ? '' : 'roleName')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PermissionOverride clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PermissionOverride copyWith(void Function(PermissionOverride) updates) =>
      super.copyWith((message) => updates(message as PermissionOverride))
          as PermissionOverride;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PermissionOverride create() => PermissionOverride._();
  @$core.override
  PermissionOverride createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PermissionOverride getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PermissionOverride>(create);
  static PermissionOverride? _defaultInstance;

  @$pb.TagNumber(1)
  $2.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($2.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get denyMask => $_getI64(2);
  @$pb.TagNumber(3)
  set denyMask($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDenyMask() => $_has(2);
  @$pb.TagNumber(3)
  void clearDenyMask() => $_clearField(3);

  @$pb.TagNumber(4)
  $fixnum.Int64 get allowMask => $_getI64(3);
  @$pb.TagNumber(4)
  set allowMask($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAllowMask() => $_has(3);
  @$pb.TagNumber(4)
  void clearAllowMask() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get roleId => $_getSZ(4);
  @$pb.TagNumber(5)
  set roleId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasRoleId() => $_has(4);
  @$pb.TagNumber(5)
  void clearRoleId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get roleName => $_getSZ(5);
  @$pb.TagNumber(6)
  set roleName($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasRoleName() => $_has(5);
  @$pb.TagNumber(6)
  void clearRoleName() => $_clearField(6);
}

class SetDefaultJoinRoleRequest extends $pb.GeneratedMessage {
  factory SetDefaultJoinRoleRequest({
    $core.String? spaceId,
    $core.String? roleId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (roleId != null) result.roleId = roleId;
    return result;
  }

  SetDefaultJoinRoleRequest._();

  factory SetDefaultJoinRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetDefaultJoinRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetDefaultJoinRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'roleId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultJoinRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultJoinRoleRequest copyWith(
          void Function(SetDefaultJoinRoleRequest) updates) =>
      super.copyWith((message) => updates(message as SetDefaultJoinRoleRequest))
          as SetDefaultJoinRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetDefaultJoinRoleRequest create() => SetDefaultJoinRoleRequest._();
  @$core.override
  SetDefaultJoinRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetDefaultJoinRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetDefaultJoinRoleRequest>(create);
  static SetDefaultJoinRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get roleId => $_getSZ(1);
  @$pb.TagNumber(2)
  set roleId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRoleId() => $_has(1);
  @$pb.TagNumber(2)
  void clearRoleId() => $_clearField(2);
}

class GetDefaultJoinRoleRequest extends $pb.GeneratedMessage {
  factory GetDefaultJoinRoleRequest({
    $core.String? spaceId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    return result;
  }

  GetDefaultJoinRoleRequest._();

  factory GetDefaultJoinRoleRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDefaultJoinRoleRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDefaultJoinRoleRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDefaultJoinRoleRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDefaultJoinRoleRequest copyWith(
          void Function(GetDefaultJoinRoleRequest) updates) =>
      super.copyWith((message) => updates(message as GetDefaultJoinRoleRequest))
          as GetDefaultJoinRoleRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDefaultJoinRoleRequest create() => GetDefaultJoinRoleRequest._();
  @$core.override
  GetDefaultJoinRoleRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDefaultJoinRoleRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDefaultJoinRoleRequest>(create);
  static GetDefaultJoinRoleRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);
}

class CheckPermissionRequest extends $pb.GeneratedMessage {
  factory CheckPermissionRequest({
    $core.String? spaceId,
    $core.String? profileId,
    $core.String? permissionName,
    $2.ChatRef? chat,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (permissionName != null) result.permissionName = permissionName;
    if (chat != null) result.chat = chat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  CheckPermissionRequest._();

  factory CheckPermissionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckPermissionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckPermissionRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'permissionName')
    ..aOM<$2.ChatRef>(4, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckPermissionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckPermissionRequest copyWith(
          void Function(CheckPermissionRequest) updates) =>
      super.copyWith((message) => updates(message as CheckPermissionRequest))
          as CheckPermissionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckPermissionRequest create() => CheckPermissionRequest._();
  @$core.override
  CheckPermissionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckPermissionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckPermissionRequest>(create);
  static CheckPermissionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get permissionName => $_getSZ(2);
  @$pb.TagNumber(3)
  set permissionName($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPermissionName() => $_has(2);
  @$pb.TagNumber(3)
  void clearPermissionName() => $_clearField(3);

  @$pb.TagNumber(4)
  $2.ChatRef get chat => $_getN(3);
  @$pb.TagNumber(4)
  set chat($2.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $2.ChatRef ensureChat() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get voiceRoomId => $_getSZ(4);
  @$pb.TagNumber(5)
  set voiceRoomId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasVoiceRoomId() => $_has(4);
  @$pb.TagNumber(5)
  void clearVoiceRoomId() => $_clearField(5);
}

class CheckPermissionResponse extends $pb.GeneratedMessage {
  factory CheckPermissionResponse({
    $core.bool? allowed,
  }) {
    final result = create();
    if (allowed != null) result.allowed = allowed;
    return result;
  }

  CheckPermissionResponse._();

  factory CheckPermissionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckPermissionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckPermissionResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'allowed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckPermissionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckPermissionResponse copyWith(
          void Function(CheckPermissionResponse) updates) =>
      super.copyWith((message) => updates(message as CheckPermissionResponse))
          as CheckPermissionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckPermissionResponse create() => CheckPermissionResponse._();
  @$core.override
  CheckPermissionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckPermissionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckPermissionResponse>(create);
  static CheckPermissionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get allowed => $_getBF(0);
  @$pb.TagNumber(1)
  set allowed($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAllowed() => $_has(0);
  @$pb.TagNumber(1)
  void clearAllowed() => $_clearField(1);
}

class ResolveVoiceRoomGrantsRequest extends $pb.GeneratedMessage {
  factory ResolveVoiceRoomGrantsRequest({
    $core.String? spaceId,
    $core.String? voiceRoomId,
    $core.String? profileId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  ResolveVoiceRoomGrantsRequest._();

  factory ResolveVoiceRoomGrantsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveVoiceRoomGrantsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveVoiceRoomGrantsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'voiceRoomId')
    ..aOS(3, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveVoiceRoomGrantsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveVoiceRoomGrantsRequest copyWith(
          void Function(ResolveVoiceRoomGrantsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveVoiceRoomGrantsRequest))
          as ResolveVoiceRoomGrantsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveVoiceRoomGrantsRequest create() =>
      ResolveVoiceRoomGrantsRequest._();
  @$core.override
  ResolveVoiceRoomGrantsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveVoiceRoomGrantsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveVoiceRoomGrantsRequest>(create);
  static ResolveVoiceRoomGrantsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get voiceRoomId => $_getSZ(1);
  @$pb.TagNumber(2)
  set voiceRoomId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasVoiceRoomId() => $_has(1);
  @$pb.TagNumber(2)
  void clearVoiceRoomId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get profileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set profileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearProfileId() => $_clearField(3);
}

class VoiceRoomGrants extends $pb.GeneratedMessage {
  factory VoiceRoomGrants({
    $core.bool? canJoin,
    $core.bool? canPublishAudio,
    $core.bool? canPublishVideo,
    $core.bool? canPublishScreenShare,
    $core.bool? canSubscribe,
    $core.bool? canMuteOthers,
    $core.bool? canDeafenOthers,
    $core.bool? canMoveOthers,
    $core.bool? canUsePtt,
    $core.bool? prioritySpeaker,
  }) {
    final result = create();
    if (canJoin != null) result.canJoin = canJoin;
    if (canPublishAudio != null) result.canPublishAudio = canPublishAudio;
    if (canPublishVideo != null) result.canPublishVideo = canPublishVideo;
    if (canPublishScreenShare != null)
      result.canPublishScreenShare = canPublishScreenShare;
    if (canSubscribe != null) result.canSubscribe = canSubscribe;
    if (canMuteOthers != null) result.canMuteOthers = canMuteOthers;
    if (canDeafenOthers != null) result.canDeafenOthers = canDeafenOthers;
    if (canMoveOthers != null) result.canMoveOthers = canMoveOthers;
    if (canUsePtt != null) result.canUsePtt = canUsePtt;
    if (prioritySpeaker != null) result.prioritySpeaker = prioritySpeaker;
    return result;
  }

  VoiceRoomGrants._();

  factory VoiceRoomGrants.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VoiceRoomGrants.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VoiceRoomGrants',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'canJoin')
    ..aOB(2, _omitFieldNames ? '' : 'canPublishAudio')
    ..aOB(3, _omitFieldNames ? '' : 'canPublishVideo')
    ..aOB(4, _omitFieldNames ? '' : 'canPublishScreenShare')
    ..aOB(5, _omitFieldNames ? '' : 'canSubscribe')
    ..aOB(6, _omitFieldNames ? '' : 'canMuteOthers')
    ..aOB(7, _omitFieldNames ? '' : 'canDeafenOthers')
    ..aOB(8, _omitFieldNames ? '' : 'canMoveOthers')
    ..aOB(9, _omitFieldNames ? '' : 'canUsePtt')
    ..aOB(10, _omitFieldNames ? '' : 'prioritySpeaker')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoomGrants clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VoiceRoomGrants copyWith(void Function(VoiceRoomGrants) updates) =>
      super.copyWith((message) => updates(message as VoiceRoomGrants))
          as VoiceRoomGrants;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VoiceRoomGrants create() => VoiceRoomGrants._();
  @$core.override
  VoiceRoomGrants createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VoiceRoomGrants getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VoiceRoomGrants>(create);
  static VoiceRoomGrants? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get canJoin => $_getBF(0);
  @$pb.TagNumber(1)
  set canJoin($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCanJoin() => $_has(0);
  @$pb.TagNumber(1)
  void clearCanJoin() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.bool get canPublishAudio => $_getBF(1);
  @$pb.TagNumber(2)
  set canPublishAudio($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCanPublishAudio() => $_has(1);
  @$pb.TagNumber(2)
  void clearCanPublishAudio() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get canPublishVideo => $_getBF(2);
  @$pb.TagNumber(3)
  set canPublishVideo($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCanPublishVideo() => $_has(2);
  @$pb.TagNumber(3)
  void clearCanPublishVideo() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get canPublishScreenShare => $_getBF(3);
  @$pb.TagNumber(4)
  set canPublishScreenShare($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCanPublishScreenShare() => $_has(3);
  @$pb.TagNumber(4)
  void clearCanPublishScreenShare() => $_clearField(4);

  /// Subscription uses the same canonical VOICE_JOIN decision as can_join.
  @$pb.TagNumber(5)
  $core.bool get canSubscribe => $_getBF(4);
  @$pb.TagNumber(5)
  set canSubscribe($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCanSubscribe() => $_has(4);
  @$pb.TagNumber(5)
  void clearCanSubscribe() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get canMuteOthers => $_getBF(5);
  @$pb.TagNumber(6)
  set canMuteOthers($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasCanMuteOthers() => $_has(5);
  @$pb.TagNumber(6)
  void clearCanMuteOthers() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.bool get canDeafenOthers => $_getBF(6);
  @$pb.TagNumber(7)
  set canDeafenOthers($core.bool value) => $_setBool(6, value);
  @$pb.TagNumber(7)
  $core.bool hasCanDeafenOthers() => $_has(6);
  @$pb.TagNumber(7)
  void clearCanDeafenOthers() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.bool get canMoveOthers => $_getBF(7);
  @$pb.TagNumber(8)
  set canMoveOthers($core.bool value) => $_setBool(7, value);
  @$pb.TagNumber(8)
  $core.bool hasCanMoveOthers() => $_has(7);
  @$pb.TagNumber(8)
  void clearCanMoveOthers() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.bool get canUsePtt => $_getBF(8);
  @$pb.TagNumber(9)
  set canUsePtt($core.bool value) => $_setBool(8, value);
  @$pb.TagNumber(9)
  $core.bool hasCanUsePtt() => $_has(8);
  @$pb.TagNumber(9)
  void clearCanUsePtt() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.bool get prioritySpeaker => $_getBF(9);
  @$pb.TagNumber(10)
  set prioritySpeaker($core.bool value) => $_setBool(9, value);
  @$pb.TagNumber(10)
  $core.bool hasPrioritySpeaker() => $_has(9);
  @$pb.TagNumber(10)
  void clearPrioritySpeaker() => $_clearField(10);
}

class ResolveVoiceRoomGrantsResponse extends $pb.GeneratedMessage {
  factory ResolveVoiceRoomGrantsResponse({
    VoiceRoomGrants? grants,
    $fixnum.Int64? policyEpoch,
  }) {
    final result = create();
    if (grants != null) result.grants = grants;
    if (policyEpoch != null) result.policyEpoch = policyEpoch;
    return result;
  }

  ResolveVoiceRoomGrantsResponse._();

  factory ResolveVoiceRoomGrantsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveVoiceRoomGrantsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveVoiceRoomGrantsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<VoiceRoomGrants>(1, _omitFieldNames ? '' : 'grants',
        subBuilder: VoiceRoomGrants.create)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'policyEpoch', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveVoiceRoomGrantsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveVoiceRoomGrantsResponse copyWith(
          void Function(ResolveVoiceRoomGrantsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveVoiceRoomGrantsResponse))
          as ResolveVoiceRoomGrantsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveVoiceRoomGrantsResponse create() =>
      ResolveVoiceRoomGrantsResponse._();
  @$core.override
  ResolveVoiceRoomGrantsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveVoiceRoomGrantsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveVoiceRoomGrantsResponse>(create);
  static ResolveVoiceRoomGrantsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  VoiceRoomGrants get grants => $_getN(0);
  @$pb.TagNumber(1)
  set grants(VoiceRoomGrants value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasGrants() => $_has(0);
  @$pb.TagNumber(1)
  void clearGrants() => $_clearField(1);
  @$pb.TagNumber(1)
  VoiceRoomGrants ensureGrants() => $_ensure(0);

  @$pb.TagNumber(2)
  $fixnum.Int64 get policyEpoch => $_getI64(1);
  @$pb.TagNumber(2)
  set policyEpoch($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPolicyEpoch() => $_has(1);
  @$pb.TagNumber(2)
  void clearPolicyEpoch() => $_clearField(2);
}

class GetEffectivePermissionsRequest extends $pb.GeneratedMessage {
  factory GetEffectivePermissionsRequest({
    $core.String? spaceId,
    $core.String? profileId,
    $2.ChatRef? chat,
    $core.String? voiceRoomId,
  }) {
    final result = create();
    if (spaceId != null) result.spaceId = spaceId;
    if (profileId != null) result.profileId = profileId;
    if (chat != null) result.chat = chat;
    if (voiceRoomId != null) result.voiceRoomId = voiceRoomId;
    return result;
  }

  GetEffectivePermissionsRequest._();

  factory GetEffectivePermissionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetEffectivePermissionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetEffectivePermissionsRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'spaceId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOM<$2.ChatRef>(3, _omitFieldNames ? '' : 'chat',
        subBuilder: $2.ChatRef.create)
    ..aOS(4, _omitFieldNames ? '' : 'voiceRoomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetEffectivePermissionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetEffectivePermissionsRequest copyWith(
          void Function(GetEffectivePermissionsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetEffectivePermissionsRequest))
          as GetEffectivePermissionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetEffectivePermissionsRequest create() =>
      GetEffectivePermissionsRequest._();
  @$core.override
  GetEffectivePermissionsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetEffectivePermissionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetEffectivePermissionsRequest>(create);
  static GetEffectivePermissionsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get spaceId => $_getSZ(0);
  @$pb.TagNumber(1)
  set spaceId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $2.ChatRef get chat => $_getN(2);
  @$pb.TagNumber(3)
  set chat($2.ChatRef value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasChat() => $_has(2);
  @$pb.TagNumber(3)
  void clearChat() => $_clearField(3);
  @$pb.TagNumber(3)
  $2.ChatRef ensureChat() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get voiceRoomId => $_getSZ(3);
  @$pb.TagNumber(4)
  set voiceRoomId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasVoiceRoomId() => $_has(3);
  @$pb.TagNumber(4)
  void clearVoiceRoomId() => $_clearField(4);
}

class PermissionSet extends $pb.GeneratedMessage {
  factory PermissionSet({
    $fixnum.Int64? effectiveMask,
    $core.Iterable<$core.String>? permissionNames,
  }) {
    final result = create();
    if (effectiveMask != null) result.effectiveMask = effectiveMask;
    if (permissionNames != null) result.permissionNames.addAll(permissionNames);
    return result;
  }

  PermissionSet._();

  factory PermissionSet.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PermissionSet.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PermissionSet',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'effectiveMask', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..pPS(2, _omitFieldNames ? '' : 'permissionNames')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PermissionSet clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PermissionSet copyWith(void Function(PermissionSet) updates) =>
      super.copyWith((message) => updates(message as PermissionSet))
          as PermissionSet;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PermissionSet create() => PermissionSet._();
  @$core.override
  PermissionSet createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PermissionSet getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PermissionSet>(create);
  static PermissionSet? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get effectiveMask => $_getI64(0);
  @$pb.TagNumber(1)
  set effectiveMask($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEffectiveMask() => $_has(0);
  @$pb.TagNumber(1)
  void clearEffectiveMask() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get permissionNames => $_getList(1);
}

class CreateRoleResponse extends $pb.GeneratedMessage {
  factory CreateRoleResponse({
    Role? role,
  }) {
    final result = create();
    if (role != null) result.role = role;
    return result;
  }

  CreateRoleResponse._();

  factory CreateRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<Role>(1, _omitFieldNames ? '' : 'role', subBuilder: Role.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateRoleResponse copyWith(void Function(CreateRoleResponse) updates) =>
      super.copyWith((message) => updates(message as CreateRoleResponse))
          as CreateRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateRoleResponse create() => CreateRoleResponse._();
  @$core.override
  CreateRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateRoleResponse>(create);
  static CreateRoleResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Role get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(Role value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);
  @$pb.TagNumber(1)
  Role ensureRole() => $_ensure(0);
}

class UpdateRoleResponse extends $pb.GeneratedMessage {
  factory UpdateRoleResponse({
    Role? role,
  }) {
    final result = create();
    if (role != null) result.role = role;
    return result;
  }

  UpdateRoleResponse._();

  factory UpdateRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory UpdateRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'UpdateRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<Role>(1, _omitFieldNames ? '' : 'role', subBuilder: Role.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  UpdateRoleResponse copyWith(void Function(UpdateRoleResponse) updates) =>
      super.copyWith((message) => updates(message as UpdateRoleResponse))
          as UpdateRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static UpdateRoleResponse create() => UpdateRoleResponse._();
  @$core.override
  UpdateRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static UpdateRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<UpdateRoleResponse>(create);
  static UpdateRoleResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Role get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(Role value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);
  @$pb.TagNumber(1)
  Role ensureRole() => $_ensure(0);
}

class DeleteRoleResponse extends $pb.GeneratedMessage {
  factory DeleteRoleResponse() => create();

  DeleteRoleResponse._();

  factory DeleteRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteRoleResponse copyWith(void Function(DeleteRoleResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteRoleResponse))
          as DeleteRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteRoleResponse create() => DeleteRoleResponse._();
  @$core.override
  DeleteRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteRoleResponse>(create);
  static DeleteRoleResponse? _defaultInstance;
}

class ListRolesResponse extends $pb.GeneratedMessage {
  factory ListRolesResponse({
    RoleList? roleList,
  }) {
    final result = create();
    if (roleList != null) result.roleList = roleList;
    return result;
  }

  ListRolesResponse._();

  factory ListRolesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListRolesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListRolesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<RoleList>(1, _omitFieldNames ? '' : 'roleList',
        subBuilder: RoleList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRolesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListRolesResponse copyWith(void Function(ListRolesResponse) updates) =>
      super.copyWith((message) => updates(message as ListRolesResponse))
          as ListRolesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListRolesResponse create() => ListRolesResponse._();
  @$core.override
  ListRolesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListRolesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListRolesResponse>(create);
  static ListRolesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  RoleList get roleList => $_getN(0);
  @$pb.TagNumber(1)
  set roleList(RoleList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRoleList() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoleList() => $_clearField(1);
  @$pb.TagNumber(1)
  RoleList ensureRoleList() => $_ensure(0);
}

class ReorderRolesResponse extends $pb.GeneratedMessage {
  factory ReorderRolesResponse() => create();

  ReorderRolesResponse._();

  factory ReorderRolesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReorderRolesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReorderRolesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderRolesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReorderRolesResponse copyWith(void Function(ReorderRolesResponse) updates) =>
      super.copyWith((message) => updates(message as ReorderRolesResponse))
          as ReorderRolesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReorderRolesResponse create() => ReorderRolesResponse._();
  @$core.override
  ReorderRolesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReorderRolesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReorderRolesResponse>(create);
  static ReorderRolesResponse? _defaultInstance;
}

class AssignRoleResponse extends $pb.GeneratedMessage {
  factory AssignRoleResponse() => create();

  AssignRoleResponse._();

  factory AssignRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AssignRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AssignRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssignRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AssignRoleResponse copyWith(void Function(AssignRoleResponse) updates) =>
      super.copyWith((message) => updates(message as AssignRoleResponse))
          as AssignRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AssignRoleResponse create() => AssignRoleResponse._();
  @$core.override
  AssignRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AssignRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AssignRoleResponse>(create);
  static AssignRoleResponse? _defaultInstance;
}

class RevokeRoleResponse extends $pb.GeneratedMessage {
  factory RevokeRoleResponse() => create();

  RevokeRoleResponse._();

  factory RevokeRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeRoleResponse copyWith(void Function(RevokeRoleResponse) updates) =>
      super.copyWith((message) => updates(message as RevokeRoleResponse))
          as RevokeRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeRoleResponse create() => RevokeRoleResponse._();
  @$core.override
  RevokeRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeRoleResponse>(create);
  static RevokeRoleResponse? _defaultInstance;
}

class GetMemberRolesResponse extends $pb.GeneratedMessage {
  factory GetMemberRolesResponse({
    RoleList? roleList,
  }) {
    final result = create();
    if (roleList != null) result.roleList = roleList;
    return result;
  }

  GetMemberRolesResponse._();

  factory GetMemberRolesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMemberRolesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMemberRolesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<RoleList>(1, _omitFieldNames ? '' : 'roleList',
        subBuilder: RoleList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberRolesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMemberRolesResponse copyWith(
          void Function(GetMemberRolesResponse) updates) =>
      super.copyWith((message) => updates(message as GetMemberRolesResponse))
          as GetMemberRolesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMemberRolesResponse create() => GetMemberRolesResponse._();
  @$core.override
  GetMemberRolesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMemberRolesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMemberRolesResponse>(create);
  static GetMemberRolesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  RoleList get roleList => $_getN(0);
  @$pb.TagNumber(1)
  set roleList(RoleList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRoleList() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoleList() => $_clearField(1);
  @$pb.TagNumber(1)
  RoleList ensureRoleList() => $_ensure(0);
}

class SetChatOverrideResponse extends $pb.GeneratedMessage {
  factory SetChatOverrideResponse() => create();

  SetChatOverrideResponse._();

  factory SetChatOverrideResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetChatOverrideResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetChatOverrideResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatOverrideResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetChatOverrideResponse copyWith(
          void Function(SetChatOverrideResponse) updates) =>
      super.copyWith((message) => updates(message as SetChatOverrideResponse))
          as SetChatOverrideResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetChatOverrideResponse create() => SetChatOverrideResponse._();
  @$core.override
  SetChatOverrideResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetChatOverrideResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetChatOverrideResponse>(create);
  static SetChatOverrideResponse? _defaultInstance;
}

class RemoveChatOverrideResponse extends $pb.GeneratedMessage {
  factory RemoveChatOverrideResponse() => create();

  RemoveChatOverrideResponse._();

  factory RemoveChatOverrideResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveChatOverrideResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveChatOverrideResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveChatOverrideResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveChatOverrideResponse copyWith(
          void Function(RemoveChatOverrideResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveChatOverrideResponse))
          as RemoveChatOverrideResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveChatOverrideResponse create() => RemoveChatOverrideResponse._();
  @$core.override
  RemoveChatOverrideResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveChatOverrideResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveChatOverrideResponse>(create);
  static RemoveChatOverrideResponse? _defaultInstance;
}

class GetChatOverridesResponse extends $pb.GeneratedMessage {
  factory GetChatOverridesResponse({
    OverrideList? overrideList,
  }) {
    final result = create();
    if (overrideList != null) result.overrideList = overrideList;
    return result;
  }

  GetChatOverridesResponse._();

  factory GetChatOverridesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetChatOverridesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetChatOverridesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OverrideList>(1, _omitFieldNames ? '' : 'overrideList',
        subBuilder: OverrideList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatOverridesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetChatOverridesResponse copyWith(
          void Function(GetChatOverridesResponse) updates) =>
      super.copyWith((message) => updates(message as GetChatOverridesResponse))
          as GetChatOverridesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetChatOverridesResponse create() => GetChatOverridesResponse._();
  @$core.override
  GetChatOverridesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetChatOverridesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetChatOverridesResponse>(create);
  static GetChatOverridesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  OverrideList get overrideList => $_getN(0);
  @$pb.TagNumber(1)
  set overrideList(OverrideList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOverrideList() => $_has(0);
  @$pb.TagNumber(1)
  void clearOverrideList() => $_clearField(1);
  @$pb.TagNumber(1)
  OverrideList ensureOverrideList() => $_ensure(0);
}

class SetVoiceRoomOverrideResponse extends $pb.GeneratedMessage {
  factory SetVoiceRoomOverrideResponse() => create();

  SetVoiceRoomOverrideResponse._();

  factory SetVoiceRoomOverrideResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetVoiceRoomOverrideResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetVoiceRoomOverrideResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVoiceRoomOverrideResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetVoiceRoomOverrideResponse copyWith(
          void Function(SetVoiceRoomOverrideResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SetVoiceRoomOverrideResponse))
          as SetVoiceRoomOverrideResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetVoiceRoomOverrideResponse create() =>
      SetVoiceRoomOverrideResponse._();
  @$core.override
  SetVoiceRoomOverrideResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetVoiceRoomOverrideResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetVoiceRoomOverrideResponse>(create);
  static SetVoiceRoomOverrideResponse? _defaultInstance;
}

class RemoveVoiceRoomOverrideResponse extends $pb.GeneratedMessage {
  factory RemoveVoiceRoomOverrideResponse() => create();

  RemoveVoiceRoomOverrideResponse._();

  factory RemoveVoiceRoomOverrideResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RemoveVoiceRoomOverrideResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RemoveVoiceRoomOverrideResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveVoiceRoomOverrideResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RemoveVoiceRoomOverrideResponse copyWith(
          void Function(RemoveVoiceRoomOverrideResponse) updates) =>
      super.copyWith(
              (message) => updates(message as RemoveVoiceRoomOverrideResponse))
          as RemoveVoiceRoomOverrideResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RemoveVoiceRoomOverrideResponse create() =>
      RemoveVoiceRoomOverrideResponse._();
  @$core.override
  RemoveVoiceRoomOverrideResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RemoveVoiceRoomOverrideResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RemoveVoiceRoomOverrideResponse>(
          create);
  static RemoveVoiceRoomOverrideResponse? _defaultInstance;
}

class GetVoiceRoomOverridesResponse extends $pb.GeneratedMessage {
  factory GetVoiceRoomOverridesResponse({
    OverrideList? overrideList,
  }) {
    final result = create();
    if (overrideList != null) result.overrideList = overrideList;
    return result;
  }

  GetVoiceRoomOverridesResponse._();

  factory GetVoiceRoomOverridesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetVoiceRoomOverridesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetVoiceRoomOverridesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<OverrideList>(1, _omitFieldNames ? '' : 'overrideList',
        subBuilder: OverrideList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceRoomOverridesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetVoiceRoomOverridesResponse copyWith(
          void Function(GetVoiceRoomOverridesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetVoiceRoomOverridesResponse))
          as GetVoiceRoomOverridesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetVoiceRoomOverridesResponse create() =>
      GetVoiceRoomOverridesResponse._();
  @$core.override
  GetVoiceRoomOverridesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetVoiceRoomOverridesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetVoiceRoomOverridesResponse>(create);
  static GetVoiceRoomOverridesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  OverrideList get overrideList => $_getN(0);
  @$pb.TagNumber(1)
  set overrideList(OverrideList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOverrideList() => $_has(0);
  @$pb.TagNumber(1)
  void clearOverrideList() => $_clearField(1);
  @$pb.TagNumber(1)
  OverrideList ensureOverrideList() => $_ensure(0);
}

class GetEffectivePermissionsResponse extends $pb.GeneratedMessage {
  factory GetEffectivePermissionsResponse({
    PermissionSet? permissionSet,
  }) {
    final result = create();
    if (permissionSet != null) result.permissionSet = permissionSet;
    return result;
  }

  GetEffectivePermissionsResponse._();

  factory GetEffectivePermissionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetEffectivePermissionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetEffectivePermissionsResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<PermissionSet>(1, _omitFieldNames ? '' : 'permissionSet',
        subBuilder: PermissionSet.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetEffectivePermissionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetEffectivePermissionsResponse copyWith(
          void Function(GetEffectivePermissionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetEffectivePermissionsResponse))
          as GetEffectivePermissionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetEffectivePermissionsResponse create() =>
      GetEffectivePermissionsResponse._();
  @$core.override
  GetEffectivePermissionsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetEffectivePermissionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetEffectivePermissionsResponse>(
          create);
  static GetEffectivePermissionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  PermissionSet get permissionSet => $_getN(0);
  @$pb.TagNumber(1)
  set permissionSet(PermissionSet value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasPermissionSet() => $_has(0);
  @$pb.TagNumber(1)
  void clearPermissionSet() => $_clearField(1);
  @$pb.TagNumber(1)
  PermissionSet ensurePermissionSet() => $_ensure(0);
}

class SetDefaultJoinRoleResponse extends $pb.GeneratedMessage {
  factory SetDefaultJoinRoleResponse() => create();

  SetDefaultJoinRoleResponse._();

  factory SetDefaultJoinRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetDefaultJoinRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetDefaultJoinRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultJoinRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetDefaultJoinRoleResponse copyWith(
          void Function(SetDefaultJoinRoleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SetDefaultJoinRoleResponse))
          as SetDefaultJoinRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetDefaultJoinRoleResponse create() => SetDefaultJoinRoleResponse._();
  @$core.override
  SetDefaultJoinRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetDefaultJoinRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetDefaultJoinRoleResponse>(create);
  static SetDefaultJoinRoleResponse? _defaultInstance;
}

class GetDefaultJoinRoleResponse extends $pb.GeneratedMessage {
  factory GetDefaultJoinRoleResponse({
    Role? role,
  }) {
    final result = create();
    if (role != null) result.role = role;
    return result;
  }

  GetDefaultJoinRoleResponse._();

  factory GetDefaultJoinRoleResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDefaultJoinRoleResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDefaultJoinRoleResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..aOM<Role>(1, _omitFieldNames ? '' : 'role', subBuilder: Role.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDefaultJoinRoleResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDefaultJoinRoleResponse copyWith(
          void Function(GetDefaultJoinRoleResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetDefaultJoinRoleResponse))
          as GetDefaultJoinRoleResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDefaultJoinRoleResponse create() => GetDefaultJoinRoleResponse._();
  @$core.override
  GetDefaultJoinRoleResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDefaultJoinRoleResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDefaultJoinRoleResponse>(create);
  static GetDefaultJoinRoleResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Role get role => $_getN(0);
  @$pb.TagNumber(1)
  set role(Role value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasRole() => $_has(0);
  @$pb.TagNumber(1)
  void clearRole() => $_clearField(1);
  @$pb.TagNumber(1)
  Role ensureRole() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
