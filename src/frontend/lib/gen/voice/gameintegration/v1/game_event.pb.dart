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

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart'
    as $0;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'game_event.pbenum.dart';

class BindingEventRecipient extends $pb.GeneratedMessage {
  factory BindingEventRecipient({
    $core.String? bindingId,
    $core.String? resolvedChatId,
  }) {
    final result = create();
    if (bindingId != null) result.bindingId = bindingId;
    if (resolvedChatId != null) result.resolvedChatId = resolvedChatId;
    return result;
  }

  BindingEventRecipient._();

  factory BindingEventRecipient.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BindingEventRecipient.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BindingEventRecipient',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'bindingId')
    ..aOS(2, _omitFieldNames ? '' : 'resolvedChatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BindingEventRecipient clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BindingEventRecipient copyWith(
          void Function(BindingEventRecipient) updates) =>
      super.copyWith((message) => updates(message as BindingEventRecipient))
          as BindingEventRecipient;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BindingEventRecipient create() => BindingEventRecipient._();
  @$core.override
  BindingEventRecipient createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BindingEventRecipient getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BindingEventRecipient>(create);
  static BindingEventRecipient? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get bindingId => $_getSZ(0);
  @$pb.TagNumber(1)
  set bindingId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasBindingId() => $_has(0);
  @$pb.TagNumber(1)
  void clearBindingId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get resolvedChatId => $_getSZ(1);
  @$pb.TagNumber(2)
  set resolvedChatId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasResolvedChatId() => $_has(1);
  @$pb.TagNumber(2)
  void clearResolvedChatId() => $_clearField(2);
}

class DirectChatEventRecipient extends $pb.GeneratedMessage {
  factory DirectChatEventRecipient({
    $core.String? chatId,
  }) {
    final result = create();
    if (chatId != null) result.chatId = chatId;
    return result;
  }

  DirectChatEventRecipient._();

  factory DirectChatEventRecipient.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory DirectChatEventRecipient.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'DirectChatEventRecipient',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'chatId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DirectChatEventRecipient clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  DirectChatEventRecipient copyWith(
          void Function(DirectChatEventRecipient) updates) =>
      super.copyWith((message) => updates(message as DirectChatEventRecipient))
          as DirectChatEventRecipient;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static DirectChatEventRecipient create() => DirectChatEventRecipient._();
  @$core.override
  DirectChatEventRecipient createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static DirectChatEventRecipient getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<DirectChatEventRecipient>(create);
  static DirectChatEventRecipient? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get chatId => $_getSZ(0);
  @$pb.TagNumber(1)
  set chatId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasChatId() => $_has(0);
  @$pb.TagNumber(1)
  void clearChatId() => $_clearField(1);
}

enum VerifiedGameEventIntent_Recipient { binding, directChat, notSet }

class VerifiedGameEventIntent extends $pb.GeneratedMessage {
  factory VerifiedGameEventIntent({
    $core.String? operationId,
    $core.String? appId,
    $core.String? appOwnerAccountId,
    $core.String? environmentId,
    $core.String? installationId,
    $core.String? botId,
    $core.String? eventId,
    $core.String? payloadHash,
    BindingEventRecipient? binding,
    DirectChatEventRecipient? directChat,
    $fixnum.Int64? authorityRevision,
    $core.int? schemaVersion,
    $0.Timestamp? occurredAt,
    $0.Timestamp? expiresAt,
    $core.String? characterBindingId,
    $core.String? stateVersion,
    $core.String? fallbackText,
    $core.String? clientMessageId,
    $core.String? eventType,
    GameCard? card,
  }) {
    final result = create();
    if (operationId != null) result.operationId = operationId;
    if (appId != null) result.appId = appId;
    if (appOwnerAccountId != null) result.appOwnerAccountId = appOwnerAccountId;
    if (environmentId != null) result.environmentId = environmentId;
    if (installationId != null) result.installationId = installationId;
    if (botId != null) result.botId = botId;
    if (eventId != null) result.eventId = eventId;
    if (payloadHash != null) result.payloadHash = payloadHash;
    if (binding != null) result.binding = binding;
    if (directChat != null) result.directChat = directChat;
    if (authorityRevision != null) result.authorityRevision = authorityRevision;
    if (schemaVersion != null) result.schemaVersion = schemaVersion;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (expiresAt != null) result.expiresAt = expiresAt;
    if (characterBindingId != null)
      result.characterBindingId = characterBindingId;
    if (stateVersion != null) result.stateVersion = stateVersion;
    if (fallbackText != null) result.fallbackText = fallbackText;
    if (clientMessageId != null) result.clientMessageId = clientMessageId;
    if (eventType != null) result.eventType = eventType;
    if (card != null) result.card = card;
    return result;
  }

