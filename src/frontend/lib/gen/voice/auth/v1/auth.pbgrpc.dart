// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/auth.proto.

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

import 'auth.pb.dart' as $0;

export 'auth.pb.dart';

/// Auth Service — registration, sessions, JWT, 2FA (Java). HTTP prefix: /api/v1/auth/** → Gateway.
@$pb.GrpcServiceName('voice.auth.v1.AuthService')
class AuthServiceClient extends $grpc.Client {
  /// The hostname for this service.
  static const $core.String defaultHost = '';

  /// OAuth scopes needed for the client.
  static const $core.List<$core.String> oauthScopes = [
    '',
  ];

  AuthServiceClient(super.channel, {super.options, super.interceptors});

  $grpc.ResponseFuture<$0.RegisterResponse> register(
    $0.RegisterRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$register, request, options: options);
  }

  $grpc.ResponseFuture<$0.LoginResponse> login(
    $0.LoginRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$login, request, options: options);
  }

  $grpc.ResponseFuture<$0.LogoutResponse> logout(
    $0.LogoutRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$logout, request, options: options);
  }

  $grpc.ResponseFuture<$0.RefreshTokenResponse> refreshToken(
    $0.RefreshTokenRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$refreshToken, request, options: options);
  }

  $grpc.ResponseFuture<$0.Enable2FAResponse> enable2FA(
    $0.Enable2FARequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$enable2FA, request, options: options);
  }

  $grpc.ResponseFuture<$0.Verify2FAResponse> verify2FA(
    $0.Verify2FARequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$verify2FA, request, options: options);
  }

  $grpc.ResponseFuture<$0.VerifyOTPResponse> verifyOTP(
    $0.VerifyOTPRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$verifyOTP, request, options: options);
  }

  $grpc.ResponseFuture<$0.ConvertGuestResponse> convertGuest(
    $0.ConvertGuestRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$convertGuest, request, options: options);
  }

  $grpc.ResponseFuture<$0.DeleteAccountResponse> deleteAccount(
    $0.DeleteAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$deleteAccount, request, options: options);
  }

  $grpc.ResponseFuture<$0.RestoreAccountResponse> restoreAccount(
    $0.RestoreAccountRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$restoreAccount, request, options: options);
  }

  /// Internal — other services / Gateway token introspection.
  $grpc.ResponseFuture<$0.ValidateTokenResponse> validateToken(
    $0.ValidateTokenRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$validateToken, request, options: options);
  }

  /// Public JWKS for JWT verification.
  $grpc.ResponseFuture<$0.GetJWKSResponse> getJWKS(
    $0.GetJWKSRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getJWKS, request, options: options);
  }

  /// Phase 13: switch active profile claim in next access JWT (primary-profile-bootstrap.md).
  $grpc.ResponseFuture<$0.SwitchActiveProfileResponse> switchActiveProfile(
    $0.SwitchActiveProfileRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$switchActiveProfile, request, options: options);
  }

  /// Phase 14: internal — platform moderation suspends account (docs/PLAN.md phase 14).
  $grpc.ResponseFuture<$0.SetAccountStatusResponse> setAccountStatus(
    $0.SetAccountStatusRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$setAccountStatus, request, options: options);
  }

  /// Phase 15: encrypted key backup (opaque blob; docs/features/encryption.md).
  $grpc.ResponseFuture<$0.PutE2EKeyBackupResponse> putE2EKeyBackup(
    $0.PutE2EKeyBackupRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$putE2EKeyBackup, request, options: options);
  }

  $grpc.ResponseFuture<$0.GetE2EKeyBackupResponse> getE2EKeyBackup(
    $0.GetE2EKeyBackupRequest request, {
    $grpc.CallOptions? options,
  }) {
    return $createUnaryCall(_$getE2EKeyBackup, request, options: options);
  }

  // method descriptors

  static final _$register =
      $grpc.ClientMethod<$0.RegisterRequest, $0.RegisterResponse>(
          '/voice.auth.v1.AuthService/Register',
          ($0.RegisterRequest value) => value.writeToBuffer(),
          $0.RegisterResponse.fromBuffer);
  static final _$login = $grpc.ClientMethod<$0.LoginRequest, $0.LoginResponse>(
      '/voice.auth.v1.AuthService/Login',
      ($0.LoginRequest value) => value.writeToBuffer(),
      $0.LoginResponse.fromBuffer);
  static final _$logout =
      $grpc.ClientMethod<$0.LogoutRequest, $0.LogoutResponse>(
          '/voice.auth.v1.AuthService/Logout',
          ($0.LogoutRequest value) => value.writeToBuffer(),
          $0.LogoutResponse.fromBuffer);
  static final _$refreshToken =
      $grpc.ClientMethod<$0.RefreshTokenRequest, $0.RefreshTokenResponse>(
          '/voice.auth.v1.AuthService/RefreshToken',
          ($0.RefreshTokenRequest value) => value.writeToBuffer(),
          $0.RefreshTokenResponse.fromBuffer);
  static final _$enable2FA =
      $grpc.ClientMethod<$0.Enable2FARequest, $0.Enable2FAResponse>(
          '/voice.auth.v1.AuthService/Enable2FA',
          ($0.Enable2FARequest value) => value.writeToBuffer(),
          $0.Enable2FAResponse.fromBuffer);
  static final _$verify2FA =
      $grpc.ClientMethod<$0.Verify2FARequest, $0.Verify2FAResponse>(
          '/voice.auth.v1.AuthService/Verify2FA',
          ($0.Verify2FARequest value) => value.writeToBuffer(),
          $0.Verify2FAResponse.fromBuffer);
  static final _$verifyOTP =
      $grpc.ClientMethod<$0.VerifyOTPRequest, $0.VerifyOTPResponse>(
          '/voice.auth.v1.AuthService/VerifyOTP',
          ($0.VerifyOTPRequest value) => value.writeToBuffer(),
          $0.VerifyOTPResponse.fromBuffer);
  static final _$convertGuest =
      $grpc.ClientMethod<$0.ConvertGuestRequest, $0.ConvertGuestResponse>(
          '/voice.auth.v1.AuthService/ConvertGuest',
          ($0.ConvertGuestRequest value) => value.writeToBuffer(),
          $0.ConvertGuestResponse.fromBuffer);
  static final _$deleteAccount =
      $grpc.ClientMethod<$0.DeleteAccountRequest, $0.DeleteAccountResponse>(
          '/voice.auth.v1.AuthService/DeleteAccount',
          ($0.DeleteAccountRequest value) => value.writeToBuffer(),
          $0.DeleteAccountResponse.fromBuffer);
  static final _$restoreAccount =
      $grpc.ClientMethod<$0.RestoreAccountRequest, $0.RestoreAccountResponse>(
          '/voice.auth.v1.AuthService/RestoreAccount',
          ($0.RestoreAccountRequest value) => value.writeToBuffer(),
          $0.RestoreAccountResponse.fromBuffer);
  static final _$validateToken =
      $grpc.ClientMethod<$0.ValidateTokenRequest, $0.ValidateTokenResponse>(
          '/voice.auth.v1.AuthService/ValidateToken',
          ($0.ValidateTokenRequest value) => value.writeToBuffer(),
          $0.ValidateTokenResponse.fromBuffer);
  static final _$getJWKS =
      $grpc.ClientMethod<$0.GetJWKSRequest, $0.GetJWKSResponse>(
          '/voice.auth.v1.AuthService/GetJWKS',
          ($0.GetJWKSRequest value) => value.writeToBuffer(),
          $0.GetJWKSResponse.fromBuffer);
  static final _$switchActiveProfile = $grpc.ClientMethod<
          $0.SwitchActiveProfileRequest, $0.SwitchActiveProfileResponse>(
      '/voice.auth.v1.AuthService/SwitchActiveProfile',
      ($0.SwitchActiveProfileRequest value) => value.writeToBuffer(),
      $0.SwitchActiveProfileResponse.fromBuffer);
  static final _$setAccountStatus = $grpc.ClientMethod<
          $0.SetAccountStatusRequest, $0.SetAccountStatusResponse>(
      '/voice.auth.v1.AuthService/SetAccountStatus',
      ($0.SetAccountStatusRequest value) => value.writeToBuffer(),
      $0.SetAccountStatusResponse.fromBuffer);
  static final _$putE2EKeyBackup =
      $grpc.ClientMethod<$0.PutE2EKeyBackupRequest, $0.PutE2EKeyBackupResponse>(
          '/voice.auth.v1.AuthService/PutE2EKeyBackup',
          ($0.PutE2EKeyBackupRequest value) => value.writeToBuffer(),
          $0.PutE2EKeyBackupResponse.fromBuffer);
  static final _$getE2EKeyBackup =
      $grpc.ClientMethod<$0.GetE2EKeyBackupRequest, $0.GetE2EKeyBackupResponse>(
          '/voice.auth.v1.AuthService/GetE2EKeyBackup',
          ($0.GetE2EKeyBackupRequest value) => value.writeToBuffer(),
          $0.GetE2EKeyBackupResponse.fromBuffer);
}

