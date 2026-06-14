// This is a generated file - do not edit.
//
// Generated from voice/subscription/v1/subscription.proto.

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

import 'subscription.pb.dart' as $0;

export 'subscription.pb.dart';

/// Billing and limits. HTTP: /api/v1/subscription/**.
@$pb.GrpcServiceName('voice.subscription.v1.SubscriptionService')
class SubscriptionServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  SubscriptionServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.GetSubscriptionResponse> getSubscription(
    $0.GetSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSubscription, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateCheckoutSessionResponse> createCheckoutSession(
    $0.CreateCheckoutSessionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createCheckoutSession, request, options: options);
  }

  $grpc.ResponseFuture<$0.CancelSubscriptionResponse> cancelSubscription(
    $0.CancelSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$cancelSubscription, request, options: options);
  }

  $grpc.ResponseFuture<$0.ResumeSubscriptionResponse> resumeSubscription(
    $0.ResumeSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resumeSubscription, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetSpaceSubscriptionResponse> getSpaceSubscription(
    $0.GetSpaceSubscriptionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getSpaceSubscription, request, options: options);
  }

  $grpc.ResponseFuture<$0.CreateSpaceCheckoutSessionResponse>
      createSpaceCheckoutSession(
    $0.CreateSpaceCheckoutSessionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createSpaceCheckoutSession, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.GetLimitsResponse> getLimits(
    $0.GetLimitsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getLimits, request, options: options);
  }

  $grpc.ResponseFuture<$0.CheckLimitResponse> checkLimit(
    $0.CheckLimitRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkLimit, request, options: options);
  }

  $grpc.ResponseFuture<$0.HandlePaddleWebhookResponse> handlePaddleWebhook(
    $0.HandlePaddleWebhookRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$handlePaddleWebhook, request, options: options);
  }

  $grpc.ResponseFuture<$0.HandleCloudPaymentsWebhookResponse>
      handleCloudPaymentsWebhook(
    $0.HandleCloudPaymentsWebhookRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$handleCloudPaymentsWebhook, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.GetBillingHistoryResponse> getBillingHistory(
    $0.GetBillingHistoryRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getBillingHistory, request, options: options);
  }

  /// Phase 13: keep selected profiles when downgrading from premium (multi-profile.md).
  $grpc.ResponseFuture<$0.ApplyDowngradeProfilesResponse>
      applyDowngradeProfiles(
    $0.ApplyDowngradeProfilesRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applyDowngradeProfiles, request,
        options: options);
  }

  // method descriptors

  static final _$getSubscription =
      $grpc.ClientMethod<$0.GetSubscriptionRequest, $0.GetSubscriptionResponse>(
          '/voice.subscription.v1.SubscriptionService/GetSubscription',
          ($0.GetSubscriptionRequest value) => value.writeToBuffer(),
          $0.GetSubscriptionResponse.fromBuffer);
  static final _$createCheckoutSession = $grpc.ClientMethod<
          $0.CreateCheckoutSessionRequest, $0.CreateCheckoutSessionResponse>(
      '/voice.subscription.v1.SubscriptionService/CreateCheckoutSession',
      ($0.CreateCheckoutSessionRequest value) => value.writeToBuffer(),
      $0.CreateCheckoutSessionResponse.fromBuffer);
  static final _$cancelSubscription = $grpc.ClientMethod<
          $0.CancelSubscriptionRequest, $0.CancelSubscriptionResponse>(
      '/voice.subscription.v1.SubscriptionService/CancelSubscription',
      ($0.CancelSubscriptionRequest value) => value.writeToBuffer(),
      $0.CancelSubscriptionResponse.fromBuffer);
  static final _$resumeSubscription = $grpc.ClientMethod<
          $0.ResumeSubscriptionRequest, $0.ResumeSubscriptionResponse>(
      '/voice.subscription.v1.SubscriptionService/ResumeSubscription',
      ($0.ResumeSubscriptionRequest value) => value.writeToBuffer(),
      $0.ResumeSubscriptionResponse.fromBuffer);
  static final _$getSpaceSubscription = $grpc.ClientMethod<
          $0.GetSpaceSubscriptionRequest, $0.GetSpaceSubscriptionResponse>(
      '/voice.subscription.v1.SubscriptionService/GetSpaceSubscription',
      ($0.GetSpaceSubscriptionRequest value) => value.writeToBuffer(),
      $0.GetSpaceSubscriptionResponse.fromBuffer);
  static final _$createSpaceCheckoutSession = $grpc.ClientMethod<
          $0.CreateSpaceCheckoutSessionRequest,
          $0.CreateSpaceCheckoutSessionResponse>(
      '/voice.subscription.v1.SubscriptionService/CreateSpaceCheckoutSession',
      ($0.CreateSpaceCheckoutSessionRequest value) => value.writeToBuffer(),
      $0.CreateSpaceCheckoutSessionResponse.fromBuffer);
  static final _$getLimits =
      $grpc.ClientMethod<$0.GetLimitsRequest, $0.GetLimitsResponse>(
          '/voice.subscription.v1.SubscriptionService/GetLimits',
          ($0.GetLimitsRequest value) => value.writeToBuffer(),
          $0.GetLimitsResponse.fromBuffer);
  static final _$checkLimit =
      $grpc.ClientMethod<$0.CheckLimitRequest, $0.CheckLimitResponse>(
          '/voice.subscription.v1.SubscriptionService/CheckLimit',
          ($0.CheckLimitRequest value) => value.writeToBuffer(),
          $0.CheckLimitResponse.fromBuffer);
  static final _$handlePaddleWebhook = $grpc.ClientMethod<
          $0.HandlePaddleWebhookRequest, $0.HandlePaddleWebhookResponse>(
      '/voice.subscription.v1.SubscriptionService/HandlePaddleWebhook',
      ($0.HandlePaddleWebhookRequest value) => value.writeToBuffer(),
      $0.HandlePaddleWebhookResponse.fromBuffer);
  static final _$handleCloudPaymentsWebhook = $grpc.ClientMethod<
          $0.HandleCloudPaymentsWebhookRequest,
          $0.HandleCloudPaymentsWebhookResponse>(
      '/voice.subscription.v1.SubscriptionService/HandleCloudPaymentsWebhook',
      ($0.HandleCloudPaymentsWebhookRequest value) => value.writeToBuffer(),
      $0.HandleCloudPaymentsWebhookResponse.fromBuffer);
  static final _$getBillingHistory = $grpc.ClientMethod<
          $0.GetBillingHistoryRequest, $0.GetBillingHistoryResponse>(
      '/voice.subscription.v1.SubscriptionService/GetBillingHistory',
      ($0.GetBillingHistoryRequest value) => value.writeToBuffer(),
      $0.GetBillingHistoryResponse.fromBuffer);
  static final _$applyDowngradeProfiles = $grpc.ClientMethod<
          $0.ApplyDowngradeProfilesRequest, $0.ApplyDowngradeProfilesResponse>(
      '/voice.subscription.v1.SubscriptionService/ApplyDowngradeProfiles',
      ($0.ApplyDowngradeProfilesRequest value) => value.writeToBuffer(),
      $0.ApplyDowngradeProfilesResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.subscription.v1.SubscriptionService')
abstract class SubscriptionServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.subscription.v1.SubscriptionService';

  SubscriptionServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.GetSubscriptionRequest,
            $0.GetSubscriptionResponse>(
        'GetSubscription',
        getSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetSubscriptionRequest.fromBuffer(value),
        ($0.GetSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateCheckoutSessionRequest,
            $0.CreateCheckoutSessionResponse>(
        'CreateCheckoutSession',
        createCheckoutSession_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateCheckoutSessionRequest.fromBuffer(value),
        ($0.CreateCheckoutSessionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CancelSubscriptionRequest,
            $0.CancelSubscriptionResponse>(
        'CancelSubscription',
        cancelSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CancelSubscriptionRequest.fromBuffer(value),
        ($0.CancelSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ResumeSubscriptionRequest,
            $0.ResumeSubscriptionResponse>(
        'ResumeSubscription',
        resumeSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ResumeSubscriptionRequest.fromBuffer(value),
        ($0.ResumeSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetSpaceSubscriptionRequest,
            $0.GetSpaceSubscriptionResponse>(
        'GetSpaceSubscription',
        getSpaceSubscription_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetSpaceSubscriptionRequest.fromBuffer(value),
        ($0.GetSpaceSubscriptionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CreateSpaceCheckoutSessionRequest,
            $0.CreateSpaceCheckoutSessionResponse>(
        'CreateSpaceCheckoutSession',
        createSpaceCheckoutSession_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.CreateSpaceCheckoutSessionRequest.fromBuffer(value),
        ($0.CreateSpaceCheckoutSessionResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetLimitsRequest, $0.GetLimitsResponse>(
        'GetLimits',
        getLimits_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetLimitsRequest.fromBuffer(value),
        ($0.GetLimitsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.CheckLimitRequest, $0.CheckLimitResponse>(
        'CheckLimit',
        checkLimit_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.CheckLimitRequest.fromBuffer(value),
        ($0.CheckLimitResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.HandlePaddleWebhookRequest,
            $0.HandlePaddleWebhookResponse>(
        'HandlePaddleWebhook',
        handlePaddleWebhook_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.HandlePaddleWebhookRequest.fromBuffer(value),
        ($0.HandlePaddleWebhookResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.HandleCloudPaymentsWebhookRequest,
            $0.HandleCloudPaymentsWebhookResponse>(
        'HandleCloudPaymentsWebhook',
        handleCloudPaymentsWebhook_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.HandleCloudPaymentsWebhookRequest.fromBuffer(value),
        ($0.HandleCloudPaymentsWebhookResponse value) =>
            value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetBillingHistoryRequest,
            $0.GetBillingHistoryResponse>(
        'GetBillingHistory',
        getBillingHistory_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetBillingHistoryRequest.fromBuffer(value),
        ($0.GetBillingHistoryResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.ApplyDowngradeProfilesRequest,
            $0.ApplyDowngradeProfilesResponse>(
        'ApplyDowngradeProfiles',
        applyDowngradeProfiles_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.ApplyDowngradeProfilesRequest.fromBuffer(value),
        ($0.ApplyDowngradeProfilesResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.GetSubscriptionResponse> getSubscription_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetSubscriptionRequest> $request) async {
    return getSubscription($call, await $request);
  }

  $async.Future<$0.GetSubscriptionResponse> getSubscription(
      $grpc.ServiceCall call, $0.GetSubscriptionRequest request);

  $async.Future<$0.CreateCheckoutSessionResponse> createCheckoutSession_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateCheckoutSessionRequest> $request) async {
    return createCheckoutSession($call, await $request);
  }

  $async.Future<$0.CreateCheckoutSessionResponse> createCheckoutSession(
      $grpc.ServiceCall call, $0.CreateCheckoutSessionRequest request);

  $async.Future<$0.CancelSubscriptionResponse> cancelSubscription_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CancelSubscriptionRequest> $request) async {
    return cancelSubscription($call, await $request);
  }

  $async.Future<$0.CancelSubscriptionResponse> cancelSubscription(
      $grpc.ServiceCall call, $0.CancelSubscriptionRequest request);

  $async.Future<$0.ResumeSubscriptionResponse> resumeSubscription_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResumeSubscriptionRequest> $request) async {
    return resumeSubscription($call, await $request);
  }

  $async.Future<$0.ResumeSubscriptionResponse> resumeSubscription(
      $grpc.ServiceCall call, $0.ResumeSubscriptionRequest request);

  $async.Future<$0.GetSpaceSubscriptionResponse> getSpaceSubscription_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetSpaceSubscriptionRequest> $request) async {
    return getSpaceSubscription($call, await $request);
  }

  $async.Future<$0.GetSpaceSubscriptionResponse> getSpaceSubscription(
      $grpc.ServiceCall call, $0.GetSpaceSubscriptionRequest request);

  $async.Future<$0.CreateSpaceCheckoutSessionResponse>
      createSpaceCheckoutSession_Pre($grpc.ServiceCall $call,
          $async.Future<$0.CreateSpaceCheckoutSessionRequest> $request) async {
    return createSpaceCheckoutSession($call, await $request);
  }

  $async.Future<$0.CreateSpaceCheckoutSessionResponse>
      createSpaceCheckoutSession(
          $grpc.ServiceCall call, $0.CreateSpaceCheckoutSessionRequest request);

  $async.Future<$0.GetLimitsResponse> getLimits_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetLimitsRequest> $request) async {
    return getLimits($call, await $request);
  }

  $async.Future<$0.GetLimitsResponse> getLimits(
      $grpc.ServiceCall call, $0.GetLimitsRequest request);

  $async.Future<$0.CheckLimitResponse> checkLimit_Pre($grpc.ServiceCall $call,
      $async.Future<$0.CheckLimitRequest> $request) async {
    return checkLimit($call, await $request);
  }

  $async.Future<$0.CheckLimitResponse> checkLimit(
      $grpc.ServiceCall call, $0.CheckLimitRequest request);

  $async.Future<$0.HandlePaddleWebhookResponse> handlePaddleWebhook_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.HandlePaddleWebhookRequest> $request) async {
    return handlePaddleWebhook($call, await $request);
  }

  $async.Future<$0.HandlePaddleWebhookResponse> handlePaddleWebhook(
      $grpc.ServiceCall call, $0.HandlePaddleWebhookRequest request);

  $async.Future<$0.HandleCloudPaymentsWebhookResponse>
      handleCloudPaymentsWebhook_Pre($grpc.ServiceCall $call,
          $async.Future<$0.HandleCloudPaymentsWebhookRequest> $request) async {
    return handleCloudPaymentsWebhook($call, await $request);
  }

  $async.Future<$0.HandleCloudPaymentsWebhookResponse>
      handleCloudPaymentsWebhook(
          $grpc.ServiceCall call, $0.HandleCloudPaymentsWebhookRequest request);

  $async.Future<$0.GetBillingHistoryResponse> getBillingHistory_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetBillingHistoryRequest> $request) async {
    return getBillingHistory($call, await $request);
  }

  $async.Future<$0.GetBillingHistoryResponse> getBillingHistory(
      $grpc.ServiceCall call, $0.GetBillingHistoryRequest request);

  $async.Future<$0.ApplyDowngradeProfilesResponse> applyDowngradeProfiles_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ApplyDowngradeProfilesRequest> $request) async {
    return applyDowngradeProfiles($call, await $request);
  }

  $async.Future<$0.ApplyDowngradeProfilesResponse> applyDowngradeProfiles(
      $grpc.ServiceCall call, $0.ApplyDowngradeProfilesRequest request);
}