  VerifiedGameEventIntent._();

  factory VerifiedGameEventIntent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory VerifiedGameEventIntent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, VerifiedGameEventIntent_Recipient>
      _VerifiedGameEventIntent_RecipientByTag = {
    9: VerifiedGameEventIntent_Recipient.binding,
    10: VerifiedGameEventIntent_Recipient.directChat,
    0: VerifiedGameEventIntent_Recipient.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'VerifiedGameEventIntent',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..oo(0, [9, 10])
    ..aOS(1, _omitFieldNames ? '' : 'operationId')
    ..aOS(2, _omitFieldNames ? '' : 'appId')
    ..aOS(3, _omitFieldNames ? '' : 'appOwnerAccountId')
    ..aOS(4, _omitFieldNames ? '' : 'environmentId')
    ..aOS(5, _omitFieldNames ? '' : 'installationId')
    ..aOS(6, _omitFieldNames ? '' : 'botId')
    ..aOS(7, _omitFieldNames ? '' : 'eventId')
    ..aOS(8, _omitFieldNames ? '' : 'payloadHash')
    ..aOM<BindingEventRecipient>(9, _omitFieldNames ? '' : 'binding',
        subBuilder: BindingEventRecipient.create)
    ..aOM<DirectChatEventRecipient>(10, _omitFieldNames ? '' : 'directChat',
        subBuilder: DirectChatEventRecipient.create)
    ..a<$fixnum.Int64>(
        11, _omitFieldNames ? '' : 'authorityRevision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aI(12, _omitFieldNames ? '' : 'schemaVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOM<$0.Timestamp>(13, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $0.Timestamp.create)
    ..aOM<$0.Timestamp>(14, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $0.Timestamp.create)
    ..aOS(15, _omitFieldNames ? '' : 'characterBindingId')
    ..aOS(16, _omitFieldNames ? '' : 'stateVersion')
    ..aOS(17, _omitFieldNames ? '' : 'fallbackText')
    ..aOS(18, _omitFieldNames ? '' : 'clientMessageId')
    ..aOS(19, _omitFieldNames ? '' : 'eventType')
    ..aOM<GameCard>(20, _omitFieldNames ? '' : 'card',
        subBuilder: GameCard.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifiedGameEventIntent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  VerifiedGameEventIntent copyWith(
          void Function(VerifiedGameEventIntent) updates) =>
      super.copyWith((message) => updates(message as VerifiedGameEventIntent))
          as VerifiedGameEventIntent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static VerifiedGameEventIntent create() => VerifiedGameEventIntent._();
  @$core.override
  VerifiedGameEventIntent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static VerifiedGameEventIntent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<VerifiedGameEventIntent>(create);
  static VerifiedGameEventIntent? _defaultInstance;

  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  VerifiedGameEventIntent_Recipient whichRecipient() =>
      _VerifiedGameEventIntent_RecipientByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(9)
  @$pb.TagNumber(10)
  void clearRecipient() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  $core.String get operationId => $_getSZ(0);
  @$pb.TagNumber(1)
  set operationId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasOperationId() => $_has(0);
  @$pb.TagNumber(1)
  void clearOperationId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get appId => $_getSZ(1);
  @$pb.TagNumber(2)
  set appId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAppId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAppId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get appOwnerAccountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set appOwnerAccountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasAppOwnerAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearAppOwnerAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get environmentId => $_getSZ(3);
  @$pb.TagNumber(4)
  set environmentId($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasEnvironmentId() => $_has(3);
  @$pb.TagNumber(4)
  void clearEnvironmentId() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get installationId => $_getSZ(4);
  @$pb.TagNumber(5)
  set installationId($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasInstallationId() => $_has(4);
  @$pb.TagNumber(5)
  void clearInstallationId() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get botId => $_getSZ(5);
  @$pb.TagNumber(6)
  set botId($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasBotId() => $_has(5);
  @$pb.TagNumber(6)
  void clearBotId() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get eventId => $_getSZ(6);
  @$pb.TagNumber(7)
  set eventId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasEventId() => $_has(6);
  @$pb.TagNumber(7)
  void clearEventId() => $_clearField(7);

  @$pb.TagNumber(8)
  $core.String get payloadHash => $_getSZ(7);
  @$pb.TagNumber(8)
  set payloadHash($core.String value) => $_setString(7, value);
  @$pb.TagNumber(8)
  $core.bool hasPayloadHash() => $_has(7);
  @$pb.TagNumber(8)
  void clearPayloadHash() => $_clearField(8);

  @$pb.TagNumber(9)
  BindingEventRecipient get binding => $_getN(8);
  @$pb.TagNumber(9)
  set binding(BindingEventRecipient value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasBinding() => $_has(8);
  @$pb.TagNumber(9)
  void clearBinding() => $_clearField(9);
  @$pb.TagNumber(9)
  BindingEventRecipient ensureBinding() => $_ensure(8);

  @$pb.TagNumber(10)
  DirectChatEventRecipient get directChat => $_getN(9);
  @$pb.TagNumber(10)
  set directChat(DirectChatEventRecipient value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasDirectChat() => $_has(9);
  @$pb.TagNumber(10)
  void clearDirectChat() => $_clearField(10);
  @$pb.TagNumber(10)
  DirectChatEventRecipient ensureDirectChat() => $_ensure(9);

  @$pb.TagNumber(11)
  $fixnum.Int64 get authorityRevision => $_getI64(10);
  @$pb.TagNumber(11)
  set authorityRevision($fixnum.Int64 value) => $_setInt64(10, value);
  @$pb.TagNumber(11)
  $core.bool hasAuthorityRevision() => $_has(10);
  @$pb.TagNumber(11)
  void clearAuthorityRevision() => $_clearField(11);

  @$pb.TagNumber(12)
  $core.int get schemaVersion => $_getIZ(11);
  @$pb.TagNumber(12)
  set schemaVersion($core.int value) => $_setUnsignedInt32(11, value);
  @$pb.TagNumber(12)
  $core.bool hasSchemaVersion() => $_has(11);
  @$pb.TagNumber(12)
  void clearSchemaVersion() => $_clearField(12);

  @$pb.TagNumber(13)
  $0.Timestamp get occurredAt => $_getN(12);
  @$pb.TagNumber(13)
  set occurredAt($0.Timestamp value) => $_setField(13, value);
  @$pb.TagNumber(13)
  $core.bool hasOccurredAt() => $_has(12);
  @$pb.TagNumber(13)
  void clearOccurredAt() => $_clearField(13);
  @$pb.TagNumber(13)
  $0.Timestamp ensureOccurredAt() => $_ensure(12);

  @$pb.TagNumber(14)
  $0.Timestamp get expiresAt => $_getN(13);
  @$pb.TagNumber(14)
  set expiresAt($0.Timestamp value) => $_setField(14, value);
  @$pb.TagNumber(14)
  $core.bool hasExpiresAt() => $_has(13);
  @$pb.TagNumber(14)
  void clearExpiresAt() => $_clearField(14);
  @$pb.TagNumber(14)
  $0.Timestamp ensureExpiresAt() => $_ensure(13);

  @$pb.TagNumber(15)
  $core.String get characterBindingId => $_getSZ(14);
  @$pb.TagNumber(15)
  set characterBindingId($core.String value) => $_setString(14, value);
  @$pb.TagNumber(15)
  $core.bool hasCharacterBindingId() => $_has(14);
  @$pb.TagNumber(15)
  void clearCharacterBindingId() => $_clearField(15);

  @$pb.TagNumber(16)
  $core.String get stateVersion => $_getSZ(15);
  @$pb.TagNumber(16)
  set stateVersion($core.String value) => $_setString(15, value);
  @$pb.TagNumber(16)
  $core.bool hasStateVersion() => $_has(15);
  @$pb.TagNumber(16)
  void clearStateVersion() => $_clearField(16);

  @$pb.TagNumber(17)
  $core.String get fallbackText => $_getSZ(16);
  @$pb.TagNumber(17)
  set fallbackText($core.String value) => $_setString(16, value);
  @$pb.TagNumber(17)
  $core.bool hasFallbackText() => $_has(16);
  @$pb.TagNumber(17)
  void clearFallbackText() => $_clearField(17);

  @$pb.TagNumber(18)
  $core.String get clientMessageId => $_getSZ(17);
  @$pb.TagNumber(18)
  set clientMessageId($core.String value) => $_setString(17, value);
  @$pb.TagNumber(18)
  $core.bool hasClientMessageId() => $_has(17);
  @$pb.TagNumber(18)
  void clearClientMessageId() => $_clearField(18);

  @$pb.TagNumber(19)
  $core.String get eventType => $_getSZ(18);
  @$pb.TagNumber(19)
  set eventType($core.String value) => $_setString(18, value);
  @$pb.TagNumber(19)
  $core.bool hasEventType() => $_has(18);
  @$pb.TagNumber(19)
  void clearEventType() => $_clearField(19);

  @$pb.TagNumber(20)
  GameCard get card => $_getN(19);
  @$pb.TagNumber(20)
  set card(GameCard value) => $_setField(20, value);
  @$pb.TagNumber(20)
  $core.bool hasCard() => $_has(19);
  @$pb.TagNumber(20)
  void clearCard() => $_clearField(20);
  @$pb.TagNumber(20)
  GameCard ensureCard() => $_ensure(19);
}

/// Declarative card payload. Messaging stores this immutable data beside the
/// message; clients do not execute or authorize actions from these fields.
class GameCard extends $pb.GeneratedMessage {
  factory GameCard({
    $core.int? schemaVersion,
    $fixnum.Int64? revision,
    $core.String? title,
    $core.String? safeSummary,
    $core.Iterable<GameCardFact>? facts,
    $core.Iterable<GameCardAction>? actions,
    $core.Iterable<$core.String>? mediaReferenceIds,
  }) {
    final result = create();
    if (schemaVersion != null) result.schemaVersion = schemaVersion;
    if (revision != null) result.revision = revision;
    if (title != null) result.title = title;
    if (safeSummary != null) result.safeSummary = safeSummary;
    if (facts != null) result.facts.addAll(facts);
    if (actions != null) result.actions.addAll(actions);
    if (mediaReferenceIds != null)
      result.mediaReferenceIds.addAll(mediaReferenceIds);
    return result;
  }

  GameCard._();

  factory GameCard.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GameCard.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GameCard',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..aI(1, _omitFieldNames ? '' : 'schemaVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..a<$fixnum.Int64>(
        2, _omitFieldNames ? '' : 'revision', $pb.PbFieldType.OU6,
        defaultOrMaker: $fixnum.Int64.ZERO)
    ..aOS(3, _omitFieldNames ? '' : 'title')
    ..aOS(4, _omitFieldNames ? '' : 'safeSummary')
    ..pPM<GameCardFact>(5, _omitFieldNames ? '' : 'facts',
        subBuilder: GameCardFact.create)
    ..pPM<GameCardAction>(6, _omitFieldNames ? '' : 'actions',
        subBuilder: GameCardAction.create)
    ..pPS(7, _omitFieldNames ? '' : 'mediaReferenceIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCard clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCard copyWith(void Function(GameCard) updates) =>
      super.copyWith((message) => updates(message as GameCard)) as GameCard;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GameCard create() => GameCard._();
  @$core.override
  GameCard createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GameCard getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<GameCard>(create);
  static GameCard? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get schemaVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set schemaVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSchemaVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearSchemaVersion() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get revision => $_getI64(1);
  @$pb.TagNumber(2)
  set revision($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRevision() => $_has(1);
  @$pb.TagNumber(2)
  void clearRevision() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get title => $_getSZ(2);
  @$pb.TagNumber(3)
  set title($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasTitle() => $_has(2);
  @$pb.TagNumber(3)
  void clearTitle() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get safeSummary => $_getSZ(3);
  @$pb.TagNumber(4)
  set safeSummary($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasSafeSummary() => $_has(3);
  @$pb.TagNumber(4)
  void clearSafeSummary() => $_clearField(4);

  @$pb.TagNumber(5)
  $pb.PbList<GameCardFact> get facts => $_getList(4);

  @$pb.TagNumber(6)
  $pb.PbList<GameCardAction> get actions => $_getList(5);

  @$pb.TagNumber(7)
  $pb.PbList<$core.String> get mediaReferenceIds => $_getList(6);
}

class GameCardFact extends $pb.GeneratedMessage {
  factory GameCardFact({
    $core.String? label,
    $core.String? value,
  }) {
    final result = create();
    if (label != null) result.label = label;
    if (value != null) result.value = value;
    return result;
  }

  GameCardFact._();

  factory GameCardFact.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GameCardFact.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GameCardFact',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'label')
    ..aOS(2, _omitFieldNames ? '' : 'value')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCardFact clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCardFact copyWith(void Function(GameCardFact) updates) =>
      super.copyWith((message) => updates(message as GameCardFact))
          as GameCardFact;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GameCardFact create() => GameCardFact._();
  @$core.override
  GameCardFact createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GameCardFact getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GameCardFact>(create);
  static GameCardFact? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get label => $_getSZ(0);
  @$pb.TagNumber(1)
  set label($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLabel() => $_has(0);
  @$pb.TagNumber(1)
  void clearLabel() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get value => $_getSZ(1);
  @$pb.TagNumber(2)
  set value($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasValue() => $_has(1);
  @$pb.TagNumber(2)
  void clearValue() => $_clearField(2);
}

class GameCardAction extends $pb.GeneratedMessage {
  factory GameCardAction({
    $core.String? actionId,
    $core.String? actionType,
    $core.String? label,
    $core.String? argumentsJson,
    $core.String? stateVersion,
    $0.Timestamp? expiresAt,
  }) {
    final result = create();
    if (actionId != null) result.actionId = actionId;
    if (actionType != null) result.actionType = actionType;
    if (label != null) result.label = label;
    if (argumentsJson != null) result.argumentsJson = argumentsJson;
    if (stateVersion != null) result.stateVersion = stateVersion;
    if (expiresAt != null) result.expiresAt = expiresAt;
    return result;
  }

  GameCardAction._();

  factory GameCardAction.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GameCardAction.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GameCardAction',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.gameintegration.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'actionId')
    ..aOS(2, _omitFieldNames ? '' : 'actionType')
    ..aOS(3, _omitFieldNames ? '' : 'label')
    ..aOS(4, _omitFieldNames ? '' : 'argumentsJson')
    ..aOS(5, _omitFieldNames ? '' : 'stateVersion')
    ..aOM<$0.Timestamp>(6, _omitFieldNames ? '' : 'expiresAt',
        subBuilder: $0.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCardAction clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GameCardAction copyWith(void Function(GameCardAction) updates) =>
      super.copyWith((message) => updates(message as GameCardAction))
          as GameCardAction;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GameCardAction create() => GameCardAction._();
  @$core.override
  GameCardAction createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GameCardAction getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GameCardAction>(create);
  static GameCardAction? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get actionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set actionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasActionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearActionId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get actionType => $_getSZ(1);
  @$pb.TagNumber(2)
  set actionType($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasActionType() => $_has(1);
  @$pb.TagNumber(2)
  void clearActionType() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get label => $_getSZ(2);
  @$pb.TagNumber(3)
  set label($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasLabel() => $_has(2);
  @$pb.TagNumber(3)
  void clearLabel() => $_clearField(3);

  /// Strict canonical JSON object, never a command or executable callback.
  @$pb.TagNumber(4)
  $core.String get argumentsJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set argumentsJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasArgumentsJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearArgumentsJson() => $_clearField(4);

  /// Signed game state precondition; executable T53 admission requires a value.
  @$pb.TagNumber(5)
  $core.String get stateVersion => $_getSZ(4);
  @$pb.TagNumber(5)
  set stateVersion($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStateVersion() => $_has(4);
  @$pb.TagNumber(5)
  void clearStateVersion() => $_clearField(5);

  /// Signed absolute expiry; old or unbounded actions fail closed at invoke.
  @$pb.TagNumber(6)
  $0.Timestamp get expiresAt => $_getN(5);
  @$pb.TagNumber(6)
  set expiresAt($0.Timestamp value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasExpiresAt() => $_has(5);
  @$pb.TagNumber(6)
  void clearExpiresAt() => $_clearField(6);
  @$pb.TagNumber(6)
  $0.Timestamp ensureExpiresAt() => $_ensure(5);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
