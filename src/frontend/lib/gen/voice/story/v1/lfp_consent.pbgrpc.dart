// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_consent.proto.

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

import 'lfp_consent.pb.dart' as $0;

export 'lfp_consent.pb.dart';

/// Implemented only by protected Matchmaking. Exact signed service:story,
/// aud=matchmaking, full method/body hash/request_id/fresh JTI, TLS and replay
/// continuity required. A bare receipt hash or public event is no authority.
@$pb.GrpcServiceName('voice.story.v1.LfpConsentRegistryService')
class LfpConsentRegistryServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  LfpConsentRegistryServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterLfpConsentResponse> registerLfpConsent(
    $0.RegisterLfpConsentRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerLfpConsent, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReadLfpConsentRegistrationResponse>
      readLfpConsentRegistration(
    $0.ReadLfpConsentRegistrationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$readLfpConsentRegistration, request,
        options: options);
  }

  // method descriptors

  static final _$registerLfpConsent = $grpc.ClientMethod<
          $0.RegisterLfpConsentRequest, $0.RegisterLfpConsentResponse>(
      '/voice.story.v1.LfpConsentRegistryService/RegisterLfpConsent',
      ($0.RegisterLfpConsentRequest value) => value.writeToBuffer(),
      $0.RegisterLfpConsentResponse.fromBuffer);
  static final _$readLfpConsentRegistration = $grpc.ClientMethod<
          $0.ReadLfpConsentRegistrationRequest,
          $0.ReadLfpConsentRegistrationResponse>(
      '/voice.story.v1.LfpConsentRegistryService/ReadLfpConsentRegistration',
      ($0.ReadLfpConsentRegistrationRequest value) => value.writeToBuffer(),
      $0.ReadLfpConsentRegistrationResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.LfpConsentRegistryService')
abstract class LfpConsentRegistryServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.LfpConsentRegistryService';

  LfpConsentRegistryServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RegisterLfpConsentRequest,
            $0.RegisterLfpConsentResponse>(
        'RegisterLfpConsent',
        registerLfpConsent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegisterLfpConsentRequest.fromBuffer(value),
        ($0.RegisterLfpConsentResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ReadLfpConsentRegistrationRequest,
            $0.ReadLfpConsentRegistrationResponse>(
        'ReadLfpConsentRegistration',
        readLfpConsentRegistration_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ReadLfpConsentRegistrationRequest.fromBuffer(value),
        ($0.ReadLfpConsentRegistrationResponse value) =>
            value.writeToBuffer()));
  }

  $async.Future<$0.RegisterLfpConsentResponse> registerLfpConsent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RegisterLfpConsentRequest> $request) async {
    return registerLfpConsent($call, await $request);
  }

  $async.Future<$0.RegisterLfpConsentResponse> registerLfpConsent(
      $grpc.ServiceCall call, $0.RegisterLfpConsentRequest request);

  $async.Future<$0.ReadLfpConsentRegistrationResponse>
      readLfpConsentRegistration_Pre($grpc.ServiceCall $call,
          $async.Future<$0.ReadLfpConsentRegistrationRequest> $request) async {
    return readLfpConsentRegistration($call, await $request);
  }

  $async.Future<$0.ReadLfpConsentRegistrationResponse>
      readLfpConsentRegistration(
          $grpc.ServiceCall call, $0.ReadLfpConsentRegistrationRequest request);
}

/// Implemented only by protected Story. Exact signed service:matchmaking,
/// aud=story, full method/body hash/request_id/fresh JTI, TLS and replay required.
@$pb.GrpcServiceName('voice.story.v1.LfpConsentOfferService')
class LfpConsentOfferServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  LfpConsentOfferServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.ValidateLfpConsentOfferResponse>
      validateLfpConsentOffer(
    $0.ValidateLfpConsentOfferRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateLfpConsentOffer, request,
        options: options);
  }

  // method descriptors

  static final _$validateLfpConsentOffer = $grpc.ClientMethod<
          $0.ValidateLfpConsentOfferRequest,
          $0.ValidateLfpConsentOfferResponse>(
      '/voice.story.v1.LfpConsentOfferService/ValidateLfpConsentOffer',
      ($0.ValidateLfpConsentOfferRequest value) => value.writeToBuffer(),
      $0.ValidateLfpConsentOfferResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.LfpConsentOfferService')
abstract class LfpConsentOfferServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.LfpConsentOfferService';

  LfpConsentOfferServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.ValidateLfpConsentOfferRequest,
            $0.ValidateLfpConsentOfferResponse>(
        'ValidateLfpConsentOffer',
        validateLfpConsentOffer_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ValidateLfpConsentOfferRequest.fromBuffer(value),
        ($0.ValidateLfpConsentOfferResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.ValidateLfpConsentOfferResponse> validateLfpConsentOffer_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ValidateLfpConsentOfferRequest> $request) async {
    return validateLfpConsentOffer($call, await $request);
  }

  $async.Future<$0.ValidateLfpConsentOfferResponse> validateLfpConsentOffer(
      $grpc.ServiceCall call, $0.ValidateLfpConsentOfferRequest request);
}
