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

@$core.Deprecated('Use verifiedFactorDescriptor instead')
const VerifiedFactor$json = {
  '1': 'VerifiedFactor',
  '2': [
    {'1': 'VERIFIED_FACTOR_UNSPECIFIED', '2': 0},
    {'1': 'VERIFIED_FACTOR_PASSWORD', '2': 1},
    {'1': 'VERIFIED_FACTOR_TOTP', '2': 2},
    {'1': 'VERIFIED_FACTOR_BACKUP_CODE', '2': 3},
  ],
};

/// Descriptor for `VerifiedFactor`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List verifiedFactorDescriptor = $convert.base64Decode(
    'Cg5WZXJpZmllZEZhY3RvchIfChtWRVJJRklFRF9GQUNUT1JfVU5TUEVDSUZJRUQQABIcChhWRV'
    'JJRklFRF9GQUNUT1JfUEFTU1dPUkQQARIYChRWRVJJRklFRF9GQUNUT1JfVE9UUBACEh8KG1ZF'
    'UklGSUVEX0ZBQ1RPUl9CQUNLVVBfQ09ERRAD');

@$core.Deprecated('Use proofPurposeDescriptor instead')
const ProofPurpose$json = {
  '1': 'ProofPurpose',
  '2': [
    {'1': 'PROOF_PURPOSE_UNSPECIFIED', '2': 0},
    {'1': 'PROOF_PURPOSE_SPACE_DELETE', '2': 1},
  ],
};

/// Descriptor for `ProofPurpose`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List proofPurposeDescriptor = $convert.base64Decode(
    'CgxQcm9vZlB1cnBvc2USHQoZUFJPT0ZfUFVSUE9TRV9VTlNQRUNJRklFRBAAEh4KGlBST09GX1'
    'BVUlBPU0VfU1BBQ0VfREVMRVRFEAE=');

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
    {
      '1': 'totp_code',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'totpCode',
      '17': true
    },
  ],
  '8': [
    {'1': '_totp_code'},
  ],
};

/// Descriptor for `DeleteAccountRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deleteAccountRequestDescriptor = $convert.base64Decode(
    'ChREZWxldGVBY2NvdW50UmVxdWVzdBIaCghwYXNzd29yZBgBIAEoCVIIcGFzc3dvcmQSIAoJdG'
    '90cF9jb2RlGAIgASgJSABSCHRvdHBDb2RliAEBQgwKCl90b3RwX2NvZGU=');

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
    {'1': 'account_type', '3': 6, '4': 1, '5': 9, '10': 'accountType'},
  ],
};

/// Descriptor for `AuthSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List authSessionDescriptor = $convert.base64Decode(
    'CgtBdXRoU2Vzc2lvbhIhCgxhY2Nlc3NfdG9rZW4YASABKAlSC2FjY2Vzc1Rva2VuEiMKDXJlZn'
    'Jlc2hfdG9rZW4YAiABKAlSDHJlZnJlc2hUb2tlbhIsChJleHBpcmVzX2luX3NlY29uZHMYAyAB'
    'KANSEGV4cGlyZXNJblNlY29uZHMSHQoKYWNjb3VudF9pZBgEIAEoCVIJYWNjb3VudElkEh0KCn'
    'Byb2ZpbGVfaWQYBSABKAlSCXByb2ZpbGVJZBIhCgxhY2NvdW50X3R5cGUYBiABKAlSC2FjY291'
    'bnRUeXBl');

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

/// Descriptor for `VerifyOTPResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List verifyOTPResponseDescriptor = $convert.base64Decode(
    'ChFWZXJpZnlPVFBSZXNwb25zZRI0CgdzZXNzaW9uGAEgASgLMhoudm9pY2UuYXV0aC52MS5BdX'
    'RoU2Vzc2lvblIHc2Vzc2lvbg==');

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
    {'1': 'account_type', '3': 6, '4': 1, '5': 9, '10': 'accountType'},
  ],
};

/// Descriptor for `TokenClaims`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List tokenClaimsDescriptor = $convert.base64Decode(
    'CgtUb2tlbkNsYWltcxIXCgd1c2VyX2lkGAEgASgJUgZ1c2VySWQSHQoKcHJvZmlsZV9pZBgCIA'
    'EoCVIJcHJvZmlsZUlkEhQKBXJvbGVzGAMgAygJUgVyb2xlcxIrChFzdWJzY3JpcHRpb25fdGll'
    'chgEIAEoCVIQc3Vic2NyaXB0aW9uVGllchI5CgpleHBpcmVzX2F0GAUgASgLMhouZ29vZ2xlLn'
    'Byb3RvYnVmLlRpbWVzdGFtcFIJZXhwaXJlc0F0EiEKDGFjY291bnRfdHlwZRgGIAEoCVILYWNj'
    'b3VudFR5cGU=');

@$core.Deprecated('Use switchActiveProfileRequestDescriptor instead')
const SwitchActiveProfileRequest$json = {
  '1': 'SwitchActiveProfileRequest',
  '2': [
    {'1': 'access_token', '3': 1, '4': 1, '5': 9, '10': 'accessToken'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'device_info_json', '3': 3, '4': 1, '5': 9, '10': 'deviceInfoJson'},
  ],
};

