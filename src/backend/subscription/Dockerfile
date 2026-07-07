# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY subscription/go.mod ./subscription/
COPY analytics/pb/voice/analytics/go.mod ./analytics/pb/voice/analytics/
COPY voice/pb/voice/subscription/go.mod ./voice/pb/voice/subscription/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh subscription
COPY pkg ./pkg
COPY analytics/pb/voice/analytics ./analytics/pb/voice/analytics
COPY user/pb/voice/common ./user/pb/voice/common
COPY user/pb/voice/user ./user/pb/voice/user
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY voice/pb/voice/subscription ./voice/pb/voice/subscription
COPY subscription ./subscription
COPY migrations/subscription_db ./migrations/subscription_db
WORKDIR /src/subscription
ENV SUBSCRIPTION_MIGRATION_PATH=/src/migrations/subscription_db/000001_init.up.sql
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
COPY --from=build /src/migrations/subscription_db /migrations/subscription_db
ENV SUBSCRIPTION_MIGRATION_PATH=/migrations/subscription_db/000001_init.up.sql
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
