// This is a generated file - do not edit.
//
// Generated from voice/space/v1/space.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class SpaceDeletionPhase extends $pb.ProtobufEnum {
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_UNSPECIFIED =
      SpaceDeletionPhase._(
          0, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_UNSPECIFIED');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_LIVE =
      SpaceDeletionPhase._(
          1, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_LIVE');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_SCHEDULE_PENDING =
      SpaceDeletionPhase._(
          2, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_SCHEDULE_PENDING');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_FREEZE_PENDING =
      SpaceDeletionPhase._(
          3, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_FREEZE_PENDING');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_SCHEDULED =
      SpaceDeletionPhase._(
          4, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_SCHEDULED');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_RESTORE_DECIDED =
      SpaceDeletionPhase._(
          5, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_RESTORE_DECIDED');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_PURGE_DECIDED =
      SpaceDeletionPhase._(
          6, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_PURGE_DECIDED');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_PURGING =
      SpaceDeletionPhase._(
          7, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_PURGING');
  static const SpaceDeletionPhase SPACE_DELETION_PHASE_PURGED =
      SpaceDeletionPhase._(
          8, _omitEnumNames ? '' : 'SPACE_DELETION_PHASE_PURGED');

  static const $core.List<SpaceDeletionPhase> values = <SpaceDeletionPhase>[
    SPACE_DELETION_PHASE_UNSPECIFIED,
    SPACE_DELETION_PHASE_LIVE,
    SPACE_DELETION_PHASE_SCHEDULE_PENDING,
    SPACE_DELETION_PHASE_FREEZE_PENDING,
    SPACE_DELETION_PHASE_SCHEDULED,
    SPACE_DELETION_PHASE_RESTORE_DECIDED,
    SPACE_DELETION_PHASE_PURGE_DECIDED,
    SPACE_DELETION_PHASE_PURGING,
    SPACE_DELETION_PHASE_PURGED,
  ];

  static final $core.List<SpaceDeletionPhase?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 8);
  static SpaceDeletionPhase? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceDeletionPhase._(super.value, super.name);
}

class ParticipantOperationState extends $pb.ProtobufEnum {
  static const ParticipantOperationState
      PARTICIPANT_OPERATION_STATE_UNSPECIFIED = ParticipantOperationState._(
          0, _omitEnumNames ? '' : 'PARTICIPANT_OPERATION_STATE_UNSPECIFIED');
  static const ParticipantOperationState
      PARTICIPANT_OPERATION_STATE_NOT_STARTED = ParticipantOperationState._(
          1, _omitEnumNames ? '' : 'PARTICIPANT_OPERATION_STATE_NOT_STARTED');
  static const ParticipantOperationState PARTICIPANT_OPERATION_STATE_IN_FLIGHT =
      ParticipantOperationState._(
          2, _omitEnumNames ? '' : 'PARTICIPANT_OPERATION_STATE_IN_FLIGHT');
  static const ParticipantOperationState PARTICIPANT_OPERATION_STATE_COMPLETE =
      ParticipantOperationState._(
          3, _omitEnumNames ? '' : 'PARTICIPANT_OPERATION_STATE_COMPLETE');
  static const ParticipantOperationState
      PARTICIPANT_OPERATION_STATE_RETRYABLE_FAILURE =
      ParticipantOperationState._(
          4,
          _omitEnumNames
              ? ''
              : 'PARTICIPANT_OPERATION_STATE_RETRYABLE_FAILURE');
  static const ParticipantOperationState
      PARTICIPANT_OPERATION_STATE_CONTRACT_MISMATCH =
      ParticipantOperationState._(
          5,
          _omitEnumNames
              ? ''
              : 'PARTICIPANT_OPERATION_STATE_CONTRACT_MISMATCH');

  static const $core.List<ParticipantOperationState> values =
      <ParticipantOperationState>[
    PARTICIPANT_OPERATION_STATE_UNSPECIFIED,
    PARTICIPANT_OPERATION_STATE_NOT_STARTED,
    PARTICIPANT_OPERATION_STATE_IN_FLIGHT,
    PARTICIPANT_OPERATION_STATE_COMPLETE,
    PARTICIPANT_OPERATION_STATE_RETRYABLE_FAILURE,
    PARTICIPANT_OPERATION_STATE_CONTRACT_MISMATCH,
  ];

  static final $core.List<ParticipantOperationState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 5);
  static ParticipantOperationState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ParticipantOperationState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
