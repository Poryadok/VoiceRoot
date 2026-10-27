// This is a generated file - do not edit.
//
// Generated from voice/catalogactor/v1/catalog_actor.proto.

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

import 'catalog_actor.pb.dart' as $0;

export 'catalog_actor.pb.dart';

/// Internal read-only User authority, registered only on its catalog mTLS listener.
/// A response is a snapshot, never a cross-service permit, lease or Auth decision.
@$pb.GrpcServiceName('voice.catalogactor.v1.CatalogActorAuthorityService')
class CatalogActorAuthorityServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  CatalogActorAuthorityServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:chat,service:file
  $grpc.ResponseFuture<$0.GetCatalogActorStatesResponse> getCatalogActorStates(
    $0.GetCatalogActorStatesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getCatalogActorStates, request, options: options);
  }

  // method descriptors

  static final _$getCatalogActorStates = $grpc.ClientMethod<
          $0.GetCatalogActorStatesRequest, $0.GetCatalogActorStatesResponse>(
      '/voice.catalogactor.v1.CatalogActorAuthorityService/GetCatalogActorStates',
      ($0.GetCatalogActorStatesRequest value) => value.writeToBuffer(),
      $0.GetCatalogActorStatesResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.catalogactor.v1.CatalogActorAuthorityService')
abstract class CatalogActorAuthorityServiceBase extends $grpc.Service {
  $core.String get $name =>
      'voice.catalogactor.v1.CatalogActorAuthorityService';

  CatalogActorAuthorityServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetCatalogActorStatesRequest,
            $0.GetCatalogActorStatesResponse>(
        'GetCatalogActorStates',
        getCatalogActorStates_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetCatalogActorStatesRequest.fromBuffer(value),
        ($0.GetCatalogActorStatesResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.GetCatalogActorStatesResponse> getCatalogActorStates_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetCatalogActorStatesRequest> $request) async {
    return getCatalogActorStates($call, await $request);
  }

  $async.Future<$0.GetCatalogActorStatesResponse> getCatalogActorStates(
      $grpc.ServiceCall call, $0.GetCatalogActorStatesRequest request);
}
