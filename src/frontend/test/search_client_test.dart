import 'dart:convert';

import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/backend/gateway_http.dart';
import 'package:voice_frontend/backend/search_client.dart';

void main() {
  test('searchInChat calls gateway in-chat route with chat_id and q', () async {
    String? path;
    Map<String, String>? query;
    final client = VoiceSearchClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          query = request.url.queryParameters;
          return http.Response(
            jsonEncode({
              'searchResults': {
                'hits': [
                  {
                    'messageId': 'msg-1',
                    'snippet': 'matched <b>hello</b>',
                    'score': 1.0,
                  },
                ],
                'nextCursor': '',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.searchInChat(
      authorization: 'Bearer t',
      chatId: 'chat-42',
      query: 'hello',
    );

    expect(path, '/api/v1/search/in-chat');
    expect(query?['chat_id'], 'chat-42');
    expect(query?['q'], 'hello');
    expect(result, isA<SearchApiOk<InChatSearchData>>());
    final data = (result as SearchApiOk<InChatSearchData>).data;
    expect(data.hits, hasLength(1));
    expect(data.hits.first.messageId, 'msg-1');
    expect(data.hits.first.snippet, contains('hello'));
  });

  test('searchGlobal defaults page_size to 20', () async {
    Map<String, String>? query;
    final client = VoiceSearchClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          query = request.url.queryParameters;
          return http.Response(
            jsonEncode({
              'globalSearchResults': {
                'messages': [],
                'profileIds': [],
                'matchedChats': [],
                'spaceIds': [],
                'nextCursor': '',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    await client.searchGlobal(
      authorization: 'Bearer t',
      query: 'raid',
    );

    expect(query?['q'], 'raid');
    expect(query?['page_size'], '20');
  });

  test('searchUsers calls users search route', () async {
    String? path;
    final client = VoiceSearchClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          return http.Response(
            jsonEncode({
              'userSearchResults': {
                'profileIds': ['profile-1'],
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.searchUsers(
      authorization: 'Bearer t',
      query: 'carol',
    );

    expect(path, '/api/v1/search/users');
    expect(result, isA<SearchApiOk<UserSearchData>>());
    expect(
      (result as SearchApiOk<UserSearchData>).data.profileIds,
      ['profile-1'],
    );
  });

  test('searchSpaces calls spaces search route with pagination cursor', () async {
    String? path;
    Map<String, String>? query;
    final client = VoiceSearchClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((request) async {
          path = request.url.path;
          query = request.url.queryParameters;
          return http.Response(
            jsonEncode({
              'spaceSearchResults': {
                'spaceIds': ['space-1'],
                'nextCursor': 'cursor-2',
              },
            }),
            200,
          );
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.searchSpaces(
      authorization: 'Bearer t',
      query: 'guild',
      cursor: 'cursor-1',
      pageSize: 20,
    );

    expect(path, '/api/v1/search/spaces');
    expect(query?['cursor'], 'cursor-1');
    expect(query?['page_size'], '20');
    expect(result, isA<SearchApiOk<SpaceSearchData>>());
    expect(
      (result as SearchApiOk<SpaceSearchData>).data.nextCursor,
      'cursor-2',
    );
  });

  test('search debounce waits 300ms before firing', () {
    fakeAsync((async) {
      var calls = 0;
      final debouncer = SearchQueryDebouncer(
        delay: SearchQueryDebouncer.defaultDelay,
        onQuery: (_) => calls++,
      );

      debouncer.schedule('a');
      debouncer.schedule('ab');
      debouncer.schedule('abc');
      expect(calls, 0);

      async.elapse(const Duration(milliseconds: 299));
      expect(calls, 0);

      async.elapse(const Duration(milliseconds: 1));
      expect(calls, 1);
    });
  });

  test('empty query is rejected without HTTP call', () async {
    var called = false;
    final client = VoiceSearchClient(
      gateway: GatewayHttpClient(
        httpClient: MockClient((_) async {
          called = true;
          return http.Response('{}', 200);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );

    final result = await client.searchGlobal(
      authorization: 'Bearer t',
      query: '   ',
    );

    expect(called, isFalse);
    expect(result, isA<SearchApiErr>());
  });
}
