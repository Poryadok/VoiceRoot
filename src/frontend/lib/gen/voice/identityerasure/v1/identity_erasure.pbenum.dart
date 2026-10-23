// This is a generated file - do not edit.
//
// Generated from voice/identityerasure/v1/identity_erasure.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class Eligibility extends $pb.ProtobufEnum {
  static const Eligibility ELIGIBILITY_UNSPECIFIED =
      Eligibility._(0, _omitEnumNames ? '' : 'ELIGIBILITY_UNSPECIFIED');
  static const Eligibility ELIGIBILITY_ELIGIBLE =
      Eligibility._(1, _omitEnumNames ? '' : 'ELIGIBILITY_ELIGIBLE');
  static const Eligibility ELIGIBILITY_INELIGIBLE =
      Eligibility._(2, _omitEnumNames ? '' : 'ELIGIBILITY_INELIGIBLE');
  static const Eligibility ELIGIBILITY_UNAVAILABLE =
      Eligibility._(3, _omitEnumNames ? '' : 'ELIGIBILITY_UNAVAILABLE');

  static const $core.List<Eligibility> values = <Eligibility>[
    ELIGIBILITY_UNSPECIFIED,
    ELIGIBILITY_ELIGIBLE,
    ELIGIBILITY_INELIGIBLE,
    ELIGIBILITY_UNAVAILABLE,
  ];

  static final $core.List<Eligibility?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static Eligibility? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const Eligibility._(super.value, super.name);
}

class ErasureDisposition extends $pb.ProtobufEnum {
  static const ErasureDisposition ERASURE_DISPOSITION_UNSPECIFIED =
      ErasureDisposition._(
          0, _omitEnumNames ? '' : 'ERASURE_DISPOSITION_UNSPECIFIED');
  static const ErasureDisposition ERASURE_DISPOSITION_NOT_IRREVERSIBLE =
      ErasureDisposition._(
          1, _omitEnumNames ? '' : 'ERASURE_DISPOSITION_NOT_IRREVERSIBLE');
  static const ErasureDisposition ERASURE_DISPOSITION_IRREVERSIBLE =
      ErasureDisposition._(
          2, _omitEnumNames ? '' : 'ERASURE_DISPOSITION_IRREVERSIBLE');
  static const ErasureDisposition ERASURE_DISPOSITION_INCOMPLETE =
      ErasureDisposition._(
          3, _omitEnumNames ? '' : 'ERASURE_DISPOSITION_INCOMPLETE');

  static const $core.List<ErasureDisposition> values = <ErasureDisposition>[
    ERASURE_DISPOSITION_UNSPECIFIED,
    ERASURE_DISPOSITION_NOT_IRREVERSIBLE,
    ERASURE_DISPOSITION_IRREVERSIBLE,
    ERASURE_DISPOSITION_INCOMPLETE,
  ];

  static final $core.List<ErasureDisposition?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ErasureDisposition? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ErasureDisposition._(super.value, super.name);
}

class AccountLifecycleState extends $pb.ProtobufEnum {
  static const AccountLifecycleState ACCOUNT_LIFECYCLE_STATE_UNSPECIFIED =
      AccountLifecycleState._(
          0, _omitEnumNames ? '' : 'ACCOUNT_LIFECYCLE_STATE_UNSPECIFIED');
  static const AccountLifecycleState ACCOUNT_LIFECYCLE_STATE_ACTIVE =
      AccountLifecycleState._(
          1, _omitEnumNames ? '' : 'ACCOUNT_LIFECYCLE_STATE_ACTIVE');
  static const AccountLifecycleState ACCOUNT_LIFECYCLE_STATE_RECOVERABLE =
      AccountLifecycleState._(
          2, _omitEnumNames ? '' : 'ACCOUNT_LIFECYCLE_STATE_RECOVERABLE');
  static const AccountLifecycleState ACCOUNT_LIFECYCLE_STATE_TERMINAL =
      AccountLifecycleState._(
          3, _omitEnumNames ? '' : 'ACCOUNT_LIFECYCLE_STATE_TERMINAL');

  static const $core.List<AccountLifecycleState> values =
      <AccountLifecycleState>[
    ACCOUNT_LIFECYCLE_STATE_UNSPECIFIED,
    ACCOUNT_LIFECYCLE_STATE_ACTIVE,
    ACCOUNT_LIFECYCLE_STATE_RECOVERABLE,
    ACCOUNT_LIFECYCLE_STATE_TERMINAL,
  ];

  static final $core.List<AccountLifecycleState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static AccountLifecycleState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AccountLifecycleState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
