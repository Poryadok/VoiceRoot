// This is a generated file - do not edit.
//
// Generated from voice/realtime/v1/realtime.proto.

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

import 'realtime.pb.dart' as $0;

export 'realtime.pb.dart';

/// No public route. Dedicated Space-workload mTLS listener only.
@$pb.GrpcServiceName('voice.realtime.v1.RealtimeLifecycleService')
class RealtimeLifecycleServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  RealtimeLifecycleServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage(
    $0.ImportSpacePurgeManifestPageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$importSpacePurgeManifestPage, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence(
    $0.ApplySpaceLifecycleFenceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applySpaceLifecycleFence, request,
        options: options);
  }

  /// @voice.security=protected;callers=service:space
  $grpc.ResponseFuture<$0.PurgeSpaceResponse> purgeSpace(
    $0.PurgeSpaceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$purgeSpace, request, options: options);
  }

  // method descriptors

  static final _$importSpacePurgeManifestPage = $grpc.ClientMethod<
          $0.ImportSpacePurgeManifestPageRequest,
          $0.ImportSpacePurgeManifestPageResponse>(
      '/voice.realtime.v1.RealtimeLifecycleService/ImportSpacePurgeManifestPage',
      ($0.ImportSpacePurgeManifestPageRequest value) => value.writeToBuffer(),
      $0.ImportSpacePurgeManifestPageResponse.fromBuffer);
  static final _$applySpaceLifecycleFence = $grpc.ClientMethod<
          $0.ApplySpaceLifecycleFenceRequest,
          $0.ApplySpaceLifecycleFenceResponse>(
      '/voice.realtime.v1.RealtimeLifecycleService/ApplySpaceLifecycleFence',
      ($0.ApplySpaceLifecycleFenceRequest value) => value.writeToBuffer(),
      $0.ApplySpaceLifecycleFenceResponse.fromBuffer);
  static final _$purgeSpace =
      $grpc.ClientMethod<$0.PurgeSpaceRequest, $0.PurgeSpaceResponse>(
          '/voice.realtime.v1.RealtimeLifecycleService/PurgeSpace',
          ($0.PurgeSpaceRequest value) => value.writeToBuffer(),
          $0.PurgeSpaceResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.realtime.v1.RealtimeLifecycleService')
abstract class RealtimeLifecycleServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.realtime.v1.RealtimeLifecycleService';

  RealtimeLifecycleServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ImportSpacePurgeManifestPageRequest,
            $0.ImportSpacePurgeManifestPageResponse>(
        'ImportSpacePurgeManifestPage',
        importSpacePurgeManifestPage_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ImportSpacePurgeManifestPageRequest.fromBuffer(value),
        ($0.ImportSpacePurgeManifestPageResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ApplySpaceLifecycleFenceRequest,
            $0.ApplySpaceLifecycleFenceResponse>(
        'ApplySpaceLifecycleFence',
        applySpaceLifecycleFence_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ApplySpaceLifecycleFenceRequest.fromBuffer(value),
        ($0.ApplySpaceLifecycleFenceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PurgeSpaceRequest, $0.PurgeSpaceResponse>(
        'PurgeSpace',
        purgeSpace_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.PurgeSpaceRequest.fromBuffer(value),
        ($0.PurgeSpaceResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ImportSpacePurgeManifestPageRequest>
              $request) async {
    return importSpacePurgeManifestPage($call, await $request);
  }

  $async.Future<$0.ImportSpacePurgeManifestPageResponse>
      importSpacePurgeManifestPage($grpc.ServiceCall call,
          $0.ImportSpacePurgeManifestPageRequest request);

  $async.Future<$0.ApplySpaceLifecycleFenceResponse>
      applySpaceLifecycleFence_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ApplySpaceLifecycleFenceRequest> $request) async {
    return applySpaceLifecycleFence($call, await $request);
  }

  $async.Future<$0.ApplySpaceLifecycleFenceResponse> applySpaceLifecycleFence(
      $grpc.ServiceCall call, $0.ApplySpaceLifecycleFenceRequest request);

  $async.Future<$0.PurgeSpaceResponse> purgeSpace_Pre($grpc.ServiceCall $call,
      $async.Future<$0.PurgeSpaceRequest> $request) async {
    return purgeSpace($call, await $request);
  }

  $async.Future<$0.PurgeSpaceResponse> purgeSpace(
      $grpc.ServiceCall call, $0.PurgeSpaceRequest request);
}
