---
name: flutter-web-client-testing
description: >-
  Plans and runs Flutter web client checks: pub get, analyze, unit/widget tests,
  optional web-target and integration_test against local or staging API,
  accessibility and semantics. Use when testing the Voice Flutter app, веб-клиент
  на Flutter, прогон тестов фронта, CI-паритет make flutter-ci, или настройка
  integration_test / widget-тестов для web.
---

# Тестирование Flutter веб-клиента

Скилл для **проверки и доработки тестового контура** Flutter-приложения с упором на **web** (в репозитории — `src/frontend/`). Не подменяет `tdd-code-workflow` при явном запросе TDD-канона.

## Когда применять

- Перед PR, после фичи, при падении CI job `flutter`, при вопросах «как покрыть веб» / «почему падает только в браузере».
- Настройка или аудит **`integration_test`** при доступном API (локальный compose или staging) — см. `docs/TESTING.md`.

## Входные данные

1. **Корень пакета**: обычно `src/frontend/`.
2. **Цель**: только виджеты/юнит, или ещё **web-сборка**, **integration** с бэкендом, **a11y**.
3. **Версия Flutter**: согласовать с CI (`.github/workflows/ci.yml`, поле `flutter-version`) и с `environment.sdk` в `pubspec.yaml`.

В PowerShell цепочки команд: **`;`**, не `&&`.

---

## Команды (базовый конвейер)

Выполнять из `src/frontend/` (или из корня через `make flutter-ci`).

| Цель | Команда |
|------|---------|
| Зависимости | `flutter pub get` |
| Статический анализ | `flutter analyze` |
| Юнит и виджет-тесты (VM по умолчанию) | `flutter test` |
| Один файл / тест | `flutter test test/foo_test.dart` ; `flutter test --name "pattern"` |
| Покрытие (если нужно отчётом) | `flutter test --coverage` ; затем при необходимости `lcov` / IDE |
| Сборка web (компиляция, tree-shaking) | `flutter build web` — ловит ошибки, которые `test` на VM может не увидеть |
| Тесты на движке Chrome | `flutter test --platform chrome` — для кода, завязанного на web API / layout в браузере (медленнее, нужен Chrome) |
| Монорепо Voice (как CI) | из корня: **`make flutter-ci`** → `flutter pub get`, `analyze`, `test` в `src/frontend/` |

`flutter run -d chrome` — ручная проверка UI; в скилле как дополнение к автотестам, не замена им.

---

## 1. Статический анализ и стиль

- [ ] **`flutter analyze`** без ошибок и без игноров «на весь файл» без причины.
- [ ] **`flutter_lints`** (или принятый набор) в `analysis_options.yaml` соблюдён; новые `ignore` с комментарием «почему».
- [ ] **`dart format`** / формат в IDE согласован с командой; дифф без шума.

---

## 2. Юнит-тесты и логика (не UI)

- [ ] Чистая логика (парсинг, маппинг DTO, валидация) вынесена туда, где **тест без `WidgetTester`** дешевле и стабильнее.
- [ ] Зависимости от **`http`** / времени / `Random` — через **инъекцию** или fakes; не бить реальный сеть в юнитах без mock `Client`.
- [ ] Для **Riverpod** (в этом репо): `ProviderContainer` в тестах, `overrideWith` для изолированных провайдеров; после теста `container.dispose()` где уместно.

---

## 3. Виджет-тесты (`flutter test`)

- [ ] **`pumpWidget` / `pumpAndSettle`** используются осмысленно; нет бесконечного settle из-за вечных анимаций/таймеров — при необходимости моки часов или `fake_async`.
- [ ] Поиск виджетов: **`find.byType` / `find.text` / `find.byKey`**; для i18n — ключи/locale в тесте, а не хрупкая привязка к одной локали без `Localizations` в дереве.
- [ ] **`MediaQuery`** / размер экрана: для web проверить **несколько ширин** (mobile / tablet / desktop), если ломается layout.
- [ ] **Золотые тесты** (`matchesGoldenFile`): только при принятом процессе (платформа, шрифты, CI-артефакты); иначе не требовать как обязательные.

---

## 4. Специфика web

