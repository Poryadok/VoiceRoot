# Chat Service

Go service for DM, group and channel lifecycle, membership, per-profile archive/mute state, folders and Quick Access. It exposes gRPC `ChatService`, persists to PostgreSQL `chat_db`, publishes `chat.events`, and is wired through Gateway `/api/v1/chats/**`.

The service is not feature-complete: sticker/GIF catalog support is absent, the documented NATS event surface is partial, and Messaging enrichment degrades to empty preview/unread fields when its S2S dependency is unavailable. Current contracts and gaps are tracked in [Chat Service](../../../docs/microservices/chat-service.md), [PLAN](../../../docs/PLAN.md), and [backend TODO](../../../docs/todo/backend.md). Handler and integration coverage lives under [`internal/grpcsvc`](internal/grpcsvc) and [`internal/store`](internal/store).
