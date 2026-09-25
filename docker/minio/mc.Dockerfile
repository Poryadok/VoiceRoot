# Repackage the official MinIO mc release binary so CI does not depend on an
# anonymous pull from the upstream minio/mc container repository.
FROM alpine:3.24.2@sha256:d56c381f961d307a21b3ca004cf1e3910f106644aefb1f43e654c8a56c4fd395 AS verify

ARG MC_RELEASE=RELEASE.2025-08-13T08-35-41Z
ARG MC_SHA256=01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891
# This official MinIO release key is published in minio/mc's Dockerfile.
ARG MINISIGN_PUBLIC_KEY=RWTx5Zr1tiHQLwG9keckT0c45M3AGeHD6IvimQHpyRywVWGbP1aVSGav

RUN apk add --no-cache ca-certificates-bundle=20260909-r0 curl=8.22.0-r0 minisign=0.12-r2 \
    && asset="mc.linux-amd64.${MC_RELEASE}" \
    && base="https://github.com/minio/mc/releases/download/${MC_RELEASE}" \
    && curl --fail --show-error --silent --location "${base}/${asset}" --output "/tmp/${asset}" \
    && curl --fail --show-error --silent --location "${base}/${asset}.sha256sum" --output "/tmp/${asset}.sha256sum" \
    && curl --fail --show-error --silent --location "${base}/${asset}.minisig" --output "/tmp/${asset}.minisig" \
    && printf '%s  %s\n' "${MC_SHA256}" "/tmp/${asset}" | sha256sum -cs \
    && grep -Fq "${MC_SHA256}" "/tmp/${asset}.sha256sum" \
    && minisign -Vqm "/tmp/${asset}" -x "/tmp/${asset}.minisig" -P "${MINISIGN_PUBLIC_KEY}" \
    && install -D -m 0755 "/tmp/${asset}" /out/mc

FROM alpine:3.24.2@sha256:d56c381f961d307a21b3ca004cf1e3910f106644aefb1f43e654c8a56c4fd395

ENV HOME=/root
COPY --from=verify /out/mc /usr/local/bin/mc
ENTRYPOINT ["mc"]
CMD ["--help"]
