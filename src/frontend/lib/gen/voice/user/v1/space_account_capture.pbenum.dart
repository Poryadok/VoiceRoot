// This is a generated file - do not edit.
//
// Generated from voice/user/v1/space_account_capture.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Private association evidence, never a current eligibility or admission grant.
class SpaceAccountCapturePurpose extends $pb.ProtobufEnum {
  static const SpaceAccountCapturePurpose
      SPACE_ACCOUNT_CAPTURE_PURPOSE_UNSPECIFIED = SpaceAccountCapturePurpose._(
          0, _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_PURPOSE_UNSPECIFIED');
  static const SpaceAccountCapturePurpose
      SPACE_ACCOUNT_CAPTURE_PURPOSE_LEGACY_QUALIFICATION =
      SpaceAccountCapturePurpose._(
          1,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_CAPTURE_PURPOSE_LEGACY_QUALIFICATION');
  static const SpaceAccountCapturePurpose
      SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT =
      SpaceAccountCapturePurpose._(2,
          _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT');
  static const SpaceAccountCapturePurpose
      SPACE_ACCOUNT_CAPTURE_PURPOSE_COMMUNITY_PUBLICATION =
      SpaceAccountCapturePurpose._(
          3,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_CAPTURE_PURPOSE_COMMUNITY_PUBLICATION');

  static const $core.List<SpaceAccountCapturePurpose> values =
      <SpaceAccountCapturePurpose>[
    SPACE_ACCOUNT_CAPTURE_PURPOSE_UNSPECIFIED,
    SPACE_ACCOUNT_CAPTURE_PURPOSE_LEGACY_QUALIFICATION,
    SPACE_ACCOUNT_CAPTURE_PURPOSE_BAN_PREFLIGHT,
    SPACE_ACCOUNT_CAPTURE_PURPOSE_COMMUNITY_PUBLICATION,
  ];

  static final $core.List<SpaceAccountCapturePurpose?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static SpaceAccountCapturePurpose? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAccountCapturePurpose._(super.value, super.name);
}

class SpaceAccountCaptureOutcome extends $pb.ProtobufEnum {
  static const SpaceAccountCaptureOutcome
      SPACE_ACCOUNT_CAPTURE_OUTCOME_UNSPECIFIED = SpaceAccountCaptureOutcome._(
          0, _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_OUTCOME_UNSPECIFIED');
  static const SpaceAccountCaptureOutcome
      SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED = SpaceAccountCaptureOutcome._(
          1, _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED');
  static const SpaceAccountCaptureOutcome
      SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE =
      SpaceAccountCaptureOutcome._(
          2,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE');
  static const SpaceAccountCaptureOutcome
      SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE = SpaceAccountCaptureOutcome._(
          3, _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE');

  static const $core.List<SpaceAccountCaptureOutcome> values =
      <SpaceAccountCaptureOutcome>[
    SPACE_ACCOUNT_CAPTURE_OUTCOME_UNSPECIFIED,
    SPACE_ACCOUNT_CAPTURE_OUTCOME_ASSOCIATED,
    SPACE_ACCOUNT_CAPTURE_OUTCOME_TERMINAL_PROFILE,
    SPACE_ACCOUNT_CAPTURE_OUTCOME_INCOMPLETE,
  ];

  static final $core.List<SpaceAccountCaptureOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static SpaceAccountCaptureOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAccountCaptureOutcome._(super.value, super.name);
}

class SpaceAccountCaptureOrigin extends $pb.ProtobufEnum {
  static const SpaceAccountCaptureOrigin
      SPACE_ACCOUNT_CAPTURE_ORIGIN_UNSPECIFIED = SpaceAccountCaptureOrigin._(
          0, _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_ORIGIN_UNSPECIFIED');
  static const SpaceAccountCaptureOrigin
      SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW =
      SpaceAccountCaptureOrigin._(
          1,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW');
  static const SpaceAccountCaptureOrigin
      SPACE_ACCOUNT_CAPTURE_ORIGIN_OWNER_HISTORY = SpaceAccountCaptureOrigin._(
          2,
          _omitEnumNames ? '' : 'SPACE_ACCOUNT_CAPTURE_ORIGIN_OWNER_HISTORY');

  static const $core.List<SpaceAccountCaptureOrigin> values =
      <SpaceAccountCaptureOrigin>[
    SPACE_ACCOUNT_CAPTURE_ORIGIN_UNSPECIFIED,
    SPACE_ACCOUNT_CAPTURE_ORIGIN_EXISTING_CURRENT_ROW,
    SPACE_ACCOUNT_CAPTURE_ORIGIN_OWNER_HISTORY,
  ];

  static final $core.List<SpaceAccountCaptureOrigin?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SpaceAccountCaptureOrigin? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAccountCaptureOrigin._(super.value, super.name);
}

/// Immutable canonical association provenance; parent enrichment never rewrites it.
class SpaceAccountAssociationOrigin extends $pb.ProtobufEnum {
  static const SpaceAccountAssociationOrigin
      SPACE_ACCOUNT_ASSOCIATION_ORIGIN_UNSPECIFIED =
      SpaceAccountAssociationOrigin._(0,
          _omitEnumNames ? '' : 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_UNSPECIFIED');
  static const SpaceAccountAssociationOrigin
      SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH = SpaceAccountAssociationOrigin._(
          1, _omitEnumNames ? '' : 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH');
  static const SpaceAccountAssociationOrigin
      SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW =
      SpaceAccountAssociationOrigin._(
          2,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW');
  static const SpaceAccountAssociationOrigin
      SPACE_ACCOUNT_ASSOCIATION_ORIGIN_VERIFIED_HISTORY =
      SpaceAccountAssociationOrigin._(
          3,
          _omitEnumNames
              ? ''
              : 'SPACE_ACCOUNT_ASSOCIATION_ORIGIN_VERIFIED_HISTORY');

  static const $core.List<SpaceAccountAssociationOrigin> values =
      <SpaceAccountAssociationOrigin>[
    SPACE_ACCOUNT_ASSOCIATION_ORIGIN_UNSPECIFIED,
    SPACE_ACCOUNT_ASSOCIATION_ORIGIN_BIRTH,
    SPACE_ACCOUNT_ASSOCIATION_ORIGIN_RETAINED_NOW,
    SPACE_ACCOUNT_ASSOCIATION_ORIGIN_VERIFIED_HISTORY,
  ];

  static final $core.List<SpaceAccountAssociationOrigin?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static SpaceAccountAssociationOrigin? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceAccountAssociationOrigin._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
