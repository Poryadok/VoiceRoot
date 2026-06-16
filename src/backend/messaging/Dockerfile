# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb, replace on ../chat and user pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY messaging/go.mod ./messaging/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY file/pb/voice/file/go.mod ./file/pb/voice/file/
COPY chat/go.mod ./chat/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY role/go.mod ./role/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY voice/pb/voice/moderation/go.mod ./voice/pb/voice/moderation/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh messaging
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY file/pb ./file/pb
COPY role/pb ./role/pb
COPY voice/pb/voice/moderation ./voice/pb/voice/moderation
COPY role/permissions ./role/permissions
COPY role ./role
COPY messaging/pb ./messaging/pb
COPY chat/pb ./chat/pb
COPY chat ./chat
COPY messaging ./messaging
WORKDIR /src/messaging
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV MESSAGING_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
