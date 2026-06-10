# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb modules under voice/pb and messaging/pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY space/go.mod ./space/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
RUN cd space && go mod download
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY space ./space
WORKDIR /src/space
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV SPACE_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
