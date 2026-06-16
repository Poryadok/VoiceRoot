# syntax=docker/dockerfile:1
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod pkg/go.sum ./pkg/
COPY search/go.mod search/go.sum ./search/
COPY voice/pb/voice/search/go.mod ./voice/pb/voice/search/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/common/go.mod user/pb/voice/common/go.sum ./user/pb/voice/common/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh search
COPY pkg ./pkg
COPY voice/pb ./voice/pb
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY messaging/pb/voice/messaging ./messaging/pb/voice/messaging
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY role/pb/voice/role ./role/pb/voice/role
COPY user/pb/voice/social ./user/pb/voice/social
COPY user/pb ./user/pb
COPY search ./search
WORKDIR /src/search
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
