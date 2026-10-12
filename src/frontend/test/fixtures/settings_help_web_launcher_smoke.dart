import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/settings/settings_sheet.dart';

import '../support/auth_test_overrides.dart';
import '../support/voice_test_theme.dart';

void main() {
  final container = ProviderContainer(
    overrides: voiceAppTestOverrides(
      client: MockClient((_) async => http.Response('{}', 200)),
    ),
  );
  runApp(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: voiceTestTheme(),
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: FilledButton(
                onPressed: () => showModalBottomSheet<void>(
                  context: context,
                  isScrollControlled: true,
                  builder: (_) => const SettingsSheet(),
                ),
                child: const Text('Open Settings Help smoke'),
              ),
            ),
          ),
        ),
      ),
    ),
  );
}
