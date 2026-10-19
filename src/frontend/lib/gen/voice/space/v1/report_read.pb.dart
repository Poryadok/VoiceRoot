// This is a generated file - do not edit.
//
// Generated from voice/space/v1/report_read.proto.

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
    as $2;

import '../../chat/v1/chat.pb.dart' as $0;
import '../../role/v1/member_admission.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Subject data is accepted only from the exact verified Messaging workload.
/// @voice.unknown_fields=reject
class ResolveSpaceReportReadContextRequest extends $pb.GeneratedMessage {
  factory ResolveSpaceReportReadContextRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? spaceId,
    $0.ChatRef? chat,
    $core.String? viewerProfileId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (chat != null) result.chat = chat;
    if (viewerProfileId != null) result.viewerProfileId = viewerProfileId;
    return result;
  }

  ResolveSpaceReportReadContextRequest._();

  factory ResolveSpaceReportReadContextRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveSpaceReportReadContextRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveSpaceReportReadContextRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$0.ChatRef>(4, _omitFieldNames ? '' : 'chat',
        subBuilder: $0.ChatRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'viewerProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveSpaceReportReadContextRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveSpaceReportReadContextRequest copyWith(
          void Function(ResolveSpaceReportReadContextRequest) updates) =>
      super.copyWith((message) =>
              updates(message as ResolveSpaceReportReadContextRequest))
          as ResolveSpaceReportReadContextRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveSpaceReportReadContextRequest create() =>
      ResolveSpaceReportReadContextRequest._();
  @$core.override
  ResolveSpaceReportReadContextRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveSpaceReportReadContextRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ResolveSpaceReportReadContextRequest>(create);
  static ResolveSpaceReportReadContextRequest? _defaultInstance;

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
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $0.ChatRef get chat => $_getN(3);
  @$pb.TagNumber(4)
  set chat($0.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.ChatRef ensureChat() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get viewerProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set viewerProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasViewerProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearViewerProfileId() => $_clearField(5);
}

