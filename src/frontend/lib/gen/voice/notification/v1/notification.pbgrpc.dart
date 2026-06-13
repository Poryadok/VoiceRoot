// This is a generated file - do not edit.
//
// Generated from voice/notification/v1/notification.proto.

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

import 'notification.pb.dart' as $0;

export 'notification.pb.dart';

/// Push, email routing, device tokens. HTTP: /api/v1/notifications/**.
@$pb.GrpcServiceName('voice.notification.v1.NotificationService')
class NotificationServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  NotificationServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterDeviceResponse> registerDevice(
    $0.RegisterDeviceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$registerDevice, request, options: options);
  }

  $grpc.ResponseFuture<$0.UnregisterDeviceResponse> unregisterDevice(
    $0.UnregisterDeviceRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$unregisterDevice, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetNotificationSettingsResponse>
      getNotificationSettings(
    $0.GetNotificationSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getNotificationSettings, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.UpdateNotificationSettingsResponse>
      updateNotificationSettings(
    $0.UpdateNotificationSettingsRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$updateNotificationSettings, request,
        options: options);
  }

  $grpc.ResponseFuture<$0.SetQuietHoursResponse> setQuietHours(
    $0.SetQuietHoursRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setQuietHours, request, options: options);
  }

  $grpc.ResponseFuture<$0.SendNotificationResponse> sendNotification(
    $0.SendNotificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendNotification, request, options: options);
  }

  $grpc.ResponseFuture<$0.SendBulkNotificationResponse> sendBulkNotification(
    $0.SendBulkNotificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$sendBulkNotification, request, options: options);
  }

  $grpc.ResponseFuture<$0.RelayNotificationResponse> relayNotification(
    $0.RelayNotificationRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$relayNotification, request, options: options);
  }

  // method descriptors

  static final _$registerDevice =
      $grpc.ClientMethod<$0.RegisterDeviceRequest, $0.RegisterDeviceResponse>(
          '/voice.notification.v1.NotificationService/RegisterDevice',
          ($0.RegisterDeviceRequest value) => value.writeToBuffer(),
          $0.RegisterDeviceResponse.fromBuffer);
  static final _$unregisterDevice = $grpc.ClientMethod<
          $0.UnregisterDeviceRequest, $0.UnregisterDeviceResponse>(
      '/voice.notification.v1.NotificationService/UnregisterDevice',
      ($0.UnregisterDeviceRequest value) => value.writeToBuffer(),
      $0.UnregisterDeviceResponse.fromBuffer);
  static final _$getNotificationSettings = $grpc.ClientMethod<
          $0.GetNotificationSettingsRequest,
          $0.GetNotificationSettingsResponse>(
      '/voice.notification.v1.NotificationService/GetNotificationSettings',
      ($0.GetNotificationSettingsRequest value) => value.writeToBuffer(),
      $0.GetNotificationSettingsResponse.fromBuffer);
  static final _$updateNotificationSettings = $grpc.ClientMethod<
          $0.UpdateNotificationSettingsRequest,
          $0.UpdateNotificationSettingsResponse>(
      '/voice.notification.v1.NotificationService/UpdateNotificationSettings',
      ($0.UpdateNotificationSettingsRequest value) => value.writeToBuffer(),
      $0.UpdateNotificationSettingsResponse.fromBuffer);
  static final _$setQuietHours =
      $grpc.ClientMethod<$0.SetQuietHoursRequest, $0.SetQuietHoursResponse>(
          '/voice.notification.v1.NotificationService/SetQuietHours',
          ($0.SetQuietHoursRequest value) => value.writeToBuffer(),
          $0.SetQuietHoursResponse.fromBuffer);
  static final _$sendNotification = $grpc.ClientMethod<
          $0.SendNotificationRequest, $0.SendNotificationResponse>(
      '/voice.notification.v1.NotificationService/SendNotification',
      ($0.SendNotificationRequest value) => value.writeToBuffer(),
      $0.SendNotificationResponse.fromBuffer);
  static final _$sendBulkNotification = $grpc.ClientMethod<
          $0.SendBulkNotificationRequest, $0.SendBulkNotificationResponse>(
      '/voice.notification.v1.NotificationService/SendBulkNotification',
      ($0.SendBulkNotificationRequest value) => value.writeToBuffer(),
      $0.SendBulkNotificationResponse.fromBuffer);
  static final _$relayNotification = $grpc.ClientMethod<
          $0.RelayNotificationRequest, $0.RelayNotificationResponse>(
      '/voice.notification.v1.NotificationService/RelayNotification',
      ($0.RelayNotificationRequest value) => value.writeToBuffer(),
      $0.RelayNotificationResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.notification.v1.NotificationService')
abstract class NotificationServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.notification.v1.NotificationService';

  NotificationServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RegisterDeviceRequest,
            $0.RegisterDeviceResponse>(
        'RegisterDevice',
        registerDevice_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RegisterDeviceRequest.fromBuffer(value),
        ($0.RegisterDeviceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UnregisterDeviceRequest,
            $0.UnregisterDeviceResponse>(
        'UnregisterDevice',
        unregisterDevice_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UnregisterDeviceRequest.fromBuffer(value),
        ($0.UnregisterDeviceResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetNotificationSettingsRequest,
            $0.GetNotificationSettingsResponse>(
        'GetNotificationSettings',
        getNotificationSettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetNotificationSettingsRequest.fromBuffer(value),
        ($0.GetNotificationSettingsResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.UpdateNotificationSettingsRequest,
            $0.UpdateNotificationSettingsResponse>(
        'UpdateNotificationSettings',
        updateNotificationSettings_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.UpdateNotificationSettingsRequest.fromBuffer(value),
        ($0.UpdateNotificationSettingsResponse value) =>
            value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.SetQuietHoursRequest, $0.SetQuietHoursResponse>(
            'SetQuietHours',
            setQuietHours_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.SetQuietHoursRequest.fromBuffer(value),
            ($0.SetQuietHoursResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SendNotificationRequest,
            $0.SendNotificationResponse>(
        'SendNotification',
        sendNotification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SendNotificationRequest.fromBuffer(value),
        ($0.SendNotificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SendBulkNotificationRequest,
            $0.SendBulkNotificationResponse>(
        'SendBulkNotification',
        sendBulkNotification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SendBulkNotificationRequest.fromBuffer(value),
        ($0.SendBulkNotificationResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RelayNotificationRequest,
            $0.RelayNotificationResponse>(
        'RelayNotification',
        relayNotification_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RelayNotificationRequest.fromBuffer(value),
        ($0.RelayNotificationResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.RegisterDeviceResponse> registerDevice_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RegisterDeviceRequest> $request) async {
    return registerDevice($call, await $request);
  }

  $async.Future<$0.RegisterDeviceResponse> registerDevice(
      $grpc.ServiceCall call, $0.RegisterDeviceRequest request);

  $async.Future<$0.UnregisterDeviceResponse> unregisterDevice_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.UnregisterDeviceRequest> $request) async {
    return unregisterDevice($call, await $request);
  }

  $async.Future<$0.UnregisterDeviceResponse> unregisterDevice(
      $grpc.ServiceCall call, $0.UnregisterDeviceRequest request);

  $async.Future<$0.GetNotificationSettingsResponse> getNotificationSettings_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetNotificationSettingsRequest> $request) async {
    return getNotificationSettings($call, await $request);
  }

  $async.Future<$0.GetNotificationSettingsResponse> getNotificationSettings(
      $grpc.ServiceCall call, $0.GetNotificationSettingsRequest request);

  $async.Future<$0.UpdateNotificationSettingsResponse>
      updateNotificationSettings_Pre($grpc.ServiceCall $call,
          $async.Future<$0.UpdateNotificationSettingsRequest> $request) async {
    return updateNotificationSettings($call, await $request);
  }

  $async.Future<$0.UpdateNotificationSettingsResponse>
      updateNotificationSettings(
          $grpc.ServiceCall call, $0.UpdateNotificationSettingsRequest request);

  $async.Future<$0.SetQuietHoursResponse> setQuietHours_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetQuietHoursRequest> $request) async {
    return setQuietHours($call, await $request);
  }

  $async.Future<$0.SetQuietHoursResponse> setQuietHours(
      $grpc.ServiceCall call, $0.SetQuietHoursRequest request);

  $async.Future<$0.SendNotificationResponse> sendNotification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SendNotificationRequest> $request) async {
    return sendNotification($call, await $request);
  }

  $async.Future<$0.SendNotificationResponse> sendNotification(
      $grpc.ServiceCall call, $0.SendNotificationRequest request);

  $async.Future<$0.SendBulkNotificationResponse> sendBulkNotification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SendBulkNotificationRequest> $request) async {
    return sendBulkNotification($call, await $request);
  }

  $async.Future<$0.SendBulkNotificationResponse> sendBulkNotification(
      $grpc.ServiceCall call, $0.SendBulkNotificationRequest request);

  $async.Future<$0.RelayNotificationResponse> relayNotification_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RelayNotificationRequest> $request) async {
    return relayNotification($call, await $request);
  }

  $async.Future<$0.RelayNotificationResponse> relayNotification(
      $grpc.ServiceCall call, $0.RelayNotificationRequest request);
}
