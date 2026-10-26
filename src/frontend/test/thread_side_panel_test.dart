import 'dart:convert';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/auth_session_storage.dart';
import 'package:voice_frontend/backend/auth_session.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/gateway_providers.dart';
import 'package:voice_frontend/theme/voice_theme_providers.dart';
import 'package:voice_frontend/ui/chat/thread_side_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/markdown_test_helpers.dart';
import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

void main() {
  Widget threadTestApp({required http.Client client}) {
    return ProviderScope(
      overrides: [
        ...voiceThemeTestOverrides(),
        profileAccentStorageProvider.overrideWithValue(
          testProfileAccentStorage,
        ),
        authSessionStorageProvider.overrideWithValue(
          InMemoryAuthSessionStorage(),
        ),
        authControllerProvider.overrideWith(authenticatedAuthController),
        gatewayConfigProvider.overrideWithValue(
          const GatewayConfig(baseUrl: 'http://api.test'),
        ),
        httpClientProvider.overrideWithValue(client),
      ],
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: const Scaffold(
          body: ThreadSidePanel(
            chatId: 'chat-thread',
            parentMessageId: 'msg-root',
            parentPreview: 'Root message',
          ),
        ),
      ),
    );
  }

  http.Response threadRepliesResponse() => http.Response(
    jsonEncode({
      'message_list': {
        'messages': [
          {
            'id': 'reply-1',
            'chat': {'id': 'chat-thread'},
            'sender_profile_id': 'profile-peer',
            'content': 'Recovered reply',
            'created_at': '2024-01-01T00:00:00Z',
          },
        ],
      },
    }),
    200,
    headers: {'content-type': 'application/json'},
  );

  testWidgets(
    'visible thread reply records and shows its group/channel view count',
    (tester) async {
      var views = 0;
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/messages/views') {
          views++;
          final body = jsonDecode(request.body) as Map<String, dynamic>;
          expect(body['message_id'], 'reply-1');
          return http.Response(
            '{"view_count":"9"}',
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'reply-1',
                  'chat': {'id': 'chat-thread', 'type': 'CHAT_TYPE_CHANNEL'},
                  'sender_profile_id': 'profile-peer',
                  'content': 'Newer thread reply',
                  'view_count': '8',
                },
              ],
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      await tester.pumpWidget(threadTestApp(client: client));
      await tester.pumpAndSettle();
      expect(views, 1);
      expect(find.byIcon(Icons.visibility_outlined), findsOneWidget);
      expect(find.text('9'), findsOneWidget);
    },
  );

  testWidgets(
    'late thread view result cannot overwrite another profile response',
    (tester) async {
      final pending = Completer<http.Response>();
      var views = 0;
      final client = MockClient((request) async {
        if (request.url.path == '/api/v1/messages/views') {
          views++;
          if (views == 1) return pending.future;
          return http.Response(
            '{"view_count":"10"}',
            200,
            headers: {'content-type': 'application/json'},
          );
        }
        return http.Response(
          jsonEncode({
            'message_list': {
              'messages': [
                {
                  'id': 'reply-1',
                  'chat': {'id': 'chat-thread', 'type': 'CHAT_TYPE_CHANNEL'},
                  'sender_profile_id': 'profile-peer',
                  'content': 'Reply',
                  'view_count': '8',
                },
              ],
            },
          }),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      await tester.pumpWidget(threadTestApp(client: client));
      await tester.pumpAndSettle();
      expect(views, 1);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(ThreadSidePanel)),
      );
      container.read(authControllerProvider.notifier).state = const AuthState(
        session: AuthSession(
          accessToken: 'other-token',
          refreshToken: 'other-refresh',
          accountId: 'other-account',
          activeProfileId: 'other-profile',
          expiresInSeconds: 900,
        ),
      );
      await tester.pumpAndSettle();
      expect(views, 2);
      expect(find.text('10'), findsOneWidget);
      pending.complete(
        http.Response(
          '{"view_count":"99"}',
          200,
          headers: {'content-type': 'application/json'},
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text('10'), findsOneWidget);
      expect(find.text('99'), findsNothing);
    },
  );

  testWidgets(
    'token rotation during thread load restarts the current profile request',
    (tester) async {
      final pending = Completer<http.Response>();
      var loads = 0;
      final client = MockClient((request) async {
        loads++;
        if (loads == 1) return pending.future;
        return http.Response(
          '{"message_list":{"messages":[{"id":"fresh","chat":{"id":"chat-thread"},"content":"Fresh reply"}]}}',
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      await tester.pumpWidget(threadTestApp(client: client));
      await tester.pumpAndSettle();
      final container = ProviderScope.containerOf(
        tester.element(find.byType(ThreadSidePanel)),
      );
      final session = container.read(authControllerProvider).session!;
      container.read(authControllerProvider.notifier).state = AuthState(
        session: AuthSession(
          accessToken: 'rotated-token',
          refreshToken: session.refreshToken,
          accountId: session.accountId,
          activeProfileId: session.activeProfileId,
          expiresInSeconds: 900,
        ),
      );
      await tester.pumpAndSettle();
      expect(loads, 2);
      expectMessagePlainText(tester, 'Fresh reply');
      pending.complete(threadRepliesResponse());
      await tester.pumpAndSettle();
      expectMessagePlainText(tester, 'Fresh reply');
      expect(richPlainText(tester), isNot(contains('Recovered reply')));
    },
  );

  testWidgets('thread load failure uses safe localized copy', (tester) async {
    final client = MockClient((_) async {
      return http.Response(
        jsonEncode({
          'error': 'internal_error',
          'message': 'database password=thread-secret-123',
        }),
        500,
        headers: {'content-type': 'application/json'},
      );
    });

    await tester.pumpWidget(threadTestApp(client: client));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ThreadSidePanel)),
    )!;
    expect(find.text(l10n.chatThreadLoadError), findsOneWidget);
    expect(find.text('database password=thread-secret-123'), findsNothing);
    expect(find.text(l10n.commonRetry), findsOneWidget);
  });

  testWidgets('thread unavailable response uses localized unavailable copy', (
    tester,
  ) async {
    final client = MockClient((_) async {
      return http.Response(
        jsonEncode({'error': 'unavailable', 'message': 'private detail'}),
        503,
        headers: {'content-type': 'application/json'},
      );
    });

    await tester.pumpWidget(threadTestApp(client: client));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ThreadSidePanel)),
    )!;
    expect(find.text(l10n.backendUnavailable), findsOneWidget);
    expect(find.text('private detail'), findsNothing);
  });

  testWidgets('thread permission denial uses localized permission copy', (
    tester,
  ) async {
    final client = MockClient((_) async {
      return http.Response(
        jsonEncode({'error': 'permission_denied', 'message': 'private detail'}),
        403,
        headers: {'content-type': 'application/json'},
      );
    });

    await tester.pumpWidget(threadTestApp(client: client));
    await tester.pumpAndSettle();

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ThreadSidePanel)),
    )!;
    expect(find.text(l10n.chatRoomPermissionDenied), findsOneWidget);
    expect(find.text('private detail'), findsNothing);
  });

  testWidgets('retry reloads replies and clears the previous error', (
    tester,
  ) async {
    var requestCount = 0;
    final client = MockClient((_) async {
      requestCount++;
      if (requestCount == 1) {
        return http.Response(
          jsonEncode({'error': 'internal_error', 'message': 'retry-secret'}),
          500,
          headers: {'content-type': 'application/json'},
        );
      }
      return threadRepliesResponse();
    });

    await tester.pumpWidget(threadTestApp(client: client));
    await tester.pumpAndSettle();
    expect(find.text('retry-secret'), findsNothing);

    final l10n = AppLocalizations.of(
      tester.element(find.byType(ThreadSidePanel)),
    )!;
    await tester.tap(find.text(l10n.commonRetry));
    await tester.pumpAndSettle();

    expect(requestCount, 2);
    expectMessagePlainText(tester, 'Recovered reply');
    expect(find.text('retry-secret'), findsNothing);
    expect(find.text(l10n.chatThreadLoadError), findsNothing);
  });
}
