// This is a generated file - do not edit.
//
// Generated from voice/role/v1/role.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Successful receipts never use UNSPECIFIED. Prepared freezes scoped authority.
class OwnershipTransferState extends $pb.ProtobufEnum {
  static const OwnershipTransferState OWNERSHIP_TRANSFER_STATE_UNSPECIFIED =
      OwnershipTransferState._(
          0, _omitEnumNames ? '' : 'OWNERSHIP_TRANSFER_STATE_UNSPECIFIED');
  static const OwnershipTransferState OWNERSHIP_TRANSFER_STATE_PREPARED =
      OwnershipTransferState._(
          1, _omitEnumNames ? '' : 'OWNERSHIP_TRANSFER_STATE_PREPARED');
  static const OwnershipTransferState OWNERSHIP_TRANSFER_STATE_FINALIZED =
      OwnershipTransferState._(
          2, _omitEnumNames ? '' : 'OWNERSHIP_TRANSFER_STATE_FINALIZED');
  static const OwnershipTransferState OWNERSHIP_TRANSFER_STATE_ABORTED =
      OwnershipTransferState._(
          3, _omitEnumNames ? '' : 'OWNERSHIP_TRANSFER_STATE_ABORTED');

  static const $core.List<OwnershipTransferState> values =
      <OwnershipTransferState>[
    OWNERSHIP_TRANSFER_STATE_UNSPECIFIED,
    OWNERSHIP_TRANSFER_STATE_PREPARED,
    OWNERSHIP_TRANSFER_STATE_FINALIZED,
    OWNERSHIP_TRANSFER_STATE_ABORTED,
  ];

  static final $core.List<OwnershipTransferState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static OwnershipTransferState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const OwnershipTransferState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
