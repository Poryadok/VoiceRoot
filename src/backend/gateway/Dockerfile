# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway .

FROM alpine:3.20
RUN adduser -D -H -u 65532 nonroot
USER nonroot:nonroot
WORKDIR /
COPY --from=build /out/gateway /gateway
EXPOSE 8080
ENV LISTEN_ADDR=:8080
ENTRYPOINT ["/gateway"]
