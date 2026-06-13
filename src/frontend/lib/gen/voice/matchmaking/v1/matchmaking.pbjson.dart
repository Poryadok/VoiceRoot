// This is a generated file - do not edit.
//
// Generated from voice/matchmaking/v1/matchmaking.proto.

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

@$core.Deprecated('Use gameDescriptor instead')
const Game$json = {
  '1': 'Game',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '10': 'name'},
    {
      '1': 'icon_url',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'iconUrl',
      '17': true
    },
    {
      '1': 'external_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'externalId',
      '17': true
    },
    {'1': 'config_json', '3': 5, '4': 1, '5': 9, '10': 'configJson'},
    {'1': 'status', '3': 6, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'created_by_profile_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '10': 'createdByProfileId'
    },
    {
      '1': 'created_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
  ],
  '8': [
    {'1': '_icon_url'},
    {'1': '_external_id'},
  ],
};

/// Descriptor for `Game`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameDescriptor = $convert.base64Decode(
    'CgRHYW1lEg4KAmlkGAEgASgJUgJpZBISCgRuYW1lGAIgASgJUgRuYW1lEh4KCGljb25fdXJsGA'
    'MgASgJSABSB2ljb25VcmyIAQESJAoLZXh0ZXJuYWxfaWQYBCABKAlIAVIKZXh0ZXJuYWxJZIgB'
    'ARIfCgtjb25maWdfanNvbhgFIAEoCVIKY29uZmlnSnNvbhIWCgZzdGF0dXMYBiABKAlSBnN0YX'
    'R1cxIxChVjcmVhdGVkX2J5X3Byb2ZpbGVfaWQYByABKAlSEmNyZWF0ZWRCeVByb2ZpbGVJZBI5'
    'CgpjcmVhdGVkX2F0GAggASgLMhouZ29vZ2xlLnByb3RvYnVmLlRpbWVzdGFtcFIJY3JlYXRlZE'
    'F0QgsKCV9pY29uX3VybEIOCgxfZXh0ZXJuYWxfaWQ=');

@$core.Deprecated('Use listGamesRequestDescriptor instead')
const ListGamesRequest$json = {
  '1': 'ListGamesRequest',
  '2': [
    {
      '1': 'page',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.common.v1.CursorPageRequest',
      '10': 'page'
    },
  ],
};

/// Descriptor for `ListGamesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listGamesRequestDescriptor = $convert.base64Decode(
    'ChBMaXN0R2FtZXNSZXF1ZXN0EjYKBHBhZ2UYASABKAsyIi52b2ljZS5jb21tb24udjEuQ3Vyc2'
    '9yUGFnZVJlcXVlc3RSBHBhZ2U=');

@$core.Deprecated('Use gameListDescriptor instead')
const GameList$json = {
  '1': 'GameList',
  '2': [
    {
      '1': 'games',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.matchmaking.v1.Game',
      '10': 'games'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `GameList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List gameListDescriptor = $convert.base64Decode(
    'CghHYW1lTGlzdBIwCgVnYW1lcxgBIAMoCzIaLnZvaWNlLm1hdGNobWFraW5nLnYxLkdhbWVSBW'
    'dhbWVzEh8KC25leHRfY3Vyc29yGAIgASgJUgpuZXh0Q3Vyc29y');

@$core.Deprecated('Use getGameRequestDescriptor instead')
const GetGameRequest$json = {
  '1': 'GetGameRequest',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
  ],
};

/// Descriptor for `GetGameRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getGameRequestDescriptor = $convert
    .base64Decode('Cg5HZXRHYW1lUmVxdWVzdBIXCgdnYW1lX2lkGAEgASgJUgZnYW1lSWQ=');

@$core.Deprecated('Use createGameRequestDescriptor instead')
const CreateGameRequest$json = {
  '1': 'CreateGameRequest',
  '2': [
    {'1': 'name', '3': 1, '4': 1, '5': 9, '10': 'name'},
    {'1': 'config_json', '3': 2, '4': 1, '5': 9, '10': 'configJson'},
  ],
};

/// Descriptor for `CreateGameRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createGameRequestDescriptor = $convert.base64Decode(
    'ChFDcmVhdGVHYW1lUmVxdWVzdBISCgRuYW1lGAEgASgJUgRuYW1lEh8KC2NvbmZpZ19qc29uGA'
    'IgASgJUgpjb25maWdKc29u');

@$core.Deprecated('Use updateGameRequestDescriptor instead')
const UpdateGameRequest$json = {
  '1': 'UpdateGameRequest',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'name', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'name', '17': true},
    {
      '1': 'config_json',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'configJson',
      '17': true
    },
    {'1': 'status', '3': 4, '4': 1, '5': 9, '9': 2, '10': 'status', '17': true},
  ],
  '8': [
    {'1': '_name'},
    {'1': '_config_json'},
    {'1': '_status'},
  ],
};

