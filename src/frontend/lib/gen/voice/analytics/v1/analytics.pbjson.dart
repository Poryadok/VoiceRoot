// This is a generated file - do not edit.
//
// Generated from voice/analytics/v1/analytics.proto.

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

@$core.Deprecated('Use ingestEventRequestDescriptor instead')
const IngestEventRequest$json = {
  '1': 'IngestEventRequest',
  '2': [
    {
      '1': 'event',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.analytics.v1.AnalyticsEvent',
      '10': 'event'
    },
  ],
};

/// Descriptor for `IngestEventRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ingestEventRequestDescriptor = $convert.base64Decode(
    'ChJJbmdlc3RFdmVudFJlcXVlc3QSOAoFZXZlbnQYASABKAsyIi52b2ljZS5hbmFseXRpY3Mudj'
    'EuQW5hbHl0aWNzRXZlbnRSBWV2ZW50');

@$core.Deprecated('Use ingestBatchRequestDescriptor instead')
const IngestBatchRequest$json = {
  '1': 'IngestBatchRequest',
  '2': [
    {
      '1': 'events',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.AnalyticsEvent',
      '10': 'events'
    },
  ],
};

/// Descriptor for `IngestBatchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ingestBatchRequestDescriptor = $convert.base64Decode(
    'ChJJbmdlc3RCYXRjaFJlcXVlc3QSOgoGZXZlbnRzGAEgAygLMiIudm9pY2UuYW5hbHl0aWNzLn'
    'YxLkFuYWx5dGljc0V2ZW50UgZldmVudHM=');

@$core.Deprecated('Use ingestEventResponseDescriptor instead')
const IngestEventResponse$json = {
  '1': 'IngestEventResponse',
};

/// Descriptor for `IngestEventResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ingestEventResponseDescriptor =
    $convert.base64Decode('ChNJbmdlc3RFdmVudFJlc3BvbnNl');

@$core.Deprecated('Use ingestBatchResponseDescriptor instead')
const IngestBatchResponse$json = {
  '1': 'IngestBatchResponse',
};

/// Descriptor for `IngestBatchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List ingestBatchResponseDescriptor =
    $convert.base64Decode('ChNJbmdlc3RCYXRjaFJlc3BvbnNl');

@$core.Deprecated('Use analyticsEventDescriptor instead')
const AnalyticsEvent$json = {
  '1': 'AnalyticsEvent',
  '2': [
    {'1': 'event_id', '3': 1, '4': 1, '5': 9, '10': 'eventId'},
    {'1': 'event_type', '3': 2, '4': 1, '5': 9, '10': 'eventType'},
    {'1': 'source_service', '3': 3, '4': 1, '5': 9, '10': 'sourceService'},
    {
      '1': 'timestamp',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'timestamp'
    },
    {'1': 'properties_json', '3': 5, '4': 1, '5': 9, '10': 'propertiesJson'},
    {
      '1': 'session_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'sessionId',
      '17': true
    },
    {
      '1': 'platform',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'platform',
      '17': true
    },
    {
      '1': 'app_version',
      '3': 8,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'appVersion',
      '17': true
    },
    {'1': 'region', '3': 9, '4': 1, '5': 9, '9': 3, '10': 'region', '17': true},
    {
      '1': 'user_id_hashed',
      '3': 10,
      '4': 1,
      '5': 9,
      '9': 4,
      '10': 'userIdHashed',
      '17': true
    },
    {
      '1': 'profile_id_hashed',
      '3': 11,
      '4': 1,
      '5': 9,
      '9': 5,
      '10': 'profileIdHashed',
      '17': true
    },
  ],
  '8': [
    {'1': '_session_id'},
    {'1': '_platform'},
    {'1': '_app_version'},
    {'1': '_region'},
    {'1': '_user_id_hashed'},
    {'1': '_profile_id_hashed'},
  ],
};

