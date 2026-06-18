# syntax=docker/dockerfile:1
# Build context must be `src/backend` (local pb modules and replace on ../social).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY user/go.mod ./user/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY social/go.mod ./social/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh user
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY social ./social
COPY user ./user
WORKDIR /src/user
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