/// Descriptor for `UpdateGameRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateGameRequestDescriptor = $convert.base64Decode(
    'ChFVcGRhdGVHYW1lUmVxdWVzdBIXCgdnYW1lX2lkGAEgASgJUgZnYW1lSWQSFwoEbmFtZRgCIA'
    'EoCUgAUgRuYW1liAEBEiQKC2NvbmZpZ19qc29uGAMgASgJSAFSCmNvbmZpZ0pzb26IAQESGwoG'
    'c3RhdHVzGAQgASgJSAJSBnN0YXR1c4gBAUIHCgVfbmFtZUIOCgxfY29uZmlnX2pzb25CCQoHX3'
    'N0YXR1cw==');

@$core.Deprecated('Use searchGamesRequestDescriptor instead')
const SearchGamesRequest$json = {
  '1': 'SearchGamesRequest',
  '2': [
    {'1': 'query', '3': 1, '4': 1, '5': 9, '10': 'query'},
  ],
};

/// Descriptor for `SearchGamesRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchGamesRequestDescriptor = $convert
    .base64Decode('ChJTZWFyY2hHYW1lc1JlcXVlc3QSFAoFcXVlcnkYASABKAlSBXF1ZXJ5');

@$core.Deprecated('Use startSearchRequestDescriptor instead')
const StartSearchRequest$json = {
  '1': 'StartSearchRequest',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 2, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'criteria_json', '3': 3, '4': 1, '5': 9, '10': 'criteriaJson'},
    {
      '1': 'party_id',
      '3': 4,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'partyId',
      '17': true
    },
  ],
  '8': [
    {'1': '_party_id'},
  ],
};

/// Descriptor for `StartSearchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startSearchRequestDescriptor = $convert.base64Decode(
    'ChJTdGFydFNlYXJjaFJlcXVlc3QSFwoHZ2FtZV9pZBgBIAEoCVIGZ2FtZUlkEhIKBG1vZGUYAi'
    'ABKAlSBG1vZGUSIwoNY3JpdGVyaWFfanNvbhgDIAEoCVIMY3JpdGVyaWFKc29uEh4KCHBhcnR5'
    'X2lkGAQgASgJSABSB3BhcnR5SWSIAQFCCwoJX3BhcnR5X2lk');

@$core.Deprecated('Use searchSessionDescriptor instead')
const SearchSession$json = {
  '1': 'SearchSession',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'profile_id', '3': 2, '4': 1, '5': 9, '10': 'profileId'},
    {
      '1': 'party_id',
      '3': 3,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'partyId',
      '17': true
    },
    {'1': 'game_id', '3': 4, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 5, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'criteria_json', '3': 6, '4': 1, '5': 9, '10': 'criteriaJson'},
    {'1': 'status', '3': 7, '4': 1, '5': 9, '10': 'status'},
    {
      '1': 'timeout_at',
      '3': 8,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 1,
      '10': 'timeoutAt',
      '17': true
    },
    {
      '1': 'matched_at',
      '3': 9,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 2,
      '10': 'matchedAt',
      '17': true
    },
    {
      '1': 'match_id',
      '3': 10,
      '4': 1,
      '5': 9,
      '9': 3,
      '10': 'matchId',
      '17': true
    },
  ],
  '8': [
    {'1': '_party_id'},
    {'1': '_timeout_at'},
    {'1': '_matched_at'},
    {'1': '_match_id'},
  ],
};

/// Descriptor for `SearchSession`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchSessionDescriptor = $convert.base64Decode(
    'Cg1TZWFyY2hTZXNzaW9uEg4KAmlkGAEgASgJUgJpZBIdCgpwcm9maWxlX2lkGAIgASgJUglwcm'
    '9maWxlSWQSHgoIcGFydHlfaWQYAyABKAlIAFIHcGFydHlJZIgBARIXCgdnYW1lX2lkGAQgASgJ'
    'UgZnYW1lSWQSEgoEbW9kZRgFIAEoCVIEbW9kZRIjCg1jcml0ZXJpYV9qc29uGAYgASgJUgxjcm'
    'l0ZXJpYUpzb24SFgoGc3RhdHVzGAcgASgJUgZzdGF0dXMSPgoKdGltZW91dF9hdBgIIAEoCzIa'
    'Lmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAVIJdGltZW91dEF0iAEBEj4KCm1hdGNoZWRfYX'
    'QYCSABKAsyGi5nb29nbGUucHJvdG9idWYuVGltZXN0YW1wSAJSCW1hdGNoZWRBdIgBARIeCght'
    'YXRjaF9pZBgKIAEoCUgDUgdtYXRjaElkiAEBQgsKCV9wYXJ0eV9pZEINCgtfdGltZW91dF9hdE'
    'INCgtfbWF0Y2hlZF9hdEILCglfbWF0Y2hfaWQ=');

