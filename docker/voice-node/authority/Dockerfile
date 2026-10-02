# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /voice/src/backend/federation
COPY src/backend/federation ./
COPY src/backend/pkg/go.mod src/backend/pkg/go.sum /voice/src/backend/pkg/
RUN --mount=type=cache,target=/go/pkg/mod go mod download
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go test -count=1 ./nodepublisher
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go vet ./nodepublisher ./mediaauthority ./cmd/node-authority ./cmd/node-media
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o /out/voice-node-authority ./cmd/node-authority
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -o /out/node-media ./cmd/node-media
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 voice && adduser -D -H -u 10001 -G voice voice
COPY --from=build /out/voice-node-authority /usr/local/bin/
COPY --from=build /out/node-media /usr/local/bin/
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/voice-node-authority"]
