# User Service

Go implementation for the Voice user service (profiles against `user_db` v1 DDL).

Public surface:

- GET `/health` — `{"service":"user","status":"ok"}`.
- gRPC on `USER_GRPC_ADDR` (default `:9090`) when `DATABASE_URL` is set — `UserService` profile RPCs: `GetProfile`, `GetProfiles`, `UpdateProfile`, `CreateProfile` (plus other RPCs still unimplemented). Caller identity for mutating calls: metadata `x-voice-user-id` (account UUID), aligned with Gateway downstream headers.

Protobuf sources live under `protos/`; committed codegen for this module is under `pb/voice/` (nested Go modules). Regenerate from repo root with `buf generate --template buf.gen.local.yaml` (see root `buf.gen.yaml` / `buf.gen.local.yaml`) and sync `pb/` if contracts change.
