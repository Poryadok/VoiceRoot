// This is a generated file - do not edit.
//
// Generated from voice/common/v1/space_lifecycle.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class ParticipantId extends $pb.ProtobufEnum {
  static const ParticipantId PARTICIPANT_ID_UNSPECIFIED =
      ParticipantId._(0, _omitEnumNames ? '' : 'PARTICIPANT_ID_UNSPECIFIED');
  static const ParticipantId PARTICIPANT_ID_ROLE =
      ParticipantId._(1, _omitEnumNames ? '' : 'PARTICIPANT_ID_ROLE');
  static const ParticipantId PARTICIPANT_ID_CHAT =
      ParticipantId._(2, _omitEnumNames ? '' : 'PARTICIPANT_ID_CHAT');
  static const ParticipantId PARTICIPANT_ID_MESSAGING =
      ParticipantId._(3, _omitEnumNames ? '' : 'PARTICIPANT_ID_MESSAGING');
  static const ParticipantId PARTICIPANT_ID_FILE =
      ParticipantId._(4, _omitEnumNames ? '' : 'PARTICIPANT_ID_FILE');
  static const ParticipantId PARTICIPANT_ID_VOICE =
      ParticipantId._(5, _omitEnumNames ? '' : 'PARTICIPANT_ID_VOICE');
  static const ParticipantId PARTICIPANT_ID_MATCHMAKING =
      ParticipantId._(6, _omitEnumNames ? '' : 'PARTICIPANT_ID_MATCHMAKING');
  static const ParticipantId PARTICIPANT_ID_SEARCH =
      ParticipantId._(7, _omitEnumNames ? '' : 'PARTICIPANT_ID_SEARCH');
  static const ParticipantId PARTICIPANT_ID_SUBSCRIPTION =
      ParticipantId._(8, _omitEnumNames ? '' : 'PARTICIPANT_ID_SUBSCRIPTION');
  static const ParticipantId PARTICIPANT_ID_BOT =
      ParticipantId._(9, _omitEnumNames ? '' : 'PARTICIPANT_ID_BOT');
  static const ParticipantId PARTICIPANT_ID_NOTIFICATION =
      ParticipantId._(10, _omitEnumNames ? '' : 'PARTICIPANT_ID_NOTIFICATION');

  static const $core.List<ParticipantId> values = <ParticipantId>[
    PARTICIPANT_ID_UNSPECIFIED,
    PARTICIPANT_ID_ROLE,
    PARTICIPANT_ID_CHAT,
    PARTICIPANT_ID_MESSAGING,
    PARTICIPANT_ID_FILE,
    PARTICIPANT_ID_VOICE,
    PARTICIPANT_ID_MATCHMAKING,
    PARTICIPANT_ID_SEARCH,
    PARTICIPANT_ID_SUBSCRIPTION,
    PARTICIPANT_ID_BOT,
    PARTICIPANT_ID_NOTIFICATION,
  ];

  static final $core.List<ParticipantId?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 10);
  static ParticipantId? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ParticipantId._(super.value, super.name);
}

class LifecycleFenceState extends $pb.ProtobufEnum {
  static const LifecycleFenceState LIFECYCLE_FENCE_STATE_UNSPECIFIED =
      LifecycleFenceState._(
          0, _omitEnumNames ? '' : 'LIFECYCLE_FENCE_STATE_UNSPECIFIED');
  static const LifecycleFenceState LIFECYCLE_FENCE_STATE_FROZEN =
      LifecycleFenceState._(
          1, _omitEnumNames ? '' : 'LIFECYCLE_FENCE_STATE_FROZEN');
  static const LifecycleFenceState LIFECYCLE_FENCE_STATE_LIVE =
      LifecycleFenceState._(
          2, _omitEnumNames ? '' : 'LIFECYCLE_FENCE_STATE_LIVE');
  static const LifecycleFenceState LIFECYCLE_FENCE_STATE_PURGE_DECIDED =
      LifecycleFenceState._(
          3, _omitEnumNames ? '' : 'LIFECYCLE_FENCE_STATE_PURGE_DECIDED');

  static const $core.List<LifecycleFenceState> values = <LifecycleFenceState>[
    LIFECYCLE_FENCE_STATE_UNSPECIFIED,
    LIFECYCLE_FENCE_STATE_FROZEN,
    LIFECYCLE_FENCE_STATE_LIVE,
    LIFECYCLE_FENCE_STATE_PURGE_DECIDED,
  ];

  static final $core.List<LifecycleFenceState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static LifecycleFenceState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LifecycleFenceState._(super.value, super.name);
}

class PurgeReceiptState extends $pb.ProtobufEnum {
  static const PurgeReceiptState PURGE_RECEIPT_STATE_UNSPECIFIED =
      PurgeReceiptState._(
          0, _omitEnumNames ? '' : 'PURGE_RECEIPT_STATE_UNSPECIFIED');
  static const PurgeReceiptState PURGE_RECEIPT_STATE_COMPLETED =
      PurgeReceiptState._(
          1, _omitEnumNames ? '' : 'PURGE_RECEIPT_STATE_COMPLETED');

  static const $core.List<PurgeReceiptState> values = <PurgeReceiptState>[
    PURGE_RECEIPT_STATE_UNSPECIFIED,
    PURGE_RECEIPT_STATE_COMPLETED,
  ];

  static final $core.List<PurgeReceiptState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static PurgeReceiptState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const PurgeReceiptState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
