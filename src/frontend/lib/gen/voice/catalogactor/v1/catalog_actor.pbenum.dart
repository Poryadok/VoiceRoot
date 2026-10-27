// This is a generated file - do not edit.
//
// Generated from voice/catalogactor/v1/catalog_actor.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class CatalogActorState extends $pb.ProtobufEnum {
  static const CatalogActorState CATALOG_ACTOR_STATE_UNSPECIFIED =
      CatalogActorState._(
          0, _omitEnumNames ? '' : 'CATALOG_ACTOR_STATE_UNSPECIFIED');
  static const CatalogActorState CATALOG_ACTOR_STATE_UNKNOWN =
      CatalogActorState._(
          1, _omitEnumNames ? '' : 'CATALOG_ACTOR_STATE_UNKNOWN');
  static const CatalogActorState CATALOG_ACTOR_STATE_NOT_DELETED =
      CatalogActorState._(
          2, _omitEnumNames ? '' : 'CATALOG_ACTOR_STATE_NOT_DELETED');
  static const CatalogActorState CATALOG_ACTOR_STATE_SOFT_DELETED =
      CatalogActorState._(
          3, _omitEnumNames ? '' : 'CATALOG_ACTOR_STATE_SOFT_DELETED');

  static const $core.List<CatalogActorState> values = <CatalogActorState>[
    CATALOG_ACTOR_STATE_UNSPECIFIED,
    CATALOG_ACTOR_STATE_UNKNOWN,
    CATALOG_ACTOR_STATE_NOT_DELETED,
    CATALOG_ACTOR_STATE_SOFT_DELETED,
  ];

  static final $core.List<CatalogActorState?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static CatalogActorState? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const CatalogActorState._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
