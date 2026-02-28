# Voice Project

Discord-like мессенджер с войс-чатом и встроенным матчмейкингом для геймеров.

## Context Files

- **[PROJECT.md](PROJECT.md)** — product vision, target audience, matchmaking mechanics
- **[PLAN.md](PLAN.md)** — phased product roadmap with task checklists
- **[ARCHITECTURE.md](ARCHITECTURE.md)** — service diagram, repos, ports, gRPC contracts, how to run

## Quick Reference

| Service | Tech | Port | Status |
|---------|------|------|--------|
| VoiceAuthService | Java 21 / Spring Boot | 8090 | Working |
| VoiceWebSocketService | Go / gRPC | 24766 | Stub |
| VoiceChannelDataService | Go / gRPC | — | Empty |
| VoiceMessageService | Go / gRPC | — | Empty |
| voiceclient | Flutter | — | Firebase P2P prototype |
