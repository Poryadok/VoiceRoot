// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/federation_management.proto.

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

import 'federation_management.pb.dart' as $0;

export 'federation_management.pb.dart';

/// Control-plane API for federation nodes (master-side operators / automation; mTLS).
/// Complements node↔master data plane in s2s.proto (same package).
@$pb.GrpcServiceName('voice.s2s.v1.FederationManagementService')
class FederationManagementServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  FederationManagementServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterNodeResponse> registerNode(
    $0.RegisterNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerNode, request, options: options);
  }

  $grpc.ResponseFuture<$0.ApproveNodeResponse> approveNode(
    $0.ApproveNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$approveNode, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeactivateNodeResponse> deactivateNode(
    $0.DeactivateNodeRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deactivateNode, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListNodesResponse> listNodes(
    $0.ListNodesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listNodes, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetNodeStatusResponse> getNodeStatus(
    $0.GetNodeStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getNodeStatus, request, options: options);
  }

  $grpc.ResponseFuture<$0.DefederateResponse> defederate(
    $0.DefederateRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$defederate, request, options: options);
  }

  // method descriptors

  static final _$registerNode =
      $grpc.ClientMethod<$0.RegisterNodeRequest, $0.RegisterNodeResponse>(
          '/voice.s2s.v1.FederationManagementService/RegisterNode',
          ($0.RegisterNodeRequest value) => value.writeToBuffer(),
          $0.RegisterNodeResponse.fromBuffer);
  static final _$approveNode =
      $grpc.ClientMethod<$0.ApproveNodeRequest, $0.ApproveNodeResponse>(
          '/voice.s2s.v1.FederationManagementService/ApproveNode',
          ($0.ApproveNodeRequest value) => value.writeToBuffer(),
          $0.ApproveNodeResponse.fromBuffer);
  static final _$deactivateNode =
      $grpc.ClientMethod<$0.DeactivateNodeRequest, $0.DeactivateNodeResponse>(
          '/voice.s2s.v1.FederationManagementService/DeactivateNode',
          ($0.DeactivateNodeRequest value) => value.writeToBuffer(),
          $0.DeactivateNodeResponse.fromBuffer);
  static final _$listNodes =
      $grpc.ClientMethod<$0.ListNodesRequest, $0.ListNodesResponse>(
          '/voice.s2s.v1.FederationManagementService/ListNodes',
          ($0.ListNodesRequest value) => value.writeToBuffer(),
          $0.ListNodesResponse.fromBuffer);
  static final _$getNodeStatus =
      $grpc.ClientMethod<$0.GetNodeStatusRequest, $0.GetNodeStatusResponse>(
          '/voice.s2s.v1.FederationManagementService/GetNodeStatus',
          ($0.GetNodeStatusRequest value) => value.writeToBuffer(),
          $0.GetNodeStatusResponse.fromBuffer);
  static final _$defederate =
      $grpc.ClientMethod<$0.DefederateRequest, $0.DefederateResponse>(
          '/voice.s2s.v1.FederationManagementService/Defederate',
          ($0.DefederateRequest value) => value.writeToBuffer(),
          $0.DefederateResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.s2s.v1.FederationManagementService')
abstract class FederationManagementServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.s2s.v1.FederationManagementService';

  FederationManagementServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.RegisterNodeRequest, $0.RegisterNodeResponse>(
            'RegisterNode',
            registerNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RegisterNodeRequest.fromBuffer(value),
            ($0.RegisterNodeResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ApproveNodeRequest, $0.ApproveNodeResponse>(
            'ApproveNode',
            approveNode_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ApproveNodeRequest.fromBuffer(value),
            ($0.ApproveNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DeactivateNodeRequest,
            $0.DeactivateNodeResponse>(
        'DeactivateNode',
        deactivateNode_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.DeactivateNodeRequest.fromBuffer(value),
        ($0.DeactivateNodeResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ListNodesRequest, $0.ListNodesResponse>(
        'ListNodes',
        listNodes_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.ListNodesRequest.fromBuffer(value),
        ($0.ListNodesResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.GetNodeStatusRequest, $0.GetNodeStatusResponse>(
            'GetNodeStatus',
            getNodeStatus_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.GetNodeStatusRequest.fromBuffer(value),
            ($0.GetNodeStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.DefederateRequest, $0.DefederateResponse>(
        'Defederate',
        defederate_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.DefederateRequest.fromBuffer(value),
        ($0.DefederateResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.RegisterNodeResponse> registerNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RegisterNodeRequest> $request) async {
    return registerNode($call, await $request);
  }

  $async.Future<$0.RegisterNodeResponse> registerNode(
      $grpc.ServiceCall call, $0.RegisterNodeRequest request);

  $async.Future<$0.ApproveNodeResponse> approveNode_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ApproveNodeRequest> $request) async {
    return approveNode($call, await $request);
  }

  $async.Future<$0.ApproveNodeResponse> approveNode(
      $grpc.ServiceCall call, $0.ApproveNodeRequest request);

  $async.Future<$0.DeactivateNodeResponse> deactivateNode_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeactivateNodeRequest> $request) async {
    return deactivateNode($call, await $request);
  }

  $async.Future<$0.DeactivateNodeResponse> deactivateNode(
      $grpc.ServiceCall call, $0.DeactivateNodeRequest request);

  $async.Future<$0.ListNodesResponse> listNodes_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListNodesRequest> $request) async {
    return listNodes($call, await $request);
  }

  $async.Future<$0.ListNodesResponse> listNodes(
      $grpc.ServiceCall call, $0.ListNodesRequest request);

  $async.Future<$0.GetNodeStatusResponse> getNodeStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetNodeStatusRequest> $request) async {
    return getNodeStatus($call, await $request);
  }

  $async.Future<$0.GetNodeStatusResponse> getNodeStatus(
      $grpc.ServiceCall call, $0.GetNodeStatusRequest request);

  $async.Future<$0.DefederateResponse> defederate_Pre($grpc.ServiceCall $call,
      $async.Future<$0.DefederateRequest> $request) async {
    return defederate($call, await $request);
  }

  $async.Future<$0.DefederateResponse> defederate(
      $grpc.ServiceCall call, $0.DefederateRequest request);
}