/// Descriptor for `SwitchActiveProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List switchActiveProfileRequestDescriptor =
    $convert.base64Decode(
        'ChpTd2l0Y2hBY3RpdmVQcm9maWxlUmVxdWVzdBIhCgxhY2Nlc3NfdG9rZW4YASABKAlSC2FjY2'
        'Vzc1Rva2VuEh0KCnByb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBIoChBkZXZpY2VfaW5mb19q'
        'c29uGAMgASgJUg5kZXZpY2VJbmZvSnNvbg==');

@$core.Deprecated('Use switchActiveProfileResponseDescriptor instead')
const SwitchActiveProfileResponse$json = {
  '1': 'SwitchActiveProfileResponse',
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

/// Descriptor for `SwitchActiveProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List switchActiveProfileResponseDescriptor =
    $convert.base64Decode(
        'ChtTd2l0Y2hBY3RpdmVQcm9maWxlUmVzcG9uc2USNAoHc2Vzc2lvbhgBIAEoCzIaLnZvaWNlLm'
        'F1dGgudjEuQXV0aFNlc3Npb25SB3Nlc3Npb24=');

@$core.Deprecated('Use setAccountStatusRequestDescriptor instead')
const SetAccountStatusRequest$json = {
  '1': 'SetAccountStatusRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'status', '3': 2, '4': 1, '5': 9, '10': 'status'},
    {'1': 'reason', '3': 3, '4': 1, '5': 9, '10': 'reason'},
  ],
};

/// Descriptor for `SetAccountStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setAccountStatusRequestDescriptor =
    $convert.base64Decode(
        'ChdTZXRBY2NvdW50U3RhdHVzUmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SW'
        'QSFgoGc3RhdHVzGAIgASgJUgZzdGF0dXMSFgoGcmVhc29uGAMgASgJUgZyZWFzb24=');

@$core.Deprecated('Use setAccountStatusResponseDescriptor instead')
const SetAccountStatusResponse$json = {
  '1': 'SetAccountStatusResponse',
};

/// Descriptor for `SetAccountStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List setAccountStatusResponseDescriptor =
    $convert.base64Decode('ChhTZXRBY2NvdW50U3RhdHVzUmVzcG9uc2U=');

@$core.Deprecated('Use putE2EKeyBackupRequestDescriptor instead')
const PutE2EKeyBackupRequest$json = {
  '1': 'PutE2EKeyBackupRequest',
  '2': [
    {'1': 'encrypted_blob', '3': 1, '4': 1, '5': 9, '10': 'encryptedBlob'},
    {
      '1': 'password_hint',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'passwordHint',
      '17': true
    },
  ],
  '8': [
    {'1': '_password_hint'},
  ],
};

/// Descriptor for `PutE2EKeyBackupRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List putE2EKeyBackupRequestDescriptor = $convert.base64Decode(
    'ChZQdXRFMkVLZXlCYWNrdXBSZXF1ZXN0EiUKDmVuY3J5cHRlZF9ibG9iGAEgASgJUg1lbmNyeX'
    'B0ZWRCbG9iEigKDXBhc3N3b3JkX2hpbnQYAiABKAlIAFIMcGFzc3dvcmRIaW50iAEBQhAKDl9w'
    'YXNzd29yZF9oaW50');

@$core.Deprecated('Use putE2EKeyBackupResponseDescriptor instead')
const PutE2EKeyBackupResponse$json = {
  '1': 'PutE2EKeyBackupResponse',
};

/// Descriptor for `PutE2EKeyBackupResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List putE2EKeyBackupResponseDescriptor =
    $convert.base64Decode('ChdQdXRFMkVLZXlCYWNrdXBSZXNwb25zZQ==');

@$core.Deprecated('Use getE2EKeyBackupRequestDescriptor instead')
const GetE2EKeyBackupRequest$json = {
  '1': 'GetE2EKeyBackupRequest',
};

/// Descriptor for `GetE2EKeyBackupRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getE2EKeyBackupRequestDescriptor =
    $convert.base64Decode('ChZHZXRFMkVLZXlCYWNrdXBSZXF1ZXN0');

@$core.Deprecated('Use getE2EKeyBackupResponseDescriptor instead')
const GetE2EKeyBackupResponse$json = {
  '1': 'GetE2EKeyBackupResponse',
  '2': [
    {'1': 'encrypted_blob', '3': 1, '4': 1, '5': 9, '10': 'encryptedBlob'},
    {
      '1': 'password_hint',
      '3': 2,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'passwordHint',
      '17': true
    },
  ],
  '8': [
    {'1': '_password_hint'},
  ],
};

/// Descriptor for `GetE2EKeyBackupResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getE2EKeyBackupResponseDescriptor = $convert.base64Decode(
    'ChdHZXRFMkVLZXlCYWNrdXBSZXNwb25zZRIlCg5lbmNyeXB0ZWRfYmxvYhgBIAEoCVINZW5jcn'
    'lwdGVkQmxvYhIoCg1wYXNzd29yZF9oaW50GAIgASgJSABSDHBhc3N3b3JkSGludIgBAUIQCg5f'
    'cGFzc3dvcmRfaGludA==');

@$core.Deprecated('Use resolvePhoneHashesRequestDescriptor instead')
const ResolvePhoneHashesRequest$json = {
  '1': 'ResolvePhoneHashesRequest',
  '2': [
    {'1': 'phone_hashes', '3': 1, '4': 3, '5': 9, '10': 'phoneHashes'},
  ],
};

