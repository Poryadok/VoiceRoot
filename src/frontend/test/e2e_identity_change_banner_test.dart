import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/chat/e2e_identity_change_banner.dart';

import 'support/test_voice_token_catalog.dart';
import 'support/voice_test_theme.dart';

/// Batch E2E-A audit: identity key rotation banner keys (docs/features/encryption.md).
void main() {
  Widget bannerTestApp({required Widget child}) {
    return ProviderScope(
      overrides: voiceThemeTestOverrides(),
      child: MaterialApp(
        theme: voiceTestTheme(),
        locale: const Locale('en'),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Scaffold(body: child),
      ),
    );
  }

  testWidgets('E2eIdentityChangeBanner exposes stable widget keys', (tester) async {
    await tester.pumpWidget(
      bannerTestApp(
        child: E2eIdentityChangeBanner(
          peerDisplayName: '@alice',
          onContinue: () {},
          onDistrust: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byKey(E2eIdentityChangeBanner.bannerKey), findsOneWidget);
    expect(find.byKey(E2eIdentityChangeBanner.continueButtonKey), findsOneWidget);
    expect(find.byKey(E2eIdentityChangeBanner.distrustButtonKey), findsOneWidget);
  });

  testWidgets('E2eIdentityChangeBanner shows encryption key changed copy', (tester) async {
    await tester.pumpWidget(
      bannerTestApp(
        child: E2eIdentityChangeBanner(
          peerDisplayName: '@alice',
          onContinue: () {},
          onDistrust: () {},
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.textContaining('encryption key changed', findRichText: true),
      findsOneWidget,
    );
    expect(find.text('Continue'), findsOneWidget);
    expect(find.text("Don't trust"), findsOneWidget);
  });
}