- [ ] Периодически или в CI-джобе расширения: **`flutter build web`** — отдельный класс ошибок (dart2js/wasm, импорты `dart:html`, условный импорт `kIsWeb`).
- [ ] Код с **`dart:html` / JS interop**: покрытие сценариями на **`--platform chrome`** или выделенными тестами под `kIsWeb`, если VM-тесты их обходят.
- [ ] **CORS, origin, cookies, storage**: автотесты ограничены; критичные пути — `integration_test` или чеклист ручной проверки + дока.
- [ ] **Router (go_router и т.д.)**: навигация и deep links проверяются тестом с MaterialApp/`routerConfig`, а не только ручным кликом.

---

## 5. Integration tests (`integration_test`)

По `docs/TESTING.md`: сценарии с бэкендом — когда API доступен (staging или локальный compose).

- [ ] Драйвер и **`integration_test`** пакет подключены в `pubspec.yaml`; тесты в `integration_test/` (или принятом пути).
- [ ] **Web-драйвер**: устройство `web-server` / `chrome` — версия Flutter и документация проекта; **ChromeDriver** совместим с Chrome на машине/CI.
- [ ] **Базовый URL** API через `--dart-define` или flavor; нет хардкода секретов.
- [ ] Тесты **идемпотентны** по данным (тестовый аккаунт, префиксы) или используют изолированный стенд.
- [ ] Таймауты и **повторы** только осознанно; стабильнее устранить гонки, чем `sleep`.

---

## 6. Доступность (a11y)

- [ ] Семантика: **`Semantics`**, подписи кнопок/полей, порядок фокуса для клавиатуры — по чеклистам в **`docs/features/accessibility.md`** (если файл есть в репо).
- [ ] В тестах: **`tester.ensureSemantics()`**, при необходимости проверка **`matchesSemantics`** / поиск по `Semantics` label.
- [ ] Контраст и масштаб — по продуктовой доке; автоматизация ограничена, ручной слой отметить в отчёте.

---

## 7. Локализация (`flutter gen-l10n`)

В проекте `flutter: generate: true` — перед тестами, зависящими от строк:

- [ ] **`flutter gen-l10n`** (или неявно через `pub get` / build) актуален; после смены ARB — нет устаревших вызовов.
- [ ] Тесты под несколько **Locale** при критичных user-facing строках.

---

## 8. Качество тестов как кода

- [ ] Имена тестов отражают поведение; один сценарий — один **`testWidgets`** / **`test`** с понятным описанием.
- [ ] Нет дублирования **огромных** `setUp`; общие фикстуры — helper / `TestWidgetsFlutterBinding`.
- [ ] **`flutter test` зелёный** локально и в том же режиме, что CI (без забытых флагов).

---

## 9. Паритет с CI и окружением Voice

- [ ] Вызван минимум: **`flutter pub get`**, **`flutter analyze`**, **`flutter test`** (как в `.github/workflows/ci.yml`).
- [ ] Полный локальный аналог CI-цепочки фронта: **`make flutter-ci`** из корня репозитория.
- [ ] Бэкенд для интеграции: **`docker compose`** / `docs/PLAN.md`, `docs/DEPLOYMENT.md` — не выдумывать URL.

---

## Шаблон отчёта

```markdown
## Flutter web — проверка тестов: <кратко задача>

**Вердикт:** OK / OK с замечаниями / Не OK

### Команды
- `flutter analyze`: OK/FAIL
- `flutter test`: OK/FAIL
- `flutter build web` / `flutter test --platform chrome`: OK/FAIL/N/A
- `integration_test`: OK/FAIL/N/A (стенд: …)

### Замечания
1. ...

### Пробелы (стенд, секреты, flaky)
- ...

### Следующие шаги
- ...
```

---

## Связь с другими скиллами

- **`tdd-code-workflow`** — если пользователь явно требует полный TDD-канон.
- **`go-microservice-task-evaluation`** / **`java-microservice-task-evaluation`** — если задача сквозная (API + клиент): сначала контракт бэкенда, потом клиент.

---

## Источники истины (Voice)

| Тема | Файл |
|------|------|
| Flutter в CI, analyze/test | `docs/TESTING.md` |
| Фаза клиента, состав | `docs/PLAN.md` |
| Доступность | `docs/features/accessibility.md` (если есть) |
| Выкат / окружения | `docs/DEPLOYMENT.md` |

Порог **минимального % покрытия** в репозитории **не задан** (`docs/TESTING.md`); оценивать **смысл** тестов на новую логику и известные edge cases.
