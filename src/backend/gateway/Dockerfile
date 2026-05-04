# syntax=docker/dockerfile:1
# Build context must be `src/backend` (includes ../pkg replace in go.mod).

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY pkg/go.mod ./pkg/
COPY gateway/go.mod ./gateway/
RUN cd gateway && go mod download
COPY pkg ./pkg
COPY gateway ./gateway
WORKDIR /src/gateway
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/gateway /gateway
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/gateway"]
