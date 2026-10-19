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

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'chat.pbenum.dart' as $1;
import 'chat_scope_authority.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'chat_scope_authority.pbenum.dart';

class GetSpaceChatScopeAuthorityRequest extends $pb.GeneratedMessage {
  factory GetSpaceChatScopeAuthorityRequest({
    $core.int? protocolVersion,
    $core.String? chatId,
    $core.String? expectedSpaceId,
    $core.String? operationId,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (chatId != null) result.chatId = chatId;
    if (expectedSpaceId != null) result.expectedSpaceId = expectedSpaceId;
    if (operationId != null) result.operationId = operationId;
    return result;
  }

  GetSpaceChatScopeAuthorityRequest._();

  factory GetSpaceChatScopeAuthorityRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceChatScopeAuthorityRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceChatScopeAuthorityRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'expectedSpaceId')
    ..aOS(4, _omitFieldNames ? '' : 'operationId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceChatScopeAuthorityRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceChatScopeAuthorityRequest copyWith(
          void Function(GetSpaceChatScopeAuthorityRequest) updates) =>
      super.copyWith((message) =>
              updates(message as GetSpaceChatScopeAuthorityRequest))
          as GetSpaceChatScopeAuthorityRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceChatScopeAuthorityRequest create() =>
      GetSpaceChatScopeAuthorityRequest._();
  @$core.override
  GetSpaceChatScopeAuthorityRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceChatScopeAuthorityRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceChatScopeAuthorityRequest>(
          create);
  static GetSpaceChatScopeAuthorityRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get expectedSpaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set expectedSpaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedSpaceId() => $_clearField(3);

  /// Fresh transport request identity; retries use fresh signed JWT/JTI.
  @$pb.TagNumber(4)
  $core.String get operationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set operationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperationId() => $_clearField(4);
}

