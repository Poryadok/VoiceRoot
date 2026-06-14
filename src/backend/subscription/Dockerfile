# syntax=docker/dockerfile:1
# Build context must be `src/backend`.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY subscription/go.mod ./subscription/
COPY voice/pb/voice/subscription/go.mod ./voice/pb/voice/subscription/
COPY voice/pb/voice/space/go.mod ./voice/pb/voice/space/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY chat/pb/voice/chat/go.mod ./chat/pb/voice/chat/
RUN cd subscription && go mod download
COPY pkg ./pkg
COPY user/pb/voice/common ./user/pb/voice/common
COPY chat/pb/voice/chat ./chat/pb/voice/chat
COPY voice/pb/voice/space ./voice/pb/voice/space
COPY voice/pb/voice/subscription ./voice/pb/voice/subscription
COPY subscription ./subscription
COPY migrations/subscription_db ./migrations/subscription_db
WORKDIR /src/subscription
ENV SUBSCRIPTION_MIGRATION_PATH=/src/migrations/subscription_db/000001_init.up.sql
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
COPY --from=build /src/migrations/subscription_db /migrations/subscription_db
ENV SUBSCRIPTION_MIGRATION_PATH=/migrations/subscription_db/000001_init.up.sql
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
