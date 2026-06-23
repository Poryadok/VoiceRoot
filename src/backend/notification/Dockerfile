# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY notification/go.mod ./notification/
COPY voice/pb/voice/notification/go.mod ./voice/pb/voice/notification/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh notification
COPY pkg ./pkg
COPY voice/pb ./voice/pb
COPY messaging/pb ./messaging/pb
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY user/pb ./user/pb
COPY notification ./notification
WORKDIR /src/notification
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
