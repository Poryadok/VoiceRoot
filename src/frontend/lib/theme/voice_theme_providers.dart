import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../state/auth_providers.dart';
import '../state/social_providers.dart';
import 'profile_accent_storage.dart';
import 'voice_theme.dart';
import 'voice_token_catalog.dart';

final profileAccentStorageProvider = Provider<ProfileAccentStorage>((ref) {
  throw UnimplementedError(
    'Override profileAccentStorageProvider in ProviderScope',
  );
});

final voiceTokenCatalogProvider = FutureProvider<VoiceTokenCatalog>((ref) {
  return VoiceTokenCatalog.load();
});

/// Resolved accent [Color] for [profileId] (server accent, then local override, then palette).
final profileAccentColorProvider = FutureProvider.family<Color, String>((
  ref,
  profileId,
) async {
  final catalog = await ref.watch(voiceTokenCatalogProvider.future);
  final profile = await ref.watch(profileProvider(profileId).future);
  final serverAccent = profile?.accentColor;
  if (serverAccent != null && serverAccent.isNotEmpty) {
    return VoiceTokenCatalog.colorFromHex(serverAccent);
  }
  final storage = ref.watch(profileAccentStorageProvider);
  final override = await storage.readOverride(profileId);
  if (override != null && override.isNotEmpty) {
    return VoiceTokenCatalog.colorFromHex(override);
  }
  var index = await storage.readProfileIndex(profileId);
  index ??= 0;
  return catalog.profileAccentAt(index);
});

final activeProfileAccentColorProvider = Provider<AsyncValue<Color>>((ref) {
  final profileId = ref.watch(authControllerProvider).activeProfileId;
  if (profileId == null) {
    return const AsyncValue.data(Color(0xFF7EC8E3));
  }
  return ref.watch(profileAccentColorProvider(profileId));
});

enum AppThemePreference { system, light, dark, highContrast }

final appThemePreferenceProvider = StateProvider<AppThemePreference>(
  (ref) => AppThemePreference.system,
);

/// When null, [MaterialApp] uses platform locale.
final appLocalePreferenceProvider = StateProvider<Locale?>((ref) => null);

VoiceThemeMode _resolveMode(AppThemePreference pref, Brightness platform) {
  return switch (pref) {
    AppThemePreference.light => VoiceThemeMode.light,
    AppThemePreference.dark => VoiceThemeMode.dark,
    AppThemePreference.highContrast => VoiceThemeMode.highContrast,
    AppThemePreference.system =>
      platform == Brightness.dark ? VoiceThemeMode.dark : VoiceThemeMode.light,
  };
}

/// Material theme for the app shell (watches catalog, accent, preference).
final voiceMaterialThemeProvider = FutureProvider<ThemeData>((ref) async {
  final catalog = await ref.watch(voiceTokenCatalogProvider.future);
  final accentAsync = ref.watch(activeProfileAccentColorProvider);
  final accent = accentAsync.value ?? catalog.profileAccentAt(0);
  final pref = ref.watch(appThemePreferenceProvider);
  final platform =
      WidgetsBinding.instance.platformDispatcher.platformBrightness;
  final mode = _resolveMode(pref, platform);
  return VoiceTheme.build(catalog: catalog, mode: mode, profileAccent: accent);
});
