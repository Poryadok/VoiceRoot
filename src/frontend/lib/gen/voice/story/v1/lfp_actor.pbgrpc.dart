// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actor.proto.

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

import 'lfp_actor.pb.dart' as $0;

export 'lfp_actor.pb.dart';

/// PRIVATE Story -> MM. Exact service principal/method/body/JTI, replay, verified
/// TLS and current admitted cohort required. Never fall back to barev1 methods.
@$pb.GrpcServiceName('voice.story.v1.LfpActorBoundConsentRegistryService')
class LfpActorBoundConsentRegistryServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  LfpActorBoundConsentRegistryServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterActorBoundLfpConsentResponse>
      registerActorBoundLfpConsent(
    $0.RegisterActorBoundLfpConsentRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerActorBoundLfpConsent, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ReadActorBoundLfpConsentRegistrationResponse>
      readActorBoundLfpConsentRegistration(
    $0.ReadActorBoundLfpConsentRegistrationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readActorBoundLfpConsentRegistration, request,
        options: options);
  }

  // method descriptors

  static final _$registerActorBoundLfpConsent = $grpc.ClientMethod<
          $0.RegisterActorBoundLfpConsentRequest,
          $0.RegisterActorBoundLfpConsentResponse>(
      '/voice.story.v1.LfpActorBoundConsentRegistryService/RegisterActorBoundLfpConsent',
      ($0.RegisterActorBoundLfpConsentRequest value) => value.writeToBuffer(),
      $0.RegisterActorBoundLfpConsentResponse.fromBuffer);
  static final _$readActorBoundLfpConsentRegistration = $grpc.ClientMethod<
          $0.ReadActorBoundLfpConsentRegistrationRequest,
          $0.ReadActorBoundLfpConsentRegistrationResponse>(
      '/voice.story.v1.LfpActorBoundConsentRegistryService/ReadActorBoundLfpConsentRegistration',
      ($0.ReadActorBoundLfpConsentRegistrationRequest value) =>
          value.writeToBuffer(),
      $0.ReadActorBoundLfpConsentRegistrationResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.LfpActorBoundConsentRegistryService')
abstract class LfpActorBoundConsentRegistryServiceBase extends $grpc.Service {
  $core.String get $name =>
      'voice.story.v1.LfpActorBoundConsentRegistryService';

  LfpActorBoundConsentRegistryServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RegisterActorBoundLfpConsentRequest,
            $0.RegisterActorBoundLfpConsentResponse>(
        'RegisterActorBoundLfpConsent',
        registerActorBoundLfpConsent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegisterActorBoundLfpConsentRequest.fromBuffer(value),
        ($0.RegisterActorBoundLfpConsentResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<
            $0.ReadActorBoundLfpConsentRegistrationRequest,
            $0.ReadActorBoundLfpConsentRegistrationResponse>(
        'ReadActorBoundLfpConsentRegistration',
        readActorBoundLfpConsentRegistration_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReadActorBoundLfpConsentRegistrationRequest.fromBuffer(value),
        ($0.ReadActorBoundLfpConsentRegistrationResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.RegisterActorBoundLfpConsentResponse>
      registerActorBoundLfpConsent_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.RegisterActorBoundLfpConsentRequest>
              $request) async {
    return registerActorBoundLfpConsent($call, await $request);
  }

  $async.Future<$0.RegisterActorBoundLfpConsentResponse>
      registerActorBoundLfpConsent($grpc.ServiceCall call,
          $0.RegisterActorBoundLfpConsentRequest request);

  $async.Future<$0.ReadActorBoundLfpConsentRegistrationResponse>
      readActorBoundLfpConsentRegistration_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ReadActorBoundLfpConsentRegistrationRequest>
              $request) async {
    return readActorBoundLfpConsentRegistration($call, await $request);
  }

  $async.Future<$0.ReadActorBoundLfpConsentRegistrationResponse>
      readActorBoundLfpConsentRegistration($grpc.ServiceCall call,
          $0.ReadActorBoundLfpConsentRegistrationRequest request);
}

/// PRIVATE MM -> Story. Load original sidecar, use explicit saved viewer type for
/// fresh privacy, then recheck exact active Story/offer/sidecar/Origin under SQL.
@$pb.GrpcServiceName('voice.story.v1.LfpActorBoundConsentOfferService')
class LfpActorBoundConsentOfferServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  LfpActorBoundConsentOfferServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.ValidateActorBoundLfpConsentOfferResponse>
      validateActorBoundLfpConsentOffer(
    $0.ValidateActorBoundLfpConsentOfferRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateActorBoundLfpConsentOffer, request,
        options: options);
  }

  // method descriptors

  static final _$validateActorBoundLfpConsentOffer = $grpc.ClientMethod<
          $0.ValidateActorBoundLfpConsentOfferRequest,
          $0.ValidateActorBoundLfpConsentOfferResponse>(
      '/voice.story.v1.LfpActorBoundConsentOfferService/ValidateActorBoundLfpConsentOffer',
      ($0.ValidateActorBoundLfpConsentOfferRequest value) =>
          value.writeToBuffer(),
      $0.ValidateActorBoundLfpConsentOfferResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.LfpActorBoundConsentOfferService')
abstract class LfpActorBoundConsentOfferServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.LfpActorBoundConsentOfferService';

  LfpActorBoundConsentOfferServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ValidateActorBoundLfpConsentOfferRequest,
            $0.ValidateActorBoundLfpConsentOfferResponse>(
        'ValidateActorBoundLfpConsentOffer',
        validateActorBoundLfpConsentOffer_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ValidateActorBoundLfpConsentOfferRequest.fromBuffer(value),
        ($0.ValidateActorBoundLfpConsentOfferResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.ValidateActorBoundLfpConsentOfferResponse>
      validateActorBoundLfpConsentOffer_Pre(
          $grpc.ServiceCall $call,
          $async.Future<$0.ValidateActorBoundLfpConsentOfferRequest>
              $request) async {
    return validateActorBoundLfpConsentOffer($call, await $request);
  }

  $async.Future<$0.ValidateActorBoundLfpConsentOfferResponse>
      validateActorBoundLfpConsentOffer($grpc.ServiceCall call,
          $0.ValidateActorBoundLfpConsentOfferRequest request);
}
