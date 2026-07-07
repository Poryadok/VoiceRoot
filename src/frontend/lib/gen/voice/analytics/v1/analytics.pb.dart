// This is a generated file - do not edit.
//
// Generated from voice/analytics/v1/analytics.proto.

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

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class IngestEventRequest extends $pb.GeneratedMessage {
  factory IngestEventRequest({
    AnalyticsEvent? event,
  }) {
    final result = create();
    if (event != null) result.event = event;
    return result;
  }

  IngestEventRequest._();

  factory IngestEventRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IngestEventRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IngestEventRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOM<AnalyticsEvent>(1, _omitFieldNames ? '' : 'event',
        subBuilder: AnalyticsEvent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestEventRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestEventRequest copyWith(void Function(IngestEventRequest) updates) =>
      super.copyWith((message) => updates(message as IngestEventRequest))
          as IngestEventRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IngestEventRequest create() => IngestEventRequest._();
  @$core.override
  IngestEventRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IngestEventRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IngestEventRequest>(create);
  static IngestEventRequest? _defaultInstance;

  @$pb.TagNumber(1)
  AnalyticsEvent get event => $_getN(0);
  @$pb.TagNumber(1)
  set event(AnalyticsEvent value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasEvent() => $_has(0);
  @$pb.TagNumber(1)
  void clearEvent() => $_clearField(1);
  @$pb.TagNumber(1)
  AnalyticsEvent ensureEvent() => $_ensure(0);
}

class IngestBatchRequest extends $pb.GeneratedMessage {
  factory IngestBatchRequest({
    $core.Iterable<AnalyticsEvent>? events,
  }) {
    final result = create();
    if (events != null) result.events.addAll(events);
    return result;
  }

  IngestBatchRequest._();

  factory IngestBatchRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IngestBatchRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IngestBatchRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..pPM<AnalyticsEvent>(1, _omitFieldNames ? '' : 'events',
        subBuilder: AnalyticsEvent.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestBatchRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestBatchRequest copyWith(void Function(IngestBatchRequest) updates) =>
      super.copyWith((message) => updates(message as IngestBatchRequest))
          as IngestBatchRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IngestBatchRequest create() => IngestBatchRequest._();
  @$core.override
  IngestBatchRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IngestBatchRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IngestBatchRequest>(create);
  static IngestBatchRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<AnalyticsEvent> get events => $_getList(0);
}

class IngestEventResponse extends $pb.GeneratedMessage {
  factory IngestEventResponse() => create();

  IngestEventResponse._();

  factory IngestEventResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IngestEventResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IngestEventResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestEventResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestEventResponse copyWith(void Function(IngestEventResponse) updates) =>
      super.copyWith((message) => updates(message as IngestEventResponse))
          as IngestEventResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IngestEventResponse create() => IngestEventResponse._();
  @$core.override
  IngestEventResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IngestEventResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IngestEventResponse>(create);
  static IngestEventResponse? _defaultInstance;
}

class IngestBatchResponse extends $pb.GeneratedMessage {
  factory IngestBatchResponse() => create();

  IngestBatchResponse._();

  factory IngestBatchResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory IngestBatchResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'IngestBatchResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestBatchResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  IngestBatchResponse copyWith(void Function(IngestBatchResponse) updates) =>
      super.copyWith((message) => updates(message as IngestBatchResponse))
          as IngestBatchResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static IngestBatchResponse create() => IngestBatchResponse._();
  @$core.override
  IngestBatchResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static IngestBatchResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<IngestBatchResponse>(create);
  static IngestBatchResponse? _defaultInstance;
}

class AnalyticsEvent extends $pb.GeneratedMessage {
  factory AnalyticsEvent({
    $core.String? eventId,
    $core.String? eventType,
    $core.String? sourceService,
    $1.Timestamp? timestamp,
    $core.String? propertiesJson,
    $core.String? sessionId,
    $core.String? platform,
    $core.String? appVersion,
    $core.String? region,
    $core.String? userIdHashed,
    $core.String? profileIdHashed,
  }) {
    final result = create();
    if (eventId != null) result.eventId = eventId;
    if (eventType != null) result.eventType = eventType;
    if (sourceService != null) result.sourceService = sourceService;
    if (timestamp != null) result.timestamp = timestamp;
    if (propertiesJson != null) result.propertiesJson = propertiesJson;
    if (sessionId != null) result.sessionId = sessionId;
    if (platform != null) result.platform = platform;
    if (appVersion != null) result.appVersion = appVersion;
    if (region != null) result.region = region;
    if (userIdHashed != null) result.userIdHashed = userIdHashed;
    if (profileIdHashed != null) result.profileIdHashed = profileIdHashed;
    return result;
  }

  AnalyticsEvent._();

  factory AnalyticsEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AnalyticsEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AnalyticsEvent',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'eventId')
    ..aOS(2, _omitFieldNames ? '' : 'eventType')
    ..aOS(3, _omitFieldNames ? '' : 'sourceService')
    ..aOM<$1.Timestamp>(4, _omitFieldNames ? '' : 'timestamp',
        subBuilder: $1.Timestamp.create)
    ..aOS(5, _omitFieldNames ? '' : 'propertiesJson')
    ..aOS(6, _omitFieldNames ? '' : 'sessionId')
    ..aOS(7, _omitFieldNames ? '' : 'platform')
    ..aOS(8, _omitFieldNames ? '' : 'appVersion')
    ..aOS(9, _omitFieldNames ? '' : 'region')
    ..aOS(10, _omitFieldNames ? '' : 'userIdHashed')
    ..aOS(11, _omitFieldNames ? '' : 'profileIdHashed')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AnalyticsEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AnalyticsEvent copyWith(void Function(AnalyticsEvent) updates) =>
      super.copyWith((message) => updates(message as AnalyticsEvent))
          as AnalyticsEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AnalyticsEvent create() => AnalyticsEvent._();
  @$core.override
  AnalyticsEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AnalyticsEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AnalyticsEvent>(create);
  static AnalyticsEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get eventId => $_getSZ(0);
  @$pb.TagNumber(1)
  set eventId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEventId() => $_has(0);
  @$pb.TagNumber(1)
  void clearEventId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get eventType => $_getSZ(1);
  @$pb.TagNumber(2)
  set eventType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasEventType() => $_has(1);
  @$pb.TagNumber(2)
  void clearEventType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get sourceService => $_getSZ(2);
  @$pb.TagNumber(3)
  set sourceService($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSourceService() => $_has(2);
  @$pb.TagNumber(3)
  void clearSourceService() => $_clearField(3);

  @$pb.TagNumber(4)
  $1.Timestamp get timestamp => $_getN(3);
  @$pb.TagNumber(4)
  set timestamp($1.Timestamp value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasTimestamp() => $_has(3);
  @$pb.TagNumber(4)
  void clearTimestamp() => $_clearField(4);
  @$pb.TagNumber(4)
  $1.Timestamp ensureTimestamp() => $_ensure(3);

  @$pb.TagNumber(5)
  $core.String get propertiesJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set propertiesJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasPropertiesJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearPropertiesJson() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get sessionId => $_getSZ(5);
  @$pb.TagNumber(6)
  set sessionId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasSessionId() => $_has(5);
  @$pb.TagNumber(6)
  void clearSessionId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get platform => $_getSZ(6);
  @$pb.TagNumber(7)
  set platform($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasPlatform() => $_has(6);
  @$pb.TagNumber(7)
  void clearPlatform() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get appVersion => $_getSZ(7);
  @$pb.TagNumber(8)
  set appVersion($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasAppVersion() => $_has(7);
  @$pb.TagNumber(8)
  void clearAppVersion() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get region => $_getSZ(8);
  @$pb.TagNumber(9)
  set region($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasRegion() => $_has(8);
  @$pb.TagNumber(9)
  void clearRegion() => $_clearField(9);

  @$pb.TagNumber(10)
  $core.String get userIdHashed => $_getSZ(9);
  @$pb.TagNumber(10)
  set userIdHashed($core.String value) => $_setString(9, value);
  @$pb.TagNumber(10)
  $core.bool hasUserIdHashed() => $_has(9);
  @$pb.TagNumber(10)
  void clearUserIdHashed() => $_clearField(10);

  @$pb.TagNumber(11)
  $core.String get profileIdHashed => $_getSZ(10);
  @$pb.TagNumber(11)
  set profileIdHashed($core.String value) => $_setString(10, value);
  @$pb.TagNumber(11)
  $core.bool hasProfileIdHashed() => $_has(10);
  @$pb.TagNumber(11)
  void clearProfileIdHashed() => $_clearField(11);
}

class MetricPoint extends $pb.GeneratedMessage {
  factory MetricPoint({
    $core.String? name,
    $core.double? value,
    $core.String? label,
  }) {
    final result = create();
    if (name != null) result.name = name;
    if (value != null) result.value = value;
    if (label != null) result.label = label;
    return result;
  }

  MetricPoint._();

  factory MetricPoint.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MetricPoint.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MetricPoint',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'name')
    ..aD(2, _omitFieldNames ? '' : 'value')
    ..aOS(3, _omitFieldNames ? '' : 'label')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricPoint clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MetricPoint copyWith(void Function(MetricPoint) updates) =>
      super.copyWith((message) => updates(message as MetricPoint))
          as MetricPoint;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MetricPoint create() => MetricPoint._();
  @$core.override
  MetricPoint createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MetricPoint getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MetricPoint>(create);
  static MetricPoint? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get name => $_getSZ(0);
  @$pb.TagNumber(1)
  set name($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasName() => $_has(0);
  @$pb.TagNumber(1)
  void clearName() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.double get value => $_getN(1);
  @$pb.TagNumber(2)
  set value($core.double value) => $_setDouble(1, value);
  @$pb.TagNumber(2)
  $core.bool hasValue() => $_has(1);
  @$pb.TagNumber(2)
  void clearValue() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get label => $_getSZ(2);
  @$pb.TagNumber(3)
  set label($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLabel() => $_has(2);
  @$pb.TagNumber(3)
  void clearLabel() => $_clearField(3);
}

class GetDashboardRequest extends $pb.GeneratedMessage {
  factory GetDashboardRequest({
    $core.String? dashboardType,
    $1.Timestamp? from,
    $1.Timestamp? to,
  }) {
    final result = create();
    if (dashboardType != null) result.dashboardType = dashboardType;
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    return result;
  }

  GetDashboardRequest._();

  factory GetDashboardRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDashboardRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDashboardRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'dashboardType')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'from',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'to',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDashboardRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDashboardRequest copyWith(void Function(GetDashboardRequest) updates) =>
      super.copyWith((message) => updates(message as GetDashboardRequest))
          as GetDashboardRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDashboardRequest create() => GetDashboardRequest._();
  @$core.override
  GetDashboardRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDashboardRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDashboardRequest>(create);
  static GetDashboardRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get dashboardType => $_getSZ(0);
  @$pb.TagNumber(1)
  set dashboardType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDashboardType() => $_has(0);
  @$pb.TagNumber(1)
  void clearDashboardType() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get from => $_getN(1);
  @$pb.TagNumber(2)
  set from($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFrom() => $_has(1);
  @$pb.TagNumber(2)
  void clearFrom() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureFrom() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.Timestamp get to => $_getN(2);
  @$pb.TagNumber(3)
  set to($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasTo() => $_has(2);
  @$pb.TagNumber(3)
  void clearTo() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureTo() => $_ensure(2);
}

class GetDashboardResponse extends $pb.GeneratedMessage {
  factory GetDashboardResponse({
    $core.String? dashboardType,
    $core.Iterable<MetricPoint>? metrics,
  }) {
    final result = create();
    if (dashboardType != null) result.dashboardType = dashboardType;
    if (metrics != null) result.metrics.addAll(metrics);
    return result;
  }

  GetDashboardResponse._();

  factory GetDashboardResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetDashboardResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetDashboardResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'dashboardType')
    ..pPM<MetricPoint>(2, _omitFieldNames ? '' : 'metrics',
        subBuilder: MetricPoint.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDashboardResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetDashboardResponse copyWith(void Function(GetDashboardResponse) updates) =>
      super.copyWith((message) => updates(message as GetDashboardResponse))
          as GetDashboardResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetDashboardResponse create() => GetDashboardResponse._();
  @$core.override
  GetDashboardResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetDashboardResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetDashboardResponse>(create);
  static GetDashboardResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get dashboardType => $_getSZ(0);
  @$pb.TagNumber(1)
  set dashboardType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasDashboardType() => $_has(0);
  @$pb.TagNumber(1)
  void clearDashboardType() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<MetricPoint> get metrics => $_getList(1);
}

class GetMetricsRequest extends $pb.GeneratedMessage {
  factory GetMetricsRequest({
    $core.String? metric,
    $1.Timestamp? from,
    $1.Timestamp? to,
    $core.Iterable<$core.MapEntry<$core.String, $core.String>>? filters,
  }) {
    final result = create();
    if (metric != null) result.metric = metric;
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    if (filters != null) result.filters.addEntries(filters);
    return result;
  }

  GetMetricsRequest._();

  factory GetMetricsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMetricsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMetricsRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'metric')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'from',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'to',
        subBuilder: $1.Timestamp.create)
    ..m<$core.String, $core.String>(4, _omitFieldNames ? '' : 'filters',
        entryClassName: 'GetMetricsRequest.FiltersEntry',
        keyFieldType: $pb.PbFieldType.OS,
        valueFieldType: $pb.PbFieldType.OS,
        packageName: const $pb.PackageName('voice.analytics.v1'))
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMetricsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMetricsRequest copyWith(void Function(GetMetricsRequest) updates) =>
      super.copyWith((message) => updates(message as GetMetricsRequest))
          as GetMetricsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMetricsRequest create() => GetMetricsRequest._();
  @$core.override
  GetMetricsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMetricsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMetricsRequest>(create);
  static GetMetricsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get metric => $_getSZ(0);
  @$pb.TagNumber(1)
  set metric($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMetric() => $_has(0);
  @$pb.TagNumber(1)
  void clearMetric() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get from => $_getN(1);
  @$pb.TagNumber(2)
  set from($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFrom() => $_has(1);
  @$pb.TagNumber(2)
  void clearFrom() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureFrom() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.Timestamp get to => $_getN(2);
  @$pb.TagNumber(3)
  set to($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasTo() => $_has(2);
  @$pb.TagNumber(3)
  void clearTo() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureTo() => $_ensure(2);

  @$pb.TagNumber(4)
  $pb.PbMap<$core.String, $core.String> get filters => $_getMap(3);
}

class GetMetricsResponse extends $pb.GeneratedMessage {
  factory GetMetricsResponse({
    $core.Iterable<MetricPoint>? points,
  }) {
    final result = create();
    if (points != null) result.points.addAll(points);
    return result;
  }

  GetMetricsResponse._();

  factory GetMetricsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetMetricsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetMetricsResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..pPM<MetricPoint>(1, _omitFieldNames ? '' : 'points',
        subBuilder: MetricPoint.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMetricsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetMetricsResponse copyWith(void Function(GetMetricsResponse) updates) =>
      super.copyWith((message) => updates(message as GetMetricsResponse))
          as GetMetricsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetMetricsResponse create() => GetMetricsResponse._();
  @$core.override
  GetMetricsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetMetricsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetMetricsResponse>(create);
  static GetMetricsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<MetricPoint> get points => $_getList(0);
}

class GetFunnelRequest extends $pb.GeneratedMessage {
  factory GetFunnelRequest({
    $core.String? funnelName,
    $1.Timestamp? from,
    $1.Timestamp? to,
  }) {
    final result = create();
    if (funnelName != null) result.funnelName = funnelName;
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    return result;
  }

  GetFunnelRequest._();

  factory GetFunnelRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFunnelRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFunnelRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'funnelName')
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'from',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'to',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFunnelRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFunnelRequest copyWith(void Function(GetFunnelRequest) updates) =>
      super.copyWith((message) => updates(message as GetFunnelRequest))
          as GetFunnelRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFunnelRequest create() => GetFunnelRequest._();
  @$core.override
  GetFunnelRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFunnelRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFunnelRequest>(create);
  static GetFunnelRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get funnelName => $_getSZ(0);
  @$pb.TagNumber(1)
  set funnelName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFunnelName() => $_has(0);
  @$pb.TagNumber(1)
  void clearFunnelName() => $_clearField(1);

  @$pb.TagNumber(2)
  $1.Timestamp get from => $_getN(1);
  @$pb.TagNumber(2)
  set from($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasFrom() => $_has(1);
  @$pb.TagNumber(2)
  void clearFrom() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureFrom() => $_ensure(1);

  @$pb.TagNumber(3)
  $1.Timestamp get to => $_getN(2);
  @$pb.TagNumber(3)
  set to($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasTo() => $_has(2);
  @$pb.TagNumber(3)
  void clearTo() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureTo() => $_ensure(2);
}

class FunnelStep extends $pb.GeneratedMessage {
  factory FunnelStep({
    $core.String? step,
    $fixnum.Int64? count,
  }) {
    final result = create();
    if (step != null) result.step = step;
    if (count != null) result.count = count;
    return result;
  }

  FunnelStep._();

  factory FunnelStep.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory FunnelStep.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'FunnelStep',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'step')
    ..aInt64(2, _omitFieldNames ? '' : 'count')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FunnelStep clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  FunnelStep copyWith(void Function(FunnelStep) updates) =>
      super.copyWith((message) => updates(message as FunnelStep)) as FunnelStep;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static FunnelStep create() => FunnelStep._();
  @$core.override
  FunnelStep createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static FunnelStep getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<FunnelStep>(create);
  static FunnelStep? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get step => $_getSZ(0);
  @$pb.TagNumber(1)
  set step($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasStep() => $_has(0);
  @$pb.TagNumber(1)
  void clearStep() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get count => $_getI64(1);
  @$pb.TagNumber(2)
  set count($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCount() => $_has(1);
  @$pb.TagNumber(2)
  void clearCount() => $_clearField(2);
}

class GetFunnelResponse extends $pb.GeneratedMessage {
  factory GetFunnelResponse({
    $core.String? funnelName,
    $core.Iterable<FunnelStep>? steps,
  }) {
    final result = create();
    if (funnelName != null) result.funnelName = funnelName;
    if (steps != null) result.steps.addAll(steps);
    return result;
  }

  GetFunnelResponse._();

  factory GetFunnelResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetFunnelResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetFunnelResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'funnelName')
    ..pPM<FunnelStep>(2, _omitFieldNames ? '' : 'steps',
        subBuilder: FunnelStep.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFunnelResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetFunnelResponse copyWith(void Function(GetFunnelResponse) updates) =>
      super.copyWith((message) => updates(message as GetFunnelResponse))
          as GetFunnelResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetFunnelResponse create() => GetFunnelResponse._();
  @$core.override
  GetFunnelResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetFunnelResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetFunnelResponse>(create);
  static GetFunnelResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get funnelName => $_getSZ(0);
  @$pb.TagNumber(1)
  set funnelName($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasFunnelName() => $_has(0);
  @$pb.TagNumber(1)
  void clearFunnelName() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<FunnelStep> get steps => $_getList(1);
}

class GetRetentionRequest extends $pb.GeneratedMessage {
  factory GetRetentionRequest({
    $1.Timestamp? cohortFrom,
    $1.Timestamp? cohortTo,
  }) {
    final result = create();
    if (cohortFrom != null) result.cohortFrom = cohortFrom;
    if (cohortTo != null) result.cohortTo = cohortTo;
    return result;
  }

  GetRetentionRequest._();

  factory GetRetentionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetRetentionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRetentionRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOM<$1.Timestamp>(1, _omitFieldNames ? '' : 'cohortFrom',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'cohortTo',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRetentionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRetentionRequest copyWith(void Function(GetRetentionRequest) updates) =>
      super.copyWith((message) => updates(message as GetRetentionRequest))
          as GetRetentionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetRetentionRequest create() => GetRetentionRequest._();
  @$core.override
  GetRetentionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetRetentionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRetentionRequest>(create);
  static GetRetentionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.Timestamp get cohortFrom => $_getN(0);
  @$pb.TagNumber(1)
  set cohortFrom($1.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCohortFrom() => $_has(0);
  @$pb.TagNumber(1)
  void clearCohortFrom() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Timestamp ensureCohortFrom() => $_ensure(0);

  @$pb.TagNumber(2)
  $1.Timestamp get cohortTo => $_getN(1);
  @$pb.TagNumber(2)
  set cohortTo($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCohortTo() => $_has(1);
  @$pb.TagNumber(2)
  void clearCohortTo() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureCohortTo() => $_ensure(1);
}

class RetentionCohort extends $pb.GeneratedMessage {
  factory RetentionCohort({
    $core.String? cohortDate,
    $fixnum.Int64? cohortSize,
    $core.double? d1,
    $core.double? d7,
    $core.double? d30,
  }) {
    final result = create();
    if (cohortDate != null) result.cohortDate = cohortDate;
    if (cohortSize != null) result.cohortSize = cohortSize;
    if (d1 != null) result.d1 = d1;
    if (d7 != null) result.d7 = d7;
    if (d30 != null) result.d30 = d30;
    return result;
  }

  RetentionCohort._();

  factory RetentionCohort.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RetentionCohort.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RetentionCohort',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'cohortDate')
    ..aInt64(2, _omitFieldNames ? '' : 'cohortSize')
    ..aD(3, _omitFieldNames ? '' : 'd1')
    ..aD(4, _omitFieldNames ? '' : 'd7')
    ..aD(5, _omitFieldNames ? '' : 'd30')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RetentionCohort clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RetentionCohort copyWith(void Function(RetentionCohort) updates) =>
      super.copyWith((message) => updates(message as RetentionCohort))
          as RetentionCohort;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RetentionCohort create() => RetentionCohort._();
  @$core.override
  RetentionCohort createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RetentionCohort getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RetentionCohort>(create);
  static RetentionCohort? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get cohortDate => $_getSZ(0);
  @$pb.TagNumber(1)
  set cohortDate($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCohortDate() => $_has(0);
  @$pb.TagNumber(1)
  void clearCohortDate() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get cohortSize => $_getI64(1);
  @$pb.TagNumber(2)
  set cohortSize($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCohortSize() => $_has(1);
  @$pb.TagNumber(2)
  void clearCohortSize() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.double get d1 => $_getN(2);
  @$pb.TagNumber(3)
  set d1($core.double value) => $_setDouble(2, value);
  @$pb.TagNumber(3)
  $core.bool hasD1() => $_has(2);
  @$pb.TagNumber(3)
  void clearD1() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.double get d7 => $_getN(3);
  @$pb.TagNumber(4)
  set d7($core.double value) => $_setDouble(3, value);
  @$pb.TagNumber(4)
  $core.bool hasD7() => $_has(3);
  @$pb.TagNumber(4)
  void clearD7() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.double get d30 => $_getN(4);
  @$pb.TagNumber(5)
  set d30($core.double value) => $_setDouble(4, value);
  @$pb.TagNumber(5)
  $core.bool hasD30() => $_has(4);
  @$pb.TagNumber(5)
  void clearD30() => $_clearField(5);
}

class GetRetentionResponse extends $pb.GeneratedMessage {
  factory GetRetentionResponse({
    $core.Iterable<RetentionCohort>? cohorts,
  }) {
    final result = create();
    if (cohorts != null) result.cohorts.addAll(cohorts);
    return result;
  }

  GetRetentionResponse._();

  factory GetRetentionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetRetentionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetRetentionResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..pPM<RetentionCohort>(1, _omitFieldNames ? '' : 'cohorts',
        subBuilder: RetentionCohort.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRetentionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetRetentionResponse copyWith(void Function(GetRetentionResponse) updates) =>
      super.copyWith((message) => updates(message as GetRetentionResponse))
          as GetRetentionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetRetentionResponse create() => GetRetentionResponse._();
  @$core.override
  GetRetentionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetRetentionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetRetentionResponse>(create);
  static GetRetentionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<RetentionCohort> get cohorts => $_getList(0);
}

class ExportDataRequest extends $pb.GeneratedMessage {
  factory ExportDataRequest({
    $1.Timestamp? from,
    $1.Timestamp? to,
    $core.String? eventType,
    $core.String? format,
  }) {
    final result = create();
    if (from != null) result.from = from;
    if (to != null) result.to = to;
    if (eventType != null) result.eventType = eventType;
    if (format != null) result.format = format;
    return result;
  }

  ExportDataRequest._();

  factory ExportDataRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ExportDataRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExportDataRequest',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOM<$1.Timestamp>(1, _omitFieldNames ? '' : 'from',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(2, _omitFieldNames ? '' : 'to',
        subBuilder: $1.Timestamp.create)
    ..aOS(3, _omitFieldNames ? '' : 'eventType')
    ..aOS(4, _omitFieldNames ? '' : 'format')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportDataRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportDataRequest copyWith(void Function(ExportDataRequest) updates) =>
      super.copyWith((message) => updates(message as ExportDataRequest))
          as ExportDataRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ExportDataRequest create() => ExportDataRequest._();
  @$core.override
  ExportDataRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ExportDataRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExportDataRequest>(create);
  static ExportDataRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $1.Timestamp get from => $_getN(0);
  @$pb.TagNumber(1)
  set from($1.Timestamp value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasFrom() => $_has(0);
  @$pb.TagNumber(1)
  void clearFrom() => $_clearField(1);
  @$pb.TagNumber(1)
  $1.Timestamp ensureFrom() => $_ensure(0);

  @$pb.TagNumber(2)
  $1.Timestamp get to => $_getN(1);
  @$pb.TagNumber(2)
  set to($1.Timestamp value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasTo() => $_has(1);
  @$pb.TagNumber(2)
  void clearTo() => $_clearField(2);
  @$pb.TagNumber(2)
  $1.Timestamp ensureTo() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get eventType => $_getSZ(2);
  @$pb.TagNumber(3)
  set eventType($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasEventType() => $_has(2);
  @$pb.TagNumber(3)
  void clearEventType() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get format => $_getSZ(3);
  @$pb.TagNumber(4)
  set format($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasFormat() => $_has(3);
  @$pb.TagNumber(4)
  void clearFormat() => $_clearField(4);
}

class ExportDataResponse extends $pb.GeneratedMessage {
  factory ExportDataResponse({
    $core.String? contentType,
    $core.List<$core.int>? body,
  }) {
    final result = create();
    if (contentType != null) result.contentType = contentType;
    if (body != null) result.body = body;
    return result;
  }

  ExportDataResponse._();

  factory ExportDataResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ExportDataResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ExportDataResponse',
      package:
          const $pb.PackageName(_omitMessageNames ? '' : 'voice.analytics.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'contentType')
    ..a<$core.List<$core.int>>(
        2, _omitFieldNames ? '' : 'body', $pb.PbFieldType.OY)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportDataResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ExportDataResponse copyWith(void Function(ExportDataResponse) updates) =>
      super.copyWith((message) => updates(message as ExportDataResponse))
          as ExportDataResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ExportDataResponse create() => ExportDataResponse._();
  @$core.override
  ExportDataResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ExportDataResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ExportDataResponse>(create);
  static ExportDataResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get contentType => $_getSZ(0);
  @$pb.TagNumber(1)
  set contentType($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasContentType() => $_has(0);
  @$pb.TagNumber(1)
  void clearContentType() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.List<$core.int> get body => $_getN(1);
  @$pb.TagNumber(2)
  set body($core.List<$core.int> value) => $_setBytes(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBody() => $_has(1);
  @$pb.TagNumber(2)
  void clearBody() => $_clearField(2);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