/// Descriptor for `ResolvePhoneHashesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolvePhoneHashesRequestDescriptor =
    $convert.base64Decode(
        'ChlSZXNvbHZlUGhvbmVIYXNoZXNSZXF1ZXN0EiEKDHBob25lX2hhc2hlcxgBIAMoCVILcGhvbm'
        'VIYXNoZXM=');

@$core.Deprecated('Use phoneHashProfileMatchDescriptor instead')
const PhoneHashProfileMatch$json = {
  '1': 'PhoneHashProfileMatch',
  '2': [
    {'1': 'phone_hash', '3': 1, '4': 1, '5': 9, '10': 'phoneHash'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `PhoneHashProfileMatch`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List phoneHashProfileMatchDescriptor = $convert.base64Decode(
    'ChVQaG9uZUhhc2hQcm9maWxlTWF0Y2gSHQoKcGhvbmVfaGFzaBgBIAEoCVIJcGhvbmVIYXNoEh'
    '0KCnByb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use resolvePhoneHashesResponseDescriptor instead')
const ResolvePhoneHashesResponse$json = {
  '1': 'ResolvePhoneHashesResponse',
  '2': [
    {
      '1': 'matches',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.auth.v1.PhoneHashProfileMatch',
      '10': 'matches'
    },
  ],
};

/// Descriptor for `ResolvePhoneHashesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resolvePhoneHashesResponseDescriptor =
    $convert.base64Decode(
        'ChpSZXNvbHZlUGhvbmVIYXNoZXNSZXNwb25zZRI+CgdtYXRjaGVzGAEgAygLMiQudm9pY2UuYX'
        'V0aC52MS5QaG9uZUhhc2hQcm9maWxlTWF0Y2hSB21hdGNoZXM=');

@$core.Deprecated('Use filterDeletedAccountIDsRequestDescriptor instead')
const FilterDeletedAccountIDsRequest$json = {
  '1': 'FilterDeletedAccountIDsRequest',
  '2': [
    {'1': 'account_ids', '3': 1, '4': 3, '5': 9, '10': 'accountIds'},
  ],
};

/// Descriptor for `FilterDeletedAccountIDsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List filterDeletedAccountIDsRequestDescriptor =
    $convert.base64Decode(
        'Ch5GaWx0ZXJEZWxldGVkQWNjb3VudElEc1JlcXVlc3QSHwoLYWNjb3VudF9pZHMYASADKAlSCm'
        'FjY291bnRJZHM=');

@$core.Deprecated('Use filterDeletedAccountIDsResponseDescriptor instead')
const FilterDeletedAccountIDsResponse$json = {
  '1': 'FilterDeletedAccountIDsResponse',
  '2': [
    {
      '1': 'deleted_account_ids',
      '3': 1,
      '4': 3,
      '5': 9,
      '10': 'deletedAccountIds'
    },
  ],
};

/// Descriptor for `FilterDeletedAccountIDsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List filterDeletedAccountIDsResponseDescriptor =
    $convert.base64Decode(
        'Ch9GaWx0ZXJEZWxldGVkQWNjb3VudElEc1Jlc3BvbnNlEi4KE2RlbGV0ZWRfYWNjb3VudF9pZH'
        'MYASADKAlSEWRlbGV0ZWRBY2NvdW50SWRz');

@$core.Deprecated('Use getGuestReminderRequestDescriptor instead')
const GetGuestReminderRequest$json = {
  '1': 'GetGuestReminderRequest',
};

/// Descriptor for `GetGuestReminderRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getGuestReminderRequestDescriptor =
    $convert.base64Decode('ChdHZXRHdWVzdFJlbWluZGVyUmVxdWVzdA==');

@$core.Deprecated('Use getGuestReminderResponseDescriptor instead')
const GetGuestReminderResponse$json = {
  '1': 'GetGuestReminderResponse',
  '2': [
    {
      '1': 'last_shown_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'lastShownAt',
      '17': true
    },
    {'1': 'should_show', '3': 2, '4': 1, '5': 8, '10': 'shouldShow'},
  ],
  '8': [
    {'1': '_last_shown_at'},
  ],
};

/// Descriptor for `GetGuestReminderResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getGuestReminderResponseDescriptor = $convert.base64Decode(
    'ChhHZXRHdWVzdFJlbWluZGVyUmVzcG9uc2USQwoNbGFzdF9zaG93bl9hdBgBIAEoCzIaLmdvb2'
    'dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAFILbGFzdFNob3duQXSIAQESHwoLc2hvdWxkX3Nob3cY'
    'AiABKAhSCnNob3VsZFNob3dCEAoOX2xhc3Rfc2hvd25fYXQ=');

@$core.Deprecated('Use markGuestReminderShownRequestDescriptor instead')
const MarkGuestReminderShownRequest$json = {
  '1': 'MarkGuestReminderShownRequest',
};

/// Descriptor for `MarkGuestReminderShownRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markGuestReminderShownRequestDescriptor =
    $convert.base64Decode('Ch1NYXJrR3Vlc3RSZW1pbmRlclNob3duUmVxdWVzdA==');

@$core.Deprecated('Use markGuestReminderShownResponseDescriptor instead')
const MarkGuestReminderShownResponse$json = {
  '1': 'MarkGuestReminderShownResponse',
  '2': [
    {
      '1': 'last_shown_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'lastShownAt'
    },
  ],
};

/// Descriptor for `MarkGuestReminderShownResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List markGuestReminderShownResponseDescriptor =
    $convert.base64Decode(
        'Ch5NYXJrR3Vlc3RSZW1pbmRlclNob3duUmVzcG9uc2USPgoNbGFzdF9zaG93bl9hdBgBIAEoCz'
        'IaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSC2xhc3RTaG93bkF0');

@$core.Deprecated('Use listSessionsRequestDescriptor instead')
const ListSessionsRequest$json = {
  '1': 'ListSessionsRequest',
};

/// Descriptor for `ListSessionsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSessionsRequestDescriptor =
    $convert.base64Decode('ChNMaXN0U2Vzc2lvbnNSZXF1ZXN0');

@$core.Deprecated('Use sessionInfoDescriptor instead')
const SessionInfo$json = {
  '1': 'SessionInfo',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'device_info_json', '3': 2, '4': 1, '5': 9, '10': 'deviceInfoJson'},
    {
      '1': 'created_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'expires_at',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
    {'1': 'current', '3': 5, '4': 1, '5': 8, '10': 'current'},
  ],
};

/// Descriptor for `SessionInfo`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List sessionInfoDescriptor = $convert.base64Decode(
    'CgtTZXNzaW9uSW5mbxIOCgJpZBgBIAEoCVICaWQSKAoQZGV2aWNlX2luZm9fanNvbhgCIAEoCV'
    'IOZGV2aWNlSW5mb0pzb24SOQoKY3JlYXRlZF9hdBgDIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5U'
    'aW1lc3RhbXBSCWNyZWF0ZWRBdBI5CgpleHBpcmVzX2F0GAQgASgLMhouZ29vZ2xlLnByb3RvYn'
    'VmLlRpbWVzdGFtcFIJZXhwaXJlc0F0EhgKB2N1cnJlbnQYBSABKAhSB2N1cnJlbnQ=');

