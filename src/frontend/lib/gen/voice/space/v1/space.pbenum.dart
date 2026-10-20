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

/// Communication timeout is an additional deny, never membership or Role authority.
class MemberCommunicationState extends $pb.ProtobufEnum {
  static const MemberCommunicationState MEMBER_COMMUNICATION_STATE_UNSPECIFIED =
      MemberCommunicationState._(
          0, _omitEnumNames ? '' : 'MEMBER_COMMUNICATION_STATE_UNSPECIFIED');
  static const MemberCommunicationState MEMBER_COMMUNICATION_STATE_CLEAR =
      MemberCommunicationState._(
          1, _omitEnumNames ? '' : 'MEMBER_COMMUNICATION_STATE_CLEAR');
  static const MemberCommunicationState MEMBER_COMMUNICATION_STATE_TIMED_OUT =
      MemberCommunicationState._(
          2, _omitEnumNames ? '' : 'MEMBER_COMMUNICATION_STATE_TIMED_OUT');

  static const $core.List<MemberCommunicationState> values =
      <MemberCommunicationState>[
    MEMBER_COMMUNICATION_STATE_UNSPECIFIED,
    MEMBER_COMMUNICATION_STATE_CLEAR,
    MEMBER_COMMUNICATION_STATE_TIMED_OUT,
  ];

  static final $core.List<MemberCommunicationState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static MemberCommunicationState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MemberCommunicationState._(super.value, super.name);
}

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

class SpaceChatCreationOutcomeState extends $pb.ProtobufEnum {
  static const SpaceChatCreationOutcomeState
      SPACE_CHAT_CREATION_OUTCOME_STATE_UNSPECIFIED =
      SpaceChatCreationOutcomeState._(
          0,
          _omitEnumNames
              ? ''
              : 'SPACE_CHAT_CREATION_OUTCOME_STATE_UNSPECIFIED');
  static const SpaceChatCreationOutcomeState
      SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED =
      SpaceChatCreationOutcomeState._(1,
          _omitEnumNames ? '' : 'SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED');
  static const SpaceChatCreationOutcomeState
      SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED =
      SpaceChatCreationOutcomeState._(
          2, _omitEnumNames ? '' : 'SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED');

  static const $core.List<SpaceChatCreationOutcomeState> values =
      <SpaceChatCreationOutcomeState>[
    SPACE_CHAT_CREATION_OUTCOME_STATE_UNSPECIFIED,
    SPACE_CHAT_CREATION_OUTCOME_STATE_COMMITTED,
    SPACE_CHAT_CREATION_OUTCOME_STATE_ABORTED,
  ];

  static final $core.List<SpaceChatCreationOutcomeState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SpaceChatCreationOutcomeState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceChatCreationOutcomeState._(super.value, super.name);
}

class JoinAdmissionState extends $pb.ProtobufEnum {
  static const JoinAdmissionState JOIN_ADMISSION_STATE_UNSPECIFIED =
      JoinAdmissionState._(
          0, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_UNSPECIFIED');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_COLLECTING =
      JoinAdmissionState._(
          1, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_COLLECTING');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_PENDING =
      JoinAdmissionState._(
          2, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_PENDING');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_PROCESSING =
      JoinAdmissionState._(
          3, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_PROCESSING');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_ADMITTED =
      JoinAdmissionState._(
          4, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_ADMITTED');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_FAILED =
      JoinAdmissionState._(
          5, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_FAILED');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_EXPIRED =
      JoinAdmissionState._(
          6, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_EXPIRED');
  static const JoinAdmissionState JOIN_ADMISSION_STATE_INVALIDATED =
      JoinAdmissionState._(
          7, _omitEnumNames ? '' : 'JOIN_ADMISSION_STATE_INVALIDATED');

  static const $core.List<JoinAdmissionState> values = <JoinAdmissionState>[
    JOIN_ADMISSION_STATE_UNSPECIFIED,
    JOIN_ADMISSION_STATE_COLLECTING,
    JOIN_ADMISSION_STATE_PENDING,
    JOIN_ADMISSION_STATE_PROCESSING,
    JOIN_ADMISSION_STATE_ADMITTED,
    JOIN_ADMISSION_STATE_FAILED,
    JOIN_ADMISSION_STATE_EXPIRED,
    JOIN_ADMISSION_STATE_INVALIDATED,
  ];

  static final $core.List<JoinAdmissionState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 7);
  static JoinAdmissionState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const JoinAdmissionState._(super.value, super.name);
}

class BotGlobalTerminalOutcome extends $pb.ProtobufEnum {
  static const BotGlobalTerminalOutcome
      BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED = BotGlobalTerminalOutcome._(
          0, _omitEnumNames ? '' : 'BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED');
  static const BotGlobalTerminalOutcome
      BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED =
      BotGlobalTerminalOutcome._(
          1,
          _omitEnumNames
              ? ''
              : 'BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED');

  static const $core.List<BotGlobalTerminalOutcome> values =
      <BotGlobalTerminalOutcome>[
    BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED,
    BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED,
  ];

  static final $core.List<BotGlobalTerminalOutcome?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static BotGlobalTerminalOutcome? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const BotGlobalTerminalOutcome._(super.value, super.name);
}

class BotPurgeInstallationDisposition extends $pb.ProtobufEnum {
  static const BotPurgeInstallationDisposition
      BOT_PURGE_INSTALLATION_DISPOSITION_UNSPECIFIED =
      BotPurgeInstallationDisposition._(
          0,
          _omitEnumNames
              ? ''
              : 'BOT_PURGE_INSTALLATION_DISPOSITION_UNSPECIFIED');
  static const BotPurgeInstallationDisposition
      BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED =
      BotPurgeInstallationDisposition._(
          1,
          _omitEnumNames
              ? ''
              : 'BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED');
  static const BotPurgeInstallationDisposition
      BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED =
      BotPurgeInstallationDisposition._(
          2,
          _omitEnumNames
              ? ''
              : 'BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED');
  static const BotPurgeInstallationDisposition
      BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED =
      BotPurgeInstallationDisposition._(
          3,
          _omitEnumNames
              ? ''
              : 'BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED');

  static const $core.List<BotPurgeInstallationDisposition> values =
      <BotPurgeInstallationDisposition>[
    BOT_PURGE_INSTALLATION_DISPOSITION_UNSPECIFIED,
    BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED,
    BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED,
    BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED,
  ];

  static final $core.List<BotPurgeInstallationDisposition?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static BotPurgeInstallationDisposition? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const BotPurgeInstallationDisposition._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
