import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'backend/auth_session_storage.dart';
import 'bootstrap/voice_app_bootstrap.dart';
import 'state/auth_providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final prefs = await SharedPreferences.getInstance();
  runApp(
    ProviderScope(
      overrides: [
        authSessionStorageProvider.overrideWithValue(
          SharedPreferencesAuthSessionStorage(prefs),
        ),
      ],
      child: const VoiceAppBootstrap(),
    ),
  );
}