@$core.Deprecated('Use listSessionsResponseDescriptor instead')
const ListSessionsResponse$json = {
  '1': 'ListSessionsResponse',
  '2': [
    {
      '1': 'sessions',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.auth.v1.SessionInfo',
      '10': 'sessions'
    },
  ],
};

/// Descriptor for `ListSessionsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listSessionsResponseDescriptor = $convert.base64Decode(
    'ChRMaXN0U2Vzc2lvbnNSZXNwb25zZRI2CghzZXNzaW9ucxgBIAMoCzIaLnZvaWNlLmF1dGgudj'
    'EuU2Vzc2lvbkluZm9SCHNlc3Npb25z');

@$core.Deprecated('Use revokeSessionRequestDescriptor instead')
const RevokeSessionRequest$json = {
  '1': 'RevokeSessionRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `RevokeSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSessionRequestDescriptor = $convert.base64Decode(
    'ChRSZXZva2VTZXNzaW9uUmVxdWVzdBIdCgpzZXNzaW9uX2lkGAEgASgJUglzZXNzaW9uSWQ=');

@$core.Deprecated('Use revokeSessionResponseDescriptor instead')
const RevokeSessionResponse$json = {
  '1': 'RevokeSessionResponse',
};

/// Descriptor for `RevokeSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List revokeSessionResponseDescriptor =
    $convert.base64Decode('ChVSZXZva2VTZXNzaW9uUmVzcG9uc2U=');

@$core.Deprecated('Use issueOwnershipTransferProofRequestDescriptor instead')
const IssueOwnershipTransferProofRequest$json = {
  '1': 'IssueOwnershipTransferProofRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'password', '3': 4, '4': 1, '5': 9, '10': 'password'},
    {'1': 'totp_code', '3': 5, '4': 1, '5': 9, '10': 'totpCode'},
    {'1': 'backup_code', '3': 6, '4': 1, '5': 9, '10': 'backupCode'},
  ],
};

/// Descriptor for `IssueOwnershipTransferProofRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueOwnershipTransferProofRequestDescriptor =
    $convert.base64Decode(
        'CiJJc3N1ZU93bmVyc2hpcFRyYW5zZmVyUHJvb2ZSZXF1ZXN0EhkKCHNwYWNlX2lkGAEgASgJUg'
        'dzcGFjZUlkEi8KFG5ld19vd25lcl9wcm9maWxlX2lkGAIgASgJUhFuZXdPd25lclByb2ZpbGVJ'
        'ZBIhCgxvcGVyYXRpb25faWQYAyABKAlSC29wZXJhdGlvbklkEhoKCHBhc3N3b3JkGAQgASgJUg'
        'hwYXNzd29yZBIbCgl0b3RwX2NvZGUYBSABKAlSCHRvdHBDb2RlEh8KC2JhY2t1cF9jb2RlGAYg'
        'ASgJUgpiYWNrdXBDb2Rl');

