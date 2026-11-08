// This is a generated file - do not edit.
//
// Generated from voice/user/v1/auth_profile_authority.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class AuthProfileMatchOutcome extends $pb.ProtobufEnum {
  static const AuthProfileMatchOutcome AUTH_PROFILE_MATCH_OUTCOME_UNSPECIFIED =
      AuthProfileMatchOutcome._(
          0, _omitEnumNames ? '' : 'AUTH_PROFILE_MATCH_OUTCOME_UNSPECIFIED');
  static const AuthProfileMatchOutcome AUTH_PROFILE_MATCH_OUTCOME_MATCH =
      AuthProfileMatchOutcome._(
          1, _omitEnumNames ? '' : 'AUTH_PROFILE_MATCH_OUTCOME_MATCH');
  static const AuthProfileMatchOutcome
      AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT = AuthProfileMatchOutcome._(
          2,
          _omitEnumNames ? '' : 'AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT');
  static const AuthProfileMatchOutcome AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE =
      AuthProfileMatchOutcome._(
          3, _omitEnumNames ? '' : 'AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE');

  static const $core.List<AuthProfileMatchOutcome> values =
      <AuthProfileMatchOutcome>[
    AUTH_PROFILE_MATCH_OUTCOME_UNSPECIFIED,
    AUTH_PROFILE_MATCH_OUTCOME_MATCH,
    AUTH_PROFILE_MATCH_OUTCOME_DIFFERENT_ACCOUNT,
    AUTH_PROFILE_MATCH_OUTCOME_INCOMPLETE,
  ];

  static final $core.List<AuthProfileMatchOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static AuthProfileMatchOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AuthProfileMatchOutcome._(super.value, super.name);
}

class AuthOwnershipTargetOutcome extends $pb.ProtobufEnum {
  static const AuthOwnershipTargetOutcome
      AUTH_OWNERSHIP_TARGET_OUTCOME_UNSPECIFIED = AuthOwnershipTargetOutcome._(
          0, _omitEnumNames ? '' : 'AUTH_OWNERSHIP_TARGET_OUTCOME_UNSPECIFIED');
  static const AuthOwnershipTargetOutcome
      AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE =
      AuthOwnershipTargetOutcome._(
          1,
          _omitEnumNames
              ? ''
              : 'AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE');
  static const AuthOwnershipTargetOutcome
      AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE =
      AuthOwnershipTargetOutcome._(
          2,
          _omitEnumNames
              ? ''
              : 'AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE');
  static const AuthOwnershipTargetOutcome
      AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE = AuthOwnershipTargetOutcome._(
          3, _omitEnumNames ? '' : 'AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE');

  static const $core.List<AuthOwnershipTargetOutcome> values =
      <AuthOwnershipTargetOutcome>[
    AUTH_OWNERSHIP_TARGET_OUTCOME_UNSPECIFIED,
    AUTH_OWNERSHIP_TARGET_OUTCOME_CURRENT_PROFILE,
    AUTH_OWNERSHIP_TARGET_OUTCOME_TERMINAL_PROFILE,
    AUTH_OWNERSHIP_TARGET_OUTCOME_INCOMPLETE,
  ];

  static final $core.List<AuthOwnershipTargetOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static AuthOwnershipTargetOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AuthOwnershipTargetOutcome._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
