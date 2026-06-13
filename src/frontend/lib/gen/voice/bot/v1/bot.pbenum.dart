// This is a generated file - do not edit.
//
// Generated from voice/bot/v1/bot.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for Bot.status (string).
class BotLifecycleStatus extends $pb.ProtobufEnum {
  static const BotLifecycleStatus BOT_LIFECYCLE_STATUS_UNSPECIFIED =
      BotLifecycleStatus._(
          0, _omitEnumNames ? '' : 'BOT_LIFECYCLE_STATUS_UNSPECIFIED');
  static const BotLifecycleStatus BOT_LIFECYCLE_STATUS_DRAFT =
      BotLifecycleStatus._(
          1, _omitEnumNames ? '' : 'BOT_LIFECYCLE_STATUS_DRAFT');
  static const BotLifecycleStatus BOT_LIFECYCLE_STATUS_LIVE =
      BotLifecycleStatus._(
          2, _omitEnumNames ? '' : 'BOT_LIFECYCLE_STATUS_LIVE');
  static const BotLifecycleStatus BOT_LIFECYCLE_STATUS_DISABLED =
      BotLifecycleStatus._(
          3, _omitEnumNames ? '' : 'BOT_LIFECYCLE_STATUS_DISABLED');

  static const $core.List<BotLifecycleStatus> values = <BotLifecycleStatus>[
    BOT_LIFECYCLE_STATUS_UNSPECIFIED,
    BOT_LIFECYCLE_STATUS_DRAFT,
    BOT_LIFECYCLE_STATUS_LIVE,
    BOT_LIFECYCLE_STATUS_DISABLED,
  ];

  static final $core.List<BotLifecycleStatus?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static BotLifecycleStatus? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const BotLifecycleStatus._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
