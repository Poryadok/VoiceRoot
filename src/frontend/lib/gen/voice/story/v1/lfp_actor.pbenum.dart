// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_actor.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class LfpActorActionKind extends $pb.ProtobufEnum {
  static const LfpActorActionKind LFP_ACTOR_ACTION_KIND_UNSPECIFIED =
      LfpActorActionKind._(
          0, _omitEnumNames ? '' : 'LFP_ACTOR_ACTION_KIND_UNSPECIFIED');
  static const LfpActorActionKind LFP_ACTOR_ACTION_KIND_PUBLISH_OFFER =
      LfpActorActionKind._(
          1, _omitEnumNames ? '' : 'LFP_ACTOR_ACTION_KIND_PUBLISH_OFFER');
  static const LfpActorActionKind LFP_ACTOR_ACTION_KIND_RESPOND =
      LfpActorActionKind._(
          2, _omitEnumNames ? '' : 'LFP_ACTOR_ACTION_KIND_RESPOND');
  static const LfpActorActionKind LFP_ACTOR_ACTION_KIND_ACCEPT =
      LfpActorActionKind._(
          3, _omitEnumNames ? '' : 'LFP_ACTOR_ACTION_KIND_ACCEPT');
  static const LfpActorActionKind LFP_ACTOR_ACTION_KIND_DECLINE =
      LfpActorActionKind._(
          4, _omitEnumNames ? '' : 'LFP_ACTOR_ACTION_KIND_DECLINE');

  static const $core.List<LfpActorActionKind> values = <LfpActorActionKind>[
    LFP_ACTOR_ACTION_KIND_UNSPECIFIED,
    LFP_ACTOR_ACTION_KIND_PUBLISH_OFFER,
    LFP_ACTOR_ACTION_KIND_RESPOND,
    LFP_ACTOR_ACTION_KIND_ACCEPT,
    LFP_ACTOR_ACTION_KIND_DECLINE,
  ];

  static final $core.List<LfpActorActionKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static LfpActorActionKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LfpActorActionKind._(super.value, super.name);
}

class LfpActorKind extends $pb.ProtobufEnum {
  static const LfpActorKind LFP_ACTOR_KIND_UNSPECIFIED =
      LfpActorKind._(0, _omitEnumNames ? '' : 'LFP_ACTOR_KIND_UNSPECIFIED');
  static const LfpActorKind LFP_ACTOR_KIND_HUMAN_DELEGATED_USER =
      LfpActorKind._(
          1, _omitEnumNames ? '' : 'LFP_ACTOR_KIND_HUMAN_DELEGATED_USER');

  static const $core.List<LfpActorKind> values = <LfpActorKind>[
    LFP_ACTOR_KIND_UNSPECIFIED,
    LFP_ACTOR_KIND_HUMAN_DELEGATED_USER,
  ];

  static final $core.List<LfpActorKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 1);
  static LfpActorKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LfpActorKind._(super.value, super.name);
}

class LfpActorAccountType extends $pb.ProtobufEnum {
  static const LfpActorAccountType LFP_ACTOR_ACCOUNT_TYPE_UNSPECIFIED =
      LfpActorAccountType._(
          0, _omitEnumNames ? '' : 'LFP_ACTOR_ACCOUNT_TYPE_UNSPECIFIED');
  static const LfpActorAccountType LFP_ACTOR_ACCOUNT_TYPE_REGULAR =
      LfpActorAccountType._(
          1, _omitEnumNames ? '' : 'LFP_ACTOR_ACCOUNT_TYPE_REGULAR');
  static const LfpActorAccountType LFP_ACTOR_ACCOUNT_TYPE_GUEST =
      LfpActorAccountType._(
          2, _omitEnumNames ? '' : 'LFP_ACTOR_ACCOUNT_TYPE_GUEST');

  static const $core.List<LfpActorAccountType> values = <LfpActorAccountType>[
    LFP_ACTOR_ACCOUNT_TYPE_UNSPECIFIED,
    LFP_ACTOR_ACCOUNT_TYPE_REGULAR,
    LFP_ACTOR_ACCOUNT_TYPE_GUEST,
  ];

  static final $core.List<LfpActorAccountType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static LfpActorAccountType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LfpActorAccountType._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
