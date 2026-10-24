// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat_lifecycle.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Chat native terminal state is distinct from the coordinator's fence commands.
class ChatLifecycleState extends $pb.ProtobufEnum {
  static const ChatLifecycleState CHAT_LIFECYCLE_STATE_UNSPECIFIED =
      ChatLifecycleState._(
          0, _omitEnumNames ? '' : 'CHAT_LIFECYCLE_STATE_UNSPECIFIED');
  static const ChatLifecycleState CHAT_LIFECYCLE_STATE_LIVE =
      ChatLifecycleState._(
          1, _omitEnumNames ? '' : 'CHAT_LIFECYCLE_STATE_LIVE');
  static const ChatLifecycleState CHAT_LIFECYCLE_STATE_FROZEN =
      ChatLifecycleState._(
          2, _omitEnumNames ? '' : 'CHAT_LIFECYCLE_STATE_FROZEN');
  static const ChatLifecycleState CHAT_LIFECYCLE_STATE_PURGE_DECIDED =
      ChatLifecycleState._(
          3, _omitEnumNames ? '' : 'CHAT_LIFECYCLE_STATE_PURGE_DECIDED');
  static const ChatLifecycleState CHAT_LIFECYCLE_STATE_PURGED =
      ChatLifecycleState._(
          4, _omitEnumNames ? '' : 'CHAT_LIFECYCLE_STATE_PURGED');

  static const $core.List<ChatLifecycleState> values = <ChatLifecycleState>[
    CHAT_LIFECYCLE_STATE_UNSPECIFIED,
    CHAT_LIFECYCLE_STATE_LIVE,
    CHAT_LIFECYCLE_STATE_FROZEN,
    CHAT_LIFECYCLE_STATE_PURGE_DECIDED,
    CHAT_LIFECYCLE_STATE_PURGED,
  ];

  static final $core.List<ChatLifecycleState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static ChatLifecycleState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ChatLifecycleState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
