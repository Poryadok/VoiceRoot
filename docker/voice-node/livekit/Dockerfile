FROM golang:1.26-alpine AS build
RUN apk add --no-cache python3 git build-base
WORKDIR /source
COPY livekit-v1.8.4.tar.gz protocol-v1.34.0.tar.gz upstream.sha256 ./
RUN sha256sum -c upstream.sha256 && tar -xzf livekit-v1.8.4.tar.gz && tar -xzf protocol-v1.34.0.tar.gz
COPY authority /voice-authority
COPY prepare.py voice_authority.go /adapter/
RUN python3 /adapter/prepare.py /source/livekit-1.8.4 /source/protocol-1.34.0 /voice-authority /adapter/voice_authority.go
WORKDIR /source/livekit-1.8.4
RUN go mod edit -replace github.com/livekit/protocol=../protocol-1.34.0
RUN gofmt -w pkg/service/roommanager.go pkg/service/voice_authority.go pkg/routing/node.go ../protocol-1.34.0/auth/grants.go pkg/voiceauthority
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build go test -mod=mod ./pkg/voiceauthority/...
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -mod=mod -trimpath -o /out/livekit-server ./cmd/server
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 voice && adduser -D -u 10001 -G voice voice
COPY --from=build /out/livekit-server /usr/local/bin/livekit-server
COPY --from=build /source/livekit-1.8.4/LICENSE /source/livekit-1.8.4/NOTICE /usr/share/licenses/livekit-server/
COPY --from=build /source/protocol-1.34.0/LICENSE /source/protocol-1.34.0/NOTICE /usr/share/licenses/livekit-protocol/
COPY --from=build /source/livekit-1.8.4/pkg/sfu/NOTICE /usr/share/licenses/livekit-sfu/NOTICE
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/livekit-server"]
