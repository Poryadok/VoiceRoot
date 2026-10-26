// This is a generated file - do not edit.
//
// Generated from voice/auth/v1/subscription_lifecycle.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class SubscriptionAccountCaptureCoverage extends $pb.ProtobufEnum {
  static const SubscriptionAccountCaptureCoverage
      SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNSPECIFIED =
      SubscriptionAccountCaptureCoverage._(
          0,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNSPECIFIED');

  /// Actual account state exists, but no captured lifecycle history qualifies it.
  static const SubscriptionAccountCaptureCoverage
      SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED =
      SubscriptionAccountCaptureCoverage._(
          1,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED');

  /// Complete retained capture history only, never paid eligibility or CURRENT.
  static const SubscriptionAccountCaptureCoverage
      SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED =
      SubscriptionAccountCaptureCoverage._(
          2,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED');

  static const $core.List<SubscriptionAccountCaptureCoverage> values =
      <SubscriptionAccountCaptureCoverage>[
    SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNSPECIFIED,
    SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED,
    SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED,
  ];

  static final $core.List<SubscriptionAccountCaptureCoverage?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SubscriptionAccountCaptureCoverage? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SubscriptionAccountCaptureCoverage._(super.value, super.name);
}

class SubscriptionAccountLifecycleKind extends $pb.ProtobufEnum {
  static const SubscriptionAccountLifecycleKind
      SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_UNSPECIFIED =
      SubscriptionAccountLifecycleKind._(
          0,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_UNSPECIFIED');
  static const SubscriptionAccountLifecycleKind
      SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED =
      SubscriptionAccountLifecycleKind._(
          1,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED');
  static const SubscriptionAccountLifecycleKind
      SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED =
      SubscriptionAccountLifecycleKind._(
          2,
          _omitEnumNames
              ? ''
              : 'SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED');

  static const $core.List<SubscriptionAccountLifecycleKind> values =
      <SubscriptionAccountLifecycleKind>[
    SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_UNSPECIFIED,
    SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED,
    SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED,
  ];

  static final $core.List<SubscriptionAccountLifecycleKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SubscriptionAccountLifecycleKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SubscriptionAccountLifecycleKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