@$core.Deprecated('Use cancelSearchRequestDescriptor instead')
const CancelSearchRequest$json = {
  '1': 'CancelSearchRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `CancelSearchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelSearchRequestDescriptor = $convert.base64Decode(
    'ChNDYW5jZWxTZWFyY2hSZXF1ZXN0Eh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZA==');

@$core.Deprecated('Use getSearchStatusRequestDescriptor instead')
const GetSearchStatusRequest$json = {
  '1': 'GetSearchStatusRequest',
  '2': [
    {'1': 'session_id', '3': 1, '4': 1, '5': 9, '10': 'sessionId'},
  ],
};

/// Descriptor for `GetSearchStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSearchStatusRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRTZWFyY2hTdGF0dXNSZXF1ZXN0Eh0KCnNlc3Npb25faWQYASABKAlSCXNlc3Npb25JZA'
        '==');

@$core.Deprecated('Use matchDescriptor instead')
const Match$json = {
  '1': 'Match',
  '2': [
    {'1': 'id', '3': 1, '4': 1, '5': 9, '10': 'id'},
    {'1': 'game_id', '3': 2, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'mode', '3': 3, '4': 1, '5': 9, '10': 'mode'},
    {'1': 'profile_ids', '3': 4, '4': 3, '5': 9, '10': 'profileIds'},
    {
      '1': 'created_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'createdAt'
    },
    {
      '1': 'voice_room_id',
      '3': 6,
      '4': 1,
      '5': 9,
      '9': 0,
      '10': 'voiceRoomId',
      '17': true
    },
    {
      '1': 'chat_id',
      '3': 7,
      '4': 1,
      '5': 9,
      '9': 1,
      '10': 'chatId',
      '17': true
    },
    {'1': 'status', '3': 8, '4': 1, '5': 9, '10': 'status'},
    {'1': 'region', '3': 9, '4': 1, '5': 9, '10': 'region'},
  ],
  '8': [
    {'1': '_voice_room_id'},
    {'1': '_chat_id'},
  ],
};

/// Descriptor for `Match`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchDescriptor = $convert.base64Decode(
    'CgVNYXRjaBIOCgJpZBgBIAEoCVICaWQSFwoHZ2FtZV9pZBgCIAEoCVIGZ2FtZUlkEhIKBG1vZG'
    'UYAyABKAlSBG1vZGUSHwoLcHJvZmlsZV9pZHMYBCADKAlSCnByb2ZpbGVJZHMSOQoKY3JlYXRl'
    'ZF9hdBgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCWNyZWF0ZWRBdBInCg12b2'
    'ljZV9yb29tX2lkGAYgASgJSABSC3ZvaWNlUm9vbUlkiAEBEhwKB2NoYXRfaWQYByABKAlIAVIG'
    'Y2hhdElkiAEBEhYKBnN0YXR1cxgIIAEoCVIGc3RhdHVzEhYKBnJlZ2lvbhgJIAEoCVIGcmVnaW'
    '9uQhAKDl92b2ljZV9yb29tX2lkQgoKCF9jaGF0X2lk');

@$core.Deprecated('Use getMatchRequestDescriptor instead')
const GetMatchRequest$json = {
  '1': 'GetMatchRequest',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
  ],
};

/// Descriptor for `GetMatchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMatchRequestDescriptor = $convert.base64Decode(
    'Cg9HZXRNYXRjaFJlcXVlc3QSGQoIbWF0Y2hfaWQYASABKAlSB21hdGNoSWQ=');

@$core.Deprecated('Use respondToMatchRequestDescriptor instead')
const RespondToMatchRequest$json = {
  '1': 'RespondToMatchRequest',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
    {'1': 'accept', '3': 2, '4': 1, '5': 8, '10': 'accept'},
  ],
};

/// Descriptor for `RespondToMatchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List respondToMatchRequestDescriptor = $convert.base64Decode(
    'ChVSZXNwb25kVG9NYXRjaFJlcXVlc3QSGQoIbWF0Y2hfaWQYASABKAlSB21hdGNoSWQSFgoGYW'
    'NjZXB0GAIgASgIUgZhY2NlcHQ=');

@$core.Deprecated('Use respondToMatchResponseDescriptor instead')
const RespondToMatchResponse$json = {
  '1': 'RespondToMatchResponse',
  '2': [
    {
      '1': 'match',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Match',
      '10': 'match'
    },
    {
      '1': 'search_session',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.SearchSession',
      '10': 'searchSession'
    },
  ],
};