@$core.Deprecated('Use issueOwnershipTransferProofResponseDescriptor instead')
const IssueOwnershipTransferProofResponse$json = {
  '1': 'IssueOwnershipTransferProofResponse',
  '2': [
    {'1': 'proof', '3': 1, '4': 1, '5': 9, '10': 'proof'},
    {
      '1': 'expires_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `IssueOwnershipTransferProofResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueOwnershipTransferProofResponseDescriptor =
    $convert.base64Decode(
        'CiNJc3N1ZU93bmVyc2hpcFRyYW5zZmVyUHJvb2ZSZXNwb25zZRIUCgVwcm9vZhgBIAEoCVIFcH'
        'Jvb2YSOQoKZXhwaXJlc19hdBgCIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWV4'
        'cGlyZXNBdA==');

@$core.Deprecated('Use consumeOwnershipTransferProofRequestDescriptor instead')
const ConsumeOwnershipTransferProofRequest$json = {
  '1': 'ConsumeOwnershipTransferProofRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'session_epoch', '3': 6, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {'1': 'proof', '3': 7, '4': 1, '5': 9, '10': 'proof'},
  ],
};

/// Descriptor for `ConsumeOwnershipTransferProofRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List consumeOwnershipTransferProofRequestDescriptor =
    $convert.base64Decode(
        'CiRDb25zdW1lT3duZXJzaGlwVHJhbnNmZXJQcm9vZlJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIA'
        'EoCVIJYWNjb3VudElkEh0KCnByb2ZpbGVfaWQYAiABKAlSCXByb2ZpbGVJZBIZCghzcGFjZV9p'
        'ZBgDIAEoCVIHc3BhY2VJZBIvChRuZXdfb3duZXJfcHJvZmlsZV9pZBgEIAEoCVIRbmV3T3duZX'
        'JQcm9maWxlSWQSIQoMb3BlcmF0aW9uX2lkGAUgASgJUgtvcGVyYXRpb25JZBIjCg1zZXNzaW9u'
        'X2Vwb2NoGAYgASgDUgxzZXNzaW9uRXBvY2gSFAoFcHJvb2YYByABKAlSBXByb29m');

@$core.Deprecated('Use consumeOwnershipTransferProofResponseDescriptor instead')
const ConsumeOwnershipTransferProofResponse$json = {
  '1': 'ConsumeOwnershipTransferProofResponse',
  '2': [
    {'1': 'receipt_id', '3': 1, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'session_epoch', '3': 7, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {
      '1': 'consumed_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'consumedAt'
    },
    {'1': 'verified_factors', '3': 9, '4': 3, '5': 9, '10': 'verifiedFactors'},
  ],
};

/// Descriptor for `ConsumeOwnershipTransferProofResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List consumeOwnershipTransferProofResponseDescriptor = $convert.base64Decode(
    'CiVDb25zdW1lT3duZXJzaGlwVHJhbnNmZXJQcm9vZlJlc3BvbnNlEh0KCnJlY2VpcHRfaWQYAS'
    'ABKAlSCXJlY2VpcHRJZBIdCgphY2NvdW50X2lkGAIgASgJUglhY2NvdW50SWQSHQoKcHJvZmls'
    'ZV9pZBgDIAEoCVIJcHJvZmlsZUlkEhkKCHNwYWNlX2lkGAQgASgJUgdzcGFjZUlkEi8KFG5ld1'
    '9vd25lcl9wcm9maWxlX2lkGAUgASgJUhFuZXdPd25lclByb2ZpbGVJZBIhCgxvcGVyYXRpb25f'
    'aWQYBiABKAlSC29wZXJhdGlvbklkEiMKDXNlc3Npb25fZXBvY2gYByABKANSDHNlc3Npb25FcG'
    '9jaBI7Cgtjb25zdW1lZF9hdBgIIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCmNv'
    'bnN1bWVkQXQSKQoQdmVyaWZpZWRfZmFjdG9ycxgJIAMoCVIPdmVyaWZpZWRGYWN0b3Jz');

@$core.Deprecated('Use getOwnershipTransferReceiptRequestDescriptor instead')
const GetOwnershipTransferReceiptRequest$json = {
  '1': 'GetOwnershipTransferReceiptRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 5, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'session_epoch', '3': 6, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {'1': 'proof_digest', '3': 7, '4': 1, '5': 9, '10': 'proofDigest'},
  ],
};

/// Descriptor for `GetOwnershipTransferReceiptRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOwnershipTransferReceiptRequestDescriptor = $convert.base64Decode(
    'CiJHZXRPd25lcnNoaXBUcmFuc2ZlclJlY2VpcHRSZXF1ZXN0Eh0KCmFjY291bnRfaWQYASABKA'
    'lSCWFjY291bnRJZBIdCgpwcm9maWxlX2lkGAIgASgJUglwcm9maWxlSWQSGQoIc3BhY2VfaWQY'
    'AyABKAlSB3NwYWNlSWQSLwoUbmV3X293bmVyX3Byb2ZpbGVfaWQYBCABKAlSEW5ld093bmVyUH'
    'JvZmlsZUlkEiEKDG9wZXJhdGlvbl9pZBgFIAEoCVILb3BlcmF0aW9uSWQSIwoNc2Vzc2lvbl9l'
    'cG9jaBgGIAEoA1IMc2Vzc2lvbkVwb2NoEiEKDHByb29mX2RpZ2VzdBgHIAEoCVILcHJvb2ZEaW'
    'dlc3Q=');

@$core.Deprecated('Use getOwnershipTransferReceiptResponseDescriptor instead')
const GetOwnershipTransferReceiptResponse$json = {
  '1': 'GetOwnershipTransferReceiptResponse',
  '2': [
    {'1': 'receipt_id', '3': 1, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'space_id', '3': 4, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'new_owner_profile_id',
      '3': 5,
      '4': 1,
      '5': 9,
      '10': 'newOwnerProfileId'
    },
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'session_epoch', '3': 7, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {
      '1': 'consumed_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'consumedAt'
    },
    {'1': 'verified_factors', '3': 9, '4': 3, '5': 9, '10': 'verifiedFactors'},
  ],
};

