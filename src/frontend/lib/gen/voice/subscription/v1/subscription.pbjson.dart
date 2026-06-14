// This is a generated file - do not edit.
//
// Generated from voice/subscription/v1/subscription.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use subscriptionDescriptor instead')
const Subscription$json = {
  '1': 'Subscription',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'account_id', '3': 2, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'plan', '3': 3, '4': 1, '5': 9, '10': 'plan'},
    {'1': 'billing_period', '3': 4, '4': 1, '5': 9, '10': 'billingPeriod'},
    {'1': 'status', '3': 5, '4': 1, '5': 9, '10': 'status'},
    {'1': 'provider', '3': 6, '4': 1, '5': 9, '10': 'provider'},
    {
      '1': 'provider_subscription_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'providerSubscriptionId'
    },
    {
      '1': 'current_period_start',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'currentPeriodStart'
    },
    {
      '1': 'current_period_end',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'currentPeriodEnd'
    },
    {
      '1': 'grace_period_end',
      '3': 10,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'gracePeriodEnd',
      '17': true
    },
    {
      '1': 'cancelled_at',
      '3': 11,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'cancelledAt',
      '17': true
    },
  ],
  '8': [
    {'1': '_grace_period_end'},
    {'1': '_cancelled_at'},
  ],
};

/// Descriptor for `Subscription`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List subscriptionDescriptor = $convert.base64Decode(
    'CgxTdWJzY3JpcHRpb24SDgoCaWQYASABKAlSAmlkEh0KCmFjY291bnRfaWQYAiABKAlSCWFjY2'
    '91bnRJZBISCgRwbGFuGAMgASgJUgRwbGFuEiUKDmJpbGxpbmdfcGVyaW9kGAQgASgJUg1iaWxs'
    'aW5nUGVyaW9kEhYKBnN0YXR1cxgFIAEoCVIGc3RhdHVzEhoKCHByb3ZpZGVyGAYgASgJUghwcm'
    '92aWRlchI4Chhwcm92aWRlcl9zdWJzY3JpcHRpb25faWQYByABKAlSFnByb3ZpZGVyU3Vic2Ny'
    'aXB0aW9uSWQSTAoUY3VycmVudF9wZXJpb2Rfc3RhcnQYCCABKAsyGi5nb29nbGUucHJvdG9idW'
    'YuVGltZXN0YW1wUhJjdXJyZW50UGVyaW9kU3RhcnQSSAoSY3VycmVudF9wZXJpb2RfZW5kGAkg'
    'ASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIQY3VycmVudFBlcmlvZEVuZBJJChBncm'
    'FjZV9wZXJpb2RfZW5kGAogASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgAUg5ncmFj'
    'ZVBlcmlvZEVuZIgBARJCCgxjYW5jZWxsZWRfYXQYCyABKAsyGi5nb29nbGUucHJvdG9idWYuVG'
    'ltZXN0YW1wSAFSC2NhbmNlbGxlZEF0iAEBQhMKEV9ncmFjZV9wZXJpb2RfZW5kQg8KDV9jYW5j'
    'ZWxsZWRfYXQ=');

@$core.Deprecated('Use getSubscriptionRequestDescriptor instead')
const GetSubscriptionRequest$json = {
  '1': 'GetSubscriptionRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
  ],
};

/// Descriptor for `GetSubscriptionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSubscriptionRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRTdWJzY3JpcHRpb25SZXF1ZXN0Eh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZA'
        '==');

@$core.Deprecated('Use createCheckoutSessionRequestDescriptor instead')
const CreateCheckoutSessionRequest$json = {
  '1': 'CreateCheckoutSessionRequest',
  '2': [
    {'1': 'plan', '3': 1, '4': 1, '5': 9, '10': 'plan'},
    {'1': 'billing_period', '3': 2, '4': 1, '5': 9, '10': 'billingPeriod'},
    {'1': 'success_url', '3': 3, '4': 1, '5': 9, '10': 'successUrl'},
    {'1': 'cancel_url', '3': 4, '4': 1, '5': 9, '10': 'cancelUrl'},
  ],
};

/// Descriptor for `CreateCheckoutSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCheckoutSessionRequestDescriptor =
    $convert.base64Decode(
        'ChxDcmVhdGVDaGVja291dFNlc3Npb25SZXF1ZXN0EhIKBHBsYW4YASABKAlSBHBsYW4SJQoOYm'
        'lsbGluZ19wZXJpb2QYAiABKAlSDWJpbGxpbmdQZXJpb2QSHwoLc3VjY2Vzc191cmwYAyABKAlS'
        'CnN1Y2Nlc3NVcmwSHQoKY2FuY2VsX3VybBgEIAEoCVIJY2FuY2VsVXJs');