@$pb.GrpcServiceName('voice.auth.v1.AuthService')
abstract class AuthServiceBase extends $grpc.Service {
  $core.String get $name => 'voice.auth.v1.AuthService';

  AuthServiceBase() {
    $addMethod($grpc.ServiceMethod<$0.RegisterRequest, $0.RegisterResponse>(
        'Register',
        register_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.RegisterRequest.fromBuffer(value),
        ($0.RegisterResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LoginRequest, $0.LoginResponse>(
        'Login',
        login_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LoginRequest.fromBuffer(value),
        ($0.LoginResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.LogoutRequest, $0.LogoutResponse>(
        'Logout',
        logout_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.LogoutRequest.fromBuffer(value),
        ($0.LogoutResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.RefreshTokenRequest, $0.RefreshTokenResponse>(
            'RefreshToken',
            refreshToken_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.RefreshTokenRequest.fromBuffer(value),
            ($0.RefreshTokenResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Enable2FARequest, $0.Enable2FAResponse>(
        'Enable2FA',
        enable2FA_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Enable2FARequest.fromBuffer(value),
        ($0.Enable2FAResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.Verify2FARequest, $0.Verify2FAResponse>(
        'Verify2FA',
        verify2FA_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.Verify2FARequest.fromBuffer(value),
        ($0.Verify2FAResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.VerifyOTPRequest, $0.VerifyOTPResponse>(
        'VerifyOTP',
        verifyOTP_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.VerifyOTPRequest.fromBuffer(value),
        ($0.VerifyOTPResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ConvertGuestRequest, $0.ConvertGuestResponse>(
            'ConvertGuest',
            convertGuest_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ConvertGuestRequest.fromBuffer(value),
            ($0.ConvertGuestResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.DeleteAccountRequest, $0.DeleteAccountResponse>(
            'DeleteAccount',
            deleteAccount_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.DeleteAccountRequest.fromBuffer(value),
            ($0.DeleteAccountResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.RestoreAccountRequest,
            $0.RestoreAccountResponse>(
        'RestoreAccount',
        restoreAccount_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.RestoreAccountRequest.fromBuffer(value),
        ($0.RestoreAccountResponse value) => value.writeToBuffer()));
    $addMethod(
        $grpc.ServiceMethod<$0.ValidateTokenRequest, $0.ValidateTokenResponse>(
            'ValidateToken',
            validateToken_Pre,
            false,
            false,
            ($core.List<$core.int> value) =>
                $0.ValidateTokenRequest.fromBuffer(value),
            ($0.ValidateTokenResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetJWKSRequest, $0.GetJWKSResponse>(
        'GetJWKS',
        getJWKS_Pre,
        false,
        false,
        ($core.List<$core.int> value) => $0.GetJWKSRequest.fromBuffer(value),
        ($0.GetJWKSResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SwitchActiveProfileRequest,
            $0.SwitchActiveProfileResponse>(
        'SwitchActiveProfile',
        switchActiveProfile_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SwitchActiveProfileRequest.fromBuffer(value),
        ($0.SwitchActiveProfileResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.SetAccountStatusRequest,
            $0.SetAccountStatusResponse>(
        'SetAccountStatus',
        setAccountStatus_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.SetAccountStatusRequest.fromBuffer(value),
        ($0.SetAccountStatusResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.PutE2EKeyBackupRequest,
            $0.PutE2EKeyBackupResponse>(
        'PutE2EKeyBackup',
        putE2EKeyBackup_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.PutE2EKeyBackupRequest.fromBuffer(value),
        ($0.PutE2EKeyBackupResponse value) => value.writeToBuffer()));
    $addMethod($grpc.ServiceMethod<$0.GetE2EKeyBackupRequest,
            $0.GetE2EKeyBackupResponse>(
        'GetE2EKeyBackup',
        getE2EKeyBackup_Pre,
        false,
        false,
        ($core.List<$core.int> value) =>
            $0.GetE2EKeyBackupRequest.fromBuffer(value),
        ($0.GetE2EKeyBackupResponse value) => value.writeToBuffer()));
  }

  $async.Future<$0.RegisterResponse> register_Pre($grpc.ServiceCall $call,
      $async.Future<$0.RegisterRequest> $request) async {
    return register($call, await $request);
  }

  $async.Future<$0.RegisterResponse> register(
      $grpc.ServiceCall call, $0.RegisterRequest request);

  $async.Future<$0.LoginResponse> login_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.LoginRequest> $request) async {
    return login($call, await $request);
  }

  $async.Future<$0.LoginResponse> login(
      $grpc.ServiceCall call, $0.LoginRequest request);

  $async.Future<$0.LogoutResponse> logout_Pre(
      $grpc.ServiceCall $call, $async.Future<$0.LogoutRequest> $request) async {
    return logout($call, await $request);
  }

  $async.Future<$0.LogoutResponse> logout(
      $grpc.ServiceCall call, $0.LogoutRequest request);

  $async.Future<$0.RefreshTokenResponse> refreshToken_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RefreshTokenRequest> $request) async {
    return refreshToken($call, await $request);
  }

  $async.Future<$0.RefreshTokenResponse> refreshToken(
      $grpc.ServiceCall call, $0.RefreshTokenRequest request);

  $async.Future<$0.Enable2FAResponse> enable2FA_Pre($grpc.ServiceCall $call,
      $async.Future<$0.Enable2FARequest> $request) async {
    return enable2FA($call, await $request);
  }

  $async.Future<$0.Enable2FAResponse> enable2FA(
      $grpc.ServiceCall call, $0.Enable2FARequest request);

  $async.Future<$0.Verify2FAResponse> verify2FA_Pre($grpc.ServiceCall $call,
      $async.Future<$0.Verify2FARequest> $request) async {
    return verify2FA($call, await $request);
  }

  $async.Future<$0.Verify2FAResponse> verify2FA(
      $grpc.ServiceCall call, $0.Verify2FARequest request);

  $async.Future<$0.VerifyOTPResponse> verifyOTP_Pre($grpc.ServiceCall $call,
      $async.Future<$0.VerifyOTPRequest> $request) async {
    return verifyOTP($call, await $request);
  }

  $async.Future<$0.VerifyOTPResponse> verifyOTP(
      $grpc.ServiceCall call, $0.VerifyOTPRequest request);

  $async.Future<$0.ConvertGuestResponse> convertGuest_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ConvertGuestRequest> $request) async {
    return convertGuest($call, await $request);
  }

  $async.Future<$0.ConvertGuestResponse> convertGuest(
      $grpc.ServiceCall call, $0.ConvertGuestRequest request);

  $async.Future<$0.DeleteAccountResponse> deleteAccount_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.DeleteAccountRequest> $request) async {
    return deleteAccount($call, await $request);
  }

  $async.Future<$0.DeleteAccountResponse> deleteAccount(
      $grpc.ServiceCall call, $0.DeleteAccountRequest request);

  $async.Future<$0.RestoreAccountResponse> restoreAccount_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.RestoreAccountRequest> $request) async {
    return restoreAccount($call, await $request);
  }

  $async.Future<$0.RestoreAccountResponse> restoreAccount(
      $grpc.ServiceCall call, $0.RestoreAccountRequest request);

  $async.Future<$0.ValidateTokenResponse> validateToken_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.ValidateTokenRequest> $request) async {
    return validateToken($call, await $request);
  }

  $async.Future<$0.ValidateTokenResponse> validateToken(
      $grpc.ServiceCall call, $0.ValidateTokenRequest request);

  $async.Future<$0.GetJWKSResponse> getJWKS_Pre($grpc.ServiceCall $call,
      $async.Future<$0.GetJWKSRequest> $request) async {
    return getJWKS($call, await $request);
  }

  $async.Future<$0.GetJWKSResponse> getJWKS(
      $grpc.ServiceCall call, $0.GetJWKSRequest request);

  $async.Future<$0.SwitchActiveProfileResponse> switchActiveProfile_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SwitchActiveProfileRequest> $request) async {
    return switchActiveProfile($call, await $request);
  }

  $async.Future<$0.SwitchActiveProfileResponse> switchActiveProfile(
      $grpc.ServiceCall call, $0.SwitchActiveProfileRequest request);

  $async.Future<$0.SetAccountStatusResponse> setAccountStatus_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.SetAccountStatusRequest> $request) async {
    return setAccountStatus($call, await $request);
  }

  $async.Future<$0.SetAccountStatusResponse> setAccountStatus(
      $grpc.ServiceCall call, $0.SetAccountStatusRequest request);

  $async.Future<$0.PutE2EKeyBackupResponse> putE2EKeyBackup_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.PutE2EKeyBackupRequest> $request) async {
    return putE2EKeyBackup($call, await $request);
  }

  $async.Future<$0.PutE2EKeyBackupResponse> putE2EKeyBackup(
      $grpc.ServiceCall call, $0.PutE2EKeyBackupRequest request);

  $async.Future<$0.GetE2EKeyBackupResponse> getE2EKeyBackup_Pre(
      $grpc.ServiceCall $call,
      $async.Future<$0.GetE2EKeyBackupRequest> $request) async {
    return getE2EKeyBackup($call, await $request);
  }

  $async.Future<$0.GetE2EKeyBackupResponse> getE2EKeyBackup(
      $grpc.ServiceCall call, $0.GetE2EKeyBackupRequest request);
}
