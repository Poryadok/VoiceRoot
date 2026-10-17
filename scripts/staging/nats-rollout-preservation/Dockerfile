# Independently built trusted helper image. Its inputs come only from this
# pinned source closure, never from an application candidate archive.
FROM --platform=linux/amd64 golang@sha256:d9c68c2c51161e12fd77e4c6320687c9cd86e1af1e3ad6e6cd63ff970641453c AS kernel-build
WORKDIR /src
COPY scripts/staging/nats-known-baseline/go.mod scripts/staging/nats-known-baseline/go.sum ./
RUN go mod download
COPY scripts/staging/nats-known-baseline/*.go ./
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=readonly -trimpath -o /out/kernel .

FROM --platform=linux/amd64 natsio/nats-box@sha256:abdc9f9f0120bb8adfbf674eb037d1551db55356eb198b7bd4ffed377f6950a6 AS nats-tools

FROM --platform=linux/amd64 nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb AS trusted-helper
USER root
RUN apk add --no-cache python3
WORKDIR /voice-nats-runner
COPY --from=nats-tools /usr/local/bin/nats /usr/local/bin/nats
COPY --from=nats-tools /usr/bin/jq /usr/bin/jq
COPY --from=nats-tools /usr/lib/libjq.so.1 /usr/lib/libjq.so.1
COPY --from=nats-tools /usr/lib/libonig.so.5 /usr/lib/libonig.so.5
COPY scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004/*.sh /voice-nats-runner/bootstrap/
COPY --from=kernel-build /out/kernel /voice-nats-runner/kernel
COPY scripts/staging/nats-rollout-preservation/nats-phase /voice-nats-runner/phase
COPY scripts/staging/nats-rollout-preservation/nats-supervisor /voice-nats-runner/nats-supervisor
COPY scripts/staging/nats-rollout-preservation/nats_contract_plan.py /voice-nats-runner/nats_contract_plan.py
COPY scripts/staging/nats-rollout-preservation/nats_contract_actor.py /voice-nats-runner/nats_contract_actor.py
COPY scripts/staging/nats-rollout-preservation/nats_contract_runner.py /voice-nats-runner/nats_contract_runner.py
COPY scripts/staging/nats-rollout-preservation/native_store.py /voice-nats-runner/native_store.py
COPY scripts/staging/nats-known-baseline/controller.py /voice-nats-runner/controller.py
RUN mkdir -p /data /verify /out /inputs /state /voice-nats-runner/bootstrap \
 && chmod 0555 /voice-nats-runner/bootstrap/*.sh \
 && chmod 0555 /voice-nats-runner/nats-supervisor \
 && chown 10000:10000 /data /verify /out /inputs /state \
 && chmod 0700 /data /verify /out /inputs /state
ENV PYTHONPATH=/voice-nats-runner PYTHONDONTWRITEBYTECODE=1
USER 10000:10000
ENTRYPOINT ["/usr/bin/python3", "/voice-nats-runner/phase"]