@$core.Deprecated('Use checkoutResponseDescriptor instead')
const CheckoutResponse$json = {
  '1': 'CheckoutResponse',
  '2': [
    {'1': 'checkout_url', '3': 1, '4': 1, '5': 9, '10': 'checkoutUrl'},
    {'1': 'session_id', '3': 2, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `CheckoutResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkoutResponseDescriptor = $convert.base64Decode(
    'ChBDaGVja291dFJlc3BvbnNlEiEKDGNoZWNrb3V0X3VybBgBIAEoCVILY2hlY2tvdXRVcmwSHQ'
    'oKc2Vzc2lvbl9pZBgCIAEoCVIJc2Vzc2lvbklk');

@$core.Deprecated('Use cancelSubscriptionRequestDescriptor instead')
const CancelSubscriptionRequest$json = {
  '1': 'CancelSubscriptionRequest',
  '2': [
    {'1': 'subscription_id', '3': 1, '4': 1, '5': 9, '10': 'subscriptionId'},
  ],
};

/// Descriptor for `CancelSubscriptionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelSubscriptionRequestDescriptor =
    $convert.base64Decode(
        'ChlDYW5jZWxTdWJzY3JpcHRpb25SZXF1ZXN0EicKD3N1YnNjcmlwdGlvbl9pZBgBIAEoCVIOc3'
        'Vic2NyaXB0aW9uSWQ=');

@$core.Deprecated('Use resumeSubscriptionRequestDescriptor instead')
const ResumeSubscriptionRequest$json = {
  '1': 'ResumeSubscriptionRequest',
  '2': [
    {'1': 'subscription_id', '3': 1, '4': 1, '5': 9, '10': 'subscriptionId'},
  ],
};

/// Descriptor for `ResumeSubscriptionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resumeSubscriptionRequestDescriptor =
    $convert.base64Decode(
        'ChlSZXN1bWVTdWJzY3JpcHRpb25SZXF1ZXN0EicKD3N1YnNjcmlwdGlvbl9pZBgBIAEoCVIOc3'
        'Vic2NyaXB0aW9uSWQ=');

@$core.Deprecated('Use spaceSubscriptionDescriptor instead')
const SpaceSubscription$json = {
  '1': 'SpaceSubscription',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {
      '1': 'space',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '10': 'space'
    },
    {
      '1': 'purchaser_account_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '10': 'purchaserAccountId'
    },
    {'1': 'plan', '3': 4, '4': 1, '5': 9, '10': 'plan'},
    {'1': 'billing_period', '3': 5, '4': 1, '5': 9, '10': 'billingPeriod'},
    {'1': 'status', '3': 6, '4': 1, '5': 9, '10': 'status'},
    {'1': 'provider', '3': 7, '4': 1, '5': 9, '10': 'provider'},
    {
      '1': 'current_period_start',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'currentPeriodStart'
    },
    {
      '1': 'current_period_end',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'currentPeriodEnd'
    },
  ],
};

/// Descriptor for `SpaceSubscription`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceSubscriptionDescriptor = $convert.base64Decode(
    'ChFTcGFjZVN1YnNjcmlwdGlvbhIOCgJpZBgBIAEoCVICaWQSLgoFc3BhY2UYAiABKAsyGC52b2'
    'ljZS5zcGFjZS52MS5TcGFjZVJlZlIFc3BhY2USMAoUcHVyY2hhc2VyX2FjY291bnRfaWQYAyAB'
    'KAlSEnB1cmNoYXNlckFjY291bnRJZBISCgRwbGFuGAQgASgJUgRwbGFuEiUKDmJpbGxpbmdfcG'
    'VyaW9kGAUgASgJUg1iaWxsaW5nUGVyaW9kEhYKBnN0YXR1cxgGIAEoCVIGc3RhdHVzEhoKCHBy'
    'b3ZpZGVyGAcgASgJUghwcm92aWRlchJMChRjdXJyZW50X3BlcmlvZF9zdGFydBgIIAEoCzIaLm'
    'dvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSEmN1cnJlbnRQZXJpb2RTdGFydBJIChJjdXJyZW50'
    'X3BlcmlvZF9lbmQYCSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUhBjdXJyZW50UG'
    'VyaW9kRW5k');

@$core.Deprecated('Use getSpaceSubscriptionRequestDescriptor instead')
const GetSpaceSubscriptionRequest$json = {
  '1': 'GetSpaceSubscriptionRequest',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '10': 'space'
    },
  ],
};

