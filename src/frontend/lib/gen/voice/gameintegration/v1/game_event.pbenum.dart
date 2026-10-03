// This is a generated file - do not edit.
//
// Generated from voice/gameintegration/v1/game_event.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class GameEventPublicationStatus extends $pb.ProtobufEnum {
  static const GameEventPublicationStatus
      GAME_EVENT_PUBLICATION_STATUS_UNSPECIFIED = GameEventPublicationStatus._(
          0, _omitEnumNames ? '' : 'GAME_EVENT_PUBLICATION_STATUS_UNSPECIFIED');
  static const GameEventPublicationStatus
      GAME_EVENT_PUBLICATION_STATUS_PUBLISHED = GameEventPublicationStatus._(
          1, _omitEnumNames ? '' : 'GAME_EVENT_PUBLICATION_STATUS_PUBLISHED');
  static const GameEventPublicationStatus
      GAME_EVENT_PUBLICATION_STATUS_EXPIRED = GameEventPublicationStatus._(
          2, _omitEnumNames ? '' : 'GAME_EVENT_PUBLICATION_STATUS_EXPIRED');

  static const $core.List<GameEventPublicationStatus> values =
      <GameEventPublicationStatus>[
    GAME_EVENT_PUBLICATION_STATUS_UNSPECIFIED,
    GAME_EVENT_PUBLICATION_STATUS_PUBLISHED,
    GAME_EVENT_PUBLICATION_STATUS_EXPIRED,
  ];

  static final $core.List<GameEventPublicationStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static GameEventPublicationStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const GameEventPublicationStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
