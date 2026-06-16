# syntax=docker/dockerfile:1
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
RUN --mount=type=cache,target=/go/pkg/mod sh /usr/local/bin/docker-go-mod-download.sh story
COPY pkg ./pkg
COPY user/pb/voice/common ./user/pb/voice/common
COPY voice/pb/voice/story ./voice/pb/voice/story
COPY user/pb/voice/social ./user/pb/voice/social
COPY story ./story
COPY migrations/story_db ./migrations/story_db
WORKDIR /src/story
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

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