/// Descriptor for `GetSpaceSubscriptionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceSubscriptionRequestDescriptor =
    $convert.base64Decode(
        'ChtHZXRTcGFjZVN1YnNjcmlwdGlvblJlcXVlc3QSLgoFc3BhY2UYASABKAsyGC52b2ljZS5zcG'
        'FjZS52MS5TcGFjZVJlZlIFc3BhY2U=');

@$core.Deprecated('Use createSpaceCheckoutSessionRequestDescriptor instead')
const CreateSpaceCheckoutSessionRequest$json = {
  '1': 'CreateSpaceCheckoutSessionRequest',
  '2': [
    {
      '1': 'space',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '10': 'space'
    },
    {'1': 'success_url', '3': 2, '4': 1, '5': 9, '10': 'successUrl'},
    {'1': 'cancel_url', '3': 3, '4': 1, '5': 9, '10': 'cancelUrl'},
  ],
};

/// Descriptor for `CreateSpaceCheckoutSessionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createSpaceCheckoutSessionRequestDescriptor =
    $convert.base64Decode(
        'CiFDcmVhdGVTcGFjZUNoZWNrb3V0U2Vzc2lvblJlcXVlc3QSLgoFc3BhY2UYASABKAsyGC52b2'
        'ljZS5zcGFjZS52MS5TcGFjZVJlZlIFc3BhY2USHwoLc3VjY2Vzc191cmwYAiABKAlSCnN1Y2Nl'
        'c3NVcmwSHQoKY2FuY2VsX3VybBgDIAEoCVIJY2FuY2VsVXJs');

@$core.Deprecated('Use getLimitsRequestDescriptor instead')
const GetLimitsRequest$json = {
  '1': 'GetLimitsRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'scope_space',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.space.v1.SpaceRef',
      '9': 0,
      '10': 'scopeSpace',
      '17': true
    },
  ],
  '8': [
    {'1': '_scope_space'},
  ],
};

/// Descriptor for `GetLimitsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getLimitsRequestDescriptor = $convert.base64Decode(
    'ChBHZXRMaW1pdHNSZXF1ZXN0Eh0KCmFjY291bnRfaWQYASABKAlSCWFjY291bnRJZBI+CgtzY2'
    '9wZV9zcGFjZRgCIAEoCzIYLnZvaWNlLnNwYWNlLnYxLlNwYWNlUmVmSABSCnNjb3BlU3BhY2WI'
    'AQFCDgoMX3Njb3BlX3NwYWNl');

@$core.Deprecated('Use limitsDescriptor instead')
const Limits$json = {
  '1': 'Limits',
  '2': [
    {'1': 'limits_json', '3': 1, '4': 1, '5': 9, '10': 'limitsJson'},
  ],
};

/// Descriptor for `Limits`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List limitsDescriptor = $convert
    .base64Decode('CgZMaW1pdHMSHwoLbGltaXRzX2pzb24YASABKAlSCmxpbWl0c0pzb24=');

@$core.Deprecated('Use checkLimitRequestDescriptor instead')
const CheckLimitRequest$json = {
  '1': 'CheckLimitRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'limit_name', '3': 2, '4': 1, '5': 9, '10': 'limitName'},
    {'1': 'delta', '3': 3, '4': 1, '5': 3, '10': 'delta'},
  ],
};

/// Descriptor for `CheckLimitRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkLimitRequestDescriptor = $convert.base64Decode(
    'ChFDaGVja0xpbWl0UmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2NvdW50SWQSHQoKbG'
    'ltaXRfbmFtZRgCIAEoCVIJbGltaXROYW1lEhQKBWRlbHRhGAMgASgDUgVkZWx0YQ==');

@$core.Deprecated('Use checkLimitResponseDescriptor instead')
const CheckLimitResponse$json = {
  '1': 'CheckLimitResponse',
  '2': [
    {'1': 'allowed', '3': 1, '4': 1, '5': 8, '10': 'allowed'},
    {'1': 'remaining', '3': 2, '4': 1, '5': 3, '10': 'remaining'},
  ],
};

