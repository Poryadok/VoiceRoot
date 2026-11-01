// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user_source_custody.proto.

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

import 'user_source_custody.pb.dart' as $0;

export 'user_source_custody.pb.dart';

/// Private User-only held grant. A verified mTLS peer and signed User service
/// principal bind the exact request and RPC; replay is denied before admission.
/// Stream lifetime cannot exceed credential expiry, and heartbeat does not
/// renew it. Loss cancels work but does not fence an in-flight User commit or
/// authorize a source transition.
@$pb.GrpcServiceName('voice.user.v1.UserSourceCustodyService')
class UserSourceCustodyServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  UserSourceCustodyServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseStream<$0.AcquireUserSourcePermitResponse>
      acquireUserSourcePermit(
    $0.AcquireUserSourcePermitRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createStreamingCall(
        _$acquireUserSourcePermit, $async.Stream.fromIterable([request]),
        options: options);
  }

  // method descriptors

  static final _$acquireUserSourcePermit = $grpc.ClientMethod<
          $0.AcquireUserSourcePermitRequest,
          $0.AcquireUserSourcePermitResponse>(
      '/voice.user.v1.UserSourceCustodyService/AcquireUserSourcePermit',
      ($0.AcquireUserSourcePermitRequest value) => value.writeToBuffer(),
      $0.AcquireUserSourcePermitResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.user.v1.UserSourceCustodyService')
abstract class UserSourceCustodyServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.user.v1.UserSourceCustodyService';

  UserSourceCustodyServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.AcquireUserSourcePermitRequest,
            $0.AcquireUserSourcePermitResponse>(
        'AcquireUserSourcePermit',
        acquireUserSourcePermit_Pre,
        false,
        true,
        ($core.List<$core.int> value) =>
            $0.AcquireUserSourcePermitRequest.fromBuffer(value),
        ($0.AcquireUserSourcePermitResponse value) => value.writeToBuffer()));
  }

  $async.Stream<$0.AcquireUserSourcePermitResponse> acquireUserSourcePermit_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.AcquireUserSourcePermitRequest> $request) async* {
    yield* acquireUserSourcePermit($call, await $request);
  }

  $async.Stream<$0.AcquireUserSourcePermitResponse> acquireUserSourcePermit(
      $grpc.ServiceCall call, $0.AcquireUserSourcePermitRequest request);
}
