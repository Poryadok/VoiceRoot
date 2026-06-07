# syntax=docker/dockerfile:1
# Build context must be `src/backend` (local pb modules and replace on ../social).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY user/go.mod ./user/
COPY user/pb/voice/common/go.mod ./user/pb/voice/common/
COPY user/pb/voice/user/go.mod ./user/pb/voice/user/
COPY user/pb/voice/social/go.mod ./user/pb/voice/social/
COPY social/go.mod ./social/
RUN cd user && go mod download
COPY pkg ./pkg
COPY user/pb ./user/pb
COPY social ./social
COPY user ./user
WORKDIR /src/user
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/service /service
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/service"]
