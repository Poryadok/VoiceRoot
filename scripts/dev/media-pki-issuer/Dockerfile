FROM alpine:3.24.1@sha256:79ff19e9084a00eece421b2523fb93e22d730e2c0e525905de047e848e56d95f

RUN apk add --no-cache openssl python3 \
    && addgroup -S -g 10001 voice-issuer \
    && adduser -S -D -H -u 10001 -G voice-issuer voice-issuer \
    && mkdir -p /state /opt/voice-media-pki \
    && chown 10001:10001 /state \
    && chmod 0700 /state

COPY issuer.py /opt/voice-media-pki/issuer.py
RUN chmod 0555 /opt/voice-media-pki/issuer.py

USER 10001:10001
WORKDIR /state
VOLUME ["/state"]
ENTRYPOINT ["python3", "/opt/voice-media-pki/issuer.py"]
