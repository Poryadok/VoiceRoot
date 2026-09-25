# Repackage the exact official MinIO release: the legacy dl.min.io archive is
# no longer served, while the signed GitHub release assets remain available.
FROM alpine:3.24.2@sha256:d56c381f961d307a21b3ca004cf1e3910f106644aefb1f43e654c8a56c4fd395 AS verify

ARG MINIO_RELEASE=RELEASE.2024-12-18T13-15-44Z
ARG MINIO_SHA256=88182336ab5793488f2caad54846056b330f0dcb5c488b60d52434beeebae3ca
# This official MinIO release key is published in minio/minio's Dockerfile.release.
ARG MINISIGN_PUBLIC_KEY=RWTx5Zr1tiHQLwG9keckT0c45M3AGeHD6IvimQHpyRywVWGbP1aVSGav

RUN apk add --no-cache ca-certificates-bundle=20260909-r0 curl=8.22.0-r0 minisign=0.12-r2 \
    && asset="minio.linux-amd64.${MINIO_RELEASE}" \
    && base="https://github.com/minio/minio/releases/download/${MINIO_RELEASE}" \
    && curl --fail --show-error --silent --location "${base}/${asset}" --output "/tmp/${asset}" \
    && curl --fail --show-error --silent --location "${base}/${asset}.sha256sum" --output "/tmp/${asset}.sha256sum" \
    && curl --fail --show-error --silent --location "${base}/${asset}.minisig" --output "/tmp/${asset}.minisig" \
    && grep -Fq "${MINIO_SHA256}" "/tmp/${asset}.sha256sum" \
    && printf '%s  %s\n' "${MINIO_SHA256}" "/tmp/${asset}" | sha256sum -cs \
    && minisign -Vqm "/tmp/${asset}" -x "/tmp/${asset}.minisig" -P "${MINISIGN_PUBLIC_KEY}" \
    && install -D -m 0755 "/tmp/${asset}" /out/minio \
    && mkdir -p /out/licenses \
    && curl --fail --show-error --silent --location https://raw.githubusercontent.com/minio/minio/16f8cf1c52f0a77eeb8f7565aaf7f7df12454583/LICENSE --output /out/licenses/LICENSE \
    && curl --fail --show-error --silent --location https://raw.githubusercontent.com/minio/minio/16f8cf1c52f0a77eeb8f7565aaf7f7df12454583/CREDITS --output /out/licenses/CREDITS

FROM alpine:3.24.2@sha256:d56c381f961d307a21b3ca004cf1e3910f106644aefb1f43e654c8a56c4fd395

ARG MINIO_RELEASE=RELEASE.2024-12-18T13-15-44Z
LABEL org.opencontainers.image.source="https://github.com/minio/minio" \
      org.opencontainers.image.revision="16f8cf1c52f0a77eeb8f7565aaf7f7df12454583" \
      org.opencontainers.image.version="${MINIO_RELEASE}" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

ENV MINIO_UPDATE_MINISIGN_PUBKEY=RWTx5Zr1tiHQLwG9keckT0c45M3AGeHD6IvimQHpyRywVWGbP1aVSGav

RUN apk add --no-cache ca-certificates-bundle=20260909-r0 curl=8.22.0-r0
COPY --from=verify /out/minio /usr/local/bin/minio
COPY --from=verify /out/licenses/ /licenses/

EXPOSE 9000 9001
VOLUME ["/data"]
ENTRYPOINT ["minio"]
CMD ["--version"]
