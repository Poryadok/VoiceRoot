// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/s2s.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for NotifyUserRequest.type (string).
class FederationPushEventType extends $pb.ProtobufEnum {
  static const FederationPushEventType FEDERATION_PUSH_EVENT_TYPE_UNSPECIFIED =
      FederationPushEventType._(
          0, _omitEnumNames ? '' : 'FEDERATION_PUSH_EVENT_TYPE_UNSPECIFIED');
  static const FederationPushEventType FEDERATION_PUSH_EVENT_TYPE_MENTION =
      FederationPushEventType._(
          1, _omitEnumNames ? '' : 'FEDERATION_PUSH_EVENT_TYPE_MENTION');
  static const FederationPushEventType FEDERATION_PUSH_EVENT_TYPE_DM =
      FederationPushEventType._(
          2, _omitEnumNames ? '' : 'FEDERATION_PUSH_EVENT_TYPE_DM');
  static const FederationPushEventType FEDERATION_PUSH_EVENT_TYPE_MATCH_FOUND =
      FederationPushEventType._(
          3, _omitEnumNames ? '' : 'FEDERATION_PUSH_EVENT_TYPE_MATCH_FOUND');
  static const FederationPushEventType
      FEDERATION_PUSH_EVENT_TYPE_CALL_INCOMING = FederationPushEventType._(
          4, _omitEnumNames ? '' : 'FEDERATION_PUSH_EVENT_TYPE_CALL_INCOMING');

  static const $core.List<FederationPushEventType> values =
      <FederationPushEventType>[
    FEDERATION_PUSH_EVENT_TYPE_UNSPECIFIED,
    FEDERATION_PUSH_EVENT_TYPE_MENTION,
    FEDERATION_PUSH_EVENT_TYPE_DM,
    FEDERATION_PUSH_EVENT_TYPE_MATCH_FOUND,
    FEDERATION_PUSH_EVENT_TYPE_CALL_INCOMING,
  ];

  static final $core.List<FederationPushEventType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static FederationPushEventType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FederationPushEventType._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
