// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/auth.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use otpTypeDescriptor instead')
const OtpType$json = {
  '1': 'OtpType',
  '2': [
    {'1': 'OTP_TYPE_UNSPECIFIED', '2': 0},
    {'1': 'OTP_TYPE_EMAIL_VERIFY', '2': 1},
    {'1': 'OTP_TYPE_PASSWORD_RESET', '2': 2},
  ],
};

/// Descriptor for `OtpType`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List otpTypeDescriptor = $convert.base64Decode(
    'CgdPdHBUeXBlEhgKFE9UUF9UWVBFX1VOU1BFQ0lGSUVEEAASGQoVT1RQX1RZUEVfRU1BSUxfVk'
    'VSSUZZEAESGwoXT1RQX1RZUEVfUEFTU1dPUkRfUkVTRVQQAg==');

@$core.Deprecated('Use registerRequestDescriptor instead')
const RegisterRequest$json = {
  '1': 'RegisterRequest',
  '2': [
    {'1': 'email', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'email', '17': true},
    {'1': 'phone', '3': 2, '4': 1, '5': 9, '9': 1, '10': 'phone', '17': true},
    {'1': 'password', '3': 3, '4': 1, '5': 9, '10': 'password'},
    {'1': 'guest', '3': 4, '4': 1, '5': 8, '10': 'guest'},
  ],
  '8': [
    {'1': '_email'},
    {'1': '_phone'},
  ],
};

/// Descriptor for `RegisterRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerRequestDescriptor = $convert.base64Decode(
    'Cg9SZWdpc3RlclJlcXVlc3QSGQoFZW1haWwYASABKAlIAFIFZW1haWyIAQESGQoFcGhvbmUYAi'
    'ABKAlIAVIFcGhvbmWIAQESGgoIcGFzc3dvcmQYAyABKAlSCHBhc3N3b3JkEhQKBWd1ZXN0GAQg'
    'ASgIUgVndWVzdEIICgZfZW1haWxCCAoGX3Bob25l');

@$core.Deprecated('Use loginRequestDescriptor instead')
const LoginRequest$json = {
  '1': 'LoginRequest',
  '2': [
    {'1': 'email', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'email', '17': true},
    {'1': 'phone', '3': 2, '4': 1, '5': 9, '9': 1, '10': 'phone', '17': true},
    {'1': 'password', '3': 3, '4': 1, '5': 9, '10': 'password'},
    {
      '1': 'totp_code',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'totpCode',
      '17': true
    },
    {'1': 'device_info_json', '3': 5, '4': 1, '5': 9, '10': 'deviceInfoJson'},
  ],
  '8': [
    {'1': '_email'},
    {'1': '_phone'},
    {'1': '_totp_code'},
  ],
};

/// Descriptor for `LoginRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List loginRequestDescriptor = $convert.base64Decode(
    'CgxMb2dpblJlcXVlc3QSGQoFZW1haWwYASABKAlIAFIFZW1haWyIAQESGQoFcGhvbmUYAiABKA'
    'lIAVIFcGhvbmWIAQESGgoIcGFzc3dvcmQYAyABKAlSCHBhc3N3b3JkEiAKCXRvdHBfY29kZRgE'
    'IAEoCUgCUgh0b3RwQ29kZYgBARIoChBkZXZpY2VfaW5mb19qc29uGAUgASgJUg5kZXZpY2VJbm'
    'ZvSnNvbkIICgZfZW1haWxCCAoGX3Bob25lQgwKCl90b3RwX2NvZGU=');

@$core.Deprecated('Use logoutRequestDescriptor instead')
const LogoutRequest$json = {
  '1': 'LogoutRequest',
  '2': [
    {'1': 'refresh_token', '3': 1, '4': 1, '5': 9, '10': 'refreshToken'},
  ],
};

/// Descriptor for `LogoutRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List logoutRequestDescriptor = $convert.base64Decode(
    'Cg1Mb2dvdXRSZXF1ZXN0EiMKDXJlZnJlc2hfdG9rZW4YASABKAlSDHJlZnJlc2hUb2tlbg==');

@$core.Deprecated('Use refreshTokenRequestDescriptor instead')
const RefreshTokenRequest$json = {
  '1': 'RefreshTokenRequest',
  '2': [
    {'1': 'refresh_token', '3': 1, '4': 1, '5': 9, '10': 'refreshToken'},
    {'1': 'device_info_json', '3': 2, '4': 1, '5': 9, '10': 'deviceInfoJson'},
  ],
};

/// Descriptor for `RefreshTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List refreshTokenRequestDescriptor = $convert.base64Decode(
    'ChNSZWZyZXNoVG9rZW5SZXF1ZXN0EiMKDXJlZnJlc2hfdG9rZW4YASABKAlSDHJlZnJlc2hUb2'
    'tlbhIoChBkZXZpY2VfaW5mb19qc29uGAIgASgJUg5kZXZpY2VJbmZvSnNvbg==');

