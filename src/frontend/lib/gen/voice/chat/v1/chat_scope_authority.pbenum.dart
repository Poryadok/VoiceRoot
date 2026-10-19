// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_scope_authority.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class SpaceReportAttachmentDisposition extends $pb.ProtobufEnum {
  static const SpaceReportAttachmentDisposition
      SPACE_REPORT_ATTACHMENT_DISPOSITION_UNSPECIFIED =
      SpaceReportAttachmentDisposition._(
          0,
          _omitEnumNames
              ? ''
              : 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNSPECIFIED');
  static const SpaceReportAttachmentDisposition
      SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN =
      SpaceReportAttachmentDisposition._(1,
          _omitEnumNames ? '' : 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN');
  static const SpaceReportAttachmentDisposition
      SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED =
      SpaceReportAttachmentDisposition._(
          2,
          _omitEnumNames
              ? ''
              : 'SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED');
  static const SpaceReportAttachmentDisposition
      SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED =
      SpaceReportAttachmentDisposition._(3,
          _omitEnumNames ? '' : 'SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED');

  static const $core.List<SpaceReportAttachmentDisposition> values =
      <SpaceReportAttachmentDisposition>[
    SPACE_REPORT_ATTACHMENT_DISPOSITION_UNSPECIFIED,
    SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN,
    SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED,
    SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED,
  ];

  static final $core.List<SpaceReportAttachmentDisposition?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static SpaceReportAttachmentDisposition? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceReportAttachmentDisposition._(super.value, super.name);
}

class SpaceReportAttachmentRetirementReason extends $pb.ProtobufEnum {
  static const SpaceReportAttachmentRetirementReason
      SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_UNSPECIFIED =
      SpaceReportAttachmentRetirementReason._(
          0,
          _omitEnumNames
              ? ''
              : 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_UNSPECIFIED');
  static const SpaceReportAttachmentRetirementReason
      SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED =
      SpaceReportAttachmentRetirementReason._(
          1,
          _omitEnumNames
              ? ''
              : 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED');
  static const SpaceReportAttachmentRetirementReason
      SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED =
      SpaceReportAttachmentRetirementReason._(
          2,
          _omitEnumNames
              ? ''
              : 'SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED');

  static const $core.List<SpaceReportAttachmentRetirementReason> values =
      <SpaceReportAttachmentRetirementReason>[
    SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_UNSPECIFIED,
    SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED,
    SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED,
  ];

  static final $core.List<SpaceReportAttachmentRetirementReason?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static SpaceReportAttachmentRetirementReason? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SpaceReportAttachmentRetirementReason._(super.value, super.name);
}

class ChatScopeManagedKind extends $pb.ProtobufEnum {
  static const ChatScopeManagedKind CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED =
      ChatScopeManagedKind._(
          0, _omitEnumNames ? '' : 'CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED');
  static const ChatScopeManagedKind CHAT_SCOPE_MANAGED_KIND_ORDINARY =
      ChatScopeManagedKind._(
          1, _omitEnumNames ? '' : 'CHAT_SCOPE_MANAGED_KIND_ORDINARY');
  static const ChatScopeManagedKind CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION =
      ChatScopeManagedKind._(
          2, _omitEnumNames ? '' : 'CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION');
  static const ChatScopeManagedKind CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD =
      ChatScopeManagedKind._(
          3, _omitEnumNames ? '' : 'CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD');

  static const $core.List<ChatScopeManagedKind> values = <ChatScopeManagedKind>[
    CHAT_SCOPE_MANAGED_KIND_UNSPECIFIED,
    CHAT_SCOPE_MANAGED_KIND_ORDINARY,
    CHAT_SCOPE_MANAGED_KIND_GAME_INTEGRATION,
    CHAT_SCOPE_MANAGED_KIND_MATCH_SQUAD,
  ];

  static final $core.List<ChatScopeManagedKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ChatScopeManagedKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ChatScopeManagedKind._(super.value, super.name);
}

class ChatScopeLifecycleState extends $pb.ProtobufEnum {
  static const ChatScopeLifecycleState CHAT_SCOPE_LIFECYCLE_STATE_UNSPECIFIED =
      ChatScopeLifecycleState._(
          0, _omitEnumNames ? '' : 'CHAT_SCOPE_LIFECYCLE_STATE_UNSPECIFIED');
  static const ChatScopeLifecycleState CHAT_SCOPE_LIFECYCLE_STATE_LIVE =
      ChatScopeLifecycleState._(
          1, _omitEnumNames ? '' : 'CHAT_SCOPE_LIFECYCLE_STATE_LIVE');
  static const ChatScopeLifecycleState CHAT_SCOPE_LIFECYCLE_STATE_FROZEN =
      ChatScopeLifecycleState._(
          2, _omitEnumNames ? '' : 'CHAT_SCOPE_LIFECYCLE_STATE_FROZEN');
  static const ChatScopeLifecycleState
      CHAT_SCOPE_LIFECYCLE_STATE_PURGE_DECIDED = ChatScopeLifecycleState._(
          3, _omitEnumNames ? '' : 'CHAT_SCOPE_LIFECYCLE_STATE_PURGE_DECIDED');
  static const ChatScopeLifecycleState CHAT_SCOPE_LIFECYCLE_STATE_PURGED =
      ChatScopeLifecycleState._(
          4, _omitEnumNames ? '' : 'CHAT_SCOPE_LIFECYCLE_STATE_PURGED');

  static const $core.List<ChatScopeLifecycleState> values =
      <ChatScopeLifecycleState>[
    CHAT_SCOPE_LIFECYCLE_STATE_UNSPECIFIED,
    CHAT_SCOPE_LIFECYCLE_STATE_LIVE,
    CHAT_SCOPE_LIFECYCLE_STATE_FROZEN,
    CHAT_SCOPE_LIFECYCLE_STATE_PURGE_DECIDED,
    CHAT_SCOPE_LIFECYCLE_STATE_PURGED,
  ];

  static final $core.List<ChatScopeLifecycleState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static ChatScopeLifecycleState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ChatScopeLifecycleState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
