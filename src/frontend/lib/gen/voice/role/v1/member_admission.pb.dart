// This is a generated file - do not edit.
//
// Generated from voice/role/v1/member_admission.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

enum MembershipBinding_Source { ordinary, community, notSet }

/// Exact Space-owned eligibility incarnation. A selector is not authority: Role
/// must verify it against a fresh protected Space observation before use.
/// BOTH is deliberately absent; each contribution selects exactly one source.
class MembershipBinding extends $pb.GeneratedMessage {
  factory MembershipBinding({
    OrdinaryMembershipBinding? ordinary,
    CommunityMembershipBinding? community,
  }) {
    final result = create();
    if (ordinary != null) result.ordinary = ordinary;
    if (community != null) result.community = community;
    return result;
  }

  MembershipBinding._();

  factory MembershipBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory MembershipBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, MembershipBinding_Source>
      _MembershipBinding_SourceByTag = {
    1: MembershipBinding_Source.ordinary,
    2: MembershipBinding_Source.community,
    0: MembershipBinding_Source.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'MembershipBinding',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..oo(0, [1, 2])
    ..aOM<OrdinaryMembershipBinding>(1, _omitFieldNames ? '' : 'ordinary',
        subBuilder: OrdinaryMembershipBinding.create)
    ..aOM<CommunityMembershipBinding>(2, _omitFieldNames ? '' : 'community',
        subBuilder: CommunityMembershipBinding.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MembershipBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  MembershipBinding copyWith(void Function(MembershipBinding) updates) =>
      super.copyWith((message) => updates(message as MembershipBinding))
          as MembershipBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static MembershipBinding create() => MembershipBinding._();
  @$core.override
  MembershipBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static MembershipBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<MembershipBinding>(create);
  static MembershipBinding? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  MembershipBinding_Source whichSource() =>
      _MembershipBinding_SourceByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  void clearSource() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  OrdinaryMembershipBinding get ordinary => $_getN(0);
  @$pb.TagNumber(1)
  set ordinary(OrdinaryMembershipBinding value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasOrdinary() => $_has(0);
  @$pb.TagNumber(1)
  void clearOrdinary() => $_clearField(1);
  @$pb.TagNumber(1)
  OrdinaryMembershipBinding ensureOrdinary() => $_ensure(0);

  @$pb.TagNumber(2)
  CommunityMembershipBinding get community => $_getN(1);
  @$pb.TagNumber(2)
  set community(CommunityMembershipBinding value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasCommunity() => $_has(1);
  @$pb.TagNumber(2)
  void clearCommunity() => $_clearField(2);
  @$pb.TagNumber(2)
  CommunityMembershipBinding ensureCommunity() => $_ensure(1);
}

class OrdinaryMembershipBinding extends $pb.GeneratedMessage {
  factory OrdinaryMembershipBinding({
    $fixnum.Int64? membershipGeneration,
    $core.String? publicationIdentity,
    $core.bool? legacy,
  }) {
    final result = create();
    if (membershipGeneration != null)
      result.membershipGeneration = membershipGeneration;
    if (publicationIdentity != null)
      result.publicationIdentity = publicationIdentity;
    if (legacy != null) result.legacy = legacy;
    return result;
  }

  OrdinaryMembershipBinding._();

  factory OrdinaryMembershipBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory OrdinaryMembershipBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'OrdinaryMembershipBinding',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'membershipGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(2, _omitFieldNames ? '' : 'publicationIdentity')
    ..aOB(3, _omitFieldNames ? '' : 'legacy')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OrdinaryMembershipBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  OrdinaryMembershipBinding copyWith(
          void Function(OrdinaryMembershipBinding) updates) =>
      super.copyWith((message) => updates(message as OrdinaryMembershipBinding))
          as OrdinaryMembershipBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static OrdinaryMembershipBinding create() => OrdinaryMembershipBinding._();
  @$core.override
  OrdinaryMembershipBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static OrdinaryMembershipBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<OrdinaryMembershipBinding>(create);
  static OrdinaryMembershipBinding? _defaultInstance;

  /// Positive int64-compatible admission generation, or zero ONLY for an
  /// explicitly tagged legacy publication issued and persisted by Space.
  @$pb.TagNumber(1)
  $fixnum.Int64 get membershipGeneration => $_getI64(0);
  @$pb.TagNumber(1)
  set membershipGeneration($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasMembershipGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearMembershipGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get publicationIdentity => $_getSZ(1);
  @$pb.TagNumber(2)
  set publicationIdentity($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasPublicationIdentity() => $_has(1);
  @$pb.TagNumber(2)
  void clearPublicationIdentity() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.bool get legacy => $_getBF(2);
  @$pb.TagNumber(3)
  set legacy($core.bool value) => $_setBool(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLegacy() => $_has(2);
  @$pb.TagNumber(3)
  void clearLegacy() => $_clearField(3);
}

class CommunityMembershipBinding extends $pb.GeneratedMessage {
  factory CommunityMembershipBinding({
    $fixnum.Int64? ownerGeneration,
    $fixnum.Int64? membershipGeneration,
    $core.String? membershipEpisodeId,
  }) {
    final result = create();
    if (ownerGeneration != null) result.ownerGeneration = ownerGeneration;
    if (membershipGeneration != null)
      result.membershipGeneration = membershipGeneration;
    if (membershipEpisodeId != null)
      result.membershipEpisodeId = membershipEpisodeId;
    return result;
  }

  CommunityMembershipBinding._();

  factory CommunityMembershipBinding.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CommunityMembershipBinding.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CommunityMembershipBinding',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.role.v1'),
      createEmptyInstance: create)
    ..a<$fixnum.Int64>(
        1, _omitFieldNames ? '' : 'ownerGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'membershipGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'membershipEpisodeId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommunityMembershipBinding clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CommunityMembershipBinding copyWith(
          void Function(CommunityMembershipBinding) updates) =>
      super.copyWith(
              (message) => updates(message as CommunityMembershipBinding))
          as CommunityMembershipBinding;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CommunityMembershipBinding create() => CommunityMembershipBinding._();
  @$core.override
  CommunityMembershipBinding createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CommunityMembershipBinding getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CommunityMembershipBinding>(create);
  static CommunityMembershipBinding? _defaultInstance;

  @$pb.TagNumber(1)
  $fixnum.Int64 get ownerGeneration => $_getI64(0);
  @$pb.TagNumber(1)
  set ownerGeneration($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOwnerGeneration() => $_has(0);
  @$pb.TagNumber(1)
  void clearOwnerGeneration() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get membershipGeneration => $_getI64(1);
  @$pb.TagNumber(2)
  set membershipGeneration($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasMembershipGeneration() => $_has(1);
  @$pb.TagNumber(2)
  void clearMembershipGeneration() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get membershipEpisodeId => $_getSZ(2);
  @$pb.TagNumber(3)
  set membershipEpisodeId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasMembershipEpisodeId() => $_has(2);
  @$pb.TagNumber(3)
  void clearMembershipEpisodeId() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
