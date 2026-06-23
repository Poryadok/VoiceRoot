# Build context must be `src/backend` (includes ../pkg replace in go.mod).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/go.mod ./messaging/
COPY file/pb/voice/file/go.mod ./file/pb/voice/file/
COPY voice/pb/voice/calls/go.mod ./voice/pb/voice/calls/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY voice/pb/voice/notification/go.mod ./voice/pb/voice/notification/
COPY voice/pb/voice/matchmaking/go.mod ./voice/pb/voice/matchmaking/
COPY voice/pb/voice/search/go.mod ./voice/pb/voice/search/
COPY voice/pb/voice/moderation/go.mod ./voice/pb/voice/moderation/
COPY voice/pb/voice/subscription/go.mod ./voice/pb/voice/subscription/
COPY voice/pb/voice/auth/go.mod ./voice/pb/voice/auth/
COPY voice/pb/voice/bot/go.mod ./voice/pb/voice/bot/
COPY voice/pb/voice/story/go.mod ./voice/pb/voice/story/
COPY gateway/go.mod ./gateway/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh gateway
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY chat/pb ./chat/pb
COPY messaging/pb ./messaging/pb
COPY messaging ./messaging
COPY file/pb ./file/pb
COPY voice/pb ./voice/pb
COPY role/pb ./role/pb
COPY gateway ./gateway
WORKDIR /src/gateway
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/gateway /gateway
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/gateway"]
