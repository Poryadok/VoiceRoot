
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY analytics/pb/voice/analytics/go.mod ./analytics/pb/voice/analytics/
COPY analytics/go.mod ./analytics/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh analytics
COPY pkg ./pkg
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY analytics ./analytics
WORKDIR /src/analytics
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV ANALYTICS_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