/// Descriptor for `AnalyticsEvent`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List analyticsEventDescriptor = $convert.base64Decode(
    'Cg5BbmFseXRpY3NFdmVudBIZCghldmVudF9pZBgBIAEoCVIHZXZlbnRJZBIdCgpldmVudF90eX'
    'BlGAIgASgJUglldmVudFR5cGUSJQoOc291cmNlX3NlcnZpY2UYAyABKAlSDXNvdXJjZVNlcnZp'
    'Y2USOAoJdGltZXN0YW1wGAQgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJdGltZX'
    'N0YW1wEicKD3Byb3BlcnRpZXNfanNvbhgFIAEoCVIOcHJvcGVydGllc0pzb24SIgoKc2Vzc2lv'
    'bl9pZBgGIAEoCUgAUglzZXNzaW9uSWSIAQESHwoIcGxhdGZvcm0YByABKAlIAVIIcGxhdGZvcm'
    '2IAQESJAoLYXBwX3ZlcnNpb24YCCABKAlIAlIKYXBwVmVyc2lvbogBARIbCgZyZWdpb24YCSAB'
    'KAlIA1IGcmVnaW9uiAEBEikKDnVzZXJfaWRfaGFzaGVkGAogASgJSARSDHVzZXJJZEhhc2hlZI'
    'gBARIvChFwcm9maWxlX2lkX2hhc2hlZBgLIAEoCUgFUg9wcm9maWxlSWRIYXNoZWSIAQFCDQoL'
    'X3Nlc3Npb25faWRCCwoJX3BsYXRmb3JtQg4KDF9hcHBfdmVyc2lvbkIJCgdfcmVnaW9uQhEKD1'
    '91c2VyX2lkX2hhc2hlZEIUChJfcHJvZmlsZV9pZF9oYXNoZWQ=');

@$core.Deprecated('Use metricPointDescriptor instead')
const MetricPoint$json = {
  '1': 'MetricPoint',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'value', '3': 2, '4': 1, '5': 1, '10': 'value'},
    {'1': 'label', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'label', '17': true},
  ],
  '8': [
    {'1': '_label'},
  ],
};

/// Descriptor for `MetricPoint`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List metricPointDescriptor = $convert.base64Decode(
    'CgtNZXRyaWNQb2ludBISCgRuYW1lGAEgASgJUgRuYW1lEhQKBXZhbHVlGAIgASgBUgV2YWx1ZR'
    'IZCgVsYWJlbBgDIAEoCUgAUgVsYWJlbIgBAUIICgZfbGFiZWw=');

@$core.Deprecated('Use getDashboardRequestDescriptor instead')
const GetDashboardRequest$json = {
  '1': 'GetDashboardRequest',
  '2': [
    {'1': 'dashboard_type', '3': 1, '4': 1, '5': 9, '10': 'dashboardType'},
    {
      '1': 'from',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'from',
      '17': true
    },
    {
      '1': 'to',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'to',
      '17': true
    },
  ],
  '8': [
    {'1': '_from'},
    {'1': '_to'},
  ],
};

/// Descriptor for `GetDashboardRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDashboardRequestDescriptor = $convert.base64Decode(
    'ChNHZXREYXNoYm9hcmRSZXF1ZXN0EiUKDmRhc2hib2FyZF90eXBlGAEgASgJUg1kYXNoYm9hcm'
    'RUeXBlEjMKBGZyb20YAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSABSBGZyb22I'
    'AQESLwoCdG8YAyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAFSAnRviAEBQgcKBV'
    '9mcm9tQgUKA190bw==');

@$core.Deprecated('Use getDashboardResponseDescriptor instead')
const GetDashboardResponse$json = {
  '1': 'GetDashboardResponse',
  '2': [
    {'1': 'dashboard_type', '3': 1, '4': 1, '5': 9, '10': 'dashboardType'},
    {
      '1': 'metrics',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.MetricPoint',
      '10': 'metrics'
    },
  ],
};

/// Descriptor for `GetDashboardResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getDashboardResponseDescriptor = $convert.base64Decode(
    'ChRHZXREYXNoYm9hcmRSZXNwb25zZRIlCg5kYXNoYm9hcmRfdHlwZRgBIAEoCVINZGFzaGJvYX'
    'JkVHlwZRI5CgdtZXRyaWNzGAIgAygLMh8udm9pY2UuYW5hbHl0aWNzLnYxLk1ldHJpY1BvaW50'
    'UgdtZXRyaWNz');

@$core.Deprecated('Use getMetricsRequestDescriptor instead')
const GetMetricsRequest$json = {
  '1': 'GetMetricsRequest',
  '2': [
    {'1': 'metric', '3': 1, '4': 1, '5': 9, '10': 'metric'},
    {
      '1': 'from',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'from',
      '17': true
    },
    {
      '1': 'to',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'to',
      '17': true
    },
    {
      '1': 'filters',
      '3': 4,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.GetMetricsRequest.FiltersEntry',
      '10': 'filters'
    },
  ],
  '3': [GetMetricsRequest_FiltersEntry$json],
  '8': [
    {'1': '_from'},
    {'1': '_to'},
  ],
};

@$core.Deprecated('Use getMetricsRequestDescriptor instead')
const GetMetricsRequest_FiltersEntry$json = {
  '1': 'FiltersEntry',
  '2': [
    {'1': 'key', '3': 1, '4': 1, '5': 9, '10': 'key'},
    {'1': 'value', '3': 2, '4': 1, '5': 9, '10': 'value'},
  ],
  '7': {'7': true},
};

