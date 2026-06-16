import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:voice_frontend/routing/app_router.dart';
import 'package:voice_frontend/routing/deep_link_parser.dart';

void main() {
  group('parseDeepLinkUrl bot page', () {
    test('https voice.gg/bots/{slug}', () {
      const slug = 'statsbot';
      final got = parseDeepLinkUrl('https://voice.gg/bots/$slug');
      expect(got.kind, DeepLinkKind.bot);
      expect(got.botSlug, slug);
      expect(got.rawUrl, 'https://voice.gg/bots/$slug');
    });

    test('voice://bots/{slug}', () {
      const slug = 'ping-bot';
      final got = parseDeepLinkUrl('voice://bots/$slug');
      expect(got.kind, DeepLinkKind.bot);
      expect(got.botSlug, slug);
    });
  });

  testWidgets('GoRouter registers /bots/:slug deep link route', (tester) async {
    GoRouterState? captured;
    final router = createVoiceGoRouter(
      shellBuilder: (context, state) {
        captured = state;
        return const Scaffold(body: Text('shell'));
      },
      onDeepLinkPath: (_) {},
    );

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp.router(routerConfig: router),
      ),
    );

    router.go('/bots/statsbot');
    await tester.pumpAndSettle();

    expect(captured?.pathParameters['slug'], 'statsbot');
    expect(find.text('shell'), findsOneWidget);
  });
}
