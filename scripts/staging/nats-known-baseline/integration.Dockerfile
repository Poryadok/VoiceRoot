FROM docker:27.4.0-cli AS dockercli
FROM python:3.12-slim@sha256:dddfd7e07f9d15aeeca61529320492139d21cac7f0070c00609243e51e4e0016
COPY --from=dockercli /usr/local/bin/docker /usr/bin/docker