/// Descriptor for `CheckLimitResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List checkLimitResponseDescriptor = $convert.base64Decode(
    'ChJDaGVja0xpbWl0UmVzcG9uc2USGAoHYWxsb3dlZBgBIAEoCFIHYWxsb3dlZBIcCglyZW1haW'
    '5pbmcYAiABKANSCXJlbWFpbmluZw==');

@$core.Deprecated('Use handlePaddleWebhookRequestDescriptor instead')
const HandlePaddleWebhookRequest$json = {
  '1': 'HandlePaddleWebhookRequest',
  '2': [
    {'1': 'raw_body', '3': 1, '4': 1, '5': 9, '10': 'rawBody'},
    {'1': 'signature', '3': 2, '4': 1, '5': 9, '10': 'signature'},
  ],
};

/// Descriptor for `HandlePaddleWebhookRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List handlePaddleWebhookRequestDescriptor =
    $convert.base64Decode(
        'ChpIYW5kbGVQYWRkbGVXZWJob29rUmVxdWVzdBIZCghyYXdfYm9keRgBIAEoCVIHcmF3Qm9keR'
        'IcCglzaWduYXR1cmUYAiABKAlSCXNpZ25hdHVyZQ==');

@$core.Deprecated('Use handleCloudPaymentsWebhookRequestDescriptor instead')
const HandleCloudPaymentsWebhookRequest$json = {
  '1': 'HandleCloudPaymentsWebhookRequest',
  '2': [
    {'1': 'raw_body', '3': 1, '4': 1, '5': 9, '10': 'rawBody'},
    {'1': 'signature', '3': 2, '4': 1, '5': 9, '10': 'signature'},
  ],
};

/// Descriptor for `HandleCloudPaymentsWebhookRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List handleCloudPaymentsWebhookRequestDescriptor =
    $convert.base64Decode(
        'CiFIYW5kbGVDbG91ZFBheW1lbnRzV2ViaG9va1JlcXVlc3QSGQoIcmF3X2JvZHkYASABKAlSB3'
        'Jhd0JvZHkSHAoJc2lnbmF0dXJlGAIgASgJUglzaWduYXR1cmU=');

@$core.Deprecated('Use getBillingHistoryRequestDescriptor instead')
const GetBillingHistoryRequest$json = {
  '1': 'GetBillingHistoryRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {
      '1': 'page',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `GetBillingHistoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBillingHistoryRequestDescriptor = $convert.base64Decode(
    'ChhHZXRCaWxsaW5nSGlzdG9yeVJlcXVlc3QSHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudE'
    'lkEjYKBHBhZ2UYAiABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc29yUGFnZVJlcXVlc3RSBHBh'
    'Z2U=');

@$core.Deprecated('Use billingHistoryListDescriptor instead')
const BillingHistoryList$json = {
  '1': 'BillingHistoryList',
  '2': [
    {
      '1': 'events',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.subscription.v1.BillingEvent',
      '10': 'events'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `BillingHistoryList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List billingHistoryListDescriptor = $convert.base64Decode(
    'ChJCaWxsaW5nSGlzdG9yeUxpc3QSOwoGZXZlbnRzGAEgAygLMiMudm9pY2Uuc3Vic2NyaXB0aW'
    '9uLnYxLkJpbGxpbmdFdmVudFIGZXZlbnRzEh8KC25leHRfY3Vyc29yGAIgASgJUgpuZXh0Q3Vy'
    'c29y');

@$core.Deprecated('Use billingEventDescriptor instead')
const BillingEvent$json = {
  '1': 'BillingEvent',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'type', '3': 2, '4': 1, '5': 9, '10': 'type'},
    {
      '1': 'occurred_at',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'occurredAt'
    },
    {'1': 'payload_json', '3': 4, '4': 1, '5': 9, '10': 'payloadJson'},
  ],
};

/// Descriptor for `BillingEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List billingEventDescriptor = $convert.base64Decode(
    'CgxCaWxsaW5nRXZlbnQSDgoCaWQYASABKAlSAmlkEhIKBHR5cGUYAiABKAlSBHR5cGUSOwoLb2'
    'NjdXJyZWRfYXQYAyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wUgpvY2N1cnJlZEF0'
    'EiEKDHBheWxvYWRfanNvbhgEIAEoCVILcGF5bG9hZEpzb24=');

