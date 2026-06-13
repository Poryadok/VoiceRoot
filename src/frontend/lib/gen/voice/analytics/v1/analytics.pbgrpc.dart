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
/// Name matches docs concept "AnalyticsIngest" via RPC prefix Ingest*.
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
