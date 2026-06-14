# syntax=docker/dockerfile:1
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY moderation/go.mod ./moderation/
COPY voice/pb/voice/moderation/go.mod ./voice/pb/voice/moderation/
COPY voice/pb/voice/auth/go.mod ./voice/pb/voice/auth/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
RUN cd moderation && go mod download
COPY pkg ./pkg
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY user/pb/voice/common ./user/pb/voice/common
COPY user/pb/voice/user ./user/pb/voice/user
COPY voice/pb/voice/auth ./voice/pb/voice/auth
COPY voice/pb/voice/moderation ./voice/pb/voice/moderation
COPY moderation ./moderation
WORKDIR /src/moderation
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
