// This is a generated file - do not edit.
//
// Generated from voice/moderation/v1/moderation.proto.

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

import '../../chat/v1/chat.pb.dart' as $3;
import '../../common/v1/common.pb.dart' as $2;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class Report extends $pb.GeneratedMessage {
  factory Report({
    $core.String? id,
    $core.String? reporterProfileId,
    $core.String? targetType,
    $core.String? targetId,
    $core.String? category,
    $core.String? description,
    $core.String? evidenceJson,
    $core.String? status,
    $core.String? assignedToProfileId,
    $1.Timestamp? resolvedAt,
    $core.String? resolutionJson,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (reporterProfileId != null) result.reporterProfileId = reporterProfileId;
    if (targetType != null) result.targetType = targetType;
    if (targetId != null) result.targetId = targetId;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (evidenceJson != null) result.evidenceJson = evidenceJson;
    if (status != null) result.status = status;
    if (assignedToProfileId != null)
      result.assignedToProfileId = assignedToProfileId;
    if (resolvedAt != null) result.resolvedAt = resolvedAt;
    if (resolutionJson != null) result.resolutionJson = resolutionJson;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  Report._();

  factory Report.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Report.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Report',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'reporterProfileId')
    ..aOS(3, _omitFieldNames ? '' : 'targetType')
    ..aOS(4, _omitFieldNames ? '' : 'targetId')
    ..aOS(5, _omitFieldNames ? '' : 'category')
    ..aOS(6, _omitFieldNames ? '' : 'description')
    ..aOS(7, _omitFieldNames ? '' : 'evidenceJson')
    ..aOS(8, _omitFieldNames ? '' : 'status')
    ..aOS(9, _omitFieldNames ? '' : 'assignedToProfileId')
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'resolvedAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(11, _omitFieldNames ? '' : 'resolutionJson')
    ..aOM<$1.Timestamp>(12, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Report clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Report copyWith(void Function(Report) updates) =>
      super.copyWith((message) => updates(message as Report)) as Report;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Report create() => Report._();
  @$core.override
  Report createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Report getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Report>(create);
  static Report? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reporterProfileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set reporterProfileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReporterProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearReporterProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get targetType => $_getSZ(2);
  @$pb.TagNumber(3)
  set targetType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTargetType() => $_has(2);
  @$pb.TagNumber(3)
  void clearTargetType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get targetId => $_getSZ(3);
  @$pb.TagNumber(4)
  set targetId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTargetId() => $_has(3);
  @$pb.TagNumber(4)
  void clearTargetId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get category => $_getSZ(4);
  @$pb.TagNumber(5)
  set category($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasCategory() => $_has(4);
  @$pb.TagNumber(5)
  void clearCategory() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get description => $_getSZ(5);
  @$pb.TagNumber(6)
  set description($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasDescription() => $_has(5);
  @$pb.TagNumber(6)
  void clearDescription() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get evidenceJson => $_getSZ(6);
  @$pb.TagNumber(7)
  set evidenceJson($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasEvidenceJson() => $_has(6);
  @$pb.TagNumber(7)
  void clearEvidenceJson() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get status => $_getSZ(7);
  @$pb.TagNumber(8)
  set status($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasStatus() => $_has(7);
  @$pb.TagNumber(8)
  void clearStatus() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get assignedToProfileId => $_getSZ(8);
  @$pb.TagNumber(9)
  set assignedToProfileId($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasAssignedToProfileId() => $_has(8);
  @$pb.TagNumber(9)
  void clearAssignedToProfileId() => $_clearField(9);

  @$pb.TagNumber(10)
  $1.Timestamp get resolvedAt => $_getN(9);
  @$pb.TagNumber(10)
  set resolvedAt($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasResolvedAt() => $_has(9);
  @$pb.TagNumber(10)
  void clearResolvedAt() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureResolvedAt() => $_ensure(9);

  @$pb.TagNumber(11)
  $core.String get resolutionJson => $_getSZ(10);
  @$pb.TagNumber(11)
  set resolutionJson($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasResolutionJson() => $_has(10);
  @$pb.TagNumber(11)
  void clearResolutionJson() => $_clearField(11);

  @$pb.TagNumber(12)
  $1.Timestamp get createdAt => $_getN(11);
  @$pb.TagNumber(12)
  set createdAt($1.Timestamp value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasCreatedAt() => $_has(11);
  @$pb.TagNumber(12)
  void clearCreatedAt() => $_clearField(12);
  @$pb.TagNumber(12)
  $1.Timestamp ensureCreatedAt() => $_ensure(11);
}

class CreateReportRequest extends $pb.GeneratedMessage {
  factory CreateReportRequest({
    $core.String? targetType,
    $core.String? targetId,
    $core.String? category,
    $core.String? description,
    $core.String? evidenceJson,
  }) {
    final result = create();
    if (targetType != null) result.targetType = targetType;
    if (targetId != null) result.targetId = targetId;
    if (category != null) result.category = category;
    if (description != null) result.description = description;
    if (evidenceJson != null) result.evidenceJson = evidenceJson;
    return result;
  }

  CreateReportRequest._();

  factory CreateReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReportRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetType')
    ..aOS(2, _omitFieldNames ? '' : 'targetId')
    ..aOS(3, _omitFieldNames ? '' : 'category')
    ..aOS(4, _omitFieldNames ? '' : 'description')
    ..aOS(5, _omitFieldNames ? '' : 'evidenceJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportRequest copyWith(void Function(CreateReportRequest) updates) =>
      super.copyWith((message) => updates(message as CreateReportRequest))
          as CreateReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateReportRequest create() => CreateReportRequest._();
  @$core.override
  CreateReportRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReportRequest>(create);
  static CreateReportRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetType => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetType() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetType() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get targetId => $_getSZ(1);
  @$pb.TagNumber(2)
  set targetId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTargetId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTargetId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get category => $_getSZ(2);
  @$pb.TagNumber(3)
  set category($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCategory() => $_has(2);
  @$pb.TagNumber(3)
  void clearCategory() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get description => $_getSZ(3);
  @$pb.TagNumber(4)
  set description($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDescription() => $_has(3);
  @$pb.TagNumber(4)
  void clearDescription() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get evidenceJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set evidenceJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasEvidenceJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearEvidenceJson() => $_clearField(5);
}

class GetReportRequest extends $pb.GeneratedMessage {
  factory GetReportRequest({
    $core.String? reportId,
  }) {
    final result = create();
    if (reportId != null) result.reportId = reportId;
    return result;
  }

  GetReportRequest._();

  factory GetReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReportRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReportRequest copyWith(void Function(GetReportRequest) updates) =>
      super.copyWith((message) => updates(message as GetReportRequest))
          as GetReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetReportRequest create() => GetReportRequest._();
  @$core.override
  GetReportRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetReportRequest>(create);
  static GetReportRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);
}

class ListReportsRequest extends $pb.GeneratedMessage {
  factory ListReportsRequest({
    $core.String? statusFilter,
    $2.CursorPageRequest? page,
    $core.String? queueFilter,
  }) {
    final result = create();
    if (statusFilter != null) result.statusFilter = statusFilter;
    if (page != null) result.page = page;
    if (queueFilter != null) result.queueFilter = queueFilter;
    return result;
  }

  ListReportsRequest._();

  factory ListReportsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListReportsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListReportsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'statusFilter')
    ..aOM<$2.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $2.CursorPageRequest.create)
    ..aOS(3, _omitFieldNames ? '' : 'queueFilter')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListReportsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListReportsRequest copyWith(void Function(ListReportsRequest) updates) =>
      super.copyWith((message) => updates(message as ListReportsRequest))
          as ListReportsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListReportsRequest create() => ListReportsRequest._();
  @$core.override
  ListReportsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListReportsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListReportsRequest>(create);
  static ListReportsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get statusFilter => $_getSZ(0);
  @$pb.TagNumber(1)
  set statusFilter($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStatusFilter() => $_has(0);
  @$pb.TagNumber(1)
  void clearStatusFilter() => $_clearField(1);

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

  /// Phase 14: "content" (user/message) vs "spaces" queues — docs/features/reports.md.
  @$pb.TagNumber(3)
  $core.String get queueFilter => $_getSZ(2);
  @$pb.TagNumber(3)
  set queueFilter($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasQueueFilter() => $_has(2);
  @$pb.TagNumber(3)
  void clearQueueFilter() => $_clearField(3);
}

class ReportList extends $pb.GeneratedMessage {
  factory ReportList({
    $core.Iterable<Report>? reports,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (reports != null) result.reports.addAll(reports);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  ReportList._();

  factory ReportList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReportList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReportList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..pPM<Report>(1, _omitFieldNames ? '' : 'reports',
        subBuilder: Report.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReportList copyWith(void Function(ReportList) updates) =>
      super.copyWith((message) => updates(message as ReportList)) as ReportList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReportList create() => ReportList._();
  @$core.override
  ReportList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReportList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReportList>(create);
  static ReportList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Report> get reports => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class ResolveReportRequest extends $pb.GeneratedMessage {
  factory ResolveReportRequest({
    $core.String? reportId,
    $core.String? resolutionJson,
    $core.String? newStatus,
    $core.String? assignedToProfileId,
  }) {
    final result = create();
    if (reportId != null) result.reportId = reportId;
    if (resolutionJson != null) result.resolutionJson = resolutionJson;
    if (newStatus != null) result.newStatus = newStatus;
    if (assignedToProfileId != null)
      result.assignedToProfileId = assignedToProfileId;
    return result;
  }

  ResolveReportRequest._();

  factory ResolveReportRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveReportRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveReportRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..aOS(2, _omitFieldNames ? '' : 'resolutionJson')
    ..aOS(3, _omitFieldNames ? '' : 'newStatus')
    ..aOS(4, _omitFieldNames ? '' : 'assignedToProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportRequest copyWith(void Function(ResolveReportRequest) updates) =>
      super.copyWith((message) => updates(message as ResolveReportRequest))
          as ResolveReportRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveReportRequest create() => ResolveReportRequest._();
  @$core.override
  ResolveReportRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveReportRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveReportRequest>(create);
  static ResolveReportRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get resolutionJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set resolutionJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResolutionJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearResolutionJson() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get newStatus => $_getSZ(2);
  @$pb.TagNumber(3)
  set newStatus($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasNewStatus() => $_has(2);
  @$pb.TagNumber(3)
  void clearNewStatus() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get assignedToProfileId => $_getSZ(3);
  @$pb.TagNumber(4)
  set assignedToProfileId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAssignedToProfileId() => $_has(3);
  @$pb.TagNumber(4)
  void clearAssignedToProfileId() => $_clearField(4);
}

class Sanction extends $pb.GeneratedMessage {
  factory Sanction({
    $core.String? id,
    $core.String? targetAccountId,
    $core.String? type,
    $core.String? reason,
    $core.String? reportId,
    $core.String? issuedByProfileId,
    $1.Timestamp? expiresAt,
    $1.Timestamp? revokedAt,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (targetAccountId != null) result.targetAccountId = targetAccountId;
    if (type != null) result.type = type;
    if (reason != null) result.reason = reason;
    if (reportId != null) result.reportId = reportId;
    if (issuedByProfileId != null) result.issuedByProfileId = issuedByProfileId;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (revokedAt != null) result.revokedAt = revokedAt;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  Sanction._();

  factory Sanction.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Sanction.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Sanction',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'targetAccountId')
    ..aOS(3, _omitFieldNames ? '' : 'type')
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..aOS(5, _omitFieldNames ? '' : 'reportId')
    ..aOS(6, _omitFieldNames ? '' : 'issuedByProfileId')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'revokedAt',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sanction clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Sanction copyWith(void Function(Sanction) updates) =>
      super.copyWith((message) => updates(message as Sanction)) as Sanction;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Sanction create() => Sanction._();
  @$core.override
  Sanction createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Sanction getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Sanction>(create);
  static Sanction? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get targetAccountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set targetAccountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTargetAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearTargetAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get type => $_getSZ(2);
  @$pb.TagNumber(3)
  set type($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasType() => $_has(2);
  @$pb.TagNumber(3)
  void clearType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get reportId => $_getSZ(4);
  @$pb.TagNumber(5)
  set reportId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasReportId() => $_has(4);
  @$pb.TagNumber(5)
  void clearReportId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get issuedByProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set issuedByProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasIssuedByProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearIssuedByProfileId() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.Timestamp get expiresAt => $_getN(6);
  @$pb.TagNumber(7)
  set expiresAt($1.Timestamp value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasExpiresAt() => $_has(6);
  @$pb.TagNumber(7)
  void clearExpiresAt() => $_clearField(7);
  @$pb.TagNumber(7)
  $1.Timestamp ensureExpiresAt() => $_ensure(6);

  @$pb.TagNumber(8)
  $1.Timestamp get revokedAt => $_getN(7);
  @$pb.TagNumber(8)
  set revokedAt($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasRevokedAt() => $_has(7);
  @$pb.TagNumber(8)
  void clearRevokedAt() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureRevokedAt() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get createdAt => $_getN(8);
  @$pb.TagNumber(9)
  set createdAt($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCreatedAt() => $_has(8);
  @$pb.TagNumber(9)
  void clearCreatedAt() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureCreatedAt() => $_ensure(8);
}

class ApplySanctionRequest extends $pb.GeneratedMessage {
  factory ApplySanctionRequest({
    $core.String? targetAccountId,
    $core.String? type,
    $core.String? reason,
    $core.String? reportId,
    $1.Timestamp? expiresAt,
  }) {
    final result = create();
    if (targetAccountId != null) result.targetAccountId = targetAccountId;
    if (type != null) result.type = type;
    if (reason != null) result.reason = reason;
    if (reportId != null) result.reportId = reportId;
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  ApplySanctionRequest._();

  factory ApplySanctionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySanctionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySanctionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'targetAccountId')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..aOS(4, _omitFieldNames ? '' : 'reportId')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySanctionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySanctionRequest copyWith(void Function(ApplySanctionRequest) updates) =>
      super.copyWith((message) => updates(message as ApplySanctionRequest))
          as ApplySanctionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySanctionRequest create() => ApplySanctionRequest._();
  @$core.override
  ApplySanctionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySanctionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySanctionRequest>(create);
  static ApplySanctionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get targetAccountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set targetAccountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTargetAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearTargetAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get reportId => $_getSZ(3);
  @$pb.TagNumber(4)
  set reportId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReportId() => $_has(3);
  @$pb.TagNumber(4)
  void clearReportId() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get expiresAt => $_getN(4);
  @$pb.TagNumber(5)
  set expiresAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasExpiresAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpiresAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureExpiresAt() => $_ensure(4);
}

class RevokeSanctionRequest extends $pb.GeneratedMessage {
  factory RevokeSanctionRequest({
    $core.String? sanctionId,
  }) {
    final result = create();
    if (sanctionId != null) result.sanctionId = sanctionId;
    return result;
  }

  RevokeSanctionRequest._();

  factory RevokeSanctionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeSanctionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeSanctionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sanctionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSanctionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSanctionRequest copyWith(
          void Function(RevokeSanctionRequest) updates) =>
      super.copyWith((message) => updates(message as RevokeSanctionRequest))
          as RevokeSanctionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeSanctionRequest create() => RevokeSanctionRequest._();
  @$core.override
  RevokeSanctionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeSanctionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeSanctionRequest>(create);
  static RevokeSanctionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sanctionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sanctionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSanctionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanctionId() => $_clearField(1);
}

class GetAccountSanctionsRequest extends $pb.GeneratedMessage {
  factory GetAccountSanctionsRequest({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  GetAccountSanctionsRequest._();

  factory GetAccountSanctionsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAccountSanctionsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAccountSanctionsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAccountSanctionsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAccountSanctionsRequest copyWith(
          void Function(GetAccountSanctionsRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetAccountSanctionsRequest))
          as GetAccountSanctionsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAccountSanctionsRequest create() => GetAccountSanctionsRequest._();
  @$core.override
  GetAccountSanctionsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAccountSanctionsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAccountSanctionsRequest>(create);
  static GetAccountSanctionsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class SanctionList extends $pb.GeneratedMessage {
  factory SanctionList({
    $core.Iterable<Sanction>? sanctions,
  }) {
    final result = create();
    if (sanctions != null) result.sanctions.addAll(sanctions);
    return result;
  }

  SanctionList._();

  factory SanctionList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SanctionList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SanctionList',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..pPM<Sanction>(1, _omitFieldNames ? '' : 'sanctions',
        subBuilder: Sanction.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SanctionList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SanctionList copyWith(void Function(SanctionList) updates) =>
      super.copyWith((message) => updates(message as SanctionList))
          as SanctionList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SanctionList create() => SanctionList._();
  @$core.override
  SanctionList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SanctionList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SanctionList>(create);
  static SanctionList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<Sanction> get sanctions => $_getList(0);
}

class GetActiveSanctionRequest extends $pb.GeneratedMessage {
  factory GetActiveSanctionRequest({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  GetActiveSanctionRequest._();

  factory GetActiveSanctionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveSanctionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveSanctionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveSanctionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveSanctionRequest copyWith(
          void Function(GetActiveSanctionRequest) updates) =>
      super.copyWith((message) => updates(message as GetActiveSanctionRequest))
          as GetActiveSanctionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveSanctionRequest create() => GetActiveSanctionRequest._();
  @$core.override
  GetActiveSanctionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveSanctionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveSanctionRequest>(create);
  static GetActiveSanctionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class Appeal extends $pb.GeneratedMessage {
  factory Appeal({
    $core.String? id,
    $core.String? sanctionId,
    $core.String? appellantAccountId,
    $core.String? reason,
    $core.String? status,
    $core.String? reviewedByProfileId,
    $1.Timestamp? createdAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (sanctionId != null) result.sanctionId = sanctionId;
    if (appellantAccountId != null)
      result.appellantAccountId = appellantAccountId;
    if (reason != null) result.reason = reason;
    if (status != null) result.status = status;
    if (reviewedByProfileId != null)
      result.reviewedByProfileId = reviewedByProfileId;
    if (createdAt != null) result.createdAt = createdAt;
    return result;
  }

  Appeal._();

  factory Appeal.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Appeal.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Appeal',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'sanctionId')
    ..aOS(3, _omitFieldNames ? '' : 'appellantAccountId')
    ..aOS(4, _omitFieldNames ? '' : 'reason')
    ..aOS(5, _omitFieldNames ? '' : 'status')
    ..aOS(6, _omitFieldNames ? '' : 'reviewedByProfileId')
    ..aOM<$1.Timestamp>(7, _omitFieldNames ? '' : 'createdAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Appeal clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Appeal copyWith(void Function(Appeal) updates) =>
      super.copyWith((message) => updates(message as Appeal)) as Appeal;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Appeal create() => Appeal._();
  @$core.override
  Appeal createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Appeal getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Appeal>(create);
  static Appeal? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sanctionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sanctionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSanctionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSanctionId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get appellantAccountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set appellantAccountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAppellantAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAppellantAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get reason => $_getSZ(3);
  @$pb.TagNumber(4)
  set reason($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasReason() => $_has(3);
  @$pb.TagNumber(4)
  void clearReason() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get status => $_getSZ(4);
  @$pb.TagNumber(5)
  set status($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStatus() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatus() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get reviewedByProfileId => $_getSZ(5);
  @$pb.TagNumber(6)
  set reviewedByProfileId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasReviewedByProfileId() => $_has(5);
  @$pb.TagNumber(6)
  void clearReviewedByProfileId() => $_clearField(6);

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
}

class SubmitAppealRequest extends $pb.GeneratedMessage {
  factory SubmitAppealRequest({
    $core.String? sanctionId,
    $core.String? reason,
  }) {
    final result = create();
    if (sanctionId != null) result.sanctionId = sanctionId;
    if (reason != null) result.reason = reason;
    return result;
  }

  SubmitAppealRequest._();

  factory SubmitAppealRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SubmitAppealRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAppealRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'sanctionId')
    ..aOS(2, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealRequest copyWith(void Function(SubmitAppealRequest) updates) =>
      super.copyWith((message) => updates(message as SubmitAppealRequest))
          as SubmitAppealRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubmitAppealRequest create() => SubmitAppealRequest._();
  @$core.override
  SubmitAppealRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SubmitAppealRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAppealRequest>(create);
  static SubmitAppealRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get sanctionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set sanctionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSanctionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanctionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get reason => $_getSZ(1);
  @$pb.TagNumber(2)
  set reason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearReason() => $_clearField(2);
}

class ReviewAppealRequest extends $pb.GeneratedMessage {
  factory ReviewAppealRequest({
    $core.String? appealId,
    $core.String? status,
    $core.String? moderatorNote,
  }) {
    final result = create();
    if (appealId != null) result.appealId = appealId;
    if (status != null) result.status = status;
    if (moderatorNote != null) result.moderatorNote = moderatorNote;
    return result;
  }

  ReviewAppealRequest._();

  factory ReviewAppealRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReviewAppealRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReviewAppealRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'appealId')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..aOS(3, _omitFieldNames ? '' : 'moderatorNote')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAppealRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAppealRequest copyWith(void Function(ReviewAppealRequest) updates) =>
      super.copyWith((message) => updates(message as ReviewAppealRequest))
          as ReviewAppealRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReviewAppealRequest create() => ReviewAppealRequest._();
  @$core.override
  ReviewAppealRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReviewAppealRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReviewAppealRequest>(create);
  static ReviewAppealRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get appealId => $_getSZ(0);
  @$pb.TagNumber(1)
  set appealId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAppealId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppealId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get moderatorNote => $_getSZ(2);
  @$pb.TagNumber(3)
  set moderatorNote($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasModeratorNote() => $_has(2);
  @$pb.TagNumber(3)
  void clearModeratorNote() => $_clearField(3);
}

class GetAppealRequest extends $pb.GeneratedMessage {
  factory GetAppealRequest({
    $core.String? appealId,
  }) {
    final result = create();
    if (appealId != null) result.appealId = appealId;
    return result;
  }

  GetAppealRequest._();

  factory GetAppealRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAppealRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAppealRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'appealId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAppealRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAppealRequest copyWith(void Function(GetAppealRequest) updates) =>
      super.copyWith((message) => updates(message as GetAppealRequest))
          as GetAppealRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAppealRequest create() => GetAppealRequest._();
  @$core.override
  GetAppealRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAppealRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAppealRequest>(create);
  static GetAppealRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get appealId => $_getSZ(0);
  @$pb.TagNumber(1)
  set appealId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAppealId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppealId() => $_clearField(1);
}

class CheckMessageRequest extends $pb.GeneratedMessage {
  factory CheckMessageRequest({
    $3.ChatRef? chat,
    $core.String? content,
    $core.String? senderProfileId,
  }) {
    final result = create();
    if (chat != null) result.chat = chat;
    if (content != null) result.content = content;
    if (senderProfileId != null) result.senderProfileId = senderProfileId;
    return result;
  }

  CheckMessageRequest._();

  factory CheckMessageRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckMessageRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckMessageRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<$3.ChatRef>(1, _omitFieldNames ? '' : 'chat',
        subBuilder: $3.ChatRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'content')
    ..aOS(3, _omitFieldNames ? '' : 'senderProfileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckMessageRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckMessageRequest copyWith(void Function(CheckMessageRequest) updates) =>
      super.copyWith((message) => updates(message as CheckMessageRequest))
          as CheckMessageRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckMessageRequest create() => CheckMessageRequest._();
  @$core.override
  CheckMessageRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckMessageRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckMessageRequest>(create);
  static CheckMessageRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $3.ChatRef get chat => $_getN(0);
  @$pb.TagNumber(1)
  set chat($3.ChatRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasChat() => $_has(0);
  @$pb.TagNumber(1)
  void clearChat() => $_clearField(1);
  @$pb.TagNumber(1)
  $3.ChatRef ensureChat() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get content => $_getSZ(1);
  @$pb.TagNumber(2)
  set content($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasContent() => $_has(1);
  @$pb.TagNumber(2)
  void clearContent() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get senderProfileId => $_getSZ(2);
  @$pb.TagNumber(3)
  set senderProfileId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSenderProfileId() => $_has(2);
  @$pb.TagNumber(3)
  void clearSenderProfileId() => $_clearField(3);
}

class CheckResult extends $pb.GeneratedMessage {
  factory CheckResult({
    $core.bool? allowed,
    $core.String? blockReason,
  }) {
    final result = create();
    if (allowed != null) result.allowed = allowed;
    if (blockReason != null) result.blockReason = blockReason;
    return result;
  }

  CheckResult._();

  factory CheckResult.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckResult.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckResult',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'allowed')
    ..aOS(2, _omitFieldNames ? '' : 'blockReason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckResult clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckResult copyWith(void Function(CheckResult) updates) =>
      super.copyWith((message) => updates(message as CheckResult))
          as CheckResult;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckResult create() => CheckResult._();
  @$core.override
  CheckResult createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckResult getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckResult>(create);
  static CheckResult? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get allowed => $_getBF(0);
  @$pb.TagNumber(1)
  set allowed($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAllowed() => $_has(0);
  @$pb.TagNumber(1)
  void clearAllowed() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get blockReason => $_getSZ(1);
  @$pb.TagNumber(2)
  set blockReason($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBlockReason() => $_has(1);
  @$pb.TagNumber(2)
  void clearBlockReason() => $_clearField(2);
}

class GetAutoModStatsRequest extends $pb.GeneratedMessage {
  factory GetAutoModStatsRequest() => create();

  GetAutoModStatsRequest._();

  factory GetAutoModStatsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAutoModStatsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAutoModStatsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAutoModStatsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAutoModStatsRequest copyWith(
          void Function(GetAutoModStatsRequest) updates) =>
      super.copyWith((message) => updates(message as GetAutoModStatsRequest))
          as GetAutoModStatsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAutoModStatsRequest create() => GetAutoModStatsRequest._();
  @$core.override
  GetAutoModStatsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAutoModStatsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAutoModStatsRequest>(create);
  static GetAutoModStatsRequest? _defaultInstance;
}

class AutoModStats extends $pb.GeneratedMessage {
  factory AutoModStats({
    $fixnum.Int64? messagesChecked,
    $fixnum.Int64? blocked,
  }) {
    final result = create();
    if (messagesChecked != null) result.messagesChecked = messagesChecked;
    if (blocked != null) result.blocked = blocked;
    return result;
  }

  AutoModStats._();

  factory AutoModStats.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AutoModStats.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AutoModStats',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aInt64(1, _omitFieldNames ? '' : 'messagesChecked')
    ..aInt64(2, _omitFieldNames ? '' : 'blocked')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AutoModStats clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AutoModStats copyWith(void Function(AutoModStats) updates) =>
      super.copyWith((message) => updates(message as AutoModStats))
          as AutoModStats;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AutoModStats create() => AutoModStats._();
  @$core.override
  AutoModStats createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AutoModStats getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AutoModStats>(create);
  static AutoModStats? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get messagesChecked => $_getI64(0);
  @$pb.TagNumber(1)
  set messagesChecked($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMessagesChecked() => $_has(0);
  @$pb.TagNumber(1)
  void clearMessagesChecked() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get blocked => $_getI64(1);
  @$pb.TagNumber(2)
  set blocked($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBlocked() => $_has(1);
  @$pb.TagNumber(2)
  void clearBlocked() => $_clearField(2);
}

class IsShadowBannedRequest extends $pb.GeneratedMessage {
  factory IsShadowBannedRequest({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  IsShadowBannedRequest._();

  factory IsShadowBannedRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IsShadowBannedRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IsShadowBannedRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsShadowBannedRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsShadowBannedRequest copyWith(
          void Function(IsShadowBannedRequest) updates) =>
      super.copyWith((message) => updates(message as IsShadowBannedRequest))
          as IsShadowBannedRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IsShadowBannedRequest create() => IsShadowBannedRequest._();
  @$core.override
  IsShadowBannedRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IsShadowBannedRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IsShadowBannedRequest>(create);
  static IsShadowBannedRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class IsShadowBannedResponse extends $pb.GeneratedMessage {
  factory IsShadowBannedResponse({
    $core.bool? shadowBanned,
  }) {
    final result = create();
    if (shadowBanned != null) result.shadowBanned = shadowBanned;
    return result;
  }

  IsShadowBannedResponse._();

  factory IsShadowBannedResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IsShadowBannedResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IsShadowBannedResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'shadowBanned')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsShadowBannedResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IsShadowBannedResponse copyWith(
          void Function(IsShadowBannedResponse) updates) =>
      super.copyWith((message) => updates(message as IsShadowBannedResponse))
          as IsShadowBannedResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IsShadowBannedResponse create() => IsShadowBannedResponse._();
  @$core.override
  IsShadowBannedResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IsShadowBannedResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IsShadowBannedResponse>(create);
  static IsShadowBannedResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get shadowBanned => $_getBF(0);
  @$pb.TagNumber(1)
  set shadowBanned($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasShadowBanned() => $_has(0);
  @$pb.TagNumber(1)
  void clearShadowBanned() => $_clearField(1);
}

class CreateReportResponse extends $pb.GeneratedMessage {
  factory CreateReportResponse({
    Report? report,
  }) {
    final result = create();
    if (report != null) result.report = report;
    return result;
  }

  CreateReportResponse._();

  factory CreateReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateReportResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Report>(1, _omitFieldNames ? '' : 'report', subBuilder: Report.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateReportResponse copyWith(void Function(CreateReportResponse) updates) =>
      super.copyWith((message) => updates(message as CreateReportResponse))
          as CreateReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateReportResponse create() => CreateReportResponse._();
  @$core.override
  CreateReportResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateReportResponse>(create);
  static CreateReportResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Report get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(Report value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  Report ensureReport() => $_ensure(0);
}

class GetReportResponse extends $pb.GeneratedMessage {
  factory GetReportResponse({
    Report? report,
  }) {
    final result = create();
    if (report != null) result.report = report;
    return result;
  }

  GetReportResponse._();

  factory GetReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetReportResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Report>(1, _omitFieldNames ? '' : 'report', subBuilder: Report.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetReportResponse copyWith(void Function(GetReportResponse) updates) =>
      super.copyWith((message) => updates(message as GetReportResponse))
          as GetReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetReportResponse create() => GetReportResponse._();
  @$core.override
  GetReportResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetReportResponse>(create);
  static GetReportResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Report get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(Report value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  Report ensureReport() => $_ensure(0);
}

class ListReportsResponse extends $pb.GeneratedMessage {
  factory ListReportsResponse({
    ReportList? reportList,
  }) {
    final result = create();
    if (reportList != null) result.reportList = reportList;
    return result;
  }

  ListReportsResponse._();

  factory ListReportsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ListReportsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ListReportsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<ReportList>(1, _omitFieldNames ? '' : 'reportList',
        subBuilder: ReportList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListReportsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ListReportsResponse copyWith(void Function(ListReportsResponse) updates) =>
      super.copyWith((message) => updates(message as ListReportsResponse))
          as ListReportsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ListReportsResponse create() => ListReportsResponse._();
  @$core.override
  ListReportsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ListReportsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ListReportsResponse>(create);
  static ListReportsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  ReportList get reportList => $_getN(0);
  @$pb.TagNumber(1)
  set reportList(ReportList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReportList() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportList() => $_clearField(1);
  @$pb.TagNumber(1)
  ReportList ensureReportList() => $_ensure(0);
}

class ResolveReportResponse extends $pb.GeneratedMessage {
  factory ResolveReportResponse({
    Report? report,
  }) {
    final result = create();
    if (report != null) result.report = report;
    return result;
  }

  ResolveReportResponse._();

  factory ResolveReportResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveReportResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveReportResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Report>(1, _omitFieldNames ? '' : 'report', subBuilder: Report.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportResponse copyWith(
          void Function(ResolveReportResponse) updates) =>
      super.copyWith((message) => updates(message as ResolveReportResponse))
          as ResolveReportResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveReportResponse create() => ResolveReportResponse._();
  @$core.override
  ResolveReportResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveReportResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveReportResponse>(create);
  static ResolveReportResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Report get report => $_getN(0);
  @$pb.TagNumber(1)
  set report(Report value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasReport() => $_has(0);
  @$pb.TagNumber(1)
  void clearReport() => $_clearField(1);
  @$pb.TagNumber(1)
  Report ensureReport() => $_ensure(0);
}

class ApplySanctionResponse extends $pb.GeneratedMessage {
  factory ApplySanctionResponse({
    Sanction? sanction,
  }) {
    final result = create();
    if (sanction != null) result.sanction = sanction;
    return result;
  }

  ApplySanctionResponse._();

  factory ApplySanctionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplySanctionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplySanctionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Sanction>(1, _omitFieldNames ? '' : 'sanction',
        subBuilder: Sanction.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySanctionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplySanctionResponse copyWith(
          void Function(ApplySanctionResponse) updates) =>
      super.copyWith((message) => updates(message as ApplySanctionResponse))
          as ApplySanctionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplySanctionResponse create() => ApplySanctionResponse._();
  @$core.override
  ApplySanctionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplySanctionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplySanctionResponse>(create);
  static ApplySanctionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Sanction get sanction => $_getN(0);
  @$pb.TagNumber(1)
  set sanction(Sanction value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSanction() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanction() => $_clearField(1);
  @$pb.TagNumber(1)
  Sanction ensureSanction() => $_ensure(0);
}

class RevokeSanctionResponse extends $pb.GeneratedMessage {
  factory RevokeSanctionResponse() => create();

  RevokeSanctionResponse._();

  factory RevokeSanctionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RevokeSanctionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RevokeSanctionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSanctionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RevokeSanctionResponse copyWith(
          void Function(RevokeSanctionResponse) updates) =>
      super.copyWith((message) => updates(message as RevokeSanctionResponse))
          as RevokeSanctionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RevokeSanctionResponse create() => RevokeSanctionResponse._();
  @$core.override
  RevokeSanctionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RevokeSanctionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RevokeSanctionResponse>(create);
  static RevokeSanctionResponse? _defaultInstance;
}

class GetAccountSanctionsResponse extends $pb.GeneratedMessage {
  factory GetAccountSanctionsResponse({
    SanctionList? sanctionList,
  }) {
    final result = create();
    if (sanctionList != null) result.sanctionList = sanctionList;
    return result;
  }

  GetAccountSanctionsResponse._();

  factory GetAccountSanctionsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAccountSanctionsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAccountSanctionsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<SanctionList>(1, _omitFieldNames ? '' : 'sanctionList',
        subBuilder: SanctionList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAccountSanctionsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAccountSanctionsResponse copyWith(
          void Function(GetAccountSanctionsResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetAccountSanctionsResponse))
          as GetAccountSanctionsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAccountSanctionsResponse create() =>
      GetAccountSanctionsResponse._();
  @$core.override
  GetAccountSanctionsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAccountSanctionsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAccountSanctionsResponse>(create);
  static GetAccountSanctionsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SanctionList get sanctionList => $_getN(0);
  @$pb.TagNumber(1)
  set sanctionList(SanctionList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSanctionList() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanctionList() => $_clearField(1);
  @$pb.TagNumber(1)
  SanctionList ensureSanctionList() => $_ensure(0);
}

class GetActiveSanctionResponse extends $pb.GeneratedMessage {
  factory GetActiveSanctionResponse({
    Sanction? sanction,
  }) {
    final result = create();
    if (sanction != null) result.sanction = sanction;
    return result;
  }

  GetActiveSanctionResponse._();

  factory GetActiveSanctionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetActiveSanctionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetActiveSanctionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Sanction>(1, _omitFieldNames ? '' : 'sanction',
        subBuilder: Sanction.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveSanctionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetActiveSanctionResponse copyWith(
          void Function(GetActiveSanctionResponse) updates) =>
      super.copyWith((message) => updates(message as GetActiveSanctionResponse))
          as GetActiveSanctionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetActiveSanctionResponse create() => GetActiveSanctionResponse._();
  @$core.override
  GetActiveSanctionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetActiveSanctionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetActiveSanctionResponse>(create);
  static GetActiveSanctionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Sanction get sanction => $_getN(0);
  @$pb.TagNumber(1)
  set sanction(Sanction value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSanction() => $_has(0);
  @$pb.TagNumber(1)
  void clearSanction() => $_clearField(1);
  @$pb.TagNumber(1)
  Sanction ensureSanction() => $_ensure(0);
}

class SubmitAppealResponse extends $pb.GeneratedMessage {
  factory SubmitAppealResponse({
    Appeal? appeal,
  }) {
    final result = create();
    if (appeal != null) result.appeal = appeal;
    return result;
  }

  SubmitAppealResponse._();

  factory SubmitAppealResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SubmitAppealResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SubmitAppealResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Appeal>(1, _omitFieldNames ? '' : 'appeal', subBuilder: Appeal.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SubmitAppealResponse copyWith(void Function(SubmitAppealResponse) updates) =>
      super.copyWith((message) => updates(message as SubmitAppealResponse))
          as SubmitAppealResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SubmitAppealResponse create() => SubmitAppealResponse._();
  @$core.override
  SubmitAppealResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SubmitAppealResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SubmitAppealResponse>(create);
  static SubmitAppealResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Appeal get appeal => $_getN(0);
  @$pb.TagNumber(1)
  set appeal(Appeal value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAppeal() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppeal() => $_clearField(1);
  @$pb.TagNumber(1)
  Appeal ensureAppeal() => $_ensure(0);
}

class ReviewAppealResponse extends $pb.GeneratedMessage {
  factory ReviewAppealResponse({
    Appeal? appeal,
  }) {
    final result = create();
    if (appeal != null) result.appeal = appeal;
    return result;
  }

  ReviewAppealResponse._();

  factory ReviewAppealResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReviewAppealResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReviewAppealResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Appeal>(1, _omitFieldNames ? '' : 'appeal', subBuilder: Appeal.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAppealResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReviewAppealResponse copyWith(void Function(ReviewAppealResponse) updates) =>
      super.copyWith((message) => updates(message as ReviewAppealResponse))
          as ReviewAppealResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReviewAppealResponse create() => ReviewAppealResponse._();
  @$core.override
  ReviewAppealResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReviewAppealResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReviewAppealResponse>(create);
  static ReviewAppealResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Appeal get appeal => $_getN(0);
  @$pb.TagNumber(1)
  set appeal(Appeal value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAppeal() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppeal() => $_clearField(1);
  @$pb.TagNumber(1)
  Appeal ensureAppeal() => $_ensure(0);
}

class GetAppealResponse extends $pb.GeneratedMessage {
  factory GetAppealResponse({
    Appeal? appeal,
  }) {
    final result = create();
    if (appeal != null) result.appeal = appeal;
    return result;
  }

  GetAppealResponse._();

  factory GetAppealResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAppealResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAppealResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<Appeal>(1, _omitFieldNames ? '' : 'appeal', subBuilder: Appeal.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAppealResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAppealResponse copyWith(void Function(GetAppealResponse) updates) =>
      super.copyWith((message) => updates(message as GetAppealResponse))
          as GetAppealResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAppealResponse create() => GetAppealResponse._();
  @$core.override
  GetAppealResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAppealResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAppealResponse>(create);
  static GetAppealResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Appeal get appeal => $_getN(0);
  @$pb.TagNumber(1)
  set appeal(Appeal value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAppeal() => $_has(0);
  @$pb.TagNumber(1)
  void clearAppeal() => $_clearField(1);
  @$pb.TagNumber(1)
  Appeal ensureAppeal() => $_ensure(0);
}

class CheckMessageResponse extends $pb.GeneratedMessage {
  factory CheckMessageResponse({
    CheckResult? checkResult,
  }) {
    final result = create();
    if (checkResult != null) result.checkResult = checkResult;
    return result;
  }

  CheckMessageResponse._();

  factory CheckMessageResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckMessageResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckMessageResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<CheckResult>(1, _omitFieldNames ? '' : 'checkResult',
        subBuilder: CheckResult.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckMessageResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckMessageResponse copyWith(void Function(CheckMessageResponse) updates) =>
      super.copyWith((message) => updates(message as CheckMessageResponse))
          as CheckMessageResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckMessageResponse create() => CheckMessageResponse._();
  @$core.override
  CheckMessageResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckMessageResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckMessageResponse>(create);
  static CheckMessageResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CheckResult get checkResult => $_getN(0);
  @$pb.TagNumber(1)
  set checkResult(CheckResult value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCheckResult() => $_has(0);
  @$pb.TagNumber(1)
  void clearCheckResult() => $_clearField(1);
  @$pb.TagNumber(1)
  CheckResult ensureCheckResult() => $_ensure(0);
}

class GetAutoModStatsResponse extends $pb.GeneratedMessage {
  factory GetAutoModStatsResponse({
    AutoModStats? autoModStats,
  }) {
    final result = create();
    if (autoModStats != null) result.autoModStats = autoModStats;
    return result;
  }

  GetAutoModStatsResponse._();

  factory GetAutoModStatsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetAutoModStatsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetAutoModStatsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.moderation.v1'),
      createEmptyInstance: create)
    ..aOM<AutoModStats>(1, _omitFieldNames ? '' : 'autoModStats',
        subBuilder: AutoModStats.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAutoModStatsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetAutoModStatsResponse copyWith(
          void Function(GetAutoModStatsResponse) updates) =>
      super.copyWith((message) => updates(message as GetAutoModStatsResponse))
          as GetAutoModStatsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetAutoModStatsResponse create() => GetAutoModStatsResponse._();
  @$core.override
  GetAutoModStatsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetAutoModStatsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetAutoModStatsResponse>(create);
  static GetAutoModStatsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AutoModStats get autoModStats => $_getN(0);
  @$pb.TagNumber(1)
  set autoModStats(AutoModStats value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasAutoModStats() => $_has(0);
  @$pb.TagNumber(1)
  void clearAutoModStats() => $_clearField(1);
  @$pb.TagNumber(1)
  AutoModStats ensureAutoModStats() => $_ensure(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
