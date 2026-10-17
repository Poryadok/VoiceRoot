FROM python:3.12-slim@sha256:dddfd7e07f9d15aeeca61529320492139d21cac7f0070c00609243e51e4e0016

WORKDIR /voice-nats-runner
COPY --chown=65532:65532 nats-known-baseline/controller.py ./controller.py
COPY --chown=65532:65532 nats-rollout-preservation/native_store.py ./native_store.py
COPY --chown=65532:65532 nats-rollout-preservation/nats-phase ./phase
RUN chmod 0444 controller.py native_store.py && chmod 0555 phase
USER 65532:65532
ENTRYPOINT ["/voice-nats-runner/phase"]
