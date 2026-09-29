# Build context: src/backend.
FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY file/pb/voice/file/go.mod ./file/pb/voice/file/
COPY voice/pb/voice/calls/go.mod ./voice/pb/voice/calls/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY voice/pb/voice/story/go.mod ./voice/pb/voice/story/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY gameintegration/go.mod ./gameintegration/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh gameintegration
COPY pkg ./pkg
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY file/pb/voice/file ./file/pb/voice/file
COPY voice/pb/voice/calls ./voice/pb/voice/calls
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY voice/pb/voice/story ./voice/pb/voice/story
COPY role/pb/voice/role ./role/pb/voice/role
COPY user/pb/voice/common ./user/pb/voice/common
COPY gameintegration ./gameintegration
WORKDIR /src/gameintegration
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
