# Build context: src/backend. This image runs tests only and is not a service image.
FROM golang:1.26-alpine
LABEL org.voice.deployable="false"
WORKDIR /src
ENV GOPROXY=https://proxy.golang.org,direct
COPY scripts/docker-go-mod-download.sh /usr/local/bin/docker-go-mod-download.sh
COPY pkg/go.mod ./pkg/
COPY controlledgame/go.mod controlledgame/go.sum ./controlledgame/
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked sh /usr/local/bin/docker-go-mod-download.sh controlledgame
COPY pkg ./pkg
COPY controlledgame/*.go ./controlledgame/
WORKDIR /src/controlledgame
RUN --mount=type=cache,target=/go/pkg/mod,id=voice-gomod,sharing=locked --mount=type=cache,target=/root/.cache/go-build,id=voice-gobuild,sharing=locked CGO_ENABLED=0 go test -short ./...
CMD ["go", "test", "-short", "./..."]