/// Descriptor for `RespondToMatchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List respondToMatchResponseDescriptor = $convert.base64Decode(
    'ChZSZXNwb25kVG9NYXRjaFJlc3BvbnNlEjEKBW1hdGNoGAEgASgLMhsudm9pY2UubWF0Y2htYW'
    'tpbmcudjEuTWF0Y2hSBW1hdGNoEkoKDnNlYXJjaF9zZXNzaW9uGAIgASgLMiMudm9pY2UubWF0'
    'Y2htYWtpbmcudjEuU2VhcmNoU2Vzc2lvblINc2VhcmNoU2Vzc2lvbg==');

@$core.Deprecated('Use getMatchHistoryRequestDescriptor instead')
const GetMatchHistoryRequest$json = {
  '1': 'GetMatchHistoryRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
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

/// Descriptor for `GetMatchHistoryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMatchHistoryRequestDescriptor = $convert.base64Decode(
    'ChZHZXRNYXRjaEhpc3RvcnlSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZB'
    'I2CgRwYWdlGAIgASgLMiIudm9pY2UuY29tbW9uLnYxLkN1cnNvclBhZ2VSZXF1ZXN0UgRwYWdl');

@$core.Deprecated('Use matchListDescriptor instead')
const MatchList$json = {
  '1': 'MatchList',
  '2': [
    {
      '1': 'matches',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.matchmaking.v1.Match',
      '10': 'matches'
    },
    {'1': 'next_cursor', '3': 2, '4': 1, '5': 9, '10': 'nextCursor'},
  ],
};

/// Descriptor for `MatchList`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List matchListDescriptor = $convert.base64Decode(
    'CglNYXRjaExpc3QSNQoHbWF0Y2hlcxgBIAMoCzIbLnZvaWNlLm1hdGNobWFraW5nLnYxLk1hdG'
    'NoUgdtYXRjaGVzEh8KC25leHRfY3Vyc29yGAIgASgJUgpuZXh0Q3Vyc29y');

@$core.Deprecated('Use completeMatchRequestDescriptor instead')
const CompleteMatchRequest$json = {
  '1': 'CompleteMatchRequest',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
  ],
};

/// Descriptor for `CompleteMatchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeMatchRequestDescriptor =
    $convert.base64Decode(
        'ChRDb21wbGV0ZU1hdGNoUmVxdWVzdBIZCghtYXRjaF9pZBgBIAEoCVIHbWF0Y2hJZA==');

@$core.Deprecated('Use completeMatchResponseDescriptor instead')
const CompleteMatchResponse$json = {
  '1': 'CompleteMatchResponse',
  '2': [
    {
      '1': 'match',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Match',
      '10': 'match'
    },
  ],
};

/// Descriptor for `CompleteMatchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List completeMatchResponseDescriptor = $convert.base64Decode(
    'ChVDb21wbGV0ZU1hdGNoUmVzcG9uc2USMQoFbWF0Y2gYASABKAsyGy52b2ljZS5tYXRjaG1ha2'
    'luZy52MS5NYXRjaFIFbWF0Y2g=');

@$core.Deprecated('Use rateMatchRequestDescriptor instead')
const RateMatchRequest$json = {
  '1': 'RateMatchRequest',
  '2': [
    {'1': 'match_id', '3': 1, '4': 1, '5': 9, '10': 'matchId'},
    {'1': 'rated_profile_id', '3': 2, '4': 1, '5': 9, '10': 'ratedProfileId'},
    {'1': 'stars', '3': 3, '4': 1, '5': 5, '10': 'stars'},
  ],
};

/// Descriptor for `RateMatchRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rateMatchRequestDescriptor = $convert.base64Decode(
    'ChBSYXRlTWF0Y2hSZXF1ZXN0EhkKCG1hdGNoX2lkGAEgASgJUgdtYXRjaElkEigKEHJhdGVkX3'
    'Byb2ZpbGVfaWQYAiABKAlSDnJhdGVkUHJvZmlsZUlkEhQKBXN0YXJzGAMgASgFUgVzdGFycw==');

@$core.Deprecated('Use getPlayerRatingRequestDescriptor instead')
const GetPlayerRatingRequest$json = {
  '1': 'GetPlayerRatingRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'game_id', '3': 2, '4': 1, '5': 9, '10': 'gameId'},
  ],
};

/// Descriptor for `GetPlayerRatingRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPlayerRatingRequestDescriptor =
    $convert.base64Decode(
        'ChZHZXRQbGF5ZXJSYXRpbmdSZXF1ZXN0Eh0KCnByb2ZpbGVfaWQYASABKAlSCXByb2ZpbGVJZB'
        'IXCgdnYW1lX2lkGAIgASgJUgZnYW1lSWQ=');

@$core.Deprecated('Use playerRatingDescriptor instead')
const PlayerRating$json = {
  '1': 'PlayerRating',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
    {'1': 'game_id', '3': 2, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'rating_value', '3': 3, '4': 1, '5': 1, '10': 'ratingValue'},
    {'1': 'games_played', '3': 4, '4': 1, '5': 5, '10': 'gamesPlayed'},
  ],
};