@$core.Deprecated('Use enable2FARequestDescriptor instead')
const Enable2FARequest$json = {
  '1': 'Enable2FARequest',
  '2': [
    {'1': 'password', '3': 1, '4': 1, '5': 9, '10': 'password'},
  ],
};

/// Descriptor for `Enable2FARequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List enable2FARequestDescriptor = $convert.base64Decode(
    'ChBFbmFibGUyRkFSZXF1ZXN0EhoKCHBhc3N3b3JkGAEgASgJUghwYXNzd29yZA==');

@$core.Deprecated('Use enable2FAResponseDescriptor instead')
const Enable2FAResponse$json = {
  '1': 'Enable2FAResponse',
  '2': [
    {'1': 'totp_uri', '3': 1, '4': 1, '5': 9, '10': 'totpUri'},
    {
      '1': 'secret_backup_hint',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'secretBackupHint'
    },
    {'1': 'backup_codes', '3': 3, '4': 3, '5': 9, '10': 'backupCodes'},
  ],
};

/// Descriptor for `Enable2FAResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List enable2FAResponseDescriptor = $convert.base64Decode(
    'ChFFbmFibGUyRkFSZXNwb25zZRIZCgh0b3RwX3VyaRgBIAEoCVIHdG90cFVyaRIsChJzZWNyZX'
    'RfYmFja3VwX2hpbnQYAiABKAlSEHNlY3JldEJhY2t1cEhpbnQSIQoMYmFja3VwX2NvZGVzGAMg'
    'AygJUgtiYWNrdXBDb2Rlcw==');

@$core.Deprecated('Use verify2FARequestDescriptor instead')
const Verify2FARequest$json = {
  '1': 'Verify2FARequest',
  '2': [
    {'1': 'totp_code', '3': 1, '4': 1, '5': 9, '10': 'totpCode'},
  ],
};

/// Descriptor for `Verify2FARequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verify2FARequestDescriptor = $convert.base64Decode(
    'ChBWZXJpZnkyRkFSZXF1ZXN0EhsKCXRvdHBfY29kZRgBIAEoCVIIdG90cENvZGU=');

@$core.Deprecated('Use verifyOTPRequestDescriptor instead')
const VerifyOTPRequest$json = {
  '1': 'VerifyOTPRequest',
  '2': [
    {'1': 'code', '3': 1, '4': 1, '5': 9, '10': 'code'},
    {'1': 'otp_type', '3': 2, '4': 1, '5': 9, '10': 'otpType'},
    {
      '1': 'otp_type_enum',
      '3': 3,
      '4': 1,
      '5': 14,
      '6': '.voice.auth.v1.OtpType',
      '9': 0,
      '10': 'otpTypeEnum',
      '17': true
    },
  ],
  '8': [
    {'1': '_otp_type_enum'},
  ],
};

/// Descriptor for `VerifyOTPRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifyOTPRequestDescriptor = $convert.base64Decode(
    'ChBWZXJpZnlPVFBSZXF1ZXN0EhIKBGNvZGUYASABKAlSBGNvZGUSGQoIb3RwX3R5cGUYAiABKA'
    'lSB290cFR5cGUSPwoNb3RwX3R5cGVfZW51bRgDIAEoDjIWLnZvaWNlLmF1dGgudjEuT3RwVHlw'
    'ZUgAUgtvdHBUeXBlRW51bYgBAUIQCg5fb3RwX3R5cGVfZW51bQ==');

@$core.Deprecated('Use convertGuestRequestDescriptor instead')
const ConvertGuestRequest$json = {
  '1': 'ConvertGuestRequest',
  '2': [
    {'1': 'email', '3': 1, '4': 1, '5': 9, '9': 0, '10': 'email', '17': true},
    {'1': 'phone', '3': 2, '4': 1, '5': 9, '9': 1, '10': 'phone', '17': true},
    {'1': 'password', '3': 3, '4': 1, '5': 9, '10': 'password'},
  ],
  '8': [
    {'1': '_email'},
    {'1': '_phone'},
  ],
};

/// Descriptor for `ConvertGuestRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List convertGuestRequestDescriptor = $convert.base64Decode(
    'ChNDb252ZXJ0R3Vlc3RSZXF1ZXN0EhkKBWVtYWlsGAEgASgJSABSBWVtYWlsiAEBEhkKBXBob2'
    '5lGAIgASgJSAFSBXBob25liAEBEhoKCHBhc3N3b3JkGAMgASgJUghwYXNzd29yZEIICgZfZW1h'
    'aWxCCAoGX3Bob25l');

