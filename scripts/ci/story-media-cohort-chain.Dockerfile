FROM docker:27-cli AS docker
FROM python:3.12-bookworm
COPY --from=docker /usr/local/bin/docker /usr/local/bin/docker
RUN openssl version
