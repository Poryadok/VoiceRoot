import 'package:auto_updater/auto_updater.dart' as auto_updater;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:livekit_client/livekit_client.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'backend/auth_session_storage.dart';
import 'backend/client_version.dart';
import 'backend/discover_hint_storage.dart';
import 'backend/guest_credentials_storage.dart';
import 'bootstrap/voice_app_bootstrap.dart';
import 'state/auth_providers.dart';
import 'state/call_providers.dart';
import 'state/message_cache_providers.dart';
import 'state/space_providers.dart';
import 'state/voice_room_providers.dart';
import 'theme/profile_accent_storage.dart';
import 'theme/voice_theme_providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  if (ClientVersion.usesDesktopAutoUpdater) {
    await auto_updater.autoUpdater.setScheduledCheckInterval(14400);
  }
  await LiveKitClient.initialize();
  final prefs = await SharedPreferences.getInstance();
  final messageCacheStore = await openDefaultMessageCacheStore();
  runApp(
    ProviderScope(
      overrides: [
        messageCacheStoreProvider.overrideWithValue(messageCacheStore),
        authSessionStorageProvider.overrideWithValue(
          SharedPreferencesAuthSessionStorage(prefs),
        ),
        guestCredentialsStorageProvider.overrideWithValue(
          FlutterGuestCredentialsStorage(prefs: prefs),
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
        joinVoiceRoomActionProvider.overrideWith((ref) {
          return ({required String voiceRoomId, required String spaceId}) async {
            await ref.read(callControllerProvider.notifier).joinVoiceRoom(
              voiceRoomId: voiceRoomId,
              spaceId: spaceId,
            );
          };
        }),
      ],
      child: const VoiceAppBootstrap(),
    ),
  );
}
