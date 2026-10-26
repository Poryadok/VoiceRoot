// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/subscription_lifecycle.proto.

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

import 'subscription_lifecycle.pb.dart' as $0;

export 'subscription_lifecycle.pb.dart';

/// Dedicated Auth-owned account/lifecycle input. It never grants paid entitlement.
@$pb.GrpcServiceName('voice.auth.v1.AuthSubscriptionLifecycleService')
class AuthSubscriptionLifecycleServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthSubscriptionLifecycleServiceClient(super.channel,
      {super.options, super.interceptors});

  /// @voice.security=protected;callers=service:subscription
  $grpc.ResponseFuture<$0.ObserveSubscriptionAccountResponse>
      observeSubscriptionAccount(
    $0.ObserveSubscriptionAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$observeSubscriptionAccount, request,
        options: options);
  }

  // method descriptors

  static final _$observeSubscriptionAccount = $grpc.ClientMethod<
          $0.ObserveSubscriptionAccountRequest,
          $0.ObserveSubscriptionAccountResponse>(
      '/voice.auth.v1.AuthSubscriptionLifecycleService/ObserveSubscriptionAccount',
      ($0.ObserveSubscriptionAccountRequest value) => value.writeToBuffer(),
      $0.ObserveSubscriptionAccountResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.auth.v1.AuthSubscriptionLifecycleService')
abstract class AuthSubscriptionLifecycleServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.auth.v1.AuthSubscriptionLifecycleService';

  AuthSubscriptionLifecycleServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ObserveSubscriptionAccountRequest,
            $0.ObserveSubscriptionAccountResponse>(
        'ObserveSubscriptionAccount',
        observeSubscriptionAccount_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ObserveSubscriptionAccountRequest.fromBuffer(value),
        ($0.ObserveSubscriptionAccountResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.ObserveSubscriptionAccountResponse>
      observeSubscriptionAccount_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ObserveSubscriptionAccountRequest> $request) async {
    return observeSubscriptionAccount($call, await $request);
  }

  $async.Future<$0.ObserveSubscriptionAccountResponse>
      observeSubscriptionAccount(
          $grpc.ServiceCall call, $0.ObserveSubscriptionAccountRequest request);
}