/// Descriptor for `GetMetricsRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMetricsRequestDescriptor = $convert.base64Decode(
    'ChFHZXRNZXRyaWNzUmVxdWVzdBIWCgZtZXRyaWMYASABKAlSBm1ldHJpYxIzCgRmcm9tGAIgAS'
    'gLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgAUgRmcm9tiAEBEi8KAnRvGAMgASgLMhou'
    'Z29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcEgBUgJ0b4gBARJMCgdmaWx0ZXJzGAQgAygLMjIudm'
    '9pY2UuYW5hbHl0aWNzLnYxLkdldE1ldHJpY3NSZXF1ZXN0LkZpbHRlcnNFbnRyeVIHZmlsdGVy'
    'cxo6CgxGaWx0ZXJzRW50cnkSEAoDa2V5GAEgASgJUgNrZXkSFAoFdmFsdWUYAiABKAlSBXZhbH'
    'VlOgI4AUIHCgVfZnJvbUIFCgNfdG8=');

@$core.Deprecated('Use getMetricsResponseDescriptor instead')
const GetMetricsResponse$json = {
  '1': 'GetMetricsResponse',
  '2': [
    {
      '1': 'points',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.MetricPoint',
      '10': 'points'
    },
  ],
};

/// Descriptor for `GetMetricsResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMetricsResponseDescriptor = $convert.base64Decode(
    'ChJHZXRNZXRyaWNzUmVzcG9uc2USNwoGcG9pbnRzGAEgAygLMh8udm9pY2UuYW5hbHl0aWNzLn'
    'YxLk1ldHJpY1BvaW50UgZwb2ludHM=');

@$core.Deprecated('Use getFunnelRequestDescriptor instead')
const GetFunnelRequest$json = {
  '1': 'GetFunnelRequest',
  '2': [
    {'1': 'funnel_name', '3': 1, '4': 1, '5': 9, '10': 'funnelName'},
    {
      '1': 'from',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'from',
      '17': true
    },
    {
      '1': 'to',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'to',
      '17': true
    },
  ],
  '8': [
    {'1': '_from'},
    {'1': '_to'},
  ],
};

/// Descriptor for `GetFunnelRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFunnelRequestDescriptor = $convert.base64Decode(
    'ChBHZXRGdW5uZWxSZXF1ZXN0Eh8KC2Z1bm5lbF9uYW1lGAEgASgJUgpmdW5uZWxOYW1lEjMKBG'
    'Zyb20YAiABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSABSBGZyb22IAQESLwoCdG8Y'
    'AyABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAFSAnRviAEBQgcKBV9mcm9tQgUKA1'
    '90bw==');

@$core.Deprecated('Use funnelStepDescriptor instead')
const FunnelStep$json = {
  '1': 'FunnelStep',
  '2': [
    {'1': 'step', '3': 1, '4': 1, '5': 9, '10': 'step'},
    {'1': 'count', '3': 2, '4': 1, '5': 3, '10': 'count'},
  ],
};

/// Descriptor for `FunnelStep`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List funnelStepDescriptor = $convert.base64Decode(
    'CgpGdW5uZWxTdGVwEhIKBHN0ZXAYASABKAlSBHN0ZXASFAoFY291bnQYAiABKANSBWNvdW50');

@$core.Deprecated('Use getFunnelResponseDescriptor instead')
const GetFunnelResponse$json = {
  '1': 'GetFunnelResponse',
  '2': [
    {'1': 'funnel_name', '3': 1, '4': 1, '5': 9, '10': 'funnelName'},
    {
      '1': 'steps',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.FunnelStep',
      '10': 'steps'
    },
  ],
};

/// Descriptor for `GetFunnelResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getFunnelResponseDescriptor = $convert.base64Decode(
    'ChFHZXRGdW5uZWxSZXNwb25zZRIfCgtmdW5uZWxfbmFtZRgBIAEoCVIKZnVubmVsTmFtZRI0Cg'
    'VzdGVwcxgCIAMoCzIeLnZvaWNlLmFuYWx5dGljcy52MS5GdW5uZWxTdGVwUgVzdGVwcw==');

@$core.Deprecated('Use getRetentionRequestDescriptor instead')
const GetRetentionRequest$json = {
  '1': 'GetRetentionRequest',
  '2': [
    {
      '1': 'cohort_from',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'cohortFrom',
      '17': true
    },
    {
      '1': 'cohort_to',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'cohortTo',
      '17': true
    },
  ],
  '8': [
    {'1': '_cohort_from'},
    {'1': '_cohort_to'},
  ],
};

