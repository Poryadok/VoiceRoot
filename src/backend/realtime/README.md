# Realtime Service

Go microservice: WebSocket `/ws`, event fanout, resume (`s` / `hello`), Redis Pub/Sub between instances. Gateway proxies client `/ws`; JWT validated on upgrade (or upstream JWT from Gateway ticket). No PostgreSQL. See `docs/microservices/realtime-service.md`.
