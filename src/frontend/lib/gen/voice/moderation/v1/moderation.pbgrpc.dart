// This is a generated file - do not edit.
//
// Generated from voice/moderation/v1/moderation.proto.

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

import 'moderation.pb.dart' as $0;

export 'moderation.pb.dart';

/// Reports, sanctions, automod. HTTP: /api/v1/moderation/**.
@$pb.GrpcServiceName('voice.moderation.v1.ModerationService')
class ModerationServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  ModerationServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.CreateReportResponse> createReport(
    $0.CreateReportRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$createReport, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetReportResponse> getReport(
    $0.GetReportRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getReport, request, options: options);
  }

  $grpc.ResponseFuture<$0.ListReportsResponse> listReports(
    $0.ListReportsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$listReports, request, options: options);
  }

  $grpc.ResponseFuture<$0.ResolveReportResponse> resolveReport(
    $0.ResolveReportRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$resolveReport, request, options: options);
  }

  $grpc.ResponseFuture<$0.ApplySanctionResponse> applySanction(
    $0.ApplySanctionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$applySanction, request, options: options);
  }

  $grpc.ResponseFuture<$0.RevokeSanctionResponse> revokeSanction(
    $0.RevokeSanctionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$revokeSanction, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetAccountSanctionsResponse> getAccountSanctions(
    $0.GetAccountSanctionsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAccountSanctions, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetActiveSanctionResponse> getActiveSanction(
    $0.GetActiveSanctionRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getActiveSanction, request, options: options);
  }

  $grpc.ResponseFuture<$0.SubmitAppealResponse> submitAppeal(
    $0.SubmitAppealRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$submitAppeal, request, options: options);
  }

  $grpc.ResponseFuture<$0.ReviewAppealResponse> reviewAppeal(
    $0.ReviewAppealRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$reviewAppeal, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetAppealResponse> getAppeal(
    $0.GetAppealRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAppeal, request, options: options);
  }

  $grpc.ResponseFuture<$0.CheckMessageResponse> checkMessage(
    $0.CheckMessageRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$checkMessage, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetAutoModStatsResponse> getAutoModStats(
    $0.GetAutoModStatsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getAutoModStats, request, options: options);
  }

  $grpc.ResponseFuture<$0.IsShadowBannedResponse> isShadowBanned(
    $0.IsShadowBannedRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$isShadowBanned, request, options: options);
  }

  // method descriptors

  static final _$createReport =
      $grpc.ClientMethod<$0.CreateReportRequest, $0.CreateReportResponse>(
          '/voice.moderation.v1.ModerationService/CreateReport',
          ($0.CreateReportRequest value) => value.writeToBuffer(),
          $0.CreateReportResponse.fromBuffer);
  static final _$getReport =
      $grpc.ClientMethod<$0.GetReportRequest, $0.GetReportResponse>(
          '/voice.moderation.v1.ModerationService/GetReport',
          ($0.GetReportRequest value) => value.writeToBuffer(),
          $0.GetReportResponse.fromBuffer);
  static final _$listReports =
      $grpc.ClientMethod<$0.ListReportsRequest, $0.ListReportsResponse>(
          '/voice.moderation.v1.ModerationService/ListReports',
          ($0.ListReportsRequest value) => value.writeToBuffer(),
          $0.ListReportsResponse.fromBuffer);
  static final _$resolveReport =
      $grpc.ClientMethod<$0.ResolveReportRequest, $0.ResolveReportResponse>(
          '/voice.moderation.v1.ModerationService/ResolveReport',
          ($0.ResolveReportRequest value) => value.writeToBuffer(),
          $0.ResolveReportResponse.fromBuffer);
  static final _$applySanction =
      $grpc.ClientMethod<$0.ApplySanctionRequest, $0.ApplySanctionResponse>(
          '/voice.moderation.v1.ModerationService/ApplySanction',
          ($0.ApplySanctionRequest value) => value.writeToBuffer(),
          $0.ApplySanctionResponse.fromBuffer);
  static final _$revokeSanction =
      $grpc.ClientMethod<$0.RevokeSanctionRequest, $0.RevokeSanctionResponse>(
          '/voice.moderation.v1.ModerationService/RevokeSanction',
          ($0.RevokeSanctionRequest value) => value.writeToBuffer(),
          $0.RevokeSanctionResponse.fromBuffer);
  static final _$getAccountSanctions = $grpc.ClientMethod<
          $0.GetAccountSanctionsRequest, $0.GetAccountSanctionsResponse>(
      '/voice.moderation.v1.ModerationService/GetAccountSanctions',
      ($0.GetAccountSanctionsRequest value) => value.writeToBuffer(),
      $0.GetAccountSanctionsResponse.fromBuffer);
  static final _$getActiveSanction = $grpc.ClientMethod<
          $0.GetActiveSanctionRequest, $0.GetActiveSanctionResponse>(
      '/voice.moderation.v1.ModerationService/GetActiveSanction',
      ($0.GetActiveSanctionRequest value) => value.writeToBuffer(),
      $0.GetActiveSanctionResponse.fromBuffer);
  static final _$submitAppeal =
      $grpc.ClientMethod<$0.SubmitAppealRequest, $0.SubmitAppealResponse>(
          '/voice.moderation.v1.ModerationService/SubmitAppeal',
          ($0.SubmitAppealRequest value) => value.writeToBuffer(),
          $0.SubmitAppealResponse.fromBuffer);
  static final _$reviewAppeal =
      $grpc.ClientMethod<$0.ReviewAppealRequest, $0.ReviewAppealResponse>(
          '/voice.moderation.v1.ModerationService/ReviewAppeal',
          ($0.ReviewAppealRequest value) => value.writeToBuffer(),
          $0.ReviewAppealResponse.fromBuffer);
  static final _$getAppeal =
      $grpc.ClientMethod<$0.GetAppealRequest, $0.GetAppealResponse>(
          '/voice.moderation.v1.ModerationService/GetAppeal',
          ($0.GetAppealRequest value) => value.writeToBuffer(),
          $0.GetAppealResponse.fromBuffer);
  static final _$checkMessage =
      $grpc.ClientMethod<$0.CheckMessageRequest, $0.CheckMessageResponse>(
          '/voice.moderation.v1.ModerationService/CheckMessage',
          ($0.CheckMessageRequest value) => value.writeToBuffer(),
          $0.CheckMessageResponse.fromBuffer);
  static final _$getAutoModStats =
      $grpc.ClientMethod<$0.GetAutoModStatsRequest, $0.GetAutoModStatsResponse>(
          '/voice.moderation.v1.ModerationService/GetAutoModStats',
          ($0.GetAutoModStatsRequest value) => value.writeToBuffer(),
          $0.GetAutoModStatsResponse.fromBuffer);
  static final _$isShadowBanned =
      $grpc.ClientMethod<$0.IsShadowBannedRequest, $0.IsShadowBannedResponse>(
          '/voice.moderation.v1.ModerationService/IsShadowBanned',
          ($0.IsShadowBannedRequest value) => value.writeToBuffer(),
          $0.IsShadowBannedResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.moderation.v1.ModerationService')
abstract class ModerationServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.moderation.v1.ModerationService';

  ModerationServiceBase() {
    $addMethod(
        $grpc.ServiceMethod<$0.CreateReportRequest, $0.CreateReportResponse>(
            'CreateReport',
            createReport_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CreateReportRequest.fromBuffer(value),
            ($0.CreateReportResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetReportRequest, $0.GetReportResponse>(
        'GetReport',
        getReport_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetReportRequest.fromBuffer(value),
        ($0.GetReportResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ListReportsRequest, $0.ListReportsResponse>(
            'ListReports',
            listReports_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ListReportsRequest.fromBuffer(value),
            ($0.ListReportsResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ResolveReportRequest, $0.ResolveReportResponse>(
            'ResolveReport',
            resolveReport_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ResolveReportRequest.fromBuffer(value),
            ($0.ResolveReportResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ApplySanctionRequest, $0.ApplySanctionResponse>(
            'ApplySanction',
            applySanction_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ApplySanctionRequest.fromBuffer(value),
            ($0.ApplySanctionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RevokeSanctionRequest,
            $0.RevokeSanctionResponse>(
        'RevokeSanction',
        revokeSanction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RevokeSanctionRequest.fromBuffer(value),
        ($0.RevokeSanctionResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetAccountSanctionsRequest,
            $0.GetAccountSanctionsResponse>(
        'GetAccountSanctions',
        getAccountSanctions_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetAccountSanctionsRequest.fromBuffer(value),
        ($0.GetAccountSanctionsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetActiveSanctionRequest,
            $0.GetActiveSanctionResponse>(
        'GetActiveSanction',
        getActiveSanction_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetActiveSanctionRequest.fromBuffer(value),
        ($0.GetActiveSanctionResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SubmitAppealRequest, $0.SubmitAppealResponse>(
            'SubmitAppeal',
            submitAppeal_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SubmitAppealRequest.fromBuffer(value),
            ($0.SubmitAppealResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ReviewAppealRequest, $0.ReviewAppealResponse>(
            'ReviewAppeal',
            reviewAppeal_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ReviewAppealRequest.fromBuffer(value),
            ($0.ReviewAppealResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetAppealRequest, $0.GetAppealResponse>(
        'GetAppeal',
        getAppeal_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetAppealRequest.fromBuffer(value),
        ($0.GetAppealResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.CheckMessageRequest, $0.CheckMessageResponse>(
            'CheckMessage',
            checkMessage_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.CheckMessageRequest.fromBuffer(value),
            ($0.CheckMessageResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetAutoModStatsRequest,
            $0.GetAutoModStatsResponse>(
        'GetAutoModStats',
        getAutoModStats_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetAutoModStatsRequest.fromBuffer(value),
        ($0.GetAutoModStatsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.IsShadowBannedRequest,
            $0.IsShadowBannedResponse>(
        'IsShadowBanned',
        isShadowBanned_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.IsShadowBannedRequest.fromBuffer(value),
        ($0.IsShadowBannedResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.CreateReportResponse> createReport_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CreateReportRequest> $request) async {
    return createReport($call, await $request);
  }

  $async.Future<$0.CreateReportResponse> createReport(
      $grpc.ServiceCall call, $0.CreateReportRequest request);

  $async.Future<$0.GetReportResponse> getReport_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetReportRequest> $request) async {
    return getReport($call, await $request);
  }

  $async.Future<$0.GetReportResponse> getReport(
      $grpc.ServiceCall call, $0.GetReportRequest request);

  $async.Future<$0.ListReportsResponse> listReports_Pre($grpc.ServiceCall $call,
      $async.Future<$0.ListReportsRequest> $request) async {
    return listReports($call, await $request);
  }

  $async.Future<$0.ListReportsResponse> listReports(
      $grpc.ServiceCall call, $0.ListReportsRequest request);

  $async.Future<$0.ResolveReportResponse> resolveReport_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ResolveReportRequest> $request) async {
    return resolveReport($call, await $request);
  }

  $async.Future<$0.ResolveReportResponse> resolveReport(
      $grpc.ServiceCall call, $0.ResolveReportRequest request);

  $async.Future<$0.ApplySanctionResponse> applySanction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ApplySanctionRequest> $request) async {
    return applySanction($call, await $request);
  }

  $async.Future<$0.ApplySanctionResponse> applySanction(
      $grpc.ServiceCall call, $0.ApplySanctionRequest request);

  $async.Future<$0.RevokeSanctionResponse> revokeSanction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RevokeSanctionRequest> $request) async {
    return revokeSanction($call, await $request);
  }

  $async.Future<$0.RevokeSanctionResponse> revokeSanction(
      $grpc.ServiceCall call, $0.RevokeSanctionRequest request);

  $async.Future<$0.GetAccountSanctionsResponse> getAccountSanctions_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetAccountSanctionsRequest> $request) async {
    return getAccountSanctions($call, await $request);
  }

  $async.Future<$0.GetAccountSanctionsResponse> getAccountSanctions(
      $grpc.ServiceCall call, $0.GetAccountSanctionsRequest request);

  $async.Future<$0.GetActiveSanctionResponse> getActiveSanction_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetActiveSanctionRequest> $request) async {
    return getActiveSanction($call, await $request);
  }

  $async.Future<$0.GetActiveSanctionResponse> getActiveSanction(
      $grpc.ServiceCall call, $0.GetActiveSanctionRequest request);

  $async.Future<$0.SubmitAppealResponse> submitAppeal_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SubmitAppealRequest> $request) async {
    return submitAppeal($call, await $request);
  }

  $async.Future<$0.SubmitAppealResponse> submitAppeal(
      $grpc.ServiceCall call, $0.SubmitAppealRequest request);

  $async.Future<$0.ReviewAppealResponse> reviewAppeal_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ReviewAppealRequest> $request) async {
    return reviewAppeal($call, await $request);
  }

  $async.Future<$0.ReviewAppealResponse> reviewAppeal(
      $grpc.ServiceCall call, $0.ReviewAppealRequest request);

  $async.Future<$0.GetAppealResponse> getAppeal_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetAppealRequest> $request) async {
    return getAppeal($call, await $request);
  }

  $async.Future<$0.GetAppealResponse> getAppeal(
      $grpc.ServiceCall call, $0.GetAppealRequest request);

  $async.Future<$0.CheckMessageResponse> checkMessage_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.CheckMessageRequest> $request) async {
    return checkMessage($call, await $request);
  }

  $async.Future<$0.CheckMessageResponse> checkMessage(
      $grpc.ServiceCall call, $0.CheckMessageRequest request);

  $async.Future<$0.GetAutoModStatsResponse> getAutoModStats_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetAutoModStatsRequest> $request) async {
    return getAutoModStats($call, await $request);
  }

  $async.Future<$0.GetAutoModStatsResponse> getAutoModStats(
      $grpc.ServiceCall call, $0.GetAutoModStatsRequest request);

  $async.Future<$0.IsShadowBannedResponse> isShadowBanned_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.IsShadowBannedRequest> $request) async {
    return isShadowBanned($call, await $request);
  }

  $async.Future<$0.IsShadowBannedResponse> isShadowBanned(
      $grpc.ServiceCall call, $0.IsShadowBannedRequest request);
}
