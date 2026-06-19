# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb modules under user/pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY social/go.mod ./social/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY voice/pb/voice/auth/go.mod ./voice/pb/voice/auth/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh social
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY voice/pb/voice/auth ./voice/pb/voice/auth
COPY social ./social
WORKDIR /src/social
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
