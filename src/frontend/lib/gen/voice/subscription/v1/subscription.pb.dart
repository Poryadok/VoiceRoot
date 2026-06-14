// This is a generated file - do not edit.
//
// Generated from voice/subscription/v1/subscription.proto.

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
    as $1;

import '../../common/v1/common.pb.dart' as $3;
import '../../space/v1/space.pb.dart' as $2;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class Subscription extends $pb.GeneratedMessage {
  factory Subscription({
    $core.String? id,
    $core.String? accountId,
    $core.String? plan,
    $core.String? billingPeriod,
    $core.String? status,
    $core.String? provider,
    $core.String? providerSubscriptionId,
    $1.Timestamp? currentPeriodStart,
    $1.Timestamp? currentPeriodEnd,
    $1.Timestamp? gracePeriodEnd,
    $1.Timestamp? cancelledAt,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (accountId != null) result.accountId = accountId;
    if (plan != null) result.plan = plan;
    if (billingPeriod != null) result.billingPeriod = billingPeriod;
    if (status != null) result.status = status;
    if (provider != null) result.provider = provider;
    if (providerSubscriptionId != null)
      result.providerSubscriptionId = providerSubscriptionId;
    if (currentPeriodStart != null)
      result.currentPeriodStart = currentPeriodStart;
    if (currentPeriodEnd != null) result.currentPeriodEnd = currentPeriodEnd;
    if (gracePeriodEnd != null) result.gracePeriodEnd = gracePeriodEnd;
    if (cancelledAt != null) result.cancelledAt = cancelledAt;
    return result;
  }

  Subscription._();

  factory Subscription.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Subscription.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Subscription',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'accountId')
    ..aOS(3, _omitFieldNames ? '' : 'plan')
    ..aOS(4, _omitFieldNames ? '' : 'billingPeriod')
    ..aOS(5, _omitFieldNames ? '' : 'status')
    ..aOS(6, _omitFieldNames ? '' : 'provider')
    ..aOS(7, _omitFieldNames ? '' : 'providerSubscriptionId')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'currentPeriodStart',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'currentPeriodEnd',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(10, _omitFieldNames ? '' : 'gracePeriodEnd',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(11, _omitFieldNames ? '' : 'cancelledAt',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Subscription clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Subscription copyWith(void Function(Subscription) updates) =>
      super.copyWith((message) => updates(message as Subscription))
          as Subscription;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Subscription create() => Subscription._();
  @$core.override
  Subscription createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Subscription getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Subscription>(create);
  static Subscription? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get accountId => $_getSZ(1);
  @$pb.TagNumber(2)
  set accountId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasAccountId() => $_has(1);
  @$pb.TagNumber(2)
  void clearAccountId() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get plan => $_getSZ(2);
  @$pb.TagNumber(3)
  set plan($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPlan() => $_has(2);
  @$pb.TagNumber(3)
  void clearPlan() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get billingPeriod => $_getSZ(3);
  @$pb.TagNumber(4)
  set billingPeriod($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasBillingPeriod() => $_has(3);
  @$pb.TagNumber(4)
  void clearBillingPeriod() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get status => $_getSZ(4);
  @$pb.TagNumber(5)
  set status($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasStatus() => $_has(4);
  @$pb.TagNumber(5)
  void clearStatus() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get provider => $_getSZ(5);
  @$pb.TagNumber(6)
  set provider($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasProvider() => $_has(5);
  @$pb.TagNumber(6)
  void clearProvider() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get providerSubscriptionId => $_getSZ(6);
  @$pb.TagNumber(7)
  set providerSubscriptionId($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasProviderSubscriptionId() => $_has(6);
  @$pb.TagNumber(7)
  void clearProviderSubscriptionId() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get currentPeriodStart => $_getN(7);
  @$pb.TagNumber(8)
  set currentPeriodStart($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCurrentPeriodStart() => $_has(7);
  @$pb.TagNumber(8)
  void clearCurrentPeriodStart() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureCurrentPeriodStart() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get currentPeriodEnd => $_getN(8);
  @$pb.TagNumber(9)
  set currentPeriodEnd($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCurrentPeriodEnd() => $_has(8);
  @$pb.TagNumber(9)
  void clearCurrentPeriodEnd() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureCurrentPeriodEnd() => $_ensure(8);

  @$pb.TagNumber(10)
  $1.Timestamp get gracePeriodEnd => $_getN(9);
  @$pb.TagNumber(10)
  set gracePeriodEnd($1.Timestamp value) => $_setField(10, value);
  @$pb.TagNumber(10)
  $core.bool hasGracePeriodEnd() => $_has(9);
  @$pb.TagNumber(10)
  void clearGracePeriodEnd() => $_clearField(10);
  @$pb.TagNumber(10)
  $1.Timestamp ensureGracePeriodEnd() => $_ensure(9);

  @$pb.TagNumber(11)
  $1.Timestamp get cancelledAt => $_getN(10);
  @$pb.TagNumber(11)
  set cancelledAt($1.Timestamp value) => $_setField(11, value);
  @$pb.TagNumber(11)
  $core.bool hasCancelledAt() => $_has(10);
  @$pb.TagNumber(11)
  void clearCancelledAt() => $_clearField(11);
  @$pb.TagNumber(11)
  $1.Timestamp ensureCancelledAt() => $_ensure(10);
}

class GetSubscriptionRequest extends $pb.GeneratedMessage {
  factory GetSubscriptionRequest({
    $core.String? accountId,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    return result;
  }

  GetSubscriptionRequest._();

  factory GetSubscriptionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSubscriptionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSubscriptionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSubscriptionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSubscriptionRequest copyWith(
          void Function(GetSubscriptionRequest) updates) =>
      super.copyWith((message) => updates(message as GetSubscriptionRequest))
          as GetSubscriptionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSubscriptionRequest create() => GetSubscriptionRequest._();
  @$core.override
  GetSubscriptionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSubscriptionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSubscriptionRequest>(create);
  static GetSubscriptionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);
}

class CreateCheckoutSessionRequest extends $pb.GeneratedMessage {
  factory CreateCheckoutSessionRequest({
    $core.String? plan,
    $core.String? billingPeriod,
    $core.String? successUrl,
    $core.String? cancelUrl,
  }) {
    final result = create();
    if (plan != null) result.plan = plan;
    if (billingPeriod != null) result.billingPeriod = billingPeriod;
    if (successUrl != null) result.successUrl = successUrl;
    if (cancelUrl != null) result.cancelUrl = cancelUrl;
    return result;
  }

  CreateCheckoutSessionRequest._();

  factory CreateCheckoutSessionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateCheckoutSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCheckoutSessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'plan')
    ..aOS(2, _omitFieldNames ? '' : 'billingPeriod')
    ..aOS(3, _omitFieldNames ? '' : 'successUrl')
    ..aOS(4, _omitFieldNames ? '' : 'cancelUrl')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCheckoutSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCheckoutSessionRequest copyWith(
          void Function(CreateCheckoutSessionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as CreateCheckoutSessionRequest))
          as CreateCheckoutSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateCheckoutSessionRequest create() =>
      CreateCheckoutSessionRequest._();
  @$core.override
  CreateCheckoutSessionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateCheckoutSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCheckoutSessionRequest>(create);
  static CreateCheckoutSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get plan => $_getSZ(0);
  @$pb.TagNumber(1)
  set plan($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasPlan() => $_has(0);
  @$pb.TagNumber(1)
  void clearPlan() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get billingPeriod => $_getSZ(1);
  @$pb.TagNumber(2)
  set billingPeriod($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasBillingPeriod() => $_has(1);
  @$pb.TagNumber(2)
  void clearBillingPeriod() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get successUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set successUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasSuccessUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearSuccessUrl() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get cancelUrl => $_getSZ(3);
  @$pb.TagNumber(4)
  set cancelUrl($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasCancelUrl() => $_has(3);
  @$pb.TagNumber(4)
  void clearCancelUrl() => $_clearField(4);
}

class CheckoutResponse extends $pb.GeneratedMessage {
  factory CheckoutResponse({
    $core.String? checkoutUrl,
    $core.String? sessionId,
  }) {
    final result = create();
    if (checkoutUrl != null) result.checkoutUrl = checkoutUrl;
    if (sessionId != null) result.sessionId = sessionId;
    return result;
  }

  CheckoutResponse._();

  factory CheckoutResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckoutResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckoutResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'checkoutUrl')
    ..aOS(2, _omitFieldNames ? '' : 'sessionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckoutResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckoutResponse copyWith(void Function(CheckoutResponse) updates) =>
      super.copyWith((message) => updates(message as CheckoutResponse))
          as CheckoutResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckoutResponse create() => CheckoutResponse._();
  @$core.override
  CheckoutResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckoutResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckoutResponse>(create);
  static CheckoutResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get checkoutUrl => $_getSZ(0);
  @$pb.TagNumber(1)
  set checkoutUrl($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasCheckoutUrl() => $_has(0);
  @$pb.TagNumber(1)
  void clearCheckoutUrl() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get sessionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set sessionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSessionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearSessionId() => $_clearField(2);
}

class CancelSubscriptionRequest extends $pb.GeneratedMessage {
  factory CancelSubscriptionRequest({
    $core.String? subscriptionId,
  }) {
    final result = create();
    if (subscriptionId != null) result.subscriptionId = subscriptionId;
    return result;
  }

  CancelSubscriptionRequest._();

  factory CancelSubscriptionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CancelSubscriptionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelSubscriptionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'subscriptionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSubscriptionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSubscriptionRequest copyWith(
          void Function(CancelSubscriptionRequest) updates) =>
      super.copyWith((message) => updates(message as CancelSubscriptionRequest))
          as CancelSubscriptionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CancelSubscriptionRequest create() => CancelSubscriptionRequest._();
  @$core.override
  CancelSubscriptionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CancelSubscriptionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelSubscriptionRequest>(create);
  static CancelSubscriptionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get subscriptionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set subscriptionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscriptionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscriptionId() => $_clearField(1);
}

class ResumeSubscriptionRequest extends $pb.GeneratedMessage {
  factory ResumeSubscriptionRequest({
    $core.String? subscriptionId,
  }) {
    final result = create();
    if (subscriptionId != null) result.subscriptionId = subscriptionId;
    return result;
  }

  ResumeSubscriptionRequest._();

  factory ResumeSubscriptionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResumeSubscriptionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResumeSubscriptionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'subscriptionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeSubscriptionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeSubscriptionRequest copyWith(
          void Function(ResumeSubscriptionRequest) updates) =>
      super.copyWith((message) => updates(message as ResumeSubscriptionRequest))
          as ResumeSubscriptionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResumeSubscriptionRequest create() => ResumeSubscriptionRequest._();
  @$core.override
  ResumeSubscriptionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResumeSubscriptionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResumeSubscriptionRequest>(create);
  static ResumeSubscriptionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get subscriptionId => $_getSZ(0);
  @$pb.TagNumber(1)
  set subscriptionId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscriptionId() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscriptionId() => $_clearField(1);
}

class SpaceSubscription extends $pb.GeneratedMessage {
  factory SpaceSubscription({
    $core.String? id,
    $2.SpaceRef? space,
    $core.String? purchaserAccountId,
    $core.String? plan,
    $core.String? billingPeriod,
    $core.String? status,
    $core.String? provider,
    $1.Timestamp? currentPeriodStart,
    $1.Timestamp? currentPeriodEnd,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (space != null) result.space = space;
    if (purchaserAccountId != null)
      result.purchaserAccountId = purchaserAccountId;
    if (plan != null) result.plan = plan;
    if (billingPeriod != null) result.billingPeriod = billingPeriod;
    if (status != null) result.status = status;
    if (provider != null) result.provider = provider;
    if (currentPeriodStart != null)
      result.currentPeriodStart = currentPeriodStart;
    if (currentPeriodEnd != null) result.currentPeriodEnd = currentPeriodEnd;
    return result;
  }

  SpaceSubscription._();

  factory SpaceSubscription.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory SpaceSubscription.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'SpaceSubscription',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOM<$2.SpaceRef>(2, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..aOS(3, _omitFieldNames ? '' : 'purchaserAccountId')
    ..aOS(4, _omitFieldNames ? '' : 'plan')
    ..aOS(5, _omitFieldNames ? '' : 'billingPeriod')
    ..aOS(6, _omitFieldNames ? '' : 'status')
    ..aOS(7, _omitFieldNames ? '' : 'provider')
    ..aOM<$1.Timestamp>(8, _omitFieldNames ? '' : 'currentPeriodStart',
        subBuilder: $1.Timestamp.create)
    ..aOM<$1.Timestamp>(9, _omitFieldNames ? '' : 'currentPeriodEnd',
        subBuilder: $1.Timestamp.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceSubscription clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  SpaceSubscription copyWith(void Function(SpaceSubscription) updates) =>
      super.copyWith((message) => updates(message as SpaceSubscription))
          as SpaceSubscription;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static SpaceSubscription create() => SpaceSubscription._();
  @$core.override
  SpaceSubscription createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static SpaceSubscription getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<SpaceSubscription>(create);
  static SpaceSubscription? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.SpaceRef get space => $_getN(1);
  @$pb.TagNumber(2)
  set space($2.SpaceRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasSpace() => $_has(1);
  @$pb.TagNumber(2)
  void clearSpace() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.SpaceRef ensureSpace() => $_ensure(1);

  @$pb.TagNumber(3)
  $core.String get purchaserAccountId => $_getSZ(2);
  @$pb.TagNumber(3)
  set purchaserAccountId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasPurchaserAccountId() => $_has(2);
  @$pb.TagNumber(3)
  void clearPurchaserAccountId() => $_clearField(3);

  @$pb.TagNumber(4)
  $core.String get plan => $_getSZ(3);
  @$pb.TagNumber(4)
  set plan($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPlan() => $_has(3);
  @$pb.TagNumber(4)
  void clearPlan() => $_clearField(4);

  @$pb.TagNumber(5)
  $core.String get billingPeriod => $_getSZ(4);
  @$pb.TagNumber(5)
  set billingPeriod($core.String value) => $_setString(4, value);
  @$pb.TagNumber(5)
  $core.bool hasBillingPeriod() => $_has(4);
  @$pb.TagNumber(5)
  void clearBillingPeriod() => $_clearField(5);

  @$pb.TagNumber(6)
  $core.String get status => $_getSZ(5);
  @$pb.TagNumber(6)
  set status($core.String value) => $_setString(5, value);
  @$pb.TagNumber(6)
  $core.bool hasStatus() => $_has(5);
  @$pb.TagNumber(6)
  void clearStatus() => $_clearField(6);

  @$pb.TagNumber(7)
  $core.String get provider => $_getSZ(6);
  @$pb.TagNumber(7)
  set provider($core.String value) => $_setString(6, value);
  @$pb.TagNumber(7)
  $core.bool hasProvider() => $_has(6);
  @$pb.TagNumber(7)
  void clearProvider() => $_clearField(7);

  @$pb.TagNumber(8)
  $1.Timestamp get currentPeriodStart => $_getN(7);
  @$pb.TagNumber(8)
  set currentPeriodStart($1.Timestamp value) => $_setField(8, value);
  @$pb.TagNumber(8)
  $core.bool hasCurrentPeriodStart() => $_has(7);
  @$pb.TagNumber(8)
  void clearCurrentPeriodStart() => $_clearField(8);
  @$pb.TagNumber(8)
  $1.Timestamp ensureCurrentPeriodStart() => $_ensure(7);

  @$pb.TagNumber(9)
  $1.Timestamp get currentPeriodEnd => $_getN(8);
  @$pb.TagNumber(9)
  set currentPeriodEnd($1.Timestamp value) => $_setField(9, value);
  @$pb.TagNumber(9)
  $core.bool hasCurrentPeriodEnd() => $_has(8);
  @$pb.TagNumber(9)
  void clearCurrentPeriodEnd() => $_clearField(9);
  @$pb.TagNumber(9)
  $1.Timestamp ensureCurrentPeriodEnd() => $_ensure(8);
}

class GetSpaceSubscriptionRequest extends $pb.GeneratedMessage {
  factory GetSpaceSubscriptionRequest({
    $2.SpaceRef? space,
  }) {
    final result = create();
    if (space != null) result.space = space;
    return result;
  }

  GetSpaceSubscriptionRequest._();

  factory GetSpaceSubscriptionRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceSubscriptionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceSubscriptionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<$2.SpaceRef>(1, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceSubscriptionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceSubscriptionRequest copyWith(
          void Function(GetSpaceSubscriptionRequest) updates) =>
      super.copyWith(
              (message) => updates(message as GetSpaceSubscriptionRequest))
          as GetSpaceSubscriptionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceSubscriptionRequest create() =>
      GetSpaceSubscriptionRequest._();
  @$core.override
  GetSpaceSubscriptionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceSubscriptionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceSubscriptionRequest>(create);
  static GetSpaceSubscriptionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.SpaceRef get space => $_getN(0);
  @$pb.TagNumber(1)
  set space($2.SpaceRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.SpaceRef ensureSpace() => $_ensure(0);
}

class CreateSpaceCheckoutSessionRequest extends $pb.GeneratedMessage {
  factory CreateSpaceCheckoutSessionRequest({
    $2.SpaceRef? space,
    $core.String? successUrl,
    $core.String? cancelUrl,
  }) {
    final result = create();
    if (space != null) result.space = space;
    if (successUrl != null) result.successUrl = successUrl;
    if (cancelUrl != null) result.cancelUrl = cancelUrl;
    return result;
  }

  CreateSpaceCheckoutSessionRequest._();

  factory CreateSpaceCheckoutSessionRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateSpaceCheckoutSessionRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateSpaceCheckoutSessionRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<$2.SpaceRef>(1, _omitFieldNames ? '' : 'space',
        subBuilder: $2.SpaceRef.create)
    ..aOS(2, _omitFieldNames ? '' : 'successUrl')
    ..aOS(3, _omitFieldNames ? '' : 'cancelUrl')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceCheckoutSessionRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceCheckoutSessionRequest copyWith(
          void Function(CreateSpaceCheckoutSessionRequest) updates) =>
      super.copyWith((message) =>
              updates(message as CreateSpaceCheckoutSessionRequest))
          as CreateSpaceCheckoutSessionRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateSpaceCheckoutSessionRequest create() =>
      CreateSpaceCheckoutSessionRequest._();
  @$core.override
  CreateSpaceCheckoutSessionRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateSpaceCheckoutSessionRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateSpaceCheckoutSessionRequest>(
          create);
  static CreateSpaceCheckoutSessionRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $2.SpaceRef get space => $_getN(0);
  @$pb.TagNumber(1)
  set space($2.SpaceRef value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpace() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpace() => $_clearField(1);
  @$pb.TagNumber(1)
  $2.SpaceRef ensureSpace() => $_ensure(0);

  @$pb.TagNumber(2)
  $core.String get successUrl => $_getSZ(1);
  @$pb.TagNumber(2)
  set successUrl($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSuccessUrl() => $_has(1);
  @$pb.TagNumber(2)
  void clearSuccessUrl() => $_clearField(2);

  @$pb.TagNumber(3)
  $core.String get cancelUrl => $_getSZ(2);
  @$pb.TagNumber(3)
  set cancelUrl($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasCancelUrl() => $_has(2);
  @$pb.TagNumber(3)
  void clearCancelUrl() => $_clearField(3);
}

class GetLimitsRequest extends $pb.GeneratedMessage {
  factory GetLimitsRequest({
    $core.String? accountId,
    $2.SpaceRef? scopeSpace,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (scopeSpace != null) result.scopeSpace = scopeSpace;
    return result;
  }

  GetLimitsRequest._();

  factory GetLimitsRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetLimitsRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetLimitsRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOM<$2.SpaceRef>(2, _omitFieldNames ? '' : 'scopeSpace',
        subBuilder: $2.SpaceRef.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLimitsRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLimitsRequest copyWith(void Function(GetLimitsRequest) updates) =>
      super.copyWith((message) => updates(message as GetLimitsRequest))
          as GetLimitsRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetLimitsRequest create() => GetLimitsRequest._();
  @$core.override
  GetLimitsRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetLimitsRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetLimitsRequest>(create);
  static GetLimitsRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $2.SpaceRef get scopeSpace => $_getN(1);
  @$pb.TagNumber(2)
  set scopeSpace($2.SpaceRef value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasScopeSpace() => $_has(1);
  @$pb.TagNumber(2)
  void clearScopeSpace() => $_clearField(2);
  @$pb.TagNumber(2)
  $2.SpaceRef ensureScopeSpace() => $_ensure(1);
}

class Limits extends $pb.GeneratedMessage {
  factory Limits({
    $core.String? limitsJson,
  }) {
    final result = create();
    if (limitsJson != null) result.limitsJson = limitsJson;
    return result;
  }

  Limits._();

  factory Limits.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory Limits.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Limits',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'limitsJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Limits clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Limits copyWith(void Function(Limits) updates) =>
      super.copyWith((message) => updates(message as Limits)) as Limits;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static Limits create() => Limits._();
  @$core.override
  Limits createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static Limits getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Limits>(create);
  static Limits? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get limitsJson => $_getSZ(0);
  @$pb.TagNumber(1)
  set limitsJson($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasLimitsJson() => $_has(0);
  @$pb.TagNumber(1)
  void clearLimitsJson() => $_clearField(1);
}

class CheckLimitRequest extends $pb.GeneratedMessage {
  factory CheckLimitRequest({
    $core.String? accountId,
    $core.String? limitName,
    $fixnum.Int64? delta,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (limitName != null) result.limitName = limitName;
    if (delta != null) result.delta = delta;
    return result;
  }

  CheckLimitRequest._();

  factory CheckLimitRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckLimitRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckLimitRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOS(2, _omitFieldNames ? '' : 'limitName')
    ..aInt64(3, _omitFieldNames ? '' : 'delta')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckLimitRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckLimitRequest copyWith(void Function(CheckLimitRequest) updates) =>
      super.copyWith((message) => updates(message as CheckLimitRequest))
          as CheckLimitRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckLimitRequest create() => CheckLimitRequest._();
  @$core.override
  CheckLimitRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckLimitRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckLimitRequest>(create);
  static CheckLimitRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get limitName => $_getSZ(1);
  @$pb.TagNumber(2)
  set limitName($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasLimitName() => $_has(1);
  @$pb.TagNumber(2)
  void clearLimitName() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get delta => $_getI64(2);
  @$pb.TagNumber(3)
  set delta($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasDelta() => $_has(2);
  @$pb.TagNumber(3)
  void clearDelta() => $_clearField(3);
}

class CheckLimitResponse extends $pb.GeneratedMessage {
  factory CheckLimitResponse({
    $core.bool? allowed,
    $fixnum.Int64? remaining,
  }) {
    final result = create();
    if (allowed != null) result.allowed = allowed;
    if (remaining != null) result.remaining = remaining;
    return result;
  }

  CheckLimitResponse._();

  factory CheckLimitResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CheckLimitResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CheckLimitResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOB(1, _omitFieldNames ? '' : 'allowed')
    ..aInt64(2, _omitFieldNames ? '' : 'remaining')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckLimitResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CheckLimitResponse copyWith(void Function(CheckLimitResponse) updates) =>
      super.copyWith((message) => updates(message as CheckLimitResponse))
          as CheckLimitResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CheckLimitResponse create() => CheckLimitResponse._();
  @$core.override
  CheckLimitResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CheckLimitResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CheckLimitResponse>(create);
  static CheckLimitResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.bool get allowed => $_getBF(0);
  @$pb.TagNumber(1)
  set allowed($core.bool value) => $_setBool(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAllowed() => $_has(0);
  @$pb.TagNumber(1)
  void clearAllowed() => $_clearField(1);

  @$pb.TagNumber(2)
  $fixnum.Int64 get remaining => $_getI64(1);
  @$pb.TagNumber(2)
  set remaining($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasRemaining() => $_has(1);
  @$pb.TagNumber(2)
  void clearRemaining() => $_clearField(2);
}

class HandlePaddleWebhookRequest extends $pb.GeneratedMessage {
  factory HandlePaddleWebhookRequest({
    $core.String? rawBody,
    $core.String? signature,
  }) {
    final result = create();
    if (rawBody != null) result.rawBody = rawBody;
    if (signature != null) result.signature = signature;
    return result;
  }

  HandlePaddleWebhookRequest._();

  factory HandlePaddleWebhookRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HandlePaddleWebhookRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HandlePaddleWebhookRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'rawBody')
    ..aOS(2, _omitFieldNames ? '' : 'signature')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandlePaddleWebhookRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandlePaddleWebhookRequest copyWith(
          void Function(HandlePaddleWebhookRequest) updates) =>
      super.copyWith(
              (message) => updates(message as HandlePaddleWebhookRequest))
          as HandlePaddleWebhookRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HandlePaddleWebhookRequest create() => HandlePaddleWebhookRequest._();
  @$core.override
  HandlePaddleWebhookRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HandlePaddleWebhookRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HandlePaddleWebhookRequest>(create);
  static HandlePaddleWebhookRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get rawBody => $_getSZ(0);
  @$pb.TagNumber(1)
  set rawBody($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRawBody() => $_has(0);
  @$pb.TagNumber(1)
  void clearRawBody() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get signature => $_getSZ(1);
  @$pb.TagNumber(2)
  set signature($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSignature() => $_has(1);
  @$pb.TagNumber(2)
  void clearSignature() => $_clearField(2);
}

class HandleCloudPaymentsWebhookRequest extends $pb.GeneratedMessage {
  factory HandleCloudPaymentsWebhookRequest({
    $core.String? rawBody,
    $core.String? signature,
  }) {
    final result = create();
    if (rawBody != null) result.rawBody = rawBody;
    if (signature != null) result.signature = signature;
    return result;
  }

  HandleCloudPaymentsWebhookRequest._();

  factory HandleCloudPaymentsWebhookRequest.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HandleCloudPaymentsWebhookRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HandleCloudPaymentsWebhookRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'rawBody')
    ..aOS(2, _omitFieldNames ? '' : 'signature')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandleCloudPaymentsWebhookRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandleCloudPaymentsWebhookRequest copyWith(
          void Function(HandleCloudPaymentsWebhookRequest) updates) =>
      super.copyWith((message) =>
              updates(message as HandleCloudPaymentsWebhookRequest))
          as HandleCloudPaymentsWebhookRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HandleCloudPaymentsWebhookRequest create() =>
      HandleCloudPaymentsWebhookRequest._();
  @$core.override
  HandleCloudPaymentsWebhookRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HandleCloudPaymentsWebhookRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HandleCloudPaymentsWebhookRequest>(
          create);
  static HandleCloudPaymentsWebhookRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get rawBody => $_getSZ(0);
  @$pb.TagNumber(1)
  set rawBody($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRawBody() => $_has(0);
  @$pb.TagNumber(1)
  void clearRawBody() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get signature => $_getSZ(1);
  @$pb.TagNumber(2)
  set signature($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasSignature() => $_has(1);
  @$pb.TagNumber(2)
  void clearSignature() => $_clearField(2);
}

class GetBillingHistoryRequest extends $pb.GeneratedMessage {
  factory GetBillingHistoryRequest({
    $core.String? accountId,
    $3.CursorPageRequest? page,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (page != null) result.page = page;
    return result;
  }

  GetBillingHistoryRequest._();

  factory GetBillingHistoryRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBillingHistoryRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBillingHistoryRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..aOM<$3.CursorPageRequest>(2, _omitFieldNames ? '' : 'page',
        subBuilder: $3.CursorPageRequest.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBillingHistoryRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBillingHistoryRequest copyWith(
          void Function(GetBillingHistoryRequest) updates) =>
      super.copyWith((message) => updates(message as GetBillingHistoryRequest))
          as GetBillingHistoryRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBillingHistoryRequest create() => GetBillingHistoryRequest._();
  @$core.override
  GetBillingHistoryRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBillingHistoryRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBillingHistoryRequest>(create);
  static GetBillingHistoryRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $3.CursorPageRequest get page => $_getN(1);
  @$pb.TagNumber(2)
  set page($3.CursorPageRequest value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasPage() => $_has(1);
  @$pb.TagNumber(2)
  void clearPage() => $_clearField(2);
  @$pb.TagNumber(2)
  $3.CursorPageRequest ensurePage() => $_ensure(1);
}

class BillingHistoryList extends $pb.GeneratedMessage {
  factory BillingHistoryList({
    $core.Iterable<BillingEvent>? events,
    $core.String? nextCursor,
  }) {
    final result = create();
    if (events != null) result.events.addAll(events);
    if (nextCursor != null) result.nextCursor = nextCursor;
    return result;
  }

  BillingHistoryList._();

  factory BillingHistoryList.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BillingHistoryList.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BillingHistoryList',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..pPM<BillingEvent>(1, _omitFieldNames ? '' : 'events',
        subBuilder: BillingEvent.create)
    ..aOS(2, _omitFieldNames ? '' : 'nextCursor')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BillingHistoryList clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BillingHistoryList copyWith(void Function(BillingHistoryList) updates) =>
      super.copyWith((message) => updates(message as BillingHistoryList))
          as BillingHistoryList;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BillingHistoryList create() => BillingHistoryList._();
  @$core.override
  BillingHistoryList createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BillingHistoryList getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BillingHistoryList>(create);
  static BillingHistoryList? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<BillingEvent> get events => $_getList(0);

  @$pb.TagNumber(2)
  $core.String get nextCursor => $_getSZ(1);
  @$pb.TagNumber(2)
  set nextCursor($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasNextCursor() => $_has(1);
  @$pb.TagNumber(2)
  void clearNextCursor() => $_clearField(2);
}

class BillingEvent extends $pb.GeneratedMessage {
  factory BillingEvent({
    $core.String? id,
    $core.String? type,
    $1.Timestamp? occurredAt,
    $core.String? payloadJson,
  }) {
    final result = create();
    if (id != null) result.id = id;
    if (type != null) result.type = type;
    if (occurredAt != null) result.occurredAt = occurredAt;
    if (payloadJson != null) result.payloadJson = payloadJson;
    return result;
  }

  BillingEvent._();

  factory BillingEvent.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory BillingEvent.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'BillingEvent',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'id')
    ..aOS(2, _omitFieldNames ? '' : 'type')
    ..aOM<$1.Timestamp>(3, _omitFieldNames ? '' : 'occurredAt',
        subBuilder: $1.Timestamp.create)
    ..aOS(4, _omitFieldNames ? '' : 'payloadJson')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BillingEvent clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  BillingEvent copyWith(void Function(BillingEvent) updates) =>
      super.copyWith((message) => updates(message as BillingEvent))
          as BillingEvent;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static BillingEvent create() => BillingEvent._();
  @$core.override
  BillingEvent createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static BillingEvent getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<BillingEvent>(create);
  static BillingEvent? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get id => $_getSZ(0);
  @$pb.TagNumber(1)
  set id($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasId() => $_has(0);
  @$pb.TagNumber(1)
  void clearId() => $_clearField(1);

  @$pb.TagNumber(2)
  $core.String get type => $_getSZ(1);
  @$pb.TagNumber(2)
  set type($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasType() => $_has(1);
  @$pb.TagNumber(2)
  void clearType() => $_clearField(2);

  @$pb.TagNumber(3)
  $1.Timestamp get occurredAt => $_getN(2);
  @$pb.TagNumber(3)
  set occurredAt($1.Timestamp value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasOccurredAt() => $_has(2);
  @$pb.TagNumber(3)
  void clearOccurredAt() => $_clearField(3);
  @$pb.TagNumber(3)
  $1.Timestamp ensureOccurredAt() => $_ensure(2);

  @$pb.TagNumber(4)
  $core.String get payloadJson => $_getSZ(3);
  @$pb.TagNumber(4)
  set payloadJson($core.String value) => $_setString(3, value);
  @$pb.TagNumber(4)
  $core.bool hasPayloadJson() => $_has(3);
  @$pb.TagNumber(4)
  void clearPayloadJson() => $_clearField(4);
}

class GetSubscriptionResponse extends $pb.GeneratedMessage {
  factory GetSubscriptionResponse({
    Subscription? subscription,
  }) {
    final result = create();
    if (subscription != null) result.subscription = subscription;
    return result;
  }

  GetSubscriptionResponse._();

  factory GetSubscriptionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSubscriptionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSubscriptionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<Subscription>(1, _omitFieldNames ? '' : 'subscription',
        subBuilder: Subscription.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSubscriptionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSubscriptionResponse copyWith(
          void Function(GetSubscriptionResponse) updates) =>
      super.copyWith((message) => updates(message as GetSubscriptionResponse))
          as GetSubscriptionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSubscriptionResponse create() => GetSubscriptionResponse._();
  @$core.override
  GetSubscriptionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSubscriptionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSubscriptionResponse>(create);
  static GetSubscriptionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Subscription get subscription => $_getN(0);
  @$pb.TagNumber(1)
  set subscription(Subscription value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscription() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscription() => $_clearField(1);
  @$pb.TagNumber(1)
  Subscription ensureSubscription() => $_ensure(0);
}

class CreateCheckoutSessionResponse extends $pb.GeneratedMessage {
  factory CreateCheckoutSessionResponse({
    CheckoutResponse? checkoutResponse,
  }) {
    final result = create();
    if (checkoutResponse != null) result.checkoutResponse = checkoutResponse;
    return result;
  }

  CreateCheckoutSessionResponse._();

  factory CreateCheckoutSessionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateCheckoutSessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateCheckoutSessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<CheckoutResponse>(1, _omitFieldNames ? '' : 'checkoutResponse',
        subBuilder: CheckoutResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCheckoutSessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateCheckoutSessionResponse copyWith(
          void Function(CreateCheckoutSessionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CreateCheckoutSessionResponse))
          as CreateCheckoutSessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateCheckoutSessionResponse create() =>
      CreateCheckoutSessionResponse._();
  @$core.override
  CreateCheckoutSessionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateCheckoutSessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateCheckoutSessionResponse>(create);
  static CreateCheckoutSessionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CheckoutResponse get checkoutResponse => $_getN(0);
  @$pb.TagNumber(1)
  set checkoutResponse(CheckoutResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCheckoutResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearCheckoutResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  CheckoutResponse ensureCheckoutResponse() => $_ensure(0);
}

class CancelSubscriptionResponse extends $pb.GeneratedMessage {
  factory CancelSubscriptionResponse({
    Subscription? subscription,
  }) {
    final result = create();
    if (subscription != null) result.subscription = subscription;
    return result;
  }

  CancelSubscriptionResponse._();

  factory CancelSubscriptionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CancelSubscriptionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CancelSubscriptionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<Subscription>(1, _omitFieldNames ? '' : 'subscription',
        subBuilder: Subscription.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSubscriptionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CancelSubscriptionResponse copyWith(
          void Function(CancelSubscriptionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as CancelSubscriptionResponse))
          as CancelSubscriptionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CancelSubscriptionResponse create() => CancelSubscriptionResponse._();
  @$core.override
  CancelSubscriptionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CancelSubscriptionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CancelSubscriptionResponse>(create);
  static CancelSubscriptionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Subscription get subscription => $_getN(0);
  @$pb.TagNumber(1)
  set subscription(Subscription value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscription() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscription() => $_clearField(1);
  @$pb.TagNumber(1)
  Subscription ensureSubscription() => $_ensure(0);
}

class ResumeSubscriptionResponse extends $pb.GeneratedMessage {
  factory ResumeSubscriptionResponse({
    Subscription? subscription,
  }) {
    final result = create();
    if (subscription != null) result.subscription = subscription;
    return result;
  }

  ResumeSubscriptionResponse._();

  factory ResumeSubscriptionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ResumeSubscriptionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ResumeSubscriptionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<Subscription>(1, _omitFieldNames ? '' : 'subscription',
        subBuilder: Subscription.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeSubscriptionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ResumeSubscriptionResponse copyWith(
          void Function(ResumeSubscriptionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ResumeSubscriptionResponse))
          as ResumeSubscriptionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ResumeSubscriptionResponse create() => ResumeSubscriptionResponse._();
  @$core.override
  ResumeSubscriptionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ResumeSubscriptionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ResumeSubscriptionResponse>(create);
  static ResumeSubscriptionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Subscription get subscription => $_getN(0);
  @$pb.TagNumber(1)
  set subscription(Subscription value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSubscription() => $_has(0);
  @$pb.TagNumber(1)
  void clearSubscription() => $_clearField(1);
  @$pb.TagNumber(1)
  Subscription ensureSubscription() => $_ensure(0);
}

class GetSpaceSubscriptionResponse extends $pb.GeneratedMessage {
  factory GetSpaceSubscriptionResponse({
    SpaceSubscription? spaceSubscription,
  }) {
    final result = create();
    if (spaceSubscription != null) result.spaceSubscription = spaceSubscription;
    return result;
  }

  GetSpaceSubscriptionResponse._();

  factory GetSpaceSubscriptionResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetSpaceSubscriptionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetSpaceSubscriptionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<SpaceSubscription>(1, _omitFieldNames ? '' : 'spaceSubscription',
        subBuilder: SpaceSubscription.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceSubscriptionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetSpaceSubscriptionResponse copyWith(
          void Function(GetSpaceSubscriptionResponse) updates) =>
      super.copyWith(
              (message) => updates(message as GetSpaceSubscriptionResponse))
          as GetSpaceSubscriptionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetSpaceSubscriptionResponse create() =>
      GetSpaceSubscriptionResponse._();
  @$core.override
  GetSpaceSubscriptionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetSpaceSubscriptionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetSpaceSubscriptionResponse>(create);
  static GetSpaceSubscriptionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  SpaceSubscription get spaceSubscription => $_getN(0);
  @$pb.TagNumber(1)
  set spaceSubscription(SpaceSubscription value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasSpaceSubscription() => $_has(0);
  @$pb.TagNumber(1)
  void clearSpaceSubscription() => $_clearField(1);
  @$pb.TagNumber(1)
  SpaceSubscription ensureSpaceSubscription() => $_ensure(0);
}

class CreateSpaceCheckoutSessionResponse extends $pb.GeneratedMessage {
  factory CreateSpaceCheckoutSessionResponse({
    CheckoutResponse? checkoutResponse,
  }) {
    final result = create();
    if (checkoutResponse != null) result.checkoutResponse = checkoutResponse;
    return result;
  }

  CreateSpaceCheckoutSessionResponse._();

  factory CreateSpaceCheckoutSessionResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory CreateSpaceCheckoutSessionResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CreateSpaceCheckoutSessionResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<CheckoutResponse>(1, _omitFieldNames ? '' : 'checkoutResponse',
        subBuilder: CheckoutResponse.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceCheckoutSessionResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CreateSpaceCheckoutSessionResponse copyWith(
          void Function(CreateSpaceCheckoutSessionResponse) updates) =>
      super.copyWith((message) =>
              updates(message as CreateSpaceCheckoutSessionResponse))
          as CreateSpaceCheckoutSessionResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static CreateSpaceCheckoutSessionResponse create() =>
      CreateSpaceCheckoutSessionResponse._();
  @$core.override
  CreateSpaceCheckoutSessionResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static CreateSpaceCheckoutSessionResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CreateSpaceCheckoutSessionResponse>(
          create);
  static CreateSpaceCheckoutSessionResponse? _defaultInstance;

  @$pb.TagNumber(1)
  CheckoutResponse get checkoutResponse => $_getN(0);
  @$pb.TagNumber(1)
  set checkoutResponse(CheckoutResponse value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasCheckoutResponse() => $_has(0);
  @$pb.TagNumber(1)
  void clearCheckoutResponse() => $_clearField(1);
  @$pb.TagNumber(1)
  CheckoutResponse ensureCheckoutResponse() => $_ensure(0);
}

class GetLimitsResponse extends $pb.GeneratedMessage {
  factory GetLimitsResponse({
    Limits? limits,
  }) {
    final result = create();
    if (limits != null) result.limits = limits;
    return result;
  }

  GetLimitsResponse._();

  factory GetLimitsResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetLimitsResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetLimitsResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<Limits>(1, _omitFieldNames ? '' : 'limits', subBuilder: Limits.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLimitsResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetLimitsResponse copyWith(void Function(GetLimitsResponse) updates) =>
      super.copyWith((message) => updates(message as GetLimitsResponse))
          as GetLimitsResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetLimitsResponse create() => GetLimitsResponse._();
  @$core.override
  GetLimitsResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetLimitsResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetLimitsResponse>(create);
  static GetLimitsResponse? _defaultInstance;

  @$pb.TagNumber(1)
  Limits get limits => $_getN(0);
  @$pb.TagNumber(1)
  set limits(Limits value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasLimits() => $_has(0);
  @$pb.TagNumber(1)
  void clearLimits() => $_clearField(1);
  @$pb.TagNumber(1)
  Limits ensureLimits() => $_ensure(0);
}

class HandlePaddleWebhookResponse extends $pb.GeneratedMessage {
  factory HandlePaddleWebhookResponse() => create();

  HandlePaddleWebhookResponse._();

  factory HandlePaddleWebhookResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HandlePaddleWebhookResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HandlePaddleWebhookResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandlePaddleWebhookResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandlePaddleWebhookResponse copyWith(
          void Function(HandlePaddleWebhookResponse) updates) =>
      super.copyWith(
              (message) => updates(message as HandlePaddleWebhookResponse))
          as HandlePaddleWebhookResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HandlePaddleWebhookResponse create() =>
      HandlePaddleWebhookResponse._();
  @$core.override
  HandlePaddleWebhookResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HandlePaddleWebhookResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HandlePaddleWebhookResponse>(create);
  static HandlePaddleWebhookResponse? _defaultInstance;
}

class HandleCloudPaymentsWebhookResponse extends $pb.GeneratedMessage {
  factory HandleCloudPaymentsWebhookResponse() => create();

  HandleCloudPaymentsWebhookResponse._();

  factory HandleCloudPaymentsWebhookResponse.fromBuffer(
          $core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory HandleCloudPaymentsWebhookResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'HandleCloudPaymentsWebhookResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandleCloudPaymentsWebhookResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  HandleCloudPaymentsWebhookResponse copyWith(
          void Function(HandleCloudPaymentsWebhookResponse) updates) =>
      super.copyWith((message) =>
              updates(message as HandleCloudPaymentsWebhookResponse))
          as HandleCloudPaymentsWebhookResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static HandleCloudPaymentsWebhookResponse create() =>
      HandleCloudPaymentsWebhookResponse._();
  @$core.override
  HandleCloudPaymentsWebhookResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static HandleCloudPaymentsWebhookResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<HandleCloudPaymentsWebhookResponse>(
          create);
  static HandleCloudPaymentsWebhookResponse? _defaultInstance;
}

class GetBillingHistoryResponse extends $pb.GeneratedMessage {
  factory GetBillingHistoryResponse({
    BillingHistoryList? billingHistoryList,
  }) {
    final result = create();
    if (billingHistoryList != null)
      result.billingHistoryList = billingHistoryList;
    return result;
  }

  GetBillingHistoryResponse._();

  factory GetBillingHistoryResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory GetBillingHistoryResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'GetBillingHistoryResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOM<BillingHistoryList>(1, _omitFieldNames ? '' : 'billingHistoryList',
        subBuilder: BillingHistoryList.create)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBillingHistoryResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  GetBillingHistoryResponse copyWith(
          void Function(GetBillingHistoryResponse) updates) =>
      super.copyWith((message) => updates(message as GetBillingHistoryResponse))
          as GetBillingHistoryResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static GetBillingHistoryResponse create() => GetBillingHistoryResponse._();
  @$core.override
  GetBillingHistoryResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static GetBillingHistoryResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<GetBillingHistoryResponse>(create);
  static GetBillingHistoryResponse? _defaultInstance;

  @$pb.TagNumber(1)
  BillingHistoryList get billingHistoryList => $_getN(0);
  @$pb.TagNumber(1)
  set billingHistoryList(BillingHistoryList value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasBillingHistoryList() => $_has(0);
  @$pb.TagNumber(1)
  void clearBillingHistoryList() => $_clearField(1);
  @$pb.TagNumber(1)
  BillingHistoryList ensureBillingHistoryList() => $_ensure(0);
}

class ApplyDowngradeProfilesRequest extends $pb.GeneratedMessage {
  factory ApplyDowngradeProfilesRequest({
    $core.String? accountId,
    $core.Iterable<$core.String>? keptProfileIds,
  }) {
    final result = create();
    if (accountId != null) result.accountId = accountId;
    if (keptProfileIds != null) result.keptProfileIds.addAll(keptProfileIds);
    return result;
  }

  ApplyDowngradeProfilesRequest._();

  factory ApplyDowngradeProfilesRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyDowngradeProfilesRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyDowngradeProfilesRequest',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..aOS(1, _omitFieldNames ? '' : 'accountId')
    ..pPS(2, _omitFieldNames ? '' : 'keptProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesRequest copyWith(
          void Function(ApplyDowngradeProfilesRequest) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyDowngradeProfilesRequest))
          as ApplyDowngradeProfilesRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesRequest create() =>
      ApplyDowngradeProfilesRequest._();
  @$core.override
  ApplyDowngradeProfilesRequest createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesRequest getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyDowngradeProfilesRequest>(create);
  static ApplyDowngradeProfilesRequest? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get accountId => $_getSZ(0);
  @$pb.TagNumber(1)
  set accountId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasAccountId() => $_has(0);
  @$pb.TagNumber(1)
  void clearAccountId() => $_clearField(1);

  @$pb.TagNumber(2)
  $pb.PbList<$core.String> get keptProfileIds => $_getList(1);
}

class ApplyDowngradeProfilesResponse extends $pb.GeneratedMessage {
  factory ApplyDowngradeProfilesResponse({
    $core.Iterable<$core.String>? keptProfileIds,
  }) {
    final result = create();
    if (keptProfileIds != null) result.keptProfileIds.addAll(keptProfileIds);
    return result;
  }

  ApplyDowngradeProfilesResponse._();

  factory ApplyDowngradeProfilesResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromBuffer(data, registry);
  factory ApplyDowngradeProfilesResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      create()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ApplyDowngradeProfilesResponse',
      package: const $pb.PackageName(
          _omitMessageNames ? '' : 'voice.subscription.v1'),
      createEmptyInstance: create)
    ..pPS(1, _omitFieldNames ? '' : 'keptProfileIds')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ApplyDowngradeProfilesResponse copyWith(
          void Function(ApplyDowngradeProfilesResponse) updates) =>
      super.copyWith(
              (message) => updates(message as ApplyDowngradeProfilesResponse))
          as ApplyDowngradeProfilesResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesResponse create() =>
      ApplyDowngradeProfilesResponse._();
  @$core.override
  ApplyDowngradeProfilesResponse createEmptyInstance() => create();
  @$core.pragma('dart2js:noInline')
  static ApplyDowngradeProfilesResponse getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<ApplyDowngradeProfilesResponse>(create);
  static ApplyDowngradeProfilesResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $pb.PbList<$core.String> get keptProfileIds => $_getList(0);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');
