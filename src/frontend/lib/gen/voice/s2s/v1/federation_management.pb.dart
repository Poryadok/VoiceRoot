// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/federation_management.proto.

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

import 'federation_management.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'federation_management.pbenum.dart';

class FederationNode extends $pb.GeneratedMessage {
  factory FederationNode({
    $core.String? id,
    $core.String? name,
    $core.String? host,
    $core.int? port,
    $core.String? description,
    $core.String? status,
    $core.String? tlsCertFingerprint,
    $1.Timestamp? lastHeartbeatAt,
    $1.Timestamp? lastSyncAt,
    $1.Timestamp? registeredAt,
    $1.Timestamp? approvedAt,
    $core.String? approvedByProfileId,
    $1.Timestamp? defederatedAt,
    FederationNodeRegistrationStatus? statusEnum,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (name != null) result.name = name;
    if (host != null) result.host = host;
    if (port != null) result.port = port;
    if (description != null) result.description = description;
    if (status != null) result.status = status;
    if (tlsCertFingerprint != null)
      result.tlsCertFingerprint = tlsCertFingerprint;
    if (lastHeartbeatAt != null) result.lastHeartbeatAt = lastHeartbeatAt;
    if (lastSyncAt != null) result.lastSyncAt = lastSyncAt;
    if (registeredAt != null) result.registeredAt = registeredAt;
    if (approvedAt != null) result.approvedAt = approvedAt;
    if (approvedByProfileId != null)
      result.approvedByProfileId = approvedByProfileId;
    if (defederatedAt != null) result.defederatedAt = defederatedAt;
    if (statusEnum != null) result.statusEnum = statusEnum;
    return result;
  }

  FederationNode._();

  factory FederationNode.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FederationNode.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FederationNode',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'name')
    ..aOS(3, _omitFieldNames ? '' : 'host')
    ..aI(4, _omitFieldNames ? '' : 'port')
    ..aOS(5, _omitFieldNames ? '' : 'description')
    ..aOS(6, _omitFieldNames ? '' : 'status')
    ..aOS(7, _omitFieldNames ? '' : 'tlsCertFingerprint')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'lastHeartbeatAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'lastSyncAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'registeredAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(11, _omitFieldNames ? '' : 'approvedAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(12, _omitFieldNames ? '' : 'approvedByProfileId')
    ..aOM<$1.Timestamp>(13, _omitFieldNames ? '' : 'defederatedAt',
        subBuilder: $1.Timestamp.create)
    ..aE<FederationNodeRegistrationStatus>(
        14, _omitFieldNames ? '' : 'statusEnum',
        enumValues: FederationNodeRegistrationStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNode clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNode copyWith(void Function(FederationNode) updates) =>
      super.copyWith((message) => updates(message as FederationNode))
          as FederationNode;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FederationNode create() => FederationNode._();
  @$core.override
  FederationNode createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FederationNode getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FederationNode>(create);
  static FederationNode? _defaultInstance;

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
  $core.String get host => $_getSZ(2);
  @$pb.TagNumber(3)
  set host($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasHost() => $_has(2);
  @$pb.TagNumber(3)
  void clearHost() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.int get port => $_getIZ(3);
  @$pb.TagNumber(4)
  set port($core.int value) => $_setSignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPort() => $_has(3);
  @$pb.TagNumber(4)
  void clearPort() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get description => $_getSZ(4);
  @$pb.TagNumber(5)
  set description($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDescription() => $_has(4);
  @$pb.TagNumber(5)
  void clearDescription() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get status => $_getSZ(5);
  @$pb.TagNumber(6)
  set status($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get tlsCertFingerprint => $_getSZ(6);
  @$pb.TagNumber(7)
  set tlsCertFingerprint($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasTlsCertFingerprint() => $_has(6);
  @$pb.TagNumber(7)
  void clearTlsCertFingerprint() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get lastHeartbeatAt => $_getN(7);
  @$pb.TagNumber(8)
  set lastHeartbeatAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasLastHeartbeatAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearLastHeartbeatAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureLastHeartbeatAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get lastSyncAt => $_getN(8);
  @$pb.TagNumber(9)
  set lastSyncAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasLastSyncAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearLastSyncAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureLastSyncAt() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get registeredAt => $_getN(9);
  @$pb.TagNumber(10)
  set registeredAt($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasRegisteredAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearRegisteredAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureRegisteredAt() => $_ensure(9);

  @$pb.TagNumber(11)
  $1.Timestamp get approvedAt => $_getN(10);
  @$pb.TagNumber(11)
  set approvedAt($1.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasApprovedAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearApprovedAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Timestamp ensureApprovedAt() => $_ensure(10);

  @$pb.TagNumber(12)
  $core.String get approvedByProfileId => $_getSZ(11);
  @$pb.TagNumber(12)
  set approvedByProfileId($core.String value) => $_setString(11, value);
  @$pb.TagNumber(12)
  $core.bool hasApprovedByProfileId() => $_has(11);
  @$pb.TagNumber(12)
  void clearApprovedByProfileId() => $_clearField(12);

  @$pb.TagNumber(13)
  $1.Timestamp get defederatedAt => $_getN(12);
  @$pb.TagNumber(13)
  set defederatedAt($1.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasDefederatedAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearDefederatedAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $1.Timestamp ensureDefederatedAt() => $_ensure(12);

  @$pb.TagNumber(14)
  FederationNodeRegistrationStatus get statusEnum => $_getN(13);
  @$pb.TagNumber(14)
  set statusEnum(FederationNodeRegistrationStatus value) =>
      $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasStatusEnum() => $_has(13);
  @$pb.TagNumber(14)
  void clearStatusEnum() => $_clearField(14);
}

class RegisterNodeRequest extends $pb.GeneratedMessage {
  factory RegisterNodeRequest({
    $core.String? name,
    $core.String? host,
    $core.int? port,
    $core.String? description,
    $core.String? tlsCertFingerprint,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (host != null) result.host = host;
    if (port != null) result.port = port;
    if (description != null) result.description = description;
    if (tlsCertFingerprint != null)
      result.tlsCertFingerprint = tlsCertFingerprint;
    return result;
  }

  RegisterNodeRequest._();

  factory RegisterNodeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterNodeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterNodeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aOS(2, _omitFieldNames ? '' : 'host')
    ..aI(3, _omitFieldNames ? '' : 'port')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'tlsCertFingerprint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterNodeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterNodeRequest copyWith(void Function(RegisterNodeRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterNodeRequest))
          as RegisterNodeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterNodeRequest create() => RegisterNodeRequest._();
  @$core.override
  RegisterNodeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterNodeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterNodeRequest>(create);
  static RegisterNodeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get host => $_getSZ(1);
  @$pb.TagNumber(2)
  set host($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasHost() => $_has(1);
  @$pb.TagNumber(2)
  void clearHost() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.int get port => $_getIZ(2);
  @$pb.TagNumber(3)
  set port($core.int value) => $_setSignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPort() => $_has(2);
  @$pb.TagNumber(3)
  void clearPort() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get tlsCertFingerprint => $_getSZ(4);
  @$pb.TagNumber(5)
  set tlsCertFingerprint($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasTlsCertFingerprint() => $_has(4);
  @$pb.TagNumber(5)
  void clearTlsCertFingerprint() => $_clearField(5);
}

class ApproveNodeRequest extends $pb.GeneratedMessage {
  factory ApproveNodeRequest({
    $core.String? nodeId,
    $core.String? approverProfileId,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (approverProfileId != null) result.approverProfileId = approverProfileId;
    return result;
  }

  ApproveNodeRequest._();

  factory ApproveNodeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApproveNodeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApproveNodeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'approverProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveNodeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveNodeRequest copyWith(void Function(ApproveNodeRequest) updates) =>
      super.copyWith((message) => updates(message as ApproveNodeRequest))
          as ApproveNodeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApproveNodeRequest create() => ApproveNodeRequest._();
  @$core.override
  ApproveNodeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApproveNodeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApproveNodeRequest>(create);
  static ApproveNodeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get approverProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set approverProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasApproverProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearApproverProfileId() => $_clearField(2);
}

class DeactivateNodeRequest extends $pb.GeneratedMessage {
  factory DeactivateNodeRequest({
    $core.String? nodeId,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    return result;
  }

  DeactivateNodeRequest._();

  factory DeactivateNodeRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeactivateNodeRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeactivateNodeRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeactivateNodeRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeactivateNodeRequest copyWith(
          void Function(DeactivateNodeRequest) updates) =>
      super.copyWith((message) => updates(message as DeactivateNodeRequest))
          as DeactivateNodeRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeactivateNodeRequest create() => DeactivateNodeRequest._();
  @$core.override
  DeactivateNodeRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeactivateNodeRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeactivateNodeRequest>(create);
  static DeactivateNodeRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);
}

class ListNodesRequest extends $pb.GeneratedMessage {
  factory ListNodesRequest({
    $core.String? statusFilter,
    $core.int? pageSize,
    $core.String? pageToken,
    FederationNodeRegistrationStatus? statusFilterEnum,
  }) {
    final result = create();
    if (statusFilter != null) result.statusFilter = statusFilter;
    if (pageSize != null) result.pageSize = pageSize;
    if (pageToken != null) result.pageToken = pageToken;
    if (statusFilterEnum != null) result.statusFilterEnum = statusFilterEnum;
    return result;
  }

  ListNodesRequest._();

  factory ListNodesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListNodesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListNodesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'statusFilter')
    ..aI(2, _omitFieldNames ? '' : 'pageSize')
    ..aOS(3, _omitFieldNames ? '' : 'pageToken')
    ..aE<FederationNodeRegistrationStatus>(
        4, _omitFieldNames ? '' : 'statusFilterEnum',
        enumValues: FederationNodeRegistrationStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListNodesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListNodesRequest copyWith(void Function(ListNodesRequest) updates) =>
      super.copyWith((message) => updates(message as ListNodesRequest))
          as ListNodesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListNodesRequest create() => ListNodesRequest._();
  @$core.override
  ListNodesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListNodesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListNodesRequest>(create);
  static ListNodesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get statusFilter => $_getSZ(0);
  @$pb.TagNumber(1)
  set statusFilter($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStatusFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatusFilter() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.int get pageSize => $_getIZ(1);
  @$pb.TagNumber(2)
  set pageSize($core.int value) => $_setSignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPageSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearPageSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get pageToken => $_getSZ(2);
  @$pb.TagNumber(3)
  set pageToken($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPageToken() => $_has(2);
  @$pb.TagNumber(3)
  void clearPageToken() => $_clearField(3);

  @$pb.TagNumber(4)
  FederationNodeRegistrationStatus get statusFilterEnum => $_getN(3);
  @$pb.TagNumber(4)
  set statusFilterEnum(FederationNodeRegistrationStatus value) =>
      $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasStatusFilterEnum() => $_has(3);
  @$pb.TagNumber(4)
  void clearStatusFilterEnum() => $_clearField(4);
}

class FederationNodeList extends $pb.GeneratedMessage {
  factory FederationNodeList({
    $core.Iterable<FederationNode>? nodes,
    $core.String? nextPageToken,
  }) {
    final result = create();
    if (nodes != null) result.nodes.addAll(nodes);
    if (nextPageToken != null) result.nextPageToken = nextPageToken;
    return result;
  }

  FederationNodeList._();

  factory FederationNodeList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FederationNodeList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FederationNodeList',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..pPM<FederationNode>(1, _omitFieldNames ? '' : 'nodes',
        subBuilder: FederationNode.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextPageToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNodeList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNodeList copyWith(void Function(FederationNodeList) updates) =>
      super.copyWith((message) => updates(message as FederationNodeList))
          as FederationNodeList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FederationNodeList create() => FederationNodeList._();
  @$core.override
  FederationNodeList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FederationNodeList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FederationNodeList>(create);
  static FederationNodeList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<FederationNode> get nodes => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextPageToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextPageToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextPageToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextPageToken() => $_clearField(2);
}

class GetNodeStatusRequest extends $pb.GeneratedMessage {
  factory GetNodeStatusRequest({
    $core.String? nodeId,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    return result;
  }

  GetNodeStatusRequest._();

  factory GetNodeStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetNodeStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNodeStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNodeStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNodeStatusRequest copyWith(void Function(GetNodeStatusRequest) updates) =>
      super.copyWith((message) => updates(message as GetNodeStatusRequest))
          as GetNodeStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetNodeStatusRequest create() => GetNodeStatusRequest._();
  @$core.override
  GetNodeStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetNodeStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNodeStatusRequest>(create);
  static GetNodeStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);
}

class FederationNodeStatus extends $pb.GeneratedMessage {
  factory FederationNodeStatus({
    $core.String? nodeId,
    $core.String? status,
    $1.Timestamp? lastHeartbeatAt,
    $1.Timestamp? lastSyncAt,
    FederationNodeRegistrationStatus? statusEnum,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (status != null) result.status = status;
    if (lastHeartbeatAt != null) result.lastHeartbeatAt = lastHeartbeatAt;
    if (lastSyncAt != null) result.lastSyncAt = lastSyncAt;
    if (statusEnum != null) result.statusEnum = statusEnum;
    return result;
  }

  FederationNodeStatus._();

  factory FederationNodeStatus.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FederationNodeStatus.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FederationNodeStatus',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'lastHeartbeatAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'lastSyncAt',
        subBuilder: $1.Timestamp.create)
    ..aE<FederationNodeRegistrationStatus>(
        5, _omitFieldNames ? '' : 'statusEnum',
        enumValues: FederationNodeRegistrationStatus.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNodeStatus clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FederationNodeStatus copyWith(void Function(FederationNodeStatus) updates) =>
      super.copyWith((message) => updates(message as FederationNodeStatus))
          as FederationNodeStatus;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FederationNodeStatus create() => FederationNodeStatus._();
  @$core.override
  FederationNodeStatus createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FederationNodeStatus getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FederationNodeStatus>(create);
  static FederationNodeStatus? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get lastHeartbeatAt => $_getN(2);
  @$pb.TagNumber(3)
  set lastHeartbeatAt($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasLastHeartbeatAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearLastHeartbeatAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureLastHeartbeatAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $1.Timestamp get lastSyncAt => $_getN(3);
  @$pb.TagNumber(4)
  set lastSyncAt($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasLastSyncAt() => $_has(3);
  @$pb.TagNumber(4)
  void clearLastSyncAt() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureLastSyncAt() => $_ensure(3);

  @$pb.TagNumber(5)
  FederationNodeRegistrationStatus get statusEnum => $_getN(4);
  @$pb.TagNumber(5)
  set statusEnum(FederationNodeRegistrationStatus value) =>
      $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasStatusEnum() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatusEnum() => $_clearField(5);
}

class DefederateRequest extends $pb.GeneratedMessage {
  factory DefederateRequest({
    $core.String? nodeId,
    $core.String? reason,
  }) {
    final result = create();
    if (nodeId != null) result.nodeId = nodeId;
    if (reason != null) result.reason = reason;
    return result;
  }

  DefederateRequest._();

  factory DefederateRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DefederateRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DefederateRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'nodeId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederateRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederateRequest copyWith(void Function(DefederateRequest) updates) =>
      super.copyWith((message) => updates(message as DefederateRequest))
          as DefederateRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DefederateRequest create() => DefederateRequest._();
  @$core.override
  DefederateRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DefederateRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DefederateRequest>(create);
  static DefederateRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get nodeId => $_getSZ(0);
  @$pb.TagNumber(1)
  set nodeId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasNodeId() => $_has(0);
  @$pb.TagNumber(1)
  void clearNodeId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class RegisterNodeResponse extends $pb.GeneratedMessage {
  factory RegisterNodeResponse({
    FederationNode? federationNode,
  }) {
    final result = create();
    if (federationNode != null) result.federationNode = federationNode;
    return result;
  }

  RegisterNodeResponse._();

  factory RegisterNodeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterNodeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterNodeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOM<FederationNode>(1, _omitFieldNames ? '' : 'federationNode',
        subBuilder: FederationNode.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterNodeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterNodeResponse copyWith(void Function(RegisterNodeResponse) updates) =>
      super.copyWith((message) => updates(message as RegisterNodeResponse))
          as RegisterNodeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterNodeResponse create() => RegisterNodeResponse._();
  @$core.override
  RegisterNodeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterNodeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterNodeResponse>(create);
  static RegisterNodeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FederationNode get federationNode => $_getN(0);
  @$pb.TagNumber(1)
  set federationNode(FederationNode value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFederationNode() => $_has(0);
  @$pb.TagNumber(1)
  void clearFederationNode() => $_clearField(1);
  @$pb.TagNumber(1)
  FederationNode ensureFederationNode() => $_ensure(0);
}

class ApproveNodeResponse extends $pb.GeneratedMessage {
  factory ApproveNodeResponse({
    FederationNode? federationNode,
  }) {
    final result = create();
    if (federationNode != null) result.federationNode = federationNode;
    return result;
  }

  ApproveNodeResponse._();

  factory ApproveNodeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApproveNodeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApproveNodeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOM<FederationNode>(1, _omitFieldNames ? '' : 'federationNode',
        subBuilder: FederationNode.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveNodeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApproveNodeResponse copyWith(void Function(ApproveNodeResponse) updates) =>
      super.copyWith((message) => updates(message as ApproveNodeResponse))
          as ApproveNodeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApproveNodeResponse create() => ApproveNodeResponse._();
  @$core.override
  ApproveNodeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApproveNodeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApproveNodeResponse>(create);
  static ApproveNodeResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FederationNode get federationNode => $_getN(0);
  @$pb.TagNumber(1)
  set federationNode(FederationNode value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFederationNode() => $_has(0);
  @$pb.TagNumber(1)
  void clearFederationNode() => $_clearField(1);
  @$pb.TagNumber(1)
  FederationNode ensureFederationNode() => $_ensure(0);
}

class DeactivateNodeResponse extends $pb.GeneratedMessage {
  factory DeactivateNodeResponse() => create();

  DeactivateNodeResponse._();

  factory DeactivateNodeResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeactivateNodeResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeactivateNodeResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeactivateNodeResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeactivateNodeResponse copyWith(
          void Function(DeactivateNodeResponse) updates) =>
      super.copyWith((message) => updates(message as DeactivateNodeResponse))
          as DeactivateNodeResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeactivateNodeResponse create() => DeactivateNodeResponse._();
  @$core.override
  DeactivateNodeResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeactivateNodeResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeactivateNodeResponse>(create);
  static DeactivateNodeResponse? _defaultInstance;
}

class ListNodesResponse extends $pb.GeneratedMessage {
  factory ListNodesResponse({
    FederationNodeList? federationNodeList,
  }) {
    final result = create();
    if (federationNodeList != null)
      result.federationNodeList = federationNodeList;
    return result;
  }

  ListNodesResponse._();

  factory ListNodesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListNodesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListNodesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOM<FederationNodeList>(1, _omitFieldNames ? '' : 'federationNodeList',
        subBuilder: FederationNodeList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListNodesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListNodesResponse copyWith(void Function(ListNodesResponse) updates) =>
      super.copyWith((message) => updates(message as ListNodesResponse))
          as ListNodesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListNodesResponse create() => ListNodesResponse._();
  @$core.override
  ListNodesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListNodesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListNodesResponse>(create);
  static ListNodesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FederationNodeList get federationNodeList => $_getN(0);
  @$pb.TagNumber(1)
  set federationNodeList(FederationNodeList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFederationNodeList() => $_has(0);
  @$pb.TagNumber(1)
  void clearFederationNodeList() => $_clearField(1);
  @$pb.TagNumber(1)
  FederationNodeList ensureFederationNodeList() => $_ensure(0);
}

class GetNodeStatusResponse extends $pb.GeneratedMessage {
  factory GetNodeStatusResponse({
    FederationNodeStatus? federationNodeStatus,
  }) {
    final result = create();
    if (federationNodeStatus != null)
      result.federationNodeStatus = federationNodeStatus;
    return result;
  }

  GetNodeStatusResponse._();

  factory GetNodeStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetNodeStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetNodeStatusResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..aOM<FederationNodeStatus>(
        1, _omitFieldNames ? '' : 'federationNodeStatus',
        subBuilder: FederationNodeStatus.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNodeStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetNodeStatusResponse copyWith(
          void Function(GetNodeStatusResponse) updates) =>
      super.copyWith((message) => updates(message as GetNodeStatusResponse))
          as GetNodeStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetNodeStatusResponse create() => GetNodeStatusResponse._();
  @$core.override
  GetNodeStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetNodeStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetNodeStatusResponse>(create);
  static GetNodeStatusResponse? _defaultInstance;

  @$pb.TagNumber(1)
  FederationNodeStatus get federationNodeStatus => $_getN(0);
  @$pb.TagNumber(1)
  set federationNodeStatus(FederationNodeStatus value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFederationNodeStatus() => $_has(0);
  @$pb.TagNumber(1)
  void clearFederationNodeStatus() => $_clearField(1);
  @$pb.TagNumber(1)
  FederationNodeStatus ensureFederationNodeStatus() => $_ensure(0);
}

class DefederateResponse extends $pb.GeneratedMessage {
  factory DefederateResponse() => create();

  DefederateResponse._();

  factory DefederateResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DefederateResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DefederateResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.s2s.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederateResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DefederateResponse copyWith(void Function(DefederateResponse) updates) =>
      super.copyWith((message) => updates(message as DefederateResponse))
          as DefederateResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DefederateResponse create() => DefederateResponse._();
  @$core.override
  DefederateResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DefederateResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DefederateResponse>(create);
  static DefederateResponse? _defaultInstance;
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
