// This is a generated file - do not edit.
//
// Generated from voice/messaging/v1/messaging.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for Message.type (string). Prefer message_kind when set — docs/REPOSITORIES.md (Protobuf).
class MessageKind extends $pb.ProtobufEnum {
  static const MessageKind MESSAGE_KIND_UNSPECIFIED =
      MessageKind._(0, _omitEnumNames ? '' : 'MESSAGE_KIND_UNSPECIFIED');
  static const MessageKind MESSAGE_KIND_REGULAR =
      MessageKind._(1, _omitEnumNames ? '' : 'MESSAGE_KIND_REGULAR');
  static const MessageKind MESSAGE_KIND_SYSTEM =
      MessageKind._(2, _omitEnumNames ? '' : 'MESSAGE_KIND_SYSTEM');
  static const MessageKind MESSAGE_KIND_FORWARD =
      MessageKind._(3, _omitEnumNames ? '' : 'MESSAGE_KIND_FORWARD');

  static const $core.List<MessageKind> values = <MessageKind>[
    MESSAGE_KIND_UNSPECIFIED,
    MESSAGE_KIND_REGULAR,
    MESSAGE_KIND_SYSTEM,
    MESSAGE_KIND_FORWARD,
  ];

  static final $core.List<MessageKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static MessageKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const MessageKind._(super.value, super.name);
}

class DeleteScope extends $pb.ProtobufEnum {
  static const DeleteScope DELETE_SCOPE_UNSPECIFIED =
      DeleteScope._(0, _omitEnumNames ? '' : 'DELETE_SCOPE_UNSPECIFIED');
  static const DeleteScope DELETE_SCOPE_FOR_EVERYONE =
      DeleteScope._(1, _omitEnumNames ? '' : 'DELETE_SCOPE_FOR_EVERYONE');
  static const DeleteScope DELETE_SCOPE_FOR_ME =
      DeleteScope._(2, _omitEnumNames ? '' : 'DELETE_SCOPE_FOR_ME');

  static const $core.List<DeleteScope> values = <DeleteScope>[
    DELETE_SCOPE_UNSPECIFIED,
    DELETE_SCOPE_FOR_EVERYONE,
    DELETE_SCOPE_FOR_ME,
  ];

  static final $core.List<DeleteScope?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static DeleteScope? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const DeleteScope._(super.value, super.name);
}

/// Shared media tabs in chat info (docs/features/text-chat.md, docs/features/search.md).
class SharedMediaKind extends $pb.ProtobufEnum {
  static const SharedMediaKind SHARED_MEDIA_KIND_UNSPECIFIED =
      SharedMediaKind._(
          0, _omitEnumNames ? '' : 'SHARED_MEDIA_KIND_UNSPECIFIED');
  static const SharedMediaKind SHARED_MEDIA_KIND_MEDIA =
      SharedMediaKind._(1, _omitEnumNames ? '' : 'SHARED_MEDIA_KIND_MEDIA');
  static const SharedMediaKind SHARED_MEDIA_KIND_FILES =
      SharedMediaKind._(2, _omitEnumNames ? '' : 'SHARED_MEDIA_KIND_FILES');
  static const SharedMediaKind SHARED_MEDIA_KIND_LINKS =
      SharedMediaKind._(3, _omitEnumNames ? '' : 'SHARED_MEDIA_KIND_LINKS');
  static const SharedMediaKind SHARED_MEDIA_KIND_VOICE =
      SharedMediaKind._(4, _omitEnumNames ? '' : 'SHARED_MEDIA_KIND_VOICE');

  static const $core.List<SharedMediaKind> values = <SharedMediaKind>[
    SHARED_MEDIA_KIND_UNSPECIFIED,
    SHARED_MEDIA_KIND_MEDIA,
    SHARED_MEDIA_KIND_FILES,
    SHARED_MEDIA_KIND_LINKS,
    SHARED_MEDIA_KIND_VOICE,
  ];

  static final $core.List<SharedMediaKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static SharedMediaKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const SharedMediaKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
