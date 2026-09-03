# Realtime Service

Go service for WebSocket `/ws`, sequenced live event fan-out, chat subscriptions, typing/read/delivery signals, NATS consumers, and Redis cross-instance delivery. Gateway proxies `/ws`; the service validates JWT on upgrade (or consumes the upstream Gateway ticket) and does not use PostgreSQL.

`resume` starts a new sequence stream and does not replay missed events; message catch-up belongs to Messaging REST per chat. Group/space subscription bootstrap, parts of presence fan-out, metrics, and newer-path coverage remain partial. See [Realtime Service](../../../docs/microservices/realtime-service.md), [PLAN](../../../docs/PLAN.md), and [backend TODO](../../../docs/todo/backend.md). Tests are colocated in this module, including WebSocket, consumer, Redis fan-out, and integration coverage.
