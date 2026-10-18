// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/report_target.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

import '../../common/v1/report_target.pbenum.dart' as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class ResolveReportTargetRequest extends $pb.GeneratedMessage {
  factory ResolveReportTargetRequest({
    $core.String? reportId,
    $core.String? messageId,
  }) {
    final result = create();
    if (reportId != null) result.reportId = reportId;
    if (messageId != null) result.messageId = messageId;
    return result;
  }

  ResolveReportTargetRequest._();

  factory ResolveReportTargetRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveReportTargetRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveReportTargetRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'reportId')
    ..aOS(2, _omitFieldNames ? '' : 'messageId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportTargetRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportTargetRequest copyWith(
          void Function(ResolveReportTargetRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveReportTargetRequest))
          as ResolveReportTargetRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveReportTargetRequest create() => ResolveReportTargetRequest._();
  @$core.override
  ResolveReportTargetRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveReportTargetRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveReportTargetRequest>(create);
  static ResolveReportTargetRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get reportId => $_getSZ(0);
  @$pb.TagNumber(1)
  set reportId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasReportId() => $_has(0);
  @$pb.TagNumber(1)
  void clearReportId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get messageId => $_getSZ(1);
  @$pb.TagNumber(2)
  set messageId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMessageId() => $_has(1);
  @$pb.TagNumber(2)
  void clearMessageId() => $_clearField(2);
}

class ResolveReportTargetResponse extends $pb.GeneratedMessage {
  factory ResolveReportTargetResponse({
    $0.ReportTargetState? state,
  }) {
    final result = create();
    if (state != null) result.state = state;
    return result;
  }

  ResolveReportTargetResponse._();

  factory ResolveReportTargetResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolveReportTargetResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolveReportTargetResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.messaging.v1'),
      createEmptyInstance: create)
    ..aE<$0.ReportTargetState>(1, _omitFieldNames ? '' : 'state',
        enumValues: $0.ReportTargetState.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportTargetResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolveReportTargetResponse copyWith(
          void Function(ResolveReportTargetResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResolveReportTargetResponse))
          as ResolveReportTargetResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolveReportTargetResponse create() =>
      ResolveReportTargetResponse._();
  @$core.override
  ResolveReportTargetResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolveReportTargetResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolveReportTargetResponse>(create);
  static ResolveReportTargetResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $0.ReportTargetState get state => $_getN(0);
  @$pb.TagNumber(1)
  set state($0.ReportTargetState value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasState() => $_has(0);
  @$pb.TagNumber(1)
  void clearState() => $_clearField(1);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
