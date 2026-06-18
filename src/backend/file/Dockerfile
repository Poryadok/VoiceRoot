# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb, replace on ../chat and user pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY file/go.mod ./file/
COPY file/pb/voice/file/go.mod ./file/pb/voice/file/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY voice/pb/voice/story/go.mod ./voice/pb/voice/story/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh file
COPY pkg ./pkg
COPY file ./file
COPY chat/pb ./chat/pb
COPY voice/pb/voice/story ./voice/pb/voice/story
COPY user/pb ./user/pb
WORKDIR /src/file
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV FILE_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]