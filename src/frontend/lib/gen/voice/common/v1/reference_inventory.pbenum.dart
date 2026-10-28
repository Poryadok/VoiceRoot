// This is a generated file - do not edit.
//
// Generated from voice/common/v1/reference_inventory.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Dedicated migration coverage; these are not Space deletion producer IDs.
class ReferenceInventoryOwnerType extends $pb.ProtobufEnum {
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_UNSPECIFIED =
      ReferenceInventoryOwnerType._(0,
          _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_UNSPECIFIED');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_MESSAGE = ReferenceInventoryOwnerType._(
          1, _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_MESSAGE');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_STORY = ReferenceInventoryOwnerType._(
          2, _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_STORY');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_PROFILE_AVATAR =
      ReferenceInventoryOwnerType._(
          3,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_OWNER_TYPE_PROFILE_AVATAR');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_CHAT_AVATAR =
      ReferenceInventoryOwnerType._(4,
          _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_CHAT_AVATAR');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_SPACE_AVATAR =
      ReferenceInventoryOwnerType._(5,
          _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_SPACE_AVATAR');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_STICKER = ReferenceInventoryOwnerType._(
          6, _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_STICKER');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_GIF_ASSET = ReferenceInventoryOwnerType._(
          7, _omitEnumNames ? '' : 'REFERENCE_INVENTORY_OWNER_TYPE_GIF_ASSET');
  static const ReferenceInventoryOwnerType
      REFERENCE_INVENTORY_OWNER_TYPE_UPLOAD_SESSION =
      ReferenceInventoryOwnerType._(
          8,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_OWNER_TYPE_UPLOAD_SESSION');

  static const $core.List<ReferenceInventoryOwnerType> values =
      <ReferenceInventoryOwnerType>[
    REFERENCE_INVENTORY_OWNER_TYPE_UNSPECIFIED,
    REFERENCE_INVENTORY_OWNER_TYPE_MESSAGE,
    REFERENCE_INVENTORY_OWNER_TYPE_STORY,
    REFERENCE_INVENTORY_OWNER_TYPE_PROFILE_AVATAR,
    REFERENCE_INVENTORY_OWNER_TYPE_CHAT_AVATAR,
    REFERENCE_INVENTORY_OWNER_TYPE_SPACE_AVATAR,
    REFERENCE_INVENTORY_OWNER_TYPE_STICKER,
    REFERENCE_INVENTORY_OWNER_TYPE_GIF_ASSET,
    REFERENCE_INVENTORY_OWNER_TYPE_UPLOAD_SESSION,
  ];

  static final $core.List<ReferenceInventoryOwnerType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 8);
  static ReferenceInventoryOwnerType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReferenceInventoryOwnerType._(super.value, super.name);
}

class ReferenceInventoryProtectionReason extends $pb.ProtobufEnum {
  static const ReferenceInventoryProtectionReason
      REFERENCE_INVENTORY_PROTECTION_REASON_UNSPECIFIED =
      ReferenceInventoryProtectionReason._(
          0,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_PROTECTION_REASON_UNSPECIFIED');
  static const ReferenceInventoryProtectionReason
      REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED =
      ReferenceInventoryProtectionReason._(
          1,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED');
  static const ReferenceInventoryProtectionReason
      REFERENCE_INVENTORY_PROTECTION_REASON_CREATE_PENDING =
      ReferenceInventoryProtectionReason._(
          2,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_PROTECTION_REASON_CREATE_PENDING');
  static const ReferenceInventoryProtectionReason
      REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING =
      ReferenceInventoryProtectionReason._(
          3,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING');

  static const $core.List<ReferenceInventoryProtectionReason> values =
      <ReferenceInventoryProtectionReason>[
    REFERENCE_INVENTORY_PROTECTION_REASON_UNSPECIFIED,
    REFERENCE_INVENTORY_PROTECTION_REASON_RETAINED,
    REFERENCE_INVENTORY_PROTECTION_REASON_CREATE_PENDING,
    REFERENCE_INVENTORY_PROTECTION_REASON_RELEASE_PENDING,
  ];

  static final $core.List<ReferenceInventoryProtectionReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ReferenceInventoryProtectionReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReferenceInventoryProtectionReason._(super.value, super.name);
}

class ReferenceInventoryIssueKind extends $pb.ProtobufEnum {
  static const ReferenceInventoryIssueKind
      REFERENCE_INVENTORY_ISSUE_KIND_UNSPECIFIED =
      ReferenceInventoryIssueKind._(0,
          _omitEnumNames ? '' : 'REFERENCE_INVENTORY_ISSUE_KIND_UNSPECIFIED');
  static const ReferenceInventoryIssueKind
      REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY =
      ReferenceInventoryIssueKind._(
          1,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY');
  static const ReferenceInventoryIssueKind
      REFERENCE_INVENTORY_ISSUE_KIND_HIGHLIGHT_COVER_UNRESOLVED =
      ReferenceInventoryIssueKind._(
          2,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_ISSUE_KIND_HIGHLIGHT_COVER_UNRESOLVED');
  static const ReferenceInventoryIssueKind
      REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING =
      ReferenceInventoryIssueKind._(
          3,
          _omitEnumNames
              ? ''
              : 'REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING');

  static const $core.List<ReferenceInventoryIssueKind> values =
      <ReferenceInventoryIssueKind>[
    REFERENCE_INVENTORY_ISSUE_KIND_UNSPECIFIED,
    REFERENCE_INVENTORY_ISSUE_KIND_HISTORICAL_MEDIA_ANOMALY,
    REFERENCE_INVENTORY_ISSUE_KIND_HIGHLIGHT_COVER_UNRESOLVED,
    REFERENCE_INVENTORY_ISSUE_KIND_CONFLICTING_BINDING,
  ];

  static final $core.List<ReferenceInventoryIssueKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ReferenceInventoryIssueKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ReferenceInventoryIssueKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