/// Descriptor for `GetOwnershipTransferReceiptResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getOwnershipTransferReceiptResponseDescriptor = $convert.base64Decode(
    'CiNHZXRPd25lcnNoaXBUcmFuc2ZlclJlY2VpcHRSZXNwb25zZRIdCgpyZWNlaXB0X2lkGAEgAS'
    'gJUglyZWNlaXB0SWQSHQoKYWNjb3VudF9pZBgCIAEoCVIJYWNjb3VudElkEh0KCnByb2ZpbGVf'
    'aWQYAyABKAlSCXByb2ZpbGVJZBIZCghzcGFjZV9pZBgEIAEoCVIHc3BhY2VJZBIvChRuZXdfb3'
    'duZXJfcHJvZmlsZV9pZBgFIAEoCVIRbmV3T3duZXJQcm9maWxlSWQSIQoMb3BlcmF0aW9uX2lk'
    'GAYgASgJUgtvcGVyYXRpb25JZBIjCg1zZXNzaW9uX2Vwb2NoGAcgASgDUgxzZXNzaW9uRXBvY2'
    'gSOwoLY29uc3VtZWRfYXQYCCABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpjb25z'
    'dW1lZEF0EikKEHZlcmlmaWVkX2ZhY3RvcnMYCSADKAlSD3ZlcmlmaWVkRmFjdG9ycw==');

@$core.Deprecated('Use issueSpaceDeletionProofRequestDescriptor instead')
const IssueSpaceDeletionProofRequest$json = {
  '1': 'IssueSpaceDeletionProofRequest',
  '2': [
    {'1': 'space_id', '3': 1, '4': 1, '5': 9, '10': 'spaceId'},
    {
      '1': 'confirmation_name',
      '3': 2,
      '4': 1,
      '5': 9,
      '10': 'confirmationName'
    },
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'password', '3': 4, '4': 1, '5': 9, '10': 'password'},
    {'1': 'totp_code', '3': 5, '4': 1, '5': 9, '10': 'totpCode'},
    {'1': 'backup_code', '3': 6, '4': 1, '5': 9, '10': 'backupCode'},
  ],
};

/// Descriptor for `IssueSpaceDeletionProofRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueSpaceDeletionProofRequestDescriptor = $convert.base64Decode(
    'Ch5Jc3N1ZVNwYWNlRGVsZXRpb25Qcm9vZlJlcXVlc3QSGQoIc3BhY2VfaWQYASABKAlSB3NwYW'
    'NlSWQSKwoRY29uZmlybWF0aW9uX25hbWUYAiABKAlSEGNvbmZpcm1hdGlvbk5hbWUSIQoMb3Bl'
    'cmF0aW9uX2lkGAMgASgJUgtvcGVyYXRpb25JZBIaCghwYXNzd29yZBgEIAEoCVIIcGFzc3dvcm'
    'QSGwoJdG90cF9jb2RlGAUgASgJUgh0b3RwQ29kZRIfCgtiYWNrdXBfY29kZRgGIAEoCVIKYmFj'
    'a3VwQ29kZQ==');

@$core.Deprecated('Use issueSpaceDeletionProofResponseDescriptor instead')
const IssueSpaceDeletionProofResponse$json = {
  '1': 'IssueSpaceDeletionProofResponse',
  '2': [
    {'1': 'proof', '3': 1, '4': 1, '5': 9, '10': 'proof'},
    {
      '1': 'expires_at',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'expiresAt'
    },
  ],
};

/// Descriptor for `IssueSpaceDeletionProofResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List issueSpaceDeletionProofResponseDescriptor =
    $convert.base64Decode(
        'Ch9Jc3N1ZVNwYWNlRGVsZXRpb25Qcm9vZlJlc3BvbnNlEhQKBXByb29mGAEgASgJUgVwcm9vZh'
        'I5CgpleHBpcmVzX2F0GAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJZXhwaXJl'
        'c0F0');

@$core.Deprecated('Use consumeSpaceDeletionProofRequestDescriptor instead')
const ConsumeSpaceDeletionProofRequest$json = {
  '1': 'ConsumeSpaceDeletionProofRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'session_epoch', '3': 4, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {'1': 'space_id', '3': 5, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'confirmation_name',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'confirmationName'
    },
    {'1': 'proof', '3': 8, '4': 1, '5': 9, '10': 'proof'},
  ],
};

/// Descriptor for `ConsumeSpaceDeletionProofRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List consumeSpaceDeletionProofRequestDescriptor = $convert.base64Decode(
    'CiBDb25zdW1lU3BhY2VEZWxldGlvblByb29mUmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW9uGA'
    'EgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKYWNjb3VudF9pZBgCIAEoCVIJYWNjb3VudElkEh0K'
    'CnByb2ZpbGVfaWQYAyABKAlSCXByb2ZpbGVJZBIjCg1zZXNzaW9uX2Vwb2NoGAQgASgDUgxzZX'
    'NzaW9uRXBvY2gSGQoIc3BhY2VfaWQYBSABKAlSB3NwYWNlSWQSIQoMb3BlcmF0aW9uX2lkGAYg'
    'ASgJUgtvcGVyYXRpb25JZBIrChFjb25maXJtYXRpb25fbmFtZRgHIAEoCVIQY29uZmlybWF0aW'
    '9uTmFtZRIUCgVwcm9vZhgIIAEoCVIFcHJvb2Y=');