@$core.Deprecated('Use deleteAccountRequestDescriptor instead')
const DeleteAccountRequest$json = {
  '1': 'DeleteAccountRequest',
  '2': [
    {'1': 'password', '3': 1, '4': 1, '5': 9, '10': 'password'},
  ],
};

/// Descriptor for `DeleteAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAccountRequestDescriptor =
    $convert.base64Decode(
        'ChREZWxldGVBY2NvdW50UmVxdWVzdBIaCghwYXNzd29yZBgBIAEoCVIIcGFzc3dvcmQ=');

@$core.Deprecated('Use restoreAccountRequestDescriptor instead')
const RestoreAccountRequest$json = {
  '1': 'RestoreAccountRequest',
  '2': [
    {'1': 'token', '3': 1, '4': 1, '5': 9, '10': 'token'},
  ],
};

/// Descriptor for `RestoreAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List restoreAccountRequestDescriptor =
    $convert.base64Decode(
        'ChVSZXN0b3JlQWNjb3VudFJlcXVlc3QSFAoFdG9rZW4YASABKAlSBXRva2Vu');

@$core.Deprecated('Use validateTokenRequestDescriptor instead')
const ValidateTokenRequest$json = {
  '1': 'ValidateTokenRequest',
  '2': [
    {'1': 'access_token', '3': 1, '4': 1, '5': 9, '10': 'accessToken'},
  ],
};

/// Descriptor for `ValidateTokenRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateTokenRequestDescriptor = $convert.base64Decode(
    'ChRWYWxpZGF0ZVRva2VuUmVxdWVzdBIhCgxhY2Nlc3NfdG9rZW4YASABKAlSC2FjY2Vzc1Rva2'
    'Vu');

