// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/auth.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $1;

import 'auth.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'auth.pbenum.dart';

class RegisterRequest extends $pb.GeneratedMessage {
  factory RegisterRequest({
    $core.String? email,
    $core.String? phone,
    $core.String? password,
    $core.bool? guest,
  }) {
    final result = create();
    if (email != null) result.email = email;
    if (phone != null) result.phone = phone;
    if (password != null) result.password = password;
    if (guest != null) result.guest = guest;
    return result;
  }

  RegisterRequest._();

  factory RegisterRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'email')
    ..aOS(2, _omitFieldNames ? '' : 'phone')
    ..aOS(3, _omitFieldNames ? '' : 'password')
    ..aOB(4, _omitFieldNames ? '' : 'guest')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterRequest copyWith(void Function(RegisterRequest) updates) =>
      super.copyWith((message) => updates(message as RegisterRequest))
          as RegisterRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterRequest create() => RegisterRequest._();
  @$core.override
  RegisterRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterRequest>(create);
  static RegisterRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get email => $_getSZ(0);
  @$pb.TagNumber(1)
  set email($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEmail() => $_has(0);
  @$pb.TagNumber(1)
  void clearEmail() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get phone => $_getSZ(1);
  @$pb.TagNumber(2)
  set phone($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPhone() => $_has(1);
  @$pb.TagNumber(2)
  void clearPhone() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get password => $_getSZ(2);
  @$pb.TagNumber(3)
  set password($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPassword() => $_has(2);
  @$pb.TagNumber(3)
  void clearPassword() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.bool get guest => $_getBF(3);
  @$pb.TagNumber(4)
  set guest($core.bool value) => $_setBool(3, value);
  @$pb.TagNumber(4)
  $core.bool hasGuest() => $_has(3);
  @$pb.TagNumber(4)
  void clearGuest() => $_clearField(4);
}

class LoginRequest extends $pb.GeneratedMessage {
  factory LoginRequest({
    $core.String? email,
    $core.String? phone,
    $core.String? password,
    $core.String? totpCode,
    $core.String? deviceInfoJson,
  }) {
    final result = create();
    if (email != null) result.email = email;
    if (phone != null) result.phone = phone;
    if (password != null) result.password = password;
    if (totpCode != null) result.totpCode = totpCode;
    if (deviceInfoJson != null) result.deviceInfoJson = deviceInfoJson;
    return result;
  }

  LoginRequest._();

  factory LoginRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LoginRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LoginRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'email')
    ..aOS(2, _omitFieldNames ? '' : 'phone')
    ..aOS(3, _omitFieldNames ? '' : 'password')
    ..aOS(4, _omitFieldNames ? '' : 'totpCode')
    ..aOS(5, _omitFieldNames ? '' : 'deviceInfoJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginRequest copyWith(void Function(LoginRequest) updates) =>
      super.copyWith((message) => updates(message as LoginRequest))
          as LoginRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LoginRequest create() => LoginRequest._();
  @$core.override
  LoginRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LoginRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LoginRequest>(create);
  static LoginRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get email => $_getSZ(0);
  @$pb.TagNumber(1)
  set email($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEmail() => $_has(0);
  @$pb.TagNumber(1)
  void clearEmail() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get phone => $_getSZ(1);
  @$pb.TagNumber(2)
  set phone($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPhone() => $_has(1);
  @$pb.TagNumber(2)
  void clearPhone() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get password => $_getSZ(2);
  @$pb.TagNumber(3)
  set password($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPassword() => $_has(2);
  @$pb.TagNumber(3)
  void clearPassword() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get totpCode => $_getSZ(3);
  @$pb.TagNumber(4)
  set totpCode($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasTotpCode() => $_has(3);
  @$pb.TagNumber(4)
  void clearTotpCode() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get deviceInfoJson => $_getSZ(4);
  @$pb.TagNumber(5)
  set deviceInfoJson($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasDeviceInfoJson() => $_has(4);
  @$pb.TagNumber(5)
  void clearDeviceInfoJson() => $_clearField(5);
}

class LogoutRequest extends $pb.GeneratedMessage {
  factory LogoutRequest({
    $core.String? refreshToken,
  }) {
    final result = create();
    if (refreshToken != null) result.refreshToken = refreshToken;
    return result;
  }

  LogoutRequest._();

  factory LogoutRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LogoutRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LogoutRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'refreshToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogoutRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogoutRequest copyWith(void Function(LogoutRequest) updates) =>
      super.copyWith((message) => updates(message as LogoutRequest))
          as LogoutRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LogoutRequest create() => LogoutRequest._();
  @$core.override
  LogoutRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LogoutRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LogoutRequest>(create);
  static LogoutRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get refreshToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set refreshToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRefreshToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearRefreshToken() => $_clearField(1);
}

class RefreshTokenRequest extends $pb.GeneratedMessage {
  factory RefreshTokenRequest({
    $core.String? refreshToken,
    $core.String? deviceInfoJson,
  }) {
    final result = create();
    if (refreshToken != null) result.refreshToken = refreshToken;
    if (deviceInfoJson != null) result.deviceInfoJson = deviceInfoJson;
    return result;
  }

  RefreshTokenRequest._();

  factory RefreshTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RefreshTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RefreshTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'refreshToken')
    ..aOS(2, _omitFieldNames ? '' : 'deviceInfoJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RefreshTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RefreshTokenRequest copyWith(void Function(RefreshTokenRequest) updates) =>
      super.copyWith((message) => updates(message as RefreshTokenRequest))
          as RefreshTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RefreshTokenRequest create() => RefreshTokenRequest._();
  @$core.override
  RefreshTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RefreshTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RefreshTokenRequest>(create);
  static RefreshTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get refreshToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set refreshToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRefreshToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearRefreshToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get deviceInfoJson => $_getSZ(1);
  @$pb.TagNumber(2)
  set deviceInfoJson($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasDeviceInfoJson() => $_has(1);
  @$pb.TagNumber(2)
  void clearDeviceInfoJson() => $_clearField(2);
}

class Enable2FARequest extends $pb.GeneratedMessage {
  factory Enable2FARequest({
    $core.String? password,
  }) {
    final result = create();
    if (password != null) result.password = password;
    return result;
  }

  Enable2FARequest._();

  factory Enable2FARequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Enable2FARequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Enable2FARequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'password')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Enable2FARequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Enable2FARequest copyWith(void Function(Enable2FARequest) updates) =>
      super.copyWith((message) => updates(message as Enable2FARequest))
          as Enable2FARequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Enable2FARequest create() => Enable2FARequest._();
  @$core.override
  Enable2FARequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Enable2FARequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Enable2FARequest>(create);
  static Enable2FARequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get password => $_getSZ(0);
  @$pb.TagNumber(1)
  set password($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPassword() => $_has(0);
  @$pb.TagNumber(1)
  void clearPassword() => $_clearField(1);
}

class Enable2FAResponse extends $pb.GeneratedMessage {
  factory Enable2FAResponse({
    $core.String? totpUri,
    $core.String? secretBackupHint,
    $core.Iterable<$core.String>? backupCodes,
  }) {
    final result = create();
    if (totpUri != null) result.totpUri = totpUri;
    if (secretBackupHint != null) result.secretBackupHint = secretBackupHint;
    if (backupCodes != null) result.backupCodes.addAll(backupCodes);
    return result;
  }

  Enable2FAResponse._();

  factory Enable2FAResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Enable2FAResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Enable2FAResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'totpUri')
    ..aOS(2, _omitFieldNames ? '' : 'secretBackupHint')
    ..pPS(3, _omitFieldNames ? '' : 'backupCodes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Enable2FAResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Enable2FAResponse copyWith(void Function(Enable2FAResponse) updates) =>
      super.copyWith((message) => updates(message as Enable2FAResponse))
          as Enable2FAResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Enable2FAResponse create() => Enable2FAResponse._();
  @$core.override
  Enable2FAResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Enable2FAResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Enable2FAResponse>(create);
  static Enable2FAResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get totpUri => $_getSZ(0);
  @$pb.TagNumber(1)
  set totpUri($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotpUri() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotpUri() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get secretBackupHint => $_getSZ(1);
  @$pb.TagNumber(2)
  set secretBackupHint($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSecretBackupHint() => $_has(1);
  @$pb.TagNumber(2)
  void clearSecretBackupHint() => $_clearField(2);

  /// One-time plaintext backup codes; shown only at enrollment (docs/features/auth-and-contacts.md).
  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get backupCodes => $_getList(2);
}

class Verify2FARequest extends $pb.GeneratedMessage {
  factory Verify2FARequest({
    $core.String? totpCode,
  }) {
    final result = create();
    if (totpCode != null) result.totpCode = totpCode;
    return result;
  }

  Verify2FARequest._();

  factory Verify2FARequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Verify2FARequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Verify2FARequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'totpCode')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Verify2FARequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Verify2FARequest copyWith(void Function(Verify2FARequest) updates) =>
      super.copyWith((message) => updates(message as Verify2FARequest))
          as Verify2FARequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Verify2FARequest create() => Verify2FARequest._();
  @$core.override
  Verify2FARequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Verify2FARequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Verify2FARequest>(create);
  static Verify2FARequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get totpCode => $_getSZ(0);
  @$pb.TagNumber(1)
  set totpCode($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasTotpCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearTotpCode() => $_clearField(1);
}

class VerifyOTPRequest extends $pb.GeneratedMessage {
  factory VerifyOTPRequest({
    $core.String? code,
    $core.String? otpType,
    OtpType? otpTypeEnum,
  }) {
    final result = create();
    if (code != null) result.code = code;
    if (otpType != null) result.otpType = otpType;
    if (otpTypeEnum != null) result.otpTypeEnum = otpTypeEnum;
    return result;
  }

  VerifyOTPRequest._();

  factory VerifyOTPRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VerifyOTPRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerifyOTPRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'code')
    ..aOS(2, _omitFieldNames ? '' : 'otpType')
    ..aE<OtpType>(3, _omitFieldNames ? '' : 'otpTypeEnum',
        enumValues: OtpType.values)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifyOTPRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifyOTPRequest copyWith(void Function(VerifyOTPRequest) updates) =>
      super.copyWith((message) => updates(message as VerifyOTPRequest))
          as VerifyOTPRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VerifyOTPRequest create() => VerifyOTPRequest._();
  @$core.override
  VerifyOTPRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VerifyOTPRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerifyOTPRequest>(create);
  static VerifyOTPRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get code => $_getSZ(0);
  @$pb.TagNumber(1)
  set code($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCode() => $_has(0);
  @$pb.TagNumber(1)
  void clearCode() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get otpType => $_getSZ(1);
  @$pb.TagNumber(2)
  set otpType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOtpType() => $_has(1);
  @$pb.TagNumber(2)
  void clearOtpType() => $_clearField(2);

  @$pb.TagNumber(3)
  OtpType get otpTypeEnum => $_getN(2);
  @$pb.TagNumber(3)
  set otpTypeEnum(OtpType value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasOtpTypeEnum() => $_has(2);
  @$pb.TagNumber(3)
  void clearOtpTypeEnum() => $_clearField(3);
}

class ConvertGuestRequest extends $pb.GeneratedMessage {
  factory ConvertGuestRequest({
    $core.String? email,
    $core.String? phone,
    $core.String? password,
  }) {
    final result = create();
    if (email != null) result.email = email;
    if (phone != null) result.phone = phone;
    if (password != null) result.password = password;
    return result;
  }

  ConvertGuestRequest._();

  factory ConvertGuestRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConvertGuestRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConvertGuestRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'email')
    ..aOS(2, _omitFieldNames ? '' : 'phone')
    ..aOS(3, _omitFieldNames ? '' : 'password')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConvertGuestRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConvertGuestRequest copyWith(void Function(ConvertGuestRequest) updates) =>
      super.copyWith((message) => updates(message as ConvertGuestRequest))
          as ConvertGuestRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConvertGuestRequest create() => ConvertGuestRequest._();
  @$core.override
  ConvertGuestRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConvertGuestRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConvertGuestRequest>(create);
  static ConvertGuestRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get email => $_getSZ(0);
  @$pb.TagNumber(1)
  set email($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEmail() => $_has(0);
  @$pb.TagNumber(1)
  void clearEmail() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get phone => $_getSZ(1);
  @$pb.TagNumber(2)
  set phone($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPhone() => $_has(1);
  @$pb.TagNumber(2)
  void clearPhone() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get password => $_getSZ(2);
  @$pb.TagNumber(3)
  set password($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPassword() => $_has(2);
  @$pb.TagNumber(3)
  void clearPassword() => $_clearField(3);
}

class DeleteAccountRequest extends $pb.GeneratedMessage {
  factory DeleteAccountRequest({
    $core.String? password,
  }) {
    final result = create();
    if (password != null) result.password = password;
    return result;
  }

  DeleteAccountRequest._();

  factory DeleteAccountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAccountRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'password')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountRequest copyWith(void Function(DeleteAccountRequest) updates) =>
      super.copyWith((message) => updates(message as DeleteAccountRequest))
          as DeleteAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteAccountRequest create() => DeleteAccountRequest._();
  @$core.override
  DeleteAccountRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAccountRequest>(create);
  static DeleteAccountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get password => $_getSZ(0);
  @$pb.TagNumber(1)
  set password($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPassword() => $_has(0);
  @$pb.TagNumber(1)
  void clearPassword() => $_clearField(1);
}

class RestoreAccountRequest extends $pb.GeneratedMessage {
  factory RestoreAccountRequest({
    $core.String? token,
  }) {
    final result = create();
    if (token != null) result.token = token;
    return result;
  }

  RestoreAccountRequest._();

  factory RestoreAccountRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RestoreAccountRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RestoreAccountRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'token')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestoreAccountRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestoreAccountRequest copyWith(
          void Function(RestoreAccountRequest) updates) =>
      super.copyWith((message) => updates(message as RestoreAccountRequest))
          as RestoreAccountRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RestoreAccountRequest create() => RestoreAccountRequest._();
  @$core.override
  RestoreAccountRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RestoreAccountRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RestoreAccountRequest>(create);
  static RestoreAccountRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get token => $_getSZ(0);
  @$pb.TagNumber(1)
  set token($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearToken() => $_clearField(1);
}

class ValidateTokenRequest extends $pb.GeneratedMessage {
  factory ValidateTokenRequest({
    $core.String? accessToken,
  }) {
    final result = create();
    if (accessToken != null) result.accessToken = accessToken;
    return result;
  }

  ValidateTokenRequest._();

  factory ValidateTokenRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateTokenRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateTokenRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accessToken')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateTokenRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateTokenRequest copyWith(void Function(ValidateTokenRequest) updates) =>
      super.copyWith((message) => updates(message as ValidateTokenRequest))
          as ValidateTokenRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateTokenRequest create() => ValidateTokenRequest._();
  @$core.override
  ValidateTokenRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateTokenRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateTokenRequest>(create);
  static ValidateTokenRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accessToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set accessToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccessToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccessToken() => $_clearField(1);
}

/// Issued session (access + refresh + account id + active profile).
class AuthSession extends $pb.GeneratedMessage {
  factory AuthSession({
    $core.String? accessToken,
    $core.String? refreshToken,
    $fixnum.Int64? expiresInSeconds,
    $core.String? accountId,
    $core.String? profileId,
    $core.String? accountType,
  }) {
    final result = create();
    if (accessToken != null) result.accessToken = accessToken;
    if (refreshToken != null) result.refreshToken = refreshToken;
    if (expiresInSeconds != null) result.expiresInSeconds = expiresInSeconds;
    if (accountId != null) result.accountId = accountId;
    if (profileId != null) result.profileId = profileId;
    if (accountType != null) result.accountType = accountType;
    return result;
  }

  AuthSession._();

  factory AuthSession.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory AuthSession.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'AuthSession',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accessToken')
    ..aOS(2, _omitFieldNames ? '' : 'refreshToken')
    ..aInt64(3, _omitFieldNames ? '' : 'expiresInSeconds')
    ..aOS(4, _omitFieldNames ? '' : 'accountId')
    ..aOS(5, _omitFieldNames ? '' : 'profileId')
    ..aOS(6, _omitFieldNames ? '' : 'accountType')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthSession clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  AuthSession copyWith(void Function(AuthSession) updates) =>
      super.copyWith((message) => updates(message as AuthSession))
          as AuthSession;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static AuthSession create() => AuthSession._();
  @$core.override
  AuthSession createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static AuthSession getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<AuthSession>(create);
  static AuthSession? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accessToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set accessToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccessToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccessToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get refreshToken => $_getSZ(1);
  @$pb.TagNumber(2)
  set refreshToken($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRefreshToken() => $_has(1);
  @$pb.TagNumber(2)
  void clearRefreshToken() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get expiresInSeconds => $_getI64(2);
  @$pb.TagNumber(3)
  set expiresInSeconds($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpiresInSeconds() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpiresInSeconds() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get accountId => $_getSZ(3);
  @$pb.TagNumber(4)
  set accountId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasAccountId() => $_has(3);
  @$pb.TagNumber(4)
  void clearAccountId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get profileId => $_getSZ(4);
  @$pb.TagNumber(5)
  set profileId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasProfileId() => $_has(4);
  @$pb.TagNumber(5)
  void clearProfileId() => $_clearField(5);

  /// regular | guest — mirrors JWT account_type claim.
  @$pb.TagNumber(6)
  $core.String get accountType => $_getSZ(5);
  @$pb.TagNumber(6)
  set accountType($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasAccountType() => $_has(5);
  @$pb.TagNumber(6)
  void clearAccountType() => $_clearField(6);
}

class RegisterResponse extends $pb.GeneratedMessage {
  factory RegisterResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  RegisterResponse._();

  factory RegisterResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RegisterResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RegisterResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RegisterResponse copyWith(void Function(RegisterResponse) updates) =>
      super.copyWith((message) => updates(message as RegisterResponse))
          as RegisterResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RegisterResponse create() => RegisterResponse._();
  @$core.override
  RegisterResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RegisterResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RegisterResponse>(create);
  static RegisterResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class LoginResponse extends $pb.GeneratedMessage {
  factory LoginResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  LoginResponse._();

  factory LoginResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LoginResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LoginResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LoginResponse copyWith(void Function(LoginResponse) updates) =>
      super.copyWith((message) => updates(message as LoginResponse))
          as LoginResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LoginResponse create() => LoginResponse._();
  @$core.override
  LoginResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LoginResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LoginResponse>(create);
  static LoginResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class LogoutResponse extends $pb.GeneratedMessage {
  factory LogoutResponse() => create();

  LogoutResponse._();

  factory LogoutResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory LogoutResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'LogoutResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogoutResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  LogoutResponse copyWith(void Function(LogoutResponse) updates) =>
      super.copyWith((message) => updates(message as LogoutResponse))
          as LogoutResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static LogoutResponse create() => LogoutResponse._();
  @$core.override
  LogoutResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static LogoutResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<LogoutResponse>(create);
  static LogoutResponse? _defaultInstance;
}

class RefreshTokenResponse extends $pb.GeneratedMessage {
  factory RefreshTokenResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  RefreshTokenResponse._();

  factory RefreshTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RefreshTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RefreshTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RefreshTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RefreshTokenResponse copyWith(void Function(RefreshTokenResponse) updates) =>
      super.copyWith((message) => updates(message as RefreshTokenResponse))
          as RefreshTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RefreshTokenResponse create() => RefreshTokenResponse._();
  @$core.override
  RefreshTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RefreshTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RefreshTokenResponse>(create);
  static RefreshTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class Verify2FAResponse extends $pb.GeneratedMessage {
  factory Verify2FAResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  Verify2FAResponse._();

  factory Verify2FAResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Verify2FAResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Verify2FAResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Verify2FAResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Verify2FAResponse copyWith(void Function(Verify2FAResponse) updates) =>
      super.copyWith((message) => updates(message as Verify2FAResponse))
          as Verify2FAResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Verify2FAResponse create() => Verify2FAResponse._();
  @$core.override
  Verify2FAResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Verify2FAResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Verify2FAResponse>(create);
  static Verify2FAResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class VerifyOTPResponse extends $pb.GeneratedMessage {
  factory VerifyOTPResponse() => create();

  VerifyOTPResponse._();

  factory VerifyOTPResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VerifyOTPResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerifyOTPResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifyOTPResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifyOTPResponse copyWith(void Function(VerifyOTPResponse) updates) =>
      super.copyWith((message) => updates(message as VerifyOTPResponse))
          as VerifyOTPResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VerifyOTPResponse create() => VerifyOTPResponse._();
  @$core.override
  VerifyOTPResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VerifyOTPResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerifyOTPResponse>(create);
  static VerifyOTPResponse? _defaultInstance;
}

class ConvertGuestResponse extends $pb.GeneratedMessage {
  factory ConvertGuestResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  ConvertGuestResponse._();

  factory ConvertGuestResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ConvertGuestResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ConvertGuestResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConvertGuestResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ConvertGuestResponse copyWith(void Function(ConvertGuestResponse) updates) =>
      super.copyWith((message) => updates(message as ConvertGuestResponse))
          as ConvertGuestResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ConvertGuestResponse create() => ConvertGuestResponse._();
  @$core.override
  ConvertGuestResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ConvertGuestResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ConvertGuestResponse>(create);
  static ConvertGuestResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class DeleteAccountResponse extends $pb.GeneratedMessage {
  factory DeleteAccountResponse() => create();

  DeleteAccountResponse._();

  factory DeleteAccountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DeleteAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DeleteAccountResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DeleteAccountResponse copyWith(
          void Function(DeleteAccountResponse) updates) =>
      super.copyWith((message) => updates(message as DeleteAccountResponse))
          as DeleteAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DeleteAccountResponse create() => DeleteAccountResponse._();
  @$core.override
  DeleteAccountResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DeleteAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DeleteAccountResponse>(create);
  static DeleteAccountResponse? _defaultInstance;
}

class RestoreAccountResponse extends $pb.GeneratedMessage {
  factory RestoreAccountResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  RestoreAccountResponse._();

  factory RestoreAccountResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory RestoreAccountResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'RestoreAccountResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestoreAccountResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  RestoreAccountResponse copyWith(
          void Function(RestoreAccountResponse) updates) =>
      super.copyWith((message) => updates(message as RestoreAccountResponse))
          as RestoreAccountResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static RestoreAccountResponse create() => RestoreAccountResponse._();
  @$core.override
  RestoreAccountResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static RestoreAccountResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<RestoreAccountResponse>(create);
  static RestoreAccountResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class ValidateTokenResponse extends $pb.GeneratedMessage {
  factory ValidateTokenResponse({
    TokenClaims? claims,
  }) {
    final result = create();
    if (claims != null) result.claims = claims;
    return result;
  }

  ValidateTokenResponse._();

  factory ValidateTokenResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ValidateTokenResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ValidateTokenResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<TokenClaims>(1, _omitFieldNames ? '' : 'claims',
        subBuilder: TokenClaims.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateTokenResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ValidateTokenResponse copyWith(
          void Function(ValidateTokenResponse) updates) =>
      super.copyWith((message) => updates(message as ValidateTokenResponse))
          as ValidateTokenResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ValidateTokenResponse create() => ValidateTokenResponse._();
  @$core.override
  ValidateTokenResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ValidateTokenResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ValidateTokenResponse>(create);
  static ValidateTokenResponse? _defaultInstance;

  @$pb.TagNumber(1)
  TokenClaims get claims => $_getN(0);
  @$pb.TagNumber(1)
  set claims(TokenClaims value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasClaims() => $_has(0);
  @$pb.TagNumber(1)
  void clearClaims() => $_clearField(1);
  @$pb.TagNumber(1)
  TokenClaims ensureClaims() => $_ensure(0);
}

class GetJWKSRequest extends $pb.GeneratedMessage {
  factory GetJWKSRequest() => create();

  GetJWKSRequest._();

  factory GetJWKSRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJWKSRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJWKSRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJWKSRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJWKSRequest copyWith(void Function(GetJWKSRequest) updates) =>
      super.copyWith((message) => updates(message as GetJWKSRequest))
          as GetJWKSRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJWKSRequest create() => GetJWKSRequest._();
  @$core.override
  GetJWKSRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJWKSRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJWKSRequest>(create);
  static GetJWKSRequest? _defaultInstance;
}

class GetJWKSResponse extends $pb.GeneratedMessage {
  factory GetJWKSResponse({
    $core.String? keysJson,
  }) {
    final result = create();
    if (keysJson != null) result.keysJson = keysJson;
    return result;
  }

  GetJWKSResponse._();

  factory GetJWKSResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetJWKSResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetJWKSResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'keysJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJWKSResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetJWKSResponse copyWith(void Function(GetJWKSResponse) updates) =>
      super.copyWith((message) => updates(message as GetJWKSResponse))
          as GetJWKSResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetJWKSResponse create() => GetJWKSResponse._();
  @$core.override
  GetJWKSResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetJWKSResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetJWKSResponse>(create);
  static GetJWKSResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get keysJson => $_getSZ(0);
  @$pb.TagNumber(1)
  set keysJson($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasKeysJson() => $_has(0);
  @$pb.TagNumber(1)
  void clearKeysJson() => $_clearField(1);
}

class TokenClaims extends $pb.GeneratedMessage {
  factory TokenClaims({
    $core.String? userId,
    $core.String? profileId,
    $core.Iterable<$core.String>? roles,
    $core.String? subscriptionTier,
    $1.Timestamp? expiresAt,
    $core.String? accountType,
  }) {
    final result = create();
    if (userId != null) result.userId = userId;
    if (profileId != null) result.profileId = profileId;
    if (roles != null) result.roles.addAll(roles);
    if (subscriptionTier != null) result.subscriptionTier = subscriptionTier;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (accountType != null) result.accountType = accountType;
    return result;
  }

  TokenClaims._();

  factory TokenClaims.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory TokenClaims.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'TokenClaims',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'userId')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..pPS(3, _omitFieldNames ? '' : 'roles')
    ..aOS(4, _omitFieldNames ? '' : 'subscriptionTier')
    ..aOM<$1.Timestamp>(5, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(6, _omitFieldNames ? '' : 'accountType')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TokenClaims clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  TokenClaims copyWith(void Function(TokenClaims) updates) =>
      super.copyWith((message) => updates(message as TokenClaims))
          as TokenClaims;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static TokenClaims create() => TokenClaims._();
  @$core.override
  TokenClaims createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static TokenClaims getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<TokenClaims>(create);
  static TokenClaims? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get userId => $_getSZ(0);
  @$pb.TagNumber(1)
  set userId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasUserId() => $_has(0);
  @$pb.TagNumber(1)
  void clearUserId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $pb.PbList<$core.String> get roles => $_getList(2);

  @$pb.TagNumber(4)
  $core.String get subscriptionTier => $_getSZ(3);
  @$pb.TagNumber(4)
  set subscriptionTier($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSubscriptionTier() => $_has(3);
  @$pb.TagNumber(4)
  void clearSubscriptionTier() => $_clearField(4);

  @$pb.TagNumber(5)
  $1.Timestamp get expiresAt => $_getN(4);
  @$pb.TagNumber(5)
  set expiresAt($1.Timestamp value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasExpiresAt() => $_has(4);
  @$pb.TagNumber(5)
  void clearExpiresAt() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.Timestamp ensureExpiresAt() => $_ensure(4);

  /// regular | guest — mirrors JWT account_type claim.
  @$pb.TagNumber(6)
  $core.String get accountType => $_getSZ(5);
  @$pb.TagNumber(6)
  set accountType($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasAccountType() => $_has(5);
  @$pb.TagNumber(6)
  void clearAccountType() => $_clearField(6);
}

class SwitchActiveProfileRequest extends $pb.GeneratedMessage {
  factory SwitchActiveProfileRequest({
    $core.String? accessToken,
    $core.String? profileId,
    $core.String? deviceInfoJson,
  }) {
    final result = create();
    if (accessToken != null) result.accessToken = accessToken;
    if (profileId != null) result.profileId = profileId;
    if (deviceInfoJson != null) result.deviceInfoJson = deviceInfoJson;
    return result;
  }

  SwitchActiveProfileRequest._();

  factory SwitchActiveProfileRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SwitchActiveProfileRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SwitchActiveProfileRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accessToken')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..aOS(3, _omitFieldNames ? '' : 'deviceInfoJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchActiveProfileRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchActiveProfileRequest copyWith(
          void Function(SwitchActiveProfileRequest) updates) =>
      super.copyWith(
              (message) => updates(message as SwitchActiveProfileRequest))
          as SwitchActiveProfileRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SwitchActiveProfileRequest create() => SwitchActiveProfileRequest._();
  @$core.override
  SwitchActiveProfileRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SwitchActiveProfileRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SwitchActiveProfileRequest>(create);
  static SwitchActiveProfileRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accessToken => $_getSZ(0);
  @$pb.TagNumber(1)
  set accessToken($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccessToken() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccessToken() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get deviceInfoJson => $_getSZ(2);
  @$pb.TagNumber(3)
  set deviceInfoJson($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDeviceInfoJson() => $_has(2);
  @$pb.TagNumber(3)
  void clearDeviceInfoJson() => $_clearField(3);
}

class SwitchActiveProfileResponse extends $pb.GeneratedMessage {
  factory SwitchActiveProfileResponse({
    AuthSession? session,
  }) {
    final result = create();
    if (session != null) result.session = session;
    return result;
  }

  SwitchActiveProfileResponse._();

  factory SwitchActiveProfileResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SwitchActiveProfileResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SwitchActiveProfileResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOM<AuthSession>(1, _omitFieldNames ? '' : 'session',
        subBuilder: AuthSession.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchActiveProfileResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SwitchActiveProfileResponse copyWith(
          void Function(SwitchActiveProfileResponse) updates) =>
      super.copyWith(
              (message) => updates(message as SwitchActiveProfileResponse))
          as SwitchActiveProfileResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SwitchActiveProfileResponse create() =>
      SwitchActiveProfileResponse._();
  @$core.override
  SwitchActiveProfileResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SwitchActiveProfileResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SwitchActiveProfileResponse>(create);
  static SwitchActiveProfileResponse? _defaultInstance;

  @$pb.TagNumber(1)
  AuthSession get session => $_getN(0);
  @$pb.TagNumber(1)
  set session(AuthSession value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSession() => $_has(0);
  @$pb.TagNumber(1)
  void clearSession() => $_clearField(1);
  @$pb.TagNumber(1)
  AuthSession ensureSession() => $_ensure(0);
}

class SetAccountStatusRequest extends $pb.GeneratedMessage {
  factory SetAccountStatusRequest({
    $core.String? accountId,
    $core.String? status,
    $core.String? reason,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (status != null) result.status = status;
    if (reason != null) result.reason = reason;
    return result;
  }

  SetAccountStatusRequest._();

  factory SetAccountStatusRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetAccountStatusRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetAccountStatusRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'status')
    ..aOS(3, _omitFieldNames ? '' : 'reason')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetAccountStatusRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetAccountStatusRequest copyWith(
          void Function(SetAccountStatusRequest) updates) =>
      super.copyWith((message) => updates(message as SetAccountStatusRequest))
          as SetAccountStatusRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetAccountStatusRequest create() => SetAccountStatusRequest._();
  @$core.override
  SetAccountStatusRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetAccountStatusRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetAccountStatusRequest>(create);
  static SetAccountStatusRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get status => $_getSZ(1);
  @$pb.TagNumber(2)
  set status($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasStatus() => $_has(1);
  @$pb.TagNumber(2)
  void clearStatus() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get reason => $_getSZ(2);
  @$pb.TagNumber(3)
  set reason($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasReason() => $_has(2);
  @$pb.TagNumber(3)
  void clearReason() => $_clearField(3);
}

class SetAccountStatusResponse extends $pb.GeneratedMessage {
  factory SetAccountStatusResponse() => create();

  SetAccountStatusResponse._();

  factory SetAccountStatusResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SetAccountStatusResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SetAccountStatusResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetAccountStatusResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SetAccountStatusResponse copyWith(
          void Function(SetAccountStatusResponse) updates) =>
      super.copyWith((message) => updates(message as SetAccountStatusResponse))
          as SetAccountStatusResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SetAccountStatusResponse create() => SetAccountStatusResponse._();
  @$core.override
  SetAccountStatusResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SetAccountStatusResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SetAccountStatusResponse>(create);
  static SetAccountStatusResponse? _defaultInstance;
}

class PutE2EKeyBackupRequest extends $pb.GeneratedMessage {
  factory PutE2EKeyBackupRequest({
    $core.String? encryptedBlob,
    $core.String? passwordHint,
  }) {
    final result = create();
    if (encryptedBlob != null) result.encryptedBlob = encryptedBlob;
    if (passwordHint != null) result.passwordHint = passwordHint;
    return result;
  }

  PutE2EKeyBackupRequest._();

  factory PutE2EKeyBackupRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PutE2EKeyBackupRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PutE2EKeyBackupRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'encryptedBlob')
    ..aOS(2, _omitFieldNames ? '' : 'passwordHint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutE2EKeyBackupRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutE2EKeyBackupRequest copyWith(
          void Function(PutE2EKeyBackupRequest) updates) =>
      super.copyWith((message) => updates(message as PutE2EKeyBackupRequest))
          as PutE2EKeyBackupRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PutE2EKeyBackupRequest create() => PutE2EKeyBackupRequest._();
  @$core.override
  PutE2EKeyBackupRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PutE2EKeyBackupRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PutE2EKeyBackupRequest>(create);
  static PutE2EKeyBackupRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get encryptedBlob => $_getSZ(0);
  @$pb.TagNumber(1)
  set encryptedBlob($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEncryptedBlob() => $_has(0);
  @$pb.TagNumber(1)
  void clearEncryptedBlob() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get passwordHint => $_getSZ(1);
  @$pb.TagNumber(2)
  set passwordHint($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPasswordHint() => $_has(1);
  @$pb.TagNumber(2)
  void clearPasswordHint() => $_clearField(2);
}

class PutE2EKeyBackupResponse extends $pb.GeneratedMessage {
  factory PutE2EKeyBackupResponse() => create();

  PutE2EKeyBackupResponse._();

  factory PutE2EKeyBackupResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PutE2EKeyBackupResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PutE2EKeyBackupResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutE2EKeyBackupResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PutE2EKeyBackupResponse copyWith(
          void Function(PutE2EKeyBackupResponse) updates) =>
      super.copyWith((message) => updates(message as PutE2EKeyBackupResponse))
          as PutE2EKeyBackupResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PutE2EKeyBackupResponse create() => PutE2EKeyBackupResponse._();
  @$core.override
  PutE2EKeyBackupResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PutE2EKeyBackupResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PutE2EKeyBackupResponse>(create);
  static PutE2EKeyBackupResponse? _defaultInstance;
}

class GetE2EKeyBackupRequest extends $pb.GeneratedMessage {
  factory GetE2EKeyBackupRequest() => create();

  GetE2EKeyBackupRequest._();

  factory GetE2EKeyBackupRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetE2EKeyBackupRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetE2EKeyBackupRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetE2EKeyBackupRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetE2EKeyBackupRequest copyWith(
          void Function(GetE2EKeyBackupRequest) updates) =>
      super.copyWith((message) => updates(message as GetE2EKeyBackupRequest))
          as GetE2EKeyBackupRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetE2EKeyBackupRequest create() => GetE2EKeyBackupRequest._();
  @$core.override
  GetE2EKeyBackupRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetE2EKeyBackupRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetE2EKeyBackupRequest>(create);
  static GetE2EKeyBackupRequest? _defaultInstance;
}

class GetE2EKeyBackupResponse extends $pb.GeneratedMessage {
  factory GetE2EKeyBackupResponse({
    $core.String? encryptedBlob,
    $core.String? passwordHint,
  }) {
    final result = create();
    if (encryptedBlob != null) result.encryptedBlob = encryptedBlob;
    if (passwordHint != null) result.passwordHint = passwordHint;
    return result;
  }

  GetE2EKeyBackupResponse._();

  factory GetE2EKeyBackupResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetE2EKeyBackupResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetE2EKeyBackupResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'encryptedBlob')
    ..aOS(2, _omitFieldNames ? '' : 'passwordHint')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetE2EKeyBackupResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetE2EKeyBackupResponse copyWith(
          void Function(GetE2EKeyBackupResponse) updates) =>
      super.copyWith((message) => updates(message as GetE2EKeyBackupResponse))
          as GetE2EKeyBackupResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetE2EKeyBackupResponse create() => GetE2EKeyBackupResponse._();
  @$core.override
  GetE2EKeyBackupResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetE2EKeyBackupResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetE2EKeyBackupResponse>(create);
  static GetE2EKeyBackupResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get encryptedBlob => $_getSZ(0);
  @$pb.TagNumber(1)
  set encryptedBlob($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasEncryptedBlob() => $_has(0);
  @$pb.TagNumber(1)
  void clearEncryptedBlob() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get passwordHint => $_getSZ(1);
  @$pb.TagNumber(2)
  set passwordHint($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPasswordHint() => $_has(1);
  @$pb.TagNumber(2)
  void clearPasswordHint() => $_clearField(2);
}

class ResolvePhoneHashesRequest extends $pb.GeneratedMessage {
  factory ResolvePhoneHashesRequest({
    $core.Iterable<$core.String>? phoneHashes,
  }) {
    final result = create();
    if (phoneHashes != null) result.phoneHashes.addAll(phoneHashes);
    return result;
  }

  ResolvePhoneHashesRequest._();

  factory ResolvePhoneHashesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolvePhoneHashesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolvePhoneHashesRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'phoneHashes')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolvePhoneHashesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolvePhoneHashesRequest copyWith(
          void Function(ResolvePhoneHashesRequest) updates) =>
      super.copyWith((message) => updates(message as ResolvePhoneHashesRequest))
          as ResolvePhoneHashesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolvePhoneHashesRequest create() => ResolvePhoneHashesRequest._();
  @$core.override
  ResolvePhoneHashesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolvePhoneHashesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolvePhoneHashesRequest>(create);
  static ResolvePhoneHashesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get phoneHashes => $_getList(0);
}

class PhoneHashProfileMatch extends $pb.GeneratedMessage {
  factory PhoneHashProfileMatch({
    $core.String? phoneHash,
    $core.String? profileId,
  }) {
    final result = create();
    if (phoneHash != null) result.phoneHash = phoneHash;
    if (profileId != null) result.profileId = profileId;
    return result;
  }

  PhoneHashProfileMatch._();

  factory PhoneHashProfileMatch.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory PhoneHashProfileMatch.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'PhoneHashProfileMatch',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'phoneHash')
    ..aOS(2, _omitFieldNames ? '' : 'profileId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PhoneHashProfileMatch clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  PhoneHashProfileMatch copyWith(
          void Function(PhoneHashProfileMatch) updates) =>
      super.copyWith((message) => updates(message as PhoneHashProfileMatch))
          as PhoneHashProfileMatch;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static PhoneHashProfileMatch create() => PhoneHashProfileMatch._();
  @$core.override
  PhoneHashProfileMatch createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static PhoneHashProfileMatch getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<PhoneHashProfileMatch>(create);
  static PhoneHashProfileMatch? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get phoneHash => $_getSZ(0);
  @$pb.TagNumber(1)
  set phoneHash($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPhoneHash() => $_has(0);
  @$pb.TagNumber(1)
  void clearPhoneHash() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get profileId => $_getSZ(1);
  @$pb.TagNumber(2)
  set profileId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProfileId() => $_has(1);
  @$pb.TagNumber(2)
  void clearProfileId() => $_clearField(2);
}

class ResolvePhoneHashesResponse extends $pb.GeneratedMessage {
  factory ResolvePhoneHashesResponse({
    $core.Iterable<PhoneHashProfileMatch>? matches,
  }) {
    final result = create();
    if (matches != null) result.matches.addAll(matches);
    return result;
  }

  ResolvePhoneHashesResponse._();

  factory ResolvePhoneHashesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResolvePhoneHashesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResolvePhoneHashesResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.auth.v1'),
      createEmptyInstance: create)
    ..pPM<PhoneHashProfileMatch>(1, _omitFieldNames ? '' : 'matches',
        subBuilder: PhoneHashProfileMatch.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolvePhoneHashesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResolvePhoneHashesResponse copyWith(
          void Function(ResolvePhoneHashesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResolvePhoneHashesResponse))
          as ResolvePhoneHashesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResolvePhoneHashesResponse create() => ResolvePhoneHashesResponse._();
  @$core.override
  ResolvePhoneHashesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResolvePhoneHashesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResolvePhoneHashesResponse>(create);
  static ResolvePhoneHashesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<PhoneHashProfileMatch> get matches => $_getList(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
