// This is a generated file - do not edit.
//
// Generated from voice/authority/v1/authority.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class AuthorityOwner extends $pb.ProtobufEnum {
  static const AuthorityOwner AUTHORITY_OWNER_UNSPECIFIED =
      AuthorityOwner._(0, _omitEnumNames ? '' : 'AUTHORITY_OWNER_UNSPECIFIED');
  static const AuthorityOwner AUTHORITY_OWNER_SPACE =
      AuthorityOwner._(1, _omitEnumNames ? '' : 'AUTHORITY_OWNER_SPACE');
  static const AuthorityOwner AUTHORITY_OWNER_ROLE =
      AuthorityOwner._(2, _omitEnumNames ? '' : 'AUTHORITY_OWNER_ROLE');
  static const AuthorityOwner AUTHORITY_OWNER_AUTH =
      AuthorityOwner._(3, _omitEnumNames ? '' : 'AUTHORITY_OWNER_AUTH');
  static const AuthorityOwner AUTHORITY_OWNER_USER =
      AuthorityOwner._(4, _omitEnumNames ? '' : 'AUTHORITY_OWNER_USER');
  static const AuthorityOwner AUTHORITY_OWNER_GAME_INTEGRATION =
      AuthorityOwner._(
          5, _omitEnumNames ? '' : 'AUTHORITY_OWNER_GAME_INTEGRATION');

  static const $core.List<AuthorityOwner> values = <AuthorityOwner>[
    AUTHORITY_OWNER_UNSPECIFIED,
    AUTHORITY_OWNER_SPACE,
    AUTHORITY_OWNER_ROLE,
    AUTHORITY_OWNER_AUTH,
    AUTHORITY_OWNER_USER,
    AUTHORITY_OWNER_GAME_INTEGRATION,
  ];

  static final $core.List<AuthorityOwner?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static AuthorityOwner? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const AuthorityOwner._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
