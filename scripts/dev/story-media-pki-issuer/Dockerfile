FROM python:3.13-slim-bookworm@sha256:a1165e272e578941b84abc79e4ab38a0305cd12803a5c4247979ac7655f4d641

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --uid 10003 --user-group --no-create-home --shell /usr/sbin/nologin issuer \
    && install -d -m 0700 -o 10003 -g 10003 /state

WORKDIR /issuer
COPY --chown=10003:10003 issuer.py /issuer/issuer.py
USER 10003:10003
ENTRYPOINT ["python3", "/issuer/issuer.py"]
