import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/chats_client.dart';
import 'package:voice_frontend/backend/gateway_config.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/auth_providers.dart';
import 'package:voice_frontend/state/chat_navigation_providers.dart';
import 'package:voice_frontend/state/chat_providers.dart';
import 'package:voice_frontend/ui/shell/quick_access_actions.dart';
import 'package:voice_frontend/ui/shell/quick_access_replace_sheet.dart';

import 'support/gateway_test_client.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('QuickAccessReplaceSheet lists slots and returns selection', (
    tester,
  ) async {
    const items = [
      VoiceQuickAccessItem(
        chatId: 'qa-1',
        chat: VoiceChat(
          id: 'qa-1',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'p1',
          name: 'Slot One',
        ),
      ),
      VoiceQuickAccessItem(
        chatId: 'qa-2',
        chat: VoiceChat(
          id: 'qa-2',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'p1',
          name: 'Slot Two',
        ),
      ),
    ];

    String? picked;
    await tester.pumpWidget(
      MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) {
            return Scaffold(
              body: TextButton(
                onPressed: () async {
                  picked = await QuickAccessReplaceSheet.show(
                    context,
                    items: items,
                  );
                },
                child: const Text('open'),
              ),
            );
          },
        ),
      ),
    );

    await tester.tap(find.text('open'));
    await tester.pumpAndSettle();

    expect(find.byKey(QuickAccessReplaceSheet.sheetKey), findsOneWidget);
    expect(find.text('Choose what to replace'), findsOneWidget);
    expect(find.text('Slot One'), findsOneWidget);

    await tester.tap(find.text('Slot Two'));
    await tester.pumpAndSettle();

    expect(picked, 'qa-2');
  });

  testWidgets('selected replacement sends one atomic add request', (
    tester,
  ) async {
    final requests = <http.Request>[];
    final client = VoiceChatsClient(
      gateway: gatewayHttpForTest(
        MockClient((request) async {
          requests.add(request);
          return http.Response('', 204);
        }),
        config: const GatewayConfig(baseUrl: 'http://api.test'),
      ),
    );
    final slots = List<VoiceQuickAccessItem>.generate(
      15,
      (index) => VoiceQuickAccessItem(
        chatId: 'old-$index',
        chat: VoiceChat(
          id: 'old-$index',
          type: 'CHAT_TYPE_DM',
          creatorProfileId: 'profile-1',
          name: 'Slot $index',
        ),
      ),
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authorizationHeaderProvider.overrideWithValue('Bearer test'),
          voiceChatsClientProvider.overrideWithValue(client),
          quickAccessListProvider.overrideWith(
            (ref) async => QuickAccessListData(items: slots),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: Consumer(
            builder: (context, ref, _) => Scaffold(
              body: TextButton(
                onPressed: () =>
                    addChatToQuickAccess(context, ref, chatId: 'new-chat'),
                child: const Text('replace'),
              ),
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.text('replace'));
    await tester.pumpAndSettle();
    await tester.drag(find.byType(ListView), const Offset(0, -1000));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('quick_access_replace_old-14')));
    await tester.pumpAndSettle();

    expect(requests, hasLength(1));
    expect(requests.single.method, 'POST');
    expect(requests.single.url.path, '/api/v1/chats/quick-access');
    expect(requests.single.body, contains('"replace_chat_id":"old-14"'));
  });

  testWidgets('failed refresh after server limit shows an error', (
    tester,
  ) async {
    final client = _ScriptedQuickAccessClient([
      const ChatsApiFailure(
        message: 'Quick Access unavailable',
        errorCode: 'failed_precondition',
      ),
    ]);
    var loads = 0;
    await tester.pumpWidget(
      _actionApp(client, () async {
        loads++;
        if (loads > 1) throw StateError('offline');
        return QuickAccessListData(items: _slots(14));
      }),
    );

    await tester.tap(find.text('replace'));
    await tester.pumpAndSettle();

    expect(loads, 2);
    expect(client.replacedChatIds, [null]);
    expect(tester.takeException(), isNull);
    expect(find.text('Quick Access unavailable'), findsOneWidget);
  });

  testWidgets('failed replacement refreshes a stale picker', (tester) async {
    final client = _ScriptedQuickAccessClient([
      const ChatsApiFailure(
        message: 'slot changed',
        errorCode: 'failed_precondition',
      ),
      const ChatsApiOk(null),
    ]);
    var loads = 0;
    await tester.pumpWidget(
      _actionApp(client, () async {
        loads++;
        return QuickAccessListData(items: _slots(loads == 1 ? 15 : 14));
      }),
    );

    await tester.tap(find.text('replace'));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('quick_access_replace_old-0')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('replace'));
    await tester.pumpAndSettle();

    expect(loads, 2);
    expect(client.replacedChatIds, ['old-0', null]);
    expect(tester.takeException(), isNull);
  });
}

List<VoiceQuickAccessItem> _slots(int count) => List.generate(
  count,
  (index) => VoiceQuickAccessItem(
    chatId: 'old-$index',
    chat: VoiceChat(
      id: 'old-$index',
      type: 'CHAT_TYPE_DM',
      creatorProfileId: 'profile-1',
      name: 'Slot $index',
    ),
  ),
);

Widget _actionApp(
  VoiceChatsClient client,
  Future<QuickAccessListData> Function() load,
) => ProviderScope(
  overrides: [
    authorizationHeaderProvider.overrideWithValue('Bearer test'),
    voiceChatsClientProvider.overrideWithValue(client),
    quickAccessListProvider.overrideWith((ref) => load()),
  ],
  child: MaterialApp(
    theme: voiceTestTheme(),
    locale: const Locale('en'),
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    home: Consumer(
      builder: (context, ref, _) => Scaffold(
        body: TextButton(
          onPressed: () =>
              addChatToQuickAccess(context, ref, chatId: 'new-chat'),
          child: const Text('replace'),
        ),
      ),
    ),
  ),
);

class _ScriptedQuickAccessClient extends VoiceChatsClient {
  _ScriptedQuickAccessClient(this.results)
    : super(
        gateway: gatewayHttpForTest(
          MockClient((_) async => http.Response('', 204)),
          config: const GatewayConfig(baseUrl: 'http://api.test'),
        ),
      );

  final List<ChatsApiResult<void>> results;
  final List<String?> replacedChatIds = [];

  @override
  Future<ChatsApiResult<void>> addQuickAccess({
    required String authorization,
    required String chatId,
    int? sortOrder,
    String? replaceChatId,
  }) async {
    replacedChatIds.add(replaceChatId);
    return results.removeAt(0);
  }
}