@$core.Deprecated('Use getSubscriptionResponseDescriptor instead')
const GetSubscriptionResponse$json = {
  '1': 'GetSubscriptionResponse',
  '2': [
    {
      '1': 'subscription',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.Subscription',
      '10': 'subscription'
    },
  ],
};

/// Descriptor for `GetSubscriptionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSubscriptionResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRTdWJzY3JpcHRpb25SZXNwb25zZRJHCgxzdWJzY3JpcHRpb24YASABKAsyIy52b2ljZS'
        '5zdWJzY3JpcHRpb24udjEuU3Vic2NyaXB0aW9uUgxzdWJzY3JpcHRpb24=');

@$core.Deprecated('Use createCheckoutSessionResponseDescriptor instead')
const CreateCheckoutSessionResponse$json = {
  '1': 'CreateCheckoutSessionResponse',
  '2': [
    {
      '1': 'checkout_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.CheckoutResponse',
      '10': 'checkoutResponse'
    },
  ],
};

/// Descriptor for `CreateCheckoutSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createCheckoutSessionResponseDescriptor =
    $convert.base64Decode(
        'Ch1DcmVhdGVDaGVja291dFNlc3Npb25SZXNwb25zZRJUChFjaGVja291dF9yZXNwb25zZRgBIA'
        'EoCzInLnZvaWNlLnN1YnNjcmlwdGlvbi52MS5DaGVja291dFJlc3BvbnNlUhBjaGVja291dFJl'
        'c3BvbnNl');

@$core.Deprecated('Use cancelSubscriptionResponseDescriptor instead')
const CancelSubscriptionResponse$json = {
  '1': 'CancelSubscriptionResponse',
  '2': [
    {
      '1': 'subscription',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.Subscription',
      '10': 'subscription'
    },
  ],
};

/// Descriptor for `CancelSubscriptionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelSubscriptionResponseDescriptor =
    $convert.base64Decode(
        'ChpDYW5jZWxTdWJzY3JpcHRpb25SZXNwb25zZRJHCgxzdWJzY3JpcHRpb24YASABKAsyIy52b2'
        'ljZS5zdWJzY3JpcHRpb24udjEuU3Vic2NyaXB0aW9uUgxzdWJzY3JpcHRpb24=');

@$core.Deprecated('Use resumeSubscriptionResponseDescriptor instead')
const ResumeSubscriptionResponse$json = {
  '1': 'ResumeSubscriptionResponse',
  '2': [
    {
      '1': 'subscription',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.Subscription',
      '10': 'subscription'
    },
  ],
};

/// Descriptor for `ResumeSubscriptionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List resumeSubscriptionResponseDescriptor =
    $convert.base64Decode(
        'ChpSZXN1bWVTdWJzY3JpcHRpb25SZXNwb25zZRJHCgxzdWJzY3JpcHRpb24YASABKAsyIy52b2'
        'ljZS5zdWJzY3JpcHRpb24udjEuU3Vic2NyaXB0aW9uUgxzdWJzY3JpcHRpb24=');

@$core.Deprecated('Use getSpaceSubscriptionResponseDescriptor instead')
const GetSpaceSubscriptionResponse$json = {
  '1': 'GetSpaceSubscriptionResponse',
  '2': [
    {
      '1': 'space_subscription',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.SpaceSubscription',
      '10': 'spaceSubscription'
    },
  ],
};

/// Descriptor for `GetSpaceSubscriptionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSpaceSubscriptionResponseDescriptor =
    $convert.base64Decode(
        'ChxHZXRTcGFjZVN1YnNjcmlwdGlvblJlc3BvbnNlElcKEnNwYWNlX3N1YnNjcmlwdGlvbhgBIA'
        'EoCzIoLnZvaWNlLnN1YnNjcmlwdGlvbi52MS5TcGFjZVN1YnNjcmlwdGlvblIRc3BhY2VTdWJz'
        'Y3JpcHRpb24=');

@$core.Deprecated('Use createSpaceCheckoutSessionResponseDescriptor instead')
const CreateSpaceCheckoutSessionResponse$json = {
  '1': 'CreateSpaceCheckoutSessionResponse',
  '2': [
    {
      '1': 'checkout_response',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.CheckoutResponse',
      '10': 'checkoutResponse'
    },
  ],
};

