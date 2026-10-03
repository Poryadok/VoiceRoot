// This is a generated file - do not edit.
//
// Generated from voice/authority/v1/authority.proto.

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

import 'authority.pb.dart' as $0;

export 'authority.pb.dart';

/// Internal owner reads. No public Gateway route or game-node caller is allowed.
/// Each server verifies its own audience and exact signed request binding.
@$pb.GrpcServiceName('voice.authority.v1.AuthoritySourceService')
class AuthoritySourceServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthoritySourceServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:federation
  $grpc.ResponseFuture<$0.ReadSnapshotResponse> readSnapshot(
    $0.ReadSnapshotRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readSnapshot, request, options: options);
  }

  /// @voice.security=protected;callers=service:federation
  $grpc.ResponseFuture<$0.ReadRevisionResponse> readRevision(
    $0.ReadRevisionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readRevision, request, options: options);
  }

  // method descriptors

  static final _$readSnapshot =
      $grpc.ClientMethod<$0.ReadSnapshotRequest, $0.ReadSnapshotResponse>(
          '/voice.authority.v1.AuthoritySourceService/ReadSnapshot',
          ($0.ReadSnapshotRequest value) => value.writeToBuffer(),
          $0.ReadSnapshotResponse.fromBuffer);
  static final _$readRevision =
      $grpc.ClientMethod<$0.ReadRevisionRequest, $0.ReadRevisionResponse>(
          '/voice.authority.v1.AuthoritySourceService/ReadRevision',
          ($0.ReadRevisionRequest value) => value.writeToBuffer(),
          $0.ReadRevisionResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.authority.v1.AuthoritySourceService')
abstract class AuthoritySourceServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.authority.v1.AuthoritySourceService';

  AuthoritySourceServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.ReadSnapshotRequest, $0.ReadSnapshotResponse>(
            'ReadSnapshot',
            readSnapshot_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReadSnapshotRequest.fromBuffer(value),
            ($0.ReadSnapshotResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReadRevisionRequest, $0.ReadRevisionResponse>(
            'ReadRevision',
            readRevision_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReadRevisionRequest.fromBuffer(value),
            ($0.ReadRevisionResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ReadSnapshotResponse> readSnapshot_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReadSnapshotRequest> $request) async {
    return readSnapshot($call, await $request);
  }

  $async.Future<$0.ReadSnapshotResponse> readSnapshot(
      $grpc.ServiceCall call, $0.ReadSnapshotRequest request);

  $async.Future<$0.ReadRevisionResponse> readRevision_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReadRevisionRequest> $request) async {
    return readRevision($call, await $request);
  }

  $async.Future<$0.ReadRevisionResponse> readRevision(
      $grpc.ServiceCall call, $0.ReadRevisionRequest request);
}