@$core.Deprecated('Use consumeSpaceDeletionProofResponseDescriptor instead')
const ConsumeSpaceDeletionProofResponse$json = {
  '1': 'ConsumeSpaceDeletionProofResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.SpaceDeletionProofReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `ConsumeSpaceDeletionProofResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List consumeSpaceDeletionProofResponseDescriptor =
    $convert.base64Decode(
        'CiFDb25zdW1lU3BhY2VEZWxldGlvblByb29mUmVzcG9uc2USQgoHcmVjZWlwdBgBIAEoCzIoLn'
        'ZvaWNlLmF1dGgudjEuU3BhY2VEZWxldGlvblByb29mUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated('Use spaceDeletionProofBindingDescriptor instead')
const SpaceDeletionProofBinding$json = {
  '1': 'SpaceDeletionProofBinding',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'session_epoch', '3': 4, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {'1': 'space_id', '3': 5, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'confirmation_name_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'confirmationNameSha256'
    },
    {
      '1': 'proof_digest_sha256',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'proofDigestSha256'
    },
    {
      '1': 'verified_factors',
      '3': 9,
      '4': 3,
      '5': 14,
      '6': '.voice.auth.v1.VerifiedFactor',
      '10': 'verifiedFactors'
    },
    {
      '1': 'purpose',
      '3': 10,
      '4': 1,
      '5': 14,
      '6': '.voice.auth.v1.ProofPurpose',
      '10': 'purpose'
    },
  ],
};

/// Descriptor for `SpaceDeletionProofBinding`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceDeletionProofBindingDescriptor = $convert.base64Decode(
    'ChlTcGFjZURlbGV0aW9uUHJvb2ZCaW5kaW5nEikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3'
    'Byb3RvY29sVmVyc2lvbhIdCgphY2NvdW50X2lkGAIgASgJUglhY2NvdW50SWQSHQoKcHJvZmls'
    'ZV9pZBgDIAEoCVIJcHJvZmlsZUlkEiMKDXNlc3Npb25fZXBvY2gYBCABKANSDHNlc3Npb25FcG'
    '9jaBIZCghzcGFjZV9pZBgFIAEoCVIHc3BhY2VJZBIhCgxvcGVyYXRpb25faWQYBiABKAlSC29w'
    'ZXJhdGlvbklkEjgKGGNvbmZpcm1hdGlvbl9uYW1lX3NoYTI1NhgHIAEoDFIWY29uZmlybWF0aW'
    '9uTmFtZVNoYTI1NhIuChNwcm9vZl9kaWdlc3Rfc2hhMjU2GAggASgMUhFwcm9vZkRpZ2VzdFNo'
    'YTI1NhJIChB2ZXJpZmllZF9mYWN0b3JzGAkgAygOMh0udm9pY2UuYXV0aC52MS5WZXJpZmllZE'
    'ZhY3RvclIPdmVyaWZpZWRGYWN0b3JzEjUKB3B1cnBvc2UYCiABKA4yGy52b2ljZS5hdXRoLnYx'
    'LlByb29mUHVycG9zZVIHcHVycG9zZQ==');

@$core.Deprecated('Use spaceDeletionProofReceiptDescriptor instead')
const SpaceDeletionProofReceipt$json = {
  '1': 'SpaceDeletionProofReceipt',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'operation_id', '3': 3, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'binding_sha256', '3': 4, '4': 1, '5': 12, '10': 'bindingSha256'},
    {
      '1': 'confirmation_name_sha256',
      '3': 5,
      '4': 1,
      '5': 12,
      '10': 'confirmationNameSha256'
    },
    {
      '1': 'consumed_at',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'consumedAt'
    },
    {
      '1': 'verified_factors',
      '3': 7,
      '4': 3,
      '5': 14,
      '6': '.voice.auth.v1.VerifiedFactor',
      '10': 'verifiedFactors'
    },
  ],
};

/// Descriptor for `SpaceDeletionProofReceipt`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceDeletionProofReceiptDescriptor = $convert.base64Decode(
    'ChlTcGFjZURlbGV0aW9uUHJvb2ZSZWNlaXB0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA1SD3'
    'Byb3RvY29sVmVyc2lvbhIdCgpyZWNlaXB0X2lkGAIgASgJUglyZWNlaXB0SWQSIQoMb3BlcmF0'
    'aW9uX2lkGAMgASgJUgtvcGVyYXRpb25JZBIlCg5iaW5kaW5nX3NoYTI1NhgEIAEoDFINYmluZG'
    'luZ1NoYTI1NhI4Chhjb25maXJtYXRpb25fbmFtZV9zaGEyNTYYBSABKAxSFmNvbmZpcm1hdGlv'
    'bk5hbWVTaGEyNTYSOwoLY29uc3VtZWRfYXQYBiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZX'
    'N0YW1wUgpjb25zdW1lZEF0EkgKEHZlcmlmaWVkX2ZhY3RvcnMYByADKA4yHS52b2ljZS5hdXRo'
    'LnYxLlZlcmlmaWVkRmFjdG9yUg92ZXJpZmllZEZhY3RvcnM=');

