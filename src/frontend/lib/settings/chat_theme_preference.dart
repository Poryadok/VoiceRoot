import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../backend/subscription_client.dart';
import '../state/auth_providers.dart';
import '../state/subscription_providers.dart';

const chatThemePreferencePrefKey = 'voice_chat_theme_preferences';

enum ChatTheme { ocean, violet, sunset, midnight }

@immutable
class ChatThemePalette {
  const ChatThemePalette({
    required this.incoming,
    required this.outgoing,
    required this.incomingBorder,
    required this.outgoingBorder,
    required this.text,
    required this.textSecondary,
  });

  final Color incoming;
  final Color outgoing;
  final Color incomingBorder;
  final Color outgoingBorder;
  final Color text;
  final Color textSecondary;

  Color backgroundFor(bool isMine) => isMine ? outgoing : incoming;

  Color borderFor(bool isMine) => isMine ? outgoingBorder : incomingBorder;

  static ChatThemePalette forTheme(ChatTheme theme) => switch (theme) {
    ChatTheme.ocean => const ChatThemePalette(
      incoming: Color(0xFFE6F2FC),
      outgoing: Color(0xFFCDEBFA),
      incomingBorder: Color(0xFFC4DDED),
      outgoingBorder: Color(0xFFA8D7F0),
      text: Color(0xFF183446),
      textSecondary: Color(0xFF365769),
    ),
    ChatTheme.violet => const ChatThemePalette(
      incoming: Color(0xFFEEE8FF),
      outgoing: Color(0xFFC9B8FF),
      incomingBorder: Color(0xFFD7CCF4),
      outgoingBorder: Color(0xFFB7A3F1),
      text: Color(0xFF30224D),
      textSecondary: Color(0xFF4E3D70),
    ),
    ChatTheme.sunset => const ChatThemePalette(
      incoming: Color(0xFFFFF3D5),
      outgoing: Color(0xFFF0A8A8),
      incomingBorder: Color(0xFFEBD9AD),
      outgoingBorder: Color(0xFFD98F91),
      text: Color(0xFF482E29),
      textSecondary: Color(0xFF644642),
    ),
    ChatTheme.midnight => const ChatThemePalette(
      incoming: Color(0xFF24333C),
      outgoing: Color(0xFF304B5A),
      incomingBorder: Color(0xFF43545E),
      outgoingBorder: Color(0xFF587487),
      text: Color(0xFFF3F7FA),
      textSecondary: Color(0xFFD0DCE3),
    ),
  };
}

final chatThemePreferenceProvider =
    AsyncNotifierProvider<ChatThemePreferenceNotifier, Map<String, ChatTheme>>(
      ChatThemePreferenceNotifier.new,
    );

bool hasCurrentAccountPremiumSubscription(
  AsyncValue<VoiceSubscription?> subscription,
  String? accountId,
) {
  final currentSubscription = subscription.asData?.value;
  return accountId != null &&
      currentSubscription?.accountId == accountId &&
      currentSubscription?.isPremium == true;
}

final effectiveChatThemeProvider = Provider.family<ChatTheme?, String>((
  ref,
  chatId,
) {
  final accountId = ref.watch(authControllerProvider).session?.accountId;
  if (!hasCurrentAccountPremiumSubscription(
    ref.watch(subscriptionProvider),
    accountId,
  )) {
    return null;
  }
  return ref.watch(chatThemePreferenceProvider).valueOrNull?[chatId];
});

class ChatThemePreferenceNotifier
    extends AsyncNotifier<Map<String, ChatTheme>> {
  @override
  Future<Map<String, ChatTheme>> build() async {
    return readChatThemePreferences(await SharedPreferences.getInstance());
  }

  Future<void> setForChat(String chatId, ChatTheme theme) async {
    if (chatId.isEmpty) throw ArgumentError.value(chatId, 'chatId');
    final current = state.valueOrNull ?? await future;
    final updated = Map<String, ChatTheme>.from(current)..[chatId] = theme;
    final prefs = await SharedPreferences.getInstance();
    final saved = await prefs.setString(
      chatThemePreferencePrefKey,
      jsonEncode({
        for (final entry in updated.entries) entry.key: entry.value.name,
      }),
    );
    if (!saved) throw StateError('Could not save chat theme preference');
    state = AsyncData(Map.unmodifiable(updated));
  }

  Future<void> resetForChat(String chatId) async {
    if (chatId.isEmpty) throw ArgumentError.value(chatId, 'chatId');
    final current = state.valueOrNull ?? await future;
    if (!current.containsKey(chatId)) return;
    final updated = Map<String, ChatTheme>.from(current)..remove(chatId);
    final prefs = await SharedPreferences.getInstance();
    final saved = await prefs.setString(
      chatThemePreferencePrefKey,
      jsonEncode({
        for (final entry in updated.entries) entry.key: entry.value.name,
      }),
    );
    if (!saved) throw StateError('Could not save chat theme preference');
    state = AsyncData(Map.unmodifiable(updated));
  }
}

Map<String, ChatTheme> readChatThemePreferences(SharedPreferences prefs) {
  final raw = prefs.getString(chatThemePreferencePrefKey);
  if (raw == null || raw.isEmpty) return const {};

  final Object? decoded;
  try {
    decoded = jsonDecode(raw);
  } on FormatException {
    return const {};
  }
  if (decoded is! Map<String, dynamic>) return const {};

  final result = <String, ChatTheme>{};
  for (final entry in decoded.entries) {
    if (entry.key.isEmpty || entry.value is! String) continue;
    for (final theme in ChatTheme.values) {
      if (theme.name == entry.value) {
        result[entry.key] = theme;
        break;
      }
    }
  }
  return Map.unmodifiable(result);
}
