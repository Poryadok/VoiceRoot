# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb modules under role/pb and chat/pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY role/go.mod ./role/
COPY role/pb/voice/role/go.mod ./role/pb/voice/role/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh role
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY role/pb ./role/pb
COPY role ./role
WORKDIR /src/role
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV ROLE_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
