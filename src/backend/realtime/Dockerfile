# syntax=docker/dockerfile:1
# Build context must be `src/backend` (includes ../pkg replace in go.mod).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY realtime/go.mod ./realtime/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY messaging/pb/voice/events/go.mod ./messaging/pb/voice/events/
RUN cd realtime && go mod download
COPY pkg ./pkg
COPY messaging/pb/voice/events ./messaging/pb/voice/events
COPY realtime ./realtime
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY user/pb/voice/common ./user/pb/voice/common
WORKDIR /src/realtime
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
