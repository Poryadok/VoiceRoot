# ExecPlan: Flutter i18n-каркас (ARB, EN+RU, gen-l10n)

## Purpose

Клиент Flutter получает каркас локализации по `docs/features/i18n.md`: ARB `lib/l10n/app_en.arb` и `app_ru.arb`, генерация через `flutter gen-l10n`, зависимости `flutter_localizations` + `intl`, `MaterialApp` с делегатами и `supportedLocales`, пользовательские строки статуса шлюза из ARB. Проверка: `flutter analyze`, `flutter test`, `make flutter-ci`.

## Context

- Docs: [docs/features/i18n.md](../../docs/features/i18n.md), [docs/TESTING.md](../../docs/TESTING.md), канон TDD [`.agent/workflows/tdd-code-workflow/SKILL.md`](../workflows/tdd-code-workflow/SKILL.md).
- Code: [src/frontend/lib/app.dart](../../src/frontend/lib/app.dart), [src/frontend/pubspec.yaml](../../src/frontend/pubspec.yaml), [src/frontend/test/voice_app_test.dart](../../src/frontend/test/voice_app_test.dart).
- Current state: нет `generate`/l10n; строки в `_GatewayStatusBar` захардкожены; в `three_column_shell.dart` синтаксическая ошибка `?header`.
- Constraints: без экрана настроек языка и синка профиля (вне scope дока для каркаса).

## Scope

- In: `pubspec.yaml`, `l10n.yaml`, `lib/l10n/*.arb`, проводка `MaterialApp`, строки статуса шлюза из ARB, виджет-тесты EN/RU, исправление `?header` для зелёного `analyze`.
- Out: ручной выбор языка, профиль, RTL, локализация API по коду.
- Documentation gaps: нет.

## Milestones

- [x] ExecPlan создан.
- [x] Тесты EN/RU на локализованный статус.
- [x] Ревью тестов (критичных разрывов с `i18n.md` нет).
- [x] Реализация + зелёные тесты.
- [x] Ревью реализации + `make flutter-ci`.

## Detailed Steps

1. Прочитаны `docs/features/i18n.md`, `docs/TESTING.md`, план в `.cursor/plans/`.
2. Добавить/обновить виджет-тесты: явная локаль `en` / `ru`, ожидания строк из будущих ARB.
3. Добавить `flutter_localizations`, `intl`, `generate: true`; `l10n.yaml`; ARB; запустить `flutter pub get`.
4. Подключить делегаты и локали в `MaterialApp`; заменить строки в `_GatewayStatusBar`; константа сообщения missing base URL в `gateway_client.dart` для сравнения без магических строк в двух местах (опционально — одна константа).
5. Исправить `three_column_shell.dart`: `if (header != null) header`.
6. `flutter analyze`, `flutter test`, `make flutter-ci`.

## Validation

- [x] `cd d:/Git/Voice/src/frontend ; flutter pub get ; flutter analyze ; flutter test`
- [x] `cd d:/Git/Voice ; make flutter-ci`

## Progress

- [x] ExecPlan файл добавлен.
- [x] Реализация завершена; `make flutter-ci` зелёный.

## Decisions

- Сообщение об отсутствии base URL локализуем через отдельные ключи ARB; остальные тексты ошибок шлюза передаются в ICU-плейсхолдер `detail` на EN/RU префиксе «Gateway»/«Шлюз» (деталь может оставаться на английском из клиента) — минимальная модель без смены контракта `GatewayHealthFailure`.
- Для детерминированных виджет-тестов добавлен опциональный `VoiceApp.locale`; в проде `main.dart` по-прежнему `const VoiceApp()` (системная локаль).
- Сгенерированные `lib/l10n/app_localizations*.dart` закоммичены рядом с ARB (генератор при `flutter pub get` / `flutter gen-l10n` перезаписывает их согласованно).

## Risks And Follow-Ups

- Версия `intl` должна удовлетворять транзитивным ограничениям Flutter SDK.
- При необходимости унифицировать все failure-сообщения через один ARB-ключ в follow-up.
