# Voice Project

Discord-like мессенджер с войс-чатом и встроенным матчмейкингом для геймеров.

## Context Files

- **[PROJECT.md](../docs/PROJECT.md)** — product vision, target audience, matchmaking mechanics
- **[PLAN.md](PLAN.md)** — phased product roadmap with task checklists
- **[OLD_ARCHITECTURE.md](OLD_ARCHITECTURE.md)** — old: service diagram, repos, ports, gRPC contracts, how to run.
- **[FEATURES.md](../docs/FEATURES.md)** — каталог фич с кратким описанием и ссылками на детальные файлы
- **[features/](../docs/features/)** — детальные описания фич по одному файлу на фичу (продуктовый дизайн, UX-решения)
- **[ARCHITECTURE_REQUIREMENTS.md](../docs/ARCHITECTURE_REQUIREMENTS.md)** — кросс-сервисные технические решения: протоколы, инфраструктура, rate limiting, S2S федерация

## Quick Reference

| Service | Tech | Port | Status |
|---------|------|------|--------|
| VoiceAuthService | Java 21 / Spring Boot | 8090 | Working |
| VoiceWebSocketService | Go / gRPC | 24766 | Stub |
| VoiceChannelDataService | Go / gRPC | — | Empty |
| VoiceMessageService | Go / gRPC | — | Empty |
| voiceclient | Flutter | — | Firebase P2P prototype |