/// Descriptor for `PlayerRating`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List playerRatingDescriptor = $convert.base64Decode(
    'CgxQbGF5ZXJSYXRpbmcSHQoKcHJvZmlsZV9pZBgBIAEoCVIJcHJvZmlsZUlkEhcKB2dhbWVfaW'
    'QYAiABKAlSBmdhbWVJZBIhCgxyYXRpbmdfdmFsdWUYAyABKAFSC3JhdGluZ1ZhbHVlEiEKDGdh'
    'bWVzX3BsYXllZBgEIAEoBVILZ2FtZXNQbGF5ZWQ=');

@$core.Deprecated('Use banFromMMRequestDescriptor instead')
const BanFromMMRequest$json = {
  '1': 'BanFromMMRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
    {'1': 'reason', '3': 2, '4': 1, '5': 9, '9': 0, '10': 'reason', '17': true},
  ],
  '8': [
    {'1': '_reason'},
  ],
};

/// Descriptor for `BanFromMMRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banFromMMRequestDescriptor = $convert.base64Decode(
    'ChBCYW5Gcm9tTU1SZXF1ZXN0EioKEXRhcmdldF9wcm9maWxlX2lkGAEgASgJUg90YXJnZXRQcm'
    '9maWxlSWQSGwoGcmVhc29uGAIgASgJSABSBnJlYXNvbogBAUIJCgdfcmVhc29u');

@$core.Deprecated('Use unbanFromMMRequestDescriptor instead')
const UnbanFromMMRequest$json = {
  '1': 'UnbanFromMMRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
  ],
};

/// Descriptor for `UnbanFromMMRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unbanFromMMRequestDescriptor = $convert.base64Decode(
    'ChJVbmJhbkZyb21NTVJlcXVlc3QSKgoRdGFyZ2V0X3Byb2ZpbGVfaWQYASABKAlSD3RhcmdldF'
    'Byb2ZpbGVJZA==');

@$core.Deprecated('Use getMMBanStatusRequestDescriptor instead')
const GetMMBanStatusRequest$json = {
  '1': 'GetMMBanStatusRequest',
  '2': [
    {'1': 'target_profile_id', '3': 1, '4': 1, '5': 9, '10': 'targetProfileId'},
  ],
};

/// Descriptor for `GetMMBanStatusRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMMBanStatusRequestDescriptor = $convert.base64Decode(
    'ChVHZXRNTUJhblN0YXR1c1JlcXVlc3QSKgoRdGFyZ2V0X3Byb2ZpbGVfaWQYASABKAlSD3Rhcm'
    'dldFByb2ZpbGVJZA==');

@$core.Deprecated('Use mMBanStatusDescriptor instead')
const MMBanStatus$json = {
  '1': 'MMBanStatus',
  '2': [
    {'1': 'banned', '3': 1, '4': 1, '5': 8, '10': 'banned'},
    {
      '1': 'until',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '9': 0,
      '10': 'until',
      '17': true
    },
  ],
  '8': [
    {'1': '_until'},
  ],
};

/// Descriptor for `MMBanStatus`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List mMBanStatusDescriptor = $convert.base64Decode(
    'CgtNTUJhblN0YXR1cxIWCgZiYW5uZWQYASABKAhSBmJhbm5lZBI1CgV1bnRpbBgCIAEoCzIaLm'
    'dvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBIAFIFdW50aWyIAQFCCAoGX3VudGls');

@$core.Deprecated('Use listGamesResponseDescriptor instead')
const ListGamesResponse$json = {
  '1': 'ListGamesResponse',
  '2': [
    {
      '1': 'game_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.GameList',
      '10': 'gameList'
    },
  ],
};

/// Descriptor for `ListGamesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List listGamesResponseDescriptor = $convert.base64Decode(
    'ChFMaXN0R2FtZXNSZXNwb25zZRI7CglnYW1lX2xpc3QYASABKAsyHi52b2ljZS5tYXRjaG1ha2'
    'luZy52MS5HYW1lTGlzdFIIZ2FtZUxpc3Q=');

@$core.Deprecated('Use getGameResponseDescriptor instead')
const GetGameResponse$json = {
  '1': 'GetGameResponse',
  '2': [
    {
      '1': 'game',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Game',
      '10': 'game'
    },
  ],
};

/// Descriptor for `GetGameResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getGameResponseDescriptor = $convert.base64Decode(
    'Cg9HZXRHYW1lUmVzcG9uc2USLgoEZ2FtZRgBIAEoCzIaLnZvaWNlLm1hdGNobWFraW5nLnYxLk'
    'dhbWVSBGdhbWU=');

@$core.Deprecated('Use createGameResponseDescriptor instead')
const CreateGameResponse$json = {
  '1': 'CreateGameResponse',
  '2': [
    {
      '1': 'game',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Game',
      '10': 'game'
    },
  ],
};

