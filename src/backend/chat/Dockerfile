# syntax=docker/dockerfile:1
# Build context must be `src/backend` (local pb, user pb, messaging events replace).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY chat/go.mod ./chat/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
RUN cd chat && go mod download
COPY user/pb ./user/pb
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY messaging/pb/voice/messaging ./messaging/pb/voice/messaging
COPY chat/pb ./chat/pb
COPY chat ./chat
WORKDIR /src/chat
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV CHAT_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
