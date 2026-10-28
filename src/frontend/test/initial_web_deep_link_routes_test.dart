import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/routing/app_router.dart';

void main() {
  for (final path in [
    '/u/vanya?utm_source=notification',
    '/dm/550e8400-e29b-41d4-a716-446655440000',
    '/invite/join-me',
  ]) {
    testWidgets('initial browser $path mounts the deep-link shell', (
      tester,
    ) async {
      tester.binding.platformDispatcher.defaultRouteNameTestValue = path;
      addTearDown(
        tester.binding.platformDispatcher.clearDefaultRouteNameTestValue,
      );
      final dispatched = <Uri>[];
      final router = createVoiceGoRouter(
        onDeepLinkPath: (state) => dispatched.add(state.uri),
        shellBuilder: (_, state) => Scaffold(body: Text('shell:${state.uri}')),
      );
      addTearDown(router.dispose);

      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();

      expect(find.text('shell:$path'), findsOneWidget);
      expect(dispatched, contains(Uri.parse(path)));
      expect(router.routerDelegate.currentConfiguration.error, isNull);
    });
  }
}
