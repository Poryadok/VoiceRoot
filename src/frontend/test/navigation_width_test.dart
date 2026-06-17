import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/shell/navigation_panel.dart';

import 'support/auth_test_overrides.dart';
import 'support/voice_test_theme.dart';

void main() {
  testWidgets('320px navigation fits Russian direct messages on one line', (
    tester,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: voiceAppTestOverrides(
          client: MockClient((_) async => http.Response('{}', 200)),
        ),
        child: MaterialApp(
          theme: voiceTestTheme(),
          locale: const Locale('ru'),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: const Scaffold(
            body: SizedBox(
              width: 320,
              child: NavigationPanel(collapsed: false),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    final label = find.text('Личные сообщения');
    expect(label, findsOneWidget);

    final box = tester.renderObject<RenderBox>(label);
    final textPainter = TextPainter(
      text: TextSpan(
        text: 'Личные сообщения',
        style: Theme.of(tester.element(label)).textTheme.bodyMedium,
      ),
      maxLines: 1,
      textDirection: TextDirection.ltr,
    )..layout(maxWidth: box.size.width);

    expect(
      box.size.height,
      lessThanOrEqualTo(textPainter.preferredLineHeight * 1.25),
      reason: 'label must fit on one line at 320px width',
    );
  });
}
