import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'auth_providers.dart';

/// Ephemeral presentation data, never a subscription or durable unread authority.
class SubscriptionReminderHint {
  const SubscriptionReminderHint({
    required this.id,
    required this.profileId,
    required this.plan,
    required this.day,
    required this.windowEnd,
    this.spaceId,
  });
  final String id;
  final String profileId;
  final String plan;
  final int day;
  final DateTime windowEnd;
  final String? spaceId;

  static final _uuid = RegExp(
    r'^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$',
  );
  static bool _id(Object? value) =>
      value is String &&
      _uuid.hasMatch(value) &&
      value != '00000000-0000-0000-0000-000000000000';

  static SubscriptionReminderHint? parse(
    Map<String, dynamic>? data, {
    required String? activeProfileId,
    required DateTime now,
  }) {
    if (data == null ||
        data['type'] != 'subscription_grace_reminder' ||
        data['schema_version'] is! int ||
        data['schema_version'] != 1 ||
        !_id(data['notification_id']) ||
        !_id(data['target_profile_id']) ||
        data['target_profile_id'] != activeProfileId) {
      return null;
    }
    const fields = {
      'type',
      'schema_version',
      'notification_id',
      'target_profile_id',
      'plan',
      'day',
      'window_end',
      'space_id',
    };
    if (data.keys.any((key) => !fields.contains(key))) return null;
    final plan = data['plan'];
    final day = data['day'];
    final rawEnd = data['window_end'];
    if (day is! int ||
        !const [1, 3, 7].contains(day) ||
        rawEnd is! String ||
        !RegExp(
          r'^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z$',
        ).hasMatch(rawEnd)) {
      return null;
    }
    final end = DateTime.tryParse(rawEnd);
    if (end == null || !now.isBefore(end)) return null;
    if (end.toIso8601String().substring(0, 19) != rawEnd.substring(0, 19)) {
      return null;
    }
    final space = data['space_id'];
    if (plan == 'premium') {
      if (data.containsKey('space_id')) return null;
    } else if (plan == 'space_pro') {
      if (!_id(space)) return null;
    } else {
      return null;
    }
    return SubscriptionReminderHint(
      id: data['notification_id'] as String,
      profileId: activeProfileId!,
      plan: plan as String,
      day: day,
      windowEnd: end,
      spaceId: space as String?,
    );
  }
}

class SubscriptionReminderHints
    extends StateNotifier<List<SubscriptionReminderHint>> {
  SubscriptionReminderHints(this.profileId, {DateTime Function()? now})
    : _now = now ?? DateTime.now,
      super(const []);
  final String? profileId;
  final DateTime Function() _now;
  final Map<String, DateTime> _seen = {};
  Timer? _expiry;

  bool onData(Map<String, dynamic>? data) {
    final now = _now();
    _seen.removeWhere((_, end) => !now.isBefore(end));
    final hint = SubscriptionReminderHint.parse(
      data,
      activeProfileId: profileId,
      now: now,
    );
    if (hint == null || _seen.containsKey(hint.id)) return false;
    _seen[hint.id] = hint.windowEnd;
    state = [...state.where((row) => now.isBefore(row.windowEnd)), hint];
    _scheduleExpiry();
    return true;
  }

  void dismiss(String id) {
    state = state.where((row) => row.id != id).toList(growable: false);
    // Keep the seen identity through its window: retries do not reopen dismissal.
    _scheduleExpiry();
  }

  void _scheduleExpiry() {
    _expiry?.cancel();
    if (state.isEmpty) return;
    final end = state
        .map((row) => row.windowEnd)
        .reduce((a, b) => a.isBefore(b) ? a : b);
    final delay = end.difference(_now());
    _expiry = Timer(delay.isNegative ? Duration.zero : delay, () {
      state = state
          .where((row) => _now().isBefore(row.windowEnd))
          .toList(growable: false);
      _scheduleExpiry();
    });
  }

  @override
  void dispose() {
    _expiry?.cancel();
    super.dispose();
  }
}

final subscriptionReminderHintsProvider =
    StateNotifierProvider<
      SubscriptionReminderHints,
      List<SubscriptionReminderHint>
    >((ref) {
      // Routine token rotation and unrelated UI auth state are the same login.
      // A replaced controller, installed login, account or profile resets hints.
      final controller = ref.watch(authControllerProvider.notifier);
      final binding = ref.watch(
        authControllerProvider.select(
          (auth) => (
            auth.session?.accountId,
            auth.activeProfileId,
            controller.sessionInstallGeneration,
          ),
        ),
      );
      return SubscriptionReminderHints(binding.$1 != null ? binding.$2 : null);
    });
