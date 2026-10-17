FROM python:3.13-slim-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10002 issuer \
    && useradd --uid 10002 --gid 10002 --no-create-home --shell /usr/sbin/nologin issuer \
    && install -d -m 0700 -o 10002 -g 10002 /state

WORKDIR /issuer
COPY --chown=10002:10002 issuer.py /issuer/issuer.py
USER 10002:10002
ENTRYPOINT ["python3", "/issuer/issuer.py"]
