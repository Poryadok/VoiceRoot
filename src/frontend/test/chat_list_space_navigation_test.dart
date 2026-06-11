import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/backend/spaces_client.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/state/space_providers.dart';
import 'package:voice_frontend/ui/shell/chat_list_body.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('space strip selects space in shell without Navigator push', (
    tester,
  ) async {
    const space = VoiceSpace(
      id: 'space-a',
      name: 'Alpha Squad',
      visibility: 'SPACE_VISIBILITY_PRIVATE',
      ownerProfileId: 'owner-1',
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          ...voiceAppTestOverrides(
            client: MockClient((req) async {
              if (req.url.path == '/api/v1/chats') {
                return http.Response(
                  jsonEncode({'chat_list': {'items': []}}),
                  200,
                );
              }
              return http.Response('{}', 404);
            }),
          ),
          mySpacesProvider.overrideWith(
            (_) async => const SpaceListData(spaces: [space]),
          ),
        ],
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('en'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SizedBox(
              width: 320,
              height: 600,
              child: ChatListBody(showHeader: true),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(ChatListBody.spaceTileKey('space-a')), findsOneWidget);
    await tester.tap(find.byKey(ChatListBody.spaceTileKey('space-a')));
    await tester.pump();

    expect(find.byType(Navigator), findsOneWidget);
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ChatListBody)),
    );
    expect(container.read(selectedSpaceIdProvider), 'space-a');
  });
}
