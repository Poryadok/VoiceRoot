// This is a generated file - do not edit.
//
// Generated from voice/story/v1/story.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

/// Canonical values for Story.type / CreateStoryRequest.type (strings).
class StoryMediaType extends $pb.ProtobufEnum {
  static const StoryMediaType STORY_MEDIA_TYPE_UNSPECIFIED =
      StoryMediaType._(0, _omitEnumNames ? '' : 'STORY_MEDIA_TYPE_UNSPECIFIED');
  static const StoryMediaType STORY_MEDIA_TYPE_PHOTO =
      StoryMediaType._(1, _omitEnumNames ? '' : 'STORY_MEDIA_TYPE_PHOTO');
  static const StoryMediaType STORY_MEDIA_TYPE_VIDEO =
      StoryMediaType._(2, _omitEnumNames ? '' : 'STORY_MEDIA_TYPE_VIDEO');
  static const StoryMediaType STORY_MEDIA_TYPE_TEXT =
      StoryMediaType._(3, _omitEnumNames ? '' : 'STORY_MEDIA_TYPE_TEXT');

  static const $core.List<StoryMediaType> values = <StoryMediaType>[
    STORY_MEDIA_TYPE_UNSPECIFIED,
    STORY_MEDIA_TYPE_PHOTO,
    STORY_MEDIA_TYPE_VIDEO,
    STORY_MEDIA_TYPE_TEXT,
  ];

  static final $core.List<StoryMediaType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 3);
  static StoryMediaType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoryMediaType._(super.value, super.name);
}

/// Canonical values for Story.visibility / CreateStoryRequest.visibility (strings).
class StoryAudience extends $pb.ProtobufEnum {
  static const StoryAudience STORY_AUDIENCE_UNSPECIFIED =
      StoryAudience._(0, _omitEnumNames ? '' : 'STORY_AUDIENCE_UNSPECIFIED');
  static const StoryAudience STORY_AUDIENCE_PUBLIC =
      StoryAudience._(1, _omitEnumNames ? '' : 'STORY_AUDIENCE_PUBLIC');
  static const StoryAudience STORY_AUDIENCE_FRIENDS =
      StoryAudience._(2, _omitEnumNames ? '' : 'STORY_AUDIENCE_FRIENDS');
  static const StoryAudience STORY_AUDIENCE_CLOSE_FRIENDS =
      StoryAudience._(3, _omitEnumNames ? '' : 'STORY_AUDIENCE_CLOSE_FRIENDS');
  static const StoryAudience STORY_AUDIENCE_CUSTOM =
      StoryAudience._(4, _omitEnumNames ? '' : 'STORY_AUDIENCE_CUSTOM');

  static const $core.List<StoryAudience> values = <StoryAudience>[
    STORY_AUDIENCE_UNSPECIFIED,
    STORY_AUDIENCE_PUBLIC,
    STORY_AUDIENCE_FRIENDS,
    STORY_AUDIENCE_CLOSE_FRIENDS,
    STORY_AUDIENCE_CUSTOM,
  ];

  static final $core.List<StoryAudience?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static StoryAudience? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const StoryAudience._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