/// Descriptor for `CreateSpaceCheckoutSessionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createSpaceCheckoutSessionResponseDescriptor =
    $convert.base64Decode(
        'CiJDcmVhdGVTcGFjZUNoZWNrb3V0U2Vzc2lvblJlc3BvbnNlElQKEWNoZWNrb3V0X3Jlc3Bvbn'
        'NlGAEgASgLMicudm9pY2Uuc3Vic2NyaXB0aW9uLnYxLkNoZWNrb3V0UmVzcG9uc2VSEGNoZWNr'
        'b3V0UmVzcG9uc2U=');

@$core.Deprecated('Use getLimitsResponseDescriptor instead')
const GetLimitsResponse$json = {
  '1': 'GetLimitsResponse',
  '2': [
    {
      '1': 'limits',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.Limits',
      '10': 'limits'
    },
  ],
};

/// Descriptor for `GetLimitsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getLimitsResponseDescriptor = $convert.base64Decode(
    'ChFHZXRMaW1pdHNSZXNwb25zZRI1CgZsaW1pdHMYASABKAsyHS52b2ljZS5zdWJzY3JpcHRpb2'
    '4udjEuTGltaXRzUgZsaW1pdHM=');

@$core.Deprecated('Use handlePaddleWebhookResponseDescriptor instead')
const HandlePaddleWebhookResponse$json = {
  '1': 'HandlePaddleWebhookResponse',
};

/// Descriptor for `HandlePaddleWebhookResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List handlePaddleWebhookResponseDescriptor =
    $convert.base64Decode('ChtIYW5kbGVQYWRkbGVXZWJob29rUmVzcG9uc2U=');

@$core.Deprecated('Use handleCloudPaymentsWebhookResponseDescriptor instead')
const HandleCloudPaymentsWebhookResponse$json = {
  '1': 'HandleCloudPaymentsWebhookResponse',
};

/// Descriptor for `HandleCloudPaymentsWebhookResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List handleCloudPaymentsWebhookResponseDescriptor =
    $convert.base64Decode('CiJIYW5kbGVDbG91ZFBheW1lbnRzV2ViaG9va1Jlc3BvbnNl');

@$core.Deprecated('Use getBillingHistoryResponseDescriptor instead')
const GetBillingHistoryResponse$json = {
  '1': 'GetBillingHistoryResponse',
  '2': [
    {
      '1': 'billing_history_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.subscription.v1.BillingHistoryList',
      '10': 'billingHistoryList'
    },
  ],
};

/// Descriptor for `GetBillingHistoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getBillingHistoryResponseDescriptor = $convert.base64Decode(
    'ChlHZXRCaWxsaW5nSGlzdG9yeVJlc3BvbnNlElsKFGJpbGxpbmdfaGlzdG9yeV9saXN0GAEgAS'
    'gLMikudm9pY2Uuc3Vic2NyaXB0aW9uLnYxLkJpbGxpbmdIaXN0b3J5TGlzdFISYmlsbGluZ0hp'
    'c3RvcnlMaXN0');

@$core.Deprecated('Use applyDowngradeProfilesRequestDescriptor instead')
const ApplyDowngradeProfilesRequest$json = {
  '1': 'ApplyDowngradeProfilesRequest',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'kept_profile_ids', '3': 2, '4': 3, '5': 9, '10': 'keptProfileIds'},
  ],
};

/// Descriptor for `ApplyDowngradeProfilesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyDowngradeProfilesRequestDescriptor =
    $convert.base64Decode(
        'Ch1BcHBseURvd25ncmFkZVByb2ZpbGVzUmVxdWVzdBIdCgphY2NvdW50X2lkGAEgASgJUglhY2'
        'NvdW50SWQSKAoQa2VwdF9wcm9maWxlX2lkcxgCIAMoCVIOa2VwdFByb2ZpbGVJZHM=');

@$core.Deprecated('Use applyDowngradeProfilesResponseDescriptor instead')
const ApplyDowngradeProfilesResponse$json = {
  '1': 'ApplyDowngradeProfilesResponse',
  '2': [
    {'1': 'kept_profile_ids', '3': 1, '4': 3, '5': 9, '10': 'keptProfileIds'},
  ],
};

/// Descriptor for `ApplyDowngradeProfilesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List applyDowngradeProfilesResponseDescriptor =
    $convert.base64Decode(
        'Ch5BcHBseURvd25ncmFkZVByb2ZpbGVzUmVzcG9uc2USKAoQa2VwdF9wcm9maWxlX2lkcxgBIA'
        'MoCVIOa2VwdFByb2ZpbGVJZHM=');
