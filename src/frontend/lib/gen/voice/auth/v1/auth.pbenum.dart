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

class EmailVerificationState extends $pb.ProtobufEnum {
  static const EmailVerificationState EMAIL_VERIFICATION_STATE_UNSPECIFIED =
      EmailVerificationState._(
          0, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_STATE_UNSPECIFIED');
  static const EmailVerificationState EMAIL_VERIFICATION_STATE_GUEST =
      EmailVerificationState._(
          1, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_STATE_GUEST');
  static const EmailVerificationState EMAIL_VERIFICATION_STATE_EMAIL_PENDING =
      EmailVerificationState._(
          2, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_STATE_EMAIL_PENDING');
  static const EmailVerificationState
      EMAIL_VERIFICATION_STATE_PROMOTION_PENDING = EmailVerificationState._(3,
          _omitEnumNames ? '' : 'EMAIL_VERIFICATION_STATE_PROMOTION_PENDING');
  static const EmailVerificationState EMAIL_VERIFICATION_STATE_REGULAR =
      EmailVerificationState._(
          4, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_STATE_REGULAR');

  static const $core.List<EmailVerificationState> values =
      <EmailVerificationState>[
    EMAIL_VERIFICATION_STATE_UNSPECIFIED,
    EMAIL_VERIFICATION_STATE_GUEST,
    EMAIL_VERIFICATION_STATE_EMAIL_PENDING,
    EMAIL_VERIFICATION_STATE_PROMOTION_PENDING,
    EMAIL_VERIFICATION_STATE_REGULAR,
  ];

  static final $core.List<EmailVerificationState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static EmailVerificationState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EmailVerificationState._(super.value, super.name);
}

class EmailVerificationCodeState extends $pb.ProtobufEnum {
  static const EmailVerificationCodeState
      EMAIL_VERIFICATION_CODE_STATE_UNSPECIFIED = EmailVerificationCodeState._(
          0, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_CODE_STATE_UNSPECIFIED');
  static const EmailVerificationCodeState EMAIL_VERIFICATION_CODE_STATE_NONE =
      EmailVerificationCodeState._(
          1, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_CODE_STATE_NONE');
  static const EmailVerificationCodeState EMAIL_VERIFICATION_CODE_STATE_ACTIVE =
      EmailVerificationCodeState._(
          2, _omitEnumNames ? '' : 'EMAIL_VERIFICATION_CODE_STATE_ACTIVE');

  static const $core.List<EmailVerificationCodeState> values =
      <EmailVerificationCodeState>[
    EMAIL_VERIFICATION_CODE_STATE_UNSPECIFIED,
    EMAIL_VERIFICATION_CODE_STATE_NONE,
    EMAIL_VERIFICATION_CODE_STATE_ACTIVE,
  ];

  static final $core.List<EmailVerificationCodeState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static EmailVerificationCodeState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EmailVerificationCodeState._(super.value, super.name);
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

class SpaceAdmissionPhase extends $pb.ProtobufEnum {
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_UNSPECIFIED =
      SpaceAdmissionPhase._(
          0, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_UNSPECIFIED');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_START =
      SpaceAdmissionPhase._(
          1, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_START');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_READ =
      SpaceAdmissionPhase._(
          2, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_READ');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_SUBMIT =
      SpaceAdmissionPhase._(
          3, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_SUBMIT');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_APPROVE =
      SpaceAdmissionPhase._(
          4, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_APPROVE');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_FIRST_PUBLISH =
      SpaceAdmissionPhase._(
          5, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_FIRST_PUBLISH');
  static const SpaceAdmissionPhase SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY =
      SpaceAdmissionPhase._(
          6, _omitEnumNames ? '' : 'SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY');

  static const $core.List<SpaceAdmissionPhase> values = <SpaceAdmissionPhase>[
    SPACE_ADMISSION_PHASE_UNSPECIFIED,
    SPACE_ADMISSION_PHASE_START,
    SPACE_ADMISSION_PHASE_READ,
    SPACE_ADMISSION_PHASE_SUBMIT,
    SPACE_ADMISSION_PHASE_APPROVE,
    SPACE_ADMISSION_PHASE_FIRST_PUBLISH,
    SPACE_ADMISSION_PHASE_SET_ENTRY_POLICY,
  ];

  static final $core.List<SpaceAdmissionPhase?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static SpaceAdmissionPhase? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAdmissionPhase._(super.value, super.name);
}

class SpaceAdmissionSubjectRole extends $pb.ProtobufEnum {
  static const SpaceAdmissionSubjectRole
      SPACE_ADMISSION_SUBJECT_ROLE_UNSPECIFIED = SpaceAdmissionSubjectRole._(
          0, _omitEnumNames ? '' : 'SPACE_ADMISSION_SUBJECT_ROLE_UNSPECIFIED');
  static const SpaceAdmissionSubjectRole
      SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT = SpaceAdmissionSubjectRole._(
          1, _omitEnumNames ? '' : 'SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT');
  static const SpaceAdmissionSubjectRole SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER =
      SpaceAdmissionSubjectRole._(
          2, _omitEnumNames ? '' : 'SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER');
  static const SpaceAdmissionSubjectRole
      SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR = SpaceAdmissionSubjectRole._(
          3,
          _omitEnumNames ? '' : 'SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR');

  static const $core.List<SpaceAdmissionSubjectRole> values =
      <SpaceAdmissionSubjectRole>[
    SPACE_ADMISSION_SUBJECT_ROLE_UNSPECIFIED,
    SPACE_ADMISSION_SUBJECT_ROLE_APPLICANT,
    SPACE_ADMISSION_SUBJECT_ROLE_REVIEWER,
    SPACE_ADMISSION_SUBJECT_ROLE_SETTINGS_ACTOR,
  ];

  static final $core.List<SpaceAdmissionSubjectRole?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static SpaceAdmissionSubjectRole? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAdmissionSubjectRole._(super.value, super.name);
}

class SpaceAdmissionDecision extends $pb.ProtobufEnum {
  static const SpaceAdmissionDecision SPACE_ADMISSION_DECISION_UNSPECIFIED =
      SpaceAdmissionDecision._(
          0, _omitEnumNames ? '' : 'SPACE_ADMISSION_DECISION_UNSPECIFIED');
  static const SpaceAdmissionDecision SPACE_ADMISSION_DECISION_ELIGIBLE =
      SpaceAdmissionDecision._(
          1, _omitEnumNames ? '' : 'SPACE_ADMISSION_DECISION_ELIGIBLE');

  static const $core.List<SpaceAdmissionDecision> values =
      <SpaceAdmissionDecision>[
    SPACE_ADMISSION_DECISION_UNSPECIFIED,
    SPACE_ADMISSION_DECISION_ELIGIBLE,
  ];

  static final $core.List<SpaceAdmissionDecision?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static SpaceAdmissionDecision? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAdmissionDecision._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
