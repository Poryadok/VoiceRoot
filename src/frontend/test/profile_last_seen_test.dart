import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:voice_frontend/l10n/app_localizations.dart';
import 'package:voice_frontend/ui/social/profile_detail_sheet.dart';

void main() {
  for (final locale in [const Locale('en'), const Locale('ru')]) {
    testWidgets('profile last seen rounds complete units in ${locale.languageCode}', (tester) async {
      late BuildContext context;
      await tester.pumpWidget(MaterialApp(
        locale: locale,
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        home: Builder(builder: (value) { context = value; return const SizedBox.shrink(); }),
      ));
      await tester.pumpAndSettle();
      final now = DateTime(2026, 10, 16, 12);
      for (final example in [
        (age: const Duration(seconds: -30), en: 'Last seen just now', ru: 'Был(а) только что'),
        (age: Duration.zero, en: 'Last seen just now', ru: 'Был(а) только что'),
        (age: const Duration(seconds: 59), en: 'Last seen just now', ru: 'Был(а) только что'),
        (age: const Duration(minutes: 1), en: 'Last seen 1 minute ago', ru: 'Был(а) 1 минуту назад'),
        (age: const Duration(minutes: 2), en: 'Last seen 2 minutes ago', ru: 'Был(а) 2 минуты назад'),
        (age: const Duration(minutes: 5), en: 'Last seen 5 minutes ago', ru: 'Был(а) 5 минут назад'),
        (age: const Duration(minutes: 21), en: 'Last seen 21 minutes ago', ru: 'Был(а) 21 минуту назад'),
        (age: const Duration(minutes: 22), en: 'Last seen 22 minutes ago', ru: 'Был(а) 22 минуты назад'),
        (age: const Duration(minutes: 59, seconds: 59), en: 'Last seen 59 minutes ago', ru: 'Был(а) 59 минут назад'),
        (age: const Duration(hours: 1), en: 'Last seen 1 hour ago', ru: 'Был(а) 1 час назад'),
        (age: const Duration(hours: 2), en: 'Last seen 2 hours ago', ru: 'Был(а) 2 часа назад'),
        (age: const Duration(hours: 5), en: 'Last seen 5 hours ago', ru: 'Был(а) 5 часов назад'),
        (age: const Duration(hours: 21), en: 'Last seen 21 hours ago', ru: 'Был(а) 21 час назад'),
        (age: const Duration(hours: 23, minutes: 59), en: 'Last seen 23 hours ago', ru: 'Был(а) 23 часа назад'),
        (age: const Duration(days: 1), en: 'Last seen 1 day ago', ru: 'Был(а) 1 день назад'),
        (age: const Duration(days: 2), en: 'Last seen 2 days ago', ru: 'Был(а) 2 дня назад'),
        (age: const Duration(days: 5), en: 'Last seen 5 days ago', ru: 'Был(а) 5 дней назад'),
        (age: const Duration(days: 6, hours: 23, minutes: 59), en: 'Last seen 6 days ago', ru: 'Был(а) 6 дней назад'),
      ]) {
        expect(profileLastSeenLabel(context, now.subtract(example.age), now: now),
            locale.languageCode == 'ru' ? example.ru : example.en,
            reason: 'age ${example.age}');
      }
      // Canon defines 1–6 days and >7 days, leaving day 7 unspecified. Preserve
      // the existing absolute date/time fallback for >=7 days until clarified.
      for (final days in [7, 8, 365]) {
        final lastSeen = now.subtract(Duration(days: days));
        final date = MaterialLocalizations.of(context).formatShortDate(lastSeen);
        final time = TimeOfDay.fromDateTime(lastSeen).format(context);
        final existingFallback = AppLocalizations.of(context)!.socialPresenceLastSeen('$date $time');
        expect(profileLastSeenLabel(context, lastSeen, now: now), existingFallback);
      }
    });
  }
}
