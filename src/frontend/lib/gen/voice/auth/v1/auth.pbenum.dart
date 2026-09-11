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

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for VerifyOTPRequest.otp_type (string).
class OtpType extends $pb.ProtobufEnum {
  static const OtpType OTP_TYPE_UNSPECIFIED =
      OtpType._(0, _omitEnumNames ? '' : 'OTP_TYPE_UNSPECIFIED');
  static const OtpType OTP_TYPE_EMAIL_VERIFY =
      OtpType._(1, _omitEnumNames ? '' : 'OTP_TYPE_EMAIL_VERIFY');
  static const OtpType OTP_TYPE_PASSWORD_RESET =
      OtpType._(2, _omitEnumNames ? '' : 'OTP_TYPE_PASSWORD_RESET');

  static const $core.List<OtpType> values = <OtpType>[
    OTP_TYPE_UNSPECIFIED,
    OTP_TYPE_EMAIL_VERIFY,
    OTP_TYPE_PASSWORD_RESET,
  ];

  static final $core.List<OtpType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static OtpType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const OtpType._(super.value, super.name);
}

class VerifiedFactor extends $pb.ProtobufEnum {
  static const VerifiedFactor VERIFIED_FACTOR_UNSPECIFIED =
      VerifiedFactor._(0, _omitEnumNames ? '' : 'VERIFIED_FACTOR_UNSPECIFIED');
  static const VerifiedFactor VERIFIED_FACTOR_PASSWORD =
      VerifiedFactor._(1, _omitEnumNames ? '' : 'VERIFIED_FACTOR_PASSWORD');
  static const VerifiedFactor VERIFIED_FACTOR_TOTP =
      VerifiedFactor._(2, _omitEnumNames ? '' : 'VERIFIED_FACTOR_TOTP');
  static const VerifiedFactor VERIFIED_FACTOR_BACKUP_CODE =
      VerifiedFactor._(3, _omitEnumNames ? '' : 'VERIFIED_FACTOR_BACKUP_CODE');

  static const $core.List<VerifiedFactor> values = <VerifiedFactor>[
    VERIFIED_FACTOR_UNSPECIFIED,
    VERIFIED_FACTOR_PASSWORD,
    VERIFIED_FACTOR_TOTP,
    VERIFIED_FACTOR_BACKUP_CODE,
  ];

  static final $core.List<VerifiedFactor?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static VerifiedFactor? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const VerifiedFactor._(super.value, super.name);
}

class ProofPurpose extends $pb.ProtobufEnum {
  static const ProofPurpose PROOF_PURPOSE_UNSPECIFIED =
      ProofPurpose._(0, _omitEnumNames ? '' : 'PROOF_PURPOSE_UNSPECIFIED');
  static const ProofPurpose PROOF_PURPOSE_SPACE_DELETE =
      ProofPurpose._(1, _omitEnumNames ? '' : 'PROOF_PURPOSE_SPACE_DELETE');

  static const $core.List<ProofPurpose> values = <ProofPurpose>[
    PROOF_PURPOSE_UNSPECIFIED,
    PROOF_PURPOSE_SPACE_DELETE,
  ];

  static final $core.List<ProofPurpose?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static ProofPurpose? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ProofPurpose._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
