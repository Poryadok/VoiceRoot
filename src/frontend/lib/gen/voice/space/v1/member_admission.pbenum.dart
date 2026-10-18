// This is a generated file - do not edit.
//
// Generated from voice/space/v1/member_admission.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class MemberAdmissionLifecycleState extends $pb.ProtobufEnum {
  static const MemberAdmissionLifecycleState
      MEMBER_ADMISSION_LIFECYCLE_STATE_UNSPECIFIED =
      MemberAdmissionLifecycleState._(0,
          _omitEnumNames ? '' : 'MEMBER_ADMISSION_LIFECYCLE_STATE_UNSPECIFIED');
  static const MemberAdmissionLifecycleState
      MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE = MemberAdmissionLifecycleState._(
          1, _omitEnumNames ? '' : 'MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE');
  static const MemberAdmissionLifecycleState
      MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN = MemberAdmissionLifecycleState._(
          2, _omitEnumNames ? '' : 'MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN');
  static const MemberAdmissionLifecycleState
      MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED =
      MemberAdmissionLifecycleState._(
          3,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED');
  static const MemberAdmissionLifecycleState
      MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED =
      MemberAdmissionLifecycleState._(
          4, _omitEnumNames ? '' : 'MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED');

  static const $core.List<MemberAdmissionLifecycleState> values =
      <MemberAdmissionLifecycleState>[
    MEMBER_ADMISSION_LIFECYCLE_STATE_UNSPECIFIED,
    MEMBER_ADMISSION_LIFECYCLE_STATE_LIVE,
    MEMBER_ADMISSION_LIFECYCLE_STATE_FROZEN,
    MEMBER_ADMISSION_LIFECYCLE_STATE_PURGE_DECIDED,
    MEMBER_ADMISSION_LIFECYCLE_STATE_RETIRED,
  ];

  static final $core.List<MemberAdmissionLifecycleState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static MemberAdmissionLifecycleState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberAdmissionLifecycleState._(super.value, super.name);
}

class MemberAdmissionOwnershipState extends $pb.ProtobufEnum {
  static const MemberAdmissionOwnershipState
      MEMBER_ADMISSION_OWNERSHIP_STATE_UNSPECIFIED =
      MemberAdmissionOwnershipState._(0,
          _omitEnumNames ? '' : 'MEMBER_ADMISSION_OWNERSHIP_STATE_UNSPECIFIED');
  static const MemberAdmissionOwnershipState
      MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE =
      MemberAdmissionOwnershipState._(1,
          _omitEnumNames ? '' : 'MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE');
  static const MemberAdmissionOwnershipState
      MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED =
      MemberAdmissionOwnershipState._(
          2, _omitEnumNames ? '' : 'MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED');
  static const MemberAdmissionOwnershipState
      MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED =
      MemberAdmissionOwnershipState._(
          3, _omitEnumNames ? '' : 'MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED');

  static const $core.List<MemberAdmissionOwnershipState> values =
      <MemberAdmissionOwnershipState>[
    MEMBER_ADMISSION_OWNERSHIP_STATE_UNSPECIFIED,
    MEMBER_ADMISSION_OWNERSHIP_STATE_AVAILABLE,
    MEMBER_ADMISSION_OWNERSHIP_STATE_PREPARED,
    MEMBER_ADMISSION_OWNERSHIP_STATE_RETIRED,
  ];

  static final $core.List<MemberAdmissionOwnershipState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static MemberAdmissionOwnershipState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberAdmissionOwnershipState._(super.value, super.name);
}

class MemberAdmissionSourceState extends $pb.ProtobufEnum {
  static const MemberAdmissionSourceState
      MEMBER_ADMISSION_SOURCE_STATE_UNSPECIFIED = MemberAdmissionSourceState._(
          0, _omitEnumNames ? '' : 'MEMBER_ADMISSION_SOURCE_STATE_UNSPECIFIED');
  static const MemberAdmissionSourceState MEMBER_ADMISSION_SOURCE_STATE_NONE =
      MemberAdmissionSourceState._(
          1, _omitEnumNames ? '' : 'MEMBER_ADMISSION_SOURCE_STATE_NONE');
  static const MemberAdmissionSourceState
      MEMBER_ADMISSION_SOURCE_STATE_PREPARING = MemberAdmissionSourceState._(
          2, _omitEnumNames ? '' : 'MEMBER_ADMISSION_SOURCE_STATE_PREPARING');
  static const MemberAdmissionSourceState MEMBER_ADMISSION_SOURCE_STATE_ACTIVE =
      MemberAdmissionSourceState._(
          3, _omitEnumNames ? '' : 'MEMBER_ADMISSION_SOURCE_STATE_ACTIVE');
  static const MemberAdmissionSourceState
      MEMBER_ADMISSION_SOURCE_STATE_RETIRED = MemberAdmissionSourceState._(
          4, _omitEnumNames ? '' : 'MEMBER_ADMISSION_SOURCE_STATE_RETIRED');

  static const $core.List<MemberAdmissionSourceState> values =
      <MemberAdmissionSourceState>[
    MEMBER_ADMISSION_SOURCE_STATE_UNSPECIFIED,
    MEMBER_ADMISSION_SOURCE_STATE_NONE,
    MEMBER_ADMISSION_SOURCE_STATE_PREPARING,
    MEMBER_ADMISSION_SOURCE_STATE_ACTIVE,
    MEMBER_ADMISSION_SOURCE_STATE_RETIRED,
  ];

  static final $core.List<MemberAdmissionSourceState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static MemberAdmissionSourceState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberAdmissionSourceState._(super.value, super.name);
}

/// Publication provenance does not grant a role or add another membership source.
class MemberAdmissionPublicationKind extends $pb.ProtobufEnum {
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_UNSPECIFIED =
      MemberAdmissionPublicationKind._(
          0,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_UNSPECIFIED');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY =
      MemberAdmissionPublicationKind._(
          1,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED =
      MemberAdmissionPublicationKind._(
          2,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE =
      MemberAdmissionPublicationKind._(
          3,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE =
      MemberAdmissionPublicationKind._(
          4,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY =
      MemberAdmissionPublicationKind._(
          5,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY');
  static const MemberAdmissionPublicationKind
      MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL =
      MemberAdmissionPublicationKind._(
          6,
          _omitEnumNames
              ? ''
              : 'MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL');

  static const $core.List<MemberAdmissionPublicationKind> values =
      <MemberAdmissionPublicationKind>[
    MEMBER_ADMISSION_PUBLICATION_KIND_UNSPECIFIED,
    MEMBER_ADMISSION_PUBLICATION_KIND_ADMISSION_ROLE_READY,
    MEMBER_ADMISSION_PUBLICATION_KIND_LEGACY_MIGRATED,
    MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE,
    MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE,
    MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY,
    MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL,
  ];

  static final $core.List<MemberAdmissionPublicationKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 6);
  static MemberAdmissionPublicationKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberAdmissionPublicationKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