/// Positioning facts, never a Role/content grant or positive retirement proof.
/// @voice.unknown_fields=reject
class ResolveSpaceReportReadContextResponse extends $pb.GeneratedMessage {
  factory ResolveSpaceReportReadContextResponse({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? spaceId,
    $0.ChatRef? chat,
    $core.String? viewerProfileId,
    $core.bool? eligible,
    $fixnum.Int64? spaceAuthorityRevision,
    $fixnum.Int64? lifecycleGeneration,
    $core.String? treeAttachmentIncarnation,
    $1.MembershipBinding? membershipBinding,
    $core.bool? historyKnown,
    $2.Timestamp? historyVisibleSince,
    $core.String? effectiveEpisodeId,
    $fixnum.Int64? effectiveEpisodeGeneration,
    $fixnum.Int64? ledgerRevision,
    $2.Timestamp? evaluatedAt,
    $core.int? validityRemainingMs,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (spaceId != null) result.spaceId = spaceId;
    if (chat != null) result.chat = chat;
    if (viewerProfileId != null) result.viewerProfileId = viewerProfileId;
    if (eligible != null) result.eligible = eligible;
    if (spaceAuthorityRevision != null)
      result.spaceAuthorityRevision = spaceAuthorityRevision;
    if (lifecycleGeneration != null)
      result.lifecycleGeneration = lifecycleGeneration;
    if (treeAttachmentIncarnation != null)
      result.treeAttachmentIncarnation = treeAttachmentIncarnation;
    if (membershipBinding != null) result.membershipBinding = membershipBinding;
    if (historyKnown != null) result.historyKnown = historyKnown;
    if (historyVisibleSince != null)
      result.historyVisibleSince = historyVisibleSince;
    if (effectiveEpisodeId != null)
      result.effectiveEpisodeId = effectiveEpisodeId;
    if (effectiveEpisodeGeneration != null)
      result.effectiveEpisodeGeneration = effectiveEpisodeGeneration;
    if (ledgerRevision != null) result.ledgerRevision = ledgerRevision;
    if (evaluatedAt != null) result.evaluatedAt = evaluatedAt;
    if (validityRemainingMs != null)
      result.validityRemainingMs = validityRemainingMs;
    return result;
  }

  ResolveSpaceReportReadContextResponse._();

  factory ResolveSpaceReportReadContextResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveSpaceReportReadContextResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveSpaceReportReadContextResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.space.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'spaceId')
    ..aOM<$0.ChatRef>(4, _omitFieldNames ? '' : 'chat',
        subBuilder: $0.ChatRef.create)
    ..aOS(5, _omitFieldNames ? '' : 'viewerProfileId')
    ..aOB(6, _omitFieldNames ? '' : 'eligible')
    ..a<$fixnum.Int64>(
        7, _omitFieldNames ? '' : 'spaceAuthorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        8, _omitFieldNames ? '' : 'lifecycleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(9, _omitFieldNames ? '' : 'treeAttachmentIncarnation')
    ..aOM<$1.MembershipBinding>(10, _omitFieldNames ? '' : 'membershipBinding',
        subBuilder: $1.MembershipBinding.create)
    ..aOB(11, _omitFieldNames ? '' : 'historyKnown')
    ..aOM<$2.Timestamp>(12, _omitFieldNames ? '' : 'historyVisibleSince',
        subBuilder: $2.Timestamp.create)
    ..aOS(13, _omitFieldNames ? '' : 'effectiveEpisodeId')
    ..a<$fixnum.Int64>(14, _omitFieldNames ? '' : 'effectiveEpisodeGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        15, _omitFieldNames ? '' : 'ledgerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOM<$2.Timestamp>(16, _omitFieldNames ? '' : 'evaluatedAt',
        subBuilder: $2.Timestamp.create)
    ..aI(17, _omitFieldNames ? '' : 'validityRemainingMs',
        fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveSpaceReportReadContextResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveSpaceReportReadContextResponse copyWith(
          void Function(ResolveSpaceReportReadContextResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ResolveSpaceReportReadContextResponse))
          as ResolveSpaceReportReadContextResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveSpaceReportReadContextResponse create() =>
      ResolveSpaceReportReadContextResponse._();
  @$core.override
  ResolveSpaceReportReadContextResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveSpaceReportReadContextResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<
          ResolveSpaceReportReadContextResponse>(create);
  static ResolveSpaceReportReadContextResponse? _defaultInstance;

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
  $core.String get spaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set spaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $0.ChatRef get chat => $_getN(3);
  @$pb.TagNumber(4)
  set chat($0.ChatRef value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasChat() => $_has(3);
  @$pb.TagNumber(4)
  void clearChat() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.ChatRef ensureChat() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get viewerProfileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set viewerProfileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasViewerProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearViewerProfileId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get eligible => $_getBF(5);
  @$pb.TagNumber(6)
  set eligible($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasEligible() => $_has(5);
  @$pb.TagNumber(6)
  void clearEligible() => $_clearField(6);

  @$pb.TagNumber(7)
  $fixnum.Int64 get spaceAuthorityRevision => $_getI64(6);
  @$pb.TagNumber(7)
  set spaceAuthorityRevision($fixnum.Int64 value) => $_setInt64(6, value);
  @$pb.TagNumber(7)
  $core.bool hasSpaceAuthorityRevision() => $_has(6);
  @$pb.TagNumber(7)
  void clearSpaceAuthorityRevision() => $_clearField(7);

  @$pb.TagNumber(8)
  $fixnum.Int64 get lifecycleGeneration => $_getI64(7);
  @$pb.TagNumber(8)
  set lifecycleGeneration($fixnum.Int64 value) => $_setInt64(7, value);
  @$pb.TagNumber(8)
  $core.bool hasLifecycleGeneration() => $_has(7);
  @$pb.TagNumber(8)
  void clearLifecycleGeneration() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get treeAttachmentIncarnation => $_getSZ(8);
  @$pb.TagNumber(9)
  set treeAttachmentIncarnation($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasTreeAttachmentIncarnation() => $_has(8);
  @$pb.TagNumber(9)
  void clearTreeAttachmentIncarnation() => $_clearField(9);

  @$pb.TagNumber(10)
  $1.MembershipBinding get membershipBinding => $_getN(9);
  @$pb.TagNumber(10)
  set membershipBinding($1.MembershipBinding value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasMembershipBinding() => $_has(9);
  @$pb.TagNumber(10)
  void clearMembershipBinding() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.MembershipBinding ensureMembershipBinding() => $_ensure(9);

  @$pb.TagNumber(11)
  $core.bool get historyKnown => $_getBF(10);
  @$pb.TagNumber(11)
  set historyKnown($core.bool value) => $_setBool(10, value);
  @$pb.TagNumber(11)
  $core.bool hasHistoryKnown() => $_has(10);
  @$pb.TagNumber(11)
  void clearHistoryKnown() => $_clearField(11);

  @$pb.TagNumber(12)
  $2.Timestamp get historyVisibleSince => $_getN(11);
  @$pb.TagNumber(12)
  set historyVisibleSince($2.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasHistoryVisibleSince() => $_has(11);
  @$pb.TagNumber(12)
  void clearHistoryVisibleSince() => $_clearField(12);
  @$pb.TagNumber(12)
  $2.Timestamp ensureHistoryVisibleSince() => $_ensure(11);

  @$pb.TagNumber(13)
  $core.String get effectiveEpisodeId => $_getSZ(12);
  @$pb.TagNumber(13)
  set effectiveEpisodeId($core.String value) => $_setString(12, value);
  @$pb.TagNumber(13)
  $core.bool hasEffectiveEpisodeId() => $_has(12);
  @$pb.TagNumber(13)
  void clearEffectiveEpisodeId() => $_clearField(13);

  @$pb.TagNumber(14)
  $fixnum.Int64 get effectiveEpisodeGeneration => $_getI64(13);
  @$pb.TagNumber(14)
  set effectiveEpisodeGeneration($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasEffectiveEpisodeGeneration() => $_has(13);
  @$pb.TagNumber(14)
  void clearEffectiveEpisodeGeneration() => $_clearField(14);

  @$pb.TagNumber(15)
  $fixnum.Int64 get ledgerRevision => $_getI64(14);
  @$pb.TagNumber(15)
  set ledgerRevision($fixnum.Int64 value) => $_setInt64(14, value);
  @$pb.TagNumber(15)
  $core.bool hasLedgerRevision() => $_has(14);
  @$pb.TagNumber(15)
  void clearLedgerRevision() => $_clearField(15);

  @$pb.TagNumber(16)
  $2.Timestamp get evaluatedAt => $_getN(15);
  @$pb.TagNumber(16)
  set evaluatedAt($2.Timestamp value) => $_setField(16, value);
  @$pb.TagNumber(16)
  $core.bool hasEvaluatedAt() => $_has(15);
  @$pb.TagNumber(16)
  void clearEvaluatedAt() => $_clearField(16);
  @$pb.TagNumber(16)
  $2.Timestamp ensureEvaluatedAt() => $_ensure(15);

  @$pb.TagNumber(17)
  $core.int get validityRemainingMs => $_getIZ(16);
  @$pb.TagNumber(17)
  set validityRemainingMs($core.int value) => $_setUnsignedInt32(16, value);
  @$pb.TagNumber(17)
  $core.bool hasValidityRemainingMs() => $_has(16);
  @$pb.TagNumber(17)
  void clearValidityRemainingMs() => $_clearField(17);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
