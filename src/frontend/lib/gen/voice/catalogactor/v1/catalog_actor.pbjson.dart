// This is a generated file - do not edit.
//
// Generated from voice/catalogactor/v1/catalog_actor.proto.

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

@$core.Deprecated('Use catalogActorStateDescriptor instead')
const CatalogActorState$json = {
  '1': 'CatalogActorState',
  '2': [
    {'1': 'CATALOG_ACTOR_STATE_UNSPECIFIED', '2': 0},
    {'1': 'CATALOG_ACTOR_STATE_UNKNOWN', '2': 1},
    {'1': 'CATALOG_ACTOR_STATE_NOT_DELETED', '2': 2},
    {'1': 'CATALOG_ACTOR_STATE_SOFT_DELETED', '2': 3},
  ],
};

/// Descriptor for `CatalogActorState`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List catalogActorStateDescriptor = $convert.base64Decode(
    'ChFDYXRhbG9nQWN0b3JTdGF0ZRIjCh9DQVRBTE9HX0FDVE9SX1NUQVRFX1VOU1BFQ0lGSUVEEA'
    'ASHwobQ0FUQUxPR19BQ1RPUl9TVEFURV9VTktOT1dOEAESIwofQ0FUQUxPR19BQ1RPUl9TVEFU'
    'RV9OT1RfREVMRVRFRBACEiQKIENBVEFMT0dfQUNUT1JfU1RBVEVfU09GVF9ERUxFVEVEEAM=');

@$core.Deprecated('Use catalogActorDescriptor instead')
const CatalogActor$json = {
  '1': 'CatalogActor',
  '2': [
    {'1': 'account_id', '3': 1, '4': 1, '5': 9, '10': 'accountId'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `CatalogActor`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List catalogActorDescriptor = $convert.base64Decode(
    'CgxDYXRhbG9nQWN0b3ISHQoKYWNjb3VudF9pZBgBIAEoCVIJYWNjb3VudElkEh0KCnByb2ZpbG'
    'VfaWQYAiABKAlSCXByb2ZpbGVJZA==');

@$core.Deprecated('Use getCatalogActorStatesRequestDescriptor instead')
const GetCatalogActorStatesRequest$json = {
  '1': 'GetCatalogActorStatesRequest',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'actors',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.catalogactor.v1.CatalogActor',
      '10': 'actors'
    },
  ],
};

/// Descriptor for `GetCatalogActorStatesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCatalogActorStatesRequestDescriptor =
    $convert.base64Decode(
        'ChxHZXRDYXRhbG9nQWN0b3JTdGF0ZXNSZXF1ZXN0EikKEHByb3RvY29sX3ZlcnNpb24YASABKA'
        '1SD3Byb3RvY29sVmVyc2lvbhI7CgZhY3RvcnMYAiADKAsyIy52b2ljZS5jYXRhbG9nYWN0b3Iu'
        'djEuQ2F0YWxvZ0FjdG9yUgZhY3RvcnM=');

@$core.Deprecated('Use catalogActorStateEntryDescriptor instead')
const CatalogActorStateEntry$json = {
  '1': 'CatalogActorStateEntry',
  '2': [
    {
      '1': 'actor',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.catalogactor.v1.CatalogActor',
      '10': 'actor'
    },
    {
      '1': 'state',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.voice.catalogactor.v1.CatalogActorState',
      '10': 'state'
    },
  ],
};

/// Descriptor for `CatalogActorStateEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List catalogActorStateEntryDescriptor = $convert.base64Decode(
    'ChZDYXRhbG9nQWN0b3JTdGF0ZUVudHJ5EjkKBWFjdG9yGAEgASgLMiMudm9pY2UuY2F0YWxvZ2'
    'FjdG9yLnYxLkNhdGFsb2dBY3RvclIFYWN0b3ISPgoFc3RhdGUYAiABKA4yKC52b2ljZS5jYXRh'
    'bG9nYWN0b3IudjEuQ2F0YWxvZ0FjdG9yU3RhdGVSBXN0YXRl');

@$core.Deprecated('Use getCatalogActorStatesResponseDescriptor instead')
const GetCatalogActorStatesResponse$json = {
  '1': 'GetCatalogActorStatesResponse',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {
      '1': 'states',
      '3': 2,
      '4': 3,
      '5': 11,
      '6': '.voice.catalogactor.v1.CatalogActorStateEntry',
      '10': 'states'
    },
  ],
};

/// Descriptor for `GetCatalogActorStatesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getCatalogActorStatesResponseDescriptor =
    $convert.base64Decode(
        'Ch1HZXRDYXRhbG9nQWN0b3JTdGF0ZXNSZXNwb25zZRIpChBwcm90b2NvbF92ZXJzaW9uGAEgAS'
        'gNUg9wcm90b2NvbFZlcnNpb24SRQoGc3RhdGVzGAIgAygLMi0udm9pY2UuY2F0YWxvZ2FjdG9y'
        'LnYxLkNhdGFsb2dBY3RvclN0YXRlRW50cnlSBnN0YXRlcw==');
