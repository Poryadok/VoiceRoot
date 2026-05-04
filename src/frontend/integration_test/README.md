# Integration tests (device / browser)

End-to-end and `integration_test` scenarios with a real API belong here once the team wires a device target (Windows, Linux desktop, or mobile) in CI — see `docs/TESTING.md` and the `flutter-web-client-testing` skill.

Until then, the repo runs **`flutter test`** over `test/` only (including `e2e_readiness_test.dart` as a layout anchor).
