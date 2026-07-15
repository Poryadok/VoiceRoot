import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../backend/notification_settings_models.dart';

const _prefPrefix = 'voice_quiet_hours_';

final notificationQuietHoursStorageProvider =
    Provider<NotificationQuietHoursStorage>((ref) {
  return NotificationQuietHoursStorage();
});

/// Local cache until Notification service exposes GET quiet-hours.
class NotificationQuietHoursStorage {
  Future<VoiceQuietHours> read(String profileId) async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getString('$_prefPrefix$profileId');
    if (raw == null || raw.isEmpty) return VoiceQuietHours.defaults;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map<String, dynamic>) return VoiceQuietHours.defaults;
      return VoiceQuietHours.fromJson(decoded);
    } catch (_) {
      return VoiceQuietHours.defaults;
    }
  }

  Future<void> write(String profileId, VoiceQuietHours quietHours) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(
      '$_prefPrefix$profileId',
      jsonEncode(quietHours.toJson()),
    );
  }
}
