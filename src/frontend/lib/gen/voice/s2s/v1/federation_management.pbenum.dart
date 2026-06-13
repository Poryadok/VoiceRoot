// This is a generated file - do not edit.
//
// Generated from voice/s2s/v1/federation_management.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for FederationNode.status (string) and FederationNodeStatus.status.
class FederationNodeRegistrationStatus extends $pb.ProtobufEnum {
  static const FederationNodeRegistrationStatus
      FEDERATION_NODE_REGISTRATION_STATUS_UNSPECIFIED =
      FederationNodeRegistrationStatus._(
          0,
          _omitEnumNames
              ? ''
              : 'FEDERATION_NODE_REGISTRATION_STATUS_UNSPECIFIED');
  static const FederationNodeRegistrationStatus
      FEDERATION_NODE_REGISTRATION_STATUS_PENDING =
      FederationNodeRegistrationStatus._(1,
          _omitEnumNames ? '' : 'FEDERATION_NODE_REGISTRATION_STATUS_PENDING');
  static const FederationNodeRegistrationStatus
      FEDERATION_NODE_REGISTRATION_STATUS_ACTIVE =
      FederationNodeRegistrationStatus._(2,
          _omitEnumNames ? '' : 'FEDERATION_NODE_REGISTRATION_STATUS_ACTIVE');
  static const FederationNodeRegistrationStatus
      FEDERATION_NODE_REGISTRATION_STATUS_SUSPENDED =
      FederationNodeRegistrationStatus._(
          3,
          _omitEnumNames
              ? ''
              : 'FEDERATION_NODE_REGISTRATION_STATUS_SUSPENDED');
  static const FederationNodeRegistrationStatus
      FEDERATION_NODE_REGISTRATION_STATUS_DEFEDERATED =
      FederationNodeRegistrationStatus._(
          4,
          _omitEnumNames
              ? ''
              : 'FEDERATION_NODE_REGISTRATION_STATUS_DEFEDERATED');

  static const $core.List<FederationNodeRegistrationStatus> values =
      <FederationNodeRegistrationStatus>[
    FEDERATION_NODE_REGISTRATION_STATUS_UNSPECIFIED,
    FEDERATION_NODE_REGISTRATION_STATUS_PENDING,
    FEDERATION_NODE_REGISTRATION_STATUS_ACTIVE,
    FEDERATION_NODE_REGISTRATION_STATUS_SUSPENDED,
    FEDERATION_NODE_REGISTRATION_STATUS_DEFEDERATED,
  ];

  static final $core.List<FederationNodeRegistrationStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static FederationNodeRegistrationStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const FederationNodeRegistrationStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