/// Descriptor for `CreateGameResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List createGameResponseDescriptor = $convert.base64Decode(
    'ChJDcmVhdGVHYW1lUmVzcG9uc2USLgoEZ2FtZRgBIAEoCzIaLnZvaWNlLm1hdGNobWFraW5nLn'
    'YxLkdhbWVSBGdhbWU=');

@$core.Deprecated('Use updateGameResponseDescriptor instead')
const UpdateGameResponse$json = {
  '1': 'UpdateGameResponse',
  '2': [
    {
      '1': 'game',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Game',
      '10': 'game'
    },
  ],
};

/// Descriptor for `UpdateGameResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List updateGameResponseDescriptor = $convert.base64Decode(
    'ChJVcGRhdGVHYW1lUmVzcG9uc2USLgoEZ2FtZRgBIAEoCzIaLnZvaWNlLm1hdGNobWFraW5nLn'
    'YxLkdhbWVSBGdhbWU=');

@$core.Deprecated('Use searchGamesResponseDescriptor instead')
const SearchGamesResponse$json = {
  '1': 'SearchGamesResponse',
  '2': [
    {
      '1': 'game_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.GameList',
      '10': 'gameList'
    },
  ],
};

/// Descriptor for `SearchGamesResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List searchGamesResponseDescriptor = $convert.base64Decode(
    'ChNTZWFyY2hHYW1lc1Jlc3BvbnNlEjsKCWdhbWVfbGlzdBgBIAEoCzIeLnZvaWNlLm1hdGNobW'
    'FraW5nLnYxLkdhbWVMaXN0UghnYW1lTGlzdA==');

@$core.Deprecated('Use startSearchResponseDescriptor instead')
const StartSearchResponse$json = {
  '1': 'StartSearchResponse',
  '2': [
    {
      '1': 'search_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.SearchSession',
      '10': 'searchSession'
    },
  ],
};

/// Descriptor for `StartSearchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List startSearchResponseDescriptor = $convert.base64Decode(
    'ChNTdGFydFNlYXJjaFJlc3BvbnNlEkoKDnNlYXJjaF9zZXNzaW9uGAEgASgLMiMudm9pY2UubW'
    'F0Y2htYWtpbmcudjEuU2VhcmNoU2Vzc2lvblINc2VhcmNoU2Vzc2lvbg==');

@$core.Deprecated('Use cancelSearchResponseDescriptor instead')
const CancelSearchResponse$json = {
  '1': 'CancelSearchResponse',
};

/// Descriptor for `CancelSearchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cancelSearchResponseDescriptor =
    $convert.base64Decode('ChRDYW5jZWxTZWFyY2hSZXNwb25zZQ==');

@$core.Deprecated('Use getSearchStatusResponseDescriptor instead')
const GetSearchStatusResponse$json = {
  '1': 'GetSearchStatusResponse',
  '2': [
    {
      '1': 'search_session',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.SearchSession',
      '10': 'searchSession'
    },
  ],
};

/// Descriptor for `GetSearchStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getSearchStatusResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRTZWFyY2hTdGF0dXNSZXNwb25zZRJKCg5zZWFyY2hfc2Vzc2lvbhgBIAEoCzIjLnZvaW'
        'NlLm1hdGNobWFraW5nLnYxLlNlYXJjaFNlc3Npb25SDXNlYXJjaFNlc3Npb24=');

@$core.Deprecated('Use getMatchResponseDescriptor instead')
const GetMatchResponse$json = {
  '1': 'GetMatchResponse',
  '2': [
    {
      '1': 'match',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.Match',
      '10': 'match'
    },
  ],
};

/// Descriptor for `GetMatchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMatchResponseDescriptor = $convert.base64Decode(
    'ChBHZXRNYXRjaFJlc3BvbnNlEjEKBW1hdGNoGAEgASgLMhsudm9pY2UubWF0Y2htYWtpbmcudj'
    'EuTWF0Y2hSBW1hdGNo');

@$core.Deprecated('Use getMatchHistoryResponseDescriptor instead')
const GetMatchHistoryResponse$json = {
  '1': 'GetMatchHistoryResponse',
  '2': [
    {
      '1': 'match_list',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.MatchList',
      '10': 'matchList'
    },
  ],
};

/// Descriptor for `GetMatchHistoryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMatchHistoryResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRNYXRjaEhpc3RvcnlSZXNwb25zZRI+CgptYXRjaF9saXN0GAEgASgLMh8udm9pY2UubW'
        'F0Y2htYWtpbmcudjEuTWF0Y2hMaXN0UgltYXRjaExpc3Q=');

@$core.Deprecated('Use rateMatchResponseDescriptor instead')
const RateMatchResponse$json = {
  '1': 'RateMatchResponse',
};

