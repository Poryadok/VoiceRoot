import 'package:auto_updater/auto_updater.dart' as auto_updater;
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:livekit_client/livekit_client.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:http/http.dart' as http;

import 'backend/auth_session_storage.dart';
import 'backend/client_version.dart';
import 'backend/chat_draft_storage.dart';
import 'backend/discover_hint_storage.dart';
import 'backend/guest_credentials_storage.dart';
import 'backend/web_gateway_config.dart';
import 'bootstrap/voice_app_bootstrap.dart';
import 'bootstrap/startup_failure.dart';
import 'state/auth_providers.dart';
import 'state/call_providers.dart';
import 'state/chat_draft_providers.dart';
import 'state/message_cache_providers.dart';
import 'state/gateway_providers.dart';
import 'state/space_providers.dart';
import 'state/voice_room_providers.dart';
import 'theme/profile_accent_storage.dart';
import 'settings/theme_preference.dart';
import 'theme/voice_theme_providers.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  const runtimeConfigRequired = bool.fromEnvironment(
    'VOICE_RUNTIME_CONFIG_REQUIRED',
    defaultValue: false,
  );
  final runtimeConfig = kIsWeb && runtimeConfigRequired
      ? await (() async {
          final client = http.Client();
          try {
            try {
              return await loadWebGatewayConfig(
                appUri: Uri.base,
                client: client,
              );
            } catch (_) {
              reportStartupFailure();
              rethrow;
            }
          } finally {
            client.close();
          }
        })()
      : null;
  if (ClientVersion.usesDesktopAutoUpdater) {
    await auto_updater.autoUpdater.setScheduledCheckInterval(14400);
  }
  await LiveKitClient.initialize();
  final prefs = await SharedPreferences.getInstance();
  final savedTheme = readThemePreference(prefs);
  final messageCacheStore = await openDefaultMessageCacheStore();
  final chatDraftStorage = await HiveChatDraftStorage.open(prefs);
  runApp(
    ProviderScope(
      overrides: [
        if (runtimeConfig != null)
          gatewayConfigProvider.overrideWithValue(runtimeConfig),
        messageCacheStoreProvider.overrideWithValue(messageCacheStore),
        chatDraftStorageProvider.overrideWithValue(chatDraftStorage),
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
        appThemePreferenceProvider.overrideWith(
          () => SeededAppThemePreferenceNotifier(savedTheme),
        ),
        spaceViewerProfileIdProvider.overrideWith(
          (ref) => ref.watch(authControllerProvider).activeProfileId,
        ),
        joinVoiceRoomActionProvider.overrideWith((ref) {
          return ({
            required String voiceRoomId,
            required String spaceId,
          }) async {
            await ref
                .read(callControllerProvider.notifier)
                .joinVoiceRoom(voiceRoomId: voiceRoomId, spaceId: spaceId);
          };
        }),
      ],
      child: const VoiceAppBootstrap(),
    ),
  );
}
