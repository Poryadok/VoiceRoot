# syntax=docker/dockerfile:1
# Build context must be `src/backend` (local pb, pkg and cross-service generated modules).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY voice/pb/voice/calls/go.mod ./voice/pb/voice/calls/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY voice/go.mod voice/go.sum ./voice/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh voice
COPY pkg ./pkg
COPY chat/pb ./chat/pb
COPY user/pb ./user/pb
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY messaging/pb/voice/messaging ./messaging/pb/voice/messaging
COPY voice ./voice
WORKDIR /src/voice
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV VOICE_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]