/// Descriptor for `RateMatchResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List rateMatchResponseDescriptor =
    $convert.base64Decode('ChFSYXRlTWF0Y2hSZXNwb25zZQ==');

@$core.Deprecated('Use getPlayerRatingResponseDescriptor instead')
const GetPlayerRatingResponse$json = {
  '1': 'GetPlayerRatingResponse',
  '2': [
    {
      '1': 'player_rating',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.PlayerRating',
      '10': 'playerRating'
    },
  ],
};

/// Descriptor for `GetPlayerRatingResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPlayerRatingResponseDescriptor =
    $convert.base64Decode(
        'ChdHZXRQbGF5ZXJSYXRpbmdSZXNwb25zZRJHCg1wbGF5ZXJfcmF0aW5nGAEgASgLMiIudm9pY2'
        'UubWF0Y2htYWtpbmcudjEuUGxheWVyUmF0aW5nUgxwbGF5ZXJSYXRpbmc=');

@$core.Deprecated('Use banFromMMResponseDescriptor instead')
const BanFromMMResponse$json = {
  '1': 'BanFromMMResponse',
};

/// Descriptor for `BanFromMMResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List banFromMMResponseDescriptor =
    $convert.base64Decode('ChFCYW5Gcm9tTU1SZXNwb25zZQ==');

@$core.Deprecated('Use unbanFromMMResponseDescriptor instead')
const UnbanFromMMResponse$json = {
  '1': 'UnbanFromMMResponse',
};

/// Descriptor for `UnbanFromMMResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List unbanFromMMResponseDescriptor =
    $convert.base64Decode('ChNVbmJhbkZyb21NTVJlc3BvbnNl');

@$core.Deprecated('Use getMMBanStatusResponseDescriptor instead')
const GetMMBanStatusResponse$json = {
  '1': 'GetMMBanStatusResponse',
  '2': [
    {
      '1': 'mm_ban_status',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.MMBanStatus',
      '10': 'mmBanStatus'
    },
  ],
};

/// Descriptor for `GetMMBanStatusResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMMBanStatusResponseDescriptor =
    $convert.base64Decode(
        'ChZHZXRNTUJhblN0YXR1c1Jlc3BvbnNlEkUKDW1tX2Jhbl9zdGF0dXMYASABKAsyIS52b2ljZS'
        '5tYXRjaG1ha2luZy52MS5NTUJhblN0YXR1c1ILbW1CYW5TdGF0dXM=');

@$core.Deprecated('Use playerGameEntryDescriptor instead')
const PlayerGameEntry$json = {
  '1': 'PlayerGameEntry',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'region', '3': 2, '4': 1, '5': 9, '10': 'region'},
    {'1': 'role', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'role', '17': true},
    {'1': 'rank', '3': 4, '4': 1, '5': 9, '9': 1, '10': 'rank', '17': true},
    {
      '1': 'updated_at',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.google.protobuf.Timestamp',
      '10': 'updatedAt'
    },
  ],
  '8': [
    {'1': '_role'},
    {'1': '_rank'},
  ],
};

/// Descriptor for `PlayerGameEntry`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List playerGameEntryDescriptor = $convert.base64Decode(
    'Cg9QbGF5ZXJHYW1lRW50cnkSFwoHZ2FtZV9pZBgBIAEoCVIGZ2FtZUlkEhYKBnJlZ2lvbhgCIA'
    'EoCVIGcmVnaW9uEhcKBHJvbGUYAyABKAlIAFIEcm9sZYgBARIXCgRyYW5rGAQgASgJSAFSBHJh'
    'bmuIAQESOQoKdXBkYXRlZF9hdBgFIAEoCzIaLmdvb2dsZS5wcm90b2J1Zi5UaW1lc3RhbXBSCX'
    'VwZGF0ZWRBdEIHCgVfcm9sZUIHCgVfcmFuaw==');

@$core.Deprecated('Use getMyPlayerProfileRequestDescriptor instead')
const GetMyPlayerProfileRequest$json = {
  '1': 'GetMyPlayerProfileRequest',
};

/// Descriptor for `GetMyPlayerProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPlayerProfileRequestDescriptor =
    $convert.base64Decode('ChlHZXRNeVBsYXllclByb2ZpbGVSZXF1ZXN0');

@$core.Deprecated('Use getMyPlayerProfileResponseDescriptor instead')
const GetMyPlayerProfileResponse$json = {
  '1': 'GetMyPlayerProfileResponse',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.matchmaking.v1.PlayerGameEntry',
      '10': 'entries'
    },
  ],
};

/// Descriptor for `GetMyPlayerProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getMyPlayerProfileResponseDescriptor =
    $convert.base64Decode(
        'ChpHZXRNeVBsYXllclByb2ZpbGVSZXNwb25zZRI/CgdlbnRyaWVzGAEgAygLMiUudm9pY2UubW'
        'F0Y2htYWtpbmcudjEuUGxheWVyR2FtZUVudHJ5UgdlbnRyaWVz');

