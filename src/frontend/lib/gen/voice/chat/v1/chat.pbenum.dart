// This is a generated file - do not edit.
//
// Generated from voice/chat/v1/chat.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class ChatType extends $pb.ProtobufEnum {
  static const ChatType CHAT_TYPE_UNSPECIFIED =
      ChatType._(0, _omitEnumNames ? '' : 'CHAT_TYPE_UNSPECIFIED');
  static const ChatType CHAT_TYPE_DM =
      ChatType._(1, _omitEnumNames ? '' : 'CHAT_TYPE_DM');
  static const ChatType CHAT_TYPE_GROUP =
      ChatType._(2, _omitEnumNames ? '' : 'CHAT_TYPE_GROUP');
  static const ChatType CHAT_TYPE_CHANNEL =
      ChatType._(3, _omitEnumNames ? '' : 'CHAT_TYPE_CHANNEL');

  static const $core.List<ChatType> values = <ChatType>[
    CHAT_TYPE_UNSPECIFIED,
    CHAT_TYPE_DM,
    CHAT_TYPE_GROUP,
    CHAT_TYPE_CHANNEL,
  ];

  static final $core.List<ChatType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static ChatType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const ChatType._(super.value, super.name);
}

class MatchSquadTeardownStatus extends $pb.ProtobufEnum {
  static const MatchSquadTeardownStatus
      MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED = MatchSquadTeardownStatus._(
          0, _omitEnumNames ? '' : 'MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED');
  static const MatchSquadTeardownStatus MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED =
      MatchSquadTeardownStatus._(
          1, _omitEnumNames ? '' : 'MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED');

  static const $core.List<MatchSquadTeardownStatus> values =
      <MatchSquadTeardownStatus>[
    MATCH_SQUAD_TEARDOWN_STATUS_UNSPECIFIED,
    MATCH_SQUAD_TEARDOWN_STATUS_COMPLETED,
  ];

  static final $core.List<MatchSquadTeardownStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static MatchSquadTeardownStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MatchSquadTeardownStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
