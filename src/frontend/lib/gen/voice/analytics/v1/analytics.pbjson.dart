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
