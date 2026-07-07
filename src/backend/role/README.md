# Role Service

Go scaffold for the Voice role service.

Current public surface:

- GET /health returns {"service":"role","status":"ok"}.

app stack0 custom roles: full gRPC surface, 42 permission bits, per-role chat/voice overrides, default join role. See `docs/microservices/role-service.md`.