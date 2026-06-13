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

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