@$core.Deprecated('Use authSessionDescriptor instead')
const AuthSession$json = {
  '1': 'AuthSession',
  '2': [
    {'1': 'access_token', '3': 1, '4': 1, '5': 9, '10': 'accessToken'},
    {'1': 'refresh_token', '3': 2, '4': 1, '5': 9, '10': 'refreshToken'},
    {
      '1': 'expires_in_seconds',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'expiresInSeconds'
    },
    {'1': 'account_id', '3': 4, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 5, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `AuthSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authSessionDescriptor = $convert.base64Decode(
    'CgtBdXRoU2Vzc2lvbhIhCgxhY2Nlc3NfdG9rZW4YASABKAlSC2FjY2Vzc1Rva2VuEiMKDXJlZn'
    'Jlc2hfdG9rZW4YAiABKAlSDHJlZnJlc2hUb2tlbhIsChJleHBpcmVzX2luX3NlY29uZHMYAyAB'
    'KANSEGV4cGlyZXNJblNlY29uZHMSHQoKYWNjb3VudF9pZBgEIAEoCVIJYWNjb3VudElkEh0KCn'
    'Byb2ZpbGVfaWQYBSABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use registerResponseDescriptor instead')
const RegisterResponse$json = {
  '1': 'RegisterResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `RegisterResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List registerResponseDescriptor = $convert.base64Decode(
    'ChBSZWdpc3RlclJlc3BvbnNlEjQKB3Nlc3Npb24YASABKAsyGi52b2ljZS5hdXRoLnYxLkF1dG'
    'hTZXNzaW9uUgdzZXNzaW9u');

@$core.Deprecated('Use loginResponseDescriptor instead')
const LoginResponse$json = {
  '1': 'LoginResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `LoginResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List loginResponseDescriptor = $convert.base64Decode(
    'Cg1Mb2dpblJlc3BvbnNlEjQKB3Nlc3Npb24YASABKAsyGi52b2ljZS5hdXRoLnYxLkF1dGhTZX'
    'NzaW9uUgdzZXNzaW9u');

@$core.Deprecated('Use logoutResponseDescriptor instead')
const LogoutResponse$json = {
  '1': 'LogoutResponse',
};

/// Descriptor for `LogoutResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List logoutResponseDescriptor =
    $convert.base64Decode('Cg5Mb2dvdXRSZXNwb25zZQ==');

@$core.Deprecated('Use refreshTokenResponseDescriptor instead')
const RefreshTokenResponse$json = {
  '1': 'RefreshTokenResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `RefreshTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List refreshTokenResponseDescriptor = $convert.base64Decode(
    'ChRSZWZyZXNoVG9rZW5SZXNwb25zZRI0CgdzZXNzaW9uGAEgASgLMhoudm9pY2UuYXV0aC52MS'
    '5BdXRoU2Vzc2lvblIHc2Vzc2lvbg==');

@$core.Deprecated('Use verify2FAResponseDescriptor instead')
const Verify2FAResponse$json = {
  '1': 'Verify2FAResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `Verify2FAResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verify2FAResponseDescriptor = $convert.base64Decode(
    'ChFWZXJpZnkyRkFSZXNwb25zZRI0CgdzZXNzaW9uGAEgASgLMhoudm9pY2UuYXV0aC52MS5BdX'
    'RoU2Vzc2lvblIHc2Vzc2lvbg==');

@$core.Deprecated('Use verifyOTPResponseDescriptor instead')
const VerifyOTPResponse$json = {
  '1': 'VerifyOTPResponse',
};

/// Descriptor for `VerifyOTPResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifyOTPResponseDescriptor =
    $convert.base64Decode('ChFWZXJpZnlPVFBSZXNwb25zZQ==');

@$core.Deprecated('Use convertGuestResponseDescriptor instead')
const ConvertGuestResponse$json = {
  '1': 'ConvertGuestResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `ConvertGuestResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List convertGuestResponseDescriptor = $convert.base64Decode(
    'ChRDb252ZXJ0R3Vlc3RSZXNwb25zZRI0CgdzZXNzaW9uGAEgASgLMhoudm9pY2UuYXV0aC52MS'
    '5BdXRoU2Vzc2lvblIHc2Vzc2lvbg==');

@$core.Deprecated('Use deleteAccountResponseDescriptor instead')
const DeleteAccountResponse$json = {
  '1': 'DeleteAccountResponse',
};

/// Descriptor for `DeleteAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAccountResponseDescriptor =
    $convert.base64Decode('ChVEZWxldGVBY2NvdW50UmVzcG9uc2U=');

@$core.Deprecated('Use restoreAccountResponseDescriptor instead')
const RestoreAccountResponse$json = {
  '1': 'RestoreAccountResponse',
  '2': [
    {
      '1': 'session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.AuthSession',
      '10': 'session'
    },
  ],
};

/// Descriptor for `RestoreAccountResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List restoreAccountResponseDescriptor =
    $convert.base64Decode(
        'ChZSZXN0b3JlQWNjb3VudFJlc3BvbnNlEjQKB3Nlc3Npb24YASABKAsyGi52b2ljZS5hdXRoLn'
        'YxLkF1dGhTZXNzaW9uUgdzZXNzaW9u');

@$core.Deprecated('Use validateTokenResponseDescriptor instead')
const ValidateTokenResponse$json = {
  '1': 'ValidateTokenResponse',
  '2': [
    {
      '1': 'claims',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.TokenClaims',
      '10': 'claims'
    },
  ],
};

/// Descriptor for `ValidateTokenResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List validateTokenResponseDescriptor = $convert.base64Decode(
    'ChVWYWxpZGF0ZVRva2VuUmVzcG9uc2USMgoGY2xhaW1zGAEgASgLMhoudm9pY2UuYXV0aC52MS'
    '5Ub2tlbkNsYWltc1IGY2xhaW1z');

@$core.Deprecated('Use getJWKSRequestDescriptor instead')
const GetJWKSRequest$json = {
  '1': 'GetJWKSRequest',
};

/// Descriptor for `GetJWKSRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getJWKSRequestDescriptor =
    $convert.base64Decode('Cg5HZXRKV0tTUmVxdWVzdA==');

@$core.Deprecated('Use getJWKSResponseDescriptor instead')
const GetJWKSResponse$json = {
  '1': 'GetJWKSResponse',
  '2': [
    {'1': 'keys_json', '3': 1, '4': 1, '5': 9, '10': 'keysJson'},
  ],
};

/// Descriptor for `GetJWKSResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getJWKSResponseDescriptor = $convert.base64Decode(
    'Cg9HZXRKV0tTUmVzcG9uc2USGwoJa2V5c19qc29uGAEgASgJUghrZXlzSnNvbg==');

@$core.Deprecated('Use tokenClaimsDescriptor instead')
const TokenClaims$json = {
  '1': 'TokenClaims',
  '2': [
    {'1': 'user_id', '3': 1, '4': 1, '5': 9, '10': 'userId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'roles', '3': 3, '4': 3, '5': 9, '10': 'roles'},
    {
      '1': 'subscription_tier',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'subscriptionTier'
    },
    {
      '1': 'expires_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `TokenClaims`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tokenClaimsDescriptor = $convert.base64Decode(
    'CgtUb2tlbkNsYWltcxIXCgd1c2VyX2lkGAEgASgJUgZ1c2VySWQSHQoKcHJvZmlsZV9pZBgCIA'
    'EoCVIJcHJvZmlsZUlkEhQKBXJvbGVzGAMgAygJUgVyb2xlcxIrChFzdWJzY3JpcHRpb25fdGll'
    'chgEIAEoCVIQc3Vic2NyaXB0aW9uVGllchI5CgpleHBpcmVzX2F0GAUgASgLMhouZ29vZ2xlLn'
    'Byb3RvYnVmLlRpbWVzdGFtcFIJZXhwaXJlc0F0');
