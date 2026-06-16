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
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh social
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY social ./social
WORKDIR /src/social
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
