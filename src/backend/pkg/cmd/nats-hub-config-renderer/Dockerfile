FROM golang:1.26-alpine AS build
WORKDIR /src/backend/pkg
COPY src/backend/pkg/go.mod src/backend/pkg/go.sum ./
RUN go mod download
COPY src/backend/pkg/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/nats-hub-config-renderer ./cmd/nats-hub-config-renderer

FROM nats@sha256:b270f5e2428354c0335612694d7dd2fb588148e567a5757fdff325ef9c9332e6 AS nats-server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/nats-hub-config-renderer /nats-hub-config-renderer
COPY --from=nats-server /nats-server /nats-server
USER 10000:10000
ENTRYPOINT ["/nats-hub-config-renderer"]