@$core.Deprecated('Use getSpaceDeletionProofReceiptRequestDescriptor instead')
const GetSpaceDeletionProofReceiptRequest$json = {
  '1': 'GetSpaceDeletionProofReceiptRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 3, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'session_epoch', '3': 4, '4': 1, '5': 3, '10': 'sessionEpoch'},
    {'1': 'space_id', '3': 5, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'operation_id', '3': 6, '4': 1, '5': 9, '10': 'operationId'},
    {
      '1': 'confirmation_name_sha256',
      '3': 7,
      '4': 1,
      '5': 12,
      '10': 'confirmationNameSha256'
    },
    {
      '1': 'proof_digest_sha256',
      '3': 8,
      '4': 1,
      '5': 12,
      '10': 'proofDigestSha256'
    },
  ],
};

/// Descriptor for `GetSpaceDeletionProofReceiptRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceDeletionProofReceiptRequestDescriptor = $convert.base64Decode(
    'CiNHZXRTcGFjZURlbGV0aW9uUHJvb2ZSZWNlaXB0UmVxdWVzdBIpChBwcm90b2NvbF92ZXJzaW'
    '9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SHQoKYWNjb3VudF9pZBgCIAEoCVIJYWNjb3VudElk'
    'Eh0KCnByb2ZpbGVfaWQYAyABKAlSCXByb2ZpbGVJZBIjCg1zZXNzaW9uX2Vwb2NoGAQgASgDUg'
    'xzZXNzaW9uRXBvY2gSGQoIc3BhY2VfaWQYBSABKAlSB3NwYWNlSWQSIQoMb3BlcmF0aW9uX2lk'
    'GAYgASgJUgtvcGVyYXRpb25JZBI4Chhjb25maXJtYXRpb25fbmFtZV9zaGEyNTYYByABKAxSFm'
    'NvbmZpcm1hdGlvbk5hbWVTaGEyNTYSLgoTcHJvb2ZfZGlnZXN0X3NoYTI1NhgIIAEoDFIRcHJv'
    'b2ZEaWdlc3RTaGEyNTY=');

@$core.Deprecated('Use getSpaceDeletionProofReceiptResponseDescriptor instead')
const GetSpaceDeletionProofReceiptResponse$json = {
  '1': 'GetSpaceDeletionProofReceiptResponse',
  '2': [
    {
      '1': 'receipt',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.auth.v1.SpaceDeletionProofReceipt',
      '10': 'receipt'
    },
  ],
};

/// Descriptor for `GetSpaceDeletionProofReceiptResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceDeletionProofReceiptResponseDescriptor =
    $convert.base64Decode(
        'CiRHZXRTcGFjZURlbGV0aW9uUHJvb2ZSZWNlaXB0UmVzcG9uc2USQgoHcmVjZWlwdBgBIAEoCz'
        'IoLnZvaWNlLmF1dGgudjEuU3BhY2VEZWxldGlvblByb29mUmVjZWlwdFIHcmVjZWlwdA==');

@$core.Deprecated(
    'Use acknowledgeSpaceDeletionProofReceiptRequestDescriptor instead')
const AcknowledgeSpaceDeletionProofReceiptRequest$json = {
  '1': 'AcknowledgeSpaceDeletionProofReceiptRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'receipt_id', '3': 2, '4': 1, '5': 9, '10': 'receiptId'},
    {'1': 'space_id', '3': 3, '4': 1, '5': 9, '10': 'spaceId'},
    {'1': 'operation_id', '3': 4, '4': 1, '5': 9, '10': 'operationId'},
    {'1': 'receipt_sha256', '3': 5, '4': 1, '5': 12, '10': 'receiptSha256'},
  ],
};

/// Descriptor for `AcknowledgeSpaceDeletionProofReceiptRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    acknowledgeSpaceDeletionProofReceiptRequestDescriptor =
    $convert.base64Decode(
        'CitBY2tub3dsZWRnZVNwYWNlRGVsZXRpb25Qcm9vZlJlY2VpcHRSZXF1ZXN0EikKEHByb3RvY2'
        '9sX3ZlcnNpb24YASABKA1SD3Byb3RvY29sVmVyc2lvbhIdCgpyZWNlaXB0X2lkGAIgASgJUgly'
        'ZWNlaXB0SWQSGQoIc3BhY2VfaWQYAyABKAlSB3NwYWNlSWQSIQoMb3BlcmF0aW9uX2lkGAQgAS'
        'gJUgtvcGVyYXRpb25JZBIlCg5yZWNlaXB0X3NoYTI1NhgFIAEoDFINcmVjZWlwdFNoYTI1Ng==');

@$core.Deprecated(
    'Use acknowledgeSpaceDeletionProofReceiptResponseDescriptor instead')
const AcknowledgeSpaceDeletionProofReceiptResponse$json = {
  '1': 'AcknowledgeSpaceDeletionProofReceiptResponse',
  '2': [
    {
      '1': 'acknowledged_at',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'acknowledgedAt'
    },
  ],
};

/// Descriptor for `AcknowledgeSpaceDeletionProofReceiptResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List
    acknowledgeSpaceDeletionProofReceiptResponseDescriptor =
    $convert.base64Decode(
        'CixBY2tub3dsZWRnZVNwYWNlRGVsZXRpb25Qcm9vZlJlY2VpcHRSZXNwb25zZRJDCg9hY2tub3'
        'dsZWRnZWRfYXQYASABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUg5hY2tub3dsZWRn'
        'ZWRBdA==');
