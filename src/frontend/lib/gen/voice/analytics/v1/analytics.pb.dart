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

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
