import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:livekit_client/livekit_client.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'backend/auth_session_storage.dart';
import 'backend/discover_hint_storage.dart';
import 'bootstrap/voice_app_bootstrap.dart';
import 'state/auth_providers.dart';
import 'state/space_providers.dart';
import 'theme/profile_accent_storage.dart';
import 'theme/voice_theme_providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await LiveKitClient.initialize();
  final prefs = await SharedPreferences.getInstance();
  runApp(
    ProviderScope(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          SharedPreferencesAuthSessionStorage(prefs),
        ),
        discoverHintStorageProvider.overrideWithValue(
          SharedPreferencesDiscoverHintStorage(prefs),
        ),
        profileAccentStorageProvider.overrideWithValue(
          SharedPreferencesProfileAccentStorage(prefs),
        ),
        spaceViewerProfileIdProvider.overrideWith(
          (ref) => ref.watch(authControllerProvider).activeProfileId,
        ),
      ],
      child: const VoiceAppBootstrap(),
    ),
  );
}
