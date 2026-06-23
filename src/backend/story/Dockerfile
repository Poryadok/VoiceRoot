# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY story/go.mod ./story/
COPY voice/pb/voice/story/go.mod ./voice/pb/voice/story/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY file/pb/voice/file/go.mod ./file/pb/voice/file/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY voice/pb/voice/subscription/go.mod ./voice/pb/voice/subscription/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh story
COPY pkg ./pkg
COPY user/pb/voice/common ./user/pb/voice/common
COPY user/pb/voice/social ./user/pb/voice/social
COPY user/pb/voice/user ./user/pb/voice/user
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY file/pb/voice/file ./file/pb/voice/file
COPY messaging/pb/voice/messaging ./messaging/pb/voice/messaging
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY voice/pb/voice/story ./voice/pb/voice/story
COPY voice/pb/voice/subscription ./voice/pb/voice/subscription
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY story ./story
COPY migrations/story_db ./migrations/story_db
WORKDIR /src/story
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN apk add --no-cache wget
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV STORY_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
