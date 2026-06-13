// This is a generated file - do not edit.
//
// Generated from voice/user/v1/user.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for presence status strings (UpdatePresenceRequest, PresenceStatus).
class PresenceOnlineStatus extends $pb.ProtobufEnum {
  static const PresenceOnlineStatus PRESENCE_ONLINE_STATUS_UNSPECIFIED =
      PresenceOnlineStatus._(
          0, _omitEnumNames ? '' : 'PRESENCE_ONLINE_STATUS_UNSPECIFIED');
  static const PresenceOnlineStatus PRESENCE_ONLINE_STATUS_ONLINE =
      PresenceOnlineStatus._(
          1, _omitEnumNames ? '' : 'PRESENCE_ONLINE_STATUS_ONLINE');
  static const PresenceOnlineStatus PRESENCE_ONLINE_STATUS_IDLE =
      PresenceOnlineStatus._(
          2, _omitEnumNames ? '' : 'PRESENCE_ONLINE_STATUS_IDLE');
  static const PresenceOnlineStatus PRESENCE_ONLINE_STATUS_DND =
      PresenceOnlineStatus._(
          3, _omitEnumNames ? '' : 'PRESENCE_ONLINE_STATUS_DND');
  static const PresenceOnlineStatus PRESENCE_ONLINE_STATUS_INVISIBLE =
      PresenceOnlineStatus._(
          4, _omitEnumNames ? '' : 'PRESENCE_ONLINE_STATUS_INVISIBLE');

  static const $core.List<PresenceOnlineStatus> values = <PresenceOnlineStatus>[
    PRESENCE_ONLINE_STATUS_UNSPECIFIED,
    PRESENCE_ONLINE_STATUS_ONLINE,
    PRESENCE_ONLINE_STATUS_IDLE,
    PRESENCE_ONLINE_STATUS_DND,
    PRESENCE_ONLINE_STATUS_INVISIBLE,
  ];

  static final $core.List<PresenceOnlineStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static PresenceOnlineStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PresenceOnlineStatus._(super.value, super.name);
}

/// Canonical values for PrivacySettings.preset (string).
class PrivacyPreset extends $pb.ProtobufEnum {
  static const PrivacyPreset PRIVACY_PRESET_UNSPECIFIED =
      PrivacyPreset._(0, _omitEnumNames ? '' : 'PRIVACY_PRESET_UNSPECIFIED');
  static const PrivacyPreset PRIVACY_PRESET_PERSONAL =
      PrivacyPreset._(1, _omitEnumNames ? '' : 'PRIVACY_PRESET_PERSONAL');
  static const PrivacyPreset PRIVACY_PRESET_GAMING =
      PrivacyPreset._(2, _omitEnumNames ? '' : 'PRIVACY_PRESET_GAMING');
  static const PrivacyPreset PRIVACY_PRESET_WORK =
      PrivacyPreset._(3, _omitEnumNames ? '' : 'PRIVACY_PRESET_WORK');

  static const $core.List<PrivacyPreset> values = <PrivacyPreset>[
    PRIVACY_PRESET_UNSPECIFIED,
    PRIVACY_PRESET_PERSONAL,
    PRIVACY_PRESET_GAMING,
    PRIVACY_PRESET_WORK,
  ];

  static final $core.List<PrivacyPreset?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static PrivacyPreset? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PrivacyPreset._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