@$core.Deprecated('Use getPlayerProfileRequestDescriptor instead')
const GetPlayerProfileRequest$json = {
  '1': 'GetPlayerProfileRequest',
  '2': [
    {'1': 'profile_id', '3': 1, '4': 1, '5': 9, '10': 'profileId'},
  ],
};

/// Descriptor for `GetPlayerProfileRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPlayerProfileRequestDescriptor =
    $convert.base64Decode(
        'ChdHZXRQbGF5ZXJQcm9maWxlUmVxdWVzdBIdCgpwcm9maWxlX2lkGAEgASgJUglwcm9maWxlSW'
        'Q=');

@$core.Deprecated('Use getPlayerProfileResponseDescriptor instead')
const GetPlayerProfileResponse$json = {
  '1': 'GetPlayerProfileResponse',
  '2': [
    {
      '1': 'entries',
      '3': 1,
      '4': 3,
      '5': 11,
      '6': '.voice.matchmaking.v1.PlayerGameEntry',
      '10': 'entries'
    },
  ],
};

/// Descriptor for `GetPlayerProfileResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List getPlayerProfileResponseDescriptor =
    $convert.base64Decode(
        'ChhHZXRQbGF5ZXJQcm9maWxlUmVzcG9uc2USPwoHZW50cmllcxgBIAMoCzIlLnZvaWNlLm1hdG'
        'NobWFraW5nLnYxLlBsYXllckdhbWVFbnRyeVIHZW50cmllcw==');

@$core.Deprecated('Use upsertPlayerGameEntryRequestDescriptor instead')
const UpsertPlayerGameEntryRequest$json = {
  '1': 'UpsertPlayerGameEntryRequest',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
    {'1': 'region', '3': 2, '4': 1, '5': 9, '10': 'region'},
    {'1': 'role', '3': 3, '4': 1, '5': 9, '9': 0, '10': 'role', '17': true},
    {'1': 'rank', '3': 4, '4': 1, '5': 9, '9': 1, '10': 'rank', '17': true},
  ],
  '8': [
    {'1': '_role'},
    {'1': '_rank'},
  ],
};

/// Descriptor for `UpsertPlayerGameEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertPlayerGameEntryRequestDescriptor =
    $convert.base64Decode(
        'ChxVcHNlcnRQbGF5ZXJHYW1lRW50cnlSZXF1ZXN0EhcKB2dhbWVfaWQYASABKAlSBmdhbWVJZB'
        'IWCgZyZWdpb24YAiABKAlSBnJlZ2lvbhIXCgRyb2xlGAMgASgJSABSBHJvbGWIAQESFwoEcmFu'
        'axgEIAEoCUgBUgRyYW5riAEBQgcKBV9yb2xlQgcKBV9yYW5r');

@$core.Deprecated('Use upsertPlayerGameEntryResponseDescriptor instead')
const UpsertPlayerGameEntryResponse$json = {
  '1': 'UpsertPlayerGameEntryResponse',
  '2': [
    {
      '1': 'entry',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.voice.matchmaking.v1.PlayerGameEntry',
      '10': 'entry'
    },
  ],
};

/// Descriptor for `UpsertPlayerGameEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List upsertPlayerGameEntryResponseDescriptor =
    $convert.base64Decode(
        'Ch1VcHNlcnRQbGF5ZXJHYW1lRW50cnlSZXNwb25zZRI7CgVlbnRyeRgBIAEoCzIlLnZvaWNlLm'
        '1hdGNobWFraW5nLnYxLlBsYXllckdhbWVFbnRyeVIFZW50cnk=');

@$core.Deprecated('Use deletePlayerGameEntryRequestDescriptor instead')
const DeletePlayerGameEntryRequest$json = {
  '1': 'DeletePlayerGameEntryRequest',
  '2': [
    {'1': 'game_id', '3': 1, '4': 1, '5': 9, '10': 'gameId'},
  ],
};

/// Descriptor for `DeletePlayerGameEntryRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deletePlayerGameEntryRequestDescriptor =
    $convert.base64Decode(
        'ChxEZWxldGVQbGF5ZXJHYW1lRW50cnlSZXF1ZXN0EhcKB2dhbWVfaWQYASABKAlSBmdhbWVJZA'
        '==');

@$core.Deprecated('Use deletePlayerGameEntryResponseDescriptor instead')
const DeletePlayerGameEntryResponse$json = {
  '1': 'DeletePlayerGameEntryResponse',
};

/// Descriptor for `DeletePlayerGameEntryResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List deletePlayerGameEntryResponseDescriptor =
    $convert.base64Decode('Ch1EZWxldGVQbGF5ZXJHYW1lRW50cnlSZXNwb25zZQ==');
