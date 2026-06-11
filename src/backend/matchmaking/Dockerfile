# syntax=docker/dockerfile:1
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY matchmaking/go.mod ./matchmaking/
COPY voice/pb/voice/matchmaking/go.mod ./voice/pb/voice/matchmaking/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
RUN cd matchmaking && go mod download
COPY pkg ./pkg
COPY voice/pb ./voice/pb
COPY user/pb ./user/pb
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY matchmaking ./matchmaking
WORKDIR /src/matchmaking
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
