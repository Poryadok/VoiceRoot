// This is a generated file - do not edit.
//
// Generated from voice/events/v1/jetstream_events.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class SubscriptionAggregateKind extends $pb.ProtobufEnum {
  static const SubscriptionAggregateKind
      SUBSCRIPTION_AGGREGATE_KIND_UNSPECIFIED = SubscriptionAggregateKind._(
          0, _omitEnumNames ? '' : 'SUBSCRIPTION_AGGREGATE_KIND_UNSPECIFIED');
  static const SubscriptionAggregateKind SUBSCRIPTION_AGGREGATE_KIND_PERSONAL =
      SubscriptionAggregateKind._(
          1, _omitEnumNames ? '' : 'SUBSCRIPTION_AGGREGATE_KIND_PERSONAL');
  static const SubscriptionAggregateKind SUBSCRIPTION_AGGREGATE_KIND_SPACE =
      SubscriptionAggregateKind._(
          2, _omitEnumNames ? '' : 'SUBSCRIPTION_AGGREGATE_KIND_SPACE');

  static const $core.List<SubscriptionAggregateKind> values =
      <SubscriptionAggregateKind>[
    SUBSCRIPTION_AGGREGATE_KIND_UNSPECIFIED,
    SUBSCRIPTION_AGGREGATE_KIND_PERSONAL,
    SUBSCRIPTION_AGGREGATE_KIND_SPACE,
  ];

  static final $core.List<SubscriptionAggregateKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SubscriptionAggregateKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SubscriptionAggregateKind._(super.value, super.name);
}

class EntitlementState extends $pb.ProtobufEnum {
  static const EntitlementState ENTITLEMENT_STATE_UNSPECIFIED =
      EntitlementState._(
          0, _omitEnumNames ? '' : 'ENTITLEMENT_STATE_UNSPECIFIED');
  static const EntitlementState ENTITLEMENT_STATE_ACTIVE =
      EntitlementState._(1, _omitEnumNames ? '' : 'ENTITLEMENT_STATE_ACTIVE');
  static const EntitlementState ENTITLEMENT_STATE_GRACE_PERIOD =
      EntitlementState._(
          2, _omitEnumNames ? '' : 'ENTITLEMENT_STATE_GRACE_PERIOD');
  static const EntitlementState ENTITLEMENT_STATE_INACTIVE =
      EntitlementState._(3, _omitEnumNames ? '' : 'ENTITLEMENT_STATE_INACTIVE');

  static const $core.List<EntitlementState> values = <EntitlementState>[
    ENTITLEMENT_STATE_UNSPECIFIED,
    ENTITLEMENT_STATE_ACTIVE,
    ENTITLEMENT_STATE_GRACE_PERIOD,
    ENTITLEMENT_STATE_INACTIVE,
  ];

  static final $core.List<EntitlementState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static EntitlementState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EntitlementState._(super.value, super.name);
}

class EntitlementReason extends $pb.ProtobufEnum {
  static const EntitlementReason ENTITLEMENT_REASON_UNSPECIFIED =
      EntitlementReason._(
          0, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_UNSPECIFIED');
  static const EntitlementReason ENTITLEMENT_REASON_STARTED =
      EntitlementReason._(
          1, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_STARTED');
  static const EntitlementReason ENTITLEMENT_REASON_RENEWED =
      EntitlementReason._(
          2, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_RENEWED');
  static const EntitlementReason ENTITLEMENT_REASON_PAYMENT_FAILED =
      EntitlementReason._(
          3, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_PAYMENT_FAILED');
  static const EntitlementReason ENTITLEMENT_REASON_PAYMENT_RECOVERED =
      EntitlementReason._(
          4, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_PAYMENT_RECOVERED');
  static const EntitlementReason ENTITLEMENT_REASON_CANCEL_SCHEDULED =
      EntitlementReason._(
          5, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_CANCEL_SCHEDULED');
  static const EntitlementReason ENTITLEMENT_REASON_CANCEL_RESUMED =
      EntitlementReason._(
          6, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_CANCEL_RESUMED');
  static const EntitlementReason ENTITLEMENT_REASON_PERIOD_ENDED =
      EntitlementReason._(
          7, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_PERIOD_ENDED');
  static const EntitlementReason ENTITLEMENT_REASON_GRACE_EXPIRED =
      EntitlementReason._(
          8, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_GRACE_EXPIRED');
  static const EntitlementReason ENTITLEMENT_REASON_ACCOUNT_DELETE_SCHEDULED =
      EntitlementReason._(9,
          _omitEnumNames ? '' : 'ENTITLEMENT_REASON_ACCOUNT_DELETE_SCHEDULED');
  static const EntitlementReason ENTITLEMENT_REASON_ACCOUNT_RESTORED =
      EntitlementReason._(
          10, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_ACCOUNT_RESTORED');
  static const EntitlementReason ENTITLEMENT_REASON_PURCHASER_PURGED =
      EntitlementReason._(
          11, _omitEnumNames ? '' : 'ENTITLEMENT_REASON_PURCHASER_PURGED');

  static const $core.List<EntitlementReason> values = <EntitlementReason>[
    ENTITLEMENT_REASON_UNSPECIFIED,
    ENTITLEMENT_REASON_STARTED,
    ENTITLEMENT_REASON_RENEWED,
    ENTITLEMENT_REASON_PAYMENT_FAILED,
    ENTITLEMENT_REASON_PAYMENT_RECOVERED,
    ENTITLEMENT_REASON_CANCEL_SCHEDULED,
    ENTITLEMENT_REASON_CANCEL_RESUMED,
    ENTITLEMENT_REASON_PERIOD_ENDED,
    ENTITLEMENT_REASON_GRACE_EXPIRED,
    ENTITLEMENT_REASON_ACCOUNT_DELETE_SCHEDULED,
    ENTITLEMENT_REASON_ACCOUNT_RESTORED,
    ENTITLEMENT_REASON_PURCHASER_PURGED,
  ];

  static final $core.List<EntitlementReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 11);
  static EntitlementReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const EntitlementReason._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
