// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user_source_custody.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class UserSourcePermitFrameKind extends $pb.ProtobufEnum {
  static const UserSourcePermitFrameKind
      USER_SOURCE_PERMIT_FRAME_KIND_UNSPECIFIED = UserSourcePermitFrameKind._(
          0, _omitEnumNames ? '' : 'USER_SOURCE_PERMIT_FRAME_KIND_UNSPECIFIED');
  static const UserSourcePermitFrameKind USER_SOURCE_PERMIT_FRAME_KIND_GRANTED =
      UserSourcePermitFrameKind._(
          1, _omitEnumNames ? '' : 'USER_SOURCE_PERMIT_FRAME_KIND_GRANTED');
  static const UserSourcePermitFrameKind
      USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT = UserSourcePermitFrameKind._(
          2, _omitEnumNames ? '' : 'USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT');

  static const $core.List<UserSourcePermitFrameKind> values =
      <UserSourcePermitFrameKind>[
    USER_SOURCE_PERMIT_FRAME_KIND_UNSPECIFIED,
    USER_SOURCE_PERMIT_FRAME_KIND_GRANTED,
    USER_SOURCE_PERMIT_FRAME_KIND_HEARTBEAT,
  ];

  static final $core.List<UserSourcePermitFrameKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static UserSourcePermitFrameKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const UserSourcePermitFrameKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
