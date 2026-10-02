FROM golang:1.26-alpine AS build
WORKDIR /voice/tests/federation-media
COPY tests/federation-media ./
COPY src/backend/federation /voice/src/backend/federation
COPY src/backend/pkg/go.mod src/backend/pkg/go.sum /voice/src/backend/pkg/
RUN go mod download
RUN CGO_ENABLED=0 go build -mod=mod -o /out/fixture-init ./cmd/fixture
RUN CGO_ENABLED=0 go test -mod=mod -c -o /out/media-test .
RUN cd /voice/src/backend/federation && CGO_ENABLED=0 go build -mod=mod -o /out/node-authority ./cmd/node-authority
FROM alpine:3.23
RUN apk add --no-cache ca-certificates
COPY --from=build /out/fixture-init /out/media-test /out/node-authority /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/media-test"]
