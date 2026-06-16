# syntax=docker/dockerfile:1
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY bot/go.mod ./bot/
COPY voice/pb/voice/bot/go.mod ./voice/pb/voice/bot/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh bot
COPY pkg ./pkg
COPY user/pb/voice/common ./user/pb/voice/common
COPY user/pb/voice/user ./user/pb/voice/user
COPY voice/pb/voice/bot ./voice/pb/voice/bot
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY messaging/pb/voice/messaging ./messaging/pb/voice/messaging
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY role/pb/voice/role ./role/pb/voice/role
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY bot ./bot
COPY migrations/bot_db ./migrations/bot_db
WORKDIR /src/bot
ENV BOT_MIGRATION_PATH=/src/migrations/bot_db/000001_init.up.sql
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
COPY --from=build /src/migrations/bot_db /migrations/bot_db
ENV BOT_MIGRATION_PATH=/migrations/bot_db/000001_init.up.sql
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV BOT_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
