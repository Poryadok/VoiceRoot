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

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
