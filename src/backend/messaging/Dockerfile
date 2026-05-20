# syntax=docker/dockerfile:1
# Build context must be `src/backend` (pb, replace on ../chat and user pb).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY messaging/go.mod ./messaging/
COPY messaging/pb/voice/messaging/go.mod ./messaging/pb/voice/messaging/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
COPY chat/go.mod ./chat/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
RUN cd messaging && go mod download
COPY user/pb ./user/pb
COPY messaging/pb ./messaging/pb
COPY chat/pb ./chat/pb
COPY chat ./chat
COPY messaging ./messaging
WORKDIR /src/messaging
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080 9090
ENV LISTEN_ADDR=:8080
ENV MESSAGING_GRPC_LISTEN=:9090
ENTRYPOINT ["/service"]
