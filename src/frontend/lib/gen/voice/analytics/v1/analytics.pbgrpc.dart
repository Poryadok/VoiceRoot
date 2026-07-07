// This is a generated file - do not edit.
//
// Generated from voice/analytics/v1/analytics.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:async' as $async;
import 'dart:core' as $core;

import 'package:grpc/service_api.dart' as $grpc;
import 'package:protobuf/protobuf.dart' as $pb;

import 'analytics.pb.dart' as $0;

export 'analytics.pb.dart';

/// Internal ingest (NATS consumer / rare gRPC). Admin REST via Gateway: /api/v1/analytics/**.
@$pb.GrpcServiceName('voice.analytics.v1.AnalyticsIngestService')
class AnalyticsIngestServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AnalyticsIngestServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.IngestEventResponse> ingestEvent(
    $0.IngestEventRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$ingestEvent, request, options: options);
  }

  $grpc.ResponseFuture<$0.IngestBatchResponse> ingestBatch(
    $0.IngestBatchRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$ingestBatch, request, options: options);
  }

  // method descriptors

  static final _$ingestEvent =
      $grpc.ClientMethod<$0.IngestEventRequest, $0.IngestEventResponse>(
          '/voice.analytics.v1.AnalyticsIngestService/IngestEvent',
          ($0.IngestEventRequest value) => value.writeToBuffer(),
          $0.IngestEventResponse.fromBuffer);
  static final _$ingestBatch =
      $grpc.ClientMethod<$0.IngestBatchRequest, $0.IngestBatchResponse>(
          '/voice.analytics.v1.AnalyticsIngestService/IngestBatch',
          ($0.IngestBatchRequest value) => value.writeToBuffer(),
          $0.IngestBatchResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.analytics.v1.AnalyticsIngestService')
abstract class AnalyticsIngestServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.analytics.v1.AnalyticsIngestService';

  AnalyticsIngestServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.IngestEventRequest, $0.IngestEventResponse>(
            'IngestEvent',
            ingestEvent_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.IngestEventRequest.fromBuffer(value),
            ($0.IngestEventResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.IngestBatchRequest, $0.IngestBatchResponse>(
            'IngestBatch',
            ingestBatch_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.IngestBatchRequest.fromBuffer(value),
            ($0.IngestBatchResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.IngestEventResponse> ingestEvent_Pre($grpc.ServiceCall $call,
      $async.Future<$0.IngestEventRequest> $request) async {
    return ingestEvent($call, await $request);
  }

  $async.Future<$0.IngestEventResponse> ingestEvent(
      $grpc.ServiceCall call, $0.IngestEventRequest request);

  $async.Future<$0.IngestBatchResponse> ingestBatch_Pre($grpc.ServiceCall $call,
      $async.Future<$0.IngestBatchRequest> $request) async {
    return ingestBatch($call, await $request);
  }

  $async.Future<$0.IngestBatchResponse> ingestBatch(
      $grpc.ServiceCall call, $0.IngestBatchRequest request);
}

/// Staff-only query API (Gateway transcoding).
@$pb.GrpcServiceName('voice.analytics.v1.AnalyticsQueryService')
class AnalyticsQueryServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AnalyticsQueryServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.GetDashboardResponse> getDashboard(
    $0.GetDashboardRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getDashboard, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetMetricsResponse> getMetrics(
    $0.GetMetricsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getMetrics, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetFunnelResponse> getFunnel(
    $0.GetFunnelRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getFunnel, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetRetentionResponse> getRetention(
    $0.GetRetentionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getRetention, request, options: options);
  }

  $grpc.ResponseFuture<$0.ExportDataResponse> exportData(
    $0.ExportDataRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$exportData, request, options: options);
  }

  // method descriptors

  static final _$getDashboard =
      $grpc.ClientMethod<$0.GetDashboardRequest, $0.GetDashboardResponse>(
          '/voice.analytics.v1.AnalyticsQueryService/GetDashboard',
          ($0.GetDashboardRequest value) => value.writeToBuffer(),
          $0.GetDashboardResponse.fromBuffer);
  static final _$getMetrics =
      $grpc.ClientMethod<$0.GetMetricsRequest, $0.GetMetricsResponse>(
          '/voice.analytics.v1.AnalyticsQueryService/GetMetrics',
          ($0.GetMetricsRequest value) => value.writeToBuffer(),
          $0.GetMetricsResponse.fromBuffer);
  static final _$getFunnel =
      $grpc.ClientMethod<$0.GetFunnelRequest, $0.GetFunnelResponse>(
          '/voice.analytics.v1.AnalyticsQueryService/GetFunnel',
          ($0.GetFunnelRequest value) => value.writeToBuffer(),
          $0.GetFunnelResponse.fromBuffer);
  static final _$getRetention =
      $grpc.ClientMethod<$0.GetRetentionRequest, $0.GetRetentionResponse>(
          '/voice.analytics.v1.AnalyticsQueryService/GetRetention',
          ($0.GetRetentionRequest value) => value.writeToBuffer(),
          $0.GetRetentionResponse.fromBuffer);
  static final _$exportData =
      $grpc.ClientMethod<$0.ExportDataRequest, $0.ExportDataResponse>(
          '/voice.analytics.v1.AnalyticsQueryService/ExportData',
          ($0.ExportDataRequest value) => value.writeToBuffer(),
          $0.ExportDataResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.analytics.v1.AnalyticsQueryService')
abstract class AnalyticsQueryServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.analytics.v1.AnalyticsQueryService';

  AnalyticsQueryServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.GetDashboardRequest, $0.GetDashboardResponse>(
            'GetDashboard',
            getDashboard_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetDashboardRequest.fromBuffer(value),
            ($0.GetDashboardResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetMetricsRequest, $0.GetMetricsResponse>(
        'GetMetrics',
        getMetrics_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetMetricsRequest.fromBuffer(value),
        ($0.GetMetricsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetFunnelRequest, $0.GetFunnelResponse>(
        'GetFunnel',
        getFunnel_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetFunnelRequest.fromBuffer(value),
        ($0.GetFunnelResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetRetentionRequest, $0.GetRetentionResponse>(
            'GetRetention',
            getRetention_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetRetentionRequest.fromBuffer(value),
            ($0.GetRetentionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ExportDataRequest, $0.ExportDataResponse>(
        'ExportData',
        exportData_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ExportDataRequest.fromBuffer(value),
        ($0.ExportDataResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.GetDashboardResponse> getDashboard_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetDashboardRequest> $request) async {
    return getDashboard($call, await $request);
  }

  $async.Future<$0.GetDashboardResponse> getDashboard(
      $grpc.ServiceCall call, $0.GetDashboardRequest request);

  $async.Future<$0.GetMetricsResponse> getMetrics_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetMetricsRequest> $request) async {
    return getMetrics($call, await $request);
  }

  $async.Future<$0.GetMetricsResponse> getMetrics(
      $grpc.ServiceCall call, $0.GetMetricsRequest request);

  $async.Future<$0.GetFunnelResponse> getFunnel_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetFunnelRequest> $request) async {
    return getFunnel($call, await $request);
  }

  $async.Future<$0.GetFunnelResponse> getFunnel(
      $grpc.ServiceCall call, $0.GetFunnelRequest request);

  $async.Future<$0.GetRetentionResponse> getRetention_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetRetentionRequest> $request) async {
    return getRetention($call, await $request);
  }

  $async.Future<$0.GetRetentionResponse> getRetention(
      $grpc.ServiceCall call, $0.GetRetentionRequest request);

  $async.Future<$0.ExportDataResponse> exportData_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ExportDataRequest> $request) async {
    return exportData($call, await $request);
  }

  $async.Future<$0.ExportDataResponse> exportData(
      $grpc.ServiceCall call, $0.ExportDataRequest request);
}
