// This is a generated file - do not edit.
//
// Generated from voice/search/v1/search.proto.

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

@$core.Deprecated('Use searchInChatRequestDescriptor instead')
const SearchInChatRequest$json = {
  '1': 'SearchInChatRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
    {'1': 'query', '3': 2, '4': 1, '5': 9, '10': 'query'},
    {
      '1': 'page',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `SearchInChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchInChatRequestDescriptor = $convert.base64Decode(
    'ChNTZWFyY2hJbkNoYXRSZXF1ZXN0EioKBGNoYXQYASABKAsyFi52b2ljZS5jaGF0LnYxLkNoYX'
    'RSZWZSBGNoYXQSFAoFcXVlcnkYAiABKAlSBXF1ZXJ5EjYKBHBhZ2UYAyABKAsyIi52b2ljZS5j'
    'b21tb24udjEuQ3Vyc29yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use searchResultsDescriptor instead')
const SearchResults$json = {
  '1': 'SearchResults',
  '2': [
    {
      '1': 'hits',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.search.v1.SearchHit',
      '10': 'hits'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `SearchResults`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchResultsDescriptor = $convert.base64Decode(
    'Cg1TZWFyY2hSZXN1bHRzEi4KBGhpdHMYASADKAsyGi52b2ljZS5zZWFyY2gudjEuU2VhcmNoSG'
    'l0UgRoaXRzEh8KC25leHRfY3Vyc29yGAIgASgJUgpuZXh0Q3Vyc29y');

@$core.Deprecated('Use searchHitDescriptor instead')
const SearchHit$json = {
  '1': 'SearchHit',
  '2': [
    {'1': 'message_id', '3': 1, '4': 1, '5': 9, '10': 'messageId'},
    {'1': 'snippet', '3': 2, '4': 1, '5': 9, '10': 'snippet'},
    {'1': 'score', '3': 3, '4': 1, '5': 1, '10': 'score'},
    {'1': 'chat_id', '3': 4, '4': 1, '5': 9, '10': 'chatId'},
  ],
};

/// Descriptor for `SearchHit`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchHitDescriptor = $convert.base64Decode(
    'CglTZWFyY2hIaXQSHQoKbWVzc2FnZV9pZBgBIAEoCVIJbWVzc2FnZUlkEhgKB3NuaXBwZXQYAi'
    'ABKAlSB3NuaXBwZXQSFAoFc2NvcmUYAyABKAFSBXNjb3JlEhcKB2NoYXRfaWQYBCABKAlSBmNo'
    'YXRJZA==');

@$core.Deprecated('Use searchGlobalRequestDescriptor instead')
const SearchGlobalRequest$json = {
  '1': 'SearchGlobalRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
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

/// Descriptor for `SearchGlobalRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchGlobalRequestDescriptor = $convert.base64Decode(
    'ChNTZWFyY2hHbG9iYWxSZXF1ZXN0EhQKBXF1ZXJ5GAEgASgJUgVxdWVyeRI2CgRwYWdlGAIgAS'
    'gLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYWdl');

@$core.Deprecated('Use globalSearchResultsDescriptor instead')
const GlobalSearchResults$json = {
  '1': 'GlobalSearchResults',
  '2': [
    {
      '1': 'messages',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.search.v1.SearchHit',
      '10': 'messages'
    },
    {'1': 'profile_ids', '3': 2, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'matched_chats',
      '3': 3,
      '4': 3,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'matchedChats'
    },
    {'1': 'space_ids', '3': 4, '4': 3, '5': 9, '10': 'spaceIds'},
    {'1': 'next_cursor', '3': 5, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `GlobalSearchResults`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List globalSearchResultsDescriptor = $convert.base64Decode(
    'ChNHbG9iYWxTZWFyY2hSZXN1bHRzEjYKCG1lc3NhZ2VzGAEgAygLMhoudm9pY2Uuc2VhcmNoLn'
    'YxLlNlYXJjaEhpdFIIbWVzc2FnZXMSHwoLcHJvZmlsZV9pZHMYAiADKAlSCnByb2ZpbGVJZHMS'
    'OwoNbWF0Y2hlZF9jaGF0cxgDIAMoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdFJlZlIMbWF0Y2hlZE'
    'NoYXRzEhsKCXNwYWNlX2lkcxgEIAMoCVIIc3BhY2VJZHMSHwoLbmV4dF9jdXJzb3IYBSABKAlS'
    'Cm5leHRDdXJzb3I=');

@$core.Deprecated('Use searchUsersRequestDescriptor instead')
const SearchUsersRequest$json = {
  '1': 'SearchUsersRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
    {'1': 'limit', '3': 2, '4': 1, '5': 5, '10': 'limit'},
  ],
};

/// Descriptor for `SearchUsersRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchUsersRequestDescriptor = $convert.base64Decode(
    'ChJTZWFyY2hVc2Vyc1JlcXVlc3QSFAoFcXVlcnkYASABKAlSBXF1ZXJ5EhQKBWxpbWl0GAIgAS'
    'gFUgVsaW1pdA==');

@$core.Deprecated('Use userSearchResultsDescriptor instead')
const UserSearchResults$json = {
  '1': 'UserSearchResults',
  '2': [
    {'1': 'profile_ids', '3': 1, '4': 3, '5': 9, '10': 'profileIds'},
  ],
};

/// Descriptor for `UserSearchResults`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List userSearchResultsDescriptor = $convert.base64Decode(
    'ChFVc2VyU2VhcmNoUmVzdWx0cxIfCgtwcm9maWxlX2lkcxgBIAMoCVIKcHJvZmlsZUlkcw==');

@$core.Deprecated('Use searchSpacesRequestDescriptor instead')
const SearchSpacesRequest$json = {
  '1': 'SearchSpacesRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
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

/// Descriptor for `SearchSpacesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchSpacesRequestDescriptor = $convert.base64Decode(
    'ChNTZWFyY2hTcGFjZXNSZXF1ZXN0EhQKBXF1ZXJ5GAEgASgJUgVxdWVyeRI2CgRwYWdlGAIgAS'
    'gLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYWdl');

@$core.Deprecated('Use spaceSearchResultsDescriptor instead')
const SpaceSearchResults$json = {
  '1': 'SpaceSearchResults',
  '2': [
    {'1': 'space_ids', '3': 1, '4': 3, '5': 9, '10': 'spaceIds'},
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `SpaceSearchResults`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List spaceSearchResultsDescriptor = $convert.base64Decode(
    'ChJTcGFjZVNlYXJjaFJlc3VsdHMSGwoJc3BhY2VfaWRzGAEgAygJUghzcGFjZUlkcxIfCgtuZX'
    'h0X2N1cnNvchgCIAEoCVIKbmV4dEN1cnNvcg==');

@$core.Deprecated('Use reindexChatRequestDescriptor instead')
const ReindexChatRequest$json = {
  '1': 'ReindexChatRequest',
  '2': [
    {
      '1': 'chat',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.chat.v1.ChatRef',
      '10': 'chat'
    },
  ],
};

/// Descriptor for `ReindexChatRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reindexChatRequestDescriptor = $convert.base64Decode(
    'ChJSZWluZGV4Q2hhdFJlcXVlc3QSKgoEY2hhdBgBIAEoCzIWLnZvaWNlLmNoYXQudjEuQ2hhdF'
    'JlZlIEY2hhdA==');

@$core.Deprecated('Use searchInChatResponseDescriptor instead')
const SearchInChatResponse$json = {
  '1': 'SearchInChatResponse',
  '2': [
    {
      '1': 'search_results',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.search.v1.SearchResults',
      '10': 'searchResults'
    },
  ],
};

/// Descriptor for `SearchInChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchInChatResponseDescriptor = $convert.base64Decode(
    'ChRTZWFyY2hJbkNoYXRSZXNwb25zZRJFCg5zZWFyY2hfcmVzdWx0cxgBIAEoCzIeLnZvaWNlLn'
    'NlYXJjaC52MS5TZWFyY2hSZXN1bHRzUg1zZWFyY2hSZXN1bHRz');

@$core.Deprecated('Use searchGlobalResponseDescriptor instead')
const SearchGlobalResponse$json = {
  '1': 'SearchGlobalResponse',
  '2': [
    {
      '1': 'global_search_results',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.search.v1.GlobalSearchResults',
      '10': 'globalSearchResults'
    },
  ],
};

/// Descriptor for `SearchGlobalResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchGlobalResponseDescriptor = $convert.base64Decode(
    'ChRTZWFyY2hHbG9iYWxSZXNwb25zZRJYChVnbG9iYWxfc2VhcmNoX3Jlc3VsdHMYASABKAsyJC'
    '52b2ljZS5zZWFyY2gudjEuR2xvYmFsU2VhcmNoUmVzdWx0c1ITZ2xvYmFsU2VhcmNoUmVzdWx0'
    'cw==');

@$core.Deprecated('Use searchUsersResponseDescriptor instead')
const SearchUsersResponse$json = {
  '1': 'SearchUsersResponse',
  '2': [
    {
      '1': 'user_search_results',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.search.v1.UserSearchResults',
      '10': 'userSearchResults'
    },
  ],
};

/// Descriptor for `SearchUsersResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchUsersResponseDescriptor = $convert.base64Decode(
    'ChNTZWFyY2hVc2Vyc1Jlc3BvbnNlElIKE3VzZXJfc2VhcmNoX3Jlc3VsdHMYASABKAsyIi52b2'
    'ljZS5zZWFyY2gudjEuVXNlclNlYXJjaFJlc3VsdHNSEXVzZXJTZWFyY2hSZXN1bHRz');

@$core.Deprecated('Use searchSpacesResponseDescriptor instead')
const SearchSpacesResponse$json = {
  '1': 'SearchSpacesResponse',
  '2': [
    {
      '1': 'space_search_results',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.search.v1.SpaceSearchResults',
      '10': 'spaceSearchResults'
    },
  ],
};

/// Descriptor for `SearchSpacesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchSpacesResponseDescriptor = $convert.base64Decode(
    'ChRTZWFyY2hTcGFjZXNSZXNwb25zZRJVChRzcGFjZV9zZWFyY2hfcmVzdWx0cxgBIAEoCzIjLn'
    'ZvaWNlLnNlYXJjaC52MS5TcGFjZVNlYXJjaFJlc3VsdHNSEnNwYWNlU2VhcmNoUmVzdWx0cw==');

@$core.Deprecated('Use reindexChatResponseDescriptor instead')
const ReindexChatResponse$json = {
  '1': 'ReindexChatResponse',
};

/// Descriptor for `ReindexChatResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List reindexChatResponseDescriptor =
    $convert.base64Decode('ChNSZWluZGV4Q2hhdFJlc3BvbnNl');