class GetSpaceChatScopeAuthorityResponse extends $pb.GeneratedMessage {
  factory GetSpaceChatScopeAuthorityResponse({
    $core.int? protocolVersion,
    $core.String? chatId,
    $core.String? expectedSpaceId,
    $core.String? operationId,
    $core.bool? exists,
    $core.bool? live,
    $1.ChatType? chatType,
    $core.String? actualSpaceId,
    $core.String? chatScopeIncarnation,
    $fixnum.Int64? chatAuthorityRevision,
    $fixnum.Int64? lifecycleGeneration,
    ChatScopeLifecycleState? lifecycleState,
    ChatScopeManagedKind? managedKind,
    $fixnum.Int64? reportAttachmentGeneration,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (chatId != null) result.chatId = chatId;
    if (expectedSpaceId != null) result.expectedSpaceId = expectedSpaceId;
    if (operationId != null) result.operationId = operationId;
    if (exists != null) result.exists = exists;
    if (live != null) result.live = live;
    if (chatType != null) result.chatType = chatType;
    if (actualSpaceId != null) result.actualSpaceId = actualSpaceId;
    if (chatScopeIncarnation != null)
      result.chatScopeIncarnation = chatScopeIncarnation;
    if (chatAuthorityRevision != null)
      result.chatAuthorityRevision = chatAuthorityRevision;
    if (lifecycleGeneration != null)
      result.lifecycleGeneration = lifecycleGeneration;
    if (lifecycleState != null) result.lifecycleState = lifecycleState;
    if (managedKind != null) result.managedKind = managedKind;
    if (reportAttachmentGeneration != null)
      result.reportAttachmentGeneration = reportAttachmentGeneration;
    return result;
  }

  GetSpaceChatScopeAuthorityResponse._();

  factory GetSpaceChatScopeAuthorityResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceChatScopeAuthorityResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceChatScopeAuthorityResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'chatId')
    ..aOS(3, _omitFieldNames ? '' : 'expectedSpaceId')
    ..aOS(4, _omitFieldNames ? '' : 'operationId')
    ..aOB(5, _omitFieldNames ? '' : 'exists')
    ..aOB(6, _omitFieldNames ? '' : 'live')
    ..aE<$1.ChatType>(7, _omitFieldNames ? '' : 'chatType',
        enumValues: $1.ChatType.values)
    ..aOS(8, _omitFieldNames ? '' : 'actualSpaceId')
    ..aOS(9, _omitFieldNames ? '' : 'chatScopeIncarnation')
    ..a<$fixnum.Int64>(
        10, _omitFieldNames ? '' : 'chatAuthorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..a<$fixnum.Int64>(
        11, _omitFieldNames ? '' : 'lifecycleGeneration', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<ChatScopeLifecycleState>(12, _omitFieldNames ? '' : 'lifecycleState',
        enumValues: ChatScopeLifecycleState.values)
    ..aE<ChatScopeManagedKind>(13, _omitFieldNames ? '' : 'managedKind',
        enumValues: ChatScopeManagedKind.values)
    ..a<$fixnum.Int64>(14, _omitFieldNames ? '' : 'reportAttachmentGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceChatScopeAuthorityResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceChatScopeAuthorityResponse copyWith(
          void Function(GetSpaceChatScopeAuthorityResponse) updates) =>
      super.copyWith((message) =>
              updates(message as GetSpaceChatScopeAuthorityResponse))
          as GetSpaceChatScopeAuthorityResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceChatScopeAuthorityResponse create() =>
      GetSpaceChatScopeAuthorityResponse._();
  @$core.override
  GetSpaceChatScopeAuthorityResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceChatScopeAuthorityResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceChatScopeAuthorityResponse>(
          create);
  static GetSpaceChatScopeAuthorityResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get chatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set chatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearChatId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get expectedSpaceId => $_getSZ(2);
  @$pb.TagNumber(3)
  set expectedSpaceId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasExpectedSpaceId() => $_has(2);
  @$pb.TagNumber(3)
  void clearExpectedSpaceId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get operationId => $_getSZ(3);
  @$pb.TagNumber(4)
  set operationId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasOperationId() => $_has(3);
  @$pb.TagNumber(4)
  void clearOperationId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.bool get exists => $_getBF(4);
  @$pb.TagNumber(5)
  set exists($core.bool value) => $_setBool(4, value);
  @$pb.TagNumber(5)
  $core.bool hasExists() => $_has(4);
  @$pb.TagNumber(5)
  void clearExists() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.bool get live => $_getBF(5);
  @$pb.TagNumber(6)
  set live($core.bool value) => $_setBool(5, value);
  @$pb.TagNumber(6)
  $core.bool hasLive() => $_has(5);
  @$pb.TagNumber(6)
  void clearLive() => $_clearField(6);

  @$pb.TagNumber(7)
  $1.ChatType get chatType => $_getN(6);
  @$pb.TagNumber(7)
  set chatType($1.ChatType value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasChatType() => $_has(6);
  @$pb.TagNumber(7)
  void clearChatType() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get actualSpaceId => $_getSZ(7);
  @$pb.TagNumber(8)
  set actualSpaceId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasActualSpaceId() => $_has(7);
  @$pb.TagNumber(8)
  void clearActualSpaceId() => $_clearField(8);

  @$pb.TagNumber(9)
  $core.String get chatScopeIncarnation => $_getSZ(8);
  @$pb.TagNumber(9)
  set chatScopeIncarnation($core.String value) => $_setString(8, value);
  @$pb.TagNumber(9)
  $core.bool hasChatScopeIncarnation() => $_has(8);
  @$pb.TagNumber(9)
  void clearChatScopeIncarnation() => $_clearField(9);

  @$pb.TagNumber(10)
  $fixnum.Int64 get chatAuthorityRevision => $_getI64(9);
  @$pb.TagNumber(10)
  set chatAuthorityRevision($fixnum.Int64 value) => $_setInt64(9, value);
  @$pb.TagNumber(10)
  $core.bool hasChatAuthorityRevision() => $_has(9);
  @$pb.TagNumber(10)
  void clearChatAuthorityRevision() => $_clearField(10);

  /// Independent Chat-owned lifecycle fence, never a membership/Role epoch.
  @$pb.TagNumber(11)
  $fixnum.Int64 get lifecycleGeneration => $_getI64(10);
  @$pb.TagNumber(11)
  set lifecycleGeneration($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasLifecycleGeneration() => $_has(10);
  @$pb.TagNumber(11)
  void clearLifecycleGeneration() => $_clearField(11);

  @$pb.TagNumber(12)
  ChatScopeLifecycleState get lifecycleState => $_getN(11);
  @$pb.TagNumber(12)
  set lifecycleState(ChatScopeLifecycleState value) => $_setField(12, value);
  @$pb.TagNumber(12)
  $core.bool hasLifecycleState() => $_has(11);
  @$pb.TagNumber(12)
  void clearLifecycleState() => $_clearField(12);

  @$pb.TagNumber(13)
  ChatScopeManagedKind get managedKind => $_getN(12);
  @$pb.TagNumber(13)
  set managedKind(ChatScopeManagedKind value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasManagedKind() => $_has(12);
  @$pb.TagNumber(13)
  void clearManagedKind() => $_clearField(13);

  /// Separate native Space attachment epoch. Type/creator changes do not retire it.
  @$pb.TagNumber(14)
  $fixnum.Int64 get reportAttachmentGeneration => $_getI64(13);
  @$pb.TagNumber(14)
  set reportAttachmentGeneration($fixnum.Int64 value) => $_setInt64(13, value);
  @$pb.TagNumber(14)
  $core.bool hasReportAttachmentGeneration() => $_has(13);
  @$pb.TagNumber(14)
  void clearReportAttachmentGeneration() => $_clearField(14);
}

class ReadSpaceReportAttachmentRequest extends $pb.GeneratedMessage {
  factory ReadSpaceReportAttachmentRequest({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? chatId,
    $core.String? expectedSpaceId,
    $fixnum.Int64? savedAttachmentGeneration,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (chatId != null) result.chatId = chatId;
    if (expectedSpaceId != null) result.expectedSpaceId = expectedSpaceId;
    if (savedAttachmentGeneration != null)
      result.savedAttachmentGeneration = savedAttachmentGeneration;
    return result;
  }

  ReadSpaceReportAttachmentRequest._();

  factory ReadSpaceReportAttachmentRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadSpaceReportAttachmentRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadSpaceReportAttachmentRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'expectedSpaceId')
    ..a<$fixnum.Int64>(5, _omitFieldNames ? '' : 'savedAttachmentGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSpaceReportAttachmentRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSpaceReportAttachmentRequest copyWith(
          void Function(ReadSpaceReportAttachmentRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ReadSpaceReportAttachmentRequest))
          as ReadSpaceReportAttachmentRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadSpaceReportAttachmentRequest create() =>
      ReadSpaceReportAttachmentRequest._();
  @$core.override
  ReadSpaceReportAttachmentRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadSpaceReportAttachmentRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadSpaceReportAttachmentRequest>(
          create);
  static ReadSpaceReportAttachmentRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get expectedSpaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedSpaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedSpaceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get savedAttachmentGeneration => $_getI64(4);
  @$pb.TagNumber(5)
  set savedAttachmentGeneration($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSavedAttachmentGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearSavedAttachmentGeneration() => $_clearField(5);
}

class ReadSpaceReportAttachmentResponse extends $pb.GeneratedMessage {
  factory ReadSpaceReportAttachmentResponse({
    $core.int? protocolVersion,
    $core.String? operationId,
    $core.String? chatId,
    $core.String? expectedSpaceId,
    $fixnum.Int64? savedAttachmentGeneration,
    SpaceReportAttachmentDisposition? disposition,
    SpaceReportAttachmentRetirementReason? reason,
    $core.String? ownerEventId,
    $fixnum.Int64? ownerRevision,
  }) {
    final result = create();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (operationId != null) result.operationId = operationId;
    if (chatId != null) result.chatId = chatId;
    if (expectedSpaceId != null) result.expectedSpaceId = expectedSpaceId;
    if (savedAttachmentGeneration != null)
      result.savedAttachmentGeneration = savedAttachmentGeneration;
    if (disposition != null) result.disposition = disposition;
    if (reason != null) result.reason = reason;
    if (ownerEventId != null) result.ownerEventId = ownerEventId;
    if (ownerRevision != null) result.ownerRevision = ownerRevision;
    return result;
  }

  ReadSpaceReportAttachmentResponse._();

  factory ReadSpaceReportAttachmentResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ReadSpaceReportAttachmentResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ReadSpaceReportAttachmentResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'voice.chat.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'operationId')
    ..aOS(3, _omitFieldNames ? '' : 'chatId')
    ..aOS(4, _omitFieldNames ? '' : 'expectedSpaceId')
    ..a<$fixnum.Int64>(5, _omitFieldNames ? '' : 'savedAttachmentGeneration',
        $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aE<SpaceReportAttachmentDisposition>(
        6, _omitFieldNames ? '' : 'disposition',
        enumValues: SpaceReportAttachmentDisposition.values)
    ..aE<SpaceReportAttachmentRetirementReason>(
        7, _omitFieldNames ? '' : 'reason',
        enumValues: SpaceReportAttachmentRetirementReason.values)
    ..aOS(8, _omitFieldNames ? '' : 'ownerEventId')
    ..a<$fixnum.Int64>(
        9, _omitFieldNames ? '' : 'ownerRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSpaceReportAttachmentResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ReadSpaceReportAttachmentResponse copyWith(
          void Function(ReadSpaceReportAttachmentResponse) updates) =>
      super.copyWith((message) =>
              updates(message as ReadSpaceReportAttachmentResponse))
          as ReadSpaceReportAttachmentResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ReadSpaceReportAttachmentResponse create() =>
      ReadSpaceReportAttachmentResponse._();
  @$core.override
  ReadSpaceReportAttachmentResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ReadSpaceReportAttachmentResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ReadSpaceReportAttachmentResponse>(
          create);
  static ReadSpaceReportAttachmentResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get operationId => $_getSZ(1);
  @$pb.TagNumber(2)
  set operationId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasOperationId() => $_has(1);
  @$pb.TagNumber(2)
  void clearOperationId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get chatId => $_getSZ(2);
  @$pb.TagNumber(3)
  set chatId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasChatId() => $_has(2);
  @$pb.TagNumber(3)
  void clearChatId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get expectedSpaceId => $_getSZ(3);
  @$pb.TagNumber(4)
  set expectedSpaceId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasExpectedSpaceId() => $_has(3);
  @$pb.TagNumber(4)
  void clearExpectedSpaceId() => $_clearField(4);

  @$pb.TagNumber(5)
  $fixnum.Int64 get savedAttachmentGeneration => $_getI64(4);
  @$pb.TagNumber(5)
  set savedAttachmentGeneration($fixnum.Int64 value) => $_setInt64(4, value);
  @$pb.TagNumber(5)
  $core.bool hasSavedAttachmentGeneration() => $_has(4);
  @$pb.TagNumber(5)
  void clearSavedAttachmentGeneration() => $_clearField(5);

  @$pb.TagNumber(6)
  SpaceReportAttachmentDisposition get disposition => $_getN(5);
  @$pb.TagNumber(6)
  set disposition(SpaceReportAttachmentDisposition value) =>
      $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasDisposition() => $_has(5);
  @$pb.TagNumber(6)
  void clearDisposition() => $_clearField(6);

  /// Present only for RETIRED. Never contains the destination Space or a viewer.
  @$pb.TagNumber(7)
  SpaceReportAttachmentRetirementReason get reason => $_getN(6);
  @$pb.TagNumber(7)
  set reason(SpaceReportAttachmentRetirementReason value) =>
      $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasReason() => $_has(6);
  @$pb.TagNumber(7)
  void clearReason() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get ownerEventId => $_getSZ(7);
  @$pb.TagNumber(8)
  set ownerEventId($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasOwnerEventId() => $_has(7);
  @$pb.TagNumber(8)
  void clearOwnerEventId() => $_clearField(8);

  @$pb.TagNumber(9)
  $fixnum.Int64 get ownerRevision => $_getI64(8);
  @$pb.TagNumber(9)
  set ownerRevision($fixnum.Int64 value) => $_setInt64(8, value);
  @$pb.TagNumber(9)
  $core.bool hasOwnerRevision() => $_has(8);
  @$pb.TagNumber(9)
  void clearOwnerRevision() => $_clearField(9);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