/// Descriptor for `GetRetentionRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRetentionRequestDescriptor = $convert.base64Decode(
    'ChNHZXRSZXRlbnRpb25SZXF1ZXN0EkAKC2NvaG9ydF9mcm9tGAEgASgLMhouZ29vZ2xlLnByb3'
    'RvYnVmLlRpbWVzdGFtcEgAUgpjb2hvcnRGcm9tiAEBEjwKCWNvaG9ydF90bxgCIAEoCzIaLmdv'
    'b2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAVIIY29ob3J0VG+IAQFCDgoMX2NvaG9ydF9mcm9tQg'
    'wKCl9jb2hvcnRfdG8=');

@$core.Deprecated('Use retentionCohortDescriptor instead')
const RetentionCohort$json = {
  '1': 'RetentionCohort',
  '2': [
    {'1': 'cohort_date', '3': 1, '4': 1, '5': 9, '10': 'cohortDate'},
    {'1': 'cohort_size', '3': 2, '4': 1, '5': 3, '10': 'cohortSize'},
    {'1': 'd1', '3': 3, '4': 1, '5': 1, '10': 'd1'},
    {'1': 'd7', '3': 4, '4': 1, '5': 1, '10': 'd7'},
    {'1': 'd30', '3': 5, '4': 1, '5': 1, '10': 'd30'},
  ],
};

/// Descriptor for `RetentionCohort`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List retentionCohortDescriptor = $convert.base64Decode(
    'Cg9SZXRlbnRpb25Db2hvcnQSHwoLY29ob3J0X2RhdGUYASABKAlSCmNvaG9ydERhdGUSHwoLY2'
    '9ob3J0X3NpemUYAiABKANSCmNvaG9ydFNpemUSDgoCZDEYAyABKAFSAmQxEg4KAmQ3GAQgASgB'
    'UgJkNxIQCgNkMzAYBSABKAFSA2QzMA==');

@$core.Deprecated('Use getRetentionResponseDescriptor instead')
const GetRetentionResponse$json = {
  '1': 'GetRetentionResponse',
  '2': [
    {
      '1': 'cohorts',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.analytics.v1.RetentionCohort',
      '10': 'cohorts'
    },
  ],
};

/// Descriptor for `GetRetentionResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getRetentionResponseDescriptor = $convert.base64Decode(
    'ChRHZXRSZXRlbnRpb25SZXNwb25zZRI9Cgdjb2hvcnRzGAEgAygLMiMudm9pY2UuYW5hbHl0aW'
    'NzLnYxLlJldGVudGlvbkNvaG9ydFIHY29ob3J0cw==');

@$core.Deprecated('Use exportDataRequestDescriptor instead')
const ExportDataRequest$json = {
  '1': 'ExportDataRequest',
  '2': [
    {
      '1': 'from',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'from',
      '17': true
    },
    {
      '1': 'to',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'to',
      '17': true
    },
    {
      '1': 'event_type',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 2,
      '10': 'eventType',
      '17': true
    },
    {'1': 'format', '3': 4, '4': 1, '5': 9, '10': 'format'},
  ],
  '8': [
    {'1': '_from'},
    {'1': '_to'},
    {'1': '_event_type'},
  ],
};

/// Descriptor for `ExportDataRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exportDataRequestDescriptor = $convert.base64Decode(
    'ChFFeHBvcnREYXRhUmVxdWVzdBIzCgRmcm9tGAEgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbW'
    'VzdGFtcEgAUgRmcm9tiAEBEi8KAnRvGAIgASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFt'
    'cEgBUgJ0b4gBARIiCgpldmVudF90eXBlGAMgASgJSAJSCWV2ZW50VHlwZYgBARIWCgZmb3JtYX'
    'QYBCABKAlSBmZvcm1hdEIHCgVfZnJvbUIFCgNfdG9CDQoLX2V2ZW50X3R5cGU=');

@$core.Deprecated('Use exportDataResponseDescriptor instead')
const ExportDataResponse$json = {
  '1': 'ExportDataResponse',
  '2': [
    {'1': 'content_type', '3': 1, '4': 1, '5': 9, '10': 'contentType'},
    {'1': 'body', '3': 2, '4': 1, '5': 12, '10': 'body'},
  ],
};

/// Descriptor for `ExportDataResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List exportDataResponseDescriptor = $convert.base64Decode(
    'ChJFeHBvcnREYXRhUmVzcG9uc2USIQoMY29udGVudF90eXBlGAEgASgJUgtjb250ZW50VHlwZR'
    'ISCgRib2R5GAIgASgMUgRib2R5');
