# Frontend (Flutter)

Voice client (Flutter Web/Desktop/Mobile). Roadmap: [docs/PLAN.md](../../docs/PLAN.md).

## Phase 0 shell

- Three-column desktop shell ([docs/features/navigation.md](../../docs/features/navigation.md)); narrow width stacks the same regions.
- API base URL: `--dart-define=VOICE_API_BASE_URL=https://your-gateway` (empty = offline / tests inject via Riverpod).
- Riverpod + thin Gateway client: `GET /health`, `GET /api/v1/version`.

## Commands

```bash
cd src/frontend
flutter pub get
flutter analyze
flutter test
```

Из корня репозитория (нужен Flutter на `PATH`): **`make flutter-ci`** — те же шаги ([Makefile](../../Makefile)).

CI: [.github/workflows/ci.yml](../../.github/workflows/ci.yml) job `flutter`. Цель **`make build-all`** Flutter не запускает — см. [docs/TESTING.md](../../docs/TESTING.md).
