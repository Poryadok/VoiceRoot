// This is a generated file - do not edit.
//
// Generated from voice/story/v1/lfp_consent.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:protobuf/protobuf.dart' as $pb;

class LfpConsentResponseType extends $pb.ProtobufEnum {
  static const LfpConsentResponseType LFP_CONSENT_RESPONSE_TYPE_UNSPECIFIED =
      LfpConsentResponseType._(
          0, _omitEnumNames ? '' : 'LFP_CONSENT_RESPONSE_TYPE_UNSPECIFIED');
  static const LfpConsentResponseType LFP_CONSENT_RESPONSE_TYPE_JOIN =
      LfpConsentResponseType._(
          1, _omitEnumNames ? '' : 'LFP_CONSENT_RESPONSE_TYPE_JOIN');
  static const LfpConsentResponseType LFP_CONSENT_RESPONSE_TYPE_INVITE =
      LfpConsentResponseType._(
          2, _omitEnumNames ? '' : 'LFP_CONSENT_RESPONSE_TYPE_INVITE');

  static const $core.List<LfpConsentResponseType> values =
      <LfpConsentResponseType>[
    LFP_CONSENT_RESPONSE_TYPE_UNSPECIFIED,
    LFP_CONSENT_RESPONSE_TYPE_JOIN,
    LFP_CONSENT_RESPONSE_TYPE_INVITE,
  ];

  static final $core.List<LfpConsentResponseType?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 2);
  static LfpConsentResponseType? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const LfpConsentResponseType._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');
