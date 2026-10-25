// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actions.proto.

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

import 'lfp_actions.pb.dart' as $0;

export 'lfp_actions.pb.dart';

/// PRIVATE Gateway -> Story delegated-user methods. Exact signature, audience,
/// method/body/request ID, replay admission and current session authority are
/// mandatory. Ordinary Story metadata cannot invoke these qualified actions.
@$pb.GrpcServiceName('voice.story.v1.LfpConsentActionService')
class LfpConsentActionServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  LfpConsentActionServiceClient(super.channel,
      {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.PublishLfpConsentOfferResponse>
      publishLfpConsentOffer(
    $0.PublishLfpConsentOfferRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$publishLfpConsentOffer, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.ObserveLfpConsentOfferResponse>
      observeLfpConsentOffer(
    $0.ObserveLfpConsentOfferRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$observeLfpConsentOffer, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.RespondWithLfpConsentResponse> respondWithLfpConsent(
    $0.RespondWithLfpConsentRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$respondWithLfpConsent, request, options: options);
  }

  // method descriptors

  static final _$publishLfpConsentOffer = $grpc.ClientMethod<
          $0.PublishLfpConsentOfferRequest, $0.PublishLfpConsentOfferResponse>(
      '/voice.story.v1.LfpConsentActionService/PublishLfpConsentOffer',
      ($0.PublishLfpConsentOfferRequest value) => value.writeToBuffer(),
      $0.PublishLfpConsentOfferResponse.fromBuffer);
  static final _$observeLfpConsentOffer = $grpc.ClientMethod<
          $0.ObserveLfpConsentOfferRequest, $0.ObserveLfpConsentOfferResponse>(
      '/voice.story.v1.LfpConsentActionService/ObserveLfpConsentOffer',
      ($0.ObserveLfpConsentOfferRequest value) => value.writeToBuffer(),
      $0.ObserveLfpConsentOfferResponse.fromBuffer);
  static final _$respondWithLfpConsent = $grpc.ClientMethod<
          $0.RespondWithLfpConsentRequest, $0.RespondWithLfpConsentResponse>(
      '/voice.story.v1.LfpConsentActionService/RespondWithLfpConsent',
      ($0.RespondWithLfpConsentRequest value) => value.writeToBuffer(),
      $0.RespondWithLfpConsentResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.story.v1.LfpConsentActionService')
abstract class LfpConsentActionServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.story.v1.LfpConsentActionService';

  LfpConsentActionServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.PublishLfpConsentOfferRequest,
            $0.PublishLfpConsentOfferResponse>(
        'PublishLfpConsentOffer',
        publishLfpConsentOffer_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.PublishLfpConsentOfferRequest.fromBuffer(value),
        ($0.PublishLfpConsentOfferResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ObserveLfpConsentOfferRequest,
            $0.ObserveLfpConsentOfferResponse>(
        'ObserveLfpConsentOffer',
        observeLfpConsentOffer_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ObserveLfpConsentOfferRequest.fromBuffer(value),
        ($0.ObserveLfpConsentOfferResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RespondWithLfpConsentRequest,
            $0.RespondWithLfpConsentResponse>(
        'RespondWithLfpConsent',
        respondWithLfpConsent_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RespondWithLfpConsentRequest.fromBuffer(value),
        ($0.RespondWithLfpConsentResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.PublishLfpConsentOfferResponse> publishLfpConsentOffer_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.PublishLfpConsentOfferRequest> $request) async {
    return publishLfpConsentOffer($call, await $request);
  }

  $async.Future<$0.PublishLfpConsentOfferResponse> publishLfpConsentOffer(
      $grpc.ServiceCall call, $0.PublishLfpConsentOfferRequest request);

  $async.Future<$0.ObserveLfpConsentOfferResponse> observeLfpConsentOffer_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ObserveLfpConsentOfferRequest> $request) async {
    return observeLfpConsentOffer($call, await $request);
  }

  $async.Future<$0.ObserveLfpConsentOfferResponse> observeLfpConsentOffer(
      $grpc.ServiceCall call, $0.ObserveLfpConsentOfferRequest request);

  $async.Future<$0.RespondWithLfpConsentResponse> respondWithLfpConsent_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RespondWithLfpConsentRequest> $request) async {
    return respondWithLfpConsent($call, await $request);
  }

  $async.Future<$0.RespondWithLfpConsentResponse> respondWithLfpConsent(
      $grpc.ServiceCall call, $0.RespondWithLfpConsentRequest request);
}
